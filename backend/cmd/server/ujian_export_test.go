package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUjianResultsExportKeepsFixedColumnsHistoricalClassAndXLSX(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)
	classAtAttempt := Kelas{Jenjang: 6, NamaRombel: "UJI"}
	currentClass := Kelas{Jenjang: 7, NamaRombel: "PINDAH"}
	for _, class := range []*Kelas{&classAtAttempt, &currentClass} {
		if err := s.db.Create(class).Error; err != nil {
			t.Fatal(err)
		}
	}
	student := PesertaDidik{Nama: "Siswa Riwayat", NIS: "H-1", NISN: "N-1", KelasID: currentClass.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{KelasID: classAtAttempt.ID, Judul: "Ujian Export", DurasiMenit: 30}
	if err := s.db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	question := BankSoal{Tipe: "pg", Pertanyaan: "Pilih jawaban", Opsi: `["A","B"]`, Kunci: "0", Poin: 1}
	if err := s.db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	item := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, KelasIDSaatUjian: classAtAttempt.ID, Mulai: &started, Status: "selesai"}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	encodedSnapshot, err := json.Marshal(ujianQuestionSnapshot{Tipe: "pg", Pertanyaan: "Pilih jawaban", Opsi: question.Opsi, Kunci: "0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianPesertaSoal{UjianPesertaID: attempt.ID, UjianSoalID: item.ID, SoalID: question.ID, Urutan: 1, Bobot: 1, SnapshotJSON: string(encodedSnapshot)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.ID, Jawaban: "=1+1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&BankSoal{}).Where("id = ?", question.ID).Update("pertanyaan", "Pertanyaan setelah diedit").Error; err != nil {
		t.Fatal(err)
	}
	blankStudent := PesertaDidik{Nama: "Siswa Kosong", NIS: "H-2", NISN: "N-2", KelasID: classAtAttempt.ID, Status: "aktif"}
	if err := s.db.Create(&blankStudent).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianPeserta{UjianID: exam.ID, PesertaDidikID: blankStudent.ID, KelasIDSaatUjian: classAtAttempt.ID, Status: "selesai"}).Error; err != nil {
		t.Fatal(err)
	}

	path := "/api/ujian/" + exam.ID + "/export"
	csvResponse, err := makeRequest(app, http.MethodGet, path, adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if csvResponse.StatusCode != http.StatusOK {
		body := readAndClose(t, csvResponse)
		t.Fatalf("CSV export status=%d body=%s", csvResponse.StatusCode, body)
	}
	csvBody := readAndClose(t, csvResponse)
	if !strings.Contains(csvBody, "Kelas 6UJI") || !strings.Contains(csvBody, "'=1+1") || strings.Count(csvBody, "Siswa Kosong") != 1 || !strings.Contains(csvBody, "Pilih jawaban") || strings.Contains(csvBody, "Pertanyaan setelah diedit") {
		t.Fatalf("CSV should preserve class and question snapshot history, neutralize formulas and include blank answers: %s", csvBody)
	}
	lines := strings.Split(strings.TrimSpace(csvBody), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header plus two participant rows, got: %s", csvBody)
	}
	for _, line := range lines {
		if strings.Count(line, ",") != strings.Count(lines[0], ",") {
			t.Fatalf("every participant row must have a fixed number of columns: %s", csvBody)
		}
	}

	xlsxResponse, err := makeRequest(app, http.MethodGet, path+"?format=xlsx", adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if xlsxResponse.StatusCode != http.StatusOK || !strings.Contains(xlsxResponse.Header.Get("Content-Type"), "spreadsheetml") {
		body := readAndClose(t, xlsxResponse)
		t.Fatalf("XLSX export status=%d body=%s", xlsxResponse.StatusCode, body)
	}
	xlsxBody := readAndClose(t, xlsxResponse)
	if len(xlsxBody) < 2 || xlsxBody[0] != 'P' || xlsxBody[1] != 'K' {
		t.Fatal("XLSX response is not a ZIP spreadsheet file")
	}
}
