package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ============================================================================

// ============================================================================
// Ujian Online — public endpoints (no JWT; authenticated by NISN + AksesKode)
// ============================================================================

// ujianOnlineAuth validates NISN + AksesKode and returns the PesertaDidik if valid.
func (s *Server) ujianOnlineAuth(c *fiber.Ctx) (*PesertaDidik, error) {
	nisn := strings.TrimSpace(c.Query("nisn"))
	if nisn == "" {
		nisn = strings.TrimSpace(c.FormValue("nisn"))
	}
	kode := strings.TrimSpace(c.Query("aksesKode"))
	if kode == "" {
		kode = strings.TrimSpace(c.FormValue("aksesKode"))
	}
	if nisn == "" || kode == "" {
		return nil, fiber.NewError(400, "NISN dan Kode Akses wajib diisi")
	}
	var pd PesertaDidik
	if s.db.Preload("Kelas").Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return nil, fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	return &pd, nil
}

// ujianOnlineKodeAuth validates NISN + AksesKode against a specific Ujian.
func (s *Server) ujianOnlineKodeAuth(c *fiber.Ctx, ujianID string) (*PesertaDidik, *Ujian, error) {
	pd, err := s.ujianOnlineAuth(c)
	if err != nil {
		return nil, nil, err
	}
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return nil, nil, fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if uj.AksesKode == "" {
		return nil, nil, fiber.NewError(403, "Ujian ini tidak memiliki kode akses")
	}
	if strings.TrimSpace(uj.AksesKode) != strings.TrimSpace(c.Query("aksesKode")) &&
		strings.TrimSpace(uj.AksesKode) != strings.TrimSpace(c.FormValue("aksesKode")) {
		return nil, nil, fiber.NewError(403, "Kode akses salah")
	}
	if pd.KelasID != uj.KelasID {
		return nil, nil, fiber.NewError(403, "Anda tidak terdaftar di kelas ujian ini")
	}
	now := time.Now()
	if now.Before(uj.WaktuMulai) {
		return nil, nil, fiber.NewError(403, "Ujian belum dimulai")
	}
	if now.After(uj.WaktuSelesai) {
		return nil, nil, fiber.NewError(403, "Ujian sudah berakhir")
	}
	return pd, &uj, nil
}

// cekUjianOnline — POST /ujian-online/cek {nisn, aksesKode}
// Returns list of exams available for the student's class that match the access code.
func (s *Server) cekUjianOnline(c *fiber.Ctx) error {
	nisn := strings.TrimSpace(c.FormValue("nisn"))
	kode := strings.TrimSpace(c.FormValue("aksesKode"))
	turnstileToken := c.FormValue("cf-turnstile-response")
	if nisn == "" || kode == "" {
		return fiber.NewError(400, "NISN dan Kode Akses wajib diisi")
	}
	if e := s.requireTurnstile(c, turnstileToken); e != nil {
		return e
	}
	var pd PesertaDidik
	if s.db.Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	var ujians []Ujian
	s.db.Preload("Mapel").Preload("Kelas").
		Where("akses_kode = ? AND kelas_id = ?", strings.TrimSpace(kode), pd.KelasID).
		Where("waktu_mulai <= ? AND waktu_selesai >= ?", time.Now(), time.Now()).
		Order("waktu_mulai desc").
		Find(&ujians)
	if len(ujians) == 0 {
		return fiber.NewError(404, "Tidak ada ujian aktif dengan kode akses tersebut untuk kelas Anda")
	}
	// Load all existing sessions in one query instead of one query per exam.
	ujianIDs := make([]string, 0, len(ujians))
	for _, uj := range ujians {
		ujianIDs = append(ujianIDs, uj.ID)
	}
	var sessions []UjianPeserta
	s.db.Where("peserta_didik_id = ? AND ujian_id IN ?", pd.ID, ujianIDs).Find(&sessions)
	sessionByExam := make(map[string]UjianPeserta, len(sessions))
	for _, session := range sessions {
		sessionByExam[session.UjianID] = session
	}
	type ujianRes struct {
		Ujian
		SudahMengerjakan bool     `json:"sudahMengerjakan"`
		Skor             *float64 `json:"skor"`
	}
	var res []ujianRes
	for _, uj := range ujians {
		r := ujianRes{Ujian: uj, SudahMengerjakan: false, Skor: nil}
		if up, ok := sessionByExam[uj.ID]; ok {
			r.SudahMengerjakan = true
			r.Skor = up.Skor
		}
		res = append(res, r)
	}
	return c.JSON(res)
}

