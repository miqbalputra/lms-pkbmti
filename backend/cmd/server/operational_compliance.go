package main

import (
	"encoding/json"
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
	// Include classes without a wali so the supervisory reports can expose that
	// configuration gap instead of silently excluding the class. Tutor choices
	// still only contain classes that actually have a wali kelas.
	var allOptionClasses []Kelas
	if err := s.db.Preload("WaliKelas").
		Where("tahun_ajaran_id = ?", academicYear.ID).
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
			if summary.TutorID != "" {
				pendingTutors[summary.TutorID] = true
			}
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

// operationalComplianceDetailFilters intentionally extends the overview
// filters with a date range. The overview stays daily (for reminders), while
// the detail reports can be narrowed to any completed portion of a semester.
type operationalComplianceDetailFilters struct {
	TahunAjaranID string `json:"tahunAjaranId"`
	SemesterID    string `json:"semesterId"`
	TutorID       string `json:"tutorId"`
	KelasID       string `json:"kelasId"`
	From          string `json:"from"`
	To            string `json:"to"`
}

type operationalComplianceDetailSummary struct {
	TotalPertemuan         int `json:"totalPertemuan"`
	PertemuanLengkap       int `json:"pertemuanLengkap"`
	PertemuanBelumLengkap  int `json:"pertemuanBelumLengkap"`
	PertemuanBelumDibuat   int `json:"pertemuanBelumDibuat"`
	PertemuanLibur         int `json:"pertemuanLibur"`
	PertemuanTidakDipantau int `json:"pertemuanTidakDipantau"`
	TotalMapel             int `json:"totalMapel"`
	MapelTerisi            int `json:"mapelTerisi"`
	MapelBelumDiisi        int `json:"mapelBelumDiisi"`
	KelasTanpaMapel        int `json:"kelasTanpaMapel"`
}

type operationalComplianceDetailWarning struct {
	Type       string `json:"type"`
	ClassID    string `json:"classId"`
	ClassLabel string `json:"classLabel"`
	Message    string `json:"message"`
}

type operationalAttendanceMeetingDetail struct {
	MeetingID       string   `json:"meetingId,omitempty"`
	Sequence        int      `json:"sequence"`
	PlannedDate     string   `json:"plannedDate"`
	ActualDate      string   `json:"actualDate,omitempty"`
	StatusPertemuan string   `json:"statusPertemuan"`
	TotalStudents   int      `json:"totalStudents"`
	FilledStudents  int      `json:"filledStudents"`
	HasSignature    bool     `json:"hasSignature"`
	PhotoCount      int      `json:"photoCount"`
	Status          string   `json:"status"`
	Issues          []string `json:"issues"`
}

type operationalAttendanceClassDetail struct {
	ClassID    string                               `json:"classId"`
	ClassLabel string                               `json:"classLabel"`
	TutorID    string                               `json:"tutorId,omitempty"`
	TutorName  string                               `json:"tutorName"`
	Meetings   []operationalAttendanceMeetingDetail `json:"meetings"`
}

type operationalJournalEntryDetail struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Materi    string `json:"materi"`
	Kegiatan  string `json:"kegiatan"`
	TutorName string `json:"tutorName"`
	HasPhoto  bool   `json:"hasPhoto"`
}

type operationalJournalSubjectDetail struct {
	MapelID    string                          `json:"mapelId"`
	MapelName  string                          `json:"mapelName"`
	TutorNames []string                        `json:"tutorNames"`
	EntryCount int                             `json:"entryCount"`
	Status     string                          `json:"status"`
	Entries    []operationalJournalEntryDetail `json:"entries"`
}

type operationalJournalClassDetail struct {
	ClassID    string                            `json:"classId"`
	ClassLabel string                            `json:"classLabel"`
	TutorID    string                            `json:"tutorId,omitempty"`
	TutorName  string                            `json:"tutorName"`
	Subjects   []operationalJournalSubjectDetail `json:"subjects"`
}

