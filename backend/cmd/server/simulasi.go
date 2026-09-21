package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const (
	simulasiTipePG          = "pg_tunggal"
	simulasiTipePGK         = "pg_kompleks"
	simulasiTipeBenarSalah  = "benar_salah"
	simulasiTipeMenjodohkan = "menjodohkan"
	simulasiTipeIsian       = "isian_singkat"
	simulasiTipeUraian      = "uraian"
)

type simulasiChoice struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	ImageID string `json:"imageId,omitempty"`
}
type simulasiStatement struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}
type simulasiRubrik struct {
	Kriteria string  `json:"kriteria"`
	Maks     float64 `json:"maks"`
}
type simulasiConfig struct {
	Choices         []simulasiChoice    `json:"choices,omitempty"`
	CorrectIDs      []string            `json:"correctIds,omitempty"`
	Statements      []simulasiStatement `json:"statements,omitempty"`
	Left            []simulasiChoice    `json:"left,omitempty"`
	Right           []simulasiChoice    `json:"right,omitempty"`
	Pairs           map[string]string   `json:"pairs,omitempty"`
	AcceptedAnswers []string            `json:"acceptedAnswers,omitempty"`
	Rubrik          []simulasiRubrik    `json:"rubrik,omitempty"`
}
type simulasiQuestionInput struct {
	MapelID          *string              `json:"mapelId"`
	Jenjang          string               `json:"jenjang"`
	KelasFase        string               `json:"kelasFase"`
	Mode             string               `json:"mode"`
	Domain           string               `json:"domain"`
	Topik            string               `json:"topik"`
	Kompetensi       string               `json:"kompetensi"`
	LevelKognitif    string               `json:"levelKognitif"`
	TingkatKesulitan string               `json:"tingkatKesulitan"`
	Tags             string               `json:"tags"`
	Tipe             string               `json:"tipe"`
	Pertanyaan       string               `json:"pertanyaan"`
	Konfigurasi      simulasiConfig       `json:"konfigurasi"`
	Pembahasan       string               `json:"pembahasan"`
	Bobot            float64              `json:"bobot"`
	Status           string               `json:"status"`
	Stimulus         []simulasiStimulusIn `json:"stimulus"`
	Revision         int                  `json:"revision"`
}
type simulasiStimulusIn struct {
	Jenis   string `json:"jenis"`
	Konten  string `json:"konten"`
	AltText string `json:"altText"`
	Urutan  int    `json:"urutan"`
}
type simulasiPaketInput struct {
	Nama                string     `json:"nama"`
	Deskripsi           string     `json:"deskripsi"`
	Mode                string     `json:"mode"`
	Jenjang             string     `json:"jenjang"`
	MapelID             *string    `json:"mapelId"`
	DurasiMenit         int        `json:"durasiMenit"`
	Instruksi           string     `json:"instruksi"`
	NilaiLulus          *float64   `json:"nilaiLulus"`
	WaktuMulai          *time.Time `json:"waktuMulai"`
	WaktuSelesai        *time.Time `json:"waktuSelesai"`
	MaksPercobaan       int        `json:"maksPercobaan"`
	AcakUrutan          bool       `json:"acakUrutan"`
	TampilkanNilai      bool       `json:"tampilkanNilai"`
	TampilkanRingkasan  bool       `json:"tampilkanRingkasan"`
	TampilkanPembahasan bool       `json:"tampilkanPembahasan"`
}

// simulasiBuilderInput is deliberately separate from the legacy package APIs.
// A canvas may be incomplete while the teacher is composing it, but publishing
// still performs the same complete validation and snapshotting as before.
type simulasiBuilderItemInput struct {
	SoalID string                 `json:"soalId"`
	Bobot  float64                `json:"bobot"`
	Soal   *simulasiQuestionInput `json:"soal,omitempty"`
}
type simulasiBuilderInput struct {
	Paket           simulasiPaketInput         `json:"paket"`
	Items           []simulasiBuilderItemInput `json:"items"`
	PesertaDidikIDs []string                   `json:"pesertaDidikIds"`
	Revision        int                        `json:"revision"`
}

type simulasiBahanInput struct {
	Judul     string `json:"judul"`
	Jenis     string `json:"jenis"`
	Konten    string `json:"konten"`
	AltText   string `json:"altText"`
	MediaURL  string `json:"mediaUrl"`
	Jenjang   string `json:"jenjang"`
	KelasFase string `json:"kelasFase"`
	Topik     string `json:"topik"`
	Tags      string `json:"tags"`
	Status    string `json:"status"`
	Revision  int    `json:"revision"`
}
type simulasiSnapshot struct {
	SoalID      string             `json:"soalId"`
	Tipe        string             `json:"tipe"`
	Pertanyaan  string             `json:"pertanyaan"`
	Stimulus    []SimulasiStimulus `json:"stimulus,omitempty"`
	Konfigurasi simulasiConfig     `json:"konfigurasi"`
	Pembahasan  string             `json:"pembahasan"`
	Metadata    map[string]string  `json:"metadata"`
}

func registerSimulasiRoutes(api fiber.Router, s *Server) {
	// Static routes must precede :id routes under Fiber.
	api.Get("/simulasi/workspace", s.simulasiWorkspaceSummary)
	api.Get("/simulasi/bahan", s.simulasiListBahan)
	api.Post("/simulasi/bahan", s.simulasiCreateBahan)
	api.Put("/simulasi/bahan/:id", s.simulasiUpdateBahan)
	api.Delete("/simulasi/bahan/:id", s.simulasiArchiveBahan)
	api.Post("/simulasi/media", s.simulasiUploadMedia)
	api.Post("/simulasi/soal/from-bank/:id", s.simulasiCopyLegacyQuestion)
	api.Get("/simulasi/soal/template", s.simulasiTemplate)
	api.Get("/simulasi/soal/export", s.simulasiExportSoal)
	api.Post("/simulasi/soal/import", s.simulasiImportSoal)
	api.Get("/simulasi/soal", s.simulasiListSoal)
	api.Post("/simulasi/soal", s.simulasiCreateSoal)
	api.Get("/simulasi/soal/:id", s.simulasiGetSoal)
	api.Put("/simulasi/soal/:id", s.simulasiUpdateSoal)
	api.Delete("/simulasi/soal/:id", s.simulasiArchiveSoal)
	api.Post("/simulasi/soal/:id/stimulus/gambar", s.simulasiUploadStimulusImage)
	api.Post("/simulasi/soal/:id/opsi/:choiceId/gambar", s.simulasiUploadChoiceImage)
	// Image files are served through an authenticated endpoint rather than a
	// public uploads directory.  A student must still have the package assigned.
	api.Get("/simulasi/stimulus/:id/file", s.simulasiStimulusFile)

	api.Get("/simulasi/paket", s.simulasiListPaket)
	api.Post("/simulasi/paket/builder", s.simulasiCreateBuilderPaket)
	api.Post("/simulasi/paket", s.simulasiCreatePaket)
	api.Get("/simulasi/paket/:id/builder", s.simulasiGetBuilderPaket)
	api.Put("/simulasi/paket/:id/builder", s.simulasiSaveBuilderPaket)
	api.Get("/simulasi/paket/:id", s.simulasiGetPaket)
	api.Put("/simulasi/paket/:id", s.simulasiUpdatePaket)
	api.Post("/simulasi/paket/:id/duplikasi", s.simulasiDuplicatePaket)
	api.Post("/simulasi/paket/:id/publikasi", s.simulasiPublishPaket)
	api.Post("/simulasi/paket/:id/arsip", s.simulasiArchivePaket)
	api.Get("/simulasi/paket/:id/soal", s.simulasiListPaketSoal)
	api.Post("/simulasi/paket/:id/soal", s.simulasiAddPaketSoal)
	api.Post("/simulasi/paket/:id/soal-acak", s.simulasiAddPaketSoalAcak)
	api.Delete("/simulasi/paket/:id/soal/:itemId", s.simulasiDeletePaketSoal)
	api.Get("/simulasi/paket/:id/penugasan", s.simulasiListPenugasan)
	api.Put("/simulasi/paket/:id/penugasan", s.simulasiSetPenugasan)
	api.Get("/simulasi/paket/:id/hasil", s.simulasiStaffHasil)
	api.Get("/simulasi/paket/:id/analisis", s.simulasiAnalisis)
	api.Get("/simulasi/paket/:id/export", s.simulasiExportHasil)
	api.Get("/simulasi/upaya/:id/detail", s.simulasiDetailUpaya)
	api.Post("/simulasi/upaya/:id/jawaban/:jawabanId/nilai", s.simulasiNilaiUraian)

	api.Get("/simulasi/saya", s.simulasiSaya)
	api.Get("/simulasi/saya/paket/:id/instruksi", s.simulasiInstruksi)
	api.Post("/simulasi/saya/paket/:id/mulai", s.simulasiMulai)
	api.Get("/simulasi/saya/upaya/:id", s.simulasiWorkspace)
	api.Put("/simulasi/saya/upaya/:id/jawaban/:upayaSoalId", s.simulasiSimpanJawaban)
	api.Put("/simulasi/saya/upaya/:id/soal/:upayaSoalId/tandai", s.simulasiTandai)
	api.Post("/simulasi/saya/upaya/:id/kirim", s.simulasiKirim)
	api.Get("/simulasi/saya/upaya/:id/hasil", s.simulasiHasilSiswa)
}

func validateSimulasiBahan(in simulasiBahanInput) error {
	in.Judul, in.Jenis, in.Konten = strings.TrimSpace(in.Judul), strings.TrimSpace(in.Jenis), strings.TrimSpace(in.Konten)
	if in.Judul == "" || in.Konten == "" {
		return fiber.NewError(400, "judul dan isi bahan wajib diisi")
	}
	if in.Jenis != "text" && in.Jenis != "table" && in.Jenis != "image" && in.Jenis != "media_link" {
		return fiber.NewError(400, "jenis bahan tidak valid")
	}
	if in.Jenis == "image" && strings.TrimSpace(in.AltText) == "" {
		return fiber.NewError(400, "teks alternatif wajib diisi untuk gambar")
	}
	if in.Jenis == "media_link" {
		u, err := url.Parse(in.Konten)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fiber.NewError(400, "tautan bahan harus HTTPS")
		}
	}
	if in.MediaURL != "" {
		u, err := url.Parse(in.MediaURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fiber.NewError(400, "tautan media harus HTTPS")
		}
	}
	if in.Status == "" {
		in.Status = "draf"
	}
	if in.Status != "draf" && in.Status != "terbit" {
		return fiber.NewError(400, "status bahan harus draf atau terbit")
	}
	return nil
}

func (s *Server) simulasiWorkspaceSummary(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	questionQuery, packageQuery, bahanQuery := s.db.Model(&SimulasiSoal{}), s.db.Model(&SimulasiPaket{}), s.db.Model(&SimulasiBahan{})
	if c.Locals("role") == "guru" {
		questionQuery = questionQuery.Where("dibuat_oleh_user_id = ?", uid)
		packageQuery = packageQuery.Where("dibuat_oleh_user_id = ?", uid)
		bahanQuery = bahanQuery.Where("dibuat_oleh_user_id = ?", uid)
	}
	var questions, packages, bahan int64
	if err := questionQuery.Count(&questions).Error; err != nil {
		return err
	}
	if err := packageQuery.Count(&packages).Error; err != nil {
		return err
	}
	if err := bahanQuery.Count(&bahan).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"questions": questions, "packages": packages, "bahan": bahan, "offlineDrafts": true})
}

func (s *Server) simulasiListBahan(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	q := s.db.Order("created_at desc")
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	if jenis := c.Query("jenis"); jenis != "" {
		q = q.Where("jenis = ?", jenis)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		term := "%" + strings.ToLower(search) + "%"
		q = q.Where("lower(judul) LIKE ? OR lower(topik) LIKE ? OR lower(tags) LIKE ?", term, term, term)
	}
	var rows []SimulasiBahan
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}

func (s *Server) simulasiCreateBahan(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	var in simulasiBahanInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi bahan tidak valid")
	}
	if in.Status == "" {
		in.Status = "draf"
	}
	if err := validateSimulasiBahan(in); err != nil {
		return err
	}
	row := SimulasiBahan{Judul: strings.TrimSpace(in.Judul), Jenis: strings.TrimSpace(in.Jenis), Konten: strings.TrimSpace(in.Konten), AltText: strings.TrimSpace(in.AltText), MediaURL: strings.TrimSpace(in.MediaURL), Jenjang: strings.TrimSpace(in.Jenjang), KelasFase: strings.TrimSpace(in.KelasFase), Topik: strings.TrimSpace(in.Topik), Tags: strings.TrimSpace(in.Tags), Status: in.Status, DibuatOlehUserID: c.Locals("userID").(string), Revision: 1}
	if err := s.db.Create(&row).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "simulasi_bahan", row.ID)
	return c.Status(201).JSON(row)
}

func (s *Server) simulasiUpdateBahan(c *fiber.Ctx) error {
	var row SimulasiBahan
	if err := s.db.First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "bahan tidak ditemukan")
	}
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	if c.Locals("role") == "guru" && row.DibuatOlehUserID != c.Locals("userID") {
		return fiber.NewError(403, "bahan hanya dapat dikelola pembuatnya")
	}
	var in simulasiBahanInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi bahan tidak valid")
	}
	if in.Status == "" {
		in.Status = "draf"
	}
	if in.Revision > 0 && row.Revision > 0 && in.Revision != row.Revision {
		return fiber.NewError(409, "bahan sudah berubah di perangkat lain; muat versi terbaru atau simpan sebagai salinan")
	}
	if err := validateSimulasiBahan(in); err != nil {
		return err
	}
	row.Judul, row.Jenis, row.Konten = strings.TrimSpace(in.Judul), strings.TrimSpace(in.Jenis), strings.TrimSpace(in.Konten)
	row.AltText, row.MediaURL, row.Jenjang, row.KelasFase = strings.TrimSpace(in.AltText), strings.TrimSpace(in.MediaURL), strings.TrimSpace(in.Jenjang), strings.TrimSpace(in.KelasFase)
	row.Topik, row.Tags, row.Status = strings.TrimSpace(in.Topik), strings.TrimSpace(in.Tags), in.Status
	if row.Revision <= 0 {
		row.Revision = 1
	}
	row.Revision++
	if err := s.db.Save(&row).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "simulasi_bahan", row.ID)
	return c.JSON(row)
}

