package main

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	statusPublikasiDraf           = "draf"
	statusPublikasiDipublikasikan = "dipublikasikan"
)

func journalStaff(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" && role != "kepala_sekolah" && role != "guru" {
		return fiber.NewError(fiber.StatusForbidden, "akses jurnal hanya untuk admin, kepala sekolah, atau tutor")
	}
	return nil
}

func (s *Server) journalVisibleClassIDs(c *fiber.Ctx) ([]string, bool, error) {
	if err := journalStaff(c); err != nil {
		return nil, false, err
	}
	if c.Locals("role") != "guru" {
		return nil, true, nil
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		return nil, false, fiber.NewError(fiber.StatusForbidden, "profil tutor tidak ditemukan")
	}
	var assignments []PenugasanGuruMapel
	if err := s.db.Select("kelas_id").Where("tutor_id = ?", *user.TutorID).Find(&assignments).Error; err != nil {
		return nil, false, err
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.KelasID != "" && !seen[assignment.KelasID] {
			seen[assignment.KelasID] = true
			ids = append(ids, assignment.KelasID)
		}
	}
	return ids, false, nil
}

// journalTutorScope applies the coupled kelas+mapel assignment rather than
// independent class and subject filters. This matters when a tutor is assigned
// different subjects in different classes.
func (s *Server) journalTutorScope(c *fiber.Ctx, q *gorm.DB) (*gorm.DB, error) {
	if c.Locals("role") != "guru" {
		return q, nil
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		return nil, fiber.NewError(fiber.StatusForbidden, "profil tutor tidak ditemukan")
	}
	var assignments []PenugasanGuruMapel
	if err := s.db.Where("tutor_id = ?", *user.TutorID).Find(&assignments).Error; err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return q.Where("1 = 0"), nil
	}
	clauses := make([]string, 0, len(assignments))
	args := make([]any, 0, len(assignments)*2)
	for _, assignment := range assignments {
		clauses = append(clauses, "(kelas_id = ? AND mapel_id = ?)")
		args = append(args, assignment.KelasID, assignment.MapelID)
	}
	return q.Where("("+strings.Join(clauses, " OR ")+")", args...).Where("tutor_id = ?", *user.TutorID), nil
}

func validPublicationStatus(value string) bool {
	return value == statusPublikasiDraf || value == statusPublikasiDipublikasikan
}

// activeJournalRows keeps cancelled batches in the audit trail without letting
// them satisfy compliance, reminders, or ordinary journal views. Legacy rows
// without a batch remain valid historical entries.
func activeJournalRows(q *gorm.DB) *gorm.DB {
	return q.Where("batch_id IS NULL OR batch_id = '' OR batch_id IN (SELECT id FROM jurnal_batches WHERE dibatalkan_at IS NULL)")
}

func validMasteryStatus(value string) bool {
	return value == "tuntas" || value == "penguatan" || value == "remedial"
}

func parseOptionalWIBDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseWIBDateTime(value)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "tanggal harus berformat YYYY-MM-DD")
	}
	return &parsed, nil
}

func sameWIBDay(a, b time.Time) bool {
	return wibTimeFormat(a, "2006-01-02") == wibTimeFormat(b, "2006-01-02")
}

// validateJournalLineReferences keeps every selected related record in the
// same class/mapel context as its journal line. This avoids a journal becoming
// a cross-class data bridge merely by posting arbitrary IDs.
func (s *Server) validateJournalLineReferences(tx *gorm.DB, kelas Kelas, line journalLineInput) error {
	if line.RPPID != "" {
		var rpp RPP
		if err := tx.First(&rpp, "id = ?", line.RPPID).Error; err != nil || rpp.MapelID != line.MapelID || rpp.Jenjang != kelas.Jenjang {
			return fiber.NewError(fiber.StatusBadRequest, "RPP harus sesuai dengan mapel dan jenjang jurnal")
		}
	}
	if line.ModulID != "" {
		var modul ModulBelajar
		if err := tx.First(&modul, "id = ?", line.ModulID).Error; err != nil || modul.MapelID != line.MapelID {
			return fiber.NewError(fiber.StatusBadRequest, "modul harus sesuai dengan mapel jurnal")
		}
	}
	if line.MateriID != "" {
		var materi Materi
		if err := tx.First(&materi, "id = ?", line.MateriID).Error; err != nil || materi.KelasID != kelas.ID || materi.MapelID != line.MapelID {
			return fiber.NewError(fiber.StatusBadRequest, "materi harus sesuai dengan kelas dan mapel jurnal")
		}
	}
	if line.TugasID != "" {
		var tugas Tugas
		if err := tx.First(&tugas, "id = ?", line.TugasID).Error; err != nil || tugas.KelasID != kelas.ID || tugas.MapelID != line.MapelID {
			return fiber.NewError(fiber.StatusBadRequest, "tugas harus sesuai dengan kelas dan mapel jurnal")
		}
	}
	if line.KompetensiID != "" {
		var kompetensi Kompetensi
		if err := tx.First(&kompetensi, "id = ?", line.KompetensiID).Error; err != nil || kompetensi.MapelID != line.MapelID {
			return fiber.NewError(fiber.StatusBadRequest, "kompetensi harus sesuai dengan mapel jurnal")
		}
	}
	if line.KelasVirtualID != "" {
		var virtual KelasVirtual
		if err := tx.First(&virtual, "id = ?", line.KelasVirtualID).Error; err != nil || virtual.KelasID != kelas.ID || virtual.MapelID != line.MapelID {
			return fiber.NewError(fiber.StatusBadRequest, "kelas virtual harus sesuai dengan kelas dan mapel jurnal")
		}
	}
	return nil
}

