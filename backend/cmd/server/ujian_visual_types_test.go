package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVisualBankQuestionTypesValidateAndGrade(t *testing.T) {
	correctThree := 3
	tests := []struct {
		name, tipe string
		config     simulasiConfig
		answer     string
		correct    bool
		manual     bool
	}{
		{"pilihan tunggal", simulasiTipePG, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"b"}}, `"b"`, true, false},
		{"pilihan kompleks", simulasiTipePGK, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}, `["a"]`, true, false},
		{"dropdown", simulasiTipeDropdown, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectIDs: []string{"a"}}, `"a"`, true, false},
		{"tabel benar salah", simulasiTipeBenarSalah, simulasiConfig{Statements: []simulasiStatement{{ID: "s1", Text: "Pernyataan", Correct: true}}}, `{"s1":true}`, true, false},
		{"menjodohkan", simulasiTipeMenjodohkan, simulasiConfig{Left: []simulasiChoice{{ID: "l1", Text: "L1"}, {ID: "l2", Text: "L2"}}, Right: []simulasiChoice{{ID: "r1", Text: "R1"}, {ID: "r2", Text: "R2"}}, Pairs: map[string]string{"l1": "r2", "l2": "r1"}}, `{"l1":"r2","l2":"r1"}`, true, false},
		{"isian singkat", simulasiTipeIsian, simulasiConfig{AcceptedAnswers: []string{"2,5"}}, `"2.5"`, true, false},
		{"uraian", simulasiTipeUraian, simulasiConfig{Rubrik: []simulasiRubrik{{Kriteria: "Alasan", Maks: 2}}}, `"Jawaban siswa"`, false, true},
		{"skala linear", simulasiTipeSkala, simulasiConfig{ScaleMin: 1, ScaleMax: 5, CorrectNumber: &correctThree}, `3`, true, false},
		{"rating", simulasiTipeRating, simulasiConfig{RatingMax: 5, CorrectNumber: &correctThree}, `3`, true, false},
		{"kisi pilihan tunggal", simulasiTipeKisiPG, simulasiConfig{Rows: []simulasiGridRow{{ID: "row1", Text: "Baris"}}, Columns: []simulasiChoice{{ID: "c1", Text: "C1"}, {ID: "c2", Text: "C2"}}, GridCorrect: map[string]string{"row1": "c2"}}, `{"row1":"c2"}`, true, false},
		{"kisi kotak centang", simulasiTipeKisiPGK, simulasiConfig{Rows: []simulasiGridRow{{ID: "row1", Text: "Baris"}}, Columns: []simulasiChoice{{ID: "c1", Text: "C1"}, {ID: "c2", Text: "C2"}}, GridMultiCorrect: map[string][]string{"row1": {"c1"}}}, `{"row1":["c1"]}`, true, false},
		{"tanggal", simulasiTipeTanggal, simulasiConfig{AcceptedAnswers: []string{"2026-09-24"}}, `"2026-09-24"`, true, false},
		{"waktu", simulasiTipeWaktu, simulasiConfig{AcceptedAnswers: []string{"08:30"}}, `"08:30"`, true, false},
		{"susun urutan", simulasiTipeUrutan, simulasiConfig{Choices: []simulasiChoice{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, CorrectOrder: []string{"b", "a"}}, `["b","a"]`, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.config)
			if err != nil {
				t.Fatal(err)
			}
			question := BankSoal{Tipe: test.tipe, Pertanyaan: "Soal uji", Poin: 2, Konfigurasi: string(encoded)}
			if err := validateBankSoal(&question); err != nil {
				t.Fatalf("valid question rejected: %v", err)
			}
			if err := validateUjianAnswer(test.tipe, "", test.answer, question.Konfigurasi); err != nil {
				t.Fatalf("valid answer rejected: %v", err)
			}
			correct, score, manual := gradeSnapshot(simulasiSnapshot{Tipe: test.tipe, Konfigurasi: test.config}, test.answer, 2)
			if correct != test.correct || manual != test.manual || (test.correct && score != 2) {
				t.Fatalf("unexpected grade: correct=%v score=%v manual=%v", correct, score, manual)
			}
		})
	}
}

