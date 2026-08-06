package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

// Tier 4: Real-World Application Scenarios (10 full E2E workflows)

// Scenario 1: Complete Academic Year Onboarding & Class Setup
func TestE2E_Tier4_Scenario1_AcademicYearOnboarding(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Step 1: Create Academic Year 2026/2027
	resYear, _ := makeRequest(app, "POST", "/api/tahun-ajaran", token, map[string]interface{}{
		"namaTahunAjaran": "2026/2027-Scenario1",
		"tanggalMulai":    time.Now(),
		"tanggalSelesai":  time.Now().AddDate(1, 0, 0),
		"isAktif":         true,
	}, "")
	if resYear.StatusCode != 201 {
		t.Fatalf("Step 1 failed: year creation returned %d", resYear.StatusCode)
	}
	var year TahunAjaran
	json.NewDecoder(resYear.Body).Decode(&year)

	// Step 2: Create Pokjar Branch
	resPokjar, _ := makeRequest(app, "POST", "/api/pokjar", token, map[string]interface{}{
		"namaPokjar": "Pokjar Scenario Branch",
		"tipe":       "binaan",
		"alamat":     "Jl. Scenario No. 1",
	}, "")
	if resPokjar.StatusCode != 201 {
		t.Fatalf("Step 2 failed: pokjar creation returned %d", resPokjar.StatusCode)
	}
	var pokjar Pokjar
	json.NewDecoder(resPokjar.Body).Decode(&pokjar)

	// Step 3: Create 2 Tutors
	resT1, _ := makeRequest(app, "POST", "/api/tutor", token, map[string]interface{}{"nama": "Tutor Alpha", "jenisKelamin": "L"}, "")
	resT2, _ := makeRequest(app, "POST", "/api/tutor", token, map[string]interface{}{"nama": "Tutor Beta", "jenisKelamin": "P"}, "")
	var t1, t2 Tutor
	json.NewDecoder(resT1.Body).Decode(&t1)
	json.NewDecoder(resT2.Body).Decode(&t2)

	// Step 4: Create Class 1A with Tutor Alpha as Wali Kelas
	resClass, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       1,
		"namaRombel":    "A",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
		"waliKelasId":   t1.ID,
	}, "")
	if resClass.StatusCode != 201 {
		t.Fatalf("Step 4 failed: class creation returned %d", resClass.StatusCode)
	}
	var class Kelas
	json.NewDecoder(resClass.Body).Decode(&class)

	// Step 5: Create Subject & Attach to Class
	resMapel, _ := makeRequest(app, "POST", "/api/mapel", token, map[string]interface{}{"namaMapel": "IPA Scenario", "kodeMapel": "IPAS", "isActive": true}, "")
	var mapel MataPelajaran
	json.NewDecoder(resMapel.Body).Decode(&mapel)

	makeRequest(app, "PUT", "/api/kelas/"+class.ID+"/mapel", token, map[string]interface{}{"mapelIds": []string{mapel.ID}}, "")

	// Step 6: Assign Tutor Beta as Subject Teacher for Class 1A
	resAssign, _ := makeRequest(app, "POST", "/api/penugasan", token, map[string]interface{}{
		"tutorId": t2.ID,
		"kelasId": class.ID,
		"mapelId": mapel.ID,
	}, "")
	if resAssign.StatusCode != 201 {
		t.Fatalf("Step 6 failed: penugasan creation returned %d", resAssign.StatusCode)
	}

	// Verification
	var count int64
	s.db.Model(&PenugasanGuruMapel{}).Where("kelas_id = ?", class.ID).Count(&count)
	if count != 1 {
		t.Errorf("Scenario 1 verification failed: expected 1 penugasan, got %d", count)
	}
}