type operationalComplianceDetailResponse struct {
	GeneratedAt string                               `json:"generatedAt"`
	Jenis       string                               `json:"jenis"`
	TahunAjaran operationalCompliancePeriod          `json:"tahunAjaran"`
	Semester    operationalCompliancePeriod          `json:"semester"`
	Filters     operationalComplianceDetailFilters   `json:"filters"`
	PeriodStart string                               `json:"periodStart,omitempty"`
	PeriodEnd   string                               `json:"periodEnd,omitempty"`
	Summary     operationalComplianceDetailSummary   `json:"summary"`
	Presensi    []operationalAttendanceClassDetail   `json:"presensi"`
	Jurnal      []operationalJournalClassDetail      `json:"jurnal"`
	Warnings    []operationalComplianceDetailWarning `json:"warnings"`
}

type operationalComplianceDetailScope struct {
	Now          time.Time
	AcademicYear TahunAjaran
	Semester     Semester
	Filters      operationalComplianceDetailFilters
	Start        time.Time
	End          time.Time
	Classes      []Kelas
	HasPeriod    bool
}

func emptyOperationalComplianceDetailResponse(now time.Time, kind string) operationalComplianceDetailResponse {
	return operationalComplianceDetailResponse{
		GeneratedAt: wibTimeFormat(now, time.RFC3339),
		Jenis:       kind,
		Presensi:    []operationalAttendanceClassDetail{},
		Jurnal:      []operationalJournalClassDetail{},
		Warnings:    []operationalComplianceDetailWarning{},
	}
}

func operationalComplianceDetailDate(value string) (time.Time, error) {
	t, err := parseWIBDateTime(value)
	if err != nil {
		return time.Time{}, err
	}
	return operationalComplianceDate(t), nil
}