type journalReferenceOptions struct {
	RPP          []RPP          `json:"rpp"`
	Modul        []ModulBelajar `json:"modul"`
	Materi       []Materi       `json:"materi"`
	Tugas        []Tugas        `json:"tugas"`
	Kompetensi   []Kompetensi   `json:"kompetensi"`
	KelasVirtual []KelasVirtual `json:"kelasVirtual"`
}

func (s *Server) journalReferences(c *fiber.Ctx) error {
	if err := journalStaff(c); err != nil {
		return err
	}
	kelasID := strings.TrimSpace(c.Query("kelasId"))
	mapelID := strings.TrimSpace(c.Query("mapelId"))
	if kelasID == "" || mapelID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "kelasId dan mapelId wajib diisi")
	}
	if allowed, err := s.tutorCanViewJournalClass(c, kelasID); err != nil || !allowed {
		if err != nil {
			return err
		}
		return fiber.NewError(fiber.StatusForbidden, "tidak ditugaskan pada kelas ini")
	}
	if c.Locals("role") == "guru" {
		var user User
		var assignment PenugasanGuruMapel
		if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil || s.db.Where("tutor_id = ? AND kelas_id = ? AND mapel_id = ?", *user.TutorID, kelasID, mapelID).First(&assignment).Error != nil {
			return fiber.NewError(fiber.StatusForbidden, "tidak ditugaskan pada mapel ini")
		}
	}
	var kelas Kelas
	if err := s.db.First(&kelas, "id = ?", kelasID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "kelas tidak ditemukan")
	}
	result := journalReferenceOptions{RPP: []RPP{}, Modul: []ModulBelajar{}, Materi: []Materi{}, Tugas: []Tugas{}, Kompetensi: []Kompetensi{}, KelasVirtual: []KelasVirtual{}}
	if err := s.db.Where("mapel_id = ? AND jenjang = ?", mapelID, kelas.Jenjang).Order("tanggal desc, created_at desc").Find(&result.RPP).Error; err != nil {
		return err
	}
	if err := s.db.Where("mapel_id = ?", mapelID).Order("urutan, created_at desc").Find(&result.Modul).Error; err != nil {
		return err
	}
	if err := s.db.Where("kelas_id = ? AND mapel_id = ?", kelasID, mapelID).Order("urutan, created_at desc").Find(&result.Materi).Error; err != nil {
		return err
	}
	if err := s.db.Where("kelas_id = ? AND mapel_id = ?", kelasID, mapelID).Order("deadline desc").Find(&result.Tugas).Error; err != nil {
		return err
	}
	if err := s.db.Where("mapel_id = ?", mapelID).Order("nama").Find(&result.Kompetensi).Error; err != nil {
		return err
	}
	if err := s.db.Where("kelas_id = ? AND mapel_id = ?", kelasID, mapelID).Order("waktu_mulai desc").Find(&result.KelasVirtual).Error; err != nil {
		return err
	}
	return c.JSON(result)
}

func (s *Server) journalBatchLock(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "hanya admin yang dapat mengunci jurnal")
	}
	var batch JurnalBatch
	if err := s.db.First(&batch, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "jurnal tidak ditemukan")
	}
	if batch.DibatalkanAt != nil {
		return fiber.NewError(fiber.StatusBadRequest, "jurnal yang dibatalkan tidak dapat dikunci")
	}
	if batch.TerkunciAt == nil {
		now, uid := time.Now(), c.Locals("userID").(string)
		batch.TerkunciAt, batch.TerkunciOlehUserID = &now, &uid
		if err := s.db.Save(&batch).Error; err != nil {
			return err
		}
		s.audit(&uid, "lock", "jurnal_batch", batch.ID)
	}
	return c.JSON(batch)
}

func (s *Server) journalBatchUnlock(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "hanya admin yang dapat membuka kunci jurnal")
	}
	var input struct {
		Alasan string `json:"alasan"`
	}
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "data pembukaan kunci tidak valid")
	}
	if strings.TrimSpace(input.Alasan) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "alasan membuka kunci wajib diisi")
	}
	var batch JurnalBatch
	if err := s.db.First(&batch, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "jurnal tidak ditemukan")
	}
	batch.TerkunciAt, batch.TerkunciOlehUserID = nil, nil
	if err := s.db.Save(&batch).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "unlock: "+strings.TrimSpace(input.Alasan), "jurnal_batch", batch.ID)
	return c.JSON(batch)
}

func (s *Server) cancelJournalBatch(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "hanya admin yang dapat membatalkan jurnal")
	}
	var input struct {
		Alasan string `json:"alasan"`
	}
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "data pembatalan tidak valid")
	}
	if strings.TrimSpace(input.Alasan) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "alasan pembatalan wajib diisi")
	}
	var batch JurnalBatch
	if err := s.db.First(&batch, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "jurnal tidak ditemukan")
	}
	if batch.DibatalkanAt == nil {
		now, uid := time.Now(), c.Locals("userID").(string)
		batch.DibatalkanAt, batch.DibatalkanOlehUserID, batch.AlasanPembatalan = &now, &uid, strings.TrimSpace(input.Alasan)
		if err := s.db.Save(&batch).Error; err != nil {
			return err
		}
		s.audit(&uid, "cancel: "+batch.AlasanPembatalan, "jurnal_batch", batch.ID)
	}
	return c.JSON(batch)
}

