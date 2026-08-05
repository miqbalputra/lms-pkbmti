package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// backupTestRow is a small table used only by the backup round-trip tests. It
// exercises text columns, a value with an embedded quote, a value with a
// newline+semicolon (to stress the SQL splitter), and a NULL.
type backupTestRow struct {
	ID   string `gorm:"primaryKey"`
	Name string
	Note string
}

func (backupTestRow) TableName() string { return "backup_test_rows" }

const testDSNPragma = "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"

func newTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+path+testDSNPragma), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&backupTestRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestBackupBinaryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := newTestDB(t, filepath.Join(dir, "src.db"))
	src.Create(&backupTestRow{ID: "1", Name: "alpha", Note: "o'brien"})
	src.Create(&backupTestRow{ID: "2", Name: "beta", Note: "line1\nline2"})
	src.Create(&backupTestRow{ID: "3", Name: "gamma", Note: ""})

	s := &Server{db: src}
	snapPath := filepath.Join(dir, "snap.db")
	if err := s.backupBinary(snapPath); err != nil {
		t.Fatalf("backupBinary: %v", err)
	}

	// The snapshot must be a complete, readable SQLite DB with all rows.
	snap, err := gorm.Open(sqlite.Open("file:"+snapPath+testDSNPragma), &gorm.Config{})
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := snap.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	var rows []backupTestRow
	snap.Find(&rows)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows in snapshot, got %d", len(rows))
	}
	want := map[string]string{"1": "o'brien", "2": "line1\nline2", "3": ""}
	for _, r := range rows {
		if r.Note != want[r.ID] {
			t.Errorf("row %s note = %q, want %q", r.ID, r.Note, want[r.ID])
		}
	}
}

func TestBackupSQLDumpRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := newTestDB(t, filepath.Join(dir, "src.db"))
	src.Create(&backupTestRow{ID: "1", Name: "alpha", Note: "o'brien"})
	src.Create(&backupTestRow{ID: "2", Name: "beta", Note: "a;b;c"})
	src.Create(&backupTestRow{ID: "3", Name: "gamma", Note: ""})

	s := &Server{db: src}
	var buf bytes.Buffer
	if err := s.dumpSQL(&buf); err != nil {
		t.Fatalf("dumpSQL: %v", err)
	}
	dump := buf.String()

	// A real dump must wrap in a transaction and include at least the table DDL
	// and an INSERT per row.
	if !contains(dump, "BEGIN TRANSACTION;") || !contains(dump, "COMMIT;") {
		t.Fatalf("dump missing transaction wrapper:\n%s", dump)
	}
	if !contains(dump, "INSERT INTO \"backup_test_rows\"") {
		t.Fatalf("dump missing INSERT:\n%s", dump)
	}

	// Replay the dump into a fresh DB exactly as applySQLRestore does.
	destPath := filepath.Join(dir, "restored.db")
	destDB, err := gorm.Open(sqlite.Open("file:"+destPath+testDSNPragma), &gorm.Config{})
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := destDB.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	sqlDB, err := destDB.DB()
	if err != nil {
		t.Fatalf("dest sqlDB: %v", err)
	}
	for _, stmt := range splitSQLStatements(dump) {
		stmt = trimSpace(stmt)
		if stmt == "" || hasPrefix(stmt, "--") {
			continue
		}
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("exec dump stmt failed: %v\nstmt: %q", err, stmt)
		}
	}

	var rows []backupTestRow
	destDB.Find(&rows)
	if len(rows) != 3 {
		t.Fatalf("expected 3 restored rows, got %d", len(rows))
	}
	want := map[string]string{"1": "o'brien", "2": "a;b;c", "3": ""}
	for _, r := range rows {
		if r.Note != want[r.ID] {
			t.Errorf("restored row %s note = %q, want %q", r.ID, r.Note, want[r.ID])
		}
	}
}

func TestUploadedBackupValidation(t *testing.T) {
	dir := t.TempDir()
	src := newTestDB(t, filepath.Join(dir, "src.db"))
	src.Create(&backupTestRow{ID: "1", Name: "full", Note: "validated"})
	s := &Server{db: src}

	dbPath := filepath.Join(dir, "full.db")
	if err := s.backupBinary(dbPath); err != nil {
		t.Fatalf("backupBinary: %v", err)
	}
	if err := validateSQLiteBackup(dbPath); err != nil {
		t.Fatalf("validateSQLiteBackup: %v", err)
	}

	var dump bytes.Buffer
	if err := s.dumpSQL(&dump); err != nil {
		t.Fatalf("dumpSQL: %v", err)
	}
	sqlPath := filepath.Join(dir, "full.sql")
	if err := os.WriteFile(sqlPath, dump.Bytes(), 0o600); err != nil {
		t.Fatalf("write SQL backup: %v", err)
	}
	if err := validateSQLBackup(sqlPath); err != nil {
		t.Fatalf("validateSQLBackup: %v", err)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	in := "INSERT INTO t VALUES('it''s');-- c\nINSERT INTO t VALUES('a;b');/* x ; y */INSERT INTO t VALUES(1);"
	out := splitSQLStatements(in)
	// Three top-level statements: the semicolons inside the quoted string and
	// inside the block comment must NOT split.
	if len(out) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(out), out)
	}
	for i, s := range out {
		if !hasSuffix(s, ";") {
			t.Errorf("statement %d does not end with ';': %q", i, s)
		}
	}
}

// tiny helpers to avoid pulling strings just for the test's readability
func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return s
}
func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func hasSuffix(s, p string) bool { return len(s) >= len(p) && s[len(s)-len(p):] == p }
