package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Database backup & restore (SQLite and PostgreSQL).
//
// Goals (per operator request): back up the FULL database via the app, in a
// portable SQL form and an exact binary form; triggerable from n8n over HTTP;
// restore must be easy and safe.
//
// Backup formats:
//   - db  (binary): VACUUM INTO — an atomic, consistent snapshot of the live
//     database written to a fresh file. Fast and the exact restore source.
//   - sql (text):   schema (sqlite_master) + per-row INSERTs. Portable, can be
//     inspected/diffed, and consumed by external tooling (sqlite3 CLI, n8n
//     archives). Format mirrors `sqlite3 .dump`: PRAGMA + BEGIN/COMMIT wrap the
//     statements so restore is atomic.
//
// Restore is restart-applied, never in-place on the running server (the live
// file is open and locked). A restore upload is staged to a pending file; on
// next startup applyPendingRestore() swaps/rebuilds the live DB AFTER backing
// up the current one to backups/pre-restore-<ts>.db. So a bad restore is always
// recoverable by hand (rename the pre-restore file back).
//
// Auth: the read endpoints (download/list) accept EITHER an admin Bearer JWT OR
// a long-lived static key (BACKUP_API_KEY env) so n8n can call them without
// minting a short-lived JWT. The write endpoints (trigger/restore) require an
// admin JWT.
//
// PostgreSQL (DATABASE_URL set) uses the pg_dump/psql CLI path in these
// endpoints — use pg_dump/pg_restore for that. The app's default is SQLite.
// ---------------------------------------------------------------------------

const (
	liveDBPath         = "pkbm-lms.db"
	pendingDBPath      = "pkbm-lms.db.restore-pending"     // binary restore staging
	pendingSQLPath     = "pkbm-lms.db.restore-pending.sql" // sql restore staging
	pendingUploadsPath = "uploads.restore-pending"         // full R2 restore staging
	defaultBackupDir   = "backups"
	backupGlob         = "pkbm-lms-*.db"
)

// pendingRestoreApplied is set true when applyPendingRestore applied a staged
// restore this startup. migrate() consults it to skip the dummy-seed step so a
// restored DB isn't polluted with synthetic data on first boot.
var pendingRestoreApplied bool

func isSQLite() bool { return os.Getenv("DATABASE_URL") == "" }

func dialect() string {
	if isSQLite() {
		return "sqlite"
	}
	return "postgresql"
}

func backupDir() string { return env("BACKUP_DIR", defaultBackupDir) }

func backupUploadLimit() int {
	mb, err := strconv.Atoi(env("BACKUP_MAX_UPLOAD_MB", "512"))
	if err != nil || mb < 8 {
		mb = 512
	}
	if mb > 4096 {
		mb = 4096
	}
	return mb * 1024 * 1024
}

// normalizeBackupFormat makes the default "full" backup the easiest restore
// source: SQLite uses its exact binary snapshot, while PostgreSQL uses a full
// pg_dump SQL file. Explicit sql/db requests remain supported for compatibility.
func normalizeBackupFormat(requested string) string {
	format := strings.ToLower(strings.TrimSpace(requested))
	if format == "" || format == "full" {
		if isSQLite() {
			return "db"
		}
		return "sql"
	}
	if isSQLite() {
		if format == "sql" {
			return "sql"
		}
		return "db"
	}
	return "sql"
}

// ---------------------------------------------------------------------------
// PostgreSQL backup/restore via pg_dump / psql CLI
// ---------------------------------------------------------------------------

type pgConnInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// parseDatabaseURL extracts connection parameters from a PostgreSQL DATABASE_URL
// of the form: postgres://user:pass@host:port/dbname?options
func parseDatabaseURL(raw string) (pgConnInfo, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return pgConnInfo{}, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return pgConnInfo{}, fmt.Errorf("DATABASE_URL must use postgres:// or postgresql://")
	}
	host := u.Hostname()
	if host == "" || u.User == nil || u.User.Username() == "" {
		return pgConnInfo{}, fmt.Errorf("DATABASE_URL must include host and user")
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		dbName = "postgres"
	}
	sslMode := u.Query().Get("sslmode")
	return pgConnInfo{
		Host:     host,
		Port:     port,
		User:     user,
		Password: pass,
		DBName:   dbName,
		SSLMode:  sslMode,
	}, nil
}

