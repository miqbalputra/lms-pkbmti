package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================

// ============================================================================
// Ujian Online — public endpoints. The first request authenticates with NISN +
// AksesKode and receives a short-lived, HttpOnly cookie. Legacy clients may
// continue sending the two fields, but the browser page uses the cookie for
// every subsequent request so credentials do not appear in URLs or logs.
// ============================================================================

const (
	examSessionCookie = "exam_session"
	examSessionTTL    = 12 * time.Hour
)

type ujianQuestionSnapshot struct {
	Tipe        string               `json:"tipe"`
	Pertanyaan  string               `json:"pertanyaan"`
	Opsi        string               `json:"opsi"`
	Kunci       string               `json:"kunci"`
	Konfigurasi simulasiConfig       `json:"konfigurasi,omitempty"`
	Stimulus    []simulasiStimulusIn `json:"stimulus,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
}

// publicUjianQuestionConfig is an allowlist: answer keys, accepted answers,
// rubrics, and scoring maps can never be serialized to the exam-taking client.
type publicUjianQuestionConfig struct {
	Choices           []simulasiChoice       `json:"choices,omitempty"`
	Statements        []publicUjianStatement `json:"statements,omitempty"`
	Left              []simulasiChoice       `json:"left,omitempty"`
	Right             []simulasiChoice       `json:"right,omitempty"`
	Rows              []simulasiGridRow      `json:"rows,omitempty"`
	Columns           []simulasiChoice       `json:"columns,omitempty"`
	ScaleMin          int                    `json:"scaleMin,omitempty"`
	ScaleMax          int                    `json:"scaleMax,omitempty"`
	ScaleMinLabel     string                 `json:"scaleMinLabel,omitempty"`
	ScaleMaxLabel     string                 `json:"scaleMaxLabel,omitempty"`
	RatingMax         int                    `json:"ratingMax,omitempty"`
	AllowedFileTypes  []string               `json:"allowedFileTypes,omitempty"`
	MaxFiles          int                    `json:"maxFiles,omitempty"`
	MaxFileSizeMB     int                    `json:"maxFileSizeMB,omitempty"`
	TextMinLength     int                    `json:"textMinLength,omitempty"`
	TextMaxLength     int                    `json:"textMaxLength,omitempty"`
	ValidationMessage string                 `json:"validationMessage,omitempty"`
}

type publicUjianStatement struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func studentSafeUjianConfig(config simulasiConfig) publicUjianQuestionConfig {
	statements := make([]publicUjianStatement, 0, len(config.Statements))
	for _, row := range config.Statements {
		statements = append(statements, publicUjianStatement{ID: row.ID, Text: row.Text})
	}
	return publicUjianQuestionConfig{
		Choices: config.Choices, Statements: statements, Left: config.Left, Right: config.Right,
		Rows: config.Rows, Columns: config.Columns, ScaleMin: config.ScaleMin, ScaleMax: config.ScaleMax,
		ScaleMinLabel: config.ScaleMinLabel, ScaleMaxLabel: config.ScaleMaxLabel, RatingMax: config.RatingMax,
		AllowedFileTypes: config.AllowedFileTypes, MaxFiles: config.MaxFiles, MaxFileSizeMB: config.MaxFileSizeMB,
		TextMinLength: config.TextMinLength, TextMaxLength: config.TextMaxLength, ValidationMessage: config.ValidationMessage,
	}
}

func hasUjianVisualConfig(config simulasiConfig) bool {
	return len(config.Choices) > 0 || len(config.Statements) > 0 || len(config.Left) > 0 ||
		len(config.Right) > 0 || len(config.Pairs) > 0 || len(config.AcceptedAnswers) > 0 ||
		len(config.Rubrik) > 0 || len(config.Rows) > 0 || len(config.Columns) > 0 ||
		len(config.GridCorrect) > 0 || len(config.GridMultiCorrect) > 0 || len(config.CorrectOrder) > 0 ||
		config.CorrectNumber != nil || len(config.AllowedFileTypes) > 0 || config.TextMinLength > 0 ||
		config.TextMaxLength > 0 || strings.TrimSpace(config.ValidationMessage) != ""
}

func ensureUjianAttemptQuestionsTx(tx *gorm.DB, up *UjianPeserta, uj *Ujian) ([]UjianPesertaSoal, error) {
	return loadUjianAttemptQuestionsTx(tx, up, uj, true)
}

func orderUjianQuestions(source []UjianSoal, sections []UjianBagian, seedKey string, shuffle bool) []UjianSoal {
	sectionOrder := make(map[string]int, len(sections))
	for index, section := range sections {
		sectionOrder[section.ClientID] = index
	}
	if len(sections) == 0 {
		if shuffle {
			random := rand.New(rand.NewSource(seedFromID(seedKey)))
			random.Shuffle(len(source), func(i, j int) { source[i], source[j] = source[j], source[i] })
		}
		return source
	}
	sectionKey := func(sectionID string) int {
		if order, ok := sectionOrder[sectionID]; ok {
			return order
		}
		return len(sections)
	}
	sort.SliceStable(source, func(i, j int) bool {
		left, right := sectionKey(source[i].BagianID), sectionKey(source[j].BagianID)
		if left != right {
			return left < right
		}
		return source[i].Urutan < source[j].Urutan
	})
	if shuffle {
		for start := 0; start < len(source); {
			end := start + 1
			for end < len(source) && sectionKey(source[end].BagianID) == sectionKey(source[start].BagianID) {
				end++
			}
			sectionSeedID := source[start].BagianID
			if sectionSeedID == "" {
				sectionSeedID = "unassigned"
			}
			random := rand.New(rand.NewSource(seedFromID(seedKey + ":section:" + sectionSeedID)))
			random.Shuffle(end-start, func(i, j int) { source[start+i], source[start+j] = source[start+j], source[start+i] })
			start = end
		}
	}
	return source
}

func loadUjianAttemptQuestionsTx(tx *gorm.DB, up *UjianPeserta, uj *Ujian, persist bool) ([]UjianPesertaSoal, error) {
	if persist {
		var current UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ? AND ujian_id = ?", up.ID, uj.ID).Error; err != nil {
			return nil, err
		}
	}
	var snapshots []UjianPesertaSoal
	if err := tx.Where("ujian_peserta_id = ?", up.ID).Order("urutan asc").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	if len(snapshots) > 0 {
		return snapshots, nil
	}
	var source []UjianSoal
	if err := tx.Preload("Soal").Where("ujian_id = ?", uj.ID).Order("urutan asc").Order("created_at asc").Find(&source).Error; err != nil {
		return nil, err
	}
	var sections []UjianBagian
	if err := tx.Where("ujian_id = ?", uj.ID).Order("urutan asc").Order("created_at asc").Find(&sections).Error; err != nil {
		return nil, err
	}
	if err := validateUjianBranchAssignments(source, sections); err != nil {
		return nil, err
	}
	sectionByID := make(map[string]UjianBagian, len(sections))
	for _, section := range sections {
		sectionByID[section.ClientID] = section
	}
	// Sectioned exams keep their authored section order while shuffling only
	// questions within each section. Legacy exams retain the original shuffle.
	source = orderUjianQuestions(source, sections, up.ID, uj.AcakSoal)
	snapshots = make([]UjianPesertaSoal, 0, len(source))
	for index, item := range source {
		var config simulasiConfig
		if strings.TrimSpace(item.Soal.Konfigurasi) != "" {
			if err := json.Unmarshal([]byte(item.Soal.Konfigurasi), &config); err != nil {
				return nil, errors.New("konfigurasi soal tidak valid")
			}
		}
		if strings.TrimSpace(item.BranchToByAnswerJSON) != "" {
			if err := json.Unmarshal([]byte(item.BranchToByAnswerJSON), &config.BranchToByAnswer); err != nil {
				return nil, errors.New("aturan alur soal tidak valid")
			}
		}
		stimulus, err := decodeBankSoalStimulus(item.Soal.StimulusJSON)
		if err != nil {
			return nil, err
		}
		metadata := map[string]string{}
		for key, value := range map[string]string{"domain": item.Soal.Domain, "topik": item.Soal.Topik, "kompetensi": item.Soal.Kompetensi, "levelKognitif": item.Soal.LevelKognitif} {
			if value = strings.TrimSpace(value); value != "" {
				metadata[key] = value
			}
		}
		snapshot, err := json.Marshal(ujianQuestionSnapshot{Tipe: item.Soal.Tipe, Pertanyaan: item.Soal.Pertanyaan, Opsi: item.Soal.Opsi, Kunci: item.Soal.Kunci, Konfigurasi: config, Stimulus: stimulus, Metadata: metadata})
		if err != nil {
			return nil, err
		}
		frozenSection := sectionByID[item.BagianID]
		snapshots = append(snapshots, UjianPesertaSoal{
			UjianPesertaID: up.ID, UjianSoalID: item.ID, SoalID: item.SoalID,
			Urutan: index + 1, BagianID: item.BagianID, NamaBagian: frozenSection.Nama,
			DeskripsiBagian: frozenSection.Deskripsi, UrutanBagian: frozenSection.Urutan,
			Bobot: item.Bobot, SnapshotJSON: string(snapshot),
		})
	}
	if len(snapshots) == 0 || !persist {
		return snapshots, nil
	}
	if err := tx.Create(&snapshots).Error; err != nil {
		if !isUniqueErr(err) {
			return nil, err
		}
		if err := tx.Where("ujian_peserta_id = ?", up.ID).Order("urutan asc").Find(&snapshots).Error; err != nil {
			return nil, err
		}
	}
	return snapshots, nil
}

func loadUjianQuestionSnapshot(raw string) (ujianQuestionSnapshot, error) {
	var snapshot ujianQuestionSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil || snapshot.Tipe == "" {
		return snapshot, errors.New("snapshot soal ujian tidak valid")
	}
	return snapshot, nil
}

// activeUjianQuestionIDs evaluates the frozen section route using only the
// learner's saved answers. It returns UjianSoal IDs, never source-bank IDs.
// A branch question with no answer holds later sections until the learner
// answers it; forward-only routes guarantee that the evaluation terminates.
func activeUjianQuestionIDs(questions []UjianPesertaSoal, answers []UjianJawaban) (map[string]bool, error) {
	active := make(map[string]bool, len(questions))
	answerBySourceID := make(map[string]string, len(answers))
	for _, answer := range answers {
		answerBySourceID[answer.SoalID] = answer.Jawaban
	}
	type sectionQuestions struct {
		id    string
		rank  int
		items []UjianPesertaSoal
	}
	groupsByID := make(map[string]*sectionQuestions)
	for _, item := range questions {
		group := groupsByID[item.BagianID]
		if group == nil {
			group = &sectionQuestions{id: item.BagianID, rank: item.UrutanBagian}
			if item.BagianID == "" {
				group.rank = int(^uint(0) >> 1)
			}
			groupsByID[item.BagianID] = group
		}
		group.items = append(group.items, item)
	}
	groups := make([]sectionQuestions, 0, len(groupsByID))
	for _, group := range groupsByID {
		sort.SliceStable(group.items, func(i, j int) bool { return group.items[i].Urutan < group.items[j].Urutan })
		groups = append(groups, *group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].rank != groups[j].rank {
			return groups[i].rank < groups[j].rank
		}
		return groups[i].id < groups[j].id
	})
	if len(groups) == 0 {
		return active, nil
	}
	groupIndex := make(map[string]int, len(groups))
	for index, group := range groups {
		groupIndex[group.id] = index
	}
	for index := 0; index < len(groups); {
		group := groups[index]
		for _, item := range group.items {
			active[item.UjianSoalID] = true
		}
		branchQuestion := UjianPesertaSoal{}
		branchSnapshot := ujianQuestionSnapshot{}
		for _, item := range group.items {
			snapshot, err := loadUjianQuestionSnapshot(item.SnapshotJSON)
			if err != nil {
				return nil, err
			}
			if len(snapshot.Konfigurasi.BranchToByAnswer) > 0 {
				if branchQuestion.UjianSoalID != "" {
					return nil, errors.New("lebih dari satu soal pengatur alur dalam satu bagian")
				}
				branchQuestion, branchSnapshot = item, snapshot
			}
		}
		if branchQuestion.UjianSoalID == "" {
			index++
			continue
		}
		rawAnswer := strings.TrimSpace(answerBySourceID[branchQuestion.SoalID])
		if rawAnswer == "" || rawAnswer == "null" {
			break
		}
		var answerID string
		if err := json.Unmarshal([]byte(rawAnswer), &answerID); err != nil {
			break
		}
		target, routed := branchSnapshot.Konfigurasi.BranchToByAnswer[answerID]
		if !routed {
			index++
			continue
		}
		if target == simulasiBranchFinish {
			break
		}
		next, exists := groupIndex[target]
		if !exists || next <= index {
			return nil, errors.New("tujuan alur bagian tidak valid")
		}
		index = next
	}
	return active, nil
}

func activeUjianAttemptQuestions(tx *gorm.DB, up *UjianPeserta, uj *Ujian) ([]UjianPesertaSoal, []UjianJawaban, map[string]bool, error) {
	questions, err := ensureUjianAttemptQuestionsTx(tx, up, uj)
	if err != nil {
		return nil, nil, nil, err
	}
	var answers []UjianJawaban
	if err := tx.Where("ujian_peserta_id = ?", up.ID).Find(&answers).Error; err != nil {
		return nil, nil, nil, err
	}
	active, err := activeUjianQuestionIDs(questions, answers)
	return questions, answers, active, err
}

func (s *Server) loadActiveUjianAttemptQuestions(up *UjianPeserta, uj *Ujian) ([]UjianPesertaSoal, []UjianJawaban, map[string]bool, error) {
	var questions []UjianPesertaSoal
	var answers []UjianJawaban
	var active map[string]bool
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		questions, answers, active, err = activeUjianAttemptQuestions(tx, up, uj)
		return err
	})
	return questions, answers, active, err
}

// Public exam responses use allowlisted DTOs rather than serializing database
// models. Ujian contains the shared access code, while the persistence models
// may gain other internal fields over time.
type publicExamMapel struct {
	ID        string `json:"id"`
	NamaMapel string `json:"namaMapel"`
	KodeMapel string `json:"kodeMapel"`
}

type publicExamMeta struct {
	ID           string          `json:"id"`
	Judul        string          `json:"judul"`
	WaktuMulai   time.Time       `json:"waktuMulai"`
	WaktuSelesai time.Time       `json:"waktuSelesai"`
	Mapel        publicExamMapel `json:"mapel"`
}

type publicExamItem struct {
	ID               string          `json:"id"`
	Judul            string          `json:"judul"`
	WaktuMulai       time.Time       `json:"waktuMulai"`
	WaktuSelesai     time.Time       `json:"waktuSelesai"`
	DurasiMenit      int             `json:"durasiMenit"`
	GracePeriodMenit int             `json:"gracePeriodMenit"`
	AcakSoal         bool            `json:"acakSoal"`
	BolehEditRespons bool            `json:"bolehEditRespons"`
	Mapel            publicExamMapel `json:"mapel"`
	SudahMengerjakan bool            `json:"sudahMengerjakan"`
	Status           string          `json:"status"`
	Skor             *float64        `json:"skor"`
}

type publicExamAnswerFile struct {
	ID       string `json:"id"`
	NamaFile string `json:"namaFile"`
	Ukuran   int64  `json:"ukuran"`
}

type publicParentExamResult struct {
	ID      string         `json:"id"`
	UjianID string         `json:"ujianId"`
	Mulai   *time.Time     `json:"mulai"`
	Selesai *time.Time     `json:"selesai"`
	Skor    *float64       `json:"skor"`
	Status  string         `json:"status"`
	Ujian   publicExamMeta `json:"ujian"`
}

type examMonitorStudent struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
	NIS  string `json:"nis"`
}

type examMonitorItem struct {
	ID             string             `json:"id"`
	UjianID        string             `json:"ujianId"`
	PesertaDidikID string             `json:"pesertaDidikId"`
	Mulai          *time.Time         `json:"mulai"`
	Selesai        *time.Time         `json:"selesai"`
	Skor           *float64           `json:"skor"`
	Status         string             `json:"status"`
	TabSwitch      int                `json:"tabSwitch"`
	UraianMenunggu int                `json:"uraianMenunggu"`
	PesertaDidik   examMonitorStudent `json:"pesertaDidik"`
}

// Parent portal responses also use narrow read models. Besides keeping the
// contract stable, this prevents internal actor IDs, file paths, share tokens,
// and signature payloads from being exposed merely because a persistence model
// gained a json tag in the future.
type parentMapelSummary struct {
	ID        string `json:"id"`
	NamaMapel string `json:"namaMapel"`
	KodeMapel string `json:"kodeMapel"`
}

type parentClassSummary struct {
	ID         string `json:"id"`
	Jenjang    int    `json:"jenjang"`
	NamaRombel string `json:"namaRombel"`
	Pokjar     struct {
		ID         string `json:"id"`
		NamaPokjar string `json:"namaPokjar"`
	} `json:"pokjar"`
}

type parentChildSummary struct {
	ID               string             `json:"id"`
	Nama             string             `json:"nama"`
	JenisKelamin     string             `json:"jenisKelamin"`
	NIS              string             `json:"nis"`
	NISN             string             `json:"nisn"`
	Status           string             `json:"status"`
	IdentitasFileExt *string            `json:"identitasFileExt"`
	Kelas            parentClassSummary `json:"kelas"`
}

type parentTugasItem struct {
	ID                string             `json:"id"`
	MapelID           string             `json:"mapelId"`
	KelasID           string             `json:"kelasId"`
	Judul             string             `json:"judul"`
	Deskripsi         string             `json:"deskripsi"`
	Deadline          time.Time          `json:"deadline"`
	Semester          string             `json:"semester"`
	BolehUpload       bool               `json:"bolehUpload"`
	ModulID           *string            `json:"modulId"`
	CreatedAt         time.Time          `json:"createdAt"`
	Mapel             parentMapelSummary `json:"mapel"`
	StatusPengumpulan string             `json:"statusPengumpulan"`
	Nilai             *float64           `json:"nilai"`
	TanggalKumpul     *time.Time         `json:"tanggalKumpul"`
}

type parentMateriItem struct {
	ID        string             `json:"id"`
	MapelID   string             `json:"mapelId"`
	KelasID   string             `json:"kelasId"`
	Judul     string             `json:"judul"`
	Deskripsi string             `json:"deskripsi"`
	Tipe      string             `json:"tipe"`
	Ukuran    int64              `json:"ukuran"`
	Semester  string             `json:"semester"`
	ModulID   *string            `json:"modulId"`
	Urutan    int                `json:"urutan"`
	Tanggal   *time.Time         `json:"tanggal"`
	LinkURL   string             `json:"linkUrl"`
	CreatedAt time.Time          `json:"createdAt"`
	Mapel     parentMapelSummary `json:"mapel"`
}

type parentPeminjamanItem struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	BukuID         string    `json:"bukuId"`
	KelasID        string    `json:"kelasId"`
	Semester       string    `json:"semester"`
	TanggalPinjam  time.Time `json:"tanggalPinjam"`
	Status         string    `json:"status"`
	Buku           struct {
		ID       string `json:"id"`
		Judul    string `json:"judul"`
		KodeBuku string `json:"kodeBuku"`
		Penerbit string `json:"penerbit"`
	} `json:"buku"`
}

type parentPresensiItem struct {
	ID              string     `json:"id"`
	PresensiID      string     `json:"presensiId"`
	StatusKehadiran string     `json:"statusKehadiran"`
	Catatan         string     `json:"catatan"`
	Tanggal         *time.Time `json:"tanggal"`
	Semester        string     `json:"semester"`
}

type parentBehaviorItem struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	KelasID        string    `json:"kelasId"`
	Tanggal        time.Time `json:"tanggal"`
	Kategori       string    `json:"kategori"`
	Deskripsi      string    `json:"deskripsi"`
}

type parentGradeItem struct {
	ID             string             `json:"id"`
	PesertaDidikID string             `json:"pesertaDidikId"`
	KelasID        string             `json:"kelasId"`
	MapelID        string             `json:"mapelId"`
	TahunAjaranID  string             `json:"tahunAjaranId"`
	Semester       string             `json:"semester"`
	NPAkhir        *float64           `json:"npAkhir"`
	PredikatNP     string             `json:"predikatNP"`
	NKAkhir        *float64           `json:"nkAkhir"`
	PredikatNK     string             `json:"predikatNK"`
	NAAkhir        *float64           `json:"naAkhir"`
	PredikatNA     string             `json:"predikatNA"`
	Mapel          parentMapelSummary `json:"mapel"`
}

type parentReportNote struct {
	ID             string    `json:"id"`
	PesertaDidikID string    `json:"pesertaDidikId"`
	TahunAjaranID  string    `json:"tahunAjaranId"`
	Semester       string    `json:"semester"`
	CatatanWali    string    `json:"catatanWali"`
	NaikKelas      *bool     `json:"naikKelas"`
	KenaikanKe     *string   `json:"kenaikanKe"`
	CreatedAt      time.Time `json:"createdAt"`
}

type parentReportResponse struct {
	RekapNilai   []parentGradeItem `json:"rekapNilai"`
	CatatanRapor *parentReportNote `json:"catatanRapor"`
}

type parentChatItem struct {
	ID         string     `json:"id"`
	Isi        string     `json:"isi"`
	Dibaca     bool       `json:"dibaca"`
	DibacaPada *time.Time `json:"dibacaPada"`
	CreatedAt  time.Time  `json:"createdAt"`
	IsMine     bool       `json:"isMine"`
}

type notificationItem struct {
	ID         string     `json:"id"`
	Judul      string     `json:"judul"`
	Isi        string     `json:"isi"`
	Tipe       string     `json:"tipe"`
	RefID      *string    `json:"refId"`
	IsRead     bool       `json:"isRead"`
	DibacaPada *time.Time `json:"dibacaPada"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type calendarEventItem struct {
	ID             string            `json:"id"`
	Judul          string            `json:"judul"`
	Deskripsi      string            `json:"deskripsi"`
	TanggalMulai   time.Time         `json:"tanggalMulai"`
	TanggalSelesai *time.Time        `json:"tanggalSelesai"`
	Tipe           string            `json:"tipe"`
	Warna          string            `json:"warna"`
	Semester       *string           `json:"semester"`
	TahunAjaranID  *string           `json:"tahunAjaranId"`
	TahunAjaran    *calendarYearItem `json:"tahunAjaran,omitempty"`
}

type calendarYearItem struct {
	ID                        string     `json:"id"`
	NamaTahunAjaran           string     `json:"namaTahunAjaran"`
	TanggalMulai              time.Time  `json:"tanggalMulai"`
	TanggalSelesai            time.Time  `json:"tanggalSelesai"`
	TanggalMulaiSemesterGenap *time.Time `json:"tanggalMulaiSemesterGenap"`
	IsAktif                   bool       `json:"isAktif"`
}

func notificationItems(rows []Notifikasi) []notificationItem {
	result := make([]notificationItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, notificationItem{
			ID: row.ID, Judul: row.Judul, Isi: row.Isi, Tipe: row.Tipe, RefID: row.RefID,
			IsRead: row.IsRead, DibacaPada: row.DibacaPada, CreatedAt: row.CreatedAt,
		})
	}
	return result
}

