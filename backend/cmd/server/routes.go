package main

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	"github.com/robfig/cron/v3"
	"github.com/skip2/go-qrcode"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Server) routes(api fiber.Router) {
	// IMPORTANT: Fiber v2.52's api.Group("", middleware) applies the middleware to
	// ALL routes registered AFTER it on the same router (empty-prefix group quirk).
	// To keep guru-accessible endpoints reachable, every bare-api route (auth only,
	// with its own inline role checks) MUST be registered BEFORE the readAll/admin
	// groups below. readAll (admin+kepala) and admin (admin-only) groups are registered
	// last, and no bare-api route follows them, so the quirk never leaks
	// managementRead/writable/admin onto guru-accessible endpoints.

	// Modul Nilai (prd_nilai.md) — admin/kepala pass through; guru is scoped to
	// PenugasanGuruMapel inside each handler.
	api.Get("/tema", s.listTema)
	api.Get("/tema/:id/grid", s.gridTema)
	api.Get("/nilai/rekap", s.getRekap)
	api.Get("/nilai/export", s.exportNilai)
	api.Get("/nilai/scope", s.getNilaiScope)
	api.Post("/tema", s.createTema)
	api.Put("/tema/:id", s.updateTema)
	api.Delete("/tema/:id", s.deleteTema)
	api.Put("/tema/:id/nilai", s.saveNilai)

	// Other bare-api reads (each handler scopes guru via inline checks).
	api.Get("/kelas", s.listKelas)
	api.Get("/kelas/:id/riwayat-wali", s.listRiwayatWali)
	api.Get("/peserta-didik", s.listSiswa)
	api.Get("/peserta-didik/export", s.exportSiswa)
	api.Get("/presensi", s.listPresensi)
	api.Get("/presensi/export", s.exportPresensi)
	api.Get("/presensi/rekap", s.rekapPresensi)
	api.Get("/presensi/rekap/pdf", s.rekapPresensiPDF)
	api.Get("/presensi/:id/pdf", s.exportPresensiPDF)
	api.Get("/penugasan", s.listPenugasan)
	api.Get("/settings/jadwal", s.getJadwal)

	// Bare-api writes (guru may create/update presensi for classes they manage;
	// canManageKelas inside each handler enforces that).
	api.Post("/presensi", s.createPresensi)
	api.Put("/presensi/:id", s.updatePresensi)
	api.Post("/presensi/:id/details", s.saveDetails)

	// Modul Peminjaman Buku (PRD_pinjam_buku.md) — bare-api reads/writes. Guru is
	// scoped to rombel they walikan via canManageKelas inside each write handler.
	api.Get("/buku-kelas", s.listBukuKelas)
	api.Get("/peminjaman-buku/aktif", s.listPeminjamanAktif)
	api.Post("/peminjaman-buku", s.createPeminjaman)
	api.Post("/peminjaman-buku/kembali", s.createPengembalian)

	// Modul B — Pengumuman (prd_fitur_simpkbm.md). Guru scoped to target=kelas &
	// kelas walinya inside each write handler (canManageKelas). List is bare-api so
	// guru can read pengumuman target=semua + milik rombelnya.
	api.Get("/pengumuman", s.listPengumuman)
	api.Post("/pengumuman", s.createPengumuman)
	api.Put("/pengumuman/:id", s.updatePengumuman)

	// Modul K — Jurnal Mengajar. Guru scoped to own jurnal (TutorID) & kelas wali;
	// approve/reject role-checked inside handler (admin||kepala_sekolah, NOT
	// canManageKelas which rejects kepala).
	api.Get("/jurnal", s.listJurnal)
	api.Post("/jurnal", s.createJurnal)
	api.Put("/jurnal/:id", s.updateJurnal)
	api.Delete("/jurnal/:id", s.deleteJurnal)
	api.Get("/jurnal/:id/foto", s.jurnalFoto)

	// Modul C — Tugas Siswa (prd_fitur_simpkbm.md). Tutor membuat tugas per mapel+kelas
	// (lampiran opsional); pengumpulan & nilai dicatat tutor (canManageKelas). Admin
	// bebas. List is bare-api so guru can read tugas for kelas they walikan.
	api.Get("/tugas", s.listTugas)
	api.Post("/tugas", s.createTugas)
	api.Put("/tugas/:id", s.updateTugas)
	api.Delete("/tugas/:id", s.deleteTugas)
	api.Get("/tugas/:id/pengumpulan", s.listPengumpulan)
	api.Post("/tugas/:id/pengumpulan", s.createPengumpulan)
	api.Post("/tugas/:id/nilai", s.nilaiPengumpulan)
	api.Get("/tugas/:id/lampiran", s.tugasLampiran)
	api.Get("/tugas/:id/pengumpulan/:pid/file", s.pengumpulanFile)

	// Modul E — Materi Pembelajaran. Tutor upload materi per mapel+kelas; download &
	// komentar scoped (canManageKelas / admin / kepala).
	api.Get("/materi", s.listMateri)
	api.Post("/materi", s.createMateri)
	api.Put("/materi/:id", s.updateMateri)
	api.Delete("/materi/:id", s.deleteMateri)
	api.Get("/materi/:id", s.getMateri)
	api.Get("/materi/:id/download", s.downloadMateri)
	api.Post("/materi/:id/komentar", s.komentarMateri)
	api.Get("/materi/:id/share", s.getMateriShare)
	api.Post("/materi/:id/share", s.shareMateri)

	// Modul R — RPP (Rencana Pelaksanaan Pembelajaran). Tutor penyu­sun (IsRPPMaker,
	// ditugaskan admin) upload file RPP per mapel+jenjang; 1 RPP dipakai bersama seluruh
	// rombel jenjang itu (distribusi tersinkron). Tutor pengajar jenjang tsb bisa lihat
	// & download. NOTE: route statis maker-status HARUS sebelum /rpp/:id (quirk Fiber
	// v2.52 empty-prefix, lihat comment di atas).
	api.Get("/rpp", s.listRPP)
	api.Get("/rpp/maker-status", s.rppMakerStatus)
	api.Get("/rpp/options", s.rppOptions)
	api.Post("/rpp", s.createRPP)
	api.Put("/rpp/:id", s.updateRPP)
	api.Delete("/rpp/:id", s.deleteRPP)
	api.Get("/rpp/:id/download", s.downloadRPP)

	// Modul F — Kelas Virtual. Jadwal kelas daring (link meeting) per mapel+kelas.
	api.Get("/kelas-virtual", s.listKelasVirtual)
	api.Post("/kelas-virtual", s.createKelasVirtual)
	api.Put("/kelas-virtual/:id", s.updateKelasVirtual)
	api.Delete("/kelas-virtual/:id", s.deleteKelasVirtual)

	// Modul D — Bank Soal + Ujian Luring (prd_fitur_simpkbm.md). Tutor CRUD bank soal
	// (scoped ke miliknya) & menyusun ujian per kelas (canManageKelas). Cetak naskah +
	// kunci via gofpdf; acak deterministik per ujianID. Kepala read-only.
	api.Get("/bank-soal", s.listBankSoal)
	api.Post("/bank-soal", s.createBankSoal)
	api.Put("/bank-soal/:id", s.updateBankSoal)
	api.Delete("/bank-soal/:id", s.deleteBankSoal)
	api.Get("/ujian", s.listUjian)
	api.Post("/ujian", s.createUjian)
	api.Put("/ujian/:id", s.updateUjian)
	api.Delete("/ujian/:id", s.deleteUjian)
	api.Get("/ujian/:id/soal", s.listUjianSoal)
	api.Post("/ujian/:id/soal", s.addUjianSoal)
	api.Delete("/ujian/:id/soal/:sid", s.deleteUjianSoal)
	api.Get("/ujian/:id/print", s.printUjian)
	api.Get("/ujian/:id/export", s.exportUjianResults)

	// Modul Notifikasi — CRUD notifikasi user.
	api.Get("/notifikasi", s.listNotifikasi)
	api.Get("/notifikasi/unread-count", s.unreadNotifikasiCount)
	api.Put("/notifikasi/:id/baca", s.bacaNotifikasi)
	api.Put("/notifikasi/baca-all", s.bacaSemuaNotifikasi)

	// Modul Kalender Akademik — event kalender.
	api.Get("/kalender", s.listKalenderEvent)
	api.Post("/kalender", s.createKalenderEvent)
	api.Put("/kalender/:id", s.updateKalenderEvent)
	api.Delete("/kalender/:id", s.deleteKalenderEvent)

	// Modul Portal Orang Tua — data anak untuk orang tua.
	api.Get("/orang-tua/anak", s.listAnakOrangTua)
	api.Get("/orang-tua/anak/:id/nilai", s.getNilaiAnak)
	api.Get("/orang-tua/anak/:id/presensi", s.getPresensiAnak)
	api.Get("/orang-tua/anak/:id/rapor", s.getRaporAnak)
	api.Get("/orang-tua/anak/:id/ujian-skor", s.getUjianSkorAnak)
	api.Get("/orang-tua/anak/:id/tugas", s.getTugasAnak)
	api.Get("/orang-tua/anak/:id/materi", s.getMateriAnak)
	api.Get("/orang-tua/anak/:id/peminjaman", s.getPeminjamanAnak)
	api.Get("/orang-tua/anak/:id/chat", s.listChatAnak)
	api.Post("/orang-tua/anak/:id/chat", s.sendChatAnak)
	api.Get("/orang-tua/anak/:id/perilaku", s.getPerilakuAnak)

	// Modul P — Kartu Pelajar (prd_fitur_simpkbm.md). Cetak PDF (ID card + QR) per
	// siswa atau massal per rombel. Guard canManageKelas via kelas siswa (admin/kepala
	// bebas). Upload foto siswa (reuse saveUpload) — admin atau wali kelas.
	api.Get("/kartu-pelajar/:pesertaDidikId/print", s.printKartuPelajar)
	api.Get("/kartu-pelajar/group/:kelasId/print", s.printKartuGroup)
	api.Post("/peserta-didik/:id/foto", s.uploadFotoSiswa)
	// Modul G — Catatan Perilaku (tutor/admin catat; guru scoped ke kelas wali).
	api.Get("/perilaku", s.listPerilaku)
	api.Post("/perilaku", s.createPerilaku)
	// Modul I — Rapor (agregasi). Guard wali via kelas siswa; admin/kepala bebas.
	api.Get("/rapor/:pesertaDidikId", s.getRapor)
	api.Get("/rapor/:pesertaDidikId/print", s.printRapor)
	api.Put("/rapor/:pesertaDidikId/catatan", s.putCatatanRapor)
	// Modul L — Modul Pembelajaran + capaian (read semua role terauth).
	api.Get("/modul-belajar", s.listModulBelajar)
	api.Get("/modul-belajar/:id/outcomes", s.listCapaianModul)
	// Modul M — Kompetensi + capaian + rombel-kompetensi + nilai (read terauth).
	api.Get("/kompetensi", s.listKompetensi)
	api.Get("/kompetensi/:id/outcomes", s.listCapaianKompetensi)
	api.Get("/rombel-kompetensi", s.listRombelKompetensi)
	api.Get("/nilai-kompetensi", s.listNilaiKompetensi)
	api.Post("/nilai-kompetensi", s.saveNilaiKompetensi)
	// Modul J — Pusat Laporan (agregator, dispatch ke handler export). Role gating
	// per-jenis di laporanExport; guru scoped via kelasId wali di handler terkait.
	api.Get("/laporan/jenis", s.laporanJenis)
	api.Get("/laporan/export", s.laporanExport)
	// Modul R — Import Terpusat. Template + import partial-success; role gating
	// per-tipe di importTerpusat (siswa admin, nilai-kompetensi admin/tutor wali).
	api.Get("/import/template/:tipe", s.importTemplate)
	api.Post("/import", s.importTerpusat)
	// Dokumen tutor: endpoint metadata melakukan pengecekan admin di handler
	// karena path /tutor/dokumen berpotensi tertangkap oleh /tutor/:id pada
	// managementRead yang didaftarkan sesudah blok bare-api ini.
	api.Get("/tutor/me/dokumen", s.getMyTutorDocuments)
	api.Get("/tutor/me/dokumen/sk-pengangkatan", s.downloadMyTutorSKPengangkatan)
	api.Get("/tutor/me/dokumen/sk-penugasan", s.downloadMyTutorSKPenugasan)
	api.Get("/tutor/dokumen", s.listTutorDocuments)

	// Admin+kepala_sekolah reads (managementRead rejects guru).
	readAll := api.Group("", s.managementRead)
	readAll.Get("/tutor", func(c *fiber.Ctx) error { return list[Tutor](s.db, c) })
	readAll.Get("/tutor/:id", func(c *fiber.Ctx) error { return get[Tutor](s.db, c) })
	readAll.Get("/orang-tua", func(c *fiber.Ctx) error { return list[OrangTua](s.db, c) })
	readAll.Get("/orang-tua/relasi", s.listRelasiOrtu)
	readAll.Get("/pokjar", func(c *fiber.Ctx) error { return list[Pokjar](s.db, c) })
	readAll.Get("/tahun-ajaran", func(c *fiber.Ctx) error { return list[TahunAjaran](s.db.Order("tanggal_mulai desc"), c) })
	readAll.Get("/mapel", func(c *fiber.Ctx) error { return list[MataPelajaran](s.db, c) })
	readAll.Get("/users", func(c *fiber.Ctx) error { return list[User](s.db, c) })
	readAll.Get("/kelas-mapel", s.listKelasMapel)
	readAll.Get("/audit-logs", s.listAuditLogs)
	readAll.Get("/arsip", s.arsip)
	readAll.Get("/settings/nilai", s.getSettingsNilai)
	readAll.Get("/buku", func(c *fiber.Ctx) error { return list[Buku](s.db.Order("judul"), c) })
	readAll.Get("/buku/rekap", s.rekapBuku)
	readAll.Get("/buku/export", s.exportBuku)
	// Modul O/N — master Program & Fase (admin+kepala read).
	readAll.Get("/program", func(c *fiber.Ctx) error { return list[Program](s.db, c) })
	readAll.Get("/fase", func(c *fiber.Ctx) error { return list[Fase](s.db, c) })
	// Modul H — Sertifikat (admin+kepala read; terbit admin-only).
	readAll.Get("/sertifikat", s.listSertifikat)
	readAll.Get("/sertifikat/:id/print", s.printSertifikat)
	// Modul S — Sumber Nilai & bobot (read admin+kepala).
	readAll.Get("/sumber-nilai", func(c *fiber.Ctx) error { return list[SumberNilai](s.db, c) })
	readAll.Get("/bobot-sumber-nilai", s.listBobotSumberNilai)
	// Modul R — riwayat import terpusat (admin+kepala).
	readAll.Get("/import/log", s.listImportLog)

	// Admin-only writes (writable rejects kepala_sekolah; admin rejects non-admin).
	admin := api.Group("", s.writable, s.admin)
	admin.Post("/seed/dummy", s.seedDummyHandler)
	s.crudTutor(admin)
	s.crudOrangTua(admin)
	s.crudPokjar(admin)
	s.crudTahunAjaran(admin)
	s.crudSemester(admin)
	s.crudMapel(admin)
	s.crudUsers(admin)
	admin.Post("/kelas", s.createKelas)
	admin.Put("/kelas/:id", s.updateKelas)
	admin.Delete("/kelas/:id", s.deleteKelas)
	admin.Post("/kelas/duplicate", s.duplicateKelas)
	admin.Put("/kelas/:id/mapel", s.setKelasMapel)
	admin.Post("/penugasan", s.createPenugasan)
	admin.Post("/penugasan/semua-kelas", s.assignAllClasses)
	admin.Delete("/penugasan/:id", s.deletePenugasan)
	admin.Post("/peserta-didik", s.createSiswa)
	admin.Post("/peserta-didik/import", s.importSiswa)
	admin.Get("/peserta-didik/template", s.siswaTemplate)
	admin.Put("/peserta-didik/:id", s.updateSiswa)
	admin.Delete("/peserta-didik/:id", s.deleteSiswa)
	admin.Post("/kenaikan-kelas", s.promote)
	admin.Put("/settings/jadwal", s.putJadwal)
	admin.Put("/settings/nilai", s.putSettingsNilai)
	s.crudBuku(admin)
	admin.Post("/buku-kelas", s.createBukuKelas)
	admin.Delete("/buku-kelas/:id", s.deleteBukuKelas)
	admin.Delete("/pengumuman/:id", func(c *fiber.Ctx) error { return deleteRow[Pengumuman](s, c, "pengumuman") })
	// Modul O/N — master Program & Fase (admin CRUD).
	admin.Post("/program", func(c *fiber.Ctx) error { return create[Program](s, c, "program") })
	admin.Put("/program/:id", func(c *fiber.Ctx) error { return update[Program](s, c, "program") })
	admin.Delete("/program/:id", func(c *fiber.Ctx) error { return deleteRow[Program](s, c, "program") })
	admin.Post("/fase", func(c *fiber.Ctx) error { return create[Fase](s, c, "fase") })
	admin.Put("/fase/:id", func(c *fiber.Ctx) error { return update[Fase](s, c, "fase") })
	admin.Delete("/fase/:id", func(c *fiber.Ctx) error { return deleteRow[Fase](s, c, "fase") })
	// Modul H — Sertifikat (terbit admin-only).
	admin.Post("/sertifikat", s.createSertifikat)
	// Modul S — Sumber Nilai (master) & bobot per mapel (admin CRUD).
	admin.Post("/sumber-nilai", func(c *fiber.Ctx) error { return create[SumberNilai](s, c, "sumber_nilai") })
	admin.Put("/sumber-nilai/:id", func(c *fiber.Ctx) error { return update[SumberNilai](s, c, "sumber_nilai") })
	admin.Delete("/sumber-nilai/:id", func(c *fiber.Ctx) error { return deleteRow[SumberNilai](s, c, "sumber_nilai") })
	admin.Post("/bobot-sumber-nilai", s.upsertBobotSumberNilai)
	admin.Delete("/bobot-sumber-nilai/:id", func(c *fiber.Ctx) error { return deleteRow[BobotSumberNilai](s, c, "bobot_sumber_nilai") })
	// Modul L — Modul Pembelajaran + capaian (admin CRUD).
	admin.Post("/modul-belajar", s.createModulBelajar)
	admin.Put("/modul-belajar/:id", s.updateModulBelajar)
	admin.Delete("/modul-belajar/:id", s.deleteModulBelajar)
	admin.Post("/modul-belajar/:id/outcomes", s.createCapaianModul)
	admin.Put("/modul-belajar/:id/outcomes/:oid", s.updateCapaianModul)
	admin.Delete("/modul-belajar/:id/outcomes/:oid", s.deleteCapaianModul)
	// Modul M — Kompetensi + capaian + rombel-kompetensi (admin CRUD).
	admin.Post("/kompetensi", s.createKompetensi)
	admin.Put("/kompetensi/:id", s.updateKompetensi)
	admin.Delete("/kompetensi/:id", s.deleteKompetensi)
	admin.Post("/kompetensi/:id/outcomes", s.createCapaianKompetensi)
	admin.Put("/kompetensi/:id/outcomes/:oid", s.updateCapaianKompetensi)
	admin.Delete("/kompetensi/:id/outcomes/:oid", s.deleteCapaianKompetensi)
	admin.Post("/rombel-kompetensi", s.createRombelKompetensi)
	admin.Delete("/rombel-kompetensi/:id", s.deleteRombelKompetensi)
	// Backup & restore (admin-only writes). n8n uses the GET /backup/download
	// read endpoint (backupReadAuth) instead; these create/restore/delete a
	// backup file and require a real admin session.
	admin.Post("/backup", s.createBackupNow)
	admin.Post("/backup/restore", s.stageRestore)
	admin.Delete("/backup/:name", s.deleteBackupFile)
}

func id(c *fiber.Ctx) string { return c.Params("id") }
func list[T any](db *gorm.DB, c *fiber.Ctx, preload ...string) error {
	q := db
	for _, p := range preload {
		q = q.Preload(p)
	}
	var rows []T
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}
func get[T any](db *gorm.DB, c *fiber.Ctx) error {
	var row T
	if e := db.First(&row, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	return c.JSON(row)
}
func create[T any](s *Server, c *fiber.Ctx, resource string) error {
	var row T
	if e := c.BodyParser(&row); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if e := s.db.Create(&row).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", resource, "")
	return c.Status(201).JSON(row)
}
func update[T any](s *Server, c *fiber.Ctx, resource string) error {
	var row T
	if e := s.db.First(&row, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := c.BodyParser(&row); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if e := s.db.Save(&row).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", resource, id(c))
	return c.JSON(row)
}
func deleteRow[T any](s *Server, c *fiber.Ctx, resource string) error {
	if e := s.db.Delete(new(T), "id = ?", id(c)).Error; e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", resource, id(c))
	return c.SendStatus(204)
}

// saveUpload reads an optional multipart file field, validates size+extension, and
// stores it under ./uploads/<dir>/<uuid><ext>. Returns the relative path
// "uploads/<dir>/<uuid><ext>" (or "" when the field is absent). There is no in-house
// precedent for disk uploads — this is the reusable helper for modul C/E/K/P.
func (s *Server) saveUpload(c *fiber.Ctx, field, dir string, maxBytes int64, exts []string) (string, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		return "", nil // field not provided — caller treats "" as "no file"
	}
	if fh.Size == 0 || fh.Size > maxBytes {
		return "", fiber.NewError(400, fmt.Sprintf("file %s harus antara 1 byte dan %d byte", field, maxBytes))
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	ok := false
	for _, e := range exts {
		if ext == "."+e {
			ok = true
			break
		}
	}
	if !ok {
		return "", fiber.NewError(400, fmt.Sprintf("ekstensi file %s tidak diizinkan", ext))
	}
	if err := os.MkdirAll("./uploads/"+dir, 0o755); err != nil {
		return "", fiber.NewError(500, "tidak dapat membuat direktori upload")
	}
	name := uuid.NewString() + ext
	if err := c.SaveFile(fh, "./uploads/"+dir+"/"+name); err != nil {
		return "", fiber.NewError(500, "tidak dapat menyimpan file")
	}
	return "uploads/" + dir + "/" + name, nil
}

// sendUpload streams a previously saved upload to the client. relPath is the value
// stored by saveUpload ("uploads/<dir>/<file>"). Path traversal is guarded: only
// paths under "uploads/" and free of ".." are accepted. Serve via scoped handlers
// (auth) — do NOT expose /uploads as a public static route (files are sensitive).
func (s *Server) sendUpload(c *fiber.Ctx, relPath string) error {
	if relPath == "" || !strings.HasPrefix(relPath, "uploads/") || strings.Contains(relPath, "..") {
		return fiber.NewError(404, "file tidak ditemukan")
	}
	return c.SendFile("./" + relPath)
}

func (s *Server) crudTutor(r fiber.Router) {
	r.Get("/tutor/dokumen/sk-penugasan/download", s.downloadTutorSKPenugasan)
	r.Post("/tutor/dokumen/sk-penugasan", s.uploadTutorSKPenugasan)
	r.Get("/tutor/:id/dokumen/sk-pengangkatan", s.downloadTutorSKPengangkatan)
	r.Post("/tutor/:id/dokumen/sk-pengangkatan", s.uploadTutorSKPengangkatan)
	r.Get("/tutor", func(c *fiber.Ctx) error { return list[Tutor](s.db, c) })
	r.Get("/tutor/:id", func(c *fiber.Ctx) error { return get[Tutor](s.db, c) })
	r.Post("/tutor", s.createTutor)
	r.Put("/tutor/:id", s.updateTutor)
	r.Delete("/tutor/:id", func(c *fiber.Ctx) error { return deleteRow[Tutor](s, c, "tutor") })
}

const sharedTutorAssignmentDocumentCode = "sk_penugasan_tutor"

type tutorInput struct {
	Nama            string  `json:"nama"`
	JenisKelamin    string  `json:"jenisKelamin"`
	NoHP            *string `json:"noHp"`
	Alamat          string  `json:"alamat"`
	TanggalBertugas *string `json:"tanggalBertugas"`
	IsRPPMaker      *bool   `json:"isRppMaker"`
}

func parseTutorAssignmentDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := parseFlexibleDate(*value)
	if err != nil {
		return nil, fiber.NewError(400, "format tanggalBertugas tidak valid (YYYY-MM-DD)")
	}
	return &parsed, nil
}

func validateTutorInput(in *tutorInput) error {
	in.Nama = strings.TrimSpace(in.Nama)
	in.JenisKelamin = strings.ToUpper(strings.TrimSpace(in.JenisKelamin))
	if in.NoHP != nil {
		trimmed := strings.TrimSpace(*in.NoHP)
		in.NoHP = &trimmed
	}
	in.Alamat = strings.TrimSpace(in.Alamat)
	if in.Nama == "" || (in.JenisKelamin != "L" && in.JenisKelamin != "P") || (in.NoHP != nil && *in.NoHP == "") {
		return fiber.NewError(400, "nama dan jenisKelamin (L/P) wajib diisi; noHp tidak boleh kosong")
	}
	return nil
}

func (s *Server) createTutor(c *fiber.Ctx) error {
	var in tutorInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if err := validateTutorInput(&in); err != nil {
		return err
	}
	tanggal, err := parseTutorAssignmentDate(in.TanggalBertugas)
	if err != nil {
		return err
	}
	noHP := ""
	if in.NoHP != nil {
		noHP = *in.NoHP
	}
	row := Tutor{Nama: in.Nama, JenisKelamin: in.JenisKelamin, NoHP: noHP, Alamat: in.Alamat, TanggalBertugas: tanggal}
	if in.IsRPPMaker != nil {
		row.IsRPPMaker = *in.IsRPPMaker
	}
	if err := s.db.Create(&row).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "tutor", row.ID)
	return c.Status(201).JSON(row)
}

func (s *Server) updateTutor(c *fiber.Ctx) error {
	var row Tutor
	if err := s.db.First(&row, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(404, "record not found")
	}
	var in tutorInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if err := validateTutorInput(&in); err != nil {
		return err
	}
	if in.TanggalBertugas != nil {
		tanggal, err := parseTutorAssignmentDate(in.TanggalBertugas)
		if err != nil {
			return err
		}
		row.TanggalBertugas = tanggal
	}
	row.Nama, row.JenisKelamin, row.Alamat = in.Nama, in.JenisKelamin, in.Alamat
	if in.NoHP != nil {
		row.NoHP = *in.NoHP
	}
	if in.IsRPPMaker != nil {
		row.IsRPPMaker = *in.IsRPPMaker
	}
	if err := s.db.Save(&row).Error; err != nil {
		return fiber.NewError(400, err.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "tutor", row.ID)
	return c.JSON(row)
}

type tutorDocumentSummary struct {
	Nama                   string `json:"nama"`
	SKPengangkatanTersedia bool   `json:"skPengangkatanTersedia"`
	SKPenugasanTersedia    bool   `json:"skPenugasanTersedia"`
	SKPengangkatanNama     string `json:"skPengangkatanNama,omitempty"`
	SKPenugasanNama        string `json:"skPenugasanNama,omitempty"`
}

type tutorDocumentAdminRow struct {
	ID                     string `json:"id"`
	Nama                   string `json:"nama"`
	SKPengangkatanTersedia bool   `json:"skPengangkatanTersedia"`
	SKPengangkatanNama     string `json:"skPengangkatanNama,omitempty"`
}

type tutorDocumentAdminResponse struct {
	SKPenugasanTersedia bool                    `json:"skPenugasanTersedia"`
	SKPenugasanNama     string                  `json:"skPenugasanNama,omitempty"`
	Tutors              []tutorDocumentAdminRow `json:"tutors"`
}

func (s *Server) currentTutor(c *fiber.Ctx) (*Tutor, error) {
	uid, ok := c.Locals("userID").(string)
	if !ok || uid == "" {
		return nil, fiber.NewError(401, "sesi pengguna tidak valid")
	}
	var user User
	if err := s.db.First(&user, "id = ?", uid).Error; err != nil || user.TutorID == nil || strings.TrimSpace(*user.TutorID) == "" {
		return nil, fiber.NewError(404, "akun tutor belum terhubung ke data tutor")
	}
	var tutor Tutor
	if err := s.db.First(&tutor, "id = ?", strings.TrimSpace(*user.TutorID)).Error; err != nil {
		return nil, fiber.NewError(404, "data tutor tidak ditemukan")
	}
	return &tutor, nil
}

func (s *Server) sharedTutorAssignment() (*DokumenSistem, error) {
	var doc DokumenSistem
	err := s.db.Where("kode = ?", sharedTutorAssignmentDocumentCode).First(&doc).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Server) getMyTutorDocuments(c *fiber.Ctx) error {
	if c.Locals("role") != "guru" {
		return fiber.NewError(403, "dokumen ini hanya untuk akun tutor")
	}
	tutor, err := s.currentTutor(c)
	if err != nil {
		return err
	}
	shared, err := s.sharedTutorAssignment()
	if err != nil {
		return err
	}
	result := tutorDocumentSummary{Nama: tutor.Nama, SKPengangkatanTersedia: tutor.SKPengangkatanPath != nil && *tutor.SKPengangkatanPath != ""}
	if tutor.SKPengangkatanNama != "" {
		result.SKPengangkatanNama = tutor.SKPengangkatanNama
	}
	if shared != nil && shared.FilePath != "" {
		result.SKPenugasanTersedia = true
		result.SKPenugasanNama = shared.Nama
	}
	return c.JSON(result)
}

func (s *Server) listTutorDocuments(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(403, "daftar dokumen tutor hanya admin")
	}
	shared, err := s.sharedTutorAssignment()
	if err != nil {
		return err
	}
	var tutors []Tutor
	if err := s.db.Order("nama asc").Find(&tutors).Error; err != nil {
		return err
	}
	result := tutorDocumentAdminResponse{Tutors: make([]tutorDocumentAdminRow, 0, len(tutors))}
	if shared != nil && shared.FilePath != "" {
		result.SKPenugasanTersedia = true
		result.SKPenugasanNama = shared.Nama
	}
	for _, tutor := range tutors {
		row := tutorDocumentAdminRow{ID: tutor.ID, Nama: tutor.Nama, SKPengangkatanTersedia: tutor.SKPengangkatanPath != nil && *tutor.SKPengangkatanPath != ""}
		if tutor.SKPengangkatanNama != "" {
			row.SKPengangkatanNama = tutor.SKPengangkatanNama
		}
		result.Tutors = append(result.Tutors, row)
	}
	return c.JSON(result)
}

func removeUpload(relPath string) {
	if relPath != "" && strings.HasPrefix(relPath, "uploads/") && !strings.Contains(relPath, "..") {
		_ = os.Remove("./" + relPath)
	}
}

func (s *Server) saveTutorPDF(c *fiber.Ctx, dir string) (string, string, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", "", fiber.NewError(400, "file PDF wajib diunggah")
	}
	path, err := s.saveUpload(c, "file", dir, 10*1024*1024, []string{"pdf"})
	if err != nil {
		return "", "", err
	}
	return path, filepath.Base(file.Filename), nil
}

func (s *Server) uploadTutorSKPengangkatan(c *fiber.Ctx) error {
	var tutor Tutor
	if err := s.db.First(&tutor, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(404, "data tutor tidak ditemukan")
	}
	path, name, err := s.saveTutorPDF(c, "tutor/sk-pengangkatan")
	if err != nil {
		return err
	}
	oldPath := ""
	if tutor.SKPengangkatanPath != nil {
		oldPath = *tutor.SKPengangkatanPath
	}
	tutor.SKPengangkatanPath = &path
	tutor.SKPengangkatanNama = name
	if err := s.db.Save(&tutor).Error; err != nil {
		removeUpload(path)
		return fiber.NewError(500, "gagal menyimpan dokumen tutor")
	}
	removeUpload(oldPath)
	uid := c.Locals("userID").(string)
	s.audit(&uid, "upload", "tutor_sk_pengangkatan", tutor.ID)
	return c.JSON(map[string]any{"ok": true, "nama": name})
}

func (s *Server) uploadTutorSKPenugasan(c *fiber.Ctx) error {
	path, name, err := s.saveTutorPDF(c, "tutor/sk-penugasan")
	if err != nil {
		return err
	}
	oldPath := ""
	if old, err := s.sharedTutorAssignment(); err != nil {
		removeUpload(path)
		return err
	} else if old != nil {
		oldPath = old.FilePath
	}
	var doc DokumenSistem
	err = s.db.Where("kode = ?", sharedTutorAssignmentDocumentCode).First(&doc).Error
	if err == gorm.ErrRecordNotFound {
		doc = DokumenSistem{Kode: sharedTutorAssignmentDocumentCode}
	} else if err != nil {
		removeUpload(path)
		return err
	}
	doc.Nama, doc.FilePath = name, path
	if err := s.db.Save(&doc).Error; err != nil {
		removeUpload(path)
		return fiber.NewError(500, "gagal menyimpan SK Penugasan")
	}
	removeUpload(oldPath)
	uid := c.Locals("userID").(string)
	s.audit(&uid, "upload", "tutor_sk_penugasan", doc.ID)
	return c.JSON(map[string]any{"ok": true, "nama": name})
}

func (s *Server) downloadTutorSKPengangkatan(c *fiber.Ctx) error {
	var tutor Tutor
	if err := s.db.First(&tutor, "id = ?", id(c)).Error; err != nil || tutor.SKPengangkatanPath == nil {
		return fiber.NewError(404, "SK Pengangkatan belum tersedia")
	}
	return s.sendUpload(c, *tutor.SKPengangkatanPath)
}

func (s *Server) downloadTutorSKPenugasan(c *fiber.Ctx) error {
	doc, err := s.sharedTutorAssignment()
	if err != nil {
		return err
	}
	if doc == nil {
		return fiber.NewError(404, "SK Penugasan belum tersedia")
	}
	return s.sendUpload(c, doc.FilePath)
}

func (s *Server) downloadMyTutorSKPengangkatan(c *fiber.Ctx) error {
	if c.Locals("role") != "guru" {
		return fiber.NewError(403, "dokumen ini hanya untuk akun tutor")
	}
	tutor, err := s.currentTutor(c)
	if err != nil {
		return err
	}
	if tutor.SKPengangkatanPath == nil {
		return fiber.NewError(404, "SK Pengangkatan belum tersedia")
	}
	return s.sendUpload(c, *tutor.SKPengangkatanPath)
}

func (s *Server) downloadMyTutorSKPenugasan(c *fiber.Ctx) error {
	if c.Locals("role") != "guru" {
		return fiber.NewError(403, "dokumen ini hanya untuk akun tutor")
	}
	if _, err := s.currentTutor(c); err != nil {
		return err
	}
	return s.downloadTutorSKPenugasan(c)
}

func (s *Server) crudOrangTua(r fiber.Router) {
	r.Get("/orang-tua", func(c *fiber.Ctx) error { return list[OrangTua](s.db, c) })
	r.Post("/orang-tua", func(c *fiber.Ctx) error { return create[OrangTua](s, c, "orang_tua") })
	r.Put("/orang-tua/:id", func(c *fiber.Ctx) error { return update[OrangTua](s, c, "orang_tua") })
	r.Delete("/orang-tua/:id", func(c *fiber.Ctx) error { return deleteRow[OrangTua](s, c, "orang_tua") })
}
func (s *Server) crudPokjar(r fiber.Router) {
	r.Get("/pokjar", func(c *fiber.Ctx) error { return list[Pokjar](s.db, c) })
	r.Post("/pokjar", func(c *fiber.Ctx) error { return create[Pokjar](s, c, "pokjar") })
	r.Put("/pokjar/:id", func(c *fiber.Ctx) error { return update[Pokjar](s, c, "pokjar") })
	r.Delete("/pokjar/:id", func(c *fiber.Ctx) error { return deleteRow[Pokjar](s, c, "pokjar") })
}
func (s *Server) crudMapel(r fiber.Router) {
	r.Get("/mapel", func(c *fiber.Ctx) error { return list[MataPelajaran](s.db, c) })
	r.Post("/mapel", s.createMapel)
	r.Put("/mapel/:id", func(c *fiber.Ctx) error { return update[MataPelajaran](s, c, "mapel") })
	r.Delete("/mapel/:id", s.deleteMapel)
}

// deleteMapel removes a MataPelajaran and all child rows that reference it via
// MapelID foreign keys. Without cascade, PostgreSQL rejects the delete.
func (s *Server) deleteMapel(c *fiber.Ctx) error {
	rid := id(c)
	uid := c.Locals("userID").(string)
	// child tables referencing mapel_id — order doesn't matter for FK-free delete
	children := []struct {
		table string
		model interface{}
	}{
		{"pengaturan_bobot_nilais", &PengaturanBobotNilai{}},
		{"ambang_predikats", &AmbangPredikat{}},
		{"rekap_nilai_akhirs", &RekapNilaiAkhir{}},
		{"kelas_mapels", &KelasMapel{}},
		{"penugasan_guru_mapels", &PenugasanGuruMapel{}},
		{"temas", &Tema{}},
		{"jurnal_mengajars", &JurnalMengajar{}},
		{"tugas", &Tugas{}},
		{"materis", &Materi{}},
		{"r_p_p_s", &RPP{}},
		{"kelas_virtuales", &KelasVirtual{}},
		{"bank_soals", &BankSoal{}},
		{"ujians", &Ujian{}},
		{"bobot_sumber_nilais", &BobotSumberNilai{}},
		{"modul_belajars", &ModulBelajar{}},
		{"kompetensis", &Kompetensi{}},
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, ch := range children {
			tx.Where("mapel_id = ?", rid).Delete(ch.model)
		}
		if e := tx.Delete(&MataPelajaran{}, "id = ?", rid).Error; e != nil {
			return e
		}
		s.auditTx(tx, &uid, "delete", "mapel", rid)
		return c.SendStatus(204)
	})
}

// createMapel replaces the generic create so a new mapel is seeded with default
// nilai settings (bobot 60:40 + 3 ambang predikat) in the same transaction.
func (s *Server) createMapel(c *fiber.Ctx) error {
	var row MataPelajaran
	if e := c.BodyParser(&row); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&row).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
		if e := tx.Create(&PengaturanBobotNilai{MapelID: row.ID, BobotKeterampilan: 60, BobotPengetahuan: 40}).Error; e != nil {
			return e
		}
		if e := s.seedAmbang(tx, row.ID, row.NamaMapel); e != nil {
			return e
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "create", "mapel", row.NamaMapel)
		return c.Status(201).JSON(row)
	})
}

// seedAmbang inserts the 3 default predikat thresholds for a mapel. Matematika
// (exact, case-insensitive) uses 80/68/60; everything else 90/78/70.
func (s *Server) seedAmbang(tx *gorm.DB, mapelID, namaMapel string) error {
	a, b, cc := 90.0, 78.0, 70.0
	if strings.EqualFold(strings.TrimSpace(namaMapel), "Matematika") {
		a, b, cc = 80, 68, 60
	}
	for i, p := range []string{"A", "B", "C"} {
		min := []float64{a, b, cc}[i]
		if e := tx.Create(&AmbangPredikat{MapelID: mapelID, Predikat: p, NilaiMinimum: min}).Error; e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) crudTahunAjaran(r fiber.Router) {
	r.Get("/tahun-ajaran", func(c *fiber.Ctx) error { return list[TahunAjaran](s.db.Order("tanggal_mulai desc"), c) })
	r.Post("/tahun-ajaran", s.createTahun)
	r.Put("/tahun-ajaran/:id", s.updateTahun)
	r.Delete("/tahun-ajaran/:id", func(c *fiber.Ctx) error { return deleteRow[TahunAjaran](s, c, "tahun_ajaran") })
}
func (s *Server) crudUsers(r fiber.Router) {
	r.Get("/users", func(c *fiber.Ctx) error { return list[User](s.db, c) })
	r.Post("/users", s.createUser)
	r.Put("/users/:id", s.updateUser)
	r.Delete("/users/:id", s.deleteUser)
}
func (s *Server) listAuditLogs(c *fiber.Ctx) error {
	q := s.db.Order("created_at desc")
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		q = q.Where("action = ?", action)
	}
	if resource := strings.TrimSpace(c.Query("resource")); resource != "" {
		q = q.Where("resource = ?", resource)
	}
	var logs []AuditLog
	if err := q.Limit(250).Find(&logs).Error; err != nil {
		return err
	}
	return c.JSON(logs)
}

func parseFlexibleDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func validateAcademicYearDates(row *TahunAjaran) error {
	if row.TanggalSelesai.Before(row.TanggalMulai) {
		return fiber.NewError(400, "tanggalSelesai tidak boleh sebelum tanggalMulai")
	}
	if row.TanggalMulaiSemesterGenap != nil {
		genap := *row.TanggalMulaiSemesterGenap
		if genap.Before(row.TanggalMulai) || genap.After(row.TanggalSelesai) {
			return fiber.NewError(400, "tanggalMulaiSemesterGenap harus berada dalam rentang tahun ajaran")
		}
	}
	return nil
}

func (s *Server) createTahun(c *fiber.Ctx) error {
	var in struct {
		NamaTahunAjaran           string  `json:"namaTahunAjaran"`
		TanggalMulai              string  `json:"tanggalMulai"`
		TanggalSelesai            string  `json:"tanggalSelesai"`
		TanggalMulaiSemesterGenap *string `json:"tanggalMulaiSemesterGenap"`
		IsAktif                   bool    `json:"isAktif"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	start, err := parseFlexibleDate(in.TanggalMulai)
	if err != nil {
		return fiber.NewError(400, "format tanggalMulai tidak valid (YYYY-MM-DD)")
	}
	end, err := parseFlexibleDate(in.TanggalSelesai)
	if err != nil {
		return fiber.NewError(400, "format tanggalSelesai tidak valid (YYYY-MM-DD)")
	}
	row := TahunAjaran{
		NamaTahunAjaran: in.NamaTahunAjaran,
		TanggalMulai:    start,
		TanggalSelesai:  end,
		IsAktif:         in.IsAktif,
	}
	if in.TanggalMulaiSemesterGenap != nil && strings.TrimSpace(*in.TanggalMulaiSemesterGenap) != "" {
		genap, e := parseFlexibleDate(*in.TanggalMulaiSemesterGenap)
		if e != nil {
			return fiber.NewError(400, "format tanggalMulaiSemesterGenap tidak valid (YYYY-MM-DD)")
		}
		row.TanggalMulaiSemesterGenap = &genap
	}
	if strings.TrimSpace(row.NamaTahunAjaran) == "" {
		return fiber.NewError(400, "namaTahunAjaran wajib diisi")
	}
	if e := validateAcademicYearDates(&row); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if row.IsAktif {
			if e := tx.Model(&TahunAjaran{}).Where("is_aktif = ?", true).Update("is_aktif", false).Error; e != nil {
				return e
			}
		}
		if e := tx.Create(&row).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
		s.syncSemesters(tx, &row)
		s.auditTx(tx, &uid, "create", "tahun_ajaran", row.NamaTahunAjaran)
		return c.Status(201).JSON(row)
	})
}
func (s *Server) updateTahun(c *fiber.Ctx) error {
	var row TahunAjaran
	if e := s.db.First(&row, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		NamaTahunAjaran           *string `json:"namaTahunAjaran"`
		TanggalMulai              *string `json:"tanggalMulai"`
		TanggalSelesai            *string `json:"tanggalSelesai"`
		TanggalMulaiSemesterGenap *string `json:"tanggalMulaiSemesterGenap"`
		IsAktif                   *bool   `json:"isAktif"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.NamaTahunAjaran != nil {
		row.NamaTahunAjaran = strings.TrimSpace(*in.NamaTahunAjaran)
	}
	if in.TanggalMulai != nil {
		t, e := parseFlexibleDate(*in.TanggalMulai)
		if e != nil {
			return fiber.NewError(400, "format tanggalMulai tidak valid (YYYY-MM-DD)")
		}
		row.TanggalMulai = t
	}
	if in.TanggalSelesai != nil {
		t, e := parseFlexibleDate(*in.TanggalSelesai)
		if e != nil {
			return fiber.NewError(400, "format tanggalSelesai tidak valid (YYYY-MM-DD)")
		}
		row.TanggalSelesai = t
	}
	if in.IsAktif != nil {
		row.IsAktif = *in.IsAktif
	}
	if in.TanggalMulaiSemesterGenap != nil {
		if strings.TrimSpace(*in.TanggalMulaiSemesterGenap) == "" {
			row.TanggalMulaiSemesterGenap = nil
		} else {
			t, e := parseFlexibleDate(*in.TanggalMulaiSemesterGenap)
			if e != nil {
				return fiber.NewError(400, "format tanggalMulaiSemesterGenap tidak valid (YYYY-MM-DD)")
			}
			row.TanggalMulaiSemesterGenap = &t
		}
	}
	if strings.TrimSpace(row.NamaTahunAjaran) == "" {
		return fiber.NewError(400, "namaTahunAjaran wajib diisi")
	}
	if e := validateAcademicYearDates(&row); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if row.IsAktif {
			if e := tx.Model(&TahunAjaran{}).Where("id <> ?", row.ID).Update("is_aktif", false).Error; e != nil {
				return e
			}
		}
		if e := tx.Save(&row).Error; e != nil {
			return e
		}
		s.syncSemesters(tx, &row)
		s.auditTx(tx, &uid, "update", "tahun_ajaran", row.NamaTahunAjaran)
		return c.JSON(row)
	})
}

// syncSemesters memastikan setiap TahunAjaran memiliki 2 record Semester
// (Ganjil & Genap). Tanggal semester diturunkan dari tanggalMulaiSemesterGenap
// bila ada, atau dari titik tengah rentang tahun ajaran.
func (s *Server) syncSemesters(tx *gorm.DB, ta *TahunAjaran) {
	genapStart := ta.TanggalMulai
	if ta.TanggalMulaiSemesterGenap != nil && !ta.TanggalMulaiSemesterGenap.IsZero() {
		genapStart = *ta.TanggalMulaiSemesterGenap
	} else {
		dur := ta.TanggalSelesai.Sub(ta.TanggalMulai)
		genapStart = ta.TanggalMulai.Add(dur / 2)
	}

	// Ganjil: mulai tahun ajaran s.d. hari sebelum genap
	ganjilEnd := genapStart.AddDate(0, 0, -1)
	if ganjilEnd.Before(ta.TanggalMulai) {
		ganjilEnd = ta.TanggalMulai
	}

	// Upsert Ganjil
	var g Semester
	if tx.Where("tahun_ajaran_id = ? AND nama_semester = ?", ta.ID, "Ganjil").First(&g).Error != nil {
		g = Semester{TahunAjaranID: ta.ID, NamaSemester: "Ganjil", TanggalMulai: ta.TanggalMulai, TanggalSelesai: ganjilEnd}
		tx.Create(&g)
	} else {
		tx.Model(&g).Updates(map[string]interface{}{
			"tanggal_mulai":   ta.TanggalMulai,
			"tanggal_selesai": ganjilEnd,
		})
	}

	// Upsert Genap
	var ge Semester
	if tx.Where("tahun_ajaran_id = ? AND nama_semester = ?", ta.ID, "Genap").First(&ge).Error != nil {
		ge = Semester{TahunAjaranID: ta.ID, NamaSemester: "Genap", TanggalMulai: genapStart, TanggalSelesai: ta.TanggalSelesai}
		tx.Create(&ge)
	} else {
		tx.Model(&ge).Updates(map[string]interface{}{
			"tanggal_mulai":   genapStart,
			"tanggal_selesai": ta.TanggalSelesai,
		})
	}
}

func (s *Server) crudSemester(r fiber.Router) {
	r.Get("/semester", func(c *fiber.Ctx) error {
		q := s.db.Preload("TahunAjaran").Order("tahun_ajaran_id desc, nama_semester asc")
		if v := c.Query("tahunAjaranId"); v != "" {
			q = q.Where("tahun_ajaran_id = ?", v)
		}
		var rows []Semester
		if e := q.Find(&rows).Error; e != nil {
			return e
		}
		return c.JSON(rows)
	})
	r.Put("/semester/:id", s.updateSemester)
}

func (s *Server) updateSemester(c *fiber.Ctx) error {
	var row Semester
	if e := s.db.First(&row, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		TanggalMulai   *string `json:"tanggalMulai"`
		TanggalSelesai *string `json:"tanggalSelesai"`
		IsArchived     *bool   `json:"isArchived"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.TanggalMulai != nil {
		t, e := time.Parse("2006-01-02", *in.TanggalMulai)
		if e != nil {
			return fiber.NewError(400, "format tanggalMulai tidak valid (YYYY-MM-DD)")
		}
		row.TanggalMulai = t
	}
	if in.TanggalSelesai != nil {
		t, e := time.Parse("2006-01-02", *in.TanggalSelesai)
		if e != nil {
			return fiber.NewError(400, "format tanggalSelesai tidak valid (YYYY-MM-DD)")
		}
		row.TanggalSelesai = t
	}
	if in.IsArchived != nil {
		row.IsArchived = *in.IsArchived
	}
	uid := c.Locals("userID").(string)
	if e := s.db.Save(&row).Error; e != nil {
		return e
	}
	s.audit(&uid, "update", "semester", row.NamaSemester)
	return c.JSON(row)
}

func (s *Server) createUser(c *fiber.Ctx) error {
	var in struct {
		Username, Email, Password, Role string
		TutorID                         *string `json:"tutorId"`
		OrangTuaID                      *string `json:"orangTuaId"`
		IsActive                        bool    `json:"isActive"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if len(in.Password) < 8 || !validRole(in.Role) || strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Email) == "" {
		return fiber.NewError(400, "username, email, valid role, and a password of at least 8 characters are required")
	}
	if in.Role == "guru" && in.TutorID == nil {
		return fiber.NewError(400, "guru account requires a tutor")
	}
	if in.Role == "orang_tua" && in.OrangTuaID == nil {
		return fiber.NewError(400, "orang_tua account requires an orangTuaId")
	}
	h, _ := bcryptHash(in.Password)
	u := User{Username: in.Username, Email: in.Email, PasswordHash: h, Role: in.Role, TutorID: in.TutorID, OrangTuaID: in.OrangTuaID, IsActive: in.IsActive}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if in.TutorID != nil {
			if err := tx.First(&Tutor{}, "id = ?", *in.TutorID).Error; err != nil {
				return fiber.NewError(400, "tutor not found")
			}
			// A tutor may be linked to at most one user account. Without this
			// check, two guru accounts sharing the same tutorId would both pass
			// canManageKelas for that tutor's classes while Tutor.UserID points
			// to whichever was written last (a broken reverse link + privilege drift).
			var dup User
			if err := tx.Where("tutor_id = ?", *in.TutorID).First(&dup).Error; err == nil {
				return fiber.NewError(400, "tutor is already linked to another user account")
			}
		}
		if in.OrangTuaID != nil {
			if err := tx.First(&OrangTua{}, "id = ?", *in.OrangTuaID).Error; err != nil {
				return fiber.NewError(400, "orang tua not found")
			}
			var dup User
			if err := tx.Where("orang_tua_id = ?", *in.OrangTuaID).First(&dup).Error; err == nil {
				return fiber.NewError(400, "orang tua is already linked to another user account")
			}
		}
		if err := tx.Create(&u).Error; err != nil {
			return fiber.NewError(400, err.Error())
		}
		if in.TutorID != nil {
			return tx.Model(&Tutor{}).Where("id = ?", *in.TutorID).Update("user_id", u.ID).Error
		}
		return nil
	}); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "user", u.Username)
	return c.Status(201).JSON(u)
}
func validRole(role string) bool {
	return role == "admin" || role == "kepala_sekolah" || role == "guru" || role == "orang_tua"
}
func bcryptHash(v string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(v), bcrypt.DefaultCost)
	return string(hash), err
}

// countOtherActiveAdmins menghitung admin aktif selain akun yang diberikan.
// Dipakai updateUser/deleteUser untuk mencegah admin tunggal menonaktifkan,
// mendemote, atau menghapus dirinya sendiri — yang akan mengunci seluruh
// sistem (tidak ada lagi admin aktif untuk masuk).
func (s *Server) countOtherActiveAdmins(excludeID string) int64 {
	var n int64
	s.db.Model(&User{}).Where("role = ? AND is_active = ? AND id <> ?", "admin", true, excludeID).Count(&n)
	return n
}

// isUniqueErr mendeteksi pelanggaran constraint unik lintas driver (SQLite
// "UNIQUE constraint failed", serta gorm.ErrDuplicatedKey untuk driver lain).
func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key value")
}
func (s *Server) updateUser(c *fiber.Ctx) error {
	var u User
	if e := s.db.First(&u, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		Username, Email, Password, Role string
		TutorID                         *string `json:"tutorId"`
		OrangTuaID                      *string `json:"orangTuaId"`
		IsActive                        bool    `json:"isActive"`
	}
	if e := c.BodyParser(&in); e != nil || !validRole(in.Role) || strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Email) == "" {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Role == "guru" && in.TutorID == nil {
		return fiber.NewError(400, "guru account requires a tutor")
	}
	uid := c.Locals("userID").(string)
	targetID := id(c)
	// Cegah admin menonaktifkan atau menurunkan peran akun sendiri — selain
	// menyebabkan lockout sesi, ini kerap tidak disengaja (body tidak lengkap).
	if targetID == uid && (!in.IsActive || in.Role != "admin") {
		return fiber.NewError(403, "tidak dapat menonaktifkan atau menurunkan peran akun sendiri")
	}
	// Invariant: minimal satu admin aktif harus tersisa. Tolak bila target
	// adalah admin aktif terakhir dan perubahan ini menonaktifkan/mendemotenya.
	if u.Role == "admin" && u.IsActive && (in.Role != "admin" || !in.IsActive) {
		if s.countOtherActiveAdmins(targetID) == 0 {
			return fiber.NewError(409, "tidak dapat mengubah: minimal satu admin aktif harus tersisa")
		}
	}
	u.Username, u.Email, u.Role, u.TutorID, u.OrangTuaID, u.IsActive = in.Username, in.Email, in.Role, in.TutorID, in.OrangTuaID, in.IsActive
	if in.Password != "" {
		if len(in.Password) < 8 {
			return fiber.NewError(400, "password must be at least 8 characters")
		}
		u.PasswordHash, _ = bcryptHash(in.Password)
	}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if in.TutorID != nil {
			if err := tx.First(&Tutor{}, "id = ?", *in.TutorID).Error; err != nil {
				return fiber.NewError(400, "tutor not found")
			}
		}
		if err := tx.Save(&u).Error; err != nil {
			return err
		}
		tx.Model(&Tutor{}).Where("user_id = ?", u.ID).Update("user_id", nil)
		if in.TutorID != nil {
			return tx.Model(&Tutor{}).Where("id = ?", *in.TutorID).Update("user_id", u.ID).Error
		}
		return nil
	}); e != nil {
		return e
	}
	s.audit(&uid, "update", "user", u.Username)
	return c.JSON(u)
}

func (s *Server) deleteUser(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	targetID := id(c)
	// Cegah admin menghapus akun sendiri → lockout.
	if targetID == uid {
		return fiber.NewError(403, "tidak dapat menghapus akun sendiri")
	}
	var u User
	if e := s.db.First(&u, "id = ?", targetID).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	// Cegah menghapus admin aktif terakhir.
	if u.Role == "admin" && u.IsActive && s.countOtherActiveAdmins(targetID) == 0 {
		return fiber.NewError(409, "tidak dapat menghapus admin aktif terakhir")
	}
	return deleteRow[User](s, c, "user")
}

func (s *Server) listKelas(c *fiber.Ctx) error {
	q := s.db.Order("jenjang,nama_rombel").Preload("Pokjar").Preload("TahunAjaran").Preload("WaliKelas")
	if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return c.JSON([]Kelas{})
		}
		q = q.Where("wali_kelas_id = ?", *user.TutorID)
	}
	return list[Kelas](q, c)
}
func (s *Server) listRiwayatWali(c *fiber.Ctx) error {
	if c.Locals("role") == "guru" {
		if err := s.canManageKelas(c, id(c)); err != nil {
			return err
		}
	}
	var class Kelas
	if err := s.db.First(&class, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(404, "class not found")
	}
	var rows []RiwayatWaliKelas
	if err := s.db.Preload("Tutor").Where("kelas_id = ?", id(c)).Order("tanggal_mulai desc, created_at desc").Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(rows)
}
func (s *Server) createKelas(c *fiber.Ctx) error {
	var k Kelas
	if e := c.BodyParser(&k); e != nil || k.Jenjang < 1 || k.Jenjang > 6 {
		return fiber.NewError(400, "jenjang must be 1 through 6")
	}
	if strings.TrimSpace(k.NamaRombel) == "" || k.PokjarID == "" || k.TahunAjaranID == "" {
		return fiber.NewError(400, "namaRombel, pokjarId, dan tahunAjaranId wajib")
	}
	uid := c.Locals("userID").(string)
	// Validasi FK dalam transaksi: SQLite tidak men-enforce foreign key, jadi
	// tanpa cek ini kelas bisa dibuat dengan pokjar/tahun ajaran/wali fiktif
	// (orphan yang merusak query dashboard & Preload).
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&Pokjar{}, "id = ?", k.PokjarID).Error; err != nil {
			return fiber.NewError(400, "pokjar tidak ditemukan")
		}
		if err := tx.First(&TahunAjaran{}, "id = ?", k.TahunAjaranID).Error; err != nil {
			return fiber.NewError(400, "tahun ajaran tidak ditemukan")
		}
		if k.WaliKelasID != nil {
			if err := tx.First(&Tutor{}, "id = ?", *k.WaliKelasID).Error; err != nil {
				return fiber.NewError(400, "wali kelas (tutor) tidak ditemukan")
			}
		}
		if err := tx.Create(&k).Error; err != nil {
			return fiber.NewError(400, err.Error())
		}
		return nil
	}); e != nil {
		return e
	}
	s.trackWali(&k, nil)
	s.audit(&uid, "create", "kelas", fmt.Sprintf("Kelas %d%s", k.Jenjang, k.NamaRombel))
	return c.Status(201).JSON(k)
}
func (s *Server) updateKelas(c *fiber.Ctx) error {
	var k Kelas
	if e := s.db.First(&k, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	var old *string
	if k.WaliKelasID != nil {
		previous := *k.WaliKelasID
		old = &previous
	}
	if e := c.BodyParser(&k); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	// Re-validasi jenjang setelah BodyParser: sebelumnya klien bisa menyetel
	// jenjang:0 atau jenjang:99 tanpa ditolak karena validasi hanya di create.
	if k.Jenjang < 1 || k.Jenjang > 6 {
		return fiber.NewError(400, "jenjang must be 1 through 6")
	}
	uid := c.Locals("userID").(string)
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&k).Error; err != nil {
			return err
		}
		if (old == nil && k.WaliKelasID == nil) || (old != nil && k.WaliKelasID != nil && *old == *k.WaliKelasID) {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&RiwayatWaliKelas{}).Where("kelas_id = ? AND tanggal_selesai IS NULL", k.ID).Update("tanggal_selesai", now).Error; err != nil {
			return err
		}
		if k.WaliKelasID != nil {
			return tx.Create(&RiwayatWaliKelas{KelasID: k.ID, TutorID: *k.WaliKelasID, TanggalMulai: now}).Error
		}
		return nil
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "kelas", fmt.Sprintf("Kelas %d%s", k.Jenjang, k.NamaRombel))
	return c.JSON(k)
}
func (s *Server) trackWali(k *Kelas, old *string) {
	if (old == nil && k.WaliKelasID == nil) || (old != nil && k.WaliKelasID != nil && *old == *k.WaliKelasID) {
		return
	}
	now := time.Now()
	s.db.Model(&RiwayatWaliKelas{}).Where("kelas_id = ? AND tanggal_selesai IS NULL", k.ID).Update("tanggal_selesai", now)
	if k.WaliKelasID != nil {
		s.db.Create(&RiwayatWaliKelas{KelasID: k.ID, TutorID: *k.WaliKelasID, TanggalMulai: now})
	}
}
func (s *Server) deleteKelas(c *fiber.Ctx) error {
	var k Kelas
	if e := s.db.First(&k, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	// SQLite tidak men-enforce foreign key, jadi hard delete kelas akan
	// meninggalkan orphan di ~16 tabel anak. Tolak jika masih direferensikan
	// data operasional; riwayat wali kelas (murni dependen) di-cascade.
	type ref struct {
		model any
		label string
	}
	refs := []ref{
		{&PesertaDidik{}, "peserta didik"},
		{&Presensi{}, "presensi"},
		{&RekapNilaiAkhir{}, "rekap nilai akhir"},
		{&Tema{}, "tema pembelajaran"},
		{&BukuKelas{}, "buku kelas"},
		{&Peminjaman{}, "peminjaman buku"},
		{&JurnalMengajar{}, "jurnal mengajar"},
		{&Tugas{}, "tugas"},
		{&Materi{}, "materi"},
		{&KelasVirtual{}, "kelas virtual"},
		{&Ujian{}, "ujian"},
		{&CatatanPerilaku{}, "catatan perilaku"},
		{&NilaiKompetensi{}, "nilai kompetensi"},
		{&RombelKompetensi{}, "rombel kompetensi"},
		{&KelasMapel{}, "kelas-mapel"},
		{&PenugasanGuruMapel{}, "penugasan guru"},
	}
	var blockers []string
	for _, r := range refs {
		var cnt int64
		s.db.Model(r.model).Where("kelas_id = ?", k.ID).Count(&cnt)
		if cnt > 0 {
			blockers = append(blockers, fmt.Sprintf("%s (%d)", r.label, cnt))
		}
	}
	if len(blockers) > 0 {
		return fiber.NewError(400, "kelas tidak dapat dihapus, masih direferensikan: "+strings.Join(blockers, ", "))
	}
	uid := c.Locals("userID").(string)
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kelas_id = ?", k.ID).Delete(&RiwayatWaliKelas{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kelas_id = ?", k.ID).Delete(&RiwayatKelasPesertaDidik{}).Error; err != nil {
			return err
		}
		return tx.Delete(&k).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "kelas", fmt.Sprintf("Kelas %d%s", k.Jenjang, k.NamaRombel))
	return c.SendStatus(204)
}
func (s *Server) duplicateKelas(c *fiber.Ctx) error {
	var in struct{ SourceTahunAjaranID, TargetTahunAjaranID string }
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	var rows []Kelas
	s.db.Where("tahun_ajaran_id = ?", in.SourceTahunAjaranID).Find(&rows)
	duplicated := 0
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		for _, k := range rows {
			k.ID = ""
			k.TahunAjaranID = in.TargetTahunAjaranID
			k.WaliKelasID = nil
			if e := tx.Create(&k).Error; e != nil {
				return e
			}
			duplicated++
		}
		return nil
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "duplicate", "kelas", fmt.Sprintf("%d classes", duplicated))
	return c.JSON(fiber.Map{"duplicated": duplicated})
}
func (s *Server) setKelasMapel(c *fiber.Ctx) error {
	var in struct {
		MapelIDs []string `json:"mapelIds"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("kelas_id = ?", id(c)).Delete(&KelasMapel{}).Error; e != nil {
			return e
		}
		for _, mapelID := range in.MapelIDs {
			if e := tx.Create(&KelasMapel{KelasID: id(c), MapelID: mapelID}).Error; e != nil {
				return e
			}
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "update", "kelas_mapel", id(c))
		return c.SendStatus(204)
	})
}
func (s *Server) listKelasMapel(c *fiber.Ctx) error {
	return list[KelasMapel](s.db.Preload("Mapel").Order("created_at desc"), c)
}
func (s *Server) createPenugasan(c *fiber.Ctx) error {
	var in struct {
		TutorID string `json:"tutorId"`
		KelasID string `json:"kelasId"`
		MapelID string `json:"mapelId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.TutorID == "" || in.KelasID == "" || in.MapelID == "" {
		return fiber.NewError(400, "tutorId, kelasId, dan mapelId wajib")
	}
	uid := c.Locals("userID").(string)
	var p PenugasanGuruMapel
	// Validasi FK dalam transaksi: tanpa ini, penugasan bisa dibuat dengan
	// tutor/kelas/mapel fiktif (SQLite tidak men-enforce FK) → Preload nanti
	// null & dashboard guru menampilkan penugasan hantu.
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&Tutor{}, "id = ?", in.TutorID).Error; err != nil {
			return fiber.NewError(400, "tutor tidak ditemukan")
		}
		if err := tx.First(&Kelas{}, "id = ?", in.KelasID).Error; err != nil {
			return fiber.NewError(400, "kelas tidak ditemukan")
		}
		if err := tx.First(&MataPelajaran{}, "id = ?", in.MapelID).Error; err != nil {
			return fiber.NewError(400, "mata pelajaran tidak ditemukan")
		}
		p = PenugasanGuruMapel{TutorID: in.TutorID, KelasID: in.KelasID, MapelID: in.MapelID}
		return tx.Create(&p).Error
	}); e != nil {
		return e
	}
	s.audit(&uid, "create", "penugasan", fmt.Sprintf("%s/%s/%s", in.TutorID, in.KelasID, in.MapelID))
	return c.Status(201).JSON(p)
}
func (s *Server) assignAllClasses(c *fiber.Ctx) error {
	var in struct {
		TutorID       string `json:"tutorId"`
		MapelID       string `json:"mapelId"`
		TahunAjaranID string `json:"tahunAjaranId"`
	}
	if err := c.BodyParser(&in); err != nil || in.TutorID == "" || in.MapelID == "" {
		return fiber.NewError(400, "tutorId and mapelId are required")
	}
	q := s.db.Table("kelas").Select("kelas.*").Joins("JOIN kelas_mapels ON kelas_mapels.kelas_id = kelas.id").Where("kelas_mapels.mapel_id = ?", in.MapelID)
	if in.TahunAjaranID != "" {
		q = q.Where("kelas.tahun_ajaran_id = ?", in.TahunAjaranID)
	}
	var classes []Kelas
	if err := q.Find(&classes).Error; err != nil {
		return err
	}
	created := 0
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, class := range classes {
			assignment := PenugasanGuruMapel{TutorID: in.TutorID, KelasID: class.ID, MapelID: in.MapelID}
			result := tx.Where("tutor_id = ? AND kelas_id = ? AND mapel_id = ?", in.TutorID, class.ID, in.MapelID).FirstOrCreate(&assignment)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				created++
			}
		}
		return nil
	}); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "penugasan_bulk", fmt.Sprintf("%d assignments", created))
	return c.JSON(fiber.Map{"created": created, "classes": len(classes)})
}
func (s *Server) listPenugasan(c *fiber.Ctx) error {
	q := s.db
	if c.Locals("role") == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return c.JSON([]PenugasanGuruMapel{})
		}
		q = q.Where("tutor_id = ?", *u.TutorID)
	}
	return list[PenugasanGuruMapel](q, c, "Tutor", "Kelas", "Mapel")
}
func (s *Server) deletePenugasan(c *fiber.Ctx) error {
	return deleteRow[PenugasanGuruMapel](s, c, "penugasan")
}

func (s *Server) listSiswa(c *fiber.Ctx) error {
	q := s.db.Preload("Kelas").Preload("OrangTua").Order("nama")
	if kelas := c.Query("kelasId"); kelas != "" {
		if err := s.canManageKelas(c, kelas); err != nil && c.Locals("role") != "admin" && c.Locals("role") != "kepala_sekolah" {
			return err
		}
		q = q.Where("kelas_id = ?", kelas)
	} else if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return c.JSON([]PesertaDidik{})
		}
		q = q.Joins("JOIN kelas ON kelas.id = peserta_didiks.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
	}
	return list[PesertaDidik](q, c)
}

// scopedSiswaQuery builds a PesertaDidik query preloaded with Kelas.Pokjar +
// OrangTua, ordered by nama, scoped to the caller's role (guru → wali kelas;
// admin/kepala → all; optional kelasId filter). Returns a human label for the
// scope (used in export headers). Mirrors listSiswa scoping.
func (s *Server) scopedSiswaQuery(c *fiber.Ctx) (*gorm.DB, string, error) {
	q := s.db.Preload("Kelas.Pokjar").Preload("OrangTua").Order("nama")
	label := "Semua Peserta Didik"
	if kelas := c.Query("kelasId"); kelas != "" {
		if err := s.canManageKelas(c, kelas); err != nil && c.Locals("role") != "admin" && c.Locals("role") != "kepala_sekolah" {
			return nil, "", err
		}
		q = q.Where("kelas_id = ?", kelas)
		var k Kelas
		if s.db.First(&k, "id = ?", kelas).Error == nil {
			label = kelasLabel(k)
		}
	} else if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return q.Where("1 = 0"), "Peserta Didik", nil
		}
		q = q.Joins("JOIN kelas ON kelas.id = peserta_didiks.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
		label = "Peserta Didik (Kelas Wali)"
	}
	return q, label, nil
}

// exportSiswa dispatches peserta-didik export to XLSX or PDF, urut abjad nama.
func (s *Server) exportSiswa(c *fiber.Ctx) error {
	q, label, err := s.scopedSiswaQuery(c)
	if err != nil {
		return err
	}
	var rows []PesertaDidik
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	if c.Query("format") == "pdf" {
		return s.exportSiswaPDF(c, rows, label)
	}
	return s.exportSiswaXLSX(c, rows, label)
}

func (s *Server) exportSiswaXLSX(c *fiber.Ctx, rows []PesertaDidik, label string) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Peserta Didik")
	_ = xlsx.SetSheetRow(sheet, "A1", &[]interface{}{"Daftar " + label + " - PKBM Tunas Ilmu"})
	headers := []interface{}{"No", "Nama", "Jenis Kelamin", "NIS", "NISN", "NIK", "Pokjar", "Kelas", "Status", "Nama Bapak", "Nama Ibu"}
	_ = xlsx.SetSheetRow(sheet, "A3", &headers)
	for i, r := range rows {
		_ = xlsx.SetSheetRow(sheet, "A"+strconv.Itoa(i+4), &[]interface{}{
			i + 1, r.Nama, r.JenisKelamin, r.NIS, r.NISN, r.NIK,
			r.Kelas.Pokjar.NamaPokjar, kelasLabel(r.Kelas), r.Status,
			r.OrangTua.NamaBapak, r.OrangTua.NamaIbu,
		})
	}
	for i, w := range []float64{6, 28, 14, 16, 16, 22, 22, 18, 10, 22, 22} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("daftar-peserta-didik.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) exportSiswaPDF(c *fiber.Ctx, rows []PesertaDidik, label string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(186, 8, "Daftar Peserta Didik PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(186, 6, label, "", 1, "C", false, 0, "")
	pdf.CellFormat(186, 6, "Total: "+strconv.Itoa(len(rows))+" siswa (urut abjad nama)", "", 1, "C", false, 0, "")
	pdf.Ln(5)
	ws := []float64{10, 70, 12, 26, 36, 32}
	hs := []string{"No", "Nama", "JK", "NISN", "Kelas", "Status"}
	pdf.SetFillColor(28, 87, 64)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	for i, h := range hs {
		pdf.CellFormat(ws[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 9)
	for i, r := range rows {
		vals := []string{strconv.Itoa(i + 1), r.Nama, r.JenisKelamin, r.NISN, kelasLabel(r.Kelas), r.Status}
		for j, v := range vals {
			a := "C"
			if j == 1 {
				a = "L"
			}
			pdf.CellFormat(ws[j], 7, v, "1", 0, a, false, 0, "")
		}
		pdf.Ln(-1)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("daftar-peserta-didik.pdf")
	return pdf.Output(c.Response().BodyWriter())
}

// relasiChild is a flattened PesertaDidik for the relationship view.
type relasiChild struct {
	ID           string `json:"id"`
	Nama         string `json:"nama"`
	JenisKelamin string `json:"jenisKelamin"`
	NIK          string `json:"nik"`
	NIS          string `json:"nis"`
	NISN         string `json:"nisn"`
	Status       string `json:"status"`
	KelasLabel   string `json:"kelasLabel"`
}

// relasiOrtu pairs an OrangTua with its children for the relationship view.
// A virtual group with an empty OrangTua represents students without a parent.
type relasiOrtu struct {
	OrangTua  OrangTua      `json:"orangTua"`
	Children  []relasiChild `json:"children"`
	AnakCount int           `json:"anakCount"`
}

func kelasLabel(k Kelas) string {
	if k.ID == "" {
		return "-"
	}
	return "Kelas " + strconv.Itoa(k.Jenjang) + k.NamaRombel
}

func toRelasiChildren(ps []PesertaDidik) []relasiChild {
	out := make([]relasiChild, 0, len(ps))
	for _, p := range ps {
		out = append(out, relasiChild{
			ID:           p.ID,
			Nama:         p.Nama,
			JenisKelamin: p.JenisKelamin,
			NIK:          p.NIK,
			NIS:          p.NIS,
			NISN:         p.NISN,
			Status:       p.Status,
			KelasLabel:   kelasLabel(p.Kelas),
		})
	}
	return out
}

// listRelasiOrtu returns parents grouped with their children for the
// relationship view. Optional ?q= filters by parent name/NIK or any child
// name/NIK/NIS/NISN. Students with no parent are appended as a virtual group.
func (s *Server) listRelasiOrtu(c *fiber.Ctx) error {
	q := strings.ToLower(strings.TrimSpace(c.Query("q", "")))
	var ortus []OrangTua
	s.db.Order("nama_ibu, nama_bapak").Find(&ortus)
	var all []PesertaDidik
	s.db.Preload("Kelas").Order("nama").Find(&all)
	byOrtu := map[string][]PesertaDidik{}
	var orphans []PesertaDidik
	for _, p := range all {
		if p.OrangTuaID == "" {
			orphans = append(orphans, p)
			continue
		}
		byOrtu[p.OrangTuaID] = append(byOrtu[p.OrangTuaID], p)
	}
	match := func(v string) bool { return q == "" || strings.Contains(strings.ToLower(v), q) }
	out := []relasiOrtu{}
	for _, o := range ortus {
		kids := byOrtu[o.ID]
		parentMatch := match(o.NamaBapak) || match(o.NamaIbu) || match(o.NIKAyah) || match(o.NIKIbu)
		var visible []PesertaDidik
		if q == "" {
			visible = kids
		} else {
			for _, k := range kids {
				if parentMatch || match(k.Nama) || match(k.NIK) || match(k.NIS) || match(k.NISN) {
					visible = append(visible, k)
				}
			}
		}
		if q != "" && !parentMatch && len(visible) == 0 {
			continue
		}
		out = append(out, relasiOrtu{OrangTua: o, Children: toRelasiChildren(visible), AnakCount: len(kids)})
	}
	if len(orphans) > 0 {
		var visible []PesertaDidik
		if q == "" {
			visible = orphans
		} else {
			for _, k := range orphans {
				if match(k.Nama) || match(k.NIK) || match(k.NIS) || match(k.NISN) {
					visible = append(visible, k)
				}
			}
		}
		if len(visible) > 0 {
			out = append(out, relasiOrtu{OrangTua: OrangTua{}, Children: toRelasiChildren(visible), AnakCount: len(orphans)})
		}
	}
	return c.JSON(out)
}
func (s *Server) createSiswa(c *fiber.Ctx) error {
	var p PesertaDidik
	if e := c.BodyParser(&p); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if p.KelasID == "" || p.PokjarID == "" {
		return fiber.NewError(400, "kelasId and pokjarId are required")
	}
	if p.Status == "" {
		p.Status = "aktif"
	}
	if p.NIK == "" {
		return fiber.NewError(400, "nik anak wajib diisi")
	}
	// Validate FK targets exist (SQLite has no FK enforcement, so without this a
	// bogus kelasId creates an orphan student + a riwayat with an empty tahun ajaran).
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		var k Kelas
		if e := tx.First(&k, "id = ?", p.KelasID).Error; e != nil {
			return fiber.NewError(400, "kelasId does not exist")
		}
		if e := tx.First(&Pokjar{}, "id = ?", p.PokjarID).Error; e != nil {
			return fiber.NewError(400, "pokjarId does not exist")
		}
		if p.OrangTuaID != "" {
			if e := tx.First(&OrangTua{}, "id = ?", p.OrangTuaID).Error; e != nil {
				return fiber.NewError(400, "orangTuaId does not exist")
			}
		}
		var dupPes PesertaDidik
		if e := tx.Where("nik = ? AND nik != ''", p.NIK).First(&dupPes).Error; e == nil {
			return fiber.NewError(400, "NIK anak sudah digunakan peserta didik lain")
		}
		if e := tx.Create(&p).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
		if e := tx.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: p.ID, KelasID: p.KelasID, TahunAjaranID: k.TahunAjaranID, Status: p.Status}).Error; e != nil {
			return e
		}
		return nil
	}); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "peserta_didik", p.Nama)
	return c.Status(201).JSON(p)
}
func (s *Server) importSiswa(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "Excel file is required")
	}
	if fileHeader.Size == 0 || fileHeader.Size > 5*1024*1024 {
		return fiber.NewError(400, "Excel file must be between 1 byte and 5 MB")
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".xlsx") {
		return fiber.NewError(400, "only .xlsx files are accepted")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(400, "cannot read uploaded file")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil || len(data) > 5*1024*1024 {
		return fiber.NewError(400, "cannot read Excel file")
	}
	xlsx, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return fiber.NewError(400, "invalid Excel file")
	}
	defer xlsx.Close()
	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return fiber.NewError(400, "Excel file has no worksheet")
	}
	rows, err := xlsx.GetRows(sheets[0])
	if err != nil || len(rows) < 2 {
		return fiber.NewError(400, "Excel must contain a header and at least one data row")
	}
	expected := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"}
	if len(rows[0]) < len(expected) {
		return fiber.NewError(400, "invalid Excel columns")
	}
	for i, header := range expected {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != header {
			return fiber.NewError(400, "Excel columns must be: "+strings.Join(expected, ", "))
		}
	}
	if len(rows)-1 > 1000 {
		return fiber.NewError(400, "Excel import is limited to 1000 rows")
	}
	type issue struct {
		Row   int    `json:"row"`
		Error string `json:"error"`
	}
	issues := []issue{}
	students := []PesertaDidik{}
	nisSeen, nisnSeen, nikSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for index, row := range rows[1:] {
		line := index + 2
		if len(row) < len(expected) {
			issues = append(issues, issue{line, "incomplete columns"})
			continue
		}
		student := PesertaDidik{Nama: strings.TrimSpace(row[0]), JenisKelamin: strings.ToUpper(strings.TrimSpace(row[1])), NIS: strings.TrimSpace(row[2]), NISN: strings.TrimSpace(row[3]), NIK: strings.TrimSpace(row[4]), KelasID: strings.TrimSpace(row[5]), PokjarID: strings.TrimSpace(row[6]), OrangTuaID: strings.TrimSpace(row[7]), Status: "aktif"}
		if student.Nama == "" || (student.JenisKelamin != "L" && student.JenisKelamin != "P") || student.NIS == "" || student.NISN == "" || student.NIK == "" || student.KelasID == "" || student.PokjarID == "" || student.OrangTuaID == "" {
			issues = append(issues, issue{line, "all columns are required; jenis_kelamin must be L or P"})
			continue
		}
		if nisSeen[student.NIS] || nisnSeen[student.NISN] || nikSeen[student.NIK] {
			issues = append(issues, issue{line, "duplicate NIS, NISN, or NIK in file"})
			continue
		}
		nisSeen[student.NIS] = true
		nisnSeen[student.NISN] = true
		nikSeen[student.NIK] = true
		students = append(students, student)
	}
	if len(issues) > 0 {
		return c.Status(422).JSON(fiber.Map{"error": "validation failed", "issues": issues})
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, student := range students {
			var class Kelas
			if err := tx.First(&class, "id = ?", student.KelasID).Error; err != nil {
				return fiber.NewError(422, "kelas_id does not exist")
			}
			if err := tx.First(&OrangTua{}, "id = ?", student.OrangTuaID).Error; err != nil {
				return fiber.NewError(422, "orang_tua_id does not exist")
			}
			if err := tx.First(&Pokjar{}, "id = ?", student.PokjarID).Error; err != nil {
				return fiber.NewError(422, "pokjar_id does not exist")
			}
			var dupPes PesertaDidik
			if err := tx.Where("nik = ? AND nik != ''", student.NIK).First(&dupPes).Error; err == nil {
				return fiber.NewError(422, "NIK already exists: "+student.NIK)
			}
			if err := tx.Create(&student).Error; err != nil {
				return fiber.NewError(422, "NIS or NISN already exists")
			}
			if err := tx.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: student.ID, KelasID: student.KelasID, TahunAjaranID: class.TahunAjaranID, Status: "aktif"}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "import", "peserta_didik", fmt.Sprintf("%d students", len(students)))
	return c.Status(201).JSON(fiber.Map{"imported": len(students)})
}
func (s *Server) siswaTemplate(c *fiber.Ctx) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	headers := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"}
	if err := xlsx.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	if err := xlsx.SetSheetRow(sheet, "A2", &[]string{"Contoh Peserta", "L", "1001", "9001001", "3201000101010001", "ID_KELAS", "ID_POKJAR", "ID_ORANG_TUA"}); err != nil {
		return err
	}
	for i, width := range []float64{28, 18, 16, 16, 22, 40, 40, 40} {
		column, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, column, column, width)
	}
	_ = xlsx.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-import-peserta-didik.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}
func (s *Server) updateSiswa(c *fiber.Ctx) error {
	var row PesertaDidik
	if e := s.db.First(&row, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := c.BodyParser(&row); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if row.NIK == "" {
		return fiber.NewError(400, "nik anak wajib diisi")
	}
	var dup PesertaDidik
	if e := s.db.Where("nik = ? AND id != ?", row.NIK, row.ID).First(&dup).Error; e == nil {
		return fiber.NewError(400, "NIK anak sudah digunakan peserta didik lain")
	}
	if row.KelasID == "" || row.PokjarID == "" {
		return fiber.NewError(400, "kelasId and pokjarId are required")
	}
	if row.Status == "" {
		row.Status = "aktif"
	}
	if e := s.db.Save(&row).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "peserta_didik", id(c))
	return c.JSON(row)
}
func (s *Server) deleteSiswa(c *fiber.Ctx) error {
	return deleteRow[PesertaDidik](s, c, "peserta_didik")
}
func (s *Server) promote(c *fiber.Ctx) error {
	var in struct {
		TargetTahunAjaranID string                                                `json:"targetTahunAjaranId"`
		Students            []struct{ ID, TargetKelasID, Status, Catatan string } `json:"students"`
	}
	if e := c.BodyParser(&in); e != nil || in.TargetTahunAjaranID == "" || len(in.Students) == 0 {
		return fiber.NewError(400, "target year and at least one student are required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var targetYear TahunAjaran
		if err := tx.First(&targetYear, "id = ?", in.TargetTahunAjaranID).Error; err != nil {
			return fiber.NewError(400, "target academic year not found")
		}
		for _, x := range in.Students {
			var p PesertaDidik
			if e := tx.First(&p, "id = ?", x.ID).Error; e != nil {
				return fiber.NewError(400, "student not found")
			}
			if x.Status == "" {
				x.Status = "naik"
			}
			if x.Status != "naik" && x.Status != "tinggal" && x.Status != "lulus" && x.Status != "pindah" && x.Status != "keluar" {
				return fiber.NewError(400, "invalid promotion status")
			}
			if (x.Status == "naik" || x.Status == "tinggal") && x.TargetKelasID == "" {
				return fiber.NewError(400, "target class is required for naik or tinggal")
			}
			p.Status = x.Status
			if x.TargetKelasID != "" {
				var targetClass Kelas
				if err := tx.First(&targetClass, "id = ? AND tahun_ajaran_id = ?", x.TargetKelasID, in.TargetTahunAjaranID).Error; err != nil {
					return fiber.NewError(400, "target class must belong to target academic year")
				}
				p.KelasID = x.TargetKelasID
			}
			if err := tx.Save(&p).Error; err != nil {
				return err
			}
			if err := tx.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: p.ID, KelasID: p.KelasID, TahunAjaranID: in.TargetTahunAjaranID, Status: x.Status, Catatan: x.Catatan}).Error; err != nil {
				return err
			}
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "promote", "peserta_didik", fmt.Sprintf("%d students to %s", len(in.Students), targetYear.NamaTahunAjaran))
		return c.SendStatus(204)
	})
}

func (s *Server) getJadwal(c *fiber.Ctx) error {
	var v PengaturanJadwal
	s.db.First(&v)
	return c.JSON(v)
}
func (s *Server) putJadwal(c *fiber.Ctx) error {
	var v PengaturanJadwal
	s.db.First(&v)
	if e := c.BodyParser(&v); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	validDays := map[string]bool{"Senin": true, "Selasa": true, "Rabu": true, "Kamis": true, "Jumat": true, "Sabtu": true, "Minggu": true}
	if !validDays[v.HariDefault] || v.ZonaWaktu != "Asia/Jakarta" {
		return fiber.NewError(400, "invalid schedule settings")
	}
	if _, err := time.Parse("15:04", v.JamGenerate); err != nil {
		return fiber.NewError(400, "jam_generate must use HH:MM format")
	}
	if e := s.db.Save(&v).Error; e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "pengaturan_jadwal", v.HariDefault+" "+v.JamGenerate)
	return c.JSON(v)
}
func (s *Server) dashboard(c *fiber.Ctx) error {
	type countRow struct {
		Label string `json:"label"`
		Total int64  `json:"total"`
	}
	studentQ := s.db.Model(&PesertaDidik{}).Where("peserta_didiks.status = ?", "aktif")
	classQ := s.db.Model(&Kelas{})
	attendanceQ := s.db.Model(&PresensiDetail{}).Where("status_kehadiran = ?", "Hadir")
	// Semester/year filter for attendance
	if sem := c.Query("semester"); sem != "" {
		year := c.Query("year")
		if year == "" {
			year = strconv.Itoa(time.Now().Year())
		}
		y, _ := strconv.Atoi(year)
		var startMonth, endMonth int
		if sem == "1" {
			startMonth, endMonth = 7, 12
		} else {
			startMonth, endMonth = 1, 6
		}
		start := time.Date(y, time.Month(startMonth), 1, 0, 0, 0, 0, time.Local)
		end := time.Date(y, time.Month(endMonth+1), 1, 0, 0, 0, 0, time.Local)
		attendanceQ = attendanceQ.Joins("JOIN presensis ON presensis.id = presensi_details.presensi_id").
			Where("presensis.tanggal >= ? AND presensis.tanggal < ?", start, end)
	}
	if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return c.JSON(fiber.Map{"pesertaDidik": 0, "kelas": 0, "hadir": 0, "perPokjar": []countRow{}, "perKelas": []countRow{}})
		}
		studentQ = studentQ.Joins("JOIN kelas ON kelas.id = peserta_didiks.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
		classQ = classQ.Where("wali_kelas_id = ?", *user.TutorID)
		attendanceQ = attendanceQ.Joins("JOIN presensis ON presensis.id = presensi_details.presensi_id").Joins("JOIN kelas ON kelas.id = presensis.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
	}
	var students, classes, attendance int64
	studentQ.Count(&students)
	classQ.Count(&classes)
	attendanceQ.Count(&attendance)
	perPokjar := []countRow{}
	pokjarQ := s.db.Table("peserta_didiks").Select("pokjars.nama_pokjar as label, COUNT(peserta_didiks.id) as total").Joins("JOIN pokjars ON pokjars.id = peserta_didiks.pokjar_id").Where("peserta_didiks.status = ?", "aktif").Group("pokjars.id, pokjars.nama_pokjar").Order("total DESC")
	// Select jenjang + nama_rombel and assemble the label in Go. The previous
	// "'Kelas ' || kelas.jenjang || kelas.nama_rombel" used the || operator on an
	// integer column, which Postgres rejects ("operator does not exist: integer
	// || text") and made /api/dashboard return 500 on a Postgres backend.
	kelasQ := s.db.Table("peserta_didiks").Select("kelas.jenjang as jenjang, kelas.nama_rombel as nama_rombel, COUNT(peserta_didiks.id) as total").Joins("JOIN kelas ON kelas.id = peserta_didiks.kelas_id").Where("peserta_didiks.status = ?", "aktif").Group("kelas.id, kelas.jenjang, kelas.nama_rombel").Order("kelas.jenjang, kelas.nama_rombel")
	if c.Locals("role") == "guru" {
		var user User
		s.db.First(&user, "id = ?", c.Locals("userID"))
		if user.TutorID != nil {
			pokjarQ = pokjarQ.Joins("JOIN kelas ON kelas.id = peserta_didiks.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
			kelasQ = kelasQ.Where("kelas.wali_kelas_id = ?", *user.TutorID)
		}
	}
	pokjarQ.Scan(&perPokjar)
	type kelasCount struct {
		Jenjang    int
		NamaRombel string
		Total      int64
	}
	var kRows []kelasCount
	kelasQ.Scan(&kRows)
	perKelas := make([]countRow, 0, len(kRows))
	for _, r := range kRows {
		perKelas = append(perKelas, countRow{Label: "Kelas " + strconv.Itoa(r.Jenjang) + r.NamaRombel, Total: r.Total})
	}
	// Upcoming kalender events (next 30 days)
	var upcomingEvents []KalenderEvent
	s.db.Where("tanggal_mulai >= ? AND tanggal_mulai <= ?", time.Now(), time.Now().AddDate(0, 0, 30)).
		Order("tanggal_mulai").Limit(5).Find(&upcomingEvents)
	// Unread notification count
	var unreadCount int64
	s.db.Model(&Notifikasi{}).Where("user_id = ? AND is_read = ?", c.Locals("userID"), false).Count(&unreadCount)
	return c.JSON(fiber.Map{
		"pesertaDidik":   students,
		"kelas":          classes,
		"hadir":          attendance,
		"perPokjar":      perPokjar,
		"perKelas":       perKelas,
		"upcomingEvents": upcomingEvents,
		"unreadNotif":    unreadCount,
	})
}
func (s *Server) arsip(c *fiber.Ctx) error {
	ta := c.Query("tahunAjaranId")
	semester := c.Query("semester")
	if ta == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "tahunAjaranId and semester (Ganjil or Genap) are required")
	}
	var rows []RiwayatKelasPesertaDidik
	q := s.db.Preload("Kelas").Where("tahun_ajaran_id = ?", ta)
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	var attendance []Presensi
	if e := s.db.Preload("Kelas").Preload("Details.PesertaDidik").Joins("JOIN kelas ON kelas.id = presensis.kelas_id").Where("kelas.tahun_ajaran_id = ? AND presensis.semester = ?", ta, semester).Order("presensis.tanggal desc").Find(&attendance).Error; e != nil {
		return e
	}
	return c.JSON(fiber.Map{"riwayatKelas": rows, "presensi": attendance, "semester": semester})
}

func (s *Server) listPresensi(c *fiber.Ctx) error {
	q := s.db.Preload("Kelas").Preload("Details.PesertaDidik").Order("tanggal desc")
	if k := c.Query("kelasId"); k != "" {
		if err := s.canManageKelas(c, k); err != nil && c.Locals("role") == "guru" {
			return err
		}
		q = q.Where("kelas_id = ?", k)
	} else if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return c.JSON([]Presensi{})
		}
		q = q.Joins("JOIN kelas ON kelas.id = presensis.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
	}
	return list[Presensi](q, c)
}
func (s *Server) exportPresensi(c *fiber.Ctx) error {
	meetings, label, err := s.scopedPresensiMeetings(c)
	if err != nil {
		return err
	}
	rows := flattenPresensi(meetings)
	switch c.Query("format") {
	case "xlsx":
		return s.exportPresensiRekapXLSX(c, rows, label)
	case "pdf":
		return s.exportPresensiRekapPDF(c, rows, label)
	default:
		return s.exportPresensiRekapCSV(c, rows, label)
	}
}

// scopedPresensiMeetings loads Presensi meetings (with Details.PesertaDidik)
// scoped by role + optional kelasId, with optional from/to date range, ordered
// tanggal asc. Returns a label for export headers.
func (s *Server) scopedPresensiMeetings(c *fiber.Ctx) ([]Presensi, string, error) {
	q := s.db.Preload("Kelas").Preload("Details.PesertaDidik").Order("tanggal asc")
	label := "Semua Kelas"
	if kelasID := c.Query("kelasId"); kelasID != "" {
		if err := s.canManageKelas(c, kelasID); err != nil && c.Locals("role") == "guru" {
			return nil, "", err
		}
		q = q.Where("kelas_id = ?", kelasID)
		var k Kelas
		if s.db.First(&k, "id = ?", kelasID).Error == nil {
			label = kelasLabel(k)
		}
	} else if c.Locals("role") == "guru" {
		var user User
		if s.db.First(&user, "id = ?", c.Locals("userID")).Error != nil || user.TutorID == nil {
			return []Presensi{}, "Presensi", nil
		}
		q = q.Joins("JOIN kelas ON kelas.id = presensis.kelas_id").Where("kelas.wali_kelas_id = ?", *user.TutorID)
		label = "Kelas Wali"
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		if t, e := time.ParseInLocation("2006-01-02", from, time.Local); e == nil {
			q = q.Where("tanggal >= ?", t)
			label += " dari " + from
		}
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		if t, e := time.ParseInLocation("2006-01-02", to, time.Local); e == nil {
			q = q.Where("tanggal < ?", t.AddDate(0, 0, 1)) // to eksklusif akhir hari
			label += " s/d " + to
		}
	}
	var meetings []Presensi
	if err := q.Find(&meetings).Error; err != nil {
		return nil, "", err
	}
	return meetings, label, nil
}

// presensiRow is one flattened (tanggal, siswa) attendance record for export.
type presensiRow struct {
	Tanggal         time.Time
	Kelas           string
	NIS, NISN, Nama string
	Status, Catatan string
	StatusPertemuan string
}

func flattenPresensi(meetings []Presensi) []presensiRow {
	var out []presensiRow
	for _, mtg := range meetings {
		kelas := kelasLabel(mtg.Kelas)
		// detail per pertemuan diurutkan by nama agar rapi di export
		details := mtg.Details
		sort.Slice(details, func(i, j int) bool { return details[i].PesertaDidik.Nama < details[j].PesertaDidik.Nama })
		for _, d := range details {
			out = append(out, presensiRow{
				Tanggal:         mtg.Tanggal,
				Kelas:           kelas,
				NIS:             d.PesertaDidik.NIS,
				NISN:            d.PesertaDidik.NISN,
				Nama:            d.PesertaDidik.Nama,
				Status:          d.StatusKehadiran,
				Catatan:         d.Catatan,
				StatusPertemuan: mtg.StatusPertemuan,
			})
		}
	}
	return out
}

func (s *Server) exportPresensiRekapCSV(c *fiber.Ctx, rows []presensiRow, label string) error {
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Attachment("rekap-presensi.csv")
	w := csv.NewWriter(c.Response().BodyWriter())
	defer w.Flush()
	_ = w.Write([]string{"Tanggal", "Kelas", "Peserta Didik", "NIS", "NISN", "Status", "Catatan", "Status Pertemuan"})
	for _, r := range rows {
		_ = w.Write([]string{r.Tanggal.Format("2006-01-02"), r.Kelas, r.Nama, r.NIS, r.NISN, r.Status, r.Catatan, r.StatusPertemuan})
	}
	return nil
}

func (s *Server) exportPresensiRekapXLSX(c *fiber.Ctx, rows []presensiRow, label string) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Presensi")
	_ = xlsx.SetSheetRow(sheet, "A1", &[]interface{}{"Rekap Presensi - PKBM Tunas Ilmu"})
	_ = xlsx.SetSheetRow(sheet, "A2", &[]interface{}{label + " - Total " + strconv.Itoa(len(rows)) + " baris"})
	headers := []interface{}{"Tanggal", "Kelas", "Nama", "NIS", "NISN", "Status Kehadiran", "Catatan", "Status Pertemuan"}
	_ = xlsx.SetSheetRow(sheet, "A4", &headers)
	for i, r := range rows {
		_ = xlsx.SetSheetRow(sheet, "A"+strconv.Itoa(i+5), &[]interface{}{
			r.Tanggal.Format("2006-01-02"), r.Kelas, r.Nama, r.NIS, r.NISN, r.Status, r.Catatan, r.StatusPertemuan,
		})
	}
	for i, w := range []float64{14, 20, 28, 14, 16, 18, 26, 18} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("rekap-presensi.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) exportPresensiRekapPDF(c *fiber.Ctx, rows []presensiRow, label string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(186, 8, "Rekap Presensi PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(186, 6, label, "", 1, "C", false, 0, "")
	pdf.Ln(4)
	ws := []float64{20, 30, 56, 24, 22, 34}
	hs := []string{"Tanggal", "Kelas", "Nama", "NISN", "Status", "Catatan"}
	pdf.SetFillColor(28, 87, 64)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	for i, h := range hs {
		pdf.CellFormat(ws[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 8)
	for _, r := range rows {
		if pdf.GetY() > 280 {
			pdf.AddPage()
			pdf.SetFont("Helvetica", "", 8)
		}
		vals := []string{r.Tanggal.Format("02-01-2006"), r.Kelas, r.Nama, r.NISN, r.Status, r.Catatan}
		for j, v := range vals {
			a := "C"
			if j == 2 || j == 5 {
				a = "L"
			}
			pdf.CellFormat(ws[j], 6, v, "1", 0, a, false, 0, "")
		}
		pdf.Ln(-1)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("rekap-presensi.pdf")
	return pdf.Output(c.Response().BodyWriter())
}
func (s *Server) rekapPresensi(c *fiber.Ctx) error {
	kelasID, semester := c.Query("kelasId"), c.Query("semester")
	if kelasID == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "kelasId and semester (Ganjil or Genap) are required")
	}
	if err := s.canManageKelas(c, kelasID); err != nil && c.Locals("role") == "guru" {
		return err
	}
	var class Kelas
	if err := s.db.First(&class, "id = ?", kelasID).Error; err != nil {
		return fiber.NewError(404, "class not found")
	}
	type recap struct {
		PesertaDidikID string `json:"pesertaDidikId"`
		Nama           string `json:"nama"`
		NIS            string `json:"nis"`
		Hadir          int64  `json:"hadir"`
		Sakit          int64  `json:"sakit"`
		Izin           int64  `json:"izin"`
		Alpa           int64  `json:"alpa"`
	}
	var rows []recap
	query := s.db.Table("peserta_didiks").Select("peserta_didiks.id as peserta_didik_id, peserta_didiks.nama, peserta_didiks.nis, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Hadir' THEN 1 ELSE 0 END) as hadir, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Sakit' THEN 1 ELSE 0 END) as sakit, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Izin' THEN 1 ELSE 0 END) as izin, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Alpa' THEN 1 ELSE 0 END) as alpa").Joins("LEFT JOIN presensi_details ON presensi_details.peserta_didik_id = peserta_didiks.id").Joins("LEFT JOIN presensis ON presensis.id = presensi_details.presensi_id AND presensis.semester = ? AND presensis.kelas_id = ?", semester, kelasID).Where("peserta_didiks.kelas_id = ?", kelasID).Group("peserta_didiks.id, peserta_didiks.nama, peserta_didiks.nis").Order("peserta_didiks.nama")
	if err := query.Scan(&rows).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"kelasId": class.ID, "semester": semester, "rows": rows})
}
func (s *Server) rekapPresensiPDF(c *fiber.Ctx) error {
	kelasID, semester := c.Query("kelasId"), c.Query("semester")
	if kelasID == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "kelasId and semester (Ganjil or Genap) are required")
	}
	if err := s.canManageKelas(c, kelasID); err != nil && c.Locals("role") == "guru" {
		return err
	}
	var class Kelas
	if err := s.db.First(&class, "id = ?", kelasID).Error; err != nil {
		return fiber.NewError(404, "class not found")
	}
	type row struct {
		Nama, NIS                string
		Hadir, Sakit, Izin, Alpa int64
	}
	var rows []row
	q := s.db.Table("peserta_didiks").Select("peserta_didiks.nama, peserta_didiks.nis, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Hadir' THEN 1 ELSE 0 END) as hadir, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Sakit' THEN 1 ELSE 0 END) as sakit, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Izin' THEN 1 ELSE 0 END) as izin, SUM(CASE WHEN presensis.id IS NOT NULL AND presensi_details.status_kehadiran = 'Alpa' THEN 1 ELSE 0 END) as alpa").Joins("LEFT JOIN presensi_details ON presensi_details.peserta_didik_id = peserta_didiks.id").Joins("LEFT JOIN presensis ON presensis.id = presensi_details.presensi_id AND presensis.semester = ? AND presensis.kelas_id = ?", semester, kelasID).Where("peserta_didiks.kelas_id = ?", kelasID).Group("peserta_didiks.id, peserta_didiks.nama, peserta_didiks.nis").Order("peserta_didiks.nama")
	if err := q.Scan(&rows).Error; err != nil {
		return err
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(186, 8, "Rekap Presensi PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(186, 6, "Kelas "+strconv.Itoa(class.Jenjang)+class.NamaRombel+" - Semester "+semester, "", 1, "C", false, 0, "")
	pdf.Ln(5)
	ws := []float64{10, 76, 28, 18, 18, 18, 18}
	hs := []string{"No", "Peserta Didik", "NIS", "Hadir", "Sakit", "Izin", "Alpa"}
	pdf.SetFillColor(28, 87, 64)
	pdf.SetTextColor(255, 255, 255)
	for i, h := range hs {
		pdf.CellFormat(ws[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 9)
	for i, r := range rows {
		vals := []string{strconv.Itoa(i + 1), r.Nama, r.NIS, strconv.FormatInt(r.Hadir, 10), strconv.FormatInt(r.Sakit, 10), strconv.FormatInt(r.Izin, 10), strconv.FormatInt(r.Alpa, 10)}
		for j, v := range vals {
			a := "C"
			if j < 3 {
				a = "L"
			}
			pdf.CellFormat(ws[j], 7, v, "1", 0, a, false, 0, "")
		}
		pdf.Ln(-1)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("rekap-presensi-" + semester + ".pdf")
	return pdf.Output(c.Response().BodyWriter())
}
func (s *Server) exportPresensiPDF(c *fiber.Ctx) error {
	var meeting Presensi
	if err := s.db.Preload("Kelas").Preload("Details.PesertaDidik").First(&meeting, "id = ?", id(c)).Error; err != nil {
		return fiber.NewError(404, "attendance meeting not found")
	}
	if err := s.canManageKelas(c, meeting.KelasID); err != nil && c.Locals("role") == "guru" {
		return err
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(180, 8, "PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(180, 7, "Daftar Presensi Pertemuan", "", 1, "C", false, 0, "")
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(42, 6, "Kelas", "", 0, "L", false, 0, "")
	pdf.CellFormat(138, 6, ": Kelas "+strconv.Itoa(meeting.Kelas.Jenjang)+meeting.Kelas.NamaRombel, "", 1, "L", false, 0, "")
	pdf.CellFormat(42, 6, "Tanggal", "", 0, "L", false, 0, "")
	pdf.CellFormat(138, 6, ": "+meeting.Tanggal.Format("02-01-2006"), "", 1, "L", false, 0, "")
	pdf.CellFormat(42, 6, "Status Pertemuan", "", 0, "L", false, 0, "")
	pdf.CellFormat(138, 6, ": "+meeting.StatusPertemuan, "", 1, "L", false, 0, "")
	pdf.Ln(5)
	widths := []float64{12, 83, 35, 30, 20}
	headers := []string{"No", "Nama Peserta Didik", "NIS", "Status", "Catatan"}
	pdf.SetFillColor(28, 87, 64)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 8, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(23, 35, 30)
	pdf.SetFont("Helvetica", "", 9)
	for i, detail := range meeting.Details {
		pdf.CellFormat(widths[0], 7, strconv.Itoa(i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 7, detail.PesertaDidik.Nama, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, detail.PesertaDidik.NIS, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 7, detail.StatusKehadiran, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 7, detail.Catatan, "1", 1, "L", false, 0, "")
	}
	pdf.Ln(12)
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(180, 6, "Wali Kelas,", "", 1, "R", false, 0, "")
	if imageBytes, ok := signatureImage(meeting.TandaTangan); ok {
		options := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
		pdf.RegisterImageOptionsReader("signature", options, bytes.NewReader(imageBytes))
		pdf.ImageOptions("signature", 145, pdf.GetY()+2, 40, 0, false, options, 0, "")
	}
	pdf.Ln(32)
	pdf.CellFormat(180, 6, "(____________________________)", "", 1, "R", false, 0, "")
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("presensi-" + meeting.Tanggal.Format("20060102") + ".pdf")
	return pdf.Output(c.Response().BodyWriter())
}
func signatureImage(value string) ([]byte, bool) {
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/png;base64,"))
	return data, err == nil && len(data) > 0
}
func validSignature(value string) bool {
	if len(value) == 0 || len(value) > 1_000_000 {
		return false
	}
	data, ok := signatureImage(value)
	return ok && len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
}
func (s *Server) canManageKelas(c *fiber.Ctx, kelasID string) error {
	if c.Locals("role") == "admin" {
		return nil
	}
	if c.Locals("role") != "guru" {
		return fiber.NewError(403, "not permitted")
	}
	var u User
	var k Kelas
	if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil || s.db.First(&k, "id = ?", kelasID).Error != nil || k.WaliKelasID == nil || *u.TutorID != *k.WaliKelasID {
		return fiber.NewError(403, "you are not this class's wali kelas")
	}
	return nil
}

// canManageKelasMapel authorizes tema/nilai writes. Per PRD §5 this is governed by
// PenugasanGuruMapel (NOT wali_kelas): a guru may manage nilai for a kelas+mapel
// only when they are explicitly assigned to teach that subject in that class.
// admin passes; kepala_sekolah is read-only; any other role is rejected.
func (s *Server) canManageKelasMapel(c *fiber.Ctx, kelasID, mapelID string) error {
	if c.Locals("role") == "admin" {
		return nil
	}
	if c.Locals("role") == "kepala_sekolah" {
		return fiber.NewError(403, "kepala sekolah has read-only access")
	}
	if c.Locals("role") != "guru" {
		return fiber.NewError(403, "not permitted")
	}
	var u User
	if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	var pg PenugasanGuruMapel
	if s.db.Where("tutor_id = ? AND kelas_id = ? AND mapel_id = ?", *u.TutorID, kelasID, mapelID).First(&pg).Error != nil {
		return fiber.NewError(403, "you are not assigned to this kelas+mapel")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Modul B — Pengumuman (prd_fitur_simpkbm.md). Broadcast internal staf.
// Guru scoped to target=kelas & kelas walinya (canManageKelas). DibuatOlehUserID
// comes from the JWT, never the client body (anti-IDOR).
// ---------------------------------------------------------------------------

// pengumumanInput is a separate input struct so the client cannot overwrite
// DibuatOlehUserID.
type pengumumanInput struct {
	Judul          string     `json:"judul"`
	Isi            string     `json:"isi"`
	Target         string     `json:"target"` // "semua" | "kelas"
	KelasID        *string    `json:"kelasId"`
	Aktif          *bool      `json:"aktif"`
	TanggalMulai   *time.Time `json:"tanggalMulai"`
	TanggalSelesai *time.Time `json:"tanggalSelesai"`
}

func (s *Server) listPengumuman(c *fiber.Ctx) error {
	q := s.db.Preload("Kelas").Order("created_at desc")
	if role := c.Locals("role").(string); role == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return fiber.NewError(403, "no tutor profile")
		}
		// guru: target=semua + target=kelas untuk rombel walinya.
		q = q.Where("target = ? OR (target = ? AND kelas_id IN (?))", "semua", "kelas",
			s.db.Model(&Kelas{}).Select("id").Where("wali_kelas_id = ?", *u.TutorID))
	}
	var rows []Pengumuman
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createPengumuman(c *fiber.Ctx) error {
	var in pengumumanInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if strings.TrimSpace(in.Judul) == "" {
		return fiber.NewError(400, "judul wajib diisi")
	}
	if in.Target == "" {
		in.Target = "semua"
	}
	role := c.Locals("role").(string)
	uid := c.Locals("userID").(string)
	if role == "guru" {
		if in.Target != "kelas" || in.KelasID == nil {
			return fiber.NewError(403, "tutor hanya dapat membuat pengumuman untuk kelas walinya")
		}
		if e := s.canManageKelas(c, *in.KelasID); e != nil {
			return e
		}
	} else if in.Target == "kelas" && in.KelasID != nil {
		var k Kelas
		if s.db.First(&k, "id = ?", *in.KelasID).Error != nil {
			return fiber.NewError(400, "kelas tidak ditemukan")
		}
	}
	aktif := true
	if in.Aktif != nil {
		aktif = *in.Aktif
	}
	p := Pengumuman{
		Judul:            in.Judul,
		Isi:              in.Isi,
		Target:           in.Target,
		KelasID:          in.KelasID,
		Aktif:            aktif,
		TanggalMulai:     in.TanggalMulai,
		TanggalSelesai:   in.TanggalSelesai,
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&p).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "pengumuman", p.ID)
	// Notify all active users about new pengumuman
	var users []User
	s.db.Where("role IN ? AND is_active = ?", []string{"admin", "kepala_sekolah", "guru"}, true).Find(&users)
	for _, u := range users {
		s.pushNotifikasi(u.ID, "Pengumuman Baru", fmt.Sprintf("\"%s\" — %s", p.Judul, p.Isi), "umum", &p.ID)
	}
	return c.Status(201).JSON(p)
}

func (s *Server) updatePengumuman(c *fiber.Ctx) error {
	var p Pengumuman
	if e := s.db.First(&p, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	role := c.Locals("role").(string)
	uid := c.Locals("userID").(string)
	if role != "admin" && p.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	var in pengumumanInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if strings.TrimSpace(in.Judul) != "" {
		p.Judul = in.Judul
	}
	p.Isi = in.Isi
	if in.Target != "" {
		p.Target = in.Target
	}
	if role == "guru" {
		// guru tidak boleh mengubah ke target=semua atau pindah kelas.
		if p.Target != "kelas" || p.KelasID == nil {
			return fiber.NewError(403, "tutor hanya dapat mengubah pengumuman untuk kelas walinya")
		}
		if e := s.canManageKelas(c, *p.KelasID); e != nil {
			return e
		}
	} else if in.KelasID != nil {
		var k Kelas
		if s.db.First(&k, "id = ?", *in.KelasID).Error != nil {
			return fiber.NewError(400, "kelas tidak ditemukan")
		}
		p.KelasID = in.KelasID
	}
	if in.Aktif != nil {
		p.Aktif = *in.Aktif
	}
	p.TanggalMulai = in.TanggalMulai
	p.TanggalSelesai = in.TanggalSelesai
	if e := s.db.Save(&p).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "pengumuman", p.ID)
	return c.JSON(p)
}

// ---------------------------------------------------------------------------
// Modul K — Jurnal Mengajar (prd_fitur_simpkbm.md). Guru mencatat kegiatan harian
// (foto bukti opsional). Jurnal LANGSUNG final (status=disetujui) saat dicatat —
// tanpa alur approve/reject. Edit/hapus oleh pemilik (TutorID) kapan saja; admin
// bebas. canManageKelas dipakai utk cek wali kelas (menolak kepala, tapi kepala
// read-only di frontend).
// ---------------------------------------------------------------------------

func (s *Server) listJurnal(c *fiber.Ctx) error {
	q := s.db.Preload("Tutor").Preload("Mapel").Preload("Kelas").Order("tanggal desc")
	if v := c.Query("tutorId"); v != "" {
		q = q.Where("tutor_id = ?", v)
	}
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	if role := c.Locals("role").(string); role == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return fiber.NewError(403, "no tutor profile")
		}
		q = q.Where("tutor_id = ?", *u.TutorID)
	}
	var rows []JurnalMengajar
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createJurnal(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	kelasID := c.FormValue("kelasId")
	if kelasID == "" {
		return fiber.NewError(400, "kelasId wajib diisi")
	}
	if e := s.canManageKelas(c, kelasID); e != nil {
		return e
	}
	tanggal, e := time.Parse("2006-01-02", c.FormValue("tanggal"))
	if e != nil {
		return fiber.NewError(400, "tanggal tidak valid (YYYY-MM-DD)")
	}
	fotoPath, e := s.saveUpload(c, "foto", "jurnal", 5*1024*1024, []string{"jpg", "jpeg", "png"})
	if e != nil {
		return e
	}
	var foto *string
	if fotoPath != "" {
		foto = &fotoPath
	}
	j := JurnalMengajar{
		TutorID:  *u.TutorID,
		MapelID:  c.FormValue("mapelId"),
		KelasID:  kelasID,
		Tanggal:  tanggal,
		Materi:   c.FormValue("materi"),
		Kegiatan: c.FormValue("kegiatan"),
		FotoPath: foto,
		Status:   "disetujui",
	}
	if e := s.db.Create(&j).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "jurnal", j.ID)
	return c.Status(201).JSON(j)
}

func (s *Server) updateJurnal(c *fiber.Ctx) error {
	var j JurnalMengajar
	if e := s.db.First(&j, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	if j.TutorID != *u.TutorID {
		return fiber.NewError(403, "hanya pemilik jurnal yang dapat mengubah")
	}
	if v := c.FormValue("mapelId"); v != "" {
		j.MapelID = v
	}
	if v := c.FormValue("kelasId"); v != "" {
		if e := s.canManageKelas(c, v); e != nil {
			return e
		}
		j.KelasID = v
	}
	if v := c.FormValue("tanggal"); v != "" {
		if t, e := time.Parse("2006-01-02", v); e == nil {
			j.Tanggal = t
		} else {
			return fiber.NewError(400, "tanggal tidak valid (YYYY-MM-DD)")
		}
	}
	if v := c.FormValue("materi"); v != "" {
		j.Materi = v
	}
	if v := c.FormValue("kegiatan"); v != "" {
		j.Kegiatan = v
	}
	fotoPath, e := s.saveUpload(c, "foto", "jurnal", 5*1024*1024, []string{"jpg", "jpeg", "png"})
	if e != nil {
		return e
	}
	if fotoPath != "" {
		j.FotoPath = &fotoPath
	}
	if e := s.db.Save(&j).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "jurnal", j.ID)
	return c.JSON(j)
}

func (s *Server) deleteJurnal(c *fiber.Ctx) error {
	var j JurnalMengajar
	if e := s.db.First(&j, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role").(string) != "admin" {
		var u User
		if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil || j.TutorID != *u.TutorID {
			return fiber.NewError(403, "hanya pemilik jurnal yang dapat menghapus")
		}
	}
	if e := s.db.Delete(&j).Error; e != nil {
		return e
	}
	s.audit(&uid, "delete", "jurnal", j.ID)
	return c.SendStatus(204)
}

func (s *Server) jurnalFoto(c *fiber.Ctx) error {
	var j JurnalMengajar
	if e := s.db.First(&j, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	role := c.Locals("role").(string)
	if role != "admin" && role != "kepala_sekolah" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil || j.TutorID != *u.TutorID {
			return fiber.NewError(403, "not permitted")
		}
	}
	if j.FotoPath == nil {
		return fiber.NewError(404, "jurnal tidak memiliki foto")
	}
	return s.sendUpload(c, *j.FotoPath)
}

// ---------------------------------------------------------------------------
// Modul C — Tugas Siswa (prd_fitur_simpkbm.md). Tutor membuat tugas per mapel+kelas
// (lampiran opsional); pengumpulan dicatat offline oleh tutor untuk siswa (upsert
// via uniqueIndex TugasID+PesertaDidikID sampai status=Dinilai). Admin bebas; kepala
// read-only (canManageKelas menolak kepala). Guru scoped ke kelas walinya.
// ---------------------------------------------------------------------------

// waliKelasIDs returns the kelas IDs a guru walis. For admin/kepala it returns
// (nil, true) meaning "all kelas". (false) means the caller has no tutor profile.
func (s *Server) waliKelasIDs(c *fiber.Ctx) ([]string, bool) {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil, true
	}
	var u User
	if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
		return nil, false
	}
	var ids []string
	s.db.Model(&Kelas{}).Where("wali_kelas_id = ?", *u.TutorID).Pluck("id", &ids)
	return ids, true
}

func parseFormTime(v string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", time.RFC3339} {
		if t, e := time.Parse(layout, v); e == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("tanggal tidak valid")
}

// formPtr returns a pointer to v when non-empty, else nil — for optional form/json
// fields backed by a nullable *string column (e.g. ModulID on Tugas/Materi).
func formPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// formInt parses a base-10 integer form value, defaulting to 0 on empty/invalid.
func formInt(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	n, e := strconv.Atoi(v)
	if e != nil {
		return 0
	}
	return n
}

// formDatePtr parses a YYYY-MM-DD form value into a *time.Time (start of day,
// local). Nil when empty or unparseable.
func formDatePtr(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	t, e := time.ParseInLocation("2006-01-02", v, time.Local)
	if e != nil {
		return nil
	}
	return &t
}

func (s *Server) listTugas(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Preload("Kelas").Order("deadline desc")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	ids, ok := s.waliKelasIDs(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil { // guru — restrict to wali kelas
		if len(ids) == 0 {
			return c.JSON([]Tugas{})
		}
		q = q.Where("kelas_id IN ?", ids)
	}
	var rows []Tugas
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createTugas(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	kelasID := c.FormValue("kelasId")
	if kelasID == "" {
		return fiber.NewError(400, "kelasId wajib diisi")
	}
	if e := s.canManageKelas(c, kelasID); e != nil {
		return e
	}
	deadline, e := parseFormTime(c.FormValue("deadline"))
	if e != nil {
		return fiber.NewError(400, e.Error())
	}
	boleh := c.FormValue("bolehUpload") != "false"
	path, e := s.saveUpload(c, "file", "tugas", 10*1024*1024, []string{"pdf", "docx", "doc", "xlsx", "xls", "png", "jpg", "jpeg"})
	if e != nil {
		return e
	}
	var fp *string
	if path != "" {
		fp = &path
	}
	t := Tugas{
		MapelID:          c.FormValue("mapelId"),
		KelasID:          kelasID,
		Judul:            c.FormValue("judul"),
		Deskripsi:        c.FormValue("deskripsi"),
		Deadline:         deadline,
		Semester:         s.semester(deadline),
		BolehUpload:      boleh,
		FilePath:         fp,
		DibuatOlehUserID: uid,
		ModulID:          formPtr(c.FormValue("modulId")),
	}
	if t.Judul == "" {
		return fiber.NewError(400, "judul wajib diisi")
	}
	if e := s.db.Create(&t).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "tugas", t.ID)
	// Notify wali kelas about new tugas
	var kelas Kelas
	if s.db.First(&kelas, "id = ?", kelasID).Error == nil && kelas.WaliKelasID != nil {
		var tutor Tutor
		if s.db.First(&tutor, "id = ?", *kelas.WaliKelasID).Error == nil && tutor.UserID != nil {
			s.pushNotifikasi(*tutor.UserID, "Tugas Baru", fmt.Sprintf("Tugas \"%s\" telah dibuat untuk kelas %d%s", t.Judul, kelas.Jenjang, kelas.NamaRombel), "tugas", &t.ID)
		}
	}
	return c.Status(201).JSON(t)
}

func (s *Server) updateTugas(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && t.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	if v := c.FormValue("mapelId"); v != "" {
		t.MapelID = v
	}
	if v := c.FormValue("kelasId"); v != "" && v != t.KelasID {
		if e := s.canManageKelas(c, v); e != nil {
			return e
		}
		t.KelasID = v
	}
	if v := c.FormValue("judul"); v != "" {
		t.Judul = v
	}
	t.Deskripsi = c.FormValue("deskripsi")
	t.ModulID = formPtr(c.FormValue("modulId"))
	if v := c.FormValue("deadline"); v != "" {
		if d, e := parseFormTime(v); e == nil {
			t.Deadline = d
			t.Semester = s.semester(d)
		} else {
			return fiber.NewError(400, e.Error())
		}
	}
	if v := c.FormValue("bolehUpload"); v != "" {
		t.BolehUpload = v != "false"
	}
	path, e := s.saveUpload(c, "file", "tugas", 10*1024*1024, []string{"pdf", "docx", "doc", "xlsx", "xls", "png", "jpg", "jpeg"})
	if e != nil {
		return e
	}
	if path != "" {
		t.FilePath = &path
	}
	if e := s.db.Save(&t).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "tugas", t.ID)
	return c.JSON(t)
}

func (s *Server) deleteTugas(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && t.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	// Cascade dalam transaksi: bila hapus pengumpulan gagal, parent tidak
	// ikut terhapus (sebelumnya error child ditelan → orphan pengumpulan).
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tugas_id = ?", t.ID).Delete(&PengumpulanTugas{}).Error; err != nil {
			return err
		}
		return tx.Delete(&t).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "tugas", t.ID)
	return c.SendStatus(204)
}

func (s *Server) scopeTugas(c *fiber.Ctx, t *Tugas) error {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil
	}
	return s.canManageKelas(c, t.KelasID)
}

func (s *Server) listPengumpulan(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tugas not found")
	}
	if e := s.scopeTugas(c, &t); e != nil {
		return e
	}
	var rows []PengumpulanTugas
	s.db.Preload("PesertaDidik").Where("tugas_id = ?", t.ID).Find(&rows)
	return c.JSON(rows)
}

func (s *Server) createPengumpulan(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tugas not found")
	}
	if e := s.scopeTugas(c, &t); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	pdID := c.FormValue("pesertaDidikId")
	if pdID == "" {
		return fiber.NewError(400, "pesertaDidikId wajib diisi")
	}
	// siswa harus berada di kelas tugas
	var pd PesertaDidik
	if s.db.First(&pd, "id = ?", pdID).Error != nil {
		return fiber.NewError(400, "peserta didik tidak ditemukan")
	}
	if pd.KelasID != t.KelasID {
		return fiber.NewError(400, "peserta didik bukan dari kelas tugas ini")
	}
	jawaban := c.FormValue("jawabanTeks")
	path, e := s.saveUpload(c, "file", "tugas", 10*1024*1024, []string{"pdf", "docx", "doc", "xlsx", "xls", "png", "jpg", "jpeg"})
	if e != nil {
		return e
	}
	now := time.Now()
	status := "Terkumpul"
	if now.After(t.Deadline) {
		status = "Terlambat"
	}
	var pk PengumpulanTugas
	found := s.db.Where("tugas_id = ? AND peserta_didik_id = ?", t.ID, pdID).First(&pk).Error == nil
	if found {
		if pk.Status == "Dinilai" {
			return fiber.NewError(400, "pengumpulan sudah dinilai, tidak dapat diubah")
		}
		pk.TanggalKumpul = now
		pk.JawabanTeks = jawaban
		if path != "" {
			pk.FilePath = &path
		}
		pk.Status = status
		if e := s.db.Save(&pk).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	} else {
		pk = PengumpulanTugas{
			TugasID:        t.ID,
			PesertaDidikID: pdID,
			TanggalKumpul:  now,
			JawabanTeks:    jawaban,
			Status:         status,
		}
		if path != "" {
			pk.FilePath = &path
		}
		if e := s.db.Create(&pk).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	}
	s.audit(&uid, "create", "pengumpulan", pk.ID)
	return c.Status(201).JSON(pk)
}

func (s *Server) nilaiPengumpulan(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tugas not found")
	}
	if e := s.scopeTugas(c, &t); e != nil {
		return e
	}
	var in struct {
		PesertaDidikID string  `json:"pesertaDidikId"`
		Nilai          float64 `json:"nilai"`
		CatatanTutor   string  `json:"catatanTutor"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.PesertaDidikID == "" {
		return fiber.NewError(400, "pesertaDidikId wajib diisi")
	}
	var pk PengumpulanTugas
	if s.db.Where("tugas_id = ? AND peserta_didik_id = ?", t.ID, in.PesertaDidikID).First(&pk).Error != nil {
		return fiber.NewError(404, "pengumpulan tidak ditemukan")
	}
	uid := c.Locals("userID").(string)
	n := in.Nilai
	pk.Nilai = &n
	pk.CatatanTutor = in.CatatanTutor
	pk.Status = "Dinilai"
	pk.DinilaiOlehUserID = &uid
	if e := s.db.Save(&pk).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "nilai", "pengumpulan", pk.ID)
	return c.JSON(pk)
}

func (s *Server) tugasLampiran(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tugas not found")
	}
	if e := s.scopeTugas(c, &t); e != nil {
		return e
	}
	if t.FilePath == nil {
		return fiber.NewError(404, "tugas tidak memiliki lampiran")
	}
	return s.sendUpload(c, *t.FilePath)
}

func (s *Server) pengumpulanFile(c *fiber.Ctx) error {
	var t Tugas
	if e := s.db.First(&t, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tugas not found")
	}
	if e := s.scopeTugas(c, &t); e != nil {
		return e
	}
	var pk PengumpulanTugas
	if s.db.Where("id = ? AND tugas_id = ?", c.Params("pid"), t.ID).First(&pk).Error != nil {
		return fiber.NewError(404, "pengumpulan not found")
	}
	if pk.FilePath == nil {
		return fiber.NewError(404, "pengumpulan tidak memiliki file")
	}
	return s.sendUpload(c, *pk.FilePath)
}

// ---------------------------------------------------------------------------
// Modul E — Materi Pembelajaran (prd_fitur_simpkbm.md). Tutor upload materi per
// mapel+kelas; download & komentar scoped (admin/kepala bebas, guru via wali kelas).
// ---------------------------------------------------------------------------

func (s *Server) listMateri(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Preload("Kelas").Order("urutan asc, created_at desc")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	ids, ok := s.waliKelasIDs(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil {
		if len(ids) == 0 {
			return c.JSON([]Materi{})
		}
		q = q.Where("kelas_id IN ?", ids)
	}
	var rows []Materi
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

var materiExts = []string{"pdf", "docx", "doc", "xlsx", "xls", "pptx", "ppt", "png", "jpg", "jpeg", "mp4", "zip"}

func (s *Server) createMateri(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	kelasID := c.FormValue("kelasId")
	if kelasID == "" {
		return fiber.NewError(400, "kelasId wajib diisi")
	}
	if e := s.canManageKelas(c, kelasID); e != nil {
		return e
	}
	linkURL := strings.TrimSpace(c.FormValue("linkUrl"))
	judul := c.FormValue("judul")
	if judul == "" {
		return fiber.NewError(400, "judul wajib diisi")
	}
	fh, _ := c.FormFile("file")
	hasFile := fh != nil && fh.Size > 0
	if !hasFile && linkURL == "" {
		return fiber.NewError(400, "file atau link materi wajib diisi")
	}
	var (
		path   string
		ukuran int64
		tipe   string
	)
	if hasFile {
		ukuran = fh.Size
		tipe = strings.ToLower(filepath.Ext(fh.Filename))
		p, e := s.saveUpload(c, "file", "materi", 10*1024*1024, materiExts)
		if e != nil {
			return e
		}
		path = p
	}
	m := Materi{
		MapelID:          c.FormValue("mapelId"),
		KelasID:          kelasID,
		Judul:            judul,
		Deskripsi:        c.FormValue("deskripsi"),
		FilePath:         path,
		Tipe:             tipe,
		Ukuran:           ukuran,
		Semester:         s.semester(time.Now()),
		DibuatOlehUserID: uid,
		ModulID:          formPtr(c.FormValue("modulId")),
		LinkURL:          linkURL,
		Urutan:           formInt(c.FormValue("urutan")),
		Tanggal:          formDatePtr(c.FormValue("tanggal")),
	}
	if e := s.db.Create(&m).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "materi", m.ID)
	return c.Status(201).JSON(m)
}

func (s *Server) updateMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && m.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	if v := c.FormValue("mapelId"); v != "" {
		m.MapelID = v
	}
	if v := c.FormValue("kelasId"); v != "" && v != m.KelasID {
		if e := s.canManageKelas(c, v); e != nil {
			return e
		}
		m.KelasID = v
	}
	if v := c.FormValue("judul"); v != "" {
		m.Judul = v
	}
	m.Deskripsi = c.FormValue("deskripsi")
	m.ModulID = formPtr(c.FormValue("modulId"))
	if v := c.FormValue("linkUrl"); v != "" || c.FormValue("linkUrlCleared") == "1" {
		m.LinkURL = strings.TrimSpace(v)
	}
	if v := c.FormValue("urutan"); v != "" {
		m.Urutan = formInt(v)
	}
	if v := c.FormValue("tanggal"); v != "" || c.FormValue("tanggalCleared") == "1" {
		m.Tanggal = formDatePtr(v)
	}
	path, e := s.saveUpload(c, "file", "materi", 10*1024*1024, materiExts)
	if e != nil {
		return e
	}
	if path != "" {
		fh, _ := c.FormFile("file")
		m.FilePath = path
		if fh != nil {
			m.Ukuran = fh.Size
			m.Tipe = strings.ToLower(filepath.Ext(fh.Filename))
		}
	}
	if e := s.db.Save(&m).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "materi", m.ID)
	return c.JSON(m)
}

func (s *Server) deleteMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && m.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("materi_id = ?", m.ID).Delete(&KomentarMateri{}).Error; err != nil {
			return err
		}
		return tx.Delete(&m).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "materi", m.ID)
	return c.SendStatus(204)
}

func (s *Server) getMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.Preload("Mapel").Preload("Kelas").Preload("Komentar").First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := s.scopeMateri(c, &m); e != nil {
		return e
	}
	return c.JSON(m)
}

func (s *Server) scopeMateri(c *fiber.Ctx, m *Materi) error {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil
	}
	return s.canManageKelas(c, m.KelasID)
}

func (s *Server) downloadMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := s.scopeMateri(c, &m); e != nil {
		return e
	}
	return s.sendUpload(c, m.FilePath)
}

// ---------------------------------------------------------------------------
// Modul R — RPP (Rencana Pelaksanaan Pembelajaran). Distribusi per-jenjang:
// 1 RPP dibuat penyu­sun (IsRPPMaker) untuk suatu mapel+jenjang, dipakai bersama
// seluruh rombel jenjang itu. Tutor pengajar jenjang tsb bisa lihat & download.
// ---------------------------------------------------------------------------

var rppExts = []string{"pdf", "docx", "doc"}

// rppJenjangsFor mengembalikan daftar jenjang (tingkat) yang diajar guru saat ini:
// kelas tempat dia wali ATAU punya PenugasanGuruMapel. Untuk admin/kepala → (nil,true)
// artinya "semua jenjang". (false) berarti tak ada profil tutor.
func (s *Server) rppJenjangsFor(c *fiber.Ctx) ([]int, bool) {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil, true
	}
	var u User
	if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
		return nil, false
	}
	set := map[int]bool{}
	var kelasWali []Kelas
	s.db.Where("wali_kelas_id = ?", *u.TutorID).Find(&kelasWali)
	for _, k := range kelasWali {
		set[k.Jenjang] = true
	}
	var penugasan []PenugasanGuruMapel
	s.db.Preload("Kelas").Where("tutor_id = ?", *u.TutorID).Find(&penugasan)
	for _, p := range penugasan {
		if p.Kelas != nil {
			set[p.Kelas.Jenjang] = true
		}
	}
	out := make([]int, 0, len(set))
	for j := range set {
		out = append(out, j)
	}
	return out, true
}

// isRppMaker: admin selalu true; guru lain → cek Tutor.IsRPPMaker.
func (s *Server) isRppMaker(c *fiber.Ctx) bool {
	if c.Locals("role") == "admin" {
		return true
	}
	var u User
	if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
		return false
	}
	var t Tutor
	if s.db.First(&t, "id = ?", *u.TutorID).Error != nil {
		return false
	}
	return t.IsRPPMaker
}

func (s *Server) rppMakerStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"isRppMaker": s.isRppMaker(c)})
}

// rppOptions mengembalikan opsi dropdown utk form RPP (mapel, jenjang, tahun ajaran,
// fase) dengan scope sesuai peran. Admin/kepala: semua. Guru: mapel dari Penugasan-
// GuruMapel + jenjang/TA/fase dari kelas walinya/penugasannya. Berbeda dgn /mapel,
// /tahun-ajaran, /fase (admin-only via readAll), endpoint ini guru-accessible agar
// penyusun RPP bisa mengisi identitas tanpa akses management read.
func (s *Server) rppOptions(c *fiber.Ctx) error {
	type mapelOpt struct {
		ID   string `json:"id"`
		Nama string `json:"nama"`
	}
	type taOpt struct {
		ID      string `json:"id"`
		Nama    string `json:"nama"`
		IsAktif bool   `json:"isAktif"`
	}
	type faseOpt struct {
		ID   string `json:"id"`
		Kode string `json:"kode"`
		Nama string `json:"nama"`
	}

	isAdmin := c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah"
	var mapelOpts []mapelOpt
	var taOpts []taOpt
	var faseOpts []faseOpt
	jenjangSet := map[int]bool{}
	taSet := map[string]bool{}
	faseIDSet := map[string]bool{}

	// addKelas mengumpulkan jenjang, tahun ajaran (preloaded), dan faseID dari sebuah kelas.
	addKelas := func(k Kelas) {
		if k.ID == "" {
			return
		}
		jenjangSet[k.Jenjang] = true
		if k.TahunAjaranID != "" && !taSet[k.TahunAjaranID] {
			taSet[k.TahunAjaranID] = true
			taOpts = append(taOpts, taOpt{ID: k.TahunAjaranID, Nama: k.TahunAjaran.NamaTahunAjaran, IsAktif: k.TahunAjaran.IsAktif})
		}
		if k.FaseID != nil {
			faseIDSet[*k.FaseID] = true
		}
	}

	if isAdmin {
		var ms []MataPelajaran
		s.db.Order("nama_mapel").Find(&ms)
		for _, m := range ms {
			mapelOpts = append(mapelOpts, mapelOpt{ID: m.ID, Nama: m.NamaMapel})
		}
		var ks []Kelas
		s.db.Preload("TahunAjaran").Find(&ks)
		for _, k := range ks {
			addKelas(k)
		}
		// admin lihat semua tahun ajaran & fase (bukan hanya yg direferensikan kelas)
		taOpts = nil
		var tas []TahunAjaran
		s.db.Order("tanggal_mulai desc").Find(&tas)
		for _, t := range tas {
			taOpts = append(taOpts, taOpt{ID: t.ID, Nama: t.NamaTahunAjaran, IsAktif: t.IsAktif})
		}
		var fs []Fase
		s.db.Order("kode").Find(&fs)
		for _, f := range fs {
			faseOpts = append(faseOpts, faseOpt{ID: f.ID, Kode: f.Kode, Nama: f.Nama})
		}
	} else {
		uid := c.Locals("userID").(string)
		var u User
		if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
			return fiber.NewError(403, "no tutor profile")
		}
		tid := *u.TutorID
		var pen []PenugasanGuruMapel
		s.db.Preload("Mapel").Preload("Kelas.TahunAjaran").Where("tutor_id = ?", tid).Find(&pen)
		mapelSeen := map[string]bool{}
		kelasSeen := map[string]bool{}
		for _, p := range pen {
			if p.Mapel.ID != "" && !mapelSeen[p.MapelID] {
				mapelSeen[p.MapelID] = true
				mapelOpts = append(mapelOpts, mapelOpt{ID: p.Mapel.ID, Nama: p.Mapel.NamaMapel})
			}
			if p.Kelas != nil && !kelasSeen[p.KelasID] {
				kelasSeen[p.KelasID] = true
				addKelas(*p.Kelas)
			}
		}
		var kelasWali []Kelas
		s.db.Preload("TahunAjaran").Where("wali_kelas_id = ?", tid).Find(&kelasWali)
		for _, k := range kelasWali {
			if !kelasSeen[k.ID] {
				kelasSeen[k.ID] = true
				addKelas(k)
			}
		}
		if len(faseIDSet) > 0 {
			ids := make([]string, 0, len(faseIDSet))
			for id := range faseIDSet {
				ids = append(ids, id)
			}
			var fs []Fase
			s.db.Where("id IN ?", ids).Order("kode").Find(&fs)
			for _, f := range fs {
				faseOpts = append(faseOpts, faseOpt{ID: f.ID, Kode: f.Kode, Nama: f.Nama})
			}
		}
	}

	jenjangs := make([]int, 0, len(jenjangSet))
	for j := range jenjangSet {
		jenjangs = append(jenjangs, j)
	}
	sort.Ints(jenjangs)
	activeTA := ""
	for _, t := range taOpts {
		if t.IsAktif {
			activeTA = t.ID
			break
		}
	}
	if activeTA == "" && len(taOpts) > 0 {
		activeTA = taOpts[0].ID
	}
	return c.JSON(fiber.Map{
		"mapel":               mapelOpts,
		"jenjang":             jenjangs,
		"tahunAjaran":         taOpts,
		"fase":                faseOpts,
		"activeTahunAjaranId": activeTA,
	})
}

func (s *Server) listRPP(c *fiber.Ctx) error {
	q := s.db.Preload("Tutor").Preload("Mapel").Preload("TahunAjaran").Preload("Fase").
		Order("tahun_ajaran_id desc, jenjang asc, mapel_id asc, created_at desc")
	if v := c.Query("jenjang"); v != "" {
		q = q.Where("jenjang = ?", formInt(v))
	}
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	if v := c.Query("tahunAjaranId"); v != "" {
		q = q.Where("tahun_ajaran_id = ?", v)
	}
	ids, ok := s.rppJenjangsFor(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil {
		uid := c.Locals("userID").(string)
		if len(ids) == 0 {
			// guru tanpa penugasan/wali: tetap bisa lihat RPP miliknya sendiri
			q = q.Where("dibuat_oleh_user_id = ?", uid)
		} else {
			q = q.Where("jenjang IN ? OR dibuat_oleh_user_id = ?", ids, uid)
		}
	}
	var rows []RPP
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createRPP(c *fiber.Ctx) error {
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	if !s.isRppMaker(c) {
		return fiber.NewError(403, "hanya penyusun RPP yang ditugaskan admin yang dapat mengunggah RPP")
	}
	mapelID := c.FormValue("mapelId")
	jenjang := formInt(c.FormValue("jenjang"))
	tahunAjaranID := c.FormValue("tahunAjaranId")
	judul := strings.TrimSpace(c.FormValue("judul"))
	if mapelID == "" || jenjang == 0 || tahunAjaranID == "" || judul == "" {
		return fiber.NewError(400, "mapelId, jenjang, tahunAjaranId dan judul wajib diisi")
	}
	fh, _ := c.FormFile("file")
	hasFile := fh != nil && fh.Size > 0
	if !hasFile {
		return fiber.NewError(400, "file RPP wajib diunggah (PDF/Word)")
	}
	var (
		ukuran int64
		tipe   string
	)
	ukuran = fh.Size
	tipe = strings.ToLower(filepath.Ext(fh.Filename))
	path, e := s.saveUpload(c, "file", "rpp", 10*1024*1024, rppExts)
	if e != nil {
		return e
	}
	if path == "" {
		return fiber.NewError(400, "file RPP wajib diunggah (PDF/Word)")
	}
	pertemuan := formInt(c.FormValue("pertemuanKe"))
	var pertemuanPtr *int
	if pertemuan > 0 {
		pertemuanPtr = &pertemuan
	}
	r := RPP{
		TutorID:          *u.TutorID,
		DibuatOlehUserID: uid,
		MapelID:          mapelID,
		Jenjang:          jenjang,
		TahunAjaranID:    tahunAjaranID,
		FaseID:           formPtr(c.FormValue("faseId")),
		Semester:         s.semester(time.Now()),
		Judul:            judul,
		PertemuanKe:      pertemuanPtr,
		AlokasiWaktu:     strings.TrimSpace(c.FormValue("alokasiWaktu")),
		Tanggal:          formDatePtr(c.FormValue("tanggal")),
		Deskripsi:        c.FormValue("deskripsi"),
		FilePath:         path,
		Tipe:             tipe,
		Ukuran:           ukuran,
	}
	if e := s.db.Create(&r).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "rpp", r.ID)
	return c.Status(201).JSON(r)
}

func (s *Server) updateRPP(c *fiber.Ctx) error {
	var r RPP
	if e := s.db.First(&r, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && r.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	if v := c.FormValue("mapelId"); v != "" {
		r.MapelID = v
	}
	if v := c.FormValue("jenjang"); v != "" {
		r.Jenjang = formInt(v)
	}
	if v := c.FormValue("tahunAjaranId"); v != "" {
		r.TahunAjaranID = v
	}
	if v := c.FormValue("faseId"); v != "" || c.FormValue("faseIdCleared") == "1" {
		r.FaseID = formPtr(v)
	}
	if v := c.FormValue("judul"); v != "" {
		r.Judul = strings.TrimSpace(v)
	}
	pertemuan := formInt(c.FormValue("pertemuanKe"))
	if c.FormValue("pertemuanKe") != "" {
		if pertemuan > 0 {
			r.PertemuanKe = &pertemuan
		} else {
			r.PertemuanKe = nil
		}
	}
	r.AlokasiWaktu = strings.TrimSpace(c.FormValue("alokasiWaktu"))
	if v := c.FormValue("tanggal"); v != "" || c.FormValue("tanggalCleared") == "1" {
		r.Tanggal = formDatePtr(v)
	}
	r.Deskripsi = c.FormValue("deskripsi")
	path, e := s.saveUpload(c, "file", "rpp", 10*1024*1024, rppExts)
	if e != nil {
		return e
	}
	if path != "" {
		if r.FilePath != "" {
			os.Remove("./" + r.FilePath) // best-effort hapus file lama
		}
		fh, _ := c.FormFile("file")
		r.FilePath = path
		if fh != nil {
			r.Ukuran = fh.Size
			r.Tipe = strings.ToLower(filepath.Ext(fh.Filename))
		}
	}
	if e := s.db.Save(&r).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "rpp", r.ID)
	return c.JSON(r)
}

func (s *Server) deleteRPP(c *fiber.Ctx) error {
	var r RPP
	if e := s.db.First(&r, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && r.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	if r.FilePath != "" {
		os.Remove("./" + r.FilePath) // best-effort
	}
	if e := s.db.Delete(&r).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "rpp", r.ID)
	return c.SendStatus(204)
}

func (s *Server) downloadRPP(c *fiber.Ctx) error {
	var r RPP
	if e := s.db.First(&r, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	// scope: admin/kepala pass; guru harus mengajar jenjang tsb ATAU pemilik.
	if c.Locals("role") != "admin" && c.Locals("role") != "kepala_sekolah" {
		uid := c.Locals("userID").(string)
		if r.DibuatOlehUserID == uid {
			return s.sendUpload(c, r.FilePath)
		}
		ids, ok := s.rppJenjangsFor(c)
		if !ok {
			return fiber.NewError(403, "no tutor profile")
		}
		allowed := false
		for _, j := range ids {
			if j == r.Jenjang {
				allowed = true
				break
			}
		}
		if !allowed {
			return fiber.NewError(403, "RPP jenjang ini tidak dapat diakses")
		}
	}
	return s.sendUpload(c, r.FilePath)
}

func (s *Server) komentarMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := s.scopeMateri(c, &m); e != nil {
		return e
	}
	var in struct {
		Isi string `json:"isi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if strings.TrimSpace(in.Isi) == "" {
		return fiber.NewError(400, "isi komentar wajib diisi")
	}
	uid := c.Locals("userID").(string)
	k := KomentarMateri{MateriID: m.ID, UserID: &uid, Isi: in.Isi}
	if e := s.db.Create(&k).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "komentar_materi", k.ID)
	return c.Status(201).JSON(k)
}

// getMateriShare returns the current share state (enabled/protected/shareUrl)
// for a materi. Read access uses scopeMateri (owner-or-admin for protected info).
func (s *Server) getMateriShare(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := s.scopeMateri(c, &m); e != nil {
		return e
	}
	resp := fiber.Map{"enabled": m.ShareToken != nil, "protected": m.SharePasswordHash != nil}
	if m.ShareToken != nil {
		resp["shareUrl"] = publicBase() + "/api/materi/share/" + *m.ShareToken
	}
	return c.JSON(resp)
}

// shareMateri toggles a public/password-protected share link for a materi.
// Only the owner (DibuatOlehUserID) or admin may share. Password (if any) is
// bcrypt-hashed and stored on SharePasswordHash; an empty password means public.
func (s *Server) shareMateri(c *fiber.Ctx) error {
	var m Materi
	if e := s.db.First(&m, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && m.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat membagikan")
	}
	var in struct {
		Enabled  bool   `json:"enabled"`
		Password string `json:"password"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Enabled {
		tok := uuid.NewString()
		m.ShareToken = &tok
		if pw := strings.TrimSpace(in.Password); pw != "" {
			h, e := bcryptHash(pw)
			if e != nil {
				return fiber.NewError(500, "gagal meng-hash password")
			}
			m.SharePasswordHash = &h
		} else {
			m.SharePasswordHash = nil
		}
	} else {
		m.ShareToken = nil
		m.SharePasswordHash = nil
	}
	if e := s.db.Save(&m).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "materi_share", m.ID)
	resp := fiber.Map{"enabled": in.Enabled, "protected": m.SharePasswordHash != nil}
	if m.ShareToken != nil {
		resp["shareUrl"] = publicBase() + "/api/materi/share/" + *m.ShareToken
		resp["shareToken"] = *m.ShareToken
	}
	return c.JSON(resp)
}

// viewSharedMateri renders a public HTML page for a shared materi (no auth).
// If the materi is password-protected and no/wrong ?pwd= is supplied, a
// password form is rendered instead of the content.
func (s *Server) viewSharedMateri(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return fiber.NewError(404, "materi tidak ditemukan")
	}
	var m Materi
	if e := s.db.Preload("Mapel").First(&m, "share_token = ?", token).Error; e != nil {
		return fiber.NewError(404, "materi tidak ditemukan")
	}
	protected := m.SharePasswordHash != nil
	pwd := c.Query("pwd")
	if protected {
		if pwd == "" || bcrypt.CompareHashAndPassword([]byte(*m.SharePasswordHash), []byte(pwd)) != nil {
			c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
			return c.SendString(renderMateriSharePasswordHTML(&m))
		}
	}
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(renderMateriShareHTML(&m, pwd))
}

// downloadSharedMateri streams the shared materi's file (no auth). If password-
// protected, a correct ?pwd= is required.
func (s *Server) downloadSharedMateri(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return fiber.NewError(404, "materi tidak ditemukan")
	}
	var m Materi
	if e := s.db.First(&m, "share_token = ?", token).Error; e != nil {
		return fiber.NewError(404, "materi tidak ditemukan")
	}
	if m.FilePath == "" {
		return fiber.NewError(404, "materi ini tidak memiliki file")
	}
	if m.SharePasswordHash != nil {
		pwd := c.Query("pwd")
		if pwd == "" || bcrypt.CompareHashAndPassword([]byte(*m.SharePasswordHash), []byte(pwd)) != nil {
			return fiber.NewError(401, "password salah")
		}
	}
	return s.sendUpload(c, m.FilePath)
}

// renderMateriShareHTML builds the public content page for a shared materi.
// pwd is forwarded to the file-download link when the materi is protected.
func renderMateriShareHTML(m *Materi, pwd string) string {
	esc := html.EscapeString
	title := esc(m.Judul)
	desc := esc(m.Deskripsi)
	mapel := ""
	if m.Mapel.ID != "" {
		mapel = esc(m.Mapel.NamaMapel)
	}
	var linkBlock string
	if u := strings.TrimSpace(m.LinkURL); u != "" {
		linkBlock = `<a class="btn" href="` + esc(u) + `" target="_blank" rel="noopener noreferrer">Buka Link Materi &#8599;</a>`
	}
	var fileBlock string
	if m.FilePath != "" {
		dl := publicBase() + "/api/materi/share/" + *m.ShareToken + "/file"
		if pwd != "" {
			dl += "?pwd=" + urlQueryEscape(pwd)
		}
		fileBlock = `<a class="btn btn-primary" href="` + esc(dl) + `">Unduh File` + fileLabel(m) + `</a>`
	}
	body := fileBlock + linkBlock
	if body == "" {
		body = `<p class="muted">Materi ini tidak memiliki file maupun link.</p>`
	}
	return `<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + title + ` &mdash; PKBM Tunas Ilmu</title>
<style>
  :root { --brand:#1c5740; --gold:#d4af37; }
  * { box-sizing: border-box; }
  body { margin:0; font-family: -apple-system, Segoe UI, Roboto, Arial, sans-serif; background:#f5f7f6; color:#222; }
  .wrap { max-width:720px; margin:32px auto; padding:0 16px; }
  .card { background:#fff; border:1px solid #e5e7eb; border-radius:16px; overflow:hidden; box-shadow:0 1px 3px rgba(0,0,0,.06); }
  .head { background:var(--brand); color:#fff; padding:18px 24px; }
  .head .org { font-size:12px; letter-spacing:.12em; text-transform:uppercase; opacity:.85; }
  .head .name { font-size:20px; font-weight:700; margin-top:2px; }
  .gold { height:3px; background:var(--gold); }
  .pad { padding:24px; }
  h1 { font-size:22px; margin:0 0 4px; }
  .meta { color:#666; font-size:13px; margin:0 0 16px; }
  .desc { white-space:pre-wrap; line-height:1.6; color:#333; margin:0 0 24px; }
  .btn { display:inline-block; padding:11px 18px; border-radius:10px; text-decoration:none; font-weight:600; font-size:14px; margin:0 8px 8px 0; border:1px solid #d1d5db; color:#1c5740; background:#fff; }
  .btn-primary { background:#1c5740; color:#fff; border-color:#1c5740; }
  .muted { color:#888; }
  .foot { padding:16px 24px; border-top:1px solid #f0f0f0; color:#999; font-size:12px; text-align:center; }
</style></head>
<body><div class="wrap"><div class="card">
  <div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Materi Pembelajaran</div></div>
  <div class="gold"></div>
  <div class="pad">
    <h1>` + title + `</h1>
    <p class="meta">` + mapel + `</p>
    <div class="desc">` + desc + `</div>
    <div>` + body + `</div>
  </div>
  <div class="foot">Dibagikan via PKBM Tunas Ilmu Learn</div>
</div></div></body></html>`
}

// renderMateriSharePasswordHTML builds the password-gate form for a protected
// shared materi. Submitting reloads the same URL with ?pwd=.
func renderMateriSharePasswordHTML(m *Materi) string {
	esc := html.EscapeString
	title := esc(m.Judul)
	action := publicBase() + "/api/materi/share/" + *m.ShareToken
	return `<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + title + ` &mdash; PKBM Tunas Ilmu</title>
<style>
  :root { --brand:#1c5740; --gold:#d4af37; }
  * { box-sizing: border-box; }
  body { margin:0; font-family: -apple-system, Segoe UI, Roboto, Arial, sans-serif; background:#f5f7f6; color:#222; }
  .wrap { max-width:420px; margin:64px auto; padding:0 16px; }
  .card { background:#fff; border:1px solid #e5e7eb; border-radius:16px; overflow:hidden; box-shadow:0 1px 3px rgba(0,0,0,.06); }
  .head { background:var(--brand); color:#fff; padding:18px 24px; }
  .head .org { font-size:12px; letter-spacing:.12em; text-transform:uppercase; opacity:.85; }
  .head .name { font-size:20px; font-weight:700; margin-top:2px; }
  .gold { height:3px; background:var(--gold); }
  .pad { padding:24px; }
  p { color:#555; font-size:14px; line-height:1.5; }
  label { display:block; font-size:13px; font-weight:600; margin:16px 0 6px; }
  input { width:100%; padding:11px 12px; border:1px solid #d1d5db; border-radius:10px; font-size:15px; }
  .btn { display:block; width:100%; margin-top:18px; padding:12px; border:0; border-radius:10px; background:#1c5740; color:#fff; font-weight:700; font-size:15px; cursor:pointer; }
</style></head>
<body><div class="wrap"><div class="card">
  <div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Materi Terproteksi</div></div>
  <div class="gold"></div>
  <div class="pad">
    <h1 style="margin:0 0 8px;font-size:18px;">` + title + `</h1>
    <p>Materi ini diproteksi password. Masukkan password untuk membuka.</p>
    <form method="get" action="` + esc(action) + `">
      <label for="pwd">Password</label>
      <input id="pwd" name="pwd" type="password" autofocus required>
      <button class="btn" type="submit">Buka Materi</button>
    </form>
  </div>
</div></div></body></html>`
}

func fileLabel(m *Materi) string {
	if m.Ukuran > 0 {
		return " (" + strings.ToUpper(strings.TrimPrefix(m.Tipe, ".")) + ", " + fmtSize(m.Ukuran) + ")"
	}
	return ""
}

// urlQueryEscape percent-encodes a string for use in a URL query parameter.
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// fmtSize formats a byte count as a human-readable string (e.g. "1.2 MB").
func fmtSize(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1024*1024:
		return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " KB"
	default:
		return strconv.FormatFloat(float64(n)/(1024*1024), 'f', 1, 64) + " MB"
	}
}

// ---------------------------------------------------------------------------
// Modul F — Kelas Virtual (prd_fitur_simpkbm.md). Jadwal kelas daring per
// mapel+kelas (link meeting). Guru scoped ke kelas walinya; admin bebas.
// ---------------------------------------------------------------------------

type kelasVirtualInput struct {
	MapelID      string    `json:"mapelId"`
	KelasID      string    `json:"kelasId"`
	Judul        string    `json:"judul"`
	Deskripsi    string    `json:"deskripsi"`
	LinkMeeting  string    `json:"linkMeeting"`
	WaktuMulai   time.Time `json:"waktuMulai"`
	WaktuSelesai time.Time `json:"waktuSelesai"`
}

func (s *Server) listKelasVirtual(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Preload("Kelas").Order("waktu_mulai desc")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	ids, ok := s.waliKelasIDs(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil {
		if len(ids) == 0 {
			return c.JSON([]KelasVirtual{})
		}
		q = q.Where("kelas_id IN ?", ids)
	}
	var rows []KelasVirtual
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createKelasVirtual(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	var in kelasVirtualInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul == "" || in.KelasID == "" || in.LinkMeeting == "" {
		return fiber.NewError(400, "judul, kelasId, dan linkMeeting wajib diisi")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	kv := KelasVirtual{
		MapelID:          in.MapelID,
		KelasID:          in.KelasID,
		Judul:            in.Judul,
		Deskripsi:        in.Deskripsi,
		LinkMeeting:      in.LinkMeeting,
		WaktuMulai:       in.WaktuMulai,
		WaktuSelesai:     in.WaktuSelesai,
		Semester:         s.semester(in.WaktuMulai),
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&kv).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "kelas_virtual", kv.ID)
	return c.Status(201).JSON(kv)
}

func (s *Server) updateKelasVirtual(c *fiber.Ctx) error {
	var kv KelasVirtual
	if e := s.db.First(&kv, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && kv.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	var in kelasVirtualInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID != "" && in.KelasID != kv.KelasID {
		if e := s.canManageKelas(c, in.KelasID); e != nil {
			return e
		}
		kv.KelasID = in.KelasID
	}
	if in.MapelID != "" {
		kv.MapelID = in.MapelID
	}
	if in.Judul != "" {
		kv.Judul = in.Judul
	}
	kv.Deskripsi = in.Deskripsi
	if in.LinkMeeting != "" {
		kv.LinkMeeting = in.LinkMeeting
	}
	if !in.WaktuMulai.IsZero() {
		kv.WaktuMulai = in.WaktuMulai
		kv.Semester = s.semester(in.WaktuMulai)
	}
	if !in.WaktuSelesai.IsZero() {
		kv.WaktuSelesai = in.WaktuSelesai
	}
	if e := s.db.Save(&kv).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "kelas_virtual", kv.ID)
	return c.JSON(kv)
}

func (s *Server) deleteKelasVirtual(c *fiber.Ctx) error {
	var kv KelasVirtual
	if e := s.db.First(&kv, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && kv.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	if e := s.db.Delete(&kv).Error; e != nil {
		return e
	}
	s.audit(&uid, "delete", "kelas_virtual", kv.ID)
	return c.SendStatus(204)
}

// ---------------------------------------------------------------------------
// Modul D — Bank Soal + Ujian Luring (prd_fitur_simpkbm.md). Tutor CRUD bank soal
// (scoped ke soal miliknya); menyusun ujian per kelas (canManageKelas). Cetak naskah
// & kunci via gofpdf; acak soal+opsi deterministik per ujianID. Kepala read-only.
// ---------------------------------------------------------------------------

type bankSoalInput struct {
	MapelID    string  `json:"mapelId"`
	Tipe       string  `json:"tipe"`
	Pertanyaan string  `json:"pertanyaan"`
	Opsi       string  `json:"opsi"` // JSON array string (untuk pg)
	Kunci      string  `json:"kunci"`
	Poin       float64 `json:"poin"`
}

func (s *Server) listBankSoal(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Order("created_at desc")
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	if c.Locals("role") == "guru" {
		q = q.Where("dibuat_oleh_user_id = ?", c.Locals("userID"))
	}
	var rows []BankSoal
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createBankSoal(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	var in bankSoalInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Tipe != "pg" && in.Tipe != "essay" {
		return fiber.NewError(400, "tipe harus pg atau essay")
	}
	if strings.TrimSpace(in.Pertanyaan) == "" {
		return fiber.NewError(400, "pertanyaan wajib diisi")
	}
	uid := c.Locals("userID").(string)
	b := BankSoal{
		MapelID:          in.MapelID,
		Tipe:             in.Tipe,
		Pertanyaan:       in.Pertanyaan,
		Opsi:             in.Opsi,
		Kunci:            in.Kunci,
		Poin:             in.Poin,
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&b).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "bank_soal", b.ID)
	return c.Status(201).JSON(b)
}

func (s *Server) updateBankSoal(c *fiber.Ctx) error {
	var b BankSoal
	if e := s.db.First(&b, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && b.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	var in bankSoalInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Tipe != "" && in.Tipe != "pg" && in.Tipe != "essay" {
		return fiber.NewError(400, "tipe harus pg atau essay")
	}
	if in.MapelID != "" {
		b.MapelID = in.MapelID
	}
	if in.Tipe != "" {
		b.Tipe = in.Tipe
	}
	if in.Pertanyaan != "" {
		b.Pertanyaan = in.Pertanyaan
	}
	b.Opsi = in.Opsi
	b.Kunci = in.Kunci
	b.Poin = in.Poin
	if e := s.db.Save(&b).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "bank_soal", b.ID)
	return c.JSON(b)
}

func (s *Server) deleteBankSoal(c *fiber.Ctx) error {
	var b BankSoal
	if e := s.db.First(&b, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && b.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	var cnt int64
	s.db.Model(&UjianSoal{}).Where("soal_id = ?", b.ID).Count(&cnt)
	if cnt > 0 {
		return fiber.NewError(400, "soal sedang dipakai dalam ujian, tidak dapat dihapus")
	}
	if e := s.db.Delete(&b).Error; e != nil {
		return e
	}
	s.audit(&uid, "delete", "bank_soal", b.ID)
	return c.SendStatus(204)
}

type ujianInput struct {
	MapelID          string    `json:"mapelId"`
	KelasID          string    `json:"kelasId"`
	Judul            string    `json:"judul"`
	WaktuMulai       time.Time `json:"waktuMulai"`
	WaktuSelesai     time.Time `json:"waktuSelesai"`
	DurasiMenit      int       `json:"durasiMenit"`
	GracePeriodMenit int       `json:"gracePeriodMenit"`
	BatasTabSwitch   int       `json:"batasTabSwitch"`
	AcakSoal         bool      `json:"acakSoal"`
	AksesKode        string    `json:"aksesKode"` // kode akses siswa ujian online
}

func (s *Server) scopeUjian(c *fiber.Ctx, u *Ujian) error {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil
	}
	return s.canManageKelas(c, u.KelasID)
}

func (s *Server) listUjian(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Preload("Kelas").Order("waktu_mulai desc")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	ids, ok := s.waliKelasIDs(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil {
		if len(ids) == 0 {
			return c.JSON([]Ujian{})
		}
		q = q.Where("kelas_id IN ?", ids)
	}
	var rows []Ujian
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createUjian(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	uid := c.Locals("userID").(string)
	var u User
	if s.db.First(&u, "id = ?", uid).Error != nil || u.TutorID == nil {
		return fiber.NewError(403, "no tutor profile")
	}
	var in ujianInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul == "" || in.KelasID == "" {
		return fiber.NewError(400, "judul dan kelasId wajib diisi")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	uj := Ujian{
		MapelID:          in.MapelID,
		KelasID:          in.KelasID,
		Judul:            in.Judul,
		WaktuMulai:       in.WaktuMulai,
		WaktuSelesai:     in.WaktuSelesai,
		DurasiMenit:      in.DurasiMenit,
		GracePeriodMenit: in.GracePeriodMenit,
		BatasTabSwitch:   in.BatasTabSwitch,
		AcakSoal:         in.AcakSoal,
		AksesKode:        in.AksesKode,
		Semester:         s.semester(in.WaktuMulai),
		DibuatOlehUserID: uid,
	}
	if e := s.db.Create(&uj).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "ujian", uj.ID)
	s.notifyNewUjian(&uj)
	return c.Status(201).JSON(uj)
}

func (s *Server) updateUjian(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && uj.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat mengubah")
	}
	var in ujianInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID != "" && in.KelasID != uj.KelasID {
		if e := s.canManageKelas(c, in.KelasID); e != nil {
			return e
		}
		uj.KelasID = in.KelasID
	}
	if in.MapelID != "" {
		uj.MapelID = in.MapelID
	}
	if in.Judul != "" {
		uj.Judul = in.Judul
	}
	if !in.WaktuMulai.IsZero() {
		uj.WaktuMulai = in.WaktuMulai
		uj.Semester = s.semester(in.WaktuMulai)
	}
	if !in.WaktuSelesai.IsZero() {
		uj.WaktuSelesai = in.WaktuSelesai
	}
	uj.DurasiMenit = in.DurasiMenit
	uj.GracePeriodMenit = in.GracePeriodMenit
	uj.BatasTabSwitch = in.BatasTabSwitch
	uj.AcakSoal = in.AcakSoal
	uj.AksesKode = in.AksesKode
	if e := s.db.Save(&uj).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "update", "ujian", uj.ID)
	return c.JSON(uj)
}

func (s *Server) deleteUjian(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	uid := c.Locals("userID").(string)
	if c.Locals("role") != "admin" && uj.DibuatOlehUserID != uid {
		return fiber.NewError(403, "hanya pembuat atau admin yang dapat menghapus")
	}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ujian_id = ?", uj.ID).Delete(&UjianSoal{}).Error; err != nil {
			return err
		}
		return tx.Delete(&uj).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "delete", "ujian", uj.ID)
	return c.SendStatus(204)
}

func (s *Server) listUjianSoal(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "ujian not found")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	var rows []UjianSoal
	s.db.Preload("Soal").Preload("Soal.Mapel").Where("ujian_id = ?", uj.ID).Order("created_at").Find(&rows)
	return c.JSON(rows)
}

func (s *Server) addUjianSoal(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "ujian not found")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	var in struct {
		SoalID string  `json:"soalId"`
		Bobot  float64 `json:"bobot"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.SoalID == "" {
		return fiber.NewError(400, "soalId wajib diisi")
	}
	var b BankSoal
	if s.db.First(&b, "id = ?", in.SoalID).Error != nil {
		return fiber.NewError(400, "soal tidak ditemukan")
	}
	// upsert via uniqueIndex (ujianId+soalId)
	var us UjianSoal
	if s.db.Where("ujian_id = ? AND soal_id = ?", uj.ID, in.SoalID).First(&us).Error == nil {
		us.Bobot = in.Bobot
		s.db.Save(&us)
	} else {
		us = UjianSoal{UjianID: uj.ID, SoalID: in.SoalID, Bobot: in.Bobot}
		if e := s.db.Create(&us).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "ujian_soal", us.ID)
	return c.Status(201).JSON(us)
}

func (s *Server) deleteUjianSoal(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "ujian not found")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	if e := s.db.Where("ujian_id = ? AND soal_id = ?", uj.ID, c.Params("sid")).Delete(&UjianSoal{}).Error; e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "ujian_soal", c.Params("sid"))
	return c.SendStatus(204)
}

// seedFromID derives a stable int64 seed from a UUID-ish string so that randomization
// (AcakSoal) is deterministic per ujianID — the naskah and kunci copies always match.
func seedFromID(s string) int64 {
	var n int64
	for _, b := range []byte(s) {
		n = n*31 + int64(b)
	}
	return n
}

// shuffleOpsi deterministically reorders PG options and returns the new position of
// the originally-correct option (kunciIdx). Used by printUjian for AcakSoal.
func shuffleOpsi(opsi []string, kunciIdx int, seed int64) ([]string, int) {
	n := len(opsi)
	if n == 0 {
		return opsi, -1
	}
	if kunciIdx < 0 || kunciIdx >= n {
		kunciIdx = 0
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	out := make([]string, n)
	newKunci := -1
	for newpos, orig := range idx {
		out[newpos] = opsi[orig]
		if orig == kunciIdx {
			newKunci = newpos
		}
	}
	return out, newKunci
}

func (s *Server) printUjian(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.Preload("Mapel").Preload("Kelas").First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "ujian not found")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}
	var us []UjianSoal
	s.db.Preload("Soal").Where("ujian_id = ?", uj.ID).Order("created_at").Find(&us)
	kunciMode := c.Query("kunci") == "1" || c.Query("kunci") == "true"

	// urutan soal (acak deterministik bila AcakSoal)
	order := us
	if uj.AcakSoal {
		seed := seedFromID(uj.ID)
		cp := make([]UjianSoal, len(us))
		copy(cp, us)
		r := rand.New(rand.NewSource(seed))
		r.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
		order = cp
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(180, 8, "PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 12)
	title := "Naskah Soal Ujian"
	if kunciMode {
		title = "Kunci Jawaban Ujian"
	}
	pdf.CellFormat(180, 7, title, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(2)
	pdf.CellFormat(40, 6, "Judul", "", 0, "L", false, 0, "")
	pdf.CellFormat(140, 6, ": "+uj.Judul, "", 1, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Mata Pelajaran", "", 0, "L", false, 0, "")
	pdf.CellFormat(140, 6, ": "+uj.Mapel.NamaMapel, "", 1, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Kelas", "", 0, "L", false, 0, "")
	pdf.CellFormat(140, 6, ": Kelas "+strconv.Itoa(uj.Kelas.Jenjang)+uj.Kelas.NamaRombel, "", 1, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Waktu", "", 0, "L", false, 0, "")
	pdf.CellFormat(140, 6, ": "+uj.WaktuMulai.Format("02-01-2006 15:04")+" s/d "+uj.WaktuSelesai.Format("15:04"), "", 1, "L", false, 0, "")
	if uj.DurasiMenit > 0 {
		pdf.CellFormat(40, 6, "Durasi", "", 0, "L", false, 0, "")
		pdf.CellFormat(140, 6, ": "+strconv.Itoa(uj.DurasiMenit)+" menit", "", 1, "L", false, 0, "")
	}
	if uj.AcakSoal {
		pdf.CellFormat(40, 6, "Catatan", "", 0, "L", false, 0, "")
		pdf.CellFormat(140, 6, ": Soal & opsi diacak (urutan identik di naskah & kunci)", "", 1, "L", false, 0, "")
	}
	// QR code: tautan ke halaman ujian online siswa
	if uj.AksesKode != "" && !kunciMode {
		studentURL := publicBase() + "/ujian"
		if qrBytes, err := qrPNG(studentURL); err == nil {
			opts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
			pdf.RegisterImageOptionsReader("qrujian", opts, bytes.NewReader(qrBytes))
			pdf.ImageOptions("qrujian", 155, pdf.GetY()-28, 25, 0, false, opts, 0, "")
			pdf.SetXY(155, pdf.GetY()-2)
			pdf.SetFont("Helvetica", "", 7)
			pdf.CellFormat(25, 4, "Scan untuk ujian", "", 1, "C", false, 0, "")
			pdf.SetXY(15, pdf.GetY()+2)
		}
	}
	pdf.Ln(4)

	totalBobot := 0.0
	for _, item := range order {
		totalBobot += item.Bobot
	}
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(180, 5, fmt.Sprintf("Jumlah soal: %d  |  Total bobot: %.0f", len(order), totalBobot), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	for i, item := range order {
		soal := item.Soal
		pdf.SetFont("Helvetica", "B", 11)
		pdf.MultiCell(180, 6, fmt.Sprintf("%d. (%.0f poin) %s", i+1, item.Bobot, soal.Pertanyaan), "", "L", false)
		if soal.Tipe == "pg" {
			var opsi []string
			if soal.Opsi != "" {
				_ = json.Unmarshal([]byte(soal.Opsi), &opsi)
			}
			kunciIdx, _ := strconv.Atoi(soal.Kunci)
			var displayOpsi []string
			markIdx := -1
			if uj.AcakSoal {
				displayOpsi, markIdx = shuffleOpsi(opsi, kunciIdx, seedFromID(uj.ID)+int64(i))
			} else {
				displayOpsi = opsi
				markIdx = kunciIdx
			}
			for j, op := range displayOpsi {
				label := string(rune('A' + j))
				mark := ""
				if kunciMode && j == markIdx {
					mark = "   ✓"
				}
				pdf.SetFont("Helvetica", "", 10)
				pdf.MultiCell(180, 5, fmt.Sprintf("   %s. %s%s", label, op, mark), "", "L", false)
			}
		} else {
			if kunciMode && strings.TrimSpace(soal.Kunci) != "" {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.MultiCell(180, 5, "   Kunci: "+soal.Kunci, "", "L", false)
			}
		}
		pdf.Ln(2)
	}

	c.Set(fiber.HeaderContentType, "application/pdf")
	fname := "naskah-" + uj.Judul
	if kunciMode {
		fname = "kunci-" + uj.Judul
	}
	c.Attachment(fname + ".pdf")
	return pdf.Output(c.Response().BodyWriter())
}

// exportUjianResults exports ujian participants and their scores as CSV.
func (s *Server) exportUjianResults(c *fiber.Ctx) error {
	var uj Ujian
	if e := s.db.First(&uj, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "ujian not found")
	}
	if e := s.scopeUjian(c, &uj); e != nil {
		return e
	}

	// Get participants with their answers
	var participants []UjianPeserta
	s.db.Preload("PesertaDidik").Preload("PesertaDidik.Kelas").
		Where("ujian_id = ?", uj.ID).Find(&participants)

	var participantIDs []string
	for _, p := range participants {
		participantIDs = append(participantIDs, p.ID)
	}
	var answers []UjianJawaban
	s.db.Where("ujian_peserta_id IN ?", participantIDs).Find(&answers)

	// Build answer map: participantID -> soalID -> jawaban
	answerMap := map[string]map[string]string{}
	for _, a := range answers {
		if answerMap[a.UjianPesertaID] == nil {
			answerMap[a.UjianPesertaID] = map[string]string{}
		}
		answerMap[a.UjianPesertaID][a.SoalID] = a.Jawaban
	}

	// Get soal count
	var soalCount int64
	s.db.Model(&UjianSoal{}).Where("ujian_id = ?", uj.ID).Count(&soalCount)

	// Get soal IDs in order for consistent column layout
	var soalIDs []string
	s.db.Model(&UjianSoal{}).Where("ujian_id = ?", uj.ID).Order("created_at").Pluck("soal_id", &soalIDs)

	// Build CSV
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	// Header
	header := []string{"No", "Nama Siswa", "NISN", "Kelas", "Status", "Skor"}
	for i := range soalIDs {
		header = append(header, fmt.Sprintf("Soal %d", i+1))
	}
	w.Write(header)

	// Data rows
	for i, p := range participants {
		row := []string{
			strconv.Itoa(i + 1),
			p.PesertaDidik.Nama,
			p.PesertaDidik.NISN,
			fmt.Sprintf("Kelas %d%s", p.PesertaDidik.Kelas.Jenjang, p.PesertaDidik.Kelas.NamaRombel),
			p.Status,
			fmt.Sprintf("%.1f", *p.Skor),
		}
		if pMap, ok := answerMap[p.ID]; ok {
			for _, sid := range soalIDs {
				row = append(row, pMap[sid])
			}
		}
		w.Write(row)
	}
	w.Flush()

	c.Set(fiber.HeaderContentType, "text/csv")
	c.Attachment("hasil-ujian-" + uj.Judul + ".csv")
	return c.Send(buf.Bytes())
}

// ---------------------------------------------------------------------------
// Modul O/N — Program & Fase (master). CRUD via generic helpers (admin group);
// read via readAll. Tidak ada handler kustom — sederhana.
// ---------------------------------------------------------------------------

// (Program & Fase handlers: CRUD memakai create/update/deleteRow generics yang
// didaftarkan di admin group; read memakai list[] di readAll.)

// ---------------------------------------------------------------------------
// Modul H — Sertifikat (prd_fitur_simpkbm.md). Terbit admin-only per siswa lulus;
// nomor unik PKBM-<tahun>-<program>-<seq>. Cetak PDF + QR (go-qrcode). Verify
// publik (no auth) mengembalikan data non-sensitif.
// ---------------------------------------------------------------------------

func publicBase() string { return env("PUBLIC_BASE_URL", "http://localhost:8080") }

func qrPNG(content string) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, 256)
}

func (s *Server) listSertifikat(c *fiber.Ctx) error {
	q := s.db.Preload("PesertaDidik").Preload("PesertaDidik.Kelas").Preload("Program").Order("created_at desc")
	if v := c.Query("pesertaDidikId"); v != "" {
		q = q.Where("peserta_didik_id = ?", v)
	}
	var rows []Sertifikat
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createSertifikat(c *fiber.Ctx) error {
	var in struct {
		PesertaDidikID string `json:"pesertaDidikId"`
		ProgramID      string `json:"programId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.PesertaDidikID == "" || in.ProgramID == "" {
		return fiber.NewError(400, "pesertaDidikId dan programId wajib diisi")
	}
	var pd PesertaDidik
	if s.db.First(&pd, "id = ?", in.PesertaDidikID).Error != nil {
		return fiber.NewError(400, "peserta didik tidak ditemukan")
	}
	if pd.Status != "lulus" {
		return fiber.NewError(400, "peserta didik belum berstatus lulus")
	}
	var prog Program
	if s.db.First(&prog, "id = ?", in.ProgramID).Error != nil {
		return fiber.NewError(400, "program tidak ditemukan")
	}
	var exist Sertifikat
	if s.db.Where("peserta_didik_id = ?", in.PesertaDidikID).First(&exist).Error == nil {
		return fiber.NewError(400, "peserta didik sudah memiliki sertifikat")
	}
	uid := c.Locals("userID").(string)
	tahun := time.Now().Year()
	prefix := fmt.Sprintf("PKBM-%d-%s-", tahun, prog.Kode)
	// Race condition: count+create non-atomik bisa collision pada nomor urut
	// (uniqueIndex) bila dua permintaan bersamaan. Bungkus dalam transaksi
	// (SQLite men-serial-kan writer) + retry bila tetap kena unique-conflict.
	var sert Sertifikat
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 5; attempt++ {
			var cnt int64
			if err := tx.Model(&Sertifikat{}).Where("nomor LIKE ?", prefix+"%").Count(&cnt).Error; err != nil {
				return err
			}
			nomor := fmt.Sprintf("%s%03d", prefix, cnt+1)
			sert = Sertifikat{
				PesertaDidikID:        in.PesertaDidikID,
				ProgramID:             in.ProgramID,
				Nomor:                 nomor,
				TanggalTerbit:         time.Now(),
				Status:                "terbit",
				DiterbitkanOlehUserID: uid,
			}
			err := tx.Create(&sert).Error
			if err == nil {
				return nil
			}
			if !isUniqueErr(err) {
				return fiber.NewError(400, err.Error())
			}
			// unique conflict pada nomor → coba lagi dengan cnt baru
		}
		return fiber.NewError(409, "gagal membuat nomor sertifikat unik setelah beberapa percobaan")
	}); e != nil {
		return e
	}
	s.audit(&uid, "create", "sertifikat", sert.ID)
	return c.Status(201).JSON(sert)
}

func (s *Server) printSertifikat(c *fiber.Ctx) error {
	var sert Sertifikat
	if e := s.db.Preload("PesertaDidik").Preload("PesertaDidik.Kelas").Preload("Program").First(&sert, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "sertifikat not found")
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(170, 10, "PKBM TUNAS ILMU", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 22)
	pdf.CellFormat(170, 14, "SERTIFIKAT", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "I", 11)
	pdf.CellFormat(170, 8, "Nomor: "+sert.Nomor, "", 1, "C", false, 0, "")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 12)
	pdf.MultiCell(170, 7, "Dengan ini menyatakan bahwa:", "", "C", false)
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 16)
	pdf.MultiCell(170, 9, sert.PesertaDidik.Nama, "", "C", false)
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "", 12)
	kelasStr := fmt.Sprintf("Kelas %d%s", sert.PesertaDidik.Kelas.Jenjang, sert.PesertaDidik.Kelas.NamaRombel)
	pdf.MultiCell(170, 7, fmt.Sprintf("telah menyelesaikan Program %s (Paket %s) pada PKBM Tunas Ilmu.", sert.Program.Nama, sert.Program.Kode), "", "C", false)
	pdf.MultiCell(170, 7, kelasStr, "", "C", false)
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(170, 6, "Tanggal Terbit: "+sert.TanggalTerbit.Format("02-01-2006"), "", 1, "C", false, 0, "")
	pdf.Ln(10)

	// QR menuju endpoint verify publik
	verifyURL := publicBase() + "/api/verify/sertifikat/" + sert.Nomor
	if png, e := qrPNG(verifyURL); e == nil {
		opts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
		pdf.RegisterImageOptionsReader("qr", opts, bytes.NewReader(png))
		pdf.ImageOptions("qr", 75, pdf.GetY(), 40, 40, false, opts, 0, "")
		pdf.SetXY(20, pdf.GetY()+42)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(170, 5, "Pindai QR untuk verifikasi keaslian sertifikat", "", 1, "C", false, 0, "")
	}

	pdf.SetXY(20, 250)
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(170, 6, "Kepala PKBM Tunas Ilmu,", "", 1, "R", false, 0, "")
	pdf.Ln(22)
	pdf.CellFormat(170, 6, "(________________________)", "", 1, "R", false, 0, "")

	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("sertifikat-" + sert.Nomor + ".pdf")
	return pdf.Output(c.Response().BodyWriter())
}

// verifySertifikat (public, no auth) — data non-sensitif untuk scan QR.
func (s *Server) verifySertifikat(c *fiber.Ctx) error {
	nomor := c.Params("nomor")
	var sert Sertifikat
	if s.db.Preload("PesertaDidik").Preload("Program").Where("nomor = ?", nomor).First(&sert).Error != nil {
		return c.Status(404).JSON(fiber.Map{"valid": false, "error": "nomor sertifikat tidak ditemukan"})
	}
	return c.JSON(fiber.Map{
		"valid":         sert.Status == "terbit",
		"nomor":         sert.Nomor,
		"nama":          sert.PesertaDidik.Nama,
		"program":       sert.Program.Nama,
		"kodeProgram":   sert.Program.Kode,
		"status":        sert.Status,
		"tanggalTerbit": sert.TanggalTerbit.Format("02-01-2006"),
	})
}

// verifySiswa (public, no auth) — verifikasi QR kartu pelajar via NISN.
func (s *Server) verifySiswa(c *fiber.Ctx) error {
	nisn := c.Params("nisn")
	var pd PesertaDidik
	if s.db.Preload("Kelas").Preload("Kelas.TahunAjaran").Where("nisn = ?", nisn).First(&pd).Error != nil {
		return c.Status(404).JSON(fiber.Map{"valid": false, "error": "NISN tidak ditemukan"})
	}
	ta := ""
	if pd.Kelas.TahunAjaranID != "" {
		ta = pd.Kelas.TahunAjaran.NamaTahunAjaran
	}
	return c.JSON(fiber.Map{
		"valid":       true,
		"nama":        pd.Nama,
		"nisn":        pd.NISN,
		"kelas":       fmt.Sprintf("Kelas %d%s", pd.Kelas.Jenjang, pd.Kelas.NamaRombel),
		"status":      pd.Status,
		"tahunAjaran": ta,
	})
}

// ---------------------------------------------------------------------------
// Modul P — Kartu Pelajar (prd_fitur_simpkbm.md). Cetak PDF ID card + QR per
// siswa atau massal per rombel. Foto siswa via upload (reuse saveUpload). Guard
// scopeKelas (admin/kepala bebas, guru wali).
// ---------------------------------------------------------------------------

func (s *Server) scopeKelas(c *fiber.Ctx, kelasID string) error {
	if c.Locals("role") == "admin" || c.Locals("role") == "kepala_sekolah" {
		return nil
	}
	return s.canManageKelas(c, kelasID)
}

func (s *Server) uploadFotoSiswa(c *fiber.Ctx) error {
	var pd PesertaDidik
	if s.db.First(&pd, "id = ?", id(c)).Error != nil {
		return fiber.NewError(404, "peserta didik not found")
	}
	if c.Locals("role") != "admin" {
		if e := s.canManageKelas(c, pd.KelasID); e != nil {
			return e
		}
	}
	path, e := s.saveUpload(c, "foto", "foto-siswa", 5*1024*1024, []string{"jpg", "jpeg", "png"})
	if e != nil {
		return e
	}
	if path == "" {
		return fiber.NewError(400, "foto wajib diunggah")
	}
	pd.FotoPath = &path
	if e := s.db.Save(&pd).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "peserta_didik_foto", pd.ID)
	return c.JSON(pd)
}

func (s *Server) printKartuPelajar(c *fiber.Ctx) error {
	var pd PesertaDidik
	if s.db.Preload("Kelas").Preload("Kelas.TahunAjaran").Preload("Kelas.Pokjar").First(&pd, "id = ?", c.Params("pesertaDidikId")).Error != nil {
		return fiber.NewError(404, "peserta didik not found")
	}
	if e := s.scopeKelas(c, pd.KelasID); e != nil {
		return e
	}
	return s.renderKartuPDF(c, []PesertaDidik{pd})
}

func (s *Server) printKartuGroup(c *fiber.Ctx) error {
	kelasID := c.Params("kelasId")
	if e := s.scopeKelas(c, kelasID); e != nil {
		return e
	}
	var siswa []PesertaDidik
	s.db.Preload("Kelas").Preload("Kelas.TahunAjaran").Preload("Kelas.Pokjar").Where("kelas_id = ?", kelasID).Order("nama").Find(&siswa)
	if len(siswa) == 0 {
		return fiber.NewError(404, "tidak ada peserta didik di kelas ini")
	}
	return s.renderKartuPDF(c, siswa)
}

// renderKartuPDF menggambar kartu pelajar (depan + belakang) per siswa, 2 kartu
// per baris, multi-halaman. ID-1-ish (90x55 mm). QR verify via NISN.
func (s *Server) renderKartuPDF(c *fiber.Ctx, siswa []PesertaDidik) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	const (
		cardW = 90.0
		cardH = 55.0
		gapX  = 5.0
		gapY  = 8.0
		leftX = 12.0
		topY  = 12.0
	)
	y := topY
	for _, pd := range siswa {
		drawKartuFront(pdf, leftX, y, cardW, cardH, pd)
		drawKartuBack(pdf, leftX+cardW+gapX, y, cardW, cardH, pd)
		y += cardH + gapY
		if y+cardH > 287 {
			pdf.AddPage()
			y = topY
		}
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("kartu-pelajar.pdf")
	return pdf.Output(c.Response().BodyWriter())
}

// kartuBrand colors for the professional student-ID style card.
const (
	kartuGreenR = 28
	kartuGreenG = 87
	kartuGreenB = 64
	kartuGoldR  = 212
	kartuGoldG  = 175
	kartuGoldB  = 55
)

func drawKartuFront(pdf *gofpdf.Fpdf, x, y, w, h float64, pd PesertaDidik) {
	// Outer border + header band (brand green) + gold accent line.
	pdf.SetDrawColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.SetLineWidth(0.4)
	pdf.Rect(x, y, w, h, "D")
	const headH = 12.0
	pdf.SetFillColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.Rect(x, y, w, headH, "F")
	pdf.SetFillColor(kartuGoldR, kartuGoldG, kartuGoldB)
	pdf.Rect(x, y+headH, w, 0.6, "F")

	// Monogram tile (white square, green "TI") di kiri header.
	tile := 7.0
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(x+2, y+2.5, tile, tile, "F")
	pdf.SetTextColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(x+2, y+4)
	pdf.CellFormat(tile, tile-2, "TI", "", 0, "C", false, 0, "")

	// Org name + "KARTU PELAJAR" subtitle di kanan monogram.
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(x+2+tile+2, y+2.2)
	pdf.CellFormat(w-(2+tile+2)-2, 4, "PKBM TUNAS ILMU", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 5.5)
	pdf.SetTextColor(kartuGoldR, kartuGoldG, kartuGoldB)
	pdf.SetXY(x+2+tile+2, y+6.4)
	pdf.CellFormat(w-(2+tile+2)-2, 3, "KARTU PELAJAR", "", 0, "L", false, 0, "")

	pdf.SetTextColor(34, 34, 34)

	// Foto (kiri bawah header) atau placeholder.
	fotoX, fotoY, fotoW, fotoH := x+3, y+headH+2, 22.0, 26.0
	if pd.FotoPath != nil && *pd.FotoPath != "" {
		fp := "./" + *pd.FotoPath
		if _, e := os.Stat(fp); e == nil {
			ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(fp), "."))
			if ext == "JPEG" {
				ext = "JPG"
			}
			pdf.Image(fp, fotoX, fotoY, fotoW, fotoH, false, ext, 0, "")
		} else {
			drawFotoPlaceholder(pdf, fotoX, fotoY, fotoW, fotoH)
		}
	} else {
		drawFotoPlaceholder(pdf, fotoX, fotoY, fotoW, fotoH)
	}

	// Info grid (kanan foto): label abu + nilai.
	tx := fotoX + fotoW + 2.5
	colW := w - (tx - x) - 1.5
	ta := ""
	if pd.Kelas.TahunAjaranID != "" {
		ta = pd.Kelas.TahunAjaran.NamaTahunAjaran
	}
	rows := [][2]string{
		{"Nama", pd.Nama},
		{"NISN", pd.NISN},
		{"NIS", pd.NIS},
		{"JK", pd.JenisKelamin},
		{"Kelas", fmt.Sprintf("%d%s", pd.Kelas.Jenjang, pd.Kelas.NamaRombel)},
		{"Pokjar", pd.Kelas.Pokjar.NamaPokjar},
		{"TA", ta},
	}
	ry := y + headH + 1.5
	for i, r := range rows {
		fsz := 7.0
		lh := 4.2
		if i == 0 { // Nama — bold, lebih besar
			pdf.SetFont("Helvetica", "B", 8.5)
			pdf.SetXY(tx, ry)
			pdf.MultiCell(colW, 4.4, r[1], "", "L", false)
			ry += 5.0
			continue
		}
		pdf.SetFont("Helvetica", "", fsz)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(tx, ry)
		pdf.CellFormat(8, lh, r[0], "", 0, "L", false, 0, "")
		pdf.SetTextColor(34, 34, 34)
		pdf.SetFont("Helvetica", "B", fsz)
		pdf.SetXY(tx+8.5, ry)
		// potong nilai agar tidak melampaui kolom (estimasi lebar per char)
		val := r[1]
		maxChars := int((colW - 9) / 1.6)
		if maxChars > 0 && len(val) > maxChars {
			val = val[:maxChars-1] + "…"
		}
		pdf.CellFormat(colW-9, lh, val, "", 0, "L", false, 0, "")
		ry += lh
	}

	// QR verifikasi (kanan bawah) + micro caption.
	verifyURL := publicBase() + "/api/verify/siswa/" + pd.NISN
	if png, e := qrPNG(verifyURL); e == nil {
		opts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
		qname := "qr-" + pd.ID
		pdf.RegisterImageOptionsReader(qname, opts, bytes.NewReader(png))
		qrS := 15.0
		pdf.ImageOptions(qname, x+w-qrS-2.5, y+h-qrS-3.5, qrS, qrS, false, opts, 0, "")
		pdf.SetFont("Helvetica", "", 4.2)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(x+w-qrS-2.5, y+h-3.2)
		pdf.CellFormat(qrS, 2.5, "Pindai utk verifikasi", "", 0, "C", false, 0, "")
	}
}

func drawFotoPlaceholder(pdf *gofpdf.Fpdf, x, y, w, h float64) {
	pdf.SetFillColor(238, 238, 238)
	pdf.Rect(x, y, w, h, "F")
	pdf.SetDrawColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.SetLineWidth(0.2)
	pdf.Rect(x, y, w, h, "D")
	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(140, 140, 140)
	pdf.SetXY(x, y+h/2-2)
	pdf.CellFormat(w, 4, "Foto", "", 0, "C", false, 0, "")
}

func drawKartuBack(pdf *gofpdf.Fpdf, x, y, w, h float64, pd PesertaDidik) {
	pdf.SetDrawColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.SetLineWidth(0.4)
	pdf.Rect(x, y, w, h, "D")
	const headH = 11.0
	pdf.SetFillColor(kartuGreenR, kartuGreenG, kartuGreenB)
	pdf.Rect(x, y, w, headH, "F")
	pdf.SetFillColor(kartuGoldR, kartuGoldG, kartuGoldB)
	pdf.Rect(x, y+headH, w, 0.6, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8.5)
	pdf.SetXY(x, y+2.2)
	pdf.CellFormat(w, 4, "PKBM TUNAS ILMU", "", 0, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 5.5)
	pdf.SetXY(x, y+6.4)
	pdf.MultiCell(w-4, 2.8, "Pusat Kegiatan Belajar Masyarakat", "", "C", false)

	pdf.SetTextColor(60, 60, 60)
	alamat := env("KARTU_ALAMAT", "Jl. Pendidikan No. 1")
	telp := env("KARTU_TELP", "(021) 000-0000")
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.SetXY(x+2, y+headH+3)
	pdf.MultiCell(w-4, 3.0, alamat, "", "C", false)
	pdf.SetXY(x+2, y+headH+7)
	pdf.MultiCell(w-4, 3.0, "Telp. "+telp, "", "C", false)

	pdf.SetTextColor(90, 90, 90)
	pdf.SetFont("Helvetica", "I", 6)
	pdf.SetXY(x+2, y+headH+12)
	pdf.MultiCell(w-4, 2.8, "Kartu ini berlaku selama status peserta didik aktif.", "", "C", false)

	// NISN footer kiri bawah.
	pdf.SetTextColor(110, 110, 110)
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetXY(x+2, y+h-5)
	pdf.CellFormat(w-4, 3, "NISN: "+pd.NISN, "", 0, "L", false, 0, "")

	// Tanda tangan Kepala PKBM (kanan bawah).
	pdf.SetTextColor(34, 34, 34)
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.SetXY(x, y+h-15)
	pdf.CellFormat(w, 3, "Kepala PKBM,", "", 0, "R", false, 0, "")
	pdf.SetXY(x, y+h-9)
	pdf.CellFormat(w, 3, "(________________)", "", 0, "R", false, 0, "")
	if nama := strings.TrimSpace(env("KARTU_KEPALA_NAMA", "")); nama != "" {
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetXY(x, y+h-6)
		pdf.CellFormat(w, 3, nama, "", 0, "R", false, 0, "")
	}
}

// ---------------------------------------------------------------------------
// Modul G — Catatan Perilaku (prd_fitur_simpkbm.md). Tutor wali mencatat
// perilaku positif/negatif per peserta didik; diagregasi ke rapor (Modul I).
// ---------------------------------------------------------------------------

func (s *Server) listPerilaku(c *fiber.Ctx) error {
	q := s.db.Preload("PesertaDidik").Preload("Kelas").Order("tanggal desc")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	if v := c.Query("pesertaDidikId"); v != "" {
		q = q.Where("peserta_didik_id = ?", v)
	}
	ids, ok := s.waliKelasIDs(c)
	if !ok {
		return fiber.NewError(403, "no tutor profile")
	}
	if ids != nil {
		if len(ids) == 0 {
			return c.JSON([]CatatanPerilaku{})
		}
		q = q.Where("kelas_id IN ?", ids)
	}
	var rows []CatatanPerilaku
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createPerilaku(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "guru" && role != "admin" {
		return fiber.NewError(403, "not permitted")
	}
	var in struct {
		PesertaDidikID string    `json:"pesertaDidikId"`
		KelasID        string    `json:"kelasId"`
		Tanggal        time.Time `json:"tanggal"`
		Kategori       string    `json:"kategori"`
		Deskripsi      string    `json:"deskripsi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.PesertaDidikID == "" || in.KelasID == "" || strings.TrimSpace(in.Deskripsi) == "" {
		return fiber.NewError(400, "pesertaDidikId, kelasId, dan deskripsi wajib diisi")
	}
	if in.Kategori != "positif" && in.Kategori != "negatif" {
		return fiber.NewError(400, "kategori harus positif atau negatif")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	t := in.Tanggal
	if t.IsZero() {
		t = time.Now()
	}
	cp := CatatanPerilaku{
		PesertaDidikID:    in.PesertaDidikID,
		KelasID:           in.KelasID,
		Tanggal:           t,
		Kategori:          in.Kategori,
		Deskripsi:         in.Deskripsi,
		DicatatOlehUserID: uid,
	}
	if e := s.db.Create(&cp).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	s.audit(&uid, "create", "catatan_perilaku", cp.ID)
	return c.Status(201).JSON(cp)
}

// ---------------------------------------------------------------------------
// Modul I — Rapor (prd_fitur_simpkbm.md). Agregasi (tidak ada tabel nilai baru):
// PesertaDidik + Kelas + TahunAjaran + RekapNilaiAkhir + CatatanPerilaku (G) +
// CatatanRapor. Guard wali via kelas siswa; admin/kepala bebas.
// ---------------------------------------------------------------------------

func (s *Server) getRapor(c *fiber.Ctx) error {
	pdID := c.Params("pesertaDidikId")
	semester := c.Query("semester")
	tahunID := c.Query("tahunAjaranId")
	var pd PesertaDidik
	if e := s.db.Preload("Kelas").Preload("Kelas.TahunAjaran").Preload("Kelas.Pokjar").First(&pd, "id = ?", pdID).Error; e != nil {
		return fiber.NewError(404, "peserta didik not found")
	}
	if e := s.scopeKelas(c, pd.KelasID); e != nil {
		return e
	}
	if tahunID == "" {
		tahunID = pd.Kelas.TahunAjaranID
	}
	if semester == "" {
		semester = s.semester(time.Now())
	}
	var rekaps []RekapNilaiAkhir
	s.db.Where("peserta_didik_id = ? AND tahun_ajaran_id = ? AND semester = ?", pdID, tahunID, semester).Find(&rekaps)
	mapelByID := loadMapelMap(s.db, rekaps)
	var perilaku []CatatanPerilaku
	s.db.Where("peserta_didik_id = ? AND kelas_id = ?", pdID, pd.KelasID).Order("tanggal desc").Find(&perilaku)
	var cr CatatanRapor
	s.db.Where("peserta_didik_id = ? AND tahun_ajaran_id = ? AND semester = ?", pdID, tahunID, semester).First(&cr)
	return c.JSON(fiber.Map{
		"pesertaDidik":  pd,
		"kelas":         pd.Kelas,
		"tahunAjaran":   pd.Kelas.TahunAjaran,
		"semester":      semester,
		"tahunAjaranId": tahunID,
		"rekap":         rekaps,
		"mapelByID":     mapelByID,
		"perilaku":      perilaku,
		"catatanRapor":  cr,
	})
}

// loadMapelMap builds a mapelID -> MataPelajaran lookup for the mapel IDs present
// in the given rekap rows (RekapNilaiAkhir has no Mapel relation field).
func loadMapelMap(db *gorm.DB, rekaps []RekapNilaiAkhir) map[string]MataPelajaran {
	ids := map[string]bool{}
	for _, r := range rekaps {
		ids[r.MapelID] = true
	}
	out := map[string]MataPelajaran{}
	if len(ids) == 0 {
		return out
	}
	keys := make([]string, 0, len(ids))
	for k := range ids {
		keys = append(keys, k)
	}
	var rows []MataPelajaran
	db.Where("id IN ?", keys).Find(&rows)
	for _, m := range rows {
		out[m.ID] = m
	}
	return out
}

func (s *Server) putCatatanRapor(c *fiber.Ctx) error {
	pdID := c.Params("pesertaDidikId")
	var pd PesertaDidik
	if s.db.First(&pd, "id = ?", pdID).Error != nil {
		return fiber.NewError(404, "peserta didik not found")
	}
	if e := s.scopeKelas(c, pd.KelasID); e != nil {
		return e
	}
	var in struct {
		TahunAjaranID string  `json:"tahunAjaranId"`
		Semester      string  `json:"semester"`
		CatatanWali   string  `json:"catatanWali"`
		NaikKelas     *bool   `json:"naikKelas"`
		KenaikanKe    *string `json:"kenaikanKe"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.TahunAjaranID == "" || (in.Semester != "Ganjil" && in.Semester != "Genap") {
		return fiber.NewError(400, "tahunAjaranId dan semester (Ganjil/Genap) wajib diisi")
	}
	uid := c.Locals("userID").(string)
	var cr CatatanRapor
	err := s.db.Where("peserta_didik_id = ? AND tahun_ajaran_id = ? AND semester = ?", pdID, in.TahunAjaranID, in.Semester).First(&cr).Error
	cr.PesertaDidikID = pdID
	cr.TahunAjaranID = in.TahunAjaranID
	cr.Semester = in.Semester
	cr.CatatanWali = in.CatatanWali
	cr.NaikKelas = in.NaikKelas
	cr.KenaikanKe = in.KenaikanKe
	if err == gorm.ErrRecordNotFound {
		if e := s.db.Create(&cr).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	} else if err != nil {
		return err
	} else {
		if e := s.db.Save(&cr).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	}
	s.audit(&uid, "update", "catatan_rapor", cr.ID)
	return c.JSON(cr)
}

func (s *Server) printRapor(c *fiber.Ctx) error {
	pdID := c.Params("pesertaDidikId")
	semester := c.Query("semester")
	tahunID := c.Query("tahunAjaranId")
	var pd PesertaDidik
	if e := s.db.Preload("Kelas").Preload("Kelas.TahunAjaran").Preload("Kelas.Pokjar").First(&pd, "id = ?", pdID).Error; e != nil {
		return fiber.NewError(404, "peserta didik not found")
	}
	if e := s.scopeKelas(c, pd.KelasID); e != nil {
		return e
	}
	if tahunID == "" {
		tahunID = pd.Kelas.TahunAjaranID
	}
	if semester == "" {
		semester = s.semester(time.Now())
	}
	var rekaps []RekapNilaiAkhir
	s.db.Where("peserta_didik_id = ? AND tahun_ajaran_id = ? AND semester = ?", pdID, tahunID, semester).Order("mapel_id").Find(&rekaps)
	mapelByID := loadMapelMap(s.db, rekaps)
	var perilaku []CatatanPerilaku
	s.db.Where("peserta_didik_id = ? AND kelas_id = ?", pdID, pd.KelasID).Order("tanggal desc").Find(&perilaku)
	var cr CatatanRapor
	s.db.Where("peserta_didik_id = ? AND tahun_ajaran_id = ? AND semester = ?", pdID, tahunID, semester).First(&cr)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(180, 8, "PKBM TUNAS ILMU", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 7, "LAPORAN HASIL BELAJAR (RAPOR)", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(180, 6, fmt.Sprintf("Tahun Ajaran %s — Semester %s", pd.Kelas.TahunAjaran.NamaTahunAjaran, semester), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 6, "Identitas Peserta Didik", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(180, 5.5, "Nama          : "+pd.Nama, "", 1, "L", false, 0, "")
	pdf.CellFormat(180, 5.5, "NISN          : "+pd.NISN, "", 1, "L", false, 0, "")
	pdf.CellFormat(180, 5.5, fmt.Sprintf("Kelas         : Kelas %d%s", pd.Kelas.Jenjang, pd.Kelas.NamaRombel), "", 1, "L", false, 0, "")
	pdf.CellFormat(180, 5.5, "Pokjar        : "+pd.Kelas.Pokjar.NamaPokjar, "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Tabel nilai per mapel.
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 6, "Nilai Akademik", "", 1, "L", false, 0, "")
	pdf.SetFillColor(28, 87, 64)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(70, 7, "Mata Pelajaran", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "NP", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "NK", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "NA", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 7, "Predikat", "1", 1, "C", true, 0, "")
	pdf.SetTextColor(23, 35, 30)
	pdf.SetFont("Helvetica", "", 9)
	for _, r := range rekaps {
		mp := mapelByID[r.MapelID]
		pdf.CellFormat(70, 6.5, mp.NamaMapel, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6.5, fmtNilai(r.NPAkhir), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6.5, fmtNilai(r.NKAkhir), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6.5, fmtNilai(r.NAAkhir), "1", 0, "C", false, 0, "")
		pred := r.PredikatNA
		if pred == "" {
			pred = r.PredikatNP
		}
		pdf.CellFormat(35, 6.5, pred, "1", 1, "C", false, 0, "")
	}
	if len(rekaps) == 0 {
		pdf.CellFormat(180, 6.5, "Belum ada nilai pada semester ini.", "1", 1, "C", false, 0, "")
	}
	pdf.Ln(4)

	// Catatan kepribadian (dari Modul G).
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 6, "Catatan Kepribadian", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	if len(perilaku) == 0 {
		pdf.MultiCell(180, 5, "-", "", "L", false)
	} else {
		for _, p := range perilaku {
			label := "[+]"
			if p.Kategori == "negatif" {
				label = "[-]"
			}
			pdf.MultiCell(180, 5, fmt.Sprintf("%s %s: %s", p.Tanggal.Format("02-01-2006"), label, p.Deskripsi), "", "L", false)
		}
	}
	pdf.Ln(3)

	// Catatan wali + kenaikan (Modul I).
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 6, "Catatan Wali Kelas", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	catatan := cr.CatatanWali
	if catatan == "" {
		catatan = "-"
	}
	pdf.MultiCell(180, 5, catatan, "", "L", false)
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "", 9)
	naikStr := "Belum ditentukan"
	if cr.NaikKelas != nil {
		if *cr.NaikKelas {
			naikStr = "NAIK"
			if cr.KenaikanKe != nil && *cr.KenaikanKe != "" {
				naikStr = "NAIK ke " + *cr.KenaikanKe
			}
		} else {
			naikStr = "TINGGAL di kelas yang sama"
		}
	}
	pdf.CellFormat(180, 5.5, "Kenaikan Kelas : "+naikStr, "", 1, "L", false, 0, "")
	pdf.Ln(10)

	// Tanda tangan.
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(90, 6, "Mengetahui,", "", 0, "C", false, 0, "")
	pdf.CellFormat(90, 6, "Wali Kelas,", "", 1, "C", false, 0, "")
	pdf.Ln(18)
	pdf.CellFormat(90, 6, "Kepala PKBM", "", 0, "C", false, 0, "")
	pdf.CellFormat(90, 6, pd.Nama, "", 1, "C", false, 0, "")

	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("rapor-" + sanitizeFilename(pd.Nama) + "-" + semester + ".pdf")
	return pdf.Output(c.Response().BodyWriter())
}

func fmtNilai(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// ---------------------------------------------------------------------------
// Modul S — Sumber Nilai & bobot (prd_fitur_simpkbm.md). Bobot per (mapel,
// sumber); upsert by composite key. NA gabungan dihitung di recomputeRekap.
// ---------------------------------------------------------------------------

func (s *Server) listBobotSumberNilai(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Preload("Sumber")
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	var rows []BobotSumberNilai
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) upsertBobotSumberNilai(c *fiber.Ctx) error {
	var in struct {
		MapelID  string  `json:"mapelId"`
		SumberID string  `json:"sumberId"`
		Bobot    float64 `json:"bobot"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.MapelID == "" || in.SumberID == "" {
		return fiber.NewError(400, "mapelId dan sumberId wajib diisi")
	}
	if in.Bobot < 0 || in.Bobot > 100 {
		return fiber.NewError(400, "bobot harus antara 0 dan 100")
	}
	var b BobotSumberNilai
	err := s.db.Where("mapel_id = ? AND sumber_id = ?", in.MapelID, in.SumberID).First(&b).Error
	b.MapelID = in.MapelID
	b.SumberID = in.SumberID
	b.Bobot = in.Bobot
	if err == gorm.ErrRecordNotFound {
		if e := s.db.Create(&b).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	} else if err != nil {
		return err
	} else {
		if e := s.db.Save(&b).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "upsert", "bobot_sumber_nilai", b.ID)
	return c.JSON(b)
}

// ---------------------------------------------------------------------------
// Modul L — Modul Pembelajaran + Capaian (prd_fitur_simpkbm.md). Master kurikulum
// per mapel: urutan, deskripsi, daftar capaian (outcomes). Admin CRUD; read semua
// role terauth. Kaitan opsional ke Materi/Tugas via ModulID (field nullable).
// ---------------------------------------------------------------------------

func (s *Server) listModulBelajar(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Order("urutan")
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	var rows []ModulBelajar
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

type modulBelajarInput struct {
	MapelID   string `json:"mapelId"`
	Judul     string `json:"judul"`
	Urutan    int    `json:"urutan"`
	Deskripsi string `json:"deskripsi"`
}

func (s *Server) createModulBelajar(c *fiber.Ctx) error {
	var in modulBelajarInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Judul == "" {
		return fiber.NewError(400, "judul wajib diisi")
	}
	m := ModulBelajar{MapelID: in.MapelID, Judul: in.Judul, Urutan: in.Urutan, Deskripsi: in.Deskripsi}
	if e := s.db.Create(&m).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "modul_belajar", m.ID)
	return c.Status(201).JSON(m)
}

func (s *Server) updateModulBelajar(c *fiber.Ctx) error {
	var m ModulBelajar
	if s.db.First(&m, "id = ?", id(c)).Error != nil {
		return fiber.NewError(404, "record not found")
	}
	var in modulBelajarInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	m.MapelID = in.MapelID
	m.Judul = in.Judul
	m.Urutan = in.Urutan
	m.Deskripsi = in.Deskripsi
	if e := s.db.Save(&m).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "modul_belajar", m.ID)
	return c.JSON(m)
}

func (s *Server) deleteModulBelajar(c *fiber.Ctx) error {
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("modul_id = ?", id(c)).Delete(&CapaianModul{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ModulBelajar{}, "id = ?", id(c)).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "modul_belajar", id(c))
	return c.SendStatus(204)
}

func (s *Server) listCapaianModul(c *fiber.Ctx) error {
	var rows []CapaianModul
	s.db.Where("modul_id = ?", id(c)).Order("kode").Find(&rows)
	return c.JSON(rows)
}

func (s *Server) createCapaianModul(c *fiber.Ctx) error {
	var in struct {
		Kode      string `json:"kode"`
		Deskripsi string `json:"deskripsi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Kode == "" {
		return fiber.NewError(400, "kode wajib diisi")
	}
	cm := CapaianModul{ModulID: id(c), Kode: in.Kode, Deskripsi: in.Deskripsi}
	if e := s.db.Create(&cm).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "capaian_modul", cm.ID)
	return c.Status(201).JSON(cm)
}

func (s *Server) updateCapaianModul(c *fiber.Ctx) error {
	var cm CapaianModul
	if s.db.First(&cm, "id = ?", c.Params("oid")).Error != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		Kode      string `json:"kode"`
		Deskripsi string `json:"deskripsi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	cm.Kode = in.Kode
	cm.Deskripsi = in.Deskripsi
	if e := s.db.Save(&cm).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "capaian_modul", cm.ID)
	return c.JSON(cm)
}

func (s *Server) deleteCapaianModul(c *fiber.Ctx) error {
	if e := s.db.Delete(&CapaianModul{}, "id = ?", c.Params("oid")).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "capaian_modul", c.Params("oid"))
	return c.SendStatus(204)
}

// ---------------------------------------------------------------------------
// Modul M — Kompetensi + Capaian + Nilai + RombelKompetensi (prd_fitur_simpkbm.md).
// Admin CRUD kompetensi/outcomes/rombel-kompetensi; tutor mengisi nilai kompetensi
// (bulk per rombel, guard canManageKelas); read nilai di-scope ke kelas wali.
// ---------------------------------------------------------------------------

func (s *Server) listKompetensi(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel").Order("nama")
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	var rows []Kompetensi
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createKompetensi(c *fiber.Ctx) error {
	var in struct {
		MapelID string `json:"mapelId"`
		Nama    string `json:"nama"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Nama == "" {
		return fiber.NewError(400, "nama wajib diisi")
	}
	k := Kompetensi{MapelID: in.MapelID, Nama: in.Nama}
	if e := s.db.Create(&k).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "kompetensi", k.ID)
	return c.Status(201).JSON(k)
}

func (s *Server) updateKompetensi(c *fiber.Ctx) error {
	var k Kompetensi
	if s.db.First(&k, "id = ?", id(c)).Error != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		MapelID string `json:"mapelId"`
		Nama    string `json:"nama"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	k.MapelID = in.MapelID
	k.Nama = in.Nama
	if e := s.db.Save(&k).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "kompetensi", k.ID)
	return c.JSON(k)
}

func (s *Server) deleteKompetensi(c *fiber.Ctx) error {
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kompetensi_id = ?", id(c)).Delete(&CapaianKompetensi{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kompetensi_id = ?", id(c)).Delete(&RombelKompetensi{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kompetensi_id = ?", id(c)).Delete(&NilaiKompetensi{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Kompetensi{}, "id = ?", id(c)).Error
	}); e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "kompetensi", id(c))
	return c.SendStatus(204)
}

func (s *Server) listCapaianKompetensi(c *fiber.Ctx) error {
	var rows []CapaianKompetensi
	s.db.Where("kompetensi_id = ?", id(c)).Order("kode").Find(&rows)
	return c.JSON(rows)
}

func (s *Server) createCapaianKompetensi(c *fiber.Ctx) error {
	var in struct {
		Kode      string `json:"kode"`
		Deskripsi string `json:"deskripsi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.Kode == "" {
		return fiber.NewError(400, "kode wajib diisi")
	}
	ck := CapaianKompetensi{KompetensiID: id(c), Kode: in.Kode, Deskripsi: in.Deskripsi}
	if e := s.db.Create(&ck).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "capaian_kompetensi", ck.ID)
	return c.Status(201).JSON(ck)
}

func (s *Server) updateCapaianKompetensi(c *fiber.Ctx) error {
	var ck CapaianKompetensi
	if s.db.First(&ck, "id = ?", c.Params("oid")).Error != nil {
		return fiber.NewError(404, "record not found")
	}
	var in struct {
		Kode      string `json:"kode"`
		Deskripsi string `json:"deskripsi"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	ck.Kode = in.Kode
	ck.Deskripsi = in.Deskripsi
	if e := s.db.Save(&ck).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "capaian_kompetensi", ck.ID)
	return c.JSON(ck)
}

func (s *Server) deleteCapaianKompetensi(c *fiber.Ctx) error {
	if e := s.db.Delete(&CapaianKompetensi{}, "id = ?", c.Params("oid")).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "capaian_kompetensi", c.Params("oid"))
	return c.SendStatus(204)
}

func (s *Server) listRombelKompetensi(c *fiber.Ctx) error {
	q := s.db
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	var rows []RombelKompetensi
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

func (s *Server) createRombelKompetensi(c *fiber.Ctx) error {
	var in struct {
		KelasID      string `json:"kelasId"`
		KompetensiID string `json:"kompetensiId"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID == "" || in.KompetensiID == "" {
		return fiber.NewError(400, "kelasId dan kompetensiId wajib diisi")
	}
	rk := RombelKompetensi{KelasID: in.KelasID, KompetensiID: in.KompetensiID}
	if e := s.db.Create(&rk).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "rombel_kompetensi", rk.ID)
	return c.Status(201).JSON(rk)
}

func (s *Server) deleteRombelKompetensi(c *fiber.Ctx) error {
	if e := s.db.Delete(&RombelKompetensi{}, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "rombel_kompetensi", id(c))
	return c.SendStatus(204)
}

func (s *Server) listNilaiKompetensi(c *fiber.Ctx) error {
	kelasID := c.Query("kelasId")
	semester := c.Query("semester")
	if kelasID == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "kelasId dan semester (Ganjil/Genap) wajib")
	}
	if e := s.scopeKelas(c, kelasID); e != nil {
		return e
	}
	var rows []NilaiKompetensi
	s.db.Where("kelas_id = ? AND semester = ?", kelasID, semester).Find(&rows)
	return c.JSON(rows)
}

func (s *Server) saveNilaiKompetensi(c *fiber.Ctx) error {
	var in struct {
		KelasID  string `json:"kelasId"`
		Semester string `json:"semester"`
		Nilai    []struct {
			PesertaDidikID string  `json:"pesertaDidikId"`
			KompetensiID   string  `json:"kompetensiId"`
			Nilai          float64 `json:"nilai"`
		} `json:"nilai"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID == "" || (in.Semester != "Ganjil" && in.Semester != "Genap") {
		return fiber.NewError(400, "kelasId dan semester (Ganjil/Genap) wajib")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, n := range in.Nilai {
			if n.PesertaDidikID == "" || n.KompetensiID == "" {
				continue
			}
			var nk NilaiKompetensi
			e := tx.Where("peserta_didik_id = ? AND kompetensi_id = ? AND kelas_id = ? AND semester = ?", n.PesertaDidikID, n.KompetensiID, in.KelasID, in.Semester).First(&nk).Error
			nk.PesertaDidikID = n.PesertaDidikID
			nk.KompetensiID = n.KompetensiID
			nk.KelasID = in.KelasID
			nk.Semester = in.Semester
			nk.Nilai = n.Nilai
			nk.DicatatOlehUserID = uid
			if e == gorm.ErrRecordNotFound {
				if ce := tx.Create(&nk).Error; ce != nil {
					return ce
				}
			} else if e != nil {
				return e
			} else {
				if ce := tx.Save(&nk).Error; ce != nil {
					return ce
				}
			}
		}
		return nil
	})
	if err != nil {
		return fiber.NewError(400, err.Error())
	}
	s.audit(&uid, "bulk", "nilai_kompetensi", in.KelasID)
	return c.JSON(fiber.Map{"ok": true, "count": len(in.Nilai)})
}

// ---------------------------------------------------------------------------
// Modul J — Pusat Laporan (prd_fitur_simpkbm.md). Agregator: /laporan/jenis
// mengembalikan katalog jenis laporan (single source of truth untuk frontend,
// sudah difilter per role); /laporan/export?jenis=&format= dispatch ke handler
// export yang sudah ada. Handler terkait (exportNilai/rekapPresensiPDF) sudah
// enforce scope guru via canManageKelas( Mapel); jenis admin/kepala-only (buku,
// siswa-pokjar) di-gate per-jenis di sini sebelum dispatch agar guru tak bypass.
// ---------------------------------------------------------------------------

func (s *Server) laporanJenis(c *fiber.Ctx) error {
	type filter struct {
		Key      string   `json:"key"`
		Label    string   `json:"label"`
		Type     string   `json:"type"` // kelas | mapel | tahunAjaran | pokjar | select | text
		Required bool     `json:"required"`
		Options  []string `json:"options,omitempty"`
	}
	type kind struct {
		Jenis   string   `json:"jenis"`
		Nama    string   `json:"nama"`
		Formats []string `json:"formats"`
		Roles   []string `json:"roles"`
		Filters []filter `json:"filters"`
	}
	kinds := []kind{
		{Jenis: "nilai", Nama: "Rekap Nilai per Kelas", Formats: []string{"xlsx", "pdf"}, Roles: []string{"admin", "kepala_sekolah", "guru"}, Filters: []filter{
			{Key: "kelasId", Label: "Kelas", Type: "kelas", Required: true},
			{Key: "semester", Label: "Semester", Type: "select", Required: true, Options: []string{"Ganjil", "Genap"}},
			{Key: "tahunAjaranId", Label: "Tahun Ajaran", Type: "tahunAjaran", Required: true},
			{Key: "mapelId", Label: "Mapel (opsional; kosongkan untuk semua mapel di kelas)", Type: "mapel", Required: false},
		}},
		{Jenis: "presensi", Nama: "Rekap Presensi per Kelas", Formats: []string{"pdf"}, Roles: []string{"admin", "kepala_sekolah", "guru"}, Filters: []filter{
			{Key: "kelasId", Label: "Kelas", Type: "kelas", Required: true},
			{Key: "semester", Label: "Semester", Type: "select", Required: true, Options: []string{"Ganjil", "Genap"}},
		}},
		{Jenis: "buku", Nama: "Rekap Peminjaman Buku", Formats: []string{"xlsx", "pdf"}, Roles: []string{"admin", "kepala_sekolah"}, Filters: []filter{
			{Key: "kelasId", Label: "Kelas (opsional)", Type: "kelas", Required: false},
			{Key: "semester", Label: "Semester (opsional)", Type: "select", Required: false, Options: []string{"Ganjil", "Genap"}},
			{Key: "tahunAjaranId", Label: "Tahun Ajaran (opsional)", Type: "tahunAjaran", Required: false},
			{Key: "status", Label: "Status (opsional)", Type: "text", Required: false},
		}},
		{Jenis: "siswa-pokjar", Nama: "Peserta Didik per Pokjar", Formats: []string{"xlsx"}, Roles: []string{"admin", "kepala_sekolah"}, Filters: []filter{
			{Key: "pokjarId", Label: "Pokjar (opsional; kosongkan untuk semua)", Type: "pokjar", Required: false},
		}},
	}
	role := c.Locals("role").(string)
	out := []kind{}
	for _, k := range kinds {
		for _, r := range k.Roles {
			if r == role {
				out = append(out, k)
				break
			}
		}
	}
	return c.JSON(out)
}

func (s *Server) laporanExport(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	switch c.Query("jenis") {
	case "nilai":
		return s.exportNilai(c) // scope guru via canManageKelasMapel di handler
	case "presensi":
		return s.rekapPresensiPDF(c) // scope guru via canManageKelas di handler
	case "buku":
		if role != "admin" && role != "kepala_sekolah" {
			return fiber.NewError(403, "hanya admin/kepala")
		}
		return s.exportBuku(c)
	case "siswa-pokjar":
		if role != "admin" && role != "kepala_sekolah" {
			return fiber.NewError(403, "hanya admin/kepala")
		}
		return s.exportSiswaPokjar(c)
	default:
		return fiber.NewError(400, "jenis laporan tidak dikenal")
	}
}

func (s *Server) exportSiswaPokjar(c *fiber.Ctx) error {
	q := s.db.Preload("Kelas.Pokjar")
	if p := strings.TrimSpace(c.Query("pokjarId")); p != "" {
		q = q.Where("pokjar_id = ?", p)
	}
	var rows []PesertaDidik
	if e := q.Order("pokjar_id, nama").Find(&rows).Error; e != nil {
		return e
	}
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Peserta Didik")
	_ = xlsx.SetSheetRow(sheet, "A1", &[]interface{}{"Rekap Peserta Didik per Pokjar — PKBM Tunas Ilmu"})
	headers := []interface{}{"No", "Nama", "Jenis Kelamin", "NIS", "NISN", "NIK", "Pokjar", "Kelas", "Status"}
	_ = xlsx.SetSheetRow(sheet, "A3", &headers)
	for i, r := range rows {
		pok := r.Kelas.Pokjar.NamaPokjar
		_ = xlsx.SetSheetRow(sheet, "A"+strconv.Itoa(i+4), &[]interface{}{
			i + 1, r.Nama, r.JenisKelamin, r.NIS, r.NISN, r.NIK, pok, kelasLabel(r.Kelas), r.Status,
		})
	}
	for i, w := range []float64{6, 28, 14, 16, 16, 22, 24, 18, 10} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("rekap-peserta-didik-pokjar.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

// ---------------------------------------------------------------------------
// Modul R — Import Terpusat (prd_fitur_simpkbm.md). Template XLSX per tipe +
// import partial-success: baris gagal di-skip & dicatat di ImportLog.ErrorJson,
// baris valid lanjut di-insert (per-row autocommit — bukan satu tx agar partial
// success alami; aman karena tidak memanggil s.semester/s.db.* dalam tx). Tipe:
// "siswa" (admin) & "nilai-kompetensi" (admin / tutor wali via kelasId di form).
// ---------------------------------------------------------------------------

type importIssue struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

func (s *Server) importTemplate(c *fiber.Ctx) error {
	switch c.Params("tipe") {
	case "siswa":
		return s.siswaTemplate(c)
	case "siswa-lengkap":
		return s.siswaLengkapTemplate(c)
	case "nilai-kompetensi":
		return s.nilaiKompetensiTemplate(c)
	case "tutor":
		return s.tutorTemplate(c)
	case "orang_tua":
		return s.orangTuaTemplate(c)
	default:
		return fiber.NewError(400, "tipe template tidak dikenal")
	}
}

func (s *Server) siswaLengkapTemplate(c *fiber.Ctx) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Peserta Didik Lengkap")
	sheet = "Peserta Didik Lengkap"
	headers := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas", "nama_ayah", "nik_ayah", "pekerjaan_ayah", "pendidikan_ayah", "penghasilan_ayah", "nama_ibu", "nik_ibu", "pekerjaan_ibu", "pendidikan_ibu", "penghasilan_ibu"}
	if err := xlsx.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	examples := []struct{ row []string }{
		{[]string{"Ahmad Fauzi", "L", "2026001", "3185070110", "3303060110100001", "1A", "Budi Santoso", "3303060110700001", "Wiraswasta", "SMA", "Rp. 3.000.000", "Siti Aminah", "3303060110720002", "Mengurus Rumah Tangga", "SMP", "Rp. 0"}},
		{[]string{"Siti Nurhaliza", "P", "2026002", "3185070111", "3303065111100002", "1A", "Ahmad Hidayat", "3303060111650001", "Karyawan", "S1", "Rp. 5.000.000", "Fatimah Azzahra", "3303065111680002", "Guru", "S1", "Rp. 4.000.000"}},
	}
	for i, ex := range examples {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = xlsx.SetSheetRow(sheet, cell, &ex.row)
	}
	widths := []float64{28, 16, 14, 16, 22, 8, 24, 22, 20, 16, 18, 24, 22, 24, 16, 18}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	_ = xlsx.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-import-peserta-didik-lengkap.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) nilaiKompetensiTemplate(c *fiber.Ctx) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Nilai Kompetensi")
	_ = xlsx.SetSheetRow(sheet, "A1", &[]interface{}{"nisn", "kompetensi_id", "nilai"})
	_ = xlsx.SetSheetRow(sheet, "A2", &[]interface{}{"9001001", "ID_KOMPETENSI", "85"})
	for i, w := range []float64{18, 40, 12} {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	_ = xlsx.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-import-nilai-kompetensi.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) tutorTemplate(c *fiber.Ctx) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Tutor")
	sheet = "Tutor"
	headers := []string{"nama", "jenis_kelamin", "no_hp", "alamat", "tanggal_mulai_tugas", "is_rpp_maker"}
	if err := xlsx.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	examples := []struct{ row []string }{
		{[]string{"Ahmad Fauzi", "L", "08123456789", "Jl. Merdeka No. 10, Jakarta", "2026-07-01", "false"}},
		{[]string{"Siti Nurhaliza", "P", "08567890123", "Jl. Pendidikan No. 5, Bandung", "2026-07-01", "true"}},
	}
	for i, ex := range examples {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = xlsx.SetSheetRow(sheet, cell, &ex.row)
	}
	widths := []float64{28, 16, 18, 40, 20, 14}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	_ = xlsx.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-import-tutor.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) orangTuaTemplate(c *fiber.Ctx) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Orang Tua")
	headers := []string{"nama", "nik", "no_hp", "alamat", "username", "password"}
	if err := xlsx.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	examples := []struct{ row []string }{
		{[]string{"Budi Santoso", "3303060110700001", "08123456789", "Jl. Merdeka No. 10, Jakarta", "ortu.budi", "OrangTua123"}},
		{[]string{"Siti Aminah", "3303060110720002", "08567890123", "Jl. Pendidikan No. 5, Bandung", "ortu.siti", "OrangTua123"}},
	}
	for i, ex := range examples {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = xlsx.SetSheetRow(sheet, cell, &ex.row)
	}
	widths := []float64{28, 22, 18, 40, 20, 18}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = xlsx.SetColWidth(sheet, col, col, w)
	}
	_ = xlsx.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("template-import-orang-tua.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

// parseKelasLabel parses a human-readable class label like "1A", "2B", "KELAS 3C"
// into jenjang (1-6) and nama_rombel ("A", "B", etc.). Returns 0 and "" on invalid.
func parseKelasLabel(label string) (int, string) {
	s := strings.ToUpper(strings.TrimSpace(label))
	s = strings.ReplaceAll(s, "KELAS", "")
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, ""
	}
	// First char should be digit 1-6
	digit := s[0]
	if digit < '1' || digit > '6' {
		return 0, ""
	}
	jenjang := int(digit - '0')
	rombel := strings.TrimSpace(s[1:])
	if rombel == "" {
		return 0, ""
	}
	return jenjang, rombel
}

func validateImportHeaders(header []string, expected []string) error {
	if len(header) < len(expected) {
		return fmt.Errorf("kolom tidak cukup; harus: %s", strings.Join(expected, ", "))
	}
	for i, h := range expected {
		if strings.ToLower(strings.TrimSpace(header[i])) != h {
			return fmt.Errorf("kolom tidak sesuai; harus: %s", strings.Join(expected, ", "))
		}
	}
	return nil
}

func validateTutorImportHeaders(header []string) error {
	if len(header) < 3 {
		return fmt.Errorf("kolom tidak cukup; minimal: nama, jenis_kelamin, no_hp")
	}
	for i, expected := range []string{"nama", "jenis_kelamin", "no_hp"} {
		if strings.ToLower(strings.TrimSpace(header[i])) != expected {
			return fmt.Errorf("kolom tidak sesuai; tiga kolom pertama harus: nama, jenis_kelamin, no_hp")
		}
	}
	allowed := map[string]bool{"alamat": true, "tanggal_mulai_tugas": true, "is_rpp_maker": true}
	seen := map[string]bool{}
	for _, raw := range header[3:] {
		name := strings.ToLower(strings.TrimSpace(raw))
		if !allowed[name] || seen[name] {
			return fmt.Errorf("kolom tutor tidak dikenal/duplikat; gunakan alamat, tanggal_mulai_tugas, dan is_rpp_maker sebagai kolom opsional")
		}
		seen[name] = true
	}
	return nil
}

func importColumnIndex(header []string) map[string]int {
	result := map[string]int{"nama": -1, "jenis_kelamin": -1, "no_hp": -1, "alamat": -1, "tanggal_mulai_tugas": -1, "is_rpp_maker": -1}
	for i, raw := range header {
		result[strings.ToLower(strings.TrimSpace(raw))] = i
	}
	return result
}

func importCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func (s *Server) importTerpusat(c *fiber.Ctx) error {
	tipe := c.FormValue("tipe")
	if tipe == "" {
		return fiber.NewError(400, "tipe wajib diisi")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(400, "file Excel wajib diunggah")
	}
	if fileHeader.Size == 0 || fileHeader.Size > 5*1024*1024 {
		return fiber.NewError(400, "ukuran file harus 1 byte - 5 MB")
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".xlsx") {
		return fiber.NewError(400, "hanya file .xlsx")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(400, "tidak dapat membaca file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 5*1024*1024+1))
	if err != nil || len(data) > 5*1024*1024 {
		return fiber.NewError(400, "tidak dapat membaca file Excel")
	}
	xlsx, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return fiber.NewError(400, "file Excel tidak valid")
	}
	defer xlsx.Close()
	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return fiber.NewError(400, "file tidak memiliki worksheet")
	}
	rows, err := xlsx.GetRows(sheets[0])
	if err != nil || len(rows) < 2 {
		return fiber.NewError(400, "file harus berisi header + minimal 1 baris data")
	}

	uid := c.Locals("userID").(string)
	role := c.Locals("role").(string)
	issues := []importIssue{}
	berhasil := 0
	total := len(rows) - 1

	switch tipe {
	case "siswa":
		if role != "admin" {
			return fiber.NewError(403, "import peserta didik hanya admin")
		}
		expected := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas_id", "pokjar_id", "orang_tua_id"}
		if e := validateImportHeaders(rows[0], expected); e != nil {
			return fiber.NewError(400, e.Error())
		}
		if total > 1000 {
			return fiber.NewError(400, "import dibatasi 1000 baris")
		}
		nisSeen, nisnSeen, nikSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for index, row := range rows[1:] {
			line := index + 2
			if len(row) < len(expected) {
				issues = append(issues, importIssue{line, "kolom tidak lengkap"})
				continue
			}
			pd := PesertaDidik{Nama: strings.TrimSpace(row[0]), JenisKelamin: strings.ToUpper(strings.TrimSpace(row[1])), NIS: strings.TrimSpace(row[2]), NISN: strings.TrimSpace(row[3]), NIK: strings.TrimSpace(row[4]), KelasID: strings.TrimSpace(row[5]), PokjarID: strings.TrimSpace(row[6]), OrangTuaID: strings.TrimSpace(row[7]), Status: "aktif"}
			if pd.Nama == "" || (pd.JenisKelamin != "L" && pd.JenisKelamin != "P") || pd.NIS == "" || pd.NISN == "" || pd.NIK == "" || pd.KelasID == "" || pd.PokjarID == "" || pd.OrangTuaID == "" {
				issues = append(issues, importIssue{line, "semua kolom wajib; jenis_kelamin L/P"})
				continue
			}
			if nisSeen[pd.NIS] || nisnSeen[pd.NISN] || nikSeen[pd.NIK] {
				issues = append(issues, importIssue{line, "duplikat NIS/NISN/NIK dalam file"})
				continue
			}
			var class Kelas
			if s.db.First(&class, "id = ?", pd.KelasID).Error != nil {
				issues = append(issues, importIssue{line, "kelas_id tidak ditemukan"})
				continue
			}
			if s.db.First(&OrangTua{}, "id = ?", pd.OrangTuaID).Error != nil {
				issues = append(issues, importIssue{line, "orang_tua_id tidak ditemukan"})
				continue
			}
			if s.db.First(&Pokjar{}, "id = ?", pd.PokjarID).Error != nil {
				issues = append(issues, importIssue{line, "pokjar_id tidak ditemukan"})
				continue
			}
			var dup PesertaDidik
			if s.db.Where("nik = ? AND nik != ''", pd.NIK).First(&dup).Error == nil {
				issues = append(issues, importIssue{line, "NIK sudah ada: " + pd.NIK})
				continue
			}
			if e := s.db.Create(&pd).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal insert (NIS/NISN duplikat?): " + e.Error()})
				continue
			}
			if e := s.db.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: pd.ID, KelasID: pd.KelasID, TahunAjaranID: class.TahunAjaranID, Status: "aktif"}).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal catat riwayat kelas: " + e.Error()})
				continue
			}
			nisSeen[pd.NIS] = true
			nisnSeen[pd.NISN] = true
			nikSeen[pd.NIK] = true
			berhasil++
		}
	case "siswa-lengkap":
		if role != "admin" {
			return fiber.NewError(403, "import peserta didik hanya admin")
		}
		expected := []string{"nama", "jenis_kelamin", "nis", "nisn", "nik", "kelas", "nama_ayah", "nik_ayah", "pekerjaan_ayah", "pendidikan_ayah", "penghasilan_ayah", "nama_ibu", "nik_ibu", "pekerjaan_ibu", "pendidikan_ibu", "penghasilan_ibu"}
		if e := validateImportHeaders(rows[0], expected); e != nil {
			return fiber.NewError(400, e.Error())
		}
		if total > 1000 {
			return fiber.NewError(400, "import dibatasi 1000 baris")
		}
		// Load default pokjar (first one) for auto-assignment
		var defaultPokjar Pokjar
		if s.db.First(&defaultPokjar).Error != nil {
			return fiber.NewError(400, "tidak ada pokjar di database")
		}
		// Pre-load active tahun ajaran
		var activeYear TahunAjaran
		if s.db.Where("is_aktif = ?", true).First(&activeYear).Error != nil {
			return fiber.NewError(400, "tidak ada tahun ajaran aktif")
		}
		nisSeen, nisnSeen, nikSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for index, row := range rows[1:] {
			line := index + 2
			if len(row) < len(expected) {
				issues = append(issues, importIssue{line, "kolom tidak lengkap"})
				continue
			}
			nama := strings.TrimSpace(row[0])
			jk := strings.ToUpper(strings.TrimSpace(row[1]))
			nis := strings.TrimSpace(row[2])
			nisn := strings.TrimSpace(row[3])
			nik := strings.TrimSpace(row[4])
			kelasStr := strings.TrimSpace(row[5])
			namaAyah := strings.TrimSpace(row[6])
			nikAyah := strings.TrimSpace(row[7])
			_ = strings.TrimSpace(row[8])  // pekerjaan_ayah
			_ = strings.TrimSpace(row[9])  // pendidikan_ayah
			_ = strings.TrimSpace(row[10]) // penghasilan_ayah
			namaIbu := strings.TrimSpace(row[11])
			nikIbu := strings.TrimSpace(row[12])
			_ = strings.TrimSpace(row[13]) // pekerjaan_ibu
			_ = strings.TrimSpace(row[14]) // pendidikan_ibu
			_ = strings.TrimSpace(row[15]) // penghasilan_ibu
			if nama == "" || (jk != "L" && jk != "P") || nis == "" || nisn == "" || nik == "" || kelasStr == "" {
				issues = append(issues, importIssue{line, "nama, jenis_kelamin (L/P), nis, nisn, nik, kelas wajib"})
				continue
			}
			if nisSeen[nis] || nisnSeen[nisn] || nikSeen[nik] {
				issues = append(issues, importIssue{line, "duplikat NIS/NISN/NIK dalam file"})
				continue
			}
			// Parse kelas string: "1A" -> jenjang=1, nama_rombel="A"; "KELAS 1 A" -> jenjang=1, nama_rombel="A"
			jenjang, rombel := parseKelasLabel(kelasStr)
			if jenjang < 1 || jenjang > 6 || rombel == "" {
				issues = append(issues, importIssue{line, "format kelas tidak valid (contoh: 1A, 2B, 3C)"})
				continue
			}
			// Find or fail on matching Kelas
			var kelas Kelas
			if s.db.Where("jenjang = ? AND nama_rombel = ? AND tahun_ajaran_id = ?", jenjang, rombel, activeYear.ID).First(&kelas).Error != nil {
				issues = append(issues, importIssue{line, fmt.Sprintf("kelas %s tidak ditemukan untuk tahun ajaran aktif", kelasStr)})
				continue
			}
			// Find or create OrangTua by matching nama_ayah + nama_ibu
			var ortu OrangTua
			ortuFound := false
			if namaAyah != "" && namaIbu != "" {
				ortuFound = s.db.Where("nama_bapak = ? AND nama_ibu = ?", namaAyah, namaIbu).First(&ortu).Error == nil
			}
			if !ortuFound && nikAyah != "" {
				ortuFound = s.db.Where("nik_ayah = ? AND nik_ayah != ''", nikAyah).First(&ortu).Error == nil
			}
			if !ortuFound && nikIbu != "" {
				ortuFound = s.db.Where("nik_ibu = ? AND nik_ibu != ''", nikIbu).First(&ortu).Error == nil
			}
			if !ortuFound {
				ortu = OrangTua{
					NamaBapak: namaAyah,
					NamaIbu:   namaIbu,
					NIKAyah:   nikAyah,
					NIKIbu:    nikIbu,
				}
				if e := s.db.Create(&ortu).Error; e != nil {
					issues = append(issues, importIssue{line, "gagal buat data orang tua: " + e.Error()})
					continue
				}
			}
			// Check NIK uniqueness against DB
			var dup PesertaDidik
			if s.db.Where("nik = ? AND nik != ''", nik).First(&dup).Error == nil {
				issues = append(issues, importIssue{line, "NIK sudah ada: " + nik})
				continue
			}
			pd := PesertaDidik{
				Nama:         nama,
				JenisKelamin: jk,
				NIS:          nis,
				NISN:         nisn,
				NIK:          nik,
				KelasID:      kelas.ID,
				PokjarID:     defaultPokjar.ID,
				OrangTuaID:   ortu.ID,
				Status:       "aktif",
			}
			if e := s.db.Create(&pd).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal insert siswa: " + e.Error()})
				continue
			}
			if e := s.db.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: pd.ID, KelasID: kelas.ID, TahunAjaranID: activeYear.ID, Status: "aktif"}).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal catat riwayat kelas: " + e.Error()})
				continue
			}
			nisSeen[nis] = true
			nisnSeen[nisn] = true
			nikSeen[nik] = true
			berhasil++
		}
	case "nilai-kompetensi":
		kelasID := c.FormValue("kelasId")
		semester := c.FormValue("semester")
		if kelasID == "" || (semester != "Ganjil" && semester != "Genap") {
			return fiber.NewError(400, "kelasId dan semester (Ganjil/Genap) wajib")
		}
		// Autorisasi sama dengan saveNilaiKompetensi: admin lewat, guru wajib
		// wali kelas, kepala_sekolah & role lain ditolak. Sebelumnya hanya
		// dicek untuk guru sehingga kepala_sekolah (read-only) bisa menulis
		// nilai kompetensi lewat import.
		if e := s.canManageKelas(c, kelasID); e != nil {
			return e
		}
		expected := []string{"nisn", "kompetensi_id", "nilai"}
		if e := validateImportHeaders(rows[0], expected); e != nil {
			return fiber.NewError(400, e.Error())
		}
		var allPD []PesertaDidik
		s.db.Where("kelas_id = ?", kelasID).Find(&allPD)
		pdByNISN := map[string]PesertaDidik{}
		for _, p := range allPD {
			if p.NISN != "" {
				pdByNISN[p.NISN] = p
			}
		}
		var rk []RombelKompetensi
		s.db.Where("kelas_id = ?", kelasID).Find(&rk)
		assigned := map[string]bool{}
		for _, r := range rk {
			assigned[r.KompetensiID] = true
		}
		for index, row := range rows[1:] {
			line := index + 2
			if len(row) < len(expected) {
				issues = append(issues, importIssue{line, "kolom tidak lengkap"})
				continue
			}
			nisn := strings.TrimSpace(row[0])
			kompetensiID := strings.TrimSpace(row[1])
			nilaiStr := strings.TrimSpace(row[2])
			pd, ok := pdByNISN[nisn]
			if !ok {
				issues = append(issues, importIssue{line, "NISN tidak ditemukan di kelas: " + nisn})
				continue
			}
			if kompetensiID == "" {
				issues = append(issues, importIssue{line, "kompetensi_id wajib"})
				continue
			}
			if !assigned[kompetensiID] {
				issues = append(issues, importIssue{line, "kompetensi tidak ditugaskan ke kelas ini"})
				continue
			}
			nilai, e := strconv.ParseFloat(nilaiStr, 64)
			if e != nil || nilai < 0 || nilai > 100 {
				issues = append(issues, importIssue{line, "nilai tidak valid (0-100)"})
				continue
			}
			var nk NilaiKompetensi
			fe := s.db.Where("peserta_didik_id = ? AND kompetensi_id = ? AND kelas_id = ? AND semester = ?", pd.ID, kompetensiID, kelasID, semester).First(&nk).Error
			nk.PesertaDidikID = pd.ID
			nk.KompetensiID = kompetensiID
			nk.KelasID = kelasID
			nk.Semester = semester
			nk.Nilai = nilai
			nk.DicatatOlehUserID = uid
			if fe == gorm.ErrRecordNotFound {
				if ce := s.db.Create(&nk).Error; ce != nil {
					issues = append(issues, importIssue{line, "gagal simpan: " + ce.Error()})
					continue
				}
			} else {
				if ce := s.db.Save(&nk).Error; ce != nil {
					issues = append(issues, importIssue{line, "gagal update: " + ce.Error()})
					continue
				}
			}
			berhasil++
		}
	case "tutor":
		if role != "admin" {
			return fiber.NewError(403, "import tutor hanya admin")
		}
		if e := validateTutorImportHeaders(rows[0]); e != nil {
			return fiber.NewError(400, e.Error())
		}
		if total > 500 {
			return fiber.NewError(400, "import tutor dibatasi 500 baris")
		}
		namaSeen := map[string]bool{}
		columns := importColumnIndex(rows[0])
		for index, row := range rows[1:] {
			line := index + 2
			if len(row) < 3 {
				issues = append(issues, importIssue{line, "kolom tidak lengkap"})
				continue
			}
			nama := importCell(row, columns["nama"])
			jk := strings.ToUpper(importCell(row, columns["jenis_kelamin"]))
			noHP := importCell(row, columns["no_hp"])
			alamat := importCell(row, columns["alamat"])
			dateValue := importCell(row, columns["tanggal_mulai_tugas"])
			isRPP := strings.ToLower(importCell(row, columns["is_rpp_maker"]))
			if nama == "" || (jk != "L" && jk != "P") || noHP == "" {
				issues = append(issues, importIssue{line, "nama, no_hp wajib; jenis_kelamin harus L/P"})
				continue
			}
			var tanggal *time.Time
			if dateValue != "" {
				parsed, dateErr := parseFlexibleDate(dateValue)
				if dateErr != nil {
					issues = append(issues, importIssue{line, "tanggal_mulai_tugas tidak valid (YYYY-MM-DD)"})
					continue
				}
				tanggal = &parsed
			}
			if namaSeen[nama] {
				issues = append(issues, importIssue{line, "duplikat nama dalam file: " + nama})
				continue
			}
			var existing Tutor
			if s.db.Where("nama = ?", nama).First(&existing).Error == nil {
				issues = append(issues, importIssue{line, "nama tutor sudah ada: " + nama})
				continue
			}
			tutor := Tutor{
				Nama:            nama,
				JenisKelamin:    jk,
				NoHP:            noHP,
				Alamat:          alamat,
				TanggalBertugas: tanggal,
				IsRPPMaker:      isRPP == "true" || isRPP == "1" || isRPP == "ya",
			}
			if ce := s.db.Create(&tutor).Error; ce != nil {
				issues = append(issues, importIssue{line, "gagal simpan: " + ce.Error()})
				continue
			}
			namaSeen[nama] = true
			berhasil++
		}
	case "orang_tua":
		if role != "admin" {
			return fiber.NewError(403, "import orang tua hanya admin")
		}
		expected := []string{"nama", "nik", "no_hp", "alamat", "username", "password"}
		if e := validateImportHeaders(rows[0], expected); e != nil {
			return fiber.NewError(400, e.Error())
		}
		if total > 500 {
			return fiber.NewError(400, "import orang tua dibatasi 500 baris")
		}
		nikSeen := map[string]bool{}
		usernameSeen := map[string]bool{}
		for index, row := range rows[1:] {
			line := index + 2
			if len(row) < len(expected) {
				issues = append(issues, importIssue{line, "kolom tidak lengkap"})
				continue
			}
			nama := strings.TrimSpace(row[0])
			nik := strings.TrimSpace(row[1])
			noHP := strings.TrimSpace(row[2])
			alamat := strings.TrimSpace(row[3])
			username := strings.TrimSpace(row[4])
			password := strings.TrimSpace(row[5])
			if nama == "" || nik == "" || username == "" || password == "" {
				issues = append(issues, importIssue{line, "nama, nik, username, password wajib"})
				continue
			}
			if nikSeen[nik] {
				issues = append(issues, importIssue{line, "duplikat NIK dalam file"})
				continue
			}
			if usernameSeen[username] {
				issues = append(issues, importIssue{line, "duplikat username dalam file"})
				continue
			}
			// Check NIK uniqueness in DB
			var existing OrangTua
			if s.db.Where("nik = ?", nik).First(&existing).Error == nil {
				issues = append(issues, importIssue{line, "NIK sudah terdaftar"})
				continue
			}
			// Check username uniqueness in DB
			var existingUser User
			if s.db.Where("username = ?", username).First(&existingUser).Error == nil {
				issues = append(issues, importIssue{line, "username sudah digunakan"})
				continue
			}
			// Create OrangTua record
			ortu := OrangTua{NamaBapak: nama, NIKAyah: nik, NamaIbu: noHP, NIKIbu: alamat}
			if e := s.db.Create(&ortu).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal insert orang tua: " + e.Error()})
				continue
			}
			// Create User account for the parent
			hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			user := User{Username: username, PasswordHash: string(hashed), Role: "orang_tua", OrangTuaID: &ortu.ID}
			if e := s.db.Create(&user).Error; e != nil {
				issues = append(issues, importIssue{line, "gagal buat akun: " + e.Error()})
				continue
			}
			nikSeen[nik] = true
			usernameSeen[username] = true
			berhasil++
		}
	default:
		return fiber.NewError(400, "tipe import tidak dikenal")
	}

	errJSON, _ := json.Marshal(issues)
	log := ImportLog{Tipe: tipe, FileName: fileHeader.Filename, TotalBaris: total, Berhasil: berhasil, Gagal: len(issues), ErrorJson: string(errJSON), Status: "selesai", UserID: uid}
	s.db.Create(&log)
	s.audit(&uid, "import", tipe, fmt.Sprintf("%d berhasil / %d gagal", berhasil, len(issues)))
	return c.JSON(fiber.Map{"logId": log.ID, "totalBaris": total, "berhasil": berhasil, "gagal": len(issues), "issues": issues})
}

func (s *Server) listImportLog(c *fiber.Ctx) error {
	var rows []ImportLog
	q := s.db.Order("created_at desc")
	if t := c.Query("tipe"); t != "" {
		q = q.Where("tipe = ?", t)
	}
	if e := q.Limit(100).Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

// activeYearID returns the ID of the currently active TahunAjaran ("" if none).
func (s *Server) activeYearID() string {
	var ta TahunAjaran
	if s.db.Where("is_aktif = ?", true).First(&ta).Error != nil {
		return ""
	}
	return ta.ID
}

// requireActiveYear blocks tema/nilai writes for archived tahun ajaran: nilai
// for a class belonging to a non-active year is read-only.
func (s *Server) requireActiveYear(c *fiber.Ctx, kelasID string) error {
	var k Kelas
	if s.db.First(&k, "id = ?", kelasID).Error != nil {
		return fiber.NewError(404, "kelas not found")
	}
	if k.TahunAjaranID != s.activeYearID() {
		return fiber.NewError(400, "nilai for archived tahun ajaran is read-only")
	}
	return nil
}
func (s *Server) createPresensi(c *fiber.Ctx) error {
	// Hanya field yang boleh diatur klien yang diparse. Sebelumnya
	// BodyParser(&p) ke seluruh struct Presensi membiarkan klien menyetel
	// TutorID (menempel metadata ke tutor lain), DibuatOtomatis=true
	// (menyamarkan presensi otomatis sebagai manual), TanggalRencana, dan
	// Semester sembarangan — field-field itu kini ditentukan server.
	var in struct {
		KelasID         string    `json:"kelasId"`
		Tanggal         time.Time `json:"tanggal"`
		Semester        string    `json:"semester"`
		StatusPertemuan string    `json:"statusPertemuan"`
		Keterangan      string    `json:"keterangan"`
		TandaTangan     string    `json:"tandaTangan"`
		BuktiFoto       string    `json:"buktiFoto"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID == "" {
		return fiber.NewError(400, "kelasId wajib")
	}
	if in.Semester != "" && in.Semester != "Ganjil" && in.Semester != "Genap" {
		return fiber.NewError(400, "semester tidak valid (Ganjil/Genap)")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	var k Kelas
	s.db.First(&k, "id = ?", in.KelasID)
	p := Presensi{
		KelasID:         in.KelasID,
		Tanggal:         in.Tanggal,
		TutorID:         k.WaliKelasID,
		DibuatOtomatis:  false,
		TanggalRencana:  nil,
		StatusPertemuan: in.StatusPertemuan,
		Keterangan:      in.Keterangan,
		TandaTangan:     in.TandaTangan,
		BuktiFoto:       in.BuktiFoto,
	}
	if in.Semester != "" {
		p.Semester = in.Semester
	} else {
		p.Semester = s.semester(in.Tanggal)
	}
	if p.StatusPertemuan == "" {
		p.StatusPertemuan = "berlangsung"
	}
	if !validSignature(p.TandaTangan) {
		return fiber.NewError(400, "a valid PNG signature is required")
	}
	if e := s.db.Create(&p).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "create", "presensi", p.KelasID+" @ "+p.Tanggal.Format("2006-01-02"))
	return c.Status(201).JSON(p)
}
func (s *Server) updatePresensi(c *fiber.Ctx) error {
	var p Presensi
	if e := s.db.First(&p, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	originalKelasID := p.KelasID
	if e := s.canManageKelas(c, p.KelasID); e != nil {
		return e
	}
	// Parse only the mutable fields the client may change. We deliberately do
	// NOT BodyParser into &p: that would let a wali overwrite kelasId/tutorId
	// (an IDOR that moves the attendance record, with its signature & details,
	// into a class they don't own) and re-attach it to another tutor.
	var in struct {
		Tanggal         time.Time `json:"tanggal"`
		StatusPertemuan string    `json:"statusPertemuan"`
		TandaTangan     string    `json:"tandaTangan"`
		BuktiFoto       string    `json:"buktiFoto"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if !validSignature(in.TandaTangan) {
		return fiber.NewError(400, "a valid PNG signature is required")
	}
	p.Tanggal = in.Tanggal
	if in.StatusPertemuan != "" {
		p.StatusPertemuan = in.StatusPertemuan
	}
	p.TandaTangan = in.TandaTangan
	if in.BuktiFoto != "" {
		p.BuktiFoto = in.BuktiFoto
	}
	// Guard against any path that could have moved the record off its class.
	if p.KelasID != originalKelasID {
		return fiber.NewError(400, "kelasId cannot be changed")
	}
	if p.TanggalRencana != nil && !sameDay(p.Tanggal, *p.TanggalRencana) && p.StatusPertemuan == "berlangsung" {
		p.StatusPertemuan = "dipindah"
	}
	if e := s.db.Save(&p).Error; e != nil {
		return fiber.NewError(400, e.Error())
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "presensi", p.ID)
	return c.JSON(p)
}
func (s *Server) saveDetails(c *fiber.Ctx) error {
	var p Presensi
	if e := s.db.First(&p, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "record not found")
	}
	if e := s.canManageKelas(c, p.KelasID); e != nil {
		return e
	}
	var details []PresensiDetail
	if e := c.BodyParser(&details); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	// Collect the students who actually belong to this class so we reject
	// pesertaDidikId values from other classes (which would otherwise inflate
	// other classes' students' attendance records via FirstOrCreate).
	var studentIDs []string
	if e := s.db.Model(&PesertaDidik{}).Where("kelas_id = ?", p.KelasID).Pluck("id", &studentIDs).Error; e != nil {
		return e
	}
	belongs := make(map[string]bool, len(studentIDs))
	for _, sid := range studentIDs {
		belongs[sid] = true
	}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range details {
			d := details[i]
			d.PresensiID = p.ID
			if d.PesertaDidikID == "" || !belongs[d.PesertaDidikID] {
				return fiber.NewError(400, "pesertaDidikId does not belong to this class")
			}
			if d.StatusKehadiran == "" {
				d.StatusKehadiran = "Hadir"
			}
			if e := tx.Where(PresensiDetail{PresensiID: p.ID, PesertaDidikID: d.PesertaDidikID}).Assign(d).FirstOrCreate(&d).Error; e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "update", "presensi_detail", p.ID)
	return c.SendStatus(204)
}

// semester is the single source of truth for the Ganjil/Genap split, shared by
// the Presensi module and the Peminjaman Buku module (PRD §4.2a). When the active
// TahunAjaran has tanggal_mulai_semester_genap set, the split is date-driven:
// Ganjil = tanggalMulai .. (day before genap); Genap = genap .. tanggalSelesai.
// When the field is empty/zero, it falls back to the legacy month-based rule
// (month>=7 → Ganjil) so existing Presensi behavior is unchanged until an admin
// fills the new field.
func (s *Server) semester(t time.Time) string {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc != nil {
		t = t.In(loc)
	}
	var ta TahunAjaran
	if s.db.Where("is_aktif = ?", true).First(&ta).Error == nil {
		// Cari semester yang mencakup waktu t
		var sems []Semester
		s.db.Where("tahun_ajaran_id = ?", ta.ID).Find(&sems)
		for _, sem := range sems {
			mulai := sem.TanggalMulai
			selesai := sem.TanggalSelesai
			if loc != nil {
				mulai = mulai.In(loc)
				selesai = selesai.In(loc)
			}
			if !t.Before(mulai) && !t.After(selesai) {
				return sem.NamaSemester
			}
		}
		// Fallback: gunakan tanggalMulaiSemesterGenap bila ada
		if ta.TanggalMulaiSemesterGenap != nil && !ta.TanggalMulaiSemesterGenap.IsZero() {
			genap := ta.TanggalMulaiSemesterGenap.In(loc)
			if t.Before(genap) {
				return "Ganjil"
			}
			return "Genap"
		}
	}
	// Legacy fallback
	if t.Month() >= 7 {
		return "Ganjil"
	}
	return "Genap"
}
func sameDay(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }
func (s *Server) startScheduler() {
	loc, e := time.LoadLocation("Asia/Jakarta")
	if e != nil {
		return
	}
	cr := cron.New(cron.WithLocation(loc))
	// Check the single-row setting every minute so schedule changes take effect without a restart.
	cr.AddFunc("* * * * *", func() {
		var setting PengaturanJadwal
		if s.db.First(&setting).Error != nil || setting.JamGenerate == "" {
			return
		}
		now := time.Now().In(loc)
		// Fire once a day at the configured generate time. generateAttendance is
		// idempotent (it skips any class whose meeting for the target day already
		// exists), so running daily instead of gating on a fixed weekday means a
		// single downtime window no longer loses a whole week of auto presensi.
		// The target weekday comes from setting.HariDefault, not a hard constant.
		if now.Format("15:04") == setting.JamGenerate {
			s.generateAttendance(loc)
		}
	})
	// Modul Buku — daily n8n reminder (PRD §5.8). Separate job from the presensi
	// generator; fires at 06:07 WIB (off the :00/:30 mark) and is a no-op unless
	// N8N_WEBHOOK_URL is set and today is a milestone day (H-14/H-7/H-1).
	cr.AddFunc("7 6 * * *", func() { s.reminderBuku(loc) })
	// Scheduled full database backup — disabled unless BACKUP_CRON is set (e.g.
	// "0 2 * * *" for 02:00 WIB daily). When unset, backups are n8n-driven via
	// GET /api/backup/download. Retention via BACKUP_RETENTION (default 14),
	// format via BACKUP_FORMAT (full|db|sql, default full).
	if sched := os.Getenv("BACKUP_CRON"); sched != "" {
		if id, err := cr.AddFunc(sched, func() { s.runScheduledBackup() }); err != nil {
			fmt.Printf("BACKUP_CRON invalid (%q): %v\n", sched, err)
		} else {
			fmt.Printf("scheduled backup job registered (id=%d, cron=%q)\n", id, sched)
		}
	}
	cr.Start()
	// Auto-finish expired ujian online sessions every 30 seconds.
	go func() {
		for {
			time.Sleep(30 * time.Second)
			s.autoFinishUjianSessions()
		}
	}()
}
func (s *Server) generateAttendance(loc *time.Location) {
	var setting PengaturanJadwal
	s.db.First(&setting)
	target := nextWeekday(time.Now().In(loc), setting.HariDefault)
	var active TahunAjaran
	if s.db.Where("is_aktif = ?", true).First(&active).Error != nil {
		return
	}
	var classes []Kelas
	s.db.Where("tahun_ajaran_id = ? AND wali_kelas_id IS NOT NULL", active.ID).Find(&classes)
	for _, k := range classes {
		var count int64
		s.db.Model(&Presensi{}).Where("kelas_id = ? AND tanggal_rencana = ?", k.ID, target).Count(&count)
		if count == 0 {
			s.db.Create(&Presensi{KelasID: k.ID, Tanggal: target, TanggalRencana: &target, Semester: s.semester(target), StatusPertemuan: "berlangsung", DibuatOtomatis: true, TutorID: k.WaliKelasID})
		}
	}
}
func nextWeekday(from time.Time, day string) time.Time {
	names := map[string]time.Weekday{"Minggu": 0, "Senin": 1, "Selasa": 2, "Rabu": 3, "Kamis": 4, "Jumat": 5, "Sabtu": 6}
	wanted := names[day]
	delta := (int(wanted) - int(from.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	return time.Date(from.Year(), from.Month(), from.Day()+delta, 0, 0, 0, 0, from.Location())
}

// ===== Modul Nilai (prd_nilai.md) =====

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// predikatFor maps a final nilai to a predikat letter using the ambang list
// (sorted desc by nilai_minimum). Nil nilai → ""; below the lowest threshold → "".
func predikatFor(v *float64, ambangs []AmbangPredikat) string {
	if v == nil {
		return ""
	}
	for _, a := range ambangs {
		if *v >= a.NilaiMinimum {
			return a.Predikat
		}
	}
	return ""
}

// listTema returns temas matching the optional kelas/mapel/semester/tahun filter,
// with Capaian + lookup relations preloaded. Guru is scoped to the kelas+mapel
// pairs they are assigned to teach; admin/kepala see all matching temas.
func (s *Server) listTema(c *fiber.Ctx) error {
	q := s.db.Preload("Capaian").Preload("Kelas").Preload("Mapel").Preload("TahunAjaran")
	if v := c.Query("kelasId"); v != "" {
		q = q.Where("kelas_id = ?", v)
	}
	if v := c.Query("mapelId"); v != "" {
		q = q.Where("mapel_id = ?", v)
	}
	if v := c.Query("semester"); v != "" {
		q = q.Where("semester = ?", v)
	}
	if v := c.Query("tahunAjaranId"); v != "" {
		q = q.Where("tahun_ajaran_id = ?", v)
	}
	if c.Locals("role") == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return c.JSON([]Tema{})
		}
		var pgs []PenugasanGuruMapel
		s.db.Where("tutor_id = ?", *u.TutorID).Find(&pgs)
		if len(pgs) == 0 {
			return c.JSON([]Tema{})
		}
		pairs := make([]string, 0, len(pgs))
		for _, pg := range pgs {
			pairs = append(pairs, "(kelas_id = '"+pg.KelasID+"' AND mapel_id = '"+pg.MapelID+"')")
		}
		q = q.Where("(" + strings.Join(pairs, " OR ") + ")")
	}
	q = q.Order("urutan asc")
	var rows []Tema
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

type temaInput struct {
	KelasID           string   `json:"kelasId"`
	MapelID           string   `json:"mapelId"`
	TahunAjaranID     string   `json:"tahunAjaranId"`
	Semester          string   `json:"semester"`
	NamaTema          string   `json:"namaTema"`
	Urutan            int      `json:"urutan"`
	JumlahCP          int      `json:"jumlahCp"`
	LabelDefaults     []string `json:"labelDefaults"`
	BobotKeterampilan *float64 `json:"bobotKeterampilan"`
	BobotPengetahuan  *float64 `json:"bobotPengetahuan"`
}

func (s *Server) createTema(c *fiber.Ctx) error {
	var in temaInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID == "" || in.MapelID == "" || in.TahunAjaranID == "" {
		return fiber.NewError(400, "kelasId, mapelId and tahunAjaranId are required")
	}
	if in.Semester != "Ganjil" && in.Semester != "Genap" {
		return fiber.NewError(400, "semester must be Ganjil or Genap")
	}
	if in.JumlahCP < 1 {
		return fiber.NewError(400, "jumlahCp must be at least 1")
	}
	if len(in.LabelDefaults) != in.JumlahCP {
		return fiber.NewError(400, "labelDefaults length must equal jumlahCp")
	}
	if e := s.canManageKelasMapel(c, in.KelasID, in.MapelID); e != nil {
		return e
	}
	if e := s.requireActiveYear(c, in.KelasID); e != nil {
		return e
	}
	// Snapshot bobot from mapel settings, unless an override is provided (sum must be 100).
	bk, bp := 60.0, 40.0
	var set PengaturanBobotNilai
	if s.db.Where("mapel_id = ?", in.MapelID).First(&set).Error == nil {
		bk, bp = set.BobotKeterampilan, set.BobotPengetahuan
	}
	if in.BobotKeterampilan != nil && in.BobotPengetahuan != nil {
		if *in.BobotKeterampilan+*in.BobotPengetahuan != 100 {
			return fiber.NewError(400, "bobot keterampilan + pengetahuan must equal 100")
		}
		bk, bp = *in.BobotKeterampilan, *in.BobotPengetahuan
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		tema := Tema{
			KelasID: in.KelasID, MapelID: in.MapelID, TahunAjaranID: in.TahunAjaranID,
			Semester: in.Semester, NamaTema: in.NamaTema, Urutan: in.Urutan, JumlahCP: in.JumlahCP,
			BobotKeterampilan: bk, BobotPengetahuan: bp,
		}
		if e := tx.Create(&tema).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
		for i := 0; i < in.JumlahCP; i++ {
			if e := tx.Create(&CapaianPembelajaran{TemaID: tema.ID, UrutanCP: i + 1, LabelDefault: in.LabelDefaults[i]}).Error; e != nil {
				return e
			}
		}
		// Seed empty NilaiCP + NilaiUM rows for every active student in the class.
		var students []PesertaDidik
		if e := tx.Where("kelas_id = ? AND status = ?", in.KelasID, "aktif").Find(&students).Error; e != nil {
			return e
		}
		for _, st := range students {
			for i := 0; i < in.JumlahCP; i++ {
				if e := tx.Create(&NilaiCP{TemaID: tema.ID, UrutanCP: i + 1, PesertaDidikID: st.ID, DeskripsiCP: in.LabelDefaults[i]}).Error; e != nil {
					return e
				}
			}
			if e := tx.Create(&NilaiUM{TemaID: tema.ID, PesertaDidikID: st.ID}).Error; e != nil {
				return e
			}
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "create", "tema", tema.NamaTema)
		return c.Status(201).JSON(tema)
	})
}

func (s *Server) updateTema(c *fiber.Ctx) error {
	var tema Tema
	if e := s.db.First(&tema, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tema not found")
	}
	if e := s.canManageKelasMapel(c, tema.KelasID, tema.MapelID); e != nil {
		return e
	}
	if e := s.requireActiveYear(c, tema.KelasID); e != nil {
		return e
	}
	var in temaInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	var students []PesertaDidik
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		if in.NamaTema != "" {
			tema.NamaTema = in.NamaTema
		}
		if in.Urutan != 0 {
			tema.Urutan = in.Urutan
		}
		newJumlah := tema.JumlahCP
		if in.JumlahCP > 0 {
			newJumlah = in.JumlahCP
		}
		if in.BobotKeterampilan != nil && in.BobotPengetahuan != nil {
			if *in.BobotKeterampilan+*in.BobotPengetahuan != 100 {
				return fiber.NewError(400, "bobot keterampilan + pengetahuan must equal 100")
			}
			tema.BobotKeterampilan = *in.BobotKeterampilan
			tema.BobotPengetahuan = *in.BobotPengetahuan
		}
		// Adjust CP rows + per-student NilaiCP rows when jumlahCp changes.
		if in.JumlahCP > 0 && in.JumlahCP != tema.JumlahCP {
			if in.JumlahCP > tema.JumlahCP {
				// Append new CPs (labelDefaults covers the appended range).
				need := in.JumlahCP - tema.JumlahCP
				if len(in.LabelDefaults) < need {
					return fiber.NewError(400, "labelDefaults must include the appended CPs")
				}
				for i := 0; i < need; i++ {
					uc := tema.JumlahCP + i + 1
					if e := tx.Create(&CapaianPembelajaran{TemaID: tema.ID, UrutanCP: uc, LabelDefault: in.LabelDefaults[i]}).Error; e != nil {
						return e
					}
					var sts []PesertaDidik
					if e := tx.Where("kelas_id = ? AND status = ?", tema.KelasID, "aktif").Find(&sts).Error; e != nil {
						return e
					}
					for _, st := range sts {
						if e := tx.Create(&NilaiCP{TemaID: tema.ID, UrutanCP: uc, PesertaDidikID: st.ID, DeskripsiCP: in.LabelDefaults[i]}).Error; e != nil {
							return e
						}
					}
				}
			} else if in.JumlahCP < tema.JumlahCP {
				if e := tx.Where("tema_id = ? AND urutan_cp > ?", tema.ID, in.JumlahCP).Delete(&CapaianPembelajaran{}).Error; e != nil {
					return e
				}
				if e := tx.Where("tema_id = ? AND urutan_cp > ?", tema.ID, in.JumlahCP).Delete(&NilaiCP{}).Error; e != nil {
					return e
				}
			}
			tema.JumlahCP = newJumlah
		}
		// Bulk-apply changed labelDefaults: update CapaianPembelajaran.LabelDefault and
		// every NilaiCP.DeskripsiCP where Manual==false.
		for i, lbl := range in.LabelDefaults {
			uc := i + 1
			if uc > tema.JumlahCP {
				continue
			}
			var cp CapaianPembelajaran
			if e := tx.Where("tema_id = ? AND urutan_cp = ?", tema.ID, uc).First(&cp).Error; e != nil {
				continue
			}
			if cp.LabelDefault != lbl {
				if e := tx.Model(&CapaianPembelajaran{}).Where("id = ?", cp.ID).Update("label_default", lbl).Error; e != nil {
					return e
				}
				if e := tx.Model(&NilaiCP{}).Where("tema_id = ? AND urutan_cp = ? AND manual = ?", tema.ID, uc, false).Update("deskripsi_cp", lbl).Error; e != nil {
					return e
				}
			}
		}
		if e := tx.Save(&tema).Error; e != nil {
			return e
		}
		if e := tx.Where("kelas_id = ? AND status = ?", tema.KelasID, "aktif").Find(&students).Error; e != nil {
			return e
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "update", "tema", tema.NamaTema)
		return nil
	}); e != nil {
		return e
	}
	// Recompute rekap for all affected students after structural changes.
	for _, st := range students {
		if e := s.recomputeRekap(s.db, st.ID, tema.MapelID, tema.KelasID, tema.TahunAjaranID, tema.Semester); e != nil {
			return e
		}
	}
	return c.JSON(tema)
}

func (s *Server) deleteTema(c *fiber.Ctx) error {
	var tema Tema
	if e := s.db.First(&tema, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tema not found")
	}
	if e := s.canManageKelasMapel(c, tema.KelasID, tema.MapelID); e != nil {
		return e
	}
	if e := s.requireActiveYear(c, tema.KelasID); e != nil {
		return e
	}
	// Collect affected students before deletion so rekap can be recomputed/cleaned.
	var students []PesertaDidik
	s.db.Where("kelas_id = ? AND status = ?", tema.KelasID, "aktif").Find(&students)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("tema_id = ?", tema.ID).Delete(&NilaiCP{}).Error; e != nil {
			return e
		}
		if e := tx.Where("tema_id = ?", tema.ID).Delete(&NilaiUM{}).Error; e != nil {
			return e
		}
		if e := tx.Where("tema_id = ?", tema.ID).Delete(&CapaianPembelajaran{}).Error; e != nil {
			return e
		}
		if e := tx.Delete(&tema).Error; e != nil {
			return e
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "delete", "tema", tema.NamaTema)
		return nil
	})
	if err != nil {
		return err
	}
	for _, st := range students {
		if e := s.recomputeRekap(s.db, st.ID, tema.MapelID, tema.KelasID, tema.TahunAjaranID, tema.Semester); e != nil {
			return e
		}
	}
	return c.SendStatus(204)
}

// gridTema returns the input grid for one tema: students × CP + UM + computed columns.
func (s *Server) gridTema(c *fiber.Ctx) error {
	var tema Tema
	if e := s.db.Preload("Capaian").First(&tema, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tema not found")
	}
	// Read access: admin/kepala pass via readAll middleware; guru must be assigned.
	if c.Locals("role") == "guru" {
		if e := s.canManageKelasMapel(c, tema.KelasID, tema.MapelID); e != nil {
			return e
		}
	}
	var students []PesertaDidik
	s.db.Where("kelas_id = ? AND status = ?", tema.KelasID, "aktif").Order("nama").Find(&students)
	// Capaian preload is not ordered by GORM; sort explicitly by urutan_cp.
	sort.Slice(tema.Capaian, func(i, j int) bool { return tema.Capaian[i].UrutanCP < tema.Capaian[j].UrutanCP })
	var cps []NilaiCP
	s.db.Where("tema_id = ?", tema.ID).Find(&cps)
	var ums []NilaiUM
	s.db.Where("tema_id = ?", tema.ID).Find(&ums)

	cpByStudent := map[string]map[int]NilaiCP{}
	for _, cp := range cps {
		m := cpByStudent[cp.PesertaDidikID]
		if m == nil {
			m = map[int]NilaiCP{}
		}
		m[cp.UrutanCP] = cp
		cpByStudent[cp.PesertaDidikID] = m
	}
	umByStudent := map[string]NilaiUM{}
	for _, u := range ums {
		umByStudent[u.PesertaDidikID] = u
	}

	type cpCell struct {
		UrutanCP          int      `json:"urutanCp"`
		LabelDefault      string   `json:"labelDefault"`
		DeskripsiCP       string   `json:"deskripsiCp"`
		Manual            bool     `json:"manual"`
		NilaiKeterampilan *float64 `json:"nilaiKeterampilan"`
		NilaiAkhir        *float64 `json:"nilaiAkhir"`
	}
	type studentRow struct {
		PesertaDidik PesertaDidik `json:"pesertaDidik"`
		CP           []cpCell     `json:"cp"`
		NilaiUm      *float64     `json:"nilaiUm"`
		NKTema       *float64     `json:"nkTema"`
	}
	rows := make([]studentRow, 0, len(students))
	bk := tema.BobotKeterampilan / 100.0
	bp := tema.BobotPengetahuan / 100.0
	for _, st := range students {
		m := cpByStudent[st.ID]
		um := umByStudent[st.ID]
		cells := make([]cpCell, 0, len(tema.Capaian))
		var nkSum float64
		var nkCnt int
		for _, cap := range tema.Capaian {
			cell := cpCell{UrutanCP: cap.UrutanCP, LabelDefault: cap.LabelDefault}
			if v, ok := m[cap.UrutanCP]; ok {
				cell.DeskripsiCP = v.DeskripsiCP
				cell.Manual = v.Manual
				cell.NilaiKeterampilan = v.NilaiKeterampilan
				if v.NilaiKeterampilan != nil {
					nkSum += *v.NilaiKeterampilan
					nkCnt++
				}
			}
			if cell.NilaiKeterampilan != nil && um.NilaiUM != nil {
				na := round2(bk**cell.NilaiKeterampilan + bp**um.NilaiUM)
				cell.NilaiAkhir = &na
			}
			cells = append(cells, cell)
		}
		row := studentRow{PesertaDidik: st, CP: cells}
		if um.NilaiUM != nil {
			row.NilaiUm = um.NilaiUM
		}
		if nkCnt > 0 {
			nk := round2(nkSum / float64(nkCnt))
			row.NKTema = &nk
		}
		rows = append(rows, row)
	}
	return c.JSON(fiber.Map{
		"tema":     tema,
		"bobot":    fiber.Map{"keterampilan": tema.BobotKeterampilan, "pengetahuan": tema.BobotPengetahuan},
		"students": rows,
	})
}

type nilaiSaveInput struct {
	Values []struct {
		PesertaDidikID string `json:"pesertaDidikId"`
		CP             []struct {
			UrutanCP          int      `json:"urutanCp"`
			DeskripsiCP       string   `json:"deskripsiCp"`
			NilaiKeterampilan *float64 `json:"nilaiKeterampilan"`
		} `json:"cp"`
		NilaiUm *float64 `json:"nilaiUm"`
	} `json:"values"`
}

func (s *Server) saveNilai(c *fiber.Ctx) error {
	var tema Tema
	if e := s.db.First(&tema, "id = ?", id(c)).Error; e != nil {
		return fiber.NewError(404, "tema not found")
	}
	if e := s.canManageKelasMapel(c, tema.KelasID, tema.MapelID); e != nil {
		return e
	}
	if e := s.requireActiveYear(c, tema.KelasID); e != nil {
		return e
	}
	var in nilaiSaveInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if len(in.Values) == 0 {
		return fiber.NewError(400, "values is required")
	}
	// Validate ALL rows first (reject entire batch on any violation).
	for _, v := range in.Values {
		if v.PesertaDidikID == "" {
			return fiber.NewError(400, "pesertaDidikId is required for every value")
		}
		for _, cp := range v.CP {
			if cp.NilaiKeterampilan != nil && (*cp.NilaiKeterampilan < 0 || *cp.NilaiKeterampilan > 100) {
				return fiber.NewError(400, "nilai keterampilan must be between 0 and 100")
			}
		}
		if v.NilaiUm != nil && (*v.NilaiUm < 0 || *v.NilaiUm > 100) {
			return fiber.NewError(400, "nilai um must be between 0 and 100")
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, v := range in.Values {
			for _, cpIn := range v.CP {
				var existing NilaiCP
				err := tx.Where("tema_id = ? AND urutan_cp = ? AND peserta_didik_id = ?", tema.ID, cpIn.UrutanCP, v.PesertaDidikID).First(&existing).Error
				// Resolve current label_default to decide the manual flag.
				var cap CapaianPembelajaran
				tx.Where("tema_id = ? AND urutan_cp = ?", tema.ID, cpIn.UrutanCP).First(&cap)
				manual := false
				if cpIn.DeskripsiCP != "" && cpIn.DeskripsiCP != cap.LabelDefault {
					manual = true
				}
				if err == gorm.ErrRecordNotFound {
					if e := tx.Create(&NilaiCP{TemaID: tema.ID, UrutanCP: cpIn.UrutanCP, PesertaDidikID: v.PesertaDidikID, DeskripsiCP: cpIn.DeskripsiCP, Manual: manual, NilaiKeterampilan: cpIn.NilaiKeterampilan}).Error; e != nil {
						return e
					}
				} else if err == nil {
					existing.DeskripsiCP = cpIn.DeskripsiCP
					if cpIn.DeskripsiCP != "" {
						existing.Manual = manual
					}
					existing.NilaiKeterampilan = cpIn.NilaiKeterampilan
					if e := tx.Save(&existing).Error; e != nil {
						return e
					}
				} else {
					return err
				}
			}
			// Upsert NilaiUM.
			var um NilaiUM
			err := tx.Where("tema_id = ? AND peserta_didik_id = ?", tema.ID, v.PesertaDidikID).First(&um).Error
			if err == gorm.ErrRecordNotFound {
				if e := tx.Create(&NilaiUM{TemaID: tema.ID, PesertaDidikID: v.PesertaDidikID, NilaiUM: v.NilaiUm}).Error; e != nil {
					return e
				}
			} else if err == nil {
				um.NilaiUM = v.NilaiUm
				if e := tx.Save(&um).Error; e != nil {
					return e
				}
			} else {
				return err
			}
		}
		// Recompute rekap for every student in the batch.
		for _, v := range in.Values {
			if e := s.recomputeRekap(tx, v.PesertaDidikID, tema.MapelID, tema.KelasID, tema.TahunAjaranID, tema.Semester); e != nil {
				return e
			}
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "update", "nilai", tema.NamaTema)
		return c.JSON(fiber.Map{"saved": len(in.Values)})
	})
}

// recomputeRekap recomputes the RekapNilaiAkhir for one student+mapel+semester+year.
// NP = average of all NilaiUM across temas in scope; NK = average of per-tema average
// of NilaiCP.NilaiKeterampilan. Null inputs are skipped. If no tema remains in scope,
// the rekap row is deleted. Run inside the caller's transaction (tx).
func (s *Server) recomputeRekap(tx *gorm.DB, pesertaDidikID, mapelID, kelasID, tahunAjaranID, semester string) error {
	var temas []Tema
	if e := tx.Where("kelas_id = ? AND mapel_id = ? AND tahun_ajaran_id = ? AND semester = ?", kelasID, mapelID, tahunAjaranID, semester).Find(&temas).Error; e != nil {
		return e
	}
	temaIDs := make([]string, len(temas))
	for i, t := range temas {
		temaIDs[i] = t.ID
	}
	// No tema left in scope → remove the rekap row entirely.
	if len(temaIDs) == 0 {
		return tx.Where("peserta_didik_id = ? AND mapel_id = ? AND semester = ? AND tahun_ajaran_id = ?", pesertaDidikID, mapelID, semester, tahunAjaranID).Delete(&RekapNilaiAkhir{}).Error
	}
	// NP = average of non-null NilaiUM across temas.
	var ums []NilaiUM
	tx.Where("peserta_didik_id = ? AND tema_id IN ?", pesertaDidikID, temaIDs).Find(&ums)
	var npSum float64
	var npCnt int
	for _, u := range ums {
		if u.NilaiUM != nil {
			npSum += *u.NilaiUM
			npCnt++
		}
	}
	var np *float64
	if npCnt > 0 {
		v := round2(npSum / float64(npCnt))
		np = &v
	}
	// NK = average of per-tema average of non-null NilaiKeterampilan.
	var nkTemaVals []float64
	for _, t := range temas {
		var cps []NilaiCP
		tx.Where("tema_id = ? AND peserta_didik_id = ?", t.ID, pesertaDidikID).Find(&cps)
		var sum float64
		var cnt int
		for _, cp := range cps {
			if cp.NilaiKeterampilan != nil {
				sum += *cp.NilaiKeterampilan
				cnt++
			}
		}
		if cnt > 0 {
			nkTemaVals = append(nkTemaVals, sum/float64(cnt))
		}
	}
	var nk *float64
	if len(nkTemaVals) > 0 {
		total := 0.0
		for _, v := range nkTemaVals {
			total += v
		}
		avg := round2(total / float64(len(nkTemaVals)))
		nk = &avg
	}
	// Predikat via AmbangPredikat (sorted desc by nilai_minimum).
	var ambangs []AmbangPredikat
	tx.Where("mapel_id = ?", mapelID).Order("nilai_minimum desc").Find(&ambangs)
	predNP := predikatFor(np, ambangs)
	predNK := predikatFor(nk, ambangs)
	// Modul S — NA gabungan berbobot bila ada BobotSumberNilai untuk mapel; bila tidak
	// ada → NA kosong (fallback: NP & NK terpisah seperti perilaku lama). Backward
	// compatible: tabel tanpa bobot tidak berubah.
	var na *float64
	var predNA string
	var bobots []BobotSumberNilai
	tx.Where("mapel_id = ?", mapelID).Find(&bobots)
	if len(bobots) > 0 {
		var sumbers []SumberNilai
		tx.Find(&sumbers)
		kodeByID := map[string]string{}
		for _, su := range sumbers {
			kodeByID[su.ID] = su.Kode
		}
		var weighted, totalBobot float64
		for _, b := range bobots {
			v := nilaiSumber(tx, pesertaDidikID, kelasID, mapelID, semester, kodeByID[b.SumberID], np)
			if v != nil {
				weighted += *v * b.Bobot
				totalBobot += b.Bobot
			}
		}
		if totalBobot > 0 {
			v := round2(weighted / totalBobot)
			na = &v
			predNA = predikatFor(na, ambangs)
		}
	}
	// Upsert rekap row.
	var r RekapNilaiAkhir
	err := tx.Where("peserta_didik_id = ? AND mapel_id = ? AND semester = ? AND tahun_ajaran_id = ?", pesertaDidikID, mapelID, semester, tahunAjaranID).First(&r).Error
	r.PesertaDidikID = pesertaDidikID
	r.KelasID = kelasID
	r.MapelID = mapelID
	r.TahunAjaranID = tahunAjaranID
	r.Semester = semester
	r.NPAkhir = np
	r.PredikatNP = predNP
	r.NKAkhir = nk
	r.PredikatNK = predNK
	r.NAAkhir = na
	r.PredikatNA = predNA
	if err == gorm.ErrRecordNotFound {
		return tx.Create(&r).Error
	} else if err != nil {
		return err
	}
	return tx.Save(&r).Error
}

// nilaiSumber returns the per-source average nilai for one (peserta, kelas, mapel,
// semester). UM = avg NilaiUM (== np yang sudah dihitung pemanggil). TUGAS = avg
// PengumpulanTugas.Nilai berstatus "Dinilai" untuk tugas di kelas+mapel+semester.
// UJIAN = avg UjianPeserta.Skor untuk ujian online di mapel+semester.
// PRAKTIK (Modul M, belum) → nil.
func nilaiSumber(tx *gorm.DB, pesertaDidikID, kelasID, mapelID, semester, kode string, np *float64) *float64 {
	switch kode {
	case "UM":
		return np
	case "TUGAS":
		var tugasRows []Tugas
		tx.Where("kelas_id = ? AND mapel_id = ? AND semester = ?", kelasID, mapelID, semester).Find(&tugasRows)
		if len(tugasRows) == 0 {
			return nil
		}
		ids := make([]string, len(tugasRows))
		for i, t := range tugasRows {
			ids[i] = t.ID
		}
		var pks []PengumpulanTugas
		tx.Where("peserta_didik_id = ? AND tugas_id IN ? AND status = ?", pesertaDidikID, ids, "Dinilai").Find(&pks)
		var sum float64
		var cnt int
		for _, p := range pks {
			if p.Nilai != nil {
				sum += *p.Nilai
				cnt++
			}
		}
		if cnt == 0 {
			return nil
		}
		v := round2(sum / float64(cnt))
		return &v
	case "UJIAN":
		var ujians []Ujian
		tx.Where("mapel_id = ? AND semester = ?", mapelID, semester).Find(&ujians)
		if len(ujians) == 0 {
			return nil
		}
		ujianIDs := make([]string, len(ujians))
		for i, u := range ujians {
			ujianIDs[i] = u.ID
		}
		var pesertas []UjianPeserta
		tx.Where("ujian_id IN ? AND peserta_didik_id = ? AND status = ?", ujianIDs, pesertaDidikID, "selesai").Find(&pesertas)
		var sum float64
		var cnt int
		for _, p := range pesertas {
			if p.Skor != nil {
				sum += *p.Skor
				cnt++
			}
		}
		if cnt == 0 {
			return nil
		}
		v := round2(sum / float64(cnt))
		return &v
	}
	return nil
}

// getSettingsNilai returns the bobot + ambang config for one mapel (or all when mapelId is empty).
func (s *Server) getSettingsNilai(c *fiber.Ctx) error {
	q := s.db.Preload("Mapel")
	if v := c.Query("mapelId"); v != "" {
		var set PengaturanBobotNilai
		if e := q.Where("mapel_id = ?", v).First(&set).Error; e != nil {
			return fiber.NewError(404, "settings not found for mapel")
		}
		var ambangs []AmbangPredikat
		s.db.Where("mapel_id = ?", v).Order("nilai_minimum desc").Find(&ambangs)
		return c.JSON(fiber.Map{"mapelId": set.MapelID, "bobotKeterampilan": set.BobotKeterampilan, "bobotPengetahuan": set.BobotPengetahuan, "ambang": ambangs, "mapel": set.Mapel})
	}
	var sets []PengaturanBobotNilai
	if e := q.Find(&sets).Error; e != nil {
		return e
	}
	return c.JSON(sets)
}

type settingsNilaiInput struct {
	MapelID           string  `json:"mapelId"`
	BobotKeterampilan float64 `json:"bobotKeterampilan"`
	BobotPengetahuan  float64 `json:"bobotPengetahuan"`
	Ambang            []struct {
		Predikat     string  `json:"predikat"`
		NilaiMinimum float64 `json:"nilaiMinimum"`
	} `json:"ambang"`
}

func (s *Server) putSettingsNilai(c *fiber.Ctx) error {
	var in settingsNilaiInput
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.MapelID == "" {
		return fiber.NewError(400, "mapelId is required")
	}
	if in.BobotKeterampilan+in.BobotPengetahuan != 100 {
		return fiber.NewError(400, "bobot keterampilan + pengetahuan must equal 100")
	}
	if len(in.Ambang) != 3 {
		return fiber.NewError(400, "exactly 3 ambang rows (A/B/C) are required")
	}
	predOK := map[string]bool{"A": true, "B": true, "C": true}
	for _, a := range in.Ambang {
		if !predOK[a.Predikat] {
			return fiber.NewError(400, "predikat must be A, B or C")
		}
		if a.NilaiMinimum < 0 || a.NilaiMinimum > 100 {
			return fiber.NewError(400, "nilai minimum must be between 0 and 100")
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var set PengaturanBobotNilai
		if e := tx.Where("mapel_id = ?", in.MapelID).First(&set).Error; e != nil {
			set = PengaturanBobotNilai{MapelID: in.MapelID}
		}
		set.BobotKeterampilan = in.BobotKeterampilan
		set.BobotPengetahuan = in.BobotPengetahuan
		if e := tx.Save(&set).Error; e != nil {
			return e
		}
		if e := tx.Where("mapel_id = ?", in.MapelID).Delete(&AmbangPredikat{}).Error; e != nil {
			return e
		}
		for _, a := range in.Ambang {
			if e := tx.Create(&AmbangPredikat{MapelID: in.MapelID, Predikat: a.Predikat, NilaiMinimum: a.NilaiMinimum}).Error; e != nil {
				return e
			}
		}
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "update", "pengaturan_nilai", in.MapelID)
		return c.JSON(fiber.Map{"ok": true})
	})
}

// getRekap returns RekapNilaiAkhir rows filtered by kelas+semester+tahun (mapel optional).
// Guru is restricted to kelas+mapel pairs they are assigned to teach.
func (s *Server) getRekap(c *fiber.Ctx) error {
	kelasID := c.Query("kelasId")
	semester := c.Query("semester")
	tahunID := c.Query("tahunAjaranId")
	mapelID := c.Query("mapelId")
	if kelasID == "" || tahunID == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "kelasId, tahunAjaranId and semester (Ganjil or Genap) are required")
	}
	q := s.db.Preload("PesertaDidik").Where("kelas_id = ? AND tahun_ajaran_id = ? AND semester = ?", kelasID, tahunID, semester)
	if mapelID != "" {
		q = q.Where("mapel_id = ?", mapelID)
		if c.Locals("role") == "guru" {
			if e := s.canManageKelasMapel(c, kelasID, mapelID); e != nil {
				return e
			}
		}
	} else if c.Locals("role") == "guru" {
		// Restrict to mapels the guru is assigned to for this kelas.
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return c.JSON([]RekapNilaiAkhir{})
		}
		var pgs []PenugasanGuruMapel
		s.db.Where("tutor_id = ? AND kelas_id = ?", *u.TutorID, kelasID).Find(&pgs)
		ids := make([]string, 0, len(pgs))
		for _, pg := range pgs {
			ids = append(ids, pg.MapelID)
		}
		if len(ids) == 0 {
			return c.JSON([]RekapNilaiAkhir{})
		}
		q = q.Where("mapel_id IN ?", ids)
	}
	var rows []RekapNilaiAkhir
	if e := q.Order("peserta_didik_id, mapel_id").Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

// nilaiScopeItem is one (kelas, mapel, tahun) triple the caller may grade/read.
type nilaiScopeItem struct {
	KelasID       string `json:"kelasId"`
	KelasLabel    string `json:"kelasLabel"`
	MapelID       string `json:"mapelId"`
	MapelNama     string `json:"mapelNama"`
	TahunAjaranID string `json:"tahunAjaranId"`
	TahunNama     string `json:"tahunNama"`
}

// getNilaiScope returns the coupled (kelas, mapel, tahun ajaran) triples the
// current user may grade/read. Guru is restricted to PenugasanGuruMapel pairs
// (PRD §5 nilai auth uses penugasan, not wali_kelas); admin & kepala_sekolah get
// every kelas-mapel assignment. Used to drive the Modul Nilai filter bar so the
// picker never offers a (kelas, mapel) the user is not authorized for.
func (s *Server) getNilaiScope(c *fiber.Ctx) error {
	type pair struct{ kelasID, mapelID string }
	seen := map[pair]bool{}
	role := c.Locals("role")
	if role == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return c.JSON([]nilaiScopeItem{})
		}
		var pgs []PenugasanGuruMapel
		s.db.Where("tutor_id = ?", *u.TutorID).Find(&pgs)
		for _, pg := range pgs {
			seen[pair{pg.KelasID, pg.MapelID}] = true
		}
	} else {
		var kms []KelasMapel
		s.db.Find(&kms)
		for _, km := range kms {
			seen[pair{km.KelasID, km.MapelID}] = true
		}
	}
	if len(seen) == 0 {
		return c.JSON([]nilaiScopeItem{})
	}
	kelasIDs := make([]string, 0, len(seen))
	mapelIDs := make([]string, 0, len(seen))
	for p := range seen {
		kelasIDs = append(kelasIDs, p.kelasID)
		mapelIDs = append(mapelIDs, p.mapelID)
	}
	var kelasRows []Kelas
	s.db.Preload("TahunAjaran").Where("id IN ?", kelasIDs).Find(&kelasRows)
	kelasByID := map[string]Kelas{}
	for _, k := range kelasRows {
		kelasByID[k.ID] = k
	}
	var mapelRows []MataPelajaran
	s.db.Where("id IN ?", mapelIDs).Find(&mapelRows)
	mapelByID := map[string]MataPelajaran{}
	for _, m := range mapelRows {
		mapelByID[m.ID] = m
	}
	items := make([]nilaiScopeItem, 0, len(seen))
	for p := range seen {
		k := kelasByID[p.kelasID]
		m := mapelByID[p.mapelID]
		items = append(items, nilaiScopeItem{
			KelasID:       p.kelasID,
			KelasLabel:    "Kelas " + strconv.Itoa(k.Jenjang) + k.NamaRombel,
			MapelID:       p.mapelID,
			MapelNama:     m.NamaMapel,
			TahunAjaranID: k.TahunAjaranID,
			TahunNama:     k.TahunAjaran.NamaTahunAjaran,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].KelasLabel != items[j].KelasLabel {
			return items[i].KelasLabel < items[j].KelasLabel
		}
		return items[i].MapelNama < items[j].MapelNama
	})
	return c.JSON(items)
}

// mapelExportData holds per-mapel rekap context for the export functions.
type mapelExportData struct {
	Mapel MataPelajaran
	Rekap []RekapNilaiAkhir
	Temas []Tema
	Grids map[string]map[int]*float64 // pesertaDidikId -> urutanCp -> nilaiKeterampilan
	UMs   map[string]*float64         // pesertaDidikId -> nilaiUm
}

// exportNilai streams the rekap as xlsx or pdf, grouped per tema.
func (s *Server) exportNilai(c *fiber.Ctx) error {
	kelasID := c.Query("kelasId")
	semester := c.Query("semester")
	tahunID := c.Query("tahunAjaranId")
	mapelID := c.Query("mapelId")
	format := strings.ToLower(c.Query("format"))
	if format == "" {
		format = "xlsx"
	}
	if kelasID == "" || tahunID == "" || (semester != "Ganjil" && semester != "Genap") {
		return fiber.NewError(400, "kelasId, tahunAjaranId and semester (Ganjil or Genap) are required")
	}
	if format != "xlsx" && format != "pdf" {
		return fiber.NewError(400, "format must be xlsx or pdf")
	}
	// Resolve target mapels (filter to guru assignments when no specific mapelId).
	var mapelIDs []string
	if mapelID != "" {
		if c.Locals("role") == "guru" {
			if e := s.canManageKelasMapel(c, kelasID, mapelID); e != nil {
				return e
			}
		}
		mapelIDs = []string{mapelID}
	} else if c.Locals("role") == "guru" {
		var u User
		if s.db.First(&u, "id = ?", c.Locals("userID")).Error != nil || u.TutorID == nil {
			return fiber.NewError(403, "no tutor profile")
		}
		var pgs []PenugasanGuruMapel
		s.db.Where("tutor_id = ? AND kelas_id = ?", *u.TutorID, kelasID).Find(&pgs)
		for _, pg := range pgs {
			mapelIDs = append(mapelIDs, pg.MapelID)
		}
		if len(mapelIDs) == 0 {
			return fiber.NewError(403, "you are not assigned to any mapel in this kelas")
		}
	}
	// Load context for the filename + header.
	var class Kelas
	s.db.First(&class, "id = ?", kelasID)
	kelasLabel := "Kelas " + strconv.Itoa(class.Jenjang) + class.NamaRombel

	var datas []mapelExportData
	if len(mapelIDs) == 0 {
		// No specific mapel: pull all rekap rows for the kelas+semester+year and
		// group by the distinct mapels present.
		var allRekap []RekapNilaiAkhir
		s.db.Preload("PesertaDidik").Where("kelas_id = ? AND tahun_ajaran_id = ? AND semester = ?", kelasID, tahunID, semester).Find(&allRekap)
		mdIdx := map[string]int{}
		for _, r := range allRekap {
			if _, ok := mdIdx[r.MapelID]; ok {
				continue
			}
			var mp MataPelajaran
			s.db.First(&mp, "id = ?", r.MapelID)
			mdIdx[r.MapelID] = len(datas)
			datas = append(datas, mapelExportData{Mapel: mp})
		}
		for _, r := range allRekap {
			if idx, ok := mdIdx[r.MapelID]; ok {
				datas[idx].Rekap = append(datas[idx].Rekap, r)
			}
		}
	} else {
		for _, mid := range mapelIDs {
			var mp MataPelajaran
			s.db.First(&mp, "id = ?", mid)
			var rekaps []RekapNilaiAkhir
			s.db.Preload("PesertaDidik").Where("kelas_id = ? AND mapel_id = ? AND tahun_ajaran_id = ? AND semester = ?", kelasID, mid, tahunID, semester).Find(&rekaps)
			var temas []Tema
			s.db.Preload("Capaian").Where("kelas_id = ? AND mapel_id = ? AND tahun_ajaran_id = ? AND semester = ?", kelasID, mid, tahunID, semester).Order("urutan").Find(&temas)
			grids := map[string]map[int]*float64{}
			ums := map[string]*float64{}
			for _, t := range temas {
				var cps []NilaiCP
				s.db.Where("tema_id = ?", t.ID).Find(&cps)
				for _, cp := range cps {
					m := grids[cp.PesertaDidikID]
					if m == nil {
						m = map[int]*float64{}
					}
					m[cp.UrutanCP] = cp.NilaiKeterampilan
					grids[cp.PesertaDidikID] = m
				}
				var umRows []NilaiUM
				s.db.Where("tema_id = ?", t.ID).Find(&umRows)
				for _, u := range umRows {
					ums[u.PesertaDidikID] = u.NilaiUM
				}
			}
			datas = append(datas, mapelExportData{Mapel: mp, Rekap: rekaps, Temas: temas, Grids: grids, UMs: ums})
		}
	}

	mapelLabel := "semua-mapel"
	if len(mapelIDs) == 1 {
		mapelLabel = sanitizeFilename(datas[0].Mapel.NamaMapel)
	}
	fname := "rekap-nilai-" + sanitizeFilename(kelasLabel) + "-" + mapelLabel + "-" + semester + "."

	if format == "pdf" {
		return s.exportNilaiPDF(c, datas, kelasLabel, semester, fname+"pdf")
	}
	return s.exportNilaiXLSX(c, datas, kelasLabel, semester, fname+"xlsx")
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func (s *Server) exportNilaiXLSX(c *fiber.Ctx, datas []mapelExportData, kelasLabel, semester, filename string) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Rekap Nilai")
	rowIdx := 1
	writeRow := func(vals []interface{}) error {
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		return xlsx.SetSheetRow(sheet, cell, &vals)
	}
	_ = writeRow([]interface{}{"Rekap Nilai PKBM Tunas Ilmu"})
	rowIdx++
	_ = writeRow([]interface{}{kelasLabel + " - Semester " + semester})
	rowIdx++
	rowIdx++
	for _, d := range datas {
		_ = writeRow([]interface{}{"Mata Pelajaran: " + d.Mapel.NamaMapel})
		rowIdx++
		maxCP := 0
		for _, t := range d.Temas {
			if t.JumlahCP > maxCP {
				maxCP = t.JumlahCP
			}
		}
		header := []interface{}{"NIS", "Nama"}
		for i := 0; i < maxCP; i++ {
			header = append(header, "CP "+strconv.Itoa(i+1)+" (NK)")
		}
		header = append(header, "UM")
		for i := 0; i < maxCP; i++ {
			header = append(header, "Nilai Akhir CP "+strconv.Itoa(i+1))
		}
		header = append(header, "NP", "Predikat NP", "NK", "Predikat NK")
		_ = writeRow(header)
		rowIdx++
		for _, r := range d.Rekap {
			row := []interface{}{r.PesertaDidik.NIS, r.PesertaDidik.Nama}
			grid := d.Grids[r.PesertaDidikID]
			for i := 0; i < maxCP; i++ {
				if grid != nil {
					if v, ok := grid[i+1]; ok && v != nil {
						row = append(row, *v)
					} else {
						row = append(row, "")
					}
				} else {
					row = append(row, "")
				}
			}
			if um, ok := d.UMs[r.PesertaDidikID]; ok && um != nil {
				row = append(row, *um)
			} else {
				row = append(row, "")
			}
			for i := 0; i < maxCP; i++ {
				row = append(row, "")
			}
			if r.NPAkhir != nil {
				row = append(row, *r.NPAkhir)
			} else {
				row = append(row, "")
			}
			row = append(row, orDash(r.PredikatNP))
			if r.NKAkhir != nil {
				row = append(row, *r.NKAkhir)
			} else {
				row = append(row, "")
			}
			row = append(row, orDash(r.PredikatNK))
			_ = writeRow(row)
			rowIdx++
		}
		rowIdx++
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment(filename)
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) exportNilaiPDF(c *fiber.Ctx, datas []mapelExportData, kelasLabel, semester, filename string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(186, 8, "Rekap Nilai PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(186, 6, kelasLabel+" - Semester "+semester, "", 1, "C", false, 0, "")
	pdf.Ln(4)
	for _, d := range datas {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(186, 7, "Mata Pelajaran: "+d.Mapel.NamaMapel, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, r := range d.Rekap {
			line := r.PesertaDidik.Nama + "  | NP: " + fmtPtr(r.NPAkhir) + " (" + orDash(r.PredikatNP) + ")  | NK: " + fmtPtr(r.NKAkhir) + " (" + orDash(r.PredikatNK) + ")"
			pdf.CellFormat(186, 6, line, "", 1, "L", false, 0, "")
		}
		pdf.Ln(3)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment(filename)
	return pdf.Output(c.Response().BodyWriter())
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
func fmtPtr(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func _unused() string { return fmt.Sprint("") }

// ===== Modul Peminjaman & Pengembalian Buku (PRD_pinjam_buku.md) =====

func (s *Server) crudBuku(r fiber.Router) {
	r.Get("/buku", func(c *fiber.Ctx) error { return list[Buku](s.db.Order("judul"), c) })
	r.Post("/buku", func(c *fiber.Ctx) error { return create[Buku](s, c, "buku") })
	r.Put("/buku/:id", func(c *fiber.Ctx) error { return update[Buku](s, c, "buku") })
	r.Delete("/buku/:id", func(c *fiber.Ctx) error { return deleteRow[Buku](s, c, "buku") })
}

// listBukuKelas returns the book-per-class assignments (§4.19). Guru needs this
// to build the peminjaman checklist matrix for the class they walikan, so it is
// bare-api (auth only); the assignments themselves are not sensitive. Filters
// by kelasId / semester / tahunAjaranId are optional.
func (s *Server) listBukuKelas(c *fiber.Ctx) error {
	q := s.db.Preload("Kelas.Pokjar").Preload("Kelas.TahunAjaran").Preload("Buku").Order("created_at desc")
	if k := strings.TrimSpace(c.Query("kelasId")); k != "" {
		q = q.Where("kelas_id = ?", k)
	}
	if sem := strings.TrimSpace(c.Query("semester")); sem != "" {
		q = q.Where("semester = ?", sem)
	}
	if ta := strings.TrimSpace(c.Query("tahunAjaranId")); ta != "" {
		q = q.Where("kelas_id IN (?)", s.db.Model(&Kelas{}).Select("id").Where("tahun_ajaran_id = ?", ta))
	}
	var rows []BukuKelas
	if e := q.Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

// createBukuKelas records a book assignment for a class. Semester is filled
// automatically from the active TahunAjaran (§4.19), never from the client.
func (s *Server) createBukuKelas(c *fiber.Ctx) error {
	var in struct {
		KelasID string `json:"kelasId"`
		BukuID  string `json:"bukuId"`
	}
	if e := c.BodyParser(&in); e != nil || in.KelasID == "" || in.BukuID == "" {
		return fiber.NewError(400, "kelasId and bukuId are required")
	}
	sem := s.semester(time.Now())
	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.First(&Kelas{}, "id = ?", in.KelasID).Error; e != nil {
			return fiber.NewError(400, "kelasId does not exist")
		}
		if e := tx.First(&Buku{}, "id = ?", in.BukuID).Error; e != nil {
			return fiber.NewError(400, "bukuId does not exist")
		}
		var dup BukuKelas
		if e := tx.Where("kelas_id = ? AND buku_id = ? AND semester = ?", in.KelasID, in.BukuID, sem).First(&dup).Error; e == nil {
			return fiber.NewError(400, "buku sudah ditetapkan untuk kelas & semester ini")
		}
		row := BukuKelas{KelasID: in.KelasID, BukuID: in.BukuID, Semester: sem}
		if e := tx.Create(&row).Error; e != nil {
			return fiber.NewError(400, e.Error())
		}
		tx.Preload("Kelas.Pokjar").Preload("Kelas.TahunAjaran").Preload("Buku").First(&row, "id = ?", row.ID)
		uid := c.Locals("userID").(string)
		s.auditTx(tx, &uid, "create", "buku_kelas", row.Semester)
		return c.Status(201).JSON(row)
	})
}

func (s *Server) deleteBukuKelas(c *fiber.Ctx) error {
	if e := s.db.Delete(&BukuKelas{}, "id = ?", id(c)).Error; e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	s.audit(&uid, "delete", "buku_kelas", id(c))
	return c.SendStatus(204)
}

type peminjamanItem struct {
	PesertaDidikID string `json:"pesertaDidikId"`
	BukuID         string `json:"bukuId"`
	TanggalPinjam  string `json:"tanggalPinjam"` // date (2006-01-02) or RFC3339; "" = now
}

// createPeminjaman records one or more book loans for a class (§4.20). A single
// signature per recording session is stored on every row. Guard IDOR (§6.7):
// the guru must walikan the kelas. Each (siswa,buku) must be a valid §4.19
// assignment for the class/semester; semester is auto from tanggal_pinjam.
func (s *Server) createPeminjaman(c *fiber.Ctx) error {
	var in struct {
		KelasID     string           `json:"kelasId"`
		Items       []peminjamanItem `json:"items"`
		TandaTangan string           `json:"tandaTangan"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if in.KelasID == "" || len(in.Items) == 0 {
		return fiber.NewError(400, "kelasId and items are required")
	}
	if !validSignature(in.TandaTangan) {
		return fiber.NewError(400, "tanda tangan PNG yang valid wajib diisi")
	}
	if e := s.canManageKelas(c, in.KelasID); e != nil {
		return e
	}
	uid := c.Locals("userID").(string)
	// Precompute each item's date + semester OUTSIDE the transaction. s.semester
	// reads via s.db; calling it inside the tx would hold SQLite's single
	// connection (SetMaxOpenConns(1)) and deadlock.
	type ready struct {
		it  peminjamanItem
		tgl time.Time
		sem string
	}
	readyItems := make([]ready, 0, len(in.Items))
	for _, it := range in.Items {
		if it.PesertaDidikID == "" || it.BukuID == "" {
			return fiber.NewError(400, "pesertaDidikId and bukuId are required on every item")
		}
		tgl := parseDate(it.TanggalPinjam)
		readyItems = append(readyItems, ready{it: it, tgl: tgl, sem: s.semester(tgl)})
	}
	created := []Peminjaman{}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		for _, r := range readyItems {
			var pd PesertaDidik
			if e := tx.First(&pd, "id = ?", r.it.PesertaDidikID).Error; e != nil {
				return fiber.NewError(400, "pesertaDidik tidak ditemukan")
			}
			if e := tx.First(&Buku{}, "id = ?", r.it.BukuID).Error; e != nil {
				return fiber.NewError(400, "buku tidak ditemukan")
			}
			var bk BukuKelas
			if e := tx.Where("kelas_id = ? AND buku_id = ? AND semester = ?", in.KelasID, r.it.BukuID, r.sem).First(&bk).Error; e != nil {
				return fiber.NewError(400, "buku belum ditetapkan untuk kelas/semester ini")
			}
			row := Peminjaman{
				PesertaDidikID: r.it.PesertaDidikID, BukuID: r.it.BukuID, KelasID: in.KelasID,
				Semester: r.sem, TanggalPinjam: r.tgl, Status: "Dipinjam",
				DicatatOlehUserID: uid, TandaTangan: in.TandaTangan,
			}
			if e := tx.Create(&row).Error; e != nil {
				return fiber.NewError(400, e.Error())
			}
			tx.Preload("PesertaDidik").Preload("Buku").Preload("Kelas").First(&row, "id = ?", row.ID)
			created = append(created, row)
		}
		s.auditTx(tx, &uid, "create", "peminjaman", strconv.Itoa(len(created))+" item")
		return nil
	}); e != nil {
		return e
	}
	return c.Status(201).JSON(created)
}

// listPeminjamanAktif returns loans still outstanding (status "Dipinjam") for a
// class, used by the pengembalian checklist. Guru is scoped via canManageKelas.
func (s *Server) listPeminjamanAktif(c *fiber.Ctx) error {
	kelasID := strings.TrimSpace(c.Query("kelasId"))
	if kelasID == "" {
		return fiber.NewError(400, "kelasId is required")
	}
	if e := s.canManageKelas(c, kelasID); e != nil {
		return e
	}
	var rows []Peminjaman
	if e := s.db.Preload("PesertaDidik").Preload("Buku").Where("kelas_id = ? AND status = ?", kelasID, "Dipinjam").Order("tanggal_pinjam desc, created_at desc").Find(&rows).Error; e != nil {
		return e
	}
	return c.JSON(rows)
}

type kembaliItem struct {
	PeminjamanID   string `json:"peminjamanId"`
	TanggalKembali string `json:"tanggalKembali"` // date (2006-01-02) or RFC3339; "" = now
	KondisiBuku    string `json:"kondisiBuku"`
	Catatan        string `json:"catatan"`
}

// createPengembalian records returns for one or more loans (§4.21). Anti-double:
// Pengembalian.peminjamanId is unique and the loan must still be "Dipinjam".
// catatan is required when kondisi is "Rusak Berat" or "Hilang" (§5.8).
func (s *Server) createPengembalian(c *fiber.Ctx) error {
	var in struct {
		Items       []kembaliItem `json:"items"`
		TandaTangan string        `json:"tandaTangan"`
	}
	if e := c.BodyParser(&in); e != nil {
		return fiber.NewError(400, "invalid request body")
	}
	if len(in.Items) == 0 {
		return fiber.NewError(400, "items are required")
	}
	if !validSignature(in.TandaTangan) {
		return fiber.NewError(400, "tanda tangan PNG yang valid wajib diisi")
	}
	for _, it := range in.Items {
		if it.KondisiBuku != "Baik" && it.KondisiBuku != "Rusak Ringan" && it.KondisiBuku != "Rusak Berat" && it.KondisiBuku != "Hilang" {
			return fiber.NewError(400, "kondisiBuku tidak valid")
		}
		if (it.KondisiBuku == "Rusak Berat" || it.KondisiBuku == "Hilang") && strings.TrimSpace(it.Catatan) == "" {
			return fiber.NewError(400, "catatan wajib diisi untuk kondisi Rusak Berat / Hilang")
		}
	}
	uid := c.Locals("userID").(string)
	// Pre-load every loan + authorize + status check OUTSIDE the transaction.
	// SQLite is capped at 1 connection; running canManageKelas (which uses s.db)
	// inside the tx would hold that single connection and deadlock.
	type pending struct {
		it kembaliItem
		p  Peminjaman
	}
	pendings := make([]pending, 0, len(in.Items))
	seen := map[string]bool{}
	for _, it := range in.Items {
		if seen[it.PeminjamanID] {
			return fiber.NewError(400, "peminjamanId tidak boleh duplikat dalam satu pengembalian")
		}
		seen[it.PeminjamanID] = true
		var p Peminjaman
		if e := s.db.Preload("Kelas").First(&p, "id = ?", it.PeminjamanID).Error; e != nil {
			return fiber.NewError(400, "peminjaman tidak ditemukan")
		}
		if e := s.canManageKelas(c, p.KelasID); e != nil {
			return e
		}
		if p.Status != "Dipinjam" {
			return fiber.NewError(400, "buku sudah dikembalikan")
		}
		pendings = append(pendings, pending{it: it, p: p})
	}
	created := []Pengembalian{}
	if e := s.db.Transaction(func(tx *gorm.DB) error {
		for _, pn := range pendings {
			row := Pengembalian{
				PeminjamanID: pn.it.PeminjamanID, TanggalKembali: parseDate(pn.it.TanggalKembali),
				KondisiBuku: pn.it.KondisiBuku, Catatan: pn.it.Catatan,
				DicatatOlehUserID: uid, TandaTangan: in.TandaTangan,
			}
			if e := tx.Create(&row).Error; e != nil {
				return fiber.NewError(400, "pengembalian untuk peminjaman ini sudah ada")
			}
			if e := tx.Model(&Peminjaman{}).Where("id = ?", pn.it.PeminjamanID).Update("status", "Dikembalikan").Error; e != nil {
				return e
			}
			created = append(created, row)
		}
		s.auditTx(tx, &uid, "create", "pengembalian", strconv.Itoa(len(created))+" item")
		return nil
	}); e != nil {
		return e
	}
	return c.Status(201).JSON(created)
}

// rekapBuku returns loans (with their return if any) for the rekap page.
// Filters: kelasId, semester, tahunAjaranId (via Peminjaman.KelasID), status.
func (s *Server) rekapBuku(c *fiber.Ctx) error {
	q := s.db.Preload("PesertaDidik").Preload("Buku").Preload("Kelas.Pokjar").Preload("Kelas.TahunAjaran")
	if k := strings.TrimSpace(c.Query("kelasId")); k != "" {
		q = q.Where("kelas_id = ?", k)
	}
	if sem := strings.TrimSpace(c.Query("semester")); sem != "" {
		q = q.Where("semester = ?", sem)
	}
	if ta := strings.TrimSpace(c.Query("tahunAjaranId")); ta != "" {
		q = q.Where("kelas_id IN (?)", s.db.Model(&Kelas{}).Select("id").Where("tahun_ajaran_id = ?", ta))
	}
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		q = q.Where("status = ?", st)
	}
	var pinjam []Peminjaman
	if e := q.Order("tanggal_pinjam desc, created_at desc").Find(&pinjam).Error; e != nil {
		return e
	}
	ids := make([]string, 0, len(pinjam))
	for _, p := range pinjam {
		ids = append(ids, p.ID)
	}
	kembali := map[string]Pengembalian{}
	if len(ids) > 0 {
		var rows []Pengembalian
		s.db.Where("peminjaman_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			kembali[r.PeminjamanID] = r
		}
	}
	type rekapRow struct {
		Peminjaman   Peminjaman    `json:"peminjaman"`
		Pengembalian *Pengembalian `json:"pengembalian"`
	}
	out := make([]rekapRow, 0, len(pinjam))
	for _, p := range pinjam {
		k, ok := kembali[p.ID]
		out = append(out, rekapRow{Peminjaman: p, Pengembalian: ifor(ok, k)})
	}
	return c.JSON(out)
}

func ifor(ok bool, v Pengembalian) *Pengembalian {
	if !ok {
		return nil
	}
	return &v
}

// parseDate accepts a date-only ("2006-01-02") or RFC3339 string; empty/invalid
// falls back to now. Frontend date inputs send date-only.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now()
	}
	if t, e := time.Parse("2006-01-02", s); e == nil {
		return t
	}
	if t, e := time.Parse(time.RFC3339, s); e == nil {
		return t
	}
	return time.Now()
}

// exportBuku serves the rekap as XLSX or PDF (§5.8). readAll → admin+kepala.
func (s *Server) exportBuku(c *fiber.Ctx) error {
	q := s.db.Preload("PesertaDidik").Preload("Buku").Preload("Kelas.Pokjar").Preload("Kelas.TahunAjaran")
	if k := strings.TrimSpace(c.Query("kelasId")); k != "" {
		q = q.Where("kelas_id = ?", k)
	}
	if sem := strings.TrimSpace(c.Query("semester")); sem != "" {
		q = q.Where("semester = ?", sem)
	}
	if ta := strings.TrimSpace(c.Query("tahunAjaranId")); ta != "" {
		q = q.Where("kelas_id IN (?)", s.db.Model(&Kelas{}).Select("id").Where("tahun_ajaran_id = ?", ta))
	}
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		q = q.Where("status = ?", st)
	}
	var pinjam []Peminjaman
	if e := q.Order("tanggal_pinjam desc, created_at desc").Find(&pinjam).Error; e != nil {
		return e
	}
	ids := make([]string, 0, len(pinjam))
	for _, p := range pinjam {
		ids = append(ids, p.ID)
	}
	kembali := map[string]Pengembalian{}
	if len(ids) > 0 {
		var rows []Pengembalian
		s.db.Where("peminjaman_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			kembali[r.PeminjamanID] = r
		}
	}
	format := strings.TrimSpace(c.Query("format"))
	if format == "pdf" {
		return s.exportBukuPDF(c, pinjam, kembali)
	}
	return s.exportBukuXLSX(c, pinjam, kembali)
}

func (s *Server) exportBukuXLSX(c *fiber.Ctx, pinjam []Peminjaman, kembali map[string]Pengembalian) error {
	xlsx := excelize.NewFile()
	sheet := xlsx.GetSheetName(0)
	_ = xlsx.SetSheetName(sheet, "Rekap Peminjaman")
	rowIdx := 1
	writeRow := func(vals []interface{}) error {
		cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
		rowIdx++
		return xlsx.SetSheetRow(sheet, cell, &vals)
	}
	_ = writeRow([]interface{}{"Rekap Peminjaman Buku PKBM Tunas Ilmu"})
	_ = writeRow([]interface{}{"Tanggal unduh: " + time.Now().Format("2006-01-02 15:04")})
	rowIdx++
	_ = writeRow([]interface{}{"No", "Siswa", "Kelas", "Buku", "Semester", "Tgl Pinjam", "Status", "Tgl Kembali", "Kondisi", "Catatan"})
	for i, p := range pinjam {
		k, ok := kembali[p.ID]
		row := []interface{}{
			i + 1, p.PesertaDidik.Nama, kelasLabel(p.Kelas), p.Buku.Judul, p.Semester,
			p.TanggalPinjam.Format("2006-01-02"), p.Status,
		}
		if ok {
			row = append(row, k.TanggalKembali.Format("2006-01-02"), k.KondisiBuku, k.Catatan)
		} else {
			row = append(row, "", "", "")
		}
		_ = writeRow(row)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Attachment("rekap-peminjaman-buku.xlsx")
	return xlsx.Write(c.Response().BodyWriter())
}

func (s *Server) exportBukuPDF(c *fiber.Ctx, pinjam []Peminjaman, kembali map[string]Pengembalian) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(186, 8, "Rekap Peminjaman Buku PKBM Tunas Ilmu", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(186, 6, "Tanggal unduh: "+time.Now().Format("2006-01-02 15:04"), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(8, 7, "No", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 7, "Siswa", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, "Kelas", "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 7, "Buku", "1", 0, "L", true, 0, "")
	pdf.CellFormat(18, 7, "Sem.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 7, "Tgl Pinjam", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "Status", "1", 1, "C", true, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	for i, p := range pinjam {
		pdf.CellFormat(8, 6, strconv.Itoa(i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 6, p.PesertaDidik.Nama, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 6, kelasLabel(p.Kelas), "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 6, p.Buku.Judul, "1", 0, "L", false, 0, "")
		pdf.CellFormat(18, 6, p.Semester, "1", 0, "C", false, 0, "")
		pdf.CellFormat(20, 6, p.TanggalPinjam.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6, p.Status, "1", 1, "C", false, 0, "")
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Attachment("rekap-peminjaman-buku.pdf")
	return pdf.Output(c.Response().BodyWriter())
}

// reminderBuku is the n8n reminder cron job (§5.8). Runs daily; when today is
// H-14/H-7/H-1 before the active TahunAjaran's genap-start (or end date), it
// POSTs the list of still-borrowed books to N8N_WEBHOOK_URL if set. No-op
// otherwise. Recovered so a transient error never kills the cron loop.
func (s *Server) reminderBuku(loc *time.Location) {
	defer func() { _ = recover() }()
	webhook := os.Getenv("N8N_WEBHOOK_URL")
	if webhook == "" {
		return
	}
	var ta TahunAjaran
	if s.db.Where("is_aktif = ?", true).First(&ta).Error != nil {
		return
	}
	anchor := ta.TanggalSelesai
	if ta.TanggalMulaiSemesterGenap != nil && !ta.TanggalMulaiSemesterGenap.IsZero() {
		anchor = *ta.TanggalMulaiSemesterGenap
	}
	now := time.Now().In(loc)
	days := int(anchor.Sub(now).Hours() / 24)
	if days != 14 && days != 7 && days != 1 {
		return
	}
	var rows []Peminjaman
	s.db.Preload("PesertaDidik").Preload("Buku").Preload("Kelas").Where("status = ?", "Dipinjam").Find(&rows)
	if len(rows) == 0 {
		return
	}
	type item struct {
		Siswa string `json:"siswa"`
		Kelas string `json:"kelas"`
		Buku  string `json:"buku"`
		Tgl   string `json:"tanggalPinjam"`
	}
	items := make([]item, 0, len(rows))
	for _, p := range rows {
		items = append(items, item{Siswa: p.PesertaDidik.Nama, Kelas: kelasLabel(p.Kelas), Buku: p.Buku.Judul, Tgl: p.TanggalPinjam.Format("2006-01-02")})
	}
	body, _ := json.Marshal(items)
	_, _ = http.Post(webhook, "application/json", strings.NewReader(string(body)))
}
