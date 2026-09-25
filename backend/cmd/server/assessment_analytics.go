package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const assessmentAnalyticsLimit = 10000

// assessmentAnalyticsRow deliberately contains only submitted-result metadata,
// never question keys or answer content. ClassID is the class captured when the
// attempt started; older rows fall back to the exam/assignment class, then the
// student's current class only when no historical context exists.
type assessmentAnalyticsRow struct {
	ID             string     `json:"id" gorm:"column:id"`
	Module         string     `json:"modul" gorm:"column:module"`
	AssessmentID   string     `json:"asesmenId" gorm:"column:assessment_id"`
	AssessmentName string     `json:"asesmen" gorm:"column:assessment_name"`
	Mode           string     `json:"mode" gorm:"column:mode"`
	Subject        string     `json:"mapel" gorm:"column:subject"`
	StudentID      string     `json:"pesertaDidikId" gorm:"column:student_id"`
	StudentName    string     `json:"namaSiswa" gorm:"column:student_name"`
	NISN           string     `json:"nisn" gorm:"column:nisn"`
	ClassID        string     `json:"kelasId" gorm:"column:class_id"`
	ClassGrade     int        `json:"jenjang" gorm:"column:class_grade"`
	ClassName      string     `json:"kelas" gorm:"column:class_name"`
	Status         string     `json:"status" gorm:"column:status"`
	StartedAt      *time.Time `json:"mulai,omitempty" gorm:"column:started_at"`
	SubmittedAt    *time.Time `json:"selesai,omitempty" gorm:"column:submitted_at"`
	Score          *float64   `json:"nilai,omitempty" gorm:"column:score"`
	Attempt        int        `json:"percobaan" gorm:"column:attempt_no"`
}

type assessmentAnalyticsSummary struct {
	Total         int      `json:"total"`
	Selesai       int      `json:"selesai"`
	Berlangsung   int      `json:"berlangsung"`
	MenungguNilai int      `json:"menungguNilai"`
	RataRata      *float64 `json:"rataRataNilai,omitempty"`
}

type assessmentStudentProgress struct {
	StudentID   string     `json:"pesertaDidikId"`
	StudentName string     `json:"namaSiswa"`
	NISN        string     `json:"nisn"`
	ClassID     string     `json:"kelasId"`
	ClassName   string     `json:"kelas"`
	Total       int        `json:"jumlahPengerjaan"`
	Completed   int        `json:"selesai"`
	Average     *float64   `json:"rataRataNilai,omitempty"`
	LastAttempt *time.Time `json:"pengerjaanTerakhir,omitempty"`
}

type assessmentClassProgress struct {
	ClassID   string   `json:"kelasId"`
	ClassName string   `json:"kelas"`
	Students  int      `json:"jumlahSiswa"`
	Total     int      `json:"jumlahPengerjaan"`
	Completed int      `json:"selesai"`
	Average   *float64 `json:"rataRataNilai,omitempty"`
}

func assessmentTimeFilter(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		parsed, err = time.ParseInLocation("2006-01-02", raw, wibLocation)
		if err != nil {
			return nil, fiber.NewError(400, "filter waktu harus ISO-8601 atau YYYY-MM-DD")
		}
		if endOfDay {
			parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), parsed.Location())
		}
	}
	return &parsed, nil
}

func (s *Server) assessmentAnalyticsFilters(c *fiber.Ctx) ([]string, error) {
	role, _ := c.Locals("role").(string)
	if role != "admin" && role != "kepala_sekolah" && role != "guru" {
		return nil, fiber.NewError(403, "laporan asesmen hanya tersedia untuk staf")
	}
	ids, ok := s.waliKelasIDs(c)
	if role == "guru" && (!ok || ids == nil) {
		return nil, fiber.NewError(403, "akun guru belum memiliki kewenangan kelas")
	}
	if classID := strings.TrimSpace(c.Query("kelasId")); classID != "" && role == "guru" && !containsString(ids, classID) {
		return nil, fiber.NewError(403, "kelas berada di luar kewenangan Anda")
	}
	return ids, nil
}

