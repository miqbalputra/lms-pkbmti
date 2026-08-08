package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func automationReminderRequest(t *testing.T, app *fiber.App, key string) *presensiReminderTestResponse {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/automation/presensi-reminders?through=2026-08-08", nil)
	if key != "" {
		req.Header.Set("X-Automation-Key", key)
	}
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("automation reminder request failed: %v", err)
	}
	defer res.Body.Close()
	result := &presensiReminderTestResponse{StatusCode: res.StatusCode}
	if res.StatusCode == fiber.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(result); err != nil {
			t.Fatalf("decode automation reminder response: %v", err)
		}
	}
	return result
}

type presensiReminderTestResponse struct {
	StatusCode int                     `json:"-"`
	Status     string                  `json:"status"`
	Through    string                  `json:"through"`
	Reminders  []presensiTutorReminder `json:"reminders"`
}

func TestPresensiAutomationReminderUsesStaticKeyAndCompactCompleteness(t *testing.T) {
	s, _ := setupE2EServer(t)
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/api/automation/presensi-reminders", s.presensiAutomationAuth, s.presensiAutomationReminders)
	t.Setenv("N8N_PRESENSI_API_KEY", "")
	if result := automationReminderRequest(t, app, ""); result.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected missing server automation key to return 503, got %d", result.StatusCode)
	}
	const automationKey = "test-presensi-automation-key-32-chars"
	t.Setenv("N8N_PRESENSI_API_KEY", automationKey)
	if result := automationReminderRequest(t, app, ""); result.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected missing automation key to return 401, got %d", result.StatusCode)
	}
	if result := automationReminderRequest(t, app, "wrong-key"); result.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected wrong automation key to return 401, got %d", result.StatusCode)
	}

	var year TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		t.Fatal(err)
	}
	year.TanggalMulai = time.Date(2026, 8, 8, 0, 0, 0, 0, wibLocation)
	year.TanggalSelesai = time.Date(2027, 6, 30, 0, 0, 0, 0, wibLocation)
	if err := s.db.Save(&year).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&Semester{}).
		Where("tahun_ajaran_id = ? AND nama_semester = ?", year.ID, "Ganjil").
		Updates(map[string]interface{}{
			"tanggal_mulai":   time.Date(2026, 8, 8, 0, 0, 0, 0, wibLocation),
			"tanggal_selesai": time.Date(2026, 12, 31, 23, 59, 59, 0, wibLocation),
			"is_archived":     false,
		}).Error; err != nil {
		t.Fatal(err)
	}

	var pokjar Pokjar
	s.db.First(&pokjar)
	tutor := Tutor{Nama: "Tutor Pengingat", JenisKelamin: "P", NoHP: "081234567890"}
	s.db.Create(&tutor)
	class := Kelas{
		Jenjang: 3, NamaRombel: "A", PokjarID: pokjar.ID,
		TahunAjaranID: year.ID, WaliKelasID: &tutor.ID,
	}
	s.db.Create(&class)
	students := []PesertaDidik{
		{Nama: "Siswa A", NIS: "REM-1", NISN: "REM-1", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"},
		{Nama: "Siswa B", NIS: "REM-2", NISN: "REM-2", KelasID: class.ID, PokjarID: pokjar.ID, Status: "aktif"},
		{Nama: "Siswa Nonaktif", NIS: "REM-3", NISN: "REM-3", KelasID: class.ID, PokjarID: pokjar.ID, Status: "nonaktif"},
	}
	for i := range students {
		s.db.Create(&students[i])
	}

	missing := automationReminderRequest(t, app, automationKey)
	if missing.StatusCode != fiber.StatusOK || missing.Status != "ok" || missing.Through != "2026-08-08" {
		t.Fatalf("unexpected reminder response: %+v", missing)
	}
	if len(missing.Reminders) != 1 || missing.Reminders[0].MissingCount != 1 {
		t.Fatalf("expected one missing Saturday reminder, got %+v", missing.Reminders)
	}
	if missing.Reminders[0].Phone != tutor.NoHP || missing.Reminders[0].MissingDates[0].Reason != "belum ada data presensi" {
		t.Fatalf("unexpected reminder content: %+v", missing.Reminders[0])
	}

	meeting := Presensi{
		KelasID: class.ID, Tanggal: time.Date(2026, 8, 8, 0, 0, 0, 0, wibLocation),
		StatusPertemuan: "berlangsung", TandaTangan: "data:image/png;base64,test", BuktiFoto: "[\"stored\"]",
	}
	s.db.Create(&meeting)
	for i := 0; i < 2; i++ {
		s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: students[i].ID, StatusKehadiran: "Hadir"})
	}
	complete := automationReminderRequest(t, app, automationKey)
	if complete.StatusCode != fiber.StatusOK || len(complete.Reminders) != 0 {
		t.Fatalf("complete presensi must not create reminders: %+v", complete.Reminders)
	}

	s.db.Model(&meeting).Updates(map[string]interface{}{"status_pertemuan": "libur", "tanda_tangan": "", "bukti_foto": ""})
	s.db.Where("presensi_id = ?", meeting.ID).Delete(&PresensiDetail{})
	holiday := automationReminderRequest(t, app, automationKey)
	if holiday.StatusCode != fiber.StatusOK || len(holiday.Reminders) != 0 {
		t.Fatalf("libur presensi must be exempt from reminders: %+v", holiday.Reminders)
	}
}
