package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TestLegacyBankQuestionTypesValidateAndNormalize(t *testing.T) {
	tests := []struct {
		name     string
		question BankSoal
		valid    bool
	}{
		{"pilihan tunggal", BankSoal{Tipe: "pg", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: "1", Poin: 1}, true},
		{"kotak centang", BankSoal{Tipe: "checkbox", Pertanyaan: "Pilih semua", Opsi: `["A","B","C"]`, Kunci: `[0,2]`, Poin: 1}, true},
		{"dropdown", BankSoal{Tipe: "dropdown", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: "0", Poin: 1}, true},
		{"benar salah default", BankSoal{Tipe: "true_false", Pertanyaan: "Pernyataan", Kunci: "1", Poin: 1}, true},
		{"benar salah pilihan harus konsisten", BankSoal{Tipe: "true_false", Pertanyaan: "Pernyataan", Opsi: `["Betul","Keliru"]`, Kunci: "1", Poin: 1}, false},
		{"jawaban singkat", BankSoal{Tipe: "short_answer", Pertanyaan: "Hitung", Kunci: `["2,5","dua koma lima"]`, Poin: 1}, true},
		{"uraian", BankSoal{Tipe: "essay", Pertanyaan: "Jelaskan", Kunci: "Rubrik guru", Poin: 2}, true},
		{"pilihan tanpa kunci", BankSoal{Tipe: "pg", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: "", Poin: 1}, false},
		{"duplikat kunci checkbox", BankSoal{Tipe: "checkbox", Pertanyaan: "Pilih", Opsi: `["A","B"]`, Kunci: `[1,1]`, Poin: 1}, false},
		{"tipe tidak dikenal", BankSoal{Tipe: "magic", Pertanyaan: "Pilih", Poin: 1}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateBankSoal(&test.question)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, got error %v", test.valid, err)
			}
			if test.name == "benar salah default" && test.question.Opsi != `["Benar","Salah"]` {
				t.Fatalf("true/false defaults were not applied: %q", test.question.Opsi)
			}
		})
	}

	for _, test := range []struct {
		name, tipe, options, key, answer string
		correct                          bool
	}{
		{"single choice", "pg", `["A","B"]`, "1", "1", true},
		{"dropdown", "dropdown", `["A","B"]`, "0", "0", true},
		{"true false", "true_false", `["Benar","Salah"]`, "1", "1", true},
		{"checkbox exact set", "checkbox", `["A","B","C"]`, `[0,2]`, `[2,0]`, true},
		{"checkbox partial set is wrong", "checkbox", `["A","B","C"]`, `[0,2]`, `[0]`, false},
		{"short answer punctuation and spaces", "short_answer", "", `["jawaban benar"]`, "  JAWABAN,   benar! ", true},
		{"decimal comma", "short_answer", "", `["2.5"]`, "2,5", true},
		{"blank is never correct", "pg", `["A","B"]`, "0", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := bankSoalAnswerCorrect(test.tipe, test.options, test.key, test.answer); got != test.correct {
				t.Fatalf("expected correct=%v, got %v", test.correct, got)
			}
		})
	}
}

