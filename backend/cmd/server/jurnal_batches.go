package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strconv"
	"strings"
	"time"

	"github.com/carmel/gooxml/common"
	"github.com/carmel/gooxml/document"
	"github.com/carmel/gooxml/measurement"
	"github.com/fogleman/gg"
	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"gorm.io/gorm"
)

// backfillJurnalBatches upgrades every pre-batch journal row once. It is safe
// to run at every startup: only rows without a BatchID are touched.
func (s *Server) backfillJurnalBatches() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var pending []JurnalMengajar
		if err := tx.Where("batch_id IS NULL OR batch_id = ''").Order("kelas_id, tanggal, created_at, id").Find(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		var existing []JurnalMengajar
		if err := tx.Select("kelas_id", "tanggal", "jam_ke").Find(&existing).Error; err != nil {
			return err
		}
		nextPeriod := map[string]int{}
		for _, row := range existing {
			key := row.KelasID + "|" + wibTimeFormat(row.Tanggal, "2006-01-02")
			if row.JamKe > nextPeriod[key] {
				nextPeriod[key] = row.JamKe
			}
		}
		for _, row := range pending {
			key := row.KelasID + "|" + wibTimeFormat(row.Tanggal, "2006-01-02")
			nextPeriod[key]++
			batch := JurnalBatch{
				TutorID: row.TutorID, KelasID: row.KelasID, Tanggal: row.Tanggal,
				FotoPath: row.FotoPath,
			}
			if err := tx.Create(&batch).Error; err != nil {
				return err
			}
			if err := tx.Model(&JurnalMengajar{}).Where("id = ?", row.ID).Updates(map[string]any{
				"batch_id": batch.ID, "jam_ke": nextPeriod[key],
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type journalLineInput struct {
	JamKe             int    `json:"jamKe"`
	MapelID           string `json:"mapelId"`
	Materi            string `json:"materi"`
	Kegiatan          string `json:"kegiatan"`
	Tujuan            string `json:"tujuan"`
	Metode            string `json:"metode"`
	Media             string `json:"media"`
	Keterlibatan      string `json:"keterlibatan"`
	Asesmen           string `json:"asesmen"`
	HasilAsesmen      string `json:"hasilAsesmen"`
	Kendala           string `json:"kendala"`
	Refleksi          string `json:"refleksi"`
	TindakLanjut      string `json:"tindakLanjut"`
	RingkasanOrangTua string `json:"ringkasanOrangTua"`
	RPPID             string `json:"rppId"`
	ModulID           string `json:"modulId"`
	MateriID          string `json:"materiId"`
	TugasID           string `json:"tugasId"`
	KompetensiID      string `json:"kompetensiId"`
	KelasVirtualID    string `json:"kelasVirtualId"`
	StatusPublikasi   string `json:"statusPublikasi"`
}

type journalAbsentStudent struct {
	PesertaDidikID  string `json:"pesertaDidikId"`
	Nama            string `json:"nama"`
	StatusKehadiran string `json:"statusKehadiran"`
}

type journalSheetLine struct {
	ID                string     `json:"id"`
	BatchID           string     `json:"batchId"`
	JamKe             int        `json:"jamKe"`
	TutorID           string     `json:"tutorId"`
	TutorNama         string     `json:"tutorNama"`
	MapelID           string     `json:"mapelId"`
	MapelNama         string     `json:"mapelNama"`
	Materi            string     `json:"materi"`
	Kegiatan          string     `json:"kegiatan,omitempty"`
	Tujuan            string     `json:"tujuan,omitempty"`
	Metode            string     `json:"metode,omitempty"`
	Media             string     `json:"media,omitempty"`
	Keterlibatan      string     `json:"keterlibatan,omitempty"`
	Asesmen           string     `json:"asesmen,omitempty"`
	HasilAsesmen      string     `json:"hasilAsesmen,omitempty"`
	Kendala           string     `json:"kendala,omitempty"`
	Refleksi          string     `json:"refleksi,omitempty"`
	TindakLanjut      string     `json:"tindakLanjut,omitempty"`
	RingkasanOrangTua string     `json:"ringkasanOrangTua,omitempty"`
	RPPID             *string    `json:"rppId,omitempty"`
	ModulID           *string    `json:"modulId,omitempty"`
	MateriID          *string    `json:"materiId,omitempty"`
	TugasID           *string    `json:"tugasId,omitempty"`
	KompetensiID      *string    `json:"kompetensiId,omitempty"`
	KelasVirtualID    *string    `json:"kelasVirtualId,omitempty"`
	StatusPublikasi   string     `json:"statusPublikasi"`
	TandaTangan       string     `json:"tandaTangan,omitempty"`
	FotoPath          *string    `json:"fotoPath,omitempty"`
	TanggalRencana    *time.Time `json:"tanggalRencana,omitempty"`
	AlasanPerubahan   string     `json:"alasanPerubahan,omitempty"`
	TerkunciAt        *time.Time `json:"terkunciAt,omitempty"`
	DibatalkanAt      *time.Time `json:"dibatalkanAt,omitempty"`
	CanEdit           bool       `json:"canEdit"`
}

type journalSheetResponse struct {
	KelasID          string                 `json:"kelasId"`
	KelasLabel       string                 `json:"kelasLabel"`
	Tanggal          string                 `json:"tanggal"`
	AttendanceStatus string                 `json:"attendanceStatus"`
	AbsentStudents   []journalAbsentStudent `json:"absentStudents"`
	Lines            []journalSheetLine     `json:"lines"`
}

func journalDateAtMidnight(t time.Time) time.Time {
	w := t.In(wibLocation)
	return time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, wibLocation)
}

func (s *Server) activeJournalSemester(now time.Time) (Semester, error) {
	today := journalDateAtMidnight(now)
	var year TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		return Semester{}, fiber.NewError(fiber.StatusBadRequest, "tahun ajaran aktif tidak ditemukan")
	}
	var semester Semester
	if err := s.db.Where("tahun_ajaran_id = ? AND is_archived = ? AND tanggal_mulai <= ? AND tanggal_selesai >= ?", year.ID, false, today, today).
		Order("tanggal_mulai desc").First(&semester).Error; err != nil {
		return Semester{}, fiber.NewError(fiber.StatusBadRequest, "semester aktif tidak ditemukan")
	}
	return semester, nil
}

func (s *Server) validateJournalDate(t time.Time) error {
	date := journalDateAtMidnight(t)
	today := journalDateAtMidnight(time.Now())
	if date.After(today) {
		return fiber.NewError(fiber.StatusBadRequest, "tanggal jurnal tidak boleh melewati hari ini")
	}
	semester, err := s.activeJournalSemester(time.Now())
	if err != nil {
		return err
	}
	start, end := journalDateAtMidnight(semester.TanggalMulai), journalDateAtMidnight(semester.TanggalSelesai)
	if date.Before(start) || date.After(end) {
		return fiber.NewError(fiber.StatusBadRequest, "tanggal jurnal harus berada pada semester aktif")
	}
	return nil
}

func (s *Server) journalTutorForRequest(c *fiber.Ctx) (string, error) {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return "", fiber.NewError(fiber.StatusForbidden, "hanya admin atau tutor yang dapat mengisi jurnal")
	}
	if role == "admin" {
		tutorID := strings.TrimSpace(c.FormValue("tutorId"))
		if tutorID == "" {
			return "", fiber.NewError(fiber.StatusBadRequest, "tutorId wajib diisi saat admin mencatat jurnal")
		}
		if err := s.db.First(&Tutor{}, "id = ?", tutorID).Error; err != nil {
			return "", fiber.NewError(fiber.StatusBadRequest, "tutor tidak ditemukan")
		}
		return tutorID, nil
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		return "", fiber.NewError(fiber.StatusForbidden, "profil tutor tidak ditemukan")
	}
	return *user.TutorID, nil
}

func (s *Server) tutorCanViewJournalClass(c *fiber.Ctx, kelasID string) (bool, error) {
	role := c.Locals("role").(string)
	if role == "admin" || role == "kepala_sekolah" {
		return true, nil
	}
	if role != "guru" {
		return false, fiber.NewError(fiber.StatusForbidden, "tidak diizinkan")
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		return false, fiber.NewError(fiber.StatusForbidden, "profil tutor tidak ditemukan")
	}
	var count int64
	if err := s.db.Model(&PenugasanGuruMapel{}).Where("tutor_id = ? AND kelas_id = ?", *user.TutorID, kelasID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func journalLineKey(jamKe int, mapelID string) string {
	return strconv.Itoa(jamKe) + "|" + mapelID
}

// preservedLineKeys are exact period/subject pairs already owned by the batch
// being edited. They may stay editable after an assignment is archived; every
// added period or changed subject is still validated against an active
// PenugasanGuruMapel.
func (s *Server) validateJournalLines(tx *gorm.DB, tutorID, kelasID string, lines []journalLineInput, date time.Time, exceptBatchID string, preservedLineKeys map[string]bool) error {
	if len(lines) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "minimal satu baris jurnal wajib diisi")
	}
	var kelas Kelas
	if err := tx.First(&kelas, "id = ?", kelasID).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "kelas tidak ditemukan")
	}
	seenPeriods := make(map[int]bool, len(lines))
	for _, line := range lines {
		if line.JamKe <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "jam ke harus bernilai positif")
		}
		if seenPeriods[line.JamKe] {
			return fiber.NewError(fiber.StatusBadRequest, "jam ke tidak boleh duplikat")
		}
		seenPeriods[line.JamKe] = true
		if strings.TrimSpace(line.MapelID) == "" || strings.TrimSpace(line.Materi) == "" {
			return fiber.NewError(fiber.StatusBadRequest, "mata pelajaran dan materi wajib diisi")
		}
		if line.StatusPublikasi == "" {
			line.StatusPublikasi = statusPublikasiDraf
		}
		if !validPublicationStatus(line.StatusPublikasi) {
			return fiber.NewError(fiber.StatusBadRequest, "status publikasi jurnal tidak valid")
		}
		if !preservedLineKeys[journalLineKey(line.JamKe, line.MapelID)] {
			var assignment PenugasanGuruMapel
			if err := tx.Preload("Mapel").Where("tutor_id = ? AND kelas_id = ? AND mapel_id = ?", tutorID, kelasID, line.MapelID).First(&assignment).Error; err != nil {
				return fiber.NewError(fiber.StatusForbidden, "mata pelajaran tidak termasuk penugasan tutor pada kelas ini")
			}
			if assignment.Mapel == nil || !assignment.Mapel.IsActive {
				return fiber.NewError(fiber.StatusBadRequest, "mata pelajaran tidak aktif")
			}
		}
		if err := s.validateJournalLineReferences(tx, kelas, line); err != nil {
			return err
		}
	}
	periods := make([]int, 0, len(lines))
	for period := range seenPeriods {
		periods = append(periods, period)
	}
	q := tx.Model(&JurnalMengajar{}).Where("kelas_id = ? AND tanggal = ? AND jam_ke IN ?", kelasID, date, periods)
	if exceptBatchID != "" {
		q = q.Where("batch_id <> ?", exceptBatchID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fiber.NewError(fiber.StatusConflict, "jam ke sudah dipakai pada jurnal kelas dan tanggal ini")
	}
	return nil
}

