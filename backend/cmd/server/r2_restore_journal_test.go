package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestR2SQLiteJournalCompletesCompositeSwap(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	if err := os.WriteFile(liveDBPath, []byte("old-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(uploadsDir(), "old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir(), "old", "file.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(backupDir(), "r2-restore-job")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	j := &r2RestoreJournal{JobID: "job", SourceKey: "pkbm/archives/manual/a.tar.gz.enc", SafetyArchive: filepath.Join(work, "safety.enc"), WorkDir: work, Dialect: "sqlite", Phase: "sqlite-staged"}
	if err := os.WriteFile(journalDBNew(j), []byte("new-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(journalUploadsNew(j), "new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journalUploadsNew(j), "new", "file.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeR2RestoreJournal(j); err != nil {
		t.Fatal(err)
	}
	if err := recoverPendingR2Restore(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	loaded, err := loadR2RestoreJournal()
	if err != nil || loaded == nil || loaded.Phase != "completed" {
		t.Fatalf("journal = %#v, %v", loaded, err)
	}
	if got, err := os.ReadFile(liveDBPath); err != nil || string(got) != "new-db" {
		t.Fatalf("database = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(uploadsDir(), "new", "file.txt")); err != nil || string(got) != "new" {
		t.Fatalf("uploads = %q, %v", got, err)
	}
}

func TestR2SQLiteJournalRollbackKeepsPreviousData(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	work := filepath.Join(backupDir(), "r2-restore-job")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	j := &r2RestoreJournal{JobID: "job", SourceKey: "x", SafetyArchive: filepath.Join(work, "safety.enc"), WorkDir: work, Dialect: "sqlite", Phase: "sqlite-db-swapped"}
	if err := os.WriteFile(liveDBPath, []byte("bad-new-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalDBOld(j), []byte("old-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rollbackSQLiteJournal(j); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(liveDBPath); err != nil || string(got) != "old-db" {
		t.Fatalf("database = %q, %v", got, err)
	}
}
