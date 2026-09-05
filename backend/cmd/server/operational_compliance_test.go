package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func getOperationalCompliance(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, token, path string) operationalComplianceResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatalf("get operational compliance: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get operational compliance: want 200, got %d", response.StatusCode)
	}
	var result operationalComplianceResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode operational compliance: %v", err)
	}
	return result
}

func TestOperationalComplianceDashboard(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	today := currentWIBDay()
	meetingDay := latestSaturday(today)

	var year TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		t.Fatal(err)
	}
	year.TanggalMulai = meetingDay
	year.TanggalSelesai = today.AddDate(0, 0, 30)
	if err := s.db.Save(&year).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&Semester{}).Where("tahun_ajaran_id = ?", year.ID).Update("is_archived", true).Error; err != nil {
		t.Fatal(err)
	}
	semester := Semester{
		TahunAjaranID: year.ID, NamaSemester: "Kepatuhan", TanggalMulai: meetingDay,
		TanggalSelesai: today.AddDate(0, 0, 30), IsArchived: false,
	}
	if err := s.db.Create(&semester).Error; err != nil {
		t.Fatal(err)
	}

	var pokjar Pokjar
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	tutorA := Tutor{Nama: "Wali Kepatuhan A", JenisKelamin: "P"}
	tutorB := Tutor{Nama: "Wali Kepatuhan B", JenisKelamin: "L"}
	if err := s.db.Create(&tutorA).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&tutorB).Error; err != nil {
		t.Fatal(err)
	}
	headToken := loginRole(t, app, adminToken, "kepala-kepatuhan", "kepala_sekolah", nil)
	guruToken := loginRole(t, app, adminToken, "guru-kepatuhan", "guru", &tutorA.ID)

	classes := []Kelas{
		{Jenjang: 11, NamaRombel: "KPA", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorA.ID}, // no attendance
		{Jenjang: 12, NamaRombel: "KPB", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorA.ID}, // partial
		{Jenjang: 13, NamaRombel: "KPC", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorA.ID}, // complete
		{Jenjang: 14, NamaRombel: "KPD", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorA.ID}, // holiday
		{Jenjang: 15, NamaRombel: "KPE", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorB.ID}, // other wali
	}
	for index := range classes {
		if err := s.db.Create(&classes[index]).Error; err != nil {
			t.Fatal(err)
		}
		student := PesertaDidik{
			Nama: "Siswa Kepatuhan", JenisKelamin: "L", NIS: "COMPLIANCE-" + classes[index].NamaRombel,
			NISN: "COMPLIANCE-" + classes[index].NamaRombel, KelasID: classes[index].ID,
			PokjarID: pokjar.ID, Status: "aktif",
		}
		if err := s.db.Create(&student).Error; err != nil {
			t.Fatal(err)
		}
	}

	partial := Presensi{
		KelasID: classes[1].ID, Tanggal: meetingDay, StatusPertemuan: "berlangsung",
		TandaTangan: validPngSignature, BuktiFoto: "[\"foto\"]",
	}
	if err := s.db.Create(&partial).Error; err != nil {
		t.Fatal(err)
	}
	complete := Presensi{
		KelasID: classes[2].ID, Tanggal: meetingDay, StatusPertemuan: "berlangsung",
		TandaTangan: validPngSignature, BuktiFoto: "[\"foto\"]",
	}
	if err := s.db.Create(&complete).Error; err != nil {
		t.Fatal(err)
	}
	var completeStudent PesertaDidik
	if err := s.db.Where("kelas_id = ?", classes[2].ID).First(&completeStudent).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: complete.ID, PesertaDidikID: completeStudent.ID, StatusKehadiran: "Hadir"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&Presensi{KelasID: classes[3].ID, Tanggal: meetingDay, StatusPertemuan: "libur"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&JurnalMengajar{TutorID: tutorA.ID, KelasID: classes[2].ID, Tanggal: today, Materi: "Sudah tercatat", Status: "disetujui"}).Error; err != nil {
		t.Fatal(err)
	}

	result := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran")
	if result.Filters.TahunAjaranID != year.ID || result.Filters.SemesterID != semester.ID {
		t.Fatalf("default active period not selected: %+v", result.Filters)
	}
	if result.Summary.PresensiTertunda != 3 || result.Summary.JurnalTertunda != 4 || result.Summary.TotalTertunda != 7 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Summary.TutorDenganTugas != 2 || result.Summary.KelasDenganTugas != 4 {
		t.Fatalf("unexpected pending teacher/class counts: %+v", result.Summary)
	}
	if len(result.RingkasanKelas) != 5 {
		t.Fatalf("want all five class summaries, got %+v", result.RingkasanKelas)
	}
	pokjarOptionFound := false
	for _, option := range result.Options.Pokjar {
		if option.ID == pokjar.ID && option.Label == pokjar.NamaPokjar {
			pokjarOptionFound = true
			break
		}
	}
	if !pokjarOptionFound {
		t.Fatalf("active year pokjar option missing: %+v", result.Options.Pokjar)
	}

	tasksByClass := map[string][]operationalComplianceTask{}
	for _, task := range result.Tasks {
		tasksByClass[task.ClassID] = append(tasksByClass[task.ClassID], task)
	}
	if len(tasksByClass[classes[0].ID]) != 2 || len(tasksByClass[classes[1].ID]) != 2 || len(tasksByClass[classes[3].ID]) != 1 || len(tasksByClass[classes[4].ID]) != 2 {
		t.Fatalf("missing/partial/holiday/other wali tasks are incorrect: %+v", tasksByClass)
	}
	if _, found := tasksByClass[classes[2].ID]; found {
		t.Fatal("complete attendance and journal must not be pending")
	}
	if len(tasksByClass[classes[3].ID]) != 1 || tasksByClass[classes[3].ID][0].Type != "jurnal" {
		t.Fatalf("holiday must exempt attendance only: %+v", tasksByClass[classes[3].ID])
	}
	partialFound := false
	for _, task := range tasksByClass[classes[1].ID] {
		if task.Type == "presensi" {
			partialFound = task.MeetingID == partial.ID && task.Reason == "kehadiran 0/1 siswa"
		}
	}
	if !partialFound {
		t.Fatalf("partial attendance direct link/context is invalid: %+v", tasksByClass[classes[1].ID])
	}

	// The supervisory dashboard can be narrowed without leaking a different
	// wali's records into a selected teacher or class result.
	forTutorA := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran?tutorId="+tutorA.ID)
	if len(forTutorA.Tasks) != 5 {
		t.Fatalf("tutor filter: want 5 tasks, got %+v", forTutorA.Tasks)
	}
	for _, task := range forTutorA.Tasks {
		if task.TutorID != tutorA.ID {
			t.Fatalf("tutor filter leaked another wali: %+v", task)
		}
	}
	presensiOnly := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran?kelasId="+classes[1].ID+"&status=presensi")
	if len(presensiOnly.Tasks) != 1 || presensiOnly.Tasks[0].Type != "presensi" || presensiOnly.Tasks[0].MeetingID != partial.ID {
		t.Fatalf("class/status filter and attendance link context invalid: %+v", presensiOnly.Tasks)
	}
	jurnalOnly := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran?status=jurnal")
	if len(jurnalOnly.Tasks) != 4 {
		t.Fatalf("journal filter: want 4 tasks, got %+v", jurnalOnly.Tasks)
	}
	for _, task := range jurnalOnly.Tasks {
		if task.Type != "jurnal" || task.Date != today.Format("2006-01-02") {
			t.Fatalf("journal task is invalid: %+v", task)
		}
	}
	noClass := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran?kelasId=not-a-class")
	if len(noClass.Tasks) != 0 || len(noClass.RingkasanKelas) != 0 {
		t.Fatalf("unknown class filter must be empty: %+v", noClass)
	}

	// Kepala sekolah has the same read-only monitoring access; guru does not.
	if headResult := getOperationalCompliance(t, app, headToken, "/api/dashboard/kepatuhan-pembelajaran"); headResult.Summary.TotalTertunda != 7 {
		t.Fatalf("kepala sekolah view mismatch: %+v", headResult.Summary)
	}
	otherPokjar := Pokjar{NamaPokjar: "Pokjar Kepatuhan Lain", Tipe: "binaan"}
	if err := s.db.Create(&otherPokjar).Error; err != nil {
		t.Fatal(err)
	}
	otherClass := Kelas{Jenjang: 16, NamaRombel: "KPF", PokjarID: otherPokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutorB.ID}
	if err := s.db.Create(&otherClass).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&PesertaDidik{Nama: "Siswa Pokjar Lain", JenisKelamin: "L", NIS: "COMPLIANCE-LAIN", NISN: "COMPLIANCE-LAIN", KelasID: otherClass.ID, PokjarID: otherPokjar.ID, Status: "aktif"}).Error; err != nil {
		t.Fatal(err)
	}
	forPokjar := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran?pokjarId="+otherPokjar.ID)
	if len(forPokjar.RingkasanKelas) != 1 || forPokjar.RingkasanKelas[0].ClassID != otherClass.ID {
		t.Fatalf("pokjar filter leaked another class: %+v", forPokjar.RingkasanKelas)
	}
	if len(forPokjar.Options.Kelas) != 1 || forPokjar.Options.Kelas[0].ID != otherClass.ID {
		t.Fatalf("pokjar filter did not narrow rombel options: %+v", forPokjar.Options.Kelas)
	}
	guruResponse, err := makeRequest(app, http.MethodGet, "/api/dashboard/kepatuhan-pembelajaran", guruToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer guruResponse.Body.Close()
	if guruResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("guru access: want 403, got %d", guruResponse.StatusCode)
	}

	if err := s.db.Model(&Semester{}).Where("id = ?", semester.ID).Update("is_archived", true).Error; err != nil {
		t.Fatal(err)
	}
	withoutActiveSemester := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran")
	if len(withoutActiveSemester.Tasks) != 0 {
		t.Fatalf("no active semester must return no tasks: %+v", withoutActiveSemester.Tasks)
	}
	if err := s.db.Model(&TahunAjaran{}).Where("id = ?", year.ID).Update("is_aktif", false).Error; err != nil {
		t.Fatal(err)
	}
	withoutActiveYear := getOperationalCompliance(t, app, adminToken, "/api/dashboard/kepatuhan-pembelajaran")
	if len(withoutActiveYear.Tasks) != 0 || withoutActiveYear.TahunAjaran.ID != "" {
		t.Fatalf("no active year must return empty monitoring data: %+v", withoutActiveYear)
	}
}

func TestOperationalComplianceRejectsInvalidStatus(t *testing.T) {
	_, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	response, err := makeRequest(app, http.MethodGet, "/api/dashboard/kepatuhan-pembelajaran?status=selesai", adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid status: want 400, got %d", response.StatusCode)
	}
}