// Scenario 2: Student Enrollment via Single Entry & Bulk Excel Import
func TestE2E_Tier4_Scenario2_StudentEnrollmentAndImport(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	parent := OrangTua{NamaIbu: "Ibu Scenario 2"}
	s.db.Create(&parent)
	class := Kelas{Jenjang: 1, NamaRombel: "S2", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	// Step 1: Single Student Enrollment
	resSingle, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama":         "Siswa Single",
		"jenisKelamin": "L",
		"nis":          "NIS-S2-0",
		"nisn":         "NISN-S2-0",
		"nik":          "NIK-S2-0",
		"kelasId":      class.ID,
		"pokjarId":     pokjar.ID,
		"orangTuaId":   parent.ID,
	}, "")
	if resSingle.StatusCode != 201 {
		t.Fatalf("Step 1 failed: single student enrollment returned %d", resSingle.StatusCode)
	}

	// Step 2: Generate Valid Excel File for Bulk Import
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	xlsx.SetSheetRow(sheet, "A1", &[]string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"})
	xlsx.SetSheetRow(sheet, "A2", &[]string{"Siswa Import 1", "L", "NIS-S2-1", "NISN-S2-1", "NIK-S2-1", class.ID, pokjar.ID, parent.ID})
	xlsx.SetSheetRow(sheet, "A3", &[]string{"Siswa Import 2", "P", "NIS-S2-2", "NISN-S2-2", "NIK-S2-2", class.ID, pokjar.ID, parent.ID})

	var content bytes.Buffer
	xlsx.Write(&content)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "siswa_bulk.xlsx")
	part.Write(content.Bytes())
	writer.Close()

	// Step 3: Execute Bulk Import Request
	reqImport := httptest.NewRequest(http.MethodPost, "/api/peserta-didik/import", &body)
	reqImport.Header.Set("Content-Type", writer.FormDataContentType())
	reqImport.Header.Set("Authorization", "Bearer "+token)

	resImport, err := app.Test(reqImport, -1)
	if err != nil || resImport.StatusCode != 201 {
		t.Fatalf("Step 3 failed: excel bulk import returned %d, err %v", resImport.StatusCode, err)
	}
	defer resImport.Body.Close()

	// Verification: Total 3 students in class
	var totalStudents int64
	s.db.Model(&PesertaDidik{}).Where("kelas_id = ?", class.ID).Count(&totalStudents)
	if totalStudents != 3 {
		t.Errorf("Scenario 2 verification failed: expected 3 total students, got %d", totalStudents)
	}
}

// Scenario 3: Weekly Attendance Lifecycle & Signature Capture
func TestE2E_Tier4_Scenario3_WeeklyAttendanceLifecycle(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	tutor := Tutor{Nama: "Wali Lifecycle", JenisKelamin: "L"}
	s.db.Create(&tutor)
	class := Kelas{Jenjang: 2, NamaRombel: "L3", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}
	s.db.Create(&class)

	student1 := PesertaDidik{Nama: "Siswa L1", JenisKelamin: "L", NIS: "L-1", NISN: "L-1", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"}
	student2 := PesertaDidik{Nama: "Siswa L2", JenisKelamin: "P", NIS: "L-2", NISN: "L-2", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student1)
	s.db.Create(&student2)

	validSig := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	// Step 1: Create Saturday Meeting
	resMeeting, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":         class.ID,
		"tanggal":         testSaturday().Format(time.RFC3339),
		"statusPertemuan": "berlangsung",
		"tandaTangan":     validSig,
	}, "")
	if resMeeting.StatusCode != 201 {
		t.Fatalf("Step 1 failed: attendance meeting creation returned %d", resMeeting.StatusCode)
	}
	var meeting Presensi
	json.NewDecoder(resMeeting.Body).Decode(&meeting)

	// Step 2: Submit Student Attendance Checklist
	resDetails, _ := makeRequest(app, "POST", "/api/presensi/"+meeting.ID+"/details", token, []map[string]interface{}{
		{"pesertaDidikId": student1.ID, "statusKehadiran": "Hadir", "catatan": "Hadir tepat waktu"},
		{"pesertaDidikId": student2.ID, "statusKehadiran": "Sakit", "catatan": "Surat dokter"},
	}, "")
	if resDetails.StatusCode != 204 {
		t.Fatalf("Step 2 failed: attendance checklist submission returned %d", resDetails.StatusCode)
	}

	// Step 3: Reschedule Meeting Date (status -> "dipindah")
	shiftedDate := meeting.Tanggal.AddDate(0, 0, 7)
	meeting.Tanggal = shiftedDate
	meeting.TanggalRencana = &meeting.Tanggal
	resReschedule, _ := makeRequest(app, "PUT", "/api/presensi/"+meeting.ID, token, meeting, "")
	if resReschedule.StatusCode != 200 {
		t.Fatalf("Step 3 failed: attendance rescheduling returned %d", resReschedule.StatusCode)
	}

	// Step 4: Export PDF
	resPDF, _ := makeRequest(app, "GET", "/api/presensi/"+meeting.ID+"/pdf", token, nil, "")
	if resPDF.StatusCode != 200 {
		t.Errorf("Step 4 failed: PDF export returned %d", resPDF.StatusCode)
	}
}

