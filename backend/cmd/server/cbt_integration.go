package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	cbtSSOIssuer   = "pkbmti-lms"
	cbtSSOAudience = "pkbmti-cbt"
)

type cbtSSOClaims struct {
	Username       string `json:"username"`
	Nama           string `json:"nama"`
	Role           string `json:"role"`
	TutorID        string `json:"tutorId,omitempty"`
	PesertaDidikID string `json:"pesertaDidikId,omitempty"`
	State          string `json:"state"`
	jwt.RegisteredClaims
}

// issueCBTSSOTicket exchanges an already authenticated LMS session for a
// short-lived, one-use CBT assertion. LMS remains the identity authority; no
// password or long-lived LMS access token is ever sent to the CBT browser.
func (s *Server) issueCBTSSOTicket(c *fiber.Ctx) error {
	secret := strings.TrimSpace(os.Getenv("CBT_SSO_HMAC_SECRET"))
	if len(secret) < 32 {
		return fiber.NewError(fiber.StatusServiceUnavailable, "SSO CBT belum dikonfigurasi oleh Administrator")
	}
	var input struct {
		State string `json:"state"`
	}
	if err := c.BodyParser(&input); err != nil || uuid.Validate(strings.TrimSpace(input.State)) != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Permintaan SSO CBT tidak valid")
	}
	userID, ok := c.Locals("userID").(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Sesi LMS diperlukan")
	}
	var user User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil || !user.IsActive {
		return fiber.NewError(fiber.StatusForbidden, "Akun LMS tidak aktif")
	}
	switch user.Role {
	case "admin", "guru", "kepala_sekolah":
	case "siswa":
		if user.PesertaDidikID == nil || strings.TrimSpace(*user.PesertaDidikID) == "" {
			return fiber.NewError(fiber.StatusForbidden, "Akun siswa belum terhubung ke data peserta didik")
		}
	default:
		return fiber.NewError(fiber.StatusForbidden, "Peran akun ini tidak memiliki akses ke CBT")
	}
	if user.Role == "siswa" {
		var student PesertaDidik
		if err := s.db.Select("id, status").First(&student, "id = ?", *user.PesertaDidikID).Error; err != nil || !strings.EqualFold(strings.TrimSpace(student.Status), "aktif") {
			return fiber.NewError(fiber.StatusForbidden, "Data peserta didik tidak aktif")
		}
	}
	if err := s.fillUserNames(&user); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Identitas akun LMS belum dapat dibaca")
	}
	tutorID, studentID := "", ""
	if user.TutorID != nil {
		tutorID = *user.TutorID
	}
	if user.PesertaDidikID != nil {
		studentID = *user.PesertaDidikID
	}
	now := time.Now().UTC()
	claims := cbtSSOClaims{
		Username: user.Username, Nama: strings.TrimSpace(user.Nama), Role: user.Role,
		TutorID: tutorID, PesertaDidikID: studentID, State: strings.TrimSpace(input.State),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: cbtSSOIssuer, Subject: user.ID, Audience: jwt.ClaimStrings{cbtSSOAudience},
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Minute)), ID: uuid.NewString(),
		},
	}
	ticket, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Tiket SSO CBT tidak dapat dibuat")
	}
	s.audit(&user.ID, "sso_launch", "cbt", user.Role)
	return c.JSON(fiber.Map{"ticket": ticket, "expiresAt": claims.ExpiresAt.Time})
}