func (s *Server) operationalComplianceDetailScope(c *fiber.Ctx) (operationalComplianceDetailScope, error) {
	now := time.Now().In(wibLocation)
	scope := operationalComplianceDetailScope{
		Now: now,
		Filters: operationalComplianceDetailFilters{
			TahunAjaranID: strings.TrimSpace(c.Query("tahunAjaranId")),
			SemesterID:    strings.TrimSpace(c.Query("semesterId")),
			TutorID:       strings.TrimSpace(c.Query("tutorId")),
			KelasID:       strings.TrimSpace(c.Query("kelasId")),
			From:          strings.TrimSpace(c.Query("from")),
			To:            strings.TrimSpace(c.Query("to")),
		},
	}

	if scope.Filters.TahunAjaranID != "" {
		if err := s.db.First(&scope.AcademicYear, "id = ?", scope.Filters.TahunAjaranID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return scope, fiber.NewError(fiber.StatusNotFound, "tahun ajaran tidak ditemukan")
			}
			return scope, err
		}
	} else if err := s.db.Where("is_aktif = ?", true).Order("tanggal_mulai desc").First(&scope.AcademicYear).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return scope, nil
		}
		return scope, err
	}
	scope.Filters.TahunAjaranID = scope.AcademicYear.ID

	today := operationalComplianceDay(now)
	if scope.Filters.SemesterID != "" {
		if err := s.db.Where("id = ? AND tahun_ajaran_id = ?", scope.Filters.SemesterID, scope.AcademicYear.ID).First(&scope.Semester).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return scope, fiber.NewError(fiber.StatusNotFound, "semester tidak ditemukan pada tahun ajaran terpilih")
			}
			return scope, err
		}
	} else if err := s.db.Where(
		"tahun_ajaran_id = ? AND is_archived = ? AND tanggal_mulai <= ? AND tanggal_selesai >= ?",
		scope.AcademicYear.ID, false, today, today,
	).Order("tanggal_mulai desc").First(&scope.Semester).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return scope, nil
		}
		return scope, err
	}
	scope.Filters.SemesterID = scope.Semester.ID

	semesterStart := operationalComplianceDate(scope.Semester.TanggalMulai)
	semesterEnd := operationalComplianceDate(scope.Semester.TanggalSelesai)
	maximumEnd := semesterEnd
	if today.Before(maximumEnd) {
		maximumEnd = today
	}
	if maximumEnd.Before(semesterStart) {
		return scope, nil
	}
	scope.Start, scope.End = semesterStart, maximumEnd
	if scope.Filters.From != "" {
		parsed, err := operationalComplianceDetailDate(scope.Filters.From)
		if err != nil {
			return scope, fiber.NewError(fiber.StatusBadRequest, "from harus berformat YYYY-MM-DD")
		}
		if parsed.Before(semesterStart) || parsed.After(maximumEnd) {
			return scope, fiber.NewError(fiber.StatusBadRequest, "from harus berada dalam periode semester yang telah berjalan")
		}
		scope.Start = parsed
	}
	if scope.Filters.To != "" {
		parsed, err := operationalComplianceDetailDate(scope.Filters.To)
		if err != nil {
			return scope, fiber.NewError(fiber.StatusBadRequest, "to harus berformat YYYY-MM-DD")
		}
		if parsed.Before(semesterStart) || parsed.After(maximumEnd) {
			return scope, fiber.NewError(fiber.StatusBadRequest, "to harus berada dalam periode semester yang telah berjalan")
		}
		scope.End = parsed
	}
	if scope.End.Before(scope.Start) {
		return scope, fiber.NewError(fiber.StatusBadRequest, "from tidak boleh setelah to")
	}
	scope.Filters.From = wibTimeFormat(scope.Start, "2006-01-02")
	scope.Filters.To = wibTimeFormat(scope.End, "2006-01-02")
	scope.HasPeriod = true

	q := s.db.Preload("WaliKelas").Where("tahun_ajaran_id = ?", scope.AcademicYear.ID).Order("jenjang, nama_rombel")
	if scope.Filters.TutorID != "" {
		q = q.Where("wali_kelas_id = ?", scope.Filters.TutorID)
	}
	if scope.Filters.KelasID != "" {
		q = q.Where("id = ?", scope.Filters.KelasID)
	}
	if err := q.Find(&scope.Classes).Error; err != nil {
		return scope, err
	}
	return scope, nil
}

func operationalAttendancePhotoCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var photos []string
	if json.Unmarshal([]byte(value), &photos) == nil {
		return len(photos)
	}
	// Older records may contain a single stored value rather than JSON.
	return 1
}

func operationalMeetingIsNewer(candidate, current Presensi) bool {
	candidateTime := candidate.UpdatedAt
	if candidateTime.IsZero() {
		candidateTime = candidate.CreatedAt
	}
	currentTime := current.UpdatedAt
	if currentTime.IsZero() {
		currentTime = current.CreatedAt
	}
	return !candidateTime.Before(currentTime)
}

