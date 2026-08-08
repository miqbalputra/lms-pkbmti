package main

import (
	"encoding/json"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// Tier 2: Boundary & Corner Cases (25+ boundary scenarios)

// 1. Auth Boundaries
func TestE2E_Tier2_Auth_Boundaries(t *testing.T) {
	_, app := setupE2EServer(t)

	// Boundary 1: Empty credentials
	resEmpty, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "",
		"password": "",
	}, "")
	if resEmpty.StatusCode != 400 {
		t.Errorf("expected 400 for empty login credentials, got %d", resEmpty.StatusCode)
	}

	// Boundary 2: Missing password
	resNoPass, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login": "admin",
	}, "")
	if resNoPass.StatusCode != 400 {
		t.Errorf("expected 400 for missing password, got %d", resNoPass.StatusCode)
	}

	// Boundary 3: Missing authorization header on protected route
	resNoAuth, _ := makeRequest(app, "GET", "/api/auth/me", "", nil, "")
	if resNoAuth.StatusCode != 401 {
		t.Errorf("expected 401 for missing auth header, got %d", resNoAuth.StatusCode)
	}

	// Boundary 4: Invalid/garbage token
	resGarbage, _ := makeRequest(app, "GET", "/api/auth/me", "invalid-token-string", nil, "")
	if resGarbage.StatusCode != 401 {
		t.Errorf("expected 401 for garbage Bearer token, got %d", resGarbage.StatusCode)
	}

	// Boundary 5: Refresh with empty cookie
	resNoCookie, _ := makeRequest(app, "POST", "/api/auth/refresh", "", nil, "")
	if resNoCookie.StatusCode != 401 {
		t.Errorf("expected 401 for refresh with no cookie, got %d", resNoCookie.StatusCode)
	}
}

// 2. Class Boundaries
func TestE2E_Tier2_Class_Boundaries(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	// Boundary 6: Jenjang < 1 (Jenjang 0)
	resJ0, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       0,
		"namaRombel":    "A",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resJ0.StatusCode != 400 {
		t.Errorf("expected 400 for jenjang 0, got %d", resJ0.StatusCode)
	}

	// Boundary 7: Jenjang > 6 (Jenjang 7)
	resJ7, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       7,
		"namaRombel":    "A",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resJ7.StatusCode != 400 {
		t.Errorf("expected 400 for jenjang 7, got %d", resJ7.StatusCode)
	}

	// Boundary 8: Duplicate class identity
	resValid, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       1,
		"namaRombel":    "X",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resValid.StatusCode != 201 {
		t.Fatalf("expected 201 for valid class, got %d", resValid.StatusCode)
	}

	resDup, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       1,
		"namaRombel":    "X",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resDup.StatusCode != 400 {
		t.Errorf("expected 400 for duplicate class identity, got %d", resDup.StatusCode)
	}

	// Boundary 9: Non-existent class ID lookup
	resNotFound, _ := makeRequest(app, "GET", "/api/tutor/non-existent-uuid", token, nil, "")
	if resNotFound.StatusCode != 404 {
		t.Errorf("expected 404 for missing tutor ID, got %d", resNotFound.StatusCode)
	}
}

// 3. User & Lockout Boundaries
func TestE2E_Tier2_User_Boundaries(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Boundary 10: Password < 8 characters
	resShortPass, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "short_user",
		"email":    "short@example.com",
		"password": "123",
		"role":     "admin",
	}, "")
	if resShortPass.StatusCode != 400 {
		t.Errorf("expected 400 for password < 8 chars, got %d", resShortPass.StatusCode)
	}

	// Boundary 11: Invalid role string
	resBadRole, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "bad_role_user",
		"email":    "badrole@example.com",
		"password": "Password123",
		"role":     "superadmin",
	}, "")
	if resBadRole.StatusCode != 400 {
		t.Errorf("expected 400 for invalid role superadmin, got %d", resBadRole.StatusCode)
	}

	// Boundary 12: Guru user without tutor link
	resGuruNoTutor, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "guru_notutor",
		"email":    "gurunotutor@example.com",
		"password": "Password123",
		"role":     "guru",
	}, "")
	if resGuruNoTutor.StatusCode != 400 {
		t.Errorf("expected 400 for guru without tutorId, got %d", resGuruNoTutor.StatusCode)
	}

	// Boundary 13: Duplicate username creation
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	existing := User{Username: "dupuser", Email: "dup@example.com", PasswordHash: string(hash), Role: "admin", IsActive: true}
	s.db.Create(&existing)

	resDupUser, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "dupuser",
		"email":    "another@example.com",
		"password": "Password123",
		"role":     "admin",
	}, "")
	if resDupUser.StatusCode != 400 {
		t.Errorf("expected 400 for duplicate username, got %d", resDupUser.StatusCode)
	}

	// Boundary 14: Account Lockout after 5 failed login attempts
	lockoutUser := User{Username: "lockme", Email: "lockme@example.com", PasswordHash: string(hash), Role: "admin", IsActive: true}
	s.db.Create(&lockoutUser)

	for i := 0; i < 5; i++ {
		makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
			"login":    "lockme",
			"password": "WrongPassword",
		}, "")
	}

	// 6th attempt with CORRECT password should be rejected due to account lockout
	resLocked, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "lockme",
		"password": "Password123",
	}, "")
	if resLocked.StatusCode != 403 {
		t.Errorf("expected 403 for locked account attempt, got %d", resLocked.StatusCode)
	}
}

