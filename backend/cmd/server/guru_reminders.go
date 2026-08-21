package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// guruReminderPresensi is a single scheduled attendance task that still needs
// attention. MeetingID is present when the meeting exists but is incomplete;
// the client uses it to open the correct edit form directly.
type guruReminderPresensi struct {
	ClassID    string `json:"classId"`
	ClassLabel string `json:"classLabel"`
	Date       string `json:"date"`
	ActualDate string `json:"actualDate,omitempty"`
	Reason     string `json:"reason"`
	MeetingID  string `json:"meetingId,omitempty"`
}

// guruReminderJurnal is intentionally per class (rather than per subject):
// the journal module has no subject timetable, so the reliable task is one
// journal entry for each wali kelas' class on the current WIB calendar day.
type guruReminderJurnal struct {
	ClassID    string `json:"classId"`
	ClassLabel string `json:"classLabel"`
	Date       string `json:"date"`
}

type guruDashboardReminderResponse struct {
	GeneratedAt string                 `json:"generatedAt"`
	Presensi    []guruReminderPresensi `json:"presensi"`
	Jurnal      []guruReminderJurnal   `json:"jurnal"`
}

func emptyGuruDashboardReminderResponse(now time.Time) guruDashboardReminderResponse {
	return guruDashboardReminderResponse{
		GeneratedAt: now.In(wibLocation).Format(time.RFC3339),
		Presensi:    []guruReminderPresensi{},
		Jurnal:      []guruReminderJurnal{},
	}
}

// guruDashboardReminders returns only the pending work for the authenticated
// guru. It deliberately does not accept tutorId/classId query parameters, so
// a browser cannot use the reminder endpoint to inspect another teacher.
func (s *Server) guruDashboardReminders(c *fiber.Ctx) error {
	if c.Locals("role") != "guru" {
		return fiber.NewError(fiber.StatusForbidden, "pengingat tugas hanya tersedia untuk guru")
	}

	now := time.Now().In(wibLocation)
	response := emptyGuruDashboardReminderResponse(now)
	var user User
	if err := s.db.First(&user, "id = ?", c.Locals("userID")).Error; err != nil || user.TutorID == nil {
		// A legacy guru account without a tutor profile has no class scope. Keep
		// the dashboard usable and simply omit reminders instead of returning a
		// repeated error every hour.
		return c.JSON(response)
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wibLocation)
	var academicYear TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&academicYear).Error; err != nil {
		return c.JSON(response)
	}
	var semester Semester
	if err := s.db.Where(
		"tahun_ajaran_id = ? AND is_archived = ? AND tanggal_mulai <= ? AND tanggal_selesai >= ?",
		academicYear.ID, false, today, today,
	).Order("tanggal_mulai desc").First(&semester).Error; err != nil {
		return c.JSON(response)
	}

	var classes []Kelas
	if err := s.db.Where("tahun_ajaran_id = ? AND wali_kelas_id = ?", academicYear.ID, *user.TutorID).
		Order("jenjang, nama_rombel").Find(&classes).Error; err != nil {
		return err
	}
	if len(classes) == 0 {
		return c.JSON(response)
	}

	semesterStart := semester.TanggalMulai.In(wibLocation)
	start := time.Date(semesterStart.Year(), semesterStart.Month(), semesterStart.Day(), 0, 0, 0, 0, wibLocation)
	semesterFinish := semester.TanggalSelesai.In(wibLocation)
	end := time.Date(semesterFinish.Year(), semesterFinish.Month(), semesterFinish.Day(), 0, 0, 0, 0, wibLocation)
	if today.Before(end) {
		end = today
	}
	if !end.Before(start) {
		presensi, err := s.guruPendingPresensi(classes, start, end)
		if err != nil {
			return err
		}
		response.Presensi = presensi
	}

	jurnal, err := s.guruPendingJurnal(*user.TutorID, classes, today)
	if err != nil {
		return err
	}
	response.Jurnal = jurnal
	return c.JSON(response)
}

