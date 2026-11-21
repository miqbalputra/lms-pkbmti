package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestReportExtraSettingsAndStudentDetails(t *testing.T) {
	s, app, token, k, students, _ := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Ganjil&tahunAjaranId=" + k.TahunAjaranID
	cfg, err := s.reportConfig(s.db, k, ReportPeriod{})
	if err != nil {
		t.Fatal(err)
	}
	on, off := true, false
	cfg.ExtrasEnabled = &on
	cfg.ExtraActivities = []string{"Pramuka", "Seni Tari"}
	p := ReportPeriod{}
	call := func(path string, body interface{}, want int) []byte {
		t.Helper()
		res, e := makeRequest(app, "PUT", path+qs, token, body, "")
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s: %d %s", path, res.StatusCode, data)
		}
		if want == 200 {
			if e = json.Unmarshal(data, &p); e != nil {
				t.Fatal(e)
			}
		}
		return data
	}
	call("/api/rapor-sumatif/config", fiber.Map{"revision": p.Revision, "config": cfg}, 200)
	details := ReportDetails{Extras: []ReportExtra{{Name: "Pramuka", Description: strings.Repeat("Aktif bekerja sama dalam kegiatan regu. ", 20)}}}
	path := "/api/rapor-sumatif/document/" + students[0].ID
	call(path, fiber.Map{"revision": p.Revision, "details": details}, 200)
	doc, _, e := s.buildReport(s.db, k, p, students[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(doc.Details.Extras) != 1 || doc.Details.Extras[0].Description != details.Extras[0].Description {
		t.Fatal("details not saved")
	}
	for _, issue := range doc.Issues {
		if strings.Contains(issue, "Ekstrakurikuler melebihi") {
			t.Fatal(issue)
		}
	}
	other, _, e := s.buildReport(s.db, k, p, students[1].ID)
	if e != nil || len(other.Details.Extras) != 0 {
		t.Fatal("another student's activities changed")
	}
	var stored ReportPeriod
	s.db.First(&stored, "id = ?", p.ID)
	loaded, e := s.reportConfig(s.db, k, stored)
	if e != nil || len(loaded.ExtraActivities) != 2 {
		t.Fatal("class activities not persisted")
	}
	legacy, e := s.reportConfig(s.db, k, ReportPeriod{})
	if e != nil || len(legacy.ExtraActivities) != 0 {
		t.Fatal("settings leaked into another semester")
	}
	cfg.ExtrasEnabled = &off
	call("/api/rapor-sumatif/config", fiber.Map{"revision": p.Revision, "config": cfg}, 200)
	details.Note = "Tetap semangat belajar."
	call(path, fiber.Map{"revision": p.Revision, "details": details}, 200)
	details.Extras[0].Description = "Diubah ketika nonaktif"
	call(path, fiber.Map{"revision": p.Revision, "details": details}, 400)
	call("/api/rapor-sumatif/config", fiber.Map{"revision": p.Revision, "config": ReportConfig{Subjects: cfg.Subjects, ExtraActivities: []string{"Pramuka", " pramuka "}}}, 400)
	if reportValidateExtras([]ReportExtra{{Name: "Pramuka"}, {Name: "pramuka"}}) == nil {
		t.Fatal("duplicate activities accepted")
	}
	if reportValidateExtras([]ReportExtra{{Name: "Pramuka", Description: strings.Repeat("x", 1001)}}) == nil {
		t.Fatal("unbounded description accepted")
	}
	if !reportExtrasEqual(nil, []ReportExtra{}) {
		t.Fatal("legacy empty lists must compare equally")
	}
}

func TestReportExtraPDF(t *testing.T) {
	var templates []reportTemplate
	if err := json.Unmarshal(reportLayout, &templates); err != nil {
		t.Fatal(err)
	}
	on, off := true, false
	d := ReportDocument{Student: PesertaDidik{Nama: "Siswa Contoh", NIS: "001", NISN: "1234567890"}, Class: Kelas{Jenjang: 1, TahunAjaran: TahunAjaran{NamaTahunAjaran: "2025/2026"}}, Config: ReportConfig{ExtrasEnabled: &on, Date: "2026-06-01", City: "Purbalingga", Teacher: "Tutor Contoh", TeacherNIP: "-"}, Details: ReportDetails{Note: "Pertahankan semangat belajar dan kebiasaan membaca setiap hari."}}
	d.Details.Extras = []ReportExtra{{"Pramuka", "Aktif mengikuti latihan serta mampu bekerja sama dalam regu."}, {"Seni Tari", strings.Repeat("Ananda mampu mengikuti gerakan dengan baik dan percaya diri saat tampil bersama kelompok. ", 7) + "TUNTAS_TARI"}, {"Futsal", "Menunjukkan sportivitas dan bekerja sama dengan anggota tim. TUNTAS_FUTSAL"}}
	// Realistic preview text; the stress fixtures below carry explicit markers.
	d.Details.Extras[1].Description = strings.ReplaceAll(d.Details.Extras[1].Description, "TUNTAS_TARI", "Perkembangan seni tari baik.")
	d.Details.Extras[2].Description = strings.ReplaceAll(d.Details.Extras[2].Description, "TUNTAS_FUTSAL", "Sportivitas berkembang baik.")
	for i := 0; i < 10; i++ {
		name := "Bahasa Indonesia"
		if i == 8 {
			name = "Pemberdayaan"
		}
		if i == 9 {
			name = "Keterampilan"
		}
		score := 85.0
		d.Subjects = append(d.Subjects, ReportSubject{ReportSubjectConfig: ReportSubjectConfig{Name: name}, Score: &score, Strength: "Ananda mampu memahami dan menerapkan materi dengan baik dalam kegiatan belajar sehari-hari.", Growth: "Terus berlatih secara mandiri dan pertahankan semangat belajar."})
	}
	for _, scenario := range []string{"ekstrakurikuler", "tanpa-ekstrakurikuler", "banyak-ekstrakurikuler", "baris-baru-ekstrakurikuler"} {
		t.Run(scenario, func(t *testing.T) {
			copy := d
			copy.Details.Extras = append([]ReportExtra(nil), d.Details.Extras...)
			if scenario == "tanpa-ekstrakurikuler" {
				copy.Config.ExtrasEnabled = &off
			}
			if scenario == "banyak-ekstrakurikuler" {
				copy.Details.Extras = nil
				for i := 1; i <= 10; i++ {
					copy.Details.Extras = append(copy.Details.Extras, ReportExtra{fmt.Sprintf("Kegiatan %d", i), strings.Repeat("Ananda aktif berlatih dan bekerja sama dengan baik dalam kelompok. ", 14) + fmt.Sprintf("TUNTAS_%d", i)})
				}
			}
			if scenario == "baris-baru-ekstrakurikuler" {
				copy.Details.Extras = []ReportExtra{{"Pramuka", strings.Repeat("Baik\n", 180) + "AKHIR_BARIS"}}
			}
			pdf := reportPDF()
			pages := reportRenderAdaptive(pdf, templates[0], copy, 36, 70, 1)
			if pages < 1 || scenario == "banyak-ekstrakurikuler" && pages < 2 {
				t.Fatal("pagination failed")
			}
			var data bytes.Buffer
			if err := pdf.Output(&data); err != nil {
				t.Fatal(err)
			}
			if directory := os.Getenv("RAPOR_QA_OUTPUT"); directory != "" {
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, scenario+".pdf"), data.Bytes(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
