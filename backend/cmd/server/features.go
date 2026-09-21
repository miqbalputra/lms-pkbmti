package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================

// ============================================================================
// Ujian Online — public endpoints. The first request authenticates with NISN +
// AksesKode and receives a short-lived, HttpOnly cookie. Legacy clients may
// continue sending the two fields, but the browser page uses the cookie for
// every subsequent request so credentials do not appear in URLs or logs.
// ============================================================================

const (
	examSessionCookie = "exam_session"
	examSessionTTL    = 12 * time.Hour
)

// Public exam responses use allowlisted DTOs rather than serializing database
// models. Ujian contains the shared access code, while the persistence models
// may gain other internal fields over time.
type publicExamMapel struct {
	ID        string `json:"id"`
	NamaMapel string `json:"namaMapel"`
	KodeMapel string `json:"kodeMapel"`
}

type publicExamMeta struct {
	ID           string          `json:"id"`
	Judul        string          `json:"judul"`
	WaktuMulai   time.Time       `json:"waktuMulai"`
	WaktuSelesai time.Time       `json:"waktuSelesai"`
	Mapel        publicExamMapel `json:"mapel"`
}

type publicExamItem struct {
	ID               string          `json:"id"`
	Judul            string          `json:"judul"`
	WaktuMulai       time.Time       `json:"waktuMulai"`
	WaktuSelesai     time.Time       `json:"waktuSelesai"`
	DurasiMenit      int             `json:"durasiMenit"`
	GracePeriodMenit int             `json:"gracePeriodMenit"`
	AcakSoal         bool            `json:"acakSoal"`
	Mapel            publicExamMapel `json:"mapel"`
	SudahMengerjakan bool            `json:"sudahMengerjakan"`
	Skor             *float64        `json:"skor"`
}

type publicParentExamResult struct {
	ID      string         `json:"id"`
	UjianID string         `json:"ujianId"`
	Mulai   *time.Time     `json:"mulai"`
	Selesai *time.Time     `json:"selesai"`
	Skor    *float64       `json:"skor"`
	Status  string         `json:"status"`
	Ujian   publicExamMeta `json:"ujian"`
}

type examMonitorStudent struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
	NIS  string `json:"nis"`
}

type examMonitorItem struct {
	ID             string             `json:"id"`
	UjianID        string             `json:"ujianId"`
	PesertaDidikID string             `json:"pesertaDidikId"`
	Mulai          *time.Time         `json:"mulai"`
	Selesai        *time.Time         `json:"selesai"`
	Skor           *float64           `json:"skor"`
	Status         string             `json:"status"`
	TabSwitch      int                `json:"tabSwitch"`
	PesertaDidik   examMonitorStudent `json:"pesertaDidik"`
}

// Parent portal responses also use narrow read models. Besides keeping the
// contract stable, this prevents internal actor IDs, file paths, share tokens,
// and signature payloads from being exposed merely because a persistence model
// gained a json tag in the future.
type parentMapelSummary struct {
	ID        string `json:"id"`
	NamaMapel string `json:"namaMapel"`
	KodeMapel string `json:"kodeMapel"`
}

type parentClassSummary struct {
	ID         string `json:"id"`
	Jenjang    int    `json:"jenjang"`
	NamaRombel string `json:"namaRombel"`
	Pokjar     struct {
		ID         string `json:"id"`
		NamaPokjar string `json:"namaPokjar"`
	} `json:"pokjar"`
}

type parentChildSummary struct {
	ID               string             `json:"id"`
	Nama             string             `json:"nama"`
	JenisKelamin     string             `json:"jenisKelamin"`
	NIS              string             `json:"nis"`
	NISN             string             `json:"nisn"`
	Status           string             `json:"status"`
	IdentitasFileExt *string            `json:"identitasFileExt"`
	Kelas            parentClassSummary `json:"kelas"`
}

type parentTugasItem struct {
	ID                string             `json:"id"`
	MapelID           string             `json:"mapelId"`
	KelasID           string             `json:"kelasId"`
	Judul             string             `json:"judul"`
	Deskripsi         string             `json:"deskripsi"`
	Deadline          time.Time          `json:"deadline"`
	Semester          string             `json:"semester"`
	BolehUpload       bool               `json:"bolehUpload"`
	ModulID           *string            `json:"modulId"`
	CreatedAt         time.Time          `json:"createdAt"`
	Mapel             parentMapelSummary `json:"mapel"`
	StatusPengumpulan string             `json:"statusPengumpulan"`
	Nilai             *float64           `json:"nilai"`
	TanggalKumpul     *time.Time         `json:"tanggalKumpul"`
}

type parentMateriItem struct {
	ID        string             `json:"id"`
	MapelID   string             `json:"mapelId"`
	KelasID   string             `json:"kelasId"`
	Judul     string             `json:"judul"`
	Deskripsi string             `json:"deskripsi"`
	Tipe      string             `json:"tipe"`
	Ukuran    int64              `json:"ukuran"`
	Semester  string             `json:"semester"`
	ModulID   *string            `json:"modulId"`
	Urutan    int                `json:"urutan"`
	Tanggal   *time.Time         `json:"tanggal"`
	LinkURL   string             `json:"linkUrl"`
	CreatedAt time.Time          `json:"createdAt"`
	Mapel     parentMapelSummary `json:"mapel"`
}

type parentPeminjamanItem struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	BukuID         string    `json:"bukuId"`
	KelasID        string    `json:"kelasId"`
	Semester       string    `json:"semester"`
	TanggalPinjam  time.Time `json:"tanggalPinjam"`
	Status         string    `json:"status"`
	Buku           struct {
		ID       string `json:"id"`
		Judul    string `json:"judul"`
		KodeBuku string `json:"kodeBuku"`
		Penerbit string `json:"penerbit"`
	} `json:"buku"`
}

type parentPresensiItem struct {
	ID              string     `json:"id"`
	PresensiID      string     `json:"presensiId"`
	StatusKehadiran string     `json:"statusKehadiran"`
	Catatan         string     `json:"catatan"`
	Tanggal         *time.Time `json:"tanggal"`
	Semester        string     `json:"semester"`
}

type parentBehaviorItem struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	KelasID        string    `json:"kelasId"`
	Tanggal        time.Time `json:"tanggal"`
	Kategori       string    `json:"kategori"`
	Deskripsi      string    `json:"deskripsi"`
}