func loadParentGradeItems(db *gorm.DB, rows []RekapNilaiAkhir) ([]parentGradeItem, error) {
	mapelIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.MapelID == "" {
			continue
		}
		if _, ok := seen[row.MapelID]; ok {
			continue
		}
		seen[row.MapelID] = struct{}{}
		mapelIDs = append(mapelIDs, row.MapelID)
	}
	mapelByID := make(map[string]MataPelajaran, len(mapelIDs))
	if len(mapelIDs) > 0 {
		var mapel []MataPelajaran
		if err := db.Where("id IN ?", mapelIDs).Find(&mapel).Error; err != nil {
			return nil, err
		}
		for _, row := range mapel {
			mapelByID[row.ID] = row
		}
	}
	result := make([]parentGradeItem, 0, len(rows))
	for _, row := range rows {
		mapel := mapelByID[row.MapelID]
		result = append(result, parentGradeItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, KelasID: row.KelasID,
			MapelID: row.MapelID, TahunAjaranID: row.TahunAjaranID, Semester: row.Semester,
			NPAkhir: row.NPAkhir, PredikatNP: row.PredikatNP, NKAkhir: row.NKAkhir,
			PredikatNK: row.PredikatNK, NAAkhir: row.NAAkhir, PredikatNA: row.PredikatNA,
			Mapel: parentMapelSummary{ID: mapel.ID, NamaMapel: mapel.NamaMapel, KodeMapel: mapel.KodeMapel},
		})
	}
	return result, nil
}

func (s *Server) issueExamSession(c *fiber.Ctx, pd *PesertaDidik, accessCode string) error {
	if pd == nil || pd.ID == "" || strings.TrimSpace(accessCode) == "" || strings.TrimSpace(s.cfg.AccessSecret) == "" {
		return errors.New("sesi ujian tidak dapat diterbitkan")
	}
	expiresAt := time.Now().Add(examSessionTTL)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": pd.ID,
		"typ": "public_exam",
		"ach": hash(strings.TrimSpace(accessCode)),
		"iat": time.Now().Unix(),
		"exp": expiresAt.Unix(),
	}).SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     examSessionCookie,
		Value:    token,
		HTTPOnly: true,
		Secure:   s.cfg.Env == "production",
		SameSite: "Strict",
		Domain:   s.cfg.CookieDomain,
		Expires:  expiresAt,
		MaxAge:   int(examSessionTTL / time.Second),
		Path:     "/api/ujian-online",
	})
	return nil
}

func (s *Server) parseExamSession(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.TrimSpace(s.cfg.AccessSecret) == "" {
		return "", "", errors.New("sesi ujian kosong")
	}
	token, err := jwt.ParseWithClaims(raw, jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.AccessSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || token == nil || !token.Valid {
		return "", "", errors.New("sesi ujian tidak valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["typ"] != "public_exam" {
		return "", "", errors.New("tipe sesi ujian tidak valid")
	}
	studentID, ok := claims["sub"].(string)
	accessHash, hashOK := claims["ach"].(string)
	if !ok || strings.TrimSpace(studentID) == "" || !hashOK || len(accessHash) != 64 {
		return "", "", errors.New("klaim sesi ujian tidak valid")
	}
	return studentID, accessHash, nil
}

func (s *Server) clearExamSession(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     examSessionCookie,
		Value:    "",
		HTTPOnly: true,
		Secure:   s.cfg.Env == "production",
		SameSite: "Strict",
		Domain:   s.cfg.CookieDomain,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		Path:     "/api/ujian-online",
	})
}

func (s *Server) logoutUjianOnline(c *fiber.Ctx) error {
	s.clearExamSession(c)
	return c.SendStatus(204)
}

// ujianOnlineAuth validates NISN + AksesKode and returns the PesertaDidik if valid.
func (s *Server) ujianOnlineAuth(c *fiber.Ctx) (*PesertaDidik, error) {
	nisn := strings.TrimSpace(c.Query("nisn"))
	if nisn == "" {
		nisn = strings.TrimSpace(c.FormValue("nisn"))
	}
	kode := strings.TrimSpace(c.Query("aksesKode"))
	if kode == "" {
		kode = strings.TrimSpace(c.FormValue("aksesKode"))
	}
	if nisn == "" && kode == "" {
		studentID, accessHash, err := s.parseExamSession(c.Cookies(examSessionCookie))
		if err == nil {
			var pd PesertaDidik
			if loadErr := s.db.Preload("Kelas").Where("id = ? AND status = ?", studentID, "aktif").First(&pd).Error; loadErr == nil {
				c.Locals("examAccessCodeHash", accessHash)
				return &pd, nil
			}
		}
	}
	if nisn == "" || kode == "" {
		return nil, fiber.NewError(400, "NISN dan Kode Akses wajib diisi")
	}
	if nisn == temporaryNISN {
		// The placeholder is intentionally shared by several learners while they
		// wait for an official NISN. It cannot identify one exam participant, so
		// never allow it to authenticate a public exam session.
		return nil, fiber.NewError(401, "NISN sementara belum dapat digunakan untuk ujian online")
	}
	var pd PesertaDidik
	if s.db.Preload("Kelas").Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return nil, fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	c.Locals("examAccessCodeHash", hash(kode))
	return &pd, nil
}

// ujianOnlineKodeAuth validates NISN + AksesKode against a specific Ujian.
func (s *Server) ujianOnlineKodeAuth(c *fiber.Ctx, ujianID string) (*PesertaDidik, *Ujian, error) {
	pd, err := s.ujianOnlineAuth(c)
	if err != nil {
		return nil, nil, err
	}
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return nil, nil, fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if uj.AksesKode == "" {
		return nil, nil, fiber.NewError(403, "Ujian ini tidak memiliki kode akses")
	}
	if sessionHash, ok := c.Locals("examAccessCodeHash").(string); ok && sessionHash != "" {
		if subtle.ConstantTimeCompare([]byte(sessionHash), []byte(hash(strings.TrimSpace(uj.AksesKode)))) != 1 {
			return nil, nil, fiber.NewError(403, "Kode akses salah")
		}
	} else if strings.TrimSpace(uj.AksesKode) != strings.TrimSpace(c.Query("aksesKode")) &&
		strings.TrimSpace(uj.AksesKode) != strings.TrimSpace(c.FormValue("aksesKode")) {
		return nil, nil, fiber.NewError(403, "Kode akses salah")
	}
	if pd.KelasID != uj.KelasID {
		return nil, nil, fiber.NewError(403, "Anda tidak terdaftar di kelas ujian ini")
	}
	now := time.Now()
	if now.Before(uj.WaktuMulai) {
		return nil, nil, fiber.NewError(403, "Ujian belum dimulai")
	}
	if now.After(uj.WaktuSelesai) {
		return nil, nil, fiber.NewError(403, "Ujian sudah berakhir")
	}
	return pd, &uj, nil
}

// cekUjianOnline — POST /ujian-online/cek {nisn, aksesKode}
// Returns list of exams available for the student's class that match the access code.
func (s *Server) cekUjianOnline(c *fiber.Ctx) error {
	nisn := strings.TrimSpace(c.FormValue("nisn"))
	kode := strings.TrimSpace(c.FormValue("aksesKode"))
	turnstileToken := c.FormValue("cf-turnstile-response")
	if nisn == "" || kode == "" {
		return fiber.NewError(400, "NISN dan Kode Akses wajib diisi")
	}
	if nisn == temporaryNISN {
		return fiber.NewError(401, "NISN sementara belum dapat digunakan untuk ujian online")
	}
	if e := s.requireTurnstile(c, turnstileToken); e != nil {
		return e
	}
	var pd PesertaDidik
	if s.db.Where("nisn = ? AND status = ?", nisn, "aktif").First(&pd).Error != nil {
		return fiber.NewError(401, "NISN tidak ditemukan atau tidak aktif")
	}
	var ujians []Ujian
	queryErr := s.db.Preload("Mapel").Preload("Kelas").
		Where("akses_kode = ? AND kelas_id = ?", strings.TrimSpace(kode), pd.KelasID).
		Where("waktu_mulai <= ? AND waktu_selesai >= ?", time.Now(), time.Now()).
		Order("waktu_mulai desc").
		Find(&ujians).Error
	if queryErr != nil {
		return fiber.NewError(500, "Gagal memuat daftar ujian")
	}
	if len(ujians) == 0 {
		return fiber.NewError(404, "Tidak ada ujian aktif dengan kode akses tersebut untuk kelas Anda")
	}
	if err := s.issueExamSession(c, &pd, kode); err != nil {
		return fiber.NewError(500, "Gagal menyiapkan sesi ujian")
	}
	// Load all existing sessions in one query instead of one query per exam.
	ujianIDs := make([]string, 0, len(ujians))
	for _, uj := range ujians {
		ujianIDs = append(ujianIDs, uj.ID)
	}
	var sessions []UjianPeserta
	if err := s.db.Where("peserta_didik_id = ? AND ujian_id IN ?", pd.ID, ujianIDs).Find(&sessions).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat status ujian")
	}
	sessionByExam := make(map[string]UjianPeserta, len(sessions))
	for _, session := range sessions {
		sessionByExam[session.UjianID] = session
	}
	res := make([]publicExamItem, 0, len(ujians))
	for _, uj := range ujians {
		r := publicExamItem{
			ID: uj.ID, Judul: uj.Judul, WaktuMulai: uj.WaktuMulai,
			WaktuSelesai: uj.WaktuSelesai, DurasiMenit: uj.DurasiMenit,
			GracePeriodMenit: uj.GracePeriodMenit, AcakSoal: uj.AcakSoal,
			BolehEditRespons: false,
			Mapel:            publicExamMapel{ID: uj.Mapel.ID, NamaMapel: uj.Mapel.NamaMapel, KodeMapel: uj.Mapel.KodeMapel},
		}
		if up, ok := sessionByExam[uj.ID]; ok {
			r.SudahMengerjakan = true
			r.Status = up.Status
			r.Skor = up.Skor
			r.BolehEditRespons = ujianResponseEditAllowed(up, uj, time.Now())
		}
		res = append(res, r)
	}
	return c.JSON(res)
}

// mulaiUjianOnline — POST /ujian-online/:ujianId/mulai {nisn, aksesKode}
// Creates a UjianPeserta record (idempotent: returns existing if already started).
func (s *Server) mulaiUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	// Idempotent: if already has a session, return it
	var up UjianPeserta
	findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error
	if findErr == nil {
		if up.Status == "mulai" {
			if err := s.db.Transaction(func(tx *gorm.DB) error {
				_, err := ensureUjianAttemptQuestionsTx(tx, &up, uj)
				return err
			}); err != nil {
				return fiber.NewError(500, "Gagal membekukan soal ujian")
			}
		}
		return c.JSON(up)
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return fiber.NewError(500, "Gagal memuat sesi ujian")
	}
	now := time.Now()
	up = UjianPeserta{
		UjianID:          uj.ID,
		PesertaDidikID:   pd.ID,
		KelasIDSaatUjian: pd.KelasID,
		Mulai:            &now,
		Status:           "mulai",
	}
	if e := s.db.Create(&up).Error; e != nil {
		// Two tabs can submit "Mulai" at the same time. The unique index is the
		// source of truth; return the winner's session instead of a 500.
		if isUniqueErr(e) {
			if lookupErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; lookupErr == nil {
				if up.Status == "mulai" {
					if snapshotErr := s.db.Transaction(func(tx *gorm.DB) error { _, err := ensureUjianAttemptQuestionsTx(tx, &up, uj); return err }); snapshotErr != nil {
						return fiber.NewError(500, "Gagal membekukan soal ujian")
					}
				}
				return c.JSON(up)
			}
		}
		return fiber.NewError(500, "Gagal memulai ujian: "+e.Error())
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		_, err := ensureUjianAttemptQuestionsTx(tx, &up, uj)
		return err
	}); err != nil {
		return fiber.NewError(500, "Gagal membekukan soal ujian")
	}
	return c.Status(201).JSON(up)
}

