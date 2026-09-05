package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Run this only against the disposable MinIO service in CI or a dedicated R2
// test bucket. It is opt-in so local and production credentials are never
// contacted by an ordinary go test invocation.
func TestR2S3CompatibleIntegration(t *testing.T) {
	if os.Getenv("RUN_R2_INTEGRATION") != "1" {
		t.Skip("set RUN_R2_INTEGRATION=1 for MinIO/R2 integration")
	}
	client, err := r2Client(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bucket := os.Getenv("BACKUP_R2_BUCKET")
	ctx, cancel := context.WithTimeout(context.Background(), r2Timeout())
	defer cancel()
	if _, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}); err != nil {
		if _, createErr := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); createErr != nil {
			t.Fatalf("bucket unavailable: %v / %v", err, createErr)
		}
	}
	key := r2ArchivePrefix() + "integration/" + time.Now().UTC().Format("20060102T150405.000000000") + ".tar.gz.enc"
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("UPLOADS_DIR", "uploads")
	s := &Server{db: newTestDB(t, liveDBPath)}
	if err = s.db.Create(&backupTestRow{ID: "integration", Name: "R2", Note: "roundtrip"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join("uploads", "proof"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join("uploads", "proof", "attachment.txt"), []byte("pkbm-r2-integration"), 0o600); err != nil {
		t.Fatal(err)
	}
	enc, _, _, _, err := s.createR2Archive(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatalf("create full archive: %v", err)
	}
	input, err := os.Open(enc)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: input, Metadata: map[string]string{"test": "true", "format": "pkbm-r2-v1"}})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	}()
	listed, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(r2ArchivePrefix() + "integration/")})
	if err != nil || len(listed.Contents) == 0 {
		t.Fatalf("list: %v", err)
	}
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil || head.Metadata["test"] != "true" {
		t.Fatalf("head metadata: %#v, %v", head.Metadata, err)
	}
	obj, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer obj.Body.Close()
	downloaded := filepath.Join(dir, "download.tar.gz.enc")
	out, err := os.OpenFile(downloaded, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(out, obj.Body)
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("download: %v / %v", err, closeErr)
	}
	plain := filepath.Join(dir, "download.tar.gz")
	if err = decryptBackupFile(downloaded, plain, os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if _, err = extractR2Archive(plain, filepath.Join(dir, "extracted")); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "extracted", "uploads", "proof", "attachment.txt"))
	if err != nil || !bytes.Equal(got, []byte("pkbm-r2-integration")) {
		t.Fatalf("attachment = %q, %v", got, err)
	}
}
