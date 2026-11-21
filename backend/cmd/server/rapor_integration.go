package main

import (
	"encoding/json"

	"gorm.io/gorm"
)

// Published report snapshots feed the existing parent grade/performance API.
// Legacy calculations remain stored independently. A new report's draft must
// never reveal a value from the legacy calculation for the same period.
func reportParentGrades(db *gorm.DB, studentID string, legacy []parentGradeItem) ([]parentGradeItem, error) {
	if !db.Migrator().HasTable(&ReportRecord{}) {
		return legacy, nil
	}
	records := []ReportRecord{}
	if err := db.Where("student_id = ?", studentID).Order("created_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	blocked := map[string]bool{}
	out := []parentGradeItem{}
	for _, rec := range records {
		var period ReportPeriod
		if err := db.First(&period, "id = ?", rec.PeriodID).Error; err != nil {
			return nil, err
		}
		blocked[period.TahunAjaranID+"/"+period.Semester] = true
		if rec.Status != "published" {
			continue
		}
		var d ReportDocument
		if err := json.Unmarshal([]byte(rec.Snapshot), &d); err != nil {
			return nil, err
		}
		for _, mp := range d.Subjects {
			out = append(out, parentGradeItem{ID: rec.ID + "/" + mp.MapelID, PesertaDidikID: studentID, KelasID: d.Scope.KelasID, MapelID: mp.MapelID, TahunAjaranID: d.Scope.TahunAjaranID, Semester: d.Scope.Semester, NAAkhir: mp.Score, Mapel: parentMapelSummary{ID: mp.MapelID, NamaMapel: mp.Name}})
		}
	}
	for _, row := range legacy {
		if !blocked[row.TahunAjaranID+"/"+row.Semester] {
			out = append(out, row)
		}
	}
	return out, nil
}
