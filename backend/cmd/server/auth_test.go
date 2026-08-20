package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db := isolatedTestDB(t, t.Name())
	if err := db.AutoMigrate(&User{}, &RefreshToken{}, &AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return &Server{db: db, cfg: Config{AccessSecret: "access-secret-for-tests-32-characters", RefreshSecret: "refresh-secret-for-tests-32-characters", Env: "development", AccessTTL: time.Minute, RefreshTTL: time.Hour}}
}

func TestTutorEmailCanBeAddedAfterAccountCreation(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureOptionalUserEmailIndex(); err != nil {
		t.Fatal(err)
	}
	firstTutor := Tutor{Nama: "Tutor Pertama", JenisKelamin: "L"}
	secondTutor := Tutor{Nama: "Tutor Kedua", JenisKelamin: "P"}
	for _, tutor := range []*Tutor{&firstTutor, &secondTutor} {
		if err := s.db.Create(tutor).Error; err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New()
	app.Post("/users", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.createUser(c)
	})
	for i, tutor := range []*Tutor{&firstTutor, &secondTutor} {
		body := strings.NewReader(fmt.Sprintf(`{"username":"tutor-no-email-%d","password":"Password123","role":"guru","tutorId":"%s","isActive":true}`, i+1, tutor.ID))
		request := httptest.NewRequest(http.MethodPost, "/users", body)
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("expected tutor account %d to be created without email, got %d", i+1, response.StatusCode)
		}
	}
	var user User
	if err := s.db.Where("username = ?", "tutor-no-email-1").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	app.Put("/auth/account", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.updateOwnAccount(c)
	})
	request := httptest.NewRequest(http.MethodPut, "/auth/account", strings.NewReader(`{"email":"tutor.pertama@gmail.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected tutor to save Gmail after login, got %d", response.StatusCode)
	}
	if err := s.db.First(&user, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "tutor.pertama@gmail.com" {
		t.Fatalf("expected saved Gmail to sync to user account, got %q", user.Email)
	}
}

func TestUserNameUsesLinkedTutorName(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}); err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Nama Tutor Lengkap", JenisKelamin: "L"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "username-tutor", Role: "guru", TutorID: &tutor.ID}
	if err := s.fillUserNames(&user); err != nil {
		t.Fatal(err)
	}
	if user.Nama != tutor.Nama {
		t.Fatalf("expected linked tutor name %q, got %q", tutor.Nama, user.Nama)
	}
}

func TestRefreshTokenRotatesAndRejectsReuse(t *testing.T) {
	s := testServer(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := User{Username: "refresh-user", Email: "refresh@example.test", PasswordHash: string(hash), Role: "admin", IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/login", s.login)
	app.Post("/refresh", s.refresh)
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"login":"refresh-user","password":"Password123"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse, err := app.Test(login)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", loginResponse.StatusCode)
	}
	cookie := loginResponse.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("password login refresh cookie should use SameSite=Lax, got %q", cookie)
	}
	refresh := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	refresh.Header.Set("Cookie", cookie)
	refreshResponse, err := app.Test(refresh)
	if err != nil {
		t.Fatal(err)
	}
	defer refreshResponse.Body.Close()
	if refreshResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected refresh 200, got %d", refreshResponse.StatusCode)
	}
	reuse := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	reuse.Header.Set("Cookie", cookie)
	reuseResponse, err := app.Test(reuse)
	if err != nil {
		t.Fatal(err)
	}
	defer reuseResponse.Body.Close()
	if reuseResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected reused token 401, got %d", reuseResponse.StatusCode)
	}
}

func TestManagementReadAndWriteGuards(t *testing.T) {
	s := testServer(t)
	app := fiber.New()
	app.Get("/management", func(c *fiber.Ctx) error { c.Locals("role", "kepala_sekolah"); return s.managementRead(c) }, func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	app.Post("/write", func(c *fiber.Ctx) error { c.Locals("role", "kepala_sekolah"); return s.writable(c) }, func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	read, err := app.Test(httptest.NewRequest(http.MethodGet, "/management", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer read.Body.Close()
	if read.StatusCode != http.StatusNoContent {
		t.Fatalf("expected management read 204, got %d", read.StatusCode)
	}
	write, err := app.Test(httptest.NewRequest(http.MethodPost, "/write", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer write.Body.Close()
	if write.StatusCode != http.StatusForbidden {
		t.Fatalf("expected write 403, got %d", write.StatusCode)
	}
}

func TestGuruCannotManageDifferentClassAttendance(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}, &Kelas{}); err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Wali Kelas", JenisKelamin: "L"}
	otherTutor := Tutor{Nama: "Wali Lain", JenisKelamin: "P"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherTutor).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "guru-idor", Email: "guru-idor@example.test", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 1, NamaRombel: "X", WaliKelasID: &otherTutor.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/kelas/:id", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.canManageKelas(c, c.Params("id"))
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/kelas/"+class.ID, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-class access 403, got %d", response.StatusCode)
	}
}

func TestImportSiswaIsAtomicWhenRowIsInvalid(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &OrangTua{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Import", Tipe: "pusat"}
	year := TahunAjaran{NamaTahunAjaran: "2101/2102", IsAktif: true}
	parent := OrangTua{NamaIbu: "Ibu Import"}
	for _, row := range []any{&pokjar, &year, &parent} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := Kelas{Jenjang: 1, NamaRombel: "I", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	if err := xlsx.SetSheetRow(sheet, "A1", &[]string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"}); err != nil {
		t.Fatal(err)
	}
	if err := xlsx.SetSheetRow(sheet, "A2", &[]string{"Valid", "L", "1001", "9001", "3201000101010001", class.ID, pokjar.ID, parent.ID}); err != nil {
		t.Fatal(err)
	}
	if err := xlsx.SetSheetRow(sheet, "A3", &[]string{"Invalid", "X", "1002", "9002", "3201000101010002", class.ID, pokjar.ID, parent.ID}); err != nil {
		t.Fatal(err)
	}
	var content bytes.Buffer
	if err := xlsx.Write(&content); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "siswa.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content.Bytes()); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	app := fiber.New()
	app.Post("/import", func(c *fiber.Ctx) error { c.Locals("userID", "test-admin"); return s.importSiswa(c) })
	request := httptest.NewRequest(http.MethodPost, "/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected import validation 422, got %d", response.StatusCode)
	}
	var count int64
	s.db.Model(&PesertaDidik{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected atomic rollback, found %d students", count)
	}
}

func TestTutorImportRequiresCoreFieldsAndKeepsOptionalFieldsOptional(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}, &ImportLog{}); err != nil {
		t.Fatal(err)
	}
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	if err := xlsx.SetSheetRow(sheet, "A1", &[]string{"nama", "jenis_kelamin", "no_hp", "alamat", "tanggal_mulai_tugas", "is_rpp_maker"}); err != nil {
		t.Fatal(err)
	}
	if err := xlsx.SetSheetRow(sheet, "A2", &[]string{"Tutor Valid", "L", "08123456789"}); err != nil {
		t.Fatal(err)
	}
	if err := xlsx.SetSheetRow(sheet, "A3", &[]string{"Tutor Tanpa HP", "P", ""}); err != nil {
		t.Fatal(err)
	}
	var content bytes.Buffer
	if err := xlsx.Write(&content); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("tipe", "tutor")
	part, err := writer.CreateFormFile("file", "tutor.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/import", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		c.Locals("role", "admin")
		return s.importTerpusat(c)
	})
	request := httptest.NewRequest(http.MethodPost, "/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected import 200, got %d", response.StatusCode)
	}
	var tutors []Tutor
	if err := s.db.Find(&tutors).Error; err != nil {
		t.Fatal(err)
	}
	if len(tutors) != 1 || tutors[0].Nama != "Tutor Valid" || tutors[0].NoHP != "08123456789" {
		t.Fatalf("expected only valid tutor row to be imported, got %+v", tutors)
	}
	if tutors[0].Alamat != "" || tutors[0].TanggalBertugas != nil {
		t.Fatalf("optional tutor fields should remain empty, got %+v", tutors[0])
	}
}

func TestSiswaLengkapTemplateIncludesBirthDate(t *testing.T) {
	s := testServer(t)
	app := fiber.New()
	app.Get("/template", s.siswaLengkapTemplate)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/template", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected template 200, got %d", response.StatusCode)
	}
	workbook, err := excelize.OpenReader(response.Body)
	if err != nil {
		t.Fatalf("template response is not a valid workbook: %v", err)
	}
	defer workbook.Close()
	rows, err := workbook.GetRows("Peserta Didik Lengkap")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || len(rows[0]) != 17 || rows[0][5] != "tanggal_lahir" {
		t.Fatalf("unexpected siswa lengkap template headers: %#v", rows)
	}
}

func TestSiswaLengkapImportStoresBirthDate(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &OrangTua{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}, &ImportLog{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Tanggal Lahir", Tipe: "pusat"}
	year := TahunAjaran{NamaTahunAjaran: "2102/2103", IsAktif: true}
	if err := s.db.Create(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&year).Error; err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}

	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	headers := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "tanggal_lahir", "kelas", "nama_ayah", "nik_ayah", "pekerjaan_ayah", "pendidikan_ayah", "penghasilan_ayah", "nama_ibu", "nik_ibu", "pekerjaan_ibu", "pendidikan_ibu", "penghasilan_ibu"}
	if err := xlsx.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatal(err)
	}
	row := []string{"Siswa Tanggal Lahir", "L", "390", "1234567890", "3201000101010001", "2018-01-02", "1A", "Ayah Siswa", "", "", "", "", "Ibu Siswa", "", "", "", ""}
	if err := xlsx.SetSheetRow(sheet, "A2", &row); err != nil {
		t.Fatal(err)
	}
	var content bytes.Buffer
	if err := xlsx.Write(&content); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("tipe", "siswa-lengkap"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "siswa-lengkap.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/import", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		c.Locals("role", "admin")
		return s.importTerpusat(c)
	})
	request := httptest.NewRequest(http.MethodPost, "/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("expected import 200, got %d: %s", response.StatusCode, payload)
	}
	payload, _ := io.ReadAll(response.Body)
	var student PesertaDidik
	if err := s.db.Where("nis = ?", "390").First(&student).Error; err != nil {
		t.Fatalf("student was not imported: %v; response: %s", err, payload)
	}
	if student.TanggalLahir == nil || student.TanggalLahir.Format("2006-01-02") != "2018-01-02" {
		t.Fatalf("birth date was not stored, got %v", student.TanggalLahir)
	}
	if student.PokjarID != pokjar.ID {
		t.Fatalf("expected imported student to follow class pokjar %q, got %q", pokjar.ID, student.PokjarID)
	}
}

func TestTutorTemplateUsesRenamedSheet(t *testing.T) {
	s := testServer(t)
	app := fiber.New()
	app.Get("/template", s.tutorTemplate)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/template", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected template 200, got %d", response.StatusCode)
	}
	workbook, err := excelize.OpenReader(response.Body)
	if err != nil {
		t.Fatalf("template response is not a valid workbook: %v", err)
	}
	defer workbook.Close()
	rows, err := workbook.GetRows("Tutor")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || len(rows[0]) != 8 || rows[0][0] != "nama" || rows[0][4] != "tanggal_lahir" || rows[0][6] != "tanggal_mulai_tugas" {
		t.Fatalf("unexpected tutor template headers: %#v", rows)
	}
}

func TestPromotionRejectsTargetClassFromAnotherYear(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Promosi", Tipe: "pusat"}
	firstYear := TahunAjaran{NamaTahunAjaran: "2102/2103", IsAktif: true}
	targetYear := TahunAjaran{NamaTahunAjaran: "2103/2104"}
	for _, row := range []any{&pokjar, &firstYear, &targetYear} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	source := Kelas{Jenjang: 1, NamaRombel: "P", PokjarID: pokjar.ID, TahunAjaranID: firstYear.ID}
	wrongTarget := Kelas{Jenjang: 2, NamaRombel: "P", PokjarID: pokjar.ID, TahunAjaranID: firstYear.ID}
	if err := s.db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&wrongTarget).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Promosi", JenisKelamin: "L", NIS: "2001", NISN: "92001", KelasID: source.ID, PokjarID: pokjar.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/promote", func(c *fiber.Ctx) error { c.Locals("userID", "test-admin"); return s.promote(c) })
	payload := `{"targetTahunAjaranId":"` + targetYear.ID + `","students":[{"id":"` + student.ID + `","targetKelasId":"` + wrongTarget.ID + `","status":"naik"}]}`
	request := httptest.NewRequest(http.MethodPost, "/promote", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected wrong target year 400, got %d", response.StatusCode)
	}
	var unchanged PesertaDidik
	if err := s.db.First(&unchanged, "id = ?", student.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.KelasID != source.ID {
		t.Fatal("student was changed despite rejected promotion")
	}
}

func TestMigratePokjarClassesMovesClassAndStudents(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}); err != nil {
		t.Fatal(err)
	}
	sourcePokjar := Pokjar{NamaPokjar: "Pokjar Asal", Tipe: "binaan"}
	targetPokjar := Pokjar{NamaPokjar: "Pokjar Tujuan", Tipe: "pusat"}
	year := TahunAjaran{NamaTahunAjaran: "2104/2105", IsAktif: true}
	for _, row := range []any{&sourcePokjar, &targetPokjar, &year} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := Kelas{Jenjang: 3, NamaRombel: "A", PokjarID: sourcePokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	students := []PesertaDidik{
		{Nama: "Siswa Satu", JenisKelamin: "L", NIS: "3101", NISN: "93101", KelasID: class.ID, PokjarID: sourcePokjar.ID, Status: "aktif"},
		{Nama: "Siswa Dua", JenisKelamin: "P", NIS: "3102", NISN: "93102", KelasID: class.ID, PokjarID: sourcePokjar.ID, Status: "aktif"},
	}
	if err := s.db.Create(&students).Error; err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Post("/migrate", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.migratePokjarClasses(c)
	})
	payload := `{"sourcePokjarId":"` + sourcePokjar.ID + `","targetPokjarId":"` + targetPokjar.ID + `","kelasIds":["` + class.ID + `"]}`
	request := httptest.NewRequest(http.MethodPost, "/migrate", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected migration 200, got %d", response.StatusCode)
	}

	var migratedClass Kelas
	if err := s.db.First(&migratedClass, "id = ?", class.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migratedClass.PokjarID != targetPokjar.ID {
		t.Fatalf("expected class to move to target pokjar, got %q", migratedClass.PokjarID)
	}
	var migratedStudents []PesertaDidik
	if err := s.db.Where("kelas_id = ?", class.ID).Find(&migratedStudents).Error; err != nil {
		t.Fatal(err)
	}
	if len(migratedStudents) != 2 {
		t.Fatalf("expected two students in migrated class, got %d", len(migratedStudents))
	}
	for _, student := range migratedStudents {
		if student.PokjarID != targetPokjar.ID {
			t.Fatalf("expected student %q to move to target pokjar, got %q", student.Nama, student.PokjarID)
		}
	}
}

func TestSyncStudentPokjarFromClassRepairsLegacyImport(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}); err != nil {
		t.Fatal(err)
	}
	wrongPokjar := Pokjar{NamaPokjar: "Pokjar Default Lama", Tipe: "pusat"}
	classPokjar := Pokjar{NamaPokjar: "Pokjar Kelas", Tipe: "binaan"}
	year := TahunAjaran{NamaTahunAjaran: "2105/2106", IsAktif: true}
	for _, row := range []any{&wrongPokjar, &classPokjar, &year} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := Kelas{Jenjang: 4, NamaRombel: "B", PokjarID: classPokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Legacy", JenisKelamin: "L", NIS: "4101", NISN: "94101", KelasID: class.ID, PokjarID: wrongPokjar.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Post("/sync", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.syncStudentPokjarFromClass(c)
	})
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/sync", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected sync 200, got %d", response.StatusCode)
	}
	var repaired PesertaDidik
	if err := s.db.First(&repaired, "id = ?", student.ID).Error; err != nil {
		t.Fatal(err)
	}
	if repaired.PokjarID != classPokjar.ID {
		t.Fatalf("expected legacy student to follow class pokjar %q, got %q", classPokjar.ID, repaired.PokjarID)
	}
}
