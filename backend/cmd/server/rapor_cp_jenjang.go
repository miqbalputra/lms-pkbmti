package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReportCPAssignment struct {
	Base
	TahunAjaranID string `gorm:"uniqueIndex:report_cp_assignment;not null" json:"tahunAjaranId"`
	Jenjang       int    `gorm:"uniqueIndex:report_cp_assignment;not null" json:"jenjang"`
	UserID        string `gorm:"not null" json:"userId"`
}

type ReportCPTemplate struct {
	Base
	TahunAjaranID string `gorm:"uniqueIndex:report_cp_template;not null"`
	Jenjang       int    `gorm:"uniqueIndex:report_cp_template;not null"`
	Semester      string `gorm:"uniqueIndex:report_cp_template;not null"`
	MapelID       string `gorm:"uniqueIndex:report_cp_template;not null"`
	Revision      int
	Definitions   string `gorm:"type:text;not null"`
}

type reportCPDefinition struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func reportCPGradeClasses(db *gorm.DB, year string, grade int) *gorm.DB {
	return db.Model(&Kelas{}).Select("id").Where("tahun_ajaran_id = ? AND jenjang = ?", year, grade)
}

func reportCPGradeSubjects(db *gorm.DB, year string, grade int) *gorm.DB {
	return db.Model(&KelasMapel{}).Select("mapel_id").Where("kelas_id IN (?)", reportCPGradeClasses(db, year, grade))
}

func (s *Server) reportListCPAssignments(c *fiber.Ctx) error {
	role := c.Locals("role")
	if role != "admin" && role != "guru" {
		return fiber.ErrForbidden
	}
	type teacher struct {
		ID   string `json:"id"`
		Nama string `json:"nama"`
	}
	type gradeRow struct {
		Jenjang  int             `json:"jenjang"`
		UserID   string          `json:"userId"`
		Subjects []MataPelajaran `json:"subjects,omitempty"`
	}
	var year TahunAjaran
	teachers, grades := []teacher{}, []gradeRow{}
	err := s.db.Where("is_aktif = ?", true).First(&year).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if year.ID != "" {
		assignments := []ReportCPAssignment{}
		if err = s.db.Where("tahun_ajaran_id = ?", year.ID).Find(&assignments).Error; err != nil {
			return err
		}
		owners := map[int]string{}
		for _, a := range assignments {
			owners[a.Jenjang] = a.UserID
		}
		var levels []int
		if err = s.db.Model(&Kelas{}).Where("tahun_ajaran_id = ? AND jenjang > 0", year.ID).Distinct().Order("jenjang").Pluck("jenjang", &levels).Error; err != nil {
			return err
		}
		for _, level := range levels {
			row := gradeRow{Jenjang: level, UserID: owners[level]}
			if role == "guru" {
				if row.UserID != c.Locals("userID") {
					continue
				}
				if err = s.db.Where("id IN (?)", reportCPGradeSubjects(s.db, year.ID, level)).Order("nama_mapel, id").Find(&row.Subjects).Error; err != nil {
					return err
				}
			}
			grades = append(grades, row)
		}
		if role == "admin" {
			if err = s.db.Table("users").Select("users.id, tutors.nama").Joins("JOIN tutors ON tutors.id = users.tutor_id").Where("users.role = ? AND users.is_active = ?", "guru", true).Order("tutors.nama, users.id").Scan(&teachers).Error; err != nil {
				return err
			}
		}
	}
	return c.JSON(fiber.Map{"year": year, "isAdmin": role == "admin", "grades": grades, "teachers": teachers})
}

func (s *Server) reportCPGrade(c *fiber.Ctx) (string, int, error) {
	year := c.Query("tahunAjaranId")
	grade, err := strconv.Atoi(c.Params("jenjang"))
	if err != nil || grade < 1 || year == "" {
		return "", 0, fiber.ErrBadRequest
	}
	if year != s.activeYearID() {
		return "", 0, fiber.NewError(409, "Tahun ajaran ini sudah tidak aktif")
	}
	return year, grade, nil
}

func (s *Server) reportSetCPAssignment(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.ErrForbidden
	}
	year, grade, err := s.reportCPGrade(c)
	if err != nil {
		return err
	}
	var in struct {
		UserID *string `json:"userId"`
	}
	if c.BodyParser(&in) != nil || in.UserID == nil {
		return fiber.ErrBadRequest
	}
	var n int64
	if err = reportCPGradeClasses(s.db, year, grade).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fiber.NewError(400, "Belum ada rombel untuk jenjang ini")
	}
	if *in.UserID != "" {
		err = s.db.Model(&User{}).Joins("JOIN tutors ON tutors.id = users.tutor_id").Where("users.id = ? AND users.role = ? AND users.is_active = ?", *in.UserID, "guru", true).Count(&n).Error
		if err != nil {
			return err
		}
		if n != 1 {
			return fiber.NewError(400, "Pilih akun guru yang aktif")
		}
	}
	a := ReportCPAssignment{TahunAjaranID: year, Jenjang: grade, UserID: *in.UserID}
	if err = s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tahun_ajaran_id"}, {Name: "jenjang"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "updated_at"}),
	}).Create(&a).Error; err != nil {
		return err
	}
	s.reportAudit(c, "penugasan_cp", fmt.Sprintf("%s/%d/%s", year, grade, a.UserID))
	return c.JSON(fiber.Map{"userId": a.UserID})
}