func TestPublicUjianConfigNeverSerializesAnswerKeys(t *testing.T) {
	correct := 5
	config := simulasiConfig{
		Choices: []simulasiChoice{{ID: "a", Text: "Pilihan A", ImageID: "stimulus-image-1", ImageAltText: "Diagram pecahan"}}, CorrectIDs: []string{"a"},
		Statements: []simulasiStatement{{ID: "s1", Text: "Pernyataan", Correct: true}},
		Pairs:      map[string]string{"l1": "r1"}, AcceptedAnswers: []string{"jawaban rahasia"},
		Rubrik:      []simulasiRubrik{{Kriteria: "Kunci rubrik", Maks: 2}},
		GridCorrect: map[string]string{"row1": "c1"}, GridMultiCorrect: map[string][]string{"row1": {"c1"}},
		CorrectOrder: []string{"a"}, CorrectNumber: &correct, TextMinLength: 3, TextMaxLength: 40, ValidationMessage: "Tulis lebih lengkap.",
	}
	encoded, err := json.Marshal(studentSafeUjianConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	response := string(encoded)
	for _, secret := range []string{"correctIds", "correct", "pairs", "acceptedAnswers", "rubrik", "gridCorrect", "gridMultiCorrect", "correctOrder", "correctNumber", "jawaban rahasia", "Kunci rubrik"} {
		if strings.Contains(response, secret) {
			t.Fatalf("student question config leaked %q: %s", secret, response)
		}
	}
	for _, safe := range []string{"choices", "statements", "Pernyataan", "Pilihan A", "stimulus-image-1", "Diagram pecahan"} {
		if !strings.Contains(response, safe) {
			t.Fatalf("student question config omitted safe display data %q: %s", safe, response)
		}
	}
	for _, safe := range []string{"textMinLength", "textMaxLength", "validationMessage", "Tulis lebih lengkap."} {
		if !strings.Contains(response, safe) {
			t.Fatalf("student config omitted text validation instruction %q: %s", safe, response)
		}
	}
}

func TestUjianTextResponseRulesValidateFrozenAttemptAnswers(t *testing.T) {
	server, _ := setupE2EServer(t)
	student, _ := simulasiStudent(t, server, "ujian-text-validation-student")
	config := simulasiConfig{AcceptedAnswers: []string{"jawaban"}, TextMinLength: 5, TextMaxLength: 10, ValidationMessage: "Jawaban harus lengkap."}
	encodedConfig, _ := json.Marshal(config)
	question := BankSoal{Tipe: simulasiTipeIsian, Pertanyaan: "Jawab", Konfigurasi: string(encodedConfig), Kunci: `"jawaban"`, Poin: 1}
	if err := server.db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Ujian validasi", DurasiMenit: 30, AksesKode: "TEXT-RULE"}
	if err := server.db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Status: "mulai"}
	if err := server.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	link := UjianSoal{UjianID: exam.ID, SoalID: question.ID, Urutan: 1, Bobot: 1}
	if err := server.db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	shortAnswer := UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.ID, Jawaban: `"abc"`}
	if err := server.db.Create(&shortAnswer).Error; err != nil {
		t.Fatal(err)
	}
	if err := server.validateUjianTextResponses(&attempt, &exam); err == nil || !strings.Contains(err.Error(), config.ValidationMessage) {
		t.Fatalf("short frozen answer should return teacher validation message, got %v", err)
	}
	if err := server.db.Model(&shortAnswer).Update("jawaban", `"jawaban"`).Error; err != nil {
		t.Fatal(err)
	}
	if err := server.validateUjianTextResponses(&attempt, &exam); err != nil {
		t.Fatalf("valid frozen answer rejected: %v", err)
	}
}
