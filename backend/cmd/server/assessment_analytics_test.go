package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func assessmentAnalyticsFixture(t *testing.T) (*Server, *fiber.App, string, string, string, string) {
	t.Helper()
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}, &Kelas{}, &PesertaDidik{}, &MataPelajaran{}, &BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}, &SimulasiPaket{}, &SimulasiPenugasan{}, &SimulasiUpaya{}, &SimulasiPaketSoal{}, &SimulasiUpayaSoal{}, &SimulasiJawaban{}); err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Tutor Analitik", JenisKelamin: "P"}
	otherTutor := Tutor{Nama: "Tutor Lain", JenisKelamin: "L"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherTutor).Error; err != nil {
		t.Fatal(err)
	}
	teacher := User{Username: "analytics-teacher", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := s.db.Create(&teacher).Error; err != nil {
		t.Fatal(err)
	}
	classA := Kelas{Jenjang: 5, NamaRombel: "A"}
	classB := Kelas{Jenjang: 5, NamaRombel: "B"}
	classA.WaliKelasID, classB.WaliKelasID = &tutor.ID, &otherTutor.ID
	for _, class := range []*Kelas{&classA, &classB} {
		if err := s.db.Create(class).Error; err != nil {
			t.Fatal(err)
		}
	}
	studentA := PesertaDidik{Nama: "Siswa Alpha", NIS: "AN-001", NISN: "NISN-001", KelasID: classB.ID, Status: "aktif"}
	studentB := PesertaDidik{Nama: "Siswa Beta", NIS: "AN-002", NISN: "NISN-002", KelasID: classB.ID, Status: "aktif"}
	for _, student := range []*PesertaDidik{&studentA, &studentB} {
		if err := s.db.Create(student).Error; err != nil {
			t.Fatal(err)
		}
	}
	mapel := MataPelajaran{NamaMapel: "Matematika"}
	if err := s.db.Create(&mapel).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	scoreExam, scoreSim := 80.0, 90.0
	examA := Ujian{MapelID: mapel.ID, KelasID: classA.ID, Judul: "Ujian Alpha", WaktuMulai: started, WaktuSelesai: started.Add(time.Hour)}
	examB := Ujian{MapelID: mapel.ID, KelasID: classB.ID, Judul: "Ujian Beta", WaktuMulai: started, WaktuSelesai: started.Add(time.Hour)}
	for _, exam := range []*Ujian{&examA, &examB} {
		if err := s.db.Create(exam).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, attempt := range []*UjianPeserta{
		{UjianID: examA.ID, PesertaDidikID: studentA.ID, KelasIDSaatUjian: classA.ID, Status: "selesai", Mulai: &started, Selesai: &started, Skor: &scoreExam},
		{UjianID: examB.ID, PesertaDidikID: studentB.ID, KelasIDSaatUjian: classB.ID, Status: "selesai", Mulai: &started, Selesai: &started, Skor: &scoreExam},
	} {
		if err := s.db.Create(attempt).Error; err != nil {
			t.Fatal(err)
		}
	}
	packageA := SimulasiPaket{Nama: "Paket Alpha", Mode: "anbk_akm", Jenjang: "SD/MI", Status: "terbit", DibuatOlehUserID: teacher.ID}
	packageB := SimulasiPaket{Nama: "Paket Beta", Mode: "tka_sd", Jenjang: "SD/MI", Status: "terbit", DibuatOlehUserID: "someone-else"}
	for _, packet := range []*SimulasiPaket{&packageA, &packageB} {
		if err := s.db.Create(packet).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, assignment := range []*SimulasiPenugasan{
		{PaketID: packageA.ID, PesertaDidikID: studentA.ID, KelasIDSaatTugas: classA.ID},
		{PaketID: packageB.ID, PesertaDidikID: studentB.ID, KelasIDSaatTugas: classB.ID},
	} {
		if err := s.db.Create(assignment).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, attempt := range []*SimulasiUpaya{
		{PaketID: packageA.ID, PesertaDidikID: studentA.ID, KelasIDSaatUjian: classA.ID, Nomor: 1, Status: "selesai", Mulai: &started, Selesai: &started, SeedUrutan: "seed-a", SkorAkhir: &scoreSim},
		{PaketID: packageB.ID, PesertaDidikID: studentB.ID, KelasIDSaatUjian: classB.ID, Nomor: 1, Status: "selesai", Mulai: &started, Selesai: &started, SeedUrutan: "seed-b", SkorAkhir: &scoreSim},
	} {
		if err := s.db.Create(attempt).Error; err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	for _, path := range []string{"/analytics", "/export", "/report.pdf", "/questions", "/questions/export"} {
		endpoint := s.assessmentAnalytics
		switch path {
		case "/export":
			endpoint = s.exportAssessmentAnalytics
		case "/report.pdf":
			endpoint = s.exportAssessmentAnalyticsPDF
		case "/questions":
			endpoint = s.assessmentQuestionAnalytics
		case "/questions/export":
			endpoint = s.exportAssessmentQuestionAnalytics
		}
		app.Get(path, func(c *fiber.Ctx) error {
			c.Locals("role", c.Get("X-Test-Role"))
			c.Locals("userID", c.Get("X-Test-User"))
			return endpoint(c)
		})
	}
	return s, app, teacher.ID, classA.ID, classB.ID, studentA.ID
}

func TestAssessmentAnalyticsCombinesModulesAndKeepsClassSnapshot(t *testing.T) {
	_, app, teacherID, classA, _, studentID := assessmentAnalyticsFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics?kelasId="+classA+"&pesertaDidikId="+studentID, nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("analytics status = %d", res.StatusCode)
	}
	var response struct {
		Summary assessmentAnalyticsSummary `json:"ringkasan"`
		Rows    []assessmentAnalyticsRow   `json:"pengerjaan"`
		Classes []assessmentClassProgress  `json:"statistikKelas"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 2 || response.Summary.Total != 2 || response.Summary.Selesai != 2 {
		t.Fatalf("expected two scoped records from both modules, got %+v", response)
	}
	modules := map[string]bool{}
	for _, row := range response.Rows {
		modules[row.Module] = true
		if row.ClassID != classA || row.ClassName != "A" || row.ClassGrade != 5 {
			t.Fatalf("attempt must report class at attempt time, got %+v", row)
		}
	}
	if !modules["ujian_online"] || !modules["simulasi"] {
		t.Fatalf("report should include both modules, got %v", modules)
	}
	if len(response.Classes) != 1 || response.Classes[0].ClassID != classA || response.Classes[0].Students != 1 || response.Classes[0].Total != 2 || response.Classes[0].Average == nil || *response.Classes[0].Average != 85 {
		t.Fatalf("class summary should aggregate both modules for the same student: %+v", response.Classes)
	}
}

func TestAssessmentAnalyticsOngoingFilterIncludesLegacyAndSimulationStatuses(t *testing.T) {
	s, app, teacherID, classA, _, studentID := assessmentAnalyticsFixture(t)
	var exam Ujian
	var packet SimulasiPaket
	if err := s.db.Where("kelas_id = ?", classA).First(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("dibuat_oleh_user_id = ?", teacherID).First(&packet).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	if err := s.db.Model(&UjianPeserta{}).Where("ujian_id = ? AND peserta_didik_id = ?", exam.ID, studentID).Updates(map[string]interface{}{"status": "mulai", "mulai": started, "selesai": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiUpaya{PaketID: packet.ID, PesertaDidikID: studentID, KelasIDSaatUjian: classA, Nomor: 2, Status: "berlangsung", Mulai: &started}).Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/analytics?kelasId="+classA+"&status=berlangsung", nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("analytics status=%d body=%s", res.StatusCode, body)
	}
	var response struct {
		Rows []assessmentAnalyticsRow `json:"pengerjaan"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("ongoing filter should include both legacy 'mulai' and Simulasi 'berlangsung' rows, got %+v", response.Rows)
	}
	modules := map[string]string{}
	for _, row := range response.Rows {
		modules[row.Module] = row.Status
	}
	if modules["ujian_online"] != "mulai" || modules["simulasi"] != "berlangsung" {
		t.Fatalf("ongoing results should retain source statuses, got %v", modules)
	}
}

func TestAssessmentTimeFilterDateOnlyUsesWIBDayBoundaries(t *testing.T) {
	from, err := assessmentTimeFilter("2026-09-25", false)
	if err != nil {
		t.Fatal(err)
	}
	until, err := assessmentTimeFilter("2026-09-25", true)
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2026, 9, 25, 0, 0, 0, 0, wibLocation)
	wantUntil := time.Date(2026, 9, 25, 23, 59, 59, int(time.Second-time.Nanosecond), wibLocation)
	if !from.Equal(wantFrom) || !until.Equal(wantUntil) {
		t.Fatalf("date-only filters should span the selected WIB calendar day: from=%s until=%s", from, until)
	}
	// Explicit timestamps carry their own timezone and must not be reinterpreted.
	explicit, err := assessmentTimeFilter("2026-09-25T00:00:00Z", false)
	if err != nil {
		t.Fatal(err)
	}
	if !explicit.Equal(time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("explicit RFC3339 timestamp changed meaning: %s", explicit)
	}
}

func TestAssessmentAnalyticsSubjectFilterAppliesAcrossModules(t *testing.T) {
	s, app, teacherID, classID, _, studentID := assessmentAnalyticsFixture(t)
	var math MataPelajaran
	if err := s.db.Where("nama_mapel = ?", "Matematika").First(&math).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&SimulasiPaket{}).Where("nama = ?", "Paket Alpha").Update("mapel_id", math.ID).Error; err != nil {
		t.Fatal(err)
	}
	otherSubject := MataPelajaran{NamaMapel: "Bahasa Indonesia"}
	if err := s.db.Create(&otherSubject).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	otherExam := Ujian{MapelID: otherSubject.ID, KelasID: classID, Judul: "Ujian Bahasa", WaktuMulai: started, WaktuSelesai: started.Add(time.Hour)}
	if err := s.db.Create(&otherExam).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianPeserta{UjianID: otherExam.ID, PesertaDidikID: studentID, KelasIDSaatUjian: classID, Status: "selesai", Mulai: &started}).Error; err != nil {
		t.Fatal(err)
	}
	otherSubjectID := otherSubject.ID
	otherPackage := SimulasiPaket{Nama: "Latihan Bahasa", Mode: "anbk_akm", Jenjang: "SD/MI", MapelID: &otherSubjectID, Status: "terbit", DibuatOlehUserID: teacherID}
	if err := s.db.Create(&otherPackage).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiUpaya{PaketID: otherPackage.ID, PesertaDidikID: studentID, KelasIDSaatUjian: classID, Nomor: 1, Status: "selesai", Mulai: &started}).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/analytics?mapelId="+math.ID, nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("analytics status = %d", res.StatusCode)
	}
	var response struct {
		Rows []assessmentAnalyticsRow `json:"pengerjaan"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("subject-filtered rows = %d, want the two matching module rows", len(response.Rows))
	}
	for _, row := range response.Rows {
		if row.Subject != "Matematika" {
			t.Fatalf("subject filter returned unrelated row: %+v", row)
		}
	}
}

func TestAssessmentAnalyticsEnforcesClassAndRoleScope(t *testing.T) {
	_, app, teacherID, _, classB, _ := assessmentAnalyticsFixture(t)
	request := func(role, user, query string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/analytics"+query, nil)
		req.Header.Set("X-Test-Role", role)
		req.Header.Set("X-Test-User", user)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if got := request("guru", teacherID, "?kelasId="+classB).StatusCode; got != http.StatusForbidden {
		t.Fatalf("guru class IDOR returned HTTP %d", got)
	}
	if got := request("siswa", "student-user", "").StatusCode; got != http.StatusForbidden {
		t.Fatalf("student access to staff analytics returned HTTP %d", got)
	}
	admin := request("admin", "admin-user", "")
	if admin.StatusCode != http.StatusOK {
		t.Fatalf("admin analytics status = %d", admin.StatusCode)
	}
	var response struct {
		Rows []assessmentAnalyticsRow `json:"pengerjaan"`
	}
	if err := json.NewDecoder(admin.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 4 {
		t.Fatalf("admin should see all module records, got %d", len(response.Rows))
	}
}

func TestAssessmentAnalyticsUsesLegacyExamAndAssignmentClassWhenSnapshotIsMissing(t *testing.T) {
	s, app, teacherID, classA, _, studentID := assessmentAnalyticsFixture(t)
	if err := s.db.Model(&UjianPeserta{}).Where("peserta_didik_id = ?", studentID).Update("kelas_id_saat_ujian", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&SimulasiUpaya{}).Where("peserta_didik_id = ?", studentID).Update("kelas_id_saat_ujian", "").Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/analytics?kelasId="+classA+"&pesertaDidikId="+studentID, nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("analytics status = %d", res.StatusCode)
	}
	var response struct {
		Rows []assessmentAnalyticsRow `json:"pengerjaan"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("legacy rows should resolve their original class from exam/assignment: %+v", response.Rows)
	}
	for _, row := range response.Rows {
		if row.ClassID != classA {
			t.Fatalf("expected historical class %s for legacy row, got %+v", classA, row)
		}
	}
}

