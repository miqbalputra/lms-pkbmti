package main

import (
	"encoding/json"
	"io"
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
			} `json:"soal"`
		} `json:"soal"`
	}
	if err := json.Unmarshal(workspacePayload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Soal) != 1 || decoded.Soal[0].Soal.Pertanyaan != "Berapakah 2 + 2?" {
		t.Fatalf("snapshot not preserved: %#v", decoded.Soal)
	}

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
