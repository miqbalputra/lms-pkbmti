package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestUpdateKelasRecordsWaliHistory(t *testing.T) {
	db := isolatedTestDB(t, "wali-history")
	if err := db.AutoMigrate(&Tutor{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &RiwayatWaliKelas{}); err != nil {
		t.Fatal(err)
	}
	first := Tutor{Nama: "Tutor Lama", JenisKelamin: "L"}
	second := Tutor{Nama: "Tutor Baru", JenisKelamin: "P"}
	pokjar := Pokjar{NamaPokjar: "Pokjar Test", Tipe: "pusat"}
	year := TahunAjaran{NamaTahunAjaran: "2099/2100", IsAktif: true}
	for _, row := range []any{&first, &second, &pokjar, &year} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &first.ID}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&RiwayatWaliKelas{KelasID: class.ID, TutorID: first.ID, TanggalMulai: time.Now().Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Put("/kelas/:id", func(c *fiber.Ctx) error { c.Locals("userID", "test-admin"); return s.updateKelas(c) })
	body := `{"jenjang":1,"namaRombel":"A","pokjarId":"` + pokjar.ID + `","tahunAjaranId":"` + year.ID + `","waliKelasId":"` + second.ID + `"}`
	request := httptest.NewRequest("PUT", "/kelas/"+class.ID, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var histories []RiwayatWaliKelas
	if err := db.Where("kelas_id = ?", class.ID).Order("tanggal_mulai desc, created_at desc").Find(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if len(histories) != 2 {
		t.Fatalf("expected two history records, got %d", len(histories))
	}
	if histories[0].TutorID != second.ID || histories[0].TanggalSelesai != nil {
		t.Fatal("new wali history is not active")
	}
	if histories[1].TutorID != first.ID || histories[1].TanggalSelesai == nil {
		t.Fatal("previous wali history was not closed")
	}
	var updated Kelas
	if err := db.First(&updated, "id = ?", class.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.WaliKelasID == nil || *updated.WaliKelasID != second.ID {
		t.Fatal("class wali kelas was not updated")
	}
}

func TestKelasCombinationMustBeUnique(t *testing.T) {
	db := isolatedTestDB(t, "kelas-unique")
	if err := db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Kelas{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Pokjar Unik", Tipe: "pusat"}
	year := TahunAjaran{NamaTahunAjaran: "2100/2101", IsAktif: true}
	if err := db.Create(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&year).Error; err != nil {
		t.Fatal(err)
	}
	first := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate class combination to be rejected")
	}
}

func TestValidSignatureAcceptsOnlyPNGBase64(t *testing.T) {
	if validSignature("") {
		t.Fatal("empty signature must be rejected")
	}
	if validSignature("data:image/jpeg;base64,/9j/") {
		t.Fatal("non-PNG signature must be rejected")
	}
	if validSignature("data:image/png;base64,aW52YWxpZA==") {
		t.Fatal("invalid PNG bytes must be rejected")
	}
	if !validSignature("data:image/png;base64,iVBORw0KGgo=") {
		t.Fatal("PNG signature header must be accepted")
	}
}

func TestWIBLocationAndPresensiDateNeverUseNilLocation(t *testing.T) {
	if wibLocation == nil {
		t.Fatal("WIB location must never be nil")
	}
	parsed, err := parsePresensiDate("2026-08-08")
	if err != nil {
		t.Fatalf("expected WIB date to parse: %v", err)
	}
	if parsed.Weekday() != time.Saturday || parsed.Location() == nil {
		t.Fatalf("unexpected parsed WIB date: %v", parsed)
	}
}

func TestNormalizeRombelName(t *testing.T) {
	tests := []struct {
		name  string
		level int
		raw   string
		want  string
	}{
		{name: "short code", level: 1, raw: "A", want: "A"},
		{name: "full code", level: 1, raw: "1A", want: "A"},
		{name: "legacy label", level: 1, raw: "KELAS 1A", want: "A"},
		{name: "legacy label with space", level: 6, raw: "Kelas 6 B", want: "B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRombelName(tt.level, tt.raw)
			if err != nil || got != tt.want {
				t.Fatalf("normalizeRombelName(%d, %q) = %q, %v; want %q", tt.level, tt.raw, got, err, tt.want)
			}
		})
	}
	if _, err := normalizeRombelName(1, "KELAS 2A"); err == nil {
		t.Fatal("expected mismatched class level to be rejected")
	}
}