func loadReportCPTemplate(db *gorm.DB, year string, grade int, semester, subject string) (ReportCPTemplate, []reportCPDefinition, error) {
	var template ReportCPTemplate
	defs := []reportCPDefinition{}
	err := db.Where("tahun_ajaran_id = ? AND jenjang = ? AND semester = ? AND mapel_id = ?", year, grade, semester, subject).First(&template).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return template, defs, nil
	}
	if err == nil {
		err = json.Unmarshal([]byte(template.Definitions), &defs)
	}
	return template, defs, err
}

func (s *Server) reportCPTemplate(c *fiber.Ctx) error {
	if c.Locals("role") != "guru" {
		return fiber.ErrForbidden
	}
	year, grade, err := s.reportCPGrade(c)
	if err != nil {
		return err
	}
	semester, subject := c.Query("semester"), c.Query("mapelId")
	if !reportSemester(semester) || subject == "" {
		return fiber.ErrBadRequest
	}
	var n int64
	if err = s.db.Model(&ReportCPAssignment{}).Where("tahun_ajaran_id = ? AND jenjang = ? AND user_id = ?", year, grade, c.Locals("userID")).Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return fiber.ErrForbidden
	}
	if err = reportCPGradeSubjects(s.db, year, grade).Where("mapel_id = ?", subject).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fiber.NewError(400, "Mapel belum terdaftar pada jenjang ini")
	}
	var sem Semester
	err = s.db.Where("tahun_ajaran_id = ? AND nama_semester = ?", year, semester).First(&sem).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if c.Method() == fiber.MethodGet {
		template, defs, err := loadReportCPTemplate(s.db, year, grade, semester, subject)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"revision": template.Revision, "cps": defs, "archived": sem.IsArchived})
	}
	if sem.IsArchived {
		return fiber.NewError(409, "Semester diarsipkan")
	}
	var in struct {
		Revision int                  `json:"revision"`
		CPs      []reportCPDefinition `json:"cps"`
	}
	if c.BodyParser(&in) != nil || len(in.CPs) < 1 || len(in.CPs) > 100 {
		return fiber.NewError(400, "Isi 1–100 CP")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Lock this assignment through commit so reassignment and CP edits are ordered.
		lock := tx.Model(&ReportCPAssignment{}).Where("tahun_ajaran_id = ? AND jenjang = ? AND user_id = ?", year, grade, c.Locals("userID")).UpdateColumn("updated_at", gorm.Expr("updated_at"))
		if lock.Error != nil {
			return lock.Error
		}
		if lock.RowsAffected != 1 {
			return fiber.ErrForbidden
		}
		template, old, e := loadReportCPTemplate(tx, year, grade, semester, subject)
		if e != nil {
			return e
		}
		if template.Revision != in.Revision {
			return fiber.NewError(409, "CP berubah. Muat ulang sebelum menyimpan")
		}
		remaining := map[string]string{}
		for _, cp := range old {
			remaining[cp.ID] = cp.Label
		}
		for i := range in.CPs {
			cp := &in.CPs[i]
			cp.Label = reportLowercaseCPStart(strings.TrimSpace(cp.Label))
			if cp.Label == "" || len([]rune(cp.Label)) > 500 {
				return fiber.NewError(400, "Label CP wajib diisi (maksimal 500 karakter)")
			}
			if cp.ID == "" {
				cp.ID = uuid.NewString()
				continue
			}
			if _, ok := remaining[cp.ID]; !ok {
				return fiber.NewError(400, "ID CP tidak valid")
			}
			delete(remaining, cp.ID)
		}
		removed := []string{}
		for id := range remaining {
			removed = append(removed, id)
		}
		if len(removed) > 0 {
			localIDs := tx.Model(&ReportCP{}).Select("id").Where("shared_cp_id IN ?", removed)
			var scores int64
			if e = tx.Model(&ReportScore{}).Where("cp_id IN (?) AND value IS NOT NULL", localIDs).Count(&scores).Error; e != nil {
				return e
			}
			if scores > 0 {
				return fiber.NewError(409, "CP yang sudah memiliki nilai siswa tidak dapat dihapus")
			}
		}
		definitions := reportJSON(in.CPs)
		if definitions == template.Definitions {
			return nil
		}
		if template.ID == "" {
			template = ReportCPTemplate{TahunAjaranID: year, Jenjang: grade, Semester: semester, MapelID: subject}
		}
		template.Revision++
		in.Revision = template.Revision
		template.Definitions = definitions
		if e = tx.Save(&template).Error; e != nil {
			return e
		}
		// Local legacy CP and frozen report snapshots are preserved. Draft grids
		// using the template refresh their revisions and flag saved narratives.
		classes := reportCPGradeClasses(tx, year, grade).Where("id IN (?)", tx.Model(&KelasMapel{}).Select("kelas_id").Where("mapel_id = ?", subject))
		legacy := tx.Model(&ReportCP{}).Select("period_id").Where("mapel_id = ? AND (shared_cp_id IS NULL OR shared_cp_id = '')", subject)
		periods := tx.Model(&ReportPeriod{}).Where("kelas_id IN (?) AND tahun_ajaran_id = ? AND semester = ?", classes, year, semester)
		shared := periods.Session(&gorm.Session{}).Where("id NOT IN (?)", legacy)
		if e = tx.Model(&ReportNarrative{}).Where("mapel_id = ? AND period_id IN (?)", subject, shared.Session(&gorm.Session{}).Select("id")).Update("reviewed_revision", -1).Error; e != nil {
			return e
		}
		if e = reportAdoptMatchingCPs(tx, periods.Session(&gorm.Session{}).Select("id"), subject, in.CPs); e != nil {
			return e
		}
		return shared.Update("revision", gorm.Expr("revision + 1")).Error
	})
	if err != nil {
		return err
	}
	s.reportAudit(c, "cp_jenjang", fmt.Sprintf("%s/%d/%s/%s", year, grade, semester, subject))
	return c.JSON(fiber.Map{"revision": in.Revision, "cps": in.CPs, "archived": false})
}

