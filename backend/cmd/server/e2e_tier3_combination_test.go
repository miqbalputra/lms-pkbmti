package main

import (
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Tier 3: Cross-Feature Combinations & Pairwise Interactions (20+ tests)

// Combination 1: Tutor Wali Assignment -> Riwayat Wali Tracking
func TestE2E_Tier3_TutorWaliHistoryCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	tutorLama := Tutor{Nama: "Tutor Old", JenisKelamin: "L"}
	tutorBaru := Tutor{Nama: "Tutor New", JenisKelamin: "P"}
	s.db.Create(&tutorLama)
	s.db.Create(&tutorBaru)

	// Create class with tutorLama
	resCreate, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       1,
		"namaRombel":    "C1",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
		"waliKelasId":   tutorLama.ID,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for class create, got %d", resCreate.StatusCode)
	}
	var class Kelas
	json.NewDecoder(resCreate.Body).Decode(&class)

	time.Sleep(10 * time.Millisecond)

	// Update class to tutorBaru
	class.WaliKelasID = &tutorBaru.ID
	resUpdate, _ := makeRequest(app, "PUT", "/api/kelas/"+class.ID, token, class, "")
	if resUpdate.StatusCode != 200 {
		t.Fatalf("expected 200 for class update, got %d", resUpdate.StatusCode)
	}

	// Verify history records
	var histories []RiwayatWaliKelas
	s.db.Where("kelas_id = ?", class.ID).Order("tanggal_mulai desc, created_at desc").Find(&histories)
	if len(histories) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(histories))
	}
	if histories[0].TutorID != tutorBaru.ID || histories[0].TanggalSelesai != nil {
		t.Errorf("new history entry is not active")
	}
	if histories[1].TutorID != tutorLama.ID || histories[1].TanggalSelesai == nil {
		t.Errorf("old history entry was not closed")
	}
}

// Combination 2: Student Creation -> Automatic Class History Entry
func TestE2E_Tier3_StudentCreationHistoryCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	parent := OrangTua{NamaIbu: "Ibu Combo"}
	s.db.Create(&parent)
	class := Kelas{Jenjang: 2, NamaRombel: "C2", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	resCreate, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama":         "Siswa Combo",
		"jenisKelamin": "P",
		"nis":          "NIS-C2",
		"nisn":         "NISN-C2",
		"nik":          "NIK-C2",
		"kelasId":      class.ID,
		"pokjarId":     pokjar.ID,
		"orangTuaId":   parent.ID,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for student create, got %d", resCreate.StatusCode)
	}
	var student PesertaDidik
	json.NewDecoder(resCreate.Body).Decode(&student)

	// Verify automatic RiwayatKelasPesertaDidik entry
	var history RiwayatKelasPesertaDidik
	if err := s.db.First(&history, "peserta_didik_id = ? AND kelas_id = ?", student.ID, class.ID).Error; err != nil {
		t.Fatalf("expected automatic student class history entry, got error: %v", err)
	}
	if history.TahunAjaranID != year.ID || history.Status != "aktif" {
		t.Errorf("history year (%s) or status (%s) mismatch", history.TahunAjaranID, history.Status)
	}
}

// Combination 3: Class Duplication Across Years
func TestE2E_Tier3_ClassDuplicationCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year1 TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year1)

	year2 := TahunAjaran{NamaTahunAjaran: "2028/2029", TanggalMulai: time.Now().AddDate(2, 0, 0), TanggalSelesai: time.Now().AddDate(3, 0, 0)}
	s.db.Create(&year2)

	tutor := Tutor{Nama: "Wali Src", JenisKelamin: "L"}
	s.db.Create(&tutor)

	c1 := Kelas{Jenjang: 1, NamaRombel: "D1", PokjarID: pokjar.ID, TahunAjaranID: year1.ID, WaliKelasID: &tutor.ID}
	c2 := Kelas{Jenjang: 2, NamaRombel: "D2", PokjarID: pokjar.ID, TahunAjaranID: year1.ID, WaliKelasID: &tutor.ID}
	s.db.Create(&c1)
	s.db.Create(&c2)

	// Duplicate classes to target year
	resDup, _ := makeRequest(app, "POST", "/api/kelas/duplicate", token, map[string]interface{}{
		"sourceTahunAjaranId": year1.ID,
		"targetTahunAjaranId": year2.ID,
	}, "")
	if resDup.StatusCode != 200 {
		t.Fatalf("expected 200 for duplicate classes, got %d", resDup.StatusCode)
	}

	// Verify duplicated classes in year2
	var newClasses []Kelas
	s.db.Where("tahun_ajaran_id = ?", year2.ID).Find(&newClasses)
	if len(newClasses) < 2 {
		t.Errorf("expected at least 2 duplicated classes in target year, got %d", len(newClasses))
	}
	for _, nc := range newClasses {
		if nc.WaliKelasID != nil {
			t.Errorf("duplicated class wali_kelas_id should be reset to nil")
		}
	}
}

