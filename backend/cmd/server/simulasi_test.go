package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func simulasiStudent(t *testing.T, s *Server, username string) (PesertaDidik, string) {
	t.Helper()
	student := PesertaDidik{Nama: username, NIS: username + "-nis", NISN: username + "-nisn", Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("Siswa123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	account := User{Username: username, PasswordHash: string(hash), Role: "siswa", IsActive: true, PesertaDidikID: &student.ID}
	if err := s.db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	return student, account.ID
}

func simulasiLogin(t *testing.T, app *fiber.App, username string) string {
	t.Helper()
	response, err := makeRequest(app, http.MethodPost, "/api/auth/login", "", map[string]string{"login": username, "password": "Siswa123"}, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("student login %q failed: %v", username, err)
	}
	defer response.Body.Close()
	var body struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.AccessToken == "" {
		t.Fatalf("decode student token: %v", err)
	}
	return body.AccessToken
}

func TestSimulasiMapelChoicesAreScopedToTutorAssignments(t *testing.T) {
	s, app := setupE2EServer(t)
	tutor := Tutor{Nama: "Tutor Asesmen", JenisKelamin: "P"}
	otherTutor := Tutor{Nama: "Tutor Lain", JenisKelamin: "L"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherTutor).Error; err != nil {
		t.Fatal(err)
	}
	teacher := User{Username: "simulasi-mapel-teacher", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := s.db.Create(&teacher).Error; err != nil {
		t.Fatal(err)
	}
	ownedClass := Kelas{Jenjang: 5, NamaRombel: "A", WaliKelasID: &tutor.ID}
	otherClass := Kelas{Jenjang: 5, NamaRombel: "B", WaliKelasID: &otherTutor.ID}
	for _, class := range []*Kelas{&ownedClass, &otherClass} {
		if err := s.db.Create(class).Error; err != nil {
			t.Fatal(err)
		}
	}
	classSubject := MataPelajaran{NamaMapel: "Matematika Kelas Wali"}
	taughtSubject := MataPelajaran{NamaMapel: "Bahasa Indonesia Ditugaskan"}
	foreignSubject := MataPelajaran{NamaMapel: "IPA Kelas Lain"}
	for _, subject := range []*MataPelajaran{&classSubject, &taughtSubject, &foreignSubject} {
		if err := s.db.Create(subject).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, relation := range []*KelasMapel{
		{KelasID: ownedClass.ID, MapelID: classSubject.ID},
		{KelasID: otherClass.ID, MapelID: foreignSubject.ID},
	} {
		if err := s.db.Create(relation).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: ownedClass.ID, MapelID: taughtSubject.ID}).Error; err != nil {
		t.Fatal(err)
	}

	teacherToken, err := s.token(teacher, s.cfg.AccessSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	leader := User{Username: "simulasi-mapel-leader", Role: "kepala_sekolah", IsActive: true}
	studentRecord := PesertaDidik{Nama: "Simulasi Mapel Student", NIS: "SMS-001", NISN: "SMS-001", Status: "aktif"}
	if err := s.db.Create(&studentRecord).Error; err != nil {
		t.Fatal(err)
	}
	student := User{Username: "simulasi-mapel-student", Role: "siswa", IsActive: true, PesertaDidikID: &studentRecord.ID}
	for _, user := range []*User{&leader, &student} {
		if err := s.db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	leaderToken, err := s.token(leader, s.cfg.AccessSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	studentToken, err := s.token(student, s.cfg.AccessSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	call := func(token string) (*http.Response, error) {
		return makeRequest(app, http.MethodGet, "/api/mapel", token, nil, "")
	}

	response, err := call(teacherToken)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("teacher subject options: status=%v err=%v", response, err)
	}
	var teacherSubjects []MataPelajaran
	if err := json.NewDecoder(response.Body).Decode(&teacherSubjects); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	got := map[string]bool{}
	for _, subject := range teacherSubjects {
		got[subject.ID] = true
	}
	if !got[classSubject.ID] || !got[taughtSubject.ID] || got[foreignSubject.ID] {
		t.Fatalf("tutor should see class and teaching assignments only, got IDs=%v", got)
	}

	response, err = call(leaderToken)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("read-only school leader subject options: status=%v err=%v", response, err)
	}
	var leaderSubjects []MataPelajaran
	if err := json.NewDecoder(response.Body).Decode(&leaderSubjects); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if len(leaderSubjects) != 3 {
		t.Fatalf("read-only leader should see all subjects, got %d", len(leaderSubjects))
	}

	response, err = call(studentToken)
	if err != nil || response.StatusCode != http.StatusForbidden {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("student must not read staff subject options: status=%v err=%v", response, err)
	}
	response.Body.Close()
}

func TestSimulasiTeacherBuilderAssignsOnlyOwnedClassStudents(t *testing.T) {
	s, app := setupE2EServer(t)
	tutor := Tutor{Nama: "Tutor Pembuat", JenisKelamin: "P"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	teacher := User{Username: "simulasi-builder-teacher", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := s.db.Create(&teacher).Error; err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 5, NamaRombel: "A", WaliKelasID: &tutor.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Kelas Tutor", NIS: "SBT-001", NISN: "SBT-001", KelasID: class.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	teacherToken, err := s.token(teacher, s.cfg.AccessSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"paket":           map[string]any{"nama": "Paket guru", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 60, "maksPercobaan": 1, "acakUrutan": true, "temaWarna": "#1c5d94"},
		"items":           []map[string]any{{"bobot": 1, "soal": map[string]any{"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": "2 + 2 = ?", "bobot": 1, "konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "3"}, {"id": "b", "text": "4"}}, "correctIds": []string{"b"}}}}},
		"pesertaDidikIds": []string{student.ID},
	}
	response, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", teacherToken, payload, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("teacher builder save: status=%v err=%v", response, err)
	}
	response.Body.Close()
	var assignments int64
	if err := s.db.Model(&SimulasiPenugasan{}).Where("peserta_didik_id = ?", student.ID).Count(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if assignments != 1 {
		t.Fatalf("expected the in-scope student assignment to be saved, got %d", assignments)
	}
}

func TestSimulasiQuestionValidationAllTypes(t *testing.T) {
	cases := []struct {
		name string
		tipe string
		cfg  simulasiConfig
	}{
		{"pg", simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}},
		{"pgk", simulasiTipePGK, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}},
		{"true-false", simulasiTipeBenarSalah, simulasiConfig{Statements: []simulasiStatement{{ID: "a", Text: "P", Correct: true}}}},
		{"match", simulasiTipeMenjodohkan, simulasiConfig{Left: []simulasiChoice{{ID: "l", Text: "L"}, {ID: "l2", Text: "L2"}}, Right: []simulasiChoice{{ID: "r", Text: "R"}, {ID: "r2", Text: "R2"}}, Pairs: map[string]string{"l": "r", "l2": "r2"}}},
		{"short", simulasiTipeIsian, simulasiConfig{AcceptedAnswers: []string{"jawaban"}}},
		{"essay", simulasiTipeUraian, simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Isi", Maks: 1}}}},
		{"dropdown", simulasiTipeDropdown, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}},
		{"linear-scale", simulasiTipeSkala, simulasiConfig{ScaleMin: 1, ScaleMax: 5, CorrectNumber: intPtr(4)}},
		{"rating", simulasiTipeRating, simulasiConfig{RatingMax: 5, CorrectNumber: intPtr(4)}},
		{"single-grid", simulasiTipeKisiPG, simulasiConfig{Rows: []simulasiGridRow{{ID: "r1", Text: "R1"}}, Columns: []simulasiChoice{{ID: "c1", Text: "C1"}, {ID: "c2", Text: "C2"}}, GridCorrect: map[string]string{"r1": "c1"}}},
		{"checkbox-grid", simulasiTipeKisiPGK, simulasiConfig{Rows: []simulasiGridRow{{ID: "r1", Text: "R1"}}, Columns: []simulasiChoice{{ID: "c1", Text: "C1"}, {ID: "c2", Text: "C2"}}, GridMultiCorrect: map[string][]string{"r1": {"c1"}}}},
		{"date", simulasiTipeTanggal, simulasiConfig{AcceptedAnswers: []string{"2026-09-24"}}},
		{"time", simulasiTipeWaktu, simulasiConfig{AcceptedAnswers: []string{"08:30"}}},
		{"ordering", simulasiTipeUrutan, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectOrder: []string{"a", "b"}}},
		{"file-upload", simulasiTipeUnggah, simulasiConfig{AllowedFileTypes: []string{"pdf", "docx"}, MaxFiles: 2, MaxFileSizeMB: 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSimulasiConfig(tc.tipe, tc.cfg); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := validateSimulasiConfig(simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}}, CorrectIDs: []string{"missing"}}); err == nil {
		t.Fatal("invalid PG key must be rejected")
	}
	if err := validateSimulasiConfig(simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}, PartialScoring: "proportional"}); err == nil {
		t.Fatal("proportional scoring must not be enabled for single-choice items")
	}
	if err := validateSimulasiConfig(simulasiTipePGK, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}, PartialScoring: "unknown"}); err == nil {
		t.Fatal("unknown partial scoring rule must be rejected")
	}
	if protected, err := json.Marshal(sanitizedConfig(simulasiTipePGK, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}, PartialScoring: "proportional"})); err != nil || strings.Contains(string(protected), "partialScoring") || strings.Contains(string(protected), "correctIds") {
		t.Fatalf("student config exposed grading policy or key: %s (err=%v)", protected, err)
	}
	if protected, err := json.Marshal(sanitizedConfig(simulasiTipeUraian, simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Rahasia", Maks: 2}}})); err != nil || strings.Contains(string(protected), "rubrik") || strings.Contains(string(protected), "Rahasia") {
		t.Fatalf("student uraian payload exposed rubric: %s (err=%v)", protected, err)
	}
	if protected, err := json.Marshal(sanitizedConfig(simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}, BranchToByAnswer: map[string]string{"a": "rahasia", "b": simulasiBranchFinish}})); err != nil || strings.Contains(string(protected), "branchToByAnswer") || strings.Contains(string(protected), "rahasia") || strings.Contains(string(protected), simulasiBranchFinish) {
		t.Fatalf("student config exposed internal branch routing: %s (err=%v)", protected, err)
	}
	if protected := studentAttemptResponse(SimulasiUpaya{SkorOtomatis: 100, SeedUrutan: "secret-seed", Status: "selesai"}, SimulasiPaket{TampilkanNilai: false}); protected["skor"] != nil || protected["seedUrutan"] != nil {
		t.Fatalf("student attempt exposed hidden fields: %#v", protected)
	}
}

func TestSimulasiBranchingValidationRequiresForwardSectionRoutes(t *testing.T) {
	sections := []SimulasiBagian{{ClientID: "awal", Urutan: 1}, {ClientID: "akhir", Urutan: 2}}
	makeItem := func(route string, required bool) SimulasiPaketSoal {
		snapshot, err := json.Marshal(simulasiSnapshot{Tipe: simulasiTipePG, WajibDijawab: required, Konfigurasi: simulasiConfig{
			Choices:    []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}},
			CorrectIDs: []string{"a"}, BranchToByAnswer: map[string]string{"a": route},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return SimulasiPaketSoal{Base: Base{ID: "branch-question"}, BagianID: "awal", SnapshotJSON: string(snapshot)}
	}
	if err := validateSimulasiPackageBranching([]SimulasiPaketSoal{makeItem("akhir", true)}, sections); err != nil {
		t.Fatalf("forward route rejected: %v", err)
	}
	if err := validateSimulasiPackageBranching([]SimulasiPaketSoal{makeItem("awal", true)}, sections); err == nil {
		t.Fatal("backward route must be rejected to prevent loops")
	}
	if err := validateSimulasiPackageBranching([]SimulasiPaketSoal{makeItem("missing", true)}, sections); err == nil {
		t.Fatal("unknown route target must be rejected")
	}
	if err := validateSimulasiPackageBranching([]SimulasiPaketSoal{makeItem("akhir", false)}, sections); err == nil {
		t.Fatal("branch question must be required so the route cannot be bypassed")
	}
	if err := validateSimulasiConfig(simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}, BranchToByAnswer: map[string]string{"missing": "akhir"}}); err == nil {
		t.Fatal("route referring to a nonexistent choice must be rejected")
	}
	if err := validateSimulasiConfig(simulasiTipeIsian, simulasiConfig{AcceptedAnswers: []string{"ok"}, BranchToByAnswer: map[string]string{"ok": "akhir"}}); err == nil {
		t.Fatal("branching must only be available on single-choice controls")
	}
}

