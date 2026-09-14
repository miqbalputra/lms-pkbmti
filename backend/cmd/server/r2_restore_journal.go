package main

// The restore journal makes a composite (database + uploads) restore
// recoverable across a process or host restart. It is deliberately stored in
// BACKUP_DIR, which is a persistent Docker volume in production.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const r2RestoreJournalFile = "r2-restore-journal.json"

type r2RestoreJournal struct {
	JobID           string    `json:"jobId"`
	SourceKey       string    `json:"sourceKey"`
	SafetyObjectKey string    `json:"safetyObjectKey,omitempty"`
	SafetyArchive   string    `json:"safetyArchive"`
	WorkDir         string    `json:"workDir"`
	Dialect         string    `json:"dialect"`
	Phase           string    `json:"phase"`
	Error           string    `json:"error,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func r2RestoreJournalPath() string { return filepath.Join(backupDir(), r2RestoreJournalFile) }

func writeR2RestoreJournal(j *r2RestoreJournal) error {
	if j == nil || j.JobID == "" || j.WorkDir == "" {
		return errors.New("restore journal tidak lengkap")
	}
	if err := validateR2RestoreJournalPaths(j); err != nil {
		return err
	}
	if err := ensureBackupDir(); err != nil {
		return err
	}
	j.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	tmp, err := os.OpenFile(r2RestoreJournalPath()+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err = os.Rename(tmp.Name(), r2RestoreJournalPath()); err != nil {
		return err
	}
	// Best effort directory sync. Windows does not permit opening all directory
	// handles, while Linux filesystems use this to persist the rename itself.
	if d, openErr := os.Open(backupDir()); openErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func loadR2RestoreJournal() (*r2RestoreJournal, error) {
	b, err := os.ReadFile(r2RestoreJournalPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j r2RestoreJournal
	if err = json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	if j.JobID == "" || j.WorkDir == "" || j.SafetyArchive == "" || j.Dialect == "" || j.Phase == "" {
		return nil, errors.New("restore journal rusak atau tidak lengkap")
	}
	if j.Dialect != "sqlite" && j.Dialect != "postgresql" {
		return nil, errors.New("dialect restore journal tidak dikenal")
	}
	switch j.Phase {
	case "safety-created", "sqlite-staged", "sqlite-db-swapping", "sqlite-db-swapped", "sqlite-uploads-swapping", "sqlite-uploads-swapped", "postgres-restoring", "postgres-db-restored", "postgres-uploads-swapping", "postgres-uploads-swapped", "postgres-migrated", "completed", "rolled-back", "rollback-failed":
	default:
		return nil, errors.New("fase restore journal tidak dikenal")
	}
	if err := validateR2RestoreJournalPaths(&j); err != nil {
		return nil, err
	}
	return &j, nil
}

func validateR2RestoreJournalPaths(j *r2RestoreJournal) error {
	if j == nil || j.WorkDir == "" || j.SafetyArchive == "" {
		return errors.New("restore journal tidak lengkap")
	}
	backupRoot, err := filepath.Abs(filepath.Clean(backupDir()))
	if err != nil {
		return errors.New("direktori backup tidak valid")
	}
	workDir, err := filepath.Abs(filepath.Clean(j.WorkDir))
	if err != nil || !pathWithin(workDir, backupRoot) || !strings.HasPrefix(filepath.Base(workDir), "r2-restore-") {
		return errors.New("workdir restore berada di luar direktori backup")
	}
	safetyArchive, err := filepath.Abs(filepath.Clean(j.SafetyArchive))
	if err != nil || !pathWithin(safetyArchive, workDir) {
		return errors.New("arsip pengaman restore berada di luar workdir")
	}
	return nil
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func removeR2RestoreJournal(j *r2RestoreJournal) {
	_ = os.Remove(r2RestoreJournalPath())
	if j != nil && j.WorkDir != "" {
		_ = os.RemoveAll(j.WorkDir)
	}
}

func updateR2RestorePhase(j *r2RestoreJournal, phase string, cause error) error {
	j.Phase = phase
	if cause != nil {
		j.Error = safeOperationError(cause)
	}
	return writeR2RestoreJournal(j)
}

func journalDBNew(j *r2RestoreJournal) string      { return filepath.Join(j.WorkDir, "database.new") }
func journalDBOld(j *r2RestoreJournal) string      { return filepath.Join(j.WorkDir, "database.old") }
func journalUploadsNew(j *r2RestoreJournal) string { return filepath.Join(j.WorkDir, "uploads.new") }
func journalUploadsOld(j *r2RestoreJournal) string { return filepath.Join(j.WorkDir, "uploads.old") }

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func swapFileKeepingOld(newPath, livePath, oldPath string) error {
	if exists(oldPath) && exists(livePath) && !exists(newPath) {
		// Crash window after the new file reached its live name but before
		// the durable journal phase update. Treat the completed swap as
		// success and preserve the safety copy for rollback.
		return nil
	}
	if !exists(newPath) {
		return fmt.Errorf("file staging restore tidak ditemukan")
	}
	if exists(oldPath) {
		if exists(livePath) {
			return fmt.Errorf("file safety restore sudah ada sementara file live masih ada")
		}
		// Crash window: live was already moved to oldPath, but newPath has not
		// been moved into place yet. Continue without deleting the safety copy.
	} else {
		if err := os.Rename(livePath, oldPath); err != nil {
			return err
		}
	}
	if err := os.Rename(newPath, livePath); err != nil {
		if !exists(livePath) {
			_ = os.Rename(oldPath, livePath)
		}
		return err
	}
	return nil
}

func swapDirectoryKeepingOld(newPath, livePath, oldPath string) error {
	if exists(oldPath) && exists(livePath) && !exists(newPath) {
		// Crash window after the directory swap completed but before the
		// durable journal phase update. The old tree remains the rollback
		// copy, so the operation is safe to resume idempotently.
		return nil
	}
	if !exists(newPath) {
		return fmt.Errorf("folder staging restore tidak ditemukan")
	}
	if exists(oldPath) {
		if exists(livePath) {
			return fmt.Errorf("folder safety restore sudah ada sementara folder live masih ada")
		}
		// Crash window: live was already moved to oldPath; preserve it and
		// finish the second rename below.
	} else {
		if err := os.Rename(livePath, oldPath); err != nil {
			return err
		}
	}
	if err := os.Rename(newPath, livePath); err != nil {
		if !exists(livePath) {
			_ = os.Rename(oldPath, livePath)
		}
		return err
	}
	return nil
}

func rollbackSQLiteJournal(j *r2RestoreJournal) error {
	if exists(journalDBOld(j)) {
		failed := fmt.Sprintf("%s.restore-failed-%d", liveDBPath, time.Now().UnixNano())
		if exists(liveDBPath) {
			_ = os.Rename(liveDBPath, failed)
		}
		if err := os.Rename(journalDBOld(j), liveDBPath); err != nil {
			return err
		}
	}
	if exists(journalUploadsOld(j)) {
		failed := fmt.Sprintf("%s.restore-failed-%d", uploadsDir(), time.Now().UnixNano())
		if exists(uploadsDir()) {
			_ = os.Rename(uploadsDir(), failed)
		}
		if err := os.Rename(journalUploadsOld(j), uploadsDir()); err != nil {
			return err
		}
	}
	return updateR2RestorePhase(j, "rolled-back", nil)
}

func applyR2SQLiteJournal(j *r2RestoreJournal) error {
	if j.Phase == "sqlite-staged" || j.Phase == "sqlite-db-swapping" {
		if err := updateR2RestorePhase(j, "sqlite-db-swapping", nil); err != nil {
			return err
		}
		if err := swapFileKeepingOld(journalDBNew(j), liveDBPath, journalDBOld(j)); err != nil {
			_ = rollbackSQLiteJournal(j)
			return err
		}
		if err := updateR2RestorePhase(j, "sqlite-db-swapped", nil); err != nil {
			return err
		}
	}
	if j.Phase == "sqlite-db-swapped" || j.Phase == "sqlite-uploads-swapping" {
		if err := updateR2RestorePhase(j, "sqlite-uploads-swapping", nil); err != nil {
			return err
		}
		if err := swapDirectoryKeepingOld(journalUploadsNew(j), uploadsDir(), journalUploadsOld(j)); err != nil {
			_ = rollbackSQLiteJournal(j)
			return err
		}
		if err := updateR2RestorePhase(j, "sqlite-uploads-swapped", nil); err != nil {
			return err
		}
	}
	if j.Phase == "sqlite-uploads-swapped" {
		_ = os.Remove(journalDBOld(j))
		_ = os.RemoveAll(journalUploadsOld(j))
		return updateR2RestorePhase(j, "completed", nil)
	}
	return nil
}

func restorePostgresSafety(j *r2RestoreJournal) error {
	plain := filepath.Join(j.WorkDir, "rollback.tar.gz")
	extracted := filepath.Join(j.WorkDir, "rollback")
	if err := decryptBackupFile(j.SafetyArchive, plain, os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
		return err
	}
	manifest, err := extractR2Archive(plain, extracted)
	if err != nil {
		return err
	}
	if manifest.Dialect != "postgresql" {
		return errors.New("backup pengaman PostgreSQL tidak valid")
	}
	return pgRestore(filepath.Join(extracted, manifest.Database))
}

func recoverPendingR2Restore() error {
	j, err := loadR2RestoreJournal()
	if err != nil || j == nil {
		return err
	}
	if j.Dialect == "sqlite" {
		if j.Phase == "rolled-back" || j.Phase == "completed" {
			return nil
		}
		return applyR2SQLiteJournal(j)
	}
	if j.Dialect != "postgresql" {
		return errors.New("dialect restore journal tidak dikenal")
	}
	if j.Phase == "completed" || j.Phase == "rolled-back" {
		return nil
	}
	// A PostgreSQL restore is transaction-bound. If the process stopped before
	// completion, use the durable local safety archive to return to the last
	// known-good database and upload tree before serving requests again.
	if j.Phase != "safety-created" {
		if err := restorePostgresSafety(j); err != nil {
			return fmt.Errorf("rollback PostgreSQL dari backup pengaman gagal: %w", err)
		}
	}
	if exists(journalUploadsOld(j)) {
		if exists(uploadsDir()) {
			failed := fmt.Sprintf("%s.restore-failed-%d", uploadsDir(), time.Now().UnixNano())
			if err := os.Rename(uploadsDir(), failed); err != nil {
				return err
			}
		}
		if err := os.Rename(journalUploadsOld(j), uploadsDir()); err != nil {
			return err
		}
	}
	return updateR2RestorePhase(j, "rolled-back", nil)
}

// reconcileR2Operations runs after the database is available. It translates a
// startup recovery result into the persistent job history and only then removes
// the on-disk journal/work directory.
func (s *Server) reconcileR2Operations() {
	j, err := loadR2RestoreJournal()
	now := time.Now()
	journalJobID := ""
	if err == nil && j != nil {
		journalJobID = j.JobID
		var job R2BackupJob
		if s.db.First(&job, "id = ?", j.JobID).Error == nil {
			switch j.Phase {
			case "completed":
				s.db.Model(&job).Updates(map[string]any{"status": "succeeded", "phase": "completed", "finished_at": &now, "recovered_at": &now, "error": ""})
				s.audit(nil, "restore_r2", "backup", j.SourceKey)
				s.notifyOperation("restore_succeeded", "restore_succeeded:"+job.ID, "Restore R2 selesai setelah restart terkontrol.", &job)
				removeR2RestoreJournal(j)
			case "rolled-back":
				s.db.Model(&job).Updates(map[string]any{"status": "failed", "phase": "rolled-back", "finished_at": &now, "recovered_at": &now, "error": "Restore dihentikan dan data sebelumnya dipulihkan secara aman."})
				s.notifyOperation("restore_failed", "restore_failed:"+job.ID, "Restore R2 dihentikan; backup pengaman telah dipulihkan.", &job)
				removeR2RestoreJournal(j)
			}
		}
	}
	// An old queued/running job without a journal cannot safely be resumed: its
	// transient inputs disappeared with the process. Preserve it as an audited
	// failed operation rather than pretending it completed.
	var orphaned []R2BackupJob
	q := s.db.Where("status IN ?", []string{"queued", "running"})
	if journalJobID != "" {
		q = q.Where("id <> ?", journalJobID)
	}
	if q.Find(&orphaned).Error == nil {
		for _, orphan := range orphaned {
			s.db.Model(&orphan).Updates(map[string]any{"status": "failed", "phase": "interrupted", "finished_at": &now, "error": "Operasi terputus saat aplikasi berhenti; jalankan ulang dari dashboard."})
		}
	}
}
