package main

import (
	"encoding/json"
	"testing"
	"time"
)

// Test E2E: Dashboard returns upcoming events and unread notification count
func TestE2E_Dashboard_UpcomingEventsAndNotifCount(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Create a calendar event in the future
	eventDate := time.Now().AddDate(0, 0, 7)
	res, _ := makeRequest(app, "POST", "/api/kalender", token, map[string]interface{}{
		"judul":          "Ujian Tengah Semester",
		"tipe":           "ujian",
		"tanggalMulai":   eventDate.Format(time.RFC3339),
		"tanggalSelesai": eventDate.Format(time.RFC3339),
	}, "")
	if res.StatusCode != 201 {
		t.Fatalf("failed to create kalender event: %d", res.StatusCode)
	}

	// Create a notification for the admin user
	var me struct{ ID string }
	resMe, _ := makeRequest(app, "GET", "/api/auth/me", token, nil, "")
	json.NewDecoder(resMe.Body).Decode(&me)
	adminNotif := Notifikasi{
		UserID: me.ID,
		Judul:  "Test Notifikasi",
		Isi:    "Ini test notifikasi",
		Tipe:   "umum",
		IsRead: false,
	}
	s.db.Create(&adminNotif)

	// Fetch dashboard
	resDash, _ := makeRequest(app, "GET", "/api/dashboard", token, nil, "")
	if resDash.StatusCode != 200 {
		t.Fatalf("dashboard returned %d", resDash.StatusCode)
	}
	var dash map[string]interface{}
	json.NewDecoder(resDash.Body).Decode(&dash)

	if _, ok := dash["upcomingEvents"]; !ok {
		t.Error("dashboard missing upcomingEvents field")
	}
	if _, ok := dash["unreadNotif"]; !ok {
		t.Error("dashboard missing unreadNotif field")
	}

	events := dash["upcomingEvents"].([]interface{})
	if len(events) == 0 {
		t.Error("expected at least 1 upcoming event")
	}

	unread := int(dash["unreadNotif"].(float64))
	if unread < 1 {
		t.Errorf("expected unread notif >= 1, got %d", unread)
	}
}

// Test E2E: Kalender CRUD
func TestE2E_KalenderEventCRUD(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Create
	eventDate := time.Now().AddDate(0, 0, 14)
	resCreate, _ := makeRequest(app, "POST", "/api/kalender", token, map[string]interface{}{
		"judul":          "Libur Nasional",
		"tipe":           "libur",
		"tanggalMulai":   eventDate.Format(time.RFC3339),
		"tanggalSelesai": eventDate.Format(time.RFC3339),
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("create kalender returned %d", resCreate.StatusCode)
	}
	var created KalenderEvent
	json.NewDecoder(resCreate.Body).Decode(&created)
	if created.Judul != "Libur Nasional" {
		t.Errorf("expected judul 'Libur Nasional', got '%s'", created.Judul)
	}

	// List
	resList, _ := makeRequest(app, "GET", "/api/kalender", token, nil, "")
	var events []KalenderEvent
	json.NewDecoder(resList.Body).Decode(&events)
	found := false
	for _, e := range events {
		if e.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created event not found in list")
	}

	// Update
	resUpdate, _ := makeRequest(app, "PUT", "/api/kalender/"+created.ID, token, map[string]interface{}{
		"judul": "Libur Nasional Updated",
	}, "")
	if resUpdate.StatusCode != 200 {
		t.Fatalf("update kalender returned %d", resUpdate.StatusCode)
	}

	// Delete
	resDelete, _ := makeRequest(app, "DELETE", "/api/kalender/"+created.ID, token, nil, "")
	if resDelete.StatusCode != 200 && resDelete.StatusCode != 204 {
		t.Fatalf("delete kalender returned %d", resDelete.StatusCode)
	}
}

// Test E2E: Notifikasi unread count and mark all read
func TestE2E_NotifikasiUnreadAndBacaAll(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Unread count
	resCount, _ := makeRequest(app, "GET", "/api/notifikasi/unread-count", token, nil, "")
	if resCount.StatusCode != 200 {
		t.Fatalf("unread count returned %d", resCount.StatusCode)
	}
	var countResp map[string]interface{}
	json.NewDecoder(resCount.Body).Decode(&countResp)
	if _, ok := countResp["count"]; !ok {
		t.Error("unread count missing 'count' field")
	}

	// Mark all as read
	resBacaAll, _ := makeRequest(app, "PUT", "/api/notifikasi/baca-all", token, nil, "")
	if resBacaAll.StatusCode != 200 {
		t.Fatalf("baca all returned %d", resBacaAll.StatusCode)
	}

	// Verify count is 0 after marking all read
	resCount2, _ := makeRequest(app, "GET", "/api/notifikasi/unread-count", token, nil, "")
	var countResp2 map[string]interface{}
	json.NewDecoder(resCount2.Body).Decode(&countResp2)
	if int(countResp2["count"].(float64)) != 0 {
		t.Errorf("expected 0 unread after baca-all, got %v", countResp2["count"])
	}
}

// Test E2E: Dashboard semester filter
func TestE2E_Dashboard_SemesterFilter(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Request dashboard with semester filter (should not error)
	res, _ := makeRequest(app, "GET", "/api/dashboard?semester=1&year=2026", token, nil, "")
	if res.StatusCode != 200 {
		t.Fatalf("dashboard with semester filter returned %d", res.StatusCode)
	}
	var dash map[string]interface{}
	json.NewDecoder(res.Body).Decode(&dash)
	if _, ok := dash["hadir"]; !ok {
		t.Error("dashboard missing 'hadir' field with semester filter")
	}
}
