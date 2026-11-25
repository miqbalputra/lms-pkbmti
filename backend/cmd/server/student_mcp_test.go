package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"pkbm-lms/backend/internal/studentmcp"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type studentMCPFixture struct {
	app     *fiber.App
	db      *gorm.DB
	key     string
	student PesertaDidik
	year    TahunAjaran
}

func setupStudentMCP(t *testing.T, key, write, mode string) studentMCPFixture {
	t.Helper()
	t.Setenv("STUDENT_MCP_API_KEY", key)
	t.Setenv("STUDENT_MCP_ALLOW_WRITE", write)
	db := isolatedTestDB(t, "student-mcp")
	s := &Server{db: db, cfg: Config{Env: mode, AccessSecret: "test-only-preview-signing-secret-123456"}}
	if err := db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}, &PresensiDetail{}, &NilaiCP{}, &NilaiUM{}, &AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureTemporaryNISNIndex(); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Uji"}
	year := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	studentMCPSeed(t, db, &pokjar, &year)
	class := Kelas{Jenjang: 4, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	studentMCPSeed(t, db, &class)
	student := PesertaDidik{Nama: "Peserta Lama", JenisKelamin: "L", NIS: "LAMA-01", NISN: "0000000123", NIK: "1111111111111111",
		KelasID: class.ID, PokjarID: class.PokjarID, OrangTuaID: "ortu-uji", Urutan: 7, Status: "aktif"}
	studentMCPSeed(t, db, &student)
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	api := app.Group("/api")
	s.registerStudentMCP(api)
	api.Get("/peserta-didik", s.auth, s.listSiswa)
	return studentMCPFixture{app: app, db: db, key: key, student: student, year: year}
}

func studentMCPSeed(t *testing.T, db *gorm.DB, rows ...any) {
	t.Helper()
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func (f studentMCPFixture) request(t *testing.T, endpoint string, input any, status int, output any) []byte {
	t.Helper()
	method := http.MethodGet
	var body io.Reader
	if input != nil {
		method = http.MethodPost
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, studentmcp.APIPath+endpoint, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.key)
	return studentMCPCheckResponse(t, f.app, req, status, output)
}

func studentMCPCheckResponse(t *testing.T, app *fiber.App, req *http.Request, status int, output any) []byte {
	t.Helper()
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("%s: status %d, want %d. %s", req.URL.Path, res.StatusCode, status, data)
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			t.Fatal(err)
		}
	}
	return data
}

func (f studentMCPFixture) batch() studentmcp.Batch {
	return studentmcp.Batch{
		Updates: []studentmcp.Update{{StudentID: f.student.ID, ExpectedName: f.student.Nama, ExpectedClassID: f.student.KelasID, NIS: "BARU-01", NISN: "0000000456"}},
		Creates: []studentmcp.Create{{Nama: "Peserta Baru", JenisKelamin: "P", NIK: "2222222222222222", NIS: "BARU-02", TanggalLahir: "2015-02-03", KelasID: f.student.KelasID}},
	}
}

