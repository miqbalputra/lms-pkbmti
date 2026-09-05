package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestR2ArchiveContainsDatabaseUploadsAndManifest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BACKUP_ENCRYPTION_KEY", "r2-archive-test-key-2026")
	s := &Server{db: newTestDB(t, liveDBPath)}
	if err := s.db.Create(&backupTestRow{ID: "r2", Name: "included", Note: "backup"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("uploads", "materi"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("uploads", "materi", "catatan.txt"), []byte("lampiran"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	enc, manifest, _, _, err := s.createR2Archive(work)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("files in manifest = %d, want database plus upload", len(manifest.Files))
	}
	plain := filepath.Join(dir, "archive.tar.gz")
	if err := decryptBackupFile(enc, plain, "r2-archive-test-key-2026"); err != nil {
		t.Fatalf("decrypt archive: %v", err)
	}
	extracted := filepath.Join(dir, "extracted")
	if _, err := extractR2Archive(plain, extracted); err != nil {
		t.Fatalf("extract archive: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(extracted, "uploads", "materi", "catatan.txt")); err != nil || string(got) != "lampiran" {
		t.Fatalf("upload missing after archive roundtrip: %q %v", got, err)
	}
	if err := validateSQLiteBackup(filepath.Join(extracted, "database.db")); err != nil {
		t.Fatalf("database missing after archive roundtrip: %v", err)
	}
}

func TestR2ArchivePathGuard(t *testing.T) {
	for _, path := range []string{"../secret", "/absolute", "uploads/../../secret", ""} {
		if _, err := safeArchivePath(path); err == nil {
			t.Fatalf("unsafe path %q accepted", path)
		}
	}
}
