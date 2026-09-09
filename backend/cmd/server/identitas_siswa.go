package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	identitasSiswaZipMaxBytes   = 256 * 1024 * 1024
	identitasSiswaFileMaxBytes  = 20 * 1024 * 1024
	identitasSiswaZipMaxEntries = 1000
)

type identitasExtractedFile struct {
	NISN string
	Path string
	Ext  string
}

type identitasSkippedFile struct {
	FileName string `json:"fileName"`
	NISN     string `json:"nisn,omitempty"`
	Reason   string `json:"reason"`
}

func validIdentitasNISN(nisn string) bool {
	if len(nisn) != 10 || nisn == temporaryNISN {
		return false
	}
	for _, char := range nisn {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validIdentitasExtension(ext string) bool {
	switch ext {
	case "jpg", "jpeg", "png", "pdf":
		return true
	default:
		return false
	}
}

func validateIdentitasFileContent(ext string, data []byte) error {
	if ext == "pdf" {
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return fmt.Errorf("bukan PDF yang valid")
		}
		return nil
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("bukan gambar yang valid")
	}
	if (ext == "png" && format != "png") || ((ext == "jpg" || ext == "jpeg") && format != "jpeg") {
		return fmt.Errorf("isi file tidak sesuai ekstensi")
	}
	return nil
}

// identitasTargetsForClass maps only unambiguous, non-placeholder NISNs. A
// duplicate NISN in the selected class is reported as skipped rather than
// risking that a private document is attached to the wrong student.
func (s *Server) identitasTargetsForClass(kelasID string) (map[string]PesertaDidik, map[string]bool, error) {
	var kelas Kelas
	if err := s.db.First(&kelas, "id = ?", kelasID).Error; err != nil {
		return nil, nil, fiber.NewError(404, "kelas tidak ditemukan")
	}
	var students []PesertaDidik
	if err := s.db.Where("kelas_id = ?", kelasID).Find(&students).Error; err != nil {
		return nil, nil, err
	}
	targets := make(map[string]PesertaDidik, len(students))
	duplicates := make(map[string]bool)
	for _, student := range students {
		if !validIdentitasNISN(student.NISN) {
			continue
		}
		if _, exists := targets[student.NISN]; exists {
			duplicates[student.NISN] = true
			delete(targets, student.NISN)
			continue
		}
		if !duplicates[student.NISN] {
			targets[student.NISN] = student
		}
	}
	return targets, duplicates, nil
}

func identityZipFileData(entry *zip.File, name, ext string) ([]byte, error) {
	if entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > identitasSiswaFileMaxBytes {
		return nil, fiber.NewError(400, fmt.Sprintf("file %s harus berukuran 1 byte sampai 20 MB", name))
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, fiber.NewError(400, fmt.Sprintf("file %s tidak dapat dibuka", name))
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, identitasSiswaFileMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > identitasSiswaFileMaxBytes {
		return nil, fiber.NewError(400, fmt.Sprintf("file %s tidak dapat dibaca atau melebihi 20 MB", name))
	}
	if err := validateIdentitasFileContent(ext, data); err != nil {
		return nil, fiber.NewError(400, fmt.Sprintf("file %s %s", name, err.Error()))
	}
	return data, nil
}

func (s *Server) extractIdentitasSiswaZip(zipPath string, targets map[string]PesertaDidik, duplicateTargets map[string]bool) ([]identitasExtractedFile, []string, []identitasSkippedFile, error) {
	reader, err := zip.OpenReader("./" + zipPath)
	if err != nil {
		return nil, nil, nil, fiber.NewError(400, "file ZIP tidak dapat dibaca")
	}
	defer reader.Close()
	if len(reader.File) == 0 {
		return nil, nil, nil, fiber.NewError(400, "file ZIP kosong")
	}

	created := make([]identitasExtractedFile, 0, len(reader.File))
	cleanup := func() {
		for _, file := range created {
			removeUpload(file.Path)
		}
	}

	seen := make(map[string]bool)
	unmatchedSet := make(map[string]bool)
	unmatched := make([]string, 0)
	skipped := make([]identitasSkippedFile, 0)
	entryCount := 0
	var totalBytes uint64
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		entryCount++
		if entryCount > identitasSiswaZipMaxEntries {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, "ZIP berisi lebih dari 1000 file")
		}
		if strings.ContainsAny(entry.Name, "/\\") {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, "nama file ZIP tidak boleh memakai folder")
		}
		name := filepath.Base(entry.Name)
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if !validIdentitasExtension(ext) {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, fmt.Sprintf("ekstensi file %s tidak diizinkan", filepath.Ext(name)))
		}
		nisn := strings.TrimSuffix(name, filepath.Ext(name))
		if !validIdentitasNISN(nisn) {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, fmt.Sprintf("nama file %s harus memakai NISN 10 digit", name))
		}
		if seen[nisn] {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, fmt.Sprintf("NISN %s muncul lebih dari sekali", nisn))
		}
		seen[nisn] = true
		if entry.UncompressedSize64 > identitasSiswaFileMaxBytes || totalBytes+entry.UncompressedSize64 > identitasSiswaZipMaxBytes {
			cleanup()
			return nil, nil, nil, fiber.NewError(400, "total isi ZIP melebihi batas 256 MB atau ada file melebihi 20 MB")
		}
		totalBytes += entry.UncompressedSize64
		data, dataErr := identityZipFileData(entry, name, ext)
		if dataErr != nil {
			cleanup()
			return nil, nil, nil, dataErr
		}
		if duplicateTargets[nisn] {
			skipped = append(skipped, identitasSkippedFile{FileName: name, NISN: nisn, Reason: "NISN duplikat pada data siswa kelas"})
			continue
		}
		if _, ok := targets[nisn]; !ok {
			if !unmatchedSet[nisn] {
				unmatchedSet[nisn] = true
				unmatched = append(unmatched, nisn)
			}
			skipped = append(skipped, identitasSkippedFile{FileName: name, NISN: nisn, Reason: "NISN tidak ditemukan di kelas terpilih"})
			continue
		}
		if err := os.MkdirAll("./uploads/identitas-siswa", 0o750); err != nil {
			cleanup()
			return nil, nil, nil, fiber.NewError(500, "direktori identitas siswa tidak dapat dibuat")
		}
		path := "uploads/identitas-siswa/" + uuid.NewString() + "." + ext
		if err := os.WriteFile("./"+path, data, 0o640); err != nil {
			cleanup()
			return nil, nil, nil, fiber.NewError(500, "file identitas tidak dapat disimpan")
		}
		created = append(created, identitasExtractedFile{NISN: nisn, Path: path, Ext: ext})
	}
	if entryCount == 0 {
		return nil, nil, nil, fiber.NewError(400, "file ZIP kosong")
	}
	sort.Strings(unmatched)
	return created, unmatched, skipped, nil
}