// Scenario 4: Year-End Student Promotion & Graduation Workflow
func TestE2E_Tier4_Scenario4_YearEndPromotionWizard(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year1 TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year1)

	// Setup Year 2
	year2 := TahunAjaran{NamaTahunAjaran: "2027/2028-Promo", TanggalMulai: time.Now().AddDate(1, 0, 0), TanggalSelesai: time.Now().AddDate(2, 0, 0)}
	s.db.Create(&year2)

	srcClass := Kelas{Jenjang: 1, NamaRombel: "P1", PokjarID: pokjar.ID, TahunAjaranID: year1.ID}
	targetClass := Kelas{Jenjang: 2, NamaRombel: "P2", PokjarID: pokjar.ID, TahunAjaranID: year2.ID}
	targetRetainedClass := Kelas{Jenjang: 1, NamaRombel: "P1-Year2", PokjarID: pokjar.ID, TahunAjaranID: year2.ID}
	s.db.Create(&srcClass)
	s.db.Create(&targetClass)
	s.db.Create(&targetRetainedClass)

	stPromoted := PesertaDidik{Nama: "Siswa Promosi", JenisKelamin: "L", NIS: "PR-1", NISN: "PR-1", KelasID: srcClass.ID, PokjarID: pokjar.ID, Status: "aktif"}
	stRetained := PesertaDidik{Nama: "Siswa Tinggal", JenisKelamin: "P", NIS: "PR-2", NISN: "PR-2", KelasID: srcClass.ID, PokjarID: pokjar.ID, Status: "aktif"}
	stGraduated := PesertaDidik{Nama: "Siswa Lulus", JenisKelamin: "L", NIS: "PR-3", NISN: "PR-3", KelasID: srcClass.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&stPromoted)
	s.db.Create(&stRetained)
	s.db.Create(&stGraduated)

	// Run Mass Promotion Wizard with overrides
	resPromo, _ := makeRequest(app, "POST", "/api/kenaikan-kelas", token, map[string]interface{}{
		"targetTahunAjaranId": year2.ID,
		"students": []map[string]interface{}{
			{"id": stPromoted.ID, "targetKelasId": targetClass.ID, "status": "naik"},
			{"id": stRetained.ID, "targetKelasId": targetRetainedClass.ID, "status": "tinggal"},
			{"id": stGraduated.ID, "status": "lulus"},
		},
	}, "")
	if resPromo.StatusCode != 204 {
		t.Fatalf("Mass promotion wizard failed with status %d", resPromo.StatusCode)
	}

	// Verify DB states
	var p1, p2, p3 PesertaDidik
	s.db.First(&p1, "id = ?", stPromoted.ID)
	s.db.First(&p2, "id = ?", stRetained.ID)
	s.db.First(&p3, "id = ?", stGraduated.ID)

	if p1.KelasID != targetClass.ID || p1.Status != "naik" {
		t.Errorf("promoted student state error")
	}
	if p2.Status != "tinggal" {
		t.Errorf("retained student state error")
	}
	if p3.Status != "lulus" {
		t.Errorf("graduated student state error")
	}
}