// CBTIntegrationNonce and CBTIntegrationEvent protect the server-to-server
// bridge. They are deliberately separate from browser authentication: a CBT
// deploy can never use this channel to act as an LMS administrator.
type CBTIntegrationNonce struct {
	Nonce     string    `gorm:"primaryKey"`
	ExpiresAt time.Time `gorm:"index"`
}
type CBTIntegrationEvent struct {
	Base
	EventID     string `gorm:"uniqueIndex" json:"eventId"`
	EventType   string `gorm:"index" json:"eventType"`
	PayloadJSON string `gorm:"type:text" json:"-"`
}
type CBTExternalAssessment struct {
	Base
	CBTID  string `gorm:"uniqueIndex;index" json:"cbtId"`
	Judul  string `json:"judul"`
	Jenis  string `json:"jenis"`
	Status string `json:"status"`
}
type CBTExternalAttempt struct {
	Base
	CBTID            string     `gorm:"uniqueIndex;index" json:"cbtId"`
	CBTAssessmentID  string     `gorm:"index" json:"cbtAssessmentId"`
	PesertaDidikID   string     `gorm:"index" json:"pesertaDidikId"`
	KelasIDSaatUjian string     `gorm:"index" json:"kelasIdSaatUjian"`
	Status           string     `gorm:"index" json:"status"`
	Skor             float64    `json:"skor"`
	Mulai            *time.Time `json:"mulai,omitempty"`
	Selesai          *time.Time `json:"selesai,omitempty"`
}
type CBTExternalAnswer struct {
	Base
	CBTAttemptID string   `gorm:"uniqueIndex:cbt_external_answer;index" json:"cbtAttemptId"`
	CBTItemID    string   `gorm:"uniqueIndex:cbt_external_answer" json:"cbtItemId"`
	SoalID       string   `gorm:"index" json:"soalId"`
	JawabanJSON  string   `gorm:"type:text" json:"jawabanJson"`
	Benar        *bool    `json:"benar,omitempty"`
	SkorOtomatis float64  `json:"skorOtomatis"`
	SkorManual   *float64 `json:"skorManual,omitempty"`
	Komentar     string   `gorm:"type:text" json:"komentar,omitempty"`
	Revision     int      `json:"revision"`
}

type cbtAccountDTO struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Nama           string    `json:"nama"`
	Role           string    `json:"role"`
	TutorID        string    `json:"tutorId"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	Active         bool      `json:"active"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type cbtKelasDTO struct {
	ID        string    `json:"id"`
	Nama      string    `json:"nama"`
	Jenjang   int       `json:"jenjang"`
	Active    bool      `json:"active"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type cbtPesertaDTO struct {
	ID        string    `json:"id"`
	Nama      string    `json:"nama"`
	NISN      string    `json:"nisn"`
	KelasID   string    `json:"kelasId"`
	Active    bool      `json:"active"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type cbtTutorDTO struct {
	ID        string    `json:"id"`
	Nama      string    `json:"nama"`
	Active    bool      `json:"active"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type cbtMapelDTO struct {
	ID        string    `json:"id"`
	Nama      string    `json:"nama"`
	Kode      string    `json:"kode"`
	Active    bool      `json:"active"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type cbtMasterDTO struct {
	Cursor       string          `json:"cursor"`
	Accounts     []cbtAccountDTO `json:"accounts"`
	Kelas        []cbtKelasDTO   `json:"kelas"`
	PesertaDidik []cbtPesertaDTO `json:"pesertaDidik"`
	Tutor        []cbtTutorDTO   `json:"tutor"`
	Mapel        []cbtMapelDTO   `json:"mapel"`
}

func cbtIntegrationSecret() string {
	return strings.TrimSpace(os.Getenv("CBT_INTEGRATION_HMAC_SECRET"))
}
func (s *Server) cbtIntegrationAuth(c *fiber.Ctx) error {
	secret := cbtIntegrationSecret()
	keyID := strings.TrimSpace(os.Getenv("CBT_INTEGRATION_KEY_ID"))
	if len(secret) < 32 || keyID == "" {
		return fiber.NewError(503, "Integrasi CBT belum dikonfigurasi")
	}
	if !hmac.Equal([]byte(c.Get("X-CBT-Key-ID")), []byte(keyID)) {
		return fiber.NewError(401, "Kredensial integrasi tidak valid")
	}
	timestampRaw, nonce := strings.TrimSpace(c.Get("X-CBT-Timestamp")), strings.TrimSpace(c.Get("X-CBT-Nonce"))
	timestamp, err := strconv.ParseInt(timestampRaw, 10, 64)
	if err != nil || nonce == "" || len(nonce) > 128 || time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute {
		return fiber.NewError(401, "Waktu atau nonce integrasi tidak valid")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.ToUpper(c.Method()) + "\n" + c.OriginalURL() + "\n" + timestampRaw + "\n" + nonce + "\n"))
	_, _ = mac.Write(c.Body())
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(strings.ToLower(strings.TrimSpace(c.Get("X-CBT-Signature"))))) {
		return fiber.NewError(401, "Tanda tangan integrasi tidak valid")
	}
	row := CBTIntegrationNonce{Nonce: nonce, ExpiresAt: time.Now().Add(10 * time.Minute)}
	if err := s.db.Create(&row).Error; err != nil {
		return fiber.NewError(409, "Permintaan integrasi telah digunakan")
	}
	_ = s.db.Where("expires_at < ?", time.Now()).Delete(&CBTIntegrationNonce{}).Error
	return c.Next()
}

