package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUjianListAPIReturnsJSONForDashboard(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	kelas := Kelas{Jenjang: 3, NamaRombel: "UJIAN"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	if err := s.db.Create(&Ujian{KelasID: kelas.ID, Judul: "Ujian Dashboard"}).Error; err != nil {
		t.Fatalf("create exam: %v", err)
	}

	res, err := makeRequest(app, http.MethodGet, "/api/ujian", token, nil, "")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("dashboard exam list returned %d", res.StatusCode)
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("dashboard exam list must return JSON, got %q", res.Header.Get("Content-Type"))
	}
	var rows []Ujian
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		t.Fatalf("dashboard exam response must decode as JSON: %v", err)
	}
	if len(rows) != 1 || rows[0].Judul != "Ujian Dashboard" {
		t.Fatalf("unexpected dashboard exam rows: %+v", rows)
	}
}

func TestUjianOnlineMonitorEndpointReturnsParticipants(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	kelas := Kelas{Jenjang: 4, NamaRombel: "MONITOR"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	siswa := PesertaDidik{Nama: "Peserta Monitor", NIS: "MON-1", NISN: "9200000001", JenisKelamin: "L", KelasID: kelas.ID, Status: "aktif"}
	if err := s.db.Create(&siswa).Error; err != nil {
		t.Fatalf("create student: %v", err)
	}
	ujian := Ujian{KelasID: kelas.ID, Judul: "Ujian Monitor"}
	if err := s.db.Create(&ujian).Error; err != nil {
		t.Fatalf("create exam: %v", err)
	}
	if err := s.db.Create(&UjianPeserta{UjianID: ujian.ID, PesertaDidikID: siswa.ID, Status: "mulai", TabSwitch: 1}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	res, err := makeRequest(app, http.MethodGet, "/api/ujian-online/monitor/"+ujian.ID, token, nil, "")
	if err != nil {
		t.Fatalf("monitor request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("monitor endpoint returned %d", res.StatusCode)
	}
	var rows []UjianPeserta
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		t.Fatalf("decode monitor response: %v", err)
	}
	if len(rows) != 1 || rows[0].PesertaDidikID != siswa.ID || rows[0].PesertaDidik.Nama != siswa.Nama || rows[0].TabSwitch != 1 {
		t.Fatalf("monitor must return the matching participant, got %+v", rows)
	}
}
