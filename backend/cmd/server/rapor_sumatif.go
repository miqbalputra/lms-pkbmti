package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"gorm.io/gorm"
)

// ReportScope is the stable identity of a report, including its historical class.
type ReportScope struct {
	KelasID       string `gorm:"uniqueIndex:report_period" json:"kelasId"`
	TahunAjaranID string `gorm:"uniqueIndex:report_period" json:"tahunAjaranId"`
	Semester      string `gorm:"uniqueIndex:report_period" json:"semester"`
}
type ReportPeriod struct {
	Base
	ReportScope
	Revision int    `json:"revision"`
	Config   string `gorm:"type:text" json:"-"`
}
type ReportCP struct {
	Base
	PeriodID string `gorm:"index" json:"periodId"`
	MapelID  string `gorm:"index" json:"mapelId"`
	Label    string `json:"label"`
	Urutan   int    `json:"urutan"`
}
type ReportScore struct {
	Base
	CPID      string   `gorm:"uniqueIndex:report_score" json:"cpId"`
	StudentID string   `gorm:"uniqueIndex:report_score" json:"studentId"`
	Value     *float64 `json:"value"`
}
type ReportNarrative struct {
	Base
	PeriodID         string `gorm:"uniqueIndex:report_narrative" json:"periodId"`
	StudentID        string `gorm:"uniqueIndex:report_narrative" json:"studentId"`
	MapelID          string `gorm:"uniqueIndex:report_narrative" json:"mapelId"`
	Strength         string `json:"strength"`
	Growth           string `json:"growth"`
	ReviewedRevision int    `json:"reviewedRevision"`
}
type ReportRecord struct {
	Base
	PeriodID  string `gorm:"uniqueIndex:report_record" json:"periodId"`
	StudentID string `gorm:"uniqueIndex:report_record" json:"studentId"`
	Status    string `json:"status"`
	Version   int    `json:"version"`
	Details   string `gorm:"type:text" json:"-"`
	Snapshot  string `gorm:"type:text" json:"-"`
}
type ReportRevision struct {
	Base
	RecordID string `gorm:"uniqueIndex:report_version" json:"recordId"`
	Version  int    `gorm:"uniqueIndex:report_version" json:"version"`
	Snapshot string `gorm:"type:text" json:"-"`
}
type ReportImport struct {
	Base
	Hash      string `gorm:"uniqueIndex" json:"hash"`
	Payload   string `gorm:"type:text" json:"-"`
	Confirmed bool   `json:"confirmed"`
}
type ReportSettings struct {
	Base
	Provider string `json:"provider"`
	Model    string `json:"model"`
}
type ReportSubjectConfig struct {
	MapelID string `json:"mapelId"`
	Name    string `json:"name"`
	Group   string `json:"group"`
}
type ReportConfig struct {
	Subjects           []ReportSubjectConfig `json:"subjects"`
	Program            string                `json:"program"`
	Phase              string                `json:"phase"`
	City               string                `json:"city"`
	Date               string                `json:"date"`
	Principal          string                `json:"principal"`
	PrincipalNIP       string                `json:"principalNip"`
	Teacher            string                `json:"teacher"`
	TeacherNIP         string                `json:"teacherNip"`
	PrincipalSignature string                `json:"principalSignature,omitempty"`
	ExtrasEnabled      *bool                 `json:"extrasEnabled,omitempty"`
	ExtraActivities    []string              `json:"extraActivities,omitempty"`
	AttendanceMode     string                `json:"attendanceMode,omitempty"`
}
type ReportExtra struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var reportIdentityKeys = map[string]bool{"birthPlace": true, "birthDate": true, "phone": true, "address": true, "religion": true, "father": true, "mother": true, "fatherJob": true, "motherJob": true, "guardian": true, "guardianJob": true, "admissionClass": true, "admissionYear": true, "admissionStatus": true}

type ReportAttendance struct {
	Sakit     int      `json:"sakit"`
	Izin      int      `json:"izin"`
	Alpa      int      `json:"alpa"`
	Conflicts []string `json:"conflicts"`
}
type ReportDetails struct {
	Note                 string                  `json:"note"`
	Promotion            string                  `json:"promotion"`
	NextClass            string                  `json:"nextClass"`
	Extras               []ReportExtra           `json:"extras"`
	HistoricalAttendance *ReportAttendance       `json:"historicalAttendance,omitempty"`
	ManualAttendance     *ReportAttendanceCounts `json:"manualAttendance,omitempty"`
	Identity             map[string]string       `json:"identity,omitempty"`
}
type ReportSubject struct {
	ReportSubjectConfig
	Score    *float64 `json:"score"`
	Strength string   `json:"strength"`
	Growth   string   `json:"growth"`
	Stale    bool     `json:"stale"`
	Complete bool     `json:"complete"`
	Number   int      `json:"number"`
}
type ReportDocument struct {
	Student          PesertaDidik     `json:"student"`
	Class            Kelas            `json:"class"`
	Config           ReportConfig     `json:"config"`
	Scope            ReportScope      `json:"scope"`
	Subjects         []ReportSubject  `json:"subjects"`
	Details          ReportDetails    `json:"details"`
	Attendance       ReportAttendance `json:"attendance"`
	AttendanceSource string           `json:"attendanceSource,omitempty"`
	Issues           []string         `json:"issues"`
}