func (s *Server) simulasiArchiveBahan(c *fiber.Ctx) error {
	var row SimulasiBahan
	if err := s.db.First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "bahan tidak ditemukan")
	}
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	if c.Locals("role") == "guru" && row.DibuatOlehUserID != c.Locals("userID") {
		return fiber.NewError(403, "bahan hanya dapat dikelola pembuatnya")
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "archive", "simulasi_bahan", row.ID)
	return c.SendStatus(204)
}

func (s *Server) simulasiUploadMedia(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	altText := strings.TrimSpace(c.FormValue("altText"))
	if altText == "" {
		return fiber.NewError(400, "teks alternatif wajib diisi untuk gambar")
	}
	path, err := s.saveUpload(c, "file", "simulasi-bahan", 5*1024*1024, []string{"png", "jpg", "jpeg", "webp"})
	if err != nil {
		return err
	}
	if path == "" {
		return fiber.NewError(400, "file media wajib diunggah")
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "upload", "simulasi_media", path)
	return c.Status(201).JSON(fiber.Map{"path": path, "altText": altText})
}

func (s *Server) simulasiCopyLegacyQuestion(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	var source BankSoal
	if err := s.db.First(&source, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal lama tidak ditemukan")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") == "guru" && source.DibuatOlehUserID != uid {
		return fiber.NewError(403, "soal lama hanya dapat disalin pembuatnya")
	}
	config := simulasiConfig{}
	if source.Tipe == "pg" {
		var options []string
		if err := json.Unmarshal([]byte(source.Opsi), &options); err != nil || len(options) < 2 {
			return fiber.NewError(400, "opsi soal lama tidak valid")
		}
		for index, option := range options {
			config.Choices = append(config.Choices, simulasiChoice{ID: fmt.Sprintf("opsi-%d", index+1), Text: option})
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(source.Kunci))
		if idx < 0 || idx >= len(config.Choices) {
			idx = 0
		}
		config.CorrectIDs = []string{config.Choices[idx].ID}
	} else if source.Tipe == "essay" {
		max := source.Poin
		if max <= 0 {
			max = 1
		}
		config.Rubrik = []simulasiRubrik{{Kriteria: "Kualitas jawaban", Maks: max}}
	} else {
		return fiber.NewError(400, "tipe soal lama tidak didukung")
	}
	var mapelID *string
	if source.MapelID != "" {
		value := source.MapelID
		mapelID = &value
	}
	tipe := simulasiTipePG
	if source.Tipe == "essay" {
		tipe = simulasiTipeUraian
	}
	encoded, _ := json.Marshal(config)
	bobot := source.Poin
	if bobot <= 0 {
		bobot = 1
	}
	row := SimulasiSoal{MapelID: mapelID, Jenjang: "SD/MI", Mode: "anbk_akm", Tipe: tipe, Pertanyaan: strings.TrimSpace(source.Pertanyaan), Konfigurasi: string(encoded), Bobot: bobot, Status: "draf", DibuatOlehUserID: uid, Revision: 1, LegacySourceID: &source.ID}
	if err := s.db.Create(&row).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	s.audit(&uid, "copy_legacy", "simulasi_soal", row.ID)
	return s.simulasiGetSoalByID(c, row.ID, 201)
}

func simulasiStaff(c *fiber.Ctx, write bool) error {
	role, _ := c.Locals("role").(string)
	if role == "admin" || (!write && role == "kepala_sekolah") || role == "guru" {
		return nil
	}
	return fiber.NewError(403, "akses simulasi staf ditolak")
}

func (s *Server) simulasiSiswa(c *fiber.Ctx) (*User, *PesertaDidik, error) {
	if c.Locals("role") != "siswa" {
		return nil, nil, fiber.NewError(403, "halaman ini hanya untuk akun siswa")
	}
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.PesertaDidikID == nil {
		return nil, nil, fiber.NewError(403, "akun siswa belum dihubungkan ke peserta didik")
	}
	var siswa PesertaDidik
	if err := s.db.Preload("Kelas").Where("id = ? AND status = ?", *user.PesertaDidikID, "aktif").First(&siswa).Error; err != nil {
		return nil, nil, fiber.NewError(403, "data peserta didik tidak aktif")
	}
	return &user, &siswa, nil
}

func validSimulasiMode(v string) bool { return v == "anbk_akm" || v == "tka_sd" }
func validSimulasiTipe(v string) bool {
	return v == simulasiTipePG || v == simulasiTipePGK || v == simulasiTipeBenarSalah || v == simulasiTipeMenjodohkan || v == simulasiTipeIsian || v == simulasiTipeUraian
}
func validQuestionStatus(v string) bool { return v == "draf" || v == "terbit" }

func ensureChoiceIDs(choices []simulasiChoice) error {
	seen := map[string]bool{}
	for _, choice := range choices {
		if strings.TrimSpace(choice.ID) == "" || strings.TrimSpace(choice.Text) == "" || seen[choice.ID] {
			return fmt.Errorf("setiap pilihan harus memiliki id dan teks unik")
		}
		seen[choice.ID] = true
	}
	return nil
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func validateSimulasiConfig(tipe string, cfg simulasiConfig) error {
	switch tipe {
	case simulasiTipePG:
		if len(cfg.Choices) < 2 || len(cfg.CorrectIDs) != 1 {
			return fmt.Errorf("PG tunggal memerlukan minimal dua pilihan dan satu kunci")
		}
		if err := ensureChoiceIDs(cfg.Choices); err != nil {
			return err
		}
		if !containsString(choiceIDs(cfg.Choices), cfg.CorrectIDs[0]) {
			return fmt.Errorf("kunci tidak ditemukan pada pilihan")
		}
	case simulasiTipePGK:
		if len(cfg.Choices) < 2 || len(cfg.CorrectIDs) < 1 {
			return fmt.Errorf("PG kompleks memerlukan pilihan dan minimal satu kunci")
		}
		if err := ensureChoiceIDs(cfg.Choices); err != nil {
			return err
		}
		for _, id := range cfg.CorrectIDs {
			if !containsString(choiceIDs(cfg.Choices), id) {
				return fmt.Errorf("kunci tidak ditemukan pada pilihan")
			}
		}
	case simulasiTipeBenarSalah:
		if len(cfg.Statements) == 0 {
			return fmt.Errorf("benar/salah memerlukan pernyataan")
		}
		seen := map[string]bool{}
		for _, statement := range cfg.Statements {
			if statement.ID == "" || strings.TrimSpace(statement.Text) == "" || seen[statement.ID] {
				return fmt.Errorf("pernyataan harus memiliki id dan teks unik")
			}
			seen[statement.ID] = true
		}
	case simulasiTipeMenjodohkan:
		if len(cfg.Left) < 2 || len(cfg.Right) < 2 || len(cfg.Pairs) != len(cfg.Left) {
			return fmt.Errorf("menjodohkan memerlukan pasangan lengkap")
		}
		if err := ensureChoiceIDs(cfg.Left); err != nil {
			return err
		}
		if err := ensureChoiceIDs(cfg.Right); err != nil {
			return err
		}
		for _, left := range cfg.Left {
			right, ok := cfg.Pairs[left.ID]
			if !ok || !containsString(choiceIDs(cfg.Right), right) {
				return fmt.Errorf("pasangan menjodohkan tidak valid")
			}
		}
	case simulasiTipeIsian:
		if len(cfg.AcceptedAnswers) == 0 {
			return fmt.Errorf("isian singkat memerlukan jawaban diterima")
		}
	case simulasiTipeUraian:
		if len(cfg.Rubrik) == 0 {
			return fmt.Errorf("uraian memerlukan minimal satu rubrik")
		}
		for _, row := range cfg.Rubrik {
			if strings.TrimSpace(row.Kriteria) == "" || row.Maks <= 0 {
				return fmt.Errorf("rubrik uraian tidak valid")
			}
		}
	default:
		return fmt.Errorf("tipe soal tidak valid")
	}
	return nil
}
func choiceIDs(values []simulasiChoice) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func (s *Server) simulasiSoalScope(c *fiber.Ctx, row *SimulasiSoal, write bool) error {
	if err := simulasiStaff(c, write); err != nil {
		return err
	}
	if c.Locals("role") == "guru" && row.DibuatOlehUserID != c.Locals("userID") {
		return fiber.NewError(403, "soal hanya dapat dikelola pembuatnya")
	}
	return nil
}
func (s *Server) simulasiPaketScope(c *fiber.Ctx, row *SimulasiPaket, write bool) error {
	if err := simulasiStaff(c, write); err != nil {
		return err
	}
	if c.Locals("role") == "guru" && row.DibuatOlehUserID != c.Locals("userID") {
		return fiber.NewError(403, "paket hanya dapat dikelola pembuatnya")
	}
	return nil
}
func applyQuestionInput(row *SimulasiSoal, in simulasiQuestionInput) error {
	in.Jenjang, in.Mode, in.Tipe = strings.TrimSpace(in.Jenjang), strings.TrimSpace(in.Mode), strings.TrimSpace(in.Tipe)
	if in.Jenjang == "" || !validSimulasiMode(in.Mode) || !validSimulasiTipe(in.Tipe) || strings.TrimSpace(in.Pertanyaan) == "" {
		return fiber.NewError(400, "jenjang, mode, tipe, dan pertanyaan wajib valid")
	}
	if in.Status == "" {
		in.Status = "draf"
	}
	if !validQuestionStatus(in.Status) {
		return fiber.NewError(400, "status soal harus draf atau terbit")
	}
	if in.Bobot <= 0 {
		in.Bobot = 1
	}
	if err := validateSimulasiConfig(in.Tipe, in.Konfigurasi); err != nil {
		return fiber.NewError(400, err.Error())
	}
	cfg, _ := json.Marshal(in.Konfigurasi)
	row.MapelID, row.Jenjang, row.KelasFase, row.Mode = in.MapelID, in.Jenjang, strings.TrimSpace(in.KelasFase), in.Mode
	row.Domain, row.Topik, row.Kompetensi = strings.TrimSpace(in.Domain), strings.TrimSpace(in.Topik), strings.TrimSpace(in.Kompetensi)
	row.LevelKognitif, row.TingkatKesulitan, row.Tags = strings.TrimSpace(in.LevelKognitif), strings.TrimSpace(in.TingkatKesulitan), strings.TrimSpace(in.Tags)
	row.Tipe, row.Pertanyaan, row.Konfigurasi, row.Pembahasan, row.Bobot, row.Status = in.Tipe, strings.TrimSpace(in.Pertanyaan), string(cfg), strings.TrimSpace(in.Pembahasan), in.Bobot, in.Status
	if row.Revision <= 0 {
		row.Revision = 1
	}
	return nil
}
func staffQuestionResponse(row SimulasiSoal) fiber.Map {
	var cfg simulasiConfig
	_ = json.Unmarshal([]byte(row.Konfigurasi), &cfg)
	return fiber.Map{"id": row.ID, "mapelId": row.MapelID, "mapel": row.Mapel, "jenjang": row.Jenjang, "kelasFase": row.KelasFase, "mode": row.Mode, "domain": row.Domain, "topik": row.Topik, "kompetensi": row.Kompetensi, "levelKognitif": row.LevelKognitif, "tingkatKesulitan": row.TingkatKesulitan, "tags": row.Tags, "tipe": row.Tipe, "pertanyaan": row.Pertanyaan, "konfigurasi": cfg, "pembahasan": row.Pembahasan, "bobot": row.Bobot, "status": row.Status, "dibuatOlehUserId": row.DibuatOlehUserID, "legacySourceId": row.LegacySourceID, "revision": row.Revision, "stimulus": row.Stimulus, "createdAt": row.CreatedAt, "updatedAt": row.UpdatedAt}
}
func (s *Server) simulasiListSoal(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	q := s.db.Preload("Mapel").Preload("Stimulus").Order("created_at desc")
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	if value := c.Query("mode"); value != "" {
		q = q.Where("mode = ?", value)
	}
	if value := c.Query("mapelId"); value != "" {
		q = q.Where("mapel_id = ?", value)
	}
	if value := c.Query("status"); value != "" {
		q = q.Where("status = ?", value)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		q = q.Where("lower(pertanyaan) LIKE ? OR lower(topik) LIKE ?", "%"+strings.ToLower(search)+"%", "%"+strings.ToLower(search)+"%")
	}
	var rows []SimulasiSoal
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	result := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffQuestionResponse(row))
	}
	return c.JSON(result)
}
func (s *Server) simulasiGetSoal(c *fiber.Ctx) error {
	var row SimulasiSoal
	if err := s.db.Preload("Mapel").Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal simulasi tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &row, false); err != nil {
		return err
	}
	return c.JSON(staffQuestionResponse(row))
}
func (s *Server) simulasiCreateSoal(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	var in simulasiQuestionInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi soal tidak valid")
	}
	row := SimulasiSoal{DibuatOlehUserID: c.Locals("userID").(string)}
	if err := applyQuestionInput(&row, in); err != nil {
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, stimulus := range in.Stimulus {
			if err := validateStimulus(stimulus); err != nil {
				return err
			}
			if err := tx.Create(&SimulasiStimulus{SoalID: row.ID, Jenis: stimulus.Jenis, Konten: stimulus.Konten, AltText: stimulus.AltText, Urutan: stimulus.Urutan}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "simulasi_soal", row.ID)
	return s.simulasiGetSoalByID(c, row.ID, 201)
}
func validateStimulus(in simulasiStimulusIn) error {
	in.Jenis, in.Konten = strings.TrimSpace(in.Jenis), strings.TrimSpace(in.Konten)
	if in.Jenis != "text" && in.Jenis != "table" && in.Jenis != "image" && in.Jenis != "media_link" {
		return fiber.NewError(400, "jenis stimulus tidak valid")
	}
	if in.Konten == "" {
		return fiber.NewError(400, "isi stimulus wajib diisi")
	}
	if in.Jenis == "image" && strings.TrimSpace(in.AltText) == "" {
		return fiber.NewError(400, "teks alternatif wajib diisi untuk gambar")
	}
	if in.Jenis == "media_link" {
		u, err := url.Parse(in.Konten)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fiber.NewError(400, "tautan media harus HTTPS")
		}
	}
	return nil
}
func (s *Server) simulasiGetSoalByID(c *fiber.Ctx, id string, status int) error {
	var row SimulasiSoal
	if err := s.db.Preload("Mapel").Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	return c.Status(status).JSON(staffQuestionResponse(row))
}
func (s *Server) simulasiUpdateSoal(c *fiber.Ctx) error {
	var row SimulasiSoal
	if err := s.db.Preload("Stimulus").First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal simulasi tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &row, true); err != nil {
		return err
	}
	var in simulasiQuestionInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi soal tidak valid")
	}
	if in.Revision > 0 && row.Revision > 0 && in.Revision != row.Revision {
		return fiber.NewError(409, "soal sudah berubah di perangkat lain; muat versi terbaru atau simpan sebagai salinan")
	}
	if row.Revision <= 0 {
		row.Revision = 1
	}
	if err := applyQuestionInput(&row, in); err != nil {
		return err
	}
	row.Revision++
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if in.Stimulus != nil {
			if err := tx.Where("soal_id = ?", row.ID).Delete(&SimulasiStimulus{}).Error; err != nil {
				return err
			}
			for _, stimulus := range in.Stimulus {
				if err := validateStimulus(stimulus); err != nil {
					return err
				}
				if err := tx.Create(&SimulasiStimulus{SoalID: row.ID, Jenis: stimulus.Jenis, Konten: stimulus.Konten, AltText: stimulus.AltText, Urutan: stimulus.Urutan}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "simulasi_soal", row.ID)
	return s.simulasiGetSoalByID(c, row.ID, 200)
}
func (s *Server) simulasiArchiveSoal(c *fiber.Ctx) error {
	var row SimulasiSoal
	if err := s.db.First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal simulasi tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &row, true); err != nil {
		return err
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "archive", "simulasi_soal", row.ID)
	return c.SendStatus(204)
}
func (s *Server) simulasiUploadStimulusImage(c *fiber.Ctx) error {
	var row SimulasiSoal
	if err := s.db.First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal simulasi tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &row, true); err != nil {
		return err
	}
	altText := strings.TrimSpace(c.FormValue("altText"))
	if altText == "" {
		return fiber.NewError(400, "teks alternatif wajib diisi untuk gambar")
	}
	path, err := s.saveUpload(c, "file", "simulasi", 5*1024*1024, []string{"png", "jpg", "jpeg"})
	if err != nil {
		return err
	}
	if path == "" {
		return fiber.NewError(400, "file gambar wajib diunggah")
	}
	var count int64
	s.db.Model(&SimulasiStimulus{}).Where("soal_id = ?", row.ID).Count(&count)
	stimulus := SimulasiStimulus{SoalID: row.ID, Jenis: "image", Konten: path, AltText: altText, Urutan: int(count)}
	if err := s.db.Create(&stimulus).Error; err != nil {
		return err
	}
	return c.Status(201).JSON(stimulus)
}