func parseJournalLines(c *fiber.Ctx) ([]journalLineInput, error) {
	raw := strings.TrimSpace(c.FormValue("lines"))
	if raw == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "baris jurnal wajib diisi")
	}
	var lines []journalLineInput
	if err := json.Unmarshal([]byte(raw), &lines); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "format baris jurnal tidak valid")
	}
	return lines, nil
}

func journalMutationError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") && strings.Contains(message, "jam_ke") {
		return fiber.NewError(fiber.StatusConflict, "jam ke sudah dipakai pada jurnal kelas dan tanggal ini")
	}
	return err
}

func (s *Server) createJournalBatch(c *fiber.Ctx) error {
	tutorID, err := s.journalTutorForRequest(c)
	if err != nil {
		return journalMutationError(err)
	}
	kelasID := strings.TrimSpace(c.FormValue("kelasId"))
	if kelasID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "kelasId wajib diisi")
	}
	tanggal, err := parseWIBDateTime(c.FormValue("tanggal"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "tanggal jurnal tidak valid")
	}
	if err := s.validateJournalDate(tanggal); err != nil {
		return journalMutationError(err)
	}
	tanggalRencana, err := parseOptionalWIBDate(c.FormValue("tanggalRencana"))
	if err != nil {
		return err
	}
	if tanggalRencana != nil {
		if err := s.validateJournalDate(*tanggalRencana); err != nil {
			return journalMutationError(err)
		}
		if !sameWIBDay(*tanggalRencana, tanggal) && strings.TrimSpace(c.FormValue("alasanPerubahan")) == "" {
			return fiber.NewError(fiber.StatusBadRequest, "alasan perubahan jadwal wajib diisi bila tanggal pelaksanaan berbeda")
		}
	}
	lines, err := parseJournalLines(c)
	if err != nil {
		return err
	}
	signature := strings.TrimSpace(c.FormValue("tandaTangan"))
	if !validSignature(signature) {
		return fiber.NewError(fiber.StatusBadRequest, "paraf PNG yang valid wajib diisi sebelum menyimpan")
	}
	fotoPath, err := s.saveUpload(c, "foto", "jurnal", 5*1024*1024, []string{"jpg", "jpeg", "png"})
	if err != nil {
		return err
	}
	var photo *string
	if fotoPath != "" {
		photo = &fotoPath
	}
	batch := JurnalBatch{TutorID: tutorID, KelasID: kelasID, Tanggal: journalDateAtMidnight(tanggal), TanggalRencana: tanggalRencana, AlasanPerubahan: strings.TrimSpace(c.FormValue("alasanPerubahan")), TandaTangan: signature, FotoPath: photo}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.validateJournalLines(tx, tutorID, kelasID, lines, batch.Tanggal, "", nil); err != nil {
			return err
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		for _, line := range lines {
			statusPublikasi := strings.TrimSpace(line.StatusPublikasi)
			if statusPublikasi == "" {
				statusPublikasi = statusPublikasiDraf
			}
			row := JurnalMengajar{BatchID: &batch.ID, JamKe: line.JamKe, TutorID: tutorID, KelasID: kelasID, MapelID: line.MapelID, Tanggal: batch.Tanggal, Materi: strings.TrimSpace(line.Materi), Kegiatan: strings.TrimSpace(line.Kegiatan), Tujuan: strings.TrimSpace(line.Tujuan), Metode: strings.TrimSpace(line.Metode), Media: strings.TrimSpace(line.Media), Keterlibatan: strings.TrimSpace(line.Keterlibatan), Asesmen: strings.TrimSpace(line.Asesmen), HasilAsesmen: strings.TrimSpace(line.HasilAsesmen), Kendala: strings.TrimSpace(line.Kendala), Refleksi: strings.TrimSpace(line.Refleksi), TindakLanjut: strings.TrimSpace(line.TindakLanjut), RingkasanOrangTua: strings.TrimSpace(line.RingkasanOrangTua), RPPID: formPtr(line.RPPID), ModulID: formPtr(line.ModulID), MateriID: formPtr(line.MateriID), TugasID: formPtr(line.TugasID), KompetensiID: formPtr(line.KompetensiID), KelasVirtualID: formPtr(line.KelasVirtualID), StatusPublikasi: statusPublikasi, Status: "disetujui"}
			if statusPublikasi == statusPublikasiDipublikasikan {
				now, uid := time.Now(), c.Locals("userID").(string)
				row.DipublikasikanAt, row.DipublikasikanOlehUserID = &now, &uid
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if photo != nil {
			removeUpload(*photo)
		}
		return journalMutationError(err)
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "jurnal_batch", batch.ID)
	return c.Status(fiber.StatusCreated).JSON(batch)
}

func (s *Server) journalBatchForEdit(c *fiber.Ctx, batchID string) (JurnalBatch, error) {
	var batch JurnalBatch
	if err := s.db.Preload("Lines").First(&batch, "id = ?", batchID).Error; err != nil {
		return batch, fiber.NewError(fiber.StatusNotFound, "jurnal tidak ditemukan")
	}
	if batch.DibatalkanAt != nil {
		return batch, fiber.NewError(fiber.StatusBadRequest, "jurnal yang dibatalkan tidak dapat diubah")
	}
	if batch.TerkunciAt != nil {
		return batch, fiber.NewError(fiber.StatusLocked, "jurnal telah dikunci")
	}
	role := c.Locals("role").(string)
	if role == "admin" {
		return batch, nil
	}
	if role != "guru" {
		return batch, fiber.NewError(fiber.StatusForbidden, "tidak diizinkan")
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil || batch.TutorID != *user.TutorID {
		return batch, fiber.NewError(fiber.StatusForbidden, "hanya pemilik jurnal yang dapat mengubah")
	}
	return batch, nil
}

func (s *Server) updateJournalBatch(c *fiber.Ctx) error {
	batch, err := s.journalBatchForEdit(c, id(c))
	if err != nil {
		return err
	}
	kelasID := strings.TrimSpace(c.FormValue("kelasId"))
	if kelasID == "" {
		kelasID = batch.KelasID
	}
	tanggal := batch.Tanggal
	if rawDate := strings.TrimSpace(c.FormValue("tanggal")); rawDate != "" {
		tanggal, err = parseWIBDateTime(rawDate)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "tanggal jurnal tidak valid")
		}
	}
	if err := s.validateJournalDate(tanggal); err != nil {
		return err
	}
	tanggalRencana := batch.TanggalRencana
	if raw := strings.TrimSpace(c.FormValue("tanggalRencana")); raw != "" || c.FormValue("tanggalRencanaCleared") == "1" {
		var parseErr error
		tanggalRencana, parseErr = parseOptionalWIBDate(raw)
		if parseErr != nil {
			return parseErr
		}
	}
	alasanPerubahan := strings.TrimSpace(c.FormValue("alasanPerubahan"))
	if alasanPerubahan == "" {
		alasanPerubahan = batch.AlasanPerubahan
	}
	if tanggalRencana != nil {
		if err := s.validateJournalDate(*tanggalRencana); err != nil {
			return err
		}
		if !sameWIBDay(*tanggalRencana, tanggal) && alasanPerubahan == "" {
			return fiber.NewError(fiber.StatusBadRequest, "alasan perubahan jadwal wajib diisi bila tanggal pelaksanaan berbeda")
		}
	}
	lines, err := parseJournalLines(c)
	if err != nil {
		return err
	}
	signature := strings.TrimSpace(c.FormValue("tandaTangan"))
	if signature == "" {
		signature = batch.TandaTangan
	}
	if !validSignature(signature) {
		return fiber.NewError(fiber.StatusBadRequest, "paraf PNG yang valid wajib diisi untuk jurnal lama")
	}
	fotoPath, err := s.saveUpload(c, "foto", "jurnal", 5*1024*1024, []string{"jpg", "jpeg", "png"})
	if err != nil {
		return err
	}
	oldPhoto := batch.FotoPath
	preservedLineKeys := map[string]bool{}
	if kelasID == batch.KelasID {
		for _, line := range batch.Lines {
			preservedLineKeys[journalLineKey(line.JamKe, line.MapelID)] = true
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.validateJournalLines(tx, batch.TutorID, kelasID, lines, journalDateAtMidnight(tanggal), batch.ID, preservedLineKeys); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("batch_id = ?", batch.ID).Delete(&JurnalMengajar{}).Error; err != nil {
			return err
		}
		batch.KelasID, batch.Tanggal, batch.TanggalRencana, batch.AlasanPerubahan, batch.TandaTangan = kelasID, journalDateAtMidnight(tanggal), tanggalRencana, alasanPerubahan, signature
		if fotoPath != "" {
			batch.FotoPath = &fotoPath
		}
		// Lines were preloaded for authorization. Do not let GORM save those
		// stale associations again after the replacement delete below.
		if err := tx.Omit("Lines").Save(&batch).Error; err != nil {
			return err
		}
		for _, line := range lines {
			statusPublikasi := strings.TrimSpace(line.StatusPublikasi)
			if statusPublikasi == "" {
				statusPublikasi = statusPublikasiDraf
			}
			row := JurnalMengajar{BatchID: &batch.ID, JamKe: line.JamKe, TutorID: batch.TutorID, KelasID: kelasID, MapelID: line.MapelID, Tanggal: batch.Tanggal, Materi: strings.TrimSpace(line.Materi), Kegiatan: strings.TrimSpace(line.Kegiatan), Tujuan: strings.TrimSpace(line.Tujuan), Metode: strings.TrimSpace(line.Metode), Media: strings.TrimSpace(line.Media), Keterlibatan: strings.TrimSpace(line.Keterlibatan), Asesmen: strings.TrimSpace(line.Asesmen), HasilAsesmen: strings.TrimSpace(line.HasilAsesmen), Kendala: strings.TrimSpace(line.Kendala), Refleksi: strings.TrimSpace(line.Refleksi), TindakLanjut: strings.TrimSpace(line.TindakLanjut), RingkasanOrangTua: strings.TrimSpace(line.RingkasanOrangTua), RPPID: formPtr(line.RPPID), ModulID: formPtr(line.ModulID), MateriID: formPtr(line.MateriID), TugasID: formPtr(line.TugasID), KompetensiID: formPtr(line.KompetensiID), KelasVirtualID: formPtr(line.KelasVirtualID), StatusPublikasi: statusPublikasi, Status: "disetujui"}
			if statusPublikasi == statusPublikasiDipublikasikan {
				now, uid := time.Now(), c.Locals("userID").(string)
				row.DipublikasikanAt, row.DipublikasikanOlehUserID = &now, &uid
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if fotoPath != "" {
			removeUpload(fotoPath)
		}
		return journalMutationError(err)
	}
	if fotoPath != "" && oldPhoto != nil && *oldPhoto != fotoPath {
		removeUpload(*oldPhoto)
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "jurnal_batch", batch.ID)
	return c.JSON(batch)
}

func (s *Server) deleteJournalBatch(c *fiber.Ctx) error {
	return s.cancelJournalBatch(c)
}

func journalClassLabel(k Kelas) string {
	return fmt.Sprintf("Kelas %d%s", k.Jenjang, k.NamaRombel)
}

func (s *Server) buildJournalSheet(c *fiber.Ctx, kelasID string, tanggal time.Time) (journalSheetResponse, error) {
	allowed, err := s.tutorCanViewJournalClass(c, kelasID)
	if err != nil {
		return journalSheetResponse{}, err
	}
	if !allowed {
		return journalSheetResponse{}, fiber.NewError(fiber.StatusForbidden, "tidak ditugaskan pada kelas ini")
	}
	date := journalDateAtMidnight(tanggal)
	var kelas Kelas
	if err := s.db.First(&kelas, "id = ?", kelasID).Error; err != nil {
		return journalSheetResponse{}, fiber.NewError(fiber.StatusNotFound, "kelas tidak ditemukan")
	}
	response := journalSheetResponse{
		KelasID: kelasID, KelasLabel: journalClassLabel(kelas), Tanggal: wibTimeFormat(date, "2006-01-02"),
		AttendanceStatus: "belum_ada", AbsentStudents: []journalAbsentStudent{}, Lines: []journalSheetLine{},
	}
	var rows []JurnalMengajar
	query := s.db.Preload("Tutor").Preload("Mapel").Preload("Batch").Where("kelas_id = ? AND tanggal = ?", kelasID, date)
	query, err = s.journalTutorScope(c, query)
	if err != nil {
		return response, err
	}
	if err := query.
		Order("jam_ke, created_at, id").Find(&rows).Error; err != nil {
		return response, err
	}
	role := c.Locals("role").(string)
	userTutorID := ""
	if role == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error == nil && user.TutorID != nil {
			userTutorID = *user.TutorID
		}
	}
	for _, row := range rows {
		batchID, signature, photo := "", "", row.FotoPath
		var terkunciAt, dibatalkanAt, tanggalRencana *time.Time
		alasanPerubahan := ""
		if row.BatchID != nil {
			batchID = *row.BatchID
		}
		if row.Batch != nil {
			if row.Batch.DibatalkanAt != nil {
				continue
			}
			signature, photo = row.Batch.TandaTangan, row.Batch.FotoPath
			terkunciAt, dibatalkanAt = row.Batch.TerkunciAt, row.Batch.DibatalkanAt
			tanggalRencana, alasanPerubahan = row.Batch.TanggalRencana, row.Batch.AlasanPerubahan
		}
		response.Lines = append(response.Lines, journalSheetLine{
			ID: row.ID, BatchID: batchID, JamKe: row.JamKe, TutorID: row.TutorID, TutorNama: row.Tutor.Nama,
			MapelID: row.MapelID, MapelNama: row.Mapel.NamaMapel, Materi: row.Materi, Kegiatan: row.Kegiatan, Tujuan: row.Tujuan, Metode: row.Metode, Media: row.Media, Keterlibatan: row.Keterlibatan, Asesmen: row.Asesmen, HasilAsesmen: row.HasilAsesmen, Kendala: row.Kendala, Refleksi: row.Refleksi, TindakLanjut: row.TindakLanjut, RingkasanOrangTua: row.RingkasanOrangTua, RPPID: row.RPPID, ModulID: row.ModulID, MateriID: row.MateriID, TugasID: row.TugasID, KompetensiID: row.KompetensiID, KelasVirtualID: row.KelasVirtualID, StatusPublikasi: row.StatusPublikasi,
			TandaTangan: signature, FotoPath: photo, CanEdit: terkunciAt == nil && (role == "admin" || (role == "guru" && row.TutorID == userTutorID)),
			TerkunciAt: terkunciAt, DibatalkanAt: dibatalkanAt,
			TanggalRencana: tanggalRencana, AlasanPerubahan: alasanPerubahan,
		})
	}
	var meeting Presensi
	if err := s.db.Preload("Details.PesertaDidik").Where("kelas_id = ? AND tanggal = ?", kelasID, date).First(&meeting).Error; err == nil {
		response.AttendanceStatus = "terisi"
		for _, detail := range meeting.Details {
			if detail.StatusKehadiran == "" || detail.StatusKehadiran == "Hadir" {
				continue
			}
			response.AbsentStudents = append(response.AbsentStudents, journalAbsentStudent{PesertaDidikID: detail.PesertaDidikID, Nama: detail.PesertaDidik.Nama, StatusKehadiran: detail.StatusKehadiran})
		}
		var total int64
		s.db.Model(&PesertaDidik{}).Where("kelas_id = ? AND status = ?", kelasID, "aktif").Count(&total)
		if int64(len(meeting.Details)) < total {
			response.AttendanceStatus = "sebagian"
		}
	}
	return response, nil
}

func (s *Server) getJournalSheet(c *fiber.Ctx) error {
	kelasID := strings.TrimSpace(c.Query("kelasId"))
	tanggal, err := parseWIBDateTime(c.Query("tanggal"))
	if kelasID == "" || err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "kelasId dan tanggal jurnal wajib diisi")
	}
	sheet, err := s.buildJournalSheet(c, kelasID, tanggal)
	if err != nil {
		return err
	}
	return c.JSON(sheet)
}

