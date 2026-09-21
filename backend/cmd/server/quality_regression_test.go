package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestGradeUjianPesertaIncludesUnansweredQuestions(t *testing.T) {
	db := isolatedTestDB(t, "exam-grade-unanswered")
	if err := db.AutoMigrate(&BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianPeserta{}, &UjianJawaban{}); err != nil {
		t.Fatal(err)
	}

	first := BankSoal{Tipe: "pg", Pertanyaan: "1 + 1?", Kunci: "0", Poin: 1}
	second := BankSoal{Tipe: "pg", Pertanyaan: "2 + 2?", Kunci: "1", Poin: 1}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ujian := Ujian{Judul: "Ujian nilai", WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), DurasiMenit: 60}
	if err := db.Create(&ujian).Error; err != nil {
		t.Fatal(err)
	}
	for _, soal := range []string{first.ID, second.ID} {
		if err := db.Create(&UjianSoal{UjianID: ujian.ID, SoalID: soal, Bobot: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	peserta := UjianPeserta{UjianID: ujian.ID, PesertaDidikID: "student-1", Status: "mulai"}
	if err := db.Create(&peserta).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UjianJawaban{UjianPesertaID: peserta.ID, SoalID: first.ID, Jawaban: "0"}).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	result, err := s.gradeUjianPesertaResult(&peserta, &ujian)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || result.Correct != 1 {
		t.Fatalf("unexpected grade counts: %+v", result)
	}
	if result.Score != 50 {
		t.Fatalf("one correct answer out of two should score 50, got %v", result.Score)
	}
}

func TestPublicExamPagesUseCSPCompatibleHandlers(t *testing.T) {
	for name, page := range map[string]string{"exam": ujianOnlineHTML, "parent": ortuPortalHTML} {
		if strings.Contains(page, "onclick=") || strings.Contains(page, "oninput=") || strings.Contains(page, "onkeydown=") {
			t.Fatalf("%s page still contains inline event handlers blocked by production CSP", name)
		}
	}
	if !strings.Contains(ujianOnlineHTML, `data-action="answer"`) || !strings.Contains(ujianOnlineHTML, `data-action="text-answer"`) {
		t.Fatal("public exam page is missing delegated answer handlers")
	}
	if !strings.Contains(ortuPortalHTML, `data-action="show-tab"`) || !strings.Contains(ortuPortalHTML, `data-action="send-chat"`) {
		t.Fatal("parent portal is missing delegated interaction handlers")
	}
	if !strings.Contains(ortuPortalHTML, `data-action="font-scale"`) || !strings.Contains(ortuPortalHTML, `aria-label="Kontras tinggi"`) {
		t.Fatal("parent portal is missing accessible text and contrast controls")
	}
}

func TestPublicExamSessionCookieAvoidsCredentialQuery(t *testing.T) {
	db := isolatedTestDB(t, "exam-session-cookie")
	if err := db.AutoMigrate(&PesertaDidik{}, &Kelas{}, &MataPelajaran{}, &Ujian{}, &UjianPeserta{}, &UjianSoal{}, &UjianJawaban{}, &BankSoal{}); err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: "pokjar-1", TahunAjaranID: "ta-1"}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Ujian", NISN: "1234567890", NIK: "nik-1", KelasID: kelas.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Ujian cookie", KelasID: kelas.ID, WaktuMulai: now.Add(-time.Minute), WaktuSelesai: now.Add(time.Hour), AksesKode: "ABC123", DurasiMenit: 30}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	question := BankSoal{MapelID: "", Tipe: "pg", Pertanyaan: "1 + 1?", Opsi: `["2","3"]`, Kunci: "0", Poin: 1}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	joinedQuestion := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Bobot: 1}
	if err := db.Create(&joinedQuestion).Error; err != nil {
		t.Fatal(err)
	}
	started := now.Add(-30 * time.Second)
	participant := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Mulai: &started, Status: "mulai"}
	if err := db.Create(&participant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UjianJawaban{UjianPesertaID: participant.ID, SoalID: question.ID, Jawaban: "1"}).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, cfg: Config{AccessSecret: "test-public-exam-secret", Env: "development"}}
	app := fiber.New()
	app.Post("/cek", s.cekUjianOnline)
	app.Get("/ujian/:ujianId/soal", s.getSoalUjianOnline)

	checkReq := httptest.NewRequest(http.MethodPost, "/cek", strings.NewReader("nisn=1234567890&aksesKode=ABC123"))
	checkReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	checkRes, err := app.Test(checkReq)
	if err != nil {
		t.Fatal(err)
	}
	checkBody := readAndClose(t, checkRes)
	if checkRes.StatusCode != http.StatusOK {
		t.Fatalf("exam check failed: %d", checkRes.StatusCode)
	}
	if strings.Contains(checkBody, `"aksesKode"`) || strings.Contains(checkBody, `"dibuatOlehUserId"`) {
		t.Fatalf("public exam listing leaked secret/internal fields: %s", checkBody)
	}
	cookie := checkRes.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, examSessionCookie+"=") {
		t.Fatalf("exam check did not issue an HttpOnly session cookie: %q", cookie)
	}

	soalReq := httptest.NewRequest(http.MethodGet, "/ujian/"+exam.ID+"/soal", nil)
	soalReq.Header.Set("Cookie", cookie)
	soalRes, err := app.Test(soalReq)
	if err != nil {
		t.Fatal(err)
	}
	soalBody := readAndClose(t, soalRes)
	if soalRes.StatusCode != http.StatusOK {
		t.Fatalf("cookie-authenticated exam request failed: %d", soalRes.StatusCode)
	}
	if strings.Contains(soalBody, `"kunci"`) || strings.Contains(soalBody, `"aksesKode"`) {
		t.Fatalf("public exam questions leaked secret fields: %s", soalBody)
	}
	if !strings.Contains(soalBody, `"ujianSoalId":"`+joinedQuestion.ID+`"`) {
		t.Fatalf("saved answer was not mapped back to its public UjianSoal id: %s", soalBody)
	}
	if strings.Contains(soalBody, `"ujianSoalId":"`+question.ID+`"`) {
		t.Fatalf("saved answer still exposes the internal bank-soal id: %s", soalBody)
	}
}

