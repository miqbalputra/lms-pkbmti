package main

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Stored in the existing report JSON; no changes to attendance sessions or schema.
type ReportAttendanceCounts struct {
	Sakit int `json:"sakit"`
	Izin  int `json:"izin"`
	Alpa  int `json:"alpa"`
}

func reportAttendanceMode(cfg ReportConfig) string {
	if cfg.AttendanceMode == "manual" {
		return "manual"
	}
	return "automatic"
}

func (s *Server) reportResolveAttendance(db *gorm.DB, k Kelas, p ReportPeriod, studentID string, cfg ReportConfig, details ReportDetails) (ReportAttendance, string, error) {
	if details.HistoricalAttendance != nil {
		return *details.HistoricalAttendance, "historical", nil
	}
	if reportAttendanceMode(cfg) == "manual" {
		result := ReportAttendance{Conflicts: []string{}}
		if counts := details.ManualAttendance; counts != nil {
			result.Sakit, result.Izin, result.Alpa = counts.Sakit, counts.Izin, counts.Alpa
		}
		return result, "manual", nil
	}
	result, err := s.reportAttendance(db, k, p, studentID)
	return result, "automatic", err
}

func (s *Server) reportSaveAttendanceMode(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.ErrForbidden
	}
	_, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	var in struct {
		Revision int    `json:"revision"`
		Mode     string `json:"mode"`
	}
	if c.BodyParser(&in) != nil || (in.Mode != "manual" && in.Mode != "automatic") {
		return fiber.NewError(400, "Pilih sumber presensi otomatis atau manual")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		if e := reportUnlocked(tx, p.ID, ""); e != nil {
			return e
		}
		cfg := map[string]json.RawMessage{}
		if p.Config != "" {
			if e := json.Unmarshal([]byte(p.Config), &cfg); e != nil {
				return e
			}
		}
		if cfg == nil {
			cfg = map[string]json.RawMessage{}
		}
		cfg["attendanceMode"] = json.RawMessage(reportJSON(in.Mode))
		return tx.Model(&p).Update("config", reportJSON(cfg)).Error
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "sumber-presensi:"+in.Mode, p.ID)
	return c.JSON(p)
}

type reportAttendanceRow struct {
	StudentID  string                  `json:"studentId"`
	Status     string                  `json:"status"`
	Source     string                  `json:"source"`
	Attendance ReportAttendance        `json:"attendance"`
	Manual     *ReportAttendanceCounts `json:"manual"`
}

func (s *Server) reportAttendanceGrid(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	if c.Locals("role") != "kepala_sekolah" {
		if err = s.canManageKelas(c, k.ID); err != nil {
			return err
		}
	}
	cfg, err := s.reportConfig(s.db, k, p)
	if err != nil {
		return err
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	records := []ReportRecord{}
	if p.ID != "" {
		if err = s.db.Where("period_id = ?", p.ID).Find(&records).Error; err != nil {
			return err
		}
	}
	byStudent := map[string]ReportRecord{}
	for _, rec := range records {
		byStudent[rec.StudentID] = rec
	}
	rows := []reportAttendanceRow{}
	for _, student := range roster {
		rec := byStudent[student.ID]
		row := reportAttendanceRow{StudentID: student.ID, Status: rec.Status}
		if row.Status == "" {
			row.Status = "draft"
		}
		if (rec.Status == "final" || rec.Status == "published") && rec.Snapshot != "" {
			var frozen ReportDocument
			if err = json.Unmarshal([]byte(rec.Snapshot), &frozen); err != nil {
				return err
			}
			row.Attendance, row.Source, row.Manual = frozen.Attendance, "frozen", frozen.Details.ManualAttendance
		} else {
			var details ReportDetails
			reportDecode(rec.Details, &details)
			row.Manual = details.ManualAttendance
			row.Attendance, row.Source, err = s.reportResolveAttendance(s.db, k, p, student.ID, cfg, details)
			if err != nil {
				return err
			}
		}
		rows = append(rows, row)
	}
	var semester Semester
	err = s.db.Where("tahun_ajaran_id = ? AND nama_semester = ?", k.TahunAjaranID, p.Semester).First(&semester).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return c.JSON(fiber.Map{"period": p, "mode": reportAttendanceMode(cfg), "rows": rows, "readOnly": !k.TahunAjaran.IsAktif || semester.IsArchived})
}

func (s *Server) reportSaveAttendance(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, true)
	if err != nil {
		return err
	}
	if err = s.canManageKelas(c, k.ID); err != nil {
		return err
	}
	var in struct {
		Revision int `json:"revision"`
		Rows     []struct {
			StudentID string `json:"studentId"`
			// Pointers distinguish an explicit zero from an omitted count.
			Sakit *int `json:"sakit"`
			Izin  *int `json:"izin"`
			Alpa  *int `json:"alpa"`
		} `json:"rows"`
	}
	if c.BodyParser(&in) != nil || len(in.Rows) == 0 {
		return fiber.NewError(400, "Isi rekap presensi yang akan disimpan")
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	valid, seen := map[string]bool{}, map[string]bool{}
	for _, student := range roster {
		valid[student.ID] = true
	}
	for _, row := range in.Rows {
		if !valid[row.StudentID] || seen[row.StudentID] {
			return fiber.NewError(400, "Siswa tidak valid atau berulang")
		}
		seen[row.StudentID] = true
		if row.Sakit == nil || row.Izin == nil || row.Alpa == nil ||
			*row.Sakit < 0 || *row.Sakit > 366 || *row.Izin < 0 || *row.Izin > 366 || *row.Alpa < 0 || *row.Alpa > 366 ||
			*row.Sakit+*row.Izin+*row.Alpa > 366 {
			return fiber.NewError(400, "Sakit, Izin, dan Alpa wajib berupa bilangan bulat 0–366 hari; total maksimal 366 hari")
		}
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if e := reportLock(tx, &p, in.Revision); e != nil {
			return e
		}
		cfg, e := s.reportConfig(tx, k, p)
		if e != nil {
			return e
		}
		if reportAttendanceMode(cfg) != "manual" {
			return fiber.NewError(409, "Admin belum mengaktifkan rekap presensi manual untuk kelas dan semester ini")
		}
		for _, row := range in.Rows {
			if e = reportUnlocked(tx, p.ID, row.StudentID); e != nil {
				return e
			}
			var rec ReportRecord
			e = tx.Where("period_id = ? AND student_id = ?", p.ID, row.StudentID).First(&rec).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			var details ReportDetails
			reportDecode(rec.Details, &details)
			if details.HistoricalAttendance != nil {
				return fiber.NewError(409, "Rekap historis hasil impor tetap dipertahankan")
			}
			details.ManualAttendance = &ReportAttendanceCounts{Sakit: *row.Sakit, Izin: *row.Izin, Alpa: *row.Alpa}
			rec.PeriodID, rec.StudentID, rec.Status, rec.Details = p.ID, row.StudentID, "draft", reportJSON(details)
			if e = tx.Save(&rec).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "rekap-presensi-manual", p.ID)
	return c.JSON(p)
}