func TestSimulasiPackageThemeAndConfirmationValidation(t *testing.T) {
	base := simulasiPaketInput{Nama: "Tema aksesibel", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 30, MaksPercobaan: 1}
	var packet SimulasiPaket
	if err := applyPaketInput(&packet, base); err != nil {
		t.Fatalf("default theme rejected: %v", err)
	}
	if packet.TemaWarna != "#1c5d94" {
		t.Fatalf("default theme = %q", packet.TemaWarna)
	}
	base.TemaWarna = "#166534"
	base.PesanKonfirmasi = "Jawaban sudah diterima."
	if err := applyPaketInput(&packet, base); err != nil {
		t.Fatalf("supported theme rejected: %v", err)
	}
	if packet.TemaWarna != "#166534" || packet.PesanKonfirmasi != "Jawaban sudah diterima." {
		t.Fatalf("theme settings not stored: %#v", packet)
	}
	base.TemaWarna = "url(javascript:alert(1))"
	if err := applyPaketInput(&packet, base); err == nil {
		t.Fatal("untrusted CSS value must be rejected")
	}
	base.TemaWarna = "#1c5d94"
	base.PesanKonfirmasi = strings.Repeat("a", 501)
	if err := applyPaketInput(&packet, base); err == nil {
		t.Fatal("confirmation message over 500 characters must be rejected")
	}
}

func TestSimulasiTextResponseValidationConfigAndUnicodeLength(t *testing.T) {
	valid := simulasiConfig{AcceptedAnswers: []string{"iya"}, TextMinLength: 2, TextMaxLength: 4, ValidationMessage: "Jawaban harus 2 sampai 4 karakter."}
	if err := validateSimulasiConfig(simulasiTipeIsian, valid); err != nil {
		t.Fatalf("valid response validation rejected: %v", err)
	}
	if err := validateSimulasiConfig(simulasiTipeIsian, simulasiConfig{AcceptedAnswers: []string{"iya"}, TextMinLength: 5, TextMaxLength: 4}); err == nil {
		t.Fatal("minimum length above maximum should be rejected")
	}
	if err := validateSimulasiConfig(simulasiTipePG, simulasiConfig{TextMinLength: 2}); err == nil {
		t.Fatal("text length rules should not be accepted for a non-text question")
	}
	snapshot := simulasiSnapshot{Tipe: simulasiTipeIsian, Konfigurasi: valid}
	if err := validateSimulasiResponseRules(snapshot, `"a"`); err == nil || !strings.Contains(err.Error(), valid.ValidationMessage) {
		t.Fatalf("short response did not return teacher feedback: %v", err)
	}
	if err := validateSimulasiResponseRules(snapshot, `"abcdef"`); err == nil {
		t.Fatal("long response should be rejected")
	}
	if err := validateSimulasiResponseRules(snapshot, `"éé"`); err != nil {
		t.Fatalf("length should count Unicode characters, not bytes: %v", err)
	}
	if err := validateSimulasiResponseRules(snapshot, `"  "`); err != nil {
		t.Fatalf("blank optional/intermediate text must remain saveable: %v", err)
	}
}

func TestSimulasiTextLengthRulesAreEnforcedAtSubmitButNotAutosave(t *testing.T) {
	s, app := setupE2EServer(t)
	_, _ = getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "validated-text-student")
	studentToken := simulasiLogin(t, app, "validated-text-student")
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}

	config := simulasiConfig{AcceptedAnswers: []string{"jawab"}, TextMinLength: 5, TextMaxLength: 12, ValidationMessage: "Tulis minimal 5 karakter dan maksimal 12 karakter."}
	encoded, _ := json.Marshal(config)
	question := SimulasiSoal{Jenjang: "SD/MI", Mode: "anbk_akm", Tipe: simulasiTipeIsian, Pertanyaan: "Jawaban singkat", Konfigurasi: string(encoded), Bobot: 1, Status: "terbit", DibuatOlehUserID: admin.ID}
	snapshot, err := snapshotFromQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	packet := SimulasiPaket{Nama: "Validasi teks", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 30, MaksPercobaan: 1, Status: "terbit", TampilkanNilai: true, DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	item := SimulasiPaketSoal{PaketID: packet.ID, Urutan: 1, Bobot: 1, SnapshotJSON: snapshot}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiPenugasan{PaketID: packet.ID, PesertaDidikID: student.ID}).Error; err != nil {
		t.Fatal(err)
	}
	started, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+packet.ID+"/mulai", studentToken, nil, "")
	if err != nil || started.StatusCode != http.StatusCreated {
		if started != nil {
			started.Body.Close()
		}
		t.Fatalf("start attempt: %v", err)
	}
	var attempt map[string]any
	_ = json.NewDecoder(started.Body).Decode(&attempt)
	started.Body.Close()
	var questionLink SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ?", attempt["id"]).First(&questionLink).Error; err != nil {
		t.Fatal(err)
	}

	saved, err := makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attempt["id"].(string)+"/jawaban/"+questionLink.ID, studentToken, map[string]any{"jawaban": "abc"}, "")
	if err != nil || saved.StatusCode != http.StatusOK {
		if saved != nil {
			saved.Body.Close()
		}
		t.Fatalf("partial text must autosave: %v", err)
	}
	saved.Body.Close()
	blocked, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attempt["id"].(string)+"/kirim", studentToken, nil, "")
	if err != nil || blocked.StatusCode != http.StatusBadRequest {
		if blocked != nil {
			blocked.Body.Close()
		}
		t.Fatalf("short text should be blocked on submit: %v", err)
	}
	body, _ := io.ReadAll(blocked.Body)
	blocked.Body.Close()
	if !strings.Contains(string(body), config.ValidationMessage) {
		t.Fatalf("submit should show teacher's validation message, got %s", body)
	}

	saved, _ = makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attempt["id"].(string)+"/jawaban/"+questionLink.ID, studentToken, map[string]any{"jawaban": "jawab"}, "")
	if saved.StatusCode != http.StatusOK {
		saved.Body.Close()
		t.Fatalf("valid answer autosave got %d", saved.StatusCode)
	}
	saved.Body.Close()
	submitted, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attempt["id"].(string)+"/kirim", studentToken, nil, "")
	if err != nil || submitted.StatusCode != http.StatusOK {
		if submitted != nil {
			submitted.Body.Close()
		}
		t.Fatalf("valid constrained response should submit: %v", err)
	}
	submitted.Body.Close()
}

func makeMultipartUploadRequest(app *fiber.App, url, token, filename string, contents []byte) (*http.Response, error) {
	return makeMultipartUploadRequestWithIdempotencyKey(app, url, token, filename, contents, "")
}

func makeMultipartUploadRequestWithIdempotencyKey(app *fiber.App, url, token, filename string, contents []byte, idempotencyKey string) (*http.Response, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(contents); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return app.Test(req, -1)
}

func makeRequestWithIdempotencyKey(app *fiber.App, method, url, token, idempotencyKey string) (*http.Response, error) {
	req := httptest.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", idempotencyKey)
	return app.Test(req, -1)
}