func TestAssessmentAnalyticsExportAndSpreadsheetFormulaProtection(t *testing.T) {
	privateSnapshot, err := json.Marshal(UjianPeserta{KelasIDSaatUjian: "private-class"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(privateSnapshot), "private-class") || strings.Contains(string(privateSnapshot), "kelasIdSaatUjian") {
		t.Fatalf("internal class snapshot must not leak in legacy student API payloads: %s", string(privateSnapshot))
	}
	if got := csvSafeField("=1+1"); got != "'=1+1" {
		t.Fatalf("CSV formula value not protected: %q", got)
	}
	_, app, teacherID, classA, _, _ := assessmentAnalyticsFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/export?modul=simulasi&kelasId="+classA, nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("export status = %d: %s", res.StatusCode, string(b))
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Paket Alpha") || strings.Contains(string(body), "Paket Beta") {
		t.Fatalf("teacher export should be scoped to owned assessment and class: %s", string(body))
	}
}

func TestAssessmentAnalyticsCountsPendingManualAsSubmittedProgress(t *testing.T) {
	started := time.Now()
	rows := []assessmentAnalyticsRow{{
		Module: "ujian_online", AssessmentID: "exam-1", StudentID: "student-1", StudentName: "Siswa",
		ClassID: "class-1", ClassGrade: 6, ClassName: "A", Status: "menunggu_nilai", StartedAt: &started,
	}}
	summary, students, classes := summarizeAssessmentRows(rows)
	if summary.MenungguNilai != 1 || summary.Selesai != 0 {
		t.Fatalf("pending manual score must have its own summary bucket: %+v", summary)
	}
	if len(students) != 1 || students[0].Completed != 1 || len(classes) != 1 || classes[0].Completed != 1 {
		t.Fatalf("submitted pending-grade attempts should count as completed work: students=%+v classes=%+v", students, classes)
	}
}