// getSoalUjianOnline — GET /ujian-online/:ujianId/soal with the signed exam
// session cookie issued by /cek (legacy credential query/form input remains
// accepted by the server for older clients).
// Returns shuffled soal list (without answers) for the exam.
func (s *Server) getSoalUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	// Check or create UjianPeserta
	var up UjianPeserta
	findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error
	if findErr != nil {
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fiber.NewError(500, "Gagal memuat sesi ujian")
		}
		now := time.Now()
		up = UjianPeserta{UjianID: uj.ID, PesertaDidikID: pd.ID, KelasIDSaatUjian: pd.KelasID, Mulai: &now, Status: "mulai"}
		if createErr := s.db.Create(&up).Error; createErr != nil && !isUniqueErr(createErr) {
			return fiber.NewError(500, "Gagal membuat sesi ujian")
		}
		if up.ID == "" {
			if lookupErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; lookupErr != nil {
				return fiber.NewError(500, "Gagal memuat sesi ujian")
			}
		}
	}
	if up.Status == "dikunci" || ((up.Status == "selesai" || up.Status == "menunggu_nilai") && !ujianResponseEditAllowed(up, *uj, time.Now())) {
		return fiber.NewError(403, "Anda sudah menyelesaikan ujian ini")
	}
	// Check if time is up (beyond grace period) only while the attempt is live.
	if up.Status == "mulai" && up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			// Hard lock: grace period expired
			now := time.Now()
			if _, finishErr := s.finishUjianAttempt(&up, uj, now, false, true, nil); finishErr != nil {
				return fiber.NewError(500, "Gagal menghitung nilai ujian")
			}
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
	}
	var order []UjianPesertaSoal
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var snapshotErr error
		order, snapshotErr = ensureUjianAttemptQuestionsTx(tx, &up, uj)
		return snapshotErr
	}); err != nil {
		return fiber.NewError(500, "Gagal memuat snapshot soal ujian")
	}
	var jawabans []UjianJawaban
	if err := s.db.Where("ujian_peserta_id = ?", up.ID).Find(&jawabans).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat jawaban ujian")
	}
	activeQuestions, err := activeUjianQuestionIDs(order, jawabans)
	if err != nil {
		return fiber.NewError(500, "Alur bagian ujian tidak valid")
	}
	// Build response: strip answers
	type soalRes struct {
		ID              string                     `json:"id"`
		UjianID         string                     `json:"ujianId"`
		Bobot           float64                    `json:"bobot"`
		Pertanyaan      string                     `json:"pertanyaan"`
		Tipe            string                     `json:"tipe"`
		Opsi            []string                   `json:"opsi"`
		OpsiIndex       []int                      `json:"opsiIndex,omitempty"`
		Konfigurasi     *publicUjianQuestionConfig `json:"konfigurasi,omitempty"`
		Stimulus        []simulasiStimulusIn       `json:"stimulus,omitempty"`
		BagianID        string                     `json:"bagianId,omitempty"`
		NamaBagian      string                     `json:"namaBagian,omitempty"`
		DeskripsiBagian string                     `json:"deskripsiBagian,omitempty"`
		UrutanBagian    int                        `json:"urutanBagian,omitempty"`
		HasBranching    bool                       `json:"hasBranching,omitempty"`
		// Benar/Kunci are intentionally excluded
	}
	var res []soalRes
	for _, item := range order {
		if !activeQuestions[item.UjianSoalID] {
			continue
		}
		snapshot, snapshotErr := loadUjianQuestionSnapshot(item.SnapshotJSON)
		if snapshotErr != nil {
			return fiber.NewError(500, "Snapshot soal ujian tidak valid")
		}
		r := soalRes{
			ID:         item.UjianSoalID,
			UjianID:    uj.ID,
			Bobot:      item.Bobot,
			Pertanyaan: snapshot.Pertanyaan,
			Tipe:       snapshot.Tipe,
			Stimulus:   snapshot.Stimulus,
			BagianID:   item.BagianID, NamaBagian: item.NamaBagian,
			DeskripsiBagian: item.DeskripsiBagian, UrutanBagian: item.UrutanBagian,
			HasBranching: len(snapshot.Konfigurasi.BranchToByAnswer) > 0,
		}
		if hasUjianVisualConfig(snapshot.Konfigurasi) {
			publicConfig := studentSafeUjianConfig(snapshot.Konfigurasi)
			if uj.AcakSoal {
				seed := seedFromID(up.ID + ":" + item.UjianSoalID + ":options")
				if len(publicConfig.Choices) > 1 {
					random := rand.New(rand.NewSource(seed))
					random.Shuffle(len(publicConfig.Choices), func(i, j int) {
						publicConfig.Choices[i], publicConfig.Choices[j] = publicConfig.Choices[j], publicConfig.Choices[i]
					})
				}
				if len(publicConfig.Right) > 1 {
					random := rand.New(rand.NewSource(seed + 1))
					random.Shuffle(len(publicConfig.Right), func(i, j int) {
						publicConfig.Right[i], publicConfig.Right[j] = publicConfig.Right[j], publicConfig.Right[i]
					})
				}
			}
			r.Konfigurasi = &publicConfig
		}
		if snapshot.Opsi != "" {
			var options []string
			if json.Unmarshal([]byte(snapshot.Opsi), &options) == nil {
				indices := make([]int, len(options))
				for i := range indices {
					indices[i] = i
				}
				if uj.AcakSoal {
					options, indices = shuffleBankOptionList(options, seedFromID(up.ID+":"+item.UjianSoalID))
				}
				r.Opsi, r.OpsiIndex = options, indices
			}
		}
		res = append(res, r)
	}
	// Also return existing answers
	type jawabanRes struct {
		UjianSoalID string                 `json:"ujianSoalId"`
		Jawaban     string                 `json:"jawaban"`
		Berkas      []publicExamAnswerFile `json:"berkas,omitempty"`
	}
	var files []UjianJawabanBerkas
	if err := s.db.Where("ujian_peserta_id = ?", up.ID).Find(&files).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat berkas jawaban")
	}
	filesByID := make(map[string]UjianJawabanBerkas, len(files))
	for _, file := range files {
		filesByID[file.ID] = file
	}
	ujianSoalIDByBankSoalID := make(map[string]string, len(order))
	for _, item := range order {
		ujianSoalIDByBankSoalID[item.SoalID] = item.UjianSoalID
	}
	jawabanList := make([]jawabanRes, 0, len(jawabans))
	for _, j := range jawabans {
		ujianSoalID, ok := ujianSoalIDByBankSoalID[j.SoalID]
		if !ok || !activeQuestions[ujianSoalID] {
			continue
		}
		row := jawabanRes{UjianSoalID: ujianSoalID, Jawaban: j.Jawaban}
		if ids, decodeErr := decodeSimulasiFileIDs([]byte(j.Jawaban)); decodeErr == nil {
			for _, fileID := range ids {
				if file, exists := filesByID[fileID]; exists && file.UjianSoalID == ujianSoalID {
					row.Berkas = append(row.Berkas, publicExamAnswerFile{ID: file.ID, NamaFile: file.NamaFile, Ukuran: file.Ukuran})
				}
			}
		}
		jawabanList = append(jawabanList, row)
	}
	return c.JSON(fiber.Map{
		"ujianPesertaId":   up.ID,
		"sisaWaktu":        s.sisaWaktu(&up, uj),
		"gracePeriodMenit": uj.GracePeriodMenit,
		"bolehEditRespons": ujianResponseEditAllowed(up, *uj, time.Now()),
		"soal":             res,
		"jawaban":          jawabanList,
	})
}

// sisaWaktu returns remaining seconds. Positive = normal countdown.
// Negative = grace period (absolute value = grace seconds remaining).
// Returns 0 if no timer or both expired.
func (s *Server) sisaWaktu(up *UjianPeserta, uj *Ujian) int {
	if up.Mulai == nil || uj.DurasiMenit == 0 {
		return 0
	}
	batas := batasWaktu(up, uj)
	sisa := time.Until(batas).Seconds()
	if sisa >= 0 {
		return int(sisa)
	}
	// Normal time expired — check grace period
	grace := batasGrace(up, uj)
	sisaGrace := time.Until(grace).Seconds()
	if sisaGrace > 0 {
		return -int(sisaGrace) // negative = grace period
	}
	return 0 // both expired
}

func regradeUjianAttemptAfterRevisionTx(tx *gorm.DB, attempt *UjianPeserta, uj *Ujian) (ujianGradeResult, error) {
	var grade ujianGradeResult
	if err := gradeUjianPesertaTx(tx, attempt, uj, &grade); err != nil {
		return grade, err
	}
	status := "selesai"
	var score *float64 = &grade.Score
	if grade.PendingManual > 0 {
		status, score = "menunggu_nilai", nil
	}
	if err := tx.Model(&UjianPeserta{}).Where("id = ?", attempt.ID).Updates(map[string]interface{}{"status": status, "skor": score}).Error; err != nil {
		return grade, err
	}
	attempt.Status, attempt.Skor = status, score
	return grade, nil
}

func ujianResponseEditAllowed(attempt UjianPeserta, uj Ujian, now time.Time) bool {
	if !uj.IzinkanEditRespons || attempt.PenutupanOtomatis || (attempt.Status != "selesai" && attempt.Status != "menunggu_nilai") {
		return false
	}
	return uj.WaktuSelesai.IsZero() || !now.After(uj.WaktuSelesai)
}

func ujianJawabanPenilaianAudit(answer UjianJawaban) (string, error) {
	return marshalJSON(map[string]interface{}{
		"benar": answer.Benar, "nilai": answer.Nilai, "nilaiManual": answer.NilaiManual,
		"komentarGuru": answer.KomentarGuru, "dinilaiOlehUserId": answer.DinilaiOlehUserID,
		"dinilaiPada": answer.DinilaiPada,
	})
}

func ujianActiveQuestionList(questions []UjianPesertaSoal, answers []UjianJawaban) ([]string, error) {
	active, err := activeUjianQuestionIDs(questions, answers)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(active))
	for _, question := range questions {
		if active[question.UjianSoalID] {
			ids = append(ids, question.UjianSoalID)
		}
	}
	return ids, nil
}

func marshalJSON(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (s *Server) reviseSubmittedUjianAnswer(pd *PesertaDidik, uj *Ujian, attempt *UjianPeserta, ujianSoalID, answerText string, c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Get("Idempotency-Key"))
	if _, err := uuid.Parse(key); err != nil {
		return fiber.NewError(400, "kunci idempotensi revisi tidak valid")
	}
	hashInput, _ := json.Marshal(struct {
		UjianSoalID string `json:"ujianSoalId"`
		Jawaban     string `json:"jawaban"`
	}{ujianSoalID, answerText})
	requestHash := hash(string(hashInput))
	var revision UjianJawabanRevisi
	created, replayed := false, false
	var grade ujianGradeResult
	activeIDs := []string{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookupErr := tx.Where("ujian_peserta_id = ? AND idempotency_key = ?", attempt.ID, key).First(&revision).Error
		if lookupErr == nil {
			if revision.UjianSoalID != ujianSoalID || revision.RequestHash != requestHash {
				return fiber.NewError(409, "kunci idempotensi sudah digunakan untuk perubahan lain")
			}
			replayed = true
			var summaryErr error
			grade, summaryErr = summarizeUjianAttemptTx(tx, attempt, uj)
			if summaryErr != nil {
				return summaryErr
			}
			questions, err := ensureUjianAttemptQuestionsTx(tx, attempt, uj)
			if err != nil {
				return err
			}
			var answers []UjianJawaban
			if err := tx.Where("ujian_peserta_id = ?", attempt.ID).Find(&answers).Error; err != nil {
				return err
			}
			activeIDs, err = ujianActiveQuestionList(questions, answers)
			if err != nil {
				return err
			}
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		var locked UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ? AND ujian_id = ? AND peserta_didik_id = ?", attempt.ID, uj.ID, pd.ID).Error; err != nil {
			return fiber.NewError(404, "percobaan ujian tidak ditemukan")
		}
		// Recheck after taking the attempt lock so simultaneous retries cannot
		// commit a duplicate revision with the same idempotency key.
		lookupErr = tx.Where("ujian_peserta_id = ? AND idempotency_key = ?", locked.ID, key).First(&revision).Error
		if lookupErr == nil {
			if revision.UjianSoalID != ujianSoalID || revision.RequestHash != requestHash {
				return fiber.NewError(409, "kunci idempotensi sudah digunakan untuk perubahan lain")
			}
			replayed = true
			var summaryErr error
			grade, summaryErr = summarizeUjianAttemptTx(tx, &locked, uj)
			if summaryErr != nil {
				return summaryErr
			}
			questions, err := ensureUjianAttemptQuestionsTx(tx, &locked, uj)
			if err != nil {
				return err
			}
			var saved []UjianJawaban
			if err := tx.Where("ujian_peserta_id = ?", locked.ID).Find(&saved).Error; err != nil {
				return err
			}
			var activeErr error
			activeIDs, activeErr = ujianActiveQuestionList(questions, saved)
			if activeErr != nil {
				return activeErr
			}
			*attempt = locked
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		var currentExam Ujian
		if err := tx.First(&currentExam, "id = ?", uj.ID).Error; err != nil {
			return fiber.NewError(404, "ujian tidak ditemukan")
		}
		if !ujianResponseEditAllowed(locked, currentExam, time.Now()) {
			return fiber.NewError(409, "perubahan respons tidak diizinkan atau masa revisi telah berakhir")
		}
		questions, err := ensureUjianAttemptQuestionsTx(tx, &locked, &currentExam)
		if err != nil {
			return err
		}
		var answers []UjianJawaban
		if err := tx.Where("ujian_peserta_id = ?", locked.ID).Find(&answers).Error; err != nil {
			return err
		}
		activeBefore, err := activeUjianQuestionIDs(questions, answers)
		if err != nil {
			return fiber.NewError(500, "alur bagian ujian tidak valid")
		}
		var frozen UjianPesertaSoal
		for _, question := range questions {
			if question.UjianSoalID == ujianSoalID {
				frozen = question
				break
			}
		}
		if frozen.ID == "" || !activeBefore[frozen.UjianSoalID] {
			return fiber.NewError(404, "soal tidak tersedia pada percobaan ini")
		}
		snapshot, err := loadUjianQuestionSnapshot(frozen.SnapshotJSON)
		if err != nil {
			return fiber.NewError(500, "snapshot soal ujian tidak valid")
		}
		configRaw := ""
		if hasUjianVisualConfig(snapshot.Konfigurasi) {
			config, err := json.Marshal(snapshot.Konfigurasi)
			if err != nil {
				return fiber.NewError(500, "konfigurasi soal tidak valid")
			}
			configRaw = string(config)
		}
		if err := validateUjianAnswer(snapshot.Tipe, snapshot.Opsi, answerText, configRaw); err != nil {
			return err
		}
		if snapshot.Tipe == simulasiTipeUnggah {
			fileIDs, err := decodeSimulasiFileIDs([]byte(answerText))
			if err != nil {
				return fiber.NewError(400, "referensi berkas jawaban tidak valid")
			}
			var count int64
			if err := tx.Model(&UjianJawabanBerkas{}).Where("id IN ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", fileIDs, locked.ID, frozen.UjianSoalID).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(fileIDs)) {
				return fiber.NewError(403, "berkas harus berasal dari jawaban soal ini")
			}
		}
		var answer UjianJawaban
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ujian_peserta_id = ? AND soal_id = ?", locked.ID, frozen.SoalID).First(&answer).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		beforeAnswer, beforeGrading := "", ""
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			answer = UjianJawaban{UjianPesertaID: locked.ID, SoalID: frozen.SoalID}
		} else {
			beforeAnswer = answer.Jawaban
			beforeGrading, err = ujianJawabanPenilaianAudit(answer)
			if err != nil {
				return err
			}
		}
		answer.Jawaban = answerText
		// A response edit invalidates any teacher grade/comment for that answer.
		answer.Benar, answer.Nilai, answer.NilaiManual = nil, 0, nil
		answer.KomentarGuru, answer.DinilaiOlehUserID, answer.DinilaiPada = "", nil, nil
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&answer).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&answer).Error; err != nil {
			return err
		}
		grade, err = regradeUjianAttemptAfterRevisionTx(tx, &locked, &currentExam)
		if err != nil {
			return err
		}
		if err := tx.Where("ujian_peserta_id = ?", locked.ID).Find(&answers).Error; err != nil {
			return err
		}
		activeIDs, err = ujianActiveQuestionList(questions, answers)
		if err != nil {
			return fiber.NewError(500, "alur bagian ujian tidak valid")
		}
		answerAfter, err := ujianJawabanPenilaianAudit(answer)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&UjianJawabanRevisi{}).Where("ujian_peserta_id = ? AND ujian_soal_id = ?", locked.ID, ujianSoalID).Count(&count).Error; err != nil {
			return err
		}
		revision = UjianJawabanRevisi{
			UjianPesertaID: locked.ID, IdempotencyKey: key, UjianSoalID: ujianSoalID,
			Nomor: int(count) + 1, PesertaDidikID: pd.ID, AktorID: pd.ID,
			RequestHash: requestHash, JawabanSebelum: beforeAnswer, JawabanSesudah: answerText,
			PenilaianSebelumJSON: beforeGrading, PenilaianSesudahJSON: answerAfter,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		*attempt = locked
		created = true
		return nil
	})
	if err != nil {
		return err
	}
	if created {
		s.audit(&pd.ID, "revise_response", "ujian_online_jawaban", revision.ID)
	}
	status := "tersimpan"
	if replayed {
		status = "sudah_tersimpan"
	}
	response := ujianSubmitResponse(attempt, grade)
	response["statusSimpan"], response["revision"] = status, revision.Nomor
	response["bolehEditRespons"] = ujianResponseEditAllowed(*attempt, *uj, time.Now())
	response["activeSoalIds"] = activeIDs
	return c.JSON(response)
}