func TestStudentMCPUpdatePreservesHistoryAndRelatedRows(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{21}, 32))
	f := setupStudentMCP(t, key, "true", "test")
	studentID := f.student.ID
	history := RiwayatKelasPesertaDidik{PesertaDidikID: studentID, KelasID: f.student.KelasID, TahunAjaranID: f.year.ID, Status: "aktif"}
	presensi := PresensiDetail{PesertaDidikID: studentID, PresensiID: "presensi-uji", StatusKehadiran: "hadir", Catatan: "Catatan uji"}
	score := 86.0
	gradeCP := NilaiCP{TemaID: "tema-uji", PesertaDidikID: studentID, UrutanCP: 1, NilaiKeterampilan: &score}
	gradeUM := NilaiUM{TemaID: "tema-uji", PesertaDidikID: studentID, NilaiUM: &score}
	studentMCPSeed(t, f.db, &history, &presensi, &gradeCP, &gradeUM)
	var classes studentmcp.Classes
	f.request(t, "/classes", nil, 200, &classes)
	if len(classes.Kelas) != 1 || classes.Kelas[0].ID != f.student.KelasID || classes.Kelas[0].TahunAjaran != f.year.NamaTahunAjaran {
		t.Fatalf("active classes: %+v", classes)
	}
	var search studentmcp.SearchResponse
	query := studentmcp.SearchRequest{Queries: []studentmcp.Query{{Nama: strings.ToLower(f.student.Nama), KelasID: f.student.KelasID}, {NIK: f.student.NIK}}}
	data := f.request(t, "/search", query, 200, &search)
	if len(search.Matches) != 2 || len(search.Matches[0].Students) != 1 || len(search.Matches[1].Students) != 1 || search.Matches[0].Students[0].ID != studentID || bytes.Contains(data, []byte(f.student.NIK)) {
		t.Fatalf("unexpected results or NIK disclosure: %s", data)
	}
	batch := f.batch()
	var preview studentmcp.Preview
	data = f.request(t, "/preview", batch, 200, &preview)
	if preview.PreviewToken == "" || len(preview.Issues) != 0 || len(preview.Changes) != 1 || len(preview.NewStudents) != 1 ||
		preview.Changes[0].Before.NISN != f.student.NISN || preview.Changes[0].After.NISN != batch.Updates[0].NISN || bytes.Contains(data, []byte(batch.Creates[0].NIK)) {
		t.Fatalf("unexpected preview: %s", data)
	}
	var before PesertaDidik
	if err := f.db.First(&before, "id = ?", studentID).Error; err != nil || before.NIS != f.student.NIS {
		t.Fatalf("preview changed stored NIS: %v", err)
	}
	var receipt studentmcp.Receipt
	request := studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}
	f.request(t, "/apply", request, 200, &receipt)
	if receipt.Updated != 1 || receipt.Created != 1 || receipt.BatchID == "" {
		t.Fatalf("bad receipt: %+v", receipt)
	}
	f.request(t, "/apply", request, 409, nil)
	var updated, created PesertaDidik
	if err := f.db.First(&updated, "id = ?", studentID).Error; err != nil {
		t.Fatal(err)
	}
	before.NIS, before.NISN, before.UpdatedAt = batch.Updates[0].NIS, batch.Updates[0].NISN, updated.UpdatedAt
	if !reflect.DeepEqual(before, updated) {
		t.Fatal("the old student changed outside the reviewed identifiers")
	}
	if err := f.db.First(&created, "nis = ?", batch.Creates[0].NIS).Error; err != nil || created.ID == studentID || created.PokjarID != f.student.PokjarID ||
		created.NISN != temporaryNISN || created.TanggalLahir == nil || created.TanggalLahir.Format(time.DateOnly) != batch.Creates[0].TanggalLahir {
		t.Fatalf("new student or derived references are incorrect: %v", err)
	}
	var newHistory RiwayatKelasPesertaDidik
	if err := f.db.First(&newHistory, "peserta_didik_id = ?", created.ID).Error; err != nil || newHistory.TahunAjaranID != f.year.ID {
		t.Fatalf("missing new student's history: %v", err)
	}
	studentMCPAssertRowUnchanged(t, f.db, presensi, presensi.ID)
	studentMCPAssertRowUnchanged(t, f.db, gradeCP, gradeCP.ID)
	studentMCPAssertRowUnchanged(t, f.db, gradeUM, gradeUM.ID)
	studentMCPAssertRowUnchanged(t, f.db, history, history.ID)
	var audit AuditLog
	if err := f.db.First(&audit, "action = ?", studentMCPAction).Error; err != nil || !strings.Contains(audit.Detail, receipt.BatchID) || strings.Contains(audit.Detail, before.NISN) {
		t.Fatalf("missing or unsafe audit receipt: %v", err)
	}
}

