package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func currentWIBDay() time.Time {
	now := time.Now().In(wibLocation)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wibLocation)
}

func latestSaturday(day time.Time) time.Time {
	delta := (int(day.Weekday()) - int(time.Saturday) + 7) % 7
	return day.AddDate(0, 0, -delta)
}

func getGuruDashboardReminders(t *testing.T, appToken string, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}) guruDashboardReminderResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "/api/dashboard/guru-reminders", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+appToken)
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatalf("get guru reminders: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get guru reminders: want 200, got %d", response.StatusCode)
	}
	var result guruDashboardReminderResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode guru reminders: %v", err)
	}
	return result
}

func TestGuruDashboardRemindersScopeAndCompleteness(t *testing.T) {
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
		TahunAjaranID: year.ID, NamaSemester: "Pengingat", TanggalMulai: meetingDay,
		TanggalSelesai: today.AddDate(0, 0, 30), IsArchived: false,
	}
	if err := s.db.Create(&semester).Error; err != nil {
		t.Fatal(err)
	}

	var pokjar Pokjar
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Guru Pengingat", JenisKelamin: "P"}
	otherTutor := Tutor{Nama: "Guru Lain", JenisKelamin: "L"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherTutor).Error; err != nil {
		t.Fatal(err)
	}
	guruToken := loginRole(t, app, adminToken, "guru-pengingat", "guru", &tutor.ID)
	otherGuruToken := loginRole(t, app, adminToken, "guru-tanpa-kelas", "guru", &otherTutor.ID)

	// A guru with no wali kelas must receive an empty but successful response.
	empty := getGuruDashboardReminders(t, otherGuruToken, app)
	if len(empty.Presensi) != 0 || len(empty.Jurnal) != 0 {
		t.Fatalf("guru without classes must have no reminders: %+v", empty)
	}

	classes := []Kelas{
		{Jenjang: 1, NamaRombel: "RM", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID},
		{Jenjang: 2, NamaRombel: "RP", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID},
		{Jenjang: 3, NamaRombel: "RC", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID},
		{Jenjang: 4, NamaRombel: "RL", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID},
		{Jenjang: 5, NamaRombel: "RO", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &otherTutor.ID},
	}
	for index := range classes {
		if err := s.db.Create(&classes[index]).Error; err != nil {
			t.Fatal(err)
		}
		student := PesertaDidik{
			Nama: "Siswa Pengingat", JenisKelamin: "L", NIS: "REMINDER-" + classes[index].NamaRombel,
			NISN: "REMINDER-" + classes[index].NamaRombel, KelasID: classes[index].ID,
			PokjarID: pokjar.ID, Status: "aktif",
		}
		if err := s.db.Create(&student).Error; err != nil {
			t.Fatal(err)
		}
	}

	// classes[0] has no meeting at all. classes[1] has a meeting but no student
	// checklist. classes[2] is complete; classes[3] is an exempt holiday.
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
	holiday := Presensi{KelasID: classes[3].ID, Tanggal: meetingDay, StatusPertemuan: "libur"}
	if err := s.db.Create(&holiday).Error; err != nil {
		t.Fatal(err)
	}

	// A journal for the complete class suppresses only that class' journal task.
	journal := JurnalMengajar{TutorID: tutor.ID, KelasID: classes[2].ID, Tanggal: today, Materi: "Sudah dicatat", Status: "disetujui"}
	if err := s.db.Create(&journal).Error; err != nil {
		t.Fatal(err)
	}

	result := getGuruDashboardReminders(t, guruToken, app)
	if len(result.Presensi) != 2 {
		t.Fatalf("want 2 attendance reminders (missing + partial), got %+v", result.Presensi)
	}
	attendanceByClass := make(map[string]guruReminderPresensi, len(result.Presensi))
	for _, reminder := range result.Presensi {
		attendanceByClass[reminder.ClassID] = reminder
	}
	if missing := attendanceByClass[classes[0].ID]; missing.Reason != "belum ada data presensi" || missing.MeetingID != "" {
		t.Fatalf("missing attendance reminder is invalid: %+v", missing)
	}
	if incomplete := attendanceByClass[classes[1].ID]; incomplete.MeetingID != partial.ID || incomplete.Reason != "kehadiran 0/1 siswa" {
		t.Fatalf("partial attendance reminder is invalid: %+v", incomplete)
	}
	if _, found := attendanceByClass[classes[2].ID]; found {
		t.Fatal("complete attendance must not be reminded")
	}
	if _, found := attendanceByClass[classes[3].ID]; found {
		t.Fatal("holiday attendance must not be reminded")
	}
	if _, found := attendanceByClass[classes[4].ID]; found {
		t.Fatal("another guru's class leaked into reminders")
	}

	if len(result.Jurnal) != 3 {
		t.Fatalf("want journals for the three unfilled owned classes, got %+v", result.Jurnal)
	}
	journalsByClass := make(map[string]guruReminderJurnal, len(result.Jurnal))
	for _, reminder := range result.Jurnal {
		journalsByClass[reminder.ClassID] = reminder
	}
	if _, found := journalsByClass[classes[2].ID]; found {
		t.Fatal("class with a journal today must not be reminded")
	}
	if _, found := journalsByClass[classes[4].ID]; found {
		t.Fatal("another guru's journal task leaked into reminders")
	}

	// The endpoint remains protected even though the UI only mounts it for guru.
	adminRequest, err := makeRequest(app, http.MethodGet, "/api/dashboard/guru-reminders", adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer adminRequest.Body.Close()
	if adminRequest.StatusCode != http.StatusForbidden {
		t.Fatalf("admin reminder endpoint: want 403, got %d", adminRequest.StatusCode)
	}

	if err := s.db.Model(&Semester{}).Where("id = ?", semester.ID).Update("is_archived", true).Error; err != nil {
		t.Fatal(err)
	}
	withoutActiveSemester := getGuruDashboardReminders(t, guruToken, app)
	if len(withoutActiveSemester.Presensi) != 0 || len(withoutActiveSemester.Jurnal) != 0 {
		t.Fatalf("no active semester must return empty reminders: %+v", withoutActiveSemester)
	}

	if err := s.db.Model(&TahunAjaran{}).Where("id = ?", year.ID).Update("is_aktif", false).Error; err != nil {
		t.Fatal(err)
	}
	withoutActiveYear := getGuruDashboardReminders(t, guruToken, app)
	if len(withoutActiveYear.Presensi) != 0 || len(withoutActiveYear.Jurnal) != 0 {
		t.Fatalf("no active academic year must return empty reminders: %+v", withoutActiveYear)
	}
}