func reportModels() []interface{} {
	return []interface{}{&ReportPeriod{}, &ReportCP{}, &ReportScore{}, &ReportNarrative{}, &ReportRecord{}, &ReportRevision{}, &ReportImport{}, &ReportSettings{}}
}
func (s *Server) reportRoutes(api fiber.Router) {
	api.Get("/rapor-sumatif/classes", s.reportClasses)
	api.Get("/rapor-sumatif/period", s.reportPeriod)
	api.Put("/rapor-sumatif/config", s.reportSaveConfig)
	api.Get("/rapor-sumatif/attendance", s.reportAttendanceGrid)
	api.Put("/rapor-sumatif/attendance", s.reportSaveAttendance)
	api.Put("/rapor-sumatif/attendance-mode", s.reportSaveAttendanceMode)
	api.Get("/rapor-sumatif/grid", s.reportGrid)
	api.Put("/rapor-sumatif/grid", s.reportSaveGrid)
	api.Get("/rapor-sumatif/document/:studentId", s.reportDocumentHandler)
	api.Put("/rapor-sumatif/document/:studentId", s.reportSaveDetails)
	api.Post("/rapor-sumatif/document/:studentId/status", s.reportSetStatus)
	api.Get("/rapor-sumatif/print", s.reportPrint)
	api.Get("/rapor-sumatif/leger", s.reportLedger)
	api.Get("/rapor-sumatif/ai/settings", s.reportAISettings)
	api.Put("/rapor-sumatif/ai/settings", s.reportAISettings)
	api.Post("/rapor-sumatif/ai/suggest", limiter.New(limiter.Config{Max: 60, Expiration: time.Minute, KeyGenerator: func(c *fiber.Ctx) string { return fmt.Sprint(c.Locals("userID")) }}), s.reportAISuggest)
	api.Post("/rapor-sumatif/import/preview", s.reportImportPreview)
	api.Post("/rapor-sumatif/import/:id/confirm", s.reportImportConfirm)
	api.Get("/orang-tua/anak/:id/rapor-terbit", s.reportParent)
	api.Get("/orang-tua/anak/:id/rapor-terbit/:reportId/print", s.reportParentPrint)
}
func (s *Server) reportClasses(c *fiber.Ctx) error {
	q := s.db.Order("jenjang, nama_rombel").Preload("TahunAjaran")
	switch c.Locals("role") {
	case "admin", "kepala_sekolah":
	case "guru":
		var user User
		if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil {
			return err
		}
		if user.TutorID == nil {
			return c.JSON([]Kelas{})
		}
		q = q.Where("wali_kelas_id = ? OR id IN (?)", *user.TutorID, s.db.Model(&PenugasanGuruMapel{}).Select("kelas_id").Where("tutor_id = ?", *user.TutorID))
	default:
		return fiber.ErrForbidden
	}
	rows := []Kelas{}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}