func pgCommandEnv(info pgConnInfo) []string {
	envVars := append([]string{}, os.Environ()...)
	envVars = append(envVars, "PGPASSWORD="+info.Password)
	if info.SSLMode != "" {
		envVars = append(envVars, "PGSSLMODE="+info.SSLMode)
	}
	return envVars
}

// pgDump runs pg_dump to create a SQL backup of the PostgreSQL database.
func pgDump(destPath string) error {
	info, err := parseDatabaseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	args := []string{
		"-h", info.Host,
		"-p", info.Port,
		"-U", info.User,
		"-d", info.DBName,
		"-f", destPath,
		"--no-owner",
		"--no-privileges",
		"--clean",
		"--if-exists",
	}
	cmd := exec.Command("pg_dump", args...)
	cmd.Env = pgCommandEnv(info)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %s — %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// pgRestore runs psql to restore a SQL backup into the PostgreSQL database.
func pgRestore(srcPath string) error {
	return pgRestoreTo(srcPath, os.Getenv("DATABASE_URL"))
}

func pgRestoreTo(srcPath, databaseURL string) error {
	info, err := parseDatabaseURL(databaseURL)
	if err != nil {
		return err
	}
	args := []string{
		"-h", info.Host,
		"-p", info.Port,
		"-U", info.User,
		"-d", info.DBName,
		"-f", srcPath,
		"--single-transaction",
		"--set=ON_ERROR_STOP=1",
	}
	cmd := exec.Command("psql", args...)
	cmd.Env = pgCommandEnv(info)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql restore failed: %s — %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// pgDumpStream runs pg_dump and writes the SQL output to w (for download-only backup).
func pgDumpStream(w io.Writer) error {
	info, err := parseDatabaseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	args := []string{
		"-h", info.Host,
		"-p", info.Port,
		"-U", info.User,
		"-d", info.DBName,
		"--no-owner",
		"--no-privileges",
		"--clean",
		"--if-exists",
	}
	cmd := exec.Command("pg_dump", args...)
	cmd.Env = pgCommandEnv(info)
	cmd.Stdout = w
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %s — %w", strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// backupReadAuth allows an admin JWT or a static BACKUP_API_KEY (header
// X-Backup-Key or ?key=). The static key exists so n8n can pull backups on a
// schedule without holding a rotating JWT.
func (s *Server) backupReadAuth(c *fiber.Ctx) error {
	if key := os.Getenv("BACKUP_API_KEY"); key != "" {
		supplied := c.Get("X-Backup-Key")
		if supplied == "" {
			supplied = c.Query("key")
		}
		if supplied != "" && supplied == key {
			return c.Next()
		}
	}
	_, uid, role, err := s.parseAccessToken(c.Get("Authorization"))
	if err != nil {
		return fiber.NewError(401, "missing access token or backup key")
	}
	if role != "admin" {
		return fiber.NewError(403, "admin access required")
	}
	c.Locals("userID", uid)
	c.Locals("role", "admin")
	return c.Next()
}

// ---------------------------------------------------------------------------
// Backup creation
// ---------------------------------------------------------------------------

// backupBinary runs VACUUM INTO to write an atomic consistent snapshot of the
// live database to destPath (which must not already exist). Requires SQLite.
func (s *Server) backupBinary(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		if err := os.Remove(destPath); err != nil {
			return err
		}
	}
	// VACUUM INTO needs a string literal (some drivers reject a bind param);
	// sqlLit single-quotes and escapes internal quotes.
	if err := s.db.Exec(fmt.Sprintf("VACUUM INTO %s", sqlLit(destPath))).Error; err != nil {
		return err
	}
	return nil
}

// dumpSQL writes a portable SQL dump of the whole database to w. Mirrors the
// `sqlite3 .dump` shape: PRAGMA foreign_keys=OFF; BEGIN; <schema>; <data>;
// COMMIT; One top-level statement may still span multiple lines (e.g. CREATE
// TABLE) — restore uses splitSQLStatements, which is quote/comment aware, so
// multi-line statements are fine.
func (s *Server) dumpSQL(w io.Writer) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "-- PKBM LMS SQL backup (SQLite). Generated at "+wibTimeFormat(time.Now(), time.RFC3339)+".")
	fmt.Fprintln(w, "PRAGMA foreign_keys=OFF;")
	fmt.Fprintln(w, "BEGIN TRANSACTION;")

	// Schema: tables, indexes, triggers, views in creation order.
	schemaRows, err := sqlDB.Query("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND type IN ('table','index','trigger','view') ORDER BY rowid")
	if err != nil {
		return err
	}
	for schemaRows.Next() {
		var sqlStr string
		if err := schemaRows.Scan(&sqlStr); err != nil {
			schemaRows.Close()
			return err
		}
		fmt.Fprintln(w, strings.TrimSpace(sqlStr)+";")
	}
	schemaRows.Close()

	// Data: every user table (skip sqlite internal tables) in rowid order.
	tableRows, err := sqlDB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY rowid")
	if err != nil {
		return err
	}
	var tables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			tableRows.Close()
			return err
		}
		tables = append(tables, name)
	}
	tableRows.Close()

	for _, t := range tables {
		r, err := sqlDB.Query(fmt.Sprintf("SELECT * FROM %s", quoteIdent(t)))
		if err != nil {
			return err
		}
		cols, _ := r.Columns()
		colTypes, _ := r.ColumnTypes()
		scanVals := make([]interface{}, len(cols))
		for i := range scanVals {
			scanVals[i] = new(interface{})
		}
		for r.Next() {
			if err := r.Scan(scanVals...); err != nil {
				r.Close()
				return err
			}
			var b strings.Builder
			b.WriteString("INSERT INTO ")
			b.WriteString(quoteIdent(t))
			b.WriteString(" VALUES(")
			for i, sv := range scanVals {
				if i > 0 {
					b.WriteByte(',')
				}
				v := sv.(*interface{})
				b.WriteString(formatSQLValue(*v, colTypes[i].DatabaseTypeName()))
			}
			b.WriteString(");")
			fmt.Fprintln(w, b.String())
		}
		if err := r.Err(); err != nil {
			r.Close()
			return err
		}
		r.Close()
	}
	fmt.Fprintln(w, "COMMIT;")
	return nil
}