func (s *Server) simulasiUploadChoiceImage(c *fiber.Ctx) error {
	var row SimulasiSoal
	if err := s.db.First(&row, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "soal simulasi tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &row, true); err != nil {
		return err
	}
	var cfg simulasiConfig
	if err := json.Unmarshal([]byte(row.Konfigurasi), &cfg); err != nil {
		return fiber.NewError(400, "konfigurasi soal tidak valid")
	}
	found := false
	for index := range cfg.Choices {
		if cfg.Choices[index].ID == c.Params("choiceId") {
			found = true
			break
		}
	}
	if !found {
		return fiber.NewError(404, "pilihan jawaban tidak ditemukan")
	}
	altText := strings.TrimSpace(c.FormValue("altText"))
	if altText == "" {
		return fiber.NewError(400, "teks alternatif wajib diisi untuk gambar")
	}
	path, err := s.saveUpload(c, "file", "simulasi", 5*1024*1024, []string{"png", "jpg", "jpeg"})
	if err != nil {
		return err
	}
	if path == "" {
		return fiber.NewError(400, "file gambar wajib diunggah")
	}
	stimulus := SimulasiStimulus{SoalID: row.ID, Jenis: "image", Konten: path, AltText: altText, Urutan: 0}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&stimulus).Error; err != nil {
			return err
		}
		for index := range cfg.Choices {
			if cfg.Choices[index].ID == c.Params("choiceId") {
				cfg.Choices[index].ImageID = stimulus.ID
			}
		}
		raw, _ := json.Marshal(cfg)
		return tx.Model(&SimulasiSoal{}).Where("id = ?", row.ID).Update("konfigurasi", string(raw)).Error
	}); err != nil {
		return err
	}
	return c.Status(201).JSON(stimulus)
}

func (s *Server) simulasiStimulusFile(c *fiber.Ctx) error {
	var stimulus SimulasiStimulus
	if err := s.db.First(&stimulus, "id = ? AND jenis = ?", c.Params("id"), "image").Error; err != nil {
		return fiber.NewError(404, "gambar stimulus tidak ditemukan")
	}
	if c.Locals("role") == "siswa" {
		_, student, err := s.simulasiSiswa(c)
		if err != nil {
			return err
		}
		// Check both assignment and the frozen snapshot. A source question can be
		// edited after publication, so source ownership alone is not sufficient.
		// The snapshot is JSON, so the LIKE pattern must contain literal quote
		// characters (not backslashes from a raw Go string).
		needle := fmt.Sprintf("%%\"id\":\"%s\"%%", stimulus.ID)
		var permitted int64
		err = s.db.Table("simulasi_penugasans").
			Joins("JOIN simulasi_pakets ON simulasi_pakets.id = simulasi_penugasans.paket_id").
			Joins("JOIN simulasi_paket_soals ON simulasi_paket_soals.paket_id = simulasi_pakets.id").
			Where("simulasi_penugasans.peserta_didik_id = ? AND simulasi_pakets.status = ? AND simulasi_paket_soals.snapshot_json LIKE ?", student.ID, "terbit", needle).
			Count(&permitted).Error
		if err != nil {
			return err
		}
		if permitted == 0 {
			return fiber.NewError(403, "gambar stimulus bukan bagian dari simulasi Anda")
		}
	} else {
		var question SimulasiSoal
		if err := s.db.First(&question, "id = ?", stimulus.SoalID).Error; err != nil {
			return fiber.NewError(404, "soal stimulus tidak ditemukan")
		}
		if err := s.simulasiSoalScope(c, &question, false); err != nil {
			return err
		}
	}
	return s.sendUpload(c, stimulus.Konten)
}

func applyPaketInput(row *SimulasiPaket, in simulasiPaketInput) error {
	in.Nama, in.Mode, in.Jenjang = strings.TrimSpace(in.Nama), strings.TrimSpace(in.Mode), strings.TrimSpace(in.Jenjang)
	if in.Nama == "" || in.Jenjang == "" || !validSimulasiMode(in.Mode) || in.DurasiMenit <= 0 || in.DurasiMenit > 360 {
		return fiber.NewError(400, "nama, jenjang, mode, dan durasi 1-360 menit wajib valid")
	}
	if in.MaksPercobaan <= 0 || in.MaksPercobaan > 10 {
		return fiber.NewError(400, "maksimal percobaan harus 1-10")
	}
	if in.WaktuMulai != nil && in.WaktuSelesai != nil && !in.WaktuSelesai.After(*in.WaktuMulai) {
		return fiber.NewError(400, "jadwal selesai harus setelah jadwal mulai")
	}
	row.Nama, row.Deskripsi, row.Mode, row.Jenjang, row.MapelID = in.Nama, strings.TrimSpace(in.Deskripsi), in.Mode, in.Jenjang, in.MapelID
	row.DurasiMenit, row.Instruksi, row.NilaiLulus, row.WaktuMulai, row.WaktuSelesai = in.DurasiMenit, strings.TrimSpace(in.Instruksi), in.NilaiLulus, in.WaktuMulai, in.WaktuSelesai
	row.MaksPercobaan, row.AcakUrutan, row.TampilkanNilai, row.TampilkanRingkasan, row.TampilkanPembahasan = in.MaksPercobaan, in.AcakUrutan, in.TampilkanNilai, in.TampilkanRingkasan, in.TampilkanPembahasan
	return nil
}

func applyBuilderPaketInput(row *SimulasiPaket, in simulasiPaketInput) error {
	if strings.TrimSpace(in.Nama) == "" {
		in.Nama = "Paket tanpa judul"
	}
	if strings.TrimSpace(in.Mode) == "" {
		in.Mode = "anbk_akm"
	}
	if strings.TrimSpace(in.Jenjang) == "" {
		in.Jenjang = "SD/MI"
	}
	if in.DurasiMenit == 0 {
		in.DurasiMenit = 60
	}
	if in.MaksPercobaan == 0 {
		in.MaksPercobaan = 1
	}
	return applyPaketInput(row, in)
}

func applyBuilderQuestionInput(row *SimulasiSoal, in simulasiQuestionInput, paket *SimulasiPaket) error {
	if strings.TrimSpace(in.Jenjang) == "" {
		in.Jenjang = paket.Jenjang
	}
	if strings.TrimSpace(in.Mode) == "" {
		in.Mode = paket.Mode
	}
	if strings.TrimSpace(in.Tipe) == "" {
		in.Tipe = simulasiTipePG
	}
	if !validSimulasiMode(strings.TrimSpace(in.Mode)) || !validSimulasiTipe(strings.TrimSpace(in.Tipe)) {
		return fiber.NewError(400, "mode atau tipe soal tidak valid")
	}
	if in.Bobot <= 0 {
		in.Bobot = 1
	}
	config, err := json.Marshal(in.Konfigurasi)
	if err != nil {
		return fiber.NewError(400, "konfigurasi soal tidak valid")
	}
	row.MapelID, row.Jenjang, row.KelasFase, row.Mode = in.MapelID, strings.TrimSpace(in.Jenjang), strings.TrimSpace(in.KelasFase), strings.TrimSpace(in.Mode)
	row.Domain, row.Topik, row.Kompetensi, row.LevelKognitif = strings.TrimSpace(in.Domain), strings.TrimSpace(in.Topik), strings.TrimSpace(in.Kompetensi), strings.TrimSpace(in.LevelKognitif)
	row.TingkatKesulitan, row.Tags, row.Tipe, row.Pertanyaan = strings.TrimSpace(in.TingkatKesulitan), strings.TrimSpace(in.Tags), strings.TrimSpace(in.Tipe), strings.TrimSpace(in.Pertanyaan)
	row.Konfigurasi, row.Pembahasan, row.Bobot, row.Status = string(config), strings.TrimSpace(in.Pembahasan), in.Bobot, "draf"
	return nil
}

func validateBuilderQuestionForPublish(row SimulasiSoal) error {
	var cfg simulasiConfig
	if err := json.Unmarshal([]byte(row.Konfigurasi), &cfg); err != nil {
		return fiber.NewError(400, "konfigurasi soal tidak valid")
	}
	return applyQuestionInput(&row, simulasiQuestionInput{MapelID: row.MapelID, Jenjang: row.Jenjang, KelasFase: row.KelasFase, Mode: row.Mode, Domain: row.Domain, Topik: row.Topik, Kompetensi: row.Kompetensi, LevelKognitif: row.LevelKognitif, TingkatKesulitan: row.TingkatKesulitan, Tags: row.Tags, Tipe: row.Tipe, Pertanyaan: row.Pertanyaan, Konfigurasi: cfg, Pembahasan: row.Pembahasan, Bobot: row.Bobot, Status: "terbit"})
}

func (s *Server) builderStudents(c *fiber.Ctx, tx *gorm.DB, requested []string) ([]PesertaDidik, error) {
	unique := map[string]bool{}
	for _, id := range requested {
		if strings.TrimSpace(id) != "" {
			unique[id] = true
		}
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	students := []PesertaDidik{}
	if len(ids) > 0 {
		if err := tx.Where("id IN ? AND status = ?", ids, "aktif").Find(&students).Error; err != nil {
			return nil, err
		}
		if len(students) != len(ids) {
			return nil, fiber.NewError(400, "sebagian peserta didik tidak aktif atau tidak ditemukan")
		}
	}
	if c.Locals("role") == "guru" {
		for _, student := range students {
			if err := s.canManageKelas(c, student.KelasID); err != nil {
				return nil, err
			}
		}
	}
	return students, nil
}

func replaceBuilderStimulus(tx *gorm.DB, soalID string, values []simulasiStimulusIn) error {
	// Image uploads use the existing hardened upload endpoint after a draft has
	// received a source-question ID. Keep them while text/table/media cards are
	// autosaved; the canvas never asks a teacher to serialise image metadata.
	if err := tx.Where("soal_id = ? AND jenis <> ?", soalID, "image").Delete(&SimulasiStimulus{}).Error; err != nil {
		return err
	}
	for index, value := range values {
		value.Jenis, value.Konten = strings.TrimSpace(value.Jenis), strings.TrimSpace(value.Konten)
		if value.Jenis == "image" {
			continue
		}
		if value.Konten == "" {
			continue
		}
		if value.Urutan <= 0 {
			value.Urutan = index + 1
		}
		if err := validateStimulus(value); err != nil {
			return err
		}
		if err := tx.Create(&SimulasiStimulus{SoalID: soalID, Jenis: value.Jenis, Konten: value.Konten, AltText: value.AltText, Urutan: value.Urutan}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) saveBuilderQuestion(c *fiber.Ctx, tx *gorm.DB, paket *SimulasiPaket, item simulasiBuilderItemInput) (SimulasiSoal, error) {
	if item.Soal == nil {
		if strings.TrimSpace(item.SoalID) == "" {
			return SimulasiSoal{}, fiber.NewError(400, "soal paket belum dipilih")
		}
		var row SimulasiSoal
		if err := tx.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&row, "id = ?", item.SoalID).Error; err != nil {
			return SimulasiSoal{}, fiber.NewError(404, "soal simulasi tidak ditemukan")
		}
		if err := s.simulasiSoalScope(c, &row, false); err != nil {
			return SimulasiSoal{}, err
		}
		return row, nil
	}

	var row SimulasiSoal
	copySource := false
	if strings.TrimSpace(item.SoalID) != "" {
		if err := tx.Preload("Stimulus").First(&row, "id = ?", item.SoalID).Error; err != nil {
			return SimulasiSoal{}, fiber.NewError(404, "soal simulasi tidak ditemukan")
		}
		if err := s.simulasiSoalScope(c, &row, false); err != nil {
			return SimulasiSoal{}, err
		}
		copySource = row.Status != "draf" || row.DibuatOlehUserID != c.Locals("userID")
	} else {
		row.DibuatOlehUserID = c.Locals("userID").(string)
	}
	if copySource {
		row = SimulasiSoal{DibuatOlehUserID: c.Locals("userID").(string)}
	}
	if err := applyBuilderQuestionInput(&row, *item.Soal, paket); err != nil {
		return SimulasiSoal{}, err
	}
	if row.ID == "" {
		if err := tx.Create(&row).Error; err != nil {
			return SimulasiSoal{}, err
		}
	} else if err := tx.Save(&row).Error; err != nil {
		return SimulasiSoal{}, err
	}
	if err := replaceBuilderStimulus(tx, row.ID, item.Soal.Stimulus); err != nil {
		return SimulasiSoal{}, err
	}
	if err := tx.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&row, "id = ?", row.ID).Error; err != nil {
		return SimulasiSoal{}, err
	}
	return row, nil
}

func (s *Server) builderResponse(c *fiber.Ctx, paket *SimulasiPaket) error {
	var items []SimulasiPaketSoal
	if err := s.db.Where("paket_id = ?", paket.ID).Order("urutan").Find(&items).Error; err != nil {
		return err
	}
	result := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		entry := fiber.Map{"id": item.ID, "soalId": item.SoalID, "bobot": item.Bobot, "urutan": item.Urutan}
		if item.SoalID != nil {
			var question SimulasiSoal
			if err := s.db.Preload("Mapel").Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&question, "id = ?", *item.SoalID).Error; err != nil {
				return err
			}
			entry["soal"] = staffQuestionResponse(question)
		}
		result = append(result, entry)
	}
	var assignments []SimulasiPenugasan
	if err := s.db.Where("paket_id = ?", paket.ID).Find(&assignments).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.PesertaDidikID)
	}
	return c.JSON(fiber.Map{"paket": paket, "items": result, "pesertaDidikIds": ids})
}