func TestSimulasiAnswerUploadIsPrivateScopedAndManuallyGradable(t *testing.T) {
	t.Setenv("UPLOADS_DIR", t.TempDir())
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "upload-student")
	_, _ = simulasiStudent(t, s, "upload-other")
	studentToken := simulasiLogin(t, app, "upload-student")
	otherToken := simulasiLogin(t, app, "upload-other")

	questionResponse, err := makeRequest(app, http.MethodPost, "/api/simulasi/soal", adminToken, map[string]any{
		"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipeUnggah, "pertanyaan": "Unggah hasil pekerjaanmu.", "bobot": 4,
		"status": "terbit", "konfigurasi": map[string]any{"allowedFileTypes": []string{"pdf"}, "maxFiles": 2, "maxFileSizeMB": 1},
	}, "")
	if err != nil || questionResponse.StatusCode != http.StatusCreated {
		if questionResponse != nil {
			questionResponse.Body.Close()
		}
		t.Fatalf("create upload question: %v", err)
	}
	var question map[string]any
	_ = json.NewDecoder(questionResponse.Body).Decode(&question)
	questionResponse.Body.Close()
	questionID := question["id"].(string)

	packetResponse, _ := makeRequest(app, http.MethodPost, "/api/simulasi/paket", adminToken, map[string]any{"nama": "Jawaban Berkas", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 15, "maksPercobaan": 1, "tampilkanNilai": true, "izinkanEditRespons": true}, "")
	if packetResponse == nil || packetResponse.StatusCode != http.StatusCreated {
		if packetResponse != nil {
			packetResponse.Body.Close()
		}
		t.Fatal("create upload packet failed")
	}
	var packet map[string]any
	_ = json.NewDecoder(packetResponse.Body).Decode(&packet)
	packetResponse.Body.Close()
	packetID := packet["id"].(string)
	for _, setup := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/simulasi/paket/" + packetID + "/soal", map[string]any{"soalId": questionID}},
		{http.MethodPut, "/api/simulasi/paket/" + packetID + "/penugasan", map[string]any{"pesertaDidikIds": []string{student.ID}}},
		{http.MethodPost, "/api/simulasi/paket/" + packetID + "/publikasi", nil},
	} {
		response, setupErr := makeRequest(app, setup.method, setup.path, adminToken, setup.body, "")
		if setupErr != nil || response == nil || response.StatusCode >= 300 {
			if response != nil {
				response.Body.Close()
			}
			t.Fatalf("upload package setup %s: %v", setup.path, setupErr)
		}
		response.Body.Close()
	}
	startResponse, _ := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+packetID+"/mulai", studentToken, nil, "")
	if startResponse == nil || startResponse.StatusCode != http.StatusCreated {
		if startResponse != nil {
			startResponse.Body.Close()
		}
		t.Fatal("start upload attempt failed")
	}
	var attempt map[string]any
	_ = json.NewDecoder(startResponse.Body).Decode(&attempt)
	startResponse.Body.Close()
	attemptID := attempt["id"].(string)
	workspaceResponse, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID, studentToken, nil, "")
	var workspace struct {
		Soal []struct {
			UpayaSoalID string `json:"upayaSoalId"`
			Soal        struct {
				Konfigurasi map[string]any `json:"konfigurasi"`
			} `json:"soal"`
		} `json:"soal"`
	}
	if workspaceResponse == nil || workspaceResponse.StatusCode != http.StatusOK {
		if workspaceResponse != nil {
			workspaceResponse.Body.Close()
		}
		t.Fatal("load upload workspace failed")
	}
	_ = json.NewDecoder(workspaceResponse.Body).Decode(&workspace)
	workspaceResponse.Body.Close()
	if len(workspace.Soal) != 1 || len(workspace.Soal[0].Soal.Konfigurasi) == 0 {
		t.Fatalf("student upload configuration missing: %#v", workspace.Soal)
	}
	if _, leaksKey := workspace.Soal[0].Soal.Konfigurasi["rubrik"]; leaksKey {
		t.Fatal("student workspace must not receive file-answer rubric")
	}

	linkID := workspace.Soal[0].UpayaSoalID
	badUpload, _ := makeMultipartUploadRequest(app, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+linkID, studentToken, "not-a-pdf.pdf", []byte("not really a pdf"))
	if badUpload == nil || badUpload.StatusCode != http.StatusBadRequest {
		if badUpload != nil {
			badUpload.Body.Close()
		}
		t.Fatal("upload must check the actual signature, not just the extension")
	}
	badUpload.Body.Close()

	fileContents := []byte("%PDF-1.7\nstudent answer\n%%EOF")
	upload, uploadErr := makeMultipartUploadRequest(app, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+linkID, studentToken, "jawaban.pdf", fileContents)
	if uploadErr != nil || upload == nil || upload.StatusCode != http.StatusCreated {
		if upload != nil {
			upload.Body.Close()
		}
		t.Fatalf("private answer upload: %v", uploadErr)
	}
	var uploaded map[string]any
	_ = json.NewDecoder(upload.Body).Decode(&uploaded)
	upload.Body.Close()
	fileID := uploaded["id"].(string)

	otherDownload, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+fileID, otherToken, nil, "")
	if otherDownload == nil || otherDownload.StatusCode != http.StatusNotFound {
		if otherDownload != nil {
			otherDownload.Body.Close()
		}
		t.Fatal("another student must not be able to download this answer file")
	}
	otherDownload.Body.Close()
	staffDetail, _ := makeRequest(app, http.MethodGet, "/api/simulasi/upaya/"+attemptID+"/detail", adminToken, nil, "")
	staffBody, _ := io.ReadAll(staffDetail.Body)
	staffDetail.Body.Close()
	if !strings.Contains(string(staffBody), "jawaban.pdf") || strings.Contains(string(staffBody), "FilePath") || strings.Contains(string(staffBody), "simulasi-jawaban") {
		t.Fatalf("staff detail must show safe file metadata without private storage paths: %s", staffBody)
	}

	deleteResponse, _ := makeRequest(app, http.MethodDelete, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+fileID, studentToken, nil, "")
	if deleteResponse == nil || deleteResponse.StatusCode != http.StatusNoContent {
		if deleteResponse != nil {
			deleteResponse.Body.Close()
		}
		t.Fatal("owner should be able to remove a file while the attempt is active")
	}
	deleteResponse.Body.Close()
	counted := int64(0)
	if err := s.db.Model(&SimulasiJawabanFile{}).Where("upaya_soal_id = ?", linkID).Count(&counted).Error; err != nil || counted != 0 {
		t.Fatalf("deleted upload should be removed from active answer references, count=%d err=%v", counted, err)
	}
	secondUpload, uploadErr := makeMultipartUploadRequest(app, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+linkID, studentToken, "jawaban-final.pdf", fileContents)
	if uploadErr != nil || secondUpload == nil || secondUpload.StatusCode != http.StatusCreated {
		if secondUpload != nil {
			secondUpload.Body.Close()
		}
		t.Fatalf("re-upload answer file: %v", uploadErr)
	}
	var secondUploaded map[string]any
	_ = json.NewDecoder(secondUpload.Body).Decode(&secondUploaded)
	secondUpload.Body.Close()
	fileID = secondUploaded["id"].(string)
	submitted, _ := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attemptID+"/kirim", studentToken, nil, "")
	if submitted == nil || submitted.StatusCode != http.StatusOK {
		if submitted != nil {
			submitted.Body.Close()
		}
		t.Fatal("uploaded response should be submittable")
	}
	submitted.Body.Close()
	var answer SimulasiJawaban
	if err := s.db.Where("upaya_soal_id = ?", linkID).First(&answer).Error; err != nil {
		t.Fatal(err)
	}
	graded, _ := makeRequest(app, http.MethodPost, "/api/simulasi/upaya/"+attemptID+"/jawaban/"+answer.ID+"/nilai", adminToken, map[string]any{"skor": 3, "komentar": "Langkah pengerjaan sudah benar."}, "")
	if graded == nil || graded.StatusCode != http.StatusOK {
		if graded != nil {
			graded.Body.Close()
		}
		t.Fatal("uploaded response should support manual grading")
	}
	graded.Body.Close()

	newContents := []byte("%PDF-1.7\nupdated student answer\n%%EOF")
	newFileKey := "00000000-0000-4000-8000-000000000001"
	postSubmitUpload, uploadErr := makeMultipartUploadRequestWithIdempotencyKey(app, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+linkID, studentToken, "jawaban-revisi.pdf", newContents, newFileKey)
	if uploadErr != nil || postSubmitUpload == nil || postSubmitUpload.StatusCode != http.StatusCreated {
		if postSubmitUpload != nil {
			postSubmitUpload.Body.Close()
		}
		t.Fatalf("post-submit upload revision: %v", uploadErr)
	}
	var revisedFile map[string]any
	_ = json.NewDecoder(postSubmitUpload.Body).Decode(&revisedFile)
	postSubmitUpload.Body.Close()
	newFileID, ok := revisedFile["id"].(string)
	if !ok || newFileID == "" {
		t.Fatalf("post-submit upload did not return a file id: %#v", revisedFile)
	}
	replayedUpload, uploadErr := makeMultipartUploadRequestWithIdempotencyKey(app, "/api/simulasi/saya/upaya/"+attemptID+"/file/"+linkID, studentToken, "jawaban-revisi.pdf", newContents, newFileKey)
	if uploadErr != nil || replayedUpload == nil || replayedUpload.StatusCode != http.StatusCreated {
		if replayedUpload != nil {
			replayedUpload.Body.Close()
		}
		t.Fatalf("retry post-submit upload: %v", uploadErr)
	}
	var replayedFile map[string]any
	_ = json.NewDecoder(replayedUpload.Body).Decode(&replayedFile)
	replayedUpload.Body.Close()
	if replayedFile["id"] != newFileID || replayedFile["revisionReplay"] != true {
		t.Fatalf("upload retry should replay the original revision: first=%#v retry=%#v", revisedFile, replayedFile)
	}
	var updatedAnswer SimulasiJawaban
	if err := s.db.Where("upaya_soal_id = ?", linkID).First(&updatedAnswer).Error; err != nil {
		t.Fatal(err)
	}
	if updatedAnswer.SkorManual != nil || updatedAnswer.KomentarGuru != "" || updatedAnswer.DinilaiPada != nil {
		t.Fatalf("changed attachment must clear its now-stale manual grade: %#v", updatedAnswer)
	}
	deleteKey := "00000000-0000-4000-8000-000000000002"
	deleteURL := "/api/simulasi/saya/upaya/" + attemptID + "/file/" + fileID
	deleteRevisedFile, deleteErr := makeRequestWithIdempotencyKey(app, http.MethodDelete, deleteURL, studentToken, deleteKey)
	if deleteErr != nil || deleteRevisedFile == nil || deleteRevisedFile.StatusCode != http.StatusNoContent {
		if deleteRevisedFile != nil {
			deleteRevisedFile.Body.Close()
		}
		t.Fatalf("post-submit delete revision: %v", deleteErr)
	}
	deleteRevisedFile.Body.Close()
	deleteRetry, deleteErr := makeRequestWithIdempotencyKey(app, http.MethodDelete, deleteURL, studentToken, deleteKey)
	if deleteErr != nil || deleteRetry == nil || deleteRetry.StatusCode != http.StatusNoContent {
		if deleteRetry != nil {
			deleteRetry.Body.Close()
		}
		t.Fatalf("retry post-submit delete: %v", deleteErr)
	}
	deleteRetry.Body.Close()
	var archivedFile SimulasiJawabanFile
	if err := s.db.First(&archivedFile, "id = ?", fileID).Error; err != nil || archivedFile.Aktif {
		t.Fatalf("submitted file should remain archived for audit, file=%#v err=%v", archivedFile, err)
	}
	var revisions int64
	if err := s.db.Model(&SimulasiJawabanRevisi{}).Where("upaya_id = ?", attemptID).Count(&revisions).Error; err != nil || revisions != 2 {
		t.Fatalf("expected one immutable revision per attachment edit, count=%d err=%v", revisions, err)
	}
	workspaceAfterEdit, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID, studentToken, nil, "")
	if workspaceAfterEdit == nil || workspaceAfterEdit.StatusCode != http.StatusOK {
		if workspaceAfterEdit != nil {
			workspaceAfterEdit.Body.Close()
		}
		t.Fatal("student should be able to reload revised file response")
	}
	var revisedWorkspace struct {
		Soal []struct {
			Jawaban []struct {
				ID string `json:"id"`
			} `json:"jawaban"`
		} `json:"soal"`
	}
	_ = json.NewDecoder(workspaceAfterEdit.Body).Decode(&revisedWorkspace)
	workspaceAfterEdit.Body.Close()
	if len(revisedWorkspace.Soal) != 1 || len(revisedWorkspace.Soal[0].Jawaban) != 1 || revisedWorkspace.Soal[0].Jawaban[0].ID != newFileID {
		t.Fatalf("student workspace should show only the current active attachment: %#v", revisedWorkspace.Soal)
	}
	treatArchivedAsActive, _ := makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attemptID+"/jawaban/"+linkID+"/revisi", studentToken, map[string]any{"jawaban": []string{fileID}, "idempotencyKey": "00000000-0000-4000-8000-000000000003"}, "")
	if treatArchivedAsActive == nil || treatArchivedAsActive.StatusCode != http.StatusBadRequest {
		if treatArchivedAsActive != nil {
			treatArchivedAsActive.Body.Close()
		}
		t.Fatal("a response revision must not reattach an archived file")
	}
	treatArchivedAsActive.Body.Close()
}

func TestSimulasiStaffResultsFilterUsesHistoricalClassWithLegacyFallback(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	oldClass := Kelas{Jenjang: 5, NamaRombel: "Hasil-Lama"}
	newClass := Kelas{Jenjang: 6, NamaRombel: "Hasil-Baru"}
	if err := s.db.Create(&oldClass).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&newClass).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Pindah Kelas", NIS: "hasil-historis-nis", NISN: "hasil-historis-nisn", KelasID: newClass.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	packet := SimulasiPaket{Nama: "Rekam Kelas Historis", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 20, MaksPercobaan: 3, Status: "terbit", DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiPenugasan{PaketID: packet.ID, PesertaDidikID: student.ID, KelasIDSaatTugas: oldClass.ID}).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Hour)
	attempts := []SimulasiUpaya{
		{PaketID: packet.ID, PesertaDidikID: student.ID, KelasIDSaatUjian: oldClass.ID, Nomor: 1, Status: "selesai", Mulai: &started, SeedUrutan: "old-class-seed"},
		{PaketID: packet.ID, PesertaDidikID: student.ID, KelasIDSaatUjian: newClass.ID, Nomor: 2, Status: "selesai", Mulai: &started, SeedUrutan: "new-class-seed"},
		{PaketID: packet.ID, PesertaDidikID: student.ID, Nomor: 3, Status: "selesai", Mulai: &started, SeedUrutan: "legacy-class-seed"},
	}
	for index := range attempts {
		if err := s.db.Create(&attempts[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	readAttemptIDs := func(classID string) []string {
		t.Helper()
		response, err := makeRequest(app, http.MethodGet, "/api/simulasi/paket/"+packet.ID+"/hasil?kelasId="+classID, adminToken, nil, "")
		if err != nil || response == nil || response.StatusCode != http.StatusOK {
			if response != nil {
				body, _ := io.ReadAll(response.Body)
				response.Body.Close()
				t.Fatalf("filter package results by class: status=%v err=%v body=%s", response.StatusCode, err, body)
			}
			t.Fatalf("filter package results by class: %v", err)
		}
		defer response.Body.Close()
		var rows []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(response.Body).Decode(&rows); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids
	}
	oldResults := readAttemptIDs(oldClass.ID)
	if len(oldResults) != 2 || !containsString(oldResults, attempts[0].ID) || !containsString(oldResults, attempts[2].ID) {
		t.Fatalf("old class should include its snapshot and assignment-context fallback attempts, got %#v", oldResults)
	}
	newResults := readAttemptIDs(newClass.ID)
	if len(newResults) != 1 || newResults[0] != attempts[1].ID {
		t.Fatalf("new class should include only its captured attempt, got %#v", newResults)
	}
}

func intPtr(value int) *int { return &value }

func TestProportionalScoringAcrossCompositeQuestionTypes(t *testing.T) {
	cases := []struct {
		name       string
		tipe       string
		config     simulasiConfig
		answer     string
		strict     string
		wantScore  float64
		wantStrict float64
	}{
		{
			name:   "multiple-select",
			tipe:   simulasiTipePGK,
			config: simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}, {ID: "c", Text: "C"}}, CorrectIDs: []string{"a", "b"}, PartialScoring: "proportional"},
			answer: ` ["a"] `, strict: `["a"]`, wantScore: 5, wantStrict: 0,
		},
		{
			name:   "true-false-table",
			tipe:   simulasiTipeBenarSalah,
			config: simulasiConfig{Statements: []simulasiStatement{{ID: "r1", Correct: true}, {ID: "r2", Correct: false}}, PartialScoring: "proportional"},
			answer: `{"r1":true}`, strict: `{"r1":true}`, wantScore: 5, wantStrict: 0,
		},
		{
			name:   "matching",
			tipe:   simulasiTipeMenjodohkan,
			config: simulasiConfig{Pairs: map[string]string{"l1": "r1", "l2": "r2"}, PartialScoring: "proportional"},
			answer: `{"l1":"r1"}`, strict: `{"l1":"r1"}`, wantScore: 5, wantStrict: 0,
		},
		{
			name:   "single-choice-grid",
			tipe:   simulasiTipeKisiPG,
			config: simulasiConfig{GridCorrect: map[string]string{"r1": "c1", "r2": "c2"}, PartialScoring: "proportional"},
			answer: `{"r1":"c1"}`, strict: `{"r1":"c1"}`, wantScore: 5, wantStrict: 0,
		},
		{
			name:   "checkbox-grid",
			tipe:   simulasiTipeKisiPGK,
			config: simulasiConfig{GridMultiCorrect: map[string][]string{"r1": {"c1", "c2"}, "r2": {"c3"}}, PartialScoring: "proportional"},
			answer: `{"r1":["c1"],"r2":["c3"]}`, strict: `{"r1":["c1"],"r2":["c3"]}`, wantScore: 7.5, wantStrict: 0,
		},
		{
			name:   "ordered-list",
			tipe:   simulasiTipeUrutan,
			config: simulasiConfig{CorrectOrder: []string{"a", "b", "c"}, PartialScoring: "proportional"},
			answer: `["a","x","c"]`, strict: `["a","x","c"]`, wantScore: 20.0 / 3, wantStrict: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := simulasiSnapshot{Tipe: tc.tipe, Konfigurasi: tc.config}
			correct, score, manual := gradeSnapshot(snapshot, tc.answer, 10)
			if correct || manual || math.Abs(score-tc.wantScore) > 0.0001 {
				t.Fatalf("partial score: correct=%v score=%v manual=%v, want %v", correct, score, manual, tc.wantScore)
			}
			strictConfig := tc.config
			strictConfig.PartialScoring = ""
			_, strictScore, strictManual := gradeSnapshot(simulasiSnapshot{Tipe: tc.tipe, Konfigurasi: strictConfig}, tc.strict, 10)
			if strictManual || strictScore != tc.wantStrict {
				t.Fatalf("default exact score=%v manual=%v, want %v", strictScore, strictManual, tc.wantStrict)
			}
		})
	}
}