func (s *Server) portfolioClassAccess(c *fiber.Ctx, studentID string) (PesertaDidik, error) {
	if err := journalStaff(c); err != nil {
		return PesertaDidik{}, err
	}
	var student PesertaDidik
	if err := s.db.First(&student, "id = ?", studentID).Error; err != nil {
		return student, fiber.NewError(fiber.StatusNotFound, "peserta didik tidak ditemukan")
	}
	if allowed, err := s.tutorCanViewJournalClass(c, student.KelasID); err != nil || !allowed {
		if err != nil {
			return student, err
		}
		return student, fiber.NewError(fiber.StatusForbidden, "tidak diizinkan mengakses portofolio anak ini")
	}
	return student, nil
}

// portfolioTutorScope narrows portfolio reads to the exact kelas+mapel pairs
// assigned to a tutor. A class-level check alone would let one subject tutor
// read another tutor's evidence for the same children.
func (s *Server) portfolioTutorScope(c *fiber.Ctx, q *gorm.DB) (*gorm.DB, error) {
	if c.Locals("role") != "guru" {
		return q, nil
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		return nil, fiber.NewError(fiber.StatusForbidden, "profil tutor tidak ditemukan")
	}
	var assignments []PenugasanGuruMapel
	if err := s.db.Where("tutor_id = ?", *user.TutorID).Find(&assignments).Error; err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return q.Where("1 = 0"), nil
	}
	clauses := make([]string, 0, len(assignments))
	args := make([]any, 0, len(assignments)*2)
	for _, assignment := range assignments {
		clauses = append(clauses, "(mapel_id = ? AND peserta_didik_id IN (?))")
		args = append(args, assignment.MapelID, s.db.Model(&PesertaDidik{}).Select("id").Where("kelas_id = ?", assignment.KelasID))
	}
	return q.Where("("+strings.Join(clauses, " OR ")+")", args...), nil
}

func (s *Server) listPortofolio(c *fiber.Ctx) error {
	classIDs, all, err := s.journalVisibleClassIDs(c)
	if err != nil {
		return err
	}
	q := s.db.Preload("PesertaDidik").Preload("Mapel").Preload("Kompetensi").Order("created_at desc")
	if studentID := strings.TrimSpace(c.Query("pesertaDidikId")); studentID != "" {
		student, err := s.portfolioClassAccess(c, studentID)
		if err != nil {
			return err
		}
		q = q.Where("peserta_didik_id = ?", student.ID)
	} else if classID := strings.TrimSpace(c.Query("kelasId")); classID != "" {
		if !all {
			allowed := false
			for _, visible := range classIDs {
				if visible == classID {
					allowed = true
					break
				}
			}
			if !allowed {
				return fiber.NewError(fiber.StatusForbidden, "tidak ditugaskan pada kelas ini")
			}
		}
		var studentIDs []string
		if err := s.db.Model(&PesertaDidik{}).Where("kelas_id = ?", classID).Pluck("id", &studentIDs).Error; err != nil {
			return err
		}
		if len(studentIDs) == 0 {
			return c.JSON([]PortofolioBelajar{})
		}
		q = q.Where("peserta_didik_id IN ?", studentIDs)
	} else if !all {
		if len(classIDs) == 0 {
			return c.JSON([]PortofolioBelajar{})
		}
		var studentIDs []string
		if err := s.db.Model(&PesertaDidik{}).Where("kelas_id IN ?", classIDs).Pluck("id", &studentIDs).Error; err != nil {
			return err
		}
		if len(studentIDs) == 0 {
			return c.JSON([]PortofolioBelajar{})
		}
		q = q.Where("peserta_didik_id IN ?", studentIDs)
	}
	q, err = s.portfolioTutorScope(c, q)
	if err != nil {
		return err
	}
	var rows []PortofolioBelajar
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}