func (s *Server) saveBuilderPaket(c *fiber.Ctx, paket *SimulasiPaket, in simulasiBuilderInput, isNew bool) error {
	if paket.Revision <= 0 {
		paket.Revision = 1
	}
	if !isNew && in.Revision != paket.Revision {
		return fiber.NewError(409, "draf paket sudah berubah di perangkat lain; muat versi terbaru atau simpan sebagai salinan")
	}
	if err := applyBuilderPaketInput(paket, in.Paket); err != nil {
		return err
	}
	if !isNew {
		paket.Revision++
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if isNew {
			if err := tx.Create(paket).Error; err != nil {
				return err
			}
		} else if err := tx.Save(paket).Error; err != nil {
			return err
		}
		students, err := s.builderStudents(c, tx, in.PesertaDidikIDs)
		if err != nil {
			return err
		}
		if err := tx.Where("paket_id = ?", paket.ID).Delete(&SimulasiPaketSoal{}).Error; err != nil {
			return err
		}
		for index, input := range in.Items {
			question, err := s.saveBuilderQuestion(c, tx, paket, input)
			if err != nil {
				return err
			}
			snapshot, err := snapshotFromQuestion(question)
			if err != nil {
				return err
			}
			weight := input.Bobot
			if weight <= 0 {
				weight = question.Bobot
			}
			if err := tx.Create(&SimulasiPaketSoal{PaketID: paket.ID, SoalID: &question.ID, Urutan: index + 1, Bobot: weight, SnapshotJSON: snapshot}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("paket_id = ?", paket.ID).Delete(&SimulasiPenugasan{}).Error; err != nil {
			return err
		}
		for _, student := range students {
			if err := tx.Create(&SimulasiPenugasan{PaketID: paket.ID, PesertaDidikID: student.ID, KelasIDSaatTugas: student.KelasID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "save_builder", "simulasi_paket", paket.ID)
	return nil
}

func (s *Server) simulasiCreateBuilderPaket(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	var in simulasiBuilderInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "draf pembuat paket tidak valid")
	}
	paket := SimulasiPaket{DibuatOlehUserID: c.Locals("userID").(string), Status: "draf"}
	if err := s.saveBuilderPaket(c, &paket, in, true); err != nil {
		return err
	}
	return s.builderResponse(c, &paket)
}

func (s *Server) simulasiGetBuilderPaket(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	return s.builderResponse(c, paket)
}

func (s *Server) simulasiSaveBuilderPaket(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	var in simulasiBuilderInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "draf pembuat paket tidak valid")
	}
	if err := s.saveBuilderPaket(c, paket, in, false); err != nil {
		return err
	}
	return s.builderResponse(c, paket)
}
func (s *Server) simulasiListPaket(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	q := s.db.Preload("Mapel").Order("created_at desc")
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	var rows []SimulasiPaket
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}
func (s *Server) simulasiCreatePaket(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	var in simulasiPaketInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi paket tidak valid")
	}
	row := SimulasiPaket{DibuatOlehUserID: c.Locals("userID").(string), Status: "draf"}
	if err := applyPaketInput(&row, in); err != nil {
		return err
	}
	if err := s.db.Create(&row).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "simulasi_paket", row.ID)
	return c.Status(201).JSON(row)
}
func (s *Server) getSimulasiPaket(id string) (*SimulasiPaket, error) {
	var row SimulasiPaket
	if err := s.db.Preload("Mapel").First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
func (s *Server) simulasiGetPaket(c *fiber.Ctx) error {
	row, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, row, false); err != nil {
		return err
	}
	return c.JSON(row)
}
func (s *Server) paketCanChange(row *SimulasiPaket) error {
	if row.Status != "draf" {
		return fiber.NewError(409, "paket terbit/arsip tidak dapat diubah; duplikasi paket untuk membuat versi baru")
	}
	var attempts int64
	if err := s.db.Model(&SimulasiUpaya{}).Where("paket_id = ?", row.ID).Count(&attempts).Error; err != nil {
		return err
	}
	if attempts > 0 {
		return fiber.NewError(409, "paket yang sudah dikerjakan tidak dapat diubah")
	}
	return nil
}
func (s *Server) simulasiUpdatePaket(c *fiber.Ctx) error {
	row, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, row, true); err != nil {
		return err
	}
	if err := s.paketCanChange(row); err != nil {
		return err
	}
	var in simulasiPaketInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "isi paket tidak valid")
	}
	if err := applyPaketInput(row, in); err != nil {
		return err
	}
	if err := s.db.Save(row).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "simulasi_paket", row.ID)
	return c.JSON(row)
}
func (s *Server) simulasiDuplicatePaket(c *fiber.Ctx) error {
	source, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, source, true); err != nil {
		return err
	}
	copy := *source
	copy.ID = ""
	copy.CreatedAt, copy.UpdatedAt, copy.WaktuPublikasi = time.Time{}, time.Time{}, nil
	copy.Nama, copy.Status = source.Nama+" (Salinan)", "draf"
	copy.WaktuMulai, copy.WaktuSelesai = nil, nil
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&copy).Error; err != nil {
			return err
		}
		var items []SimulasiPaketSoal
		if err := tx.Where("paket_id = ?", source.ID).Order("urutan").Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			item.ID = ""
			item.PaketID = copy.ID
			item.CreatedAt, item.UpdatedAt = time.Time{}, time.Time{}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		var assignments []SimulasiPenugasan
		if err := tx.Where("paket_id = ?", source.ID).Find(&assignments).Error; err != nil {
			return err
		}
		for _, assignment := range assignments {
			assignment.ID = ""
			assignment.PaketID = copy.ID
			assignment.CreatedAt, assignment.UpdatedAt = time.Time{}, time.Time{}
			if err := tx.Create(&assignment).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "duplicate", "simulasi_paket", copy.ID)
	return c.Status(201).JSON(copy)
}
func (s *Server) simulasiArchivePaket(c *fiber.Ctx) error {
	row, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, row, true); err != nil {
		return err
	}
	row.Status = "arsip"
	if err := s.db.Save(row).Error; err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "archive", "simulasi_paket", row.ID)
	return c.JSON(row)
}