func TestProportionalScoringPenalizesUnknownAnswerFields(t *testing.T) {
	tests := []struct {
		name   string
		tipe   string
		config simulasiConfig
		answer string
	}{
		{"extra tf row", simulasiTipeBenarSalah, simulasiConfig{Statements: []simulasiStatement{{ID: "r1", Correct: true}}, PartialScoring: "proportional"}, `{"r1":true,"foreign":true}`},
		{"extra pair", simulasiTipeMenjodohkan, simulasiConfig{Pairs: map[string]string{"l1": "r1"}, PartialScoring: "proportional"}, `{"l1":"r1","foreign":"r1"}`},
		{"extra grid row", simulasiTipeKisiPG, simulasiConfig{GridCorrect: map[string]string{"r1": "c1"}, PartialScoring: "proportional"}, `{"r1":"c1","foreign":"c1"}`},
		{"extra checkbox grid row", simulasiTipeKisiPGK, simulasiConfig{GridMultiCorrect: map[string][]string{"r1": {"c1"}}, PartialScoring: "proportional"}, `{"r1":["c1"],"foreign":["c1"]}`},
		{"extra order item", simulasiTipeUrutan, simulasiConfig{CorrectOrder: []string{"a"}, PartialScoring: "proportional"}, `["a","foreign"]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, score, _ := gradeSnapshot(simulasiSnapshot{Tipe: tc.tipe, Konfigurasi: tc.config}, tc.answer, 10)
			if score != 0 {
				t.Fatalf("unknown extra answer fields must not receive full credit; score=%v", score)
			}
		})
	}
}

func TestSimulasiAddedQuestionTypesGradeAndHideKeys(t *testing.T) {
	cases := []struct {
		name     string
		snapshot simulasiSnapshot
		answer   string
	}{
		{"dropdown", simulasiSnapshot{Tipe: simulasiTipeDropdown, Konfigurasi: simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}}, CorrectIDs: []string{"a"}}}, `"a"`},
		{"linear-scale", simulasiSnapshot{Tipe: simulasiTipeSkala, Konfigurasi: simulasiConfig{ScaleMin: 1, ScaleMax: 5, CorrectNumber: intPtr(4)}}, `4`},
		{"rating", simulasiSnapshot{Tipe: simulasiTipeRating, Konfigurasi: simulasiConfig{RatingMax: 5, CorrectNumber: intPtr(4)}}, `4`},
		{"single-grid", simulasiSnapshot{Tipe: simulasiTipeKisiPG, Konfigurasi: simulasiConfig{GridCorrect: map[string]string{"r1": "c1", "r2": "c2"}}}, `{"r1":"c1","r2":"c2"}`},
		{"checkbox-grid", simulasiSnapshot{Tipe: simulasiTipeKisiPGK, Konfigurasi: simulasiConfig{GridMultiCorrect: map[string][]string{"r1": {"c1", "c2"}}}}, `{"r1":["c2","c1"]}`},
		{"date", simulasiSnapshot{Tipe: simulasiTipeTanggal, Konfigurasi: simulasiConfig{AcceptedAnswers: []string{"2026-09-24"}}}, `"2026-09-24"`},
		{"time", simulasiSnapshot{Tipe: simulasiTipeWaktu, Konfigurasi: simulasiConfig{AcceptedAnswers: []string{"08:30"}}}, `"08:30"`},
		{"ordering", simulasiSnapshot{Tipe: simulasiTipeUrutan, Konfigurasi: simulasiConfig{CorrectOrder: []string{"a", "b", "c"}}}, `["a","b","c"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			correct, score, manual := gradeSnapshot(tc.snapshot, tc.answer, 5)
			if !correct || score != 5 || manual {
				t.Fatalf("expected full automatic score, got correct=%v score=%v manual=%v", correct, score, manual)
			}
			encoded, err := json.Marshal(sanitizedConfig(tc.snapshot.Tipe, tc.snapshot.Konfigurasi))
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"correctIds", "correctNumber", "gridCorrect", "gridMultiCorrect", "correctOrder", "acceptedAnswers"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("student config leaked %q: %s", secret, encoded)
				}
			}
		})
	}
	invalidScale := simulasiConfig{ScaleMin: 1, ScaleMax: 5, CorrectNumber: intPtr(8)}
	if err := validateSimulasiConfig(simulasiTipeSkala, invalidScale); err == nil {
		t.Fatal("out-of-range scale key must be rejected")
	}
	invalidGrid := simulasiConfig{Rows: []simulasiGridRow{{ID: "r1", Text: "R1"}}, Columns: []simulasiChoice{{ID: "c1", Text: "C1"}, {ID: "c2", Text: "C2"}}, GridCorrect: map[string]string{"r1": "missing"}}
	if err := validateSimulasiConfig(simulasiTipeKisiPG, invalidGrid); err == nil {
		t.Fatal("invalid grid key must be rejected")
	}
}

func TestSimulasiBuilderAutosavePublishesDraftSources(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "builder-student")
	payload := map[string]any{
		"paket":           map[string]any{"nama": "", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1, "acakUrutan": true, "tampilkanNilai": true, "tampilkanRingkasan": true},
		"items":           []map[string]any{{"bobot": 1, "soal": map[string]any{"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": "2 + 2 = ?", "bobot": 1, "konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "3"}, {"id": "b", "text": "4"}}, "correctIds": []string{"b"}}}}},
		"pesertaDidikIds": []string{student.ID},
	}
	response, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, payload, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("create builder draft: %v", err)
	}
	var body struct {
		Paket SimulasiPaket `json:"paket"`
		Items []struct {
			SoalID string `json:"soalId"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if body.Paket.Nama != "Paket tanpa judul" || len(body.Items) != 1 || body.Items[0].SoalID == "" {
		t.Fatalf("unexpected autosaved builder response: %+v", body)
	}
	publish, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+body.Paket.ID+"/publikasi", adminToken, nil, "")
	if err != nil || publish.StatusCode != http.StatusOK {
		if publish != nil {
			publish.Body.Close()
		}
		t.Fatalf("publish builder packet: %v", err)
	}
	publish.Body.Close()
	var source SimulasiSoal
	if err := s.db.First(&source, "id = ?", body.Items[0].SoalID).Error; err != nil || source.Status != "terbit" {
		t.Fatalf("draft source was not promoted: %v, status=%s", err, source.Status)
	}
}

func TestSimulasiBuilderSectionsSnapshotAndLearnerOrder(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "sections-student")
	makeQuestion := func(text string) map[string]any {
		return map[string]any{"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": text, "bobot": 1, "konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "A"}, {"id": "b", "text": "B"}}, "correctIds": []string{"b"}}}
	}
	payload := map[string]any{
		"paket":           map[string]any{"nama": "Form berbagian", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1, "acakUrutan": false},
		"sections":        []map[string]string{{"id": "bagian-pemahaman", "nama": "Pemahaman teks", "deskripsi": "Baca informasi pada stimulus."}, {"id": "bagian-nalar", "nama": "Penalaran"}},
		"items":           []map[string]any{{"bagianId": "bagian-nalar", "bobot": 1, "soal": makeQuestion("Soal bagian dua")}, {"bagianId": "bagian-pemahaman", "bobot": 1, "soal": makeQuestion("Soal bagian satu A")}, {"bagianId": "bagian-pemahaman", "bobot": 1, "soal": makeQuestion("Soal bagian satu B")}},
		"pesertaDidikIds": []string{student.ID},
	}
	created, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, payload, "")
	if err != nil || created.StatusCode != http.StatusOK {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("create section builder draft: status=%v err=%v", created, err)
	}
	var body struct {
		Paket    SimulasiPaket `json:"paket"`
		Sections []struct {
			ID   string `json:"id"`
			Nama string `json:"nama"`
		} `json:"sections"`
		Items []struct {
			BagianID string `json:"bagianId"`
		} `json:"items"`
	}
	if err := json.NewDecoder(created.Body).Decode(&body); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()
	if len(body.Sections) != 2 || len(body.Items) != 3 || body.Items[0].BagianID != "bagian-nalar" {
		t.Fatalf("sections/items were not returned from draft: %+v", body)
	}
	reloaded, err := makeRequest(app, http.MethodGet, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, nil, "")
	if err != nil || reloaded.StatusCode != http.StatusOK {
		if reloaded != nil {
			reloaded.Body.Close()
		}
		t.Fatalf("reload builder sections: status=%v err=%v", reloaded, err)
	}
	var persisted struct {
		Sections []struct {
			ID   string `json:"id"`
			Nama string `json:"nama"`
		} `json:"sections"`
	}
	if err := json.NewDecoder(reloaded.Body).Decode(&persisted); err != nil {
		reloaded.Body.Close()
		t.Fatal(err)
	}
	reloaded.Body.Close()
	if len(persisted.Sections) != 2 || persisted.Sections[0].ID != "bagian-pemahaman" || persisted.Sections[0].Nama != "Pemahaman teks" {
		t.Fatalf("builder reload did not preserve section settings: %+v", persisted.Sections)
	}
	duplicated, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+body.Paket.ID+"/duplikasi", adminToken, nil, "")
	if err != nil || duplicated.StatusCode != http.StatusCreated {
		if duplicated != nil {
			duplicated.Body.Close()
		}
		t.Fatalf("duplicate section packet: status=%v err=%v", duplicated, err)
	}
	var duplicate SimulasiPaket
	if err := json.NewDecoder(duplicated.Body).Decode(&duplicate); err != nil {
		duplicated.Body.Close()
		t.Fatal(err)
	}
	duplicated.Body.Close()
	var copiedSections []SimulasiBagian
	var copiedItems []SimulasiPaketSoal
	if err := s.db.Where("paket_id = ?", duplicate.ID).Find(&copiedSections).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("paket_id = ?", duplicate.ID).Find(&copiedItems).Error; err != nil {
		t.Fatal(err)
	}
	if len(copiedSections) != 2 || len(copiedItems) != 3 || copiedItems[0].BagianID == "" {
		t.Fatalf("duplicate did not keep section relations: sections=%+v items=%+v", copiedSections, copiedItems)
	}
	var savedSections []SimulasiBagian
	if err := s.db.Where("paket_id = ?", body.Paket.ID).Order("urutan").Find(&savedSections).Error; err != nil || len(savedSections) != 2 {
		t.Fatalf("persist section rows: %v, rows=%d", err, len(savedSections))
	}
	publish, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+body.Paket.ID+"/publikasi", adminToken, nil, "")
	if err != nil || publish.StatusCode != http.StatusOK {
		if publish != nil {
			publish.Body.Close()
		}
		t.Fatalf("publish section packet: status=%v err=%v", publish, err)
	}
	publish.Body.Close()
	studentToken := simulasiLogin(t, app, "sections-student")
	started, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+body.Paket.ID+"/mulai", studentToken, nil, "")
	if err != nil || started.StatusCode != http.StatusCreated {
		if started != nil {
			started.Body.Close()
		}
		t.Fatalf("start section packet: status=%v err=%v", started, err)
	}
	var attempt SimulasiUpaya
	if err := json.NewDecoder(started.Body).Decode(&attempt); err != nil {
		started.Body.Close()
		t.Fatal(err)
	}
	started.Body.Close()
	var links []SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ?", attempt.ID).Order("urutan_tampil").Find(&links).Error; err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 || links[0].NamaBagian != "Pemahaman teks" || links[1].NamaBagian != "Pemahaman teks" || links[2].NamaBagian != "Penalaran" || links[0].UrutanBagian != 1 {
		t.Fatalf("attempt question sections/order not snapshotted: %+v", links)
	}
	workspace, err := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attempt.ID, studentToken, nil, "")
	if err != nil || workspace.StatusCode != http.StatusOK {
		if workspace != nil {
			workspace.Body.Close()
		}
		t.Fatalf("read learner workspace: status=%v err=%v", workspace, err)
	}
	var learnerPayload map[string]any
	if err := json.NewDecoder(workspace.Body).Decode(&learnerPayload); err != nil {
		workspace.Body.Close()
		t.Fatal(err)
	}
	workspace.Body.Close()
	encoded, _ := json.Marshal(learnerPayload)
	if strings.Contains(string(encoded), "correctIds") || !strings.Contains(string(encoded), "Pemahaman teks") {
		t.Fatalf("learner section snapshot missing or answer key leaked: %s", encoded)
	}
}

func TestSimulasiBranchingLimitsLearnerPathAndScoring(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "branching-student")
	question := func(text string, choices []map[string]string, correct []string, branches map[string]string) map[string]any {
		config := map[string]any{"choices": choices, "correctIds": correct}
		if branches != nil {
			config["branchToByAnswer"] = branches
		}
		return map[string]any{"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": text, "bobot": 1, "wajibDijawab": true, "konfigurasi": config}
	}
	payload := map[string]any{
		"paket":    map[string]any{"nama": "Alur bercabang", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1, "acakUrutan": true},
		"sections": []map[string]string{{"id": "awal", "nama": "Pertanyaan awal"}, {"id": "dilewati", "nama": "Pendalaman opsional"}, {"id": "lanjutan", "nama": "Pendalaman sesuai jawaban"}},
		"items": []map[string]any{
			{"bagianId": "awal", "bobot": 1, "soal": question("Pilih jalur", []map[string]string{{"id": "lanjut", "text": "Lanjut"}, {"id": "selesai", "text": "Selesai"}}, []string{"lanjut"}, map[string]string{"lanjut": "lanjutan", "selesai": simulasiBranchFinish})},
			{"bagianId": "dilewati", "bobot": 100, "soal": question("Pertanyaan yang dilewati", []map[string]string{{"id": "x", "text": "X"}, {"id": "y", "text": "Y"}}, []string{"x"}, nil)},
			{"bagianId": "lanjutan", "bobot": 1, "soal": question("Pertanyaan lanjutan", []map[string]string{{"id": "x", "text": "X"}, {"id": "y", "text": "Y"}}, []string{"x"}, nil)},
		},
		"pesertaDidikIds": []string{student.ID},
	}
	created, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, payload, "")
	if err != nil || created.StatusCode != http.StatusOK {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("create branched package: response=%v err=%v", created, err)
	}
	var packageResponse struct {
		Paket struct {
			ID string `json:"id"`
		} `json:"paket"`
	}
	if err := json.NewDecoder(created.Body).Decode(&packageResponse); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()
	packageID := packageResponse.Paket.ID
	published, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+packageID+"/publikasi", adminToken, nil, "")
	if err != nil || published.StatusCode != http.StatusOK {
		if published != nil {
			body, _ := io.ReadAll(published.Body)
			published.Body.Close()
			t.Fatalf("publish branched package: status=%d body=%s err=%v", published.StatusCode, body, err)
		}
		t.Fatalf("publish branched package: %v", err)
	}
	published.Body.Close()
	studentToken := simulasiLogin(t, app, "branching-student")
	started, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+packageID+"/mulai", studentToken, nil, "")
	if err != nil || started.StatusCode != http.StatusCreated {
		if started != nil {
			started.Body.Close()
		}
		t.Fatalf("start branched package: response=%v err=%v", started, err)
	}
	var attempt map[string]any
	if err := json.NewDecoder(started.Body).Decode(&attempt); err != nil {
		started.Body.Close()
		t.Fatal(err)
	}
	started.Body.Close()
	attemptID := attempt["id"].(string)
	type learnerQuestion struct {
		UpayaSoalID string `json:"upayaSoalId"`
		BagianID    string `json:"bagianId"`
		Soal        struct {
			Pertanyaan string `json:"pertanyaan"`
		} `json:"soal"`
	}
	readWorkspace := func() []learnerQuestion {
		t.Helper()
		response, err := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID, studentToken, nil, "")
		if err != nil || response.StatusCode != http.StatusOK {
			if response != nil {
				response.Body.Close()
			}
			t.Fatalf("read learner branch path: response=%v err=%v", response, err)
		}
		var body struct {
			Soal []learnerQuestion `json:"soal"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		return body.Soal
	}
	initial := readWorkspace()
	if len(initial) != 1 || initial[0].BagianID != "awal" || strings.Contains(initial[0].Soal.Pertanyaan, "dilewati") {
		t.Fatalf("before choosing a route, only the first section should be returned: %+v", initial)
	}
	var skippedItem SimulasiPaketSoal
	if err := s.db.Where("paket_id = ? AND bagian_id = ?", packageID, "dilewati").First(&skippedItem).Error; err != nil {
		t.Fatal(err)
	}
	var skippedLink SimulasiUpayaSoal
	if err := s.db.Where("upaya_id = ? AND paket_soal_id = ?", attemptID, skippedItem.ID).First(&skippedLink).Error; err != nil {
		t.Fatal(err)
	}
	response, err := makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attemptID+"/jawaban/"+initial[0].UpayaSoalID, studentToken, map[string]string{"jawaban": "lanjut"}, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("save branch answer: response=%v err=%v", response, err)
	}
	response.Body.Close()
	path := readWorkspace()
	if len(path) != 2 || path[0].BagianID != "awal" || path[1].BagianID != "lanjutan" || path[1].Soal.Pertanyaan != "Pertanyaan lanjutan" {
		t.Fatalf("learner path did not follow the selected route: %+v", path)
	}
	blocked, err := makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attemptID+"/jawaban/"+skippedLink.ID, studentToken, map[string]string{"jawaban": "x"}, "")
	if err != nil || blocked.StatusCode != http.StatusNotFound {
		if blocked != nil {
			blocked.Body.Close()
		}
		t.Fatalf("skipped question must be rejected server-side: response=%v err=%v", blocked, err)
	}
	blocked.Body.Close()
	response, err = makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attemptID+"/jawaban/"+path[1].UpayaSoalID, studentToken, map[string]string{"jawaban": "x"}, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("save target section answer: response=%v err=%v", response, err)
	}
	response.Body.Close()
	submitted, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attemptID+"/kirim", studentToken, nil, "")
	if err != nil || submitted.StatusCode != http.StatusOK {
		if submitted != nil {
			body, _ := io.ReadAll(submitted.Body)
			submitted.Body.Close()
			t.Fatalf("submit branched attempt: status=%d body=%s err=%v", submitted.StatusCode, body, err)
		}
		t.Fatalf("submit branched attempt: %v", err)
	}
	submitted.Body.Close()
	var completed SimulasiUpaya
	if err := s.db.First(&completed, "id = ?", attemptID).Error; err != nil {
		t.Fatal(err)
	}
	if completed.SkorOtomatis != 100 {
		t.Fatalf("score should use only the selected path (two correct answers), got %v", completed.SkorOtomatis)
	}
	if err := s.db.First(&skippedLink, "id = ?", skippedLink.ID).Error; err != nil || skippedLink.Aktif {
		t.Fatalf("skipped section should be recorded inactive: row=%+v err=%v", skippedLink, err)
	}
}