func TestAssessmentQuestionAnalyticsCombinesModulesAndSeparatesPendingAndBlank(t *testing.T) {
	s, app, teacherID, classA, _, studentID := assessmentAnalyticsFixture(t)
	var exam Ujian
	var examAttempt UjianPeserta
	var simulationPackage SimulasiPaket
	var simulationAttempt SimulasiUpaya
	if err := s.db.Where("kelas_id = ?", classA).First(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", exam.ID, studentID).First(&examAttempt).Error; err != nil {
		t.Fatal(err)
	}
	correct := true
	objective := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "=perhitungan", Opsi: `["A","B"]`, Kunci: "0", Poin: 2, Domain: "Numerasi", Topik: "Pola bilangan", Kompetensi: "Menemukan pola", LevelKognitif: "Menalar"}
	if err := s.db.Create(&objective).Error; err != nil {
		t.Fatal(err)
	}
	objectiveConfig := simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}
	objectiveSnapshot, _ := json.Marshal(ujianQuestionSnapshot{Tipe: simulasiTipePG, Pertanyaan: objective.Pertanyaan, Opsi: objective.Opsi, Kunci: "0", Konfigurasi: objectiveConfig, Metadata: map[string]string{"domain": objective.Domain, "topik": objective.Topik, "kompetensi": objective.Kompetensi, "levelKognitif": objective.LevelKognitif}})
	examItem := UjianSoal{UjianID: exam.ID, SoalID: objective.ID, Urutan: 1, Bobot: 2}
	if err := s.db.Create(&examItem).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianPesertaSoal{UjianPesertaID: examAttempt.ID, UjianSoalID: examItem.ID, SoalID: objective.ID, Urutan: 1, Bobot: 2, SnapshotJSON: string(objectiveSnapshot)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianJawaban{UjianPesertaID: examAttempt.ID, SoalID: objective.ID, Jawaban: `"a"`, Benar: &correct, Nilai: 2}).Error; err != nil {
		t.Fatal(err)
	}
	essay := BankSoal{Tipe: "essay", Pertanyaan: "Jelaskan alasanmu", Poin: 4}
	if err := s.db.Create(&essay).Error; err != nil {
		t.Fatal(err)
	}
	essaySnapshot, _ := json.Marshal(ujianQuestionSnapshot{Tipe: "essay", Pertanyaan: essay.Pertanyaan, Kunci: "rubrik"})
	essayItem := UjianSoal{UjianID: exam.ID, SoalID: essay.ID, Urutan: 2, Bobot: 4}
	if err := s.db.Create(&essayItem).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianPesertaSoal{UjianPesertaID: examAttempt.ID, UjianSoalID: essayItem.ID, SoalID: essay.ID, Urutan: 2, Bobot: 4, SnapshotJSON: string(essaySnapshot)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianJawaban{UjianPesertaID: examAttempt.ID, SoalID: essay.ID, Jawaban: "Jawaban esai"}).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.db.Where("dibuat_oleh_user_id = ?", teacherID).First(&simulationPackage).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("paket_id = ? AND peserta_didik_id = ?", simulationPackage.ID, studentID).First(&simulationAttempt).Error; err != nil {
		t.Fatal(err)
	}
	studentQuestion := simulasiSnapshot{Tipe: simulasiTipePG, Pertanyaan: "Pilih jawaban yang tepat", Konfigurasi: objectiveConfig, Metadata: map[string]string{"domain": "Numerasi", "topik": "Pola bilangan", "kompetensi": "Menemukan pola"}}
	studentSnapshot, _ := json.Marshal(studentQuestion)
	simItem := SimulasiPaketSoal{PaketID: simulationPackage.ID, SoalID: &objective.ID, Urutan: 1, Bobot: 3, SnapshotJSON: string(studentSnapshot)}
	if err := s.db.Create(&simItem).Error; err != nil {
		t.Fatal(err)
	}
	simLink := SimulasiUpayaSoal{UpayaID: simulationAttempt.ID, PaketSoalID: simItem.ID, UrutanTampil: 1}
	if err := s.db.Create(&simLink).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiJawaban{UpayaSoalID: simLink.ID, JawabanJSON: `"a"`, Benar: &correct, SkorOtomatis: 3, SkorAkhir: 3}).Error; err != nil {
		t.Fatal(err)
	}
	blankSnapshot, _ := json.Marshal(simulasiSnapshot{Tipe: simulasiTipePG, Pertanyaan: "Soal kosong contoh", Konfigurasi: objectiveConfig})
	blankItem := SimulasiPaketSoal{PaketID: simulationPackage.ID, Urutan: 2, Bobot: 1, SnapshotJSON: string(blankSnapshot)}
	if err := s.db.Create(&blankItem).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&SimulasiUpayaSoal{UpayaID: simulationAttempt.ID, PaketSoalID: blankItem.ID, UrutanTampil: 2}).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/questions?kelasId="+classA, nil)
	req.Header.Set("X-Test-Role", "guru")
	req.Header.Set("X-Test-User", teacherID)
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("question analytics status=%d body=%s", response.StatusCode, body)
	}
	var body struct {
		Questions []assessmentQuestionStat `json:"soal"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	byQuestion := map[string]assessmentQuestionStat{}
	for _, question := range body.Questions {
		byQuestion[question.Question] = question
	}
	if question := byQuestion[objective.Pertanyaan]; question.Correct != 1 || question.Answered != 1 || question.SuccessRate == nil || *question.SuccessRate != 100 || question.AverageScore == nil || *question.AverageScore != 100 || question.Domain != "Numerasi" || question.Topic != "Pola bilangan" || question.Competency != "Menemukan pola" || question.EarnedPoints != 2 || question.GradedWeight != 2 {
		t.Fatalf("Ujian Online objective statistics should be fully correct: %+v", question)
	}
	if question := byQuestion[essay.Pertanyaan]; question.PendingGrade != 1 || question.SuccessRate != nil || question.Blank != 0 {
		t.Fatalf("manual response must remain pending and excluded from rate: %+v", question)
	}
	if question := byQuestion[studentQuestion.Pertanyaan]; question.Attempts != 1 || question.Correct != 1 || question.SuccessRate == nil || *question.SuccessRate != 100 || question.Domain != "Numerasi" || question.Competency != "Menemukan pola" || question.EarnedPoints != 3 || question.GradedWeight != 3 {
		t.Fatalf("simulation answered question stats mismatch: %+v", question)
	}
	if question := byQuestion["Soal kosong contoh"]; question.Attempts != 1 || question.Blank != 1 || question.Answered != 0 || question.SuccessRate == nil || *question.SuccessRate != 0 {
		t.Fatalf("simulation blank response should be counted separately: %+v", question)
	}
}

func TestAssessmentQuestionAnalyticsExportIsScopedAndSupportsXLSX(t *testing.T) {
	s, app, teacherID, classA, _, studentID := assessmentAnalyticsFixture(t)
	var exam Ujian
	var attempt UjianPeserta
	if err := s.db.Where("kelas_id = ?", classA).First(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", exam.ID, studentID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	question := BankSoal{Tipe: simulasiTipePG, Pertanyaan: "=1+1", Opsi: `["2","3"]`, Kunci: "0", Poin: 1, Domain: "Numerasi", Topik: "Operasi hitung", Kompetensi: "Menjumlah bilangan"}
	if err := s.db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	item := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(ujianQuestionSnapshot{Tipe: simulasiTipePG, Pertanyaan: question.Pertanyaan, Opsi: question.Opsi, Kunci: question.Kunci, Metadata: map[string]string{"domain": question.Domain, "topik": question.Topik, "kompetensi": question.Kompetensi}})
	if err := s.db.Create(&UjianPesertaSoal{UjianPesertaID: attempt.ID, UjianSoalID: item.ID, SoalID: question.ID, Bobot: 1, SnapshotJSON: string(snapshot)}).Error; err != nil {
		t.Fatal(err)
	}
	request := func(path string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Role", "guru")
		req.Header.Set("X-Test-User", teacherID)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	csvResponse := request("/questions/export?kelasId=" + classA)
	if csvResponse.StatusCode != http.StatusOK {
		t.Fatalf("CSV export status=%d", csvResponse.StatusCode)
	}
	csvBody, _ := io.ReadAll(csvResponse.Body)
	if !strings.Contains(string(csvBody), "'=1+1") || !strings.Contains(string(csvBody), "Numerasi") || !strings.Contains(string(csvBody), "Menjumlah bilangan") {
		t.Fatalf("CSV export must mitigate spreadsheet formula injection and include learning metadata: %s", csvBody)
	}
	xlsxResponse := request("/questions/export?format=xlsx&kelasId=" + classA)
	if xlsxResponse.StatusCode != http.StatusOK || !strings.Contains(xlsxResponse.Header.Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("XLSX export status=%d type=%s", xlsxResponse.StatusCode, xlsxResponse.Header.Get("Content-Type"))
	}
}

func TestAssessmentAnalyticsPDFRequiresScopeFilterAndReturnsReport(t *testing.T) {
	_, app, teacherID, classA, _, _ := assessmentAnalyticsFixture(t)
	request := func(path string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Role", "guru")
		req.Header.Set("X-Test-User", teacherID)
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	if response := request("/report.pdf"); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("PDF without a class/student filter must be rejected, got %d", response.StatusCode)
	}
	response := request("/report.pdf?kelasId=" + classA)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/pdf" {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("PDF export status=%d type=%s body=%q", response.StatusCode, response.Header.Get("Content-Type"), body)
	}
	body, _ := io.ReadAll(response.Body)
	if len(body) < 5 || string(body[:5]) != "%PDF-" || strings.Contains(string(body), "Siswa Beta") {
		t.Fatalf("PDF should be valid and scoped to the selected class")
	}
}
