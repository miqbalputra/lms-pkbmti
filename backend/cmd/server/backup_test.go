package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
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

func TestBackupBinaryNeverClobbersExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := newTestDB(t, filepath.Join(dir, "src.db"))
	dest := filepath.Join(dir, "existing.db")
	if err := os.WriteFile(dest, []byte("operator-safety-copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Server{db: src}).backupBinary(dest); err == nil {
		t.Fatal("backupBinary accepted an existing destination")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "operator-safety-copy" {
		t.Fatalf("existing destination was modified: %q", got)
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

func TestPreRestoreBackupPreservesCommittedWALRows(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))

	db := newTestDB(t, liveDBPath)
	if err := db.Create(&backupTestRow{ID: "wal-row", Name: "committed", Note: "must survive"}).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	safety, err := savePreRestoreBackup()
	if err != nil {
		t.Fatal(err)
	}
	safetyDB, err := gorm.Open(sqlite.Open("file:"+safety+testDSNPragma), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var row backupTestRow
	if err := safetyDB.First(&row, "id = ?", "wal-row").Error; err != nil {
		t.Fatalf("pre-restore safety copy lost committed WAL row: %v", err)
	}
	if row.Note != "must survive" {
		t.Fatalf("pre-restore safety copy changed row: %+v", row)
	}
	if safetySQLDB, err := safetyDB.DB(); err == nil {
		_ = safetySQLDB.Close()
	}
}

func TestPendingUploadSwapKeepsPreviousTree(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("UPLOADS_DIR", filepath.Join(dir, "uploads"))
	if err := os.MkdirAll(filepath.Join(uploadsDir(), "old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir(), "old", "file.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pendingUploadsPath, "new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pendingUploadsPath, "new", "file.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyPendingUploads(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(uploadsDir(), "new", "file.txt")); err != nil || string(got) != "new" {
		t.Fatalf("live uploads = %q, %v", got, err)
	}
	oldTrees, err := filepath.Glob(uploadsDir() + ".pre-restore-*")
	if err != nil || len(oldTrees) != 1 {
		t.Fatalf("expected one preserved previous upload tree, got %v, %v", oldTrees, err)
	}
	if got, err := os.ReadFile(filepath.Join(oldTrees[0], "old", "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("preserved uploads = %q, %v", got, err)
	}
}

func TestReplacingPendingRestorePreservesOlderArtifacts(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	if err := os.WriteFile(pendingDBPath, []byte("older-db-restore"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pendingSQLPath, []byte("older-sql-restore"), 0o600); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "new-restore.db")
	if err := os.WriteFile(tmp, []byte("new-restore"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replacePendingRestore(tmp, pendingDBPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(pendingDBPath); err != nil || string(got) != "new-restore" {
		t.Fatalf("new pending restore = %q, %v", got, err)
	}
	archived, err := filepath.Glob(filepath.Join(backupDir(), "pre-restore-pending-*"))
	if err != nil || len(archived) != 2 {
		t.Fatalf("expected two preserved pending restores, got %v, %v", archived, err)
	}
	contents := make(map[string]bool)
	for _, path := range archived {
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		contents[string(b)] = true
	}
	if !contents["older-db-restore"] || !contents["older-sql-restore"] {
		t.Fatalf("preserved pending restore contents = %#v", contents)
	}
}

func TestDownloadOffsiteBackupIsEncryptedAndPortable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BACKUP_ENCRYPTION_KEY", "offsite-test-key-that-is-long-enough-2026")
	src := newTestDB(t, filepath.Join(dir, "src.db"))
	src.Create(&backupTestRow{ID: "1", Name: "offsite", Note: "sensitive"})
	s := &Server{db: src}
	app := fiber.New()
	app.Get("/backup/offsite", s.downloadOffsiteBackup)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/backup/offsite?format=sql", nil))
	if err != nil {
		t.Fatalf("request offsite backup: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	_ = res.Body.Close()
	if bytes.Contains(body, []byte("sensitive")) {
		t.Fatal("offsite response contains plaintext database content")
	}
	if strings.Contains(res.Header.Get("Content-Disposition"), dir) {
		t.Fatal("offsite response exposed a local filesystem path")
	}
	encPath := filepath.Join(dir, "backup.sql.enc")
	plainPath := filepath.Join(dir, "backup.sql")
	if err := os.WriteFile(encPath, body, 0o600); err != nil {
		t.Fatalf("write encrypted response: %v", err)
	}
	if err := decryptBackupFile(encPath, plainPath, os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		t.Fatalf("decrypt response: %v", err)
	}
	if err := validateSQLBackup(plainPath); err != nil {
		t.Fatalf("validate decrypted SQL: %v", err)
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

func TestLatestAutomaticBackupAtIgnoresManualBackups(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	if err := ensureBackupDir(); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(backupDir(), "pkbm-lms-20990101-010101.db")
	automatic := filepath.Join(backupDir(), "pkbm-lms-20990102-010101-auto.db")
	for _, path := range []string{manual, automatic} {
		if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(automatic, want, want); err != nil {
		t.Fatal(err)
	}
	got, err := latestAutomaticBackupAt()
	if err != nil {
		t.Fatal(err)
	}
	if got.IsZero() || got.Sub(want).Abs() > time.Second {
		t.Fatalf("latest automatic backup time = %v, want %v", got, want)
	}
}

func TestPruneBackupsKeepsNewestAutomaticFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	if err := ensureBackupDir(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	files := []struct {
		name string
		at   time.Time
	}{
		{"pkbm-lms-20990101-010101-auto.db", now.Add(-3 * time.Hour)},
		{"pkbm-lms-20990102-010101-auto.db", now.Add(-2 * time.Hour)},
		{"pkbm-lms-20990103-010101-auto.db", now.Add(-time.Hour)},
		{"pkbm-lms-20990104-010101.db", now.Add(-4 * time.Hour)},
	}
	for _, file := range files {
		path := filepath.Join(backupDir(), file.name)
		if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, file.at, file.at); err != nil {
			t.Fatal(err)
		}
	}
	pruneBackups(2)
	if _, err := os.Stat(filepath.Join(backupDir(), files[0].name)); !os.IsNotExist(err) {
		t.Fatalf("oldest automatic backup was not pruned, err=%v", err)
	}
	for _, file := range files[1:] {
		if _, err := os.Stat(filepath.Join(backupDir(), file.name)); err != nil {
			t.Fatalf("backup %s should remain: %v", file.name, err)
		}
	}
}

func TestHealthPayloadReportsLocalBackupFreshness(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	t.Setenv("BACKUP_CRON", "0 2 * * *")
	if err := ensureBackupDir(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(backupDir(), "pkbm-lms-20990105-010101-auto.db")
	if err := os.WriteFile(path, []byte("verified backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t, filepath.Join(dir, "health.db"))
	s := &Server{db: db, startedAt: time.Now()}
	payload, healthy := s.healthPayload()
	if !healthy {
		t.Fatal("database health should be healthy")
	}
	backup, ok := payload["backup"].(fiber.Map)
	if !ok {
		t.Fatalf("health payload backup has unexpected type: %#v", payload["backup"])
	}
	if backup["localConfigured"] != true || backup["localHealthy"] != true {
		t.Fatalf("health payload did not report fresh local backup: %#v", backup)
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