func applyAssessmentAnalyticsFilters(q *gorm.DB, c *fiber.Ctx, classExpr, studentExpr, statusExpr, startedExpr, assessmentExpr, subjectExpr string) (*gorm.DB, error) {
	if v := strings.TrimSpace(c.Query("asesmenId")); v != "" {
		q = q.Where(assessmentExpr+" = ?", v)
	}
	if v := strings.TrimSpace(c.Query("mapelId")); v != "" {
		q = q.Where(subjectExpr+" = ?", v)
	}
	if v := strings.TrimSpace(c.Query("kelasId")); v != "" {
		q = q.Where(classExpr+" = ?", v)
	}
	if v := strings.TrimSpace(c.Query("pesertaDidikId")); v != "" {
		q = q.Where(studentExpr+" = ?", v)
	}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		// The legacy online-exam schema calls an active attempt "mulai" while
		// Simulasi uses "berlangsung". The shared UI exposes one normalized
		// status so both modules must be included by the same filter.
		if v == "berlangsung" {
			q = q.Where(statusExpr+" IN ?", []string{"mulai", "berlangsung"})
		} else {
			q = q.Where(statusExpr+" = ?", v)
		}
	}
	if from, err := assessmentTimeFilter(c.Query("dari"), false); err != nil {
		return nil, err
	} else if from != nil {
		q = q.Where(startedExpr+" >= ?", *from)
	}
	if until, err := assessmentTimeFilter(c.Query("sampai"), true); err != nil {
		return nil, err
	} else if until != nil {
		q = q.Where(startedExpr+" <= ?", *until)
	}
	return q, nil
}

func (s *Server) loadAssessmentAnalytics(c *fiber.Ctx, export bool) ([]assessmentAnalyticsRow, bool, error) {
	classIDs, err := s.assessmentAnalyticsFilters(c)
	if err != nil {
		return nil, false, err
	}
	module := strings.TrimSpace(c.Query("modul"))
	if module != "" && module != "ujian_online" && module != "simulasi" {
		return nil, false, fiber.NewError(400, "modul harus ujian_online atau simulasi")
	}
	limit := assessmentAnalyticsLimit
	if !export {
		limit = 2000
	}
	rows := make([]assessmentAnalyticsRow, 0)
	truncated := false
	if module == "" || module == "ujian_online" {
		const classExpr = "COALESCE(NULLIF(up.kelas_id_saat_ujian, ''), NULLIF(uj.kelas_id, ''), pd.kelas_id)"
		q := s.db.Table("ujian_peserta up").
			Select("up.id AS id, 'ujian_online' AS module, uj.id AS assessment_id, uj.judul AS assessment_name, 'ujian_online' AS mode, COALESCE(mp.nama_mapel, '') AS subject, pd.id AS student_id, pd.nama AS student_name, pd.nisn AS nisn, " + classExpr + " AS class_id, COALESCE(k.jenjang, 0) AS class_grade, COALESCE(k.nama_rombel, '') AS class_name, up.status AS status, up.mulai AS started_at, up.selesai AS submitted_at, up.skor AS score, 1 AS attempt_no").
			Joins("JOIN ujians uj ON uj.id = up.ujian_id").
			Joins("JOIN peserta_didiks pd ON pd.id = up.peserta_didik_id").
			Joins("LEFT JOIN kelas k ON k.id = " + classExpr).
			Joins("LEFT JOIN mata_pelajarans mp ON mp.id = uj.mapel_id")
		q, err = applyAssessmentAnalyticsFilters(q, c, classExpr, "pd.id", "up.status", "COALESCE(up.mulai, up.created_at)", "uj.id", "uj.mapel_id")
		if err != nil {
			return nil, false, err
		}
		if c.Locals("role") == "guru" {
			q = q.Where("uj.kelas_id IN ?", classIDs)
		}
		var exams []assessmentAnalyticsRow
		if err := q.Order("COALESCE(up.mulai, up.created_at) DESC, up.id").Limit(limit + 1).Scan(&exams).Error; err != nil {
			return nil, false, err
		}
		if len(exams) > limit {
			exams, truncated = exams[:limit], true
		}
		rows = append(rows, exams...)
	}
	if module == "" || module == "simulasi" {
		const classExpr = "COALESCE(NULLIF(up.kelas_id_saat_ujian, ''), NULLIF(sa.kelas_id_saat_tugas, ''), pd.kelas_id)"
		q := s.db.Table("simulasi_upayas up").
			Select("up.id AS id, 'simulasi' AS module, paket.id AS assessment_id, paket.nama AS assessment_name, paket.mode AS mode, COALESCE(mp.nama_mapel, '') AS subject, pd.id AS student_id, pd.nama AS student_name, pd.nisn AS nisn, " + classExpr + " AS class_id, COALESCE(k.jenjang, 0) AS class_grade, COALESCE(k.nama_rombel, '') AS class_name, up.status AS status, up.mulai AS started_at, up.selesai AS submitted_at, up.skor_akhir AS score, up.nomor AS attempt_no").
			Joins("JOIN simulasi_pakets paket ON paket.id = up.paket_id").
			Joins("JOIN peserta_didiks pd ON pd.id = up.peserta_didik_id").
			Joins("LEFT JOIN simulasi_penugasans sa ON sa.paket_id = paket.id AND sa.peserta_didik_id = pd.id").
			Joins("LEFT JOIN kelas k ON k.id = " + classExpr).
			Joins("LEFT JOIN mata_pelajarans mp ON mp.id = paket.mapel_id")
		q, err = applyAssessmentAnalyticsFilters(q, c, classExpr, "pd.id", "up.status", "COALESCE(up.mulai, up.created_at)", "paket.id", "paket.mapel_id")
		if err != nil {
			return nil, false, err
		}
		if c.Locals("role") == "guru" {
			q = q.Where("paket.dibuat_oleh_user_id = ?", c.Locals("userID")).Where(classExpr+" IN ?", classIDs)
		}
		var simulations []assessmentAnalyticsRow
		if err := q.Order("COALESCE(up.mulai, up.created_at) DESC, up.id").Limit(limit + 1).Scan(&simulations).Error; err != nil {
			return nil, false, err
		}
		if len(simulations) > limit {
			simulations, truncated = simulations[:limit], true
		}
		rows = append(rows, simulations...)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].StartedAt == nil {
			return false
		}
		if rows[j].StartedAt == nil {
			return true
		}
		return rows[i].StartedAt.After(*rows[j].StartedAt)
	})
	if len(rows) > limit {
		rows, truncated = rows[:limit], true
	}
	return rows, truncated, nil
}