// mulaiUjianOnline — POST /ujian-online/:ujianId/mulai {nisn, aksesKode}
// Creates a UjianPeserta record (idempotent: returns existing if already started).
func (s *Server) mulaiUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	// Idempotent: if already has a session, return it
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error == nil {
		return c.JSON(up)
	}
	now := time.Now()
	up = UjianPeserta{
		UjianID:        uj.ID,
		PesertaDidikID: pd.ID,
		Mulai:          &now,
		Status:         "mulai",
	}
	if e := s.db.Create(&up).Error; e != nil {
		// Two tabs can submit "Mulai" at the same time. The unique index is the
		// source of truth; return the winner's session instead of a 500.
		if isUniqueErr(e) {
			if lookupErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; lookupErr == nil {
				return c.JSON(up)
			}
		}
		return fiber.NewError(500, "Gagal memulai ujian: "+e.Error())
	}
	return c.Status(201).JSON(up)
}

// getSoalUjianOnline — GET /ujian-online/:ujianId/soal?nisn=...&aksesKode=...
// Returns shuffled soal list (without answers) for the exam.
func (s *Server) getSoalUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	// Check or create UjianPeserta
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		now := time.Now()
		up = UjianPeserta{UjianID: uj.ID, PesertaDidikID: pd.ID, Mulai: &now, Status: "mulai"}
		s.db.Create(&up)
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Anda sudah menyelesaikan ujian ini")
	}
	// Check if time is up (beyond grace period)
	if up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			// Hard lock: grace period expired
			up.Status = "selesai"
			up.Selesai = &time.Time{}
			skor := s.gradeUjianPeserta(&up, uj)
			up.Skor = &skor
			s.db.Save(&up)
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
	}
	var us []UjianSoal
	s.db.Preload("Soal").Where("ujian_id = ?", uj.ID).Order("created_at").Find(&us)
	// Shuffle if AcakSoal
	order := us
	if uj.AcakSoal {
		seed := seedFromID(uj.ID)
		cp := make([]UjianSoal, len(us))
		copy(cp, us)
		r := make([]int, len(us))
		for i := range r {
			r[i] = i
		}
		// deterministic shuffle
		for i := len(r) - 1; i > 0; i-- {
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			j := int(seed) % (i + 1)
			r[i], r[j] = r[j], r[i]
		}
		shuffled := make([]UjianSoal, len(us))
		for i, idx := range r {
			shuffled[i] = cp[idx]
		}
		order = shuffled
	}
	// Build response: strip answers
	type soalRes struct {
		ID         string   `json:"id"`
		UjianID    string   `json:"ujianId"`
		Bobot      float64  `json:"bobot"`
		Pertanyaan string   `json:"pertanyaan"`
		Tipe       string   `json:"tipe"`
		Opsi       []string `json:"opsi"`
		// Benar/Kunci are intentionally excluded
	}
	var res []soalRes
	for _, item := range order {
		r := soalRes{
			ID:         item.ID,
			UjianID:    item.UjianID,
			Bobot:      item.Bobot,
			Pertanyaan: item.Soal.Pertanyaan,
			Tipe:       item.Soal.Tipe,
		}
		if item.Soal.Tipe == "pg" && item.Soal.Opsi != "" {
			var opsi []string
			if json.Unmarshal([]byte(item.Soal.Opsi), &opsi) == nil {
				r.Opsi = opsi
			}
		}
		res = append(res, r)
	}
	// Also return existing answers
	type jawabanRes struct {
		UjianSoalID string `json:"ujianSoalId"`
		Jawaban     string `json:"jawaban"`
	}
	var jawabans []UjianJawaban
	s.db.Where("ujian_peserta_id = ?", up.ID).Find(&jawabans)
	var jawabanList []jawabanRes
	for _, j := range jawabans {
		jawabanList = append(jawabanList, jawabanRes{UjianSoalID: j.SoalID, Jawaban: j.Jawaban})
	}
	return c.JSON(fiber.Map{
		"ujianPesertaId":   up.ID,
		"sisaWaktu":        s.sisaWaktu(&up, uj),
		"gracePeriodMenit": uj.GracePeriodMenit,
		"soal":             res,
		"jawaban":          jawabanList,
	})
}