// Combination 4: Subject Pivot -> Bulk Teacher Assignment
func TestE2E_Tier3_SubjectPivotBulkAssignCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	mapelIPA := MataPelajaran{NamaMapel: "IPA Terpadu", KodeMapel: "IPA", IsActive: true}
	s.db.Create(&mapelIPA)
	tutorIPA := Tutor{Nama: "Guru IPA", JenisKelamin: "P"}
	s.db.Create(&tutorIPA)

	c1 := Kelas{Jenjang: 3, NamaRombel: "E1", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	c2 := Kelas{Jenjang: 4, NamaRombel: "E2", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&c1)
	s.db.Create(&c2)

	// Attach IPA mapel only to c1
	makeRequest(app, "PUT", "/api/kelas/"+c1.ID+"/mapel", token, map[string]interface{}{
		"mapelIds": []string{mapelIPA.ID},
	}, "")

	// Bulk assign Guru IPA to all classes with IPA subject
	resBulk, _ := makeRequest(app, "POST", "/api/penugasan/semua-kelas", token, map[string]interface{}{
		"tutorId":       tutorIPA.ID,
		"mapelId":       mapelIPA.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resBulk.StatusCode != 200 {
		t.Fatalf("expected 200 for assignAllClasses, got %d", resBulk.StatusCode)
	}

	// Verify assignment created for c1 but not c2
	var assignCount int64
	s.db.Model(&PenugasanGuruMapel{}).Where("tutor_id = ? AND mapel_id = ?", tutorIPA.ID, mapelIPA.ID).Count(&assignCount)
	if assignCount != 1 {
		t.Errorf("expected exactly 1 bulk assignment matching c1, got %d", assignCount)
	}
}

// Combination 5: Student Promotion -> History Archive Retrieval
func TestE2E_Tier3_PromotionHistoryArchiveCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year1 TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year1)
	year2 := TahunAjaran{NamaTahunAjaran: "2029/2030"}
	s.db.Create(&year2)

	class1 := Kelas{Jenjang: 1, NamaRombel: "F1", PokjarID: pokjar.ID, TahunAjaranID: year1.ID}
	class2 := Kelas{Jenjang: 2, NamaRombel: "F2", PokjarID: pokjar.ID, TahunAjaranID: year2.ID}
	s.db.Create(&class1)
	s.db.Create(&class2)

	student := PesertaDidik{Nama: "Siswa Archive", JenisKelamin: "L", NIS: "A-1", NISN: "A-1", KelasID: class1.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student)

	// Promote student to class2 in year2
	makeRequest(app, "POST", "/api/kenaikan-kelas", token, map[string]interface{}{
		"targetTahunAjaranId": year2.ID,
		"students": []map[string]interface{}{
			{
				"id":            student.ID,
				"targetKelasId": class2.ID,
				"status":        "naik",
			},
		},
	}, "")

	// Query Arsip for year2 Semester Ganjil
	resArsip, _ := makeRequest(app, "GET", "/api/arsip?tahunAjaranId="+year2.ID+"&semester=Ganjil", token, nil, "")
	if resArsip.StatusCode != 200 {
		t.Fatalf("expected 200 for archive query, got %d", resArsip.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resArsip.Body).Decode(&result)

	records, ok := result["riwayatKelas"].([]interface{})
	if !ok || len(records) == 0 {
		t.Errorf("expected riwayatKelas array in archive response")
	}
}

// Combination 6: User Actions -> Audit Log Generation
func TestE2E_Tier3_UserAuditLogCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Perform Tutor Create
	resCreate, _ := makeRequest(app, "POST", "/api/tutor", token, map[string]interface{}{
		"nama":         "Audit Tutor",
		"jenisKelamin": "L",
	}, "")
	var tutor Tutor
	json.NewDecoder(resCreate.Body).Decode(&tutor)

	// Perform Tutor Delete
	makeRequest(app, "DELETE", "/api/tutor/"+tutor.ID, token, nil, "")

	// Check Audit Log table
	var logs []AuditLog
	s.db.Where("resource = ?", "tutor").Order("created_at desc").Find(&logs)
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit log records for tutor resource, got %d", len(logs))
	}
}

