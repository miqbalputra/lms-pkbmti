package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type reportCPBeforeJenjang struct {
	Base
	PeriodID string `gorm:"index"`
	MapelID  string `gorm:"index"`
	Label    string
	Urutan   int
}

func (reportCPBeforeJenjang) TableName() string { return "report_cps" }

func TestReportCPJenjangMigrationPreservesOldRows(t *testing.T) {
	db := isolatedTestDB(t, t.Name())
	if err := db.AutoMigrate(&reportCPBeforeJenjang{}, &ReportScore{}); err != nil {
		t.Fatal(err)
	}
	old := reportCPBeforeJenjang{PeriodID: "old-period", MapelID: "mapel", Label: "Teks Lama", Urutan: 1}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	value := 79.5
	score := ReportScore{CPID: old.ID, StudentID: "old-student", Value: &value}
	if err := db.Create(&score).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(reportModels()...); err != nil {
		t.Fatal(err)
	}
	var migrated ReportCP
	if err := db.First(&migrated, "id = ?", old.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.Label != old.Label || migrated.MapelID != old.MapelID || migrated.PeriodID != old.PeriodID || migrated.Urutan != old.Urutan || migrated.SharedCPID != "" {
		t.Fatalf("old CP changed: %+v", migrated)
	}
	var persisted ReportScore
	if err := db.First(&persisted, "id = ?", score.ID).Error; err != nil || persisted.CPID != old.ID || persisted.Value == nil || *persisted.Value != value {
		t.Fatalf("old score changed: %+v (%v)", persisted, err)
	}
}

func TestReportCPJenjangAssignmentAndPreservation(t *testing.T) {
	s, app, admin, k, students, mp := reportFixture(t)
	create := func(row interface{}) {
		t.Helper()
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	teacher := func(account string, tutorID *string) (User, string) {
		t.Helper()
		if tutorID == nil {
			tutor := Tutor{Nama: account}
			create(&tutor)
			tutorID = &tutor.ID
		}
		user := User{Username: account, Role: "guru", TutorID: tutorID, IsActive: true}
		create(&user)
		token, err := s.token(user, s.cfg.AccessSecret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return user, token
	}
	owner, ownerToken := teacher("guru-penyusun", k.WaliKelasID)
	other, otherToken := teacher("guru-mapel-lain", nil)
	create(&PenugasanGuruMapel{TutorID: *other.TutorID, KelasID: k.ID, MapelID: mp.ID})
	class := func(name string, grade int) Kelas {
		t.Helper()
		row := Kelas{Jenjang: grade, NamaRombel: name, PokjarID: k.PokjarID, TahunAjaranID: k.TahunAjaranID}
		create(&row)
		create(&KelasMapel{KelasID: row.ID, MapelID: mp.ID})
		return row
	}
	parallel := class("S", 1)
	legacy := class("T", 1)
	matching := class("U", 1)
	otherGrade := class("S", 2)
	sharedStudent := students[0].ID
	create(&RiwayatKelasPesertaDidik{PesertaDidikID: sharedStudent, KelasID: parallel.ID, TahunAjaranID: k.TahunAjaranID})
	seedLegacy := func(row Kelas, label string) (ReportCP, PesertaDidik) {
		t.Helper()
		student := PesertaDidik{Nama: "Siswa " + row.NamaRombel, NIS: "L-" + row.NamaRombel, NISN: "LN-" + row.NamaRombel, KelasID: row.ID}
		create(&student)
		period := ReportPeriod{ReportScope: ReportScope{row.ID, row.TahunAjaranID, "Ganjil"}, Revision: 7}
		create(&period)
		cp := ReportCP{PeriodID: period.ID, MapelID: mp.ID, Label: label, Urutan: 1}
		create(&cp)
		if err := s.db.Model(&cp).UpdateColumn("shared_cp_id", nil).Error; err != nil {
			t.Fatal(err)
		}
		zero := 0.0
		create(&ReportScore{CPID: cp.ID, StudentID: student.ID, Value: &zero})
		return cp, student
	}
	oldCP, oldStudent := seedLegacy(legacy, "Membilang sampai 20")
	matchingCP, matchingStudent := seedLegacy(matching, "Membaca teks")
	const frozen = "Capaian di rapor yang sudah terbit."
	create(&ReportRecord{PeriodID: matchingCP.PeriodID, StudentID: matchingStudent.ID, Status: "published", Version: 1, Snapshot: reportJSON(ReportDocument{
		Subjects: []ReportSubject{{ReportSubjectConfig: ReportSubjectConfig{MapelID: mp.ID}, Strength: frozen}},
	})})
	base := "/api/rapor-sumatif"
	year := "?tahunAjaranId=" + k.TahunAjaranID
	scope := func(row Kelas, semester string) string { return year + "&kelasId=" + row.ID + "&semester=" + semester }
	assign := base + "/cp-assignments/1" + year
	template := base + "/cp-templates/1" + year + "&semester=Ganjil&mapelId=" + mp.ID
	gridPath := func(row Kelas, semester string) string {
		return base + "/grid" + scope(row, semester) + "&mapelId=" + mp.ID
	}
	call := func(method, path, token string, body interface{}, want int, out interface{}) {
		t.Helper()
		res, err := makeRequest(app, method, path, token, body, "")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s %s => %d (want %d): %s (%v)", method, path, res.StatusCode, want, data, err)
		}
		if out != nil {
			if err := json.Unmarshal(data, out); err != nil {
				t.Fatal(err)
			}
		}
	}
	call("PUT", assign, ownerToken, fiber.Map{"userId": owner.ID}, 403, nil)
	call("PUT", assign, admin, fiber.Map{"userId": owner.ID}, 200, nil)
	var menu struct {
		Grades []struct {
			Jenjang  int
			Subjects []MataPelajaran
		}
	}
	call("GET", base+"/cp-assignments", ownerToken, nil, 200, &menu)
	if len(menu.Grades) != 1 || menu.Grades[0].Jenjang != 1 || len(menu.Grades[0].Subjects) != 1 || menu.Grades[0].Subjects[0].ID != mp.ID {
		t.Fatalf("unexpected assignment menu: %+v", menu)
	}
	defs := []reportCPDefinition{{Label: "Membaca teks"}}
	request := fiber.Map{"revision": 0, "cps": defs}
	call("PUT", template, admin, request, 403, nil)
	call("PUT", template, otherToken, request, 403, nil)
	call("PUT", gridPath(k, "Ganjil"), otherToken, reportGridInput{CPs: []ReportCP{{Label: "CP lokal"}}}, 409, nil)
	var saved struct {
		Revision int
		CPs      []reportCPDefinition
	}
	call("PUT", template, ownerToken, request, 200, &saved)
	type gridResponse struct {
		Period   ReportPeriod
		CPSource string
		CPs      []ReportCP
		Scores   []ReportScore
	}
	grid := func(row Kelas, semester string) gridResponse {
		t.Helper()
		var result gridResponse
		call("GET", gridPath(row, semester), admin, nil, 200, &result)
		return result
	}
	a, b := grid(k, "Ganjil"), grid(parallel, "Ganjil")
	if a.CPSource != "shared" || b.CPSource != "shared" || len(a.CPs) != 1 || len(b.CPs) != 1 || a.CPs[0].Label != "membaca teks" || a.CPs[0].ID != b.CPs[0].ID {
		t.Fatalf("parallel classes did not inherit CP: %+v / %+v", a, b)
	}
	if grid(k, "Genap").CPSource != "pending" || grid(otherGrade, "Ganjil").CPSource != "pending" {
		t.Fatal("CP leaked across semester or grade")
	}
	for _, sample := range []struct {
		row    Kelas
		result gridResponse
		token  string
		value  float64
	}{{k, a, otherToken, 81}, {parallel, b, admin, 94}} {
		call("PUT", gridPath(sample.row, "Ganjil"), sample.token, reportGridInput{Revision: sample.result.Period.Revision, CPs: sample.result.CPs, Scores: []ReportScore{{CPID: sample.result.CPs[0].ID, StudentID: sharedStudent, Value: &sample.value}}}, 200, nil)
	}
	a, b = grid(k, "Ganjil"), grid(parallel, "Ganjil")
	if len(a.Scores) != 1 || len(b.Scores) != 1 || a.Scores[0].CPID == b.Scores[0].CPID || *a.Scores[0].Value != 81 || *b.Scores[0].Value != 94 {
		t.Fatalf("scores crossed class boundaries: %+v / %+v", a, b)
	}
	call("PUT", assign, admin, fiber.Map{"userId": other.ID}, 200, nil)
	saved.CPs[0].Label = "membaca teks sederhana"
	request = fiber.Map{"revision": saved.Revision, "cps": saved.CPs}
	call("PUT", template, ownerToken, request, 403, nil)
	call("PUT", template, otherToken, request, 200, &saved)
	call("PUT", gridPath(k, "Ganjil"), otherToken, reportGridInput{Revision: a.Period.Revision, CPs: a.CPs}, 409, nil)
	call("PUT", template, otherToken, fiber.Map{"revision": saved.Revision, "cps": []reportCPDefinition{{Label: "CP pengganti"}}}, 409, nil)
	document := func(row Kelas, student string) ReportSubject {
		t.Helper()
		var result struct{ Document ReportDocument }
		call("GET", base+"/document/"+student+scope(row, "Ganjil"), admin, nil, 200, &result)
		return result.Document.Subjects[0]
	}
	if d := document(parallel, sharedStudent); d.Score == nil || *d.Score != 94 || !strings.Contains(d.Strength, "membaca teks sederhana") {
		t.Fatalf("shared CP not reflected in report: %+v", d)
	}
	if g := grid(matching, "Ganjil"); g.CPSource != "shared" || g.CPs[0].ID != matchingCP.ID || len(g.Scores) != 1 || *g.Scores[0].Value != 0 {
		t.Fatalf("matching historical CP was not adopted with its score: %+v", g)
	}
	if d := document(matching, matchingStudent.ID); d.Strength != frozen {
		t.Fatalf("published report changed: %+v", d)
	}
	if g := grid(legacy, "Ganjil"); g.CPSource != "legacy" || g.CPs[0].ID != oldCP.ID || g.CPs[0].Label != oldCP.Label || g.Period.Revision != 7 {
		t.Fatalf("local historical CP was overwritten: %+v", g)
	}
	if d := document(legacy, oldStudent.ID); d.Score == nil || *d.Score != 0 || !strings.Contains(d.Strength, "membilang sampai 20") {
		t.Fatalf("local historical report changed: %+v", d)
	}
}
