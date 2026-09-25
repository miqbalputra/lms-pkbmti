package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestReorderUjianSoalPersistsExactOrderAndRejectsForeignIDs(t *testing.T) {
	s := testServer(t)
	if err := s.db.AutoMigrate(&Tutor{}, &Kelas{}, &Ujian{}, &UjianSoal{}); err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 6, NamaRombel: "A"}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	ujian := Ujian{Judul: "Ujian Urutan", KelasID: kelas.ID}
	if err := s.db.Create(&ujian).Error; err != nil {
		t.Fatal(err)
	}
	items := []UjianSoal{{UjianID: ujian.ID, SoalID: "question-a"}, {UjianID: ujian.ID, SoalID: "question-b"}, {UjianID: ujian.ID, SoalID: "question-c"}}
	for index := range items {
		if err := s.db.Create(&items[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Put("/ujian/:id/soal/urutan", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		c.Locals("userID", c.Get("X-Test-User"))
		return s.reorderUjianSoal(c)
	})
	request := func(role string, ids []string) *http.Response {
		body, _ := json.Marshal(map[string][]string{"urutanIds": ids})
		req := httptest.NewRequest(http.MethodPut, "/ujian/"+ujian.ID+"/soal/urutan", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", role)
		req.Header.Set("X-Test-User", "test-user")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	orderedIDs := []string{items[2].ID, items[0].ID, items[1].ID}
	if got := request("admin", orderedIDs).StatusCode; got != http.StatusOK {
		t.Fatalf("admin reorder returned HTTP %d", got)
	}
	var ordered []UjianSoal
	if err := s.db.Where("ujian_id = ?", ujian.ID).Order("urutan asc").Find(&ordered).Error; err != nil {
		t.Fatal(err)
	}
	for index, item := range ordered {
		if item.ID != orderedIDs[index] || item.Urutan != index+1 {
			t.Fatalf("stored order mismatch at position %d: %+v", index, item)
		}
	}
	if got := request("admin", []string{items[0].ID, items[0].ID, items[2].ID}).StatusCode; got != http.StatusBadRequest {
		t.Fatalf("duplicate reorder ID returned HTTP %d", got)
	}
	if got := request("admin", []string{items[0].ID, items[1].ID, "foreign-link"}).StatusCode; got != http.StatusBadRequest {
		t.Fatalf("foreign reorder ID returned HTTP %d", got)
	}
	if got := request("guru", orderedIDs).StatusCode; got != http.StatusForbidden {
		t.Fatalf("out-of-scope teacher reorder returned HTTP %d", got)
	}
	var afterRejected []UjianSoal
	if err := s.db.Where("ujian_id = ?", ujian.ID).Order("urutan asc").Find(&afterRejected).Error; err != nil {
		t.Fatal(err)
	}
	for index, item := range afterRejected {
		if item.ID != orderedIDs[index] {
			t.Fatalf("invalid reorder changed persisted state: %+v", afterRejected)
		}
	}
}
