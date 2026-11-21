package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

func TestReportCalculation(t *testing.T) {
	cps := []ReportCP{}
	scores := []ReportScore{}
	for i, v := range []float64{0, 65, 90, 90, 100, 70, 80} {
		id := fmt.Sprint(i)
		cps = append(cps, ReportCP{Base: Base{ID: id}, Label: "CP" + id})
		value := v
		scores = append(scores, ReportScore{CPID: id, Value: &value})
	}
	value, a, b, complete := reportCalculate(cps, scores)
	if value == nil || *value != 71 || !complete {
		t.Fatalf("calculation: %v complete %v", value, complete)
	}
	if !strings.Contains(a, "CP4, CP2 dan CP3") || !strings.Contains(b, "CP0, CP1 dan CP5") {
		t.Fatalf("ranking: %q / %q", a, b)
	}
	_, _, b, complete = reportCalculate(cps[:2], scores[:1])
	if complete || b != "" {
		t.Fatal("incomplete score set or overlapping growth")
	}
	if value, _, _, complete = reportCalculate(cps, nil); value != nil || complete {
		t.Fatal("missing values must not become zero")
	}
	for i := 0; i < 2; i++ {
		v := 84.5
		scores[i].Value = &v
	}
	value, _, _, _ = reportCalculate(cps[:2], scores[:2])
	if *value != 85 {
		t.Fatal("half rounding must match Excel")
	}
}

// Opt-in isolated browser harness: never opens the user's application database.
func TestReportBrowserServer(t *testing.T) {
	if os.Getenv("RAPOR_BROWSER_SERVER") != "1" {
		t.Skip("browser harness is opt-in")
	}
	s, app, _, k, _, mp := reportFixture(t)
	tutor := Tutor{Nama: "Tutor Mapel Rapor"}
	s.db.Create(&tutor)
	hash, _ := bcryptHash("Guru12345")
	user := User{Username: "guru-rapor", Email: "guru.rapor.fixture@gmail.com", PasswordHash: hash, Role: "guru", TutorID: &tutor.ID, IsActive: true}
	s.db.Create(&user)
	s.db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: k.ID, MapelID: mp.ID})
	app.Static("/", "../../../frontend/dist")
	app.Get("/*", func(c *fiber.Ctx) error { return c.SendFile("../../../frontend/dist/index.html") })
	if err := app.Listen("127.0.0.1:19089"); err != nil {
		t.Fatal(err)
	}
}