// sisaWaktu returns remaining seconds. Positive = normal countdown.
// Negative = grace period (absolute value = grace seconds remaining).
// Returns 0 if no timer or both expired.
func (s *Server) sisaWaktu(up *UjianPeserta, uj *Ujian) int {
	if up.Mulai == nil || uj.DurasiMenit == 0 {
		return 0
	}
	batas := batasWaktu(up, uj)
	sisa := time.Until(batas).Seconds()
	if sisa >= 0 {
		return int(sisa)
	}
	// Normal time expired — check grace period
	grace := batasGrace(up, uj)
	sisaGrace := time.Until(grace).Seconds()
	if sisaGrace > 0 {
		return -int(sisaGrace) // negative = grace period
	}
	return 0 // both expired
}

// jawabSoal — POST /ujian-online/:ujianId/jawab {nisn, aksesKode, ujianSoalId, jawaban}
func (s *Server) jawabSoal(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	// Check time (beyond grace period = hard lock)
	if up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			up.Status = "selesai"
			skor := s.gradeUjianPeserta(&up, uj)
			up.Skor = &skor
			s.db.Save(&up)
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
	}
	var in struct {
		UjianSoalID string `json:"ujianSoalId"`
		Jawaban     string `json:"jawaban"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.UjianSoalID == "" {
		return fiber.NewError(400, "ujianSoalId wajib diisi")
	}
	// Verify the soal belongs to this ujian
	var us UjianSoal
	if s.db.Where("ujian_id = ? AND id = ?", uj.ID, in.UjianSoalID).First(&us).Error != nil {
		return fiber.NewError(400, "Soal tidak ditemukan dalam ujian ini")
	}
	// Upsert jawaban
	var jawaban UjianJawaban
	if s.db.Where("ujian_peserta_id = ? AND soal_id = ?", up.ID, us.SoalID).First(&jawaban).Error == nil {
		jawaban.Jawaban = in.Jawaban
		s.db.Save(&jawaban)
	} else {
		jawaban = UjianJawaban{
			UjianPesertaID: up.ID,
			SoalID:         us.SoalID,
			Jawaban:        in.Jawaban,
		}
		if e := s.db.Create(&jawaban).Error; e != nil {
			return fiber.NewError(500, "Gagal menyimpan jawaban")
		}
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// gradeUjianPeserta auto-grades all answers for a participant and computes the
// final score. Used by selesaiUjianOnline (manual finish) and
// autoFinishUjianSessions (server-side timeout/tab-lock).
func (s *Server) gradeUjianPeserta(up *UjianPeserta, uj *Ujian) float64 {
	var jawabans []UjianJawaban
	s.db.Where("ujian_peserta_id = ?", up.ID).Find(&jawabans)
	var ujianSoals []UjianSoal
	s.db.Preload("Soal").Where("ujian_id = ?", uj.ID).Find(&ujianSoals)

	soalLookup := map[string]UjianSoal{}
	for _, us := range ujianSoals {
		soalLookup[us.SoalID] = us
	}

	totalSkor := 0.0
	totalBobot := 0.0
	for i := range jawabans {
		us, ok := soalLookup[jawabans[i].SoalID]
		if !ok {
			s.db.Save(&jawabans[i])
			continue
		}
		totalBobot += us.Bobot
		kunci := strings.TrimSpace(us.Soal.Kunci)
		jawaban := strings.TrimSpace(jawabans[i].Jawaban)
		if kunci == "" || jawaban == "" {
			s.db.Save(&jawabans[i])
			continue
		}
		var benar bool
		switch us.Soal.Tipe {
		case "essay":
			benar = strings.Contains(strings.ToLower(jawaban), strings.ToLower(kunci))
		default:
			benar = kunci == jawaban
		}
		jawabans[i].Benar = &benar
		if benar {
			jawabans[i].Nilai = us.Bobot
			totalSkor += us.Bobot
		}
		s.db.Save(&jawabans[i])
	}

	var skor float64
	if totalBobot > 0 {
		skor = (totalSkor / totalBobot) * 100
	}
	return skor
}

// batasWaktu returns the normal deadline (Mulai + DurasiMenit).
// batasGrace returns the hard deadline including grace period.
func batasWaktu(up *UjianPeserta, uj *Ujian) time.Time {
	if up.Mulai == nil || uj.DurasiMenit == 0 {
		return time.Time{}
	}
	return up.Mulai.Add(time.Duration(uj.DurasiMenit) * time.Minute)
}
func batasGrace(up *UjianPeserta, uj *Ujian) time.Time {
	b := batasWaktu(up, uj)
	if b.IsZero() {
		return b
	}
	gp := uj.GracePeriodMenit
	if gp <= 0 {
		gp = 5 // default 5 menit grace period
	}
	return b.Add(time.Duration(gp) * time.Minute)
}

// autoFinishUjianSessions runs every 30s and closes any ujian session whose
// grace period has expired. This ensures exams are graded even if the student
// closes their browser without clicking "Selesai".
func (s *Server) autoFinishUjianSessions() {
	var pesertas []UjianPeserta
	s.db.Preload("Ujian").Where("status = ? AND mulai IS NOT NULL", "mulai").Find(&pesertas)
	now := time.Now()
	for _, up := range pesertas {
		if up.Ujian.DurasiMenit == 0 || up.Mulai == nil {
			continue
		}
		// Only auto-finish after the FULL grace period has expired
		if now.After(batasGrace(&up, &up.Ujian)) {
			up.Selesai = &now
			up.Status = "selesai"
			skor := s.gradeUjianPeserta(&up, &up.Ujian)
			up.Skor = &skor
			s.db.Save(&up)
		}
	}
}

// selesaiUjianOnline — POST /ujian-online/:ujianId/selesai {nisn, aksesKode}
// Auto-grades PG answers, computes score, returns result.
func (s *Server) selesaiUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	now := time.Now()
	up.Selesai = &now
	up.Status = "selesai"

	skor := s.gradeUjianPeserta(&up, uj)

	up.Skor = &skor
	s.db.Save(&up)
	return c.JSON(fiber.Map{
		"skor":   skor,
		"status": "selesai",
	})
}

// tabSwitchUjianOnline — POST /ujian-online/:ujianId/tab-switch {nisn, aksesKode}
func (s *Server) tabSwitchUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	up.TabSwitch++
	// Auto-lock if batas terlampaui
	if uj.BatasTabSwitch > 0 && up.TabSwitch >= uj.BatasTabSwitch {
		now := time.Now()
		up.Selesai = &now
		up.Status = "dikunci"
		skor := s.gradeUjianPeserta(&up, uj)
		up.Skor = &skor
		s.db.Save(&up)
		return c.JSON(fiber.Map{"tabSwitch": up.TabSwitch, "locked": true, "skor": skor})
	}
	s.db.Save(&up)
	return c.JSON(fiber.Map{"tabSwitch": up.TabSwitch})
}

// ============================================================================
// Monitoring Ujian Online — protected (teacher/admin only)
// ============================================================================

// monitorUjianOnline — GET /ujian-online/monitor/:ujianId
func (s *Server) monitorUjianOnline(c *fiber.Ctx) error {
	ujianID := c.Params("ujianId")
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	var pesertas []UjianPeserta
	s.db.Preload("PesertaDidik").Where("ujian_id = ?", ujianID).Find(&pesertas)
	return c.JSON(pesertas)
}

// ============================================================================
// Notifikasi — protected (JWT)
// ============================================================================

func (s *Server) listNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var rows []Notifikasi
	s.db.Where("user_id = ?", uid).Order("created_at desc").Limit(50).Find(&rows)
	return c.JSON(rows)
}

func (s *Server) unreadNotifikasiCount(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var n int64
	s.db.Model(&Notifikasi{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&n)
	return c.JSON(fiber.Map{"count": n})
}

func (s *Server) bacaNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	now := time.Now()
	result := s.db.Model(&Notifikasi{}).
		Where("id = ? AND user_id = ?", c.Params("id"), uid).
		Updates(map[string]interface{}{"is_read": true, "dibaca_pada": now})
	if result.RowsAffected == 0 {
		return fiber.NewError(404, "Notifikasi tidak ditemukan")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *Server) bacaSemuaNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	now := time.Now()
	s.db.Model(&Notifikasi{}).
		Where("user_id = ? AND is_read = ?", uid, false).
		Updates(map[string]interface{}{"is_read": true, "dibaca_pada": now})
	return c.JSON(fiber.Map{"status": "ok"})
}

// streamNotifikasi — Server-Sent Events: pushes new notifications to the client
// in real-time. Uses a simple polling loop that checks every 5 seconds.
// Accepts token via query param (EventSource can't set headers).
func (s *Server) streamNotifikasi(c *fiber.Ctx) error {
	// Accept token from query param since EventSource can't set Authorization header
	token := c.Query("token")
	if token == "" {
		h := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		token = h
	}
	if token == "" {
		return fiber.NewError(401, "missing access token")
	}
	// Parse JWT to get userID. Use the same algorithm and claim validation as
	// ordinary API requests so malformed SSE tokens cannot panic the process.
	_, uid, _, err := s.parseAccessToken(token)
	if err != nil {
		return fiber.NewError(401, "invalid access token")
	}

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStream(nil, -1)
	w := c.Response().BodyWriter()

	flusher, ok := w.(interface{ Flush() })
	if !ok {
		return fiber.NewError(500, "streaming not supported")
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastCheck time.Time = time.Now()

	for {
		select {
		case <-c.Context().Done():
			return nil
		case <-ticker.C:
			var newNotifs []Notifikasi
			s.db.Where("user_id = ? AND created_at > ?", uid, lastCheck).
				Order("created_at desc").Find(&newNotifs)
			if len(newNotifs) > 0 {
				data, _ := json.Marshal(newNotifs)
				fmt.Fprintf(w, "event: notifikasi\ndata: %s\n\n", string(data))
				flusher.Flush()
			}
			// Also send unread count
			var count int64
			s.db.Model(&Notifikasi{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&count)
			fmt.Fprintf(w, "event: unread\ndata: %d\n\n", count)
			flusher.Flush()
			lastCheck = time.Now()
		}
	}
}

// ============================================================================
// Kalender Akademik — protected (JWT)
// ============================================================================

func (s *Server) listKalenderEvent(c *fiber.Ctx) error {
	q := s.db.Preload("TahunAjaran").Order("tanggal_mulai")
	if v := c.Query("tahunAjaranId"); v != "" {
		q = q.Where("tahun_ajaran_id = ?", v)
	}
	if v := c.Query("bulan"); v != "" {
		// Filter by month (YYYY-MM)
		q = q.Where("tanggal_mulai >= ? AND tanggal_mulai < ?",
			v+"-01", v+"-32")
	}
	var rows []KalenderEvent
	q.Find(&rows)
	return c.JSON(rows)
}

func (s *Server) createKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat membuat event kalender")
	}
	uid := c.Locals("userID").(string)
	var in struct {
		Judul          string     `json:"judul"`
		Deskripsi      string     `json:"deskripsi"`
		TanggalMulai   time.Time  `json:"tanggalMulai"`
		TanggalSelesai *time.Time `json:"tanggalSelesai"`
		Tipe           string     `json:"tipe"`
		Warna          string     `json:"warna"`
		Semester       *string    `json:"semester"`
		TahunAjaranID  *string    `json:"tahunAjaranId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul == "" || in.TanggalMulai.IsZero() {
		return fiber.NewError(400, "judul dan tanggalMulai wajib diisi")
	}
	ev := KalenderEvent{
		Judul:            in.Judul,
		Deskripsi:        in.Deskripsi,
		TanggalMulai:     in.TanggalMulai,
		TanggalSelesai:   in.TanggalSelesai,
		Tipe:             in.Tipe,
		Warna:            in.Warna,
		Semester:         in.Semester,
		TahunAjaranID:    in.TahunAjaranID,
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&ev).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "kalender", ev.ID)
	return c.Status(201).JSON(ev)
}