// jawabSoal — POST /ujian-online/:ujianId/jawab {nisn, aksesKode, ujianSoalId, jawaban}
func (s *Server) jawabSoal(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if findErr := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error; findErr != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "dikunci" || (up.Status != "mulai" && !ujianResponseEditAllowed(up, *uj, time.Now())) {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	// Check time (beyond grace period = hard lock)
	if up.Status == "mulai" && up.Mulai != nil && uj.DurasiMenit > 0 {
		if time.Now().After(batasGrace(&up, uj)) {
			now := time.Now()
			if _, finishErr := s.finishUjianAttempt(&up, uj, now, false, true, nil); finishErr != nil {
				return fiber.NewError(500, "Gagal menghitung nilai ujian")
			}
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
	}
	var in struct {
		UjianSoalID string `json:"ujianSoalId"`
		Jawaban     string `json:"jawaban"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.UjianSoalID == "" {
		return fiber.NewError(400, "ujianSoalId wajib diisi")
	}
	in.Jawaban = strings.TrimSpace(in.Jawaban)
	if len([]byte(in.Jawaban)) > 64*1024 {
		return fiber.NewError(413, "jawaban terlalu panjang")
	}
	if up.Status == "selesai" || up.Status == "menunggu_nilai" {
		return s.reviseSubmittedUjianAnswer(pd, uj, &up, in.UjianSoalID, in.Jawaban, c)
	}
	// Accept only question IDs frozen into this student's attempt. The snapshot
	// remains valid if staff archive/remove the source question during the exam.
	var attemptQuestions []UjianPesertaSoal
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var snapshotErr error
		attemptQuestions, snapshotErr = ensureUjianAttemptQuestionsTx(tx, &up, uj)
		return snapshotErr
	}); err != nil {
		return fiber.NewError(500, "Gagal memeriksa snapshot soal")
	}
	var savedAnswers []UjianJawaban
	if err := s.db.Where("ujian_peserta_id = ?", up.ID).Find(&savedAnswers).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat jawaban ujian")
	}
	activeBeforeSave, err := activeUjianQuestionIDs(attemptQuestions, savedAnswers)
	if err != nil {
		return fiber.NewError(500, "Alur bagian ujian tidak valid")
	}
	var frozen UjianPesertaSoal
	for _, item := range attemptQuestions {
		if item.UjianSoalID == in.UjianSoalID {
			frozen = item
			break
		}
	}
	if frozen.ID == "" {
		return fiber.NewError(400, "Soal tidak ditemukan dalam ujian ini")
	}
	if !activeBeforeSave[frozen.UjianSoalID] {
		return fiber.NewError(404, "Soal berada pada bagian yang tidak aktif untuk jawaban Anda")
	}
	snapshot, err := loadUjianQuestionSnapshot(frozen.SnapshotJSON)
	if err != nil {
		return fiber.NewError(500, "Snapshot soal ujian tidak valid")
	}
	configRaw := ""
	if hasUjianVisualConfig(snapshot.Konfigurasi) {
		configJSON, marshalErr := json.Marshal(snapshot.Konfigurasi)
		if marshalErr != nil {
			return fiber.NewError(500, "Konfigurasi soal tidak valid")
		}
		configRaw = string(configJSON)
	}
	if err := validateUjianAnswer(snapshot.Tipe, snapshot.Opsi, in.Jawaban, configRaw); err != nil {
		return fiber.NewError(400, err.Error())
	}
	if snapshot.Tipe == simulasiTipeUnggah {
		fileIDs, decodeErr := decodeSimulasiFileIDs([]byte(in.Jawaban))
		if decodeErr != nil {
			return fiber.NewError(400, "referensi berkas jawaban tidak valid")
		}
		var count int64
		if err := s.db.Model(&UjianJawabanBerkas{}).Where("id IN ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", fileIDs, up.ID, frozen.UjianSoalID).Count(&count).Error; err != nil {
			return fiber.NewError(500, "Gagal memeriksa kepemilikan berkas")
		}
		if count != int64(len(fileIDs)) {
			return fiber.NewError(403, "Berkas harus berasal dari jawaban soal ini")
		}
	}
	// Upsert jawaban
	var jawaban UjianJawaban
	findAnswerErr := s.db.Where("ujian_peserta_id = ? AND soal_id = ?", up.ID, frozen.SoalID).First(&jawaban).Error
	if findAnswerErr == nil {
		jawaban.Jawaban = in.Jawaban
		if e := s.db.Model(&UjianJawaban{}).Where("id = ?", jawaban.ID).Updates(map[string]interface{}{"jawaban": jawaban.Jawaban}).Error; e != nil {
			return fiber.NewError(500, "Gagal menyimpan jawaban")
		}
	} else if errors.Is(findAnswerErr, gorm.ErrRecordNotFound) {
		jawaban = UjianJawaban{
			UjianPesertaID: up.ID,
			SoalID:         frozen.SoalID,
			Jawaban:        in.Jawaban,
		}
		if e := s.db.Create(&jawaban).Error; e != nil {
			// Concurrent tabs may both submit the first answer. Let the unique
			// index choose the row, then update that winner instead of returning
			// a transient 500 to the student.
			if !isUniqueErr(e) {
				return fiber.NewError(500, "Gagal menyimpan jawaban")
			}
			if lookupErr := s.db.Where("ujian_peserta_id = ? AND soal_id = ?", up.ID, frozen.SoalID).First(&jawaban).Error; lookupErr != nil {
				return fiber.NewError(500, "Gagal memuat jawaban ujian")
			}
			if updateErr := s.db.Model(&UjianJawaban{}).Where("id = ?", jawaban.ID).Updates(map[string]interface{}{"jawaban": in.Jawaban}).Error; updateErr != nil {
				return fiber.NewError(500, "Gagal menyimpan jawaban")
			}
		}
	} else {
		return fiber.NewError(500, "Gagal memuat jawaban ujian")
	}
	updated := false
	for index := range savedAnswers {
		if savedAnswers[index].SoalID == frozen.SoalID {
			savedAnswers[index].Jawaban = in.Jawaban
			updated = true
			break
		}
	}
	if !updated {
		savedAnswers = append(savedAnswers, UjianJawaban{UjianPesertaID: up.ID, SoalID: frozen.SoalID, Jawaban: in.Jawaban})
	}
	activeAfterSave, err := activeUjianQuestionIDs(attemptQuestions, savedAnswers)
	if err != nil {
		return fiber.NewError(500, "Alur bagian ujian tidak valid")
	}
	activeIDs := make([]string, 0, len(activeAfterSave))
	for _, question := range attemptQuestions {
		if activeAfterSave[question.UjianSoalID] {
			activeIDs = append(activeIDs, question.UjianSoalID)
		}
	}
	return c.JSON(fiber.Map{"status": "ok", "activeSoalIds": activeIDs, "hasBranching": len(snapshot.Konfigurasi.BranchToByAnswer) > 0})
}

func ujianAttemptWritable(s *Server, attempt *UjianPeserta, ujian *Ujian) error {
	if ujianResponseEditAllowed(*attempt, *ujian, time.Now()) {
		return nil
	}
	if attempt.Status != "mulai" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	if attempt.Mulai != nil && ujian.DurasiMenit > 0 && time.Now().After(batasGrace(attempt, ujian)) {
		if _, err := s.finishUjianAttempt(attempt, ujian, time.Now(), false, true, nil); err != nil {
			return fiber.NewError(500, "Gagal menghitung nilai ujian")
		}
		return fiber.NewError(403, "Waktu ujian sudah habis")
	}
	return nil
}

func (s *Server) ujianOnlineUploadAnswerFile(c *fiber.Ctx) error {
	pd, ujian, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var attempt UjianPeserta
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", ujian.ID, pd.ID).First(&attempt).Error; err != nil {
		return fiber.NewError(404, "Percobaan ujian tidak ditemukan")
	}
	if err := ujianAttemptWritable(s, &attempt, ujian); err != nil {
		return err
	}
	frozen, _, activeQuestions, err := s.loadActiveUjianAttemptQuestions(&attempt, ujian)
	if err != nil {
		return fiber.NewError(500, "Gagal memeriksa snapshot ujian")
	}
	var question *UjianPesertaSoal
	for index := range frozen {
		if frozen[index].UjianSoalID == c.Params("ujianSoalId") {
			question = &frozen[index]
			break
		}
	}
	if question == nil {
		return fiber.NewError(404, "Soal tidak ditemukan dalam percobaan ini")
	}
	if !activeQuestions[question.UjianSoalID] {
		return fiber.NewError(404, "Soal berada pada bagian ujian yang tidak aktif")
	}
	snapshot, err := loadUjianQuestionSnapshot(question.SnapshotJSON)
	if err != nil || snapshot.Tipe != simulasiTipeUnggah {
		return fiber.NewError(400, "Soal ini tidak menerima unggahan")
	}
	file, err := c.FormFile("file")
	if err != nil || file == nil {
		return fiber.NewError(400, "Pilih berkas jawaban terlebih dahulu")
	}
	allowed := normalizedFileExtensions(snapshot.Konfigurasi.AllowedFileTypes)
	if len(allowed) == 0 {
		allowed = []string{"pdf", "docx", "xlsx", "png", "jpg", "jpeg"}
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	if !containsString(allowed, extension) {
		return fiber.NewError(400, "Jenis berkas ini tidak diizinkan oleh guru")
	}
	maxFiles := snapshot.Konfigurasi.MaxFiles
	if maxFiles < 1 {
		maxFiles = 3
	}
	maxSizeMB := snapshot.Konfigurasi.MaxFileSizeMB
	if maxSizeMB < 1 {
		maxSizeMB = 10
	}
	isSubmitted := attempt.Status == "selesai" || attempt.Status == "menunggu_nilai"
	revisionKey, requestHash := "", ""
	if isSubmitted {
		revisionKey = strings.TrimSpace(c.Get("Idempotency-Key"))
		if _, err := uuid.Parse(revisionKey); err != nil {
			return fiber.NewError(400, "kunci idempotensi revisi berkas tidak valid")
		}
		opened, err := file.Open()
		if err != nil {
			return fiber.NewError(400, "berkas jawaban tidak dapat dibaca")
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, opened)
		_ = opened.Close()
		if copyErr != nil {
			return fiber.NewError(400, "berkas jawaban tidak dapat dibaca")
		}
		requestData, _ := json.Marshal(struct {
			QuestionID string `json:"questionId"`
			Filename   string `json:"filename"`
			Size       int64  `json:"size"`
			Content    string `json:"content"`
		}{question.UjianSoalID, safeSimulasiSubmittedFilename(file.Filename), file.Size, hex.EncodeToString(digest.Sum(nil))})
		requestHash = hash(string(requestData))
	}
	path, err := s.saveUpload(c, "file", "ujian-online-jawaban", int64(maxSizeMB)*1024*1024, allowed)
	if err != nil {
		return err
	}
	if path == "" {
		return fiber.NewError(400, "Berkas jawaban wajib diunggah")
	}
	answerFile := UjianJawabanBerkas{UjianPesertaID: attempt.ID, UjianSoalID: question.UjianSoalID, SoalID: question.SoalID, FilePath: path, NamaFile: safeSimulasiSubmittedFilename(file.Filename), ContentType: simulasiUploadMIME(filepath.Ext(file.Filename)), Ukuran: file.Size}
	var replayFile *UjianJawabanBerkas
	var revision UjianJawabanRevisi
	revisionCreated := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var locked UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", attempt.ID).Error; err != nil {
			return err
		}
		isRevision := locked.Status == "selesai" || locked.Status == "menunggu_nilai"
		if isRevision && !ujianResponseEditAllowed(locked, *ujian, time.Now()) {
			return fiber.NewError(409, "perubahan respons tidak diizinkan atau masa revisi telah berakhir")
		}
		if locked.Status != "mulai" && !isRevision {
			return fiber.NewError(403, "Ujian sudah selesai")
		}
		if !isRevision && locked.Mulai != nil && ujian.DurasiMenit > 0 && time.Now().After(batasGrace(&locked, ujian)) {
			return fiber.NewError(403, "Waktu ujian sudah habis")
		}
		if isRevision {
			var prior UjianJawabanRevisi
			lookupErr := tx.Where("ujian_peserta_id = ? AND idempotency_key = ?", locked.ID, revisionKey).First(&prior).Error
			if lookupErr == nil {
				if prior.UjianSoalID != question.UjianSoalID || prior.RequestHash != requestHash {
					return fiber.NewError(409, "kunci idempotensi sudah digunakan untuk perubahan lain")
				}
				before := map[string]bool{}
				oldIDs, oldErr := decodeSimulasiFileIDs([]byte(prior.JawabanSebelum))
				if oldErr == nil {
					for _, id := range oldIDs {
						before[id] = true
					}
				}
				newIDs, newErr := decodeSimulasiFileIDs([]byte(prior.JawabanSesudah))
				if newErr != nil {
					return fiber.NewError(500, "riwayat berkas revisi tidak valid")
				}
				for _, id := range newIDs {
					if !before[id] {
						var saved UjianJawabanBerkas
						if err := tx.First(&saved, "id = ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", id, locked.ID, question.UjianSoalID).Error; err != nil {
							return fiber.NewError(500, "riwayat berkas revisi tidak ditemukan")
						}
						replayFile = &saved
						break
					}
				}
				if replayFile == nil {
					return fiber.NewError(409, "riwayat idempotensi bukan unggahan berkas")
				}
				revision = prior
				return nil
			}
			if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return lookupErr
			}
		}
		var answer UjianJawaban
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ujian_peserta_id = ? AND soal_id = ?", attempt.ID, question.SoalID).First(&answer).Error
		ids := []string{}
		if findErr == nil {
			var decodeErr error
			ids, decodeErr = decodeSimulasiFileIDs([]byte(answer.Jawaban))
			if decodeErr != nil {
				return fiber.NewError(500, "Referensi berkas sebelumnya tidak valid")
			}
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if len(ids) >= maxFiles {
			return fiber.NewError(400, fmt.Sprintf("Maksimal %d berkas untuk soal ini", maxFiles))
		}
		if err := tx.Create(&answerFile).Error; err != nil {
			return err
		}
		ids = append(ids, answerFile.ID)
		encoded, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		if findErr == nil {
			answer.Jawaban = string(encoded)
		} else {
			answer = UjianJawaban{UjianPesertaID: attempt.ID, SoalID: question.SoalID, Jawaban: string(encoded)}
		}
		beforeAnswer, beforeGrading := "", ""
		if findErr == nil && isRevision {
			beforeAnswer = answer.Jawaban
			// answer.Jawaban was just replaced; recover the exact pre-edit list from
			// the IDs loaded above rather than writing the new list twice.
			oldIDs := ids[:len(ids)-1]
			oldEncoded, _ := json.Marshal(oldIDs)
			beforeAnswer = string(oldEncoded)
			beforeGrading, err = ujianJawabanPenilaianAudit(answer)
			if err != nil {
				return err
			}
			answer.Benar, answer.Nilai, answer.NilaiManual = nil, 0, nil
			answer.KomentarGuru, answer.DinilaiOlehUserID, answer.DinilaiPada = "", nil, nil
		}
		if findErr == nil {
			if err := tx.Save(&answer).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&answer).Error; err != nil {
			return err
		}
		if isRevision {
			var lockedExam Ujian
			if err := tx.First(&lockedExam, "id = ?", ujian.ID).Error; err != nil {
				return err
			}
			if !ujianResponseEditAllowed(locked, lockedExam, time.Now()) {
				return fiber.NewError(409, "perubahan respons tidak diizinkan atau masa revisi telah berakhir")
			}
			if _, gradeErr := regradeUjianAttemptAfterRevisionTx(tx, &locked, &lockedExam); gradeErr != nil {
				return gradeErr
			}
			if err := tx.First(&answer, "ujian_peserta_id = ? AND soal_id = ?", locked.ID, question.SoalID).Error; err != nil {
				return err
			}
			afterGrading, err := ujianJawabanPenilaianAudit(answer)
			if err != nil {
				return err
			}
			var revisionCount int64
			if err := tx.Model(&UjianJawabanRevisi{}).Where("ujian_peserta_id = ? AND ujian_soal_id = ?", locked.ID, question.UjianSoalID).Count(&revisionCount).Error; err != nil {
				return err
			}
			revision = UjianJawabanRevisi{UjianPesertaID: locked.ID, IdempotencyKey: revisionKey, UjianSoalID: question.UjianSoalID, Nomor: int(revisionCount) + 1, PesertaDidikID: pd.ID, AktorID: pd.ID, RequestHash: requestHash, JawabanSebelum: beforeAnswer, JawabanSesudah: answer.Jawaban, PenilaianSebelumJSON: beforeGrading, PenilaianSesudahJSON: afterGrading}
			if err := tx.Create(&revision).Error; err != nil {
				return err
			}
			revisionCreated = true
		}
		return nil
	})
	if err != nil {
		removeUpload(path)
		return err
	}
	if replayFile != nil {
		removeUpload(path)
		return c.Status(201).JSON(publicExamAnswerFile{ID: replayFile.ID, NamaFile: replayFile.NamaFile, Ukuran: replayFile.Ukuran})
	}
	if revisionCreated {
		s.audit(&pd.ID, "revise_response", "ujian_online_jawaban", revision.ID)
	}
	s.audit(&pd.ID, "upload_answer", "ujian_jawaban_berkas", answerFile.ID)
	return c.Status(201).JSON(publicExamAnswerFile{ID: answerFile.ID, NamaFile: answerFile.NamaFile, Ukuran: answerFile.Ukuran})
}

func (s *Server) ujianOnlineDeleteAnswerFile(c *fiber.Ctx) error {
	pd, ujian, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var attempt UjianPeserta
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", ujian.ID, pd.ID).First(&attempt).Error; err != nil {
		return fiber.NewError(404, "Percobaan ujian tidak ditemukan")
	}
	if err := ujianAttemptWritable(s, &attempt, ujian); err != nil {
		return err
	}
	var file UjianJawabanBerkas
	if err := s.db.Where("id = ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", c.Params("fileId"), attempt.ID, c.Params("ujianSoalId")).First(&file).Error; err != nil {
		return fiber.NewError(404, "Berkas jawaban tidak ditemukan")
	}
	_, _, activeQuestions, err := s.loadActiveUjianAttemptQuestions(&attempt, ujian)
	if err != nil {
		return fiber.NewError(500, "Gagal memeriksa alur ujian")
	}
	if !activeQuestions[file.UjianSoalID] {
		return fiber.NewError(404, "Soal berada pada bagian ujian yang tidak aktif")
	}
	if attempt.Status == "selesai" || attempt.Status == "menunggu_nilai" {
		return s.reviseSubmittedUjianFileDeletion(c, pd, ujian, &attempt, &file)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var answer UjianJawaban
		if err := tx.Where("ujian_peserta_id = ? AND soal_id = ?", attempt.ID, file.SoalID).First(&answer).Error; err != nil {
			return err
		}
		ids, err := decodeSimulasiFileIDs([]byte(answer.Jawaban))
		if err != nil {
			return err
		}
		kept := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != file.ID {
				kept = append(kept, id)
			}
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return err
		}
		if err := tx.Model(&answer).Update("jawaban", string(encoded)).Error; err != nil {
			return err
		}
		return tx.Delete(&file).Error
	}); err != nil {
		return fiber.NewError(500, "Gagal menghapus berkas jawaban")
	}
	removeUpload(file.FilePath)
	s.audit(&pd.ID, "delete_answer_upload", "ujian_jawaban_berkas", file.ID)
	return c.SendStatus(204)
}

func (s *Server) reviseSubmittedUjianFileDeletion(c *fiber.Ctx, pd *PesertaDidik, uj *Ujian, attempt *UjianPeserta, file *UjianJawabanBerkas) error {
	key := strings.TrimSpace(c.Get("Idempotency-Key"))
	if _, err := uuid.Parse(key); err != nil {
		return fiber.NewError(400, "kunci idempotensi revisi berkas tidak valid")
	}
	requestData, _ := json.Marshal(struct {
		QuestionID string `json:"questionId"`
		FileID     string `json:"fileId"`
		Action     string `json:"action"`
	}{file.UjianSoalID, file.ID, "remove"})
	requestHash := hash(string(requestData))
	var revision UjianJawabanRevisi
	created, replayed := false, false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookupErr := tx.Where("ujian_peserta_id = ? AND idempotency_key = ?", attempt.ID, key).First(&revision).Error
		if lookupErr == nil {
			if revision.UjianSoalID != file.UjianSoalID || revision.RequestHash != requestHash {
				return fiber.NewError(409, "kunci idempotensi sudah digunakan untuk perubahan lain")
			}
			replayed = true
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		var locked UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ? AND peserta_didik_id = ?", attempt.ID, pd.ID).Error; err != nil {
			return fiber.NewError(404, "percobaan ujian tidak ditemukan")
		}
		lookupErr = tx.Where("ujian_peserta_id = ? AND idempotency_key = ?", locked.ID, key).First(&revision).Error
		if lookupErr == nil {
			if revision.UjianSoalID != file.UjianSoalID || revision.RequestHash != requestHash {
				return fiber.NewError(409, "kunci idempotensi sudah digunakan untuk perubahan lain")
			}
			replayed = true
			*attempt = locked
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		var lockedExam Ujian
		if err := tx.First(&lockedExam, "id = ?", uj.ID).Error; err != nil {
			return fiber.NewError(404, "ujian tidak ditemukan")
		}
		if !ujianResponseEditAllowed(locked, lockedExam, time.Now()) {
			return fiber.NewError(409, "perubahan respons tidak diizinkan atau masa revisi telah berakhir")
		}
		questions, err := ensureUjianAttemptQuestionsTx(tx, &locked, &lockedExam)
		if err != nil {
			return err
		}
		activeAnswers := []UjianJawaban{}
		if err := tx.Where("ujian_peserta_id = ?", locked.ID).Find(&activeAnswers).Error; err != nil {
			return err
		}
		active, err := activeUjianQuestionIDs(questions, activeAnswers)
		if err != nil {
			return err
		}
		if !active[file.UjianSoalID] {
			return fiber.NewError(404, "soal berada pada bagian yang tidak aktif")
		}
		var answer UjianJawaban
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ujian_peserta_id = ? AND soal_id = ?", locked.ID, file.SoalID).First(&answer).Error; err != nil {
			return fiber.NewError(404, "jawaban berkas tidak ditemukan")
		}
		ids, err := decodeSimulasiFileIDs([]byte(answer.Jawaban))
		if err != nil || !containsString(ids, file.ID) {
			return fiber.NewError(409, "berkas ini bukan bagian dari jawaban aktif")
		}
		beforeAnswer := answer.Jawaban
		beforeGrading, err := ujianJawabanPenilaianAudit(answer)
		if err != nil {
			return err
		}
		kept := make([]string, 0, len(ids)-1)
		for _, id := range ids {
			if id != file.ID {
				kept = append(kept, id)
			}
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return err
		}
		answer.Jawaban = string(encoded)
		answer.Benar, answer.Nilai, answer.NilaiManual = nil, 0, nil
		answer.KomentarGuru, answer.DinilaiOlehUserID, answer.DinilaiPada = "", nil, nil
		if err := tx.Save(&answer).Error; err != nil {
			return err
		}
		if _, err := regradeUjianAttemptAfterRevisionTx(tx, &locked, &lockedExam); err != nil {
			return err
		}
		if err := tx.First(&answer, "id = ?", answer.ID).Error; err != nil {
			return err
		}
		afterGrading, err := ujianJawabanPenilaianAudit(answer)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&UjianJawabanRevisi{}).Where("ujian_peserta_id = ? AND ujian_soal_id = ?", locked.ID, file.UjianSoalID).Count(&count).Error; err != nil {
			return err
		}
		revision = UjianJawabanRevisi{UjianPesertaID: locked.ID, IdempotencyKey: key, UjianSoalID: file.UjianSoalID, Nomor: int(count) + 1, PesertaDidikID: pd.ID, AktorID: pd.ID, RequestHash: requestHash, JawabanSebelum: beforeAnswer, JawabanSesudah: answer.Jawaban, PenilaianSebelumJSON: beforeGrading, PenilaianSesudahJSON: afterGrading}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		*attempt = locked
		created = true
		return nil
	})
	if err != nil {
		return err
	}
	if created {
		s.audit(&pd.ID, "revise_response", "ujian_online_jawaban", revision.ID)
	}
	_ = replayed // a replay intentionally returns the same empty success response.
	return c.SendStatus(204)
}

func (s *Server) ujianOnlineDownloadAnswerFile(c *fiber.Ctx) error {
	pd, ujian, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var attempt UjianPeserta
	if err := s.db.Where("ujian_id = ? AND peserta_didik_id = ?", ujian.ID, pd.ID).First(&attempt).Error; err != nil {
		return fiber.NewError(404, "Percobaan ujian tidak ditemukan")
	}
	var file UjianJawabanBerkas
	if err := s.db.Where("id = ? AND ujian_peserta_id = ? AND ujian_soal_id = ?", c.Params("fileId"), attempt.ID, c.Params("ujianSoalId")).First(&file).Error; err != nil {
		return fiber.NewError(404, "Berkas jawaban tidak ditemukan")
	}
	_, _, activeQuestions, err := s.loadActiveUjianAttemptQuestions(&attempt, ujian)
	if err != nil {
		return fiber.NewError(500, "Gagal memeriksa alur ujian")
	}
	if !activeQuestions[file.UjianSoalID] {
		return fiber.NewError(404, "Soal berada pada bagian ujian yang tidak aktif")
	}
	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", file.NamaFile))
	return s.sendUpload(c, file.FilePath)
}

func (s *Server) ujianOnlineStaffDownloadAnswerFile(c *fiber.Ctx) error {
	var exam Ujian
	if err := s.db.First(&exam, "id = ?", c.Params("ujianId")).Error; err != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if err := s.scopeUjian(c, &exam); err != nil {
		return err
	}
	var attempt UjianPeserta
	if err := s.db.First(&attempt, "id = ? AND ujian_id = ?", c.Params("attemptId"), exam.ID).Error; err != nil {
		return fiber.NewError(404, "Percobaan tidak ditemukan")
	}
	var file UjianJawabanBerkas
	if err := s.db.Where("id = ? AND ujian_peserta_id = ?", c.Params("fileId"), attempt.ID).First(&file).Error; err != nil {
		return fiber.NewError(404, "Berkas jawaban tidak ditemukan")
	}
	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", file.NamaFile))
	return s.sendUpload(c, file.FilePath)
}

// gradeUjianPeserta auto-grades objective answers and calculates the final
// score. Essay answers are deliberately held for staff review.
type ujianGradeResult struct {
	Score         float64
	Correct       int
	Total         int
	PendingManual int
}

func (s *Server) gradeUjianPesertaResult(up *UjianPeserta, uj *Ujian) (ujianGradeResult, error) {
	var result ujianGradeResult
	err := s.db.Transaction(func(tx *gorm.DB) error { return gradeUjianPesertaTx(tx, up, uj, &result) })
	return result, err
}

func isManualUjianQuestion(tipe string) bool {
	switch strings.ToLower(strings.TrimSpace(tipe)) {
	case "essay", "uraian", "paragraf", simulasiTipeUnggah:
		return true
	default:
		return false
	}
}

func ujianAttemptNeedsManualGrade(up *UjianPeserta) bool {
	return up.Status == "menunggu_nilai" || (up.Status == "dikunci" && up.Skor == nil)
}

func gradeUjianPesertaTx(tx *gorm.DB, up *UjianPeserta, uj *Ujian, result *ujianGradeResult) error {
	questions, jawabans, activeQuestions, err := activeUjianAttemptQuestions(tx, up, uj)
	if err != nil {
		return err
	}
	answerByQuestion := make(map[string]*UjianJawaban, len(jawabans))
	for i := range jawabans {
		answerByQuestion[jawabans[i].SoalID] = &jawabans[i]
	}
	totalSkor, totalBobot := 0.0, 0.0
	activeTotal := 0
	for _, question := range questions {
		if activeQuestions[question.UjianSoalID] {
			activeTotal++
		}
	}
	*result = ujianGradeResult{Total: activeTotal}
	for _, frozen := range questions {
		if !activeQuestions[frozen.UjianSoalID] {
			continue
		}
		snapshot, err := loadUjianQuestionSnapshot(frozen.SnapshotJSON)
		if err != nil {
			return err
		}
		if frozen.Bobot < 0 || math.IsNaN(frozen.Bobot) || math.IsInf(frozen.Bobot, 0) {
			return errors.New("bobot soal tidak valid")
		}
		totalBobot += frozen.Bobot
		answer := answerByQuestion[frozen.SoalID]
		if answer == nil {
			continue
		}
		jawaban := strings.TrimSpace(answer.Jawaban)
		hasAnswer := hasAssessmentAnswer(jawaban)
		updates := map[string]interface{}{"benar": nil, "nilai": 0.0}
		if isManualUjianQuestion(snapshot.Tipe) {
			if hasAnswer && answer.NilaiManual == nil {
				result.PendingManual++
			} else if answer.NilaiManual != nil {
				if *answer.NilaiManual < 0 || *answer.NilaiManual > frozen.Bobot || math.IsNaN(*answer.NilaiManual) || math.IsInf(*answer.NilaiManual, 0) {
					return errors.New("nilai manual di luar rentang bobot soal")
				}
				answer.Nilai = *answer.NilaiManual
				totalSkor += answer.Nilai
				updates["nilai"] = answer.Nilai
			}
		} else if jawaban != "" && hasUjianVisualConfig(snapshot.Konfigurasi) {
			benar, score, manual := gradeSnapshot(simulasiSnapshot{Tipe: snapshot.Tipe, Konfigurasi: snapshot.Konfigurasi}, jawaban, frozen.Bobot)
			if manual {
				result.PendingManual++
			} else {
				answer.Benar = &benar
				answer.Nilai = score
				totalSkor += score
				if benar {
					result.Correct++
				}
				updates["benar"] = benar
				updates["nilai"] = score
			}
		} else if jawaban != "" && strings.TrimSpace(snapshot.Kunci) != "" {
			benar := bankSoalAnswerCorrect(snapshot.Tipe, snapshot.Opsi, snapshot.Kunci, jawaban)
			answer.Benar = &benar
			updates["benar"] = benar
			if benar {
				answer.Nilai = frozen.Bobot
				totalSkor += frozen.Bobot
				result.Correct++
				updates["nilai"] = frozen.Bobot
			}
		}
		if err := tx.Model(&UjianJawaban{}).Where("id = ?", answer.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	if result.PendingManual == 0 {
		if totalBobot > 0 {
			result.Score = totalSkor / totalBobot * 100
		}
	}
	return nil
}

// summarizeUjianAttemptTx produces an idempotent submission result without
// recalculating historical grades.
func summarizeUjianAttemptTx(tx *gorm.DB, up *UjianPeserta, uj *Ujian) (ujianGradeResult, error) {
	var result ujianGradeResult
	questions, answers, activeQuestions, err := activeUjianAttemptQuestions(tx, up, uj)
	if err != nil {
		return result, err
	}
	for _, question := range questions {
		if activeQuestions[question.UjianSoalID] {
			result.Total++
		}
	}
	answerByQuestion := make(map[string]UjianJawaban, len(answers))
	for _, answer := range answers {
		answerByQuestion[answer.SoalID] = answer
	}
	trackPending := ujianAttemptNeedsManualGrade(up)
	for _, question := range questions {
		if !activeQuestions[question.UjianSoalID] {
			continue
		}
		snapshot, err := loadUjianQuestionSnapshot(question.SnapshotJSON)
		if err != nil {
			return result, err
		}
		answer, ok := answerByQuestion[question.SoalID]
		if ok && answer.Benar != nil && *answer.Benar {
			result.Correct++
		}
		if trackPending && ok && isManualUjianQuestion(snapshot.Tipe) && hasAssessmentAnswer(answer.Jawaban) && answer.NilaiManual == nil {
			result.PendingManual++
		}
	}
	if up.Skor != nil {
		result.Score = *up.Skor
	}
	return result, nil
}

func (s *Server) finishUjianAttempt(up *UjianPeserta, uj *Ujian, now time.Time, locked, autoClosed bool, tabSwitch *int) (ujianGradeResult, error) {
	var result ujianGradeResult
	newlySubmitted := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ? AND ujian_id = ?", up.ID, uj.ID).Error; err != nil {
			return err
		}
		if current.Status != "mulai" {
			var summaryErr error
			result, summaryErr = summarizeUjianAttemptTx(tx, &current, uj)
			*up = current
			return summaryErr
		}
		if err := gradeUjianPesertaTx(tx, &current, uj, &result); err != nil {
			return err
		}
		status := "selesai"
		if locked {
			status = "dikunci"
		} else if result.PendingManual > 0 {
			status = "menunggu_nilai"
		}
		updates := map[string]interface{}{"status": status, "selesai": now, "penutupan_otomatis": autoClosed || locked}
		if result.PendingManual > 0 {
			updates["skor"] = nil
			current.Skor = nil
		} else {
			updates["skor"] = result.Score
			current.Skor = &result.Score
		}
		if tabSwitch != nil {
			updates["tab_switch"] = *tabSwitch
			current.TabSwitch = *tabSwitch
		}
		write := tx.Model(&UjianPeserta{}).Where("id = ? AND status = ?", current.ID, "mulai").Updates(updates)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected == 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", current.ID).Error; err != nil {
				return err
			}
			var summaryErr error
			result, summaryErr = summarizeUjianAttemptTx(tx, &current, uj)
			*up = current
			return summaryErr
		}
		current.Status, current.Selesai = status, &now
		current.PenutupanOtomatis = autoClosed || locked
		*up = current
		newlySubmitted = true
		return nil
	})
	if err == nil && newlySubmitted {
		s.audit(nil, "submit", "ujian_online", up.ID)
	}
	return result, err
}

func ujianSubmitResponse(up *UjianPeserta, grade ujianGradeResult) fiber.Map {
	var score interface{}
	if up.Skor != nil {
		score = *up.Skor
	}
	return fiber.Map{"skor": score, "benar": grade.Correct, "total": grade.Total, "status": up.Status, "menungguPenilaian": grade.PendingManual > 0, "uraianMenunggu": grade.PendingManual}
}

func (s *Server) gradeUjianPeserta(up *UjianPeserta, uj *Ujian) float64 {
	result, err := s.gradeUjianPesertaResult(up, uj)
	if err != nil {
		return 0
	}
	return result.Score
}

// batasWaktu returns the normal deadline (Mulai + DurasiMenit).
// batasGrace returns the hard deadline including grace period.
func batasWaktu(up *UjianPeserta, uj *Ujian) time.Time {
	if up.Mulai == nil || uj.DurasiMenit == 0 {
		return time.Time{}
	}
	return up.Mulai.Add(time.Duration(uj.DurasiMenit) * time.Minute)
}
func batasGrace(up *UjianPeserta, uj *Ujian) time.Time {
	b := batasWaktu(up, uj)
	if b.IsZero() {
		return b
	}
	gp := uj.GracePeriodMenit
	if gp <= 0 {
		gp = 5 // default 5 menit grace period
	}
	return b.Add(time.Duration(gp) * time.Minute)
}

// autoFinishUjianSessions runs every 30s and closes any ujian session whose
// grace period has expired. This ensures exams are graded even if the student
// closes their browser without clicking "Selesai".
func (s *Server) autoFinishUjianSessions() {
	var pesertas []UjianPeserta
	if err := s.db.Preload("Ujian").Where("status = ? AND mulai IS NOT NULL", "mulai").Find(&pesertas).Error; err != nil {
		operationLog("exam_auto_finish_query_failed", map[string]any{})
		return
	}
	now := time.Now()
	for _, up := range pesertas {
		if up.Ujian.DurasiMenit == 0 || up.Mulai == nil {
			continue
		}
		// Only auto-finish after the FULL grace period has expired
		if now.After(batasGrace(&up, &up.Ujian)) {
			if _, err := s.finishUjianAttempt(&up, &up.Ujian, now, false, true, nil); err != nil {
				operationLog("exam_auto_finish_update_failed", map[string]any{})
			}
		}
	}
}

// selesaiUjianOnline — POST /ujian-online/:ujianId/selesai {nisn, aksesKode}
// Auto-grades PG answers, computes score, returns result.
func (s *Server) validateUjianTextResponses(up *UjianPeserta, uj *Ujian) error {
	var attemptQuestions []UjianPesertaSoal
	var answers []UjianJawaban
	var activeQuestions map[string]bool
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		attemptQuestions, answers, activeQuestions, err = activeUjianAttemptQuestions(tx, up, uj)
		return err
	}); err != nil {
		return fiber.NewError(500, "Gagal memeriksa jawaban ujian")
	}
	answerByQuestion := make(map[string]UjianJawaban, len(answers))
	for _, answer := range answers {
		answerByQuestion[answer.SoalID] = answer
	}
	for _, item := range attemptQuestions {
		if !activeQuestions[item.UjianSoalID] {
			continue
		}
		snapshot, err := loadUjianQuestionSnapshot(item.SnapshotJSON)
		if err != nil {
			return fiber.NewError(500, "Snapshot soal ujian tidak valid")
		}
		if len(snapshot.Konfigurasi.BranchToByAnswer) > 0 {
			answer, exists := answerByQuestion[item.SoalID]
			if !exists || strings.TrimSpace(answer.Jawaban) == "" || strings.TrimSpace(answer.Jawaban) == "null" {
				return fiber.NewError(400, fmt.Sprintf("Soal %d: pilih jawaban untuk menentukan bagian berikutnya sebelum mengirim ujian", item.Urutan))
			}
		}
		if snapshot.Tipe != simulasiTipeIsian && snapshot.Tipe != simulasiTipeUraian {
			continue
		}
		answer, exists := answerByQuestion[item.SoalID]
		if !exists {
			continue
		}
		validationErr := validateSimulasiResponseRules(simulasiSnapshot{Tipe: snapshot.Tipe, Konfigurasi: snapshot.Konfigurasi}, answer.Jawaban)
		if validationErr != nil {
			return fiber.NewError(400, fmt.Sprintf("Soal %d: %s", item.Urutan, validationErr.Error()))
		}
	}
	return nil
}

func (s *Server) selesaiUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "menunggu_nilai" || up.Status == "dikunci" {
		grade, summaryErr := summarizeUjianAttemptTx(s.db, &up, uj)
		if summaryErr != nil {
			return fiber.NewError(500, "Gagal memuat hasil ujian")
		}
		response := ujianSubmitResponse(&up, grade)
		response["bolehEditRespons"] = ujianResponseEditAllowed(up, *uj, time.Now())
		return c.JSON(response)
	}
	now := time.Now()
	autoClosed := up.Mulai != nil && uj.DurasiMenit > 0 && now.After(batasGrace(&up, uj))
	if !autoClosed {
		if err := s.validateUjianTextResponses(&up, uj); err != nil {
			return err
		}
	}
	grade, finishErr := s.finishUjianAttempt(&up, uj, now, false, autoClosed, nil)
	if finishErr != nil {
		return fiber.NewError(500, "Gagal menghitung nilai ujian")
	}
	response := ujianSubmitResponse(&up, grade)
	response["bolehEditRespons"] = ujianResponseEditAllowed(up, *uj, time.Now())
	return c.JSON(response)
}

// tabSwitchUjianOnline — POST /ujian-online/:ujianId/tab-switch {nisn, aksesKode}
func (s *Server) tabSwitchUjianOnline(c *fiber.Ctx) error {
	pd, uj, err := s.ujianOnlineKodeAuth(c, c.Params("ujianId"))
	if err != nil {
		return err
	}
	var up UjianPeserta
	if s.db.Where("ujian_id = ? AND peserta_didik_id = ?", uj.ID, pd.ID).First(&up).Error != nil {
		return fiber.NewError(400, "Anda belum memulai ujian ini")
	}
	if up.Status == "selesai" || up.Status == "menunggu_nilai" || up.Status == "dikunci" {
		return fiber.NewError(403, "Ujian sudah selesai")
	}
	up.TabSwitch++
	// Auto-lock if batas terlampaui
	if uj.BatasTabSwitch > 0 && up.TabSwitch >= uj.BatasTabSwitch {
		now := time.Now()
		grade, finishErr := s.finishUjianAttempt(&up, uj, now, true, true, &up.TabSwitch)
		if finishErr != nil {
			return fiber.NewError(500, "Gagal menghitung nilai ujian")
		}
		response := ujianSubmitResponse(&up, grade)
		response["tabSwitch"], response["locked"] = up.TabSwitch, true
		return c.JSON(response)
	}
	if err := s.db.Model(&UjianPeserta{}).Where("id = ?", up.ID).Update("tab_switch", up.TabSwitch).Error; err != nil {
		return fiber.NewError(500, "Gagal menyimpan status ujian")
	}
	return c.JSON(fiber.Map{"tabSwitch": up.TabSwitch})
}

// ============================================================================
// Monitoring Ujian Online — protected (teacher/admin only)
// ============================================================================

// monitorUjianOnline — GET /ujian-online/monitor/:ujianId
func (s *Server) monitorUjianOnline(c *fiber.Ctx) error {
	ujianID := c.Params("ujianId")
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	var pesertas []UjianPeserta
	if err := s.db.Preload("PesertaDidik").Where("ujian_id = ?", ujianID).Find(&pesertas).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat peserta ujian")
	}
	var questions []UjianSoal
	if err := s.db.Preload("Soal").Where("ujian_id = ?", ujianID).Find(&questions).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat ringkasan soal")
	}
	attemptIDs := make([]string, 0, len(pesertas))
	pendingAttempts := make(map[string]bool, len(pesertas))
	for _, peserta := range pesertas {
		attemptIDs = append(attemptIDs, peserta.ID)
		pendingAttempts[peserta.ID] = ujianAttemptNeedsManualGrade(&peserta)
	}
	var answers []UjianJawaban
	if len(attemptIDs) > 0 {
		if err := s.db.Where("ujian_peserta_id IN ?", attemptIDs).Find(&answers).Error; err != nil {
			return fiber.NewError(500, "Gagal memuat ringkasan jawaban")
		}
	}
	var snapshots []UjianPesertaSoal
	if len(attemptIDs) > 0 {
		if err := s.db.Where("ujian_peserta_id IN ?", attemptIDs).Find(&snapshots).Error; err != nil {
			return fiber.NewError(500, "Gagal memuat snapshot soal")
		}
	}
	gradeByAttempt := make(map[string]ujianGradeResult, len(pesertas))
	legacyQuestionTypeByID := make(map[string]string, len(questions))
	for _, question := range questions {
		legacyQuestionTypeByID[question.SoalID] = question.Soal.Tipe
	}
	answersByAttempt := make(map[string][]UjianJawaban, len(pesertas))
	for _, answer := range answers {
		answersByAttempt[answer.UjianPesertaID] = append(answersByAttempt[answer.UjianPesertaID], answer)
	}
	snapshotsByAttempt := make(map[string][]UjianPesertaSoal, len(pesertas))
	for _, question := range snapshots {
		snapshotsByAttempt[question.UjianPesertaID] = append(snapshotsByAttempt[question.UjianPesertaID], question)
	}
	questionTypeByAttemptAndID := make(map[string]string, len(snapshots))
	activeSourceByAttemptAndID := make(map[string]bool, len(snapshots))
	questionTotalByAttempt := make(map[string]int, len(pesertas))
	for _, peserta := range pesertas {
		attemptQuestions := snapshotsByAttempt[peserta.ID]
		if len(attemptQuestions) == 0 {
			for _, sourceQuestion := range questions {
				activeSourceByAttemptAndID[peserta.ID+":"+sourceQuestion.SoalID] = true
			}
			questionTotalByAttempt[peserta.ID] = len(questions)
			continue
		}
		activeQuestions, err := activeUjianQuestionIDs(attemptQuestions, answersByAttempt[peserta.ID])
		if err != nil {
			return fiber.NewError(500, "Alur bagian ujian tidak valid")
		}
		for _, question := range attemptQuestions {
			if !activeQuestions[question.UjianSoalID] {
				continue
			}
			snapshot, err := loadUjianQuestionSnapshot(question.SnapshotJSON)
			if err != nil {
				return fiber.NewError(500, "Snapshot soal ujian tidak valid")
			}
			questionTypeByAttemptAndID[question.UjianPesertaID+":"+question.SoalID] = snapshot.Tipe
			activeSourceByAttemptAndID[question.UjianPesertaID+":"+question.SoalID] = true
			questionTotalByAttempt[question.UjianPesertaID]++
		}
	}
	for _, answer := range answers {
		if !activeSourceByAttemptAndID[answer.UjianPesertaID+":"+answer.SoalID] {
			continue
		}
		grade := gradeByAttempt[answer.UjianPesertaID]
		if answer.Benar != nil && *answer.Benar {
			grade.Correct++
		}
		questionType := questionTypeByAttemptAndID[answer.UjianPesertaID+":"+answer.SoalID]
		if questionType == "" {
			questionType = legacyQuestionTypeByID[answer.SoalID]
		}
		if pendingAttempts[answer.UjianPesertaID] && isManualUjianQuestion(questionType) && strings.TrimSpace(answer.Jawaban) != "" && answer.NilaiManual == nil {
			grade.PendingManual++
		}
		gradeByAttempt[answer.UjianPesertaID] = grade
	}
	result := make([]examMonitorItem, 0, len(pesertas))
	for _, peserta := range pesertas {
		grade := gradeByAttempt[peserta.ID]
		grade.Total = len(questions)
		if questionTotalByAttempt[peserta.ID] > 0 {
			grade.Total = questionTotalByAttempt[peserta.ID]
		}
		result = append(result, examMonitorItem{
			ID: peserta.ID, UjianID: peserta.UjianID, PesertaDidikID: peserta.PesertaDidikID,
			Mulai: peserta.Mulai, Selesai: peserta.Selesai, Skor: peserta.Skor,
			Status: peserta.Status, TabSwitch: peserta.TabSwitch, UraianMenunggu: grade.PendingManual,
			PesertaDidik: examMonitorStudent{ID: peserta.PesertaDidik.ID, Nama: peserta.PesertaDidik.Nama, NIS: peserta.PesertaDidik.NIS},
		})
	}
	return c.JSON(result)
}

type examReviewQuestion struct {
	UjianSoalID  string                 `json:"ujianSoalId"`
	Aktif        bool                   `json:"aktif"`
	JawabanID    string                 `json:"jawabanId,omitempty"`
	Tipe         string                 `json:"tipe"`
	Pertanyaan   string                 `json:"pertanyaan"`
	Opsi         []string               `json:"opsi,omitempty"`
	Konfigurasi  simulasiConfig         `json:"konfigurasi,omitempty"`
	Kunci        string                 `json:"kunci"`
	Bobot        float64                `json:"bobot"`
	Jawaban      string                 `json:"jawaban"`
	Nilai        float64                `json:"nilai"`
	NilaiManual  *float64               `json:"nilaiManual,omitempty"`
	KomentarGuru string                 `json:"komentarGuru"`
	DinilaiPada  *time.Time             `json:"dinilaiPada,omitempty"`
	Berkas       []publicExamAnswerFile `json:"berkas,omitempty"`
}

// getUjianAttemptReview is staff-only and intentionally separate from the
// public monitor summary, so answer keys can never leak to student endpoints.
func (s *Server) getUjianAttemptReview(c *fiber.Ctx) error {
	ujianID, attemptID := c.Params("ujianId"), c.Params("attemptId")
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if err := s.scopeUjian(c, &uj); err != nil {
		return err
	}
	var attempt UjianPeserta
	if err := s.db.Preload("PesertaDidik").First(&attempt, "id = ? AND ujian_id = ?", attemptID, ujianID).Error; err != nil {
		return fiber.NewError(404, "Percobaan tidak ditemukan")
	}
	var questions []UjianPesertaSoal
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		questions, err = loadUjianAttemptQuestionsTx(tx, &attempt, &uj, false)
		return err
	}); err != nil {
		return fiber.NewError(500, "Gagal memuat soal ujian")
	}
	var answers []UjianJawaban
	if err := s.db.Where("ujian_peserta_id = ?", attempt.ID).Find(&answers).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat jawaban peserta")
	}
	answerByQuestion := make(map[string]UjianJawaban, len(answers))
	for _, answer := range answers {
		answerByQuestion[answer.SoalID] = answer
	}
	var answerFiles []UjianJawabanBerkas
	if err := s.db.Where("ujian_peserta_id = ?", attempt.ID).Find(&answerFiles).Error; err != nil {
		return fiber.NewError(500, "Gagal memuat berkas jawaban peserta")
	}
	fileByID := make(map[string]UjianJawabanBerkas, len(answerFiles))
	for _, file := range answerFiles {
		fileByID[file.ID] = file
	}
	activeQuestions, err := activeUjianQuestionIDs(questions, answers)
	if err != nil {
		return fiber.NewError(500, "Alur bagian ujian tidak valid")
	}
	review := make([]examReviewQuestion, 0, len(questions))
	for _, question := range questions {
		snapshot, err := loadUjianQuestionSnapshot(question.SnapshotJSON)
		if err != nil {
			return fiber.NewError(500, "Snapshot soal ujian tidak valid")
		}
		row := examReviewQuestion{
			UjianSoalID: question.UjianSoalID, Aktif: activeQuestions[question.UjianSoalID], Tipe: snapshot.Tipe,
			Pertanyaan: snapshot.Pertanyaan, Kunci: snapshot.Kunci, Bobot: question.Bobot,
			Konfigurasi: snapshot.Konfigurasi,
		}
		if snapshot.Opsi != "" {
			_ = json.Unmarshal([]byte(snapshot.Opsi), &row.Opsi)
		}
		if answer, ok := answerByQuestion[question.SoalID]; ok {
			row.JawabanID, row.Jawaban, row.Nilai = answer.ID, answer.Jawaban, answer.Nilai
			row.NilaiManual, row.KomentarGuru, row.DinilaiPada = answer.NilaiManual, answer.KomentarGuru, answer.DinilaiPada
			if ids, decodeErr := decodeSimulasiFileIDs([]byte(answer.Jawaban)); decodeErr == nil {
				for _, fileID := range ids {
					if file, exists := fileByID[fileID]; exists && file.UjianSoalID == question.UjianSoalID {
						row.Berkas = append(row.Berkas, publicExamAnswerFile{ID: file.ID, NamaFile: file.NamaFile, Ukuran: file.Ukuran})
					}
				}
			}
		}
		review = append(review, row)
	}
	var grade ujianGradeResult
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		grade, err = summarizeUjianAttemptTx(tx, &attempt, &uj)
		return err
	}); err != nil {
		return fiber.NewError(500, "Gagal menghitung ringkasan penilaian")
	}
	return c.JSON(fiber.Map{
		"attempt": fiber.Map{
			"id": attempt.ID, "status": attempt.Status, "skor": attempt.Skor,
			"mulai": attempt.Mulai, "selesai": attempt.Selesai,
			"pesertaDidik": examMonitorStudent{ID: attempt.PesertaDidik.ID, Nama: attempt.PesertaDidik.Nama, NIS: attempt.PesertaDidik.NIS},
			"benar":        grade.Correct, "total": grade.Total, "uraianMenunggu": grade.PendingManual,
		},
		"soal": review,
	})
}

func (s *Server) gradeUjianOnlineAnswer(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" && role != "guru" {
		return fiber.NewError(403, "Hanya admin atau guru yang dapat menilai jawaban")
	}
	ujianID, attemptID, answerID := c.Params("ujianId"), c.Params("attemptId"), c.Params("answerId")
	var uj Ujian
	if s.db.First(&uj, "id = ?", ujianID).Error != nil {
		return fiber.NewError(404, "Ujian tidak ditemukan")
	}
	if err := s.scopeUjian(c, &uj); err != nil {
		return err
	}
	if role == "guru" && uj.DibuatOlehUserID != c.Locals("userID") && !s.assessmentCollaboratorCanGrade(assessmentModuleUjian, uj.ID, c.Locals("userID").(string)) {
		return fiber.NewError(403, "peran kolaborator ini tidak dapat menilai jawaban")
	}
	var in struct {
		Nilai    float64 `json:"nilai"`
		Komentar string  `json:"komentar"`
	}
	if err := c.BodyParser(&in); err != nil || math.IsNaN(in.Nilai) || math.IsInf(in.Nilai, 0) || in.Nilai < 0 {
		return fiber.NewError(400, "Nilai jawaban tidak valid")
	}
	in.Komentar = strings.TrimSpace(in.Komentar)
	if len([]byte(in.Komentar)) > 4000 {
		return fiber.NewError(400, "Komentar terlalu panjang")
	}
	uid, _ := c.Locals("userID").(string)
	var result ujianGradeResult
	var finalStatus string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var attempt UjianPeserta
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ? AND ujian_id = ?", attemptID, ujianID).Error; err != nil {
			return fiber.NewError(404, "Percobaan tidak ditemukan")
		}
		if attempt.Status == "mulai" {
			return fiber.NewError(409, "Ujian belum dikirim oleh peserta")
		}
		var answer UjianJawaban
		if err := tx.First(&answer, "id = ? AND ujian_peserta_id = ?", answerID, attempt.ID).Error; err != nil {
			return fiber.NewError(404, "Jawaban tidak ditemukan pada percobaan ini")
		}
		questions, _, activeQuestions, err := activeUjianAttemptQuestions(tx, &attempt, &uj)
		if err != nil {
			return err
		}
		var question *UjianPesertaSoal
		for i := range questions {
			if questions[i].SoalID == answer.SoalID {
				question = &questions[i]
				break
			}
		}
		if question == nil {
			return fiber.NewError(404, "Soal tidak ditemukan dalam ujian ini")
		}
		if !activeQuestions[question.UjianSoalID] {
			return fiber.NewError(409, "Jawaban berasal dari bagian yang dilewati dan tidak masuk penilaian")
		}
		snapshot, err := loadUjianQuestionSnapshot(question.SnapshotJSON)
		if err != nil {
			return fiber.NewError(500, "Snapshot soal ujian tidak valid")
		}
		if !isManualUjianQuestion(snapshot.Tipe) {
			return fiber.NewError(400, "Soal ini tidak memerlukan penilaian manual")
		}
		if question.Bobot < 0 || in.Nilai > question.Bobot {
			return fiber.NewError(400, "Nilai harus berada dalam rentang 0 sampai bobot soal")
		}
		now, score := time.Now(), in.Nilai
		answer.NilaiManual, answer.KomentarGuru = &score, in.Komentar
		answer.DinilaiOlehUserID, answer.DinilaiPada = &uid, &now
		if err := tx.Model(&UjianJawaban{}).Where("id = ?", answer.ID).Updates(map[string]interface{}{
			"nilai_manual": score, "komentar_guru": in.Komentar,
			"dinilai_oleh_user_id": uid, "dinilai_pada": now,
		}).Error; err != nil {
			return err
		}
		if err := gradeUjianPesertaTx(tx, &attempt, &uj, &result); err != nil {
			return err
		}
		finalStatus = attempt.Status
		if finalStatus != "dikunci" {
			if result.PendingManual > 0 {
				finalStatus = "menunggu_nilai"
			} else {
				finalStatus = "selesai"
			}
		}
		updates := map[string]interface{}{"status": finalStatus}
		if result.PendingManual > 0 {
			updates["skor"] = nil
		} else {
			updates["skor"] = result.Score
		}
		if err := tx.Model(&UjianPeserta{}).Where("id = ?", attempt.ID).Updates(updates).Error; err != nil {
			return err
		}
		s.auditTx(tx, &uid, "grade", "ujian_online_jawaban", answer.ID)
		return nil
	})
	if err != nil {
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return err
		}
		return fiber.NewError(500, "Gagal menyimpan penilaian")
	}
	var score interface{}
	if result.PendingManual == 0 {
		score = result.Score
	}
	return c.JSON(fiber.Map{"jawabanId": answerID, "nilai": in.Nilai, "komentar": in.Komentar, "skorUjian": score, "status": finalStatus, "uraianMenunggu": result.PendingManual})
}

// ============================================================================
// Notifikasi — protected (JWT)
// ============================================================================

func (s *Server) listNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var rows []Notifikasi
	if err := s.db.Where("user_id = ?", uid).Order("created_at desc").Limit(50).Find(&rows).Error; err != nil {
		return fiber.NewError(500, "gagal memuat notifikasi")
	}
	return c.JSON(notificationItems(rows))
}

func (s *Server) unreadNotifikasiCount(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var n int64
	s.db.Model(&Notifikasi{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&n)
	return c.JSON(fiber.Map{"count": n})
}

func (s *Server) bacaNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	now := time.Now()
	result := s.db.Model(&Notifikasi{}).
		Where("id = ? AND user_id = ?", c.Params("id"), uid).
		Updates(map[string]interface{}{"is_read": true, "dibaca_pada": now})
	if result.RowsAffected == 0 {
		return fiber.NewError(404, "Notifikasi tidak ditemukan")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *Server) bacaSemuaNotifikasi(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	now := time.Now()
	s.db.Model(&Notifikasi{}).
		Where("user_id = ? AND is_read = ?", uid, false).
		Updates(map[string]interface{}{"is_read": true, "dibaca_pada": now})
	return c.JSON(fiber.Map{"status": "ok"})
}

// issueNotificationStreamTicket creates a short-lived, one-time credential for
// EventSource. Browser EventSource cannot set an Authorization header, but a
// long-lived JWT in a URL would leak through browser history and proxy logs.
func (s *Server) issueNotificationStreamTicket(c *fiber.Ctx) error {
	uid, ok := c.Locals("userID").(string)
	if !ok || strings.TrimSpace(uid) == "" {
		return fiber.NewError(401, "sesi tidak valid")
	}
	ticket := uuid.NewString() + uuid.NewString()
	now := time.Now()
	s.streamTicketsMu.Lock()
	if s.streamTickets == nil {
		s.streamTickets = make(map[string]notificationStreamTicket)
	}
	for key, value := range s.streamTickets {
		if !value.ExpiresAt.After(now) {
			delete(s.streamTickets, key)
		}
	}
	s.streamTickets[ticket] = notificationStreamTicket{UserID: uid, ExpiresAt: now.Add(60 * time.Second)}
	s.streamTicketsMu.Unlock()
	return c.JSON(fiber.Map{"ticket": ticket, "expiresInSeconds": 60})
}

func (s *Server) consumeNotificationStreamTicket(ticket string) (string, bool) {
	s.streamTicketsMu.Lock()
	defer s.streamTicketsMu.Unlock()
	value, ok := s.streamTickets[ticket]
	if ok {
		delete(s.streamTickets, ticket)
	}
	return value.UserID, ok && value.ExpiresAt.After(time.Now())
}

// streamNotifikasi — Server-Sent Events: pushes new notifications to the client
// in real-time. Uses a simple polling loop that checks every 5 seconds.
// The preferred credential is the one-time ticket issued by
// /notifikasi/stream-ticket. Authorization remains supported for non-browser
// clients that can set headers; JWT query parameters are intentionally rejected.
func (s *Server) streamNotifikasi(c *fiber.Ctx) error {
	uid := ""
	if ticket := strings.TrimSpace(c.Query("ticket")); ticket != "" {
		var ok bool
		uid, ok = s.consumeNotificationStreamTicket(ticket)
		if !ok {
			return fiber.NewError(401, "ticket tidak valid atau sudah digunakan")
		}
	} else {
		if c.Query("token") != "" {
			return fiber.NewError(401, "gunakan stream ticket, bukan token pada URL")
		}
		var err error
		_, uid, _, err = s.parseAccessToken(c.Get("Authorization"))
		if err != nil {
			return fiber.NewError(401, "missing access token")
		}
		var user User
		if err := s.db.Select("id, is_active").First(&user, "id = ?", uid).Error; err != nil || !user.IsActive {
			return fiber.NewError(401, "sesi tidak lagi aktif")
		}
	}

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStream(nil, -1)
	w := c.Response().BodyWriter()

	flusher, ok := w.(interface{ Flush() })
	if !ok {
		return fiber.NewError(500, "streaming not supported")
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastCheck time.Time = time.Now()

	for {
		select {
		case <-c.Context().Done():
			return nil
		case <-ticker.C:
			var newNotifs []Notifikasi
			queryErr := s.db.Where("user_id = ? AND created_at > ?", uid, lastCheck).
				Order("created_at desc").Find(&newNotifs)
			if queryErr.Error != nil {
				operationLog("notification_stream_query_failed", map[string]any{})
			} else if len(newNotifs) > 0 {
				data, _ := json.Marshal(notificationItems(newNotifs))
				fmt.Fprintf(w, "event: notifikasi\ndata: %s\n\n", string(data))
				flusher.Flush()
			}
			// Also send unread count
			var count int64
			s.db.Model(&Notifikasi{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&count)
			fmt.Fprintf(w, "event: unread\ndata: %d\n\n", count)
			flusher.Flush()
			lastCheck = time.Now()
		}
	}
}

// ============================================================================
// Kalender Akademik — protected (JWT)
// ============================================================================

func (s *Server) listKalenderEvent(c *fiber.Ctx) error {
	q := s.db.Preload("TahunAjaran").Order("tanggal_mulai")
	if v := c.Query("tahunAjaranId"); v != "" {
		q = q.Where("tahun_ajaran_id = ?", v)
	}
	if v := c.Query("bulan"); v != "" {
		// Filter by month (YYYY-MM) using WIB calendar boundaries.
		if start, err := time.ParseInLocation("2006-01", v, wibLocation); err == nil {
			q = q.Where("tanggal_mulai >= ? AND tanggal_mulai < ?", start, start.AddDate(0, 1, 0))
		}
	}
	var rows []KalenderEvent
	if err := q.Find(&rows).Error; err != nil {
		return fiber.NewError(500, "gagal memuat kalender")
	}
	result := make([]calendarEventItem, 0, len(rows))
	for _, row := range rows {
		item := calendarEventItem{
			ID: row.ID, Judul: row.Judul, Deskripsi: row.Deskripsi, TanggalMulai: row.TanggalMulai,
			TanggalSelesai: row.TanggalSelesai, Tipe: row.Tipe, Warna: row.Warna,
			Semester: row.Semester, TahunAjaranID: row.TahunAjaranID,
		}
		if row.TahunAjaran != nil {
			item.TahunAjaran = &calendarYearItem{
				ID: row.TahunAjaran.ID, NamaTahunAjaran: row.TahunAjaran.NamaTahunAjaran,
				TanggalMulai: row.TahunAjaran.TanggalMulai, TanggalSelesai: row.TahunAjaran.TanggalSelesai,
				TanggalMulaiSemesterGenap: row.TahunAjaran.TanggalMulaiSemesterGenap,
				IsAktif:                   row.TahunAjaran.IsAktif,
			}
		}
		result = append(result, item)
	}
	return c.JSON(result)
}

func (s *Server) createKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat membuat event kalender")
	}
	uid := c.Locals("userID").(string)
	var in struct {
		Judul          string     `json:"judul"`
		Deskripsi      string     `json:"deskripsi"`
		TanggalMulai   time.Time  `json:"tanggalMulai"`
		TanggalSelesai *time.Time `json:"tanggalSelesai"`
		Tipe           string     `json:"tipe"`
		Warna          string     `json:"warna"`
		Semester       *string    `json:"semester"`
		TahunAjaranID  *string    `json:"tahunAjaranId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul == "" || in.TanggalMulai.IsZero() {
		return fiber.NewError(400, "judul dan tanggalMulai wajib diisi")
	}
	in.TanggalMulai = in.TanggalMulai.In(wibLocation)
	if in.TanggalSelesai != nil {
		value := in.TanggalSelesai.In(wibLocation)
		in.TanggalSelesai = &value
	}
	ev := KalenderEvent{
		Judul:            in.Judul,
		Deskripsi:        in.Deskripsi,
		TanggalMulai:     in.TanggalMulai,
		TanggalSelesai:   in.TanggalSelesai,
		Tipe:             in.Tipe,
		Warna:            in.Warna,
		Semester:         in.Semester,
		TahunAjaranID:    in.TahunAjaranID,
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&ev).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "kalender", ev.ID)
	return c.Status(201).JSON(ev)
}

func (s *Server) updateKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat mengubah event kalender")
	}
	uid := c.Locals("userID").(string)
	var ev KalenderEvent
	if s.db.First(&ev, "id = ?", c.Params("id")).Error != nil {
		return fiber.NewError(404, "event tidak ditemukan")
	}
	var in struct {
		Judul          string     `json:"judul"`
		Deskripsi      string     `json:"deskripsi"`
		TanggalMulai   time.Time  `json:"tanggalMulai"`
		TanggalSelesai *time.Time `json:"tanggalSelesai"`
		Tipe           string     `json:"tipe"`
		Warna          string     `json:"warna"`
		Semester       *string    `json:"semester"`
		TahunAjaranID  *string    `json:"tahunAjaranId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if !in.TanggalMulai.IsZero() {
		in.TanggalMulai = in.TanggalMulai.In(wibLocation)
	}
	if in.TanggalSelesai != nil {
		value := in.TanggalSelesai.In(wibLocation)
		in.TanggalSelesai = &value
	}
	if in.Judul != "" {
		ev.Judul = in.Judul
	}
	if in.Deskripsi != "" {
		ev.Deskripsi = in.Deskripsi
	}
	if !in.TanggalMulai.IsZero() {
		ev.TanggalMulai = in.TanggalMulai
	}
	if in.TanggalSelesai != nil {
		ev.TanggalSelesai = in.TanggalSelesai
	}
	if in.Tipe != "" {
		ev.Tipe = in.Tipe
	}
	if in.Warna != "" {
		ev.Warna = in.Warna
	}
	if e := s.db.Save(&ev).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "kalender", ev.ID)
	return c.JSON(ev)
}

func (s *Server) deleteKalenderEvent(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return fiber.NewError(403, "hanya admin yang dapat menghapus event kalender")
	}
	uid := c.Locals("userID").(string)
	if e := s.db.Delete(&KalenderEvent{}, "id = ?", c.Params("id")).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "kalender", c.Params("id"))
	return c.SendStatus(204)
}

// ============================================================================
// Portal Orang Tua — login by NIK + NISN (no JWT), then JWT session
// ============================================================================

// loginOrangTua — POST /orang-tua/login {nisn, tanggalLahir}
// Authenticates parent by NISN (child) + tanggal lahir (child), returns JWT with role=orang_tua.
func (s *Server) loginOrangTua(c *fiber.Ctx) error {
	var in struct {
		NISN           string `json:"nisn"`
		TanggalLahir   string `json:"tanggalLahir"` // format: DDMMYYYY
		TurnstileToken string `json:"cf-turnstile-response"`
	}
	if e := c.BodyParser(&in); e != nil || in.NISN == "" || in.TanggalLahir == "" {
		return fiber.NewError(400, "NISN dan tanggal lahir wajib diisi")
	}
	if e := s.requireTurnstile(c, in.TurnstileToken); e != nil {
		return e
	}
	// Parse tanggal lahir DDMMYYYY
	if len(in.TanggalLahir) != 8 {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	day, err1 := strconv.Atoi(in.TanggalLahir[0:2])
	month, err2 := strconv.Atoi(in.TanggalLahir[2:4])
	year, err3 := strconv.Atoi(in.TanggalLahir[4:8])
	if err1 != nil || err2 != nil || err3 != nil || day < 1 || day > 31 || month < 1 || month > 12 || year < 1900 {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	tl := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if tl.Day() != day || int(tl.Month()) != month || tl.Year() != year {
		return fiber.NewError(400, "Format tanggal lahir tidak valid (DDMMYYYY)")
	}
	// Find student by NISN
	var pd PesertaDidik
	if in.NISN == temporaryNISN {
		return fiber.NewError(401, "NISN sementara belum dapat digunakan untuk portal orang tua")
	}
	if s.db.Preload("OrangTua").Preload("Kelas").Where("nisn = ? AND status = ?", in.NISN, "aktif").First(&pd).Error != nil {
		return fiber.NewError(401, "NISN tidak ditemukan")
	}
	// Verify tanggal lahir matches
	if pd.TanggalLahir == nil {
		return fiber.NewError(401, "Data tanggal lahir siswa belum diisi. Hubungi admin sekolah.")
	}
	if pd.TanggalLahir.Year() != tl.Year() || pd.TanggalLahir.Month() != tl.Month() || pd.TanggalLahir.Day() != tl.Day() {
		return fiber.NewError(401, "Tanggal lahir tidak cocok")
	}
	// Check parent exists
	ortu := pd.OrangTua
	if ortu.ID == "" {
		return fiber.NewError(401, "Peserta didik tidak memiliki data orang tua")
	}
	// Find or create user account for this orang tua
	var u User
	if s.db.Where("orang_tua_id = ? AND role = ?", ortu.ID, "orang_tua").First(&u).Error != nil {
		// Auto-create user account
		// Use the stable parent record ID instead of a name-derived username;
		// two families may legitimately have the same mother's name.
		username := "ortu-" + ortu.ID
		// Parent access is provisioned through NISN + date of birth. Give the
		// backing User a random unusable password so a leaked username cannot be
		// converted into a second login path using a shared default credential.
		passwordHash, hashErr := bcryptHash(uuid.NewString() + uuid.NewString())
		if hashErr != nil {
			return fiber.NewError(500, "gagal menyiapkan akun orang tua")
		}
		u = User{
			Username:     username,
			Email:        username + "@pkbm.local",
			PasswordHash: passwordHash,
			Role:         "orang_tua",
			OrangTuaID:   &ortu.ID,
			IsActive:     true,
		}
		if err := s.db.Create(&u).Error; err != nil {
			return fiber.NewError(500, "gagal membuat akun orang tua")
		}
	}
	if !u.IsActive {
		return fiber.NewError(403, "Akun orang tua nonaktif")
	}
	access, e := s.token(u, s.cfg.AccessSecret, s.cfg.AccessTTL)
	if e != nil {
		return fiber.NewError(500, "gagal membuat token")
	}
	raw := uuid.NewString() + uuid.NewString()
	if err := s.db.Create(&RefreshToken{UserID: u.ID, TokenHash: hash(raw), ExpiresAt: time.Now().Add(s.cfg.RefreshTTL)}).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan sesi")
	}
	c.Cookie(&fiber.Cookie{
		Name: "refresh_token", Value: raw, HTTPOnly: true,
		Secure: s.cfg.Env == "production", SameSite: "Strict", Domain: s.cfg.CookieDomain,
		Expires: time.Now().Add(s.cfg.RefreshTTL), Path: "/api/auth",
	})
	s.audit(&u.ID, "login", "orang_tua", "")
	return c.JSON(fiber.Map{
		"accessToken": access,
		"user":        u,
	})
}

// listAnakOrangTua — GET /orang-tua/anak
func (s *Server) listAnakOrangTua(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil {
		return fiber.NewError(401, "unauthorized")
	}
	if u.Role != "orang_tua" || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var anak []PesertaDidik
	if err := s.db.Preload("Kelas").Preload("Kelas.Pokjar").Where("orang_tua_id = ?", *u.OrangTuaID).Find(&anak).Error; err != nil {
		return fiber.NewError(500, "gagal memuat daftar anak")
	}
	result := make([]parentChildSummary, 0, len(anak))
	for _, child := range anak {
		row := parentChildSummary{
			ID: child.ID, Nama: child.Nama, JenisKelamin: child.JenisKelamin,
			NIS: child.NIS, NISN: child.NISN, Status: child.Status,
			IdentitasFileExt: child.IdentitasFileExt,
			Kelas:            parentClassSummary{ID: child.Kelas.ID, Jenjang: child.Kelas.Jenjang, NamaRombel: child.Kelas.NamaRombel},
		}
		row.Kelas.Pokjar.ID = child.Kelas.Pokjar.ID
		row.Kelas.Pokjar.NamaPokjar = child.Kelas.Pokjar.NamaPokjar
		result = append(result, row)
	}
	return c.JSON(result)
}

// getNilaiAnak — GET /orang-tua/anak/:id/nilai
func (s *Server) getNilaiAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	// Verify this child belongs to this parent
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	var rekap []RekapNilaiAkhir
	if err := s.db.Where("peserta_didik_id = ?", anakID).Find(&rekap).Error; err != nil {
		return fiber.NewError(500, "gagal memuat nilai anak")
	}
	result, err := loadParentGradeItems(s.db, rekap)
	if err != nil {
		return fiber.NewError(500, "gagal memuat mata pelajaran nilai anak")
	}
	return c.JSON(result)
}

// getPresensiAnak — GET /orang-tua/anak/:id/presensi
func (s *Server) getPresensiAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	var details []PresensiDetail
	if err := s.db.
		Where("peserta_didik_id = ?", anakID).
		Order("created_at desc").
		Limit(100).
		Find(&details).Error; err != nil {
		return fiber.NewError(500, "gagal memuat presensi anak")
	}
	presensiIDs := make([]string, 0, len(details))
	for _, detail := range details {
		if detail.PresensiID != "" {
			presensiIDs = append(presensiIDs, detail.PresensiID)
		}
	}
	presensiByID := make(map[string]Presensi, len(presensiIDs))
	if len(presensiIDs) > 0 {
		var presensi []Presensi
		if err := s.db.Select("id, tanggal, semester").Where("id IN ?", presensiIDs).Find(&presensi).Error; err != nil {
			return fiber.NewError(500, "gagal memuat tanggal presensi anak")
		}
		for _, row := range presensi {
			presensiByID[row.ID] = row
		}
	}
	result := make([]parentPresensiItem, 0, len(details))
	for _, detail := range details {
		var tanggal *time.Time
		semester := ""
		if presensi, ok := presensiByID[detail.PresensiID]; ok {
			tanggal = &presensi.Tanggal
			semester = presensi.Semester
		}
		result = append(result, parentPresensiItem{
			ID: detail.ID, PresensiID: detail.PresensiID, StatusKehadiran: detail.StatusKehadiran,
			Catatan: detail.Catatan, Tanggal: tanggal, Semester: semester,
		})
	}
	return c.JSON(result)
}

// getRaporAnak — GET /orang-tua/anak/:id/rapor
func (s *Server) getRaporAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	var rekap []RekapNilaiAkhir
	if err := s.db.Where("peserta_didik_id = ?", anakID).Find(&rekap).Error; err != nil {
		return fiber.NewError(500, "gagal memuat rapor anak")
	}
	grades, err := loadParentGradeItems(s.db, rekap)
	if err != nil {
		return fiber.NewError(500, "gagal memuat mata pelajaran rapor anak")
	}
	var catatan CatatanRapor
	var note *parentReportNote
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("created_at desc").First(&catatan).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(500, "gagal memuat catatan rapor anak")
		}
	} else {
		note = &parentReportNote{
			ID: catatan.ID, PesertaDidikID: catatan.PesertaDidikID, TahunAjaranID: catatan.TahunAjaranID,
			Semester: catatan.Semester, CatatanWali: catatan.CatatanWali, NaikKelas: catatan.NaikKelas,
			KenaikanKe: catatan.KenaikanKe, CreatedAt: catatan.CreatedAt,
		}
	}
	return c.JSON(parentReportResponse{RekapNilai: grades, CatatanRapor: note})
}

// --- Helper: verify child belongs to parent ---
func (s *Server) verifyOrangTuaAnak(c *fiber.Ctx, anakID string) (*User, error) {
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.OrangTuaID == nil {
		return nil, fiber.NewError(403, "akun ini bukan akun orang tua")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ? AND orang_tua_id = ?", anakID, *u.OrangTuaID).Error != nil {
		return nil, fiber.NewError(403, "anak tidak terdaftar di bawah akun ini")
	}
	return &u, nil
}

// getUjianSkorAnak — GET /orang-tua/anak/:id/ujian-skor
func (s *Server) getUjianSkorAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pesertas []UjianPeserta
	if err := s.db.Preload("Ujian").Preload("Ujian.Mapel").
		Where("peserta_didik_id = ? AND status IN ?", anakID, []string{"selesai", "menunggu_nilai", "dikunci"}).
		Order("created_at desc").
		Find(&pesertas).Error; err != nil {
		return fiber.NewError(500, "gagal memuat hasil ujian anak")
	}
	// UjianPeserta.Ujian contains AksesKode. Parents only need the result
	// summary; never serialize the persistence graph into the portal response.
	result := make([]publicParentExamResult, 0, len(pesertas))
	for _, peserta := range pesertas {
		result = append(result, publicParentExamResult{
			ID: peserta.ID, UjianID: peserta.UjianID, Mulai: peserta.Mulai,
			Selesai: peserta.Selesai, Skor: peserta.Skor, Status: peserta.Status,
			Ujian: publicExamMeta{
				ID: peserta.Ujian.ID, Judul: peserta.Ujian.Judul,
				WaktuMulai: peserta.Ujian.WaktuMulai, WaktuSelesai: peserta.Ujian.WaktuSelesai,
				Mapel: publicExamMapel{ID: peserta.Ujian.Mapel.ID, NamaMapel: peserta.Ujian.Mapel.NamaMapel, KodeMapel: peserta.Ujian.Mapel.KodeMapel},
			},
		})
	}
	return c.JSON(result)
}

type parentSimulasiAttempt struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Nomor        int        `json:"nomor"`
	Mulai        *time.Time `json:"mulai,omitempty"`
	Selesai      *time.Time `json:"selesai,omitempty"`
	Skor         *float64   `json:"skor,omitempty"`
	SkorTersedia bool       `json:"skorTersedia"`
}

type parentSimulasiSummary struct {
	ID                 string                  `json:"id"`
	Nama               string                  `json:"nama"`
	Deskripsi          string                  `json:"deskripsi"`
	Mode               string                  `json:"mode"`
	DurasiMenit        int                     `json:"durasiMenit"`
	MaksPercobaan      int                     `json:"maksPercobaan"`
	WaktuMulai         *time.Time              `json:"waktuMulai,omitempty"`
	WaktuSelesai       *time.Time              `json:"waktuSelesai,omitempty"`
	TampilkanNilai     bool                    `json:"tampilkanNilai"`
	TampilkanRingkasan bool                    `json:"tampilkanRingkasan"`
	Tersedia           bool                    `json:"tersedia"`
	PercobaanTerpakai  int                     `json:"percobaanTerpakai"`
	Percobaan          []parentSimulasiAttempt `json:"percobaan"`
}

// getSimulasiAnak returns only assignment/result summaries. It deliberately
// never serializes questions, snapshots, answer keys, rubrics, or teacher
// comments to the parent portal.
func (s *Server) getSimulasiAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var assignments []SimulasiPenugasan
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("created_at desc").Find(&assignments).Error; err != nil {
		return fiber.NewError(500, "gagal memuat penugasan simulasi anak")
	}
	if len(assignments) == 0 {
		return c.JSON([]parentSimulasiSummary{})
	}
	packageIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		packageIDs = append(packageIDs, assignment.PaketID)
	}
	var packages []SimulasiPaket
	if err := s.db.Where("id IN ? AND status = ?", packageIDs, "terbit").Find(&packages).Error; err != nil {
		return fiber.NewError(500, "gagal memuat paket simulasi anak")
	}
	packagesByID := make(map[string]SimulasiPaket, len(packages))
	for _, paket := range packages {
		packagesByID[paket.ID] = paket
	}
	var attempts []SimulasiUpaya
	if err := s.db.Where("peserta_didik_id = ? AND paket_id IN ?", anakID, packageIDs).Order("nomor desc").Find(&attempts).Error; err != nil {
		return fiber.NewError(500, "gagal memuat riwayat simulasi anak")
	}
	attemptsByPackage := make(map[string][]SimulasiUpaya)
	for _, attempt := range attempts {
		attemptsByPackage[attempt.PaketID] = append(attemptsByPackage[attempt.PaketID], attempt)
	}
	now := time.Now()
	result := make([]parentSimulasiSummary, 0, len(assignments))
	seen := make(map[string]bool)
	for _, assignment := range assignments {
		paket, ok := packagesByID[assignment.PaketID]
		if !ok || seen[paket.ID] {
			continue
		}
		seen[paket.ID] = true
		tersedia := (paket.WaktuMulai == nil || !now.Before(*paket.WaktuMulai)) && (paket.WaktuSelesai == nil || !now.After(*paket.WaktuSelesai))
		rows := attemptsByPackage[paket.ID]
		entry := parentSimulasiSummary{
			ID: paket.ID, Nama: paket.Nama, Deskripsi: paket.Deskripsi, Mode: paket.Mode,
			DurasiMenit: paket.DurasiMenit, MaksPercobaan: paket.MaksPercobaan,
			WaktuMulai: paket.WaktuMulai, WaktuSelesai: paket.WaktuSelesai,
			TampilkanNilai: paket.TampilkanNilai, TampilkanRingkasan: paket.TampilkanRingkasan,
			Tersedia: tersedia, PercobaanTerpakai: len(rows), Percobaan: make([]parentSimulasiAttempt, 0, len(rows)),
		}
		for _, attempt := range rows {
			row := parentSimulasiAttempt{ID: attempt.ID, Status: attempt.Status, Nomor: attempt.Nomor, Mulai: attempt.Mulai, Selesai: attempt.Selesai, SkorTersedia: paket.TampilkanNilai}
			if paket.TampilkanNilai && attempt.SkorAkhir != nil {
				score := *attempt.SkorAkhir
				row.Skor = &score
			}
			entry.Percobaan = append(entry.Percobaan, row)
		}
		result = append(result, entry)
	}
	return c.JSON(result)
}

// getTugasAnak — GET /orang-tua/anak/:id/tugas
func (s *Server) getTugasAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pd PesertaDidik
	if err := s.db.First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	var tugasList []Tugas
	if err := s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("deadline desc").Find(&tugasList).Error; err != nil {
		return fiber.NewError(500, "gagal memuat tugas anak")
	}
	// Get all submissions for this student in one query; the old loop caused an
	// extra database round-trip for every task in the parent portal.
	tugasIDs := make([]string, 0, len(tugasList))
	for _, tugas := range tugasList {
		tugasIDs = append(tugasIDs, tugas.ID)
	}
	var submissions []PengumpulanTugas
	if len(tugasIDs) > 0 {
		if err := s.db.Where("peserta_didik_id = ? AND tugas_id IN ?", anakID, tugasIDs).Find(&submissions).Error; err != nil {
			return fiber.NewError(500, "gagal memuat pengumpulan tugas")
		}
	}
	submissionByTask := make(map[string]PengumpulanTugas, len(submissions))
	for _, submission := range submissions {
		submissionByTask[submission.TugasID] = submission
	}
	result := make([]parentTugasItem, 0, len(tugasList))
	for _, t := range tugasList {
		tr := parentTugasItem{
			ID: t.ID, MapelID: t.MapelID, KelasID: t.KelasID, Judul: t.Judul,
			Deskripsi: t.Deskripsi, Deadline: t.Deadline, Semester: t.Semester,
			BolehUpload: t.BolehUpload, ModulID: t.ModulID, CreatedAt: t.CreatedAt,
			Mapel:             parentMapelSummary{ID: t.Mapel.ID, NamaMapel: t.Mapel.NamaMapel, KodeMapel: t.Mapel.KodeMapel},
			StatusPengumpulan: "belum",
		}
		if pk, ok := submissionByTask[t.ID]; ok {
			tr.StatusPengumpulan = pk.Status
			tr.Nilai = pk.Nilai
			tr.TanggalKumpul = &pk.TanggalKumpul
		}
		result = append(result, tr)
	}
	return c.JSON(result)
}

// getMateriAnak — GET /orang-tua/anak/:id/materi
func (s *Server) getMateriAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var pd PesertaDidik
	if err := s.db.First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	var materiList []Materi
	if err := s.db.Preload("Mapel").Where("kelas_id = ?", pd.KelasID).Order("created_at desc").Find(&materiList).Error; err != nil {
		return fiber.NewError(500, "gagal memuat materi anak")
	}
	result := make([]parentMateriItem, 0, len(materiList))
	for _, m := range materiList {
		result = append(result, parentMateriItem{
			ID: m.ID, MapelID: m.MapelID, KelasID: m.KelasID, Judul: m.Judul,
			Deskripsi: m.Deskripsi, Tipe: m.Tipe, Ukuran: m.Ukuran, Semester: m.Semester,
			ModulID: m.ModulID, Urutan: m.Urutan, Tanggal: m.Tanggal, LinkURL: m.LinkURL,
			CreatedAt: m.CreatedAt,
			Mapel:     parentMapelSummary{ID: m.Mapel.ID, NamaMapel: m.Mapel.NamaMapel, KodeMapel: m.Mapel.KodeMapel},
		})
	}
	return c.JSON(result)
}

// getPeminjamanAnak — GET /orang-tua/anak/:id/peminjaman
func (s *Server) getPeminjamanAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var peminjaman []Peminjaman
	if err := s.db.Preload("Buku").Where("peserta_didik_id = ?", anakID).Order("created_at desc").Find(&peminjaman).Error; err != nil {
		return fiber.NewError(500, "gagal memuat riwayat buku anak")
	}
	result := make([]parentPeminjamanItem, 0, len(peminjaman))
	for _, row := range peminjaman {
		item := parentPeminjamanItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, BukuID: row.BukuID, KelasID: row.KelasID,
			Semester: row.Semester, TanggalPinjam: row.TanggalPinjam, Status: row.Status,
		}
		item.Buku.ID = row.Buku.ID
		item.Buku.Judul = row.Buku.Judul
		item.Buku.KodeBuku = row.Buku.KodeBuku
		item.Buku.Penerbit = row.Buku.Penerbit
		result = append(result, item)
	}
	return c.JSON(result)
}

// listChatAnak — GET /orang-tua/anak/:id/chat
func (s *Server) listChatAnak(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var messages []ChatMessage
	if err := s.db.Where("(pengirim_user_id = ? OR penerima_user_id = ?) AND peserta_didik_id = ?", uid, uid, anakID).
		Order("created_at asc").
		Limit(200).
		Find(&messages).Error; err != nil {
		return fiber.NewError(500, "gagal memuat percakapan")
	}
	result := make([]parentChatItem, 0, len(messages))
	for _, message := range messages {
		result = append(result, parentChatItem{
			ID: message.ID, Isi: message.Isi, Dibaca: message.Dibaca,
			DibacaPada: message.DibacaPada, CreatedAt: message.CreatedAt,
			IsMine: message.PengirimUserID == uid,
		})
	}
	return c.JSON(result)
}

// sendChatAnak — POST /orang-tua/anak/:id/chat {isi}
func (s *Server) sendChatAnak(c *fiber.Ctx) error {
	const maxParentChatMessageBytes = 16 * 1024
	uid := c.Locals("userID").(string)
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var in struct {
		Isi string `json:"isi"`
	}
	if e := c.BodyParser(&in); e != nil || strings.TrimSpace(in.Isi) == "" {
		return fiber.NewError(400, "pesan wajib diisi")
	}
	in.Isi = strings.TrimSpace(in.Isi)
	if len([]byte(in.Isi)) > maxParentChatMessageBytes {
		return fiber.NewError(413, "pesan terlalu panjang")
	}
	// Find wali kelas as recipient
	var pd PesertaDidik
	if err := s.db.Preload("Kelas").First(&pd, "id = ?", anakID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat data anak")
	}
	if pd.KelasID == "" {
		return fiber.NewError(400, "siswa tidak memiliki kelas")
	}
	var kelas Kelas
	if err := s.db.First(&kelas, "id = ?", pd.KelasID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat kelas anak")
	}
	if kelas.WaliKelasID == nil {
		return fiber.NewError(400, "kelas belum memiliki wali kelas")
	}
	var tutor Tutor
	if err := s.db.First(&tutor, "id = ?", *kelas.WaliKelasID).Error; err != nil {
		return fiber.NewError(500, "gagal memuat wali kelas")
	}
	if tutor.UserID == nil {
		return fiber.NewError(400, "wali kelas belum memiliki akun")
	}
	msg := ChatMessage{
		PesertaDidikID: anakID,
		PengirimUserID: uid,
		PenerimaUserID: *tutor.UserID,
		Isi:            in.Isi,
	}
	if err := s.db.Create(&msg).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan pesan")
	}
	// Push notifikasi to guru wali
	s.pushNotifikasi(*tutor.UserID, "Pesan dari Orang Tua", msg.Isi, "chat", &msg.ID)
	return c.Status(201).JSON(msg)
}

// getPerilakuAnak — GET /orang-tua/anak/:id/perilaku
func (s *Server) getPerilakuAnak(c *fiber.Ctx) error {
	anakID := c.Params("id")
	if _, err := s.verifyOrangTuaAnak(c, anakID); err != nil {
		return err
	}
	var catatan []CatatanPerilaku
	if err := s.db.Where("peserta_didik_id = ?", anakID).Order("tanggal desc").Find(&catatan).Error; err != nil {
		return fiber.NewError(500, "gagal memuat catatan perilaku anak")
	}
	result := make([]parentBehaviorItem, 0, len(catatan))
	for _, row := range catatan {
		result = append(result, parentBehaviorItem{
			ID: row.ID, PesertaDidikID: row.PesertaDidikID, KelasID: row.KelasID,
			Tanggal: row.Tanggal, Kategori: row.Kategori, Deskripsi: row.Deskripsi,
		})
	}
	return c.JSON(result)
}

// ============================================================================
// Ujian/AksesKode — update createUjian to support aksesKode
// ============================================================================

// Helper to notify users of new ujian (for Notifikasi system)
func (s *Server) notifyNewUjian(uj *Ujian) {
	// Find all users in the same class
	var kelas Kelas
	s.db.First(&kelas, "id = ?", uj.KelasID)
	// Notify tutor wali kelas
	if kelas.WaliKelasID != nil {
		var tutor Tutor
		s.db.First(&tutor, "id = ?", *kelas.WaliKelasID)
		if tutor.UserID != nil {
			s.db.Create(&Notifikasi{
				UserID: *tutor.UserID,
				Judul:  "Ujian Online Baru",
				Isi:    fmt.Sprintf("Ujian \"%s\" telah dibuat untuk kelas %d%s", uj.Judul, kelas.Jenjang, kelas.NamaRombel),
				Tipe:   "ujian",
				RefID:  &uj.ID,
			})
		}
	}
}

// Helper to send push notification (placeholder for future WebSocket/SSE)
func (s *Server) pushNotifikasi(userID string, judul, isi, tipe string, refID *string) {
	s.db.Create(&Notifikasi{
		UserID: userID,
		Judul:  judul,
		Isi:    isi,
		Tipe:   tipe,
		RefID:  refID,
	})
}