func TestReportScopeAndPublication(t *testing.T) {
	s, app, admin, k, students, mp := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Ganjil"
	parent := OrangTua{NamaBapak: "Ayah"}
	s.db.Create(&parent)
	s.db.Model(&PesertaDidik{}).Where("id = ?", students[0].ID).Update("orang_tua_id", parent.ID)
	teacher := Tutor{Nama: "Guru mapel"}
	other := Tutor{Nama: "Guru lain"}
	s.db.Create(&teacher)
	s.db.Create(&other)
	s.db.Create(&PenugasanGuruMapel{TutorID: teacher.ID, KelasID: k.ID, MapelID: mp.ID})
	for _, tc := range []struct {
		role    string
		tutor   *string
		parent  *string
		grid    int
		details int
	}{{"kepala_sekolah", nil, nil, 200, 200}, {"guru", &teacher.ID, nil, 200, 403}, {"guru", &other.ID, nil, 403, 403}, {"orang_tua", nil, &parent.ID, 403, 403}} {
		user := User{Username: "role-" + tc.role + fmt.Sprint(tc.grid), Role: tc.role, TutorID: tc.tutor, OrangTuaID: tc.parent, IsActive: true}
		s.db.Create(&user)
		token, err := s.token(user, s.cfg.AccessSecret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		for path, want := range map[string]int{"/api/rapor-sumatif/grid" + qs + "&mapelId=" + mp.ID: tc.grid, "/api/rapor-sumatif/document/" + students[0].ID + qs: tc.details} {
			res, e := makeRequest(app, "GET", path, token, nil, "")
			if e != nil {
				t.Fatal(e)
			}
			res.Body.Close()
			if res.StatusCode != want {
				t.Fatalf("%s %s = %d want %d", tc.role, path, res.StatusCode, want)
			}
		}
		res, e := makeRequest(app, "PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, token, reportGridInput{CPs: []ReportCP{{Label: "Membaca"}}}, "")
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if tc.role != "guru" || tc.tutor == &other.ID {
			if res.StatusCode != 403 {
				t.Fatalf("%s write allowed: %d", tc.role, res.StatusCode)
			}
		}
		if tc.role == "orang_tua" {
			res, e = makeRequest(app, "GET", "/api/orang-tua/anak/"+students[0].ID+"/rapor-terbit", token, nil, "")
			if e != nil {
				t.Fatal(e)
			}
			data, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if string(data) != "[]" {
				t.Fatalf("draft disclosed to parent: %s", data)
			}
			res, _ = makeRequest(app, "GET", "/api/orang-tua/anak/"+students[1].ID+"/rapor-terbit", token, nil, "")
			res.Body.Close()
			if res.StatusCode != 403 {
				t.Fatal("other child visible")
			}
		}
	}
	// Archived semesters remain read-only, including for admins.
	s.db.Model(&Semester{}).Where("tahun_ajaran_id = ? AND nama_semester = ?", k.TahunAjaranID, "Ganjil").Update("is_archived", true)
	res, _ := makeRequest(app, "PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, admin, reportGridInput{}, "")
	res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("archived write %d", res.StatusCode)
	}
}

func reportFixture(t *testing.T) (*Server, *fiber.App, string, Kelas, []PesertaDidik, MataPelajaran) {
	t.Helper()
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)
	var year TahunAjaran
	var pokjar Pokjar
	s.db.Where("is_aktif = ?", true).First(&year)
	s.db.First(&pokjar)
	teacher := Tutor{Nama: "Tutor Rapor"}
	if err := s.db.Create(&teacher).Error; err != nil {
		t.Fatal(err)
	}
	k := Kelas{Jenjang: 1, NamaRombel: "R", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &teacher.ID}
	if err := s.db.Create(&k).Error; err != nil {
		t.Fatal(err)
	}
	students := []PesertaDidik{{Nama: "Siswa A", NIS: "R-001", NISN: "1234567890", KelasID: k.ID}, {Nama: "Siswa B", NIS: "R-002", NISN: "1234567891", KelasID: k.ID}}
	for i := range students {
		if err := s.db.Create(&students[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	mp := MataPelajaran{NamaMapel: "Bahasa Indonesia", KodeMapel: "BIND", IsActive: true}
	s.db.Create(&mp)
	s.db.Create(&KelasMapel{KelasID: k.ID, MapelID: mp.ID})
	return s, app, token, k, students, mp
}
func TestReportLifecycle(t *testing.T) {
	s, app, token, k, students, mp := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Ganjil&tahunAjaranId=" + k.TahunAjaranID
	call := func(method, path string, body interface{}, status int) []byte {
		t.Helper()
		res, err := makeRequest(app, method, path, token, body, "")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("%s %s = %d want %d: %s", method, path, res.StatusCode, status, data)
		}
		return data
	}
	var state struct {
		Period    ReportPeriod
		Config    ReportConfig
		CanManage bool
		CanInput  []string
	}
	json.Unmarshal(call("GET", "/api/rapor-sumatif/period"+qs, nil, 200), &state)
	if !state.CanManage || len(state.CanInput) != 1 || state.Period.ID != "" {
		t.Fatal("initial scope")
	}
	var grid struct {
		Period ReportPeriod
		CPs    []ReportCP
	}
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{CPs: []ReportCP{{Label: "Membaca"}, {Label: "Menulis"}}}, 200), &grid)
	if len(grid.CPs) != 2 {
		t.Fatal("missing CP IDs")
	}
	v1, v2 := 84.5, 85.5
	scores := []ReportScore{{CPID: grid.CPs[0].ID, StudentID: students[0].ID, Value: &v1}, {CPID: grid.CPs[1].ID, StudentID: students[0].ID, Value: &v2}}
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: grid.Period.Revision, CPs: grid.CPs, Scores: scores}, 200), &grid)
	call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: 0, CPs: grid.CPs}, 409)
	// Reject a score from another class and roll back the period's revision.
	call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: grid.Period.Revision, CPs: grid.CPs, Scores: []ReportScore{{CPID: grid.CPs[0].ID, StudentID: "other-student", Value: &v1}}}, 400)
	var p ReportPeriod
	s.db.First(&p, "id = ?", grid.Period.ID)
	if p.Revision != grid.Period.Revision {
		t.Fatal("failed request changed revision")
	}
	state.Config.Date = "2026-06-01"
	state.Config.Teacher = "Tutor Rapor"
	state.Config.Principal = "Kepala PKBM"
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/config"+qs, fiber.Map{"revision": p.Revision, "config": state.Config}, 200), &p)
	// A valid manual description may exceed the original print box and must
	// remain eligible for review, finalisation and publication.
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: p.Revision, CPs: grid.CPs, Narratives: []ReportNarrative{{StudentID: students[0].ID, Strength: strings.Repeat("Ananda mampu membaca dan memahami teks dengan baik. ", 30), Growth: "Terus berlatih menulis."}}}, 200), &grid)
	p = grid.Period
	docURL := "/api/rapor-sumatif/document/" + students[0].ID
	var result struct {
		Document ReportDocument
		Record   ReportRecord
		Period   ReportPeriod
	}
	json.Unmarshal(call("GET", docURL+qs, nil, 200), &result)
	if len(result.Document.Issues) > 0 || *result.Document.Subjects[0].Score != 85 {
		t.Fatalf("document issues: %v", result.Document.Issues)
	}
	status := func(next string, expected int) {
		data := call("POST", docURL+"/status"+qs, fiber.Map{"revision": p.Revision, "status": next}, expected)
		if expected == 200 {
			json.Unmarshal(data, &p)
		}
	}
	status("final", 409)
	status("ready", 200)
	status("final", 200)
	call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: p.Revision, CPs: grid.CPs, Scores: []ReportScore{{CPID: grid.CPs[0].ID, StudentID: students[0].ID}}}, 409)
	status("published", 200)
	// The report remains bound to its historical class after the student moves.
	s.db.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: students[0].ID, KelasID: k.ID, TahunAjaranID: k.TahunAjaranID})
	s.db.Model(&PesertaDidik{}).Where("id = ?", students[0].ID).Update("nama", "Nama berubah")
	json.Unmarshal(call("GET", docURL+qs, nil, 200), &result)
	if result.Document.Student.Nama != "Siswa A" {
		t.Fatal("published snapshot changed")
	}
	status("draft", 400)
	json.Unmarshal(call("POST", docURL+"/status"+qs, fiber.Map{"revision": p.Revision, "status": "draft", "reason": "Koreksi tutor"}, 200), &p)
	narrative := ReportNarrative{StudentID: students[0].ID, Strength: "Mampu membaca lancar.", Growth: "Terus berlatih menulis."}
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: p.Revision, CPs: grid.CPs, Narratives: []ReportNarrative{narrative}}, 200), &grid)
	p = grid.Period
	v1 = 80
	scores[0].Value = &v1
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: p.Revision, CPs: grid.CPs, Scores: scores}, 200), &grid)
	p = grid.Period
	json.Unmarshal(call("GET", docURL+qs, nil, 200), &result)
	if !result.Document.Subjects[0].Stale {
		t.Fatal("manual description must be marked stale")
	}
	call("GET", "/api/rapor-sumatif/print"+qs+"&studentId="+students[0].ID, nil, 200)
	for _, format := range []string{"xlsx", "pdf"} {
		data := call("GET", "/api/rapor-sumatif/leger"+qs+"&format="+format, nil, 200)
		if len(data) < 500 {
			t.Fatalf("empty %s export", format)
		}
	}
	call("GET", "/api/rapor-sumatif/period?kelasId="+k.ID+"&semester=bad", nil, 400)
	call("GET", "/api/rapor-sumatif/print"+qs+"&studentId="+students[0].ID+"&version=1", nil, 200)
	var revisions int64
	s.db.Model(&ReportRevision{}).Count(&revisions)
	if revisions != 1 {
		t.Fatalf("revision history lost: %d", revisions)
	}
	json.Unmarshal(call("PUT", "/api/rapor-sumatif/grid"+qs+"&mapelId="+mp.ID, reportGridInput{Revision: p.Revision, CPs: grid.CPs, Narratives: []ReportNarrative{narrative}}, 200), &grid)
	p = grid.Period
	status("ready", 200)
	status("final", 200)
	status("published", 200)
	s.db.Model(&ReportRevision{}).Count(&revisions)
	if revisions != 2 {
		t.Fatalf("revision history overwritten: %d", revisions)
	}
}