// 4. Attendance Signature Boundaries
func TestE2E_Tier2_Attendance_Boundaries(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	class := Kelas{Jenjang: 1, NamaRombel: "B", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	// Boundary 15: Empty signature
	resEmptySig, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":     class.ID,
		"tandaTangan": "",
	}, "")
	if resEmptySig.StatusCode != 400 {
		t.Errorf("expected 400 for empty signature, got %d", resEmptySig.StatusCode)
	}

	// Boundary 16: Non-PNG signature (JPEG)
	resJPEG, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":     class.ID,
		"tandaTangan": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEASABIAAD",
	}, "")
	if resJPEG.StatusCode != 400 {
		t.Errorf("expected 400 for JPEG signature header, got %d", resJPEG.StatusCode)
	}

	// Boundary 17: Invalid PNG magic header bytes
	resBadPNG, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":     class.ID,
		"tandaTangan": "data:image/png;base64,aW52YWxpZGJ5dGVz",
	}, "")
	if resBadPNG.StatusCode != 400 {
		t.Errorf("expected 400 for corrupted PNG signature, got %d", resBadPNG.StatusCode)
	}

	// Boundary 18: Presensi Non-existent ID lookup
	resNoMeeting, _ := makeRequest(app, "GET", "/api/presensi/non-existent-id/pdf", token, nil, "")
	if resNoMeeting.StatusCode != 404 {
		t.Errorf("expected 404 for missing presensi PDF, got %d", resNoMeeting.StatusCode)
	}
}