func summarizeAssessmentRows(rows []assessmentAnalyticsRow) (assessmentAnalyticsSummary, []assessmentStudentProgress, []assessmentClassProgress) {
	result := assessmentAnalyticsSummary{Total: len(rows)}
	totalScore, scored := 0.0, 0
	progressByStudent := map[string]*assessmentStudentProgress{}
	scoreByStudent := map[string]float64{}
	scoredByStudent := map[string]int{}
	progressByClass := map[string]*assessmentClassProgress{}
	studentsByClass := map[string]map[string]struct{}{}
	scoreByClass := map[string]float64{}
	scoredByClass := map[string]int{}
	for _, row := range rows {
		switch row.Status {
		case "selesai", "dikunci", "kedaluwarsa":
			result.Selesai++
		case "menunggu_nilai":
			result.MenungguNilai++
		default:
			result.Berlangsung++
		}
		if row.Score != nil {
			totalScore += *row.Score
			scored++
		}
		classKey := row.ClassID
		if classKey == "" {
			classKey = "__belum_tercatat__"
		}
		classProgress := progressByClass[classKey]
		if classProgress == nil {
			classLabel := "Belum tercatat"
			if row.ClassID != "" {
				classLabel = "Kelas " + strconv.Itoa(row.ClassGrade) + " " + row.ClassName
			}
			classProgress = &assessmentClassProgress{ClassID: row.ClassID, ClassName: classLabel}
			progressByClass[classKey] = classProgress
			studentsByClass[classKey] = map[string]struct{}{}
		}
		classProgress.Total++
		studentsByClass[classKey][row.StudentID] = struct{}{}
		if row.Status == "selesai" || row.Status == "menunggu_nilai" || row.Status == "dikunci" || row.Status == "kedaluwarsa" {
			classProgress.Completed++
		}
		if row.Score != nil {
			scoreByClass[classKey] += *row.Score
			scoredByClass[classKey]++
		}
		progress := progressByStudent[row.StudentID]
		if progress == nil {
			classLabel := "Belum tercatat"
			if row.ClassID != "" {
				classLabel = "Kelas " + strconv.Itoa(row.ClassGrade) + " " + row.ClassName
			}
			progress = &assessmentStudentProgress{StudentID: row.StudentID, StudentName: row.StudentName, NISN: row.NISN, ClassID: row.ClassID, ClassName: classLabel}
			progressByStudent[row.StudentID] = progress
		}
		progress.Total++
		if row.Status == "selesai" || row.Status == "menunggu_nilai" || row.Status == "dikunci" || row.Status == "kedaluwarsa" {
			progress.Completed++
		}
		if row.Score != nil {
			scoreByStudent[row.StudentID] += *row.Score
			scoredByStudent[row.StudentID]++
		}
		if row.StartedAt != nil && (progress.LastAttempt == nil || row.StartedAt.After(*progress.LastAttempt)) {
			progress.LastAttempt = row.StartedAt
		}
	}
	if scored > 0 {
		average := totalScore / float64(scored)
		result.RataRata = &average
	}
	progressRows := make([]assessmentStudentProgress, 0, len(progressByStudent))
	for id, progress := range progressByStudent {
		if scoredByStudent[id] > 0 {
			average := scoreByStudent[id] / float64(scoredByStudent[id])
			progress.Average = &average
		}
		progressRows = append(progressRows, *progress)
	}
	sort.Slice(progressRows, func(i, j int) bool {
		return strings.ToLower(progressRows[i].StudentName) < strings.ToLower(progressRows[j].StudentName)
	})
	classRows := make([]assessmentClassProgress, 0, len(progressByClass))
	for id, progress := range progressByClass {
		progress.Students = len(studentsByClass[id])
		if scoredByClass[id] > 0 {
			average := scoreByClass[id] / float64(scoredByClass[id])
			progress.Average = &average
		}
		classRows = append(classRows, *progress)
	}
	sort.Slice(classRows, func(i, j int) bool {
		return strings.ToLower(classRows[i].ClassName) < strings.ToLower(classRows[j].ClassName)
	})
	return result, progressRows, classRows
}

