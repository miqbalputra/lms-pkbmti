package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TestUjianSectionsAreScopedSavedAndFrozenInRandomizedAttempts(t *testing.T) {
	db := isolatedTestDB(t, "ujian-section-snapshots")
	if err := db.AutoMigrate(&AuditLog{}, &BankSoal{}, &Kelas{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}); err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 6, NamaRombel: "Bagian"}
	if err := db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Ujian bertahap", KelasID: kelas.ID, AcakSoal: true}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Put("/ujian/:id/bagian", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		c.Locals("userID", "section-admin")
		return s.saveUjianBagian(c)
	})
	app.Post("/ujian/:id/soal", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		c.Locals("userID", "section-admin")
		return s.addUjianSoal(c)
	})
	request := func(method, path, role, body string) *http.Response {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", role)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	sectionsJSON := `{"bagian":[{"id":"read","nama":"Literasi","deskripsi":"Baca teks terlebih dahulu."},{"id":"math","nama":"Numerasi","deskripsi":"Gunakan informasi pada soal."}]}`
	if response := request(http.MethodPut, "/ujian/"+exam.ID+"/bagian", "admin", sectionsJSON); response.StatusCode != http.StatusOK {
		t.Fatalf("admin save sections returned HTTP %d", response.StatusCode)
	}
	if response := request(http.MethodPut, "/ujian/"+exam.ID+"/bagian", "guru", `{"bagian":[]}`); response.StatusCode != http.StatusForbidden {
		t.Fatalf("out-of-scope teacher changed sections: HTTP %d", response.StatusCode)
	}
	if response := request(http.MethodPut, "/ujian/"+exam.ID+"/bagian", "kepala_sekolah", `{"bagian":[]}`); response.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only headmaster changed sections: HTTP %d", response.StatusCode)
	}
	questionIDs := make([]string, 0, 4)
	for _, item := range []struct{ text, section string }{
		{"Literasi satu", "read"}, {"Numerasi satu", "math"}, {"Literasi dua", "read"}, {"Numerasi dua", "math"},
	} {
		question := BankSoal{Tipe: "pg", Pertanyaan: item.text, Opsi: `["A","B"]`, Kunci: "0", Poin: 1}
		if err := db.Create(&question).Error; err != nil {
			t.Fatal(err)
		}
		questionIDs = append(questionIDs, question.ID)
		if len(questionIDs) == 1 {
			body, _ := json.Marshal(map[string]any{"soalId": question.ID, "bobot": 1, "bagianId": "foreign-section"})
			if response := request(http.MethodPost, "/ujian/"+exam.ID+"/soal", "admin", string(body)); response.StatusCode != http.StatusBadRequest {
				t.Fatalf("foreign section assignment returned HTTP %d", response.StatusCode)
			}
		}
		body, _ := json.Marshal(map[string]any{"soalId": question.ID, "bobot": 1, "bagianId": item.section})
		if response := request(http.MethodPost, "/ujian/"+exam.ID+"/soal", "admin", string(body)); response.StatusCode != http.StatusCreated {
			responseText, _ := io.ReadAll(response.Body)
			t.Fatalf("assigning question to section returned HTTP %d: %s", response.StatusCode, responseText)
		}
	}

	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "section-student", Status: "mulai"}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	var first, resumed []UjianPesertaSoal
	for _, output := range []*[]UjianPesertaSoal{&first, &resumed} {
		if err := db.Transaction(func(tx *gorm.DB) error {
			var loadErr error
			*output, loadErr = ensureUjianAttemptQuestionsTx(tx, &attempt, &exam)
			return loadErr
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(first) != 4 || len(resumed) != 4 {
		t.Fatalf("expected four frozen questions, got first=%d resumed=%d", len(first), len(resumed))
	}
	for index := range first {
		if first[index].UjianSoalID != resumed[index].UjianSoalID || first[index].Urutan != resumed[index].Urutan {
			t.Fatal("randomized section order changed after attempt resume")
		}
		if index < 2 && (first[index].BagianID != "read" || first[index].NamaBagian != "Literasi" || first[index].DeskripsiBagian == "") {
			t.Fatalf("first section snapshot metadata missing at question %d: %+v", index, first[index])
		}
		if index >= 2 && (first[index].BagianID != "math" || first[index].NamaBagian != "Numerasi") {
			t.Fatalf("second section was not kept after first section: %+v", first[index])
		}
	}

	// Renaming or deleting a source section after a learner starts cannot rewrite
	// the frozen attempt context used for history or review.
	if err := db.Model(&UjianBagian{}).Where("ujian_id = ? AND client_id = ?", exam.ID, "read").Update("nama", "Renamed later").Error; err != nil {
		t.Fatal(err)
	}
	var frozen []UjianPesertaSoal
	if err := db.Where("ujian_peserta_id = ?", attempt.ID).Order("urutan asc").Find(&frozen).Error; err != nil {
		t.Fatal(err)
	}
	if len(frozen) == 0 || frozen[0].NamaBagian != "Literasi" {
		t.Fatal("section title changed in an existing student attempt")
	}
}
