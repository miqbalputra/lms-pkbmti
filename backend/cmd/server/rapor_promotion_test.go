package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestReportPromotionBrowserServer(t *testing.T) {
	if os.Getenv("REPORT_PROMOTION_BROWSER_SERVER") != "1" {
		t.Skip("opt-in isolated promotion browser fixture")
	}
	s, app, _, k, _, mp := reportFixture(t)
	for _, grade := range []int{6, 9, 12} {
		class := Kelas{Jenjang: grade, NamaRombel: "DEMO", PokjarID: k.PokjarID, TahunAjaranID: k.TahunAjaranID, WaliKelasID: k.WaliKelasID}
		if err := s.db.Create(&class).Error; err != nil {
			t.Fatal(err)
		}
		if err := s.db.Create(&KelasMapel{KelasID: class.ID, MapelID: mp.ID}).Error; err != nil {
			t.Fatal(err)
		}
		if err := s.db.Create(&PesertaDidik{Nama: fmt.Sprintf("Siswa Contoh Kelas %d", grade), NIS: fmt.Sprintf("DEMO-%d", grade), NISN: fmt.Sprintf("%010d", grade), KelasID: class.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	app.Get("/__report-audit", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"isolated": true, "storage": "memory"}) })
	app.Static("/", "../../../frontend/dist")
	app.Get("/*", func(c *fiber.Ctx) error { return c.SendFile("../../../frontend/dist/index.html") })
	if err := app.Listen("127.0.0.1:19090"); err != nil {
		t.Fatal(err)
	}
}

func TestReportPromotionAutomaticTargets(t *testing.T) {
	for grade := 1; grade <= 12; grade++ {
		t.Run(fmt.Sprint(grade), func(t *testing.T) {
			rules := reportPromotionRules(grade)
			completion := grade == 6 || grade == 9 || grade == 12
			if !rules.ValidGrade || rules.Completion != completion {
				t.Fatal(rules)
			}
			decisions := []string{"naik", "tinggal"}
			if completion {
				decisions = []string{"lulus", "belum_lulus"}
				if rules.NextClass != "" {
					t.Fatal("completion must not promote into another program or grade 13")
				}
			} else if rules.NextClass != reportClassName(grade+1) {
				t.Fatal("wrong automatic target", rules)
			}
			for _, decision := range decisions {
				next := ReportDetails{Promotion: decision, NextClass: "TUJUAN PALSU"}
				if err := reportPreparePromotion(grade, "Genap", ReportDetails{}, &next, true); err != nil {
					t.Fatal(err)
				}
				want := ""
				if decision == "naik" {
					want = reportClassName(grade + 1)
				}
				if next.NextClass != want {
					t.Fatal("client controlled target", next.NextClass)
				}
				text := reportValues(ReportDocument{Student: PesertaDidik{Nama: "Siswa Dummy"}, Class: Kelas{Jenjang: grade}, Scope: ReportScope{Semester: "Genap"}, Details: next}, "CETAK RAPOR")["A52"]
				phrase := map[string]string{"naik": "NAIK KELAS ke " + want, "tinggal": "TINGGAL DI KELAS " + reportClassName(grade), "lulus": "dinyatakan LULUS", "belum_lulus": "dinyatakan BELUM LULUS"}[decision]
				if !strings.Contains(text, phrase) || !strings.Contains(text, "Siswa Dummy") {
					t.Fatal(text)
				}
			}
			wrong := "lulus"
			if completion {
				wrong = "naik"
			}
			next := ReportDetails{Promotion: wrong}
			if reportPreparePromotion(grade, "Genap", ReportDetails{}, &next, true) == nil {
				t.Fatal("wrong decision for grade accepted")
			}
		})
	}
	for _, grade := range []int{0, 13} {
		if reportPromotionRules(grade).ValidGrade {
			t.Fatal("invalid grade accepted")
		}
	}
}

