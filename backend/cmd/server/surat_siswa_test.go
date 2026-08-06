package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func suratZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func suratMultipart(t *testing.T, zipBytes []byte, kelasID string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("judul", "Surat Berkelakuan Baik")
	_ = writer.WriteField("cakupan", "kelas")
	_ = writer.WriteField("kelasId", kelasID)
	_ = writer.WriteField("pesertaDidikIds", "[]")
	part, err := writer.CreateFormFile("file", "surat.zip")
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

func TestUploadSuratSiswaMatchesZipByNISN(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &SuratSiswa{}, &SuratSiswaFile{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Surat"}
	year := TahunAjaran{NamaTahunAjaran: "2099/2100"}
	if err := s.db.Create(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&year).Error; err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 6, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	students := []PesertaDidik{
		{Nama: "Anak Satu", NIS: "NIS-1", NISN: "111", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"},
		{Nama: "Anak Dua", NIS: "NIS-2", NISN: "222", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"},
	}
	for i := range students {
		if err := s.db.Create(&students[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		var files []SuratSiswaFile
		s.db.Find(&files)
		for _, file := range files {
			removeUpload(file.FilePath)
		}
	})

	app := fiber.New()
	app.Post("/surat-siswa", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.uploadSuratSiswa(c)
	})
	zipBytes := suratZip(t, map[string]string{"111.pdf": "%PDF-1.7 siswa satu", "222.pdf": "%PDF-1.7 siswa dua"})
	body, contentType := suratMultipart(t, zipBytes, kelas.ID)
	request := httptest.NewRequest(http.MethodPost, "/surat-siswa", body)
	request.Header.Set("Content-Type", contentType)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("expected 201, got %d: %s", response.StatusCode, data)
	}
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["uploadedCount"] != float64(2) {
		t.Fatalf("expected two uploaded files, got %#v", result["uploadedCount"])
	}
	var count int64
	s.db.Model(&SuratSiswaFile{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected two mapped files, got %d", count)
	}
}

func TestUploadSuratSiswaRejectsUnknownNISN(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &SuratSiswa{}, &SuratSiswaFile{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Surat Unknown"}
	year := TahunAjaran{NamaTahunAjaran: "2099/2100-unknown"}
	s.db.Create(&pokjar)
	s.db.Create(&year)
	kelas := Kelas{Jenjang: 6, NamaRombel: "B", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	s.db.Create(&kelas)
	s.db.Create(&PesertaDidik{Nama: "Anak Target", NIS: "NIS-3", NISN: "333", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"})
	app := fiber.New()
	app.Post("/surat-siswa", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.uploadSuratSiswa(c)
	})
	body, contentType := suratMultipart(t, suratZip(t, map[string]string{"999.pdf": "%PDF-1.7 wrong"}), kelas.ID)
	request := httptest.NewRequest(http.MethodPost, "/surat-siswa", body)
	request.Header.Set("Content-Type", contentType)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown NISN, got %d", response.StatusCode)
	}
	var count int64
	s.db.Model(&SuratSiswa{}).Count(&count)
	if count != 0 {
		t.Fatalf("invalid upload must not create publication, got %d", count)
	}
}
