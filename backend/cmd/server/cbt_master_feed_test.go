package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestCBTMasterFeedUsesStableJSONAndIncludesCursorBoundary(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Kelas{}, &PesertaDidik{}, &Tutor{}, &MataPelajaran{}); err != nil {
		t.Fatal(err)
	}
	class := Kelas{Jenjang: 1, NamaRombel: "Paket A Kelas 1", PokjarID: "pokjar-1", TahunAjaranID: "ta-1"}
	student := PesertaDidik{Nama: "Siswa Satu", NIS: "S-001", NISN: "1234567890", KelasID: "", Status: " AKTIF "}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student.KelasID = class.ID
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	// Give both rows the same update boundary to exercise bulk import/edit
	// timestamps. The incremental feed must replay records exactly on cursor.
	boundary := time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC)
	for _, model := range []any{&class, &student} {
		if err := s.db.Model(model).UpdateColumn("updated_at", boundary).Error; err != nil {
			t.Fatal(err)
		}
	}

	app := fiber.New()
	app.Get("/master", s.cbtMasterFeed)
	request := httptest.NewRequest("GET", "/master?cursor="+url.QueryEscape(boundary.Format(time.RFC3339Nano)), nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	var payload struct {
		Kelas []struct {
			ID      string `json:"id"`
			Nama    string `json:"nama"`
			Active  bool   `json:"active"`
			Jenjang int    `json:"jenjang"`
		} `json:"kelas"`
		Students []struct {
			ID      string `json:"id"`
			Nama    string `json:"nama"`
			KelasID string `json:"kelasId"`
			Active  bool   `json:"active"`
		} `json:"pesertaDidik"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Kelas) != 1 || payload.Kelas[0].ID != class.ID || payload.Kelas[0].Nama != "Paket A Kelas 1" || !payload.Kelas[0].Active {
		t.Fatalf("class was missing or malformed at the cursor boundary: %+v", payload.Kelas)
	}
	if len(payload.Students) != 1 || payload.Students[0].ID != student.ID || payload.Students[0].KelasID != class.ID || !payload.Students[0].Active {
		t.Fatalf("student was missing or malformed at the cursor boundary: %+v", payload.Students)
	}
}