func (s *Server) cbtMasterFeed(c *fiber.Ctx) error {
	sinceRaw := strings.TrimSpace(c.Query("cursor"))
	var since time.Time
	if sinceRaw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, sinceRaw); err == nil {
			since = parsed
		}
	}
	filter := func(db *gorm.DB) *gorm.DB {
		if since.IsZero() {
			return db
		}
		// Include the boundary. Multiple source records can share the same
		// UpdatedAt (bulk import/edit); the CBT upsert is idempotent, so replaying
		// boundary rows is safer than silently losing tied updates.
		return db.Where("updated_at >= ?", since)
	}
	var users []User
	var classes []Kelas
	var students []PesertaDidik
	var tutors []Tutor
	var subjects []MataPelajaran
	if err := filter(s.db).Find(&users).Error; err != nil {
		return err
	}
	if err := filter(s.db).Find(&classes).Error; err != nil {
		return err
	}
	if err := filter(s.db).Find(&students).Error; err != nil {
		return err
	}
	if err := filter(s.db).Find(&tutors).Error; err != nil {
		return err
	}
	if err := filter(s.db).Find(&subjects).Error; err != nil {
		return err
	}
	result := cbtMasterDTO{Accounts: make([]cbtAccountDTO, 0, len(users)), Kelas: make([]cbtKelasDTO, 0, len(classes)), PesertaDidik: make([]cbtPesertaDTO, 0, len(students)), Tutor: make([]cbtTutorDTO, 0, len(tutors)), Mapel: make([]cbtMapelDTO, 0, len(subjects))}
	latest := since
	bump := func(value time.Time) {
		if value.After(latest) {
			latest = value
		}
	}
	for _, row := range users {
		tutorID, participantID := "", ""
		if row.TutorID != nil {
			tutorID = *row.TutorID
		}
		if row.PesertaDidikID != nil {
			participantID = *row.PesertaDidikID
		}
		result.Accounts = append(result.Accounts, cbtAccountDTO{ID: row.ID, Username: row.Username, Nama: row.Nama, Role: row.Role, TutorID: tutorID, PesertaDidikID: participantID, Active: row.IsActive, UpdatedAt: row.UpdatedAt})
		bump(row.UpdatedAt)
	}
	for _, row := range classes {
		result.Kelas = append(result.Kelas, cbtKelasDTO{ID: row.ID, Nama: row.NamaRombel, Jenjang: row.Jenjang, Active: true, UpdatedAt: row.UpdatedAt})
		bump(row.UpdatedAt)
	}
	for _, row := range students {
		result.PesertaDidik = append(result.PesertaDidik, cbtPesertaDTO{ID: row.ID, Nama: row.Nama, NISN: row.NISN, KelasID: row.KelasID, Active: strings.EqualFold(strings.TrimSpace(row.Status), "aktif"), UpdatedAt: row.UpdatedAt})
		bump(row.UpdatedAt)
	}
	for _, row := range tutors {
		result.Tutor = append(result.Tutor, cbtTutorDTO{ID: row.ID, Nama: row.Nama, Active: true, UpdatedAt: row.UpdatedAt})
		bump(row.UpdatedAt)
	}
	for _, row := range subjects {
		result.Mapel = append(result.Mapel, cbtMapelDTO{ID: row.ID, Nama: row.NamaMapel, Kode: row.KodeMapel, Active: row.IsActive, UpdatedAt: row.UpdatedAt})
		bump(row.UpdatedAt)
	}
	if latest.IsZero() {
		latest = time.Now().UTC()
	}
	result.Cursor = latest.UTC().Format(time.RFC3339Nano)
	return c.JSON(result)
}