func TestReportImportTransaction(t *testing.T) {
	s, app, token, k, students, mp := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Genap"
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "IDENTITAS PD")
	f.NewSheet("1 BIND")
	f.NewSheet("CATATAN & PRESENSI")
	f.SetCellValue("IDENTITAS PD", "B4", students[0].Nama)
	f.SetCellValue("IDENTITAS PD", "D4", students[0].NIS)
	f.SetCellValue("IDENTITAS PD", "E4", students[0].NISN)
	f.SetCellValue("1 BIND", "A2", mp.NamaMapel)
	f.SetCellValue("1 BIND", "F6", "Membaca")
	f.SetCellValue("1 BIND", "C7", students[0].NIS)
	f.SetCellValue("1 BIND", "D7", students[0].NISN)
	f.SetCellValue("1 BIND", "F7", 0)
	f.SetCellValue("CATATAN & PRESENSI", "F4", 2)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	preview := func(status int) []byte {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "rapor.xlsm")
		part.Write(buf.Bytes())
		writer.Close()
		req := httptest.NewRequest("POST", "/api/rapor-sumatif/import/preview"+qs, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		res, e := app.Test(req, -1)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("preview %d: %s", res.StatusCode, data)
		}
		return data
	}
	var v struct {
		ID      string
		Preview reportImportPayload
	}
	json.Unmarshal(preview(200), &v)
	if v.Preview.Students[0].StudentID != students[0].ID {
		t.Fatal("NIS/NISN match failed")
	}
	var count int64
	s.db.Model(&ReportScore{}).Count(&count)
	if count != 0 {
		t.Fatal("preview saved academic data")
	}
	body := fiber.Map{"revision": 0, "students": map[int]string{4: students[0].ID}, "subjects": map[string]string{"1 BIND": mp.ID}}
	res, err := makeRequest(app, "POST", "/api/rapor-sumatif/import/"+v.ID+"/confirm"+qs, token, body, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("confirm %d: %s", res.StatusCode, data)
	}
	var score ReportScore
	s.db.First(&score)
	if score.Value == nil || *score.Value != 0 {
		t.Fatal("zero imported as blank")
	}
	preview(409)
}

