package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func identitasTestJPEG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.RGBA{R: 20, G: 100, B: 200, A: 255})
	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func identitasTestZIP(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func identitasUploadRequest(t *testing.T, zipBytes []byte, kelasID string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("kelasId", kelasID); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "identitas.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(zipBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func identityTestServer(t *testing.T) *Server {
	t.Helper()
	s := testServer(t)
	if err := s.db.AutoMigrate(&Kelas{}, &PesertaDidik{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var students []PesertaDidik
		s.db.Find(&students)
		for _, student := range students {
			if student.IdentitasFilePath != nil {
				removeUpload(*student.IdentitasFilePath)
			}
		}
	})
	return s
}

func TestUploadIdentitasSiswaZipMapsClassAndReplacesFile(t *testing.T) {
	s := identityTestServer(t)
	kelasA := Kelas{Jenjang: 6, NamaRombel: "A", PokjarID: "pokjar-identity-a", TahunAjaranID: "year-identity-a"}
	kelasB := Kelas{Jenjang: 6, NamaRombel: "B", PokjarID: "pokjar-identity-b", TahunAjaranID: "year-identity-b"}
	if err := s.db.Create(&kelasA).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&kelasB).Error; err != nil {
		t.Fatal(err)
	}
	oldPath := "uploads/identitas-siswa/old-identity-test.pdf"
	if err := os.MkdirAll("./uploads/identitas-siswa", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("./"+oldPath, []byte("%PDF-1.7 old"), 0o640); err != nil {
		t.Fatal(err)
	}
	oldExt := "pdf"
	student := PesertaDidik{Nama: "Anak Satu", NIS: "NIS-ID-1", NISN: "0012345678", KelasID: kelasA.ID, Status: "aktif", IdentitasFilePath: &oldPath, IdentitasFileExt: &oldExt}
	other := PesertaDidik{Nama: "Anak Lain", NIS: "NIS-ID-2", NISN: "0098765432", KelasID: kelasB.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Post("/identitas-siswa/zip", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.uploadIdentitasSiswaZip(c)
	})
	photo := identitasTestJPEG(t)
	body, contentType := identitasUploadRequest(t, identitasTestZIP(t, map[string][]byte{
		"0012345678.jpg": photo,
		"0055555555.pdf": []byte("%PDF-1.7 unknown"),
	}), kelasA.ID)
	request := httptest.NewRequest(http.MethodPost, "/identitas-siswa/zip", body)
	request.Header.Set("Content-Type", contentType)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, data)
	}
	var result struct {
		UploadedCount  int      `json:"uploadedCount"`
		ReplacedCount  int      `json:"replacedCount"`
		UnmatchedNISNs []string `json:"unmatchedNISNs"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.UploadedCount != 1 || result.ReplacedCount != 1 || len(result.UnmatchedNISNs) != 1 || result.UnmatchedNISNs[0] != "0055555555" {
		t.Fatalf("unexpected upload result: %#v", result)
	}
	if err := s.db.First(&student, "id = ?", student.ID).Error; err != nil {
		t.Fatal(err)
	}
	if student.IdentitasFilePath == nil || student.IdentitasFileExt == nil || *student.IdentitasFileExt != "jpg" || *student.IdentitasFilePath == oldPath {
		t.Fatalf("identity document was not replaced: %#v", student)
	}
	if _, err := os.Stat("./" + *student.IdentitasFilePath); err != nil {
		t.Fatalf("new identity file missing: %v", err)
	}
	savedPhoto, err := os.ReadFile("./" + *student.IdentitasFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(savedPhoto, photo) {
		t.Fatal("foto harus disimpan tanpa resize, konversi, atau kompresi ulang")
	}
	if _, err := os.Stat("./" + oldPath); !os.IsNotExist(err) {
		t.Fatalf("old identity file should be removed, stat error: %v", err)
	}
	if err := s.db.First(&other, "id = ?", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if other.IdentitasFilePath != nil {
		t.Fatal("student outside selected class must not be updated")
	}
	var auditCount int64
	if err := s.db.Model(&AuditLog{}).Where("resource = ?", "identitas_siswa").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one identity audit log, got %d", auditCount)
	}
}

func TestUploadIdentitasSiswaZipRejectsUnsafeArchiveAndKeepsStudent(t *testing.T) {
	s := identityTestServer(t)
	kelas := Kelas{Jenjang: 7, NamaRombel: "A", PokjarID: "pokjar-identity-invalid", TahunAjaranID: "year-identity-invalid"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Anak Aman", NIS: "NIS-ID-3", NISN: "0011223344", KelasID: kelas.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Post("/identitas-siswa/zip", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.uploadIdentitasSiswaZip(c)
	})
	body, contentType := identitasUploadRequest(t, identitasTestZIP(t, map[string][]byte{
		"nested/0011223344.jpg": identitasTestJPEG(t),
	}), kelas.ID)
	request := httptest.NewRequest(http.MethodPost, "/identitas-siswa/zip", body)
	request.Header.Set("Content-Type", contentType)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("expected 400, got %d: %s", response.StatusCode, data)
	}
	if err := s.db.First(&student, "id = ?", student.ID).Error; err != nil {
		t.Fatal(err)
	}
	if student.IdentitasFilePath != nil || student.IdentitasFileExt != nil {
		t.Fatal("unsafe archive must not update student")
	}
}

func TestDownloadIdentitasAnakIsParentScoped(t *testing.T) {
	s := identityTestServer(t)
	if err := s.db.AutoMigrate(&OrangTua{}); err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 8, NamaRombel: "A", PokjarID: "pokjar-identity-parent", TahunAjaranID: "year-identity-parent"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	parent := OrangTua{NamaBapak: "Wali Satu"}
	otherParent := OrangTua{NamaBapak: "Wali Dua"}
	if err := s.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherParent).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "wali-identity", Role: "orang_tua", OrangTuaID: &parent.ID, IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	path := "uploads/identitas-siswa/parent-scope-test.jpg"
	if err := os.MkdirAll("./uploads/identitas-siswa", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("./"+path, identitasTestJPEG(t), 0o640); err != nil {
		t.Fatal(err)
	}
	ext := "jpg"
	child := PesertaDidik{Nama: "Anak Wali", NIS: "NIS-ID-4", NISN: "0044556677", KelasID: kelas.ID, OrangTuaID: parent.ID, Status: "aktif", IdentitasFilePath: &path, IdentitasFileExt: &ext}
	otherChild := PesertaDidik{Nama: "Anak Lain", NIS: "NIS-ID-5", NISN: "0077665544", KelasID: kelas.ID, OrangTuaID: otherParent.ID, Status: "aktif", IdentitasFilePath: &path, IdentitasFileExt: &ext}
	if err := s.db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&otherChild).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeUpload(path) })

	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/orang-tua/anak/:id/identitas/download", func(c *fiber.Ctx) error {
		c.Locals("userID", user.ID)
		return s.downloadIdentitasAnak(c)
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/orang-tua/anak/"+child.ID+"/identitas/download", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, data)
	}
	if got := response.Header.Get("Content-Disposition"); got != `attachment; filename="0044556677.jpg"` {
		t.Fatalf("unexpected download name: %q", got)
	}
	if response.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected content type: %q", response.Header.Get("Content-Type"))
	}
	denied, err := app.Test(httptest.NewRequest(http.MethodGet, "/orang-tua/anak/"+otherChild.ID+"/identitas/download", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		data, _ := io.ReadAll(denied.Body)
		t.Fatalf("expected IDOR attempt to be denied, got %d: %s", denied.StatusCode, data)
	}
}