func TestSimulasiSectionShuffleKeepsSectionOrderAndSeed(t *testing.T) {
	sections := []SimulasiBagian{{ClientID: "one", Urutan: 1}, {ClientID: "two", Urutan: 2}}
	items := []SimulasiPaketSoal{{Base: Base{ID: "q3"}, BagianID: "two", Urutan: 1}, {Base: Base{ID: "q2"}, BagianID: "one", Urutan: 3}, {Base: Base{ID: "q1"}, BagianID: "one", Urutan: 2}}
	first := orderSimulasiPackageItems(items, sections, "stable-seed", true)
	second := orderSimulasiPackageItems(items, sections, "stable-seed", true)
	if len(first) != 3 || first[0].BagianID != "one" || first[1].BagianID != "one" || first[2].BagianID != "two" {
		t.Fatalf("shuffle crossed a section boundary: %+v", first)
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("same seed produced different order: %+v vs %+v", first, second)
		}
	}
}

func TestSimulasiBuilderStimulusCardsPersistAndQuestionArchive(t *testing.T) {
	_, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	question := map[string]any{
		"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": "Pertanyaan stimulus", "bobot": 1,
		"konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "A"}, {"id": "b", "text": "B"}}, "correctIds": []string{"a"}},
		"stimulus":    []map[string]any{{"jenis": "text", "konten": "", "urutan": 1}},
	}
	created, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, map[string]any{
		"paket": map[string]any{"nama": "Draf stimulus", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items": []map[string]any{{"bobot": 1, "soal": question}}, "pesertaDidikIds": []string{},
	}, "")
	if err != nil || created.StatusCode != http.StatusOK {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("create stimulus draft: status=%v err=%v", created, err)
	}
	var body struct {
		Paket SimulasiPaket `json:"paket"`
		Items []struct {
			SoalID string `json:"soalId"`
			Soal   struct {
				Stimulus []SimulasiStimulus `json:"stimulus"`
			} `json:"soal"`
		}
	}
	if err := json.NewDecoder(created.Body).Decode(&body); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()
	if len(body.Items) != 1 || body.Items[0].SoalID == "" || len(body.Items[0].Soal.Stimulus) != 1 {
		t.Fatalf("empty stimulus card was not persisted: %+v", body.Items)
	}
	questionID := body.Items[0].SoalID

	question["stimulus"] = []map[string]any{{"jenis": "table", "konten": "Nama\tNilai", "urutan": 1}, {"jenis": "media_link", "konten": "https://example.com/media", "urutan": 2}}
	updated, err := makeRequest(app, http.MethodPut, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, map[string]any{
		"revision": body.Paket.Revision, "paket": map[string]any{"nama": "Draf stimulus", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items": []map[string]any{{"soalId": questionID, "bobot": 1, "soal": question}}, "pesertaDidikIds": []string{},
	}, "")
	if err != nil || updated.StatusCode != http.StatusOK {
		if updated != nil {
			updated.Body.Close()
		}
		t.Fatalf("update stimulus draft: status=%v err=%v", updated, err)
	}
	var updatedBody struct {
		Items []struct {
			Soal struct {
				Stimulus []SimulasiStimulus `json:"stimulus"`
			} `json:"soal"`
		}
	}
	if err := json.NewDecoder(updated.Body).Decode(&updatedBody); err != nil {
		updated.Body.Close()
		t.Fatal(err)
	}
	updated.Body.Close()
	if len(updatedBody.Items) != 1 || len(updatedBody.Items[0].Soal.Stimulus) != 2 {
		t.Fatalf("text/table/media stimulus was not saved: %+v", updatedBody)
	}

	archived, err := makeRequest(app, http.MethodDelete, "/api/simulasi/soal/"+questionID, adminToken, nil, "")
	if err != nil || archived.StatusCode != http.StatusConflict {
		if archived != nil {
			archived.Body.Close()
		}
		t.Fatalf("question referenced by draft should be protected: status=%v err=%v", archived, err)
	}
	archived.Body.Close()

	// Remove the item from the draft first, then archive the source question.
	removed, err := makeRequest(app, http.MethodPut, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, map[string]any{
		"revision": body.Paket.Revision + 1, "paket": map[string]any{"nama": "Draf stimulus", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items": []any{}, "pesertaDidikIds": []string{},
	}, "")
	if err != nil || removed.StatusCode != http.StatusOK {
		if removed != nil {
			removed.Body.Close()
		}
		t.Fatalf("remove question from draft: status=%v err=%v", removed, err)
	}
	removed.Body.Close()
	archived, err = makeRequest(app, http.MethodDelete, "/api/simulasi/soal/"+questionID, adminToken, nil, "")
	if err != nil || archived.StatusCode != http.StatusNoContent {
		if archived != nil {
			archived.Body.Close()
		}
		t.Fatalf("archive question after removal: status=%v err=%v", archived, err)
	}
	archived.Body.Close()
}

func TestSimulasiBuilderRevisionConflict(t *testing.T) {
	_, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	payload := map[string]any{
		"paket":           map[string]any{"nama": "Draf konflik", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items":           []any{},
		"pesertaDidikIds": []string{},
		"revision":        0,
	}
	created, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, payload, "")
	if err != nil || created.StatusCode != http.StatusOK {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("create conflict draft: %v", err)
	}
	var body struct {
		Paket SimulasiPaket `json:"paket"`
	}
	if err := json.NewDecoder(created.Body).Decode(&body); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()
	if body.Paket.Revision != 1 {
		t.Fatalf("new builder revision = %d, want 1", body.Paket.Revision)
	}
	update := map[string]any{
		"paket":           map[string]any{"nama": "Draf konflik versi satu", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items":           []any{},
		"pesertaDidikIds": []string{},
		"revision":        body.Paket.Revision,
	}
	first, err := makeRequest(app, http.MethodPut, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, update, "")
	if err != nil || first.StatusCode != http.StatusOK {
		if first != nil {
			first.Body.Close()
		}
		t.Fatalf("first builder update: %v", err)
	}
	first.Body.Close()
	second, err := makeRequest(app, http.MethodPut, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, update, "")
	if err != nil || second.StatusCode != http.StatusConflict {
		if second != nil {
			second.Body.Close()
		}
		t.Fatalf("stale builder update status = %v, want 409", second)
	}
	second.Body.Close()
	missingRevision, err := makeRequest(app, http.MethodPut, "/api/simulasi/paket/"+body.Paket.ID+"/builder", adminToken, map[string]any{
		"paket":           map[string]any{"nama": "Tanpa revision", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1},
		"items":           []any{},
		"pesertaDidikIds": []string{},
	}, "")
	if err != nil || missingRevision.StatusCode != http.StatusConflict {
		if missingRevision != nil {
			missingRevision.Body.Close()
		}
		t.Fatalf("missing builder revision status = %v, want 409", missingRevision)
	}
	missingRevision.Body.Close()
}

func TestSimulasiShareTokenPreviewAndRevoke(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "share-student")
	_, _ = simulasiStudent(t, s, "share-student-unassigned")

	created, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/builder", adminToken, map[string]any{
		"paket": map[string]any{"nama": "Paket tautan aman", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 20, "maksPercobaan": 1, "tampilkanNilai": true},
		"items": []map[string]any{{"bobot": 1, "soal": map[string]any{
			"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": "Kunci rahasia?", "bobot": 1,
			"konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "Benar"}, {"id": "b", "text": "Salah"}}, "correctIds": []string{"a"}},
		}}},
		"pesertaDidikIds": []string{student.ID},
	}, "")
	if err != nil || created.StatusCode != http.StatusOK {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("create shareable builder: status=%v err=%v", created, err)
	}
	var draft struct {
		Paket SimulasiPaket `json:"paket"`
	}
	if err := json.NewDecoder(created.Body).Decode(&draft); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()

	published, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+draft.Paket.ID+"/publikasi", adminToken, nil, "")
	if err != nil || published.StatusCode != http.StatusOK {
		if published != nil {
			published.Body.Close()
		}
		t.Fatalf("publish shareable package: status=%v err=%v", published, err)
	}
	published.Body.Close()

	createdToken, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token", adminToken, map[string]any{"label": "Kelas 6A"}, "")
	if err != nil || createdToken.StatusCode != http.StatusCreated {
		if createdToken != nil {
			createdToken.Body.Close()
		}
		t.Fatalf("create share token: status=%v err=%v", createdToken, err)
	}
	var share struct {
		ID     string `json:"id"`
		Token  string `json:"token"`
		URL    string `json:"url"`
		Prefix string `json:"tokenPrefix"`
	}
	if err := json.NewDecoder(createdToken.Body).Decode(&share); err != nil {
		createdToken.Body.Close()
		t.Fatal(err)
	}
	createdToken.Body.Close()
	if share.ID == "" || share.Token == "" || share.URL == "" || share.Prefix == "" {
		t.Fatalf("share response missing one-time credential fields: %+v", share)
	}
	publicApp := fiber.New(fiber.Config{ErrorHandler: apiError})
	publicApp.Get("/api/simulasi/share/:token", s.simulasiSharedInfo)
	publicInfo, err := publicApp.Test(httptest.NewRequest(http.MethodGet, "/api/simulasi/share/"+share.Token, nil))
	if err != nil || publicInfo.StatusCode != http.StatusOK {
		if publicInfo != nil {
			publicInfo.Body.Close()
		}
		t.Fatalf("public shared info: status=%v err=%v", publicInfo, err)
	}
	publicBody, _ := io.ReadAll(publicInfo.Body)
	publicInfo.Body.Close()
	if !strings.Contains(string(publicBody), `"requiresLogin":true`) || strings.Contains(string(publicBody), "correctIds") {
		t.Fatalf("public shared info leaked answer data or omitted login gate: %s", publicBody)
	}

	// The list endpoint must never return the raw credential again.
	listed, err := makeRequest(app, http.MethodGet, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token", adminToken, nil, "")
	if err != nil || listed.StatusCode != http.StatusOK {
		if listed != nil {
			listed.Body.Close()
		}
		t.Fatalf("list share tokens: status=%v err=%v", listed, err)
	}
	listedBody, _ := io.ReadAll(listed.Body)
	listed.Body.Close()
	if strings.Contains(string(listedBody), share.Token) || !strings.Contains(string(listedBody), share.Prefix) {
		t.Fatalf("share list leaked raw token or omitted prefix: %s", listedBody)
	}

	// A student may use the shared package without a pre-created assignment,
	// but still needs a normal student login.
	studentToken := simulasiLogin(t, app, "share-student-unassigned")
	studentRequest := httptest.NewRequest(http.MethodGet, "/api/simulasi/saya", nil)
	studentRequest.Header.Set("Authorization", "Bearer "+studentToken)
	studentRequest.Header.Set("X-Simulasi-Share-Token", share.Token)
	studentResponse, err := app.Test(studentRequest)
	if err != nil || studentResponse.StatusCode != http.StatusOK {
		if studentResponse != nil {
			studentResponse.Body.Close()
		}
		t.Fatalf("student shared list: status=%v err=%v", studentResponse, err)
	}
	studentBody, _ := io.ReadAll(studentResponse.Body)
	studentResponse.Body.Close()
	if !strings.Contains(string(studentBody), draft.Paket.ID) || strings.Contains(string(studentBody), "correctIds") || strings.Contains(string(studentBody), "pembahasan") {
		t.Fatalf("student shared payload is wrong or leaks answer data: %s", studentBody)
	}

	var packageItem SimulasiPaketSoal
	if err := s.db.Where("paket_id = ?", draft.Paket.ID).First(&packageItem).Error; err != nil {
		t.Fatal(err)
	}
	invalidEmbedOrigin, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token", adminToken, map[string]any{"embedOrigins": []string{"https://school.example; script-src *"}}, "")
	if err != nil || invalidEmbedOrigin.StatusCode != http.StatusBadRequest {
		if invalidEmbedOrigin != nil {
			invalidEmbedOrigin.Body.Close()
		}
		t.Fatalf("CSP-injecting embed origin should be rejected: status=%v err=%v", invalidEmbedOrigin, err)
	}
	invalidEmbedOrigin.Body.Close()
	invalidPrefill, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token", adminToken, map[string]any{"prefill": map[string]string{packageItem.ID: "not-a-choice"}}, "")
	if err != nil || invalidPrefill.StatusCode != http.StatusBadRequest {
		if invalidPrefill != nil {
			invalidPrefill.Body.Close()
		}
		t.Fatalf("invalid prefill answer should be rejected: status=%v err=%v", invalidPrefill, err)
	}
	invalidPrefill.Body.Close()

	prefillPayload, _ := json.Marshal(map[string]any{"label": "Isian awal", "prefill": map[string]string{packageItem.ID: "a"}, "embedOrigins": []string{"https://school.example"}})
	prefillRequest := httptest.NewRequest(http.MethodPost, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token", bytes.NewReader(prefillPayload))
	prefillRequest.Header.Set("Authorization", "Bearer "+adminToken)
	prefillRequest.Header.Set("Content-Type", "application/json")
	prefillResponse, err := app.Test(prefillRequest)
	if err != nil || prefillResponse.StatusCode != http.StatusCreated {
		if prefillResponse != nil {
			prefillResponse.Body.Close()
		}
		t.Fatalf("create prefilled share link: status=%v err=%v", prefillResponse, err)
	}
	var prefilledShare struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(prefillResponse.Body).Decode(&prefilledShare); err != nil {
		prefillResponse.Body.Close()
		t.Fatal(err)
	}
	prefillResponse.Body.Close()
	parsedPrefillURL, err := url.Parse(prefilledShare.URL)
	if err != nil || parsedPrefillURL.Query().Get("share") == "" || len(parsedPrefillURL.Query()) != 1 || strings.Contains(prefilledShare.URL, "prefill") {
		t.Fatalf("prefill data or parameters must not be embedded in the URL: %s", prefilledShare.URL)
	}
	var storedPrefillToken SimulasiAksesToken
	if err := s.db.First(&storedPrefillToken, "id = ?", prefilledShare.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storedPrefillToken.PrefillJSON, packageItem.ID) || !strings.Contains(storedPrefillToken.PrefillJSON, `"a"`) {
		t.Fatalf("prefill was not stored on the protected token record: %s", storedPrefillToken.PrefillJSON)
	}
	if storedPrefillToken.EmbedOriginsJSON != `["https://school.example"]` {
		t.Fatalf("expected a validated exact embed origin, got %s", storedPrefillToken.EmbedOriginsJSON)
	}
	if ancestors := s.simulasiFrameAncestors(parsedPrefillURL.Query().Get("share")); ancestors != "https://school.example" {
		t.Fatalf("valid embedded share token should allow its configured origin only, got %q", ancestors)
	}
	if ancestors := s.simulasiFrameAncestors(share.Token); ancestors != "'none'" {
		t.Fatalf("ordinary share token must not enable framing, got %q", ancestors)
	}
	studentStartRequest := httptest.NewRequest(http.MethodPost, "/api/simulasi/saya/paket/"+draft.Paket.ID+"/mulai", nil)
	studentStartRequest.Header.Set("Authorization", "Bearer "+studentToken)
	studentStartRequest.Header.Set("X-Simulasi-Share-Token", parsedPrefillURL.Query().Get("share"))
	studentStartResponse, err := app.Test(studentStartRequest)
	if err != nil || studentStartResponse.StatusCode != http.StatusCreated {
		if studentStartResponse != nil {
			studentStartResponse.Body.Close()
		}
		t.Fatalf("start attempt from prefilled link: status=%v err=%v", studentStartResponse, err)
	}
	var startedAttempt map[string]any
	if err := json.NewDecoder(studentStartResponse.Body).Decode(&startedAttempt); err != nil {
		studentStartResponse.Body.Close()
		t.Fatal(err)
	}
	studentStartResponse.Body.Close()
	workspaceRequest := httptest.NewRequest(http.MethodGet, "/api/simulasi/saya/upaya/"+startedAttempt["id"].(string), nil)
	workspaceRequest.Header.Set("Authorization", "Bearer "+studentToken)
	workspaceRequest.Header.Set("X-Simulasi-Share-Token", parsedPrefillURL.Query().Get("share"))
	workspaceResponse, err := app.Test(workspaceRequest)
	if err != nil || workspaceResponse.StatusCode != http.StatusOK {
		if workspaceResponse != nil {
			workspaceResponse.Body.Close()
		}
		t.Fatalf("load prefilled student attempt: status=%v err=%v", workspaceResponse, err)
	}
	workspaceBody, _ := io.ReadAll(workspaceResponse.Body)
	workspaceResponse.Body.Close()
	if !strings.Contains(string(workspaceBody), `"jawaban":"a"`) || strings.Contains(string(workspaceBody), "correctIds") {
		t.Fatalf("student attempt should contain the prefilled response without answer keys: %s", workspaceBody)
	}

	revoked, err := makeRequest(app, http.MethodDelete, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token/"+share.ID, adminToken, nil, "")
	if err != nil || revoked.StatusCode != http.StatusOK {
		if revoked != nil {
			revoked.Body.Close()
		}
		t.Fatalf("revoke share token: status=%v err=%v", revoked, err)
	}
	revoked.Body.Close()
	_, invalid, err := s.findValidSimulasiShareToken(share.Token)
	if err == nil || invalid != nil {
		t.Fatalf("revoked token remained valid: package=%v err=%v", invalid, err)
	}
	revokedPrefill, err := makeRequest(app, http.MethodDelete, "/api/simulasi/paket/"+draft.Paket.ID+"/share-token/"+prefilledShare.ID, adminToken, nil, "")
	if err != nil || revokedPrefill.StatusCode != http.StatusOK {
		if revokedPrefill != nil {
			revokedPrefill.Body.Close()
		}
		t.Fatalf("revoke prefilled share token: status=%v err=%v", revokedPrefill, err)
	}
	revokedPrefill.Body.Close()
	if ancestors := s.simulasiFrameAncestors(parsedPrefillURL.Query().Get("share")); ancestors != "'none'" {
		t.Fatalf("revoked token must no longer permit framing, got %q", ancestors)
	}
}

func TestSimulasiEmbedOriginsAreExactAndRestrictive(t *testing.T) {
	origins, err := normalizeSimulasiEmbedOrigins([]string{"HTTPS://School.Example", "https://school.example", "https://kelas.example:8443"})
	if err != nil || len(origins) != 2 || origins[0] != "https://school.example" || origins[1] != "https://kelas.example:8443" {
		t.Fatalf("origins should normalize and deduplicate: %v err=%v", origins, err)
	}
	for _, invalid := range []string{"https://school.example/path", "http://school.example", "https://school.example; script-src *", "https://user@school.example", "https://school.example?x=1"} {
		if _, err := normalizeSimulasiEmbedOrigins([]string{invalid}); err == nil {
			t.Errorf("unsafe/non-origin embed URL was accepted: %q", invalid)
		}
	}
	if policy := productionContentSecurityPolicy("test-nonce"); !strings.Contains(policy, "frame-ancestors 'none'") {
		t.Fatalf("default pages must remain unframeable: %s", policy)
	}
	policy := productionContentSecurityPolicyWithFrameAncestors("test-nonce", strings.Join(origins, " "))
	if !strings.Contains(policy, "frame-ancestors https://school.example https://kelas.example:8443") {
		t.Fatalf("embed policy should allow only its explicit origins: %s", policy)
	}
}

func TestSimulasiWorkspaceBahanLegacyCopyAndRevision(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, adminID := getAdminToken(t, app)
	legacy := BankSoal{MapelID: "", Tipe: "pg", Pertanyaan: "Berapakah 3 + 3?", Opsi: `["5","6"]`, Kunci: "1", Poin: 2, DibuatOlehUserID: adminID, Domain: "Numerasi", Topik: "Penjumlahan", Kompetensi: "Menjumlahkan bilangan", LevelKognitif: "Menerapkan"}
	if err := s.db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	copied, err := makeRequest(app, http.MethodPost, "/api/simulasi/soal/from-bank/"+legacy.ID, adminToken, nil, "")
	if err != nil || copied.StatusCode != http.StatusCreated {
		if copied != nil {
			copied.Body.Close()
		}
		t.Fatalf("legacy copy: %v", err)
	}
	var copiedBody map[string]any
	if err := json.NewDecoder(copied.Body).Decode(&copiedBody); err != nil {
		copied.Body.Close()
		t.Fatal(err)
	}
	copied.Body.Close()
	if copiedBody["legacySourceId"] != legacy.ID || copiedBody["status"] != "draf" || copiedBody["domain"] != "Numerasi" || copiedBody["topik"] != "Penjumlahan" || copiedBody["kompetensi"] != "Menjumlahkan bilangan" || copiedBody["levelKognitif"] != "Menerapkan" {
		t.Fatalf("unexpected copied question: %#v", copiedBody)
	}

	bahan, err := makeRequest(app, http.MethodPost, "/api/simulasi/bahan", adminToken, map[string]any{"judul": "Bacaan energi", "jenis": "text", "konten": "Energi membantu benda bergerak.", "status": "draf"}, "")
	if err != nil || bahan.StatusCode != http.StatusCreated {
		if bahan != nil {
			bahan.Body.Close()
		}
		t.Fatalf("create bahan: %v", err)
	}
	var bahanBody SimulasiBahan
	if err := json.NewDecoder(bahan.Body).Decode(&bahanBody); err != nil {
		bahan.Body.Close()
		t.Fatal(err)
	}
	bahan.Body.Close()
	if bahanBody.Revision != 1 {
		t.Fatalf("initial bahan revision = %d", bahanBody.Revision)
	}
	stale, _ := makeRequest(app, http.MethodPut, "/api/simulasi/bahan/"+bahanBody.ID, adminToken, map[string]any{"judul": "Usang", "jenis": "text", "konten": "Versi lama", "revision": 1}, "")
	if stale.StatusCode != http.StatusOK {
		stale.Body.Close()
		t.Fatalf("first revision update got %d", stale.StatusCode)
	}
	var updated SimulasiBahan
	_ = json.NewDecoder(stale.Body).Decode(&updated)
	stale.Body.Close()
	second, _ := makeRequest(app, http.MethodPut, "/api/simulasi/bahan/"+bahanBody.ID, adminToken, map[string]any{"judul": "Stale", "jenis": "text", "konten": "Konflik", "revision": 1}, "")
	if second.StatusCode != http.StatusConflict {
		second.Body.Close()
		t.Fatalf("stale revision got %d", second.StatusCode)
	}
	second.Body.Close()
}

func TestSimulasiStudentWorkflowProtectsKeysAndIsIdempotent(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "sim-student")
	_, _ = simulasiStudent(t, s, "other-student")
	studentToken := simulasiLogin(t, app, "sim-student")
	otherToken := simulasiLogin(t, app, "other-student")

	questionBody := map[string]any{
		"jenjang": "SD/MI", "mode": "anbk_akm", "tipe": simulasiTipePG, "pertanyaan": "Berapakah 2 + 2?", "bobot": 1,
		"status": "terbit", "konfigurasi": map[string]any{"choices": []map[string]string{{"id": "a", "text": "3"}, {"id": "b", "text": "4"}}, "correctIds": []string{"b"}},
	}
	createdQuestion, err := makeRequest(app, http.MethodPost, "/api/simulasi/soal", adminToken, questionBody, "")
	if err != nil || createdQuestion.StatusCode != http.StatusCreated {
		if createdQuestion != nil {
			createdQuestion.Body.Close()
		}
		t.Fatalf("create question: %v", err)
	}
	var question map[string]any
	_ = json.NewDecoder(createdQuestion.Body).Decode(&question)
	createdQuestion.Body.Close()
	questionID := question["id"].(string)

	createdPaket, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket", adminToken, map[string]any{"nama": "Paket workflow", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1, "acakUrutan": true, "tampilkanNilai": true, "temaWarna": "#166534", "pesanKonfirmasi": "Jawaban diterima dengan aman."}, "")
	if err != nil || createdPaket.StatusCode != http.StatusCreated {
		if createdPaket != nil {
			createdPaket.Body.Close()
		}
		t.Fatalf("create packet: %v", err)
	}
	var paket map[string]any
	_ = json.NewDecoder(createdPaket.Body).Decode(&paket)
	createdPaket.Body.Close()
	paketID := paket["id"].(string)
	for _, requestData := range []struct {
		method, url string
		body        any
	}{
		{http.MethodPost, "/api/simulasi/paket/" + paketID + "/soal", map[string]any{"soalId": questionID}},
		{http.MethodPut, "/api/simulasi/paket/" + paketID + "/penugasan", map[string]any{"pesertaDidikIds": []string{student.ID}}},
		{http.MethodPost, "/api/simulasi/paket/" + paketID + "/publikasi", nil},
	} {
		response, e := makeRequest(app, requestData.method, requestData.url, adminToken, requestData.body, "")
		if e != nil || response.StatusCode >= 300 {
			if response != nil {
				response.Body.Close()
			}
			t.Fatalf("packet setup %s: %v", requestData.url, e)
		}
		response.Body.Close()
	}
	instruction, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya/paket/"+paketID+"/instruksi", studentToken, nil, "")
	if instruction.StatusCode != http.StatusOK {
		instruction.Body.Close()
		t.Fatalf("student instructions got %d", instruction.StatusCode)
	}
	var instructionPayload map[string]any
	_ = json.NewDecoder(instruction.Body).Decode(&instructionPayload)
	instruction.Body.Close()
	studentPacket, _ := instructionPayload["paket"].(map[string]any)
	if studentPacket["temaWarna"] != "#166534" {
		t.Fatalf("student package theme missing/incorrect: %#v", studentPacket)
	}

	// The frozen snapshot must outlive a source edit after publication.
	if err := s.db.Model(&SimulasiSoal{}).Where("id = ?", questionID).Update("pertanyaan", "SOURCE MUST NOT LEAK").Error; err != nil {
		t.Fatal(err)
	}
	denied, _ := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+paketID+"/mulai", otherToken, nil, "")
	if denied.StatusCode != http.StatusForbidden {
		denied.Body.Close()
		t.Fatalf("unassigned student got %d", denied.StatusCode)
	}
	denied.Body.Close()

	started, err := makeRequest(app, http.MethodPost, "/api/simulasi/saya/paket/"+paketID+"/mulai", studentToken, nil, "")
	if err != nil || started.StatusCode != http.StatusCreated {
		if started != nil {
			started.Body.Close()
		}
		t.Fatalf("start: %v", err)
	}
	var attempt map[string]any
	_ = json.NewDecoder(started.Body).Decode(&attempt)
	started.Body.Close()
	attemptID := attempt["id"].(string)
	listed, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya", studentToken, nil, "")
	var listedPackages []struct {
		Paket struct {
			ID string `json:"id"`
		} `json:"paket"`
		Tersedia bool `json:"tersedia"`
	}
	if listed.StatusCode != http.StatusOK {
		listed.Body.Close()
		t.Fatalf("student package list while attempt is ongoing got %d", listed.StatusCode)
	}
	_ = json.NewDecoder(listed.Body).Decode(&listedPackages)
	listed.Body.Close()
	if len(listedPackages) != 1 || listedPackages[0].Paket.ID != paketID || !listedPackages[0].Tersedia {
		t.Fatalf("ongoing attempt must remain available even at the try limit: %#v", listedPackages)
	}
	workspace, err := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID, studentToken, nil, "")
	if err != nil || workspace.StatusCode != http.StatusOK {
		if workspace != nil {
			workspace.Body.Close()
		}
		t.Fatalf("workspace: %v", err)
	}
	workspacePayload, readErr := io.ReadAll(workspace.Body)
	workspace.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(workspacePayload), "correctIds") || strings.Contains(string(workspacePayload), "acceptedAnswers") || strings.Contains(string(workspacePayload), "rubrik") || strings.Contains(string(workspacePayload), "seedUrutan") || strings.Contains(string(workspacePayload), "SOURCE MUST NOT LEAK") {
		t.Fatalf("student workspace leaked protected data: %s", workspacePayload)
	}
	var decoded struct {
		Soal []struct {
			UpayaSoalID string `json:"upayaSoalId"`
			Soal        struct {
				Pertanyaan string `json:"pertanyaan"`
				Wajib      bool   `json:"wajibDijawab"`
			} `json:"soal"`
		} `json:"soal"`
	}
	if err := json.Unmarshal(workspacePayload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Soal) != 1 || decoded.Soal[0].Soal.Pertanyaan != "Berapakah 2 + 2?" || !decoded.Soal[0].Soal.Wajib {
		t.Fatalf("snapshot not preserved: %#v", decoded.Soal)
	}
	missing, _ := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attemptID+"/kirim", studentToken, nil, "")
	if missing.StatusCode != http.StatusBadRequest {
		missing.Body.Close()
		t.Fatalf("required blank answer should be blocked, got %d", missing.StatusCode)
	}
	missing.Body.Close()

	saved, _ := makeRequest(app, http.MethodPut, "/api/simulasi/saya/upaya/"+attemptID+"/jawaban/"+decoded.Soal[0].UpayaSoalID, studentToken, map[string]any{"jawaban": "b"}, "")
	if saved.StatusCode != http.StatusOK {
		saved.Body.Close()
		t.Fatalf("autosave got %d", saved.StatusCode)
	}
	saved.Body.Close()
	for i := 0; i < 2; i++ {
		submitted, _ := makeRequest(app, http.MethodPost, "/api/simulasi/saya/upaya/"+attemptID+"/kirim", studentToken, nil, "")
		if submitted.StatusCode != http.StatusOK {
			submitted.Body.Close()
			t.Fatalf("submit #%d got %d", i+1, submitted.StatusCode)
		}
		submitted.Body.Close()
	}
	var count int64
	s.db.Model(&SimulasiUpaya{}).Where("paket_id = ? AND peserta_didik_id = ?", paketID, student.ID).Count(&count)
	if count != 1 {
		t.Fatalf("attempt must be idempotent, got %d", count)
	}
	result, _ := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attemptID+"/hasil", studentToken, nil, "")
	if result.StatusCode != http.StatusOK {
		result.Body.Close()
		t.Fatalf("result got %d", result.StatusCode)
	}
	var outcome map[string]any
	_ = json.NewDecoder(result.Body).Decode(&outcome)
	result.Body.Close()
	if outcome["skor"] != float64(100) {
		t.Fatalf("automatic score = %#v", outcome["skor"])
	}
	if outcome["pesanKonfirmasi"] != "Jawaban diterima dengan aman." || outcome["temaWarna"] != "#166534" {
		t.Fatalf("student result settings were not preserved: %#v", outcome)
	}
	listed, _ = makeRequest(app, http.MethodGet, "/api/simulasi/saya", studentToken, nil, "")
	listedPackages = nil
	_ = json.NewDecoder(listed.Body).Decode(&listedPackages)
	listed.Body.Close()
	if len(listedPackages) != 1 || listedPackages[0].Tersedia {
		t.Fatalf("completed attempt at the try limit must not be startable again: %#v", listedPackages)
	}
}

