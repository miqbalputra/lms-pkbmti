package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRppOptionsReturnsAssignedMapelForGuru(t *testing.T) {
	db := isolatedTestDB(t, "rpp-options")
	if err := db.AutoMigrate(&User{}, &Tutor{}, &MataPelajaran{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &KelasMapel{}, &PenugasanGuruMapel{}, &Fase{}); err != nil {
		t.Fatal(err)
	}

	tutor := Tutor{Nama: "Tutor Penyusun RPP", JenisKelamin: "P"}
	mapel := MataPelajaran{NamaMapel: "Bahasa Indonesia", KodeMapel: "BIN", IsActive: true}
	pokjar := Pokjar{NamaPokjar: "Pokjar RPP", Tipe: "pusat"}
	tahun := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	for _, row := range []any{&tutor, &mapel, &pokjar, &tahun} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	kelas := Kelas{Jenjang: 4, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: tahun.ID}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "guru-rpp", Role: "guru", TutorID: &tutor.ID, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: kelas.ID, MapelID: mapel.ID}).Error; err != nil {
		t.Fatal(err)
	}
	// Legacy databases may contain an assignment whose mapel was removed.
	// It must not make the entire RPP options endpoint fail for the valid rows.
	if err := db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: kelas.ID, MapelID: "mapel-lama-yang-sudah-dihapus"}).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/rpp/options", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.rppOptions(c)
	})
	response, err := app.Test(httptest.NewRequest("GET", "/rpp/options", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("rpp options returned %d", response.StatusCode)
	}
	var result struct {
		Mapel []struct {
			ID string `json:"id"`
		} `json:"mapel"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Mapel) != 1 || result.Mapel[0].ID != mapel.ID {
		t.Fatalf("expected assigned mapel %s in RPP options, got %+v", mapel.ID, result.Mapel)
	}
}

func TestRppOptionsResolvesReverseTutorLinkForLegacyGuruAccount(t *testing.T) {
	db := isolatedTestDB(t, "rpp-options-reverse")
	if err := db.AutoMigrate(&User{}, &Tutor{}, &MataPelajaran{}, &Pokjar{}, &TahunAjaran{}, &Kelas{}, &KelasMapel{}, &PenugasanGuruMapel{}, &Fase{}); err != nil {
		t.Fatal(err)
	}

	tutor := Tutor{Nama: "Tutor Legacy", JenisKelamin: "P"}
	mapel := MataPelajaran{NamaMapel: "Matematika", KodeMapel: "MTK", IsActive: true}
	pokjar := Pokjar{NamaPokjar: "Pokjar Legacy", Tipe: "pusat"}
	tahun := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	for _, row := range []any{&tutor, &mapel, &pokjar, &tahun} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	kelas := Kelas{Jenjang: 5, NamaRombel: "A", PokjarID: pokjar.ID, TahunAjaranID: tahun.ID}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	user := User{Username: "guru-legacy-rpp", Role: "guru", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&tutor).Update("user_id", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: kelas.ID, MapelID: mapel.ID}).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Get("/rpp/options", func(c *fiber.Ctx) error {
		c.Locals("role", "guru")
		c.Locals("userID", user.ID)
		return s.rppOptions(c)
	})
	response, err := app.Test(httptest.NewRequest("GET", "/rpp/options", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("rpp options returned %d", response.StatusCode)
	}
	var result struct {
		Mapel []struct {
			ID string `json:"id"`
		} `json:"mapel"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Mapel) != 1 || result.Mapel[0].ID != mapel.ID {
		t.Fatalf("expected reverse-linked tutor mapel %s in RPP options, got %+v", mapel.ID, result.Mapel)
	}
}
