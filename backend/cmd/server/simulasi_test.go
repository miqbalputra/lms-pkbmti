package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	if protected, err := json.Marshal(sanitizedConfig(simulasiTipeUraian, simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Rahasia", Maks: 2}}})); err != nil || strings.Contains(string(protected), "rubrik") || strings.Contains(string(protected), "Rahasia") {
		t.Fatalf("student uraian payload exposed rubric: %s (err=%v)", protected, err)
	}
	if protected := studentAttemptResponse(SimulasiUpaya{SkorOtomatis: 100, SeedUrutan: "secret-seed", Status: "selesai"}, SimulasiPaket{TampilkanNilai: false}); protected["skor"] != nil || protected["seedUrutan"] != nil {
		t.Fatalf("student attempt exposed hidden fields: %#v", protected)
	}
}

func makeMultipartUploadRequest(app *fiber.App, url, token, filename string, contents []byte) (*http.Response, error) {
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

	packetResponse, _ := makeRequest(app, http.MethodPost, "/api/simulasi/paket", adminToken, map[string]any{"nama": "Jawaban Berkas", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 15, "maksPercobaan": 1, "tampilkanNilai": true}, "")
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
	secondUpload.Body.Close()
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
}

func intPtr(value int) *int { return &value }

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
}

func TestSimulasiWorkspaceBahanLegacyCopyAndRevision(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, adminID := getAdminToken(t, app)
	legacy := BankSoal{MapelID: "", Tipe: "pg", Pertanyaan: "Berapakah 3 + 3?", Opsi: `["5","6"]`, Kunci: "1", Poin: 2, DibuatOlehUserID: adminID}
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
	if copiedBody["legacySourceId"] != legacy.ID || copiedBody["status"] != "draf" {
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

	createdPaket, err := makeRequest(app, http.MethodPost, "/api/simulasi/paket", adminToken, map[string]any{"nama": "Paket workflow", "mode": "anbk_akm", "jenjang": "SD/MI", "durasiMenit": 30, "maksPercobaan": 1, "acakUrutan": true, "tampilkanNilai": true}, "")
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
