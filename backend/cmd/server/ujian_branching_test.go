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

func TestUjianBranchValidationAndVisualRouteAPI(t *testing.T) {
	db := isolatedTestDB(t, "ujian-branch-api")
	if err := db.AutoMigrate(&AuditLog{}, &BankSoal{}, &Kelas{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Branch"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Ujian bercabang", KelasID: class.ID, WaktuMulai: time.Now().Add(-time.Hour), WaktuSelesai: time.Now().Add(time.Hour)}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	sections := []UjianBagian{{UjianID: exam.ID, ClientID: "awal", Nama: "Awal", Urutan: 1}, {UjianID: exam.ID, ClientID: "lanjut", Nama: "Lanjutan", Urutan: 2}, {UjianID: exam.ID, ClientID: "akhir", Nama: "Akhir", Urutan: 3}}
	if err := db.Create(&sections).Error; err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(simulasiConfig{Choices: []simulasiChoice{{ID: "ya", Text: "Ya"}, {ID: "tidak", Text: "Tidak"}}, CorrectIDs: []string{"ya"}})
	branchQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Lanjut?", Konfigurasi: string(config), Poin: 1}
	otherQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Soal lanjutan", Konfigurasi: string(config), Poin: 1}
	conflictQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Pengatur alur kedua", Konfigurasi: string(config), Poin: 1}
	if err := db.Create(&branchQuestion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherQuestion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&conflictQuestion).Error; err != nil {
		t.Fatal(err)
	}
	branchRow := UjianSoal{UjianID: exam.ID, SoalID: branchQuestion.ID, BagianID: "awal", Urutan: 1, Bobot: 1}
	otherRow := UjianSoal{UjianID: exam.ID, SoalID: otherQuestion.ID, BagianID: "akhir", Urutan: 2, Bobot: 1}
	if err := db.Create(&branchRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherRow).Error; err != nil {
		t.Fatal(err)
	}
	conflictRow := UjianSoal{UjianID: exam.ID, SoalID: conflictQuestion.ID, BagianID: "awal", Urutan: 2, Bobot: 1, BranchToByAnswerJSON: `{"ya":"akhir"}`}

	s := &Server{db: db}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Put("/ujian/:id/soal/:sid/branch", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		c.Locals("userID", "branch-admin")
		return s.updateUjianSoalBranch(c)
	})
	request := func(role, body string) *http.Response {
		req := httptest.NewRequest(http.MethodPut, "/ujian/"+exam.ID+"/soal/"+branchRow.ID+"/branch", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", role)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	if response := request("admin", `{"branchToByAnswer":{"ya":"akhir","tidak":"__selesai__"}}`); response.StatusCode != http.StatusOK {
		t.Fatalf("valid branch route returned HTTP %d", response.StatusCode)
	}
	var saved UjianSoal
	if err := db.First(&saved, "id = ?", branchRow.ID).Error; err != nil {
		t.Fatal(err)
	}
	var routes map[string]string
	if err := json.Unmarshal([]byte(saved.BranchToByAnswerJSON), &routes); err != nil || routes["ya"] != "akhir" {
		t.Fatalf("branch route was not persisted: %q, err=%v", saved.BranchToByAnswerJSON, err)
	}
	if err := db.Create(&conflictRow).Error; err != nil {
		t.Fatal(err)
	}
	if response := request("admin", `{"branchToByAnswer":{"ya":"akhir"}}`); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("second branching question in one section should be rejected, got HTTP %d", response.StatusCode)
	}
	if err := db.Delete(&conflictRow).Error; err != nil {
		t.Fatal(err)
	}
	if response := request("admin", `{"branchToByAnswer":{"ya":"awal"}}`); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("backward branch route should be rejected, got HTTP %d", response.StatusCode)
	}
	if response := request("kepala_sekolah", `{"branchToByAnswer":{}}`); response.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only role changed branch route, got HTTP %d", response.StatusCode)
	}
}

