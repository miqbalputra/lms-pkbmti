package main

import (
	"encoding/json"
	"io"
	"testing"
)

func TestReportNarrativePreservesHistoricalData(t *testing.T) {
	for _, status := range []string{"final", "published"} {
		t.Run(status, func(t *testing.T) {
			s, app, token, k, students, mp := reportFixture(t)
			period := ReportPeriod{ReportScope: ReportScope{KelasID: k.ID, TahunAjaranID: k.TahunAjaranID, Semester: "Ganjil"}, Revision: 7}
			if err := s.db.Create(&period).Error; err != nil {
				t.Fatal(err)
			}
			cps := []ReportCP{{PeriodID: period.ID, MapelID: mp.ID, Label: "  Membaca huruf, suku kata, dan kata  ", Urutan: 1}, {PeriodID: period.ID, MapelID: mp.ID, Label: "Menulis kalimat tentang Indonesia", Urutan: 2}}
			for i := range cps {
				if err := s.db.Create(&cps[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, student := range students {
				for i, value := range []float64{90, 60} {
					v := value
					if err := s.db.Create(&ReportScore{CPID: cps[i].ID, StudentID: student.ID, Value: &v}).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			manual := ReportNarrative{PeriodID: period.ID, StudentID: students[0].ID, MapelID: mp.ID, Strength: "Deskripsi manual lama: Membaca Indonesia, tanpa perubahan.", Growth: "Catatan tutor tetap utuh.", ReviewedRevision: 7}
			if err := s.db.Create(&manual).Error; err != nil {
				t.Fatal(err)
			}
			oldScore := 75.0
			snapshot := ReportDocument{Student: students[1], Class: k, Scope: period.ReportScope, Subjects: []ReportSubject{{ReportSubjectConfig: ReportSubjectConfig{MapelID: mp.ID, Name: mp.NamaMapel}, Score: &oldScore, Strength: "Ananda menunjukkan penguasaan yang sangat baik dalam hal Membaca dan Menulis.", Growth: "Perlu bimbingan dan pendampingan lebih lanjut dalam hal berhitung."}}}
			var frozenManual ReportNarrative
			if status == "final" {
				frozenManual = ReportNarrative{PeriodID: period.ID, StudentID: students[1].ID, MapelID: mp.ID, Strength: snapshot.Subjects[0].Strength, Growth: snapshot.Subjects[0].Growth, ReviewedRevision: period.Revision}
				if err := s.db.Create(&frozenManual).Error; err != nil {
					t.Fatal(err)
				}
			}
			record := ReportRecord{PeriodID: period.ID, StudentID: students[1].ID, Status: status, Version: 1, Snapshot: reportJSON(snapshot), Details: reportJSON(ReportDetails{Note: "Catatan historis"})}
			if err := s.db.Create(&record).Error; err != nil {
				t.Fatal(err)
			}
			if err := s.db.Create(&ReportRevision{RecordID: record.ID, Version: 1, Snapshot: record.Snapshot}).Error; err != nil {
				t.Fatal(err)
			}
			stored := func() string {
				t.Helper()
				var periods []ReportPeriod
				var labels []ReportCP
				var scores []ReportScore
				var narratives []ReportNarrative
				var records []ReportRecord
				var revisions []ReportRevision
				for _, dest := range []interface{}{&periods, &labels, &scores, &narratives, &records, &revisions} {
					if err := s.db.Order("id").Find(dest).Error; err != nil {
						t.Fatal(err)
					}
				}
				return reportJSON([]interface{}{periods, labels, scores, narratives, records, revisions})
			}
			before := stored()
			qs := "?kelasId=" + k.ID + "&semester=Ganjil&mapelId=" + mp.ID
			read := func(path string, dest interface{}) {
				t.Helper()
				res, err := makeRequest(app, "GET", path, token, nil, "")
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 200 {
					b, _ := io.ReadAll(res.Body)
					t.Fatalf("GET %s: %d %s", path, res.StatusCode, b)
				}
				if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
					t.Fatal(err)
				}
			}
			for _, student := range students {
				var response struct{ Document ReportDocument }
				read("/api/rapor-sumatif/document/"+student.ID+qs, &response)
				wantA, wantB := manual.Strength, manual.Growth
				if student.ID == students[1].ID {
					wantA, wantB = snapshot.Subjects[0].Strength, snapshot.Subjects[0].Growth
				}
				if response.Document.Subjects[0].Strength != wantA || response.Document.Subjects[0].Growth != wantB {
					t.Fatalf("old %s description changed", student.ID)
				}
			}
			var grid struct{ Narratives []ReportNarrative }
			read("/api/rapor-sumatif/grid"+qs, &grid)
			if len(grid.Narratives) != 2 {
				t.Fatalf("grid must retain manual and frozen descriptions: %+v", grid.Narratives)
			}
			for _, narrative := range grid.Narratives {
				if narrative.StudentID == students[1].ID && frozenManual.ID != "" && narrative.ID != frozenManual.ID {
					t.Fatal("grid discarded the historical narrative identifier")
				}
				if narrative.StudentID == students[1].ID && (narrative.Strength != snapshot.Subjects[0].Strength || narrative.Growth != snapshot.Subjects[0].Growth) {
					t.Fatal("grid regenerated a frozen description")
				}
			}
			if after := stored(); after != before {
				t.Fatal("reading with new rules rewrote historical CPs, scores, descriptions, records, or revision snapshots")
			}
			// A score-only save must preserve historical uppercase CP labels and
			// the saved manual text; the locked student's score remains untouched.
			newValue := 91.0
			res, err := makeRequest(app, "PUT", "/api/rapor-sumatif/grid"+qs, token, reportGridInput{Revision: period.Revision, CPs: cps, Scores: []ReportScore{{CPID: cps[0].ID, StudentID: students[0].ID, Value: &newValue}}}, "")
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("score-only save: %d %s", res.StatusCode, b)
			}
			var savedCP ReportCP
			var savedNarrative ReportNarrative
			var savedRecord ReportRecord
			var savedRevision ReportRevision
			for _, row := range []struct {
				dest interface{}
				id   string
			}{{&savedCP, cps[0].ID}, {&savedNarrative, manual.ID}, {&savedRecord, record.ID}} {
				if err := s.db.First(row.dest, "id = ?", row.id).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := s.db.First(&savedRevision, "record_id = ?", record.ID).Error; err != nil {
				t.Fatal(err)
			}
			if savedCP.Label != cps[0].Label || savedNarrative.Strength != manual.Strength || savedNarrative.Growth != manual.Growth || savedRecord.Snapshot != record.Snapshot || savedRevision.Snapshot != record.Snapshot {
				t.Fatal("score save rewrote old labels, custom text, or frozen history")
			}
		})
	}
}
