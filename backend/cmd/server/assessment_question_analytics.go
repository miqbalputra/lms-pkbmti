package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

type assessmentQuestionStat struct {
	Module       string   `json:"modul"`
	AssessmentID string   `json:"asesmenId"`
	Assessment   string   `json:"asesmen"`
	QuestionID   string   `json:"soalId"`
	Question     string   `json:"pertanyaan"`
	Type         string   `json:"tipe"`
	Domain       string   `json:"domain,omitempty"`
	Topic        string   `json:"topik,omitempty"`
	Competency   string   `json:"kompetensi,omitempty"`
	Attempts     int      `json:"percobaan"`
	Answered     int      `json:"terjawab"`
	Blank        int      `json:"kosong"`
	Correct      int      `json:"benar"`
	Incorrect    int      `json:"salah"`
	PendingGrade int      `json:"menungguNilai"`
	SuccessRate  *float64 `json:"tingkatKeberhasilan,omitempty"`
	AverageScore *float64 `json:"rataRataSkor,omitempty"`
	EarnedPoints float64  `json:"nilaiTercapai"`
	GradedWeight float64  `json:"bobotDinilai"`
}

type assessmentQuestionStatAccumulator struct {
	assessmentQuestionStat
	totalPoints float64
	totalWeight float64
}

func hasAssessmentAnswer(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == `""` || raw == "[]" || raw == "{}" {
		return false
	}
	var value interface{}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return raw != ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	case nil:
		return false
	default:
		return true
	}
}

func (acc *assessmentQuestionStatAccumulator) record(answered, pending bool, correct *bool, score, weight float64) {
	acc.Attempts++
	if !answered {
		acc.Blank++
	} else {
		acc.Answered++
	}
	if pending {
		acc.PendingGrade++
		return
	}
	if correct != nil {
		if *correct {
			acc.Correct++
		} else if answered {
			acc.Incorrect++
		}
	}
	if weight > 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0) {
		acc.totalWeight += weight
		if score > 0 && !math.IsNaN(score) && !math.IsInf(score, 0) {
			acc.totalPoints += math.Min(score, weight)
		}
		acc.EarnedPoints = acc.totalPoints
		acc.GradedWeight = acc.totalWeight
	}
}

func (acc *assessmentQuestionStatAccumulator) result() assessmentQuestionStat {
	if graded := acc.Attempts - acc.PendingGrade; graded > 0 {
		rate := float64(acc.Correct) / float64(graded) * 100
		acc.SuccessRate = &rate
	}
	if acc.totalWeight > 0 {
		average := acc.totalPoints / acc.totalWeight * 100
		acc.AverageScore = &average
	}
	return acc.assessmentQuestionStat
}