// uploadIdentitasSiswaZip accepts a best-effort class ZIP: invalid archive
// content rejects the request, but valid files whose NISNs are not in the class
// are skipped and returned in the summary.
func (s *Server) uploadIdentitasSiswaZip(c *fiber.Ctx) error {
	kelasID := strings.TrimSpace(c.FormValue("kelasId"))
	if kelasID == "" {
		return fiber.NewError(400, "kelas wajib dipilih")
	}
	targets, duplicateTargets, err := s.identitasTargetsForClass(kelasID)
	if err != nil {
		return err
	}
	zipPath, err := s.saveUpload(c, "file", "identitas-siswa-tmp", identitasSiswaZipMaxBytes, []string{"zip"})
	if err != nil {
		return err
	}
	if zipPath == "" {
		return fiber.NewError(400, "file ZIP wajib diunggah")
	}
	defer removeUpload(zipPath)

	extracted, unmatched, skipped, err := s.extractIdentitasSiswaZip(zipPath, targets, duplicateTargets)
	if err != nil {
		return err
	}
	keepNewFiles := false
	defer func() {
		if !keepNewFiles {
			for _, file := range extracted {
				removeUpload(file.Path)
			}
		}
	}()

	oldPaths := make(map[string]bool)
	replacedCount := 0
	uid := c.Locals("userID").(string)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, file := range extracted {
			student, ok := targets[file.NISN]
			if !ok {
				return fmt.Errorf("target identitas berubah")
			}
			if student.IdentitasFilePath != nil && *student.IdentitasFilePath != "" {
				oldPaths[*student.IdentitasFilePath] = true
				replacedCount++
			}
			result := tx.Model(&PesertaDidik{}).
				Where("id = ? AND kelas_id = ?", student.ID, kelasID).
				Updates(map[string]any{"identitas_file_path": file.Path, "identitas_file_ext": file.Ext})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("peserta didik tujuan berubah")
			}
		}
		s.auditTx(tx, &uid, "upload", "identitas_siswa", fmt.Sprintf("kelas=%s siswa=%d", kelasID, len(extracted)))
		return nil
	}); err != nil {
		return fiber.NewError(500, "identitas siswa tidak dapat disimpan")
	}
	keepNewFiles = true
	for path := range oldPaths {
		removeUpload(path)
	}
	return c.JSON(fiber.Map{
		"kelasId":        kelasID,
		"uploadedCount":  len(extracted),
		"replacedCount":  replacedCount,
		"unmatchedNISNs": unmatched,
		"skippedFiles":   skipped,
	})
}

func identitasContentType(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "pdf":
		return "application/pdf"
	default:
		return ""
	}
}

func (s *Server) downloadIdentitasAnak(c *fiber.Ctx) error {
	if _, err := s.parentChild(c); err != nil {
		return err
	}
	var student PesertaDidik
	if err := s.db.Select("id", "nisn", "identitas_file_path", "identitas_file_ext").First(&student, "id = ?", c.Params("id")).Error; err != nil {
		return fiber.NewError(404, "peserta didik tidak ditemukan")
	}
	if student.IdentitasFilePath == nil || student.IdentitasFileExt == nil || !validIdentitasExtension(*student.IdentitasFileExt) {
		return fiber.NewError(404, "identitas foto belum tersedia")
	}
	contentType := identitasContentType(*student.IdentitasFileExt)
	if contentType == "" || !validIdentitasNISN(student.NISN) {
		return fiber.NewError(404, "identitas foto belum tersedia")
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", student.NISN+"."+*student.IdentitasFileExt))
	return s.sendUpload(c, *student.IdentitasFilePath)
}
