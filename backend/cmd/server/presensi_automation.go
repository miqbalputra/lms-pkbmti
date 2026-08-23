package main

import (
	"crypto/subtle"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type presensiAutomationMeeting struct {
	ID                 string
	KelasID            string
	Tanggal            time.Time
	TanggalRencana     *time.Time
	StatusPertemuan    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	HasTandaTangan     bool
	HasBuktiFoto       bool
	FilledStudentCount int64
}

type presensiMissingDate struct {
	ClassID    string `json:"classId"`
	ClassLabel string `json:"classLabel"`
	Date       string `json:"date"`
	ActualDate string `json:"actualDate,omitempty"`
	Reason     string `json:"reason"`
}

type presensiTutorReminder struct {
	TutorID      string                `json:"tutorId"`
	TutorName    string                `json:"tutorName"`
	Phone        string                `json:"phone"`
	MissingCount int                   `json:"missingCount"`
	MissingDates []presensiMissingDate `json:"missingDates"`
}

func (s *Server) presensiAutomationAuth(c *fiber.Ctx) error {
	expected := strings.TrimSpace(os.Getenv("N8N_PRESENSI_API_KEY"))
	if len(expected) < 32 {
		return fiber.NewError(fiber.StatusServiceUnavailable, "N8N_PRESENSI_API_KEY wajib dikonfigurasi minimal 32 karakter")
	}
	supplied := strings.TrimSpace(c.Get("X-Automation-Key"))
	if len(supplied) != len(expected) || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
		return fiber.NewError(fiber.StatusUnauthorized, "automation key tidak valid")
	}
	return c.Next()
}

func presensiAutomationDate(t time.Time) string {
	return wibTimeFormat(t, "2006-01-02")
}

func latestPresensiAutomationMeeting(a, b presensiAutomationMeeting) presensiAutomationMeeting {
	aTime := a.UpdatedAt
	if aTime.IsZero() {
		aTime = a.CreatedAt
	}
	bTime := b.UpdatedAt
	if bTime.IsZero() {
		bTime = b.CreatedAt
	}
	if !bTime.Before(aTime) {
		return b
	}
	return a
}

func (s *Server) presensiAutomationReminders(c *fiber.Ctx) error {
	through := time.Now().In(wibLocation)
	if raw := strings.TrimSpace(c.Query("through")); raw != "" {
		parsed, err := parsePresensiDate(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "through harus berformat YYYY-MM-DD")
		}
		through = parsed
	}
	through = time.Date(through.Year(), through.Month(), through.Day(), 0, 0, 0, 0, wibLocation)

	var academicYear TahunAjaran
	if err := s.db.Where("is_aktif = ?", true).First(&academicYear).Error; err != nil {
		return c.JSON(fiber.Map{
			"status": "no_active_academic_year", "through": wibTimeFormat(through, "2006-01-02"),
			"scheduleDay": "Sabtu", "reminders": []presensiTutorReminder{},
		})
	}

	var semester Semester
	if err := s.db.Where(
		"tahun_ajaran_id = ? AND is_archived = ? AND tanggal_mulai <= ? AND tanggal_selesai >= ?",
		academicYear.ID, false, through, through,
	).Order("tanggal_mulai desc").First(&semester).Error; err != nil {
		return c.JSON(fiber.Map{
			"status": "outside_active_semester", "through": wibTimeFormat(through, "2006-01-02"),
			"scheduleDay": "Sabtu", "tahunAjaran": academicYear.NamaTahunAjaran,
			"reminders": []presensiTutorReminder{},
		})
	}

	semesterStart := semester.TanggalMulai.In(wibLocation)
	start := time.Date(semesterStart.Year(), semesterStart.Month(), semesterStart.Day(), 0, 0, 0, 0, wibLocation)
	end := through
	semesterFinish := semester.TanggalSelesai.In(wibLocation)
	semesterEnd := time.Date(semesterFinish.Year(), semesterFinish.Month(), semesterFinish.Day(), 0, 0, 0, 0, wibLocation)
	if semesterEnd.Before(end) {
		end = semesterEnd
	}

	var classes []Kelas
	if err := s.db.Preload("WaliKelas").Where(
		"tahun_ajaran_id = ? AND wali_kelas_id IS NOT NULL", academicYear.ID,
	).Order("jenjang, nama_rombel").Find(&classes).Error; err != nil {
		return err
	}
	classIDs := make([]string, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}
	if len(classIDs) == 0 {
		return c.JSON(fiber.Map{
			"status": "ok", "through": wibTimeFormat(through, "2006-01-02"), "scheduleDay": "Sabtu",
			"tahunAjaran": academicYear.NamaTahunAjaran, "semester": semester.NamaSemester,
			"reminders": []presensiTutorReminder{},
		})
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
		return err
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
		return err
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
			return err
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

	grouped := make(map[string]*presensiTutorReminder)
	for _, class := range classes {
		if class.WaliKelasID == nil || class.WaliKelas == nil {
			continue
		}
		totalStudents := studentCountByClass[class.ID]
		if totalStudents == 0 {
			continue
		}
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			if date.Weekday() != time.Saturday {
				continue
			}
			dateISO := wibTimeFormat(date, "2006-01-02")
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
			tutorID := *class.WaliKelasID
			group := grouped[tutorID]
			if group == nil {
				group = &presensiTutorReminder{
					TutorID: tutorID, TutorName: class.WaliKelas.Nama, Phone: class.WaliKelas.NoHP,
					MissingDates: []presensiMissingDate{},
				}
				grouped[tutorID] = group
			}
			group.MissingDates = append(group.MissingDates, presensiMissingDate{
				ClassID: class.ID, ClassLabel: kelasLabel(class), Date: dateISO,
				ActualDate: actualDate, Reason: strings.Join(reasons, ", "),
			})
		}
	}

	reminders := make([]presensiTutorReminder, 0, len(grouped))
	for _, group := range grouped {
		sort.SliceStable(group.MissingDates, func(i, j int) bool {
			if group.MissingDates[i].Date != group.MissingDates[j].Date {
				return group.MissingDates[i].Date < group.MissingDates[j].Date
			}
			return group.MissingDates[i].ClassLabel < group.MissingDates[j].ClassLabel
		})
		group.MissingCount = len(group.MissingDates)
		reminders = append(reminders, *group)
	}
	sort.SliceStable(reminders, func(i, j int) bool { return reminders[i].TutorName < reminders[j].TutorName })

	return c.JSON(fiber.Map{
		"status": "ok", "generatedAt": time.Now().In(wibLocation).Format(time.RFC3339),
		"through": wibTimeFormat(through, "2006-01-02"), "scheduleDay": "Sabtu",
		"tahunAjaran": academicYear.NamaTahunAjaran, "semester": semester.NamaSemester,
		"semesterStart": wibTimeFormat(start, "2006-01-02"), "semesterEnd": wibTimeFormat(semesterEnd, "2006-01-02"),
		"reminders": reminders,
	})
}