// cbtResultReceiver stores a full, immutable replica of CBT responses. LMS
// staff/parent handlers can later read this model without receiving CBT keys.
func (s *Server) cbtResultReceiver(c *fiber.Ctx) error {
	var input struct {
		EventID string `json:"eventId"`
		Attempt struct {
			ID, AssessmentID, StudentID, ClassIDAtAttempt, Status string
			Score                                                 float64
			StartedAt, SubmittedAt                                *time.Time
		} `json:"attempt"`
		Assessment struct{ ID, Title, Kind, Status string } `json:"assessment"`
		Answers    []struct {
			AttemptItemID, QuestionID, ValueJSON, Comment string
			Correct                                       *bool
			AutoScore                                     float64
			ManualScore                                   *float64
			Revision                                      int
		} `json:"answers"`
	}
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.EventID) == "" || input.Attempt.ID == "" || input.Assessment.ID == "" {
		return fiber.NewError(400, "Payload hasil CBT tidak valid")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var prior CBTIntegrationEvent
		if err := tx.Where("event_id = ?", input.EventID).First(&prior).Error; err == nil {
			return c.JSON(fiber.Map{"status": "duplicate", "eventId": prior.EventID})
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		raw := string(c.Body())
		if err := tx.Create(&CBTIntegrationEvent{EventID: input.EventID, EventType: "assessment.result.v1", PayloadJSON: raw}).Error; err != nil {
			return err
		}
		assessment := CBTExternalAssessment{CBTID: input.Assessment.ID, Judul: input.Assessment.Title, Jenis: input.Assessment.Kind, Status: input.Assessment.Status}
		if err := tx.Where("cbt_id = ?", assessment.CBTID).Assign(assessment).FirstOrCreate(&assessment).Error; err != nil {
			return err
		}
		attempt := CBTExternalAttempt{CBTID: input.Attempt.ID, CBTAssessmentID: input.Assessment.ID, PesertaDidikID: input.Attempt.StudentID, KelasIDSaatUjian: input.Attempt.ClassIDAtAttempt, Status: input.Attempt.Status, Skor: input.Attempt.Score, Mulai: input.Attempt.StartedAt, Selesai: input.Attempt.SubmittedAt}
		if err := tx.Where("cbt_id = ?", attempt.CBTID).Assign(attempt).FirstOrCreate(&attempt).Error; err != nil {
			return err
		}
		for _, source := range input.Answers {
			row := CBTExternalAnswer{CBTAttemptID: input.Attempt.ID, CBTItemID: source.AttemptItemID, SoalID: source.QuestionID, JawabanJSON: source.ValueJSON, Benar: source.Correct, SkorOtomatis: source.AutoScore, SkorManual: source.ManualScore, Komentar: source.Comment, Revision: source.Revision}
			if err := tx.Where("cbt_attempt_id = ? AND cbt_item_id = ?", row.CBTAttemptID, row.CBTItemID).Assign(row).FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
		return c.JSON(fiber.Map{"status": "accepted", "eventId": input.EventID})
	})
}