func journalAbsenceText(absences []journalAbsentStudent) string {
	if len(absences) == 0 {
		return "—"
	}
	values := make([]string, 0, len(absences))
	for _, absent := range absences {
		values = append(values, fmt.Sprintf("%s (%s)", absent.Nama, absent.StatusKehadiran))
	}
	return strings.Join(values, ", ")
}

func journalFileName(sheet journalSheetResponse, ext string) string {
	label := strings.NewReplacer(" ", "-", "/", "-", "\\", "-").Replace(strings.ToLower(sheet.KelasLabel))
	return "jurnal-" + label + "-" + strings.ReplaceAll(sheet.Tanggal, "-", "") + "." + ext
}

func journalBatchFirstLines(lines []journalSheetLine) map[string]bool {
	first := make(map[string]bool, len(lines))
	for _, line := range lines {
		if line.BatchID != "" && !first[line.BatchID] {
			first[line.BatchID] = true
		}
	}
	return first
}

// gofpdf is stricter than browser canvas/image decoders for some otherwise
// readable PNG signatures. Re-encoding normalizes the PNG stream before it is
// embedded, so a user's valid canvas signature cannot break the full export.
func journalCanonicalPNG(data []byte) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}

func (s *Server) exportJournalPDF(c *fiber.Ctx, sheet journalSheetResponse) error {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(277, 8, "PKBM Tunas Ilmu - Jurnal Mengajar", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(277, 6, sheet.KelasLabel+" | "+sheet.Tanggal, "", 1, "C", false, 0, "")
	pdf.CellFormat(277, 6, "Peserta didik tidak hadir: "+journalAbsenceText(sheet.AbsentStudents), "", 1, "L", false, 0, "")
	pdf.Ln(3)
	widths := []float64{12, 38, 45, 82, 60, 40}
	headers := []string{"Jam Ke", "Nama Tutor", "Mata Pelajaran", "Materi", "Peserta Didik Tidak Hadir", "Paraf"}
	pdf.SetFont("Helvetica", "B", 9)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 8, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)
	seenBatch := map[string]bool{}
	for _, line := range sheet.Lines {
		pdf.SetFont("Helvetica", "", 8)
		cells := []string{strconv.Itoa(line.JamKe), line.TutorNama, line.MapelNama, line.Materi, journalAbsenceText(sheet.AbsentStudents)}
		for i, value := range cells {
			pdf.CellFormat(widths[i], 13, journalTruncate(value, 65), "1", 0, "L", false, 0, "")
		}
		if line.BatchID != "" && !seenBatch[line.BatchID] && line.TandaTangan != "" {
			if data, ok := signatureImage(line.TandaTangan); ok {
				data, ok = journalCanonicalPNG(data)
				if !ok {
					pdf.CellFormat(widths[5], 13, "—", "1", 0, "C", false, 0, "")
					seenBatch[line.BatchID] = true
					pdf.Ln(-1)
					continue
				}
				name := "signature-" + line.BatchID
				opts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
				pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(data))
				pdf.CellFormat(widths[5], 13, "", "1", 0, "C", false, 0, "")
				pdf.ImageOptions(name, 251, pdf.GetY()+1, 28, 0, false, opts, 0, "")
			} else {
				pdf.CellFormat(widths[5], 13, "—", "1", 0, "C", false, 0, "")
			}
			seenBatch[line.BatchID] = true
		} else {
			pdf.CellFormat(widths[5], 13, "", "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment(journalFileName(sheet, "pdf"))
	return pdf.Output(c.Response().BodyWriter())
}

func journalTruncate(value string, max int) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "\r", " ")
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

func journalDocxText(cell document.Cell, value string, bold bool) {
	p := cell.AddParagraph()
	r := p.AddRun()
	r.AddText(value)
	r.Properties().SetBold(bold)
	r.Properties().SetSize(9 * measurement.Point)
}

func (s *Server) exportJournalDOCX(c *fiber.Ctx, sheet journalSheetResponse) error {
	doc := document.New()
	title := doc.AddParagraph().AddRun()
	title.AddText("PKBM Tunas Ilmu - Jurnal Mengajar")
	title.Properties().SetBold(true)
	title.Properties().SetSize(15 * measurement.Point)
	doc.AddParagraph().AddRun().AddText(sheet.KelasLabel + " | " + sheet.Tanggal)
	doc.AddParagraph().AddRun().AddText("Peserta didik tidak hadir: " + journalAbsenceText(sheet.AbsentStudents))
	table := doc.AddTable()
	headers := []string{"Jam Ke", "Nama Tutor", "Mata Pelajaran", "Materi", "Peserta Didik Tidak Hadir", "Paraf"}
	header := table.AddRow()
	for _, value := range headers {
		journalDocxText(header.AddCell(), value, true)
	}
	seenBatch := map[string]bool{}
	for _, line := range sheet.Lines {
		row := table.AddRow()
		for _, value := range []string{strconv.Itoa(line.JamKe), line.TutorNama, line.MapelNama, line.Materi, journalAbsenceText(sheet.AbsentStudents)} {
			journalDocxText(row.AddCell(), value, false)
		}
		cell := row.AddCell()
		if line.BatchID != "" && !seenBatch[line.BatchID] && line.TandaTangan != "" {
			if data, ok := signatureImage(line.TandaTangan); ok {
				if img, err := common.ImageFromBytes(data); err == nil {
					if ref, err := doc.AddImage(img); err == nil {
						run := cell.AddParagraph().AddRun()
						if drawing, err := run.AddDrawingInline(ref); err == nil {
							drawing.SetSize(32*measurement.Millimeter, 10*measurement.Millimeter)
						}
					}
				}
			}
			seenBatch[line.BatchID] = true
		} else {
			journalDocxText(cell, "", false)
		}
	}
	var out bytes.Buffer
	if err := doc.Save(&out); err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	c.Attachment(journalFileName(sheet, "docx"))
	return c.Send(out.Bytes())
}

func journalSetFont(dc *gg.Context, size float64) error {
	font, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(font, &opentype.FaceOptions{Size: size, DPI: 72})
	if err != nil {
		return err
	}
	dc.SetFontFace(face)
	return nil
}

func (s *Server) exportJournalJPG(c *fiber.Ctx, sheet journalSheetResponse) error {
	const width = 3508
	height := 620 + len(sheet.Lines)*170
	dc := gg.NewContext(width, height)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0.08, 0.1, 0.14)
	if err := journalSetFont(dc, 48); err != nil {
		return err
	}
	dc.DrawStringAnchored("PKBM Tunas Ilmu - Jurnal Mengajar", width/2, 70, .5, .5)
	if err := journalSetFont(dc, 29); err != nil {
		return err
	}
	dc.DrawStringAnchored(sheet.KelasLabel+" | "+sheet.Tanggal, width/2, 125, .5, .5)
	dc.DrawString("Peserta didik tidak hadir: "+journalTruncate(journalAbsenceText(sheet.AbsentStudents), 180), 70, 175)
	columns := []float64{70, 250, 780, 1300, 2150, 2940, 3438}
	y := 230.0
	if err := journalSetFont(dc, 25); err != nil {
		return err
	}
	for index, title := range []string{"Jam Ke", "Nama Tutor", "Mata Pelajaran", "Materi", "Peserta Didik Tidak Hadir", "Paraf"} {
		dc.SetRGB(0.1, 0.2, 0.35)
		dc.DrawRectangle(columns[index], y, columns[index+1]-columns[index], 58)
		dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawStringAnchored(title, (columns[index]+columns[index+1])/2, y+30, .5, .5)
	}
	y += 58
	seenBatch := map[string]bool{}
	for _, line := range sheet.Lines {
		dc.SetRGB(0.1, 0.1, 0.1)
		for i := range columns[:len(columns)-1] {
			dc.DrawRectangle(columns[i], y, columns[i+1]-columns[i], 130)
			dc.Stroke()
		}
		if err := journalSetFont(dc, 23); err != nil {
			return err
		}
		values := []string{strconv.Itoa(line.JamKe), journalTruncate(line.TutorNama, 24), journalTruncate(line.MapelNama, 31), journalTruncate(line.Materi, 55), journalTruncate(journalAbsenceText(sheet.AbsentStudents), 46)}
		for index, value := range values {
			dc.DrawString(value, columns[index]+12, y+45)
		}
		if line.BatchID != "" && !seenBatch[line.BatchID] {
			if data, ok := signatureImage(line.TandaTangan); ok {
				if signature, _, err := image.Decode(bytes.NewReader(data)); err == nil {
					dc.Push()
					dc.Translate(columns[5]+8, y+12)
					dc.Scale(0.32, 0.32)
					dc.DrawImage(signature, 0, 0)
					dc.Pop()
				}
			}
			seenBatch[line.BatchID] = true
		}
		y += 130
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dc.Image(), &jpeg.Options{Quality: 92}); err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "image/jpeg")
	c.Attachment(journalFileName(sheet, "jpg"))
	return c.Send(out.Bytes())
}

func (s *Server) exportJournal(c *fiber.Ctx) error {
	kelasID := strings.TrimSpace(c.Query("kelasId"))
	tanggal, err := parseWIBDateTime(c.Query("tanggal"))
	if kelasID == "" || err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "kelasId dan tanggal jurnal wajib diisi")
	}
	sheet, err := s.buildJournalSheet(c, kelasID, tanggal)
	if err != nil {
		return err
	}
	switch strings.ToLower(c.Query("format")) {
	case "pdf":
		return s.exportJournalPDF(c, sheet)
	case "docx", "word":
		return s.exportJournalDOCX(c, sheet)
	case "jpg", "jpeg":
		return s.exportJournalJPG(c, sheet)
	default:
		return fiber.NewError(fiber.StatusBadRequest, "format ekspor harus pdf, docx, atau jpg")
	}
}