func (s *Server) assessmentAnalytics(c *fiber.Ctx) error {
	rows, truncated, err := s.loadAssessmentAnalytics(c, false)
	if err != nil {
		return err
	}
	summary, progress, classes := summarizeAssessmentRows(rows)
	return c.JSON(fiber.Map{"ringkasan": summary, "perkembanganSiswa": progress, "statistikKelas": classes, "pengerjaan": rows, "terpotong": truncated, "batasBaris": 2000})
}

func assessmentReportHeaders() []string {
	return []string{"Modul", "Nama asesmen", "Mode", "Mata pelajaran", "Nama siswa", "NISN", "Kelas saat mengerjakan", "Status", "Percobaan", "Mulai", "Selesai", "Nilai"}
}

func assessmentReportValues(row assessmentAnalyticsRow) []string {
	class := "Belum tercatat"
	if row.ClassID != "" {
		class = "Kelas " + strconv.Itoa(row.ClassGrade) + " " + row.ClassName
	}
	started, submitted, score := "", "", ""
	if row.StartedAt != nil {
		started = row.StartedAt.Format(time.RFC3339)
	}
	if row.SubmittedAt != nil {
		submitted = row.SubmittedAt.Format(time.RFC3339)
	}
	if row.Score != nil {
		score = strconv.FormatFloat(*row.Score, 'f', 2, 64)
	}
	return []string{row.Module, row.AssessmentName, row.Mode, row.Subject, row.StudentName, row.NISN, class, row.Status, strconv.Itoa(row.Attempt), started, submitted, score}
}

func csvSafeField(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func (s *Server) exportAssessmentAnalytics(c *fiber.Ctx) error {
	rows, truncated, err := s.loadAssessmentAnalytics(c, true)
	if err != nil {
		return err
	}
	if truncated {
		return fiber.NewError(413, "laporan melebihi 10.000 baris; persempit filter kelas, siswa, asesmen, atau tanggal")
	}
	if strings.EqualFold(c.Query("format"), "xlsx") {
		f := excelize.NewFile()
		const sheet = "Hasil Asesmen"
		f.SetSheetName("Sheet1", sheet)
		for column, header := range assessmentReportHeaders() {
			cell, _ := excelize.CoordinatesToCellName(column+1, 1)
			_ = f.SetCellValue(sheet, cell, header)
		}
		for rowIndex, row := range rows {
			for column, value := range assessmentReportValues(row) {
				cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+2)
				_ = f.SetCellValue(sheet, cell, value)
			}
		}
		_ = f.SetColWidth(sheet, "A", "L", 20)
		_ = f.SetColWidth(sheet, "E", "E", 32)
		var output bytes.Buffer
		if err := f.Write(&output); err != nil {
			return fiber.NewError(500, "gagal membuat file laporan")
		}
		c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Set(fiber.HeaderContentDisposition, "attachment; filename=laporan-hasil-asesmen.xlsx")
		return c.Send(output.Bytes())
	}
	var output bytes.Buffer
	w := csv.NewWriter(&output)
	_ = w.Write(assessmentReportHeaders())
	for _, row := range rows {
		values := assessmentReportValues(row)
		for index := range values {
			values[index] = csvSafeField(values[index])
		}
		_ = w.Write(values)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fiber.NewError(500, fmt.Sprintf("gagal menulis laporan: %v", err))
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, "attachment; filename=laporan-hasil-asesmen.csv")
	return c.Send(output.Bytes())
}