func snapshotFromQuestion(q SimulasiSoal) (string, error) {
	var cfg simulasiConfig
	if err := json.Unmarshal([]byte(q.Konfigurasi), &cfg); err != nil {
		return "", err
	}
	data, err := json.Marshal(simulasiSnapshot{SoalID: q.ID, Tipe: q.Tipe, Pertanyaan: q.Pertanyaan, Stimulus: q.Stimulus, Konfigurasi: cfg, Pembahasan: q.Pembahasan, Metadata: map[string]string{"domain": q.Domain, "topik": q.Topik, "kompetensi": q.Kompetensi}})
	return string(data), err
}
func (s *Server) refreshPaketSnapshots(tx *gorm.DB, paketID string) error {
	var items []SimulasiPaketSoal
	if err := tx.Where("paket_id = ?", paketID).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if item.SoalID == nil {
			continue
		}
		var question SimulasiSoal
		if err := tx.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&question, "id = ?", *item.SoalID).Error; err != nil {
			return fmt.Errorf("soal paket tidak tersedia")
		}
		snap, err := snapshotFromQuestion(question)
		if err != nil {
			return err
		}
		if err := tx.Model(&SimulasiPaketSoal{}).Where("id = ?", item.ID).Update("snapshot_json", snap).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Server) simulasiListPaketSoal(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	var items []SimulasiPaketSoal
	if err := s.db.Where("paket_id = ?", paket.ID).Order("urutan").Find(&items).Error; err != nil {
		return err
	}
	result := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		var snap simulasiSnapshot
		_ = json.Unmarshal([]byte(item.SnapshotJSON), &snap)
		result = append(result, fiber.Map{"id": item.ID, "soalId": item.SoalID, "urutan": item.Urutan, "bobot": item.Bobot, "soal": snap})
	}
	return c.JSON(result)
}
func (s *Server) simulasiAddPaketSoal(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	var in struct {
		SoalID string  `json:"soalId"`
		Bobot  float64 `json:"bobot"`
	}
	if err := c.BodyParser(&in); err != nil || in.SoalID == "" {
		return fiber.NewError(400, "soalId wajib diisi")
	}
	var question SimulasiSoal
	if err := s.db.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&question, "id = ?", in.SoalID).Error; err != nil {
		return fiber.NewError(404, "soal tidak ditemukan")
	}
	if err := s.simulasiSoalScope(c, &question, false); err != nil {
		return err
	}
	if question.Status != "terbit" {
		return fiber.NewError(400, "hanya soal terbit yang dapat dimasukkan ke paket")
	}
	snap, err := snapshotFromQuestion(question)
	if err != nil {
		return err
	}
	if in.Bobot <= 0 {
		in.Bobot = question.Bobot
	}
	var count int64
	s.db.Model(&SimulasiPaketSoal{}).Where("paket_id = ?", paket.ID).Count(&count)
	item := SimulasiPaketSoal{PaketID: paket.ID, SoalID: &question.ID, Urutan: int(count) + 1, Bobot: in.Bobot, SnapshotJSON: snap}
	if err := s.db.Create(&item).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "add", "simulasi_paket_soal", item.ID)
	return c.Status(201).JSON(item)
}
func (s *Server) simulasiAddPaketSoalAcak(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	var in struct {
		Count            int    `json:"count"`
		MapelID          string `json:"mapelId"`
		Mode             string `json:"mode"`
		Tipe             string `json:"tipe"`
		TingkatKesulitan string `json:"tingkatKesulitan"`
	}
	if err := c.BodyParser(&in); err != nil || in.Count < 1 || in.Count > 100 {
		return fiber.NewError(400, "jumlah soal acak harus 1-100")
	}
	q := s.db.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).Where("status = ?", "terbit")
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	if in.MapelID != "" {
		q = q.Where("mapel_id = ?", in.MapelID)
	}
	if in.Mode != "" {
		q = q.Where("mode = ?", in.Mode)
	}
	if in.Tipe != "" {
		q = q.Where("tipe = ?", in.Tipe)
	}
	if in.TingkatKesulitan != "" {
		q = q.Where("tingkat_kesulitan = ?", in.TingkatKesulitan)
	}
	var questions []SimulasiSoal
	if err := q.Order("RANDOM()").Limit(in.Count).Find(&questions).Error; err != nil {
		return err
	}
	if len(questions) < in.Count {
		return fiber.NewError(400, "soal sesuai filter tidak mencukupi")
	}
	var existing int64
	if err := s.db.Model(&SimulasiPaketSoal{}).Where("paket_id = ?", paket.ID).Count(&existing).Error; err != nil {
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for index, question := range questions {
			snap, err := snapshotFromQuestion(question)
			if err != nil {
				return err
			}
			item := SimulasiPaketSoal{PaketID: paket.ID, SoalID: &question.ID, Urutan: int(existing) + index + 1, Bobot: question.Bobot, SnapshotJSON: snap}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return c.Status(201).JSON(fiber.Map{"added": len(questions)})
}
func (s *Server) simulasiDeletePaketSoal(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	if err := s.db.Where("id = ? AND paket_id = ?", c.Params("itemId"), paket.ID).Delete(&SimulasiPaketSoal{}).Error; err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (s *Server) simulasiListPenugasan(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	var rows []SimulasiPenugasan
	if err := s.db.Where("paket_id = ?", paket.ID).Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}
func (s *Server) simulasiSetPenugasan(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	var in struct {
		PesertaDidikIDs []string `json:"pesertaDidikIds"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "data penugasan tidak valid")
	}
	unique := map[string]bool{}
	for _, id := range in.PesertaDidikIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		unique[id] = true
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	var students []PesertaDidik
	if len(ids) > 0 {
		if err := s.db.Where("id IN ? AND status = ?", ids, "aktif").Find(&students).Error; err != nil {
			return err
		}
		if len(students) != len(ids) {
			return fiber.NewError(400, "sebagian peserta didik tidak aktif atau tidak ditemukan")
		}
	}
	if c.Locals("role") == "guru" {
		for _, student := range students {
			if err := s.canManageKelas(c, student.KelasID); err != nil {
				return err
			}
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paket_id = ?", paket.ID).Delete(&SimulasiPenugasan{}).Error; err != nil {
			return err
		}
		for _, student := range students {
			if err := tx.Create(&SimulasiPenugasan{PaketID: paket.ID, PesertaDidikID: student.ID, KelasIDSaatTugas: student.KelasID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "assign", "simulasi_paket", paket.ID)
	return c.JSON(fiber.Map{"assigned": len(students)})
}
func (s *Server) simulasiPublishPaket(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, true); err != nil {
		return err
	}
	if err := s.paketCanChange(paket); err != nil {
		return err
	}
	var itemCount, assignmentCount int64
	if err := s.db.Model(&SimulasiPaketSoal{}).Where("paket_id = ?", paket.ID).Count(&itemCount).Error; err != nil {
		return err
	}
	if err := s.db.Model(&SimulasiPenugasan{}).Where("paket_id = ?", paket.ID).Count(&assignmentCount).Error; err != nil {
		return err
	}
	if itemCount == 0 || assignmentCount == 0 {
		return fiber.NewError(400, "paket harus memiliki soal dan peserta sebelum diterbitkan")
	}
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var items []SimulasiPaketSoal
		if err := tx.Where("paket_id = ?", paket.ID).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			if item.SoalID == nil {
				return fiber.NewError(400, "setiap item paket harus memiliki soal")
			}
			var question SimulasiSoal
			if err := tx.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).First(&question, "id = ?", *item.SoalID).Error; err != nil {
				return err
			}
			if err := validateBuilderQuestionForPublish(question); err != nil {
				return err
			}
			for _, stimulus := range question.Stimulus {
				if err := validateStimulus(simulasiStimulusIn{Jenis: stimulus.Jenis, Konten: stimulus.Konten, AltText: stimulus.AltText, Urutan: stimulus.Urutan}); err != nil {
					return err
				}
			}
			if question.Status == "draf" {
				if c.Locals("role") == "guru" && question.DibuatOlehUserID != c.Locals("userID") {
					return fiber.NewError(403, "soal draf hanya dapat diterbitkan oleh pembuatnya")
				}
				if err := tx.Model(&SimulasiSoal{}).Where("id = ? AND status = ?", question.ID, "draf").Update("status", "terbit").Error; err != nil {
					return err
				}
			}
		}
		if err := s.refreshPaketSnapshots(tx, paket.ID); err != nil {
			return err
		}
		result := tx.Model(&SimulasiPaket{}).Where("id = ? AND status = ?", paket.ID, "draf").Updates(map[string]interface{}{"status": "terbit", "waktu_publikasi": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fiber.NewError(409, "paket sudah diterbitkan atau diubah oleh pengguna lain")
		}
		return nil
	}); err != nil {
		return err
	}
	paket.Status, paket.WaktuPublikasi = "terbit", &now
	uid := c.Locals("userID").(string)
	s.audit(&uid, "publish", "simulasi_paket", paket.ID)
	return c.JSON(paket)
}

func (s *Server) simulasiSaya(c *fiber.Ctx) error {
	_, student, err := s.simulasiSiswa(c)
	if err != nil {
		return err
	}
	now := time.Now()
	var assignments []SimulasiPenugasan
	if err := s.db.Where("peserta_didik_id = ?", student.ID).Find(&assignments).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.PaketID)
	}
	if len(ids) == 0 {
		return c.JSON([]fiber.Map{})
	}
	var packages []SimulasiPaket
	if err := s.db.Preload("Mapel").Where("id IN ? AND status = ?", ids, "terbit").Order("waktu_mulai asc").Find(&packages).Error; err != nil {
		return err
	}
	result := make([]fiber.Map, 0, len(packages))
	for _, paket := range packages {
		available := (paket.WaktuMulai == nil || !now.Before(*paket.WaktuMulai)) && (paket.WaktuSelesai == nil || !now.After(*paket.WaktuSelesai))
		var attempts []SimulasiUpaya
		if err := s.db.Where("paket_id = ? AND peserta_didik_id = ?", paket.ID, student.ID).Order("nomor desc").Find(&attempts).Error; err != nil {
			return err
		}
		attemptResponses := make([]fiber.Map, 0, len(attempts))
		for _, attempt := range attempts {
			attemptResponses = append(attemptResponses, studentAttemptResponse(attempt, paket))
		}
		result = append(result, fiber.Map{"paket": studentPaketResponse(paket), "tersedia": available, "attempts": attemptResponses})
	}
	return c.JSON(result)
}
func (s *Server) assignedPaketForStudent(paketID, studentID string) (*SimulasiPaket, error) {
	var assignment SimulasiPenugasan
	if err := s.db.Where("paket_id = ? AND peserta_didik_id = ?", paketID, studentID).First(&assignment).Error; err != nil {
		return nil, fiber.NewError(403, "paket tidak ditugaskan kepada Anda")
	}
	paket, err := s.getSimulasiPaket(paketID)
	if err != nil || paket.Status != "terbit" {
		return nil, fiber.NewError(404, "paket simulasi tidak tersedia")
	}
	return paket, nil
}
func ensurePaketLive(paket *SimulasiPaket) error {
	now := time.Now()
	if paket.WaktuMulai != nil && now.Before(*paket.WaktuMulai) {
		return fiber.NewError(403, "simulasi belum dimulai")
	}
	if paket.WaktuSelesai != nil && now.After(*paket.WaktuSelesai) {
		return fiber.NewError(403, "simulasi sudah berakhir")
	}
	return nil
}
func (s *Server) simulasiInstruksi(c *fiber.Ctx) error {
	_, student, err := s.simulasiSiswa(c)
	if err != nil {
		return err
	}
	paket, err := s.assignedPaketForStudent(c.Params("id"), student.ID)
	if err != nil {
		return err
	}
	var count int64
	if err := s.db.Model(&SimulasiPaketSoal{}).Where("paket_id = ?", paket.ID).Count(&count).Error; err != nil {
		return err
	}
	var ongoing SimulasiUpaya
	if err := s.db.Where("paket_id = ? AND peserta_didik_id = ? AND status = ?", paket.ID, student.ID, "berlangsung").First(&ongoing).Error; err != nil && !errorsIsNotFound(err) {
		return err
	}
	return c.JSON(fiber.Map{"paket": studentPaketResponse(*paket), "jumlahSoal": count, "upayaBerlangsung": ongoing.ID != "", "upayaId": ongoing.ID})
}
func rankForSeed(seed, id string) string {
	sum := sha256.Sum256([]byte(seed + ":" + id))
	return fmt.Sprintf("%x", sum[:])
}
func (s *Server) simulasiMulai(c *fiber.Ctx) error {
	_, student, err := s.simulasiSiswa(c)
	if err != nil {
		return err
	}
	paket, err := s.assignedPaketForStudent(c.Params("id"), student.ID)
	if err != nil {
		return err
	}
	if err := ensurePaketLive(paket); err != nil {
		return err
	}
	var ongoing SimulasiUpaya
	if err := s.db.Where("paket_id = ? AND peserta_didik_id = ? AND status = ?", paket.ID, student.ID, "berlangsung").First(&ongoing).Error; err == nil {
		return c.JSON(studentAttemptResponse(ongoing, *paket))
	}
	var count int64
	if err := s.db.Model(&SimulasiUpaya{}).Where("paket_id = ? AND peserta_didik_id = ?", paket.ID, student.ID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= paket.MaksPercobaan {
		return fiber.NewError(403, "batas percobaan sudah tercapai")
	}
	now := time.Now()
	deadline := now.Add(time.Duration(paket.DurasiMenit) * time.Minute)
	seed := uuid.NewString()
	attempt := SimulasiUpaya{PaketID: paket.ID, PesertaDidikID: student.ID, Nomor: int(count) + 1, Status: "berlangsung", Mulai: &now, BatasWaktu: &deadline, SeedUrutan: seed}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		var items []SimulasiPaketSoal
		if err := tx.Where("paket_id = ?", paket.ID).Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return fiber.NewError(400, "paket belum memiliki soal")
		}
		if paket.AcakUrutan {
			sort.Slice(items, func(i, j int) bool { return rankForSeed(seed, items[i].ID) < rankForSeed(seed, items[j].ID) })
		} else {
			sort.Slice(items, func(i, j int) bool { return items[i].Urutan < items[j].Urutan })
		}
		for index, item := range items {
			if err := tx.Create(&SimulasiUpayaSoal{UpayaID: attempt.ID, PaketSoalID: item.ID, UrutanTampil: index + 1}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if isUniqueErr(err) {
			if e := s.db.Where("paket_id = ? AND peserta_didik_id = ? AND status = ?", paket.ID, student.ID, "berlangsung").First(&ongoing).Error; e == nil {
				return c.JSON(ongoing)
			}
		}
		return err
	}
	return c.Status(201).JSON(studentAttemptResponse(attempt, *paket))
}
func sanitizedConfig(tipe string, config simulasiConfig) interface{} {
	switch tipe {
	case simulasiTipePG, simulasiTipePGK:
		return fiber.Map{"choices": config.Choices}
	case simulasiTipeBenarSalah:
		statements := make([]fiber.Map, 0, len(config.Statements))
		for _, row := range config.Statements {
			statements = append(statements, fiber.Map{"id": row.ID, "text": row.Text})
		}
		return fiber.Map{"statements": statements}
	case simulasiTipeMenjodohkan:
		return fiber.Map{"left": config.Left, "right": config.Right}
	case simulasiTipeUraian:
		// Rubrics are staff-only grading guidance. Returning them to students
		// would reveal the assessment key and undermine the integrity of the
		// response workflow.
		return fiber.Map{}
	default:
		return fiber.Map{}
	}
}

// studentPaketResponse deliberately exposes only fields needed to render the
// student portal. In particular, ownership and revision metadata never leave
// the staff API surface.
func studentPaketResponse(p SimulasiPaket) fiber.Map {
	return fiber.Map{
		"id": p.ID, "nama": p.Nama, "deskripsi": p.Deskripsi, "mode": p.Mode,
		"jenjang": p.Jenjang, "durasiMenit": p.DurasiMenit, "instruksi": p.Instruksi,
		"nilaiLulus": p.NilaiLulus, "waktuMulai": p.WaktuMulai, "waktuSelesai": p.WaktuSelesai,
		"maksPercobaan": p.MaksPercobaan, "acakUrutan": p.AcakUrutan,
		"tampilkanNilai": p.TampilkanNilai, "tampilkanRingkasan": p.TampilkanRingkasan,
		"tampilkanPembahasan": p.TampilkanPembahasan, "status": p.Status,
	}
}

func studentAttemptResponse(a SimulasiUpaya, p SimulasiPaket) fiber.Map {
	result := fiber.Map{
		"id": a.ID, "nomor": a.Nomor, "status": a.Status,
		"mulai": a.Mulai, "batasWaktu": a.BatasWaktu, "selesai": a.Selesai,
	}
	if p.TampilkanNilai && a.Status != "menunggu_nilai" {
		if a.SkorAkhir != nil {
			result["skor"] = *a.SkorAkhir
		} else {
			result["skor"] = a.SkorOtomatis
		}
	}
	return result
}
func (s *Server) ownedUpaya(c *fiber.Ctx) (*SimulasiUpaya, *SimulasiPaket, *PesertaDidik, error) {
	_, student, err := s.simulasiSiswa(c)
	if err != nil {
		return nil, nil, nil, err
	}
	var attempt SimulasiUpaya
	if err := s.db.First(&attempt, "id = ? AND peserta_didik_id = ?", c.Params("id"), student.ID).Error; err != nil {
		return nil, nil, nil, fiber.NewError(404, "upaya tidak ditemukan")
	}
	paket, err := s.assignedPaketForStudent(attempt.PaketID, student.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	return &attempt, paket, student, nil
}
func (s *Server) simulasiWorkspace(c *fiber.Ctx) error {
	attempt, paket, _, err := s.ownedUpaya(c)
	if err != nil {
		return err
	}
	if attempt.Status == "berlangsung" && attempt.BatasWaktu != nil && time.Now().After(*attempt.BatasWaktu) {
		_, _ = s.finishSimulasiAttempt(attempt.ID, "kedaluwarsa")
		_ = s.db.First(attempt, "id = ?", attempt.ID)
	}
	var links []SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ?", attempt.ID).Order("urutan_tampil").Find(&links).Error; err != nil {
		return err
	}
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.PaketSoalID)
	}
	var items []SimulasiPaketSoal
	if len(ids) > 0 {
		if err := s.db.Where("id IN ?", ids).Find(&items).Error; err != nil {
			return err
		}
	}
	byID := map[string]SimulasiPaketSoal{}
	for _, item := range items {
		byID[item.ID] = item
	}
	var answers []SimulasiJawaban
	if len(links) > 0 {
		if err := s.db.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
			return err
		}
	}
	answerByLink := map[string]SimulasiJawaban{}
	for _, answer := range answers {
		answerByLink[answer.UpayaSoalID] = answer
	}
	questions := make([]fiber.Map, 0, len(links))
	for _, link := range links {
		item := byID[link.PaketSoalID]
		if item.ID == "" || strings.TrimSpace(item.SnapshotJSON) == "" {
			return fiber.NewError(500, "snapshot soal upaya tidak ditemukan")
		}
		var snap simulasiSnapshot
		if err := json.Unmarshal([]byte(item.SnapshotJSON), &snap); err != nil {
			return fiber.NewError(500, "snapshot soal upaya tidak valid")
		}
		answer := answerByLink[link.ID]
		// json.RawMessage("") is invalid JSON and makes Fiber fail while encoding
		// an untouched workspace. Explicit null keeps autosave/resume payloads
		// stable before the first answer exists.
		jawaban := json.RawMessage("null")
		if strings.TrimSpace(answer.JawabanJSON) != "" {
			jawaban = json.RawMessage(answer.JawabanJSON)
		}
		questions = append(questions, fiber.Map{"upayaSoalId": link.ID, "urutan": link.UrutanTampil, "ditandai": link.Ditandai, "bobot": item.Bobot, "soal": fiber.Map{"tipe": snap.Tipe, "pertanyaan": snap.Pertanyaan, "stimulus": snap.Stimulus, "konfigurasi": sanitizedConfig(snap.Tipe, snap.Konfigurasi)}, "jawaban": jawaban})
	}
	remaining := 0
	if attempt.BatasWaktu != nil {
		remaining = int(time.Until(*attempt.BatasWaktu).Seconds())
		if remaining < 0 {
			remaining = 0
		}
	}
	return c.JSON(fiber.Map{"upaya": studentAttemptResponse(*attempt, *paket), "paket": studentPaketResponse(*paket), "sisaDetik": remaining, "soal": questions})
}
func linkIDs(links []SimulasiUpayaSoal) []string {
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.ID)
	}
	return ids
}
func (s *Server) validateStudentAnswer(tx *gorm.DB, link SimulasiUpayaSoal, raw json.RawMessage) error {
	var item SimulasiPaketSoal
	if err := tx.First(&item, "id = ?", link.PaketSoalID).Error; err != nil {
		return err
	}
	var snap simulasiSnapshot
	if err := json.Unmarshal([]byte(item.SnapshotJSON), &snap); err != nil {
		return err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	switch snap.Tipe {
	case simulasiTipePG:
		var value string
		if json.Unmarshal(raw, &value) != nil || !containsString(choiceIDs(snap.Konfigurasi.Choices), value) {
			return fiber.NewError(400, "jawaban PG tidak valid")
		}
	case simulasiTipePGK:
		var values []string
		if json.Unmarshal(raw, &values) != nil {
			return fiber.NewError(400, "jawaban PG kompleks tidak valid")
		}
		for _, v := range values {
			if !containsString(choiceIDs(snap.Konfigurasi.Choices), v) {
				return fiber.NewError(400, "pilihan tidak valid")
			}
		}
	case simulasiTipeBenarSalah:
		var values map[string]bool
		if json.Unmarshal(raw, &values) != nil {
			return fiber.NewError(400, "jawaban benar/salah tidak valid")
		}
		for id := range values {
			found := false
			for _, statement := range snap.Konfigurasi.Statements {
				if statement.ID == id {
					found = true
				}
			}
			if !found {
				return fiber.NewError(400, "pernyataan tidak valid")
			}
		}
	case simulasiTipeMenjodohkan:
		var values map[string]string
		if json.Unmarshal(raw, &values) != nil {
			return fiber.NewError(400, "jawaban menjodohkan tidak valid")
		}
		for left, right := range values {
			if !containsString(choiceIDs(snap.Konfigurasi.Left), left) || !containsString(choiceIDs(snap.Konfigurasi.Right), right) {
				return fiber.NewError(400, "pasangan tidak valid")
			}
		}
	case simulasiTipeIsian, simulasiTipeUraian:
		var value string
		if json.Unmarshal(raw, &value) != nil || len([]byte(value)) > 64*1024 {
			return fiber.NewError(400, "jawaban teks tidak valid")
		}
	}
	return nil
}
func (s *Server) simulasiSimpanJawaban(c *fiber.Ctx) error {
	attempt, _, _, err := s.ownedUpaya(c)
	if err != nil {
		return err
	}
	if attempt.Status != "berlangsung" {
		return fiber.NewError(409, "upaya sudah tidak dapat diubah")
	}
	if attempt.BatasWaktu != nil && time.Now().After(*attempt.BatasWaktu) {
		_, _ = s.finishSimulasiAttempt(attempt.ID, "kedaluwarsa")
		return fiber.NewError(409, "waktu simulasi sudah habis")
	}
	var in struct {
		Jawaban json.RawMessage `json:"jawaban"`
	}
	if err := c.BodyParser(&in); err != nil || len(in.Jawaban) > 64*1024 {
		return fiber.NewError(400, "jawaban tidak valid")
	}
	var link SimulasiUpayaSoal
	if err := s.db.Where("id = ? AND upaya_id = ?", c.Params("upayaSoalId"), attempt.ID).First(&link).Error; err != nil {
		return fiber.NewError(404, "soal upaya tidak ditemukan")
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.validateStudentAnswer(tx, link, in.Jawaban); err != nil {
			return err
		}
		var answer SimulasiJawaban
		e := tx.Where("upaya_soal_id = ?", link.ID).First(&answer).Error
		if e == nil {
			return tx.Model(&answer).Update("jawaban_json", string(in.Jawaban)).Error
		}
		if !errorsIsNotFound(e) {
			return e
		}
		return tx.Create(&SimulasiJawaban{UpayaSoalID: link.ID, JawabanJSON: string(in.Jawaban)}).Error
	}); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"status": "tersimpan", "savedAt": time.Now()})
}
func errorsIsNotFound(err error) bool { return err != nil && err == gorm.ErrRecordNotFound }
func (s *Server) simulasiTandai(c *fiber.Ctx) error {
	attempt, _, _, err := s.ownedUpaya(c)
	if err != nil {
		return err
	}
	if attempt.Status != "berlangsung" {
		return fiber.NewError(409, "upaya sudah tidak dapat diubah")
	}
	if attempt.BatasWaktu != nil && time.Now().After(*attempt.BatasWaktu) {
		_, _ = s.finishSimulasiAttempt(attempt.ID, "kedaluwarsa")
		return fiber.NewError(409, "waktu simulasi sudah habis")
	}
	var in struct {
		Ditandai bool `json:"ditandai"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "status tanda tidak valid")
	}
	result := s.db.Model(&SimulasiUpayaSoal{}).Where("id = ? AND upaya_id = ?", c.Params("upayaSoalId"), attempt.ID).Update("ditandai", in.Ditandai)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiber.NewError(404, "soal upaya tidak ditemukan")
	}
	return c.JSON(fiber.Map{"ditandai": in.Ditandai})
}
func normalizeShortAnswer(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.Join(strings.Fields(value), " ")
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '.' {
			return r
		}
		return -1
	}, value)
}
func gradeSnapshot(snap simulasiSnapshot, raw string, weight float64) (bool, float64, bool) {
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return false, 0, snap.Tipe == simulasiTipeUraian
	}
	switch snap.Tipe {
	case simulasiTipePG:
		var answer string
		if json.Unmarshal([]byte(raw), &answer) != nil {
			return false, 0, false
		}
		correct := len(snap.Konfigurasi.CorrectIDs) == 1 && answer == snap.Konfigurasi.CorrectIDs[0]
		return correct, boolScore(correct, weight), false
	case simulasiTipePGK:
		var answer []string
		_ = json.Unmarshal([]byte(raw), &answer)
		sort.Strings(answer)
		expected := append([]string(nil), snap.Konfigurasi.CorrectIDs...)
		sort.Strings(expected)
		correct := strings.Join(answer, "|") == strings.Join(expected, "|")
		return correct, boolScore(correct, weight), false
	case simulasiTipeBenarSalah:
		var answer map[string]bool
		_ = json.Unmarshal([]byte(raw), &answer)
		correct := len(answer) == len(snap.Konfigurasi.Statements)
		for _, row := range snap.Konfigurasi.Statements {
			if answer[row.ID] != row.Correct {
				correct = false
			}
		}
		return correct, boolScore(correct, weight), false
	case simulasiTipeMenjodohkan:
		var answer map[string]string
		_ = json.Unmarshal([]byte(raw), &answer)
		correct := len(answer) == len(snap.Konfigurasi.Pairs)
		for left, right := range snap.Konfigurasi.Pairs {
			if answer[left] != right {
				correct = false
			}
		}
		return correct, boolScore(correct, weight), false
	case simulasiTipeIsian:
		var answer string
		_ = json.Unmarshal([]byte(raw), &answer)
		normalized := normalizeShortAnswer(answer)
		correct := false
		for _, expected := range snap.Konfigurasi.AcceptedAnswers {
			if normalized == normalizeShortAnswer(expected) {
				correct = true
			}
		}
		return correct, boolScore(correct, weight), false
	case simulasiTipeUraian:
		return false, 0, true
	}
	return false, 0, false
}
func boolScore(correct bool, weight float64) float64 {
	if correct {
		return weight
	}
	return 0
}
func (s *Server) finishSimulasiAttempt(attemptID, reason string) (*SimulasiUpaya, error) {
	var attempt SimulasiUpaya
	if err := s.db.First(&attempt, "id = ?", attemptID).Error; err != nil {
		return nil, err
	}
	if attempt.Status != "berlangsung" {
		return &attempt, nil
	}
	var paket SimulasiPaket
	if err := s.db.First(&paket, "id = ?", attempt.PaketID).Error; err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var links []SimulasiUpayaSoal
		if err := tx.Where("upaya_id = ?", attempt.ID).Find(&links).Error; err != nil {
			return err
		}
		var itemIDs []string
		for _, link := range links {
			itemIDs = append(itemIDs, link.PaketSoalID)
		}
		var items []SimulasiPaketSoal
		if err := tx.Where("id IN ?", itemIDs).Find(&items).Error; err != nil {
			return err
		}
		byID := map[string]SimulasiPaketSoal{}
		for _, item := range items {
			byID[item.ID] = item
		}
		var answers []SimulasiJawaban
		if len(links) > 0 {
			if err := tx.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
				return err
			}
		}
		byLink := map[string]SimulasiJawaban{}
		for _, answer := range answers {
			byLink[answer.UpayaSoalID] = answer
		}
		total, earned := 0.0, 0.0
		pending := false
		for _, link := range links {
			item, itemExists := byID[link.PaketSoalID]
			if !itemExists || strings.TrimSpace(item.SnapshotJSON) == "" {
				return fiber.NewError(500, "snapshot soal upaya tidak ditemukan")
			}
			total += item.Bobot
			var snap simulasiSnapshot
			if err := json.Unmarshal([]byte(item.SnapshotJSON), &snap); err != nil {
				return err
			}
			answer, ok := byLink[link.ID]
			correct, score, manual := gradeSnapshot(snap, answer.JawabanJSON, item.Bobot)
			if manual {
				pending = true
			}
			if !ok {
				answer = SimulasiJawaban{UpayaSoalID: link.ID, JawabanJSON: ""}
			}
			answer.Benar = &correct
			answer.SkorOtomatis = score
			answer.SkorAkhir = score
			earned += score
			if ok {
				if err := tx.Save(&answer).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Create(&answer).Error; err != nil {
					return err
				}
			}
		}
		now := time.Now()
		status := "selesai"
		if pending {
			status = "menunggu_nilai"
		}
		if reason == "kedaluwarsa" {
			status = "kedaluwarsa"
			if pending {
				status = "menunggu_nilai"
			}
		}
		score := 0.0
		if total > 0 {
			score = earned / total * 100
		}
		res := tx.Model(&SimulasiUpaya{}).Where("id = ? AND status = ?", attempt.ID, "berlangsung").Updates(map[string]interface{}{"status": status, "selesai": now, "skor_otomatis": score})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		attempt.Status, attempt.Selesai, attempt.SkorOtomatis = status, &now, score
		return nil
	})
	return &attempt, err
}
func (s *Server) simulasiKirim(c *fiber.Ctx) error {
	attempt, paket, _, err := s.ownedUpaya(c)
	if err != nil {
		return err
	}
	finished, err := s.finishSimulasiAttempt(attempt.ID, "dikirim")
	if err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "submit", "simulasi_upaya", attempt.ID)
	return c.JSON(studentAttemptResponse(*finished, *paket))
}
func (s *Server) autoFinishSimulasiSessions() {
	var attempts []SimulasiUpaya
	if err := s.db.Where("status = ? AND batas_waktu < ?", "berlangsung", time.Now()).Find(&attempts).Error; err != nil {
		return
	}
	for _, attempt := range attempts {
		_, _ = s.finishSimulasiAttempt(attempt.ID, "kedaluwarsa")
	}
}