func (s *Server) createPortofolio(c *fiber.Ctx) error {
	student, err := s.portfolioClassAccess(c, strings.TrimSpace(c.FormValue("pesertaDidikId")))
	if err != nil {
		return err
	}
	mapelID := strings.TrimSpace(c.FormValue("mapelId"))
	judul := strings.TrimSpace(c.FormValue("judul"))
	if mapelID == "" || judul == "" {
		return fiber.NewError(fiber.StatusBadRequest, "mapel dan judul portofolio wajib diisi")
	}
	if err := s.canManageKelasMapel(c, student.KelasID, mapelID); err != nil {
		return err
	}
	if err := s.db.First(&MataPelajaran{}, "id = ?", mapelID).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "mapel tidak ditemukan")
	}
	if kompetensiID := strings.TrimSpace(c.FormValue("kompetensiId")); kompetensiID != "" {
		var kompetensi Kompetensi
		if err := s.db.First(&kompetensi, "id = ?", kompetensiID).Error; err != nil || kompetensi.MapelID != mapelID {
			return fiber.NewError(fiber.StatusBadRequest, "kompetensi harus sesuai dengan mapel")
		}
	}
	journalID := strings.TrimSpace(c.FormValue("jurnalId"))
	if journalID != "" {
		var journal JurnalMengajar
		if err := s.db.First(&journal, "id = ? AND kelas_id = ? AND mapel_id = ?", journalID, student.KelasID, mapelID).Error; err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "jurnal harus sesuai dengan kelas dan mapel portofolio")
		}
	}
	path, err := s.saveUpload(c, "file", "portofolio", 25*1024*1024, []string{"pdf", "docx", "doc", "png", "jpg", "jpeg", "mp3", "m4a", "wav", "mp4"})
	if err != nil {
		return err
	}
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "berkas bukti belajar wajib diunggah")
	}
	fh, _ := c.FormFile("file")
	status := strings.TrimSpace(c.FormValue("statusPublikasi"))
	if status == "" {
		status = statusPublikasiDraf
	}
	if !validPublicationStatus(status) {
		removeUpload(path)
		return fiber.NewError(fiber.StatusBadRequest, "status publikasi tidak valid")
	}
	uid := c.Locals("userID").(string)
	portfolio := PortofolioBelajar{PesertaDidikID: student.ID, JurnalID: formPtr(journalID), MapelID: mapelID, KompetensiID: formPtr(strings.TrimSpace(c.FormValue("kompetensiId"))), Judul: judul, TipeBukti: strings.TrimSpace(c.FormValue("tipeBukti")), FilePath: path, KomentarTutor: strings.TrimSpace(c.FormValue("komentarTutor")), Rubrik: strings.TrimSpace(c.FormValue("rubrik")), StatusPublikasi: status, DibuatOlehUserID: uid}
	if fh != nil {
		portfolio.FileName = filepath.Base(fh.Filename)
	}
	if portfolio.TipeBukti == "" {
		portfolio.TipeBukti = strings.TrimPrefix(strings.ToLower(filepath.Ext(portfolio.FileName)), ".")
	}
	if status == statusPublikasiDipublikasikan {
		now := time.Now()
		portfolio.DipublikasikanAt, portfolio.DipublikasikanOlehUserID = &now, &uid
	}
	if err := s.db.Create(&portfolio).Error; err != nil {
		removeUpload(path)
		return err
	}
	s.audit(&uid, "create", "portofolio_belajar", portfolio.ID)
	return c.Status(fiber.StatusCreated).JSON(portfolio)
}

func (s *Server) updatePortofolio(c *fiber.Ctx) error {
	var portfolio PortofolioBelajar
	if err := s.db.First(&portfolio, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "portofolio tidak ditemukan")
	}
	student, err := s.portfolioClassAccess(c, portfolio.PesertaDidikID)
	if err != nil {
		return err
	}
	if err := s.canManageKelasMapel(c, student.KelasID, portfolio.MapelID); err != nil {
		return err
	}
	if title := strings.TrimSpace(c.FormValue("judul")); title != "" {
		portfolio.Judul = title
	}
	portfolio.KomentarTutor = strings.TrimSpace(c.FormValue("komentarTutor"))
	portfolio.Rubrik = strings.TrimSpace(c.FormValue("rubrik"))
	if status := strings.TrimSpace(c.FormValue("statusPublikasi")); status != "" {
		if !validPublicationStatus(status) {
			return fiber.NewError(fiber.StatusBadRequest, "status publikasi tidak valid")
		}
		portfolio.StatusPublikasi = status
		if status == statusPublikasiDipublikasikan && portfolio.DipublikasikanAt == nil {
			now, uid := time.Now(), c.Locals("userID").(string)
			portfolio.DipublikasikanAt, portfolio.DipublikasikanOlehUserID = &now, &uid
		}
		if status == statusPublikasiDraf {
			portfolio.DipublikasikanAt, portfolio.DipublikasikanOlehUserID = nil, nil
		}
	}
	if err := s.db.Save(&portfolio).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "portofolio_belajar", portfolio.ID)
	return c.JSON(portfolio)
}

func (s *Server) deletePortofolio(c *fiber.Ctx) error {
	var portfolio PortofolioBelajar
	if err := s.db.First(&portfolio, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "portofolio tidak ditemukan")
	}
	student, err := s.portfolioClassAccess(c, portfolio.PesertaDidikID)
	if err != nil {
		return err
	}
	if err := s.canManageKelasMapel(c, student.KelasID, portfolio.MapelID); err != nil {
		return err
	}
	if err := s.db.Delete(&portfolio).Error; err != nil {
		return err
	}
	removeUpload(portfolio.FilePath)
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "portofolio_belajar", portfolio.ID)
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) downloadPortofolio(c *fiber.Ctx) error {
	if err := journalStaff(c); err != nil {
		return err
	}
	var portfolio PortofolioBelajar
	if err := s.db.First(&portfolio, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "portofolio tidak ditemukan")
	}
	student, err := s.portfolioClassAccess(c, portfolio.PesertaDidikID)
	if err != nil {
		return err
	}
	if c.Locals("role") == "guru" {
		if err := s.canManageKelasMapel(c, student.KelasID, portfolio.MapelID); err != nil {
			return err
		}
	}
	c.Set(fiber.HeaderContentDisposition, "attachment; filename="+strconvQuote(filepath.Base(portfolio.FileName)))
	return s.sendUpload(c, portfolio.FilePath)
}

func strconvQuote(value string) string { return `"` + strings.ReplaceAll(value, `"`, "'") + `"` }