// formatSQLValue renders a Go value (from database/sql Scan into interface{}) as
// a SQLite literal. NULL → NULL; numbers bare; text single-quoted (with ”
// escaping); BLOB → X'hex'. Column type name disambiguates text vs blob (both
// arrive as []byte).
func formatSQLValue(v interface{}, typeName string) string {
	if v == nil {
		return "NULL"
	}
	switch x := v.(type) {
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case []byte:
		if strings.ToUpper(typeName) == "BLOB" {
			return "X'" + hex.EncodeToString(x) + "'"
		}
		return sqlLit(string(x))
	case string:
		return sqlLit(x)
	default:
		return sqlLit(fmt.Sprint(v))
	}
}

// sqlLit single-quotes a string for SQLite, escaping internal single quotes as
// ” (the SQLite string-literal escape). Newlines inside are valid in SQLite
// string literals and are left as-is.
func sqlLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// quoteIdent wraps a table/column name in double quotes (SQLite identifier
// quoting), escaping internal double quotes as "".
func quoteIdent(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

// ---------------------------------------------------------------------------
// Backup listing & pruning
// ---------------------------------------------------------------------------

type backupInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	ModTime   string `json:"modTime"`
	Format    string `json:"format"` // "db" | "sql"
	Automatic bool   `json:"automatic"`
}