// Scenario 5: Multi-Role RBAC Compliance Verification
func TestE2E_Tier4_Scenario5_MultiRoleRBACCompliance(t *testing.T) {
	s, app := setupE2EServer(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	adminU := User{Username: "admin_rbac", Email: "admin_r@example.com", PasswordHash: string(hash), Role: "admin", IsActive: true}
	kepsekU := User{Username: "kepsek_rbac", Email: "kepsek_r@example.com", PasswordHash: string(hash), Role: "kepala_sekolah", IsActive: true}
	s.db.Create(&adminU)
	s.db.Create(&kepsekU)

	resA, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{"login": "admin_rbac", "password": "Password123"}, "")
	resK, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{"login": "kepsek_rbac", "password": "Password123"}, "")

	var bodyA, bodyK map[string]interface{}
	json.NewDecoder(resA.Body).Decode(&bodyA)
	json.NewDecoder(resK.Body).Decode(&bodyK)
	tokenAdmin := bodyA["accessToken"].(string)
	tokenKepsek := bodyK["accessToken"].(string)

	// Admin write succeeds (201)
	resAdminWrite, _ := makeRequest(app, "POST", "/api/pokjar", tokenAdmin, map[string]interface{}{"namaPokjar": "RBAC Pokjar", "tipe": "binaan"}, "")
	if resAdminWrite.StatusCode != 201 {
		t.Errorf("expected Admin write 201, got %d", resAdminWrite.StatusCode)
	}

	// Kepsek write fails (403)
	resKepsekWrite, _ := makeRequest(app, "POST", "/api/pokjar", tokenKepsek, map[string]interface{}{"namaPokjar": "RBAC Pokjar 2", "tipe": "binaan"}, "")
	if resKepsekWrite.StatusCode != 403 {
		t.Errorf("expected Kepsek write 403, got %d", resKepsekWrite.StatusCode)
	}

	// Kepsek read succeeds (200)
	resKepsekRead, _ := makeRequest(app, "GET", "/api/pokjar", tokenKepsek, nil, "")
	if resKepsekRead.StatusCode != 200 {
		t.Errorf("expected Kepsek read 200, got %d", resKepsekRead.StatusCode)
	}
}

// Scenario 6: Multi-Year Archive Retrieval Workflow
func TestE2E_Tier4_Scenario6_MultiYearArchiveRetrieval(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	// Query Arsip for Ganjil & Genap
	resGanjil, _ := makeRequest(app, "GET", "/api/arsip?tahunAjaranId="+year.ID+"&semester=Ganjil", token, nil, "")
	resGenap, _ := makeRequest(app, "GET", "/api/arsip?tahunAjaranId="+year.ID+"&semester=Genap", token, nil, "")

	if resGanjil.StatusCode != 200 || resGenap.StatusCode != 200 {
		t.Errorf("Archive query failed for Ganjil (%d) or Genap (%d)", resGanjil.StatusCode, resGenap.StatusCode)
	}
}

// Scenario 7: Security Audit Logging & System Hardening
func TestE2E_Tier4_Scenario7_SecurityAuditLogging(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Execute sensitive user management action
	resUser, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "audit_target_user",
		"email":    "audittarget@example.com",
		"password": "Password123",
		"role":     "admin",
	}, "")
	if resUser.StatusCode != 201 {
		t.Fatalf("User creation failed: status %d", resUser.StatusCode)
	}

	// Query audit logs
	resLogs, _ := makeRequest(app, "GET", "/api/audit-logs?resource=user", token, nil, "")
	if resLogs.StatusCode != 200 {
		t.Fatalf("Audit log query failed with status %d", resLogs.StatusCode)
	}
	var logs []AuditLog
	json.NewDecoder(resLogs.Body).Decode(&logs)
	if len(logs) == 0 {
		t.Errorf("Expected at least one audit log entry for resource=user")
	}
}

