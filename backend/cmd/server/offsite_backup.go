package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	offsiteBackupMagic = "PKBMENC1"
	offsiteChunkSize   = 4 * 1024 * 1024
)

func deriveBackupKey(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 16 {
		return nil, errors.New("BACKUP_ENCRYPTION_KEY must contain at least 16 characters")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

func offsiteEncryptionKey() ([]byte, error) {
	return deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY"))
}

func makeChunkNonce(base [12]byte, sequence uint64) []byte {
	nonce := base
	binary.BigEndian.PutUint64(nonce[4:], sequence)
	return nonce[:]
}

func chunkAAD(sequence uint64) []byte {
	var aad [16]byte
	copy(aad[:8], offsiteBackupMagic)
	binary.BigEndian.PutUint64(aad[8:], sequence)
	return aad[:]
}

// encryptBackupFile writes a streaming authenticated envelope. Each chunk is
// independently authenticated, keeping memory bounded for multi-gigabyte DBs.
func encryptBackupFile(srcPath, destPath, secret string) error {
	key, err := deriveBackupKey(secret)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	removeOnError := true
	defer func() {
		_ = out.Close()
		if removeOnError {
			_ = os.Remove(destPath)
		}
	}()
	if _, err := io.WriteString(out, offsiteBackupMagic); err != nil {
		return err
	}
	var baseNonce [12]byte
	if _, err := io.ReadFull(rand.Reader, baseNonce[:]); err != nil {
		return err
	}
	if _, err := out.Write(baseNonce[:]); err != nil {
		return err
	}
	buffer := make([]byte, offsiteChunkSize)
	for sequence := uint64(0); ; sequence++ {
		n, readErr := io.ReadFull(in, buffer)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		if n > 0 {
			sealed := gcm.Seal(nil, makeChunkNonce(baseNonce, sequence), buffer[:n], chunkAAD(sequence))
			if uint64(len(sealed)) > uint64(^uint32(0)) {
				return errors.New("encrypted backup chunk is too large")
			}
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(len(sealed)))
			if _, err := out.Write(length[:]); err != nil {
				return err
			}
			if _, err := out.Write(sealed); err != nil {
				return err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	removeOnError = false
	return nil
}

func decryptBackupFile(srcPath, destPath, secret string) error {
	key, err := deriveBackupKey(secret)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	header := make([]byte, len(offsiteBackupMagic))
	if _, err := io.ReadFull(in, header); err != nil || string(header) != offsiteBackupMagic {
		return errors.New("invalid encrypted backup header")
	}
	var baseNonce [12]byte
	if _, err := io.ReadFull(in, baseNonce[:]); err != nil {
		return errors.New("encrypted backup is truncated")
	}
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	removeOnError := true
	defer func() {
		_ = out.Close()
		if removeOnError {
			_ = os.Remove(destPath)
		}
	}()
	for sequence := uint64(0); ; sequence++ {
		var length [4]byte
		_, readErr := io.ReadFull(in, length[:])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return errors.New("encrypted backup is truncated")
		}
		sealedLength := binary.BigEndian.Uint32(length[:])
		if sealedLength < uint32(gcm.Overhead()) || sealedLength > uint32(offsiteChunkSize+gcm.Overhead()) {
			return errors.New("encrypted backup chunk length is invalid")
		}
		sealed := make([]byte, sealedLength)
		if _, err := io.ReadFull(in, sealed); err != nil {
			return errors.New("encrypted backup is truncated")
		}
		plain, err := gcm.Open(nil, makeChunkNonce(baseNonce, sequence), sealed, chunkAAD(sequence))
		if err != nil {
			return errors.New("encrypted backup authentication failed")
		}
		if _, err := out.Write(plain); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	removeOnError = false
	return nil
}

func saveRestoreUploadForRestore(c *fiber.Ctx, fh *multipart.FileHeader) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".enc" {
		path, err := saveRestoreUpload(c, fh)
		return path, ext, err
	}
	innerName := strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename))
	innerExt := strings.ToLower(filepath.Ext(innerName))
	if innerExt != ".db" && innerExt != ".sql" {
		return "", "", fiber.NewError(400, "file .enc harus berasal dari backup .db.enc atau .sql.enc")
	}
	uploadedPath, err := saveRestoreUpload(c, fh)
	if err != nil {
		return "", "", err
	}
	secret := os.Getenv("BACKUP_ENCRYPTION_KEY")
	if _, err := deriveBackupKey(secret); err != nil {
		_ = os.Remove(uploadedPath)
		return "", "", fiber.NewError(503, "BACKUP_ENCRYPTION_KEY belum dikonfigurasi untuk restore terenkripsi")
	}
	decrypted, err := os.CreateTemp(".", "pkbm-restore-decrypted-*")
	if err != nil {
		_ = os.Remove(uploadedPath)
		return "", "", fiber.NewError(500, "tidak dapat membuat file restore sementara")
	}
	decryptedPath := decrypted.Name()
	if err := decrypted.Close(); err != nil {
		_ = os.Remove(uploadedPath)
		_ = os.Remove(decryptedPath)
		return "", "", fiber.NewError(500, "tidak dapat menyiapkan file restore sementara")
	}
	if err := decryptBackupFile(uploadedPath, decryptedPath, secret); err != nil {
		_ = os.Remove(uploadedPath)
		_ = os.Remove(decryptedPath)
		return "", "", fiber.NewError(400, "file backup terenkripsi tidak valid")
	}
	_ = os.Remove(uploadedPath)
	return decryptedPath, innerExt, nil
}

func offsiteUploadTimeout() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(env("BACKUP_OFFSITE_TIMEOUT", "5m")))
	if err != nil || d < 30*time.Second {
		return 5 * time.Minute
	}
	return d
}