type tindakLanjutInput struct {
	PesertaDidikID    string  `json:"pesertaDidikId"`
	JurnalID          *string `json:"jurnalId"`
	MapelID           string  `json:"mapelId"`
	KompetensiID      *string `json:"kompetensiId"`
	StatusKetuntasan  string  `json:"statusKetuntasan"`
	Rencana           string  `json:"rencana"`
	PenanggungJawab   string  `json:"penanggungJawab"`
	Tenggat           string  `json:"tenggat"`
	Hasil             string  `json:"hasil"`
	SelesaiAt         string  `json:"selesaiAt"`
	RingkasanOrangTua string  `json:"ringkasanOrangTua"`
	StatusPublikasi   string  `json:"statusPublikasi"`
}

func parseTindakLanjutInput(c *fiber.Ctx) (tindakLanjutInput, error) {
	var input tindakLanjutInput
	if err := c.BodyParser(&input); err != nil {
		return input, fiber.NewError(fiber.StatusBadRequest, "data tindak lanjut tidak valid")
	}
	input.PesertaDidikID, input.MapelID, input.StatusKetuntasan, input.Rencana = strings.TrimSpace(input.PesertaDidikID), strings.TrimSpace(input.MapelID), strings.TrimSpace(input.StatusKetuntasan), strings.TrimSpace(input.Rencana)
	if !validMasteryStatus(input.StatusKetuntasan) || input.Rencana == "" {
		return input, fiber.NewError(fiber.StatusBadRequest, "status ketuntasan dan rencana wajib diisi")
	}
	if input.StatusPublikasi == "" {
		input.StatusPublikasi = statusPublikasiDraf
	}
	if !validPublicationStatus(input.StatusPublikasi) {
		return input, fiber.NewError(fiber.StatusBadRequest, "status publikasi tidak valid")
	}
	return input, nil
}

// resolveTindakLanjutMapel keeps each follow-up in one mapel context. Mapel
// can be supplied directly or inferred from its journal/competency, but those
// records must always agree.
func (s *Server) resolveTindakLanjutMapel(input tindakLanjutInput) (string, error) {
	mapelID := input.MapelID
	if mapelID != "" {
		if err := s.db.First(&MataPelajaran{}, "id = ?", mapelID).Error; err != nil {
			return "", fiber.NewError(fiber.StatusBadRequest, "mapel tidak ditemukan")
		}
	}
	if input.KompetensiID != nil && strings.TrimSpace(*input.KompetensiID) != "" {
		var kompetensi Kompetensi
		if err := s.db.First(&kompetensi, "id = ?", strings.TrimSpace(*input.KompetensiID)).Error; err != nil {
			return "", fiber.NewError(fiber.StatusBadRequest, "kompetensi tidak ditemukan")
		}
		if mapelID != "" && mapelID != kompetensi.MapelID {
			return "", fiber.NewError(fiber.StatusBadRequest, "kompetensi harus sesuai dengan mapel tindak lanjut")
		}
		mapelID = kompetensi.MapelID
	}
	if input.JurnalID != nil && strings.TrimSpace(*input.JurnalID) != "" {
		var journal JurnalMengajar
		if err := s.db.First(&journal, "id = ?", strings.TrimSpace(*input.JurnalID)).Error; err != nil {
			return "", fiber.NewError(fiber.StatusBadRequest, "jurnal tidak ditemukan")
		}
		if mapelID != "" && mapelID != journal.MapelID {
			return "", fiber.NewError(fiber.StatusBadRequest, "jurnal harus sesuai dengan mapel tindak lanjut")
		}
		mapelID = journal.MapelID
	}
	if mapelID == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "mapel, jurnal, atau kompetensi wajib diisi")
	}
	return mapelID, nil
}

func (s *Server) validateTindakLanjut(c *fiber.Ctx, input tindakLanjutInput) (PesertaDidik, *time.Time, *time.Time, error) {
	student, err := s.portfolioClassAccess(c, input.PesertaDidikID)
	if err != nil {
		return student, nil, nil, err
	}
	mapelID, err := s.resolveTindakLanjutMapel(input)
	if err != nil {
		return student, nil, nil, err
	}
	if err := s.canManageKelasMapel(c, student.KelasID, mapelID); err != nil {
		return student, nil, nil, err
	}
	if input.JurnalID != nil && strings.TrimSpace(*input.JurnalID) != "" {
		var journal JurnalMengajar
		if err := s.db.First(&journal, "id = ? AND kelas_id = ?", strings.TrimSpace(*input.JurnalID), student.KelasID).Error; err != nil {
			return student, nil, nil, fiber.NewError(fiber.StatusBadRequest, "jurnal tidak sesuai dengan kelas anak")
		}
	}
	tenggat, err := parseOptionalWIBDate(input.Tenggat)
	if err != nil {
		return student, nil, nil, err
	}
	selesai, err := parseOptionalWIBDate(input.SelesaiAt)
	if err != nil {
		return student, nil, nil, err
	}
	return student, tenggat, selesai, nil
}