// Scenario 8: Scheduled Cron Attendance Auto-Generation
func TestE2E_Tier4_Scenario8_ScheduledCronGeneration(t *testing.T) {
	s, _ := setupE2EServer(t)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	tutor := Tutor{Nama: "Wali Cron", JenisKelamin: "L"}
	s.db.Create(&tutor)
	class := Kelas{Jenjang: 1, NamaRombel: "C8", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}
	s.db.Create(&class)

	loc, _ := time.LoadLocation("Asia/Jakarta")

	// Trigger generateAttendance manually
	s.generateAttendance(loc)

	// Verify attendance meeting created automatically
	var meetings []Presensi
	s.db.Where("kelas_id = ? AND dibuat_otomatis = ?", class.ID, true).Find(&meetings)
	if len(meetings) == 0 {
		t.Errorf("Cron auto-generation failed to create attendance meeting")
	}
}

// Scenario 9: Token Lifecycle & Refresh Rotation
func TestE2E_Tier4_Scenario9_TokenLifecycleAndRotation(t *testing.T) {
	_, app := setupE2EServer(t)

	// Login
	resLogin, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{"login": "admin", "password": "Admin123"}, "")
	cookie := resLogin.Header.Get("Set-Cookie")

	// Refresh token
	resRefresh, _ := makeRequest(app, "POST", "/api/auth/refresh", "", nil, cookie)
	if resRefresh.StatusCode != 200 {
		t.Fatalf("Refresh token rotation failed with status %d", resRefresh.StatusCode)
	}
	newCookie := resRefresh.Header.Get("Set-Cookie")

	// Attempt re-use of old cookie -> Must be rejected (401)
	resReuse, _ := makeRequest(app, "POST", "/api/auth/refresh", "", nil, cookie)
	if resReuse.StatusCode != 401 {
		t.Errorf("Reused refresh token expected 401, got %d", resReuse.StatusCode)
	}

	// Logout using newCookie
	var refreshRes map[string]interface{}
	json.NewDecoder(resRefresh.Body).Decode(&refreshRes)
	newToken := refreshRes["accessToken"].(string)

	resLogout, _ := makeRequest(app, "POST", "/api/auth/logout", newToken, nil, newCookie)
	if resLogout.StatusCode != 204 && resLogout.StatusCode != 240 {
		t.Errorf("Logout expected 204, got %d", resLogout.StatusCode)
	}
}

// Scenario 10: Fail-Safe Bulk Import & Transaction Isolation
func TestE2E_Tier4_Scenario10_FailSafeBulkImportIsolation(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	parent := OrangTua{NamaIbu: "Ibu S10"}
	s.db.Create(&parent)
	class := Kelas{Jenjang: 1, NamaRombel: "S10", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	// Create Excel containing 1 valid row and 1 invalid row (invalid jenis_kelamin "X")
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	xlsx.SetSheetRow(sheet, "A1", &[]string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"})
	xlsx.SetSheetRow(sheet, "A2", &[]string{"Valid S10", "L", "NIS-S10-1", "NISN-S10-1", "NIK-S10-1", class.ID, pokjar.ID, parent.ID})
	xlsx.SetSheetRow(sheet, "A3", &[]string{"Invalid S10", "X", "NIS-S10-2", "NISN-S10-2", "NIK-S10-2", class.ID, pokjar.ID, parent.ID})

	var content bytes.Buffer
	xlsx.Write(&content)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "siswa_invalid.xlsx")
	part.Write(content.Bytes())
	writer.Close()

	reqImport := httptest.NewRequest(http.MethodPost, "/api/peserta-didik/import", &body)
	reqImport.Header.Set("Content-Type", writer.FormDataContentType())
	reqImport.Header.Set("Authorization", "Bearer "+token)

	resImport, _ := app.Test(reqImport, -1)
	if resImport.StatusCode != 422 {
		t.Errorf("Expected 422 Unprocessable Entity for invalid Excel row, got %d", resImport.StatusCode)
	}

	// Verification: Zero students inserted (Atomic Rollback)
	var count int64
	s.db.Model(&PesertaDidik{}).Where("kelas_id = ?", class.ID).Count(&count)
	if count != 0 {
		t.Errorf("Scenario 10 atomic rollback failed: expected 0 students, found %d", count)
	}
}