type parentGradeItem struct {
	ID             string             `json:"id"`
	PesertaDidikID string             `json:"pesertaDidikId"`
	KelasID        string             `json:"kelasId"`
	MapelID        string             `json:"mapelId"`
	TahunAjaranID  string             `json:"tahunAjaranId"`
	Semester       string             `json:"semester"`
	NPAkhir        *float64           `json:"npAkhir"`
	PredikatNP     string             `json:"predikatNP"`
	NKAkhir        *float64           `json:"nkAkhir"`
	PredikatNK     string             `json:"predikatNK"`
	NAAkhir        *float64           `json:"naAkhir"`
	PredikatNA     string             `json:"predikatNA"`
	Mapel          parentMapelSummary `json:"mapel"`
}

type parentReportNote struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	TahunAjaranID  string    `json:"tahunAjaranId"`
	Semester       string    `json:"semester"`
	CatatanWali    string    `json:"catatanWali"`
	NaikKelas      *bool     `json:"naikKelas"`
	KenaikanKe     *string   `json:"kenaikanKe"`
	CreatedAt      time.Time `json:"createdAt"`
}

type parentReportResponse struct {
	RekapNilai   []parentGradeItem `json:"rekapNilai"`
	CatatanRapor *parentReportNote `json:"catatanRapor"`
}

type parentChatItem struct {
	ID         string     `json:"id"`
	Isi        string     `json:"isi"`
	Dibaca     bool       `json:"dibaca"`
	DibacaPada *time.Time `json:"dibacaPada"`
	CreatedAt  time.Time  `json:"createdAt"`
	IsMine     bool       `json:"isMine"`
}

