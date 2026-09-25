package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUjianEssayWaitsForManualGradeAndSubmissionIsIdempotent(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)

	class := Kelas{Jenjang: 6, NamaRombel: "URAIAN"}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	student := PesertaDidik{Nama: "Peserta Uraian", NIS: "UR-1", NISN: "9900000001", KelasID: class.ID, Status: "aktif"}
	if err := s.db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	exam := Ujian{KelasID: class.ID, Judul: "Ujian Uraian", WaktuMulai: now.Add(-time.Hour), WaktuSelesai: now.Add(time.Hour), AksesKode: "URAIAN", DurasiMenit: 60}
	if err := s.db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	objective := BankSoal{Tipe: "pg", Pertanyaan: "Pilih jawaban", Opsi: `["Ya","Tidak"]`, Kunci: "1", Poin: 1}
	essay := BankSoal{Tipe: "essay", Pertanyaan: "Jelaskan alasanmu", Kunci: "banjir", Poin: 2}
	for _, question := range []*BankSoal{&objective, &essay} {
		if err := s.db.Create(question).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []UjianSoal{{UjianID: exam.ID, SoalID: objective.ID, Urutan: 1, Bobot: 1}, {UjianID: exam.ID, SoalID: essay.ID, Urutan: 2, Bobot: 2}} {
		if err := s.db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	started := now.Add(-10 * time.Minute)
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: student.ID, Mulai: &started, Status: "mulai"}
	if err := s.db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&UjianJawaban{UjianPesertaID: attempt.ID, SoalID: objective.ID, Jawaban: "1"}).Error; err != nil {
		t.Fatal(err)
	}
	essayAnswer := UjianJawaban{UjianPesertaID: attempt.ID, SoalID: essay.ID, Jawaban: "Alasan ini menyebut banjir sebagai kata kunci, tetapi perlu dinilai pemahamannya."}
	if err := s.db.Create(&essayAnswer).Error; err != nil {
		t.Fatal(err)
	}

	grade, err := s.finishUjianAttempt(&attempt, &exam, time.Now(), false, false, nil)
	if err != nil {
		t.Fatalf("submit attempt: %v", err)
	}
	if attempt.Status != "menunggu_nilai" || attempt.Skor != nil || grade.PendingManual != 1 || grade.Correct != 1 {
		t.Fatalf("essay must remain pending, not be substring-graded: attempt=%+v grade=%+v", attempt, grade)
	}
	var storedEssay UjianJawaban
	if err := s.db.First(&storedEssay, "id = ?", essayAnswer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedEssay.Benar != nil || storedEssay.Nilai != 0 {
		t.Fatalf("essay must not be auto-scored from the answer key: %+v", storedEssay)
	}

	secondGrade, err := s.finishUjianAttempt(&attempt, &exam, time.Now(), false, false, nil)
	if err != nil {
		t.Fatalf("repeat submit: %v", err)
	}
	if attempt.Status != "menunggu_nilai" || attempt.Skor != nil || secondGrade.PendingManual != 1 {
		t.Fatalf("repeat submit must return stable pending state: attempt=%+v grade=%+v", attempt, secondGrade)
	}
	var submitAudits int64
	if err := s.db.Model(&AuditLog{}).Where("resource = ? AND detail = ?", "ujian_online", attempt.ID).Count(&submitAudits).Error; err != nil {
		t.Fatal(err)
	}
	if submitAudits != 1 {
		t.Fatalf("idempotent submit should emit exactly one audit, got %d", submitAudits)
	}

	otherAttempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "another-student", Status: "menunggu_nilai"}
	if err := s.db.Create(&otherAttempt).Error; err != nil {
		t.Fatal(err)
	}
	foreignAnswer := UjianJawaban{UjianPesertaID: otherAttempt.ID, SoalID: essay.ID, Jawaban: "Jawaban siswa lain"}
	if err := s.db.Create(&foreignAnswer).Error; err != nil {
		t.Fatal(err)
	}
	foreignResponse, err := makeRequest(app, http.MethodPost, "/api/ujian-online/monitor/"+exam.ID+"/attempt/"+attempt.ID+"/answer/"+foreignAnswer.ID+"/grade", adminToken, map[string]interface{}{"nilai": 1, "komentar": ""}, "")
	if err != nil {
		t.Fatal(err)
	}
	foreignResponse.Body.Close()
	if foreignResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-attempt answer grading must be rejected, got %d", foreignResponse.StatusCode)
	}

	tooHighResponse, err := makeRequest(app, http.MethodPost, "/api/ujian-online/monitor/"+exam.ID+"/attempt/"+attempt.ID+"/answer/"+essayAnswer.ID+"/grade", adminToken, map[string]interface{}{"nilai": 2.1, "komentar": ""}, "")
	if err != nil {
		t.Fatal(err)
	}
	tooHighResponse.Body.Close()
	if tooHighResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("grade above item weight must be rejected, got %d", tooHighResponse.StatusCode)
	}

	gradeResponse, err := makeRequest(app, http.MethodPost, "/api/ujian-online/monitor/"+exam.ID+"/attempt/"+attempt.ID+"/answer/"+essayAnswer.ID+"/grade", adminToken, map[string]interface{}{"nilai": 1, "komentar": "Alasan sudah tepat, uraikan bukti lebih rinci."}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer gradeResponse.Body.Close()
	if gradeResponse.StatusCode != http.StatusOK {
		body := readAndClose(t, gradeResponse)
		t.Fatalf("manual grade failed with %d: %s", gradeResponse.StatusCode, body)
	}
	var gradePayload map[string]interface{}
	if err := json.NewDecoder(gradeResponse.Body).Decode(&gradePayload); err != nil {
		t.Fatal(err)
	}
	if gradePayload["status"] != "selesai" || gradePayload["uraianMenunggu"] != float64(0) || gradePayload["skorUjian"] == nil {
		t.Fatalf("final manual grade should release score: %+v", gradePayload)
	}
	var storedAttempt UjianPeserta
	if err := s.db.First(&storedAttempt, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedAttempt.Status != "selesai" || storedAttempt.Skor == nil || *storedAttempt.Skor < 66.6 || *storedAttempt.Skor > 66.7 {
		t.Fatalf("expected objective + manual score normalized by all weights, got %+v", storedAttempt)
	}
	if err := s.db.First(&storedEssay, "id = ?", essayAnswer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedEssay.NilaiManual == nil || *storedEssay.NilaiManual != 1 || storedEssay.DinilaiOlehUserID == nil || storedEssay.KomentarGuru == "" {
		t.Fatalf("manual grading metadata was not recorded: %+v", storedEssay)
	}

	monitorResponse, err := makeRequest(app, http.MethodGet, "/api/ujian-online/monitor/"+exam.ID, adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	monitorBody := readAndClose(t, monitorResponse)
	if strings.Contains(monitorBody, "banjir") || strings.Contains(monitorBody, "kunci") || strings.Contains(monitorBody, "KomentarGuru") {
		t.Fatalf("summary monitor must not expose answers or keys: %s", monitorBody)
	}
	reviewResponse, err := makeRequest(app, http.MethodGet, "/api/ujian-online/monitor/"+exam.ID+"/attempt/"+attempt.ID, adminToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	reviewBody := readAndClose(t, reviewResponse)
	if reviewResponse.StatusCode != http.StatusOK || !strings.Contains(reviewBody, "banjir") || !strings.Contains(reviewBody, "Alasan sudah tepat") {
		t.Fatalf("authorized staff review should include answer and scoring detail, status=%d body=%s", reviewResponse.StatusCode, reviewBody)
	}
}

func TestHistoricalCompletedEssayIsNotRetroactivelyMovedToPending(t *testing.T) {
	db := isolatedTestDB(t, "legacy-completed-essay")
	if err := db.AutoMigrate(&BankSoal{}, &Ujian{}, &UjianBagian{}, &UjianSoal{}, &UjianPeserta{}, &UjianPesertaSoal{}, &UjianJawaban{}); err != nil {
		t.Fatal(err)
	}
	question := BankSoal{Tipe: "essay", Pertanyaan: "Jelaskan", Kunci: "kunci"}
	if err := db.Create(&question).Error; err != nil {
		t.Fatal(err)
	}
	exam := Ujian{Judul: "Riwayat lama"}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UjianSoal{UjianID: exam.ID, SoalID: question.ID, Bobot: 1}).Error; err != nil {
		t.Fatal(err)
	}
	oldScore := 85.0
	attempt := UjianPeserta{UjianID: exam.ID, PesertaDidikID: "legacy-student", Status: "selesai", Skor: &oldScore}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.ID, Jawaban: "Jawaban bersejarah yang tidak dinilai dengan sistem baru"}).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db}
	grade, err := server.finishUjianAttempt(&attempt, &exam, time.Now(), false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Status != "selesai" || attempt.Skor == nil || *attempt.Skor != oldScore || grade.PendingManual != 0 {
		t.Fatalf("existing results must remain unchanged by additive grading rules: attempt=%+v grade=%+v", attempt, grade)
	}
}