func (s *Server) loadAssessmentQuestionStats(c *fiber.Ctx) ([]assessmentQuestionStat, bool, error) {
	attempts, truncated, err := s.loadAssessmentAnalytics(c, false)
	if err != nil {
		return nil, false, err
	}
	stats := make(map[string]*assessmentQuestionStatAccumulator)
	get := func(module, assessmentID, assessmentName, questionID, question, questionType string, metadata map[string]string) *assessmentQuestionStatAccumulator {
		key := module + "\x00" + assessmentID + "\x00" + questionID
		if stat := stats[key]; stat != nil {
			return stat
		}
		stat := &assessmentQuestionStatAccumulator{assessmentQuestionStat: assessmentQuestionStat{
			Module: module, AssessmentID: assessmentID, Assessment: assessmentName,
			QuestionID: questionID, Question: question, Type: questionType,
			Domain: metadata["domain"], Topic: metadata["topik"], Competency: metadata["kompetensi"],
		}}
		stats[key] = stat
		return stat
	}

	examAttemptIDs := make([]string, 0)
	simulationAttemptIDs := make([]string, 0)
	rowByAttempt := make(map[string]assessmentAnalyticsRow, len(attempts))
	for _, row := range attempts {
		rowByAttempt[row.ID] = row
		if row.Module == "ujian_online" {
			examAttemptIDs = append(examAttemptIDs, row.ID)
		} else if row.Module == "simulasi" {
			simulationAttemptIDs = append(simulationAttemptIDs, row.ID)
		}
	}

	if len(examAttemptIDs) > 0 {
		var frozen []UjianPesertaSoal
		if err := s.db.Where("ujian_peserta_id IN ?", examAttemptIDs).Order("urutan asc").Find(&frozen).Error; err != nil {
			return nil, false, err
		}
		var answers []UjianJawaban
		if err := s.db.Where("ujian_peserta_id IN ?", examAttemptIDs).Find(&answers).Error; err != nil {
			return nil, false, err
		}
		answerByAttemptQuestion := make(map[string]UjianJawaban, len(answers))
		answersByAttempt := make(map[string][]UjianJawaban)
		for _, answer := range answers {
			answerByAttemptQuestion[answer.UjianPesertaID+"\x00"+answer.SoalID] = answer
			answersByAttempt[answer.UjianPesertaID] = append(answersByAttempt[answer.UjianPesertaID], answer)
		}
		frozenByAttempt := make(map[string][]UjianPesertaSoal)
		for _, item := range frozen {
			frozenByAttempt[item.UjianPesertaID] = append(frozenByAttempt[item.UjianPesertaID], item)
		}
		for attemptID, row := range rowByAttempt {
			if row.Module != "ujian_online" || len(frozenByAttempt[attemptID]) > 0 {
				continue
			}
			// Older online attempts predate per-attempt question snapshots. Use the
			// currently attached source question as a best-effort report fallback.
			var source []UjianSoal
			if err := s.db.Preload("Soal").Where("ujian_id = ?", row.AssessmentID).Order("urutan asc").Find(&source).Error; err != nil {
				return nil, false, err
			}
			for _, item := range source {
				config := simulasiConfig{}
				if strings.TrimSpace(item.Soal.Konfigurasi) != "" {
					_ = json.Unmarshal([]byte(item.Soal.Konfigurasi), &config)
				}
				metadata := map[string]string{}
				for key, value := range map[string]string{"domain": item.Soal.Domain, "topik": item.Soal.Topik, "kompetensi": item.Soal.Kompetensi, "levelKognitif": item.Soal.LevelKognitif} {
					if value = strings.TrimSpace(value); value != "" {
						metadata[key] = value
					}
				}
				snapshot, _ := json.Marshal(ujianQuestionSnapshot{Tipe: item.Soal.Tipe, Pertanyaan: item.Soal.Pertanyaan, Opsi: item.Soal.Opsi, Kunci: item.Soal.Kunci, Konfigurasi: config, Metadata: metadata})
				frozenByAttempt[attemptID] = append(frozenByAttempt[attemptID], UjianPesertaSoal{UjianPesertaID: attemptID, UjianSoalID: item.ID, SoalID: item.SoalID, Bobot: item.Bobot, SnapshotJSON: string(snapshot)})
			}
		}
		for attemptID, items := range frozenByAttempt {
			row := rowByAttempt[attemptID]
			activeQuestions, activeErr := activeUjianQuestionIDs(items, answersByAttempt[attemptID])
			if activeErr != nil {
				return nil, false, fiber.NewError(500, "alur bagian Ujian Online tidak valid")
			}
			for _, item := range items {
				if !activeQuestions[item.UjianSoalID] {
					continue
				}
				snapshot, parseErr := loadUjianQuestionSnapshot(item.SnapshotJSON)
				if parseErr != nil {
					return nil, false, fiber.NewError(500, "snapshot pertanyaan Ujian Online tidak valid")
				}
				questionID := item.UjianSoalID
				if questionID == "" {
					questionID = item.SoalID
				}
				stat := get(row.Module, row.AssessmentID, row.AssessmentName, questionID, snapshot.Pertanyaan, snapshot.Tipe, snapshot.Metadata)
				answer, hasAnswer := answerByAttemptQuestion[attemptID+"\x00"+item.SoalID]
				answered := hasAnswer && hasAssessmentAnswer(answer.Jawaban)
				pending := answered && isManualUjianQuestion(snapshot.Tipe) && answer.NilaiManual == nil
				var correct *bool
				if hasAnswer {
					correct = answer.Benar
				}
				score := 0.0
				if hasAnswer {
					score = answer.Nilai
				}
				stat.record(answered, pending, correct, score, item.Bobot)
			}
		}
	}

	if len(simulationAttemptIDs) > 0 {
		var links []SimulasiUpayaSoal
		if err := s.db.Where("upaya_id IN ? AND aktif = ?", simulationAttemptIDs, true).Order("urutan_tampil asc").Find(&links).Error; err != nil {
			return nil, false, err
		}
		itemIDs := make([]string, 0, len(links))
		linkIDs := make([]string, 0, len(links))
		for _, link := range links {
			itemIDs = append(itemIDs, link.PaketSoalID)
			linkIDs = append(linkIDs, link.ID)
		}
		var items []SimulasiPaketSoal
		if len(itemIDs) > 0 {
			if err := s.db.Where("id IN ?", itemIDs).Find(&items).Error; err != nil {
				return nil, false, err
			}
		}
		itemByID := make(map[string]SimulasiPaketSoal, len(items))
		for _, item := range items {
			itemByID[item.ID] = item
		}
		var answers []SimulasiJawaban
		if len(linkIDs) > 0 {
			if err := s.db.Where("upaya_soal_id IN ?", linkIDs).Find(&answers).Error; err != nil {
				return nil, false, err
			}
		}
		answerByLink := make(map[string]SimulasiJawaban, len(answers))
		for _, answer := range answers {
			answerByLink[answer.UpayaSoalID] = answer
		}
		for _, link := range links {
			row := rowByAttempt[link.UpayaID]
			item := itemByID[link.PaketSoalID]
			if item.ID == "" || strings.TrimSpace(item.SnapshotJSON) == "" {
				continue
			}
			var snapshot simulasiSnapshot
			if err := json.Unmarshal([]byte(item.SnapshotJSON), &snapshot); err != nil {
				return nil, false, fiber.NewError(500, "snapshot pertanyaan Simulasi tidak valid")
			}
			questionID := item.ID
			if item.SoalID != nil && *item.SoalID != "" {
				questionID = *item.SoalID
			}
			stat := get(row.Module, row.AssessmentID, row.AssessmentName, questionID, snapshot.Pertanyaan, snapshot.Tipe, snapshot.Metadata)
			answer, hasAnswer := answerByLink[link.ID]
			answered := hasAnswer && hasAssessmentAnswer(answer.JawabanJSON)
			manual := snapshot.Tipe == simulasiTipeUraian || snapshot.Tipe == simulasiTipeUnggah
			pending := answered && manual && answer.SkorManual == nil
			var correct *bool
			if hasAnswer {
				correct = answer.Benar
			}
			score := 0.0
			if hasAnswer {
				score = answer.SkorAkhir
			}
			stat.record(answered, pending, correct, score, item.Bobot)
		}
	}

	results := make([]assessmentQuestionStat, 0, len(stats))
	for _, stat := range stats {
		results = append(results, stat.result())
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Assessment != results[j].Assessment {
			return strings.ToLower(results[i].Assessment) < strings.ToLower(results[j].Assessment)
		}
		return results[i].QuestionID < results[j].QuestionID
	})
	return results, truncated, nil
}