func (s *Server) simulasiHasilSiswa(c *fiber.Ctx) error {
	attempt, paket, _, err := s.ownedUpaya(c)
	if err != nil {
		return err
	}
	if attempt.Status == "berlangsung" {
		return fiber.NewError(409, "simulasi belum dikirim")
	}
	result := fiber.Map{"status": attempt.Status, "nomor": attempt.Nomor, "selesai": attempt.Selesai}
	if paket.TampilkanNilai && attempt.Status != "menunggu_nilai" {
		result["skor"] = attempt.SkorAkhir
		if attempt.SkorAkhir == nil {
			result["skor"] = attempt.SkorOtomatis
		}
	}
	if paket.TampilkanRingkasan {
		var links []SimulasiUpayaSoal
		if err := s.db.Where("upaya_id = ?", attempt.ID).Find(&links).Error; err != nil {
			return err
		}
		var answers []SimulasiJawaban
		if len(links) > 0 {
			if err := s.db.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
				return err
			}
		}
		benar, salah, kosong := 0, 0, 0
		for _, answer := range answers {
			if strings.TrimSpace(answer.JawabanJSON) == "" {
				kosong++
			} else if answer.Benar != nil && *answer.Benar {
				benar++
			} else {
				salah++
			}
		}
		result["ringkasan"] = fiber.Map{"benar": benar, "salah": salah, "kosong": kosong}
	}
	if paket.TampilkanPembahasan {
		var links []SimulasiUpayaSoal
		if err := s.db.Where("upaya_id = ?", attempt.ID).Order("urutan_tampil").Find(&links).Error; err != nil {
			return err
		}
		reviews := make([]fiber.Map, 0, len(links))
		for _, link := range links {
			var item SimulasiPaketSoal
			if err := s.db.First(&item, "id = ?", link.PaketSoalID).Error; err != nil {
				return err
			}
			var snapshot simulasiSnapshot
			if err := json.Unmarshal([]byte(item.SnapshotJSON), &snapshot); err != nil {
				return err
			}
			// The review intentionally contains no CorrectIDs, accepted answers,
			// truth values, or matching pairs. Keys remain staff-only even when a
			// teacher opts in to publish an explanatory note.
			reviews = append(reviews, fiber.Map{"urutan": link.UrutanTampil, "pertanyaan": snapshot.Pertanyaan, "pembahasan": snapshot.Pembahasan})
		}
		result["pembahasan"] = reviews
	}
	return c.JSON(result)
}

