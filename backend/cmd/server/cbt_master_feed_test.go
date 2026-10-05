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
	if err := s.db.AutoMigrate(&Pokjar{}, &TahunAjaran{}, &Program{}, &Fase{}, &Kelas{}, &PesertaDidik{}, &Tutor{}, &MataPelajaran{}); err != nil {
		t.Fatal(err)
	}
	pokjar := Pokjar{NamaPokjar: "Kelompok Belajar Utama", Tipe: "Dalam Kota"}
	year := TahunAjaran{NamaTahunAjaran: "2026/2027", IsAktif: true}
	program := Program{Kode: "A", Nama: "Paket A", JenjangSetara: "SD"}
	phase := Fase{Kode: "A", Nama: "Fase A", JenjangSetara: "SD"}
	for _, row := range []any{&pokjar, &year, &program, &phase} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := Kelas{Jenjang: 1, NamaRombel: "Kelas 1", PokjarID: pokjar.ID, TahunAjaranID: year.ID, ProgramID: &program.ID, FaseID: &phase.ID}
	student := PesertaDidik{Nama: "Siswa Satu", NIS: "S-001", NISN: "1234567890", JenisKelamin: "P", KelasID: "", PokjarID: pokjar.ID, ProgramID: &program.ID, Status: " AKTIF "}
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
	for _, model := range []any{&class, &student, &pokjar, &year, &program, &phase} {
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
		SchemaVersion int `json:"schemaVersion"`
		Kelas         []struct {
			ID            string `json:"id"`
			Nama          string `json:"nama"`
			Active        bool   `json:"active"`
			Jenjang       int    `json:"jenjang"`
			PokjarID      string `json:"pokjarId"`
			TahunAjaranID string `json:"tahunAjaranId"`
			ProgramID     string `json:"programId"`
			FaseID        string `json:"faseId"`
		} `json:"kelas"`
		Students []struct {
			ID           string `json:"id"`
			Nama         string `json:"nama"`
			NIS          string `json:"nis"`
			NISN         string `json:"nisn"`
			JenisKelamin string `json:"jenisKelamin"`
			KelasID      string `json:"kelasId"`
			PokjarID     string `json:"pokjarId"`
			Active       bool   `json:"active"`
		} `json:"pesertaDidik"`
		Pokjars []struct {
			ID         string `json:"id"`
			NamaPokjar string `json:"namaPokjar"`
		} `json:"kelompokBelajar"`
		Years []struct {
			ID   string `json:"id"`
			Nama string `json:"namaTahunAjaran"`
		} `json:"tahunAjaran"`
		Programs []struct {
			ID   string `json:"id"`
			Nama string `json:"nama"`
		} `json:"program"`
		Phases []struct {
			ID   string `json:"id"`
			Nama string `json:"nama"`
		} `json:"fase"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != 2 {
		t.Fatalf("expected full-roster feed schema version 2, got %d", payload.SchemaVersion)
	}
	if len(payload.Kelas) != 1 || payload.Kelas[0].ID != class.ID || payload.Kelas[0].Nama != "Kelas 1" || !payload.Kelas[0].Active || payload.Kelas[0].PokjarID != pokjar.ID || payload.Kelas[0].TahunAjaranID != year.ID || payload.Kelas[0].ProgramID != program.ID || payload.Kelas[0].FaseID != phase.ID {
		t.Fatalf("class was missing or malformed at the cursor boundary: %+v", payload.Kelas)
	}
	if len(payload.Students) != 1 || payload.Students[0].ID != student.ID || payload.Students[0].KelasID != class.ID || payload.Students[0].NIS != student.NIS || payload.Students[0].NISN != student.NISN || payload.Students[0].JenisKelamin != student.JenisKelamin || payload.Students[0].PokjarID != pokjar.ID || !payload.Students[0].Active {
		t.Fatalf("student was missing or malformed at the cursor boundary: %+v", payload.Students)
	}
	if len(payload.Pokjars) != 1 || payload.Pokjars[0].ID != pokjar.ID || payload.Pokjars[0].NamaPokjar != pokjar.NamaPokjar {
		t.Fatalf("learning group was missing from feed: %+v", payload.Pokjars)
	}
	if len(payload.Years) != 1 || payload.Years[0].ID != year.ID || payload.Years[0].Nama != year.NamaTahunAjaran || len(payload.Programs) != 1 || payload.Programs[0].ID != program.ID || len(payload.Phases) != 1 || payload.Phases[0].ID != phase.ID {
		t.Fatalf("class roster metadata was missing from feed: years=%+v programs=%+v phases=%+v", payload.Years, payload.Programs, payload.Phases)
	}
}