func (s *Server) guruPendingPresensi(classes []Kelas, start, end time.Time) ([]guruReminderPresensi, error) {
	classIDs := make([]string, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}

	type classStudentCount struct {
		KelasID string
		Total   int64
	}
	var studentCounts []classStudentCount
	if err := s.db.Model(&PesertaDidik{}).
		Select("kelas_id, COUNT(*) AS total").
		Where("kelas_id IN ? AND status = ?", classIDs, "aktif").
		Group("kelas_id").Scan(&studentCounts).Error; err != nil {
		return nil, err
	}
	studentCountByClass := make(map[string]int64, len(studentCounts))
	for _, row := range studentCounts {
		studentCountByClass[row.KelasID] = row.Total
	}

	meetingRangeEnd := end.AddDate(0, 0, 1)
	var meetings []presensiAutomationMeeting
	if err := s.db.Model(&Presensi{}).
		Select(`id, kelas_id, tanggal, tanggal_rencana, status_pertemuan, created_at, updated_at,
			COALESCE(tanda_tangan, '') <> '' AS has_tanda_tangan,
			COALESCE(bukti_foto, '') <> '' AS has_bukti_foto`).
		Where("kelas_id IN ? AND ((tanggal >= ? AND tanggal < ?) OR (tanggal_rencana >= ? AND tanggal_rencana < ?))",
			classIDs, start, meetingRangeEnd, start, meetingRangeEnd).
		Scan(&meetings).Error; err != nil {
		return nil, err
	}

	meetingIDs := make([]string, 0, len(meetings))
	for _, meeting := range meetings {
		meetingIDs = append(meetingIDs, meeting.ID)
	}
	if len(meetingIDs) > 0 {
		type detailCount struct {
			PresensiID string
			Total      int64
		}
		var detailCounts []detailCount
		if err := s.db.Model(&PresensiDetail{}).
			Select("presensi_id, COUNT(DISTINCT peserta_didik_id) AS total").
			Where("presensi_id IN ?", meetingIDs).Group("presensi_id").Scan(&detailCounts).Error; err != nil {
			return nil, err
		}
		countByMeeting := make(map[string]int64, len(detailCounts))
		for _, row := range detailCounts {
			countByMeeting[row.PresensiID] = row.Total
		}
		for i := range meetings {
			meetings[i].FilledStudentCount = countByMeeting[meetings[i].ID]
		}
	}

	meetingByClassDate := make(map[string]presensiAutomationMeeting)
	for _, meeting := range meetings {
		dates := []string{presensiAutomationDate(meeting.Tanggal)}
		if meeting.TanggalRencana != nil {
			dates = append(dates, presensiAutomationDate(*meeting.TanggalRencana))
		}
		seen := map[string]bool{}
		for _, date := range dates {
			if seen[date] {
				continue
			}
			seen[date] = true
			key := meeting.KelasID + "|" + date
			candidate := meeting
			if current, ok := meetingByClassDate[key]; ok {
				candidate = latestPresensiAutomationMeeting(current, candidate)
			}
			meetingByClassDate[key] = candidate
		}
	}

	reminders := make([]guruReminderPresensi, 0)
	for _, class := range classes {
		totalStudents := studentCountByClass[class.ID]
		// A class with no active students has no attendance checklist to fill.
		if totalStudents == 0 {
			continue
		}
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			if date.Weekday() != time.Saturday {
				continue
			}
			dateISO := date.Format("2006-01-02")
			meeting, exists := meetingByClassDate[class.ID+"|"+dateISO]
			if exists && strings.EqualFold(meeting.StatusPertemuan, "libur") {
				continue
			}
			reasons := make([]string, 0, 3)
			actualDate := ""
			if !exists {
				reasons = append(reasons, "belum ada data presensi")
			} else {
				if meeting.FilledStudentCount < totalStudents {
					reasons = append(reasons, fmt.Sprintf("kehadiran %d/%d siswa", meeting.FilledStudentCount, totalStudents))
				}
				if !meeting.HasTandaTangan {
					reasons = append(reasons, "tanda tangan tutor")
				}
				if !meeting.HasBuktiFoto {
					reasons = append(reasons, "foto bukti KBM")
				}
				if planned := meeting.TanggalRencana; planned != nil && !sameDay(meeting.Tanggal, *planned) {
					actualDate = presensiAutomationDate(meeting.Tanggal)
				}
			}
			if len(reasons) == 0 {
				continue
			}
			reminder := guruReminderPresensi{
				ClassID: class.ID, ClassLabel: kelasLabel(class), Date: dateISO,
				ActualDate: actualDate, Reason: strings.Join(reasons, ", "),
			}
			if exists {
				reminder.MeetingID = meeting.ID
			}
			reminders = append(reminders, reminder)
		}
	}
	sort.SliceStable(reminders, func(i, j int) bool {
		if reminders[i].Date != reminders[j].Date {
			return reminders[i].Date < reminders[j].Date
		}
		return reminders[i].ClassLabel < reminders[j].ClassLabel
	})
	return reminders, nil
}

func (s *Server) guruPendingJurnal(tutorID string, classes []Kelas, today time.Time) ([]guruReminderJurnal, error) {
	classIDs := make([]string, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}
	nextDay := today.AddDate(0, 0, 1)
	var completedClassIDs []string
	if err := s.db.Model(&JurnalMengajar{}).
		Where("tutor_id = ? AND kelas_id IN ? AND tanggal >= ? AND tanggal < ?", tutorID, classIDs, today, nextDay).
		Distinct("kelas_id").Pluck("kelas_id", &completedClassIDs).Error; err != nil {
		return nil, err
	}
	completed := make(map[string]bool, len(completedClassIDs))
	for _, classID := range completedClassIDs {
		completed[classID] = true
	}

	reminders := make([]guruReminderJurnal, 0, len(classes))
	for _, class := range classes {
		if completed[class.ID] {
			continue
		}
		reminders = append(reminders, guruReminderJurnal{
			ClassID: class.ID, ClassLabel: kelasLabel(class), Date: today.Format("2006-01-02"),
		})
	}
	sort.SliceStable(reminders, func(i, j int) bool { return reminders[i].ClassLabel < reminders[j].ClassLabel })
	return reminders, nil
}
