package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type reportImportStudent struct {
	Row        int               `json:"row"`
	Name       string            `json:"name"`
	NIS        string            `json:"nis"`
	NISN       string            `json:"nisn"`
	StudentID  string            `json:"studentId"`
	Note       string            `json:"note"`
	Attendance ReportAttendance  `json:"attendance"`
	Identity   map[string]string `json:"identity"`
}
type reportImportSubject struct {
	Sheet   string             `json:"sheet"`
	Name    string             `json:"name"`
	MapelID string             `json:"mapelId"`
	Labels  []string           `json:"labels"`
	Values  map[int][]*float64 `json:"values"`
	Issues  []string           `json:"issues"`
}
type reportImportPayload struct {
	Scope    ReportScope           `json:"scope"`
	Revision int                   `json:"revision"`
	Students []reportImportStudent `json:"students"`
	Subjects []reportImportSubject `json:"subjects"`
	Issues   []string              `json:"issues"`
	Source   map[string]string     `json:"source"`
}

func reportMatchName(v string) string {
	v = strings.ToLower(v)
	for _, s := range []string{"pendidikan ", "muatan lokal", "(", ")", " ", ".", "-"} {
		v = strings.ReplaceAll(v, s, "")
	}
	return v
}
func (s *Server) reportImportPreview(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.ErrForbidden
	}
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	file, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "Pilih workbook .xlsm/.xlsx")
	}
	if file.Size > 15*1024*1024 {
		return fiber.NewError(413, "Workbook maksimal 15 MB")
	}
	stream, err := file.Open()
	if err != nil {
		return err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, 15*1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 15*1024*1024 {
		return fiber.ErrRequestEntityTooLarge
	}
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 64 * 1024 * 1024, UnzipXMLSizeLimit: 16 * 1024 * 1024})
	if err != nil {
		return fiber.NewError(400, "Workbook tidak dapat dibaca")
	}
	defer f.Close()
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	cfg, err := s.reportConfig(s.db, k, p)
	if err != nil {
		return err
	}
	payload := reportImportPayload{Scope: p.ReportScope, Revision: p.Revision, Students: []reportImportStudent{}, Subjects: []reportImportSubject{}, Issues: []string{}}
	value := func(sheet, cell string) string { v, _ := f.GetCellValue(sheet, cell); return strings.TrimSpace(v) }
	payload.Source = map[string]string{"year": value("CETAK RAPOR", "AA14"), "semester": value("CETAK RAPOR", "AA13"), "class": value("CETAK RAPOR", "AA11")}
	for key, cell := range map[string]string{"program": "A9", "phase": "AA12", "placeDate": "U61", "teacher": "T68", "teacherNip": "U69", "principal": "J78", "principalNip": "J79"} {
		payload.Source[key] = value("CETAK RAPOR", cell)
	}
	if sourceYear := payload.Source["year"]; sourceYear != "" && strings.ReplaceAll(sourceYear, " ", "") != strings.ReplaceAll(k.TahunAjaran.NamaTahunAjaran, " ", "") {
		payload.Issues = append(payload.Issues, "Tahun ajaran workbook ("+sourceYear+") berbeda dari periode tujuan")
	}
	if sourceSemester := payload.Source["semester"]; sourceSemester != "" {
		semester := ""
		if strings.HasPrefix(sourceSemester, "1") {
			semester = "Ganjil"
		}
		if strings.HasPrefix(sourceSemester, "2") {
			semester = "Genap"
		}
		if semester != "" && semester != p.Semester {
			payload.Issues = append(payload.Issues, "Semester workbook berbeda dari semester tujuan")
		}
	}
	if sourceClass := strings.Fields(payload.Source["class"]); len(sourceClass) > 0 && sourceClass[0] != reportRoman(k.Jenjang) && sourceClass[0] != strconv.Itoa(k.Jenjang) {
		payload.Issues = append(payload.Issues, "Jenjang workbook berbeda dari kelas tujuan")
	}
	if index, e := f.GetSheetIndex("IDENTITAS PD"); e != nil || index < 0 {
		return fiber.NewError(400, "Sheet IDENTITAS PD tidak ditemukan")
	}
	for row := 4; row <= 1000; row++ {
		name := value("IDENTITAS PD", fmt.Sprintf("B%d", row))
		if name == "" {
			continue
		}
		nis := value("IDENTITAS PD", fmt.Sprintf("D%d", row))
		nisn := value("IDENTITAS PD", fmt.Sprintf("E%d", row))
		student := reportImportStudent{Row: row, Name: name, NIS: nis, NISN: nisn, Note: value("CATATAN & PRESENSI", fmt.Sprintf("E%d", row)), Attendance: ReportAttendance{Conflicts: []string{}}}
		student.Identity = map[string]string{}
		for col, key := range map[string]string{"F": "birthPlace", "H": "phone", "I": "address", "J": "father", "K": "mother", "L": "fatherJob", "M": "motherJob", "N": "guardian", "O": "guardianJob"} {
			student.Identity[key] = value("IDENTITAS PD", fmt.Sprintf("%s%d", col, row))
		}
		raw, _ := f.GetCellValue("IDENTITAS PD", fmt.Sprintf("G%d", row), excelize.Options{RawCellValue: true})
		if serial, e := strconv.ParseFloat(raw, 64); e == nil {
			if date, e := excelize.ExcelDateToTime(serial, false); e == nil {
				student.Identity["birthDate"] = date.Format("2006-01-02")
			}
		}
		candidates := []string{}
		for _, pd := range roster {
			if nis != "" && pd.NIS == nis || nisn != "" && nisn != temporaryNISN && pd.NISN == nisn {
				candidates = append(candidates, pd.ID)
			}
		}
		if len(candidates) == 1 {
			student.StudentID = candidates[0]
		}
		counts := []*int{&student.Attendance.Sakit, &student.Attendance.Izin, &student.Attendance.Alpa}
		for i, col := range []string{"F", "G", "H"} {
			raw := value("CATATAN & PRESENSI", fmt.Sprintf("%s%d", col, row))
			if raw == "" || raw == "-" {
				continue
			}
			n, e := strconv.Atoi(raw)
			if e != nil || n < 0 {
				payload.Issues = append(payload.Issues, fmt.Sprintf("Presensi %s baris %d tidak valid", col, row))
			} else {
				*counts[i] = n
			}
		}
		payload.Students = append(payload.Students, student)
	}
	if len(payload.Students) == 0 {
		return fiber.NewError(400, "Tidak ditemukan baris siswa")
	}
	for _, sheet := range f.GetSheetList() {
		if sheet == "IDENTITAS PD" || strings.HasPrefix(sheet, "CETAK") || sheet == "LEGER" || sheet == "CATATAN & PRESENSI" {
			continue
		}
		sub := reportImportSubject{Sheet: sheet, Name: value(sheet, "A2"), Labels: []string{}, Values: map[int][]*float64{}, Issues: []string{}}
		if sub.Name == "" {
			continue
		}
		gradeRows := map[int]int{}
		for sourceRow := 7; sourceRow <= 1003; sourceRow++ {
			nis := value(sheet, fmt.Sprintf("C%d", sourceRow))
			nisn := value(sheet, fmt.Sprintf("D%d", sourceRow))
			if nis == "" && nisn == "" {
				continue
			}
			for _, pd := range payload.Students {
				if nis != "" && nis == pd.NIS || nisn != "" && nisn == pd.NISN && nisn != temporaryNISN {
					if previous, exists := gradeRows[pd.Row]; exists && previous != sourceRow {
						sub.Issues = append(sub.Issues, "Identitas siswa duplikat pada baris nilai: "+pd.Name)
					}
					gradeRows[pd.Row] = sourceRow
					if nis != "" && pd.NIS != "" && nis != pd.NIS || nisn != "" && pd.NISN != "" && nisn != pd.NISN {
						sub.Issues = append(sub.Issues, "NIS dan NISN tidak konsisten: "+pd.Name)
					}
				}
			}
		}
		for _, pd := range payload.Students {
			if gradeRows[pd.Row] == 0 {
				sub.Issues = append(sub.Issues, "Identitas siswa tidak ditemukan pada sheet nilai: "+pd.Name)
			}
		}
		for _, mp := range cfg.Subjects {
			if reportMatchName(mp.Name) == reportMatchName(sub.Name) {
				sub.MapelID = mp.MapelID
			}
		}
		for col := 6; col <= 15; col++ {
			colName, _ := excelize.ColumnNumberToName(col)
			label := value(sheet, colName+"6")
			active := label != ""
			vals := map[int]*float64{}
			for _, pd := range payload.Students {
				row := gradeRows[pd.Row]
				if row == 0 {
					continue
				}
				raw := value(sheet, fmt.Sprintf("%s%d", colName, row))
				if raw == "" {
					continue
				}
				active = true
				n, e := strconv.ParseFloat(raw, 64)
				if e != nil || n < 0 || n > 100 {
					sub.Issues = append(sub.Issues, fmt.Sprintf("%s%d: nilai tidak valid (%s)", colName, row, raw))
					continue
				}
				v := n
				vals[pd.Row] = &v
			}
			if !active {
				continue
			}
			if label == "" {
				label = fmt.Sprintf("CP %d", col-5)
				sub.Issues = append(sub.Issues, "Label "+label+" belum diisi di workbook")
			}
			sub.Labels = append(sub.Labels, label)
			for _, pd := range payload.Students {
				sub.Values[pd.Row] = append(sub.Values[pd.Row], vals[pd.Row])
			}
		}
		payload.Subjects = append(payload.Subjects, sub)
	}
	// Bind deduplication to the destination; preview alone never writes academic data.
	hash := sha256.Sum256(append(data, []byte(reportJSON(p.ReportScope))...))
	hashText := hex.EncodeToString(hash[:])
	var imp ReportImport
	err = s.db.Where("hash = ?", hashText).First(&imp).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if imp.Confirmed {
		return fiber.NewError(409, "Workbook ini sudah diimpor ke periode tersebut")
	}
	imp.Hash = hashText
	imp.Payload = reportJSON(payload)
	if err = s.db.Save(&imp).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"id": imp.ID, "preview": payload, "students": roster, "subjects": cfg.Subjects})
}
func (s *Server) reportImportConfirm(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.ErrForbidden
	}
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	var in struct {
		Revision int               `json:"revision"`
		Students map[int]string    `json:"students"`
		Subjects map[string]string `json:"subjects"`
	}
	if c.BodyParser(&in) != nil {
		return fiber.ErrBadRequest
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var imp ReportImport
		if e := tx.First(&imp, "id = ?", c.Params("id")).Error; e != nil {
			return fiber.ErrNotFound
		}
		if imp.Confirmed {
			return fiber.NewError(409, "Impor sudah dikonfirmasi")
		}
		var payload reportImportPayload
		if e := json.Unmarshal([]byte(imp.Payload), &payload); e != nil {
			return e
		}
		if payload.Scope != p.ReportScope {
			return fiber.NewError(400, "Periode impor berbeda")
		}
		if len(payload.Issues) > 0 {
			return fiber.NewError(400, "Perbaiki masalah workbook sebelum mengimpor")
		}
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		if e := reportUnlocked(tx, p.ID, ""); e != nil {
			return e
		}
		roster, e := s.reportRoster(tx, k)
		if e != nil {
			return e
		}
		valid := map[string]bool{}
		for _, pd := range roster {
			valid[pd.ID] = true
		}
		studentIDs := map[int]string{}
		used := map[string]bool{}
		for _, pd := range payload.Students {
			id := pd.StudentID
			if override, ok := in.Students[pd.Row]; ok {
				id = override
			}
			if id == "skip" {
				continue
			}
			if !valid[id] || used[id] {
				return fiber.NewError(400, "Cocokkan semua siswa atau pilih Lewati secara eksplisit")
			}
			used[id] = true
			studentIDs[pd.Row] = id
		}
		if len(studentIDs) == 0 {
			return fiber.NewError(400, "Tidak ada siswa dipilih")
		}
		cfg, e := s.reportConfig(tx, k, p)
		if e != nil {
			return e
		}
		validMapel := map[string]bool{}
		for _, mp := range cfg.Subjects {
			validMapel[mp.MapelID] = true
		}
		usedMapel := map[string]bool{}
		importOrder := []ReportSubjectConfig{}
		for _, sub := range payload.Subjects {
			mp := sub.MapelID
			if override, ok := in.Subjects[sub.Sheet]; ok {
				mp = override
			}
			if mp == "skip" {
				continue
			}
			if !validMapel[mp] || usedMapel[mp] {
				return fiber.NewError(400, "Cocokkan setiap sheet mapel atau pilih Lewati")
			}
			usedMapel[mp] = true
			for _, subject := range cfg.Subjects {
				if subject.MapelID == mp {
					subject.Name = sub.Name
					importOrder = append(importOrder, subject)
					break
				}
			}
			if len(sub.Issues) > 0 {
				return fiber.NewError(400, sub.Sheet+": "+strings.Join(sub.Issues, "; "))
			}
			var existing int64
			if e = tx.Model(&ReportCP{}).Where("period_id = ? AND mapel_id = ?", p.ID, mp).Count(&existing).Error; e != nil {
				return e
			}
			if existing > 0 {
				return fiber.NewError(409, sub.Name+": sudah ada CP; impor tidak menimpa data")
			}
			for i, label := range sub.Labels {
				cp := ReportCP{PeriodID: p.ID, MapelID: mp, Label: label, Urutan: i + 1}
				if e = tx.Create(&cp).Error; e != nil {
					return e
				}
				for row, id := range studentIDs {
					values := sub.Values[row]
					if i >= len(values) || values[i] == nil {
						continue
					}
					score := ReportScore{CPID: cp.ID, StudentID: id, Value: values[i]}
					if e = tx.Create(&score).Error; e != nil {
						return e
					}
				}
			}
		}
		for _, pd := range payload.Students {
			id := studentIDs[pd.Row]
			if id == "" {
				continue
			}
			var rec ReportRecord
			e = tx.Where("period_id = ? AND student_id = ?", p.ID, id).First(&rec).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			if rec.ID != "" {
				return fiber.NewError(409, "Catatan siswa sudah ada; impor tidak menimpa data")
			}
			rec.PeriodID = p.ID
			rec.StudentID = id
			rec.Status = "draft"
			rec.Details = reportJSON(ReportDetails{Note: pd.Note, HistoricalAttendance: &pd.Attendance, Identity: pd.Identity})
			if e = tx.Create(&rec).Error; e != nil {
				return e
			}
		}
		result := tx.Model(&ReportImport{}).Where("id = ? AND confirmed = ?", imp.ID, false).Update("confirmed", true)
		if p.Config == "" {
			for _, subject := range cfg.Subjects {
				if !usedMapel[subject.MapelID] {
					importOrder = append(importOrder, subject)
				}
			}
			cfg.Subjects = importOrder
			for key, target := range map[string]*string{"program": &cfg.Program, "phase": &cfg.Phase, "teacher": &cfg.Teacher, "teacherNip": &cfg.TeacherNIP, "principal": &cfg.Principal, "principalNip": &cfg.PrincipalNIP} {
				if value := payload.Source[key]; value != "" && len([]rune(value)) < 200 {
					if strings.HasSuffix(key, "Nip") {
						value = strings.TrimSpace(strings.TrimPrefix(value, "NIP."))
					}
					*target = value
				}
			}
			if parts := strings.SplitN(payload.Source["placeDate"], ",", 2); len(parts) == 2 {
				cfg.City = strings.TrimSpace(parts[0])
				tokens := strings.Fields(parts[1])
				if len(tokens) == 3 {
					months := map[string]int{"januari": 1, "februari": 2, "maret": 3, "april": 4, "mei": 5, "juni": 6, "juli": 7, "agustus": 8, "september": 9, "oktober": 10, "november": 11, "desember": 12}
					day, e1 := strconv.Atoi(tokens[0])
					year, e2 := strconv.Atoi(tokens[2])
					if e1 == nil && e2 == nil {
						iso := fmt.Sprintf("%04d-%02d-%02d", year, months[strings.ToLower(tokens[1])], day)
						if _, e := time.Parse("2006-01-02", iso); e == nil {
							cfg.Date = iso
						}
					}
				}
			}
			if e := tx.Model(&p).Update("config", reportJSON(cfg)).Error; e != nil {
				return e
			}
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fiber.ErrConflict
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "import", c.Params("id"))
	return c.JSON(fiber.Map{"period": p, "message": "Impor berhasil; nilai kosong tetap belum terisi"})
}