func TestUjianBranchLearnerPathAndScoringIgnoreSkippedSections(t *testing.T) {
	branchConfig := simulasiConfig{Choices: []simulasiChoice{{ID: "go", Text: "Lanjut"}, {ID: "stop", Text: "Selesai"}}, CorrectIDs: []string{"go"}, BranchToByAnswer: map[string]string{"go": "bagian-3", "stop": simulasiBranchFinish}}
	regularConfig2 := simulasiConfig{Choices: []simulasiChoice{{ID: "wrong", Text: "Salah"}, {ID: "right", Text: "Benar"}}, CorrectIDs: []string{"right"}}
	regularConfig3 := simulasiConfig{Choices: []simulasiChoice{{ID: "right-3", Text: "Benar"}, {ID: "wrong-3", Text: "Salah"}}, CorrectIDs: []string{"right-3"}}
	makeSnapshot := func(questionType, prompt string, config simulasiConfig) string {
		raw, err := json.Marshal(ujianQuestionSnapshot{Tipe: questionType, Pertanyaan: prompt, Konfigurasi: config})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	questions := []UjianPesertaSoal{
		{UjianSoalID: "link-1", SoalID: "bank-1", BagianID: "bagian-1", UrutanBagian: 1, Urutan: 1, Bobot: 1, SnapshotJSON: makeSnapshot(simulasiTipePG, "Pilih rute", branchConfig)},
		{UjianSoalID: "link-2", SoalID: "bank-2", BagianID: "bagian-2", UrutanBagian: 2, Urutan: 2, Bobot: 4, SnapshotJSON: makeSnapshot(simulasiTipePG, "Dilewati", regularConfig2)},
		{UjianSoalID: "link-3", SoalID: "bank-3", BagianID: "bagian-3", UrutanBagian: 3, Urutan: 3, Bobot: 1, SnapshotJSON: makeSnapshot(simulasiTipePG, "Tujuan", regularConfig3)},
		{UjianSoalID: "link-4", SoalID: "bank-4", Urutan: 4, Bobot: 2, SnapshotJSON: makeSnapshot(simulasiTipePG, "Tanpa bagian", regularConfig3)},
	}
	answerGo := UjianJawaban{SoalID: "bank-1", Jawaban: `"go"`}
	active, err := activeUjianQuestionIDs(questions, []UjianJawaban{answerGo})
	if err != nil {
		t.Fatal(err)
	}
	if !active["link-1"] || active["link-2"] || !active["link-3"] || !active["link-4"] {
		t.Fatalf("wrong active path for forward branch: %#v", active)
	}
	active, err = activeUjianQuestionIDs(questions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !active["link-1"] || active["link-2"] || active["link-3"] {
		t.Fatalf("unanswered branch should hold later sections: %#v", active)
	}
	active, err = activeUjianQuestionIDs(questions, []UjianJawaban{{SoalID: "bank-1", Jawaban: `"stop"`}})
	if err != nil {
		t.Fatal(err)
	}
	if !active["link-1"] || active["link-2"] || active["link-3"] || active["link-4"] {
		t.Fatalf("end route should omit subsequent sections: %#v", active)
	}

	db := isolatedTestDB(t, "ujian-branch-grade")
	if err := db.AutoMigrate(&BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}); err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Nilai bercabang"}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "student-branch", Status: "mulai"}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	for _, question := range questions[:3] {
		question.UjianPesertaID = attempt.ID
		if err := db.Create(&question).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, answer := range []UjianJawaban{
		{UjianPesertaID: attempt.ID, SoalID: "bank-1", Jawaban: `"go"`},
		{UjianPesertaID: attempt.ID, SoalID: "bank-2", Jawaban: `"right"`}, // correct but skipped
		{UjianPesertaID: attempt.ID, SoalID: "bank-3", Jawaban: `"wrong-3"`},
	} {
		if err := db.Create(&answer).Error; err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{db: db}
	grade, err := server.gradeUjianPesertaResult(&attempt, &exam)
	if err != nil {
		t.Fatal(err)
	}
	if grade.Total != 2 || grade.Correct != 1 || grade.Score != 50 {
		t.Fatalf("skipped section affected score or question count: %+v", grade)
	}
}

func TestUjianBranchLearnerAPIHidesAndRejectsSkippedQuestions(t *testing.T) {
	db := isolatedTestDB(t, "ujian-branch-learner-api")
	if err := db.AutoMigrate(&AuditLog{}, &Kelas{}, &PesertaDidik{}, &BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &UjianJawabanBerkas{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 6, NamaRombel: "Ujian Cabang"}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Cabang", NISN: "9000000001", KelasID: class.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian dengan cabang", KelasID: class.ID, AksesKode: "KODECABANG", WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), DurasiMenit: 45}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	sections := []UjianBagian{{UjianID: exam.ID, ClientID: "mulai", Nama: "Bagian awal", Urutan: 1}, {UjianID: exam.ID, ClientID: "dilewati", Nama: "Bagian tidak dipilih", Urutan: 2}, {UjianID: exam.ID, ClientID: "tujuan", Nama: "Bagian tujuan", Urutan: 3}}
	if err := db.Create(&sections).Error; err != nil {
		t.Fatal(err)
	}
	branchConfig, _ := json.Marshal(simulasiConfig{Choices: []simulasiChoice{{ID: "lanjut", Text: "Lanjut"}, {ID: "berhenti", Text: "Berhenti"}}, CorrectIDs: []string{"lanjut"}})
	plainConfig, _ := json.Marshal(simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}})
	branchQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Pilih jalur", Konfigurasi: string(branchConfig), Kunci: `"lanjut"`, Poin: 1, Domain: "Numerasi", Kompetensi: "LABEL INTERNAL ANALITIK"}
	skippedQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Seharusnya tersembunyi", Konfigurasi: string(plainConfig), Kunci: `"a"`, Poin: 1}
	targetQuestion := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "Soal tujuan", Konfigurasi: string(plainConfig), Kunci: `"a"`, Poin: 1}
	for _, question := range []*BankSoal{&branchQuestion, &skippedQuestion, &targetQuestion} {
		if err := db.Create(question).Error; err != nil {
			t.Fatal(err)
		}
	}
	branchRow := UjianSoal{UjianID: exam.ID, SoalID: branchQuestion.ID, BagianID: "mulai", Urutan: 1, Bobot: 1, BranchToByAnswerJSON: `{"lanjut":"tujuan","berhenti":"__selesai__"}`}
	skippedRow := UjianSoal{UjianID: exam.ID, SoalID: skippedQuestion.ID, BagianID: "dilewati", Urutan: 2, Bobot: 1}
	targetRow := UjianSoal{UjianID: exam.ID, SoalID: targetQuestion.ID, BagianID: "tujuan", Urutan: 3, Bobot: 1}
	for _, row := range []*UjianSoal{&branchRow, &skippedRow, &targetRow} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db, cfg: Config{AccessSecret: "test-secret"}}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/ujian-online/:ujianId/soal", server.getSoalUjianOnline)
	app.Post("/ujian-online/:ujianId/jawab", server.jawabSoal)
	app.Post("/ujian-online/:ujianId/selesai", server.selesaiUjianOnline)
	request := func(method, path string, form url.Values) *http.Response {
		request := httptest.NewRequest(method, path, bytes.NewBufferString(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	credentials := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}}
	firstResponse := request(http.MethodGet, "/ujian-online/"+exam.ID+"/soal", credentials)
	if firstResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(firstResponse.Body)
		t.Fatalf("initial learner view returned HTTP %d: %s", firstResponse.StatusCode, body)
	}
	firstBody, _ := io.ReadAll(firstResponse.Body)
	var first struct {
		Soal []struct {
			ID           string `json:"id"`
			HasBranching bool   `json:"hasBranching"`
		} `json:"soal"`
	}
	if err := json.Unmarshal(firstBody, &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Soal) != 1 || first.Soal[0].ID != branchRow.ID || !first.Soal[0].HasBranching {
		t.Fatalf("unanswered branch should show only its controlling question: %+v", first.Soal)
	}
	for _, leaked := range []string{"correctIds", "branchToByAnswer", "kunci", "Numerasi", "LABEL INTERNAL ANALITIK", "metadata", "Seharusnya tersembunyi", "Soal tujuan"} {
		if bytes.Contains(firstBody, []byte(leaked)) {
			t.Fatalf("student payload leaked hidden question or staff field %q", leaked)
		}
	}

	answerForm := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}, "ujianSoalId": {skippedRow.ID}, "jawaban": {`"a"`}}
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answerForm); response.StatusCode != http.StatusNotFound {
		t.Fatalf("student submitted an inactive question: HTTP %d", response.StatusCode)
	}
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/selesai", credentials); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("student must answer the branch question before submitting, got HTTP %d", response.StatusCode)
	}
	branchAnswer := url.Values{"nisn": {student.NISN}, "aksesKode": {exam.AksesKode}, "ujianSoalId": {branchRow.ID}, "jawaban": {`"lanjut"`}}
	branchResponse := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", branchAnswer)
	if branchResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(branchResponse.Body)
		t.Fatalf("saving branch answer returned HTTP %d: %s", branchResponse.StatusCode, body)
	}
	var routeResult struct {
		ActiveIDs []string `json:"activeSoalIds"`
	}
	if err := json.NewDecoder(branchResponse.Body).Decode(&routeResult); err != nil {
		t.Fatal(err)
	}
	if len(routeResult.ActiveIDs) != 2 || routeResult.ActiveIDs[0] != branchRow.ID || routeResult.ActiveIDs[1] != targetRow.ID {
		t.Fatalf("answer should reveal only the selected forward section: %+v", routeResult.ActiveIDs)
	}
	answerForm.Set("jawaban", `"a"`)
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/jawab", answerForm); response.StatusCode != http.StatusNotFound {
		t.Fatalf("student accessed a skipped question after branching: HTTP %d", response.StatusCode)
	}
	finalResponse := request(http.MethodGet, "/ujian-online/"+exam.ID+"/soal", credentials)
	finalBody, _ := io.ReadAll(finalResponse.Body)
	if finalResponse.StatusCode != http.StatusOK || bytes.Contains(finalBody, []byte("Seharusnya tersembunyi")) || !bytes.Contains(finalBody, []byte("Soal tujuan")) {
		t.Fatalf("learner view does not match selected branch, HTTP %d: %s", finalResponse.StatusCode, finalBody)
	}
	if response := request(http.MethodPost, "/ujian-online/"+exam.ID+"/selesai", credentials); response.StatusCode != http.StatusOK {
		t.Fatalf("a completed branch choice should allow submission, got HTTP %d", response.StatusCode)
	}
}