func TestParentExamResultsDoNotLeakAccessCode(t *testing.T) {
	db := isolatedTestDB(t, "parent-exam-result-dto")
	if err := db.AutoMigrate(&User{}, &OrangTua{}, &PesertaDidik{}, &Ujian{}, &UjianPeserta{}, &MataPelajaran{}); err != nil {
		t.Fatal(err)
	}
	parent := OrangTua{NamaIbu: "Wali Siswa"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Anak Wali", NISN: "1234567891", NIK: "nik-parent", KelasID: "kelas-parent", OrangTuaID: parent.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "parent-dto", Role: "orang_tua", OrangTuaID: &parent.ID, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	mapel := MataPelajaran{NamaMapel: "Matematika"}
	if err := db.Create(&mapel).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Ujian selesai", MapelID: mapel.ID, AksesKode: "RAHASIA", WaktuMulai: time.Now().Add(-time.Hour), WaktuSelesai: time.Now().Add(time.Hour)}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	score := 87.5
	participant := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Status: "selesai", Skor: &score}
	if err := db.Create(&participant).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/orang-tua/anak/:id/ujian-skor", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getUjianSkorAnak(c)
	})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/orang-tua/anak/"+student.ID+"/ujian-skor", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("parent exam result failed: %d %s", res.StatusCode, body)
	}
	if strings.Contains(body, `"aksesKode"`) || !strings.Contains(body, `"judul":"Ujian selesai"`) {
		t.Fatalf("parent exam DTO is unsafe or incomplete: %s", body)
	}
}

func TestParentSimulationSummaryHonorsResultPolicy(t *testing.T) {
	db := isolatedTestDB(t, "parent-simulation-summary")
	if err := db.AutoMigrate(&User{}, &OrangTua{}, &PesertaDidik{}, &SimulasiPaket{}, &SimulasiPenugasan{}, &SimulasiUpaya{}); err != nil {
		t.Fatal(err)
	}
	parent := OrangTua{NamaIbu: "Wali Simulasi"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Anak Simulasi", NISN: "1234567893", NIK: "nik-simulasi", KelasID: "kelas-simulasi", OrangTuaID: parent.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "parent-simulation", Role: "orang_tua", OrangTuaID: &parent.ID, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	packageRow := SimulasiPaket{Nama: "Literasi aman", Mode: "anbk_akm", Status: "terbit", DurasiMenit: 30, MaksPercobaan: 2, TampilkanNilai: false, TampilkanRingkasan: false}
	if err := db.Create(&packageRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&SimulasiPenugasan{PaketID: packageRow.ID, PesertaDidikID: student.ID}).Error; err != nil {
		t.Fatal(err)
	}
	score := 91.0
	if err := db.Create(&SimulasiUpaya{PaketID: packageRow.ID, PesertaDidikID: student.ID, Nomor: 1, Status: "selesai", SkorAkhir: &score, SeedUrutan: "seed"}).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	app := fiber.New()
	app.Get("/orang-tua/anak/:id/simulasi", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getSimulasiAnak(c)
	})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/orang-tua/anak/"+student.ID+"/simulasi", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("parent simulation result failed: %d %s", res.StatusCode, body)
	}
	if strings.Contains(body, "91") || strings.Contains(body, "kunci") {
		t.Fatalf("parent response leaked hidden result or internal fields: %s", body)
	}
	if !strings.Contains(body, `"nama":"Literasi aman"`) || !strings.Contains(body, `"status":"selesai"`) {
		t.Fatalf("parent response missing safe simulation summary: %s", body)
	}
}

