package main

import (
	"encoding/json"
	"io"
	"testing"
)

func TestReportPeriodExposesLocksWithoutChangingData(t *testing.T) {
	s, app, token, k, students, _ := reportFixture(t)
	read := func() struct {
		SemesterArchived bool
		ConfigLocked     bool
		Records          []ReportRecord
	} { t.Helper(); res, err := makeRequest(app, "GET", "/api/rapor-sumatif/period?kelasId="+k.ID+"&semester=Ganjil", token, nil, ""); if err != nil {
		t.Fatal(err)
	}; defer res.Body.Close(); data, _ := io.ReadAll(res.Body); if res.StatusCode != 200 {
		t.Fatalf("period: %d %s", res.StatusCode, data)
	}; var state struct {
		SemesterArchived bool
		ConfigLocked     bool
		Records          []ReportRecord
	}; if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}; return state }
	if state := read(); state.SemesterArchived || state.ConfigLocked {
		t.Fatal("new period unexpectedly locked")
	}
	var periods int64
	s.db.Model(&ReportPeriod{}).Count(&periods)
	if periods != 0 {
		t.Fatal("reading capabilities created a period")
	}
	p := ReportPeriod{ReportScope: ReportScope{k.ID, k.TahunAjaranID, "Ganjil"}, Revision: 9, Config: `{"city":"Kota lama"}`}
	s.db.Create(&p)
	rec := ReportRecord{PeriodID: p.ID, StudentID: students[0].ID, Status: "published", Version: 2, Details: `{"note":"Catatan lama"}`, Snapshot: `{"details":{"note":"Versi final lama"}}`}
	s.db.Create(&rec)
	s.db.Model(&Semester{}).Where("tahun_ajaran_id = ? AND nama_semester = ?", k.TahunAjaranID, "Ganjil").Update("is_archived", true)
	state := read()
	if !state.SemesterArchived || !state.ConfigLocked {
		t.Fatal("archive/final lock missing from response")
	}
	var after ReportRecord
	s.db.First(&after, "id = ?", rec.ID)
	var afterPeriod ReportPeriod
	s.db.First(&afterPeriod, "id = ?", p.ID)
	if after.Details != rec.Details || after.Snapshot != rec.Snapshot || after.Version != rec.Version || afterPeriod.Config != p.Config || afterPeriod.Revision != p.Revision {
		t.Fatal("reading locks rewrote historical data")
	}
}