// Combination 7: Presensi Details -> Summary Rekap Metrics
func TestE2E_Tier3_PresensiRekapExportCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	class := Kelas{Jenjang: 1, NamaRombel: "R1", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	student := PesertaDidik{Nama: "Siswa Rekap", JenisKelamin: "L", NIS: "R-1", NISN: "R-1", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student)

	validSig := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	resMeeting, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":         class.ID,
		"tanggal":         testSaturday().Format(time.RFC3339),
		"statusPertemuan": "berlangsung",
		"tandaTangan":     validSig,
	}, "")
	var meeting Presensi
	json.NewDecoder(resMeeting.Body).Decode(&meeting)

	// Submit Hadir status
	makeRequest(app, "POST", "/api/presensi/"+meeting.ID+"/details", token, []map[string]interface{}{
		{"pesertaDidikId": student.ID, "statusKehadiran": "Hadir"},
	}, "")

	// Call rekapPresensi endpoint
	resRekap, _ := makeRequest(app, "GET", "/api/presensi/rekap?kelasId="+class.ID+"&semester="+meeting.Semester, token, nil, "")
	if resRekap.StatusCode != 200 {
		t.Fatalf("expected 200 for rekapPresensi, got %d", resRekap.StatusCode)
	}

	// Call rekapPresensiPDF endpoint
	resPDF, _ := makeRequest(app, "GET", "/api/presensi/rekap/pdf?kelasId="+class.ID+"&semester="+meeting.Semester, token, nil, "")
	if resPDF.StatusCode != 200 {
		t.Errorf("expected 200 for rekapPresensiPDF, got %d", resPDF.StatusCode)
	}
}

// Combination 8: Guru Role Scoped Dashboard Metrics
func TestE2E_Tier3_GuruScopedDashboardCombination(t *testing.T) {
	s, app := setupE2EServer(t)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	tutor1 := Tutor{Nama: "Wali Scoped 1", JenisKelamin: "L"}
	tutor2 := Tutor{Nama: "Wali Scoped 2", JenisKelamin: "P"}
	s.db.Create(&tutor1)
	s.db.Create(&tutor2)

	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	guruUser := User{Username: "guru_scoped", Email: "guruscope@example.com", PasswordHash: string(hash), Role: "guru", TutorID: &tutor1.ID, IsActive: true}
	s.db.Create(&guruUser)

	c1 := Kelas{Jenjang: 1, NamaRombel: "S1", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor1.ID}
	c2 := Kelas{Jenjang: 2, NamaRombel: "S2", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor2.ID}
	s.db.Create(&c1)
	s.db.Create(&c2)

	s1 := PesertaDidik{Nama: "Siswa Guru 1", JenisKelamin: "L", NIS: "SG-1", NISN: "SG-1", KelasID: c1.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s2 := PesertaDidik{Nama: "Siswa Guru 2", JenisKelamin: "P", NIS: "SG-2", NISN: "SG-2", KelasID: c2.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&s1)
	s.db.Create(&s2)

	// Login as Guru 1
	resLogin, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "guru_scoped",
		"password": "Password123",
	}, "")
	var loginRes map[string]interface{}
	json.NewDecoder(resLogin.Body).Decode(&loginRes)
	guruToken := loginRes["accessToken"].(string)

	// Call dashboard as Guru 1 -> should only see 1 student & 1 class
	resDash, _ := makeRequest(app, "GET", "/api/dashboard", guruToken, nil, "")
	if resDash.StatusCode != 200 {
		t.Fatalf("expected 200 for guru dashboard, got %d", resDash.StatusCode)
	}
	var dashRes map[string]interface{}
	json.NewDecoder(resDash.Body).Decode(&dashRes)

	pesertaCount, _ := dashRes["pesertaDidik"].(float64)
	kelasCount, _ := dashRes["kelas"].(float64)
	if int(pesertaCount) != 1 || int(kelasCount) != 1 {
		t.Errorf("guru scoped dashboard counts expected 1 student and 1 class, got %v students, %v classes", pesertaCount, kelasCount)
	}
}

// Combination 9: Academic Year Active Switch Auto-Deactivates Others
func TestE2E_Tier3_AcademicYearActiveSwitchCombination(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Create Year A (Active)
	resA, _ := makeRequest(app, "POST", "/api/tahun-ajaran", token, map[string]interface{}{
		"namaTahunAjaran": "Year A",
		"tanggalMulai":    time.Now(),
		"tanggalSelesai":  time.Now().AddDate(1, 0, 0),
		"isAktif":         true,
	}, "")
	var yearA TahunAjaran
	json.NewDecoder(resA.Body).Decode(&yearA)

	// Create Year B (Active) -> Should auto deactivate Year A
	resB, _ := makeRequest(app, "POST", "/api/tahun-ajaran", token, map[string]interface{}{
		"namaTahunAjaran": "Year B",
		"tanggalMulai":    time.Now().AddDate(1, 0, 0),
		"tanggalSelesai":  time.Now().AddDate(2, 0, 0),
		"isAktif":         true,
	}, "")
	var yearB TahunAjaran
	json.NewDecoder(resB.Body).Decode(&yearB)

	// Verify Year A is now inactive
	var refreshedA TahunAjaran
	s.db.First(&refreshedA, "id = ?", yearA.ID)
	if refreshedA.IsAktif {
		t.Errorf("expected Year A to be automatically deactivated when Year B was set to active")
	}
}