func TestReportReferenceWorkbook(t *testing.T) {
	path := os.Getenv("RAPOR_REFERENCE_WORKBOOK")
	if path == "" {
		t.Skip("reference workbook not provided")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, app, token, k, _, _ := reportFixture(t)
	value := func(sheet, cell string) string { v, _ := f.GetCellValue(sheet, cell); return strings.TrimSpace(v) }
	year := value("CETAK RAPOR", "AA14")
	if err = s.db.Model(&TahunAjaran{}).Where("id = ?", k.TahunAjaranID).Update("nama_tahun_ajaran", year).Error; err != nil {
		t.Fatal(err)
	}
	for row := 4; row <= 1000; row++ {
		name := value("IDENTITAS PD", fmt.Sprintf("B%d", row))
		if name == "" {
			continue
		}
		pd := PesertaDidik{Nama: name, NIS: value("IDENTITAS PD", fmt.Sprintf("D%d", row)), NISN: value("IDENTITAS PD", fmt.Sprintf("E%d", row)), KelasID: k.ID}
		if err = s.db.Create(&pd).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, sheet := range f.GetSheetList() {
		if strings.HasPrefix(sheet, "CETAK") || sheet == "IDENTITAS PD" || sheet == "LEGER" || sheet == "CATATAN & PRESENSI" {
			continue
		}
		name := value(sheet, "A2")
		if name == "" {
			continue
		}
		var mp MataPelajaran
		if s.db.Where("nama_mapel = ?", name).First(&mp).Error != nil {
			mp = MataPelajaran{NamaMapel: name, IsActive: true}
			if err = s.db.Create(&mp).Error; err != nil {
				t.Fatal(err)
			}
		}
		var n int64
		s.db.Model(&KelasMapel{}).Where("kelas_id = ? AND mapel_id = ?", k.ID, mp.ID).Count(&n)
		if n == 0 {
			s.db.Create(&KelasMapel{KelasID: k.ID, MapelID: mp.ID})
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "reference.xlsm")
	part.Write(data)
	writer.Close()
	req := httptest.NewRequest("POST", "/api/rapor-sumatif/import/preview?kelasId="+k.ID+"&semester=Genap", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var result struct {
		ID      string
		Preview reportImportPayload
	}
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || len(result.Preview.Students) != 20 || len(result.Preview.Subjects) != 10 || len(result.Preview.Issues) > 0 {
		t.Fatalf("reference contract: status=%d students=%d subjects=%d issues=%v", res.StatusCode, len(result.Preview.Students), len(result.Preview.Subjects), result.Preview.Issues)
	}
	for _, mp := range result.Preview.Subjects {
		if len(mp.Issues) > 0 {
			t.Fatalf("%s: %v", mp.Sheet, mp.Issues)
		}
	}
	confirmed, err := makeRequest(app, "POST", "/api/rapor-sumatif/import/"+result.ID+"/confirm?kelasId="+k.ID+"&semester=Genap", token, fiber.Map{"revision": 0}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer confirmed.Body.Close()
	if confirmed.StatusCode != 200 {
		data, _ := io.ReadAll(confirmed.Body)
		t.Fatalf("reference confirmation %d: %s", confirmed.StatusCode, data)
	}
	var imported ReportPeriod
	s.db.Where("kelas_id = ? AND semester = ?", k.ID, "Genap").First(&imported)
	var printConfig ReportConfig
	reportDecode(imported.Config, &printConfig)
	if printConfig.Date != "2026-06-01" || printConfig.Teacher != value("CETAK RAPOR", "T68") || printConfig.Subjects[0].Name != value("1 PAI", "A2") {
		t.Fatal("historical print settings/order not preserved")
	}
}

func TestReportPDFLayout(t *testing.T) {
	d := ReportDocument{Student: PesertaDidik{Nama: "Siswa Contoh", NIS: "001", NISN: "1234567890"}, Class: Kelas{Jenjang: 1, TahunAjaran: TahunAjaran{NamaTahunAjaran: "2025/2026"}}, Scope: ReportScope{Semester: "Genap"}, Config: ReportConfig{Program: "PAKET A", Phase: "A", Date: "2026-06-01", City: "Purbalingga", Teacher: "Tutor Contoh, S.Pd.", Principal: "Kepala PKBM", TeacherNIP: "-", PrincipalNIP: "-"}, Details: ReportDetails{Promotion: "naik", NextClass: "II (Dua)", Note: "Terus semangat belajar dan pertahankan kebiasaan baik."}}
	names := []string{"Pendidikan Agama dan Budi Pekerti", "Pendidikan Pancasila", "Bahasa Indonesia", "Matematika", "Pendidikan Jasmani, Olahraga dan Kesehatan", "Seni dan Budaya", "Muatan Lokal (Bahasa Arab)", "Muatan Lokal (Baca Tulis Al-Qur'an)", "Pemberdayaan", "Keterampilan"}
	for i, name := range names {
		v := 85.0
		group := "umum"
		if i > 7 {
			group = "pemberdayaan"
		}
		d.Subjects = append(d.Subjects, ReportSubject{ReportSubjectConfig: ReportSubjectConfig{MapelID: fmt.Sprint(i), Name: name, Group: group}, Score: &v, Strength: "Ananda menunjukkan penguasaan yang sangat baik dalam hal membaca dan memahami materi.", Growth: "Perlu bimbingan dan pendampingan lebih lanjut dalam hal menerapkan materi secara mandiri.", Complete: true})
	}
	app := fiber.New()
	app.Get("/pdf", func(c *fiber.Ctx) error { return reportSendPDF(c, []ReportDocument{d}, "lengkap") })
	res, err := app.Test(httptest.NewRequest("GET", "/pdf", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("PDF failed: %d %s", res.StatusCode, data[:min(300, len(data))])
	}
	if directory := os.Getenv("RAPOR_QA_OUTPUT"); directory != "" {
		if err = os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(directory, "rapor-app.pdf"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	pages := regexp.MustCompile(`/Type /Page\b`)
	if got := len(pages.FindAll(data, -1)); got != 7 {
		t.Fatalf("padded bundle has %d pages, want 7", got)
	}
	for _, name := range []string{"Ilmu Pengetahuan Alam", "Ilmu Pengetahuan Sosial", "Bahasa Inggris", "Proyek Pemberdayaan"} {
		value := 86.0
		group := "umum"
		if name == "Proyek Pemberdayaan" {
			group = "pemberdayaan"
		}
		d.Subjects = append(d.Subjects, ReportSubject{ReportSubjectConfig: ReportSubjectConfig{MapelID: name, Name: name, Group: group}, Score: &value, Strength: "Ananda memahami dan menerapkan materi.", Growth: "Terus berlatih secara mandiri.", Complete: true})
	}
	d.Class.Jenjang = 7
	d.Config.Program = "PAKET B"
	d.Config.Phase = "D"
	res, err = app.Test(httptest.NewRequest("GET", "/pdf", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	upper, _ := io.ReadAll(res.Body)
	if got := len(pages.FindAll(upper, -1)); got != 9 {
		t.Fatalf("upper class bundle has %d pages, want 9", got)
	}
	if directory := os.Getenv("RAPOR_QA_OUTPUT"); directory != "" {
		if err = os.WriteFile(filepath.Join(directory, "rapor-upper.pdf"), upper, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
