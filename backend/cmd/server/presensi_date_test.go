package main

import (
	"encoding/json"
	"net/http"
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
