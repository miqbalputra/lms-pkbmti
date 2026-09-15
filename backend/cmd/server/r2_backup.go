package main

// Cloudflare R2 full backup support. R2 implements the S3 API; the browser
// never receives its credentials. An R2 object contains one encrypted tar.gz
// with a database dump, every application upload, and an integrity manifest.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const r2ArchiveVersion = 1

type R2BackupJob struct {
	Base
	Kind            string     `gorm:"index;not null" json:"kind"`   // manual, scheduled, restore
	Status          string     `gorm:"index;not null" json:"status"` // queued, running, succeeded, failed
	ObjectKey       string     `gorm:"index" json:"objectKey,omitempty"`
	SourceKey       string     `json:"sourceKey,omitempty"`
	Size            int64      `json:"size"`
	FileCount       int        `json:"fileCount"`
	Checksum        string     `json:"checksum,omitempty"`
	Phase           string     `json:"phase,omitempty"`
	SafetyObjectKey string     `json:"safetyObjectKey,omitempty"`
	RecoveredAt     *time.Time `json:"recoveredAt,omitempty"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	Error           string     `json:"error,omitempty"`
}

type r2Coordinator struct {
	mu          sync.Mutex
	active      bool
	maintenance atomic.Bool
}

type r2ManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type r2Manifest struct {
	Version   int              `json:"version"`
	CreatedAt time.Time        `json:"createdAt"`
	Dialect   string           `json:"dialect"`
	Database  string           `json:"database"`
	Files     []r2ManifestFile `json:"files"`
}

type r2ArchiveInfo struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
	Automatic bool      `json:"automatic"`
}

func r2Enabled() bool { return strings.TrimSpace(os.Getenv("BACKUP_R2_BUCKET")) != "" }

func r2Endpoint() string {
	// A custom endpoint is intentionally test/development-only, so a production
	// deployment always talks to the account-bound Cloudflare R2 endpoint.
	if env("APP_ENV", "development") != "production" {
		if endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("BACKUP_R2_ENDPOINT")), "/"); endpoint != "" {
			return endpoint
		}
	}
	return "https://" + strings.TrimSpace(os.Getenv("BACKUP_R2_ACCOUNT_ID")) + ".r2.cloudflarestorage.com"
}

func r2Prefix() string {
	p := strings.Trim(strings.TrimSpace(env("BACKUP_R2_PREFIX", "pkbm-lms")), "/")
	if p == "" {
		return "pkbm-lms"
	}
	return p
}
func r2RetentionDays() int {
	days, err := strconv.Atoi(env("BACKUP_R2_RETENTION_DAYS", "36"))
	if err != nil || days < 3 || days > 3650 {
		return 36
	}
	return days
}
func uploadsDir() string      { return env("UPLOADS_DIR", "uploads") }
func r2ArchivePrefix() string { return r2Prefix() + "/archives/" }

func r2ConfigError() error {
	if !r2Enabled() {
		return errors.New("backup R2 belum dikonfigurasi")
	}
	for _, k := range []string{"BACKUP_R2_ACCOUNT_ID", "BACKUP_R2_BUCKET", "BACKUP_R2_ACCESS_KEY_ID", "BACKUP_R2_SECRET_ACCESS_KEY"} {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			return fmt.Errorf("%s wajib diisi", k)
		}
	}
	_, err := deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY"))
	return err
}

func r2Timeout() time.Duration {
	v, err := time.ParseDuration(env("BACKUP_R2_TIMEOUT", "2m"))
	if err != nil || v <= 0 || v > 15*time.Minute {
		return 2 * time.Minute
	}
	return v
}

func backupArchiveLimit() int64 {
	mb, err := strconv.ParseInt(strings.TrimSpace(env("BACKUP_MAX_ARCHIVE_MB", "4096")), 10, 64)
	if err != nil || mb < 512 {
		mb = 4096
	}
	if mb > 32768 {
		mb = 32768
	}
	return mb * 1024 * 1024
}

func r2Client(ctx context.Context) (*s3.Client, error) {
	if err := r2ConfigError(); err != nil {
		return nil, err
	}
	endpoint := r2Endpoint()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithHTTPClient(&http.Client{Timeout: r2Timeout()}),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(os.Getenv("BACKUP_R2_ACCESS_KEY_ID")), strings.TrimSpace(os.Getenv("BACKUP_R2_SECRET_ACCESS_KEY")), "")),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true }), nil
}

func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func safeArchivePath(p string) (string, error) {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return "", errors.New("path arsip tidak aman")
	}
	return p, nil
}

func addArchiveFile(tw *tar.Writer, diskPath, archivePath string, files *[]r2ManifestFile) error {
	archivePath, err := safeArchivePath(archivePath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(diskPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("backup hanya menerima file biasa")
	}
	checksum, size, err := sha256File(diskPath)
	if err != nil {
		return err
	}
	h := &tar.Header{Name: archivePath, Mode: 0o600, Size: size, ModTime: info.ModTime()}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(tw, f); err != nil {
		return err
	}
	*files = append(*files, r2ManifestFile{Path: archivePath, Size: size, SHA256: checksum})
	return nil
}

// createR2Archive snapshots the database first, then packs a deterministic
// view of uploads. Upload writers retain their normal file-rename behavior;
// database dumps are already consistent snapshots for both supported engines.
func (s *Server) createR2Archive(workDir string) (string, r2Manifest, string, int64, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	dbName := "database.sql"
	if isSQLite() {
		dbName = "database.db"
	}
	dbPath := filepath.Join(workDir, dbName)
	if isSQLite() {
		if err := s.backupBinary(dbPath); err != nil {
			return "", r2Manifest{}, "", 0, err
		}
	} else if err := pgDump(dbPath); err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	if _, err := verifyBackupArtifact(dbPath); err != nil {
		return "", r2Manifest{}, "", 0, fmt.Errorf("verifikasi database: %w", err)
	}

	tarPath := filepath.Join(workDir, "backup.tar.gz")
	out, err := os.OpenFile(tarPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	manifest := r2Manifest{Version: r2ArchiveVersion, CreatedAt: time.Now().UTC(), Dialect: dialect(), Database: dbName}
	var archiveBytes int64
	closeWithError := func(e error) error { _ = tw.Close(); _ = gz.Close(); _ = out.Close(); return e }
	if err := addArchiveFile(tw, dbPath, dbName, &manifest.Files); err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
	}
	archiveBytes = manifest.Files[len(manifest.Files)-1].Size
	if archiveBytes > backupArchiveLimit() {
		return "", r2Manifest{}, "", 0, closeWithError(errors.New("ukuran database melebihi batas arsip backup"))
	}
	var paths []string
	root := uploadsDir()
	if _, err := os.Stat(root); err == nil {
		err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("symlink tidak diizinkan dalam uploads")
			}
			if info.Mode().IsRegular() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", r2Manifest{}, "", 0, closeWithError(err)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", r2Manifest{}, "", 0, closeWithError(err)
		}
		if err := addArchiveFile(tw, path, filepath.ToSlash(filepath.Join("uploads", rel)), &manifest.Files); err != nil {
			return "", r2Manifest{}, "", 0, closeWithError(err)
		}
		archiveBytes += manifest.Files[len(manifest.Files)-1].Size
		if archiveBytes > backupArchiveLimit() {
			return "", r2Manifest{}, "", 0, closeWithError(errors.New("ukuran data melebihi batas arsip backup"))
		}
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifestJSON)), ModTime: manifest.CreatedAt}); err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
	}
	if _, err := tw.Write(manifestJSON); err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
	}
	if err := tw.Close(); err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		return "", r2Manifest{}, "", 0, err
	}
	if err := out.Close(); err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	encPath := tarPath + ".enc"
	if err := encryptBackupFile(tarPath, encPath, os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	checksum, size, err := sha256File(encPath)
	if err != nil {
		return "", r2Manifest{}, "", 0, err
	}
	return encPath, manifest, checksum, size, nil
}

func (s *Server) uploadR2Archive(ctx context.Context, path string, kind string) (string, error) {
	client, err := r2Client(ctx)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	key := fmt.Sprintf("%s%s/%s-%s.tar.gz.enc", r2ArchivePrefix(), kind, wibTimeFormat(time.Now(), "20060102-150405"), newUUID())
	uploader := manager.NewUploader(client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{Bucket: aws.String(os.Getenv("BACKUP_R2_BUCKET")), Key: aws.String(key), Body: f, ContentType: aws.String("application/octet-stream"), Metadata: map[string]string{"format": "pkbm-r2-v1", "kind": kind}})
	return key, err
}

func newUUID() string { return uuid.NewString() }

func (s *Server) updateR2Job(job *R2BackupJob, fields map[string]interface{}) {
	_ = s.db.Model(job).Updates(fields).Error
}

func (s *Server) updateR2Phase(job *R2BackupJob, phase string) {
	job.Phase = phase
	s.updateR2Job(job, map[string]interface{}{"phase": phase})
}

func (s *Server) startR2Backup(kind string, sourceKey string) (*R2BackupJob, error) {
	if err := r2ConfigError(); err != nil {
		return nil, fiber.NewError(503, "R2 belum siap: "+err.Error())
	}
	s.r2.mu.Lock()
	defer s.r2.mu.Unlock()
	if s.r2.active {
		return nil, fiber.NewError(409, "backup atau restore lain masih berjalan")
	}
	job := &R2BackupJob{Kind: kind, Status: "queued", SourceKey: sourceKey}
	if err := s.db.Create(job).Error; err != nil {
		return nil, err
	}
	s.r2.active = true
	go func() { defer func() { s.r2.mu.Lock(); s.r2.active = false; s.r2.mu.Unlock() }(); s.runR2Backup(job) }()
	return job, nil
}

func (s *Server) runR2Backup(job *R2BackupJob) {
	now := time.Now()
	job.Status = "running"
	s.updateR2Job(job, map[string]interface{}{"status": "running", "phase": "archiving", "started_at": &now, "error": ""})
	if err := ensureBackupDir(); err != nil {
		s.finishR2Job(job, err)
		return
	}
	work, err := os.MkdirTemp(backupDir(), "r2-backup-*")
	if err == nil {
		defer os.RemoveAll(work)
	}
	if err != nil {
		s.finishR2Job(job, err)
		return
	}
	enc, manifest, checksum, size, err := s.createR2Archive(work)
	if err == nil {
		s.updateR2Phase(job, "uploading")
		var key string
		key, err = s.uploadR2Archive(context.Background(), enc, job.Kind)
		if err == nil {
			job.ObjectKey = key
			s.updateR2Job(job, map[string]interface{}{"object_key": key, "size": size, "file_count": len(manifest.Files), "checksum": checksum})
		}
	}
	if err != nil {
		s.finishR2Job(job, err)
		return
	}
	finished := time.Now()
	job.Status, job.Phase = "succeeded", "completed"
	s.updateR2Job(job, map[string]interface{}{"status": "succeeded", "phase": "completed", "finished_at": &finished})
	s.metrics.recordSuccess(true, true)
	s.audit(nil, "backup_r2", "backup", job.ObjectKey)
}

func (s *Server) finishR2Job(job *R2BackupJob, err error) {
	finished := time.Now()
	job.Status, job.Phase = "failed", "failed"
	s.updateR2Job(job, map[string]interface{}{"status": "failed", "phase": "failed", "finished_at": &finished, "error": safeOperationError(err)})
	s.metrics.recordFailure()
	event := "backup_failed"
	if job.Kind == "restore" {
		event = "restore_failed"
	}
	s.notifyOperation(event, event+":"+job.Kind, "Backup atau restore R2 gagal. Periksa dashboard backup.", job)
	operationLog("r2_job_failed", map[string]any{"kind": job.Kind, "jobId": job.ID})
}
func safeBackupError(err error) string {
	if err == nil {
		return ""
	}
	return "Operasi backup gagal: " + strings.TrimSpace(err.Error())
}

func (s *Server) listR2Archives(c *fiber.Ctx) error {
	client, err := r2Client(c.Context())
	if err != nil {
		return fiber.NewError(503, "R2 belum siap")
	}
	pageSize := int32(50)
	if raw := strings.TrimSpace(c.Query("pageSize")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 1 && parsed <= 100 {
			pageSize = int32(parsed)
		}
	}
	input := &s3.ListObjectsV2Input{Bucket: aws.String(os.Getenv("BACKUP_R2_BUCKET")), Prefix: aws.String(r2ArchivePrefix()), MaxKeys: aws.Int32(pageSize)}
	if pageToken := strings.TrimSpace(c.Query("pageToken")); pageToken != "" {
		if len(pageToken) > 2048 {
			return fiber.NewError(400, "page token tidak valid")
		}
		input.ContinuationToken = aws.String(pageToken)
	}
	paginator := s3.NewListObjectsV2Paginator(client, input)
	if !paginator.HasMorePages() {
		return c.JSON(fiber.Map{"archives": []r2ArchiveInfo{}})
	}
	out, err := paginator.NextPage(c.Context())
	if err != nil {
		return fiber.NewError(502, "daftar backup R2 tidak dapat dimuat")
	}
	items := make([]r2ArchiveInfo, 0, len(out.Contents))
	for _, o := range out.Contents {
		if o.Key != nil && strings.HasSuffix(*o.Key, ".tar.gz.enc") {
			items = append(items, r2ArchiveInfo{Key: *o.Key, CreatedAt: aws.ToTime(o.LastModified), Size: aws.ToInt64(o.Size), Automatic: strings.Contains(*o.Key, "/scheduled/")})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return c.JSON(fiber.Map{"archives": items, "nextPageToken": aws.ToString(out.NextContinuationToken)})
}

func (s *Server) r2Status(c *fiber.Ctx) error {
	offsiteConfigured := strings.TrimSpace(os.Getenv("BACKUP_OFFSITE_URL")) != ""
	_, offsiteKeyErr := deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY"))
	offsiteKeyOK := offsiteKeyErr == nil
	status := fiber.Map{
		"enabled": r2Enabled(), "prefix": r2Prefix(), "retentionDays": r2RetentionDays(),
		"schedule": "02:00 WIB, setiap 72 jam", "maintenance": s.r2.maintenance.Load(),
		"offsiteConfigured": offsiteConfigured, "offsiteEncrypted": !offsiteConfigured || offsiteKeyOK,
		"offsiteTransport": "HTTP gateway (Google Drive / S3-compatible)",
	}
	var last R2BackupJob
	if s.db.Where("kind = ? AND status = ?", "scheduled", "succeeded").Order("finished_at desc").First(&last).Error == nil && last.FinishedAt != nil {
		age := time.Since(*last.FinishedAt)
		status["lastAutomaticBackupAt"] = wibTimeFormat(*last.FinishedAt, time.RFC3339)
		status["backupAgeHours"] = int64(age.Hours())
		status["backupHealthy"] = age <= backupAlertMaxAge()
	} else if r2Enabled() {
		status["backupHealthy"] = false
	}
	if r2Enabled() {
		status["bucket"] = os.Getenv("BACKUP_R2_BUCKET")
		status["configured"] = r2ConfigError() == nil
	}
	return c.JSON(status)
}
func (s *Server) testR2(c *fiber.Ctx) error {
	client, err := r2Client(c.Context())
	if err != nil {
		return fiber.NewError(503, "R2 belum siap")
	}
	_, err = client.ListObjectsV2(c.Context(), &s3.ListObjectsV2Input{Bucket: aws.String(os.Getenv("BACKUP_R2_BUCKET")), Prefix: aws.String(r2ArchivePrefix()), MaxKeys: aws.Int32(1)})
	if err != nil {
		return fiber.NewError(502, "koneksi R2 gagal")
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (s *Server) createR2BackupHandler(c *fiber.Ctx) error {
	job, err := s.startR2Backup("manual", "")
	if err != nil {
		return err
	}
	return c.Status(202).JSON(job)
}
func (s *Server) listR2Jobs(c *fiber.Ctx) error {
	var jobs []R2BackupJob
	if err := s.db.Order("created_at desc").Limit(20).Find(&jobs).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"jobs": jobs})
}

func (s *Server) shouldRunR2Backup() bool {
	var last R2BackupJob
	if s.db.Where("kind = ? AND status = ?", "scheduled", "succeeded").Order("finished_at desc").First(&last).Error != nil {
		return true
	}
	return last.FinishedAt == nil || time.Since(*last.FinishedAt) >= 72*time.Hour
}
func (s *Server) enqueueScheduledR2Backup() {
	if r2Enabled() && s.shouldRunR2Backup() {
		if _, err := s.startR2Backup("scheduled", ""); err != nil {
			fmt.Printf("scheduled R2 backup skipped: %v\n", err)
		}
	}
}

func (s *Server) restoreR2ArchiveHandler(c *fiber.Ctx) error {
	var input struct {
		Key          string `json:"key"`
		Confirmation string `json:"confirmation"`
	}
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(400, "payload restore tidak valid")
	}
	if input.Key == "" || input.Confirmation != input.Key || !strings.HasPrefix(input.Key, r2ArchivePrefix()) || !strings.HasSuffix(input.Key, ".tar.gz.enc") {
		return fiber.NewError(400, "konfirmasi nama backup tidak cocok")
	}
	job, err := s.startR2Restore(input.Key)
	if err != nil {
		return err
	}
	return c.Status(202).JSON(job)
}

func (s *Server) startR2Restore(key string) (*R2BackupJob, error) {
	if err := r2ConfigError(); err != nil {
		return nil, fiber.NewError(503, "R2 belum siap: "+err.Error())
	}
	s.r2.mu.Lock()
	defer s.r2.mu.Unlock()
	if s.r2.active {
		return nil, fiber.NewError(409, "backup atau restore lain masih berjalan")
	}
	job := &R2BackupJob{Kind: "restore", Status: "queued", SourceKey: key}
	if err := s.db.Create(job).Error; err != nil {
		return nil, err
	}
	s.r2.active = true
	go func() { defer func() { s.r2.mu.Lock(); s.r2.active = false; s.r2.mu.Unlock() }(); s.runR2Restore(job) }()
	return job, nil
}

// stageLocalFullRestore accepts the same encrypted full archive produced by
// the local, offsite, and R2 backup flows.  It intentionally shares the R2
// coordinator: restoring a database and uploads is a single destructive
// operation, so a second backup/restore must never run concurrently.
func (s *Server) stageLocalFullRestore(c *fiber.Ctx) error {
	if _, err := deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		return fiber.NewError(503, "BACKUP_ENCRYPTION_KEY belum dikonfigurasi untuk restore backup lengkap")
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "file backup lengkap wajib diunggah (field name=file)")
	}
	if !isFullBackupArchiveName(fh.Filename) {
		return fiber.NewError(400, "file backup lengkap harus berekstensi .tar.gz.enc")
	}
	if fh.Size <= 0 || fh.Size > backupArchiveLimit() {
		return fiber.NewError(413, "ukuran file backup lengkap tidak valid atau melebihi batas")
	}
	if err := ensureBackupDir(); err != nil {
		return fiber.NewError(500, "tidak dapat menyiapkan direktori backup")
	}

	s.r2.mu.Lock()
	defer s.r2.mu.Unlock()
	if s.r2.active {
		return fiber.NewError(409, "backup atau restore lain masih berjalan")
	}
	job := &R2BackupJob{Kind: "restore-local", Status: "queued", SourceKey: "local-upload"}
	if err := s.db.Create(job).Error; err != nil {
		return err
	}
	work := filepath.Join(backupDir(), "r2-restore-"+job.ID)
	archivePath := filepath.Join(work, "uploaded.tar.gz.enc")
	if err := os.MkdirAll(work, 0o700); err != nil {
		_ = s.db.Delete(job).Error
		return fiber.NewError(500, "tidak dapat menyiapkan restore backup lengkap")
	}
	if err := c.SaveFile(fh, archivePath); err != nil {
		_ = os.RemoveAll(work)
		_ = s.db.Delete(job).Error
		return fiber.NewError(500, "gagal menyimpan backup lengkap untuk restore")
	}
	secureBackupFile(archivePath)
	s.r2.active = true
	go func() {
		defer func() {
			s.r2.mu.Lock()
			s.r2.active = false
			s.r2.mu.Unlock()
		}()
		s.runLocalFullRestore(job, work, archivePath)
	}()
	return c.Status(202).JSON(job)
}

func (s *Server) downloadR2Object(ctx context.Context, key, dest string) error {
	client, err := r2Client(ctx)
	if err != nil {
		return err
	}
	obj, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(os.Getenv("BACKUP_R2_BUCKET")), Key: aws.String(key)})
	if err != nil {
		return err
	}
	defer obj.Body.Close()
	if obj.ContentLength != nil && *obj.ContentLength > backupArchiveLimit() {
		return fmt.Errorf("objek backup melebihi batas arsip yang diizinkan")
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, obj.Body)
	return err
}

func extractR2Archive(tarPath, dest string) (r2Manifest, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return r2Manifest{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return r2Manifest{}, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var manifest r2Manifest
	found := false
	var extractedBytes int64
	const maxArchiveFiles = 100000
	fileCount := 0
	seenArchivePaths := make(map[string]struct{})
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return r2Manifest{}, err
		}
		name, err := safeArchivePath(h.Name)
		if err != nil {
			return r2Manifest{}, err
		}
		if _, exists := seenArchivePaths[name]; exists {
			return r2Manifest{}, errors.New("arsip memiliki path duplikat")
		}
		seenArchivePaths[name] = struct{}{}
		if h.Typeflag != tar.TypeReg {
			return r2Manifest{}, errors.New("arsip berisi tipe file yang tidak diizinkan")
		}
		fileCount++
		if fileCount > maxArchiveFiles {
			return r2Manifest{}, errors.New("arsip memiliki terlalu banyak file")
		}
		if h.Size < 0 || h.Size > backupArchiveLimit() || extractedBytes > backupArchiveLimit()-h.Size {
			return r2Manifest{}, errors.New("ukuran file arsip tidak valid")
		}
		extractedBytes += h.Size
		if name == "manifest.json" {
			b, e := io.ReadAll(io.LimitReader(tr, h.Size+1))
			if e != nil {
				return r2Manifest{}, e
			}
			if int64(len(b)) != h.Size {
				return r2Manifest{}, errors.New("manifest terpotong")
			}
			if e = json.Unmarshal(b, &manifest); e != nil {
				return r2Manifest{}, e
			}
			found = true
			continue
		}
		if name != "database.db" && name != "database.sql" && !strings.HasPrefix(name, "uploads/") {
			return r2Manifest{}, errors.New("arsip memiliki path tak dikenal")
		}
		full := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(dest)+string(os.PathSeparator)) {
			return r2Manifest{}, errors.New("path ekstraksi tidak aman")
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return r2Manifest{}, err
		}
		out, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return r2Manifest{}, err
		}
		_, copyErr := io.CopyN(out, tr, h.Size)
		closeErr := out.Close()
		if copyErr != nil {
			return r2Manifest{}, copyErr
		}
		if closeErr != nil {
			return r2Manifest{}, closeErr
		}
	}
	expectedDatabase := "database.sql"
	if manifest.Dialect == "sqlite" {
		expectedDatabase = "database.db"
	}
	if !found || manifest.Version != r2ArchiveVersion || manifest.Database != expectedDatabase {
		return r2Manifest{}, errors.New("manifest backup tidak valid")
	}
	if len(manifest.Files) > maxArchiveFiles {
		return r2Manifest{}, errors.New("manifest backup memiliki terlalu banyak file")
	}
	manifestPaths := make(map[string]struct{}, len(manifest.Files))
	for _, item := range manifest.Files {
		p, e := safeArchivePath(item.Path)
		if e != nil {
			return r2Manifest{}, e
		}
		if p == "manifest.json" {
			return r2Manifest{}, errors.New("manifest backup tidak boleh menunjuk ke dirinya sendiri")
		}
		if _, exists := manifestPaths[p]; exists {
			return r2Manifest{}, errors.New("manifest backup memiliki path duplikat")
		}
		manifestPaths[p] = struct{}{}
		checksum, size, e := sha256File(filepath.Join(dest, filepath.FromSlash(p)))
		if e != nil || size != item.Size || !strings.EqualFold(checksum, item.SHA256) {
			return r2Manifest{}, errors.New("integritas isi backup gagal")
		}
	}
	for p := range seenArchivePaths {
		if p != "manifest.json" {
			if _, listed := manifestPaths[p]; !listed {
				return r2Manifest{}, errors.New("arsip berisi file yang tidak tercatat di manifest")
			}
		}
	}
	return manifest, nil
}

// verifyFullBackupArtifact validates the encryption envelope, manifest, every
// uploaded file checksum, and the embedded database before a scheduled full
// backup is considered successful.
func verifyFullBackupArtifact(path string) (bool, error) {
	if _, err := deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		return false, err
	}
	work, err := os.MkdirTemp("", "pkbm-full-verify-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(work)
	plain := filepath.Join(work, "backup.tar.gz")
	if err := decryptBackupFile(path, plain, os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		return false, err
	}
	manifest, err := extractR2Archive(plain, filepath.Join(work, "extracted"))
	if err != nil {
		return false, err
	}
	if manifest.Dialect != dialect() {
		return false, errors.New("engine database backup tidak cocok dengan aplikasi ini")
	}
	return verifyBackupArtifact(filepath.Join(work, "extracted", manifest.Database))
}

func copyDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink tidak diizinkan")
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return errors.New("tipe file tidak didukung")
		}
		return copyFile(path, target)
	})
}

func (s *Server) runR2Restore(job *R2BackupJob) {
	now := time.Now()
	job.Status = "running"
	s.updateR2Job(job, map[string]interface{}{"status": "running", "phase": "downloading", "started_at": &now, "error": ""})
	s.r2.maintenance.Store(true)
	keepMaintenance := false
	defer func() {
		if !keepMaintenance {
			s.r2.maintenance.Store(false)
		}
	}()
	if err := ensureBackupDir(); err != nil {
		s.finishR2Job(job, err)
		return
	}
	work := filepath.Join(backupDir(), "r2-restore-"+job.ID)
	if err := os.MkdirAll(work, 0o700); err != nil {
		s.finishR2Job(job, err)
		return
	}
	enc := filepath.Join(work, "remote.tar.gz.enc")
	if err := s.downloadR2Object(context.Background(), job.SourceKey, enc); err != nil {
		_ = os.RemoveAll(work)
		s.finishR2Job(job, err)
		return
	}
	keepMaintenance = s.runFullArchiveRestore(job, work, enc, true)
}

func (s *Server) runLocalFullRestore(job *R2BackupJob, work, archivePath string) {
	now := time.Now()
	job.Status = "running"
	s.updateR2Job(job, map[string]interface{}{"status": "running", "phase": "validating", "started_at": &now, "error": ""})
	s.r2.maintenance.Store(true)
	keepMaintenance := s.runFullArchiveRestore(job, work, archivePath, false)
	if !keepMaintenance {
		s.r2.maintenance.Store(false)
	}
}

// runFullArchiveRestore restores a validated encrypted archive containing a
// database snapshot and the complete uploads tree.  R2 restores additionally
// copy their safety archive to R2; local restores retain that safety archive
// in BACKUP_DIR so they do not require cloud credentials.
func (s *Server) runFullArchiveRestore(job *R2BackupJob, work, enc string, uploadSafetyToR2 bool) (keepMaintenance bool) {
	// Before the journal is durable, this directory is only a temporary
	// decrypt/extract workspace. Remove it on every validation or staging
	// failure so encrypted backups and extracted student files do not
	// accumulate on the persistent backup volume. Once the journal is written,
	// recovery owns the directory and it must survive a crash.
	journalDurable := false
	defer func() {
		if !journalDurable {
			_ = os.RemoveAll(work)
		}
	}()
	plain := filepath.Join(work, "remote.tar.gz")
	extracted := filepath.Join(work, "extracted")
	s.updateR2Phase(job, "validating")
	err := decryptBackupFile(enc, plain, os.Getenv("BACKUP_ENCRYPTION_KEY"))
	var manifest r2Manifest
	if err == nil {
		manifest, err = extractR2Archive(plain, extracted)
	}
	if err == nil && manifest.Dialect != dialect() {
		err = errors.New("engine database backup tidak cocok dengan aplikasi ini")
	}
	if err != nil {
		s.finishR2Job(job, err)
		return
	}
	// Keep the encrypted safety snapshot on the persistent backup volume until
	// every database and uploads step has completed. R2 is an optional second
	// copy, not the only rollback source should the network be unavailable.
	safetyDir := filepath.Join(work, "safety")
	var safetyEnc, safetyKey string
	if safetyEnc, _, _, _, err = s.createR2Archive(safetyDir); err == nil && uploadSafetyToR2 {
		safetyKey, err = s.uploadR2Archive(context.Background(), safetyEnc, "pre-restore")
	}
	if err != nil {
		s.finishR2Job(job, fmt.Errorf("backup pengaman gagal: %w", err))
		return
	}
	journal := &r2RestoreJournal{JobID: job.ID, SourceKey: job.SourceKey, SafetyObjectKey: safetyKey, SafetyArchive: safetyEnc, WorkDir: work, Dialect: dialect(), Phase: "safety-created"}
	if err = writeR2RestoreJournal(journal); err != nil {
		s.finishR2Job(job, err)
		return
	}
	journalDurable = true
	job.SafetyObjectKey = safetyKey
	s.updateR2Job(job, map[string]interface{}{"safety_object_key": safetyKey, "phase": "safety-created"})
	if isSQLite() {
		err = copyFile(filepath.Join(extracted, manifest.Database), journalDBNew(journal))
		if err == nil {
			err = os.MkdirAll(journalUploadsNew(journal), 0o700)
		}
		if err == nil && exists(filepath.Join(extracted, "uploads")) {
			err = copyDirectory(filepath.Join(extracted, "uploads"), journalUploadsNew(journal))
		}
		if err == nil {
			err = updateR2RestorePhase(journal, "sqlite-staged", nil)
			s.updateR2Phase(job, "sqlite-staged")
		}
		if err == nil && scheduleRestoreRestart() {
			keepMaintenance = true
		}
	} else {
		err = updateR2RestorePhase(journal, "postgres-restoring", nil)
		s.updateR2Phase(job, "postgres-restoring")
		if err == nil {
			err = pgRestore(filepath.Join(extracted, manifest.Database))
		}
		if err == nil {
			err = updateR2RestorePhase(journal, "postgres-db-restored", nil)
		}
		if err == nil {
			if exists(filepath.Join(extracted, "uploads")) {
				err = copyDirectory(filepath.Join(extracted, "uploads"), journalUploadsNew(journal))
			} else {
				err = os.MkdirAll(journalUploadsNew(journal), 0o700)
			}
		}
		if err == nil {
			err = updateR2RestorePhase(journal, "postgres-uploads-swapping", nil)
			s.updateR2Phase(job, "postgres-uploads-swapping")
		}
		if err == nil {
			err = swapDirectoryKeepingOld(journalUploadsNew(journal), uploadsDir(), journalUploadsOld(journal))
		}
		if err == nil {
			err = updateR2RestorePhase(journal, "postgres-uploads-swapped", nil)
		}
		if err == nil {
			err = s.migrateSchema()
		}
		if err == nil {
			err = updateR2RestorePhase(journal, "postgres-migrated", nil)
		}
	}
	if err != nil {
		if isSQLite() {
			// Nothing has been swapped until the next controlled restart; discard
			// the journal so a failed staging operation cannot block a later one.
			_ = updateR2RestorePhase(journal, "rolled-back", err)
			removeR2RestoreJournal(journal)
		}
		if !isSQLite() {
			if rollbackErr := restorePostgresSafety(journal); rollbackErr == nil {
				if exists(journalUploadsOld(journal)) {
					if exists(uploadsDir()) {
						failedUploads := fmt.Sprintf("%s.restore-failed-%d", uploadsDir(), time.Now().UnixNano())
						if renameErr := os.Rename(uploadsDir(), failedUploads); renameErr != nil {
							_ = updateR2RestorePhase(journal, "rollback-failed", renameErr)
							s.finishR2Job(job, renameErr)
							return
						}
					}
					_ = os.Rename(journalUploadsOld(journal), uploadsDir())
				}
				_ = updateR2RestorePhase(journal, "rolled-back", nil)
			} else {
				_ = updateR2RestorePhase(journal, "rollback-failed", rollbackErr)
			}
		}
		s.finishR2Job(job, err)
		return
	}
	if isSQLite() {
		// The job is finalized by reconcileR2Operations after the supervisor
		// restarts this process and applies the journal before opening SQLite.
		return keepMaintenance
	}
	_ = os.RemoveAll(journalUploadsOld(journal))
	if err = updateR2RestorePhase(journal, "completed", nil); err != nil {
		s.finishR2Job(job, err)
		return
	}
	removeR2RestoreJournal(journal)
	finished := time.Now()
	job.Status, job.Phase = "succeeded", "completed"
	s.updateR2Job(job, map[string]interface{}{"status": "succeeded", "phase": "completed", "finished_at": &finished})
	auditAction := "restore_r2"
	message := "Restore R2 selesai dengan backup pengaman tersedia di R2."
	if !uploadSafetyToR2 {
		auditAction = "restore_full"
		message = "Restore backup lengkap selesai dengan backup pengaman tersimpan di server."
	}
	s.audit(nil, auditAction, "backup", job.SourceKey)
	s.notifyOperation("restore_succeeded", "restore_succeeded:"+job.ID, message, job)
	return false
}
