package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) serveUjianCBTApp(c *fiber.Ctx) error {
	// Cutover is explicitly configuration-gated. Leaving CBT_PUBLIC_URL empty
	// preserves the current local/legacy route; production is redirected only
	// after the independent CBT stack, migration, and verification are ready.
	if target := strings.TrimRight(strings.TrimSpace(os.Getenv("CBT_PUBLIC_URL")), "/"); target != "" {
		return c.Redirect(target+"/ujian", fiber.StatusTemporaryRedirect)
	}
	// Production serves the built React app here. Keep the old embedded page as
	// a safe fallback for local backend-only runs and older deployment bundles.
	if err := c.SendFile("./public/index.html"); err == nil {
		return nil
	}
	return s.serveUjianOnlinePage(c)
}

func (s *Server) ujianOnlinePublicConfig(c *fiber.Ctx) error {
	siteKey := strings.TrimSpace(env("TURNSTILE_SITE_KEY", ""))
	if siteKey == "" && s.cfg.Env != "production" {
		siteKey = "1x00000000000000000000AA"
	}
	return c.JSON(fiber.Map{"turnstileSiteKey": siteKey})
}

func (s *Server) tandaiSoalUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var input struct {
		UjianSoalID string `json:"ujianSoalId"`
		Ditandai    bool   `json:"ditandai"`
	}
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.UjianSoalID) == "" {
		return fiber.NewError(400, "ujianSoalId dan status penanda wajib diisi")
	}
	var attempt UjianPeserta
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&attempt).Error; err != nil {
		return fiber.NewError(404, "Sesi ujian tidak ditemukan")
	}
	if attempt.Status != "mulai" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	if attempt.Mulai != nil && uj.DurasiMenit > 0 && time.Now().After(batasGrace(&attempt, uj)) {
		return fiber.NewError(403, "Waktu ujian sudah habis")
	}
	var question UjianPesertaSoal
	if err := s.db.Where("ujian_peserta_id = ? AND ujian_soal_id = ?", attempt.ID, input.UjianSoalID).First(&question).Error; err != nil {
		return fiber.NewError(404, "Soal tidak ditemukan dalam sesi ini")
	}
	if err := s.db.Model(&question).Update("ditandai", input.Ditandai).Error; err != nil {
		return fiber.NewError(500, "Status penanda belum tersimpan")
	}
	return c.JSON(fiber.Map{"status": "ok", "ditandai": input.Ditandai})
}

type ujianRecoveryAnswer struct {
	UjianSoalID  string `json:"ujianSoalId"`
	Jawaban      string `json:"jawaban"`
	BaseRevision int    `json:"baseRevision"`
	Urutan       int    `json:"urutan,omitempty"`
	Pertanyaan   string `json:"pertanyaan,omitempty"`
}