func (s *Server) simulasiStaffHasil(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	q := s.db.Preload("PesertaDidik").Where("simulasi_upayas.paket_id = ?", paket.ID).Order("simulasi_upayas.mulai desc")
	if status := c.Query("status"); status != "" {
		q = q.Where("simulasi_upayas.status = ?", status)
	}
	if siswa := c.Query("pesertaDidikId"); siswa != "" {
		q = q.Where("simulasi_upayas.peserta_didik_id = ?", siswa)
	}
	if kelas := c.Query("kelasId"); kelas != "" {
		q = q.Joins("JOIN peserta_didiks ON peserta_didiks.id = simulasi_upayas.peserta_didik_id").Where("peserta_didiks.kelas_id = ?", kelas)
	}
	if from := c.Query("dari"); from != "" {
		when, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return fiber.NewError(400, "parameter dari harus ISO-8601")
		}
		q = q.Where("simulasi_upayas.mulai >= ?", when)
	}
	if until := c.Query("sampai"); until != "" {
		when, err := time.Parse(time.RFC3339, until)
		if err != nil {
			return fiber.NewError(400, "parameter sampai harus ISO-8601")
		}
		q = q.Where("simulasi_upayas.mulai <= ?", when)
	}
	var rows []SimulasiUpaya
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}

// simulasiDetailUpaya is intentionally staff-only. It returns the frozen
// snapshot (including key/rubric) alongside each persisted answer for review
// and manual essay grading; student workspace responses never call this path.
func (s *Server) simulasiDetailUpaya(c *fiber.Ctx) error {
	var attempt SimulasiUpaya
	if err := s.db.Preload("Paket").Preload("PesertaDidik").First(&attempt, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "upaya tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, &attempt.Paket, false); err != nil {
		return err
	}
	var links []SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ?", attempt.ID).Order("urutan_tampil").Find(&links).Error; err != nil {
		return err
	}
	var answers []SimulasiJawaban
	if len(links) > 0 {
		if err := s.db.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
			return err
		}
	}
	answerByLink := map[string]SimulasiJawaban{}
	for _, answer := range answers {
		answerByLink[answer.UpayaSoalID] = answer
	}
	items := make([]fiber.Map, 0, len(links))
	for _, link := range links {
		var paketSoal SimulasiPaketSoal
		if err := s.db.First(&paketSoal, "id = ?", link.PaketSoalID).Error; err != nil {
			return err
		}
		var snapshot simulasiSnapshot
		if err := json.Unmarshal([]byte(paketSoal.SnapshotJSON), &snapshot); err != nil {
			return err
		}
		items = append(items, fiber.Map{"upayaSoalId": link.ID, "urutan": link.UrutanTampil, "ditandai": link.Ditandai, "bobot": paketSoal.Bobot, "soal": snapshot, "jawaban": answerByLink[link.ID]})
	}
	return c.JSON(fiber.Map{"upaya": attempt, "pesertaDidik": attempt.PesertaDidik, "items": items})
}
func (s *Server) simulasiAnalisis(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	var items []SimulasiPaketSoal
	if err := s.db.Where("paket_id = ?", paket.ID).Order("urutan").Find(&items).Error; err != nil {
		return err
	}
	result := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		var links []SimulasiUpayaSoal
		if err := s.db.Joins("JOIN simulasi_upayas ON simulasi_upayas.id = simulasi_upaya_soals.upaya_id").Where("simulasi_upayas.paket_id = ? AND simulasi_upaya_soals.paket_soal_id = ?", paket.ID, item.ID).Find(&links).Error; err != nil {
			return err
		}
		var answers []SimulasiJawaban
		if len(links) > 0 {
			if err := s.db.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
				return err
			}
		}
		benar, salah, kosong := 0, 0, 0
		for _, answer := range answers {
			if strings.TrimSpace(answer.JawabanJSON) == "" {
				kosong++
			} else if answer.Benar != nil && *answer.Benar {
				benar++
			} else {
				salah++
			}
		}
		success := 0.0
		if len(answers) > 0 {
			success = float64(benar) / float64(len(answers)) * 100
		}
		var snap simulasiSnapshot
		if err := json.Unmarshal([]byte(item.SnapshotJSON), &snap); err != nil {
			return fiber.NewError(500, "snapshot soal tidak valid")
		}
		result = append(result, fiber.Map{"paketSoalId": item.ID, "urutan": item.Urutan, "pertanyaan": snap.Pertanyaan, "benar": benar, "salah": salah, "kosong": kosong, "tingkatKeberhasilan": success})
	}
	return c.JSON(result)
}
func (s *Server) simulasiNilaiUraian(c *fiber.Ctx) error {
	var attempt SimulasiUpaya
	if err := s.db.Preload("Paket").First(&attempt, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "upaya tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, &attempt.Paket, true); err != nil {
		return err
	}
	if attempt.Status != "menunggu_nilai" && attempt.Status != "selesai" {
		return fiber.NewError(409, "upaya belum siap untuk penilaian uraian")
	}
	var answer SimulasiJawaban
	if err := s.db.First(&answer, "id = ?", c.Params("jawabanId")).Error; err != nil {
		return fiber.NewError(404, "jawaban tidak ditemukan")
	}
	var in struct {
		Skor     float64 `json:"skor"`
		Komentar string  `json:"komentar"`
	}
	if err := c.BodyParser(&in); err != nil || in.Skor < 0 {
		return fiber.NewError(400, "nilai uraian tidak valid")
	}
	var link SimulasiUpayaSoal
	if err := s.db.First(&link, "id = ? AND upaya_id = ?", answer.UpayaSoalID, attempt.ID).Error; err != nil {
		return fiber.NewError(404, "jawaban tidak terkait upaya")
	}
	var item SimulasiPaketSoal
	if err := s.db.First(&item, "id = ?", link.PaketSoalID).Error; err != nil {
		return fiber.NewError(404, "soal paket tidak ditemukan")
	}
	var snapshot simulasiSnapshot
	if err := json.Unmarshal([]byte(item.SnapshotJSON), &snapshot); err != nil {
		return fiber.NewError(500, "snapshot soal tidak valid")
	}
	if snapshot.Tipe != simulasiTipeUraian {
		return fiber.NewError(400, "jawaban ini bukan soal uraian")
	}
	if in.Skor > item.Bobot {
		return fiber.NewError(400, "nilai melebihi bobot soal")
	}
	now := time.Now()
	uid := c.Locals("userID").(string)
	answer.SkorManual, answer.SkorAkhir, answer.KomentarGuru, answer.DinilaiOlehUserID, answer.DinilaiPada = &in.Skor, in.Skor, strings.TrimSpace(in.Komentar), &uid, &now
	if err := s.db.Save(&answer).Error; err != nil {
		return err
	}
	if err := s.refreshFinalScore(&attempt); err != nil {
		return err
	}
	s.audit(&uid, "grade", "simulasi_jawaban", answer.ID)
	return c.JSON(answer)
}
func (s *Server) refreshFinalScore(attempt *SimulasiUpaya) error {
	var links []SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ?", attempt.ID).Find(&links).Error; err != nil {
		return err
	}
	var answers []SimulasiJawaban
	if len(links) > 0 {
		if err := s.db.Where("upaya_soal_id IN ?", linkIDs(links)).Find(&answers).Error; err != nil {
			return err
		}
	}
	byLink := map[string]SimulasiJawaban{}
	for _, answer := range answers {
		byLink[answer.UpayaSoalID] = answer
	}
	total, earned := 0.0, 0.0
	pending := false
	for _, link := range links {
		var item SimulasiPaketSoal
		if err := s.db.First(&item, "id = ?", link.PaketSoalID).Error; err != nil {
			return err
		}
		total += item.Bobot
		var snap simulasiSnapshot
		if err := json.Unmarshal([]byte(item.SnapshotJSON), &snap); err != nil {
			return err
		}
		answer := byLink[link.ID]
		if snap.Tipe == simulasiTipeUraian {
			if answer.SkorManual == nil {
				pending = true
			} else {
				earned += *answer.SkorManual
			}
		} else {
			earned += answer.SkorAkhir
		}
	}
	if pending {
		return nil
	}
	score := 0.0
	if total > 0 {
		score = earned / total * 100
	}
	return s.db.Model(&SimulasiUpaya{}).Where("id = ?", attempt.ID).Updates(map[string]interface{}{"status": "selesai", "skor_akhir": score}).Error
}

func (s *Server) simulasiExportHasil(c *fiber.Ctx) error {
	paket, err := s.getSimulasiPaket(c.Params("id"))
	if err != nil {
		return fiber.NewError(404, "paket simulasi tidak ditemukan")
	}
	if err := s.simulasiPaketScope(c, paket, false); err != nil {
		return err
	}
	var rows []SimulasiUpaya
	if err := s.db.Preload("PesertaDidik").Where("paket_id = ?", paket.ID).Order("created_at").Find(&rows).Error; err != nil {
		return err
	}
	headers := []string{"Nama", "NISN", "Status", "Mulai", "Selesai", "Skor"}
	values := make([][]string, 0, len(rows))
	for _, row := range rows {
		score := ""
		if row.SkorAkhir != nil {
			score = strconv.FormatFloat(*row.SkorAkhir, 'f', 2, 64)
		} else if row.Status != "menunggu_nilai" {
			score = strconv.FormatFloat(row.SkorOtomatis, 'f', 2, 64)
		}
		values = append(values, []string{row.PesertaDidik.Nama, row.PesertaDidik.NISN, row.Status, wibTimeFormatPtr(row.Mulai), wibTimeFormatPtr(row.Selesai), score})
	}
	if c.Query("format") == "xlsx" {
		book := excelize.NewFile()
		defer book.Close()
		book.SetSheetName("Sheet1", "Hasil")
		for column, value := range headers {
			cell, _ := excelize.CoordinatesToCellName(column+1, 1)
			book.SetCellValue("Hasil", cell, value)
		}
		for rowIndex, valuesRow := range values {
			for column, value := range valuesRow {
				cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+2)
				book.SetCellValue("Hasil", cell, value)
			}
		}
		book.SetColWidth("Hasil", "A", "F", 22)
		c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Attachment("hasil-simulasi-" + paket.ID + ".xlsx")
		return book.Write(c.Response().BodyWriter())
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Attachment("hasil-simulasi-" + paket.ID + ".csv")
	writer := csv.NewWriter(c.Response().BodyWriter())
	_ = writer.Write(headers)
	for _, value := range values {
		_ = writer.Write(value)
	}
	writer.Flush()
	return writer.Error()
}
func wibTimeFormatPtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return wibTimeFormat(*value, "02-01-2006 15:04")
}

// seedSimulasiSamples provides small, original development fixtures. The text
// and questions are authored for this application; they do not copy official
// test material, government assets, or a government visual identity.
func (s *Server) seedSimulasiSamples() error {
	const literasiName = "Contoh Simulasi Literasi Membaca SD"
	var existing int64
	if err := s.db.Model(&SimulasiPaket{}).Where("nama = ?", literasiName).Count(&existing).Error; err != nil || existing > 0 {
		return err
	}
	var owner User
	if err := s.db.Where("role = ? AND is_active = ?", "admin", true).Order("created_at").First(&owner).Error; err != nil {
		return nil // A minimal test/development database may have no account yet.
	}
	var students []PesertaDidik
	if err := s.db.Where("status = ?", "aktif").Order("created_at").Limit(12).Find(&students).Error; err != nil || len(students) == 0 {
		return err
	}
	now := time.Now()
	makeQuestion := func(tipe, prompt, domain, topik string, cfg simulasiConfig, stimulus string) SimulasiSoal {
		payload, _ := json.Marshal(cfg)
		return SimulasiSoal{Jenjang: "SD/MI", KelasFase: "Kelas 5-6 / Fase C", Mode: "anbk_akm", Domain: domain, Topik: topik, Kompetensi: "Memahami informasi dan bernalar", LevelKognitif: "Memahami", TingkatKesulitan: "sedang", Tags: "contoh,orisinil", Tipe: tipe, Pertanyaan: prompt, Konfigurasi: string(payload), Pembahasan: "Pembahasan internal contoh untuk guru.", Bobot: 1, Status: "terbit", DibuatOlehUserID: owner.ID, Stimulus: []SimulasiStimulus{{Jenis: "text", Konten: stimulus, Urutan: 1}}}
	}
	literasi := []SimulasiSoal{
		makeQuestion(simulasiTipePG, "Mengapa Rani datang lebih awal ke perpustakaan?", "Literasi membaca", "Informasi tersurat", simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "Menukar buku"}, {ID: "b", Text: "Membantu menata buku"}, {ID: "c", Text: "Membeli alat tulis"}}, CorrectIDs: []string{"b"}}, "Pada Jumat pagi, Rani datang ke perpustakaan sekolah sebelum bel berbunyi. Ia membantu Bu Sari menata buku cerita yang baru kembali. Setelah itu, Rani memilih satu buku tentang kebun sekolah."),
		makeQuestion(simulasiTipeBenarSalah, "Tentukan benar atau salah berdasarkan teks stimulus.", "Literasi membaca", "Mengevaluasi informasi", simulasiConfig{Statements: []simulasiStatement{{ID: "p1", Text: "Rani datang sebelum bel berbunyi.", Correct: true}, {ID: "p2", Text: "Rani memilih buku tentang olahraga.", Correct: false}}}, "Pada Jumat pagi, Rani datang ke perpustakaan sekolah sebelum bel berbunyi. Ia membantu Bu Sari menata buku cerita yang baru kembali. Setelah itu, Rani memilih satu buku tentang kebun sekolah."),
		makeQuestion(simulasiTipeIsian, "Berapa jenis kegiatan yang dilakukan Rani sebelum memilih buku?", "Literasi membaca", "Menarik informasi", simulasiConfig{AcceptedAnswers: []string{"dua", "2"}}, "Rani datang lebih awal, membantu menata buku, lalu memilih satu buku untuk dibaca."),
		makeQuestion(simulasiTipeUraian, "Tuliskan satu sikap baik Rani dan jelaskan alasanmu berdasarkan teks.", "Literasi membaca", "Refleksi", simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Sikap baik sesuai teks", Maks: 1}, {Kriteria: "Alasan dengan bukti teks", Maks: 1}}}, "Pada Jumat pagi, Rani membantu Bu Sari menata buku cerita yang baru kembali."),
	}
	numerasi := []SimulasiSoal{
		makeQuestion(simulasiTipePGK, "Pilih semua pernyataan yang benar tentang 3/4.", "Numerasi", "Pecahan", simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "3/4 lebih besar daripada 1/2"}, {ID: "b", Text: "3/4 sama dengan 6/8"}, {ID: "c", Text: "3/4 sama dengan 4/3"}}, CorrectIDs: []string{"a", "b"}}, "Satu pizza dibagi menjadi empat bagian sama besar. Tiga bagian dimakan bersama."),
		makeQuestion(simulasiTipeMenjodohkan, "Jodohkan bentuk pecahan dengan nilainya.", "Numerasi", "Pecahan", simulasiConfig{Left: []simulasiChoice{{ID: "l1", Text: "1/2"}, {ID: "l2", Text: "1/4"}}, Right: []simulasiChoice{{ID: "r1", Text: "0,25"}, {ID: "r2", Text: "0,5"}}, Pairs: map[string]string{"l1": "r2", "l2": "r1"}}, "Gunakan hubungan antara pecahan dan desimal untuk memasangkan setiap bentuk."),
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		createPackage := func(name string, questions []SimulasiSoal) error {
			paket := SimulasiPaket{Nama: name, Deskripsi: "Paket contoh pengembangan dengan soal orisinal.", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 30, Instruksi: "Baca stimulus dan soal dengan teliti. Jawaban tersimpan otomatis.", MaksPercobaan: 1, AcakUrutan: true, TampilkanNilai: true, TampilkanRingkasan: true, Status: "terbit", WaktuPublikasi: &now, DibuatOlehUserID: owner.ID}
			if err := tx.Create(&paket).Error; err != nil {
				return err
			}
			for index := range questions {
				// Insert the source first, then persist stimuli explicitly. Omit avoids
				// GORM auto-saving the association a second time.
				if err := tx.Omit("Stimulus").Create(&questions[index]).Error; err != nil {
					return err
				}
				for stimulusIndex := range questions[index].Stimulus {
					questions[index].Stimulus[stimulusIndex].SoalID = questions[index].ID
					if err := tx.Create(&questions[index].Stimulus[stimulusIndex]).Error; err != nil {
						return err
					}
				}
				snap, err := snapshotFromQuestion(questions[index])
				if err != nil {
					return err
				}
				if err := tx.Create(&SimulasiPaketSoal{PaketID: paket.ID, SoalID: &questions[index].ID, Urutan: index + 1, Bobot: questions[index].Bobot, SnapshotJSON: snap}).Error; err != nil {
					return err
				}
			}
			for _, student := range students {
				if err := tx.Create(&SimulasiPenugasan{PaketID: paket.ID, PesertaDidikID: student.ID, KelasIDSaatTugas: student.KelasID}).Error; err != nil {
					return err
				}
			}
			return nil
		}
		if err := createPackage(literasiName, literasi); err != nil {
			return err
		}
		return createPackage("Contoh Simulasi Numerasi Matematika SD", numerasi)
	})
}