func (s *Server) operationalComplianceAttendanceDetail(classes []Kelas, start, end time.Time, summary *operationalComplianceDetailSummary) ([]operationalAttendanceClassDetail, error) {
	rows := make([]operationalAttendanceClassDetail, 0, len(classes))
	if len(classes) == 0 || end.Before(start) {
		return rows, nil
	}
	classIDs := make([]string, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}

	type studentCount struct {
		KelasID string
		Total   int
	}
	var studentCounts []studentCount
	if err := s.db.Model(&PesertaDidik{}).
		Select("kelas_id AS kelas_id, COUNT(*) AS total").
		Where("kelas_id IN ? AND status = ?", classIDs, "aktif").
		Group("kelas_id").Scan(&studentCounts).Error; err != nil {
		return nil, err
	}
	studentCountByClass := make(map[string]int, len(studentCounts))
	for _, count := range studentCounts {
		studentCountByClass[count.KelasID] = count.Total
	}

	meetingEnd := end.AddDate(0, 0, 1)
	var meetings []Presensi
	if err := s.db.Where(
		"kelas_id IN ? AND ((tanggal >= ? AND tanggal < ?) OR (tanggal_rencana >= ? AND tanggal_rencana < ?))",
		classIDs, start, meetingEnd, start, meetingEnd,
	).Find(&meetings).Error; err != nil {
		return nil, err
	}
	meetingIDs := make([]string, 0, len(meetings))
	for _, meeting := range meetings {
		meetingIDs = append(meetingIDs, meeting.ID)
	}
	filledByMeeting := map[string]int{}
	if len(meetingIDs) > 0 {
		type filledCount struct {
			PresensiID string
			Total      int
		}
		var filledCounts []filledCount
		if err := s.db.Table("presensi_details").
			Select("presensi_details.presensi_id AS presensi_id, COUNT(DISTINCT presensi_details.peserta_didik_id) AS total").
			Joins("JOIN peserta_didiks ON peserta_didiks.id = presensi_details.peserta_didik_id AND peserta_didiks.status = ?", "aktif").
			Where("presensi_details.presensi_id IN ?", meetingIDs).
			Group("presensi_details.presensi_id").Scan(&filledCounts).Error; err != nil {
			return nil, err
		}
		for _, count := range filledCounts {
			filledByMeeting[count.PresensiID] = count.Total
		}
	}

	meetingByClassDate := map[string]Presensi{}
	for _, meeting := range meetings {
		dates := []string{wibTimeFormat(meeting.Tanggal, "2006-01-02")}
		if meeting.TanggalRencana != nil {
			dates = append(dates, wibTimeFormat(*meeting.TanggalRencana, "2006-01-02"))
		}
		seen := map[string]bool{}
		for _, date := range dates {
			if seen[date] {
				continue
			}
			seen[date] = true
			key := meeting.KelasID + "|" + date
			if current, exists := meetingByClassDate[key]; !exists || operationalMeetingIsNewer(meeting, current) {
				meetingByClassDate[key] = meeting
			}
		}
	}

	for _, class := range classes {
		tutorID := ""
		if class.WaliKelasID != nil {
			tutorID = *class.WaliKelasID
		}
		classRow := operationalAttendanceClassDetail{
			ClassID: class.ID, ClassLabel: kelasLabel(class), TutorID: tutorID,
			TutorName: complianceTutorName(class), Meetings: []operationalAttendanceMeetingDetail{},
		}
		sequence := 0
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			if date.Weekday() != time.Saturday {
				continue
			}
			sequence++
			plannedDate := wibTimeFormat(date, "2006-01-02")
			meeting, exists := meetingByClassDate[class.ID+"|"+plannedDate]
			row := operationalAttendanceMeetingDetail{
				Sequence: sequence, PlannedDate: plannedDate, TotalStudents: studentCountByClass[class.ID], Issues: []string{},
			}
			if row.TotalStudents == 0 {
				row.Status = "tidak_dipantau"
				row.Issues = append(row.Issues, "Tidak ada peserta didik aktif")
				summary.PertemuanTidakDipantau++
				classRow.Meetings = append(classRow.Meetings, row)
				continue
			}
			summary.TotalPertemuan++
			if !exists {
				row.Status = "belum_dibuat"
				row.StatusPertemuan = "belum dibuat"
				row.Issues = append(row.Issues, "Belum ada data presensi")
				summary.PertemuanBelumDibuat++
				classRow.Meetings = append(classRow.Meetings, row)
				continue
			}
			row.MeetingID = meeting.ID
			row.StatusPertemuan = meeting.StatusPertemuan
			row.FilledStudents = filledByMeeting[meeting.ID]
			row.HasSignature = strings.TrimSpace(meeting.TandaTangan) != ""
			row.PhotoCount = operationalAttendancePhotoCount(meeting.BuktiFoto)
			if meeting.TanggalRencana != nil && !sameDay(meeting.Tanggal, *meeting.TanggalRencana) {
				row.ActualDate = wibTimeFormat(meeting.Tanggal, "2006-01-02")
			}
			if strings.EqualFold(meeting.StatusPertemuan, "libur") {
				row.Status = "libur"
				summary.PertemuanLibur++
				classRow.Meetings = append(classRow.Meetings, row)
				continue
			}
			if row.FilledStudents < row.TotalStudents {
				row.Issues = append(row.Issues, "Kehadiran siswa belum lengkap")
			}
			if !row.HasSignature {
				row.Issues = append(row.Issues, "Tanda tangan tutor belum ada")
			}
			if row.PhotoCount == 0 {
				row.Issues = append(row.Issues, "Foto bukti KBM belum ada")
			}
			if len(row.Issues) == 0 {
				row.Status = "lengkap"
				summary.PertemuanLengkap++
			} else {
				row.Status = "belum_lengkap"
				summary.PertemuanBelumLengkap++
			}
			classRow.Meetings = append(classRow.Meetings, row)
		}
		rows = append(rows, classRow)
	}
	return rows, nil
}

