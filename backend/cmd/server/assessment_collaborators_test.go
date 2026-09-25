package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func assessmentTestTeacher(t *testing.T, s *Server, username string) User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("Tutor1234"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	tutorID := uuid.NewString()
	user := User{Username: username, PasswordHash: string(hash), Role: "guru", IsActive: true, TutorID: &tutorID}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func assessmentTestTeacherToken(t *testing.T, app *fiber.App, username string) string {
	t.Helper()
	response, err := makeRequest(app, http.MethodPost, "/api/auth/login", "", map[string]string{"login": username, "password": "Tutor1234"}, "")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("teacher login %q failed: %v", username, err)
	}
	defer response.Body.Close()
	var body struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.AccessToken == "" {
		t.Fatalf("teacher token missing: %v", err)
	}
	return body.AccessToken
}

func TestAssessmentCollaboratorRolesAndScopeAcrossModules(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	editor := assessmentTestTeacher(t, s, "assessment-editor")
	viewer := assessmentTestTeacher(t, s, "assessment-viewer")
	grader := assessmentTestTeacher(t, s, "assessment-grader")
	outsider := assessmentTestTeacher(t, s, "assessment-outsider")
	editorToken := assessmentTestTeacherToken(t, app, editor.Username)
	viewerToken := assessmentTestTeacherToken(t, app, viewer.Username)
	graderToken := assessmentTestTeacherToken(t, app, grader.Username)
	outsiderToken := assessmentTestTeacherToken(t, app, outsider.Username)

	packet := SimulasiPaket{Nama: "Paket kolaboratif", Mode: "anbk_akm", Jenjang: "SD/MI", DurasiMenit: 20, MaksPercobaan: 1, Status: "draf", DibuatOlehUserID: admin.ID}
	exam := Ujian{Judul: "Ujian kolaboratif", KelasID: "kelas-lain", DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&packet).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	for _, resource := range []struct{ module, id string }{{assessmentModuleSimulasi, packet.ID}, {assessmentModuleUjian, exam.ID}} {
		for _, grant := range []struct{ username, role string }{{editor.Username, assessmentRoleEditor}, {viewer.Username, assessmentRoleViewer}, {grader.Username, assessmentRoleGrader}} {
			response, err := makeRequest(app, http.MethodPost, "/api/assessment/"+resource.module+"/"+resource.id+"/collaborators", adminToken, map[string]string{"username": grant.username, "peran": grant.role}, "")
			if err != nil || response.StatusCode != http.StatusCreated {
				if response != nil {
					response.Body.Close()
				}
				t.Fatalf("grant %s/%s to %s: status %v err %v", resource.module, resource.id, grant.username, response, err)
			}
			response.Body.Close()
		}
	}

	for _, tc := range []struct {
		name, module, id, token string
		canManage               bool
	}{
		{"editor", assessmentModuleSimulasi, packet.ID, editorToken, false},
		{"viewer", assessmentModuleSimulasi, packet.ID, viewerToken, false},
		{"grader", assessmentModuleUjian, exam.ID, graderToken, false},
	} {
		response, err := makeRequest(app, http.MethodGet, "/api/assessment/"+tc.module+"/"+tc.id+"/collaborators", tc.token, nil, "")
		if err != nil || response.StatusCode != http.StatusOK {
			if response != nil {
				response.Body.Close()
			}
			t.Fatalf("%s collaborator list got %v (%v)", tc.name, response, err)
		}
		var payload struct {
			CanManage bool `json:"canManage"`
		}
		_ = json.NewDecoder(response.Body).Decode(&payload)
		response.Body.Close()
		if payload.CanManage != tc.canManage {
			t.Fatalf("%s unexpectedly canManage collaborators: %#v", tc.name, payload)
		}
	}

	// Shared assessments become discoverable, but permission remains role-scoped.
	for _, tc := range []struct{ token, endpoint, resourceID string }{
		{editorToken, "/api/simulasi/paket", packet.ID},
		{editorToken, "/api/ujian", exam.ID},
		{viewerToken, "/api/simulasi/paket", packet.ID},
		{viewerToken, "/api/ujian", exam.ID},
	} {
		response, err := makeRequest(app, http.MethodGet, tc.endpoint, tc.token, nil, "")
		if err != nil || response.StatusCode != http.StatusOK {
			if response != nil {
				response.Body.Close()
			}
			t.Fatalf("shared list %s got %v (%v)", tc.endpoint, response, err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if !bytes.Contains(data, []byte(tc.resourceID)) {
			t.Fatalf("shared resource missing from %s: %s", tc.endpoint, data)
		}
	}

	// Editors can use write routes; graders/viewers are read-only except grading.
	allowedEdit, _ := makeRequest(app, http.MethodPut, "/api/ujian/"+exam.ID+"/bagian", editorToken, map[string]any{"bagian": []map[string]string{{"id": "awal", "nama": "Bagian awal"}}}, "")
	if allowedEdit.StatusCode != http.StatusOK {
		allowedEdit.Body.Close()
		t.Fatalf("editor Ujian write got %d", allowedEdit.StatusCode)
	}
	allowedEdit.Body.Close()
	for _, token := range []string{viewerToken, graderToken, outsiderToken} {
		blocked, _ := makeRequest(app, http.MethodPut, "/api/ujian/"+exam.ID+"/bagian", token, map[string]any{"bagian": []map[string]string{{"id": "lain", "nama": "Tidak boleh"}}}, "")
		if blocked.StatusCode != http.StatusForbidden {
			blocked.Body.Close()
			t.Fatalf("read-only/outsider write got %d", blocked.StatusCode)
		}
		blocked.Body.Close()
	}
	graderRead, _ := makeRequest(app, http.MethodGet, "/api/ujian/"+exam.ID+"/bagian", graderToken, nil, "")
	if graderRead.StatusCode != http.StatusOK {
		graderRead.Body.Close()
		t.Fatalf("grader read got %d", graderRead.StatusCode)
	}
	graderRead.Body.Close()
	viewerWrite, _ := makeRequest(app, http.MethodPost, "/api/simulasi/paket/"+packet.ID+"/soal", viewerToken, map[string]string{"soalId": "missing-question"}, "")
	if viewerWrite.StatusCode != http.StatusForbidden {
		viewerWrite.Body.Close()
		t.Fatalf("viewer Simulasi write got %d", viewerWrite.StatusCode)
	}
	viewerWrite.Body.Close()

	// An editor may not elevate access by sharing onward; owners/admins may revoke.
	deniedGrant, _ := makeRequest(app, http.MethodPost, "/api/assessment/simulasi/"+packet.ID+"/collaborators", editorToken, map[string]string{"username": outsider.Username, "peran": assessmentRoleEditor}, "")
	if deniedGrant.StatusCode != http.StatusForbidden {
		deniedGrant.Body.Close()
		t.Fatalf("editor delegated access got %d", deniedGrant.StatusCode)
	}
	deniedGrant.Body.Close()
	adminList, _ := makeRequest(app, http.MethodGet, "/api/assessment/simulasi/"+packet.ID+"/collaborators", adminToken, nil, "")
	var collaboratorPayload struct {
		Items []AsesmenKolaborator `json:"items"`
	}
	_ = json.NewDecoder(adminList.Body).Decode(&collaboratorPayload)
	adminList.Body.Close()
	var viewerRow AsesmenKolaborator
	for _, row := range collaboratorPayload.Items {
		if row.UserID == viewer.ID {
			viewerRow = row
		}
	}
	if viewerRow.ID == "" {
		t.Fatal("viewer collaborator record missing")
	}
	revoked, _ := makeRequest(app, http.MethodDelete, "/api/assessment/simulasi/"+packet.ID+"/collaborators/"+viewerRow.ID, adminToken, nil, "")
	if revoked.StatusCode != http.StatusNoContent {
		revoked.Body.Close()
		t.Fatalf("revoke got %d", revoked.StatusCode)
	}
	revoked.Body.Close()
	viewerList, _ := makeRequest(app, http.MethodGet, "/api/simulasi/paket", viewerToken, nil, "")
	data, _ := io.ReadAll(viewerList.Body)
	viewerList.Body.Close()
	if bytes.Contains(data, []byte(packet.ID)) {
		t.Fatalf("revoked viewer still sees shared package: %s", data)
	}
}

func TestAssessmentGraderCanGradeManualUjianAnswerButViewerCannot(t *testing.T) {
	s, app := setupE2EServer(t)
	_, _ = getAdminToken(t, app)
	var admin User
	if err := s.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	grader := assessmentTestTeacher(t, s, "assessment-essay-grader")
	viewer := assessmentTestTeacher(t, s, "assessment-essay-viewer")
	graderToken := assessmentTestTeacherToken(t, app, grader.Username)
	viewerToken := assessmentTestTeacherToken(t, app, viewer.Username)

	class := Kelas{Jenjang: 6, NamaRombel: "KOLABORASI"}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{KelasID: class.ID, Judul: "Ujian penilaian bersama", DibuatOlehUserID: admin.ID}
	if err := s.db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	question := BankSoal{Tipe: "essay", Pertanyaan: "Jelaskan alasanmu", Kunci: "jawaban", Poin: 2}
	if err := s.db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 2}).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Peserta Kolaborasi", NIS: "KOL-1", NISN: "9900000021", KelasID: class.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Mulai: &started, Status: "menunggu_nilai"}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	answer := UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.ID, Jawaban: "Alasan yang perlu dinilai guru."}
	if err := s.db.Create(&answer).Error; err != nil {
		t.Fatal(err)
	}
	for _, grant := range []AsesmenKolaborator{
		{Modul: assessmentModuleUjian, AsesmenID: exam.ID, UserID: grader.ID, Peran: assessmentRoleGrader, DibuatOlehUserID: admin.ID},
		{Modul: assessmentModuleUjian, AsesmenID: exam.ID, UserID: viewer.ID, Peran: assessmentRoleViewer, DibuatOlehUserID: admin.ID},
	} {
		if err := s.db.Create(&grant).Error; err != nil {
			t.Fatal(err)
		}
	}
	gradeURL := "/api/ujian-online/monitor/" + exam.ID + "/attempt/" + attempt.ID + "/answer/" + answer.ID + "/grade"
	viewerResponse, err := makeRequest(app, http.MethodPost, gradeURL, viewerToken, map[string]any{"nilai": 1, "komentar": "Tidak boleh"}, "")
	if err != nil {
		t.Fatal(err)
	}
	viewerResponse.Body.Close()
	if viewerResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer grading should be denied, got %d", viewerResponse.StatusCode)
	}
	graderResponse, err := makeRequest(app, http.MethodPost, gradeURL, graderToken, map[string]any{"nilai": 1, "komentar": "Jawaban sudah menunjukkan alasan."}, "")
	if err != nil {
		t.Fatal(err)
	}
	graderResponse.Body.Close()
	if graderResponse.StatusCode != http.StatusOK {
		t.Fatalf("assigned grader should be able to grade, got %d", graderResponse.StatusCode)
	}
	var stored UjianJawaban
	if err := s.db.First(&stored, "id = ?", answer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.NilaiManual == nil || *stored.NilaiManual != 1 || stored.DinilaiOlehUserID == nil || *stored.DinilaiOlehUserID != grader.ID {
		t.Fatalf("manual score must be attributed to the assigned grader, got %+v", stored)
	}
}