func TestParentPortalResponsesAllowlistPrivateFields(t *testing.T) {
	db := isolatedTestDB(t, "parent-portal-dto")
	if err := db.AutoMigrate(&User{}, &OrangTua{}, &Pokjar{}, &Kelas{}, &PesertaDidik{}, &MataPelajaran{}, &Materi{}, &Tugas{}, &PengumpulanTugas{}, &RekapNilaiAkhir{}, &CatatanRapor{}, &ChatMessage{}); err != nil {
		t.Fatal(err)
	}
	parent := OrangTua{NamaBapak: "Ayah", NamaIbu: "Ibu", NIKAyah: "nik-ayah-rahasia", NIKIbu: "nik-ibu-rahasia"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar DTO"}
	if err := db.Create(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 3, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: "ta-dto"}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{
		Nama: "Anak DTO", NIS: "NIS-DTO", NISN: "1234567892", NIK: "nik-anak",
		KelasID: kelas.ID, OrangTuaID: parent.ID, Status: "aktif",
	}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "parent-portal-dto", Role: "orang_tua", OrangTuaID: &parent.ID, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	mapel := MataPelajaran{NamaMapel: "Bahasa Indonesia"}
	if err := db.Create(&mapel).Error; err != nil {
		t.Fatal(err)
	}
	shareToken := "SHARE-TOKEN-MUST-NOT-LEAK"
	if err := db.Create(&Materi{
		MapelID: mapel.ID, KelasID: kelas.ID, Judul: "Materi privat", FilePath: "materi/private.pdf",
		ShareToken: &shareToken, DibuatOlehUserID: "operator-private-id",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Tugas{
		MapelID: mapel.ID, KelasID: kelas.ID, Judul: "Tugas privat", FilePath: func() *string { v := "tugas/private.pdf"; return &v }(),
		DibuatOlehUserID: "operator-private-id", Deadline: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&RekapNilaiAkhir{PesertaDidikID: student.ID, KelasID: kelas.ID, MapelID: mapel.ID, TahunAjaranID: "ta-dto", Semester: "Ganjil", NPAkhir: func() *float64 { v := 88.0; return &v }()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&CatatanRapor{PesertaDidikID: student.ID, TahunAjaranID: "ta-dto", Semester: "Ganjil", CatatanWali: "Pertahankan semangat belajar", KenaikanKe: func() *string { v := "4"; return &v }()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ChatMessage{PesertaDidikID: student.ID, PengirimUserID: user.ID, PenerimaUserID: "teacher-internal-id", Isi: "Pesan orang tua"}).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/orang-tua/anak", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.listAnakOrangTua(c)
	})
	app.Get("/orang-tua/anak/:id/materi", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getMateriAnak(c)
	})
	app.Get("/orang-tua/anak/:id/tugas", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getTugasAnak(c)
	})
	app.Get("/orang-tua/anak/:id/nilai", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getNilaiAnak(c)
	})
	app.Get("/orang-tua/anak/:id/rapor", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.getRaporAnak(c)
	})
	app.Get("/orang-tua/anak/:id/chat", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.listChatAnak(c)
	})

	paths := []string{"/orang-tua/anak", "/orang-tua/anak/" + student.ID + "/materi", "/orang-tua/anak/" + student.ID + "/tugas", "/orang-tua/anak/" + student.ID + "/nilai", "/orang-tua/anak/" + student.ID + "/rapor", "/orang-tua/anak/" + student.ID + "/chat"}
	for _, path := range paths {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body := readAndClose(t, res)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("parent portal endpoint failed: path=%s status=%d body=%s", path, res.StatusCode, body)
		}
		for _, secret := range []string{"nik-ayah-rahasia", "nik-ibu-rahasia", "SHARE-TOKEN-MUST-NOT-LEAK", "materi/private.pdf", "tugas/private.pdf", "operator-private-id", "teacher-internal-id", `"pesertaDidik"`, `"tahunAjaran"`} {
			if strings.Contains(body, secret) {
				t.Fatalf("parent endpoint %s leaked private field %q: %s", path, secret, body)
			}
		}
	}
}