type cbtMigrationQuestion struct {
	Base
	OwnerID, Title, Type, Prompt, Description, ConfigJSON, AnswerJSON, RubricJSON string
	Points                                                                        float64
	Status                                                                        string
}
type cbtMigrationAssessment struct {
	Base
	OwnerID, Kind, Title, Description, ClassID, SubjectID, Status string
	AccessCodeHash                                                string `json:"accessCodeHash"`
	DurationMinute                                                int
	StartsAt, EndsAt                                              *time.Time
	Randomize, ShowResult                                         bool
	Revision                                                      int
}
type cbtMigrationItem struct {
	Base
	AssessmentID, QuestionID string
	Position                 int
	Weight                   float64
	SnapshotJSON             string
}
type cbtMigrationAssignment struct {
	Base
	AssessmentID, StudentID string
}
type cbtMigrationAttempt struct {
	Base
	AssessmentID, StudentID, ClassIDAtAttempt string
	Number                                    int
	Status, Seed                              string
	StartedAt, SubmittedAt, DeadlineAt        *time.Time
	Score                                     float64
	NeedsManual                               bool
	SourceAttemptID                           string
}
type cbtMigrationAttemptItem struct {
	Base
	AttemptID, AssessmentItemID, QuestionID string
	Position                                int
	Weight                                  float64
	SnapshotJSON                            string
	Flagged                                 bool
}
type cbtMigrationAnswer struct {
	Base
	AttemptID, AttemptItemID, ValueJSON, Comment string
	Correct                                      *bool
	AutoScore                                    float64
	ManualScore                                  *float64
	Revision                                     int
}
type cbtMigrationSnapshot struct {
	ID, Title, Type, Prompt, Description, ConfigJSON, AnswerJSON, RubricJSON string
	Points                                                                   float64
}

func legacyQuestionType(raw string) string {
	switch raw {
	case "pg", "true_false":
		return "pg_tunggal"
	case "checkbox":
		return "pg_kompleks"
	case "short_answer":
		return "isian_singkat"
	case "essay":
		return "uraian"
	default:
		return raw
	}
}
func jsonString(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }

func simAnswerJSON(questionType string, config simulasiConfig) string {
	var value any
	switch questionType {
	case simulasiTipePG, simulasiTipeDropdown:
		if len(config.CorrectIDs) == 1 {
			value = config.CorrectIDs[0]
		}
	case simulasiTipePGK:
		value = config.CorrectIDs
	case simulasiTipeMenjodohkan:
		value = config.Pairs
	case simulasiTipeIsian:
		value = config.AcceptedAnswers
	case simulasiTipeUrutan:
		value = config.CorrectOrder
	case simulasiTipeSkala, simulasiTipeRating:
		value = config.CorrectNumber
	case simulasiTipeKisiPG:
		value = config.GridCorrect
	case simulasiTipeKisiPGK:
		value = config.GridMultiCorrect
	case simulasiTipeBenarSalah:
		rows := map[string]bool{}
		for _, statement := range config.Statements {
			rows[statement.ID] = statement.Correct
		}
		value = rows
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// cbtSnapshotFromSimulasi preserves the published, frozen source snapshot in
// the target's generic item format. Answer material remains inside the target
// snapshot only; the student API strips it before responding.
func cbtSnapshotFromSimulasi(raw string, fallback cbtMigrationQuestion) string {
	var source simulasiSnapshot
	if err := json.Unmarshal([]byte(raw), &source); err != nil {
		return ""
	}
	config, _ := json.Marshal(source.Konfigurasi)
	stimulus, _ := json.Marshal(source.Stimulus)
	description := fallback.Description
	if len(stimulus) > 2 {
		description = strings.TrimSpace(description + "\n\nStimulus arsip: " + string(stimulus))
	}
	encoded, _ := json.Marshal(cbtMigrationSnapshot{ID: source.SoalID, Title: fallback.Title, Type: source.Tipe, Prompt: source.Pertanyaan, Description: description, ConfigJSON: string(config), AnswerJSON: simAnswerJSON(source.Tipe, source.Konfigurasi), RubricJSON: fallback.RubricJSON, Points: fallback.Points})
	return string(encoded)
}

// The migration export is copy-only. It translates the Ujian Online snapshot
// into CBT's independent format while leaving every source row untouched.
func (s *Server) cbtMigrationExport(c *fiber.Ctx) error {
	var bank []BankSoal
	var exams []Ujian
	var examItems []UjianSoal
	var attempts []UjianPeserta
	var attemptItems []UjianPesertaSoal
	var answers []UjianJawaban
	if err := s.db.Find(&bank).Error; err != nil {
		return err
	}
	if err := s.db.Find(&exams).Error; err != nil {
		return err
	}
	if err := s.db.Find(&examItems).Error; err != nil {
		return err
	}
	if err := s.db.Find(&attempts).Error; err != nil {
		return err
	}
	if err := s.db.Find(&attemptItems).Error; err != nil {
		return err
	}
	if err := s.db.Find(&answers).Error; err != nil {
		return err
	}
	questions := make([]cbtMigrationQuestion, 0, len(bank))
	questionByID := make(map[string]cbtMigrationQuestion, len(bank))
	for _, source := range bank {
		config := source.Konfigurasi
		if strings.TrimSpace(config) == "" {
			var choices any
			_ = json.Unmarshal([]byte(source.Opsi), &choices)
			configBytes, _ := json.Marshal(fiber.Map{"choices": choices})
			config = string(configBytes)
		}
		row := cbtMigrationQuestion{Base: source.Base, OwnerID: source.DibuatOlehUserID, Title: source.Pertanyaan, Type: legacyQuestionType(source.Tipe), Prompt: source.Pertanyaan, ConfigJSON: config, AnswerJSON: jsonString(source.Kunci), Points: source.Poin, Status: "published"}
		questions = append(questions, row)
		questionByID[row.ID] = row
	}
	assessments := make([]cbtMigrationAssessment, 0, len(exams))
	for _, source := range exams {
		assessments = append(assessments, cbtMigrationAssessment{Base: source.Base, OwnerID: source.DibuatOlehUserID, Kind: "ujian_online", Title: source.Judul, ClassID: source.KelasID, SubjectID: source.MapelID, Status: "published", AccessCodeHash: hash(strings.TrimSpace(source.AksesKode)), DurationMinute: source.DurasiMenit, StartsAt: &source.WaktuMulai, EndsAt: &source.WaktuSelesai, Randomize: source.AcakSoal, Revision: 1})
	}
	items := make([]cbtMigrationItem, 0, len(examItems))
	legacyItemToCBT := make(map[string]string, len(examItems))
	for _, source := range examItems {
		question := questionByID[source.SoalID]
		snapshotBytes, _ := json.Marshal(cbtMigrationSnapshot{ID: question.ID, Title: question.Title, Type: question.Type, Prompt: question.Prompt, Description: question.Description, ConfigJSON: question.ConfigJSON, AnswerJSON: question.AnswerJSON, RubricJSON: question.RubricJSON, Points: question.Points})
		items = append(items, cbtMigrationItem{Base: source.Base, AssessmentID: source.UjianID, QuestionID: source.SoalID, Position: source.Urutan, Weight: source.Bobot, SnapshotJSON: string(snapshotBytes)})
		legacyItemToCBT[source.ID] = source.ID
	}
	cbtAttempts := make([]cbtMigrationAttempt, 0, len(attempts))
	for _, source := range attempts {
		score := 0.0
		if source.Skor != nil {
			score = *source.Skor
		}
		status := source.Status
		if status == "selesai" {
			status = "completed"
		}
		if status == "mulai" {
			status = "started"
		}
		cbtAttempts = append(cbtAttempts, cbtMigrationAttempt{Base: source.Base, AssessmentID: source.UjianID, StudentID: source.PesertaDidikID, ClassIDAtAttempt: source.KelasIDSaatUjian, Number: 1, Status: status, Seed: source.ID, StartedAt: source.Mulai, SubmittedAt: source.Selesai, Score: score, SourceAttemptID: source.ID})
	}
	cbtAttemptItems := make([]cbtMigrationAttemptItem, 0, len(attemptItems))
	attemptItemByQuestion := map[string]string{}
	for _, source := range attemptItems {
		cbtAttemptItems = append(cbtAttemptItems, cbtMigrationAttemptItem{Base: source.Base, AttemptID: source.UjianPesertaID, AssessmentItemID: legacyItemToCBT[source.UjianSoalID], QuestionID: source.SoalID, Position: source.Urutan, Weight: source.Bobot, SnapshotJSON: source.SnapshotJSON, Flagged: source.Ditandai})
		attemptItemByQuestion[source.UjianPesertaID+":"+source.SoalID] = source.ID
	}
	cbtAnswers := make([]cbtMigrationAnswer, 0, len(answers))
	for _, source := range answers {
		cbtAnswers = append(cbtAnswers, cbtMigrationAnswer{Base: source.Base, AttemptID: source.UjianPesertaID, AttemptItemID: attemptItemByQuestion[source.UjianPesertaID+":"+source.SoalID], ValueJSON: jsonString(source.Jawaban), Correct: source.Benar, AutoScore: source.Nilai, ManualScore: source.NilaiManual, Comment: source.KomentarGuru, Revision: source.Revision})
	}
	var simQuestions []SimulasiSoal
	var simPackages []SimulasiPaket
	var simItems []SimulasiPaketSoal
	var simAssignments []SimulasiPenugasan
	var simAttempts []SimulasiUpaya
	var simAttemptItems []SimulasiUpayaSoal
	var simAnswers []SimulasiJawaban
	if err := s.db.Preload("Stimulus", func(db *gorm.DB) *gorm.DB { return db.Order("urutan") }).Find(&simQuestions).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simPackages).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simItems).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simAssignments).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simAttempts).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simAttemptItems).Error; err != nil {
		return err
	}
	if err := s.db.Find(&simAnswers).Error; err != nil {
		return err
	}
	simQuestionByID := map[string]cbtMigrationQuestion{}
	for _, source := range simQuestions {
		config := source.Konfigurasi
		var parsed simulasiConfig
		_ = json.Unmarshal([]byte(config), &parsed)
		stimulus, _ := json.Marshal(source.Stimulus)
		description := ""
		if len(stimulus) > 2 {
			description = "Stimulus arsip: " + string(stimulus)
		}
		row := cbtMigrationQuestion{Base: source.Base, OwnerID: source.DibuatOlehUserID, Title: source.Pertanyaan, Type: source.Tipe, Prompt: source.Pertanyaan, Description: description, ConfigJSON: config, AnswerJSON: simAnswerJSON(source.Tipe, parsed), RubricJSON: "[]", Points: source.Bobot, Status: "published"}
		questions = append(questions, row)
		simQuestionByID[row.ID] = row
	}
	assignments := make([]cbtMigrationAssignment, 0, len(simAssignments))
	for _, source := range simPackages {
		subjectID := ""
		if source.MapelID != nil {
			subjectID = *source.MapelID
		}
		assessments = append(assessments, cbtMigrationAssessment{Base: source.Base, OwnerID: source.DibuatOlehUserID, Kind: "simulasi", Title: source.Nama, Description: strings.TrimSpace(source.Deskripsi + "\n\n" + source.Instruksi), SubjectID: subjectID, Status: map[bool]string{true: "published", false: "draft"}[source.Status == "terbit"], DurationMinute: source.DurasiMenit, StartsAt: source.WaktuMulai, EndsAt: source.WaktuSelesai, Randomize: source.AcakUrutan, ShowResult: source.TampilkanNilai, Revision: source.Revision})
	}
	for _, source := range simAssignments {
		assignments = append(assignments, cbtMigrationAssignment{Base: source.Base, AssessmentID: source.PaketID, StudentID: source.PesertaDidikID})
	}
	simItemByID := map[string]cbtMigrationItem{}
	for _, source := range simItems {
		questionID := ""
		if source.SoalID != nil {
			questionID = *source.SoalID
		}
		question := simQuestionByID[questionID]
		snapshot := cbtSnapshotFromSimulasi(source.SnapshotJSON, question)
		if snapshot == "" {
			snapshotBytes, _ := json.Marshal(cbtMigrationSnapshot{ID: questionID, Title: question.Title, Type: question.Type, Prompt: question.Prompt, Description: question.Description, ConfigJSON: question.ConfigJSON, AnswerJSON: question.AnswerJSON, RubricJSON: question.RubricJSON, Points: question.Points})
			snapshot = string(snapshotBytes)
		}
		row := cbtMigrationItem{Base: source.Base, AssessmentID: source.PaketID, QuestionID: questionID, Position: source.Urutan, Weight: source.Bobot, SnapshotJSON: snapshot}
		items = append(items, row)
		simItemByID[row.ID] = row
	}
	for _, source := range simAttempts {
		score := source.SkorOtomatis
		if source.SkorAkhir != nil {
			score = *source.SkorAkhir
		}
		status := source.Status
		if status == "selesai" {
			status = "completed"
		}
		if status == "mulai" {
			status = "started"
		}
		cbtAttempts = append(cbtAttempts, cbtMigrationAttempt{Base: source.Base, AssessmentID: source.PaketID, StudentID: source.PesertaDidikID, ClassIDAtAttempt: source.KelasIDSaatUjian, Number: source.Nomor, Status: status, Seed: source.SeedUrutan, StartedAt: source.Mulai, SubmittedAt: source.Selesai, DeadlineAt: source.BatasWaktu, Score: score, NeedsManual: source.SkorAkhir == nil, SourceAttemptID: source.ID})
	}
	for _, source := range simAttemptItems {
		legacy := simItemByID[source.PaketSoalID]
		cbtAttemptItems = append(cbtAttemptItems, cbtMigrationAttemptItem{Base: source.Base, AttemptID: source.UpayaID, AssessmentItemID: source.PaketSoalID, QuestionID: legacy.QuestionID, Position: source.UrutanTampil, Weight: legacy.Weight, SnapshotJSON: legacy.SnapshotJSON, Flagged: source.Ditandai})
	}
	for _, source := range simAnswers {
		cbtAnswers = append(cbtAnswers, cbtMigrationAnswer{Base: source.Base, AttemptID: "", AttemptItemID: source.UpayaSoalID, ValueJSON: source.JawabanJSON, Correct: source.Benar, AutoScore: source.SkorOtomatis, ManualScore: source.SkorManual, Comment: source.KomentarGuru, Revision: 1})
	}
	// Answers refer to attempt-items; recover their owning attempt without
	// trusting a browser-provided identifier.
	for index := range cbtAnswers {
		if cbtAnswers[index].AttemptID != "" {
			continue
		}
		for _, link := range simAttemptItems {
			if link.ID == cbtAnswers[index].AttemptItemID {
				cbtAnswers[index].AttemptID = link.UpayaID
				break
			}
		}
	}
	batchDigest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%d:%d:%d:%d", len(questions), len(assessments), len(items), len(assignments), len(cbtAttempts), len(cbtAttemptItems), len(cbtAnswers))))
	return c.JSON(fiber.Map{"batchId": "lms-" + hex.EncodeToString(batchDigest[:]), "questions": questions, "assessments": assessments, "items": items, "assignments": assignments, "attempts": cbtAttempts, "attemptItems": cbtAttemptItems, "answers": cbtAnswers})
}