func TestSimulasiManualEssayGradeFinalizesAttempt(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "essay-student")
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	config := simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Ketepatan alasan", Maks: 2}}}
	payload, _ := json.Marshal(config)
	question := SimulasiSoal{Jenjang: "SD/MI", Mode: "anbk_akm", Tipe: simulasiTipeUraian, Pertanyaan: "Jelaskan alasanmu.", Konfigurasi: string(payload), Bobot: 2, Status: "terbit", DibuatOlehUserID: admin.ID}
	snapshot, err := snapshotFromQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	packet := SimulasiPaket{Nama: "Paket Uraian", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 15, MaksPercobaan: 1, Status: "terbit", DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	item := SimulasiPaketSoal{PaketID: packet.ID, Urutan: 1, Bobot: 2, SnapshotJSON: snapshot}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	attempt := SimulasiUpaya{PaketID: packet.ID, PesertaDidikID: student.ID, Nomor: 1, Status: "menunggu_nilai", SeedUrutan: "seed"}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	link := SimulasiUpayaSoal{UpayaID: attempt.ID, PaketSoalID: item.ID, UrutanTampil: 1}
	if err := s.db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	answer := SimulasiJawaban{UpayaSoalID: link.ID, JawabanJSON: `"Karena ada bukti pada teks."`}
	if err := s.db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}
	detail, _ := makeRequest(app, http.MethodGet, "/api/simulasi/upaya/"+attempt.ID+"/detail", adminToken, nil, "")
	if detail.StatusCode != http.StatusOK {
		detail.Body.Close()
		t.Fatalf("staff detail got %d", detail.StatusCode)
	}
	detail.Body.Close()
	graded, _ := makeRequest(app, http.MethodPost, "/api/simulasi/upaya/"+attempt.ID+"/jawaban/"+answer.ID+"/nilai", adminToken, map[string]any{"skor": 2, "komentar": "Alasan tepat."}, "")
	if graded.StatusCode != http.StatusOK {
		graded.Body.Close()
		t.Fatalf("manual grade got %d", graded.StatusCode)
	}
	graded.Body.Close()
	if err := s.db.First(&attempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Status != "selesai" || attempt.SkorAkhir == nil || *attempt.SkorAkhir != 100 {
		t.Fatalf("manual grading did not finalize attempt: %#v", attempt)
	}
}

