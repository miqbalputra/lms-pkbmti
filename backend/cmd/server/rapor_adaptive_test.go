package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReportAdaptiveRows(t *testing.T) {
	var templates []reportTemplate
	if err := json.Unmarshal(reportLayout, &templates); err != nil {
		t.Fatal(err)
	}
	base := templates[0]
	p := reportPDF()
	short := map[string]string{"N19": "Ananda mampu membaca dengan lancar."}
	if got := reportGrowDescriptions(p, base, short); !reflect.DeepEqual(got.Rows, base.Rows) {
		t.Fatal("standard dimensions changed")
	}
	long := strings.Repeat("Ananda mampu membaca serta memahami teks dan menjelaskan hasil belajar secara mandiri. ", 18)
	got := reportGrowDescriptions(p, base, map[string]string{"N19": long})
	for i, h := range base.Rows {
		if i == 18 {
			if got.Rows[i] <= h {
				t.Fatal("overflow did not grow")
			}
		} else if got.Rows[i] != h {
			t.Fatalf("unrelated row %d changed", i+1)
		}
	}
	if !reflect.DeepEqual(base.Rows, templates[0].Rows) {
		t.Fatal("shared template mutated")
	}
	for _, scenario := range []struct {
		name, a, b string
		extra      bool
	}{
		{"standard", "Mampu membaca.", "Berlatih menulis.", false},
		{"expanded", strings.Repeat("Ananda memahami bacaan dengan baik. ", 6), "Terus berlatih.", false},
		{"multipage", long + " AKHIRCAPAIAN", long + " AKHIRPENGUATAN", true},
		{"growth-only", "", long + " AKHIRPENGUATAN", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			d := ReportDocument{Student: PesertaDidik{Nama: "Siswa Contoh", NIS: "001", NISN: "1234567890"}, Class: Kelas{Jenjang: 1, TahunAjaran: TahunAjaran{NamaTahunAjaran: "2025/2026"}}, Scope: ReportScope{Semester: "Genap"}, Config: ReportConfig{Program: "PAKET A", Phase: "A"}}
			score := 85.0
			for i := 0; i < 10; i++ {
				d.Subjects = append(d.Subjects, ReportSubject{ReportSubjectConfig: ReportSubjectConfig{Name: "Bahasa Indonesia"}, Score: &score, Strength: "Mampu membaca.", Growth: "Terus belajar."})
			}
			d.Subjects[0].Strength = scenario.a
			d.Subjects[0].Growth = scenario.b
			pdf := reportPDF()
			n := reportRenderAdaptive(pdf, base, d, 1, 24, 1)
			if scenario.extra && n < 2 {
				t.Fatal("expected continuation pages")
			}
			if !scenario.extra && n != 1 {
				t.Fatalf("unexpected page count %d", n)
			}
			if scenario.extra {
				for _, section := range []struct{ from, to, index int }{{25, 35, 3}, {36, 70, 8}} {
					other := d
					other.Subjects = append([]ReportSubject(nil), d.Subjects...)
					other.Subjects[section.index].Strength = scenario.a
					other.Subjects[section.index].Growth = scenario.b
					extra := reportPDF()
					if reportRenderAdaptive(extra, base, other, section.from, section.to, 1) < 2 {
						t.Fatal("later section did not paginate")
					}
					var data bytes.Buffer
					if err := extra.Output(&data); err != nil {
						t.Fatal(err)
					}
				}
			}
			var out bytes.Buffer
			if err := pdf.Output(&out); err != nil {
				t.Fatal(err)
			}
			if directory := os.Getenv("RAPOR_QA_OUTPUT"); directory != "" {
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "adaptive-"+scenario.name+".pdf"), out.Bytes(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