func TestReportPaketAGraduationNote(t *testing.T) {
	d := ReportDocument{Student: PesertaDidik{Nama: "Liyana Nafisa Amri"}, Class: Kelas{Jenjang: 6}, Scope: ReportScope{Semester: "Genap"}, Config: ReportConfig{Phase: "C"}, Details: ReportDetails{Promotion: "lulus"}}
	want := "Berdasarkan pertimbangan kelulusan peserta didik dan Laporan Hasil belajar (Rapor) Fase A, B, C, serta hasil Ujian Pendidikan Kesetaraan (UPK) PKBM Tunas Ilmu, maka peserta didik Liyana Nafisa Amri dinyatakan LULUS dari Program Kesetaraan Paket A PKBM Tunas Ilmu."
	if got := reportValues(d, "CETAK RAPOR")["A52"]; got != want {
		t.Fatalf("graduation wording: %q", got)
	}
	d.Student.Nama = "Siswa Contoh Lain"
	if got := reportValues(d, "CETAK RAPOR")["A52"]; got != strings.Replace(want, "Liyana Nafisa Amri", d.Student.Nama, 1) {
		t.Fatal("student name is not dynamic", got)
	}
	d.Details.Promotion = "belum_lulus"
	if got := reportValues(d, "CETAK RAPOR")["A52"]; !strings.Contains(got, "dinyatakan BELUM LULUS dari Program Kesetaraan Paket A") {
		t.Fatal("decision must match the tutor's choice", got)
	}
	d.Scope.Semester = "Ganjil"
	if got := reportValues(d, "CETAK RAPOR")["A52"]; got != "" {
		t.Fatal("semester one must not show the decision")
	}
	d.Scope.Semester = "Genap"
	for _, grade := range []int{5, 9, 12} {
		d.Class.Jenjang = grade
		if got := reportValues(d, "CETAK RAPOR")["A52"]; strings.Contains(got, "Program Kesetaraan Paket A") {
			t.Fatal("Paket A wording leaked to another grade", grade, got)
		}
	}
}

