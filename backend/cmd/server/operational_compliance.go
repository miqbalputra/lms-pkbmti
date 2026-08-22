package main

import (
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// operationalComplianceOption is deliberately small so the dashboard can use
// the same response both for the results and its dependent filters.
type operationalComplianceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type operationalCompliancePeriod struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type operationalComplianceFilters struct {
	TahunAjaranID string `json:"tahunAjaranId"`
	SemesterID    string `json:"semesterId"`
	TutorID       string `json:"tutorId"`
	KelasID       string `json:"kelasId"`
	Status        string `json:"status"`
}

type operationalComplianceSummary struct {
	PresensiTertunda int `json:"presensiTertunda"`
	JurnalTertunda   int `json:"jurnalTertunda"`
	TotalTertunda    int `json:"totalTertunda"`
	TutorDenganTugas int `json:"tutorDenganTugas"`
	KelasDenganTugas int `json:"kelasDenganTugas"`
}

// operationalComplianceClassSummary keeps the monitoring unit explicit: a
// wali kelas and one of their classes. It is a read-only summary; no approval
// state is introduced for either attendance or journals.
type operationalComplianceClassSummary struct {
	ClassID          string `json:"classId"`
	ClassLabel       string `json:"classLabel"`
	TutorID          string `json:"tutorId"`
	TutorName        string `json:"tutorName"`
	PresensiTertunda int    `json:"presensiTertunda"`
	JurnalTertunda   int    `json:"jurnalTertunda"`
	TotalTertunda    int    `json:"totalTertunda"`
	TanggalTertua    string `json:"tanggalTertua,omitempty"`
}

type operationalComplianceTask struct {
	Type       string `json:"type"`
	ClassID    string `json:"classId"`
	ClassLabel string `json:"classLabel"`
	TutorID    string `json:"tutorId"`
	TutorName  string `json:"tutorName"`
	Date       string `json:"date"`
	ActualDate string `json:"actualDate,omitempty"`
	Reason     string `json:"reason"`
	MeetingID  string `json:"meetingId,omitempty"`
}

type operationalComplianceOptions struct {
	TahunAjaran []operationalComplianceOption `json:"tahunAjaran"`
	Semester    []operationalComplianceOption `json:"semester"`
	Tutor       []operationalComplianceOption `json:"tutor"`
	Kelas       []operationalComplianceOption `json:"kelas"`
}

type operationalComplianceResponse struct {
	GeneratedAt    string                              `json:"generatedAt"`
	TahunAjaran    operationalCompliancePeriod         `json:"tahunAjaran"`
	Semester       operationalCompliancePeriod         `json:"semester"`
	Filters        operationalComplianceFilters        `json:"filters"`
	Summary        operationalComplianceSummary        `json:"summary"`
	RingkasanKelas []operationalComplianceClassSummary `json:"ringkasanKelas"`
	Tasks          []operationalComplianceTask         `json:"tasks"`
	Options        operationalComplianceOptions        `json:"options"`
}

func emptyOperationalComplianceResponse(now time.Time) operationalComplianceResponse {
	return operationalComplianceResponse{
		GeneratedAt:    now.In(wibLocation).Format(time.RFC3339),
		RingkasanKelas: []operationalComplianceClassSummary{},
		Tasks:          []operationalComplianceTask{},
		Options: operationalComplianceOptions{
			TahunAjaran: []operationalComplianceOption{},
			Semester:    []operationalComplianceOption{},
			Tutor:       []operationalComplianceOption{},
			Kelas:       []operationalComplianceOption{},
		},
	}
}

func operationalComplianceDay(now time.Time) time.Time {
	local := now.In(wibLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, wibLocation)
}

func operationalComplianceDate(t time.Time) time.Time {
	local := t.In(wibLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, wibLocation)
}

func complianceTutorName(class Kelas) string {
	if class.WaliKelas == nil || strings.TrimSpace(class.WaliKelas.Nama) == "" {
		return "Wali kelas belum ditetapkan"
	}
	return class.WaliKelas.Nama
}

// operationalCompliance is the supervisory, read-only view of the exact
// attendance and journal rules shown in the guru reminder. It is intentionally
// available only to admin and kepala sekolah; guru already receive a private,
// own-class reminder and cannot inspect colleagues' tasks through this route.
func (s *Server) operationalCompliance(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" && role != "kepala_sekolah" {
		return fiber.NewError(fiber.StatusForbidden, "dashboard kepatuhan hanya tersedia untuk admin dan kepala sekolah")
	}

	now := time.Now().In(wibLocation)
	today := operationalComplianceDay(now)
	response := emptyOperationalComplianceResponse(now)
	filters := operationalComplianceFilters{
		TahunAjaranID: strings.TrimSpace(c.Query("tahunAjaranId")),
		SemesterID:    strings.TrimSpace(c.Query("semesterId")),
		TutorID:       strings.TrimSpace(c.Query("tutorId")),
		KelasID:       strings.TrimSpace(c.Query("kelasId")),
		Status:        strings.TrimSpace(c.Query("status")),
	}
	if filters.Status == "" {
		filters.Status = "all"
	}
	if filters.Status != "all" && filters.Status != "presensi" && filters.Status != "jurnal" {
		return fiber.NewError(fiber.StatusBadRequest, "status harus all, presensi, atau jurnal")
	}
	response.Filters = filters

	var academicYears []TahunAjaran
	if err := s.db.Order("tanggal_mulai desc, nama_tahun_ajaran desc").Find(&academicYears).Error; err != nil {
		return err
	}
	for _, academicYear := range academicYears {
		response.Options.TahunAjaran = append(response.Options.TahunAjaran, operationalComplianceOption{
			ID: academicYear.ID, Label: academicYear.NamaTahunAjaran,
		})
	}

	var academicYear TahunAjaran
	if filters.TahunAjaranID != "" {
		if err := s.db.First(&academicYear, "id = ?", filters.TahunAjaranID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fiber.NewError(fiber.StatusNotFound, "tahun ajaran tidak ditemukan")
			}
			return err
		}
	} else {
		if err := s.db.Where("is_aktif = ?", true).Order("tanggal_mulai desc").First(&academicYear).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.JSON(response)
			}
			return err
		}
		response.Filters.TahunAjaranID = academicYear.ID
	}
	response.TahunAjaran = operationalCompliancePeriod{ID: academicYear.ID, Label: academicYear.NamaTahunAjaran}

	var semesters []Semester
	if err := s.db.Where("tahun_ajaran_id = ?", academicYear.ID).Order("tanggal_mulai asc").Find(&semesters).Error; err != nil {
		return err
	}
	for _, semester := range semesters {
		response.Options.Semester = append(response.Options.Semester, operationalComplianceOption{
			ID: semester.ID, Label: semester.NamaSemester,
		})
	}

	var semester Semester
	hasSemester := false
	if filters.SemesterID != "" {
		if err := s.db.Where("id = ? AND tahun_ajaran_id = ?", filters.SemesterID, academicYear.ID).First(&semester).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fiber.NewError(fiber.StatusNotFound, "semester tidak ditemukan pada tahun ajaran terpilih")
			}
			return err
		}
		hasSemester = true
	} else if err := s.db.Where(
		"tahun_ajaran_id = ? AND is_archived = ? AND tanggal_mulai <= ? AND tanggal_selesai >= ?",
		academicYear.ID, false, today, today,
	).Order("tanggal_mulai desc").First(&semester).Error; err == nil {
		hasSemester = true
	} else if err != gorm.ErrRecordNotFound {
		return err
	}
	if hasSemester {
		response.Filters.SemesterID = semester.ID
		response.Semester = operationalCompliancePeriod{ID: semester.ID, Label: semester.NamaSemester}
	}

	// Keep filter options meaningful even if a semester is not currently active.
	// Tutor choices are derived from every class in the chosen year, whereas the
	// class choice narrows after a tutor is selected. This lets a supervisor
	// switch directly from one wali kelas to another without resetting filters.
	var allOptionClasses []Kelas
	if err := s.db.Preload("WaliKelas").
		Where("tahun_ajaran_id = ? AND wali_kelas_id IS NOT NULL", academicYear.ID).
		Order("jenjang, nama_rombel").Find(&allOptionClasses).Error; err != nil {
		return err
	}
	optionClasses := make([]Kelas, 0, len(allOptionClasses))
	for _, class := range allOptionClasses {
		if filters.TutorID == "" || (class.WaliKelasID != nil && *class.WaliKelasID == filters.TutorID) {
			optionClasses = append(optionClasses, class)
		}
	}
	tutorSeen := map[string]bool{}
	for _, class := range allOptionClasses {
		if class.WaliKelasID == nil || tutorSeen[*class.WaliKelasID] {
			continue
		}
		tutorSeen[*class.WaliKelasID] = true
		response.Options.Tutor = append(response.Options.Tutor, operationalComplianceOption{
			ID: *class.WaliKelasID, Label: complianceTutorName(class),
		})
	}
	for _, class := range optionClasses {
		response.Options.Kelas = append(response.Options.Kelas, operationalComplianceOption{ID: class.ID, Label: kelasLabel(class)})
	}
	sort.SliceStable(response.Options.Tutor, func(i, j int) bool {
		return response.Options.Tutor[i].Label < response.Options.Tutor[j].Label
	})

	classes := make([]Kelas, 0, len(optionClasses))
	for _, class := range optionClasses {
		if filters.KelasID == "" || class.ID == filters.KelasID {
			classes = append(classes, class)
		}
	}
	if !hasSemester || len(classes) == 0 {
		return c.JSON(response)
	}

	classSummaries := make(map[string]*operationalComplianceClassSummary, len(classes))
	classByID := make(map[string]Kelas, len(classes))
	for _, class := range classes {
		tutorID := ""
		if class.WaliKelasID != nil {
			tutorID = *class.WaliKelasID
		}
		classByID[class.ID] = class
		classSummaries[class.ID] = &operationalComplianceClassSummary{
			ClassID: class.ID, ClassLabel: kelasLabel(class), TutorID: tutorID, TutorName: complianceTutorName(class),
		}
	}

	appendTask := func(task operationalComplianceTask) {
		response.Tasks = append(response.Tasks, task)
		summary := classSummaries[task.ClassID]
		if summary == nil {
			return
		}
		summary.TotalTertunda++
		if task.Type == "presensi" {
			summary.PresensiTertunda++
			response.Summary.PresensiTertunda++
		} else {
			summary.JurnalTertunda++
			response.Summary.JurnalTertunda++
		}
		if summary.TanggalTertua == "" || task.Date < summary.TanggalTertua {
			summary.TanggalTertua = task.Date
		}
	}

	semesterStart := operationalComplianceDate(semester.TanggalMulai)
	semesterEnd := operationalComplianceDate(semester.TanggalSelesai)
	attendanceEnd := semesterEnd
	if today.Before(attendanceEnd) {
		attendanceEnd = today
	}
	if filters.Status != "jurnal" && !attendanceEnd.Before(semesterStart) {
		presensi, err := s.guruPendingPresensi(classes, semesterStart, attendanceEnd)
		if err != nil {
			return err
		}
		for _, reminder := range presensi {
			class := classByID[reminder.ClassID]
			tutorID := ""
			if class.WaliKelasID != nil {
				tutorID = *class.WaliKelasID
			}
			appendTask(operationalComplianceTask{
				Type: "presensi", ClassID: reminder.ClassID, ClassLabel: reminder.ClassLabel,
				TutorID: tutorID, TutorName: complianceTutorName(class), Date: reminder.Date,
				ActualDate: reminder.ActualDate, Reason: reminder.Reason, MeetingID: reminder.MeetingID,
			})
		}
	}

	if filters.Status != "presensi" && !today.Before(semesterStart) && !today.After(semesterEnd) {
		classesByTutor := map[string][]Kelas{}
		for _, class := range classes {
			if class.WaliKelasID != nil {
				classesByTutor[*class.WaliKelasID] = append(classesByTutor[*class.WaliKelasID], class)
			}
		}
		for tutorID, tutorClasses := range classesByTutor {
			jurnal, err := s.guruPendingJurnal(tutorID, tutorClasses, today)
			if err != nil {
				return err
			}
			for _, reminder := range jurnal {
				class := classByID[reminder.ClassID]
				appendTask(operationalComplianceTask{
					Type: "jurnal", ClassID: reminder.ClassID, ClassLabel: reminder.ClassLabel,
					TutorID: tutorID, TutorName: complianceTutorName(class), Date: reminder.Date,
					Reason: "belum ada jurnal kelas hari ini",
				})
			}
		}
	}

	response.Summary.TotalTertunda = response.Summary.PresensiTertunda + response.Summary.JurnalTertunda
	pendingTutors := map[string]bool{}
	for _, summary := range classSummaries {
		response.RingkasanKelas = append(response.RingkasanKelas, *summary)
		if summary.TotalTertunda > 0 {
			response.Summary.KelasDenganTugas++
			pendingTutors[summary.TutorID] = true
		}
	}
	response.Summary.TutorDenganTugas = len(pendingTutors)
	sort.SliceStable(response.RingkasanKelas, func(i, j int) bool {
		if response.RingkasanKelas[i].TotalTertunda != response.RingkasanKelas[j].TotalTertunda {
			return response.RingkasanKelas[i].TotalTertunda > response.RingkasanKelas[j].TotalTertunda
		}
		return response.RingkasanKelas[i].ClassLabel < response.RingkasanKelas[j].ClassLabel
	})
	sort.SliceStable(response.Tasks, func(i, j int) bool {
		if response.Tasks[i].Date != response.Tasks[j].Date {
			return response.Tasks[i].Date < response.Tasks[j].Date
		}
		if response.Tasks[i].Type != response.Tasks[j].Type {
			return response.Tasks[i].Type < response.Tasks[j].Type
		}
		return response.Tasks[i].ClassLabel < response.Tasks[j].ClassLabel
	})
	return c.JSON(response)
}