// uploadOffsiteBackup encrypts a local backup and sends it to an optional
// presigned S3 URL, n8n gateway, or other HTTPS-compatible archival endpoint.
// The local backup remains the source of truth if the remote target is down.
func uploadOffsiteBackup(srcPath string) (bool, error) {
	destination := strings.TrimSpace(os.Getenv("BACKUP_OFFSITE_URL"))
	if destination == "" {
		return false, nil
	}
	secret := os.Getenv("BACKUP_ENCRYPTION_KEY")
	if _, err := deriveBackupKey(secret); err != nil {
		return false, err
	}
	if err := os.MkdirAll(backupDir(), 0o700); err != nil {
		return false, err
	}
	name := filepath.Base(srcPath) + ".enc"
	tmp, err := os.CreateTemp(backupDir(), ".offsite-*.enc")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	defer os.Remove(tmpPath)
	if err := encryptBackupFile(srcPath, tmpPath, secret); err != nil {
		return false, err
	}
	in, err := os.Open(tmpPath)
	if err != nil {
		return false, err
	}
	defer in.Close()
	method := strings.ToUpper(strings.TrimSpace(env("BACKUP_OFFSITE_METHOD", "PUT")))
	if method != http.MethodPut && method != http.MethodPost {
		return false, fmt.Errorf("BACKUP_OFFSITE_METHOD must be PUT or POST")
	}
	req, err := http.NewRequest(method, destination, in)
	if err != nil {
		return false, err
	}
	if stat, err := in.Stat(); err == nil {
		req.ContentLength = stat.Size()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Backup-Name", name)
	if token := strings.TrimSpace(os.Getenv("BACKUP_OFFSITE_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: offsiteUploadTimeout()}).Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf("offsite endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return true, nil
}

// verifyBackupArtifact performs an integrity check before a scheduled backup
// is reported as successful. PostgreSQL gets a real restore drill when an
// isolated BACKUP_DRILL_DATABASE_URL is configured; otherwise the file is
// still checked for existence and non-zero size.
func verifyBackupArtifact(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if stat.Size() == 0 {
		return false, errors.New("backup file is empty")
	}
	if isSQLite() {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".db":
			return true, validateSQLiteBackup(path)
		case ".sql":
			return true, validateSQLBackup(path)
		default:
			return false, errors.New("unsupported SQLite backup extension")
		}
	}
	drillURL := strings.TrimSpace(os.Getenv("BACKUP_DRILL_DATABASE_URL"))
	if drillURL == "" {
		return false, nil
	}
	if strings.ToLower(filepath.Ext(path)) != ".sql" {
		return false, errors.New("PostgreSQL restore drill requires a .sql backup")
	}
	return true, pgRestoreTo(path, drillURL)
}