func (s *Server) submitUjianOnlineRecovery(c *fiber.Ctx) error {
	pd, err := s.ujianOnlineAuth(c)
	if err != nil {
		return err
	}
	var uj Ujian
	if err := s.db.First(&uj, "id = ?", c.Params("ujianId")).Error; err != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	accessHash, ok := c.Locals("examAccessCodeHash").(string)
	if !ok || uj.AksesKode == "" || subtle.ConstantTimeCompare([]byte(accessHash), []byte(hash(strings.TrimSpace(uj.AksesKode)))) != 1 {
		return fiber.NewError(403, "Kode akses tidak sesuai untuk ujian ini")
	}
	if pd.KelasID != uj.KelasID {
		return fiber.NewError(403, "Anda tidak terdaftar di kelas ujian ini")
	}
	var input struct {
		IdempotencyKey string                `json:"idempotencyKey"`
		Answers        []ujianRecoveryAnswer `json:"answers"`
	}
	if err := c.BodyParser(&input); err != nil || len(input.Answers) == 0 || len(input.Answers) > 250 {
		return fiber.NewError(400, "Daftar jawaban pemulihan tidak valid")
	}
	if _, err := uuid.Parse(input.IdempotencyKey); err != nil {
		return fiber.NewError(400, "Kunci pemulihan tidak valid")
	}
	var attempt UjianPeserta
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&attempt).Error; err != nil {
		return fiber.NewError(404, "Sesi ujian tidak ditemukan")
	}
	if attempt.Status == "dikunci" || attempt.Status == "selesai" || attempt.Status == "menunggu_nilai" {
		// completed attempts can submit recovery records; review never changes the score
	} else if attempt.Mulai == nil || time.Now().Before(batasGrace(&attempt, &uj)) {
		return fiber.NewError(409, "Pemulihan hanya tersedia setelah batas waktu ujian")
	}
	payload, err := json.Marshal(input.Answers)
	if err != nil || len(payload) > 1024*1024 {
		return fiber.NewError(413, "Data jawaban pemulihan terlalu besar")
	}
	var prior UjianJawabanPemulihan
	if err := s.db.Where("idempotency_key = ?", input.IdempotencyKey).First(&prior).Error; err == nil {
		if prior.UjianPesertaID != attempt.ID || prior.JawabanJSON != string(payload) {
			return fiber.NewError(409, "Kunci pemulihan sudah digunakan untuk data lain")
		}
		return c.JSON(fiber.Map{"status": "menunggu", "id": prior.ID, "diterima": true})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(500, "Gagal memeriksa kiriman pemulihan")
	}
	questions, _, activeQuestions, err := s.loadActiveUjianAttemptQuestions(&attempt, &uj)
	if err != nil {
		return fiber.NewError(500, "Gagal memeriksa snapshot ujian")
	}
	byID := make(map[string]UjianPesertaSoal, len(questions))
	for _, item := range questions {
		byID[item.UjianSoalID] = item
	}
	seen := map[string]bool{}
	for _, answer := range input.Answers {
		item, ok := byID[answer.UjianSoalID]
		if !ok || !activeQuestions[answer.UjianSoalID] || seen[answer.UjianSoalID] || len(answer.Jawaban) > 64*1024 {
			return fiber.NewError(400, "Jawaban pemulihan berisi soal yang tidak valid")
		}
		seen[answer.UjianSoalID] = true
		snapshot, err := loadUjianQuestionSnapshot(item.SnapshotJSON)
		if err != nil {
			return fiber.NewError(500, "Snapshot soal ujian tidak valid")
		}
		config := ""
		if hasUjianVisualConfig(snapshot.Konfigurasi) {
			encoded, _ := json.Marshal(snapshot.Konfigurasi)
			config = string(encoded)
		}
		if err := validateUjianAnswer(snapshot.Tipe, snapshot.Opsi, answer.Jawaban, config); err != nil {
			return fiber.NewError(400, "Jawaban pemulihan tidak valid: "+err.Error())
		}
		if snapshot.Tipe == simulasiTipeUnggah {
			fileIDs, decodeErr := decodeSimulasiFileIDs([]byte(answer.Jawaban))
			if decodeErr != nil {
				return fiber.NewError(400, "referensi berkas jawaban tidak valid")
			}
			var owned int64
			if err := s.db.Model(&UjianJawabanBerkas{}).Where("id IN ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", fileIDs, attempt.ID, answer.UjianSoalID).Count(&owned).Error; err != nil {
				return fiber.NewError(500, "Gagal memeriksa kepemilikan berkas")
			}
			if owned != int64(len(fileIDs)) {
				return fiber.NewError(403, "Berkas pemulihan harus berasal dari jawaban Anda")
			}
		}
	}
	recovery := UjianJawabanPemulihan{UjianID: uj.ID, UjianPesertaID: attempt.ID, PesertaDidikID: pd.ID, IdempotencyKey: input.IdempotencyKey, JawabanJSON: string(payload), Status: "menunggu"}
	if err := s.db.Create(&recovery).Error; err != nil {
		if isUniqueErr(err) {
			var priorAttempt UjianJawabanPemulihan
			if lookupErr := s.db.Where("ujian_peserta_id = ?", attempt.ID).First(&priorAttempt).Error; lookupErr != nil {
				return fiber.NewError(500, "Kiriman pemulihan ganda tidak dapat dibaca")
			}
			if priorAttempt.JawabanJSON != string(payload) {
				return fiber.NewError(409, "Pemulihan untuk ujian ini sudah dikirim dan sedang ditinjau")
			}
			return c.JSON(fiber.Map{"status": priorAttempt.Status, "id": priorAttempt.ID, "diterima": true})
		}
		return fiber.NewError(500, "Gagal menyimpan jawaban pemulihan")
	}
	s.audit(&pd.ID, "submit_late_answer_recovery", "ujian_online_recovery", recovery.ID)
	return c.Status(202).JSON(fiber.Map{"status": recovery.Status, "id": recovery.ID, "diterima": true, "pesan": "Jawaban tersimpan untuk ditinjau guru; nilai ujian tidak berubah."})
}

