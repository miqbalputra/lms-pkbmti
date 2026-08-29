package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func getOperationalComplianceDetail(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, token, path string) operationalComplianceDetailResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatalf("get operational compliance detail: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get operational compliance detail: want 200, got %d", response.StatusCode)
	}
	var result operationalComplianceDetailResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode operational compliance detail: %v", err)
	}
	return result
}

func TestOperationalComplianceDetailReports(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	today := currentWIBDay()
	meetingDay := latestSaturday(today)

	var year TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		t.Fatal(err)
	}
	year.TanggalMulai = meetingDay.AddDate(0, 0, -7)
	year.TanggalSelesai = today.AddDate(0, 0, 14)
	if err := s.db.Save(&year).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&Semester{}).Where("tahun_ajaran_id = ?", year.ID).Update("is_archived", true).Error; err != nil {
		t.Fatal(err)
	}
	semester := Semester{
		TahunAjaranID: year.ID, NamaSemester: "Detail Kepatuhan", TanggalMulai: meetingDay.AddDate(0, 0, -7),
		TanggalSelesai: today.AddDate(0, 0, 14), IsArchived: false,
	}
	if err := s.db.Create(&semester).Error; err != nil {
		t.Fatal(err)
	}

	var pokjar Pokjar
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Wali Detail Kepatuhan", JenisKelamin: "L"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	headToken := loginRole(t, app, adminToken, "kepala-detail-kepatuhan", "kepala_sekolah", nil)
	guruToken := loginRole(t, app, adminToken, "guru-detail-kepatuhan", "guru", &tutor.ID)

	classes := []Kelas{
		{Jenjang: 21, NamaRombel: "DTA", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}, // missing attendance + journal
		{Jenjang: 22, NamaRombel: "DTB", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}, // partial attendance + journal
		{Jenjang: 23, NamaRombel: "DTC", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}, // moved + complete attendance
		{Jenjang: 24, NamaRombel: "DTD", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}, // holiday + no mapel setup
	}
	for index := range classes {
		if err := s.db.Create(&classes[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	studentsByClass := map[string][]PesertaDidik{}
	for _, class := range classes {
		studentCount := 1
		if class.ID == classes[1].ID || class.ID == classes[2].ID {
			studentCount = 2
		}
		for index := 0; index < studentCount; index++ {
			student := PesertaDidik{
				Nama: "Siswa Detail " + class.NamaRombel + string(rune('A'+index)), JenisKelamin: "L",
				NIS: "DETAIL-" + class.NamaRombel + string(rune('A'+index)), NISN: "DETAIL-" + class.NamaRombel + string(rune('A'+index)),
				KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif",
			}
			if err := s.db.Create(&student).Error; err != nil {
				t.Fatal(err)
			}
			studentsByClass[class.ID] = append(studentsByClass[class.ID], student)
		}
	}

	mapelA := MataPelajaran{NamaMapel: "Mapel Detail A", KodeMapel: "DTA"}
	mapelB := MataPelajaran{NamaMapel: "Mapel Detail B", KodeMapel: "DTB"}
	if err := s.db.Create(&mapelA).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&mapelB).Error; err != nil {
		t.Fatal(err)
	}
	for _, mapping := range []KelasMapel{
		{KelasID: classes[0].ID, MapelID: mapelA.ID},
		{KelasID: classes[1].ID, MapelID: mapelA.ID},
		{KelasID: classes[2].ID, MapelID: mapelA.ID},
		{KelasID: classes[2].ID, MapelID: mapelB.ID},
	} {
		if err := s.db.Create(&mapping).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, assignment := range []PenugasanGuruMapel{
		{TutorID: tutor.ID, KelasID: classes[1].ID, MapelID: mapelA.ID},
		{TutorID: tutor.ID, KelasID: classes[2].ID, MapelID: mapelA.ID},
		{TutorID: tutor.ID, KelasID: classes[2].ID, MapelID: mapelB.ID},
	} {
		if err := s.db.Create(&assignment).Error; err != nil {
			t.Fatal(err)
		}
	}

	partial := Presensi{KelasID: classes[1].ID, Tanggal: meetingDay, StatusPertemuan: "berlangsung", TandaTangan: validPngSignature, BuktiFoto: "[\"foto-partial\"]"}
	if err := s.db.Create(&partial).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: partial.ID, PesertaDidikID: studentsByClass[classes[1].ID][0].ID, StatusKehadiran: "Hadir"}).Error; err != nil {
		t.Fatal(err)
	}

	actualDate := meetingDay.AddDate(0, 0, 1)
	moved := Presensi{KelasID: classes[2].ID, Tanggal: actualDate, TanggalRencana: &meetingDay, StatusPertemuan: "dipindah", TandaTangan: validPngSignature, BuktiFoto: "[\"foto-satu\",\"foto-dua\"]"}
	if err := s.db.Create(&moved).Error; err != nil {
		t.Fatal(err)
	}
	for _, student := range studentsByClass[classes[2].ID] {
		if err := s.db.Create(&PresensiDetail{PresensiID: moved.ID, PesertaDidikID: student.ID, StatusKehadiran: "Hadir"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.db.Create(&Presensi{KelasID: classes[3].ID, Tanggal: meetingDay, StatusPertemuan: "libur"}).Error; err != nil {
		t.Fatal(err)
	}

	for _, journal := range []JurnalMengajar{
		{TutorID: tutor.ID, KelasID: classes[1].ID, MapelID: mapelA.ID, Tanggal: meetingDay, Materi: "Materi DTB", Kegiatan: "Kegiatan DTB"},
		{TutorID: tutor.ID, KelasID: classes[2].ID, MapelID: mapelA.ID, Tanggal: meetingDay, Materi: "Materi DTC", Kegiatan: "Kegiatan DTC"},
	} {
		if err := s.db.Create(&journal).Error; err != nil {
			t.Fatal(err)
		}
	}

	date := wibTimeFormat(meetingDay, "2006-01-02")
	presensiPath := "/api/dashboard/kepatuhan-pembelajaran/detail?jenis=presensi&tahunAjaranId=" + year.ID + "&semesterId=" + semester.ID + "&from=" + date + "&to=" + date
	presensi := getOperationalComplianceDetail(t, app, adminToken, presensiPath)
	if presensi.PeriodStart != date || presensi.PeriodEnd != date {
		t.Fatalf("report range must preserve WIB date: %+v", presensi)
	}
	if presensi.Summary.TotalPertemuan != 4 || presensi.Summary.PertemuanLengkap != 1 || presensi.Summary.PertemuanBelumLengkap != 1 || presensi.Summary.PertemuanBelumDibuat != 1 || presensi.Summary.PertemuanLibur != 1 {
		t.Fatalf("unexpected attendance summary: %+v", presensi.Summary)
	}
	if len(presensi.Presensi) != len(classes) {
		t.Fatalf("want %d attendance classes, got %+v", len(classes), presensi.Presensi)
	}
	attendanceByClass := map[string]operationalAttendanceMeetingDetail{}
	for _, class := range presensi.Presensi {
		if len(class.Meetings) != 1 || class.Meetings[0].Sequence != 1 {
			t.Fatalf("meeting sequence must start at one: %+v", class)
		}
		attendanceByClass[class.ClassID] = class.Meetings[0]
	}
	if row := attendanceByClass[classes[0].ID]; row.Status != "belum_dibuat" || len(row.Issues) != 1 {
		t.Fatalf("missing attendance row invalid: %+v", row)
	}
	if row := attendanceByClass[classes[1].ID]; row.Status != "belum_lengkap" || row.FilledStudents != 1 || row.TotalStudents != 2 || len(row.Issues) != 1 {
		t.Fatalf("partial attendance row invalid: %+v", row)
	}
	if row := attendanceByClass[classes[2].ID]; row.Status != "lengkap" || row.ActualDate != wibTimeFormat(actualDate, "2006-01-02") || row.PhotoCount != 2 || row.MeetingID != moved.ID {
		t.Fatalf("moved complete attendance row invalid: %+v", row)
	}
	if row := attendanceByClass[classes[3].ID]; row.Status != "libur" {
		t.Fatalf("holiday attendance row invalid: %+v", row)
	}
	followUps := operationalAttendanceFollowUpRows(presensi)
	if len(followUps) != 2 {
		t.Fatalf("follow-up export must contain only missing and incomplete attendance: %+v", followUps)
	}
	for _, followUp := range followUps {
		if followUp.ClassLabel == kelasLabel(classes[2]) || followUp.ClassLabel == kelasLabel(classes[3]) {
			t.Fatalf("complete/holiday attendance leaked into follow-up export: %+v", followUps)
		}
	}

	jurnalPath := "/api/dashboard/kepatuhan-pembelajaran/detail?jenis=jurnal&tahunAjaranId=" + year.ID + "&semesterId=" + semester.ID + "&from=" + date + "&to=" + date
	jurnal := getOperationalComplianceDetail(t, app, adminToken, jurnalPath)
	if jurnal.Summary.TotalMapel != 4 || jurnal.Summary.MapelTerisi != 2 || jurnal.Summary.MapelBelumDiisi != 2 || jurnal.Summary.KelasTanpaMapel != 1 {
		t.Fatalf("unexpected journal summary: %+v", jurnal.Summary)
	}
	journalByClass := map[string]operationalJournalClassDetail{}
	for _, class := range jurnal.Jurnal {
		journalByClass[class.ClassID] = class
	}
	if len(journalByClass[classes[0].ID].Subjects) != 1 || journalByClass[classes[0].ID].Subjects[0].Status != "belum_diisi" {
		t.Fatalf("missing journal subject invalid: %+v", journalByClass[classes[0].ID])
	}
	if row := journalByClass[classes[1].ID].Subjects[0]; row.Status != "terisi" || row.EntryCount != 1 || len(row.TutorNames) != 1 || row.Entries[0].Materi != "Materi DTB" {
		t.Fatalf("filled journal subject invalid: %+v", row)
	}
	if len(journalByClass[classes[2].ID].Subjects) != 2 || journalByClass[classes[2].ID].Subjects[1].Status != "belum_diisi" {
		t.Fatalf("class/mapel journal grouping invalid: %+v", journalByClass[classes[2].ID])
	}
	if len(jurnal.Warnings) != 1 || jurnal.Warnings[0].ClassID != classes[3].ID {
		t.Fatalf("unconfigured class warning invalid: %+v", jurnal.Warnings)
	}

	filtered := getOperationalComplianceDetail(t, app, adminToken, jurnalPath+"&kelasId="+classes[1].ID)
	if len(filtered.Jurnal) != 1 || filtered.Jurnal[0].ClassID != classes[1].ID {
		t.Fatalf("class filter leaked journal data: %+v", filtered.Jurnal)
	}
	if head := getOperationalComplianceDetail(t, app, headToken, presensiPath); head.Summary != presensi.Summary {
		t.Fatalf("kepala sekolah detail mismatch: %+v", head.Summary)
	}

	exportPath := "/api/dashboard/kepatuhan-pembelajaran/export?jenis=presensi&format=pdf&tahunAjaranId=" + year.ID + "&semesterId=" + semester.ID + "&from=" + date + "&to=" + date
	exportResponse, err := makeRequest(app, http.MethodGet, exportPath, adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	exportBody, readErr := io.ReadAll(exportResponse.Body)
	exportResponse.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if exportResponse.StatusCode != http.StatusOK {
		t.Fatalf("attendance follow-up export: want 200, got %d", exportResponse.StatusCode)
	}
	if !strings.HasPrefix(exportResponse.Header.Get("Content-Type"), "application/pdf") {
		t.Fatalf("attendance follow-up export content type: %q", exportResponse.Header.Get("Content-Type"))
	}
	contentDisposition := exportResponse.Header.Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "laporan-tindak-lanjut-presensi-") || !strings.Contains(contentDisposition, ".pdf") {
		t.Fatalf("attendance follow-up export filename: %q", contentDisposition)
	}
	if !bytes.HasPrefix(exportBody, []byte("%PDF")) {
		t.Fatalf("attendance follow-up export is not a PDF")
	}

	guruExport, err := makeRequest(app, http.MethodGet, exportPath, guruToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	guruExport.Body.Close()
	if guruExport.StatusCode != http.StatusForbidden {
		t.Fatalf("guru export access: want 403, got %d", guruExport.StatusCode)
	}

	guruResponse, err := makeRequest(app, http.MethodGet, presensiPath, guruToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer guruResponse.Body.Close()
	if guruResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("guru detail access: want 403, got %d", guruResponse.StatusCode)
	}

	invalidPath := "/api/dashboard/kepatuhan-pembelajaran/detail?jenis=presensi&tahunAjaranId=" + year.ID + "&semesterId=" + semester.ID + "&from=2000-01-01&to=" + date
	invalidRange, err := makeRequest(app, http.MethodGet, invalidPath, adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer invalidRange.Body.Close()
	if invalidRange.StatusCode != http.StatusBadRequest {
		t.Fatalf("out-of-semester range: want 400, got %d", invalidRange.StatusCode)
	}
}