func (s *Server) listTindakLanjut(c *fiber.Ctx) error {
	classIDs, all, err := s.journalVisibleClassIDs(c)
	if err != nil {
		return err
	}
	q := s.db.Preload("PesertaDidik").Preload("Mapel").Preload("Kompetensi").Order("created_at desc")
	if studentID := strings.TrimSpace(c.Query("pesertaDidikId")); studentID != "" {
		if _, err := s.portfolioClassAccess(c, studentID); err != nil {
			return err
		}
		q = q.Where("peserta_didik_id = ?", studentID)
	} else if !all {
		if len(classIDs) == 0 {
			return c.JSON([]TindakLanjutBelajar{})
		}
		var students []string
		if err := s.db.Model(&PesertaDidik{}).Where("kelas_id IN ?", classIDs).Pluck("id", &students).Error; err != nil {
			return err
		}
		if len(students) == 0 {
			return c.JSON([]TindakLanjutBelajar{})
		}
		q = q.Where("peserta_didik_id IN ?", students)
	}
	if status := strings.TrimSpace(c.Query("statusKetuntasan")); status != "" {
		q = q.Where("status_ketuntasan = ?", status)
	}
	q, err = s.portfolioTutorScope(c, q)
	if err != nil {
		return err
	}
	var rows []TindakLanjutBelajar
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}

func (s *Server) createTindakLanjut(c *fiber.Ctx) error {
	if err := journalStaff(c); err != nil {
		return err
	}
	input, err := parseTindakLanjutInput(c)
	if err != nil {
		return err
	}
	_, tenggat, selesai, err := s.validateTindakLanjut(c, input)
	if err != nil {
		return err
	}
	mapelID, err := s.resolveTindakLanjutMapel(input)
	if err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	row := TindakLanjutBelajar{PesertaDidikID: input.PesertaDidikID, JurnalID: input.JurnalID, MapelID: mapelID, KompetensiID: input.KompetensiID, StatusKetuntasan: input.StatusKetuntasan, Rencana: input.Rencana, PenanggungJawab: strings.TrimSpace(input.PenanggungJawab), Tenggat: tenggat, Hasil: strings.TrimSpace(input.Hasil), SelesaiAt: selesai, RingkasanOrangTua: strings.TrimSpace(input.RingkasanOrangTua), StatusPublikasi: input.StatusPublikasi, DibuatOlehUserID: uid}
	if row.StatusPublikasi == statusPublikasiDipublikasikan {
		now := time.Now()
		row.DipublikasikanAt, row.DipublikasikanOlehUserID = &now, &uid
	}
	if err := s.db.Create(&row).Error; err != nil {
		return err
	}
	s.audit(&uid, "create", "tindak_lanjut_belajar", row.ID)
	return c.Status(fiber.StatusCreated).JSON(row)
}

func (s *Server) updateTindakLanjut(c *fiber.Ctx) error {
	if err := journalStaff(c); err != nil {
		return err
	}
	var existing TindakLanjutBelajar
	if err := s.db.First(&existing, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tindak lanjut tidak ditemukan")
	}
	input, err := parseTindakLanjutInput(c)
	if err != nil {
		return err
	}
	if input.PesertaDidikID == "" {
		input.PesertaDidikID = existing.PesertaDidikID
	}
	if input.JurnalID == nil {
		input.JurnalID = existing.JurnalID
	}
	if input.MapelID == "" {
		input.MapelID = existing.MapelID
	}
	if input.KompetensiID == nil {
		input.KompetensiID = existing.KompetensiID
	}
	_, tenggat, selesai, err := s.validateTindakLanjut(c, input)
	if err != nil {
		return err
	}
	mapelID, err := s.resolveTindakLanjutMapel(input)
	if err != nil {
		return err
	}
	existing.JurnalID, existing.MapelID, existing.KompetensiID, existing.StatusKetuntasan, existing.Rencana = input.JurnalID, mapelID, input.KompetensiID, input.StatusKetuntasan, input.Rencana
	existing.PenanggungJawab, existing.Tenggat, existing.Hasil, existing.SelesaiAt = strings.TrimSpace(input.PenanggungJawab), tenggat, strings.TrimSpace(input.Hasil), selesai
	existing.RingkasanOrangTua, existing.StatusPublikasi = strings.TrimSpace(input.RingkasanOrangTua), input.StatusPublikasi
	uid := c.Locals("userID").(string)
	if existing.StatusPublikasi == statusPublikasiDipublikasikan && existing.DipublikasikanAt == nil {
		now := time.Now()
		existing.DipublikasikanAt, existing.DipublikasikanOlehUserID = &now, &uid
	}
	if existing.StatusPublikasi == statusPublikasiDraf {
		existing.DipublikasikanAt, existing.DipublikasikanOlehUserID = nil, nil
	}
	if err := s.db.Save(&existing).Error; err != nil {
		return err
	}
	s.audit(&uid, "update", "tindak_lanjut_belajar", existing.ID)
	return c.JSON(existing)
}

type learningSupportStudent struct {
	PesertaDidikID string `json:"pesertaDidikId"`
	Nama           string `json:"nama"`
	KelasID        string `json:"kelasId"`
	JumlahAbsen    int    `json:"jumlahAbsen,omitempty"`
}

type journalSupportDashboard struct {
	Total            int                      `json:"total"`
	Items            []TindakLanjutBelajar    `json:"items"`
	TanpaPortofolio  []learningSupportStudent `json:"tanpaPortofolio"`
	SeringAbsen      []learningSupportStudent `json:"seringAbsen"`
	PeriodeKehadiran string                   `json:"periodeKehadiran"`
}

func (s *Server) journalSupportStudents(classIDs []string, all bool) ([]PesertaDidik, error) {
	q := s.db.Where("status = ?", "aktif").Order("nama")
	if !all {
		if len(classIDs) == 0 {
			return []PesertaDidik{}, nil
		}
		q = q.Where("kelas_id IN ?", classIDs)
	}
	var students []PesertaDidik
	if err := q.Find(&students).Error; err != nil {
		return nil, err
	}
	return students, nil
}

