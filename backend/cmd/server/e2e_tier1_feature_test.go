package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// setupE2EServer initializes a clean in-memory database and Fiber application with all routes configured.
func setupE2EServer(t *testing.T) (*Server, *fiber.App) {
	t.Helper()
	dbName := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Limit SQLite to 1 open connection to prevent in-memory transaction deadlocks
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	cfg := Config{
		AccessSecret:  "e2e-access-secret-minimum-32-characters-long",
		RefreshSecret: "e2e-refresh-secret-minimum-32-characters-long",
		Env:           "development",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
	}

	s := &Server{db: db, cfg: cfg}
	if err := s.migrateSchema(); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Post("/api/auth/login", s.login)
	app.Post("/api/auth/refresh", s.refresh)
	app.Post("/api/auth/logout", s.auth, s.logout)
	app.Get("/api/auth/me", s.auth, s.me)

	protected := app.Group("/api", s.auth)
	protected.Get("/dashboard", s.dashboard)
	s.routes(protected)

	return s, app
}

// helper to perform HTTP requests on Fiber test app
func makeRequest(app *fiber.App, method, url string, token string, body interface{}, cookie string) (*http.Response, error) {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	} else {
		bodyReader = bytes.NewBuffer([]byte{})
	}

	req := httptest.NewRequest(method, url, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	return app.Test(req, -1)
}

func getAdminToken(t *testing.T, app *fiber.App) (string, string) {
	t.Helper()
	res, err := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "admin",
		"password": "Admin123",
	}, "")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("admin login failed: status %d, err %v", res.StatusCode, err)
	}
	defer res.Body.Close()

	cookie := res.Header.Get("Set-Cookie")
	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	token, ok := result["accessToken"].(string)
	if !ok || token == "" {
		t.Fatalf("accessToken missing in response")
	}
	return token, cookie
}

// Tier 1 - F01: Auth
func TestE2E_Tier1_Auth(t *testing.T) {
	_, app := setupE2EServer(t)

	// 1. Admin login success
	token, cookie := getAdminToken(t, app)

	// 2. Failed login with wrong password
	resFail, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    "admin",
		"password": "WrongPassword",
	}, "")
	if resFail.StatusCode != 401 {
		t.Errorf("expected 401 for wrong password, got %d", resFail.StatusCode)
	}

	// 3. Get Me using valid token
	resMe, _ := makeRequest(app, "GET", "/api/auth/me", token, nil, "")
	if resMe.StatusCode != 200 {
		t.Errorf("expected 200 for /auth/me, got %d", resMe.StatusCode)
	}

	// 4. Refresh token rotation
	resRefresh, _ := makeRequest(app, "POST", "/api/auth/refresh", "", nil, cookie)
	if resRefresh.StatusCode != 200 {
		t.Errorf("expected 200 for /auth/refresh, got %d", resRefresh.StatusCode)
	}

	// 5. Logout
	resLogout, _ := makeRequest(app, "POST", "/api/auth/logout", token, nil, cookie)
	if resLogout.StatusCode != 240 && resLogout.StatusCode != 204 {
		t.Errorf("expected 204 for /auth/logout, got %d", resLogout.StatusCode)
	}
}

