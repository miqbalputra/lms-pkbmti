package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestReportManualAttendanceFlowAndPreservation(t *testing.T) {
	s, app, admin, k, students, _ := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Ganjil"
	p := ReportPeriod{}
	call := func(method, path, token string, body interface{}, want int) []byte {
		t.Helper()
		res, err := makeRequest(app, method, "/api/rapor-sumatif/"+path+qs, token, body, "")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.StatusCode, want, data)
		}
		if method != "GET" && want == 200 {
			if err = json.Unmarshal(data, &p); err != nil {
				t.Fatal(err)
			}
			if err = s.db.First(&p, "id = ?", p.ID).Error; err != nil {
				t.Fatal(err)
			}
		}
		return data
	}
	counts := func(id string, sick, izin, alpa int) fiber.Map {
		return fiber.Map{"studentId": id, "sakit": sick, "izin": izin, "alpa": alpa}
	}
	save := func(rows []fiber.Map, revision, want int) {
		t.Helper()
		call("PUT", "attendance", admin, fiber.Map{"revision": revision, "rows": rows}, want)
	}
	mode := func(value string, want int) {
		t.Helper()
		call("PUT", "attendance-mode", admin, fiber.Map{"revision": p.Revision, "mode": value}, want)
	}
	day := Presensi{KelasID: k.ID, Semester: "Ganjil", Tanggal: time.Date(2026, 8, 1, 9, 0, 0, 0, wibLocation), StatusPertemuan: "berlangsung"}
	s.db.Create(&day)
	daily := PresensiDetail{PresensiID: day.ID, PesertaDidikID: students[0].ID, StatusKehadiran: "Sakit"}
	s.db.Create(&daily)
	var originalDaily PresensiDetail
	s.db.First(&originalDaily, "id = ?", daily.ID)
	var grid struct {
		Mode string
		Rows []reportAttendanceRow
	}
	json.Unmarshal(call("GET", "attendance", admin, nil, 200), &grid)
	if grid.Mode != "automatic" || grid.Rows[0].Attendance.Sakit != 1 {
		t.Fatalf("default: %+v", grid)
	}
	save([]fiber.Map{counts(students[0].ID, 4, 2, 0)}, 0, 409)
	mode("manual", 200)
	var storedConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(p.Config), &storedConfig); err != nil || len(storedConfig) != 1 {
		t.Fatal("enabling manual recap must only add its setting", p.Config)
	}
	doc, _, err := s.buildReport(s.db, k, p, students[0].ID)
	if err != nil || !strings.Contains(strings.Join(doc.Issues, " "), "Lengkapi rekap presensi manual") {
		t.Fatal("missing recap must block finalization", err, doc.Issues)
	}
	call("POST", "document/"+students[0].ID+"/status", admin, fiber.Map{"revision": p.Revision, "status": "ready"}, 200)
	data := call("POST", "document/"+students[0].ID+"/status", admin, fiber.Map{"revision": p.Revision, "status": "final"}, 409)
	if !strings.Contains(string(data), "rekap presensi manual") {
		t.Fatal(string(data))
	}
	save([]fiber.Map{counts(students[0].ID, 4, 2, 0), counts(students[1].ID, 0, 0, 0)}, p.Revision, 200)
	doc, rec, err := s.buildReport(s.db, k, p, students[0].ID)
	if err != nil || doc.AttendanceSource != "manual" || doc.Attendance.Sakit != 4 || doc.Attendance.Izin != 2 || doc.Attendance.Alpa != 0 {
		t.Fatalf("manual recap: %+v %v", doc.Attendance, err)
	}
	other, _, _ := s.buildReport(s.db, k, p, students[1].ID)
	if other.Details.ManualAttendance == nil || strings.Contains(strings.Join(other.Issues, " "), "rekap presensi manual") {
		t.Fatal("explicit zeros must be complete")
	}
	// Editing notes from an old client must retain the manually entered recap.
	call("PUT", "document/"+students[0].ID, admin, fiber.Map{"revision": p.Revision, "details": ReportDetails{Note: "Catatan lama tetap aman", Identity: map[string]string{"address": "Alamat contoh"}, Extras: []ReportExtra{{Name: "Pramuka", Description: "Aktif"}}}}, 200)
	doc, rec, _ = s.buildReport(s.db, k, p, students[0].ID)
	if doc.Details.ManualAttendance == nil || doc.Attendance.Sakit != 4 {
		t.Fatal("note save erased recap")
	}
	storedDetails := rec.Details
	// Reject missing, fractional, negative, oversized, duplicate and out-of-class data.
	for _, rows := range [][]fiber.Map{
		{{"studentId": students[0].ID, "sakit": 0, "izin": 0}},
		{{"studentId": students[0].ID, "sakit": 0.5, "izin": 0, "alpa": 0}},
		{counts(students[0].ID, -1, 0, 0)}, {counts(students[0].ID, 367, 0, 0)}, {counts(students[0].ID, 200, 200, 0)},
		{counts(students[0].ID, 0, 0, 0), counts(students[0].ID, 1, 0, 0)}, {counts("other-class-student", 0, 0, 0)},
	} {
		save(rows, p.Revision, 400)
	}
	save([]fiber.Map{counts(students[0].ID, 9, 9, 9)}, p.Revision-1, 409)
	var unchanged ReportRecord
	s.db.First(&unchanged, "id = ?", rec.ID)
	if unchanged.Details != storedDetails {
		t.Fatal("invalid requests mutated existing details")
	}
	// A normal config save cannot turn off the admin-selected mode.
	cfg, _ := s.reportConfig(s.db, k, p)
	cfg.AttendanceMode = "automatic"
	call("PUT", "config", admin, fiber.Map{"revision": p.Revision, "config": cfg}, 200)
	json.Unmarshal(call("GET", "attendance", admin, nil, 200), &grid)
	if grid.Mode != "manual" {
		t.Fatal("general config changed source")
	}
	mode("automatic", 200)
	doc, _, _ = s.buildReport(s.db, k, p, students[0].ID)
	if doc.Attendance.Sakit != 1 || doc.Details.ManualAttendance.Sakit != 4 {
		t.Fatal("switching source removed data or added both totals")
	}
	mode("manual", 200)
	doc, rec, _ = s.buildReport(s.db, k, p, students[0].ID)
	if doc.Attendance.Sakit != 4 || doc.Details.Note != "Catatan lama tetap aman" || len(doc.Details.Extras) != 1 || doc.Details.Identity["address"] != "Alamat contoh" {
		t.Fatal("source toggle changed report details")
	}
	// Frozen report values and previous revisions remain byte-for-byte intact.
	rec.Status, rec.Snapshot, rec.Version = "published", reportJSON(doc), 1
	s.db.Save(&rec)
	revision := ReportRevision{RecordID: rec.ID, Version: 1, Snapshot: rec.Snapshot}
	s.db.Create(&revision)
	mode("automatic", 409)
	save([]fiber.Map{counts(students[0].ID, 8, 0, 0)}, p.Revision, 409)
	// The first writable row must roll back when a later row is final.
	save([]fiber.Map{counts(students[1].ID, 9, 0, 0), counts(students[0].ID, 8, 0, 0)}, p.Revision, 409)
	other, _, _ = s.buildReport(s.db, k, p, students[1].ID)
	if other.Attendance.Sakit != 0 {
		t.Fatal("batch partially saved")
	}
	json.Unmarshal(call("GET", "attendance", admin, nil, 200), &grid)
	if grid.Rows[0].Source != "frozen" || grid.Rows[0].Attendance.Sakit != 4 {
		t.Fatal("frozen report changed")
	}
	s.db.First(&unchanged, "id = ?", rec.ID)
	var oldRevision ReportRevision
	s.db.First(&oldRevision, "id = ?", revision.ID)
	s.db.First(&daily, "id = ?", daily.ID)
	if unchanged.Snapshot != rec.Snapshot || oldRevision.Snapshot != rec.Snapshot || daily != originalDaily {
		t.Fatal("historical data changed")
	}
	// A different semester retains its automatic default.
	res, e := makeRequest(app, "GET", "/api/rapor-sumatif/attendance?kelasId="+k.ID+"&semester=Genap", admin, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	var next struct{ Mode string }
	json.NewDecoder(res.Body).Decode(&next)
	if next.Mode != "automatic" {
		t.Fatal("setting leaked into another semester")
	}
}

