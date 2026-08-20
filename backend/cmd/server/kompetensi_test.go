package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestKompetensiTutorOptionsAndScope(t *testing.T) {
	db := isolatedTestDB(t, "kompetensi-tutor")
	if err := db.AutoMigrate(&User{}, &Tutor{}, &MataPelajaran{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &PenugasanGuruMapel{}, &Kompetensi{}, &CapaianKompetensi{}, &RombelKompetensi{}, &NilaiKompetensi{}); err != nil {
		t.Fatal(err)
	}

	tutor := Tutor{Nama: "Tutor Kompetensi", JenisKelamin: "P"}
	matematika := MataPelajaran{NamaMapel: "Matematika", KodeMapel: "MTK", IsActive: true}
	bahasa := MataPelajaran{NamaMapel: "Bahasa Indonesia", KodeMapel: "BIN", IsActive: true}
	pokjar := Pokjar{NamaPokjar: "Pokjar Kompetensi", Tipe: "pusat"}
	tahun := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	for _, row := range []any{&tutor, &matematika, &bahasa, &pokjar, &tahun} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	kelas := Kelas{Jenjang: 2, NamaRombel: "B", PokjarID: pokjar.ID, TahunAjaranID: tahun.ID}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: kelas.ID, MapelID: matematika.ID}).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "guru-kompetensi", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/kompetensi/options", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.kompetensiOptions(c)
	})
	app.Post("/kompetensi", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.createKompetensi(c)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/kompetensi/options", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("kompetensi options returned %d", response.StatusCode)
	}
	var options struct {
		Mapel []struct {
			ID string `json:"id"`
		} `json:"mapel"`
		Kelas []struct {
			ID string `json:"id"`
		} `json:"kelas"`
	}
	if err := json.NewDecoder(response.Body).Decode(&options); err != nil {
		t.Fatal(err)
	}
	if len(options.Mapel) != 1 || options.Mapel[0].ID != matematika.ID {
		t.Fatalf("expected only assigned mapel in options, got %+v", options.Mapel)
	}
	if len(options.Kelas) != 1 || options.Kelas[0].ID != kelas.ID {
		t.Fatalf("expected only assigned class in options, got %+v", options.Kelas)
	}

	createRequest := func(mapelID string) *http.Request {
		req := httptest.NewRequest("POST", "/kompetensi", strings.NewReader(`{"mapelId":"`+mapelID+`","nama":"Memahami bilangan"}`))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	response, err = app.Test(createRequest(matematika.ID))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 {
		response.Body.Close()
		t.Fatalf("assigned tutor mapel create returned %d", response.StatusCode)
	}
	response.Body.Close()

	response, err = app.Test(createRequest(bahasa.ID))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 403 {
		response.Body.Close()
		t.Fatalf("unassigned tutor mapel create returned %d, want 403", response.StatusCode)
	}
	response.Body.Close()
}