func (s *Server) updateKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat mengubah event kalender")
	}
	uid := c.Locals("userID").(string)
	var ev KalenderEvent
	if s.db.First(&ev, "id = ?", c.Params("id")).Error != nil {
		return fiber.NewError(404, "event tidak ditemukan")
	}
	var in struct {
		Judul          string     `json:"judul"`
		Deskripsi      string     `json:"deskripsi"`
		TanggalMulai   time.Time  `json:"tanggalMulai"`
		TanggalSelesai *time.Time `json:"tanggalSelesai"`
		Tipe           string     `json:"tipe"`
		Warna          string     `json:"warna"`
		Semester       *string    `json:"semester"`
		TahunAjaranID  *string    `json:"tahunAjaranId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul != "" {
		ev.Judul = in.Judul
	}
	if in.Deskripsi != "" {
		ev.Deskripsi = in.Deskripsi
	}
	if !in.TanggalMulai.IsZero() {
		ev.TanggalMulai = in.TanggalMulai
	}
	if in.TanggalSelesai != nil {
		ev.TanggalSelesai = in.TanggalSelesai
	}
	if in.Tipe != "" {
		ev.Tipe = in.Tipe
	}
	if in.Warna != "" {
		ev.Warna = in.Warna
	}
	if e := s.db.Save(&ev).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "kalender", ev.ID)
	return c.JSON(ev)
}

func (s *Server) deleteKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat menghapus event kalender")
	}
	uid := c.Locals("userID").(string)
	if e := s.db.Delete(&KalenderEvent{}, "id = ?", c.Params("id")).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "kalender", c.Params("id"))
	return c.SendStatus(204)
}

// ============================================================================
// Portal Orang Tua — login by NIK + NISN (no JWT), then JWT session
// ============================================================================

// loginOrangTua — POST /orang-tua/login {nisn, tanggalLahir}
// Authenticates parent by NISN (child) + tanggal lahir (child), returns JWT with role=orang_tua.
func (s *Server) loginOrangTua(c *fiber.Ctx) error {
	var in struct {
		NISN           string `json:"nisn"`
		TanggalLahir   string `json:"tanggalLahir"` // format: DDMMYYYY
		TurnstileToken string `json:"cf-turnstile-response"`
	}
	if e := c.BodyParser(&in); e != nil || in.NISN == "" || in.TanggalLahir == "" {
		return fiber.NewError(400, "NISN dan tanggal lahir wajib diisi")
	}
	if e := s.requireTurnstile(c, in.TurnstileToken); e != nil {
		return e
	}
	// Parse tanggal lahir DDMMYYYY
	if len(in.TanggalLahir) != 8 {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	day, err1 := strconv.Atoi(in.TanggalLahir[0:2])
	month, err2 := strconv.Atoi(in.TanggalLahir[2:4])
	year, err3 := strconv.Atoi(in.TanggalLahir[4:8])
	if err1 != nil || err2 != nil || err3 != nil || day < 1 || day > 31 || month < 1 || month > 12 || year < 1900 {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	tl := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if tl.Day() != day || int(tl.Month()) != month || tl.Year() != year {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	// Find student by NISN
	var pd PesertaDidik
	if s.db.Preload("OrangTua").Preload("Kelas").Where("nisn = ?", in.NISN).First(&pd).Error != nil {
		return fiber.NewError(401, "NISN tidak ditemukan")
	}
	// Verify tanggal lahir matches
	if pd.TanggalLahir == nil {
		return fiber.NewError(401, "Data tanggal lahir siswa belum diisi. Hubungi admin sekolah.")
	}
	if pd.TanggalLahir.Year() != tl.Year() || pd.TanggalLahir.Month() != tl.Month() || pd.TanggalLahir.Day() != tl.Day() {
		return fiber.NewError(401, "Tanggal lahir tidak cocok")
	}
	// Check parent exists
	ortu := pd.OrangTua
	if ortu.ID == "" {
		return fiber.NewError(401, "Peserta didik tidak memiliki data orang tua")
	}
	// Find or create user account for this orang tua
	var u User
	if s.db.Where("orang_tua_id = ? AND role = ?", ortu.ID, "orang_tua").First(&u).Error != nil {
		// Auto-create user account
		// Use the stable parent record ID instead of a name-derived username;
		// two families may legitimately have the same mother's name.
		username := "ortu-" + ortu.ID
		passwordHash, hashErr := bcryptHash("OrangTua123")
		if hashErr != nil {
			return fiber.NewError(500, "gagal menyiapkan akun orang tua")
		}
		u = User{
			Username:     username,
			Email:        username + "@pkbm.local",
			PasswordHash: passwordHash,
			Role:         "orang_tua",
			OrangTuaID:   &ortu.ID,
			IsActive:     true,
		}
		if err := s.db.Create(&u).Error; err != nil {
			return fiber.NewError(500, "gagal membuat akun orang tua")
		}
	}
	if !u.IsActive {
		return fiber.NewError(403, "Akun orang tua nonaktif")
	}
	access, e := s.token(u, s.cfg.AccessSecret, s.cfg.AccessTTL)
	if e != nil {
		return fiber.NewError(500, "gagal membuat token")
	}
	raw := uuid.NewString() + uuid.NewString()
	if err := s.db.Create(&RefreshToken{UserID: u.ID, TokenHash: hash(raw), ExpiresAt: time.Now().Add(s.cfg.RefreshTTL)}).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan sesi")
	}
	c.Cookie(&fiber.Cookie{
		Name: "refresh_token", Value: raw, HTTPOnly: true,
		Secure: s.cfg.Env == "production", SameSite: "Strict", Domain: s.cfg.CookieDomain,
		Expires: time.Now().Add(s.cfg.RefreshTTL), Path: "/api/auth",
	})
	s.audit(&u.ID, "login", "orang_tua", "")
	return c.JSON(fiber.Map{
		"accessToken": access,
		"user":        u,
	})
}

// listAnakOrangTua — GET /orang-tua/anak
func (s *Server) listAnakOrangTua(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil {
		return fiber.NewError(401, "unauthorized")
	}
	if u.Role != "orang_tua" || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var anak []PesertaDidik
	s.db.Preload("Kelas").Preload("Kelas.Pokjar").Where("orang_tua_id = ?", *u.OrangTuaID).Find(&anak)
	return c.JSON(anak)
}

// getNilaiAnak — GET /orang-tua/anak/:id/nilai
func (s *Server) getNilaiAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	// Verify this child belongs to this parent
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	var rekap []RekapNilaiAkhir
	s.db.Preload("Mapel").Where("peserta_didik_id = ?", anakID).Find(&rekap)
	return c.JSON(rekap)
}

// getPresensiAnak — GET /orang-tua/anak/:id/presensi
func (s *Server) getPresensiAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	var details []PresensiDetail
	s.db.Preload("Presensi").Preload("Presensi.Kelas").
		Where("peserta_didik_id = ?", anakID).
		Order("created_at desc").
		Limit(100).
		Find(&details)
	return c.JSON(details)
}

// getRaporAnak — GET /orang-tua/anak/:id/rapor
func (s *Server) getRaporAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	// Return rekap + catatan rapor
	type raporRes struct {
		RekapNilai   []RekapNilaiAkhir `json:"rekapNilai"`
		CatatanRapor *CatatanRapor     `json:"catatanRapor"`
	}
	var rekap []RekapNilaiAkhir
	s.db.Preload("Mapel").Where("peserta_didik_id = ?", anakID).Find(&rekap)
	var catatan CatatanRapor
	// Get latest catatan
	s.db.Where("peserta_didik_id = ?", anakID).Order("created_at desc").First(&catatan)
	return c.JSON(raporRes{RekapNilai: rekap, CatatanRapor: &catatan})
}

// --- Helper: verify child belongs to parent ---
func (s *Server) verifyOrangTuaAnak(c *fiber.Ctx, anakID string) (*User, error) {
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return nil, fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return nil, fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	return &u, nil
}

// getUjianSkorAnak — GET /orang-tua/anak/:id/ujian-skor
func (s *Server) getUjianSkorAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pesertas []UjianPeserta
	s.db.Preload("Ujian").Preload("Ujian.Mapel").
		Where("peserta_didik_id = ? AND status = ?", anakID, "selesai").
		Order("created_at desc").
		Find(&pesertas)
	return c.JSON(pesertas)
}

// getTugasAnak — GET /orang-tua/anak/:id/tugas
func (s *Server) getTugasAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pd PesertaDidik
	s.db.First(&pd, "id = ?", anakID)
	var tugasList []Tugas
	s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("deadline desc").Find(&tugasList)
	// Get all submissions for this student in one query; the old loop caused an
	// extra database round-trip for every task in the parent portal.
	tugasIDs := make([]string, 0, len(tugasList))
	for _, tugas := range tugasList {
		tugasIDs = append(tugasIDs, tugas.ID)
	}
	var submissions []PengumpulanTugas
	if len(tugasIDs) > 0 {
		s.db.Where("peserta_didik_id = ? AND tugas_id IN ?", anakID, tugasIDs).Find(&submissions)
	}
	submissionByTask := make(map[string]PengumpulanTugas, len(submissions))
	for _, submission := range submissions {
		submissionByTask[submission.TugasID] = submission
	}
	type tugasRes struct {
		Tugas
		StatusPengumpulan string     `json:"statusPengumpulan"`
		Nilai             *float64   `json:"nilai"`
		TanggalKumpul     *time.Time `json:"tanggalKumpul"`
	}
	var result []tugasRes
	for _, t := range tugasList {
		tr := tugasRes{Tugas: t, StatusPengumpulan: "belum"}
		if pk, ok := submissionByTask[t.ID]; ok {
			tr.StatusPengumpulan = pk.Status
			tr.Nilai = pk.Nilai
			tr.TanggalKumpul = &pk.TanggalKumpul
		}
		result = append(result, tr)
	}
	return c.JSON(result)
}

// getMateriAnak — GET /orang-tua/anak/:id/materi
func (s *Server) getMateriAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pd PesertaDidik
	s.db.First(&pd, "id = ?", anakID)
	var materiList []Materi
	s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("created_at desc").Find(&materiList)
	return c.JSON(materiList)
}

// getPeminjamanAnak — GET /orang-tua/anak/:id/peminjaman
func (s *Server) getPeminjamanAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var peminjaman []Peminjaman
	s.db.Preload("Buku").Where("peserta_didik_id = ?", anakID).Order("created_at desc").Find(&peminjaman)
	return c.JSON(peminjaman)
}

// listChatAnak — GET /orang-tua/anak/:id/chat
func (s *Server) listChatAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var messages []ChatMessage
	s.db.Where("(pengirim_user_id = ? OR penerima_user_id = ?) AND peserta_didik_id = ?", uid, uid, anakID).
		Order("created_at asc").
		Limit(200).
		Find(&messages)
	return c.JSON(messages)
}

// sendChatAnak — POST /orang-tua/anak/:id/chat {isi}
func (s *Server) sendChatAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var in struct {
		Isi string `json:"isi"`
	}
	if e := c.BodyParser(&in); e != nil || strings.TrimSpace(in.Isi) == "" {
		return fiber.NewError(400, "pesan wajib diisi")
	}
	// Find wali kelas as recipient
	var pd PesertaDidik
	s.db.Preload("Kelas").First(&pd, "id = ?", anakID)
	if pd.KelasID == "" {
		return fiber.NewError(400, "siswa tidak memiliki kelas")
	}
	var kelas Kelas
	s.db.First(&kelas, "id = ?", pd.KelasID)
	if kelas.WaliKelasID == nil {
		return fiber.NewError(400, "kelas belum memiliki wali kelas")
	}
	var tutor Tutor
	s.db.First(&tutor, "id = ?", *kelas.WaliKelasID)
	if tutor.UserID == nil {
		return fiber.NewError(400, "wali kelas belum memiliki akun")
	}
	msg := ChatMessage{
		PesertaDidikID: anakID,
		PengirimUserID: uid,
		PenerimaUserID: *tutor.UserID,
		Isi:            strings.TrimSpace(in.Isi),
	}
	s.db.Create(&msg)
	// Push notifikasi to guru wali
	s.pushNotifikasi(*tutor.UserID, "Pesan dari Orang Tua", in.Isi, "chat", &msg.ID)
	return c.Status(201).JSON(msg)
}

// getPerilakuAnak — GET /orang-tua/anak/:id/perilaku
func (s *Server) getPerilakuAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var catatan []CatatanPerilaku
	s.db.Where("peserta_didik_id = ?", anakID).Order("tanggal desc").Find(&catatan)
	return c.JSON(catatan)
}

// ============================================================================
// Ujian/AksesKode — update createUjian to support aksesKode
// ============================================================================

// Helper to notify users of new ujian (for Notifikasi system)
func (s *Server) notifyNewUjian(uj *Ujian) {
	// Find all users in the same class
	var kelas Kelas
	s.db.First(&kelas, "id = ?", uj.KelasID)
	// Notify tutor wali kelas
	if kelas.WaliKelasID != nil {
		var tutor Tutor
		s.db.First(&tutor, "id = ?", *kelas.WaliKelasID)
		if tutor.UserID != nil {
			s.db.Create(&Notifikasi{
				UserID: *tutor.UserID,
				Judul:  "Ujian Online Baru",
				Isi:    fmt.Sprintf("Ujian \"%s\" telah dibuat untuk kelas %d%s", uj.Judul, kelas.Jenjang, kelas.NamaRombel),
				Tipe:   "ujian",
				RefID:  &uj.ID,
			})
		}
	}
}

// Helper to send push notification (placeholder for future WebSocket/SSE)
func (s *Server) pushNotifikasi(userID string, judul, isi, tipe string, refID *string) {
	s.db.Create(&Notifikasi{
		UserID: userID,
		Judul:  judul,
		Isi:    isi,
		Tipe:   tipe,
		RefID:  refID,
	})
}
