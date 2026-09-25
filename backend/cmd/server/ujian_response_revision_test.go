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
)

func TestUjianOnlineSubmittedResponseCanBeRevisedIdempotently(t *testing.T) {
	db := isolatedTestDB(t, "ujian-response-revision")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanRevisi{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Revisi"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Revisi", NISN: "9012345678", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian Revisi", KelasID: class.ID, WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), DurasiMenit: 45, AksesKode: "REVISI", IzinkanEditRespons: true}
	question := BankSoal{Tipe: "pg", Pertanyaan: "Pilih jawaban", Opsi: `["Benar","Salah"]`, Kunci: "0", Poin: 1}
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
	server := &Server{db: db, cfg: Config{AccessSecret: "revision-test-secret"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/ujian-online/:ujianId/soal", server.getSoalUjianOnline)
	app.Post("/ujian-online/:ujianId/jawab", server.jawabSoal)
	app.Post("/ujian-online/:ujianId/selesai", server.selesaiUjianOnline)
	request := func(method, path string, form url.Values, key string) *http.Response {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	credentials := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}}
	view := request(http.MethodGet, "/ujian-online/"+exam.ID+"/soal", credentials, "")
	viewBody, _ := io.ReadAll(view.Body)
	if view.StatusCode != http.StatusOK || bytes.Contains(viewBody, []byte("kunci")) || !bytes.Contains(viewBody, []byte(`"bolehEditRespons":false`)) {
		t.Fatalf("initial question response should be safe and not advertise edit mode, status %d: %s", view.StatusCode, viewBody)
	}
	if err := view.Body.Close(); err != nil {
		t.Fatal(err)
	}

	answer := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}, "ujianSoalId": {link.ID}, "jawaban": {"0"}}
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answer, ""); response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("initial answer save returned HTTP %d: %s", response.StatusCode, body)
	}
	submitted := request(http.MethodPost, "/ujian-online/"+exam.ID+"/selesai", credentials, "")
	var submittedBody map[string]interface{}
	if err := json.NewDecoder(submitted.Body).Decode(&submittedBody); err != nil {
		t.Fatal(err)
	}
	if submitted.StatusCode != http.StatusOK || submittedBody["skor"] != float64(100) || submittedBody["bolehEditRespons"] != true {
		t.Fatalf("expected completed score with edit permission, status %d: %+v", submitted.StatusCode, submittedBody)
	}
	_ = submitted.Body.Close()

	answer.Set("jawaban", "1")
	key := "45e880d2-8ab9-46f2-8171-ec984595aad3"
	revised := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answer, key)
	var revisedBody map[string]interface{}
	if err := json.NewDecoder(revised.Body).Decode(&revisedBody); err != nil {
		t.Fatal(err)
	}
	if revised.StatusCode != http.StatusOK || revisedBody["skor"] != float64(0) || revisedBody["statusSimpan"] != "tersimpan" || revisedBody["revision"] != float64(1) {
		t.Fatalf("submitted answer edit should be graded and versioned, status %d: %+v", revised.StatusCode, revisedBody)
	}
	_ = revised.Body.Close()

	replay := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answer, key)
	var replayBody map[string]interface{}
	if err := json.NewDecoder(replay.Body).Decode(&replayBody); err != nil {
		t.Fatal(err)
	}
	if replay.StatusCode != http.StatusOK || replayBody["statusSimpan"] != "sudah_tersimpan" || replayBody["revision"] != float64(1) {
		t.Fatalf("retry must return the original revision, status %d: %+v", replay.StatusCode, replayBody)
	}
	_ = replay.Body.Close()
	answer.Set("jawaban", "0")
	conflict := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answer, key)
	conflictBody, _ := io.ReadAll(conflict.Body)
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("reusing a key for different response should conflict, got HTTP %d: %s", conflict.StatusCode, conflictBody)
	}
	_ = conflict.Body.Close()

	var revisions []UjianJawabanRevisi
	var savedAttempt UjianPeserta
	if err := db.Where("ujian_id = ? AND peserta_didik_id = ?", exam.ID, student.ID).First(&savedAttempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("ujian_peserta_id = ?", savedAttempt.ID).Find(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].JawabanSebelum != "0" || revisions[0].JawabanSesudah != "1" {
		t.Fatalf("revision history should preserve exact before/after response: %+v", revisions)
	}
	var auditCount int64
	if err := db.Model(&AuditLog{}).Where("action = ? AND resource = ?", "revise_response", "ujian_online_jawaban").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one audit entry for one committed revision, got %d", auditCount)
	}

	finishedView := request(http.MethodGet, "/ujian-online/"+exam.ID+"/soal", credentials, "")
	finishedViewBody, _ := io.ReadAll(finishedView.Body)
	if finishedView.StatusCode != http.StatusOK || !bytes.Contains(finishedViewBody, []byte(`"bolehEditRespons":true`)) || bytes.Contains(finishedViewBody, []byte("correctIds")) || bytes.Contains(finishedViewBody, []byte(`"kunci"`)) {
		t.Fatalf("permitted edit session should load without exposing answer keys, status %d: %s", finishedView.StatusCode, finishedViewBody)
	}
	_ = finishedView.Body.Close()
}