func TestReportManualAttendancePermissionsAndArchives(t *testing.T) {
	s, app, admin, k, students, mp := reportFixture(t)
	qs := "?kelasId=" + k.ID + "&semester=Ganjil"
	for _, tc := range []struct {
		name, role     string
		wali, assigned bool
		read, write    int
	}{
		{"wali", "guru", true, false, 200, 200}, {"mapel", "guru", false, true, 403, 403},
		{"other", "guru", false, false, 403, 403}, {"head", "kepala_sekolah", false, false, 200, 403},
		{"parent", "orang_tua", false, false, 403, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tutor := Tutor{Nama: tc.name}
			s.db.Create(&tutor)
			if tc.wali {
				tutor.ID = *k.WaliKelasID
			}
			u := User{Username: "attendance-" + tc.name, Role: tc.role, TutorID: &tutor.ID, IsActive: true}
			s.db.Create(&u)
			if tc.assigned {
				s.db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: k.ID, MapelID: mp.ID})
			}
			token, _ := s.token(u, s.cfg.AccessSecret, time.Hour)
			for _, endpoint := range []struct {
				method, path string
				want         int
				body         interface{}
			}{
				{"GET", "attendance", tc.read, nil}, {"PUT", "attendance-mode", 403, fiber.Map{"mode": "manual"}},
				{"PUT", "attendance", tc.write, fiber.Map{"revision": 1, "rows": []fiber.Map{{"studentId": students[0].ID, "sakit": 0, "izin": 1, "alpa": 0}}}},
			} {
				if tc.wali && endpoint.path == "attendance" && endpoint.method == "PUT" {
					cfg, _ := s.reportConfig(s.db, k, ReportPeriod{})
					cfg.AttendanceMode = "manual"
					s.db.Create(&ReportPeriod{ReportScope: ReportScope{k.ID, k.TahunAjaranID, "Ganjil"}, Revision: 1, Config: reportJSON(cfg)})
				}
				res, err := makeRequest(app, endpoint.method, "/api/rapor-sumatif/"+endpoint.path+qs, token, endpoint.body, "")
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != endpoint.want {
					data, _ := io.ReadAll(res.Body)
					t.Fatalf("%s %s: %d want %d %s", endpoint.method, endpoint.path, res.StatusCode, endpoint.want, data)
				}
			}
		})
	}
	s.db.Model(&Semester{}).Where("tahun_ajaran_id = ? AND nama_semester = ?", k.TahunAjaranID, "Ganjil").Update("is_archived", true)
	res, err := makeRequest(app, "GET", "/api/rapor-sumatif/attendance"+qs, admin, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var archived struct {
		ReadOnly bool `json:"readOnly"`
	}
	json.NewDecoder(res.Body).Decode(&archived)
	res.Body.Close()
	if !archived.ReadOnly {
		t.Fatal("archived recap must be read-only in the UI")
	}
	for _, path := range []string{"attendance", "attendance-mode"} {
		res, err := makeRequest(app, "PUT", "/api/rapor-sumatif/"+path+qs, admin, fiber.Map{}, "")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 409 {
			t.Fatal("archived semester accepted write", path, res.StatusCode)
		}
	}
}

