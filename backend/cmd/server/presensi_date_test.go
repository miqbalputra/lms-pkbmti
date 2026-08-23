package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPresensiOnlyAcceptsSaturdayWIBOnCreateAndUpdate(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	kelas := Kelas{Jenjang: 1, NamaRombel: "SABTU"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	saturday := time.Date(2026, time.August, 8, 8, 0, 0, 0, time.UTC)
	sunday := saturday.AddDate(0, 0, 1)

	resInvalidCreate, _ := makeRequest(app, http.MethodPost, "/api/presensi", token, map[string]interface{}{
		"kelasId": kelas.ID, "tanggal": sunday.Format(time.RFC3339), "tandaTangan": validPngSignature,
	}, "")
	if resInvalidCreate.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-Saturday attendance create must be rejected, got %d", resInvalidCreate.StatusCode)
	}
	var count int64
	if err := s.db.Model(&Presensi{}).Where("kelas_id = ?", kelas.ID).Count(&count).Error; err != nil {
		t.Fatalf("count meetings after rejected create: %v", err)
	}
	if count != 0 {
		t.Fatalf("rejected non-Saturday create must not persist a meeting, got %d rows", count)
	}

	resCreate, _ := makeRequest(app, http.MethodPost, "/api/presensi", token, map[string]interface{}{
		"kelasId": kelas.ID, "tanggal": saturday.Format(time.RFC3339), "tandaTangan": validPngSignature,
	}, "")
	if resCreate.StatusCode != http.StatusCreated {
		t.Fatalf("Saturday attendance create returned %d", resCreate.StatusCode)
	}
	var meeting Presensi
	if err := json.NewDecoder(resCreate.Body).Decode(&meeting); err != nil {
		t.Fatalf("decode meeting: %v", err)
	}

	resInvalidUpdate, _ := makeRequest(app, http.MethodPut, "/api/presensi/"+meeting.ID, token, map[string]interface{}{
		"tanggal": sunday.Format(time.RFC3339), "tandaTangan": validPngSignature,
	}, "")
	if resInvalidUpdate.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-Saturday attendance update must be rejected, got %d", resInvalidUpdate.StatusCode)
	}
	var persisted Presensi
	if err := s.db.First(&persisted, "id = ?", meeting.ID).Error; err != nil {
		t.Fatalf("load persisted meeting: %v", err)
	}
	if !sameDay(persisted.Tanggal, saturday.In(wibLocation)) {
		t.Fatalf("rejected update must preserve original Saturday, got %s", persisted.Tanggal)
	}
}

func TestPresensiExportKeepsSaturdayCalendarDateInWIB(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	kelas := Kelas{Jenjang: 1, NamaRombel: "EXPORT-WIB"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	siswa := PesertaDidik{Nama: "Siswa Export WIB", NIS: "WIB-1", NISN: "WIB-1", KelasID: kelas.ID, Status: "aktif"}
	if err := s.db.Create(&siswa).Error; err != nil {
		t.Fatalf("create student: %v", err)
	}

	// This instant is Saturday 00:00 WIB, but is commonly returned by a
	// database driver as Friday 17:00 UTC.
	meeting := Presensi{
		KelasID:         kelas.ID,
		Tanggal:         time.Date(2026, time.August, 7, 17, 0, 0, 0, time.UTC),
		Semester:        "Ganjil",
		StatusPertemuan: "berlangsung",
	}
	if err := s.db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: siswa.ID, StatusKehadiran: "Hadir"}).Error; err != nil {
		t.Fatalf("create attendance detail: %v", err)
	}

	res, err := makeRequest(app, http.MethodGet, "/api/presensi/export?kelasId="+kelas.ID, token, nil, "")
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("export returned %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if !strings.Contains(string(body), "2026-08-08") {
		t.Fatalf("export must contain Saturday WIB date, got %s", body)
	}
	if strings.Contains(string(body), "2026-08-07") {
		t.Fatalf("export must not contain previous UTC calendar date, got %s", body)
	}
}