func (s *Server) listUjianOnlineRecoveries(c *fiber.Ctx) error {
	var uj Ujian
	if err := s.db.First(&uj, "id = ?", c.Params("ujianId")).Error; err != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if err := s.scopeUjian(c, &uj); err != nil {
		return err
	}
	var rows []UjianJawabanPemulihan
	if err := s.db.Where("ujian_id = ?", uj.ID).Order("created_at desc").Find(&rows).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat jawaban pemulihan")
	}
	type recoveryView struct {
		ID               string                `json:"id"`
		UjianPesertaID   string                `json:"ujianPesertaId"`
		PesertaDidikID   string                `json:"pesertaDidikId"`
		NamaPeserta      string                `json:"namaPeserta"`
		Status           string                `json:"status"`
		DibuatPada       time.Time             `json:"dibuatPada"`
		DitinjauOlehID   *string               `json:"ditinjauOlehId,omitempty"`
		DitinjauPada     *time.Time            `json:"ditinjauPada,omitempty"`
		KomentarPeninjau string                `json:"komentarPeninjau,omitempty"`
		Jawaban          []ujianRecoveryAnswer `json:"jawaban"`
	}
	result := make([]recoveryView, 0, len(rows))
	for _, row := range rows {
		var answers []ujianRecoveryAnswer
		if err := json.Unmarshal([]byte(row.JawabanJSON), &answers); err != nil {
			return fiber.NewError(500, "Data jawaban pemulihan tidak valid")
		}
		var attempt UjianPeserta
		if err := s.db.Preload("PesertaDidik").First(&attempt, "id = ?", row.UjianPesertaID).Error; err != nil {
			return fiber.NewError(500, "Sesi jawaban pemulihan tidak ditemukan")
		}
		var questions []UjianPesertaSoal
		if err := s.db.Where("ujian_peserta_id = ?", row.UjianPesertaID).Order("urutan asc").Find(&questions).Error; err != nil {
			return fiber.NewError(500, "Snapshot soal pemulihan tidak dapat dimuat")
		}
		questionByID := make(map[string]UjianPesertaSoal, len(questions))
		for _, question := range questions {
			questionByID[question.UjianSoalID] = question
		}
		for index := range answers {
			if question, exists := questionByID[answers[index].UjianSoalID]; exists {
				snapshot, snapshotErr := loadUjianQuestionSnapshot(question.SnapshotJSON)
				if snapshotErr != nil {
					return fiber.NewError(500, "Snapshot soal pemulihan tidak valid")
				}
				answers[index].Urutan = question.Urutan
				answers[index].Pertanyaan = snapshot.Pertanyaan
			}
		}
		result = append(result, recoveryView{ID: row.ID, UjianPesertaID: row.UjianPesertaID, PesertaDidikID: row.PesertaDidikID, NamaPeserta: attempt.PesertaDidik.Nama, Status: row.Status, DibuatPada: row.CreatedAt, DitinjauOlehID: row.DitinjauOlehID, DitinjauPada: row.DitinjauPada, KomentarPeninjau: row.KomentarPeninjau, Jawaban: answers})
	}
	return c.JSON(result)
}

func (s *Server) reviewUjianOnlineRecovery(c *fiber.Ctx) error {
	var recovery UjianJawabanPemulihan
	if err := s.db.First(&recovery, "id = ?", c.Params("recoveryId")).Error; err != nil {
		return fiber.NewError(404, "Kiriman pemulihan tidak ditemukan")
	}
	var uj Ujian
	if err := s.db.First(&uj, "id = ?", recovery.UjianID).Error; err != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if err := s.scopeUjian(c, &uj); err != nil {
		return err
	}
	var input struct {
		Status   string `json:"status"`
		Komentar string `json:"komentar"`
	}
	if err := c.BodyParser(&input); err != nil || (input.Status != "diterima" && input.Status != "ditolak") {
		return fiber.NewError(400, "Status tinjauan harus diterima atau ditolak")
	}
	input.Komentar = strings.TrimSpace(input.Komentar)
	if len(input.Komentar) > 4000 {
		return fiber.NewError(400, "Catatan tinjauan maksimal 4000 karakter")
	}
	if recovery.Status != "menunggu" {
		return fiber.NewError(409, "Kiriman ini sudah ditinjau")
	}
	now := time.Now()
	actor, _ := c.Locals("userID").(string)
	updates := map[string]interface{}{"status": input.Status, "komentar_peninjau": input.Komentar, "ditinjau_pada": now}
	if actor != "" {
		updates["ditinjau_oleh_id"] = actor
	}
	result := s.db.Model(&UjianJawabanPemulihan{}).Where("id = ? AND status = ?", recovery.ID, "menunggu").Updates(updates)
	if result.Error != nil {
		return fiber.NewError(500, "Tinjauan belum tersimpan")
	}
	if result.RowsAffected == 0 {
		return fiber.NewError(409, "Kiriman ini sudah ditinjau")
	}
	if actor != "" {
		s.audit(&actor, "review_late_answer_recovery_"+input.Status, "ujian_online_recovery", recovery.ID)
	}
	return c.JSON(fiber.Map{"status": input.Status, "skorDiubah": false})
}
