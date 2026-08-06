package main

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func TestExportSiswaIncludesStudentDataWithoutParentData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:siswa-export?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Tutor{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &PesertaDidik{}, &Program{}); err != nil {
		t.Fatal(err)
	}

	tutor := Tutor{Nama: "Tutor Export", JenisKelamin: "L"}
	pokjar := Pokjar{NamaPokjar: "Pokjar Export", Tipe: "pusat"}
	tahun := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	program := Program{Kode: "B", Nama: "Paket B"}
	for _, row := range []any{&tutor, &pokjar, &tahun, &program} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	kelas := Kelas{
		Jenjang: 3, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: tahun.ID,
		WaliKelasID: &tutor.ID,
	}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	dob := time.Date(2017, time.March, 14, 0, 0, 0, 0, time.UTC)
	foto := "uploads/foto-export.png"
	programID := program.ID
	student := PesertaDidik{
		Nama: "Alya Export", JenisKelamin: "P", NIS: "NIS-EXPORT", NISN: "NISN-EXPORT",
		NIK: "NIK-EXPORT", TanggalLahir: &dob, KelasID: kelas.ID, PokjarID: pokjar.ID,
		OrangTuaID: "parent-export-id", ProgramID: &programID, FotoPath: &foto, Status: "aktif",
	}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/export", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		return s.exportSiswa(c)
	})

	request := httptest.NewRequest("GET", "/export?format=xlsx", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	xlsxBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("xlsx export returned %d: %s", response.StatusCode, string(xlsxBody))
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(xlsxBody))
	if err != nil {
		t.Fatalf("open exported xlsx: %v", err)
	}
	defer workbook.Close()
	rows, err := workbook.GetRows("Peserta Didik")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 4 {
		t.Fatalf("expected header and student row, got %d rows", len(rows))
	}

	headerIndex := map[string]int{}
	for i, header := range rows[2] {
		headerIndex[header] = i
	}
	for _, header := range []string{"Nama Lengkap", "NIS", "NISN", "NIK", "Program", "Tanggal Lahir", "Pokjar", "Kelas", "Tahun Ajaran", "Wali Kelas", "Status", "Foto"} {
		if _, ok := headerIndex[header]; !ok {
			t.Errorf("missing export header %q", header)
		}
	}
	for _, header := range []string{"Nama Bapak", "Nama Ibu"} {
		if _, ok := headerIndex[header]; ok {
			t.Errorf("parent header %q must not be exported", header)
		}
	}

	data := rows[3]
	cell := func(header string) string {
		index := headerIndex[header]
		if index >= len(data) {
			return ""
		}
		return data[index]
	}
	checks := map[string]string{
		"Nama Lengkap":  "Alya Export",
		"Program":       "B - Paket B",
		"Tanggal Lahir": "14-03-2017",
		"Pokjar":        "Pokjar Export",
		"Kelas":         "Kelas 3A",
		"Tahun Ajaran":  "2026/2027",
		"Wali Kelas":    "Tutor Export",
		"Foto":          "Ada",
	}
	for header, want := range checks {
		if got := cell(header); got != want {
			t.Errorf("%s: got %q, want %q", header, got, want)
		}
	}

	pdfRequest := httptest.NewRequest("GET", "/export?format=pdf", nil)
	pdfResponse, err := app.Test(pdfRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer pdfResponse.Body.Close()
	if pdfResponse.StatusCode != 200 {
		t.Fatalf("pdf export returned %d", pdfResponse.StatusCode)
	}
	pdfBody, err := io.ReadAll(pdfResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBody, []byte("%PDF-")) {
		t.Fatal("pdf export did not return a PDF document")
	}
	pdfText := string(pdfBody)
	if !strings.Contains(pdfText, "Nama Lengkap") || !strings.Contains(pdfText, "Alya Export") {
		t.Fatal("pdf export is missing student headers or data")
	}
}