func (s *Server) assessmentQuestionAnalytics(c *fiber.Ctx) error {
	stats, truncated, err := s.loadAssessmentQuestionStats(c)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"soal": stats, "terpotong": truncated, "batasPercobaan": 2000})
}

func (s *Server) exportAssessmentQuestionAnalytics(c *fiber.Ctx) error {
	stats, truncated, err := s.loadAssessmentQuestionStats(c)
	if err != nil {
		return err
	}
	if truncated {
		return fiber.NewError(413, "analisis dibatasi pada 2.000 pengerjaan; persempit filter sebelum mengunduh")
	}
	headers := []string{"Modul", "Asesmen", "Soal", "Jenis", "Domain", "Topik", "Kompetensi", "Percobaan", "Terjawab", "Kosong", "Benar", "Salah", "Menunggu nilai", "Tingkat keberhasilan (%)", "Rata-rata skor (%)", "Nilai tercapai", "Bobot dinilai"}
	values := func(row assessmentQuestionStat) []string {
		rate, average := "", ""
		if row.SuccessRate != nil {
			rate = strconv.FormatFloat(*row.SuccessRate, 'f', 2, 64)
		}
		if row.AverageScore != nil {
			average = strconv.FormatFloat(*row.AverageScore, 'f', 2, 64)
		}
		return []string{row.Module, row.Assessment, row.Question, row.Type, row.Domain, row.Topic, row.Competency, strconv.Itoa(row.Attempts), strconv.Itoa(row.Answered), strconv.Itoa(row.Blank), strconv.Itoa(row.Correct), strconv.Itoa(row.Incorrect), strconv.Itoa(row.PendingGrade), rate, average, strconv.FormatFloat(row.EarnedPoints, 'f', 2, 64), strconv.FormatFloat(row.GradedWeight, 'f', 2, 64)}
	}
	if strings.EqualFold(c.Query("format"), "xlsx") {
		file := excelize.NewFile()
		const sheet = "Analisis per Soal"
		file.SetSheetName("Sheet1", sheet)
		for column, header := range headers {
			cell, _ := excelize.CoordinatesToCellName(column+1, 1)
			_ = file.SetCellValue(sheet, cell, header)
		}
		for rowIndex, stat := range stats {
			for column, value := range values(stat) {
				cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+2)
				_ = file.SetCellValue(sheet, cell, csvSafeField(value))
			}
		}
		_ = file.SetColWidth(sheet, "A", "Q", 20)
		_ = file.SetColWidth(sheet, "C", "C", 48)
		var output bytes.Buffer
		if err := file.Write(&output); err != nil {
			return fiber.NewError(500, "gagal membuat analisis per soal")
		}
		c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Set(fiber.HeaderContentDisposition, "attachment; filename=analisis-per-soal.xlsx")
		return c.Send(output.Bytes())
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write(headers); err != nil {
		return fiber.NewError(500, "gagal menulis laporan analisis")
	}
	for _, stat := range stats {
		row := values(stat)
		for index := range row {
			row[index] = csvSafeField(row[index])
		}
		if err := writer.Write(row); err != nil {
			return fiber.NewError(500, "gagal menulis laporan analisis")
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fiber.NewError(500, fmt.Sprintf("gagal menulis laporan: %v", err))
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, "attachment; filename=analisis-per-soal.csv")
	return c.Send(output.Bytes())
}