func (s *Server) exportAssessmentAnalyticsPDF(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Query("kelasId")) == "" && strings.TrimSpace(c.Query("pesertaDidikId")) == "" {
		return fiber.NewError(400, "pilih kelas atau siswa sebelum mengunduh laporan PDF")
	}
	rows, truncated, err := s.loadAssessmentAnalytics(c, true)
	if err != nil {
		return err
	}
	if truncated {
		return fiber.NewError(413, "laporan PDF melebihi 10.000 pengerjaan; persempit filter tanggal atau modul")
	}

	title := "Laporan Hasil Asesmen"
	filename := "laporan-asesmen"
	if studentID := strings.TrimSpace(c.Query("pesertaDidikId")); studentID != "" {
		label := "Siswa"
		for _, row := range rows {
			if row.StudentID == studentID {
				label = row.StudentName
				break
			}
		}
		title += " - " + label
		filename = "laporan-asesmen-siswa-" + sanitizeFilename(label)
	} else if classID := strings.TrimSpace(c.Query("kelasId")); classID != "" {
		label := "Kelas"
		for _, row := range rows {
			if row.ClassID == classID {
				label = row.ClassName
				break
			}
		}
		title += " - " + label
		filename = "laporan-asesmen-kelas-" + sanitizeFilename(label)
	}
	generatedAt := time.Now().In(wibLocation).Format("02-01-2006 15:04 WIB")

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(273, 5, fmt.Sprintf("PKBM Tunas Ilmu - dibuat %s - halaman %d", generatedAt, pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(28, 87, 64)
	pdf.CellFormat(273, 6, "PKBM TUNAS ILMU", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(273, 9, title, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)
	pdf.CellFormat(273, 5, fmt.Sprintf("%d pengerjaan - modul: %s - dibuat: %s", len(rows), valueOr(strings.TrimSpace(c.Query("modul")), "semua"), generatedAt), "", 1, "C", false, 0, "")
	pdf.Ln(3)

	widths := []float64{9, 37, 25, 29, 24, 68, 24, 17, 40}
	headers := []string{"No", "Nama siswa", "NISN", "Kelas saat ujian", "Modul", "Nama asesmen", "Status", "Nilai", "Waktu mulai"}
	drawHeader := func() {
		pdf.SetFillColor(28, 87, 64)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 7.5)
		for index, header := range headers {
			pdf.CellFormat(widths[index], 8, header, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(0, 0, 0)
	}
	drawHeader()
	if len(rows) == 0 {
		pdf.CellFormat(273, 12, "Belum ada hasil asesmen pada filter ini.", "1", 1, "C", false, 0, "")
	} else {
		for index, row := range rows {
			className := row.ClassName
			if className == "" {
				className = "Kelas tidak tercatat"
			} else if row.ClassID != "" {
				className = fmt.Sprintf("Kelas %d %s", row.ClassGrade, row.ClassName)
			}
			module := "Ujian Online"
			if row.Module == "simulasi" {
				module = "Simulasi"
			}
			score := "-"
			if row.Score != nil {
				score = strconv.FormatFloat(*row.Score, 'f', 1, 64)
			}
			started := "-"
			if row.StartedAt != nil {
				started = row.StartedAt.In(wibLocation).Format("02-01-2006 15:04")
			}
			values := []string{strconv.Itoa(index + 1), row.StudentName, row.NISN, className, module, row.AssessmentName, row.Status, score, started}
			maxLines := 1
			for column, value := range values {
				if lines := len(pdf.SplitLines([]byte(value), widths[column]-3)); lines > maxLines {
					maxLines = lines
				}
			}
			rowHeight := float64(maxLines)*3.8 + 2.2
			if pdf.GetY()+rowHeight > 193 {
				pdf.AddPage()
				drawHeader()
			}
			x, y := 12.0, pdf.GetY()
			for column, value := range values {
				pdf.Rect(x, y, widths[column], rowHeight, "D")
				pdf.SetXY(x+1.5, y+1)
				align := "L"
				if column == 0 || column == 7 {
					align = "C"
				}
				pdf.MultiCell(widths[column]-3, 3.8, value, "", align, false)
				x += widths[column]
			}
			pdf.SetXY(12, y+rowHeight)
		}
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return fiber.NewError(500, "gagal membuat laporan PDF")
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment(filename + ".pdf")
	return c.Send(output.Bytes())
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
