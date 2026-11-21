package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestReportSemesterSignatures(t *testing.T) {
	var templates []reportTemplate
	if err := json.Unmarshal(reportLayout, &templates); err != nil {
		t.Fatal(err)
	}
	stamp := image.NewRGBA(image.Rect(0, 0, 160, 60))
	for x := 10; x < 150; x++ {
		for y := 25; y < 30; y++ {
			stamp.Set(x, y, color.RGBA{0, 60, 160, 255})
		}
	}
	var signature bytes.Buffer
	if err := png.Encode(&signature, stamp); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, semester string
		overflow       bool
	}{{"semester-1", "Ganjil", false}, {"semester-2", "Genap", false}, {"semester-2-lanjutan", "Genap", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			d := ReportDocument{Student: PesertaDidik{Base: Base{ID: scenario.name}, Nama: "Siswa Contoh", NIS: "001", NISN: "1234567890"}, Class: Kelas{Jenjang: 1, TahunAjaran: TahunAjaran{NamaTahunAjaran: "2025/2026"}}, Scope: ReportScope{Semester: scenario.semester}, Config: ReportConfig{Program: "PAKET A", Phase: "A", Date: "2026-06-01", City: "Purbalingga", Teacher: "Wali Kelas Contoh, S.Pd.", TeacherNIP: "-", Principal: "Kepala Contoh, S.Kom.", PrincipalNIP: "-", PrincipalSignature: "data:image/png;base64," + base64.StdEncoding.EncodeToString(signature.Bytes())}, Details: ReportDetails{Note: "Pertahankan semangat belajar dan kebiasaan membaca setiap hari."}}
			if scenario.semester == "Genap" {
				d.Details.Promotion = "naik"
				d.Details.NextClass = "II (Dua)"
			}
			if scenario.name == "semester-2" {
				d.Config.PrincipalSignature = ""
			}
			for i, name := range []string{"Pendidikan Agama dan Budi Pekerti", "Pendidikan Pancasila", "Bahasa Indonesia", "Matematika", "Pendidikan Jasmani, Olahraga dan Kesehatan", "Seni dan Budaya", "Muatan Lokal (Bahasa Arab)", "Muatan Lokal (Baca Tulis Al-Qur'an)", "Pemberdayaan", "Keterampilan"} {
				score := 85.0
				group := "umum"
				if i >= 8 {
					group = "pemberdayaan"
				}
				d.Subjects = append(d.Subjects, ReportSubject{ReportSubjectConfig: ReportSubjectConfig{Name: name, Group: group}, Number: i + 1, Score: &score, Strength: "Ananda mampu memahami dan menerapkan materi dengan baik dalam kegiatan belajar sehari-hari.", Growth: "Terus berlatih secara mandiri dan pertahankan semangat belajar."})
			}
			d.Details.Extras = []ReportExtra{{"Pramuka", "Aktif mengikuti latihan serta mampu bekerja sama dalam regu."}, {"Seni Tari", "Ananda mampu mengikuti gerakan dengan baik dan percaya diri saat tampil bersama kelompok."}}
			if scenario.overflow {
				d.Details.Extras = nil
				for i := 1; i <= 10; i++ {
					d.Details.Extras = append(d.Details.Extras, ReportExtra{fmt.Sprintf("Kegiatan %d", i), strings.Repeat("Aktif mengikuti latihan dan bekerja sama dengan kelompok. ", 12) + fmt.Sprintf("TUNTAS_%d", i)})
				}
			}
			want := 2
			if scenario.semester == "Genap" {
				want = 3
			}
			if reportSignatureCount(d) != want {
				t.Fatal("wrong signatories")
			}
			p := reportPDF()
			prepared := reportSignatureTemplate(p, reportStyledTemplate(templates[0]), d)
			if prepared.signatureRow != 61 || prepared.Rows[60]*25.4/72 < 45 || prepared.Rows[60]*25.4/72 > 150 {
				t.Fatal("invalid signature reservation")
			}
			f := reportPageFragment(prepared, []int{56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70}, reportValues(d, prepared.Name), prepared.Rows, true)
			if f.signatureRow != 6 {
				t.Fatal("signature position lost on continuation")
			}
			app := fiber.New()
			app.Get("/pdf", func(c *fiber.Ctx) error { return reportSendPDF(c, []ReportDocument{d}, "rapor") })
			res, err := app.Test(httptest.NewRequest("GET", "/pdf", nil), -1)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			data, _ := io.ReadAll(res.Body)
			if res.StatusCode != 200 || !bytes.HasPrefix(data, []byte("%PDF-")) {
				t.Fatal("report rendering failed")
			}
			if directory := os.Getenv("RAPOR_QA_OUTPUT"); directory != "" {
				if err = os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, scenario.name+".pdf"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