func reportDecode(raw string, into interface{}) {
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), into)
	}
}
func reportJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
func reportSemester(v string) bool    { return v == "Ganjil" || v == "Genap" }
func (s *Server) reportScope(c *fiber.Ctx, write bool) (Kelas, ReportPeriod, error) {
	var k Kelas
	var p ReportPeriod
	if !reportSemester(c.Query("semester")) {
		return k, p, fiber.NewError(400, "Semester Ganjil/Genap wajib dipilih")
	}
	if err := s.db.Preload("TahunAjaran").Preload("WaliKelas").First(&k, "id = ?", c.Query("kelasId")).Error; err != nil {
		return k, p, fiber.NewError(404, "Kelas tidak ditemukan")
	}
	if c.Query("tahunAjaranId") != "" && c.Query("tahunAjaranId") != k.TahunAjaranID {
		return k, p, fiber.NewError(400, "Tahun ajaran tidak sesuai kelas")
	}
	role := c.Locals("role")
	if role != "admin" && role != "kepala_sekolah" {
		if role != "guru" {
			return k, p, fiber.ErrForbidden
		}
		if err := s.canManageKelas(c, k.ID); err != nil {
			var u User
			var count int64
			s.db.First(&u, "id = ?", c.Locals("userID"))
			if u.TutorID == nil {
				return k, p, fiber.ErrForbidden
			}
			s.db.Model(&PenugasanGuruMapel{}).Where("kelas_id = ? AND tutor_id = ?", k.ID, *u.TutorID).Count(&count)
			if count == 0 {
				return k, p, fiber.ErrForbidden
			}
		}
	}
	if write {
		if role == "kepala_sekolah" {
			return k, p, fiber.ErrForbidden
		}
		if err := s.requireActiveYear(c, k.ID); err != nil {
			return k, p, err
		}
		var sem Semester
		err := s.db.Where("tahun_ajaran_id = ? AND nama_semester = ?", k.TahunAjaranID, c.Query("semester")).First(&sem).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return k, p, err
		}
		if sem.IsArchived {
			return k, p, fiber.NewError(409, "Semester diarsipkan")
		}
	}
	err := s.db.Where("kelas_id = ? AND tahun_ajaran_id = ? AND semester = ?", k.ID, k.TahunAjaranID, c.Query("semester")).First(&p).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return k, p, err
	}
	if p.ID == "" {
		p.ReportScope = ReportScope{k.ID, k.TahunAjaranID, c.Query("semester")}
	}
	return k, p, nil
}
func (s *Server) reportRoster(db *gorm.DB, k Kelas) ([]PesertaDidik, error) {
	rows := []PesertaDidik{}
	err := db.Where("kelas_id = ? OR id IN (?)", k.ID, db.Model(&RiwayatKelasPesertaDidik{}).Select("peserta_didik_id").Where("kelas_id = ? AND tahun_ajaran_id = ?", k.ID, k.TahunAjaranID)).Order("urutan, nama, id").Find(&rows).Error
	return rows, err
}
func (s *Server) reportConfig(db *gorm.DB, k Kelas, p ReportPeriod) (ReportConfig, error) {
	cfg := ReportConfig{Program: "PAKET A", Phase: "A", City: "Purbalingga", Principal: "Muhamad Riana Yudianto, S.Kom.", PrincipalNIP: "-", TeacherNIP: "-"}
	if k.Jenjang > 6 {
		cfg.Program = "PAKET B"
	}
	if k.Jenjang > 9 {
		cfg.Program = "PAKET C"
	}
	if k.Jenjang > 2 {
		cfg.Phase = "B"
	}
	if k.Jenjang > 4 {
		cfg.Phase = "C"
	}
	if k.Jenjang > 6 {
		cfg.Phase = "D"
	}
	if k.Jenjang > 9 {
		cfg.Phase = "E"
	}
	if k.Jenjang > 10 {
		cfg.Phase = "F"
	}
	if k.WaliKelas != nil {
		cfg.Teacher = k.WaliKelas.Nama
	}
	reportDecode(p.Config, &cfg)
	{
		var links []KelasMapel
		if err := db.Preload("Mapel").Where("kelas_id = ?", k.ID).Order("created_at, id").Find(&links).Error; err != nil {
			return cfg, err
		}
		for _, link := range links {
			exists := false
			for _, mp := range cfg.Subjects {
				if mp.MapelID == link.MapelID {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			group := "umum"
			if strings.Contains(strings.ToLower(link.Mapel.NamaMapel), "pemberdayaan") || strings.EqualFold(link.Mapel.NamaMapel, "Keterampilan") {
				group = "pemberdayaan"
			}
			cfg.Subjects = append(cfg.Subjects, ReportSubjectConfig{link.MapelID, link.Mapel.NamaMapel, group})
		}
	}
	sort.SliceStable(cfg.Subjects, func(i, j int) bool {
		return cfg.Subjects[i].Group != "pemberdayaan" && cfg.Subjects[j].Group == "pemberdayaan"
	})
	return cfg, nil
}

// reportLock serializes every write in a period on SQLite and PostgreSQL.
func reportLock(tx *gorm.DB, p *ReportPeriod, expected int) error {
	if p.ID == "" {
		if expected != 0 {
			return fiber.NewError(409, "Data berubah; muat ulang")
		}
		p.Revision = 1
		if err := tx.Create(p).Error; err != nil {
			if isUniqueErr(err) {
				return fiber.NewError(409, "Periode baru saja dibuat pengguna lain; muat ulang")
			}
			return err
		}
		return nil
	}
	r := tx.Model(&ReportPeriod{}).Where("id = ? AND revision = ?", p.ID, expected).Update("revision", gorm.Expr("revision + 1"))
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return fiber.NewError(409, "Data berubah; muat ulang sebelum menyimpan")
	}
	p.Revision = expected + 1
	return nil
}
func reportUnlocked(tx *gorm.DB, periodID, studentID string) error {
	q := tx.Model(&ReportRecord{}).Where("period_id = ? AND status IN ?", periodID, []string{"final", "published"})
	if studentID != "" {
		q = q.Where("student_id = ?", studentID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fiber.NewError(409, "Rapor final harus dibuka sebagai revisi terlebih dahulu")
	}
	return nil
}
func reportCalculate(cps []ReportCP, scores []ReportScore) (*float64, string, string, bool) {
	values := map[string]*float64{}
	for _, v := range scores {
		values[v.CPID] = v.Value
	}
	count := 0
	sum := 0.0
	for _, cp := range cps {
		if v := values[cp.ID]; v != nil {
			count++
			sum += *v
		}
	}
	if count == 0 {
		return nil, "", "", false
	}
	avg := math.Round(sum / float64(count))
	a, b := reportAutomaticNarrative(cps, scores)
	return &avg, a, b, count == len(cps)
}
func (s *Server) reportPeriod(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	cfg, err := s.reportConfig(s.db, k, p)
	if err != nil {
		return err
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	records := []ReportRecord{}
	if p.ID != "" {
		if err = s.db.Where("period_id = ?", p.ID).Find(&records).Error; err != nil {
			return err
		}
	}
	canManage := s.canManageKelas(c, k.ID) == nil
	canInput := []string{}
	for _, m := range cfg.Subjects {
		if s.canManageKelasMapel(c, k.ID, m.MapelID) == nil {
			canInput = append(canInput, m.MapelID)
		}
	}
	return c.JSON(fiber.Map{"period": p, "config": cfg, "students": roster, "records": records, "class": k, "canManage": canManage, "canInput": canInput, "isAdmin": c.Locals("role") == "admin", "promotionRules": reportPromotionRules(k.Jenjang)})
}
func (s *Server) reportSaveConfig(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	if err = s.canManageKelas(c, k.ID); err != nil {
		return err
	}
	var in struct {
		Revision int          `json:"revision"`
		Config   ReportConfig `json:"config"`
	}
	if c.BodyParser(&in) != nil {
		return fiber.ErrBadRequest
	}
	if len(in.Config.Subjects) == 0 {
		return fiber.NewError(400, "Pilih minimal satu mapel")
	}
	if in.Config.Date != "" {
		if _, e := time.Parse("2006-01-02", in.Config.Date); e != nil {
			return fiber.NewError(400, "Tanggal rapor tidak valid")
		}
	}
	for _, value := range []string{in.Config.Program, in.Config.Phase, in.Config.City, in.Config.Teacher, in.Config.TeacherNIP, in.Config.Principal, in.Config.PrincipalNIP} {
		if len([]rune(value)) > 200 {
			return fiber.NewError(400, "Pengaturan cetak terlalu panjang")
		}
	}
	if in.Config.PrincipalSignature != "" {
		if !validSignature(in.Config.PrincipalSignature) || len(in.Config.PrincipalSignature) > 400000 || !strings.HasPrefix(in.Config.PrincipalSignature, "data:image/png;base64,") {
			return fiber.NewError(400, "Tanda tangan harus gambar PNG maksimal 300 KB")
		}
	}
	if err := reportValidateExtraActivities(in.Config.ExtraActivities); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, mp := range in.Config.Subjects {
		var count int64
		s.db.Model(&KelasMapel{}).Where("kelas_id = ? AND mapel_id = ?", k.ID, mp.MapelID).Count(&count)
		if count != 1 || seen[mp.MapelID] || strings.TrimSpace(mp.Name) == "" || (mp.Group != "umum" && mp.Group != "pemberdayaan") {
			return fiber.NewError(400, "Daftar mapel tidak valid")
		}
		seen[mp.MapelID] = true
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		if e := reportUnlocked(tx, p.ID, ""); e != nil {
			return e
		}
		// Only the dedicated admin endpoint can change the attendance source.
		var previous ReportConfig
		reportDecode(p.Config, &previous)
		in.Config.AttendanceMode = previous.AttendanceMode
		return tx.Model(&p).Update("config", reportJSON(in.Config)).Error
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "config", p.ID)
	return c.JSON(p)
}
func (s *Server) reportAudit(c *fiber.Ctx, action, id string) {
	uid, _ := c.Locals("userID").(string)
	s.audit(&uid, action, "rapor_sumatif", id)
}

func (s *Server) reportGrid(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	mp := c.Query("mapelId")
	if err = s.reportReadMapel(c, k.ID, mp); err != nil {
		return err
	}
	cps := []ReportCP{}
	scores := []ReportScore{}
	narratives := []ReportNarrative{}
	if p.ID != "" {
		if err = s.db.Where("period_id = ? AND mapel_id = ?", p.ID, mp).Order("urutan, id").Find(&cps).Error; err != nil {
			return err
		}
		ids := []string{}
		for _, cp := range cps {
			ids = append(ids, cp.ID)
		}
		if len(ids) > 0 {
			if err = s.db.Where("cp_id IN ?", ids).Find(&scores).Error; err != nil {
				return err
			}
		}
		if err = s.db.Where("period_id = ? AND mapel_id = ?", p.ID, mp).Find(&narratives).Error; err != nil {
			return err
		}
		// Final reports may have used an automatic description without a saved
		// ReportNarrative. Display their frozen text, never regenerate it using
		// today's rules. This modifies only the response, not stored data.
		frozen := []ReportRecord{}
		if err = s.db.Where("period_id = ? AND status IN ? AND snapshot <> ?", p.ID, []string{"final", "published"}, "").Find(&frozen).Error; err != nil {
			return err
		}
		for _, record := range frozen {
			var document ReportDocument
			if err = json.Unmarshal([]byte(record.Snapshot), &document); err != nil {
				return err
			}
			narrative := ReportNarrative{PeriodID: p.ID, StudentID: record.StudentID, MapelID: mp, ReviewedRevision: p.Revision}
			for _, subject := range document.Subjects {
				if subject.MapelID == mp {
					narrative.Strength, narrative.Growth = subject.Strength, subject.Growth
					break
				}
			}
			index := len(narratives)
			for i, saved := range narratives {
				if saved.StudentID == record.StudentID {
					index = i
					break
				}
			}
			if index == len(narratives) {
				narratives = append(narratives, narrative)
			} else {
				narratives[index].Strength = narrative.Strength
				narratives[index].Growth = narrative.Growth
				narratives[index].ReviewedRevision = narrative.ReviewedRevision
			}
		}
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"period": p, "cps": cps, "scores": scores, "narratives": narratives, "students": roster})
}
func (s *Server) reportReadMapel(c *fiber.Ctx, kelasID, mapelID string) error {
	var n int64
	if err := s.db.Model(&KelasMapel{}).Where("kelas_id = ? AND mapel_id = ?", kelasID, mapelID).Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return fiber.NewError(400, "Mapel tidak terdaftar pada kelas")
	}
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil
	}
	if s.canManageKelas(c, kelasID) == nil {
		return nil
	}
	return s.canManageKelasMapel(c, kelasID, mapelID)
}