func TestSimulasiSubmittedResponseRevisionIsAuditedIdempotentAndRegraded(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "response-revision-student")
	studentToken := simulasiLogin(t, app, "response-revision-student")
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}

	correct := true
	initialScore := 1.0
	question := SimulasiSoal{
		Jenjang: "SD/MI", Mode: "anbk_akm", Tipe: simulasiTipePG,
		Pertanyaan: "Pilih jawaban yang benar.", Bobot: 1, Status: "terbit", DibuatOlehUserID: admin.ID,
	}
	questionConfig, _ := json.Marshal(simulasiConfig{
		Choices:    []simulasiChoice{{ID: "a", Text: "Benar"}, {ID: "b", Text: "Salah"}},
		CorrectIDs: []string{"a"},
	})
	question.Konfigurasi = string(questionConfig)
	snapshot, err := snapshotFromQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	packet := SimulasiPaket{
		Nama: "Paket revisi respons", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 20,
		MaksPercobaan: 1, Status: "terbit", TampilkanNilai: true, IzinkanEditRespons: true,
		DibuatOlehUserID: admin.ID,
	}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	item := SimulasiPaketSoal{PaketID: packet.ID, Urutan: 1, Bobot: 1, SnapshotJSON: snapshot}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiPenugasan{PaketID: packet.ID, PesertaDidikID: student.ID}).Error; err != nil {
		t.Fatal(err)
	}
	attempt := SimulasiUpaya{PaketID: packet.ID, PesertaDidikID: student.ID, Nomor: 1, Status: "selesai", SeedUrutan: "revision-seed", SkorOtomatis: 100, SkorAkhir: &initialScore}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	link := SimulasiUpayaSoal{UpayaID: attempt.ID, PaketSoalID: item.ID, UrutanTampil: 1, Aktif: true}
	if err := s.db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	answer := SimulasiJawaban{UpayaSoalID: link.ID, JawabanJSON: `"a"`, Benar: &correct, SkorOtomatis: 1, SkorAkhir: 1}
	if err := s.db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}

	result, err := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attempt.ID+"/hasil", studentToken, nil, "")
	if err != nil || result.StatusCode != http.StatusOK {
		if result != nil {
			result.Body.Close()
		}
		t.Fatalf("load completed result: %v", err)
	}
	var resultBody map[string]any
	resultBytes, _ := io.ReadAll(result.Body)
	result.Body.Close()
	if err := json.Unmarshal(resultBytes, &resultBody); err != nil {
		t.Fatal(err)
	}
	if resultBody["bolehEditRespons"] != true || resultBody["upayaId"] != attempt.ID {
		t.Fatalf("result did not expose safe edit capability: %#v", resultBody)
	}
	if strings.Contains(string(resultBytes), "correctIds") || strings.Contains(string(resultBytes), "acceptedAnswers") {
		t.Fatalf("student result leaked grading keys: %s", resultBytes)
	}
	workspace, err := makeRequest(app, http.MethodGet, "/api/simulasi/saya/upaya/"+attempt.ID, studentToken, nil, "")
	if err != nil || workspace.StatusCode != http.StatusOK {
		if workspace != nil {
			workspace.Body.Close()
		}
		t.Fatalf("load revision workspace: %v", err)
	}
	workspace.Body.Close()

	key := "9e0cbeb2-9e9a-44e7-8a50-3ad4f8c7811d"
	path := "/api/simulasi/saya/upaya/" + attempt.ID + "/jawaban/" + link.ID + "/revisi"
	edit := map[string]any{"jawaban": "b", "idempotencyKey": key}
	response, err := makeRequest(app, http.MethodPut, path, studentToken, edit, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			t.Fatalf("save response revision: %v; %s", err, body)
		}
		t.Fatalf("save response revision: %v", err)
	}
	response.Body.Close()
	if err := s.db.First(&answer, "id = ?", answer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if answer.JawabanJSON != `"b"` || answer.SkorAkhir != 0 || answer.Benar == nil || *answer.Benar {
		t.Fatalf("edited answer was not automatically regraded: %#v", answer)
	}
	if err := s.db.First(&attempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Status != "selesai" || attempt.SkorAkhir == nil || *attempt.SkorAkhir != 0 {
		t.Fatalf("attempt score was not recalculated: %#v", attempt)
	}
	var revision SimulasiJawabanRevisi
	if err := s.db.Where("upaya_id = ? AND idempotency_key = ?", attempt.ID, key).First(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if revision.Nomor != 1 || revision.PesertaDidikID != student.ID || revision.AktorUserID == "" || revision.JawabanSebelumJSON != `"a"` || revision.JawabanSesudahJSON != `"b"` {
		t.Fatalf("immutable before/after record is incomplete: %#v", revision)
	}
	staffDetail, _ := makeRequest(app, http.MethodGet, "/api/simulasi/upaya/"+attempt.ID+"/detail", adminToken, nil, "")
	if staffDetail.StatusCode != http.StatusOK {
		staffDetail.Body.Close()
		t.Fatalf("staff could not inspect revision history: %d", staffDetail.StatusCode)
	}
	var detailBody struct {
		Items []struct {
			Revisions []map[string]any `json:"revisions"`
		} `json:"items"`
	}
	if err := json.NewDecoder(staffDetail.Body).Decode(&detailBody); err != nil {
		staffDetail.Body.Close()
		t.Fatal(err)
	}
	staffDetail.Body.Close()
	if len(detailBody.Items) != 1 || len(detailBody.Items[0].Revisions) != 1 || detailBody.Items[0].Revisions[0]["jawabanSebelum"] != "a" || detailBody.Items[0].Revisions[0]["jawabanSesudah"] != "b" {
		t.Fatalf("staff detail omitted response history: %#v", detailBody)
	}

	replay, _ := makeRequest(app, http.MethodPut, path, studentToken, edit, "")
	if replay.StatusCode != http.StatusOK {
		replay.Body.Close()
		t.Fatalf("idempotent retry got %d", replay.StatusCode)
	}
	var replayBody map[string]any
	_ = json.NewDecoder(replay.Body).Decode(&replayBody)
	replay.Body.Close()
	if replayBody["status"] != "sudah_tersimpan" {
		t.Fatalf("retry response was not identified: %#v", replayBody)
	}
	var revisionCount int64
	s.db.Model(&SimulasiJawabanRevisi{}).Where("upaya_id = ?", attempt.ID).Count(&revisionCount)
	if revisionCount != 1 {
		t.Fatalf("idempotent retry created %d revisions", revisionCount)
	}
	conflictingReplay, _ := makeRequest(app, http.MethodPut, path, studentToken, map[string]any{"jawaban": "a", "idempotencyKey": key}, "")
	if conflictingReplay.StatusCode != http.StatusConflict {
		conflictingReplay.Body.Close()
		t.Fatalf("reusing an idempotency key with a different answer got %d", conflictingReplay.StatusCode)
	}
	conflictingReplay.Body.Close()

	otherStudent, _ := simulasiStudent(t, s, "response-revision-other")
	otherToken := simulasiLogin(t, app, "response-revision-other")
	_ = otherStudent
	foreign, _ := makeRequest(app, http.MethodPut, path, otherToken, map[string]any{"jawaban": "a", "idempotencyKey": "0c69b4c9-7d8a-4a6d-a18b-4d8d4f8e5f40"}, "")
	if foreign.StatusCode != http.StatusNotFound {
		foreign.Body.Close()
		t.Fatalf("another learner could revise this attempt: %d", foreign.StatusCode)
	}
	foreign.Body.Close()

	packet.IzinkanEditRespons = false
	if err := s.db.Save(&packet).Error; err != nil {
		t.Fatal(err)
	}
	denied, _ := makeRequest(app, http.MethodPut, path, studentToken, map[string]any{"jawaban": "a", "idempotencyKey": "e790aa35-fdb8-4df4-9d70-0c72a93785f2"}, "")
	if denied.StatusCode != http.StatusConflict {
		denied.Body.Close()
		t.Fatalf("revision should be denied when package policy is disabled: %d", denied.StatusCode)
	}
	denied.Body.Close()
}

func TestSimulasiResponseRevisionResetsManualScoreAndHonorsWindow(t *testing.T) {
	s, app := setupE2EServer(t)
	_, _ = getAdminToken(t, app)
	student, _ := simulasiStudent(t, s, "response-revision-essay")
	studentToken := simulasiLogin(t, app, "response-revision-essay")
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	questionConfig, _ := json.Marshal(simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Alasan", Maks: 2}}})
	question := SimulasiSoal{Jenjang: "SD/MI", Mode: "anbk_akm", Tipe: simulasiTipeUraian, Pertanyaan: "Jelaskan jawabanmu.", Konfigurasi: string(questionConfig), Bobot: 2, Status: "terbit", DibuatOlehUserID: admin.ID}
	snapshot, err := snapshotFromQuestion(question)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	closedAt := now.Add(-time.Minute)
	packet := SimulasiPaket{Nama: "Paket revisi uraian", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 15, MaksPercobaan: 1, Status: "terbit", IzinkanEditRespons: true, WaktuSelesai: &closedAt, DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	item := SimulasiPaketSoal{PaketID: packet.ID, Urutan: 1, Bobot: 2, SnapshotJSON: snapshot}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiPenugasan{PaketID: packet.ID, PesertaDidikID: student.ID}).Error; err != nil {
		t.Fatal(err)
	}
	score := 2.0
	attempt := SimulasiUpaya{PaketID: packet.ID, PesertaDidikID: student.ID, Nomor: 1, Status: "selesai", SeedUrutan: "essay-revision-seed", SkorOtomatis: 100, SkorAkhir: &score}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	link := SimulasiUpayaSoal{UpayaID: attempt.ID, PaketSoalID: item.ID, UrutanTampil: 1, Aktif: true}
	if err := s.db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	graderID := admin.ID
	answer := SimulasiJawaban{UpayaSoalID: link.ID, JawabanJSON: `"Jawaban lama"`, SkorManual: &score, SkorAkhir: score, KomentarGuru: "Sudah dinilai", DinilaiOlehUserID: &graderID}
	if err := s.db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/simulasi/saya/upaya/" + attempt.ID + "/jawaban/" + link.ID + "/revisi"
	closed, _ := makeRequest(app, http.MethodPut, path, studentToken, map[string]any{"jawaban": "\"Jawaban baru\"", "idempotencyKey": "fbdddc9c-3a82-4ce8-8d9e-f6e69bb68684"}, "")
	if closed.StatusCode != http.StatusConflict {
		closed.Body.Close()
		t.Fatalf("expired package window should reject edits, got %d", closed.StatusCode)
	}
	closed.Body.Close()

	packet.WaktuSelesai = nil
	if err := s.db.Save(&packet).Error; err != nil {
		t.Fatal(err)
	}
	updated, _ := makeRequest(app, http.MethodPut, path, studentToken, map[string]any{"jawaban": "Jawaban baru", "idempotencyKey": "fbdddc9c-3a82-4ce8-8d9e-f6e69bb68685"}, "")
	if updated.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updated.Body)
		updated.Body.Close()
		t.Fatalf("valid essay response edit failed: %s", body)
	}
	updated.Body.Close()
	if err := s.db.First(&answer, "id = ?", answer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if answer.SkorManual != nil || answer.KomentarGuru != "" || answer.DinilaiPada != nil {
		t.Fatalf("editing a manually graded answer must clear the obsolete grade: %#v", answer)
	}
	if err := s.db.First(&attempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.Status != "menunggu_nilai" || attempt.SkorAkhir != nil {
		t.Fatalf("edited essay should return to pending manual grading: %#v", attempt)
	}
}