type notificationItem struct {
	ID         string     `json:"id"`
	Judul      string     `json:"judul"`
	Isi        string     `json:"isi"`
	Tipe       string     `json:"tipe"`
	RefID      *string    `json:"refId"`
	IsRead     bool       `json:"isRead"`
	DibacaPada *time.Time `json:"dibacaPada"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type calendarEventItem struct {
	ID             string            `json:"id"`
	Judul          string            `json:"judul"`
	Deskripsi      string            `json:"deskripsi"`
	TanggalMulai   time.Time         `json:"tanggalMulai"`
	TanggalSelesai *time.Time        `json:"tanggalSelesai"`
	Tipe           string            `json:"tipe"`
	Warna          string            `json:"warna"`
	Semester       *string           `json:"semester"`
	TahunAjaranID  *string           `json:"tahunAjaranId"`
	TahunAjaran    *calendarYearItem `json:"tahunAjaran,omitempty"`
}

type calendarYearItem struct {
	ID                        string     `json:"id"`
	NamaTahunAjaran           string     `json:"namaTahunAjaran"`
	TanggalMulai              time.Time  `json:"tanggalMulai"`
	TanggalSelesai            time.Time  `json:"tanggalSelesai"`
	TanggalMulaiSemesterGenap *time.Time `json:"tanggalMulaiSemesterGenap"`
	IsAktif                   bool       `json:"isAktif"`
}

func notificationItems(rows []Notifikasi) []notificationItem {
	result := make([]notificationItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, notificationItem{
			ID: row.ID, Judul: row.Judul, Isi: row.Isi, Tipe: row.Tipe, RefID: row.RefID,
			IsRead: row.IsRead, DibacaPada: row.DibacaPada, CreatedAt: row.CreatedAt,
		})
	}
	return result
}

func loadParentGradeItems(db *gorm.DB, rows []RekapNilaiAkhir) ([]parentGradeItem, error) {
	mapelIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.MapelID == "" {
			continue
		}
		if _, ok := seen[row.MapelID]; ok {
			continue
		}
		seen[row.MapelID] = struct{}{}
		mapelIDs = append(mapelIDs, row.MapelID)
	}
	mapelByID := make(map[string]MataPelajaran, len(mapelIDs))
	if len(mapelIDs) > 0 {
		var mapel []MataPelajaran
		if err := db.Where("id IN ?", mapelIDs).Find(&mapel).Error; err != nil {
			return nil, err
		}
		for _, row := range mapel {
			mapelByID[row.ID] = row
		}
	}
	result := make([]parentGradeItem, 0, len(rows))
	for _, row := range rows {
		mapel := mapelByID[row.MapelID]
		result = append(result, parentGradeItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, KelasID: row.KelasID,
			MapelID: row.MapelID, TahunAjaranID: row.TahunAjaranID, Semester: row.Semester,
			NPAkhir: row.NPAkhir, PredikatNP: row.PredikatNP, NKAkhir: row.NKAkhir,
			PredikatNK: row.PredikatNK, NAAkhir: row.NAAkhir, PredikatNA: row.PredikatNA,
			Mapel: parentMapelSummary{ID: mapel.ID, NamaMapel: mapel.NamaMapel, KodeMapel: mapel.KodeMapel},
		})
	}
	return result, nil
}

func (s *Server) issueExamSession(c *fiber.Ctx, pd *PesertaDidik, accessCode string) error {
	if pd == nil || pd.ID == "" || strings.TrimSpace(accessCode) == "" || strings.TrimSpace(s.cfg.AccessSecret) == "" {
		return errors.New("sesi ujian tidak dapat diterbitkan")
	}
	expiresAt := time.Now().Add(examSessionTTL)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": pd.ID,
		"typ": "public_exam",
		"ach": hash(strings.TrimSpace(accessCode)),
		"iat": time.Now().Unix(),
		"exp": expiresAt.Unix(),
	}).SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     examSessionCookie,
		Value:    token,
		HTTPOnly: true,
		Secure:   s.cfg.Env == "production",
		SameSite: "Strict",
		Domain:   s.cfg.CookieDomain,
		Expires:  expiresAt,
		MaxAge:   int(examSessionTTL / time.Second),
		Path:     "/api/ujian-online",
	})
	return nil
}

func (s *Server) parseExamSession(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.TrimSpace(s.cfg.AccessSecret) == "" {
		return "", "", errors.New("sesi ujian kosong")
	}
	token, err := jwt.ParseWithClaims(raw, jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.AccessSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || token == nil || !token.Valid {
		return "", "", errors.New("sesi ujian tidak valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["typ"] != "public_exam" {
		return "", "", errors.New("tipe sesi ujian tidak valid")
	}
	studentID, ok := claims["sub"].(string)
	accessHash, hashOK := claims["ach"].(string)
	if !ok || strings.TrimSpace(studentID) == "" || !hashOK || len(accessHash) != 64 {
		return "", "", errors.New("klaim sesi ujian tidak valid")
	}
	return studentID, accessHash, nil
}

func (s *Server) clearExamSession(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     examSessionCookie,
		Value:    "",
		HTTPOnly: true,
		Secure:   s.cfg.Env == "production",
		SameSite: "Strict",
		Domain:   s.cfg.CookieDomain,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		Path:     "/api/ujian-online",
	})
}

func (s *Server) logoutUjianOnline(c *fiber.Ctx) error {
	s.clearExamSession(c)
	return c.SendStatus(204)
}

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
	if nisn == "" && kode == "" {
		studentID, accessHash, err := s.parseExamSession(c.Cookies(examSessionCookie))
		if err == nil {
			var pd PesertaDidik
			if loadErr := s.db.Preload("Kelas").Where("id = ? AND status = ?", studentID, "aktif").First(&pd).Error; loadErr == nil {
				c.Locals("examAccessCodeHash", accessHash)
				return &pd, nil
			}
		}
	}
	if nisn == "" || kode == "" {
		return nil, fiber.NewError(400, "NISN dan Kode Akses wajib diisi")
	}
	if nisn == temporaryNISN {
		// The placeholder is intentionally shared by several learners while they
		// wait for an official NISN. It cannot identify one exam participant, so
		// never allow it to authenticate a public exam session.
		return nil, fiber.NewError(401, "NISN sementara belum dapat digunakan untuk ujian online")
	}
	var pd PesertaDidik
	if s.db.Preload("Kelas").Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return nil, fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	c.Locals("examAccessCodeHash", hash(kode))
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
	if sessionHash, ok := c.Locals("examAccessCodeHash").(string); ok && sessionHash != "" {
		if subtle.ConstantTimeCompare([]byte(sessionHash), []byte(hash(strings.TrimSpace(uj.AksesKode)))) != 1 {
			return nil, nil, fiber.NewError(403, "Kode akses salah")
		}
	} else if strings.TrimSpace(uj.AksesKode) != strings.TrimSpace(c.Query("aksesKode")) &&
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
	if nisn == temporaryNISN {
		return fiber.NewError(401, "NISN sementara belum dapat digunakan untuk ujian online")
	}
	if e := s.requireTurnstile(c, turnstileToken); e != nil {
		return e
	}
	var pd PesertaDidik
	if s.db.Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	var ujians []Ujian
	queryErr := s.db.Preload("Mapel").Preload("Kelas").
		Where("akses_kode = ? AND kelas_id = ?", strings.TrimSpace(kode), pd.KelasID).
		Where("waktu_mulai <= ? AND waktu_selesai >= ?", time.Now(), time.Now()).
		Order("waktu_mulai desc").
		Find(&ujians).Error
	if queryErr != nil {
		return fiber.NewError(500, "Gagal memuat daftar ujian")
	}
	if len(ujians) == 0 {
		return fiber.NewError(404, "Tidak ada ujian aktif dengan kode akses tersebut untuk kelas Anda")
	}
	if err := s.issueExamSession(c, &pd, kode); err != nil {
		return fiber.NewError(500, "Gagal menyiapkan sesi ujian")
	}
	// Load all existing sessions in one query instead of one query per exam.
	ujianIDs := make([]string, 0, len(ujians))
	for _, uj := range ujians {
		ujianIDs = append(ujianIDs, uj.ID)
	}
	var sessions []UjianPeserta
	if err := s.db.Where("peserta_didik_id = ? AND ujian_id IN ?", pd.ID, ujianIDs).Find(&sessions).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat status ujian")
	}
	sessionByExam := make(map[string]UjianPeserta, len(sessions))
	for _, session := range sessions {
		sessionByExam[session.UjianID] = session
	}
	res := make([]publicExamItem, 0, len(ujians))
	for _, uj := range ujians {
		r := publicExamItem{
			ID: uj.ID, Judul: uj.Judul, WaktuMulai: uj.WaktuMulai,
			WaktuSelesai: uj.WaktuSelesai, DurasiMenit: uj.DurasiMenit,
			GracePeriodMenit: uj.GracePeriodMenit, AcakSoal: uj.AcakSoal,
			Mapel: publicExamMapel{ID: uj.Mapel.ID, NamaMapel: uj.Mapel.NamaMapel, KodeMapel: uj.Mapel.KodeMapel},
		}
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
	findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error
	if findErr == nil {
		return c.JSON(up)
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return fiber.NewError(500, "Gagal memuat sesi ujian")
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

// getSoalUjianOnline — GET /ujian-online/:ujianId/soal with the signed exam
// session cookie issued by /cek (legacy credential query/form input remains
// accepted by the server for older clients).
// Returns shuffled soal list (without answers) for the exam.
func (s *Server) getSoalUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	// Check or create UjianPeserta
	var up UjianPeserta
	findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error
	if findErr != nil {
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fiber.NewError(500, "Gagal memuat sesi ujian")
		}
		now := time.Now()
		up = UjianPeserta{UjianID: uj.ID, PesertaDidikID: pd.ID, Mulai: &now, Status: "mulai"}
		if createErr := s.db.Create(&up).Error; createErr != nil && !isUniqueErr(createErr) {
			return fiber.NewError(500, "Gagal membuat sesi ujian")
		}
		if up.ID == "" {
			if lookupErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; lookupErr != nil {
				return fiber.NewError(500, "Gagal memuat sesi ujian")
			}
		}
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Anda sudah menyelesaikan ujian ini")
	}
	// Check if time is up (beyond grace period)
	if up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			// Hard lock: grace period expired
			up.Status = "selesai"
			now := time.Now()
			up.Selesai = &now
			grade, gradeErr := s.gradeUjianPesertaResult(&up, uj)
			if gradeErr != nil {
				return fiber.NewError(500, "Gagal menghitung nilai ujian")
			}
			up.Skor = &grade.Score
			if saveErr := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Updates(map[string]interface{}{
				"status": "selesai", "selesai": now, "skor": grade.Score,
			}).Error; saveErr != nil {
				return fiber.NewError(500, "Gagal menyimpan hasil ujian")
			}
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
	}
	var us []UjianSoal
	if err := s.db.Preload("Soal").Where("ujian_id = ?", uj.ID).Order("created_at").Find(&us).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat soal ujian")
	}
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
	if err := s.db.Where("ujian_peserta_id = ?", up.ID).Find(&jawabans).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat jawaban ujian")
	}
	ujianSoalIDByBankSoalID := make(map[string]string, len(order))
	for _, item := range order {
		ujianSoalIDByBankSoalID[item.SoalID] = item.ID
	}
	jawabanList := make([]jawabanRes, 0, len(jawabans))
	for _, j := range jawabans {
		ujianSoalID, ok := ujianSoalIDByBankSoalID[j.SoalID]
		if !ok {
			continue
		}
		jawabanList = append(jawabanList, jawabanRes{UjianSoalID: ujianSoalID, Jawaban: j.Jawaban})
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
	if findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; findErr != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	// Check time (beyond grace period = hard lock)
	if up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			up.Status = "selesai"
			now := time.Now()
			up.Selesai = &now
			grade, gradeErr := s.gradeUjianPesertaResult(&up, uj)
			if gradeErr != nil {
				return fiber.NewError(500, "Gagal menghitung nilai ujian")
			}
			up.Skor = &grade.Score
			if saveErr := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Updates(map[string]interface{}{
				"status": "selesai", "selesai": now, "skor": grade.Score,
			}).Error; saveErr != nil {
				return fiber.NewError(500, "Gagal menyimpan hasil ujian")
			}
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
	in.Jawaban = strings.TrimSpace(in.Jawaban)
	if len([]byte(in.Jawaban)) > 64*1024 {
		return fiber.NewError(413, "jawaban terlalu panjang")
	}
	// Verify the soal belongs to this ujian
	var us UjianSoal
	if s.db.Where("ujian_id = ? AND id = ?", uj.ID, in.UjianSoalID).First(&us).Error != nil {
		return fiber.NewError(400, "Soal tidak ditemukan dalam ujian ini")
	}
	// Upsert jawaban
	var jawaban UjianJawaban
	findAnswerErr := s.db.Where("ujian_peserta_id = ? AND soal_id = ?", up.ID, us.SoalID).First(&jawaban).Error
	if findAnswerErr == nil {
		jawaban.Jawaban = in.Jawaban
		if e := s.db.Model(&UjianJawaban{}).Where("id = ?", jawaban.ID).Updates(map[string]interface{}{"jawaban": jawaban.Jawaban}).Error; e != nil {
			return fiber.NewError(500, "Gagal menyimpan jawaban")
		}
	} else if errors.Is(findAnswerErr, gorm.ErrRecordNotFound) {
		jawaban = UjianJawaban{
			UjianPesertaID: up.ID,
			SoalID:         us.SoalID,
			Jawaban:        in.Jawaban,
		}
		if e := s.db.Create(&jawaban).Error; e != nil {
			// Concurrent tabs may both submit the first answer. Let the unique
			// index choose the row, then update that winner instead of returning
			// a transient 500 to the student.
			if !isUniqueErr(e) {
				return fiber.NewError(500, "Gagal menyimpan jawaban")
			}
			if lookupErr := s.db.Where("ujian_peserta_id = ? AND soal_id = ?", up.ID, us.SoalID).First(&jawaban).Error; lookupErr != nil {
				return fiber.NewError(500, "Gagal memuat jawaban ujian")
			}
			if updateErr := s.db.Model(&UjianJawaban{}).Where("id = ?", jawaban.ID).Updates(map[string]interface{}{"jawaban": in.Jawaban}).Error; updateErr != nil {
				return fiber.NewError(500, "Gagal menyimpan jawaban")
			}
		}
	} else {
		return fiber.NewError(500, "Gagal memuat jawaban ujian")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// gradeUjianPeserta auto-grades all answers for a participant and computes the
// final score. Used by selesaiUjianOnline (manual finish) and
// autoFinishUjianSessions (server-side timeout/tab-lock).
type ujianGradeResult struct {
	Score   float64
	Correct int
	Total   int
}

func (s *Server) gradeUjianPesertaResult(up *UjianPeserta, uj *Ujian) (ujianGradeResult, error) {
	var result ujianGradeResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var ujianSoals []UjianSoal
		if err := tx.Preload("Soal").Where("ujian_id = ?", uj.ID).Find(&ujianSoals).Error; err != nil {
			return err
		}
		var jawabans []UjianJawaban
		if err := tx.Where("ujian_peserta_id = ?", up.ID).Find(&jawabans).Error; err != nil {
			return err
		}

		soalLookup := map[string]UjianSoal{}
		for _, us := range ujianSoals {
			soalLookup[us.SoalID] = us
		}

		totalSkor := 0.0
		totalBobot := 0.0
		for _, us := range ujianSoals {
			if us.Bobot < 0 {
				return errors.New("bobot soal tidak valid")
			}
			totalBobot += us.Bobot
		}
		result = ujianGradeResult{Total: len(ujianSoals)}
		for i := range jawabans {
			us, ok := soalLookup[jawabans[i].SoalID]
			if !ok {
				continue
			}
			kunci := strings.TrimSpace(us.Soal.Kunci)
			jawaban := strings.TrimSpace(jawabans[i].Jawaban)
			if kunci == "" || jawaban == "" {
				if err := tx.Model(&UjianJawaban{}).Where("id = ?", jawabans[i].ID).Updates(map[string]interface{}{"benar": nil, "nilai": 0}).Error; err != nil {
					return err
				}
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
				result.Correct++
			} else {
				jawabans[i].Nilai = 0
			}
			if err := tx.Model(&UjianJawaban{}).Where("id = ?", jawabans[i].ID).Updates(map[string]interface{}{"benar": jawabans[i].Benar, "nilai": jawabans[i].Nilai}).Error; err != nil {
				return err
			}
		}

		if totalBobot > 0 {
			result.Score = (totalSkor / totalBobot) * 100
		}
		return nil
	})
	return result, err
}

func (s *Server) gradeUjianPeserta(up *UjianPeserta, uj *Ujian) float64 {
	result, err := s.gradeUjianPesertaResult(up, uj)
	if err != nil {
		return 0
	}
	return result.Score
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
	if err := s.db.Preload("Ujian").Where("status = ? AND mulai IS NOT NULL", "mulai").Find(&pesertas).Error; err != nil {
		operationLog("exam_auto_finish_query_failed", map[string]any{})
		return
	}
	now := time.Now()
	for _, up := range pesertas {
		if up.Ujian.DurasiMenit == 0 || up.Mulai == nil {
			continue
		}
		// Only auto-finish after the FULL grace period has expired
		if now.After(batasGrace(&up, &up.Ujian)) {
			up.Selesai = &now
			up.Status = "selesai"
			grade, err := s.gradeUjianPesertaResult(&up, &up.Ujian)
			if err != nil {
				continue
			}
			up.Skor = &grade.Score
			if err := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Updates(map[string]interface{}{
				"status": "selesai", "selesai": now, "skor": grade.Score,
			}).Error; err != nil {
				operationLog("exam_auto_finish_update_failed", map[string]any{})
			}
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

	grade, gradeErr := s.gradeUjianPesertaResult(&up, uj)
	if gradeErr != nil {
		return fiber.NewError(500, "Gagal menghitung nilai ujian")
	}

	up.Skor = &grade.Score
	if saveErr := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Updates(map[string]interface{}{
		"status": "selesai", "selesai": now, "skor": grade.Score,
	}).Error; saveErr != nil {
		return fiber.NewError(500, "Gagal menyimpan hasil ujian")
	}
	return c.JSON(fiber.Map{
		"skor":   grade.Score,
		"benar":  grade.Correct,
		"total":  grade.Total,
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
		grade, gradeErr := s.gradeUjianPesertaResult(&up, uj)
		if gradeErr != nil {
			return fiber.NewError(500, "Gagal menghitung nilai ujian")
		}
		up.Skor = &grade.Score
		if saveErr := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Updates(map[string]interface{}{
			"tab_switch": up.TabSwitch, "status": "dikunci", "selesai": now, "skor": grade.Score,
		}).Error; saveErr != nil {
			return fiber.NewError(500, "Gagal menyimpan status ujian")
		}
		return c.JSON(fiber.Map{"tabSwitch": up.TabSwitch, "locked": true, "skor": grade.Score})
	}
	if err := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Update("tab_switch", up.TabSwitch).Error; err != nil {
		return fiber.NewError(500, "Gagal menyimpan status ujian")
	}
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
	if err := s.db.Preload("PesertaDidik").Where("ujian_id = ?", ujianID).Find(&pesertas).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat peserta ujian")
	}
	result := make([]examMonitorItem, 0, len(pesertas))
	for _, peserta := range pesertas {
		result = append(result, examMonitorItem{
			ID: peserta.ID, UjianID: peserta.UjianID, PesertaDidikID: peserta.PesertaDidikID,
			Mulai: peserta.Mulai, Selesai: peserta.Selesai, Skor: peserta.Skor,
			Status: peserta.Status, TabSwitch: peserta.TabSwitch,
			PesertaDidik: examMonitorStudent{ID: peserta.PesertaDidik.ID, Nama: peserta.PesertaDidik.Nama, NIS: peserta.PesertaDidik.NIS},
		})
	}
	return c.JSON(result)
}

// ============================================================================
// Notifikasi — protected (JWT)
// ============================================================================

func (s *Server) listNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var rows []Notifikasi
	if err := s.db.Where("user_id = ?", uid).Order("created_at desc").Limit(50).Find(&rows).Error; err != nil {
		return fiber.NewError(500, "gagal memuat notifikasi")
	}
	return c.JSON(notificationItems(rows))
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

// issueNotificationStreamTicket creates a short-lived, one-time credential for
// EventSource. Browser EventSource cannot set an Authorization header, but a
// long-lived JWT in a URL would leak through browser history and proxy logs.
func (s *Server) issueNotificationStreamTicket(c *fiber.Ctx) error {
	uid, ok := c.Locals("userID").(string)
	if !ok || strings.TrimSpace(uid) == "" {
		return fiber.NewError(401, "sesi tidak valid")
	}
	ticket := uuid.NewString() + uuid.NewString()
	now := time.Now()
	s.streamTicketsMu.Lock()
	if s.streamTickets == nil {
		s.streamTickets = make(map[string]notificationStreamTicket)
	}
	for key, value := range s.streamTickets {
		if !value.ExpiresAt.After(now) {
			delete(s.streamTickets, key)
		}
	}
	s.streamTickets[ticket] = notificationStreamTicket{UserID: uid, ExpiresAt: now.Add(60 * time.Second)}
	s.streamTicketsMu.Unlock()
	return c.JSON(fiber.Map{"ticket": ticket, "expiresInSeconds": 60})
}

func (s *Server) consumeNotificationStreamTicket(ticket string) (string, bool) {
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	value, ok := s.streamTickets[ticket]
	if ok {
		delete(s.streamTickets, ticket)
	}
	return value.UserID, ok && value.ExpiresAt.After(time.Now())
}

// streamNotifikasi — Server-Sent Events: pushes new notifications to the client
// in real-time. Uses a simple polling loop that checks every 5 seconds.
// The preferred credential is the one-time ticket issued by
// /notifikasi/stream-ticket. Authorization remains supported for non-browser
// clients that can set headers; JWT query parameters are intentionally rejected.
func (s *Server) streamNotifikasi(c *fiber.Ctx) error {
	uid := ""
	if ticket := strings.TrimSpace(c.Query("ticket")); ticket != "" {
		var ok bool
		uid, ok = s.consumeNotificationStreamTicket(ticket)
		if !ok {
			return fiber.NewError(401, "ticket tidak valid atau sudah digunakan")
		}
	} else {
		if c.Query("token") != "" {
			return fiber.NewError(401, "gunakan stream ticket, bukan token pada URL")
		}
		var err error
		_, uid, _, err = s.parseAccessToken(c.Get("Authorization"))
		if err != nil {
			return fiber.NewError(401, "missing access token")
		}
		var user User
		if err := s.db.Select("id, is_active").First(&user, "id = ?", uid).Error; err != nil || !user.IsActive {
			return fiber.NewError(401, "sesi tidak lagi aktif")
		}
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
			queryErr := s.db.Where("user_id = ? AND created_at > ?", uid, lastCheck).
				Order("created_at desc").Find(&newNotifs)
			if queryErr.Error != nil {
				operationLog("notification_stream_query_failed", map[string]any{})
			} else if len(newNotifs) > 0 {
				data, _ := json.Marshal(notificationItems(newNotifs))
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
		// Filter by month (YYYY-MM) using WIB calendar boundaries.
		if start, err := time.ParseInLocation("2006-01", v, wibLocation); err == nil {
			q = q.Where("tanggal_mulai >= ? AND tanggal_mulai < ?", start, start.AddDate(0, 1, 0))
		}
	}
	var rows []KalenderEvent
	if err := q.Find(&rows).Error; err != nil {
		return fiber.NewError(500, "gagal memuat kalender")
	}
	result := make([]calendarEventItem, 0, len(rows))
	for _, row := range rows {
		item := calendarEventItem{
			ID: row.ID, Judul: row.Judul, Deskripsi: row.Deskripsi, TanggalMulai: row.TanggalMulai,
			TanggalSelesai: row.TanggalSelesai, Tipe: row.Tipe, Warna: row.Warna,
			Semester: row.Semester, TahunAjaranID: row.TahunAjaranID,
		}
		if row.TahunAjaran != nil {
			item.TahunAjaran = &calendarYearItem{
				ID: row.TahunAjaran.ID, NamaTahunAjaran: row.TahunAjaran.NamaTahunAjaran,
				TanggalMulai: row.TahunAjaran.TanggalMulai, TanggalSelesai: row.TahunAjaran.TanggalSelesai,
				TanggalMulaiSemesterGenap: row.TahunAjaran.TanggalMulaiSemesterGenap,
				IsAktif:                   row.TahunAjaran.IsAktif,
			}
		}
		result = append(result, item)
	}
	return c.JSON(result)
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
	in.TanggalMulai = in.TanggalMulai.In(wibLocation)
	if in.TanggalSelesai != nil {
		value := in.TanggalSelesai.In(wibLocation)
		in.TanggalSelesai = &value
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
	if !in.TanggalMulai.IsZero() {
		in.TanggalMulai = in.TanggalMulai.In(wibLocation)
	}
	if in.TanggalSelesai != nil {
		value := in.TanggalSelesai.In(wibLocation)
		in.TanggalSelesai = &value
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
	if in.NISN == temporaryNISN {
		return fiber.NewError(401, "NISN sementara belum dapat digunakan untuk portal orang tua")
	}
	if s.db.Preload("OrangTua").Preload("Kelas").Where("nisn = ? AND status = ?", in.NISN, "aktif").First(&pd).Error != nil {
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
		// Parent access is provisioned through NISN + date of birth. Give the
		// backing User a random unusable password so a leaked username cannot be
		// converted into a second login path using a shared default credential.
		passwordHash, hashErr := bcryptHash(uuid.NewString() + uuid.NewString())
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
	if err := s.db.Preload("Kelas").Preload("Kelas.Pokjar").Where("orang_tua_id = ?", *u.OrangTuaID).Find(&anak).Error; err != nil {
		return fiber.NewError(500, "gagal memuat daftar anak")
	}
	result := make([]parentChildSummary, 0, len(anak))
	for _, child := range anak {
		row := parentChildSummary{
			ID: child.ID, Nama: child.Nama, JenisKelamin: child.JenisKelamin,
			NIS: child.NIS, NISN: child.NISN, Status: child.Status,
			IdentitasFileExt: child.IdentitasFileExt,
			Kelas:            parentClassSummary{ID: child.Kelas.ID, Jenjang: child.Kelas.Jenjang, NamaRombel: child.Kelas.NamaRombel},
		}
		row.Kelas.Pokjar.ID = child.Kelas.Pokjar.ID
		row.Kelas.Pokjar.NamaPokjar = child.Kelas.Pokjar.NamaPokjar
		result = append(result, row)
	}
	return c.JSON(result)
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
	if err := s.db.Where("peserta_didik_id = ?", anakID).Find(&rekap).Error; err != nil {
		return fiber.NewError(500, "gagal memuat nilai anak")
	}
	result, err := loadParentGradeItems(s.db, rekap)
	if err != nil {
		return fiber.NewError(500, "gagal memuat mata pelajaran nilai anak")
	}
	return c.JSON(result)
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
	if err := s.db.
		Where("peserta_didik_id = ?", anakID).
		Order("created_at desc").
		Limit(100).
		Find(&details).Error; err != nil {
		return fiber.NewError(500, "gagal memuat presensi anak")
	}
	presensiIDs := make([]string, 0, len(details))
	for _, detail := range details {
		if detail.PresensiID != "" {
			presensiIDs = append(presensiIDs, detail.PresensiID)
		}
	}
	presensiByID := make(map[string]Presensi, len(presensiIDs))
	if len(presensiIDs) > 0 {
		var presensi []Presensi
		if err := s.db.Select("id, tanggal, semester").Where("id IN ?", presensiIDs).Find(&presensi).Error; err != nil {
			return fiber.NewError(500, "gagal memuat tanggal presensi anak")
		}
		for _, row := range presensi {
			presensiByID[row.ID] = row
		}
	}
	result := make([]parentPresensiItem, 0, len(details))
	for _, detail := range details {
		var tanggal *time.Time
		semester := ""
		if presensi, ok := presensiByID[detail.PresensiID]; ok {
			tanggal = &presensi.Tanggal
			semester = presensi.Semester
		}
		result = append(result, parentPresensiItem{
			ID: detail.ID, PresensiID: detail.PresensiID, StatusKehadiran: detail.StatusKehadiran,
			Catatan: detail.Catatan, Tanggal: tanggal, Semester: semester,
		})
	}
	return c.JSON(result)
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
	var rekap []RekapNilaiAkhir
	if err := s.db.Where("peserta_didik_id = ?", anakID).Find(&rekap).Error; err != nil {
		return fiber.NewError(500, "gagal memuat rapor anak")
	}
	grades, err := loadParentGradeItems(s.db, rekap)
	if err != nil {
		return fiber.NewError(500, "gagal memuat mata pelajaran rapor anak")
	}
	var catatan CatatanRapor
	var note *parentReportNote
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("created_at desc").First(&catatan).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(500, "gagal memuat catatan rapor anak")
		}
	} else {
		note = &parentReportNote{
			ID: catatan.ID, PesertaDidikID: catatan.PesertaDidikID, TahunAjaranID: catatan.TahunAjaranID,
			Semester: catatan.Semester, CatatanWali: catatan.CatatanWali, NaikKelas: catatan.NaikKelas,
			KenaikanKe: catatan.KenaikanKe, CreatedAt: catatan.CreatedAt,
		}
	}
	return c.JSON(parentReportResponse{RekapNilai: grades, CatatanRapor: note})
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
	if err := s.db.Preload("Ujian").Preload("Ujian.Mapel").
		Where("peserta_didik_id = ? AND status = ?", anakID, "selesai").
		Order("created_at desc").
		Find(&pesertas).Error; err != nil {
		return fiber.NewError(500, "gagal memuat hasil ujian anak")
	}
	// UjianPeserta.Ujian contains AksesKode. Parents only need the result
	// summary; never serialize the persistence graph into the portal response.
	result := make([]publicParentExamResult, 0, len(pesertas))
	for _, peserta := range pesertas {
		result = append(result, publicParentExamResult{
			ID: peserta.ID, UjianID: peserta.UjianID, Mulai: peserta.Mulai,
			Selesai: peserta.Selesai, Skor: peserta.Skor, Status: peserta.Status,
			Ujian: publicExamMeta{
				ID: peserta.Ujian.ID, Judul: peserta.Ujian.Judul,
				WaktuMulai: peserta.Ujian.WaktuMulai, WaktuSelesai: peserta.Ujian.WaktuSelesai,
				Mapel: publicExamMapel{ID: peserta.Ujian.Mapel.ID, NamaMapel: peserta.Ujian.Mapel.NamaMapel, KodeMapel: peserta.Ujian.Mapel.KodeMapel},
			},
		})
	}
	return c.JSON(result)
}

type parentSimulasiAttempt struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Nomor        int        `json:"nomor"`
	Mulai        *time.Time `json:"mulai,omitempty"`
	Selesai      *time.Time `json:"selesai,omitempty"`
	Skor         *float64   `json:"skor,omitempty"`
	SkorTersedia bool       `json:"skorTersedia"`
}

type parentSimulasiSummary struct {
	ID                 string                  `json:"id"`
	Nama               string                  `json:"nama"`
	Deskripsi          string                  `json:"deskripsi"`
	Mode               string                  `json:"mode"`
	DurasiMenit        int                     `json:"durasiMenit"`
	MaksPercobaan      int                     `json:"maksPercobaan"`
	WaktuMulai         *time.Time              `json:"waktuMulai,omitempty"`
	WaktuSelesai       *time.Time              `json:"waktuSelesai,omitempty"`
	TampilkanNilai     bool                    `json:"tampilkanNilai"`
	TampilkanRingkasan bool                    `json:"tampilkanRingkasan"`
	Tersedia           bool                    `json:"tersedia"`
	PercobaanTerpakai  int                     `json:"percobaanTerpakai"`
	Percobaan          []parentSimulasiAttempt `json:"percobaan"`
}

// getSimulasiAnak returns only assignment/result summaries. It deliberately
// never serializes questions, snapshots, answer keys, rubrics, or teacher
// comments to the parent portal.
func (s *Server) getSimulasiAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var assignments []SimulasiPenugasan
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("created_at desc").Find(&assignments).Error; err != nil {
		return fiber.NewError(500, "gagal memuat penugasan simulasi anak")
	}
	if len(assignments) == 0 {
		return c.JSON([]parentSimulasiSummary{})
	}
	packageIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		packageIDs = append(packageIDs, assignment.PaketID)
	}
	var packages []SimulasiPaket
	if err := s.db.Where("id IN ? AND status = ?", packageIDs, "terbit").Find(&packages).Error; err != nil {
		return fiber.NewError(500, "gagal memuat paket simulasi anak")
	}
	packagesByID := make(map[string]SimulasiPaket, len(packages))
	for _, paket := range packages {
		packagesByID[paket.ID] = paket
	}
	var attempts []SimulasiUpaya
	if err := s.db.Where("peserta_didik_id = ? AND paket_id IN ?", anakID, packageIDs).Order("nomor desc").Find(&attempts).Error; err != nil {
		return fiber.NewError(500, "gagal memuat riwayat simulasi anak")
	}
	attemptsByPackage := make(map[string][]SimulasiUpaya)
	for _, attempt := range attempts {
		attemptsByPackage[attempt.PaketID] = append(attemptsByPackage[attempt.PaketID], attempt)
	}
	now := time.Now()
	result := make([]parentSimulasiSummary, 0, len(assignments))
	seen := make(map[string]bool)
	for _, assignment := range assignments {
		paket, ok := packagesByID[assignment.PaketID]
		if !ok || seen[paket.ID] {
			continue
		}
		seen[paket.ID] = true
		tersedia := (paket.WaktuMulai == nil || !now.Before(*paket.WaktuMulai)) && (paket.WaktuSelesai == nil || !now.After(*paket.WaktuSelesai))
		rows := attemptsByPackage[paket.ID]
		entry := parentSimulasiSummary{
			ID: paket.ID, Nama: paket.Nama, Deskripsi: paket.Deskripsi, Mode: paket.Mode,
			DurasiMenit: paket.DurasiMenit, MaksPercobaan: paket.MaksPercobaan,
			WaktuMulai: paket.WaktuMulai, WaktuSelesai: paket.WaktuSelesai,
			TampilkanNilai: paket.TampilkanNilai, TampilkanRingkasan: paket.TampilkanRingkasan,
			Tersedia: tersedia, PercobaanTerpakai: len(rows), Percobaan: make([]parentSimulasiAttempt, 0, len(rows)),
		}
		for _, attempt := range rows {
			row := parentSimulasiAttempt{ID: attempt.ID, Status: attempt.Status, Nomor: attempt.Nomor, Mulai: attempt.Mulai, Selesai: attempt.Selesai, SkorTersedia: paket.TampilkanNilai}
			if paket.TampilkanNilai && attempt.SkorAkhir != nil {
				score := *attempt.SkorAkhir
				row.Skor = &score
			}
			entry.Percobaan = append(entry.Percobaan, row)
		}
		result = append(result, entry)
	}
	return c.JSON(result)
}

// getTugasAnak — GET /orang-tua/anak/:id/tugas
func (s *Server) getTugasAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pd PesertaDidik
	if err := s.db.First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	var tugasList []Tugas
	if err := s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("deadline desc").Find(&tugasList).Error; err != nil {
		return fiber.NewError(500, "gagal memuat tugas anak")
	}
	// Get all submissions for this student in one query; the old loop caused an
	// extra database round-trip for every task in the parent portal.
	tugasIDs := make([]string, 0, len(tugasList))
	for _, tugas := range tugasList {
		tugasIDs = append(tugasIDs, tugas.ID)
	}
	var submissions []PengumpulanTugas
	if len(tugasIDs) > 0 {
		if err := s.db.Where("peserta_didik_id = ? AND tugas_id IN ?", anakID, tugasIDs).Find(&submissions).Error; err != nil {
			return fiber.NewError(500, "gagal memuat pengumpulan tugas")
		}
	}
	submissionByTask := make(map[string]PengumpulanTugas, len(submissions))
	for _, submission := range submissions {
		submissionByTask[submission.TugasID] = submission
	}
	result := make([]parentTugasItem, 0, len(tugasList))
	for _, t := range tugasList {
		tr := parentTugasItem{
			ID: t.ID, MapelID: t.MapelID, KelasID: t.KelasID, Judul: t.Judul,
			Deskripsi: t.Deskripsi, Deadline: t.Deadline, Semester: t.Semester,
			BolehUpload: t.BolehUpload, ModulID: t.ModulID, CreatedAt: t.CreatedAt,
			Mapel:             parentMapelSummary{ID: t.Mapel.ID, NamaMapel: t.Mapel.NamaMapel, KodeMapel: t.Mapel.KodeMapel},
			StatusPengumpulan: "belum",
		}
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
	if err := s.db.First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	var materiList []Materi
	if err := s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("created_at desc").Find(&materiList).Error; err != nil {
		return fiber.NewError(500, "gagal memuat materi anak")
	}
	result := make([]parentMateriItem, 0, len(materiList))
	for _, m := range materiList {
		result = append(result, parentMateriItem{
			ID: m.ID, MapelID: m.MapelID, KelasID: m.KelasID, Judul: m.Judul,
			Deskripsi: m.Deskripsi, Tipe: m.Tipe, Ukuran: m.Ukuran, Semester: m.Semester,
			ModulID: m.ModulID, Urutan: m.Urutan, Tanggal: m.Tanggal, LinkURL: m.LinkURL,
			CreatedAt: m.CreatedAt,
			Mapel:     parentMapelSummary{ID: m.Mapel.ID, NamaMapel: m.Mapel.NamaMapel, KodeMapel: m.Mapel.KodeMapel},
		})
	}
	return c.JSON(result)
}

// getPeminjamanAnak — GET /orang-tua/anak/:id/peminjaman
func (s *Server) getPeminjamanAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var peminjaman []Peminjaman
	if err := s.db.Preload("Buku").Where("peserta_didik_id = ?", anakID).Order("created_at desc").Find(&peminjaman).Error; err != nil {
		return fiber.NewError(500, "gagal memuat riwayat buku anak")
	}
	result := make([]parentPeminjamanItem, 0, len(peminjaman))
	for _, row := range peminjaman {
		item := parentPeminjamanItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, BukuID: row.BukuID, KelasID: row.KelasID,
			Semester: row.Semester, TanggalPinjam: row.TanggalPinjam, Status: row.Status,
		}
		item.Buku.ID = row.Buku.ID
		item.Buku.Judul = row.Buku.Judul
		item.Buku.KodeBuku = row.Buku.KodeBuku
		item.Buku.Penerbit = row.Buku.Penerbit
		result = append(result, item)
	}
	return c.JSON(result)
}

// listChatAnak — GET /orang-tua/anak/:id/chat
func (s *Server) listChatAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var messages []ChatMessage
	if err := s.db.Where("(pengirim_user_id = ? OR penerima_user_id = ?) AND peserta_didik_id = ?", uid, uid, anakID).
		Order("created_at asc").
		Limit(200).
		Find(&messages).Error; err != nil {
		return fiber.NewError(500, "gagal memuat percakapan")
	}
	result := make([]parentChatItem, 0, len(messages))
	for _, message := range messages {
		result = append(result, parentChatItem{
			ID: message.ID, Isi: message.Isi, Dibaca: message.Dibaca,
			DibacaPada: message.DibacaPada, CreatedAt: message.CreatedAt,
			IsMine: message.PengirimUserID == uid,
		})
	}
	return c.JSON(result)
}

// sendChatAnak — POST /orang-tua/anak/:id/chat {isi}
func (s *Server) sendChatAnak(c *fiber.Ctx) error {
	const maxParentChatMessageBytes = 16 * 1024
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
	in.Isi = strings.TrimSpace(in.Isi)
	if len([]byte(in.Isi)) > maxParentChatMessageBytes {
		return fiber.NewError(413, "pesan terlalu panjang")
	}
	// Find wali kelas as recipient
	var pd PesertaDidik
	if err := s.db.Preload("Kelas").First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	if pd.KelasID == "" {
		return fiber.NewError(400, "siswa tidak memiliki kelas")
	}
	var kelas Kelas
	if err := s.db.First(&kelas, "id = ?", pd.KelasID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat kelas anak")
	}
	if kelas.WaliKelasID == nil {
		return fiber.NewError(400, "kelas belum memiliki wali kelas")
	}
	var tutor Tutor
	if err := s.db.First(&tutor, "id = ?", *kelas.WaliKelasID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat wali kelas")
	}
	if tutor.UserID == nil {
		return fiber.NewError(400, "wali kelas belum memiliki akun")
	}
	msg := ChatMessage{
		PesertaDidikID: anakID,
		PengirimUserID: uid,
		PenerimaUserID: *tutor.UserID,
		Isi:            in.Isi,
	}
	if err := s.db.Create(&msg).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan pesan")
	}
	// Push notifikasi to guru wali
	s.pushNotifikasi(*tutor.UserID, "Pesan dari Orang Tua", msg.Isi, "chat", &msg.ID)
	return c.Status(201).JSON(msg)
}

// getPerilakuAnak — GET /orang-tua/anak/:id/perilaku
func (s *Server) getPerilakuAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var catatan []CatatanPerilaku
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("tanggal desc").Find(&catatan).Error; err != nil {
		return fiber.NewError(500, "gagal memuat catatan perilaku anak")
	}
	result := make([]parentBehaviorItem, 0, len(catatan))
	for _, row := range catatan {
		result = append(result, parentBehaviorItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, KelasID: row.KelasID,
			Tanggal: row.Tanggal, Kategori: row.Kategori, Deskripsi: row.Deskripsi,
		})
	}
	return c.JSON(result)
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