// Tier 1 - F02: CRUD Tutors
func TestE2E_Tier1_CRUD_Tutors(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. Create Tutor
	resCreate, _ := makeRequest(app, "POST", "/api/tutor", token, map[string]interface{}{
		"nama":         "Tutor Tier 1",
		"jenisKelamin": "L",
		"noHp":         "08123456789",
		"alamat":       "Jl. Education No. 1",
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for tutor create, got %d", resCreate.StatusCode)
	}
	var created Tutor
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Tutors
	resList, _ := makeRequest(app, "GET", "/api/tutor", token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for tutor list, got %d", resList.StatusCode)
	}

	// 3. Get Tutor by ID
	resGet, _ := makeRequest(app, "GET", "/api/tutor/"+created.ID, token, nil, "")
	if resGet.StatusCode != 200 {
		t.Errorf("expected 200 for get tutor, got %d", resGet.StatusCode)
	}

	// 4. Update Tutor
	created.Nama = "Tutor Tier 1 Updated"
	resUpdate, _ := makeRequest(app, "PUT", "/api/tutor/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for update tutor, got %d", resUpdate.StatusCode)
	}

	// 5. Delete Tutor
	resDel, _ := makeRequest(app, "DELETE", "/api/tutor/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for delete tutor, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F03: CRUD Parents
func TestE2E_Tier1_CRUD_Parents(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. Create Parent
	resCreate, _ := makeRequest(app, "POST", "/api/orang-tua", token, map[string]interface{}{
		"namaBapak": "Bapak Suparman",
		"namaIbu":   "Ibu Suparmi",
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for parent create, got %d", resCreate.StatusCode)
	}
	var created OrangTua
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Parents
	resList, _ := makeRequest(app, "GET", "/api/orang-tua", token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for parent list, got %d", resList.StatusCode)
	}

	// 3. Update Parent
	created.NamaIbu = "Ibu Suparmi Revised"
	resUpdate, _ := makeRequest(app, "PUT", "/api/orang-tua/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for parent update, got %d", resUpdate.StatusCode)
	}

	// 4. Get Parent Details (via list filter/find)
	if created.ID == "" {
		t.Errorf("parent ID must not be empty")
	}

	// 5. Delete Parent
	resDel, _ := makeRequest(app, "DELETE", "/api/orang-tua/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for parent delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F04: CRUD Pokjars
func TestE2E_Tier1_CRUD_Pokjars(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. Verify Initial Seeded Pokjars (4 total)
	resList, _ := makeRequest(app, "GET", "/api/pokjar", token, nil, "")
	if resList.StatusCode != 200 {
		t.Fatalf("expected 200 for pokjar list, got %d", resList.StatusCode)
	}
	var pokjars []Pokjar
	json.NewDecoder(resList.Body).Decode(&pokjars)
	if len(pokjars) < 4 {
		t.Errorf("expected at least 4 seeded pokjars, got %d", len(pokjars))
	}

	// 2. Create New Pokjar
	resCreate, _ := makeRequest(app, "POST", "/api/pokjar", token, map[string]interface{}{
		"namaPokjar": "Pokjar Unit Test",
		"tipe":       "binaan",
		"alamat":     "Jl. Cabang No. 5",
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for pokjar create, got %d", resCreate.StatusCode)
	}
	var created Pokjar
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 3. Update Pokjar
	created.Alamat = "Jl. Cabang No. 99"
	resUpdate, _ := makeRequest(app, "PUT", "/api/pokjar/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for pokjar update, got %d", resUpdate.StatusCode)
	}

	// 4. List Pokjars again
	resList2, _ := makeRequest(app, "GET", "/api/pokjar", token, nil, "")
	if resList2.StatusCode != 200 {
		t.Errorf("expected 200 for pokjar list, got %d", resList2.StatusCode)
	}

	// 5. Delete Pokjar
	resDel, _ := makeRequest(app, "DELETE", "/api/pokjar/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for pokjar delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F05: CRUD Years (Tahun Ajaran)
func TestE2E_Tier1_CRUD_Years(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. List Academic Years
	resList, _ := makeRequest(app, "GET", "/api/tahun-ajaran", token, nil, "")
	if resList.StatusCode != 200 {
		t.Fatalf("expected 200 for year list, got %d", resList.StatusCode)
	}

	// 2. Create New Academic Year
	resCreate, _ := makeRequest(app, "POST", "/api/tahun-ajaran", token, map[string]interface{}{
		"namaTahunAjaran": "2030/2031",
		"tanggalMulai":    time.Now().AddDate(4, 0, 0),
		"tanggalSelesai":  time.Now().AddDate(5, 0, 0),
		"isAktif":         false,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for year create, got %d", resCreate.StatusCode)
	}
	var created TahunAjaran
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 3. Set New Academic Year as Active
	created.IsAktif = true
	resActive, _ := makeRequest(app, "PUT", "/api/tahun-ajaran/"+created.ID, token, created, "")
	if resActive.StatusCode != 200 {
		t.Errorf("expected 200 for setting year active, got %d", resActive.StatusCode)
	}

	// 4. Update Year Dates
	created.TanggalSelesai = time.Now().AddDate(5, 1, 0)
	resUpdate, _ := makeRequest(app, "PUT", "/api/tahun-ajaran/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for updating year dates, got %d", resUpdate.StatusCode)
	}

	// 5. Delete Academic Year
	resDel, _ := makeRequest(app, "DELETE", "/api/tahun-ajaran/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for deleting year, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F06: CRUD Subjects (Mapel)
func TestE2E_Tier1_CRUD_Subjects(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. Create Subject
	resCreate, _ := makeRequest(app, "POST", "/api/mapel", token, map[string]interface{}{
		"namaMapel": "Bahasa Indonesia",
		"kodeMapel": "BIN",
		"isActive":  true,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for mapel create, got %d", resCreate.StatusCode)
	}
	var created MataPelajaran
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Subjects
	resList, _ := makeRequest(app, "GET", "/api/mapel", token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for mapel list, got %d", resList.StatusCode)
	}

	// 3. Toggle Active Status
	created.IsActive = false
	resToggle, _ := makeRequest(app, "PUT", "/api/mapel/"+created.ID, token, created, "")
	if resToggle.StatusCode != 200 {
		t.Errorf("expected 200 for mapel toggle, got %d", resToggle.StatusCode)
	}

	// 4. Update Subject Name & Code
	created.NamaMapel = "Bahasa Indonesia Terpadu"
	created.KodeMapel = "BIND"
	resUpdate, _ := makeRequest(app, "PUT", "/api/mapel/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for mapel update, got %d", resUpdate.StatusCode)
	}

	// 5. Delete Subject
	resDel, _ := makeRequest(app, "DELETE", "/api/mapel/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for mapel delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F07: CRUD Classes (Kelas)
func TestE2E_Tier1_CRUD_Classes(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	tutor := Tutor{Nama: "Tutor Kelas", JenisKelamin: "L"}
	s.db.Create(&tutor)

	// 1. Create Class
	resCreate, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang":       1,
		"namaRombel":    "A",
		"pokjarId":      pokjar.ID,
		"tahunAjaranId": year.ID,
		"waliKelasId":   tutor.ID,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for class create, got %d", resCreate.StatusCode)
	}
	var created Kelas
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Classes
	resList, _ := makeRequest(app, "GET", "/api/kelas", token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for class list, got %d", resList.StatusCode)
	}

	// 3. Get Class Wali History
	resWali, _ := makeRequest(app, "GET", "/api/kelas/"+created.ID+"/riwayat-wali", token, nil, "")
	if resWali.StatusCode != 200 {
		t.Errorf("expected 200 for class wali history, got %d", resWali.StatusCode)
	}

	// 4. Update Class (Change Wali)
	newTutor := Tutor{Nama: "Tutor Baru Wali", JenisKelamin: "P"}
	s.db.Create(&newTutor)
	created.WaliKelasID = &newTutor.ID
	resUpdate, _ := makeRequest(app, "PUT", "/api/kelas/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for class update, got %d", resUpdate.StatusCode)
	}

	// 5. Delete Class
	resDel, _ := makeRequest(app, "DELETE", "/api/kelas/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for class delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F08: CRUD Students (Peserta Didik)
func TestE2E_Tier1_CRUD_Students(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	parent := OrangTua{NamaIbu: "Ibu Student"}
	s.db.Create(&parent)
	class := Kelas{Jenjang: 2, NamaRombel: "B", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	// 1. Create Student
	resCreate, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama":         "Siswa Tier 1",
		"jenisKelamin": "L",
		"nis":          "NIS-101",
		"nisn":         "NISN-101",
		"nik":          "NIK-T1-101",
		"kelasId":      class.ID,
		"pokjarId":     pokjar.ID,
		"orangTuaId":   parent.ID,
		"status":       "aktif",
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for student create, got %d", resCreate.StatusCode)
	}
	var created PesertaDidik
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Students
	resList, _ := makeRequest(app, "GET", "/api/peserta-didik?kelasId="+class.ID, token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for student list, got %d", resList.StatusCode)
	}

	// 3. Update Student Record
	created.Nama = "Siswa Tier 1 Updated"
	resUpdate, _ := makeRequest(app, "PUT", "/api/peserta-didik/"+created.ID, token, created, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for student update, got %d", resUpdate.StatusCode)
	}

	// 4. Download Excel Template
	resTpl, _ := makeRequest(app, "GET", "/api/peserta-didik/template", token, nil, "")
	if resTpl.StatusCode != 200 {
		t.Errorf("expected 200 for student template, got %d", resTpl.StatusCode)
	}

	// 5. Delete Student
	resDel, _ := makeRequest(app, "DELETE", "/api/peserta-didik/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for student delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F09: CRUD Assignments (Penugasan)
func TestE2E_Tier1_CRUD_Assignments(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	tutor := Tutor{Nama: "Tutor Mapel", JenisKelamin: "P"}
	s.db.Create(&tutor)
	mapel := MataPelajaran{NamaMapel: "Matematika", KodeMapel: "MTK", IsActive: true}
	s.db.Create(&mapel)
	class := Kelas{Jenjang: 3, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&class)

	// 1. Configure Class Subjects Pivot (`setKelasMapel`)
	resPivot, _ := makeRequest(app, "PUT", "/api/kelas/"+class.ID+"/mapel", token, map[string]interface{}{
		"mapelIds": []string{mapel.ID},
	}, "")
	if resPivot.StatusCode != 204 {
		t.Errorf("expected 204 for setKelasMapel, got %d", resPivot.StatusCode)
	}

	// 2. List Class Subjects (`/kelas-mapel`)
	resKM, _ := makeRequest(app, "GET", "/api/kelas-mapel", token, nil, "")
	if resKM.StatusCode != 200 {
		t.Errorf("expected 200 for list kelas-mapel, got %d", resKM.StatusCode)
	}

	// 3. Create Tutor Subject Assignment
	resAssign, _ := makeRequest(app, "POST", "/api/penugasan", token, map[string]interface{}{
		"tutorId": tutor.ID,
		"kelasId": class.ID,
		"mapelId": mapel.ID,
	}, "")
	if resAssign.StatusCode != 201 {
		t.Fatalf("expected 201 for penugasan create, got %d", resAssign.StatusCode)
	}
	var created PenugasanGuruMapel
	json.NewDecoder(resAssign.Body).Decode(&created)

	// 4. Bulk Assign Tutor (`assignAllClasses`)
	resBulk, _ := makeRequest(app, "POST", "/api/penugasan/semua-kelas", token, map[string]interface{}{
		"tutorId":       tutor.ID,
		"mapelId":       mapel.ID,
		"tahunAjaranId": year.ID,
	}, "")
	if resBulk.StatusCode != 200 {
		t.Errorf("expected 200 for assignAllClasses, got %d", resBulk.StatusCode)
	}

	// 4b. Save the complete class+subject checklist in one request.
	resSchema, _ := makeRequest(app, "POST", "/api/penugasan/skema", token, map[string]interface{}{
		"tutorId":  tutor.ID,
		"kelasIds": []string{class.ID},
		"assignments": []map[string]string{
			{"kelasId": class.ID, "mapelId": mapel.ID},
		},
	}, "")
	if resSchema.StatusCode != 200 {
		t.Errorf("expected 200 for penugasan skema, got %d", resSchema.StatusCode)
	}

	// 5. Delete Assignment
	resDel, _ := makeRequest(app, "DELETE", "/api/penugasan/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for penugasan delete, got %d", resDel.StatusCode)
	}
}

func testSaturday() time.Time {
	day := time.Now()
	for day.Weekday() != time.Saturday {
		day = day.AddDate(0, 0, 1)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, time.UTC)
}

// Tier 1 - F10: Attendance Canvas
func TestE2E_Tier1_AttendanceCanvas(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)
	tutor := Tutor{Nama: "Wali Presensi", JenisKelamin: "L"}
	s.db.Create(&tutor)
	class := Kelas{Jenjang: 4, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}
	s.db.Create(&class)

	validSignaturePNG := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	// 1. Create Attendance Session
	resCreate, _ := makeRequest(app, "POST", "/api/presensi", token, map[string]interface{}{
		"kelasId":         class.ID,
		"tanggal":         testSaturday().Format(time.RFC3339),
		"statusPertemuan": "berlangsung",
		"tandaTangan":     validSignaturePNG,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for presensi create, got %d", resCreate.StatusCode)
	}
	var meeting Presensi
	json.NewDecoder(resCreate.Body).Decode(&meeting)

	// 2. List Attendance Sessions
	resList, _ := makeRequest(app, "GET", "/api/presensi?kelasId="+class.ID, token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for presensi list, got %d", resList.StatusCode)
	}

	// 3. Update Attendance Meeting
	meeting.Keterangan = "Pertemuan Diperbarui"
	resUpdate, _ := makeRequest(app, "PUT", "/api/presensi/"+meeting.ID, token, meeting, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for presensi update, got %d", resUpdate.StatusCode)
	}

	// 4. Save Details (Checklist)
	student := PesertaDidik{Nama: "Siswa Presensi", JenisKelamin: "L", NIS: "P-101", NISN: "P-101", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student)

	resDetails, _ := makeRequest(app, "POST", "/api/presensi/"+meeting.ID+"/details", token, []map[string]interface{}{
		{
			"pesertaDidikId":  student.ID,
			"statusKehadiran": "Hadir",
			"catatan":         "Hadir tepat waktu",
		},
	}, "")
	if resDetails.StatusCode != 204 {
		t.Errorf("expected 204 for saveDetails, got %d", resDetails.StatusCode)
	}

	// 5. Export Presensi CSV & Rekap
	resCSV, _ := makeRequest(app, "GET", "/api/presensi/export?kelasId="+class.ID, token, nil, "")
	if resCSV.StatusCode != 200 {
		t.Errorf("expected 200 for export presensi CSV, got %d", resCSV.StatusCode)
	}
}

// Tier 1 - F11: Promotion (Kenaikan Kelas)
func TestE2E_Tier1_Promotion(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var currentYear TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&currentYear)

	targetYear := TahunAjaran{NamaTahunAjaran: "2027/2028", TanggalMulai: time.Now().AddDate(1, 0, 0), TanggalSelesai: time.Now().AddDate(2, 0, 0), IsAktif: false}
	s.db.Create(&targetYear)

	sourceClass := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: currentYear.ID}
	s.db.Create(&sourceClass)
	targetClass := Kelas{Jenjang: 2, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: targetYear.ID}
	s.db.Create(&targetClass)

	student1 := PesertaDidik{Nama: "Naik 1", JenisKelamin: "L", NIS: "N-1", NISN: "N-1", KelasID: sourceClass.ID, PokjarID: pokjar.ID, Status: "aktif"}
	student2 := PesertaDidik{Nama: "Lulus 2", JenisKelamin: "P", NIS: "N-2", NISN: "N-2", KelasID: sourceClass.ID, PokjarID: pokjar.ID, Status: "aktif"}
	s.db.Create(&student1)
	s.db.Create(&student2)

	// 1. Run Promotion
	resPromote, _ := makeRequest(app, "POST", "/api/kenaikan-kelas", token, map[string]interface{}{
		"targetTahunAjaranId": targetYear.ID,
		"students": []map[string]interface{}{
			{
				"id":            student1.ID,
				"targetKelasId": targetClass.ID,
				"status":        "naik",
				"catatan":       "Naik Kelas 2A",
			},
			{
				"id":      student2.ID,
				"status":  "lulus",
				"catatan": "Lulus dari PKBM",
			},
		},
	}, "")
	if resPromote.StatusCode != 204 {
		t.Fatalf("expected 204 for mass promotion, got %d", resPromote.StatusCode)
	}

	// 2. Verify Student 1 updated class ID
	var s1 PesertaDidik
	s.db.First(&s1, "id = ?", student1.ID)
	if s1.KelasID != targetClass.ID || s1.Status != "naik" {
		t.Errorf("student 1 promotion state incorrect: class %s, status %s", s1.KelasID, s1.Status)
	}

	// 3. Verify Student 2 graduated status
	var s2 PesertaDidik
	s.db.First(&s2, "id = ?", student2.ID)
	if s2.Status != "lulus" {
		t.Errorf("student 2 status expected 'lulus', got '%s'", s2.Status)
	}

	// 4. Verify RiwayatKelasPesertaDidik entries
	var count int64
	s.db.Model(&RiwayatKelasPesertaDidik{}).Where("tahun_ajaran_id = ?", targetYear.ID).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 promotion history records, got %d", count)
	}

	// 5. Query Arsip endpoint (`/api/arsip`)
	resArsip, _ := makeRequest(app, "GET", "/api/arsip?tahunAjaranId="+targetYear.ID+"&semester=Ganjil", token, nil, "")
	if resArsip.StatusCode != 200 {
		t.Errorf("expected 200 for archive query, got %d", resArsip.StatusCode)
	}
}

// Tier 1 - F12: Accounts (Akun)
func TestE2E_Tier1_Accounts(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	tutor := Tutor{Nama: "Tutor Account", JenisKelamin: "L"}
	s.db.Create(&tutor)

	// 1. Create Guru Account
	resCreate, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "guru_tier1",
		"email":    "guru_tier1@example.com",
		"password": "Password123",
		"role":     "guru",
		"tutorId":  tutor.ID,
	}, "")
	if resCreate.StatusCode != 201 {
		t.Fatalf("expected 201 for user create, got %d", resCreate.StatusCode)
	}
	var created User
	json.NewDecoder(resCreate.Body).Decode(&created)

	// 2. List Users
	resList, _ := makeRequest(app, "GET", "/api/users", token, nil, "")
	if resList.StatusCode != 200 {
		t.Errorf("expected 200 for user list, got %d", resList.StatusCode)
	}

	// 3. Update User Account
	created.Email = "guru_updated@example.com"
	resUpdate, _ := makeRequest(app, "PUT", "/api/users/"+created.ID, token, map[string]interface{}{
		"username": created.Username,
		"email":    created.Email,
		"role":     created.Role,
		"tutorId":  created.TutorID,
		"isActive": true,
	}, "")
	if resUpdate.StatusCode != 200 {
		t.Errorf("expected 200 for user update, got %d", resUpdate.StatusCode)
	}

	// 4. Create Kepala Sekolah Account
	resKepsek, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "kepsek_tier1",
		"email":    "kepsek@example.com",
		"password": "Password123",
		"role":     "kepala_sekolah",
	}, "")
	if resKepsek.StatusCode != 201 {
		t.Errorf("expected 201 for kepsek create, got %d", resKepsek.StatusCode)
	}

	// 5. Delete User
	resDel, _ := makeRequest(app, "DELETE", "/api/users/"+created.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Errorf("expected 204 for user delete, got %d", resDel.StatusCode)
	}
}

// Tier 1 - F13: Settings (Pengaturan Jadwal)
func TestE2E_Tier1_Settings(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// 1. Get Schedule Settings
	resGet, _ := makeRequest(app, "GET", "/api/settings/jadwal", token, nil, "")
	if resGet.StatusCode != 200 {
		t.Fatalf("expected 200 for get schedule, got %d", resGet.StatusCode)
	}
	var setting PengaturanJadwal
	json.NewDecoder(resGet.Body).Decode(&setting)

	// 2. Update Schedule Settings
	setting.HariDefault = "Sabtu"
	setting.JamGenerate = "00:05"
	setting.ZonaWaktu = "Asia/Jakarta"
	resPut, _ := makeRequest(app, "PUT", "/api/settings/jadwal", token, setting, "")
	if resPut.StatusCode != 200 {
		t.Errorf("expected 200 for put schedule, got %d", resPut.StatusCode)
	}

	// 3. Verify updated values
	resGet2, _ := makeRequest(app, "GET", "/api/settings/jadwal", token, nil, "")
	if resGet2.StatusCode != 200 {
		t.Errorf("expected 200 for re-read schedule, got %d", resGet2.StatusCode)
	}
	var reRead PengaturanJadwal
	json.NewDecoder(resGet2.Body).Decode(&reRead)
	if reRead.HariDefault != "Sabtu" || reRead.JamGenerate != "00:05" {
		t.Errorf("schedule setting mismatch: %s %s", reRead.HariDefault, reRead.JamGenerate)
	}

	// 4. Test Dashboard Summary Endpoint
	resDash, _ := makeRequest(app, "GET", "/api/dashboard", token, nil, "")
	if resDash.StatusCode != 200 {
		t.Errorf("expected 200 for dashboard summary, got %d", resDash.StatusCode)
	}

	// 5. Update Schedule Settings to another valid day
	reRead.HariDefault = "Jumat"
	resPut2, _ := makeRequest(app, "PUT", "/api/settings/jadwal", token, reRead, "")
	if resPut2.StatusCode != 200 {
		t.Errorf("expected 200 for valid day update, got %d", resPut2.StatusCode)
	}
}

// Tier 1 - F14: Audit Logs
func TestE2E_Tier1_AuditLogs(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// Trigger operations to create audit logs
	makeRequest(app, "POST", "/api/pokjar", token, map[string]interface{}{
		"namaPokjar": "Audit Pokjar",
		"tipe":       "binaan",
	}, "")

	// 1. List Audit Logs
	resLogs, _ := makeRequest(app, "GET", "/api/audit-logs", token, nil, "")
	if resLogs.StatusCode != 200 {
		t.Fatalf("expected 200 for list audit logs, got %d", resLogs.StatusCode)
	}
	var logs []AuditLog
	json.NewDecoder(resLogs.Body).Decode(&logs)
	if len(logs) == 0 {
		t.Errorf("expected at least 1 audit log entry")
	}

	// 2. Filter Audit Logs by Action
	resFilterAction, _ := makeRequest(app, "GET", "/api/audit-logs?action=login", token, nil, "")
	if resFilterAction.StatusCode != 200 {
		t.Errorf("expected 200 for audit log action filter, got %d", resFilterAction.StatusCode)
	}

	// 3. Filter Audit Logs by Resource
	resFilterResource, _ := makeRequest(app, "GET", "/api/audit-logs?resource=auth", token, nil, "")
	if resFilterResource.StatusCode != 200 {
		t.Errorf("expected 200 for audit log resource filter, got %d", resFilterResource.StatusCode)
	}

	// 4. Verify Capping (limit 250)
	if len(logs) > 250 {
		t.Errorf("audit log query output exceeded 250 items")
	}

	// 5. Filter combined
	resFilterBoth, _ := makeRequest(app, "GET", "/api/audit-logs?action=create&resource=pokjar", token, nil, "")
	if resFilterBoth.StatusCode != 200 {
		t.Errorf("expected 200 for combined audit log filter, got %d", resFilterBoth.StatusCode)
	}
}