func TestSharedMateriPasswordUsesCookieInsteadOfURL(t *testing.T) {
	db := isolatedTestDB(t, "materi-share-cookie")
	if err := db.AutoMigrate(&Materi{}, &MataPelajaran{}); err != nil {
		t.Fatal(err)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("rahasia-share"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	shareToken := "share-token-1"
	materi := Materi{Judul: "Materi privat", ShareToken: &shareToken, SharePasswordHash: func() *string { v := string(passwordHash); return &v }()}
	if err := db.Create(&materi).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:8080")
	s := &Server{db: db, cfg: Config{AccessSecret: "test-materi-share-secret", Env: "development"}}
	app := fiber.New()
	app.Get("/materi/share/:token", s.viewSharedMateri)
	app.Post("/materi/share/:token/unlock", s.unlockSharedMateri)

	gateReq := httptest.NewRequest(http.MethodGet, "/materi/share/"+shareToken, nil)
	gateRes, err := app.Test(gateReq)
	if err != nil {
		t.Fatal(err)
	}
	gateBody := readAndClose(t, gateRes)
	if gateRes.StatusCode != http.StatusOK || !strings.Contains(gateBody, `method="post"`) || strings.Contains(gateBody, "?pwd=") {
		t.Fatalf("protected share gate still exposes password in URL: status=%d body=%s", gateRes.StatusCode, gateBody)
	}

	unlockReq := httptest.NewRequest(http.MethodPost, "/materi/share/"+shareToken+"/unlock", strings.NewReader("pwd=rahasia-share"))
	unlockReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unlockRes, err := app.Test(unlockReq)
	if err != nil {
		t.Fatal(err)
	}
	unlockRes.Body.Close()
	if unlockRes.StatusCode != http.StatusSeeOther {
		t.Fatalf("unlock should redirect to a clean URL, got %d", unlockRes.StatusCode)
	}
	setCookie := unlockRes.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, materiShareSessionCookie+"=") || !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("unlock did not issue an HttpOnly share cookie: %q", setCookie)
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/materi/share/"+shareToken, nil)
	pageReq.Header.Set("Cookie", strings.Split(setCookie, ";")[0])
	pageRes, err := app.Test(pageReq)
	if err != nil {
		t.Fatal(err)
	}
	pageBody := readAndClose(t, pageRes)
	if pageRes.StatusCode != http.StatusOK || !strings.Contains(pageBody, "Materi privat") {
		t.Fatalf("cookie-authenticated share page failed: status=%d body=%s", pageRes.StatusCode, pageBody)
	}
}

func TestStudentVerificationQRUsesSignedTokenAndKeepsLegacyNISN(t *testing.T) {
	db := isolatedTestDB(t, "student-verification-token")
	if err := db.AutoMigrate(&PesertaDidik{}, &Kelas{}, &TahunAjaran{}); err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 2, NamaRombel: "B"}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Siswa Verifikasi", NISN: "9876543210", KelasID: kelas.ID, Status: "aktif"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, cfg: Config{AccessSecret: "test-student-verification-secret-32-chars", Env: "development"}}
	token, err := s.issuePublicStudentVerificationToken(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	if token == student.NISN || !strings.Contains(token, ".") {
		t.Fatalf("student verification token is not signed/opaque: %q", token)
	}
	if got, signed := s.parsePublicStudentVerificationToken(token); !signed || got != student.ID {
		t.Fatalf("signed student token did not resolve to the student: %q signed=%v", got, signed)
	}

	app := fiber.New()
	app.Get("/verify/siswa/:nisn", s.verifySiswa)
	for _, path := range []string{"/verify/siswa/" + token, "/verify/siswa/" + student.NISN} {
		res, requestErr := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		body := readAndClose(t, res)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, "Siswa Verifikasi") {
			t.Fatalf("student verification path failed: path=%s status=%d body=%s", path, res.StatusCode, body)
		}
	}
}

func readAndClose(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