func TestReportPromotionAPIAndHistoricalPreservation(t *testing.T) {
	s, app, token, k, students, _ := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Genap"
	call := func(method, path string, body interface{}, want int) []byte {
		t.Helper()
		res, err := makeRequest(app, method, "/api/rapor-sumatif/"+path+qs, token, body, "")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d %s", method, path, res.StatusCode, want, data)
		}
		return data
	}
	var period ReportPeriod
	// New clients need only the decision. A forged class target is ignored too.
	json.Unmarshal(call("PUT", "document/"+students[0].ID, fiber.Map{"revision": 0, "details": ReportDetails{Promotion: "naik", NextClass: "XIII (Tiga Belas)", Note: "Catatan dummy"}}, 200), &period)
	var rec ReportRecord
	s.db.Where("period_id = ? AND student_id = ?", period.ID, students[0].ID).First(&rec)
	var details ReportDetails
	reportDecode(rec.Details, &details)
	if details.NextClass != "II (Dua)" || details.Note != "Catatan dummy" {
		t.Fatal(details)
	}
	var student PesertaDidik
	s.db.First(&student, "id = ?", students[0].ID)
	if student.KelasID != k.ID {
		t.Fatal("report decision moved the student in master data")
	}
	// A historical custom target survives reads and unrelated note saves.
	details.NextClass = "Kelas II (format lama)"
	details.Extras = []ReportExtra{{Name: "Pramuka", Description: "Aktif"}}
	details.ManualAttendance = &ReportAttendanceCounts{Sakit: 3, Izin: 1, Alpa: 0}
	details.Identity = map[string]string{"father": "Ayah Contoh"}
	rec.Details = reportJSON(details)
	s.db.Save(&rec)
	before := rec.Details
	call("GET", "document/"+students[0].ID, nil, 200)
	var after ReportRecord
	s.db.First(&after, "id = ?", rec.ID)
	if after.Details != before {
		t.Fatal("reading rewrote historical data")
	}
	details.Note = "Catatan baru, keputusan lama tetap"
	json.Unmarshal(call("PUT", "document/"+students[0].ID, fiber.Map{"revision": period.Revision, "details": details}, 200), &period)
	s.db.First(&after, "id = ?", rec.ID)
	reportDecode(after.Details, &details)
	if details.NextClass != "Kelas II (format lama)" || details.ManualAttendance.Sakit != 3 || len(details.Extras) != 1 || details.Identity["father"] != "Ayah Contoh" {
		t.Fatal("note save lost historical data")
	}
	json.Unmarshal(call("PUT", "document/"+students[0].ID, fiber.Map{"revision": period.Revision, "updatePromotion": true, "details": details}, 200), &period)
	s.db.First(&after, "id = ?", rec.ID)
	reportDecode(after.Details, &details)
	if details.NextClass != "II (Dua)" {
		t.Fatal("explicit automatic target failed")
	}
	// Frozen reports retain even their older custom wording.
	s.db.First(&period, "id = ?", period.ID)
	doc, rec, err := s.buildReport(s.db, k, period, students[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	doc.Details.NextClass = "Kelas II (versi final lama)"
	rec.Status, rec.Snapshot, rec.Version = "final", reportJSON(doc), 1
	s.db.Save(&rec)
	response := call("GET", "document/"+students[0].ID, nil, 200)
	if !strings.Contains(string(response), "Kelas II (versi final lama)") {
		t.Fatal("final snapshot changed")
	}
	call("PUT", "document/"+students[0].ID, fiber.Map{"revision": period.Revision, "updatePromotion": true, "details": details}, 409)
	s.db.First(&after, "id = ?", rec.ID)
	if after.Snapshot != rec.Snapshot {
		t.Fatal("final snapshot was rewritten")
	}
}

func TestReportCompletionDecisionValidation(t *testing.T) {
	for _, grade := range []int{6, 9, 12} {
		t.Run(fmt.Sprint(grade), func(t *testing.T) {
			s, app, token, k, students, _ := reportFixture(t)
			k.Jenjang = grade
			s.db.Save(&k)
			p := ReportPeriod{ReportScope: ReportScope{k.ID, k.TahunAjaranID, "Genap"}, Revision: 1}
			s.db.Create(&p)
			old := ReportDetails{Promotion: "naik", NextClass: "Tujuan lama yang diimpor"}
			rec := ReportRecord{PeriodID: p.ID, StudentID: students[0].ID, Status: "draft", Details: reportJSON(old)}
			s.db.Create(&rec)
			doc, _, err := s.buildReport(s.db, k, p, students[0].ID)
			if err != nil || !strings.Contains(strings.Join(doc.Issues, " "), "keputusan kelulusan") {
				t.Fatal("terminal grade needs a human completion decision", err, doc.Issues)
			}
			for _, decision := range []string{"naik", "lulus", "belum_lulus"} {
				want := 200
				if decision == "naik" {
					want = 400
				}
				res, err := makeRequest(app, "PUT", "/api/rapor-sumatif/document/"+students[0].ID+"?kelasId="+k.ID+"&semester=Genap", token, fiber.Map{"revision": p.Revision, "updatePromotion": true, "details": ReportDetails{Promotion: decision, NextClass: "XIII"}}, "")
				if err != nil {
					t.Fatal(err)
				}
				data, _ := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode != want {
					t.Fatalf("%s: %d %s", decision, res.StatusCode, data)
				}
				if want == 200 {
					json.Unmarshal(data, &p)
					s.db.First(&p, "id = ?", p.ID)
					doc, _, err = s.buildReport(s.db, k, p, students[0].ID)
					if err != nil || doc.Details.NextClass != "" || doc.Details.Promotion != decision || strings.Contains(strings.Join(doc.Issues, " "), "keputusan kelulusan") {
						t.Fatal("completion decision not applied")
					}
				}
			}
		})
	}
	previous := ReportDetails{Promotion: "naik", NextClass: "Format lama"}
	next := ReportDetails{Promotion: "lulus", NextClass: "Palsu"}
	if err := reportPreparePromotion(1, "Ganjil", previous, &next, true); err != nil || next.Promotion != previous.Promotion || next.NextClass != previous.NextClass {
		t.Fatal("semester one changed hidden decisions")
	}
	if reportValues(ReportDocument{Scope: ReportScope{Semester: "Ganjil"}, Details: next}, "CETAK RAPOR")["A52"] != "" {
		t.Fatal("semester one printed promotion")
	}
}
