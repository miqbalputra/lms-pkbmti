package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TestUjianOnlineAnswerRevisionAndFlagAutosave(t *testing.T) {
	db := isolatedTestDB(t, "ujian-answer-revision")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianBagian{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Autosave"}
	student := PesertaDidik{Nama: "Siswa Autosave", NISN: "9876543210", Status: "aktif"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student.KelasID = class.ID
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian autosave", KelasID: class.ID, WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), DurasiMenit: 60, AksesKode: "AUTO"}
	question := BankSoal{Tipe: "pg", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: "0", Poin: 1}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	link := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db, cfg: Config{AccessSecret: "answer-revision-test"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/ujian-online/:ujianId/soal", server.getSoalUjianOnline)
	app.Post("/ujian-online/:ujianId/jawab", server.jawabSoal)
	app.Post("/ujian-online/:ujianId/tandai", server.tandaiSoalUjianOnline)
	credentials := "?nisn=" + url.QueryEscape(student.NISN) + "&aksesKode=" + url.QueryEscape(exam.AksesKode)
	view, err := app.Test(httptest.NewRequest(http.MethodGet, "/ujian-online/"+exam.ID+"/soal"+credentials, nil))
	if err != nil {
		t.Fatal(err)
	}
	viewBody, _ := io.ReadAll(view.Body)
	if view.StatusCode != http.StatusOK {
		t.Fatalf("load attempt failed: HTTP %d: %s", view.StatusCode, viewBody)
	}
	var viewData struct {
		Soal []struct {
			ID string `json:"id"`
		} `json:"soal"`
	}
	if err := json.Unmarshal(viewBody, &viewData); err != nil || len(viewData.Soal) != 1 {
		t.Fatalf("invalid attempt response: %s, err=%v", viewBody, err)
	}
	questionID := viewData.Soal[0].ID
	postAnswer := func(base int, key string) *http.Response {
		body, _ := json.Marshal(map[string]any{"ujianSoalId": questionID, "jawaban": "0", "baseRevision": base})
		req := httptest.NewRequest(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab"+credentials, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	firstKey := "11111111-1111-4111-8111-111111111111"
	first := postAnswer(0, firstKey)
	firstBody, _ := io.ReadAll(first.Body)
	if first.StatusCode != http.StatusOK || !bytes.Contains(firstBody, []byte(`"revision":1`)) {
		t.Fatalf("first autosave failed: HTTP %d: %s", first.StatusCode, firstBody)
	}
	retry := postAnswer(0, firstKey)
	retryBody, _ := io.ReadAll(retry.Body)
	if retry.StatusCode != http.StatusOK || !bytes.Contains(retryBody, []byte(`"revision":1`)) {
		t.Fatalf("idempotent retry changed revision: HTTP %d: %s", retry.StatusCode, retryBody)
	}
	conflict := postAnswer(0, "22222222-2222-4222-8222-222222222222")
	conflictBody, _ := io.ReadAll(conflict.Body)
	if conflict.StatusCode != http.StatusConflict || !bytes.Contains(conflictBody, []byte(`"revision":1`)) {
		t.Fatalf("stale revision should conflict: HTTP %d: %s", conflict.StatusCode, conflictBody)
	}
	latest := postAnswer(1, "33333333-3333-4333-8333-333333333333")
	latestBody, _ := io.ReadAll(latest.Body)
	if latest.StatusCode != http.StatusOK || !bytes.Contains(latestBody, []byte(`"revision":2`)) {
		t.Fatalf("next revision failed: HTTP %d: %s", latest.StatusCode, latestBody)
	}

	flagBody := bytes.NewBufferString(`{"ujianSoalId":"` + questionID + `","ditandai":true}`)
	flagRequest := httptest.NewRequest(http.MethodPost, "/ujian-online/"+exam.ID+"/tandai"+credentials, flagBody)
	flagRequest.Header.Set("Content-Type", "application/json")
	flag, err := app.Test(flagRequest)
	if err != nil || flag.StatusCode != http.StatusOK {
		t.Fatalf("flag endpoint failed: response=%v err=%v", flag, err)
	}
	var savedQuestion UjianPesertaSoal
	var attempt UjianPeserta
	if err := db.Where("ujian_id = ? AND peserta_didik_id = ?", exam.ID, student.ID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("ujian_peserta_id = ? AND ujian_soal_id = ?", attempt.ID, questionID).First(&savedQuestion).Error; err != nil || !savedQuestion.Ditandai {
		t.Fatalf("flag state was not saved: %+v err=%v", savedQuestion, err)
	}
}

func TestUjianOnlineLateRecoveryIsReviewedWithoutChangingScore(t *testing.T) {
	db := isolatedTestDB(t, "ujian-late-recovery")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianBagian{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanBerkas{}, &UjianJawabanRevisi{}, &UjianJawabanPemulihan{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Recovery"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Pemulihan", NISN: "8765432109", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian pemulihan", KelasID: class.ID, WaktuMulai: now.Add(-4 * time.Hour), WaktuSelesai: now.Add(-3 * time.Hour), DurasiMenit: 60, AksesKode: "PULIH"}
	question := BankSoal{Tipe: "pg", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: "0", Poin: 1}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	link := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	originalScore := 73.5
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, KelasIDSaatUjian: class.ID, Mulai: ptrTime(now.Add(-4 * time.Hour)), Selesai: ptrTime(now.Add(-2 * time.Hour)), Status: "selesai", Skor: &originalScore}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db, cfg: Config{AccessSecret: "recovery-test-secret"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Post("/ujian-online/:ujianId/pemulihan", server.submitUjianOnlineRecovery)
	app.Get("/ujian-online/monitor/:ujianId/recoveries", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		c.Locals("userID", "admin-reviewer")
		return server.listUjianOnlineRecoveries(c)
	})
	app.Put("/ujian-online/recoveries/:recoveryId/review", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		c.Locals("userID", "admin-reviewer")
		return server.reviewUjianOnlineRecovery(c)
	})
	key := "44444444-4444-4444-8444-444444444444"
	body := []byte(`{"idempotencyKey":"` + key + `","answers":[{"ujianSoalId":"` + link.ID + `","jawaban":"0","baseRevision":0}]}`)
	post := func(payload []byte) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/ujian-online/"+exam.ID+"/pemulihan?nisn="+student.NISN+"&aksesKode="+exam.AksesKode, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	first := post(body)
	firstBody, _ := io.ReadAll(first.Body)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("recovery submission failed: HTTP %d: %s", first.StatusCode, firstBody)
	}
	duplicate := post(body)
	if duplicate.StatusCode != http.StatusOK {
		duplicateBody, _ := io.ReadAll(duplicate.Body)
		t.Fatalf("idempotent recovery retry failed: HTTP %d: %s", duplicate.StatusCode, duplicateBody)
	}
	conflicting := bytes.Replace(body, []byte(`"jawaban":"0"`), []byte(`"jawaban":"1"`), 1)
	if response := post(conflicting); response.StatusCode != http.StatusConflict {
		resultBody, _ := io.ReadAll(response.Body)
		t.Fatalf("reused recovery key should conflict: HTTP %d: %s", response.StatusCode, resultBody)
	}
	var count int64
	if err := db.Model(&UjianJawabanPemulihan{}).Where("ujian_peserta_id = ?", attempt.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("expected one recovery row, got %d err=%v", count, err)
	}
	var savedAttempt UjianPeserta
	if err := db.First(&savedAttempt, "id = ?", attempt.ID).Error; err != nil || savedAttempt.Skor == nil || *savedAttempt.Skor != originalScore {
		t.Fatalf("recovery changed the existing score: %+v err=%v", savedAttempt, err)
	}
	var savedAnswer UjianJawaban
	if err := db.Where("ujian_peserta_id = ?", attempt.ID).First(&savedAnswer).Error; err == nil {
		t.Fatal("late recovery must not be applied as a normal answer")
	}
	list, err := app.Test(httptest.NewRequest(http.MethodGet, "/ujian-online/monitor/"+exam.ID+"/recoveries", nil))
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(list.Body)
	if list.StatusCode != http.StatusOK || !bytes.Contains(listBody, []byte(`"jawaban":"0"`)) || !bytes.Contains(listBody, []byte(`"namaPeserta":"Siswa Pemulihan"`)) || !bytes.Contains(listBody, []byte(`"pertanyaan":"Pilih"`)) {
		t.Fatalf("staff recovery view must expose responses for review: HTTP %d: %s", list.StatusCode, listBody)
	}
	decision := httptest.NewRequest(http.MethodPut, "/ujian-online/recoveries/"+mustRecoveryID(t, db, attempt.ID)+"/review", bytes.NewBufferString(`{"status":"ditolak","komentar":"Tidak dapat diverifikasi"}`))
	decision.Header.Set("Content-Type", "application/json")
	review, err := app.Test(decision)
	if err != nil || review.StatusCode != http.StatusOK {
		t.Fatalf("recovery review failed: response=%v err=%v", review, err)
	}
	review.Body.Close()
	decisionAgain := httptest.NewRequest(http.MethodPut, "/ujian-online/recoveries/"+mustRecoveryID(t, db, attempt.ID)+"/review", bytes.NewBufferString(`{"status":"diterima"}`))
	decisionAgain.Header.Set("Content-Type", "application/json")
	secondReview, err := app.Test(decisionAgain)
	if err != nil || secondReview.StatusCode != http.StatusConflict {
		t.Fatalf("reviewing an already decided recovery must conflict: response=%v err=%v", secondReview, err)
	}
	secondReview.Body.Close()
	if err := db.First(&savedAttempt, "id = ?", attempt.ID).Error; err != nil || savedAttempt.Skor == nil || *savedAttempt.Skor != originalScore {
		t.Fatalf("review mutated grade unexpectedly: %+v err=%v", savedAttempt, err)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func mustRecoveryID(t *testing.T, db *gorm.DB, attemptID string) string {
	t.Helper()
	var recovery UjianJawabanPemulihan
	if err := db.First(&recovery, "ujian_peserta_id = ?", attemptID).Error; err != nil {
		t.Fatal(err)
	}
	return recovery.ID
}