func listBackupFiles() ([]backupInfo, error) {
	dir := backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []backupInfo{}, nil
		}
		return nil, err
	}
	out := make([]backupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "pkbm-lms-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt2 := "db"
		if strings.HasSuffix(name, ".sql") {
			fmt2 = "sql"
		} else if !strings.HasSuffix(name, ".db") {
			continue // pre-restore-* etc. are excluded by the prefix check anyway
		}
		out = append(out, backupInfo{
			Name:      name,
			Size:      info.Size(),
			ModTime:   wibTimeFormat(info.ModTime(), time.RFC3339),
			Format:    fmt2,
			Automatic: strings.Contains(name, "-auto-"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime > out[j].ModTime })
	return out, nil
}

// pruneBackups deletes the oldest backups beyond retention, keeping only the
// newest `retention` files. Only scheduled (-auto-) backups are pruned; manual
// and pre-restore safety backups are left untouched.
func pruneBackups(retention int) {
	if retention <= 0 {
		return
	}
	files, err := listBackupFiles()
	if err != nil {
		return
	}
	var autos []backupInfo
	for _, f := range files {
		if f.Automatic {
			autos = append(autos, f)
		}
	}
	if len(autos) <= retention {
		return
	}
	// listBackupFiles returns newest first; drop the newest `retention`, delete the rest.
	for _, f := range autos[retention:] {
		_ = os.Remove(filepath.Join(backupDir(), f.Name))
	}
}

// ---------------------------------------------------------------------------
// Scheduled backup
// ---------------------------------------------------------------------------

// runScheduledBackup creates one timestamped backup in the backup dir using the
// configured format and prunes old automatic backups. Called by the cron job
// registered in startScheduler (routes.go) when BACKUP_CRON is set. Audited with
// no user actor (system-initiated).
func (s *Server) runScheduledBackup() {
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		fmt.Printf("scheduled backup: mkdir failed: %v\n", err)
		s.metrics.recordFailure()
		return
	}
	format := normalizeBackupFormat(env("BACKUP_FORMAT", "full"))
	ts := wibTimeFormat(time.Now(), "20060102-150405")
	name := fmt.Sprintf("pkbm-lms-%s-auto.%s", ts, format)
	dest := filepath.Join(backupDir(), name)
	var d string
	if isSQLite() {
		if format == "sql" {
			f, err := os.Create(dest)
			if err != nil {
				fmt.Printf("scheduled backup: create failed: %v\n", err)
				s.metrics.recordFailure()
				return
			}
			err = s.dumpSQL(f)
			f.Close()
			if err != nil {
				fmt.Printf("scheduled backup: dump failed: %v\n", err)
				s.metrics.recordFailure()
				_ = os.Remove(dest)
				return
			}
			d = dest
		} else {
			if err := s.backupBinary(dest); err != nil {
				fmt.Printf("scheduled backup: VACUUM INTO failed: %v\n", err)
				s.metrics.recordFailure()
				_ = os.Remove(dest)
				return
			}
			d = dest
		}
	} else {
		if err := pgDump(dest); err != nil {
			fmt.Printf("scheduled backup: pg_dump failed: %v\n", err)
			_ = os.Remove(dest)
			s.metrics.recordFailure()
			return
		}
		d = dest
	}
	drillVerified, err := verifyBackupArtifact(d)
	if err != nil {
		fmt.Printf("scheduled backup: verification failed: %v\n", err)
		s.metrics.recordFailure()
		return
	}
	offsiteUploaded, err := uploadOffsiteBackup(d)
	if err != nil {
		fmt.Printf("scheduled backup: offsite upload failed: %v\n", err)
		s.metrics.recordFailure()
		return
	}
	// Best-effort audit (system actor: nil uid).
	s.audit(nil, "backup", "system", "scheduled backup -> "+d)
	s.metrics.recordSuccess(offsiteUploaded, drillVerified)
	if r, e := strconv.Atoi(env("BACKUP_RETENTION", "14")); e == nil {
		pruneBackups(r)
	}
	fmt.Printf("scheduled backup written: %s\n", d)
}

// ---------------------------------------------------------------------------
// Restore (applied on next startup)
// ---------------------------------------------------------------------------