func (s *Server) simulasiTemplate(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	book := excelize.NewFile()
	defer book.Close()
	book.SetSheetName("Sheet1", "Soal")
	headers := []string{"ref", "jenjang", "kelas_fase", "mode", "mapel_id", "domain", "topik", "kompetensi", "level_kognitif", "kesulitan", "tags", "tipe", "pertanyaan", "bobot", "status", "konfigurasi_json", "pembahasan"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Soal", cell, header)
	}
	book.SetCellValue("Soal", "A2", "literasi-1")
	book.SetCellValue("Soal", "B2", "SD/MI")
	book.SetCellValue("Soal", "D2", "tka_sd")
	book.SetCellValue("Soal", "L2", simulasiTipePG)
	book.SetCellValue("Soal", "P2", `{"choices":[{"id":"a","text":"Pilihan A"},{"id":"b","text":"Pilihan B"}],"correctIds":["a"]}`)
	book.SetCellValue("Soal", "Q2", "Konfigurasi dapat ditaruh di sini atau sheet Opsi-Konfigurasi.")
	book.SetColWidth("Soal", "A", "Q", 22)
	_, _ = book.NewSheet("Stimulus")
	for i, value := range []string{"soal_ref", "jenis", "konten", "alt_text", "urutan"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Stimulus", cell, value)
	}
	book.SetCellValue("Stimulus", "A2", "literasi-1")
	book.SetCellValue("Stimulus", "B2", "text")
	book.SetCellValue("Stimulus", "C2", "Teks stimulus contoh")
	book.SetCellValue("Stimulus", "E2", 1)
	book.SetColWidth("Stimulus", "A", "E", 26)
	_, _ = book.NewSheet("Opsi-Konfigurasi")
	for i, value := range []string{"soal_ref", "konfigurasi_json", "catatan"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Opsi-Konfigurasi", cell, value)
	}
	book.SetCellValue("Opsi-Konfigurasi", "A2", "literasi-1")
	book.SetCellValue("Opsi-Konfigurasi", "B2", `{"choices":[{"id":"a","text":"Pilihan A"},{"id":"b","text":"Pilihan B"}],"correctIds":["a"]}`)
	book.SetCellValue("Opsi-Konfigurasi", "C2", "Jika diisi, menggantikan konfigurasi_json pada sheet Soal.")
	book.SetColWidth("Opsi-Konfigurasi", "A", "C", 38)
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-bank-soal-simulasi.xlsx")
	return book.Write(c.Response().BodyWriter())
}
func (s *Server) simulasiExportSoal(c *fiber.Ctx) error {
	if err := simulasiStaff(c, false); err != nil {
		return err
	}
	q := s.db.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).Order("created_at desc")
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	var rows []SimulasiSoal
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	book := excelize.NewFile()
	defer book.Close()
	book.SetSheetName("Sheet1", "Soal")
	headers := []string{"ref", "id", "jenjang", "kelas_fase", "mode", "mapel_id", "domain", "topik", "kompetensi", "level_kognitif", "kesulitan", "tags", "tipe", "pertanyaan", "bobot", "status", "konfigurasi_json", "pembahasan"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Soal", cell, header)
	}
	for r, row := range rows {
		values := []interface{}{row.ID, row.ID, row.Jenjang, row.KelasFase, row.Mode, row.MapelID, row.Domain, row.Topik, row.Kompetensi, row.LevelKognitif, row.TingkatKesulitan, row.Tags, row.Tipe, row.Pertanyaan, row.Bobot, row.Status, row.Konfigurasi, row.Pembahasan}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, r+2)
			book.SetCellValue("Soal", cell, value)
		}
	}
	book.SetColWidth("Soal", "A", "R", 22)
	_, _ = book.NewSheet("Stimulus")
	for i, value := range []string{"soal_ref", "jenis", "konten", "alt_text", "urutan"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Stimulus", cell, value)
	}
	_, _ = book.NewSheet("Opsi-Konfigurasi")
	for i, value := range []string{"soal_ref", "konfigurasi_json"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		book.SetCellValue("Opsi-Konfigurasi", cell, value)
	}
	stimulusRow, configRow := 2, 2
	for _, row := range rows {
		book.SetCellValue("Opsi-Konfigurasi", fmt.Sprintf("A%d", configRow), row.ID)
		book.SetCellValue("Opsi-Konfigurasi", fmt.Sprintf("B%d", configRow), row.Konfigurasi)
		configRow++
		for _, stimulus := range row.Stimulus {
			for column, value := range []interface{}{row.ID, stimulus.Jenis, stimulus.Konten, stimulus.AltText, stimulus.Urutan} {
				cell, _ := excelize.CoordinatesToCellName(column+1, stimulusRow)
				book.SetCellValue("Stimulus", cell, value)
			}
			stimulusRow++
		}
	}
	book.SetColWidth("Stimulus", "A", "E", 28)
	book.SetColWidth("Opsi-Konfigurasi", "A", "B", 40)
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("bank-soal-simulasi.xlsx")
	return book.Write(c.Response().BodyWriter())
}
func (s *Server) simulasiImportSoal(c *fiber.Ctx) error {
	if err := simulasiStaff(c, true); err != nil {
		return err
	}
	file, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "file XLSX wajib diunggah")
	}
	stream, err := file.Open()
	if err != nil {
		return err
	}
	defer stream.Close()
	book, err := excelize.OpenReader(stream)
	if err != nil {
		return fiber.NewError(400, "file XLSX tidak valid")
	}
	defer book.Close()
	rows, err := book.GetRows("Soal")
	if err != nil || len(rows) < 2 {
		return fiber.NewError(400, "sheet Soal kosong")
	}
	headers := map[string]int{}
	for i, h := range rows[0] {
		headers[strings.TrimSpace(strings.ToLower(h))] = i
	}
	configByReference := map[string]string{}
	if configRows, configErr := book.GetRows("Opsi-Konfigurasi"); configErr == nil && len(configRows) > 1 {
		configHeaders := map[string]int{}
		for i, h := range configRows[0] {
			configHeaders[strings.TrimSpace(strings.ToLower(h))] = i
		}
		refColumn, hasRef := configHeaders["soal_ref"]
		configColumn, hasConfig := configHeaders["konfigurasi_json"]
		if !hasRef || !hasConfig {
			return fiber.NewError(400, "sheet Opsi-Konfigurasi harus memiliki soal_ref dan konfigurasi_json")
		}
		for _, row := range configRows[1:] {
			if refColumn < len(row) && configColumn < len(row) && strings.TrimSpace(row[refColumn]) != "" && strings.TrimSpace(row[configColumn]) != "" {
				configByReference[strings.TrimSpace(row[refColumn])] = row[configColumn]
			}
		}
	}
	type importedStimulus struct {
		reference string
		value     simulasiStimulusIn
	}
	stimuli := []importedStimulus{}
	if stimulusRows, stimulusErr := book.GetRows("Stimulus"); stimulusErr == nil && len(stimulusRows) > 1 {
		stimulusHeaders := map[string]int{}
		for i, h := range stimulusRows[0] {
			stimulusHeaders[strings.TrimSpace(strings.ToLower(h))] = i
		}
		_, hasRef := stimulusHeaders["soal_ref"]
		_, hasJenis := stimulusHeaders["jenis"]
		_, hasKonten := stimulusHeaders["konten"]
		if !hasRef || !hasJenis || !hasKonten {
			return fiber.NewError(400, "sheet Stimulus harus memiliki soal_ref, jenis, dan konten")
		}
		for _, row := range stimulusRows[1:] {
			get := func(column string) string {
				index, ok := stimulusHeaders[column]
				if !ok || index >= len(row) {
					return ""
				}
				return row[index]
			}
			if strings.TrimSpace(get("soal_ref")) == "" || strings.TrimSpace(get("jenis")) == "" {
				continue
			}
			order, _ := strconv.Atoi(get("urutan"))
			stimuli = append(stimuli, importedStimulus{reference: strings.TrimSpace(get("soal_ref")), value: simulasiStimulusIn{Jenis: get("jenis"), Konten: get("konten"), AltText: get("alt_text"), Urutan: order}})
		}
	}
	needed := []string{"jenjang", "mode", "tipe", "pertanyaan"}
	if _, inQuestionSheet := headers["konfigurasi_json"]; !inQuestionSheet && len(configByReference) == 0 {
		needed = append(needed, "konfigurasi_json")
	}
	for _, name := range needed {
		if _, ok := headers[name]; !ok {
			return fiber.NewError(400, "kolom "+name+" wajib ada")
		}
	}
	created := 0
	err = s.db.Transaction(func(tx *gorm.DB) error {
		createdByReference := map[string]string{}
		for number, row := range rows[1:] {
			get := func(name string) string {
				index, ok := headers[name]
				if !ok || index >= len(row) {
					return ""
				}
				return row[index]
			}
			if strings.TrimSpace(get("pertanyaan")) == "" {
				continue
			}
			reference := strings.TrimSpace(get("ref"))
			if reference == "" {
				reference = fmt.Sprintf("row-%d", number+2)
			}
			configuration := get("konfigurasi_json")
			if fromSheet, ok := configByReference[reference]; ok {
				configuration = fromSheet
			}
			var cfg simulasiConfig
			if err := json.Unmarshal([]byte(configuration), &cfg); err != nil {
				return fmt.Errorf("baris %d konfigurasi_json tidak valid", number+2)
			}
			weight, _ := strconv.ParseFloat(get("bobot"), 64)
			in := simulasiQuestionInput{Jenjang: get("jenjang"), KelasFase: get("kelas_fase"), Mode: get("mode"), Domain: get("domain"), Topik: get("topik"), Kompetensi: get("kompetensi"), LevelKognitif: get("level_kognitif"), TingkatKesulitan: get("kesulitan"), Tags: get("tags"), Tipe: get("tipe"), Pertanyaan: get("pertanyaan"), Konfigurasi: cfg, Pembahasan: get("pembahasan"), Bobot: weight, Status: get("status")}
			if mapel := strings.TrimSpace(get("mapel_id")); mapel != "" {
				in.MapelID = &mapel
			}
			question := SimulasiSoal{DibuatOlehUserID: c.Locals("userID").(string)}
			if err := applyQuestionInput(&question, in); err != nil {
				return fmt.Errorf("baris %d: %w", number+2, err)
			}
			if err := tx.Create(&question).Error; err != nil {
				return err
			}
			createdByReference[reference] = question.ID
			created++
		}
		for _, imported := range stimuli {
			soalID, ok := createdByReference[imported.reference]
			if !ok {
				return fmt.Errorf("stimulus merujuk soal_ref %q yang tidak ada", imported.reference)
			}
			if err := validateStimulus(imported.value); err != nil {
				return err
			}
			if err := tx.Create(&SimulasiStimulus{SoalID: soalID, Jenis: imported.value.Jenis, Konten: imported.value.Konten, AltText: imported.value.AltText, Urutan: imported.value.Urutan}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "import", "simulasi_soal", strconv.Itoa(created))
	return c.JSON(fiber.Map{"created": created})
}