// 5. Role Restriction & IDOR Boundaries
func TestE2E_Tier2_Role_Boundaries(t *testing.T) {
	s, app := setupE2EServer(t)

	// Create Kepala Sekolah User & Login
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	kepsek := User{Username: "kepsek_test", Email: "kepsek_test@example.com", PasswordHash: string(hash), Role: "kepala_sekolah", IsActive: true}
	s.db.Create(&kepsek)

	resLogin, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "kepsek_test",
		"password": "Password123",
	}, "")
	var loginRes map[string]interface{}
	json.NewDecoder(resLogin.Body).Decode(&loginRes)
	kepsekToken := loginRes["accessToken"].(string)

	// Boundary 19: Kepala Sekolah POST write attempt -> 403 Forbidden
	resPost, _ := makeRequest(app, "POST", "/api/tutor", kepsekToken, map[string]interface{}{
		"nama": "Unauthorized Tutor",
	}, "")
	if resPost.StatusCode != 403 {
		t.Errorf("expected 403 for kepsek POST, got %d", resPost.StatusCode)
	}

	// Boundary 20: Kepala Sekolah PUT write attempt -> 403 Forbidden
	resPut, _ := makeRequest(app, "PUT", "/api/pokjar/123", kepsekToken, map[string]interface{}{
		"namaPokjar": "Hacked Pokjar",
	}, "")
	if resPut.StatusCode != 403 {
		t.Errorf("expected 403 for kepsek PUT, got %d", resPut.StatusCode)
	}

	// Boundary 21: Kepala Sekolah DELETE write attempt -> 403 Forbidden
	resDel, _ := makeRequest(app, "DELETE", "/api/users/123", kepsekToken, nil, "")
	if resDel.StatusCode != 403 {
		t.Errorf("expected 403 for kepsek DELETE, got %d", resDel.StatusCode)
	}

	// Boundary 22: Guru IDOR cross-class attendance management attempt
	tutor1 := Tutor{Nama: "Tutor 1", JenisKelamin: "L"}
	tutor2 := Tutor{Nama: "Tutor 2", JenisKelamin: "P"}
	s.db.Create(&tutor1)
	s.db.Create(&tutor2)

	guruUser := User{Username: "guru_idor_user", Email: "guru_idor@example.com", PasswordHash: string(hash), Role: "guru", TutorID: &tutor1.ID, IsActive: true}
	s.db.Create(&guruUser)

	resGuruLogin, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "guru_idor_user",
		"password": "Password123",
	}, "")
	var guruLoginRes map[string]interface{}
	json.NewDecoder(resGuruLogin.Body).Decode(&guruLoginRes)
	guruToken := guruLoginRes["accessToken"].(string)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	otherClass := Kelas{Jenjang: 5, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor2.ID}
	s.db.Create(&otherClass)

	// Guru 1 attempts to create presensi for Guru 2's class
	validSig := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	resIDOR, _ := makeRequest(app, "POST", "/api/presensi", guruToken, map[string]interface{}{
		"kelasId":     otherClass.ID,
		"tandaTangan": validSig,
	}, "")
	if resIDOR.StatusCode != 403 {
		t.Errorf("expected 403 for guru cross-class IDOR, got %d", resIDOR.StatusCode)
	}
	foreignMeeting := Presensi{KelasID: otherClass.ID, Tanggal: testSaturday(), TandaTangan: validSig, StatusPertemuan: "berlangsung"}
	s.db.Create(&foreignMeeting)
	resKepsekDetail, _ := makeRequest(app, "GET", "/api/presensi/"+foreignMeeting.ID, kepsekToken, nil, "")
	if resKepsekDetail.StatusCode != 200 {
		t.Errorf("expected kepala sekolah to read presensi detail, got %d", resKepsekDetail.StatusCode)
	}
	resKepsekDelete, _ := makeRequest(app, "DELETE", "/api/presensi/"+foreignMeeting.ID, kepsekToken, nil, "")
	if resKepsekDelete.StatusCode != 403 {
		t.Errorf("expected kepala sekolah presensi delete to stay read-only, got %d", resKepsekDelete.StatusCode)
	}
	resDeleteIDOR, _ := makeRequest(app, "DELETE", "/api/presensi/"+foreignMeeting.ID, guruToken, nil, "")
	if resDeleteIDOR.StatusCode != 403 {
		t.Errorf("expected 403 for guru cross-class presensi delete, got %d", resDeleteIDOR.StatusCode)
	}
}

// 6. Promotion Boundaries
func TestE2E_Tier2_Promotion_Boundaries(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Boundary 23: Promotion missing target academic year
	resNoYear, _ := makeRequest(app, "POST", "/api/kenaikan-kelas", token, map[string]interface{}{
		"students": []map[string]interface{}{{"id": "123"}},
	}, "")
	if resNoYear.StatusCode != 400 {
		t.Errorf("expected 400 for missing target year, got %d", resNoYear.StatusCode)
	}

	// Boundary 24: Promotion with invalid status string
	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	targetYear := TahunAjaran{NamaTahunAjaran: "2099/2100"}
	s.db.Create(&targetYear)
	class := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)
	student := PesertaDidik{Nama: "Boundary Student", JenisKelamin: "L", NIS: "B-1", NISN: "B-1", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student)

	resBadStatus, _ := makeRequest(app, "POST", "/api/kenaikan-kelas", token, map[string]interface{}{
		"targetTahunAjaranId": targetYear.ID,
		"students": []map[string]interface{}{
			{
				"id":     student.ID,
				"status": "invalid_promotion_status",
			},
		},
	}, "")
	if resBadStatus.StatusCode != 400 {
		t.Errorf("expected 400 for invalid promotion status, got %d", resBadStatus.StatusCode)
	}
}

// 7. Settings Boundaries
func TestE2E_Tier2_Settings_Boundaries(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Boundary 25: Schedule settings with invalid day
	resBadDay, _ := makeRequest(app, "PUT", "/api/settings/jadwal", token, map[string]interface{}{
		"hariDefault": "Someday",
		"jamGenerate": "00:05",
		"zonaWaktu":   "Asia/Jakarta",
	}, "")
	if resBadDay.StatusCode != 400 {
		t.Errorf("expected 400 for invalid default day, got %d", resBadDay.StatusCode)
	}

	// Boundary 26: Schedule settings with invalid HH:MM format
	resBadTime, _ := makeRequest(app, "PUT", "/api/settings/jadwal", token, map[string]interface{}{
		"hariDefault": "Sabtu",
		"jamGenerate": "25:70",
		"zonaWaktu":   "Asia/Jakarta",
	}, "")
	if resBadTime.StatusCode != 400 {
		t.Errorf("expected 400 for invalid jamGenerate time format, got %d", resBadTime.StatusCode)
	}
}
