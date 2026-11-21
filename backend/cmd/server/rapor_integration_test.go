package main

import (
	"testing"
	"time"
)

func TestReportParentGradesOnlyPublished(t *testing.T) {
	s, _, _, k, students, mp := reportFixture(t)
	p := ReportPeriod{ReportScope: ReportScope{KelasID: k.ID, TahunAjaranID: k.TahunAjaranID, Semester: "Ganjil"}, Revision: 1}
	s.db.Create(&p)
	v := 85.0
	d := ReportDocument{Student: students[0], Scope: p.ReportScope, Subjects: []ReportSubject{{ReportSubjectConfig: ReportSubjectConfig{MapelID: mp.ID, Name: mp.NamaMapel}, Score: &v}}}
	rec := ReportRecord{PeriodID: p.ID, StudentID: students[0].ID, Status: "draft", Snapshot: reportJSON(d)}
	s.db.Create(&rec)
	old := parentGradeItem{ID: "old", PesertaDidikID: students[0].ID, TahunAjaranID: k.TahunAjaranID, Semester: "Ganjil", MapelID: mp.ID, NPAkhir: &v}
	out, err := reportParentGrades(s.db, students[0].ID, []parentGradeItem{old})
	if err != nil || len(out) != 0 {
		t.Fatalf("draft visible %+v %v", out, err)
	}
	s.db.Model(&rec).Update("status", "published")
	out, err = reportParentGrades(s.db, students[0].ID, []parentGradeItem{old})
	if err != nil || len(out) != 1 || out[0].NAAkhir == nil || *out[0].NAAkhir != 85 || out[0].NPAkhir != nil {
		t.Fatalf("published projection %+v %v", out, err)
	}
}

func TestReportAttendanceUniqueDaysAndConflict(t *testing.T) {
	s, _, _, k, students, _ := reportFixture(t)
	p := ReportPeriod{ReportScope: ReportScope{KelasID: k.ID, TahunAjaranID: k.TahunAjaranID, Semester: "Ganjil"}}
	day := time.Date(2026, 7, 25, 9, 0, 0, 0, wibLocation)
	for _, status := range []string{"Sakit", "Sakit", "Izin"} {
		meeting := Presensi{KelasID: k.ID, Tanggal: day, Semester: "Ganjil", StatusPertemuan: "berlangsung"}
		s.db.Create(&meeting)
		s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: students[0].ID, StatusKehadiran: status})
	}
	got, err := s.reportAttendance(s.db, k, p, students[0].ID)
	if err != nil || got.Sakit+got.Izin != 1 || len(got.Conflicts) == 0 {
		t.Fatalf("attendance %+v %v", got, err)
	}
}