func (s *Server) journalFollowUpDashboard(c *fiber.Ctx) error {
	if err := journalStaff(c); err != nil {
		return err
	}
	classIDs, all, err := s.journalVisibleClassIDs(c)
	if err != nil {
		return err
	}
	q := s.db.Preload("PesertaDidik").Preload("Mapel").Preload("Kompetensi").Where("status_ketuntasan IN ?", []string{"penguatan", "remedial"}).Order("tenggat asc, created_at desc")
	if !all {
		if len(classIDs) == 0 {
			return c.JSON(journalSupportDashboard{Items: []TindakLanjutBelajar{}, TanpaPortofolio: []learningSupportStudent{}, SeringAbsen: []learningSupportStudent{}, PeriodeKehadiran: "30 hari terakhir"})
		}
		var students []string
		if err := s.db.Model(&PesertaDidik{}).Where("kelas_id IN ?", classIDs).Pluck("id", &students).Error; err != nil {
			return err
		}
		if len(students) == 0 {
			return c.JSON(journalSupportDashboard{Items: []TindakLanjutBelajar{}, TanpaPortofolio: []learningSupportStudent{}, SeringAbsen: []learningSupportStudent{}, PeriodeKehadiran: "30 hari terakhir"})
		}
		q = q.Where("peserta_didik_id IN ?", students)
	}
	q, err = s.portfolioTutorScope(c, q)
	if err != nil {
		return err
	}
	var items []TindakLanjutBelajar
	if err := q.Find(&items).Error; err != nil {
		return err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Tenggat == nil {
			return false
		}
		if items[j].Tenggat == nil {
			return true
		}
		return items[i].Tenggat.Before(*items[j].Tenggat)
	})
	students, err := s.journalSupportStudents(classIDs, all)
	if err != nil {
		return err
	}
	studentSet := make(map[string]PesertaDidik, len(students))
	studentIDs := make([]string, 0, len(students))
	for _, student := range students {
		studentSet[student.ID] = student
		studentIDs = append(studentIDs, student.ID)
	}
	withoutEvidence := make([]learningSupportStudent, 0)
	if len(studentSet) > 0 {
		var evidenceStudentIDs []string
		if err := s.db.Model(&PortofolioBelajar{}).Where("peserta_didik_id IN ?", studentIDs).Distinct().Pluck("peserta_didik_id", &evidenceStudentIDs).Error; err != nil {
			return err
		}
		hasEvidence := make(map[string]bool, len(evidenceStudentIDs))
		for _, studentID := range evidenceStudentIDs {
			hasEvidence[studentID] = true
		}
		for _, student := range students {
			if !hasEvidence[student.ID] {
				withoutEvidence = append(withoutEvidence, learningSupportStudent{PesertaDidikID: student.ID, Nama: student.Nama, KelasID: student.KelasID})
			}
		}
	}
	type absenceTotal struct {
		PesertaDidikID string
		Total          int
	}
	periodStart := journalDateAtMidnight(time.Now().AddDate(0, 0, -29))
	var absenceTotals []absenceTotal
	if len(studentSet) > 0 {
		if err := s.db.Table("presensi_details").Select("peserta_didik_id, COUNT(*) AS total").Joins("JOIN presensis ON presensis.id = presensi_details.presensi_id").Where("peserta_didik_id IN ? AND presensis.tanggal >= ? AND status_kehadiran NOT IN ?", studentIDs, periodStart, []string{"Hadir", ""}).Group("peserta_didik_id").Scan(&absenceTotals).Error; err != nil {
			return err
		}
	}
	frequentAbsence := make([]learningSupportStudent, 0)
	for _, absence := range absenceTotals {
		if absence.Total < 3 {
			continue
		}
		if student, ok := studentSet[absence.PesertaDidikID]; ok {
			frequentAbsence = append(frequentAbsence, learningSupportStudent{PesertaDidikID: student.ID, Nama: student.Nama, KelasID: student.KelasID, JumlahAbsen: absence.Total})
		}
	}
	needsSupport := map[string]bool{}
	for _, item := range items {
		needsSupport[item.PesertaDidikID] = true
	}
	for _, item := range withoutEvidence {
		needsSupport[item.PesertaDidikID] = true
	}
	for _, item := range frequentAbsence {
		needsSupport[item.PesertaDidikID] = true
	}
	return c.JSON(journalSupportDashboard{Total: len(needsSupport), Items: items, TanpaPortofolio: withoutEvidence, SeringAbsen: frequentAbsence, PeriodeKehadiran: "30 hari terakhir"})
}

type parentJournalItem struct {
	ID           string    `json:"id"`
	Tanggal      time.Time `json:"tanggal"`
	Mapel        string    `json:"mapel"`
	Tujuan       string    `json:"tujuan"`
	Materi       string    `json:"materi"`
	Kegiatan     string    `json:"kegiatan"`
	Media        string    `json:"media"`
	Ringkasan    string    `json:"ringkasan"`
	TindakLanjut string    `json:"tindakLanjut"`
	Kehadiran    string    `json:"kehadiran"`
	TugasID      *string   `json:"tugasId,omitempty"`
	TugasJudul   string    `json:"tugasJudul,omitempty"`
	StatusTugas  string    `json:"statusTugas,omitempty"`
}

