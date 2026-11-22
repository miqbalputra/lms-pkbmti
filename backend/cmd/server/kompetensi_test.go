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
	app.Get("/kompetensi", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.listKompetensi(c)
	})
	app.Get("/rombel-kompetensi", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.listRombelKompetensi(c)
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

	var own Kompetensi
	if err = db.Where("mapel_id = ?", matematika.ID).First(&own).Error; err != nil {
		t.Fatal(err)
	}
	foreign := Kompetensi{MapelID: bahasa.ID, Nama: "Tidak boleh tampil"}
	if err = db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&RombelKompetensi{KelasID: kelas.ID, KompetensiID: own.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&RombelKompetensi{KelasID: "kelas-lain", KompetensiID: foreign.ID}).Error; err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/kompetensi", "/rombel-kompetensi"} {
		res, e := app.Test(httptest.NewRequest("GET", route, nil))
		if e != nil {
			t.Fatal(e)
		}
		var rows []map[string]interface{}
		if res.StatusCode != 200 {
			res.Body.Close()
			t.Fatalf("%s: %d", route, res.StatusCode)
		}
		e = json.NewDecoder(res.Body).Decode(&rows)
		res.Body.Close()
		if e != nil || len(rows) != 1 {
			t.Fatalf("%s did not return only assigned data: %v %v", route, rows, e)
		}
		if route == "/kompetensi" && rows[0]["id"] != own.ID {
			t.Fatal("unassigned subject exposed")
		}
		if route == "/rombel-kompetensi" && rows[0]["kelasId"] != kelas.ID {
			t.Fatal("unassigned class exposed")
		}
	}

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
