package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestExportUjianResultsHandlesUnscoredParticipant(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	kelas := Kelas{Jenjang: 2, NamaRombel: "A"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	siswa := PesertaDidik{Nama: "Siswa Belum Dinilai", NIS: "EKS-001", NISN: "9000000001", JenisKelamin: "L", KelasID: kelas.ID, Status: "aktif"}
	if err := s.db.Create(&siswa).Error; err != nil {
		t.Fatalf("create student: %v", err)
	}
	ujian := Ujian{KelasID: kelas.ID, Judul: "Ujian Belum Dinilai"}
	if err := s.db.Create(&ujian).Error; err != nil {
		t.Fatalf("create exam: %v", err)
	}
	if err := s.db.Create(&UjianPeserta{UjianID: ujian.ID, PesertaDidikID: siswa.ID, Status: "mulai"}).Error; err != nil {
		t.Fatalf("create unscored participant: %v", err)
	}

	res, err := makeRequest(app, http.MethodGet, "/api/ujian/"+ujian.ID+"/export", token, nil, "")
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected CSV export 200, got %d: %s", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/csv") && !strings.HasPrefix(got, "application/vnd.ms-excel") {
		t.Fatalf("expected a CSV-compatible content type, got %q", got)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Siswa Belum Dinilai") || !strings.Contains(string(body), ",mulai,-") {
		t.Fatalf("CSV must include the unscored participant with a dash score, got %q", body)
	}
}
