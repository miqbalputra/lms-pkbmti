package main

import (
	"encoding/json"
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

func TestR2FileSwapCrashWindowKeepsSafetyCopy(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "database.new")
	livePath := filepath.Join(dir, "database.live")
	oldPath := filepath.Join(dir, "database.old")
	if err := os.WriteFile(newPath, []byte("new-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := swapFileKeepingOld(newPath, livePath, oldPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(livePath); err != nil || string(got) != "new-db" {
		t.Fatalf("live database = %q, %v", got, err)
	}
	if got, err := os.ReadFile(oldPath); err != nil || string(got) != "old-db" {
		t.Fatalf("safety database = %q, %v", got, err)
	}
}

func TestR2DirectorySwapCrashWindowKeepsSafetyCopy(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "uploads.new")
	livePath := filepath.Join(dir, "uploads.live")
	oldPath := filepath.Join(dir, "uploads.old")
	if err := os.MkdirAll(newPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newPath, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := swapDirectoryKeepingOld(newPath, livePath, oldPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(livePath, "new.txt")); err != nil || string(got) != "new" {
		t.Fatalf("live uploads = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(oldPath, "old.txt")); err != nil || string(got) != "old" {
		t.Fatalf("safety uploads = %q, %v", got, err)
	}
}

func TestR2FileSwapAfterCompletedRenameIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "database.new")
	livePath := filepath.Join(dir, "database.live")
	oldPath := filepath.Join(dir, "database.old")
	if err := os.WriteFile(livePath, []byte("new-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old-db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := swapFileKeepingOld(newPath, livePath, oldPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(livePath); err != nil || string(got) != "new-db" {
		t.Fatalf("live database = %q, %v", got, err)
	}
	if got, err := os.ReadFile(oldPath); err != nil || string(got) != "old-db" {
		t.Fatalf("safety database = %q, %v", got, err)
	}
}

func TestR2DirectorySwapAfterCompletedRenameIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "uploads.new")
	livePath := filepath.Join(dir, "uploads.live")
	oldPath := filepath.Join(dir, "uploads.old")
	if err := os.MkdirAll(livePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(livePath, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := swapDirectoryKeepingOld(newPath, livePath, oldPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(livePath, "new.txt")); err != nil || string(got) != "new" {
		t.Fatalf("live uploads = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(oldPath, "old.txt")); err != nil || string(got) != "old" {
		t.Fatalf("safety uploads = %q, %v", got, err)
	}
}

func TestR2RestoreJournalRejectsPathsOutsideBackupVolume(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	if err := os.MkdirAll(backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside-restore")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "must-survive.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	j := r2RestoreJournal{
		JobID: "tampered", SourceKey: "ignored", SafetyArchive: filepath.Join(outside, "safety.enc"),
		WorkDir: outside, Dialect: "sqlite", Phase: "sqlite-staged",
	}
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r2RestoreJournalPath(), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadR2RestoreJournal(); err == nil {
		t.Fatal("tampered journal path was accepted")
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep" {
		t.Fatalf("outside sentinel changed: %q, %v", got, err)
	}
}