// applyPendingRestore applies a staged restore BEFORE the live DB is opened.
// A binary pending file replaces the live file; an SQL pending file rebuilds
// the live DB from the dump. In both cases the current DB is first copied to
// backups/pre-restore-<ts>.db as a safety net. Returns nil if nothing is
// pending. Safe to call on every startup.
func applyPendingRestore() error {
	if !isSQLite() {
		return nil
	}
	if _, err := os.Stat(pendingDBPath); err == nil {
		return applyBinaryRestore()
	}
	if _, err := os.Stat(pendingSQLPath); err == nil {
		return applySQLRestore()
	}
	return nil
}

// savePreRestoreBackup copies the current live DB (+ removes WAL/SHM sidecars)
// to backups/pre-restore-<ts>.db so a bad restore can be rolled back by hand.
func savePreRestoreBackup() (string, error) {
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(backupDir(), fmt.Sprintf("pre-restore-%s.db", wibTimeFormat(time.Now(), "20060102-150405")))
	if err := copyFile(liveDBPath, dest); err != nil {
		return "", err
	}
	// WAL/SHM sidecars belong to the pre-restore DB state; drop them so the
	// restored DB starts clean.
	_ = os.Remove(liveDBPath + "-wal")
	_ = os.Remove(liveDBPath + "-shm")
	return dest, nil
}

func applyBinaryRestore() error {
	safety, err := savePreRestoreBackup()
	if err != nil {
		return fmt.Errorf("pre-restore backup failed: %w", err)
	}
	if err := atomicReplaceLiveDB(pendingDBPath); err != nil {
		return fmt.Errorf("swap pending db failed: %w", err)
	}
	pendingRestoreApplied = true
	if err := applyPendingUploads(); err != nil {
		return fmt.Errorf("restore database berhasil tetapi restore uploads gagal: %w", err)
	}
	fmt.Printf("RESTORE applied (binary). Pre-restore safety backup: %s\n", safety)
	return nil
}

