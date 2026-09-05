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
)

const r2ArchiveVersion = 1

type R2BackupJob struct {
	Base
	Kind       string     `gorm:"index;not null" json:"kind"`   // manual, scheduled, restore
	Status     string     `gorm:"index;not null" json:"status"` // queued, running, succeeded, failed
	ObjectKey  string     `gorm:"index" json:"objectKey,omitempty"`
	SourceKey  string     `json:"sourceKey,omitempty"`
	Size       int64      `json:"size"`
	FileCount  int        `json:"fileCount"`
	Checksum   string     `json:"checksum,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
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

func r2Client(ctx context.Context) (*s3.Client, error) {
	if err := r2ConfigError(); err != nil {
		return nil, err
	}
	endpoint := "https://" + strings.TrimSpace(os.Getenv("BACKUP_R2_ACCOUNT_ID")) + ".r2.cloudflarestorage.com"
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
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
	closeWithError := func(e error) error { _ = tw.Close(); _ = gz.Close(); _ = out.Close(); return e }
	if err := addArchiveFile(tw, dbPath, dbName, &manifest.Files); err != nil {
		return "", r2Manifest{}, "", 0, closeWithError(err)
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

func newUUID() string { return strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "") }

func (s *Server) updateR2Job(job *R2BackupJob, fields map[string]interface{}) {
	_ = s.db.Model(job).Updates(fields).Error
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
	s.updateR2Job(job, map[string]interface{}{"status": "running", "started_at": &now, "error": ""})
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
	s.updateR2Job(job, map[string]interface{}{"status": "succeeded", "finished_at": &finished})
	s.metrics.recordSuccess(true, true)
	s.audit(nil, "backup_r2", "backup", job.ObjectKey)
}

func (s *Server) finishR2Job(job *R2BackupJob, err error) {
	finished := time.Now()
	s.updateR2Job(job, map[string]interface{}{"status": "failed", "finished_at": &finished, "error": safeBackupError(err)})
	s.metrics.recordFailure()
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
	out, err := client.ListObjectsV2(c.Context(), &s3.ListObjectsV2Input{Bucket: aws.String(os.Getenv("BACKUP_R2_BUCKET")), Prefix: aws.String(r2ArchivePrefix())})
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
	return c.JSON(fiber.Map{"archives": items})
}

func (s *Server) r2Status(c *fiber.Ctx) error {
	status := fiber.Map{"enabled": r2Enabled(), "prefix": r2Prefix(), "retentionDays": r2RetentionDays(), "schedule": "02:00 WIB, setiap 72 jam", "maintenance": s.r2.maintenance.Load()}
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
		if h.Typeflag != tar.TypeReg {
			return r2Manifest{}, errors.New("arsip berisi tipe file yang tidak diizinkan")
		}
		if h.Size < 0 || h.Size > int64(backupUploadLimit())*4 {
			return r2Manifest{}, errors.New("ukuran file arsip tidak valid")
		}
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
	if !found || manifest.Version != r2ArchiveVersion || manifest.Database == "" {
		return r2Manifest{}, errors.New("manifest backup tidak valid")
	}
	for _, item := range manifest.Files {
		p, e := safeArchivePath(item.Path)
		if e != nil {
			return r2Manifest{}, e
		}
		checksum, size, e := sha256File(filepath.Join(dest, filepath.FromSlash(p)))
		if e != nil || size != item.Size || !strings.EqualFold(checksum, item.SHA256) {
			return r2Manifest{}, errors.New("integritas isi backup gagal")
		}
	}
	return manifest, nil
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

func replaceUploads(staged string) error {
	if _, err := os.Stat(staged); os.IsNotExist(err) {
		return os.MkdirAll(uploadsDir(), 0o700)
	}
	old := uploadsDir() + ".pre-restore"
	_ = os.RemoveAll(old)
	if err := os.Rename(uploadsDir(), old); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, uploadsDir()); err != nil {
		_ = os.Rename(old, uploadsDir())
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}

func (s *Server) runR2Restore(job *R2BackupJob) {
	now := time.Now()
	s.updateR2Job(job, map[string]interface{}{"status": "running", "started_at": &now, "error": ""})
	s.r2.maintenance.Store(true)
	defer s.r2.maintenance.Store(false)
	work, err := os.MkdirTemp(backupDir(), "r2-restore-*")
	if err != nil {
		s.finishR2Job(job, err)
		return
	}
	defer os.RemoveAll(work)
	enc := filepath.Join(work, "remote.tar.gz.enc")
	plain := filepath.Join(work, "remote.tar.gz")
	extracted := filepath.Join(work, "extracted")
	if err = s.downloadR2Object(context.Background(), job.SourceKey, enc); err == nil {
		err = decryptBackupFile(enc, plain, os.Getenv("BACKUP_ENCRYPTION_KEY"))
	}
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
	// A full safety snapshot is sent to R2 before any live data is touched.
	safetyDir := filepath.Join(work, "safety")
	var safetyEnc string
	var safetyChecksum string
	var safetySize int64
	if safetyEnc, _, safetyChecksum, safetySize, err = s.createR2Archive(safetyDir); err == nil {
		_, err = s.uploadR2Archive(context.Background(), safetyEnc, "pre-restore")
	}
	if err != nil {
		s.finishR2Job(job, fmt.Errorf("backup pengaman gagal: %w", err))
		return
	}
	_ = safetyChecksum
	_ = safetySize
	if isSQLite() {
		if err = copyFile(filepath.Join(extracted, manifest.Database), pendingDBPath); err == nil {
			_ = os.RemoveAll("uploads.restore-pending")
			err = os.MkdirAll("uploads.restore-pending", 0o700)
			if _, e := os.Stat(filepath.Join(extracted, "uploads")); e == nil {
				err = copyDirectory(filepath.Join(extracted, "uploads"), "uploads.restore-pending")
			}
		}
		if err == nil {
			scheduleRestoreRestart()
		}
	} else {
		err = pgRestore(filepath.Join(extracted, manifest.Database))
		if err == nil {
			err = replaceUploads(filepath.Join(extracted, "uploads"))
		}
		if err == nil {
			err = s.migrateSchema()
		}
	}
	if err != nil {
		s.finishR2Job(job, err)
		return
	}
	finished := time.Now()
	s.updateR2Job(job, map[string]interface{}{"status": "succeeded", "finished_at": &finished})
	s.audit(nil, "restore_r2", "backup", job.SourceKey)
}