func (s *Server) operationalComplianceJournalDetail(classes []Kelas, start, end time.Time, summary *operationalComplianceDetailSummary) ([]operationalJournalClassDetail, []operationalComplianceDetailWarning, error) {
	rows := make([]operationalJournalClassDetail, 0, len(classes))
	warnings := []operationalComplianceDetailWarning{}
	if len(classes) == 0 || end.Before(start) {
		return rows, warnings, nil
	}
	classIDs := make([]string, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}

	var classMapel []KelasMapel
	if err := s.db.Preload("Mapel").Where("kelas_id IN ?", classIDs).Find(&classMapel).Error; err != nil {
		return nil, nil, err
	}
	var assignments []PenugasanGuruMapel
	if err := s.db.Preload("Tutor").Where("kelas_id IN ?", classIDs).Find(&assignments).Error; err != nil {
		return nil, nil, err
	}
	nextDay := end.AddDate(0, 0, 1)
	var journals []JurnalMengajar
	if err := s.db.Preload("Tutor").
		Where("kelas_id IN ? AND tanggal >= ? AND tanggal < ?", classIDs, start, nextDay).
		Order("tanggal desc, created_at desc").Find(&journals).Error; err != nil {
		return nil, nil, err
	}

	mapelByClass := map[string][]KelasMapel{}
	for _, mapping := range classMapel {
		mapelByClass[mapping.KelasID] = append(mapelByClass[mapping.KelasID], mapping)
	}
	tutorNamesByPair := map[string][]string{}
	for _, assignment := range assignments {
		if assignment.Tutor == nil || strings.TrimSpace(assignment.Tutor.Nama) == "" {
			continue
		}
		key := assignment.KelasID + "|" + assignment.MapelID
		tutorNamesByPair[key] = append(tutorNamesByPair[key], assignment.Tutor.Nama)
	}
	for key, names := range tutorNamesByPair {
		sort.Strings(names)
		tutorNamesByPair[key] = uniqueOperationalStrings(names)
	}
	journalByPair := map[string][]JurnalMengajar{}
	for _, journal := range journals {
		key := journal.KelasID + "|" + journal.MapelID
		journalByPair[key] = append(journalByPair[key], journal)
	}

	for _, class := range classes {
		tutorID := ""
		if class.WaliKelasID != nil {
			tutorID = *class.WaliKelasID
		}
		classRow := operationalJournalClassDetail{
			ClassID: class.ID, ClassLabel: kelasLabel(class), TutorID: tutorID,
			TutorName: complianceTutorName(class), Subjects: []operationalJournalSubjectDetail{},
		}
		mappings := mapelByClass[class.ID]
		if len(mappings) == 0 {
			summary.KelasTanpaMapel++
			warnings = append(warnings, operationalComplianceDetailWarning{
				Type: "kelas_tanpa_mapel", ClassID: class.ID, ClassLabel: kelasLabel(class),
				Message: "Kelas belum memiliki mapel yang dikonfigurasi.",
			})
			rows = append(rows, classRow)
			continue
		}
		sort.SliceStable(mappings, func(i, j int) bool { return mappings[i].Mapel.NamaMapel < mappings[j].Mapel.NamaMapel })
		for _, mapping := range mappings {
			key := class.ID + "|" + mapping.MapelID
			entries := journalByPair[key]
			subject := operationalJournalSubjectDetail{
				MapelID: mapping.MapelID, MapelName: mapping.Mapel.NamaMapel,
				TutorNames: append([]string{}, tutorNamesByPair[key]...),
				Entries:    []operationalJournalEntryDetail{},
			}
			for _, journal := range entries {
				tutorName := strings.TrimSpace(journal.Tutor.Nama)
				if tutorName == "" {
					tutorName = "Tutor tidak diketahui"
				}
				subject.Entries = append(subject.Entries, operationalJournalEntryDetail{
					ID: journal.ID, Date: wibTimeFormat(journal.Tanggal, "2006-01-02"), Materi: journal.Materi,
					Kegiatan: journal.Kegiatan, TutorName: tutorName,
					HasPhoto: journal.FotoPath != nil && strings.TrimSpace(*journal.FotoPath) != "",
				})
			}
			subject.EntryCount = len(subject.Entries)
			summary.TotalMapel++
			if subject.EntryCount == 0 {
				subject.Status = "belum_diisi"
				summary.MapelBelumDiisi++
			} else {
				subject.Status = "terisi"
				summary.MapelTerisi++
			}
			classRow.Subjects = append(classRow.Subjects, subject)
		}
		rows = append(rows, classRow)
	}
	return rows, warnings, nil
}

func uniqueOperationalStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

// operationalComplianceDetail powers the two supervisory detail tabs. It
// deliberately omits attendance signatures, Base64 images, and student rows;
// the existing scoped detail endpoints are fetched only when a row is opened.
func (s *Server) operationalComplianceDetail(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" && role != "kepala_sekolah" {
		return fiber.NewError(fiber.StatusForbidden, "laporan detail kepatuhan hanya tersedia untuk admin dan kepala sekolah")
	}
	kind := strings.TrimSpace(c.Query("jenis"))
	if kind != "presensi" && kind != "jurnal" {
		return fiber.NewError(fiber.StatusBadRequest, "jenis harus presensi atau jurnal")
	}
	scope, err := s.operationalComplianceDetailScope(c)
	if err != nil {
		return err
	}
	response := emptyOperationalComplianceDetailResponse(scope.Now, kind)
	response.Filters = scope.Filters
	if scope.AcademicYear.ID == "" || scope.Semester.ID == "" || !scope.HasPeriod {
		return c.JSON(response)
	}
	response.TahunAjaran = operationalCompliancePeriod{ID: scope.AcademicYear.ID, Label: scope.AcademicYear.NamaTahunAjaran}
	response.Semester = operationalCompliancePeriod{ID: scope.Semester.ID, Label: scope.Semester.NamaSemester}
	response.PeriodStart = wibTimeFormat(scope.Start, "2006-01-02")
	response.PeriodEnd = wibTimeFormat(scope.End, "2006-01-02")
	if kind == "presensi" {
		rows, err := s.operationalComplianceAttendanceDetail(scope.Classes, scope.Start, scope.End, &response.Summary)
		if err != nil {
			return err
		}
		response.Presensi = rows
	} else {
		rows, warnings, err := s.operationalComplianceJournalDetail(scope.Classes, scope.Start, scope.End, &response.Summary)
		if err != nil {
			return err
		}
		response.Jurnal = rows
		response.Warnings = warnings
	}
	return c.JSON(response)
}