func TestUjianOnlineResponseRevisionRequiresOptInAndUnlockedAttempt(t *testing.T) {
	db := isolatedTestDB(t, "ujian-response-revision-policy")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanRevisi{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Revisi policy"}
	student := PesertaDidik{Nama: "Siswa Policy", NISN: "9012345679", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student.KelasID = class.ID
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Tanpa revisi", KelasID: class.ID, WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), AksesKode: "NOREVISI", IzinkanEditRespons: false}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Status: "selesai", Selesai: &now}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db, cfg: Config{AccessSecret: "revision-policy-secret"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/ujian-online/:ujianId/soal", server.getSoalUjianOnline)
	app.Post("/ujian-online/:ujianId/jawab", server.jawabSoal)
	request := func(method, path string) *http.Response {
		form := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}, "ujianSoalId": {"missing-question"}, "jawaban": {"response"}}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Idempotency-Key", "28f0ea12-8ea1-457f-a49a-6c0f4123ab8c")
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	if response := request(http.MethodGet, "/ujian-online/"+exam.ID+"/soal"); response.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("completed attempt without teacher opt-in must remain closed, got HTTP %d: %s", response.StatusCode, body)
	}
	_ = db.Model(&Ujian{}).Where("id = ?", exam.ID).Update("izinkan_edit_respons", true).Error
	_ = db.Model(&UjianPeserta{}).Where("id = ?", attempt.ID).Update("status", "dikunci").Error
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab"); response.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("teacher opt-in must not unlock an attempt locked for integrity violations, got HTTP %d: %s", response.StatusCode, body)
	}
}

func TestUjianOnlineSubmittedUploadRevisionPreservesHistoricalFile(t *testing.T) {
	db := isolatedTestDB(t, "ujian-upload-revision")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanRevisi{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Revisi unggahan"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Berkas", NISN: "9012345680", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian revisi berkas", KelasID: class.ID, WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), AksesKode: "BERKAS", IzinkanEditRespons: true}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	question := BankSoal{Tipe: simulasiTipeUnggah, Pertanyaan: "Unggah bukti", Poin: 1}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	link := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Status: "menunggu_nilai", Mulai: &now, Selesai: &now}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	snapshotJSON, _ := json.Marshal(ujianQuestionSnapshot{Tipe: simulasiTipeUnggah, Pertanyaan: question.Pertanyaan, Konfigurasi: simulasiConfig{AllowedFileTypes: []string{"pdf"}, MaxFiles: 2, MaxFileSizeMB: 5}})
	frozen := UjianPesertaSoal{UjianPesertaID: attempt.ID, UjianSoalID: link.ID, SoalID: question.ID, Urutan: 1, Bobot: 1, SnapshotJSON: string(snapshotJSON)}
	if err := db.Create(&frozen).Error; err != nil {
		t.Fatal(err)
	}
	file := UjianJawabanBerkas{UjianPesertaID: attempt.ID, UjianSoalID: link.ID, SoalID: question.ID, FilePath: "private/retained-answer.pdf", NamaFile: "jawaban.pdf", ContentType: "application/pdf", Ukuran: 10}
	if err := db.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	fileIDs, _ := json.Marshal([]string{file.ID})
	answer := UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.ID, Jawaban: string(fileIDs)}
	if err := db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db, cfg: Config{AccessSecret: "revision-upload-secret"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Delete("/ujian-online/:ujianId/soal/:ujianSoalId/file/:fileId", server.ujianOnlineDeleteAnswerFile)
	key := "8a33ccf1-09da-4a81-b1b3-91eb678b2501"
	request := func() *http.Response {
		form := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}}
		req := httptest.NewRequest(http.MethodDelete, "/ujian-online/"+exam.ID+"/soal/"+link.ID+"/file/"+file.ID, bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Idempotency-Key", key)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	first := request()
	if first.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(first.Body)
		t.Fatalf("submitted file removal returned HTTP %d: %s", first.StatusCode, body)
	}
	_ = first.Body.Close()
	retry := request()
	if retry.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(retry.Body)
		t.Fatalf("idempotent file removal retry returned HTTP %d: %s", retry.StatusCode, body)
	}
	_ = retry.Body.Close()
	var saved UjianJawabanBerkas
	if err := db.First(&saved, "id = ?", file.ID).Error; err != nil {
		t.Fatalf("removed attachment must remain recoverable in submitted history: %v", err)
	}
	if saved.FilePath != file.FilePath {
		t.Fatalf("revision must not replace historical file path: %+v", saved)
	}
	if err := db.First(&answer, "id = ?", answer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if answer.Jawaban != "[]" {
		t.Fatalf("current file response should no longer reference the removed file: %q", answer.Jawaban)
	}
	var revisions []UjianJawabanRevisi
	if err := db.Where("ujian_peserta_id = ?", attempt.ID).Find(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].JawabanSebelum != string(fileIDs) || revisions[0].JawabanSesudah != "[]" {
		t.Fatalf("expected one immutable before/after file revision: %+v", revisions)
	}
	if err := db.First(&attempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Status != "selesai" || attempt.Skor == nil || *attempt.Skor != 0 {
		t.Fatalf("removing the only manually scored upload should recompute the submitted result: %+v", attempt)
	}
}

func TestUjianOnlineAutomaticClosureIsFinalEvenWhenResponseEditsAreEnabled(t *testing.T) {
	db := isolatedTestDB(t, "ujian-response-auto-close")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanRevisi{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Auto close"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Auto", NISN: "9012345690", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-30 * time.Minute)
	exam := Ujian{Judul: "Tenggat", KelasID: class.ID, DurasiMenit: 10, IzinkanEditRespons: true}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Status: "mulai", Mulai: &started}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db}
	if _, err := server.finishUjianAttempt(&attempt, &exam, time.Now(), false, true, nil); err != nil {
		t.Fatal(err)
	}
	var saved UjianPeserta
	if err := db.First(&saved, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.PenutupanOtomatis || ujianResponseEditAllowed(saved, exam, time.Now()) {
		t.Fatalf("automatic timeout closure must be final even when response editing is enabled: %+v", saved)
	}
}
