package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedBackupRoundTripUsesBoundedChunks(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.sql")
	encPath := filepath.Join(dir, "source.sql.enc")
	outPath := filepath.Join(dir, "restored.sql")
	content := bytes.Repeat([]byte("0123456789abcdef\n"), offsiteChunkSize/16+123)
	if err := os.WriteFile(srcPath, content, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := encryptBackupFile(srcPath, encPath, "offsite-test-key-2026"); err != nil {
		t.Fatalf("encrypt backup: %v", err)
	}
	if err := decryptBackupFile(encPath, outPath, "offsite-test-key-2026"); err != nil {
		t.Fatalf("decrypt backup: %v", err)
	}
	restored, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if !bytes.Equal(content, restored) {
		t.Fatal("encrypted backup did not round-trip exactly")
	}
}

func TestUploadOffsiteBackupSendsEncryptedPayload(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "pkbm-lms-test.sql")
	receivedPath := filepath.Join(dir, "received.enc")
	content := []byte("CREATE TABLE test_rows (id text);\n")
	if err := os.WriteFile(srcPath, content, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("X-Backup-Name") != "pkbm-lms-test.sql.enc" {
			t.Fatalf("unexpected offsite request: %s %s", r.Method, r.Header.Get("X-Backup-Name"))
		}
		body, err := os.Create(receivedPath)
		if err != nil {
			t.Fatalf("create received file: %v", err)
		}
		_, copyErr := body.ReadFrom(r.Body)
		_ = body.Close()
		if copyErr != nil {
			t.Fatalf("copy received file: %v", copyErr)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("BACKUP_DIR", dir)
	t.Setenv("BACKUP_OFFSITE_URL", server.URL)
	t.Setenv("BACKUP_OFFSITE_METHOD", "PUT")
	t.Setenv("BACKUP_ENCRYPTION_KEY", "offsite-test-key-2026")
	uploaded, err := uploadOffsiteBackup(srcPath)
	if err != nil || !uploaded {
		t.Fatalf("upload offsite: uploaded=%v err=%v", uploaded, err)
	}
	restoredPath := filepath.Join(dir, "restored.sql")
	if err := decryptBackupFile(receivedPath, restoredPath, "offsite-test-key-2026"); err != nil {
		t.Fatalf("decrypt received payload: %v", err)
	}
	restored, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatalf("read restored payload: %v", err)
	}
	if !bytes.Equal(content, restored) {
		t.Fatal("offsite payload did not decrypt to original backup")
	}
}
