package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"pkbm-lms/backend/internal/studentmcp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

const (
	studentMCPResource   = "peserta_didik"
	studentMCPAction     = "mcp_apply"
	studentMCPPreviewTTL = 15 * time.Minute
)

var (
	studentMCPNIS   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,39}$`)
	studentMCPNISN  = regexp.MustCompile(`^[0-9]{10}$`)
	studentMCPNIK   = regexp.MustCompile(`^[0-9]{16}$`)
	studentMCPRowID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

type studentMCPAPI struct {
	db           *gorm.DB
	keyHash      [sha256.Size]byte
	signingKey   []byte
	production   bool
	writeEnabled bool
}

func (s *Server) registerStudentMCP(api fiber.Router) {
	key := os.Getenv("STUDENT_MCP_API_KEY")
	bridge := &studentMCPAPI{
		db:           s.db.Session(&gorm.Session{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}),
		production:   s.cfg.Env == "production",
		writeEnabled: os.Getenv("STUDENT_MCP_ALLOW_WRITE") == "true",
	}
	if studentmcp.ValidAPIKey(key) {
		bridge.keyHash = sha256.Sum256([]byte(key))
		// The client holds the API key. Only the LMS can sign a review token.
		signer := hmac.New(sha256.New, []byte(s.cfg.AccessSecret))
		signer.Write([]byte("pkbm-lms/student-mcp/v1/preview\x00"))
		signer.Write(bridge.keyHash[:])
		bridge.signingKey = signer.Sum(nil)
	}

	group := api.Group(studentmcp.APIPrefix, bridge.authorize, limiter.New(limiter.Config{
		Max: 60, Expiration: time.Minute, KeyGenerator: func(*fiber.Ctx) string { return "student-mcp" },
	}))
	group.Get("/classes", bridge.classes)
	group.Post("/search", bridge.search)
	group.Post("/preview", bridge.preview)
	group.Post("/apply", bridge.apply)
}

func (b *studentMCPAPI) authorize(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if len(b.signingKey) == 0 {
		return fiber.ErrNotFound
	}
	if (b.production && c.Protocol() != "https") || c.Get(fiber.HeaderOrigin) != "" {
		return fiber.ErrForbidden
	}
	key, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok || !studentmcp.ValidAPIKey(key) {
		return fiber.ErrUnauthorized
	}
	digest := sha256.Sum256([]byte(key))
	if subtle.ConstantTimeCompare(digest[:], b.keyHash[:]) != 1 {
		return fiber.ErrUnauthorized
	}
	return c.Next()
}

func studentMCPDecode(c *fiber.Ctx, out any) error {
	body := c.BodyRaw()
	if len(body) > studentmcp.MaxBodyBytes {
		return fiber.ErrRequestEntityTooLarge
	}
	mediaType, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if err != nil || mediaType != fiber.MIMEApplicationJSON || c.Get(fiber.HeaderContentEncoding) != "" {
		return fiber.ErrUnsupportedMediaType
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return fiber.NewError(400, "Badan permintaan harus berupa objek JSON.")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		return fiber.NewError(400, "JSON atau kolom permintaan tidak valid.")
	}
	return nil
}

func studentMCPValidName(name string) bool {
	return len(name) > 0 && len(name) <= 200 && strings.IndexFunc(name, unicode.IsControl) == -1
}

func studentMCPValidNIK(nik string) bool {
	return studentMCPNIK.MatchString(nik) && nik != "0000000000000000"
}

func studentMCPValidNISN(nisn string) bool {
	return studentMCPNISN.MatchString(nisn) && !isTemporaryNISN(nisn)
}

func studentMCPActiveClasses(db *gorm.DB) *gorm.DB {
	return db.Table("kelas AS k").
		Joins("JOIN tahun_ajarans AS ta ON ta.id = k.tahun_ajaran_id AND ta.is_aktif = ?", true).
		Joins("JOIN pokjars AS p ON p.id = k.pokjar_id").
		Select("k.id, k.jenjang, k.nama_rombel, p.nama_pokjar AS pokjar, ta.nama_tahun_ajaran AS tahun_ajaran")
}

func (b *studentMCPAPI) classes(c *fiber.Ctx) error {
	result := studentmcp.Classes{Kelas: []studentmcp.Class{}}
	if err := studentMCPActiveClasses(b.db).Order("ta.nama_tahun_ajaran, p.nama_pokjar, k.jenjang, k.nama_rombel, k.id").Find(&result.Kelas).Error; err != nil {
		return fiber.ErrInternalServerError
	}
	return c.JSON(result)
}

const studentMCPColumns = "id, nama, kelas_id, nis, nisn, status"

func (b *studentMCPAPI) search(c *fiber.Ctx) error {
	var input studentmcp.SearchRequest
	if err := studentMCPDecode(c, &input); err != nil {
		return err
	}
	if len(input.Queries) == 0 || len(input.Queries) > studentmcp.MaxRows {
		return fiber.NewError(422, "Pencarian menerima 1 sampai 100 baris.")
	}
	result := studentmcp.SearchResponse{Matches: make([]studentmcp.SearchMatch, 0, len(input.Queries))}
	for i, row := range input.Queries {
		row.Nama = strings.TrimSpace(row.Nama)
		if (row.Nama == "" && row.NIS == "" && row.NISN == "" && row.NIK == "") ||
			(row.Nama != "" && !studentMCPValidName(row.Nama)) ||
			(row.KelasID != "" && !studentMCPRowID.MatchString(row.KelasID)) ||
			(row.NIS != "" && (len(row.NIS) > 100 || strings.IndexFunc(row.NIS, unicode.IsControl) != -1)) ||
			(row.NISN != "" && !studentMCPValidNISN(row.NISN)) ||
			(row.NIK != "" && !studentMCPValidNIK(row.NIK)) {
			return fiber.NewError(422, fmt.Sprintf("Kriteria pencarian baris %d tidak valid.", i+1))
		}
		query := b.db.Model(&PesertaDidik{}).Select(studentMCPColumns)
		if row.Nama != "" {
			query = query.Where("LOWER(TRIM(nama)) = LOWER(?)", row.Nama)
		}
		for _, filter := range []struct{ column, value string }{
			{"kelas_id", row.KelasID}, {"nis", row.NIS}, {"nisn", row.NISN}, {"nik", row.NIK},
		} {
			if filter.value != "" {
				query = query.Where(filter.column+" = ?", filter.value)
			}
		}
		match := studentmcp.SearchMatch{Row: i + 1, Students: []studentmcp.Student{}}
		if err := query.Order("id").Limit(5).Find(&match.Students).Error; err != nil {
			return fiber.ErrInternalServerError
		}
		match.Ambiguous = len(match.Students) > 1
		result.Matches = append(result.Matches, match)
	}
	return c.JSON(result)
}

type studentMCPPlan struct {
	review    studentmcp.Preview
	revisions []time.Time
	created   []studentMCPCreate
}

type studentMCPCreate struct {
	student       PesertaDidik
	tahunAjaranID string
}

type studentMCPActiveClass struct {
	studentmcp.Class
	PokjarID      string
	TahunAjaranID string
}

type studentMCPTargets struct {
	ids   map[string]bool
	nis   map[string]bool
	nisn  map[string]bool
	nik   map[string]bool
	names map[string]bool
}

func (t studentMCPTargets) checkIdentifiers(db *gorm.DB, id, nis, nisn, nik string) (string, error) {
	for _, field := range []struct {
		column, display, value string
		seen                   map[string]bool
	}{
		{"nis", "NIS", nis, t.nis},
		{"nisn", "NISN", nisn, t.nisn},
		{"nik", "NIK", nik, t.nik},
	} {
		if field.value == "" || (field.column == "nisn" && isTemporaryNISN(field.value)) {
			continue
		}
		if field.seen[field.value] {
			return field.display + " muncul lebih dari sekali di batch.", nil
		}
		var count int64
		if err := db.Model(&PesertaDidik{}).Where(field.column+" = ? AND id <> ?", field.value, id).Count(&count).Error; err != nil {
			return "", err
		}
		if count > 0 {
			return field.display + " sudah tercatat untuk peserta didik lain.", nil
		}
		field.seen[field.value] = true
	}
	return "", nil
}

func studentMCPView(row PesertaDidik) studentmcp.Student {
	return studentmcp.Student{ID: row.ID, Nama: row.Nama, KelasID: row.KelasID, NIS: row.NIS, NISN: row.NISN, Status: row.Status}
}

func (b *studentMCPAPI) plan(db *gorm.DB, input studentmcp.Batch, applying bool) (studentMCPPlan, error) {
	count := len(input.Updates) + len(input.Creates)
	plan := studentMCPPlan{review: studentmcp.Preview{
		Changes: []studentmcp.Change{}, NewStudents: []studentmcp.NewStudent{}, Issues: []studentmcp.Issue{}, WriteEnabled: b.writeEnabled,
	}}
	if count == 0 || count > studentmcp.MaxRows {
		return plan, fiber.NewError(422, "Batch menerima 1 sampai 100 baris gabungan.")
	}
	targets := studentMCPTargets{ids: map[string]bool{}, nis: map[string]bool{}, nisn: map[string]bool{}, nik: map[string]bool{}, names: map[string]bool{}}
	for i, row := range input.Updates {
		before, after, revision, issue, err := studentMCPPlanUpdate(db, targets, row)
		if err != nil {
			return plan, err
		}
		if issue != "" {
			plan.review.Issues = append(plan.review.Issues, studentmcp.Issue{Section: "updates", Row: i + 1, Message: issue})
			continue
		}
		plan.revisions = append(plan.revisions, revision)
		if before == after {
			plan.review.Unchanged++
			continue
		}
		plan.review.Changes = append(plan.review.Changes, studentmcp.Change{Row: i + 1, Before: before, After: after})
	}
	for i, row := range input.Creates {
		entry, class, issue, err := studentMCPPlanCreate(db, targets, row, applying)
		if err != nil {
			return plan, err
		}
		if issue != "" {
			plan.review.Issues = append(plan.review.Issues, studentmcp.Issue{Section: "creates", Row: i + 1, Message: issue})
			continue
		}
		plan.created = append(plan.created, entry)
		plan.review.NewStudents = append(plan.review.NewStudents, studentmcp.NewStudent{Row: i + 1, Student: studentMCPView(entry.student), Class: class})
	}
	return plan, nil
}

func studentMCPPlanUpdate(db *gorm.DB, targets studentMCPTargets, row studentmcp.Update) (before, after studentmcp.Student, revision time.Time, issue string, err error) {
	row.ExpectedName = strings.TrimSpace(row.ExpectedName)
	if !studentMCPRowID.MatchString(row.StudentID) || !studentMCPRowID.MatchString(row.ExpectedClassID) || !studentMCPValidName(row.ExpectedName) ||
		(row.ExpectedNIK != "" && !studentMCPValidNIK(row.ExpectedNIK)) {
		return before, after, revision, "ID internal, nama, kelas, atau NIK verifikasi tidak valid.", nil
	}
	if (row.NIS == "" && row.NISN == "") || (row.NIS != "" && !studentMCPNIS.MatchString(row.NIS)) || (row.NISN != "" && !studentMCPValidNISN(row.NISN)) {
		return before, after, revision, "Isi NIS yang valid dan/atau NISN resmi 10 digit.", nil
	}
	if targets.ids[row.StudentID] {
		return before, after, revision, "ID peserta didik muncul lebih dari sekali di batch.", nil
	}
	targets.ids[row.StudentID] = true
	var current PesertaDidik
	err = db.Select(studentMCPColumns+", nik, updated_at").First(&current, "id = ?", row.StudentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return before, after, revision, "ID peserta didik tidak ditemukan.", nil
	}
	if err != nil {
		return before, after, revision, "", err
	}
	if !strings.EqualFold(strings.TrimSpace(current.Nama), row.ExpectedName) || current.KelasID != row.ExpectedClassID ||
		(row.ExpectedNIK != "" && row.ExpectedNIK != current.NIK) {
		return before, after, revision, "Nama, kelas, atau NIK verifikasi tidak cocok dengan ID yang dipilih.", nil
	}
	var sameName int64
	if err := db.Model(&PesertaDidik{}).Where("kelas_id = ? AND LOWER(TRIM(nama)) = LOWER(?) AND id <> ?", current.KelasID, strings.TrimSpace(current.Nama), current.ID).Count(&sameName).Error; err != nil {
		return before, after, revision, "", err
	}
	if sameName > 0 && row.ExpectedNIK == "" {
		return before, after, revision, "Ada nama yang sama dalam kelas. Cocokkan NIK dari sumber resmi.", nil
	}
	if issue, err = targets.checkIdentifiers(db, current.ID, row.NIS, row.NISN, row.ExpectedNIK); issue != "" || err != nil {
		return before, after, revision, issue, err
	}
	before = studentMCPView(current)
	if row.NIS != "" {
		current.NIS = row.NIS
	}
	if row.NISN != "" {
		current.NISN = row.NISN
	}
	return before, studentMCPView(current), current.UpdatedAt, "", nil
}

func studentMCPPlanCreate(db *gorm.DB, targets studentMCPTargets, row studentmcp.Create, applying bool) (entry studentMCPCreate, class studentmcp.Class, issue string, err error) {
	row.Nama = strings.TrimSpace(row.Nama)
	if !studentMCPValidName(row.Nama) || !studentMCPRowID.MatchString(row.KelasID) || (row.JenisKelamin != "L" && row.JenisKelamin != "P") ||
		!studentMCPNIS.MatchString(row.NIS) || !studentMCPValidNIK(row.NIK) || (row.NISN != "" && !studentMCPNISN.MatchString(row.NISN)) {
		return entry, class, "Nama, kelas, jenis kelamin, NIK, NIS, atau NISN tidak valid.", nil
	}
	var birthDate *time.Time
	if row.TanggalLahir != "" {
		date, err := time.Parse(time.DateOnly, row.TanggalLahir)
		if err != nil || date.Year() < 1900 || date.Format(time.DateOnly) > time.Now().In(wibLocation).Format(time.DateOnly) {
			return entry, class, "Tanggal lahir tidak valid. Gunakan YYYY-MM-DD.", nil
		}
		birthDate = &date
	}
	nameKey := strings.ToLower(row.Nama)
	var existingName int64
	if err := db.Model(&PesertaDidik{}).Where("LOWER(TRIM(nama)) = LOWER(?)", row.Nama).Count(&existingName).Error; err != nil {
		return entry, class, "", err
	}
	if existingName > 0 || targets.names[nameKey] {
		return entry, class, "Nama sudah ditemukan. Periksa identitas sebelum menambahkan peserta.", nil
	}
	targets.names[nameKey] = true
	if issue, err = targets.checkIdentifiers(db, "", row.NIS, row.NISN, row.NIK); issue != "" || err != nil {
		return entry, class, issue, err
	}
	var activeClass studentMCPActiveClass
	classes := studentMCPActiveClasses(db).Select("k.id, k.jenjang, k.nama_rombel, p.nama_pokjar AS pokjar, ta.nama_tahun_ajaran AS tahun_ajaran, k.pokjar_id, k.tahun_ajaran_id")
	if applying && db.Dialector.Name() == "postgres" {
		// Keep these references valid until the student and history are committed.
		classes = classes.Clauses(clause.Locking{Strength: "SHARE"})
	}
	result := classes.Where("k.id = ?", row.KelasID).Limit(1).Find(&activeClass)
	if result.Error != nil {
		return entry, class, "", result.Error
	}
	if result.RowsAffected == 0 {
		return entry, class, "Kelas atau pokjar tidak ditemukan pada tahun ajaran aktif.", nil
	}
	if row.NISN == "" {
		row.NISN = temporaryNISN
	}
	entry = studentMCPCreate{student: PesertaDidik{Nama: row.Nama, JenisKelamin: row.JenisKelamin, NIS: row.NIS, NISN: row.NISN, NIK: row.NIK,
		TanggalLahir: birthDate, KelasID: row.KelasID, PokjarID: activeClass.PokjarID, Status: "aktif"}, tahunAjaranID: activeClass.TahunAjaranID}
	return entry, activeClass.Class, "", nil
}

func studentMCPWritable(plan studentMCPPlan) bool {
	return len(plan.review.Issues) == 0 && len(plan.review.Changes)+len(plan.review.NewStudents) > 0
}

func (b *studentMCPAPI) sign(expiry string, input studentmcp.Batch, plan studentMCPPlan) string {
	mac := hmac.New(sha256.New, b.signingKey)
	// Sign the supplied identifiers without putting them inside the token.
	// Sign the database observations as well, so a stale preview cannot write.
	refs := make([][2]string, 0, len(plan.created))
	for _, row := range plan.created {
		refs = append(refs, [2]string{row.student.PokjarID, row.tahunAjaranID})
	}
	_ = json.NewEncoder(mac).Encode(struct {
		Expiry    string
		Input     studentmcp.Batch
		Changes   []studentmcp.Change
		New       []studentmcp.NewStudent
		Revisions []time.Time
		Refs      [][2]string
	}{expiry, input, plan.review.Changes, plan.review.NewStudents, plan.revisions, refs})
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (b *studentMCPAPI) preview(c *fiber.Ctx) error {
	var input studentmcp.Batch
	if err := studentMCPDecode(c, &input); err != nil {
		return err
	}
	plan, err := b.plan(b.db, input, false)
	if err != nil {
		return studentMCPSafeError(err)
	}
	if studentMCPWritable(plan) {
		expires := time.Now().UTC().Add(studentMCPPreviewTTL).Truncate(time.Second)
		expiry := strconv.FormatInt(expires.Unix(), 10)
		plan.review.PreviewToken = expiry + "." + b.sign(expiry, input, plan)
		plan.review.ExpiresAt = expires.Format(time.RFC3339)
	}
	return c.JSON(plan.review)
}

func (b *studentMCPAPI) apply(c *fiber.Ctx) error {
	if !b.writeEnabled {
		return fiber.NewError(403, "Penulisan peserta didik lewat MCP belum diaktifkan.")
	}
	var input studentmcp.ApplyRequest
	if err := studentMCPDecode(c, &input); err != nil {
		return err
	}
	expiry, signature, ok := strings.Cut(input.PreviewToken, ".")
	if !ok || len(expiry) > 12 || !studentmcp.ValidAPIKey(signature) {
		return fiber.NewError(422, "Token pratinjau tidak valid.")
	}
	until, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || until <= time.Now().Unix() {
		return fiber.NewError(409, "Pratinjau sudah kedaluwarsa. Buat pratinjau baru.")
	}

	digest := sha256.Sum256([]byte(input.PreviewToken))
	batchID := hex.EncodeToString(digest[:16])
	ctx, cancel := context.WithTimeout(c.UserContext(), 12*time.Second)
	defer cancel()
	var receipt studentmcp.Receipt
	err = b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			// Serialize the duplicate checks against all roster writers. Only
			// the student table is locked here and reads may continue normally.
			if err := tx.Exec("SET LOCAL lock_timeout = '3s'").Error; err != nil {
				return err
			}
			if err := tx.Exec("LOCK TABLE peserta_didiks IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				return err
			}
		}
		if until <= time.Now().Unix() {
			return fiber.NewError(409, "Pratinjau sudah kedaluwarsa. Buat pratinjau baru.")
		}
		var prior AuditLog
		err := tx.Where("action = ? AND resource = ? AND detail LIKE ?", studentMCPAction, studentMCPResource, "batch="+batchID+" %").First(&prior).Error
		if err == nil {
			return fiber.NewError(409, "Pratinjau ini sudah diterapkan. Periksa data terbaru sebelum mengulang.")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		plan, err := b.plan(tx, input.Changes, true)
		if err != nil {
			return err
		}
		if !studentMCPWritable(plan) || !hmac.Equal([]byte(signature), []byte(b.sign(expiry, input.Changes, plan))) {
			return fiber.NewError(409, "Batch atau data LMS berubah sejak pratinjau. Buat pratinjau baru dan tinjau lagi.")
		}
		for _, change := range plan.review.Changes {
			fields := make(map[string]any, 2)
			if change.Before.NIS != change.After.NIS {
				fields["nis"] = change.After.NIS
			}
			if change.Before.NISN != change.After.NISN {
				fields["nisn"] = change.After.NISN
			}
			result := tx.Model(&PesertaDidik{}).Where("id = ?", change.Before.ID).Updates(fields)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fiber.NewError(409, "Peserta didik berubah selama proses. Buat pratinjau baru.")
			}
		}
		for _, entry := range plan.created {
			if err := tx.Create(&entry.student).Error; err != nil {
				return err
			}
			history := RiwayatKelasPesertaDidik{PesertaDidikID: entry.student.ID, KelasID: entry.student.KelasID,
				TahunAjaranID: entry.tahunAjaranID, Status: entry.student.Status}
			if err := tx.Create(&history).Error; err != nil {
				return err
			}
		}
		receipt = studentmcp.Receipt{BatchID: batchID, Updated: len(plan.review.Changes), Created: len(plan.created), Unchanged: plan.review.Unchanged}
		detail := fmt.Sprintf("batch=%s updated=%d created=%d unchanged=%d", batchID, receipt.Updated, receipt.Created, receipt.Unchanged)
		return tx.Create(&AuditLog{Action: studentMCPAction, Resource: studentMCPResource, Detail: detail}).Error
	})
	if err != nil {
		return studentMCPSafeError(err)
	}
	return c.JSON(receipt)
}

func studentMCPSafeError(err error) error {
	var safe *fiber.Error
	if errors.As(err, &safe) {
		return safe
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fiber.NewError(409, "Identitas peserta didik sudah digunakan. Buat pratinjau baru.")
	}
	return fiber.ErrInternalServerError
}