func studentMCPAssertRowUnchanged[T any](t *testing.T, db *gorm.DB, expected T, id string) {
	t.Helper()
	var actual T
	if err := db.First(&actual, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	oldJSON, _ := json.Marshal(expected)
	newJSON, _ := json.Marshal(actual)
	if !bytes.Equal(oldJSON, newJSON) {
		t.Fatalf("a linked %T row changed", expected)
	}
}

func TestStudentMCPRejectsStaleTamperedAndConflictingChanges(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{22}, 32))
	f := setupStudentMCP(t, key, "true", "test")
	batch := f.batch()
	var preview studentmcp.Preview
	f.request(t, "/preview", batch, 200, &preview)
	if preview.PreviewToken == "" {
		t.Fatal("valid batch was not signed")
	}
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: "1." + strings.SplitN(preview.PreviewToken, ".", 2)[1]}, 409, nil)
	changed := f.batch()
	changed.Updates[0].NIS = "DITAMBAH"
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: changed, PreviewToken: preview.PreviewToken}, 409, nil)
	if err := f.db.Model(&PesertaDidik{}).Where("id = ?", f.student.ID).Update("urutan", 9).Error; err != nil {
		t.Fatal(err)
	}
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}, 409, nil)
	conflict := f.batch()
	conflict.Creates[0].NIS = f.student.NIS
	preview = studentmcp.Preview{}
	f.request(t, "/preview", conflict, 200, &preview)
	if preview.PreviewToken != "" || len(preview.Issues) != 1 || preview.Issues[0].Section != "creates" {
		t.Fatalf("existing NIS conflict was not blocked: %+v", preview)
	}
	conflict = f.batch()
	conflict.Creates[0].NISN = conflict.Updates[0].NISN
	preview = studentmcp.Preview{}
	f.request(t, "/preview", conflict, 200, &preview)
	if preview.PreviewToken != "" || len(preview.Issues) != 1 {
		t.Fatalf("duplicate NISN in batch was not blocked: %+v", preview)
	}
	conflict = f.batch()
	conflict.Updates[0].NISN = temporaryNISN
	preview = studentmcp.Preview{}
	f.request(t, "/preview", conflict, 200, &preview)
	if preview.PreviewToken != "" || len(preview.Issues) != 1 || preview.Issues[0].Section != "updates" {
		t.Fatalf("an official NISN could be replaced with the placeholder: %+v", preview)
	}
	var old PesertaDidik
	if err := f.db.First(&old, "id = ?", f.student.ID).Error; err != nil || old.NIS != f.student.NIS || old.NISN != f.student.NISN {
		t.Fatalf("a rejected batch changed the existing record: %v", err)
	}
}

func TestStudentMCPRollsBackOnAuditFailure(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{23}, 32))
	f := setupStudentMCP(t, key, "true", "test")
	batch := f.batch()
	var preview studentmcp.Preview
	f.request(t, "/preview", batch, 200, &preview)
	if err := f.db.Exec(`CREATE TRIGGER student_mcp_audit_failure BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'audit failed'); END`).Error; err != nil {
		t.Fatal(err)
	}
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}, 500, nil)
	var old PesertaDidik
	if err := f.db.First(&old, "id = ?", f.student.ID).Error; err != nil || old.NIS != f.student.NIS || old.NISN != f.student.NISN {
		t.Fatalf("update escaped a failed transaction: %v", err)
	}
	if err := f.db.First(&PesertaDidik{}, "nis = ?", batch.Creates[0].NIS).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("create escaped a failed transaction: %v", err)
	}
	var histories int64
	if err := f.db.Model(&RiwayatKelasPesertaDidik{}).Count(&histories).Error; err != nil || histories != 0 {
		t.Fatalf("class history escaped a failed transaction: %v", err)
	}
}

func TestStudentMCPAuthAndWriteSwitch(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{24}, 32))
	t.Run("disabled", func(t *testing.T) {
		f := setupStudentMCP(t, "", "true", "test")
		f.key = key
		f.request(t, "/classes", nil, 404, nil)
	})
	t.Run("scoped read only", func(t *testing.T) {
		f := setupStudentMCP(t, key, "false", "test")
		req := httptest.NewRequest("GET", studentmcp.APIPath+"/classes?key="+key, nil)
		studentMCPCheckResponse(t, f.app, req, 401, nil)
		req = httptest.NewRequest("GET", "/api/peserta-didik", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		studentMCPCheckResponse(t, f.app, req, 401, nil)
		var review studentmcp.Preview
		f.request(t, "/preview", f.batch(), 200, &review)
		if review.WriteEnabled {
			t.Fatal("write enabled by default")
		}
		f.request(t, "/apply", studentmcp.ApplyRequest{Changes: f.batch(), PreviewToken: review.PreviewToken}, 403, nil)
		f.request(t, "/preview", map[string]any{"nama": "kolom tidak diizinkan"}, 400, nil)
	})
	t.Run("production HTTPS only", func(t *testing.T) {
		f := setupStudentMCP(t, key, "false", "production")
		f.request(t, "/classes", nil, 403, nil)
		req := httptest.NewRequest("GET", studentmcp.APIPath+"/classes", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("X-Forwarded-Proto", "https")
		studentMCPCheckResponse(t, f.app, req, 200, nil)
		req.Header.Set("Origin", "https://example.com")
		studentMCPCheckResponse(t, f.app, req, 403, nil)
	})
}