func TestUjianOnlineAttemptSnapshotsFreezeOrderContentAndWeight(t *testing.T) {
	db := isolatedTestDB(t, "ujian-online-question-snapshot")
	if err := db.AutoMigrate(&BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{Judul: "Snapshot", AcakSoal: true, WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), DurasiMenit: 60}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	var sourceIDs []string
	for i := 0; i < 20; i++ {
		question := BankSoal{Tipe: "pg", Pertanyaan: "Versi asli " + string(rune('A'+i)), Opsi: `["Salah","Benar"]`, Kunci: "1", Poin: 2, Domain: "Numerasi", Topik: "Bilangan", Kompetensi: "Memecahkan masalah kontekstual", LevelKognitif: "Menalar"}
		if err := db.Create(&question).Error; err != nil {
			t.Fatal(err)
		}
		sourceIDs = append(sourceIDs, question.ID)
		if err := db.Create(&UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: i + 1, Bobot: 2}).Error; err != nil {
			t.Fatal(err)
		}
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "snapshot-student", Mulai: &now, Status: "mulai"}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	var firstOrder, resumedOrder []UjianPesertaSoal
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		firstOrder, err = ensureUjianAttemptQuestionsTx(tx, &attempt, &exam)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		resumedOrder, err = ensureUjianAttemptQuestionsTx(tx, &attempt, &exam)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(firstOrder) != 20 || len(resumedOrder) != len(firstOrder) {
		t.Fatalf("attempt should snapshot all questions once: first=%d resumed=%d", len(firstOrder), len(resumedOrder))
	}
	for i := range firstOrder {
		if firstOrder[i].UjianSoalID != resumedOrder[i].UjianSoalID || firstOrder[i].Urutan != resumedOrder[i].Urutan {
			t.Fatal("question order changed after resuming the same attempt")
		}
	}
	var otherAttemptOrder []UjianPesertaSoal
	secondAttempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "snapshot-student-2", Status: "mulai"}
	if err := db.Create(&secondAttempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		otherAttemptOrder, err = ensureUjianAttemptQuestionsTx(tx, &secondAttempt, &exam)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	identical := len(firstOrder) == len(otherAttemptOrder)
	for i := range firstOrder {
		if firstOrder[i].UjianSoalID != otherAttemptOrder[i].UjianSoalID {
			identical = false
			break
		}
	}
	if identical {
		t.Fatal("different student attempts should receive independent shuffled question order")
	}

	gradedQuestion := firstOrder[0]
	var oldSnapshot ujianQuestionSnapshot
	if err := json.Unmarshal([]byte(gradedQuestion.SnapshotJSON), &oldSnapshot); err != nil {
		t.Fatal(err)
	}
	if oldSnapshot.Metadata["domain"] != "Numerasi" || oldSnapshot.Metadata["kompetensi"] != "Memecahkan masalah kontekstual" {
		t.Fatalf("attempt snapshot should include learning metadata: %+v", oldSnapshot.Metadata)
	}
	if err := db.Model(&BankSoal{}).Where("id = ?", gradedQuestion.SoalID).Updates(map[string]interface{}{"pertanyaan": "Konten diedit setelah mulai", "kunci": "0", "opsi": `["Baru 1","Baru 2"]`, "domain": "Literasi", "kompetensi": "Kompetensi baru"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UjianSoal{}).Where("id = ?", gradedQuestion.UjianSoalID).Update("bobot", 99).Error; err != nil {
		t.Fatal(err)
	}
	for _, frozen := range firstOrder {
		if err := db.Create(&UjianJawaban{UjianPesertaID: attempt.ID, SoalID: frozen.SoalID, Jawaban: "1"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var grade ujianGradeResult
	if err := db.Transaction(func(tx *gorm.DB) error { return gradeUjianPesertaTx(tx, &attempt, &exam, &grade) }); err != nil {
		t.Fatal(err)
	}
	if grade.Correct != len(firstOrder) || grade.Score != 100 {
		t.Fatalf("grading should use the original key and frozen weight: %+v", grade)
	}
	var stored UjianPesertaSoal
	if err := db.First(&stored, "id = ?", gradedQuestion.ID).Error; err != nil {
		t.Fatal(err)
	}
	var preserved ujianQuestionSnapshot
	if err := json.Unmarshal([]byte(stored.SnapshotJSON), &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved.Pertanyaan != oldSnapshot.Pertanyaan || preserved.Kunci != oldSnapshot.Kunci || preserved.Metadata["domain"] != "Numerasi" || preserved.Metadata["kompetensi"] != "Memecahkan masalah kontekstual" || stored.Bobot != 2 {
		t.Fatalf("published content/answer key/weight were not preserved: %+v %+v", preserved, stored)
	}
}

func TestBankSoalStimulusValidationAndUjianSnapshot(t *testing.T) {
	items, err := normalizeBankSoalStimulus([]simulasiStimulusIn{
		{Jenis: "text", Konten: "  Bacaan pengantar  ", Urutan: 99},
		{Jenis: "table", Konten: "Nama\tJumlah\nJeruk\t3"},
		{Jenis: "media_link", Konten: "https://media.example.org/video"},
	})
	if err != nil {
		t.Fatalf("valid stimulus rejected: %v", err)
	}
	if len(items) != 3 || items[0].Konten != "Bacaan pengantar" || items[0].Urutan != 1 || items[2].Urutan != 3 {
		t.Fatalf("stimulus should be normalized and ordered: %+v", items)
	}
	if _, err := normalizeBankSoalStimulus([]simulasiStimulusIn{{Jenis: "media_link", Konten: "http://media.example.org/video"}}); err == nil {
		t.Fatal("non-HTTPS media links must be rejected")
	}
	if _, err := normalizeBankSoalStimulus([]simulasiStimulusIn{{Jenis: "image", Konten: "uploads/private.png", AltText: "Gambar"}}); err == nil {
		t.Fatal("unverified image paths must never be accepted as stimulus")
	}

	db := isolatedTestDB(t, "ujian-stimulus-snapshot")
	if err := db.AutoMigrate(&BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianBagian{}, &UjianPeserta{}, &UjianPesertaSoal{}); err != nil {
		t.Fatal(err)
	}
	stimulusJSON, _ := json.Marshal(items)
	exam := Ujian{Judul: "Stimulus Snapshot", WaktuMulai: time.Now().Add(-time.Hour), WaktuSelesai: time.Now().Add(time.Hour), DurasiMenit: 60}
	question := BankSoal{Tipe: "pg", Pertanyaan: "Apa informasi utama?", Opsi: `["A","B"]`, Kunci: "0", Poin: 1, StimulusJSON: string(stimulusJSON)}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "stimulus-student", Status: "mulai"}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	var snapshotRows []UjianPesertaSoal
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		snapshotRows, err = ensureUjianAttemptQuestionsTx(tx, &attempt, &exam)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(snapshotRows) != 1 {
		t.Fatalf("expected one frozen question, got %d", len(snapshotRows))
	}
	var snapshot ujianQuestionSnapshot
	if err := json.Unmarshal([]byte(snapshotRows[0].SnapshotJSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Stimulus) != 3 || snapshot.Stimulus[0].Konten != "Bacaan pengantar" || snapshot.Stimulus[1].Jenis != "table" {
		t.Fatalf("stimulus was not included in the attempt snapshot: %+v", snapshot.Stimulus)
	}
	if err := db.Model(&BankSoal{}).Where("id = ?", question.ID).Update("stimulus_json", `[{"jenis":"text","konten":"Sumber telah diubah","urutan":1}]`).Error; err != nil {
		t.Fatal(err)
	}
	var resumed []UjianPesertaSoal
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		resumed, err = ensureUjianAttemptQuestionsTx(tx, &attempt, &exam)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var preserved ujianQuestionSnapshot
	if err := json.Unmarshal([]byte(resumed[0].SnapshotJSON), &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved.Stimulus[0].Konten != "Bacaan pengantar" {
		t.Fatalf("changing a source stimulus must not rewrite an in-progress attempt: %+v", preserved.Stimulus)
	}
}

func TestBankSoalAPIStoresAndListsStructuredStimulus(t *testing.T) {
	db := isolatedTestDB(t, "bank-soal-stimulus-api")
	if err := db.AutoMigrate(&BankSoal{}, &MataPelajaran{}, &AuditLog{}); err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db}
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Post("/bank-soal", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		c.Locals("userID", "stimulus-admin")
		return server.createBankSoal(c)
	})
	app.Get("/bank-soal", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		return server.listBankSoal(c)
	})
	app.Put("/bank-soal/:id", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		c.Locals("userID", "stimulus-admin")
		return server.updateBankSoal(c)
	})
	body := `{"tipe":"pg_tunggal","pertanyaan":"Apa informasi pada tabel?","domain":"Numerasi","topik":"Membaca tabel","kompetensi":"Menafsirkan data sederhana","levelKognitif":"Memahami","konfigurasi":{"choices":[{"id":"a","text":"Pilihan A"},{"id":"b","text":"Pilihan B"}],"correctIds":["a"]},"stimulus":[{"jenis":"text","konten":"Bacaan awal"},{"jenis":"table","konten":"Hari\tJumlah\nSenin\t12"}],"poin":1}`
	createRequest := httptest.NewRequest(http.MethodPost, "/bank-soal", strings.NewReader(body))
	createRequest.Header.Set("Content-Type", "application/json")
	created, err := app.Test(createRequest, -1)
	if err != nil {
		t.Fatal(err)
	}
	createdBody := readAndClose(t, created)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create stimulus question failed: %d %s", created.StatusCode, createdBody)
	}
	var createdQuestion BankSoal
	if err := json.Unmarshal([]byte(createdBody), &createdQuestion); err != nil {
		t.Fatal(err)
	}
	if len(createdQuestion.Stimulus) != 2 || createdQuestion.Stimulus[1].Jenis != "table" {
		t.Fatalf("create response should return structured stimulus items: %s", createdBody)
	}
	if createdQuestion.Domain != "Numerasi" || createdQuestion.Topik != "Membaca tabel" || createdQuestion.Kompetensi != "Menafsirkan data sederhana" || createdQuestion.LevelKognitif != "Memahami" {
		t.Fatalf("create response should retain learning metadata: %+v", createdQuestion)
	}
	var stored BankSoal
	if err := db.First(&stored, "id = ?", createdQuestion.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored.StimulusJSON, "Bacaan awal") || strings.Contains(stored.StimulusJSON, `"kunci"`) {
		t.Fatalf("stimulus should be stored separately from the answer key: %q", stored.StimulusJSON)
	}
	updateBody := `{"tipe":"pg_tunggal","pertanyaan":"Apa informasi pada tabel?","konfigurasi":{"choices":[{"id":"a","text":"Pilihan A"},{"id":"b","text":"Pilihan B"}],"correctIds":["a"]},"poin":1}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/bank-soal/"+createdQuestion.ID, strings.NewReader(updateBody))
	updateRequest.Header.Set("Content-Type", "application/json")
	updated, err := app.Test(updateRequest, -1)
	if err != nil {
		t.Fatal(err)
	}
	updatedBody := readAndClose(t, updated)
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("legacy-shaped update failed: %d %s", updated.StatusCode, updatedBody)
	}
	var updatedQuestion BankSoal
	if err := json.Unmarshal([]byte(updatedBody), &updatedQuestion); err != nil {
		t.Fatal(err)
	}
	if updatedQuestion.Domain != "Numerasi" || updatedQuestion.Kompetensi != "Menafsirkan data sederhana" {
		t.Fatalf("omitted optional metadata must not be cleared by older clients: %+v", updatedQuestion)
	}

	listed, err := app.Test(httptest.NewRequest(http.MethodGet, "/bank-soal", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	listedBody := readAndClose(t, listed)
	if listed.StatusCode != http.StatusOK || !strings.Contains(listedBody, `"jenis":"table"`) || !strings.Contains(listedBody, `"konten":"Bacaan awal"`) {
		t.Fatalf("list response should restore structured stimulus items: %d %s", listed.StatusCode, listedBody)
	}
}

func TestShuffleBankOptionsReturnsStableOriginalIndexes(t *testing.T) {
	options := []string{"A", "B", "C", "D"}
	first, indexes := shuffleBankOptionList(options, 42)
	second, indexesAgain := shuffleBankOptionList(options, 42)
	if jsonValue(first) != jsonValue(second) || jsonValue(indexes) != jsonValue(indexesAgain) {
		t.Fatal("same seed should produce a stable option order")
	}
	seen := map[int]bool{}
	for i, index := range indexes {
		if index < 0 || index >= len(options) || seen[index] || first[i] != options[index] {
			t.Fatalf("shuffled options must retain their original indexes: options=%v indexes=%v", first, indexes)
		}
		seen[index] = true
	}
}

func TestUjianQuestionOrderKeepsSectionsAndShufflesOnlyWithinEachSection(t *testing.T) {
	sections := []UjianBagian{
		{ClientID: "literasi", Nama: "Literasi", Urutan: 1},
		{ClientID: "numerasi", Nama: "Numerasi", Urutan: 2},
	}
	source := []UjianSoal{
		{Base: Base{ID: "n-1"}, BagianID: "numerasi", Urutan: 1},
		{Base: Base{ID: "l-1"}, BagianID: "literasi", Urutan: 1},
		{Base: Base{ID: "n-2"}, BagianID: "numerasi", Urutan: 2},
		{Base: Base{ID: "l-2"}, BagianID: "literasi", Urutan: 2},
	}
	first := orderUjianQuestions(append([]UjianSoal(nil), source...), sections, "attempt-seed", true)
	second := orderUjianQuestions(append([]UjianSoal(nil), source...), sections, "attempt-seed", true)
	if len(first) != 4 || len(second) != len(first) {
		t.Fatalf("unexpected result length: %d, %d", len(first), len(second))
	}
	for index := range first {
		if first[index].ID != second[index].ID {
			t.Fatal("same attempt seed must reproduce section question order")
		}
		if index < 2 && first[index].BagianID != "literasi" {
			t.Fatalf("first authored section changed position: %+v", first)
		}
		if index >= 2 && first[index].BagianID != "numerasi" {
			t.Fatalf("second authored section changed position: %+v", first)
		}
	}

	legacy := orderUjianQuestions(append([]UjianSoal(nil), source...), nil, "attempt-seed", true)
	legacyExpected := append([]UjianSoal(nil), source...)
	rand.New(rand.NewSource(seedFromID("attempt-seed"))).Shuffle(len(legacyExpected), func(i, j int) {
		legacyExpected[i], legacyExpected[j] = legacyExpected[j], legacyExpected[i]
	})
	if jsonValue(legacy) != jsonValue(legacyExpected) {
		t.Fatal("legacy exam shuffle changed after adding optional sections")
	}
}

func jsonValue(value interface{}) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