func TestReportManualAttendanceKeepsImportedRecap(t *testing.T) {
	s, app, admin, k, students, _ := reportFixture(t)
	cfg, _ := s.reportConfig(s.db, k, ReportPeriod{})
	cfg.AttendanceMode = "manual"
	p := ReportPeriod{ReportScope: ReportScope{k.ID, k.TahunAjaranID, "Ganjil"}, Revision: 1, Config: reportJSON(cfg)}
	s.db.Create(&p)
	details := ReportDetails{Note: "Impor lama", HistoricalAttendance: &ReportAttendance{Sakit: 7, Izin: 3, Alpa: 2}}
	rec := ReportRecord{PeriodID: p.ID, StudentID: students[0].ID, Status: "draft", Details: reportJSON(details)}
	s.db.Create(&rec)
	doc, _, err := s.buildReport(s.db, k, p, students[0].ID)
	if err != nil || doc.AttendanceSource != "historical" || doc.Attendance.Sakit != 7 || strings.Contains(strings.Join(doc.Issues, " "), "rekap presensi manual") {
		t.Fatal("historical recap not preserved")
	}
	res, err := makeRequest(app, "PUT", "/api/rapor-sumatif/attendance?kelasId="+k.ID+"&semester=Ganjil", admin, fiber.Map{"revision": 1, "rows": []fiber.Map{{"studentId": students[0].ID, "sakit": 0, "izin": 0, "alpa": 0}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatal("imported recap was editable")
	}
	var after ReportRecord
	s.db.First(&after, "id = ?", rec.ID)
	if after.Details != rec.Details {
		t.Fatal("imported data changed")
	}
}