type reportGridInput struct {
	Revision   int               `json:"revision"`
	CPs        []ReportCP        `json:"cps"`
	Scores     []ReportScore     `json:"scores"`
	Narratives []ReportNarrative `json:"narratives"`
}

func (s *Server) reportSaveGrid(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	mp := c.Query("mapelId")
	if err = s.reportReadMapel(c, k.ID, mp); err != nil {
		return err
	}
	if err = s.canManageKelasMapel(c, k.ID, mp); err != nil {
		return err
	}
	var in reportGridInput
	if c.BodyParser(&in) != nil {
		return fiber.ErrBadRequest
	}
	if len(in.CPs) == 0 || len(in.CPs) > 100 {
		return fiber.NewError(400, "Isi 1–100 CP")
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	validStudents := map[string]bool{}
	for _, pd := range roster {
		validStudents[pd.ID] = true
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		// CP changes affect all reports. Score and narrative edits affect only their student.
		old := []ReportCP{}
		if e := tx.Where("period_id = ? AND mapel_id = ?", p.ID, mp).Order("urutan, id").Find(&old).Error; e != nil {
			return e
		}
		oldByID := map[string]ReportCP{}
		for _, cp := range old {
			oldByID[cp.ID] = cp
		}
		changed := len(old) != len(in.CPs)
		ids := []string{}
		seen := map[string]bool{}
		for i := range in.CPs {
			cp := &in.CPs[i]
			originalLabel := cp.Label
			cp.Label = strings.TrimSpace(cp.Label)
			if cp.Label == "" || len([]rune(cp.Label)) > 500 {
				return fiber.NewError(400, "Label CP wajib diisi (maksimal 500 karakter)")
			}
			if cp.ID != "" {
				prev, ok := oldByID[cp.ID]
				if !ok || seen[cp.ID] {
					return fiber.NewError(400, "ID CP tidak valid")
				}
				// An unchanged historical label must survive a score-only save.
				if prev.Label == originalLabel {
					cp.Label = prev.Label
				} else {
					cp.Label = reportLowercaseCPStart(cp.Label)
				}
				if prev.Label != cp.Label || prev.Urutan != i+1 {
					changed = true
				}
			}
			if cp.ID == "" {
				cp.Label = reportLowercaseCPStart(cp.Label)
				changed = true
			}
			seen[cp.ID] = true
			cp.PeriodID = p.ID
			cp.MapelID = mp
			cp.Urutan = i + 1
		}
		if changed {
			if e := reportUnlocked(tx, p.ID, ""); e != nil {
				return e
			}
		}
		for i := range in.CPs {
			cp := &in.CPs[i]
			if e := tx.Save(cp).Error; e != nil {
				return e
			}
			ids = append(ids, cp.ID)
		}
		for _, cp := range old {
			if !seen[cp.ID] {
				if e := tx.Where("cp_id = ?", cp.ID).Delete(&ReportScore{}).Error; e != nil {
					return e
				}
				if e := tx.Delete(&cp).Error; e != nil {
					return e
				}
			}
		}
		validCP := map[string]bool{}
		for _, id := range ids {
			validCP[id] = true
		}
		seenScores := map[string]bool{}
		for _, v := range in.Scores {
			key := v.StudentID + "/" + v.CPID
			if !validCP[v.CPID] || !validStudents[v.StudentID] || seenScores[key] || v.Value != nil && (math.IsNaN(*v.Value) || math.IsInf(*v.Value, 0) || *v.Value < 0 || *v.Value > 100) {
				return fiber.NewError(400, "Nilai, CP, atau siswa tidak valid")
			}
			seenScores[key] = true
			var oldScore ReportScore
			e := tx.Where("cp_id = ? AND student_id = ?", v.CPID, v.StudentID).First(&oldScore).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			same := oldScore.Value == nil && v.Value == nil || oldScore.Value != nil && v.Value != nil && *oldScore.Value == *v.Value
			if same {
				continue
			}
			if e := reportUnlocked(tx, p.ID, v.StudentID); e != nil {
				return e
			}
			oldScore.CPID = v.CPID
			oldScore.StudentID = v.StudentID
			oldScore.Value = v.Value
			if e := tx.Save(&oldScore).Error; e != nil {
				return e
			}
			if e := tx.Model(&ReportNarrative{}).Where("period_id = ? AND mapel_id = ? AND student_id = ?", p.ID, mp, v.StudentID).Update("reviewed_revision", -1).Error; e != nil {
				return e
			}
		}
		for _, n := range in.Narratives {
			if !validStudents[n.StudentID] || len([]rune(n.Strength)) > 2000 || len([]rune(n.Growth)) > 2000 {
				return fiber.NewError(400, "Deskripsi tidak valid")
			}
			if e := reportUnlocked(tx, p.ID, n.StudentID); e != nil {
				return e
			}
			var existing ReportNarrative
			e := tx.Where("period_id = ? AND student_id = ? AND mapel_id = ?", p.ID, n.StudentID, mp).First(&existing).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			existing.PeriodID = p.ID
			existing.StudentID = n.StudentID
			existing.MapelID = mp
			existing.Strength = strings.TrimSpace(n.Strength)
			existing.Growth = strings.TrimSpace(n.Growth)
			existing.ReviewedRevision = p.Revision
			if e := tx.Save(&existing).Error; e != nil {
				return e
			}
		}
		if changed {
			return tx.Model(&ReportNarrative{}).Where("period_id = ? AND mapel_id = ?", p.ID, mp).Update("reviewed_revision", -1).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "nilai", p.ID)
	return c.JSON(fiber.Map{"period": p, "cps": in.CPs})
}

func (s *Server) reportAttendance(db *gorm.DB, k Kelas, p ReportPeriod, studentID string) (ReportAttendance, error) {
	result := ReportAttendance{Conflicts: []string{}}
	type attendance struct {
		Tanggal         time.Time
		StatusKehadiran string
	}
	rows := []attendance{}
	err := db.Table("presensi_details").Select("presensis.tanggal, presensi_details.status_kehadiran").Joins("JOIN presensis ON presensis.id = presensi_details.presensi_id").Where("presensi_details.peserta_didik_id = ? AND presensis.kelas_id = ? AND presensis.semester = ? AND presensis.status_pertemuan <> ?", studentID, k.ID, p.Semester, "Batal").Scan(&rows).Error
	if err != nil {
		return result, err
	}
	byDay := map[string]string{}
	for _, r := range rows {
		day := r.Tanggal.Format("2006-01-02")
		if prev, ok := byDay[day]; ok && prev != r.StatusKehadiran {
			result.Conflicts = append(result.Conflicts, day)
		}
		byDay[day] = r.StatusKehadiran
	}
	for _, v := range byDay {
		switch v {
		case "Sakit":
			result.Sakit++
		case "Izin":
			result.Izin++
		case "Alpa", "Alfa":
			result.Alpa++
		}
	}
	return result, nil
}
func (s *Server) buildReport(db *gorm.DB, k Kelas, p ReportPeriod, studentID string) (ReportDocument, ReportRecord, error) {
	var d ReportDocument
	var rec ReportRecord
	roster, err := s.reportRoster(db, k)
	if err != nil {
		return d, rec, err
	}
	found := false
	for _, pd := range roster {
		if pd.ID == studentID {
			d.Student = pd
			found = true
			break
		}
	}
	if !found {
		return d, rec, fiber.NewError(404, "Siswa tidak terdaftar pada kelas dan periode ini")
	}
	rec.PeriodID = p.ID
	rec.StudentID = studentID
	rec.Status = "draft"
	if p.ID != "" {
		err = db.Where("period_id = ? AND student_id = ?", p.ID, studentID).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return d, rec, err
		}
	}
	if (rec.Status == "final" || rec.Status == "published") && rec.Snapshot != "" {
		if err = json.Unmarshal([]byte(rec.Snapshot), &d); err != nil {
			return d, rec, err
		}
		return d, rec, nil
	}
	d.Class = k
	d.Scope = p.ReportScope
	d.Issues = []string{}
	d.Subjects = []ReportSubject{}
	d.Config, err = s.reportConfig(db, k, p)
	if err != nil {
		return d, rec, err
	}
	reportDecode(rec.Details, &d.Details)
	if d.Details.Identity == nil {
		d.Details.Identity = map[string]string{}
	}
	if d.Student.OrangTuaID != "" {
		var parent OrangTua
		err = db.First(&parent, "id = ?", d.Student.OrangTuaID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return d, rec, err
		}
		if d.Details.Identity["father"] == "" {
			d.Details.Identity["father"] = parent.NamaBapak
		}
		if d.Details.Identity["mother"] == "" {
			d.Details.Identity["mother"] = parent.NamaIbu
		}
	}
	if len(d.Config.Subjects) == 0 {
		d.Issues = append(d.Issues, "Belum ada mapel kelas")
	}
	for _, mp := range d.Config.Subjects {
		cps := []ReportCP{}
		scores := []ReportScore{}
		var narrative ReportNarrative
		if p.ID != "" {
			if err = db.Where("period_id = ? AND mapel_id = ?", p.ID, mp.MapelID).Order("urutan, id").Find(&cps).Error; err != nil {
				return d, rec, err
			}
			ids := []string{}
			for _, cp := range cps {
				ids = append(ids, cp.ID)
			}
			if len(ids) > 0 {
				if err = db.Where("cp_id IN ? AND student_id = ?", ids, studentID).Find(&scores).Error; err != nil {
					return d, rec, err
				}
			}
			err = db.Where("period_id = ? AND student_id = ? AND mapel_id = ?", p.ID, studentID, mp.MapelID).First(&narrative).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return d, rec, err
			}
		}
		value, a, b, complete := reportCalculate(cps, scores)
		stale := false
		if narrative.ID != "" {
			a = narrative.Strength
			b = narrative.Growth
			stale = narrative.ReviewedRevision < 0
		}
		item := ReportSubject{ReportSubjectConfig: mp, Score: value, Strength: a, Growth: b, Stale: stale, Complete: complete, Number: len(d.Subjects) + 1}
		d.Subjects = append(d.Subjects, item)
		if !complete {
			d.Issues = append(d.Issues, mp.Name+": nilai CP belum lengkap")
		}
		if stale {
			d.Issues = append(d.Issues, mp.Name+": tinjau kembali deskripsi")
		}
		if a == "" {
			d.Issues = append(d.Issues, mp.Name+": deskripsi capaian kosong")
		}
	}
	d.Attendance, d.AttendanceSource, err = s.reportResolveAttendance(db, k, p, studentID, d.Config, d.Details)
	if err != nil {
		return d, rec, err
	}
	if d.AttendanceSource == "manual" && d.Details.ManualAttendance == nil {
		d.Issues = append(d.Issues, "Lengkapi rekap presensi manual: Sakit, Izin, dan Alpa (isi 0 jika nihil)")
	}
	if len(d.Attendance.Conflicts) > 0 {
		d.Issues = append(d.Issues, "Konflik presensi pada tanggal yang sama")
	}
	if d.Config.Date == "" || d.Config.Teacher == "" || p.Semester == "Genap" && d.Config.Principal == "" {
		d.Issues = append(d.Issues, "Lengkapi tanggal dan nama penandatangan")
	}
	if p.Semester == "Genap" {
		if !reportPromotionAllowed(k.Jenjang, d.Details.Promotion) {
			issue := "Lengkapi keputusan kenaikan kelas"
			if reportPromotionRules(k.Jenjang).Completion {
				issue = "Lengkapi keputusan kelulusan: Lulus atau Belum lulus"
			}
			d.Issues = append(d.Issues, issue)
		} else if d.Details.Promotion == "naik" && d.Details.NextClass == "" {
			// Response/snapshot projection only; the historical record stays intact.
			d.Details.NextClass = reportPromotionRules(k.Jenjang).NextClass
		}
	}
	if len([]rune(d.Details.Note)) > 450 {
		d.Issues = append(d.Issues, "Catatan wali terlalu panjang (maksimal 450 karakter)")
	}
	if !reportAreaFits(d.Details.Note, 105, 3) {
		d.Issues = append(d.Issues, "Catatan wali melebihi ruang cetak")
	}
	if reportExtrasEnabled(d.Config) {
		for _, extra := range d.Details.Extras {
			if strings.TrimSpace(extra.Name) == "" || strings.TrimSpace(extra.Description) == "" {
				d.Issues = append(d.Issues, "Lengkapi nama dan keterangan ekstrakurikuler")
				break
			}
		}
	}
	return d, rec, nil
}
func (s *Server) reportDocumentHandler(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	if err = s.scopeKelas(c, k.ID); err != nil {
		return err
	}
	d, rec, err := s.buildReport(s.db, k, p, c.Params("studentId"))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"document": d, "record": rec, "period": p})
}
func (s *Server) reportSaveDetails(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	if err = s.canManageKelas(c, k.ID); err != nil {
		return err
	}
	var in struct {
		Revision        int           `json:"revision"`
		Details         ReportDetails `json:"details"`
		UpdatePromotion bool          `json:"updatePromotion"`
	}
	if c.BodyParser(&in) != nil {
		return fiber.ErrBadRequest
	}
	if in.Details.Promotion != "" && in.Details.Promotion != "naik" && in.Details.Promotion != "tinggal" && in.Details.Promotion != "lulus" && in.Details.Promotion != "belum_lulus" {
		return fiber.ErrBadRequest
	}
	if in.Details.HistoricalAttendance != nil {
		return fiber.NewError(400, "Rekap historis hanya dapat dibuat melalui impor")
	}
	if len(in.Details.Extras) > 10 || len([]rune(in.Details.Note)) > 2000 {
		return fiber.ErrBadRequest
	}
	for key, value := range in.Details.Identity {
		if !reportIdentityKeys[key] || len([]rune(value)) > 800 {
			return fiber.NewError(400, "Identitas rapor tidak valid")
		}
	}
	if err := reportValidateExtras(in.Details.Extras); err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		document, rec, e := s.buildReport(tx, k, p, c.Params("studentId"))
		if e != nil {
			return e
		}
		if e = reportUnlocked(tx, p.ID, rec.StudentID); e != nil {
			return e
		}
		var prev ReportDetails
		reportDecode(rec.Details, &prev)
		if e = reportPreparePromotion(k.Jenjang, p.Semester, prev, &in.Details, in.UpdatePromotion); e != nil {
			return e
		}
		if !reportExtrasEnabled(document.Config) && !reportExtrasEqual(prev.Extras, in.Details.Extras) {
			return fiber.NewError(400, "Aktifkan ekstrakurikuler pada pengaturan kelas sebelum mengubah kegiatannya")
		}
		in.Details.HistoricalAttendance = prev.HistoricalAttendance
		// A note/identity save, including from older clients, must not erase the recap.
		in.Details.ManualAttendance = prev.ManualAttendance
		rec.Details = reportJSON(in.Details)
		rec.Status = "draft"
		return tx.Save(&rec).Error
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "catatan", p.ID)
	return c.JSON(p)
}
func (s *Server) reportSetStatus(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	if err = s.canManageKelas(c, k.ID); err != nil {
		return err
	}
	var in struct {
		Revision int    `json:"revision"`
		Status   string `json:"status"`
		Reason   string `json:"reason"`
	}
	if c.BodyParser(&in) != nil {
		return fiber.ErrBadRequest
	}
	if len(in.Reason) > 1000 {
		return fiber.NewError(400, "Alasan revisi terlalu panjang")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		d, rec, e := s.buildReport(tx, k, p, c.Params("studentId"))
		if e != nil {
			return e
		}
		switch in.Status {
		case "ready":
			if rec.Status != "draft" && rec.Status != "ready" {
				return fiber.ErrConflict
			}
		case "final":
			if rec.Status != "ready" {
				return fiber.NewError(409, "Tandai siap diperiksa terlebih dahulu")
			}
			if len(d.Issues) > 0 {
				return fiber.NewError(409, strings.Join(d.Issues, "; "))
			}
			rec.Version++
			rec.Snapshot = reportJSON(d)
		case "published":
			if rec.Status != "final" {
				return fiber.NewError(409, "Finalkan rapor terlebih dahulu")
			}
		case "draft":
			if (rec.Status == "final" || rec.Status == "published") && strings.TrimSpace(in.Reason) == "" {
				return fiber.NewError(400, "Alasan revisi wajib diisi")
			}
			rec.Snapshot = ""
		default:
			return fiber.ErrBadRequest
		}
		rec.Status = in.Status
		if e := tx.Save(&rec).Error; e != nil {
			return e
		}
		if in.Status == "final" {
			return tx.Create(&ReportRevision{RecordID: rec.ID, Version: rec.Version, Snapshot: rec.Snapshot}).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "status:"+in.Status+":"+in.Reason, p.ID+"/"+c.Params("studentId"))
	return c.JSON(p)
}

func (s *Server) reportParent(c *fiber.Ctx) error {
	if _, err := s.verifyOrangTuaAnak(c, c.Params("id")); err != nil {
		return err
	}
	records := []ReportRecord{}
	if err := s.db.Where("student_id = ? AND status = ?", c.Params("id"), "published").Order("created_at desc").Find(&records).Error; err != nil {
		return err
	}
	out := []fiber.Map{}
	for _, rec := range records {
		var d ReportDocument
		if err := json.Unmarshal([]byte(rec.Snapshot), &d); err != nil {
			return err
		}
		out = append(out, fiber.Map{"id": rec.ID, "version": rec.Version, "document": d})
	}
	return c.JSON(out)
}
func (s *Server) reportParentPrint(c *fiber.Ctx) error {
	if _, err := s.verifyOrangTuaAnak(c, c.Params("id")); err != nil {
		return err
	}
	var rec ReportRecord
	if err := s.db.Where("id = ? AND student_id = ? AND status = ?", c.Params("reportId"), c.Params("id"), "published").First(&rec).Error; err != nil {
		return fiber.ErrNotFound
	}
	var d ReportDocument
	if err := json.Unmarshal([]byte(rec.Snapshot), &d); err != nil {
		return err
	}
	return reportSendPDF(c, []ReportDocument{d}, "rapor")
}

func reportRoman(n int) string {
	v := []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}
	if n >= 1 && n <= 12 {
		return v[n]
	}
	return fmt.Sprint(n)
}