func reportAdoptMatchingCPs(db *gorm.DB, periods *gorm.DB, subject string, defs []reportCPDefinition) error {
	// Legacy CPs are matched as a complete list. Scores stay attached to the
	// same local IDs and even matching labels keep their original stored text.
	var rows []ReportCP
	if err := db.Where("period_id IN (?) AND mapel_id = ?", periods, subject).Order("period_id, urutan, id").Find(&rows).Error; err != nil {
		return err
	}
	byPeriod := map[string][]ReportCP{}
	for _, row := range rows {
		byPeriod[row.PeriodID] = append(byPeriod[row.PeriodID], row)
	}
	for _, cps := range byPeriod {
		if len(cps) != len(defs) {
			continue
		}
		matches := true
		for i, cp := range cps {
			if cp.SharedCPID != "" || reportLowercaseCPStart(strings.TrimSpace(cp.Label)) != defs[i].Label {
				matches = false
				break
			}
		}
		if matches {
			for i, cp := range cps {
				if err := db.Model(&cp).UpdateColumn("shared_cp_id", defs[i].ID).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type reportCPGrid struct {
	CPs    []ReportCP
	Source string
}

func lockReportCPAssignment(db *gorm.DB, k Kelas) error {
	return db.Model(&ReportCPAssignment{}).Where("tahun_ajaran_id = ? AND jenjang = ?", k.TahunAjaranID, k.Jenjang).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error
}

func reportEffectiveCPs(db *gorm.DB, k Kelas, p ReportPeriod, subject string) (reportCPGrid, error) {
	grid := reportCPGrid{CPs: []ReportCP{}, Source: "pending"}
	mapped := map[string]ReportCP{}
	if p.ID != "" {
		var rows []ReportCP
		if err := db.Where("period_id = ? AND mapel_id = ?", p.ID, subject).Order("urutan, id").Find(&rows).Error; err != nil {
			return grid, err
		}
		for _, row := range rows {
			if row.SharedCPID == "" {
				grid.CPs = append(grid.CPs, row)
			} else {
				mapped[row.SharedCPID] = row
			}
		}
	}
	if len(grid.CPs) > 0 {
		grid.Source = "legacy"
		return grid, nil
	}
	_, defs, err := loadReportCPTemplate(db, k.TahunAjaranID, k.Jenjang, p.Semester, subject)
	if err != nil || len(defs) == 0 {
		return grid, err
	}
	grid.Source = "shared"
	for i, def := range defs {
		cp := mapped[def.ID]
		if cp.ID == "" {
			cp.ID = def.ID
		}
		cp.SharedCPID, cp.PeriodID, cp.MapelID, cp.Label, cp.Urutan = def.ID, p.ID, subject, def.Label, i+1
		grid.CPs = append(grid.CPs, cp)
	}
	return grid, nil
}

func reportCPIDs(cps []ReportCP) []string {
	ids := make([]string, 0, len(cps))
	for _, cp := range cps {
		ids = append(ids, cp.ID)
	}
	return ids
}