func (s *Server) getJurnalAnak(c *fiber.Ctx) error {
	studentID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, studentID); err != nil {
		return err
	}
	var student PesertaDidik
	if err := s.db.First(&student, "id = ?", studentID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "peserta didik tidak ditemukan")
	}
	var journals []JurnalMengajar
	if err := s.db.Preload("Mapel").Preload("Batch").Where("kelas_id = ? AND status_publikasi = ?", student.KelasID, statusPublikasiDipublikasikan).Order("tanggal desc, jam_ke desc").Limit(100).Find(&journals).Error; err != nil {
		return err
	}
	result := make([]parentJournalItem, 0, len(journals))
	for _, journal := range journals {
		if journal.Batch != nil && journal.Batch.DibatalkanAt != nil {
			continue
		}
		item := parentJournalItem{ID: journal.ID, Tanggal: journal.Tanggal, Mapel: journal.Mapel.NamaMapel, Tujuan: journal.Tujuan, Materi: journal.Materi, Kegiatan: journal.Kegiatan, Media: journal.Media, Ringkasan: journal.RingkasanOrangTua, TindakLanjut: journal.TindakLanjut, Kehadiran: "Belum dicatat", TugasID: journal.TugasID}
		var attendance PresensiDetail
		if err := s.db.Joins("JOIN presensis ON presensis.id = presensi_details.presensi_id").Where("presensi_details.peserta_didik_id = ? AND presensis.kelas_id = ? AND presensis.tanggal = ?", student.ID, student.KelasID, journal.Tanggal).First(&attendance).Error; err == nil {
			item.Kehadiran = attendance.StatusKehadiran
		}
		if journal.TugasID != nil {
			var task Tugas
			if s.db.First(&task, "id = ?", *journal.TugasID).Error == nil {
				item.TugasJudul = task.Judul
				item.StatusTugas = "Belum dicatat"
				var submission PengumpulanTugas
				if s.db.Where("tugas_id = ? AND peserta_didik_id = ?", task.ID, student.ID).First(&submission).Error == nil {
					item.StatusTugas = submission.Status
				}
			}
		}
		result = append(result, item)
	}
	return c.JSON(result)
}

type parentPortfolioItem struct {
	ID            string    `json:"id"`
	Judul         string    `json:"judul"`
	TipeBukti     string    `json:"tipeBukti"`
	FileName      string    `json:"fileName"`
	KomentarTutor string    `json:"komentarTutor"`
	Rubrik        string    `json:"rubrik"`
	Mapel         string    `json:"mapel"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (s *Server) getPortofolioAnak(c *fiber.Ctx) error {
	studentID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, studentID); err != nil {
		return err
	}
	var rows []PortofolioBelajar
	if err := s.db.Preload("Mapel").Where("peserta_didik_id = ? AND status_publikasi = ?", studentID, statusPublikasiDipublikasikan).Order("created_at desc").Find(&rows).Error; err != nil {
		return err
	}
	result := make([]parentPortfolioItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, parentPortfolioItem{ID: row.ID, Judul: row.Judul, TipeBukti: row.TipeBukti, FileName: row.FileName, KomentarTutor: row.KomentarTutor, Rubrik: row.Rubrik, Mapel: row.Mapel.NamaMapel, CreatedAt: row.CreatedAt})
	}
	return c.JSON(result)
}

func (s *Server) downloadPortofolioAnak(c *fiber.Ctx) error {
	studentID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, studentID); err != nil {
		return err
	}
	var row PortofolioBelajar
	if err := s.db.Where("id = ? AND peserta_didik_id = ? AND status_publikasi = ?", c.Params("portofolioId"), studentID, statusPublikasiDipublikasikan).First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "portofolio tidak tersedia")
	}
	c.Set(fiber.HeaderContentDisposition, "attachment; filename="+strconvQuote(filepath.Base(row.FileName)))
	return s.sendUpload(c, row.FilePath)
}

type parentFollowUpItem struct {
	ID               string     `json:"id"`
	StatusKetuntasan string     `json:"statusKetuntasan"`
	Mapel            string     `json:"mapel"`
	Rencana          string     `json:"rencana"`
	PenanggungJawab  string     `json:"penanggungJawab"`
	Tenggat          *time.Time `json:"tenggat"`
	Hasil            string     `json:"hasil"`
	Ringkasan        string     `json:"ringkasan"`
	Kompetensi       string     `json:"kompetensi"`
}

func (s *Server) getTindakLanjutAnak(c *fiber.Ctx) error {
	studentID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, studentID); err != nil {
		return err
	}
	var rows []TindakLanjutBelajar
	if err := s.db.Preload("Mapel").Preload("Kompetensi").Where("peserta_didik_id = ? AND status_publikasi = ?", studentID, statusPublikasiDipublikasikan).Order("created_at desc").Find(&rows).Error; err != nil {
		return err
	}
	result := make([]parentFollowUpItem, 0, len(rows))
	for _, row := range rows {
		name := ""
		if row.Kompetensi != nil {
			name = row.Kompetensi.Nama
		}
		result = append(result, parentFollowUpItem{ID: row.ID, StatusKetuntasan: row.StatusKetuntasan, Mapel: row.Mapel.NamaMapel, Rencana: row.Rencana, PenanggungJawab: row.PenanggungJawab, Tenggat: row.Tenggat, Hasil: row.Hasil, Ringkasan: row.RingkasanOrangTua, Kompetensi: name})
	}
	return c.JSON(result)
}