func applySQLRestore() error {
	content, err := os.ReadFile(pendingSQLPath)
	if err != nil {
		return fmt.Errorf("read pending sql failed: %w", err)
	}
	// Build and validate the restored database beside the live database first.
	// A malformed upload therefore cannot delete or leave the active DB empty.
	tmp, err := os.CreateTemp(".", "pkbm-restore-apply-*")
	if err != nil {
		return fmt.Errorf("create restored db failed: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpPath)
	defer os.Remove(tmpPath)
	if err := replaySQLToDatabase(tmpPath, string(content)); err != nil {
		return fmt.Errorf("replay dump failed: %w", err)
	}
	if err := validateSQLiteBackup(tmpPath); err != nil {
		return fmt.Errorf("restored db integrity check failed: %w", err)
	}
	safety, err := savePreRestoreBackup()
	if err != nil {
		return fmt.Errorf("pre-restore backup failed: %w", err)
	}
	if err := atomicReplaceLiveDB(tmpPath); err != nil {
		return fmt.Errorf("swap restored db failed: %w", err)
	}
	_ = os.Remove(pendingSQLPath)
	pendingRestoreApplied = true
	if err := applyPendingUploads(); err != nil {
		return fmt.Errorf("restore database berhasil tetapi restore uploads gagal: %w", err)
	}
	fmt.Printf("RESTORE applied (sql). Pre-restore safety backup: %s\n", safety)
	return nil
}

// applyPendingUploads is only present for a composite R2 restore. Legacy
// database-only restores leave current uploads untouched.
func applyPendingUploads() error {
	if _, err := os.Stat(pendingUploadsPath); os.IsNotExist(err) {
		return nil
	}
	old := uploadsDir() + ".pre-restore"
	_ = os.RemoveAll(old)
	if err := os.Rename(uploadsDir(), old); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(pendingUploadsPath, uploadsDir()); err != nil {
		_ = os.Rename(old, uploadsDir())
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}

func atomicReplaceLiveDB(stagedPath string) error {
	oldPath := liveDBPath + ".restore-old"
	_ = os.Remove(oldPath)
	if err := os.Rename(liveDBPath, oldPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stagedPath, liveDBPath); err != nil {
		_ = os.Rename(oldPath, liveDBPath)
		return err
	}
	_ = os.Remove(oldPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// splitSQLStatements splits a SQL dump into top-level statements on `;`
// boundaries, respecting single-quote string literals (” escape), double-quote
// identifiers, -- line comments and /* */ block comments. This mirrors the
// `sqlite3 .dump` format and the output of dumpSQL above, so both this app's own
// dumps and externally-produced dumps restore the same way.
func splitSQLStatements(content string) []string {
	var stmts []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	inLine, inBlock := false, false
	r := []rune(content)
	for i := 0; i < len(r); i++ {
		c := r[i]
		cur.WriteRune(c)
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			if c == '*' && i+1 < len(r) && r[i+1] == '/' {
				cur.WriteRune(r[i+1])
				i++
				inBlock = false
			}
		case inSingle:
			if c == '\'' {
				if i+1 < len(r) && r[i+1] == '\'' {
					cur.WriteRune(r[i+1])
					i++ // escaped quote
				} else {
					inSingle = false
				}
			}
		case inDouble:
			if c == '"' {
				if i+1 < len(r) && r[i+1] == '"' {
					cur.WriteRune(r[i+1])
					i++
				} else {
					inDouble = false
				}
			}
		default:
			switch c {
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case '-':
				if i+1 < len(r) && r[i+1] == '-' {
					inLine = true
				}
			case '/':
				if i+1 < len(r) && r[i+1] == '*' {
					cur.WriteRune(r[i+1])
					i++
					inBlock = true
				}
			case ';':
				stmts = append(stmts, cur.String())
				cur.Reset()
			}
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		stmts = append(stmts, cur.String())
	}
	return stmts
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// GET /backup — list existing backups in the backup dir. (backupReadAuth)
func (s *Server) listBackupsHandler(c *fiber.Ctx) error {
	files, err := listBackupFiles()
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(fiber.Map{"dir": backupDir(), "backups": files, "dialect": dialect()})
}

// GET /backup/download?format=db|sql — generate a fresh full backup and stream
// it. For SQLite: db → binary snapshot (VACUUM INTO); sql → text dump.
// For PostgreSQL: always produces a .sql dump via pg_dump. (backupReadAuth)
func (s *Server) downloadBackup(c *fiber.Ctx) error {
	format := normalizeBackupFormat(c.Query("format", "full"))
	ts := wibTimeFormat(time.Now(), "20060102-150405")
	fname := fmt.Sprintf("pkbm-lms-%s.%s", ts, format)
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		return fiber.NewError(500, "tidak dapat membuat direktori backup")
	}
	tmp, err := os.CreateTemp("", "pkbm-dl-*")
	if err != nil {
		return fiber.NewError(500, "tidak dapat membuat file sementara")
	}
	tmpPath := tmp.Name()
	if isSQLite() {
		if format == "sql" {
			err = s.dumpSQL(tmp)
			tmp.Close()
		} else {
			tmp.Close()
			_ = os.Remove(tmpPath)
			err = s.backupBinary(tmpPath)
		}
	} else {
		tmp.Close()
		_ = os.Remove(tmpPath)
		err = pgDump(tmpPath)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return fiber.NewError(500, "gagal membuat backup: "+err.Error())
	}
	c.Set("Content-Disposition", `attachment; filename="`+fname+`"`)
	if format == "sql" {
		c.Set("Content-Type", "application/sql; charset=utf-8")
	} else {
		c.Set("Content-Type", "application/octet-stream")
	}
	if err := c.SendFile(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Remove(tmpPath)
	return nil
}

// GET /backup/file/:name — download a previously created backup file by name.
// (backupReadAuth) Path traversal guarded.
func (s *Server) downloadBackupFile(c *fiber.Ctx) error {
	name := c.Params("name")
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fiber.NewError(400, "nama file tidak valid")
	}
	if !strings.HasPrefix(name, "pkbm-lms-") && !strings.HasPrefix(name, "pre-restore-") {
		return fiber.NewError(400, "hanya file backup yang dapat diunduh")
	}
	full := filepath.Join(backupDir(), name)
	if _, err := os.Stat(full); err != nil {
		return fiber.NewError(404, "file backup tidak ditemukan")
	}
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return c.SendFile(full)
}

// POST /backup (admin JWT) — create a backup now in the backup dir and return
// its metadata. The admin UI uses this; n8n uses GET /backup/download instead.
func (s *Server) createBackupNow(c *fiber.Ctx) error {
	format := normalizeBackupFormat(c.Query("format", "full"))
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		return fiber.NewError(500, "tidak dapat membuat direktori backup")
	}
	ts := wibTimeFormat(time.Now(), "20060102-150405")
	name := fmt.Sprintf("pkbm-lms-%s.%s", ts, format)
	dest := filepath.Join(backupDir(), name)
	if isSQLite() {
		if format == "sql" {
			f, err := os.Create(dest)
			if err != nil {
				return fiber.NewError(500, err.Error())
			}
			err = s.dumpSQL(f)
			f.Close()
			if err != nil {
				_ = os.Remove(dest)
				return fiber.NewError(500, "gagal dump: "+err.Error())
			}
		} else {
			if err := s.backupBinary(dest); err != nil {
				_ = os.Remove(dest)
				return fiber.NewError(500, "gagal backup: "+err.Error())
			}
		}
	} else {
		if err := pgDump(dest); err != nil {
			_ = os.Remove(dest)
			return fiber.NewError(500, "gagal pg_dump: "+err.Error())
		}
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "backup", "system", "manual backup -> "+dest)
	info, err := os.Stat(dest)
	if err != nil {
		return fiber.NewError(500, "backup selesai tetapi file hasil tidak dapat diverifikasi")
	}
	return c.JSON(fiber.Map{"name": name, "size": info.Size(), "path": dest, "format": format})
}

// POST /backup/restore (admin JWT) — stage an uploaded backup file for restore.
// SQLite: staged as pending file, applied on next restart.
// PostgreSQL: applied immediately via psql (no restart needed).
func (s *Server) stageRestore(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "file backup wajib diunggah (field name=file)")
	}
	if isSQLite() {
		tmpPath, ext, err := saveRestoreUploadForRestore(c, fh)
		if err != nil {
			return err
		}
		switch ext {
		case ".db":
			if err := validateSQLiteBackup(tmpPath); err != nil {
				_ = os.Remove(tmpPath)
				return fiber.NewError(400, "file .db tidak valid: "+err.Error())
			}
			if err := replacePendingRestore(tmpPath, pendingDBPath); err != nil {
				return fiber.NewError(500, "gagal menyimpan file restore")
			}
		case ".sql":
			if err := validateSQLBackup(tmpPath); err != nil {
				_ = os.Remove(tmpPath)
				return fiber.NewError(400, "file .sql tidak valid: "+err.Error())
			}
			if err := replacePendingRestore(tmpPath, pendingSQLPath); err != nil {
				return fiber.NewError(500, "gagal menyimpan file restore")
			}
		default:
			return fiber.NewError(400, "ekstensi file harus .db atau .sql")
		}
		uid := c.Locals("userID").(string)
		mode := ext[1:]
		s.audit(&uid, "restore", "system", "staged "+mode+" restore pending restart")
		restartScheduled := scheduleRestoreRestart()
		message := "Restore disiapkan."
		if restartScheduled {
			message += " Server akan restart otomatis untuk menerapkannya."
		} else {
			message += " Restart server untuk menerapkan."
		}
		message += " DB saat ini otomatis di-backup ke backups/pre-restore-<waktu>.db sebagai pengaman."
		return c.JSON(fiber.Map{
			"ok":               true,
			"mode":             mode,
			"restartScheduled": restartScheduled,
			"message":          message,
		})
	}

	// PostgreSQL: apply immediately via psql
	tmpPath, ext, err := saveRestoreUploadForRestore(c, fh)
	if err != nil {
		return err
	}
	if ext != ".sql" {
		_ = os.Remove(tmpPath)
		return fiber.NewError(400, "restore PostgreSQL hanya menerima file .sql")
	}
	stat, err := os.Stat(tmpPath)
	if err != nil || stat.Size() == 0 {
		_ = os.Remove(tmpPath)
		return fiber.NewError(400, "file restore PostgreSQL kosong atau tidak dapat dibaca")
	}
	uid := c.Locals("userID").(string)
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fiber.NewError(500, "tidak dapat menyiapkan folder backup pengaman")
	}
	preRestorePath := filepath.Join(backupDir(), fmt.Sprintf("pre-restore-%s.sql", wibTimeFormat(time.Now(), "20060102-150405")))
	if err := pgDump(preRestorePath); err != nil {
		_ = os.Remove(tmpPath)
		return fiber.NewError(500, "backup pengaman sebelum restore gagal")
	}
	if err := pgRestore(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		s.audit(&uid, "restore", "system", "psql restore FAILED; safety backup: "+preRestorePath+"; "+err.Error())
		return fiber.NewError(500, "gagal restore PostgreSQL; backup pengaman tersimpan di "+preRestorePath)
	}
	_ = os.Remove(tmpPath)
	s.audit(&uid, "restore", "system", "psql restore applied successfully; safety backup: "+preRestorePath)
	return c.JSON(fiber.Map{
		"ok":      true,
		"mode":    "sql",
		"message": "Restore PostgreSQL berhasil diterapkan. Backup pengaman tersimpan di " + preRestorePath + ".",
	})
}

func saveRestoreUpload(c *fiber.Ctx, fh *multipart.FileHeader) (string, error) {
	tmp, err := os.CreateTemp(".", "pkbm-restore-upload-*")
	if err != nil {
		return "", fiber.NewError(500, "tidak dapat membuat file sementara")
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fiber.NewError(500, "tidak dapat menyiapkan file sementara")
	}
	if err := c.SaveFile(fh, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fiber.NewError(500, "gagal menyimpan file restore")
	}
	return tmpPath, nil
}

func replacePendingRestore(tmpPath, pendingPath string) error {
	// Only replace pending files after the uploaded file has passed validation.
	if err := os.Remove(pendingDBPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(pendingSQLPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, pendingPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func validateSQLiteBackup(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	hdr := make([]byte, 16)
	n, readErr := f.Read(hdr)
	_ = f.Close()
	if readErr != nil || n < 16 || string(hdr[:15]) != "SQLite format 3" {
		return fmt.Errorf("bukan database SQLite yang valid")
	}
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var result string
	if err := sqlDB.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(result)) != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}
	return nil
}

func validateSQLBackup(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return fmt.Errorf("file kosong")
	}
	tmp, err := os.CreateTemp(".", "pkbm-restore-validate-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpPath)
	defer os.Remove(tmpPath)
	return replaySQLToDatabase(tmpPath, string(content))
}

func replaySQLToDatabase(path, content string) error {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	for _, stmt := range splitSQLStatements(content) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := sqlDB.Exec(stmt); err != nil {
			_ = sqlDB.Close()
			return fmt.Errorf("statement gagal: %w", err)
		}
	}
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		_ = sqlDB.Close()
		return err
	}
	closeErr := sqlDB.Close()
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	return closeErr
}

func scheduleRestoreRestart() bool {
	if env("APP_ENV", "development") != "production" || strings.EqualFold(env("BACKUP_AUTO_RESTART", "true"), "false") {
		return false
	}
	// Let Fiber flush the upload response before the process exits. Docker,
	// Coolify, systemd, or another supervisor then starts the server again and
	// applyPendingRestore performs the atomic swap before opening the DB.
	time.AfterFunc(750*time.Millisecond, func() { os.Exit(0) })
	return true
}

// deleteBackupFile (admin JWT) — DELETE /backup/:name — remove a backup file.
func (s *Server) deleteBackupFile(c *fiber.Ctx) error {
	name := c.Params("name")
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fiber.NewError(400, "nama file tidak valid")
	}
	if !strings.HasPrefix(name, "pkbm-lms-") {
		return fiber.NewError(400, "hanya file backup terjadwal/manual yang dapat dihapus (pre-restore pengaman tidak boleh dihapus dari sini)")
	}
	full := filepath.Join(backupDir(), name)
	if err := os.Remove(full); err != nil {
		return fiber.NewError(404, "file backup tidak ditemukan")
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "backup", "deleted backup "+name)
	return c.JSON(fiber.Map{"ok": true})
}
