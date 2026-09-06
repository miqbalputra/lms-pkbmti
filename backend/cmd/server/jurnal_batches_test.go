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
)

func journalBatchRequest(method, path, token string, values url.Values) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(values.Encode()))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func setupJournalBatchFixture(t *testing.T) (*Server, interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, string, string, Kelas, MataPelajaran, MataPelajaran, time.Time) {
	t.Helper()
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	day := latestSaturday(currentWIBDay())
	var year TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		t.Fatal(err)
	}
	year.TanggalMulai, year.TanggalSelesai = day.AddDate(0, 0, -14), day.AddDate(0, 0, 14)
	if err := s.db.Save(&year).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&Semester{}).Where("tahun_ajaran_id = ?", year.ID).Update("is_archived", true).Error; err != nil {
		t.Fatal(err)
	}
	semester := Semester{TahunAjaranID: year.ID, NamaSemester: "Jurnal Batch", TanggalMulai: year.TanggalMulai, TanggalSelesai: year.TanggalSelesai}
	if err := s.db.Create(&semester).Error; err != nil {
		t.Fatal(err)
	}
	var pokjar Pokjar
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Tutor Jurnal Batch", JenisKelamin: "P"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	guruToken := loginRole(t, app, adminToken, "guru-jurnal-batch", "guru", &tutor.ID)
	kelas := Kelas{Jenjang: 9, NamaRombel: "JB", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	mapelA := MataPelajaran{NamaMapel: "Matematika Batch", KodeMapel: "JBA", IsActive: true}
	mapelB := MataPelajaran{NamaMapel: "IPA Batch", KodeMapel: "JBB", IsActive: true}
	if err := s.db.Create(&mapelA).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&mapelB).Error; err != nil {
		t.Fatal(err)
	}
	for _, mapelID := range []string{mapelA.ID, mapelB.ID} {
		if err := s.db.Create(&PenugasanGuruMapel{Base: Base{CreatedAt: day.Add(-time.Hour)}, TutorID: tutor.ID, KelasID: kelas.ID, MapelID: mapelID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	student := PesertaDidik{Nama: "Siswa Tidak Hadir", JenisKelamin: "L", NIS: "JB-001", NISN: "JB-001", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	return s, app, adminToken, guruToken, kelas, mapelA, mapelB, day
}

func TestJournalBatchSheetAttendanceAndExports(t *testing.T) {
	s, app, _, guruToken, kelas, mapelA, mapelB, day := setupJournalBatchFixture(t)
	lines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapelA.ID, Materi: "Bilangan bulat"}, {JamKe: 2, MapelID: mapelB.ID, Materi: "Makhluk hidup"}})
	create := journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{
		"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(lines)},
	})
	response, err := app.Test(create, -1)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		response.Body.Close()
		t.Fatalf("create batch: want 201, got %d", response.StatusCode)
	}
	var batch JurnalBatch
	if err := json.NewDecoder(response.Body).Decode(&batch); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if batch.ID == "" {
		t.Fatal("created batch has no ID")
	}
	updatedLines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapelA.ID, Materi: "Bilangan bulat lanjutan"}, {JamKe: 2, MapelID: mapelB.ID, Materi: "Makhluk hidup"}})
	updated, err := app.Test(journalBatchRequest(http.MethodPut, "/api/jurnal/batches/"+batch.ID, guruToken, url.Values{
		"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "lines": {string(updatedLines)},
	}), -1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.StatusCode != http.StatusOK {
		updated.Body.Close()
		t.Fatalf("update batch: want 200, got %d", updated.StatusCode)
	}
	updated.Body.Close()

	getSheet := func() journalSheetResponse {
		request := journalBatchRequest(http.MethodGet, "/api/jurnal/sheet?kelasId="+url.QueryEscape(kelas.ID)+"&tanggal="+wibTimeFormat(day, "2006-01-02"), guruToken, nil)
		result, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Body.Close()
		if result.StatusCode != http.StatusOK {
			t.Fatalf("sheet: want 200, got %d", result.StatusCode)
		}
		var sheet journalSheetResponse
		if err := json.NewDecoder(result.Body).Decode(&sheet); err != nil {
			t.Fatal(err)
		}
		return sheet
	}
	sheet := getSheet()
	if sheet.AttendanceStatus != "belum_ada" || len(sheet.Lines) != 2 || sheet.Lines[0].JamKe != 1 || sheet.Lines[0].Materi != "Bilangan bulat lanjutan" || sheet.Lines[1].JamKe != 2 {
		t.Fatalf("unexpected initial sheet: %+v", sheet)
	}

	meeting := Presensi{KelasID: kelas.ID, Tanggal: day, StatusPertemuan: "berlangsung", TandaTangan: validPngSignature, BuktiFoto: "[\"foto\"]"}
	if err := s.db.Create(&meeting).Error; err != nil {
		t.Fatal(err)
	}
	var student PesertaDidik
	if err := s.db.Where("kelas_id = ?", kelas.ID).First(&student).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: student.ID, StatusKehadiran: "Izin"}).Error; err != nil {
		t.Fatal(err)
	}
	sheet = getSheet()
	if sheet.AttendanceStatus != "terisi" || len(sheet.AbsentStudents) != 1 || sheet.AbsentStudents[0].Nama != student.Nama || sheet.AbsentStudents[0].StatusKehadiran != "Izin" {
		t.Fatalf("attendance was not synchronized: %+v", sheet)
	}

	duplicate, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapelA.ID, Materi: "Bentrok"}})
	conflict, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(duplicate)}}), -1)
	if err != nil {
		t.Fatal(err)
	}
	if conflict.StatusCode != http.StatusConflict {
		conflict.Body.Close()
		t.Fatalf("duplicate period: want 409, got %d", conflict.StatusCode)
	}
	conflict.Body.Close()

	for _, format := range []struct {
		name  string
		magic []byte
	}{{"pdf", []byte("%PDF")}, {"docx", []byte("PK")}, {"jpg", []byte{0xff, 0xd8, 0xff}}} {
		request := journalBatchRequest(http.MethodGet, "/api/jurnal/export?kelasId="+url.QueryEscape(kelas.ID)+"&tanggal="+wibTimeFormat(day, "2006-01-02")+"&format="+format.name, guruToken, nil)
		result, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(result.Body)
		result.Body.Close()
		if result.StatusCode != http.StatusOK || len(body) < len(format.magic) || !bytes.Equal(body[:len(format.magic)], format.magic) {
			t.Fatalf("export %s invalid: status=%d body=%q", format.name, result.StatusCode, body)
		}
	}
}

func TestJournalBatchRejectsInvalidDateAndUnauthorizedSubject(t *testing.T) {
	s, app, _, guruToken, kelas, mapelA, _, day := setupJournalBatchFixture(t)
	lines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapelA.ID, Materi: "Materi"}})
	weekday := day.AddDate(0, 0, -1)
	invalidDate, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(weekday, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(lines)}}), -1)
	if err != nil {
		t.Fatal(err)
	}
	if invalidDate.StatusCode != http.StatusBadRequest {
		invalidDate.Body.Close()
		t.Fatalf("weekday journal: want 400, got %d", invalidDate.StatusCode)
	}
	invalidDate.Body.Close()

	mapelTanpaTugas := MataPelajaran{NamaMapel: "Mapel Tanpa Penugasan", KodeMapel: "JBC", IsActive: true}
	if err := s.db.Create(&mapelTanpaTugas).Error; err != nil {
		t.Fatal(err)
	}
	unauthorizedLines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapelTanpaTugas.ID, Materi: "Tidak boleh"}})
	unauthorized, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{
		"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(unauthorizedLines)},
	}), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusForbidden {
		t.Fatalf("unassigned subject: want 403, got %d", unauthorized.StatusCode)
	}
}
