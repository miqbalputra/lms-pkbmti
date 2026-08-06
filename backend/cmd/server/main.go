package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Base struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

type User struct {
	Base
	Username     string  `gorm:"uniqueIndex;not null" json:"username"`
	Email        string  `gorm:"index" json:"email"`
	Nama         string  `gorm:"-" json:"nama,omitempty"`
	PasswordHash string  `json:"-"`
	Role         string  `gorm:"not null" json:"role"`
	TutorID      *string `json:"tutorId"`
	// OrangTuaID diisi untuk role="orang_tua" (Portal Orang Tua) — akun login
	// penuh yang melihat data anak-anak terhubung. Nullable agar akun staf
	// lama tetap valid (backward compatible).
	OrangTuaID   *string `gorm:"index" json:"orangTuaId"`
	IsActive     bool    `gorm:"default:true" json:"isActive"`
	FailedLogins int
	LockedUntil  *time.Time
}
type RefreshToken struct {
	Base
	UserID    string `gorm:"index;not null"`
	TokenHash string `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time
	RevokedAt *time.Time
}
type AuditLog struct {
	Base
	UserID   *string `gorm:"index" json:"userId"`
	Action   string  `gorm:"index:idx_audit_action_resource" json:"action"`
	Resource string  `gorm:"index:idx_audit_action_resource" json:"resource"`
	Detail   string  `json:"detail"`
}
type Tutor struct {
	Base
	Nama               string     `json:"nama"`
	JenisKelamin       string     `json:"jenisKelamin"`
	NIK                string     `json:"nik"`
	TempatLahir        string     `json:"tempatLahir"`
	TanggalLahir       *time.Time `json:"tanggalLahir"`
	TanggalBertugas    *time.Time `json:"tanggalBertugas"`
	NoHP               string     `json:"noHp"`
	Alamat             string     `json:"alamat"`
	UserID             *string    `json:"userId"`
	SKPengangkatanPath *string    `json:"-"`
	SKPengangkatanNama string     `json:"-"`
	IsRPPMaker         bool       `gorm:"default:false" json:"isRppMaker"` // Modul R — ditugaskan admin utk menyusun RPP per jenjang
}

// DokumenSistem menyimpan dokumen yang dipakai bersama oleh banyak akun,
// misalnya SK Penugasan seluruh tutor. File tetap disajikan melalui endpoint
// terautentikasi, bukan static public /uploads.
type DokumenSistem struct {
	Base
	Kode     string `gorm:"uniqueIndex;not null" json:"kode"`
	Nama     string `json:"nama"`
	FilePath string `json:"-"`
}

// SuratSiswa adalah satu publikasi surat untuk satu atau banyak peserta didik.
// File individual disimpan di SuratSiswaFile agar satu judul dapat tampil
// berbeda untuk setiap anak di Portal Orang Tua.
type SuratSiswa struct {
	Base
	Judul            string  `gorm:"not null" json:"judul"`
	Cakupan          string  `gorm:"index;not null" json:"cakupan"` // semua_kelas, kelas, anak
	KelasID          *string `gorm:"index" json:"kelasId,omitempty"`
	UploadedByUserID string  `gorm:"index;not null" json:"uploadedByUserId"`
}

type SuratSiswaFile struct {
	Base
	SuratSiswaID   string `gorm:"uniqueIndex:surat_siswa_file_target;not null" json:"suratSiswaId"`
	PesertaDidikID string `gorm:"uniqueIndex:surat_siswa_file_target;index;not null" json:"pesertaDidikId"`
	NISN           string `gorm:"index;not null" json:"nisn"`
	FilePath       string `gorm:"not null" json:"-"`
	FileName       string `gorm:"not null" json:"fileName"`
}
type OrangTua struct {
	Base
	NamaBapak string `json:"namaBapak"`
	NamaIbu   string `json:"namaIbu"`
	NIKAyah   string `json:"nikAyah"` // NIK bapak/ayah, opsional
	NIKIbu    string `json:"nikIbu"`  // NIK ibu, opsional
}
type Pokjar struct {
	Base
	NamaPokjar string `gorm:"uniqueIndex" json:"namaPokjar"`
	Tipe       string `json:"tipe"`
	Alamat     string `json:"alamat"`
}
type TahunAjaran struct {
	Base
	NamaTahunAjaran           string     `gorm:"uniqueIndex" json:"namaTahunAjaran"`
	TanggalMulai              time.Time  `json:"tanggalMulai"`
	TanggalSelesai            time.Time  `json:"tanggalSelesai"`
	TanggalMulaiSemesterGenap *time.Time `json:"tanggalMulaiSemesterGenap"` // legacy — dipertahankan utk backward-compat; semester kini dikelola via model Semester
	IsAktif                   bool       `gorm:"index" json:"isAktif"`
}

// Semester merepresentasikan satu semester dalam sebuah tahun ajaran (Ganjil / Genap).
// Setiap TahunAjaran otomatis memiliki 2 record Semester. Admin dapat mengarsipkan
// semester yang sudah lewat dan membukanya kembali.
type Semester struct {
	Base
	TahunAjaranID  string      `gorm:"index" json:"tahunAjaranId"`
	NamaSemester   string      `gorm:"index" json:"namaSemester"` // "Ganjil" / "Genap"
	TanggalMulai   time.Time   `json:"tanggalMulai"`
	TanggalSelesai time.Time   `json:"tanggalSelesai"`
	IsArchived     bool        `json:"isArchived"`
	TahunAjaran    TahunAjaran `json:"tahunAjaran"`
}

type Kelas struct {
	Base
	Jenjang       int         `gorm:"uniqueIndex:kelas_identitas" json:"jenjang"`
	NamaRombel    string      `gorm:"uniqueIndex:kelas_identitas" json:"namaRombel"`
	PokjarID      string      `gorm:"uniqueIndex:kelas_identitas" json:"pokjarId"`
	TahunAjaranID string      `gorm:"uniqueIndex:kelas_identitas" json:"tahunAjaranId"`
	WaliKelasID   *string     `gorm:"index" json:"waliKelasId"`
	ProgramID     *string     `gorm:"index" json:"programId"` // Modul O — opsional
	FaseID        *string     `gorm:"index" json:"faseId"`    // Modul N — opsional
	Pokjar        Pokjar      `json:"pokjar"`
	TahunAjaran   TahunAjaran `json:"tahunAjaran"`
	WaliKelas     *Tutor      `json:"waliKelas"`
}
type RiwayatWaliKelas struct {
	Base
	KelasID        string     `json:"kelasId"`
	TutorID        string     `json:"tutorId"`
	TanggalMulai   time.Time  `json:"tanggalMulai"`
	TanggalSelesai *time.Time `json:"tanggalSelesai"`
	Tutor          Tutor      `json:"tutor"`
}
type MataPelajaran struct {
	Base
	NamaMapel string `gorm:"uniqueIndex" json:"namaMapel"`
	KodeMapel string `json:"kodeMapel"`
	IsActive  bool   `gorm:"default:true" json:"isActive"`
}
type KelasMapel struct {
	Base
	KelasID string        `gorm:"uniqueIndex:kelas_mapel" json:"kelasId"`
	MapelID string        `gorm:"uniqueIndex:kelas_mapel" json:"mapelId"`
	Mapel   MataPelajaran `json:"mapel"`
}
type PenugasanGuruMapel struct {
	Base
	TutorID string         `gorm:"uniqueIndex:penugasan" json:"tutorId"`
	KelasID string         `gorm:"uniqueIndex:penugasan" json:"kelasId"`
	MapelID string         `gorm:"uniqueIndex:penugasan" json:"mapelId"`
	Tutor   *Tutor         `gorm:"foreignKey:TutorID" json:"tutor,omitempty"`
	Kelas   *Kelas         `gorm:"foreignKey:KelasID" json:"kelas,omitempty"`
	Mapel   *MataPelajaran `gorm:"foreignKey:MapelID" json:"mapel,omitempty"`
}

// temporaryNISN is used while a student's official NISN has not been issued.
// It is intentionally shared by those students; parent login remains disabled
// for this placeholder until the real NISN is available.
const temporaryNISN = "0000000000"

type PesertaDidik struct {
	Base
	Nama         string     `json:"nama"`
	JenisKelamin string     `json:"jenisKelamin"`
	NIS          string     `gorm:"uniqueIndex" json:"nis"`
	NISN         string     `gorm:"index" json:"nisn"`
	NIK          string     `json:"nik"` // NIK anak, wajib; keunikan dicek di handler
	TanggalLahir *time.Time `json:"tanggalLahir"`
	KelasID      string     `gorm:"index" json:"kelasId"`
	PokjarID     string     `json:"pokjarId"`
	OrangTuaID   string     `gorm:"index" json:"orangTuaId"`
	ProgramID    *string    `gorm:"index" json:"programId"` // Modul O — opsional
	FotoPath     *string    `json:"fotoPath"`               // Modul P — foto kartu pelajar
	Status       string     `gorm:"default:aktif" json:"status"`
	Kelas        Kelas      `json:"kelas"`
	OrangTua     OrangTua   `json:"orangTua"`
}
type RiwayatKelasPesertaDidik struct {
	Base
	PesertaDidikID string `json:"pesertaDidikId"`
	KelasID        string `json:"kelasId"`
	TahunAjaranID  string `json:"tahunAjaranId"`
	Status         string `json:"status"`
	Catatan        string `json:"catatan"`
	Kelas          Kelas  `json:"kelas"`
}
type PengaturanJadwal struct {
	Base
	HariDefault string `json:"hariDefault"`
	JamGenerate string `json:"jamGenerate"`
	ZonaWaktu   string `json:"zonaWaktu"`
}
type Presensi struct {
	Base
	KelasID         string           `gorm:"index" json:"kelasId"`
	Tanggal         time.Time        `json:"tanggal"`
	TanggalRencana  *time.Time       `json:"tanggalRencana"`
	Semester        string           `json:"semester"`
	StatusPertemuan string           `json:"statusPertemuan"`
	DibuatOtomatis  bool             `json:"dibuatOtomatis"`
	Keterangan      string           `json:"keterangan"`
	TutorID         *string          `json:"tutorId"`
	TandaTangan     string           `json:"tandaTangan"`
	BuktiFoto       string           `json:"buktiFoto"`
	Kelas           Kelas            `json:"kelas"`
	Details         []PresensiDetail `json:"details"`
}
type PresensiDetail struct {
	Base
	PresensiID      string       `gorm:"uniqueIndex:presensi_siswa" json:"presensiId"`
	PesertaDidikID  string       `gorm:"uniqueIndex:presensi_siswa" json:"pesertaDidikId"`
	StatusKehadiran string       `json:"statusKehadiran"`
	Catatan         string       `json:"catatan"`
	PesertaDidik    PesertaDidik `json:"pesertaDidik"`
}

// Modul Nilai — grading subsystem. See prd_nilai.md.
type Tema struct {
	Base
	KelasID           string                `gorm:"column:kelas_id;uniqueIndex:tema_identitas" json:"kelasId"`
	MapelID           string                `gorm:"column:mapel_id;uniqueIndex:tema_identitas" json:"mapelId"`
	TahunAjaranID     string                `gorm:"column:tahun_ajaran_id;uniqueIndex:tema_identitas" json:"tahunAjaranId"`
	Semester          string                `gorm:"column:semester;uniqueIndex:tema_identitas" json:"semester"`
	NamaTema          string                `gorm:"column:nama_tema" json:"namaTema"`
	Urutan            int                   `gorm:"column:urutan" json:"urutan"`
	JumlahCP          int                   `gorm:"column:jumlah_cp" json:"jumlahCp"`
	BobotKeterampilan float64               `gorm:"column:bobot_keterampilan;type:decimal(5,2)" json:"bobotKeterampilan"`
	BobotPengetahuan  float64               `gorm:"column:bobot_pengetahuan;type:decimal(5,2)" json:"bobotPengetahuan"`
	Kelas             Kelas                 `gorm:"foreignKey:KelasID" json:"kelas"`
	Mapel             MataPelajaran         `gorm:"foreignKey:MapelID" json:"mapel"`
	TahunAjaran       TahunAjaran           `gorm:"foreignKey:TahunAjaranID" json:"tahunAjaran"`
	Capaian           []CapaianPembelajaran `gorm:"foreignKey:TemaID" json:"capaian"`
}
type CapaianPembelajaran struct {
	Base
	TemaID       string `gorm:"column:tema_id;uniqueIndex:cp_unique" json:"temaId"`
	UrutanCP     int    `gorm:"column:urutan_cp;uniqueIndex:cp_unique" json:"urutanCp"`
	LabelDefault string `gorm:"column:label_default" json:"labelDefault"`
	Tema         Tema   `gorm:"foreignKey:TemaID" json:"tema"`
}
type NilaiCP struct {
	Base
	TemaID            string       `gorm:"column:tema_id;uniqueIndex:nilai_cp_unique" json:"temaId"`
	UrutanCP          int          `gorm:"column:urutan_cp;uniqueIndex:nilai_cp_unique" json:"urutanCp"`
	PesertaDidikID    string       `gorm:"column:peserta_didik_id;uniqueIndex:nilai_cp_unique" json:"pesertaDidikId"`
	DeskripsiCP       string       `gorm:"column:deskripsi_cp" json:"deskripsiCp"`
	Manual            bool         `gorm:"column:manual" json:"manual"`
	NilaiKeterampilan *float64     `gorm:"column:nilai_keterampilan;type:decimal(5,2)" json:"nilaiKeterampilan"`
	Tema              Tema         `gorm:"foreignKey:TemaID" json:"tema"`
	PesertaDidik      PesertaDidik `gorm:"foreignKey:PesertaDidikID" json:"pesertaDidik"`
}
type NilaiUM struct {
	Base
	TemaID         string       `gorm:"column:tema_id;uniqueIndex:nilai_um_unique" json:"temaId"`
	PesertaDidikID string       `gorm:"column:peserta_didik_id;uniqueIndex:nilai_um_unique" json:"pesertaDidikId"`
	NilaiUM        *float64     `gorm:"column:nilai_um;type:decimal(5,2)" json:"nilaiUm"`
	Tema           Tema         `gorm:"foreignKey:TemaID" json:"tema"`
	PesertaDidik   PesertaDidik `gorm:"foreignKey:PesertaDidikID" json:"pesertaDidik"`
}
type PengaturanBobotNilai struct {
	Base
	MapelID           string        `gorm:"column:mapel_id;uniqueIndex" json:"mapelId"`
	BobotKeterampilan float64       `gorm:"column:bobot_keterampilan;type:decimal(5,2)" json:"bobotKeterampilan"`
	BobotPengetahuan  float64       `gorm:"column:bobot_pengetahuan;type:decimal(5,2)" json:"bobotPengetahuan"`
	Mapel             MataPelajaran `gorm:"foreignKey:MapelID" json:"mapel"`
}
type AmbangPredikat struct {
	Base
	MapelID      string        `gorm:"column:mapel_id;uniqueIndex:ambang_unique" json:"mapelId"`
	Predikat     string        `gorm:"column:predikat;uniqueIndex:ambang_unique" json:"predikat"`
	NilaiMinimum float64       `gorm:"column:nilai_minimum;type:decimal(5,2)" json:"nilaiMinimum"`
	Mapel        MataPelajaran `gorm:"foreignKey:MapelID" json:"mapel"`
}
type RekapNilaiAkhir struct {
	Base
	PesertaDidikID string       `gorm:"column:peserta_didik_id;uniqueIndex:rekap_unique" json:"pesertaDidikId"`
	KelasID        string       `gorm:"column:kelas_id;index" json:"kelasId"`
	MapelID        string       `gorm:"column:mapel_id;uniqueIndex:rekap_unique" json:"mapelId"`
	TahunAjaranID  string       `gorm:"column:tahun_ajaran_id;uniqueIndex:rekap_unique" json:"tahunAjaranId"`
	Semester       string       `gorm:"column:semester;uniqueIndex:rekap_unique" json:"semester"`
	NPAkhir        *float64     `gorm:"column:np_akhir;type:decimal(5,2)" json:"npAkhir"`
	PredikatNP     string       `gorm:"column:predikat_np" json:"predikatNP"`
	NKAkhir        *float64     `gorm:"column:nk_akhir;type:decimal(5,2)" json:"nkAkhir"`
	PredikatNK     string       `gorm:"column:predikat_nk" json:"predikatNK"`
	NAAkhir        *float64     `gorm:"column:na_akhir;type:decimal(5,2)" json:"naAkhir"` // Modul S — NA gabungan berbobot (bila ada BobotSumberNilai)
	PredikatNA     string       `gorm:"column:predikat_na" json:"predikatNA"`             // Modul S — predikat untuk NA gabungan
	PesertaDidik   PesertaDidik `gorm:"foreignKey:PesertaDidikID" json:"pesertaDidik"`
}

// Modul Peminjaman & Pengembalian Buku — see PRD_pinjam_buku.md (§4.18–§4.21).
type Buku struct {
	Base
	Judul    string `gorm:"not null" json:"judul"`
	KodeBuku string `json:"kodeBuku"`
	Penerbit string `json:"penerbit"`
}
type BukuKelas struct {
	Base
	KelasID  string `gorm:"uniqueIndex:bukukelas_uniq" json:"kelasId"`
	BukuID   string `gorm:"uniqueIndex:bukukelas_uniq" json:"bukuId"`
	Semester string `gorm:"uniqueIndex:bukukelas_uniq" json:"semester"` // "Ganjil"|"Genap"
	Kelas    Kelas  `json:"kelas"`
	Buku     Buku   `json:"buku"`
}
type Peminjaman struct {
	Base
	PesertaDidikID    string       `gorm:"index" json:"pesertaDidikId"`
	BukuID            string       `gorm:"index" json:"bukuId"`
	KelasID           string       `gorm:"index" json:"kelasId"` // snapshot kelas siswa saat transaksi
	Semester          string       `json:"semester"`
	TanggalPinjam     time.Time    `json:"tanggalPinjam"`
	Status            string       `json:"status"` // "Dipinjam"|"Dikembalikan"
	DicatatOlehUserID string       `gorm:"index" json:"dicatatOlehUserId"`
	TandaTangan       string       `json:"tandaTangan"` // data:image/png;base64,...
	PesertaDidik      PesertaDidik `json:"pesertaDidik"`
	Buku              Buku         `json:"buku"`
	Kelas             Kelas        `json:"kelas"`
}
type Pengembalian struct {
	Base
	PeminjamanID      string    `gorm:"uniqueIndex" json:"peminjamanId"` // 1 peminjaman → maks 1 pengembalian
	TanggalKembali    time.Time `json:"tanggalKembali"`
	KondisiBuku       string    `json:"kondisiBuku"` // "Baik"|"Rusak Ringan"|"Rusak Berat"|"Hilang"
	Catatan           string    `json:"catatan"`
	DicatatOlehUserID string    `gorm:"index" json:"dicatatOlehUserId"`
	TandaTangan       string    `json:"tandaTangan"`
}

// Modul B — Pengumuman (prd_fitur_simpkbm.md). Broadcast internal staf; sisi siswa
// dihapus (tidak ada portal siswa). Tutor hanya boleh target=kelas & kelas walinya.
type Pengumuman struct {
	Base
	Judul            string     `gorm:"not null" json:"judul"`
	Isi              string     `gorm:"type:text" json:"isi"`
	Target           string     `json:"target"`               // "semua" | "kelas"
	KelasID          *string    `gorm:"index" json:"kelasId"` // null bila target=semua
	Aktif            bool       `gorm:"default:true" json:"aktif"`
	TanggalMulai     *time.Time `json:"tanggalMulai"`
	TanggalSelesai   *time.Time `json:"tanggalSelesai"`
	DibuatOlehUserID string     `gorm:"index" json:"dibuatOlehUserId"`
	Kelas            Kelas      `json:"kelas"`
}

// Modul K — Jurnal Mengajar. Tutor mencatat kegiatan harian (foto bukti opsional).
// Jurnal LANGSUNG final (status=disetujui) saat dicatat — tanpa alur approve/reject.
type JurnalMengajar struct {
	Base
	TutorID         string        `gorm:"index" json:"tutorId"`
	MapelID         string        `gorm:"index" json:"mapelId"`
	KelasID         string        `gorm:"index" json:"kelasId"`
	Tanggal         time.Time     `json:"tanggal"`
	Materi          string        `gorm:"type:text" json:"materi"`
	Kegiatan        string        `gorm:"type:text" json:"kegiatan"`
	FotoPath        *string       `json:"fotoPath"`                      // relatif ke ./uploads/jurnal
	Status          string        `gorm:"default:pending" json:"status"` // "pending"|"disetujui"|"ditolak"
	CatatanReviewer string        `gorm:"type:text" json:"catatanReviewer"`
	ReviewedBy      *string       `gorm:"index" json:"reviewedBy"`
	ReviewedAt      *time.Time    `json:"reviewedAt"`
	Tutor           Tutor         `json:"tutor"`
	Mapel           MataPelajaran `json:"mapel"`
	Kelas           Kelas         `json:"kelas"`
}

// Modul C — Tugas Siswa (prd_fitur_simpkbm.md). Tutor membuat tugas per mapel+kelas
// (lampiran opsional); pengumpulan dicatat offline oleh tutor untuk siswa. Pengumpulan
// bersifat upsert (uniqueIndex TugasID+PesertaDidikID) sampai dinilai.
type Tugas struct {
	Base
	MapelID          string        `gorm:"index" json:"mapelId"`
	KelasID          string        `gorm:"index" json:"kelasId"`
	Judul            string        `gorm:"not null" json:"judul"`
	Deskripsi        string        `gorm:"type:text" json:"deskripsi"`
	Deadline         time.Time     `json:"deadline"`
	Semester         string        `json:"semester"`
	BolehUpload      bool          `gorm:"default:true" json:"bolehUpload"`
	FilePath         *string       `json:"filePath"` // relatif ke ./uploads/tugas (lampiran)
	DibuatOlehUserID string        `gorm:"index" json:"dibuatOlehUserId"`
	ModulID          *string       `gorm:"index" json:"modulId"` // Modul L — opsional, kaitkan ke modul pembelajaran
	Mapel            MataPelajaran `json:"mapel"`
	Kelas            Kelas         `json:"kelas"`
}
type PengumpulanTugas struct {
	Base
	TugasID           string       `gorm:"uniqueIndex:pengumpulan" json:"tugasId"`
	PesertaDidikID    string       `gorm:"uniqueIndex:pengumpulan" json:"pesertaDidikId"`
	TanggalKumpul     time.Time    `json:"tanggalKumpul"`
	JawabanTeks       string       `gorm:"type:text" json:"jawabanTeks"`
	FilePath          *string      `json:"filePath"`                        // relatif ke ./uploads/tugas
	Status            string       `gorm:"default:Terkumpul" json:"status"` // "Terkumpul"|"Terlambat"|"Dinilai"
	Nilai             *float64     `json:"nilai"`
	CatatanTutor      string       `gorm:"type:text" json:"catatanTutor"`
	DinilaiOlehUserID *string      `gorm:"index" json:"dinilaiOlehUserId"`
	PesertaDidik      PesertaDidik `json:"pesertaDidik"`
}

// Modul E — Materi Pembelajaran. Tutor mengunggah materi per mapel+kelas; komentar
// internal staf (tanpa PesertaDidikID). File dilayani via scoped download handler.
type Materi struct {
	Base
	MapelID           string           `gorm:"index" json:"mapelId"`
	KelasID           string           `gorm:"index" json:"kelasId"`
	Judul             string           `gorm:"not null" json:"judul"`
	Deskripsi         string           `gorm:"type:text" json:"deskripsi"`
	FilePath          string           `gorm:"not null" json:"filePath"` // relatif ke ./uploads/materi; "" bila materi hanya link
	Tipe              string           `json:"tipe"`
	Ukuran            int64            `json:"ukuran"`
	Semester          string           `json:"semester"`
	DibuatOlehUserID  string           `gorm:"index" json:"dibuatOlehUserId"`
	ModulID           *string          `gorm:"index" json:"modulId"` // Modul L — opsional, kaitkan ke modul pembelajaran
	Urutan            int              `gorm:"default:0" json:"urutan"`
	Tanggal           *time.Time       `gorm:"index" json:"tanggal"`
	LinkURL           string           `gorm:"type:text" json:"linkUrl"`
	ShareToken        *string          `gorm:"uniqueIndex" json:"shareToken,omitempty"`
	SharePasswordHash *string          `json:"-"`
	Mapel             MataPelajaran    `json:"mapel"`
	Kelas             Kelas            `json:"kelas"`
	Komentar          []KomentarMateri `gorm:"foreignKey:MateriID" json:"komentar"`
}
type KomentarMateri struct {
	Base
	MateriID string  `gorm:"index" json:"materiId"`
	UserID   *string `gorm:"index" json:"userId"`
	Isi      string  `gorm:"type:text" json:"isi"`
}

// Modul R — RPP (Rencana Pelaksanaan Pembelajaran). Tutor penyu­sun (ditugaskan admin
// via IsRPPMaker) mengunggah file RPP (PDF/Word) per mapel+jenjang; 1 RPP melayani
// semua rombel di jenjang itu (distribusi tersinkron). Tutor lain yang mengajar
// jenjang tsb bisa lihat & download. File dilayani via scoped download handler.
type RPP struct {
	Base
	TutorID          string        `gorm:"index" json:"tutorId"`
	DibuatOlehUserID string        `gorm:"index" json:"dibuatOlehUserId"`
	MapelID          string        `gorm:"index" json:"mapelId"`
	Jenjang          int           `gorm:"index" json:"jenjang"`
	TahunAjaranID    string        `gorm:"index" json:"tahunAjaranId"`
	FaseID           *string       `gorm:"index" json:"faseId"`
	Semester         string        `json:"semester"`
	Judul            string        `gorm:"not null" json:"judul"`
	PertemuanKe      *int          `json:"pertemuanKe"`
	AlokasiWaktu     string        `json:"alokasiWaktu"`
	Tanggal          *time.Time    `gorm:"index" json:"tanggal"`
	Deskripsi        string        `gorm:"type:text" json:"deskripsi"`
	FilePath         string        `gorm:"not null" json:"filePath"` // relatif ke ./uploads/rpp; wajib (mode upload-only)
	Tipe             string        `json:"tipe"`
	Ukuran           int64         `json:"ukuran"`
	Tutor            *Tutor        `gorm:"foreignKey:TutorID" json:"tutor,omitempty"`
	Mapel            MataPelajaran `json:"mapel"`
	TahunAjaran      TahunAjaran   `json:"tahunAjaran"`
	Fase             *Fase         `gorm:"foreignKey:FaseID" json:"fase,omitempty"`
}

// Modul F — Kelas Virtual. Jadwal kelas daring per mapel+kelas (link meeting).
type KelasVirtual struct {
	Base
	MapelID          string        `gorm:"index" json:"mapelId"`
	KelasID          string        `gorm:"index" json:"kelasId"`
	Judul            string        `gorm:"not null" json:"judul"`
	Deskripsi        string        `gorm:"type:text" json:"deskripsi"`
	LinkMeeting      string        `gorm:"not null" json:"linkMeeting"`
	WaktuMulai       time.Time     `json:"waktuMulai"`
	WaktuSelesai     time.Time     `json:"waktuSelesai"`
	Semester         string        `json:"semester"`
	DibuatOlehUserID string        `gorm:"index" json:"dibuatOlehUserId"`
	Mapel            MataPelajaran `json:"mapel"`
	Kelas            Kelas         `json:"kelas"`
}

// Modul D — Bank Soal + Ujian Luring (prd_fitur_simpkbm.md). Tutor membuat bank soal
// per mapel (PG/essay) & menyusun ujian luring (cetak naskah). Tidak ada pengerjaan
// online (JawabanUjian/SesiUjian dihapus). UjianSoal mengaitkan soal ke ujian + bobot.
type BankSoal struct {
	Base
	MapelID          string        `gorm:"index" json:"mapelId"`
	Tipe             string        `json:"tipe"` // "pg" | "essay"
	Pertanyaan       string        `gorm:"type:text" json:"pertanyaan"`
	Opsi             string        `gorm:"type:text" json:"opsi"` // JSON array string (untuk pg)
	Kunci            string        `json:"kunci"`                 // pg: index (0..n) | essay: teks kunci
	Poin             float64       `json:"poin"`
	DibuatOlehUserID string        `gorm:"index" json:"dibuatOlehUserId"`
	Mapel            MataPelajaran `json:"mapel"`
}
type Ujian struct {
	Base
	MapelID          string        `gorm:"index" json:"mapelId"`
	KelasID          string        `gorm:"index" json:"kelasId"`
	Judul            string        `gorm:"not null" json:"judul"`
	WaktuMulai       time.Time     `json:"waktuMulai"`
	WaktuSelesai     time.Time     `json:"waktuSelesai"`
	DurasiMenit      int           `json:"durasiMenit"`
	GracePeriodMenit int           `json:"gracePeriodMenit"` // waktu toleransi setelah durasi habis (default 5 mnt)
	BatasTabSwitch   int           `json:"batasTabSwitch"`   // 0 = tanpa batas; jika terlampaui ujian dikunci otomatis
	AcakSoal         bool          `json:"acakSoal"`
	AksesKode        string        `gorm:"index" json:"aksesKode"` // kode akses siswa (tanpa login)
	Semester         string        `json:"semester"`
	DibuatOlehUserID string        `gorm:"index" json:"dibuatOlehUserId"`
	Mapel            MataPelajaran `json:"mapel"`
	Kelas            Kelas         `json:"kelas"`
}
type UjianSoal struct {
	Base
	UjianID string   `gorm:"uniqueIndex:ujian_soal" json:"ujianId"`
	SoalID  string   `gorm:"uniqueIndex:ujian_soal" json:"soalId"`
	Bobot   float64  `json:"bobot"`
	Soal    BankSoal `json:"soal"`
}

// Ujian Online — Sesi pengerjaan ujian online oleh peserta didik.
type UjianPeserta struct {
	Base
	UjianID        string       `gorm:"uniqueIndex:ujian_peserta_uniq" json:"ujianId"`
	PesertaDidikID string       `gorm:"index;uniqueIndex:ujian_peserta_uniq" json:"pesertaDidikId"`
	Mulai          *time.Time   `json:"mulai"`
	Selesai        *time.Time   `json:"selesai"`
	Skor           *float64     `gorm:"type:decimal(6,2)" json:"skor"`
	Status         string       `gorm:"default:mulai;index" json:"status"` // "mulai"|"selesai"|"dikunci"
	TabSwitch      int          `json:"tabSwitch"`
	Ujian          Ujian        `gorm:"foreignKey:UjianID" json:"ujian"`
	PesertaDidik   PesertaDidik `gorm:"foreignKey:PesertaDidikID" json:"pesertaDidik"`
}

// UjianJawaban — jawaban per soal oleh peserta didik ujian online.
type UjianJawaban struct {
	Base
	UjianPesertaID string   `gorm:"uniqueIndex:ujian_jawaban_uniq" json:"ujianPesertaId"`
	SoalID         string   `gorm:"uniqueIndex:ujian_jawaban_uniq" json:"soalId"`
	Jawaban        string   `gorm:"type:text" json:"jawaban"`
	Benar          *bool    `json:"benar"`
	Nilai          float64  `gorm:"type:decimal(6,2)" json:"nilai"`
	Soal           BankSoal `gorm:"foreignKey:SoalID" json:"soal"`
}

// Notifikasi — push notification internal untuk user.
type Notifikasi struct {
	Base
	UserID     string     `gorm:"index" json:"userId"`
	Judul      string     `gorm:"not null" json:"judul"`
	Isi        string     `gorm:"type:text" json:"isi"`
	Tipe       string     `json:"tipe"` // "ujian"|"tugas"|"presensi"|"umum"|"rapor"
	RefID      *string    `json:"refId"`
	IsRead     bool       `gorm:"default:false" json:"isRead"`
	DibacaPada *time.Time `json:"dibacaPada"`
}

// ChatMessage — pesan antara orang tua dan guru wali.
type ChatMessage struct {
	Base
	PesertaDidikID string     `gorm:"index" json:"pesertaDidikId"`
	PengirimUserID string     `gorm:"index" json:"pengirimUserId"` // siapa yang kirim
	PenerimaUserID string     `gorm:"index" json:"penerimaUserID"` // siapa yang terima
	Isi            string     `gorm:"type:text" json:"isi"`
	Dibaca         bool       `gorm:"default:false" json:"dibaca"`
	DibacaPada     *time.Time `json:"dibacaPada"`
}

// KalenderEvent — event kalender akademik.
type KalenderEvent struct {
	Base
	Judul            string       `gorm:"not null" json:"judul"`
	Deskripsi        string       `gorm:"type:text" json:"deskripsi"`
	TanggalMulai     time.Time    `gorm:"index" json:"tanggalMulai"`
	TanggalSelesai   *time.Time   `json:"tanggalSelesai"`
	Tipe             string       `json:"tipe"` // "libur"|"ujian"|"kegiatan"|"upacara"|"rapat"
	Warna            string       `json:"warna"`
	Semester         *string      `json:"semester"`
	TahunAjaranID    *string      `gorm:"index" json:"tahunAjaranId"`
	DibuatOlehUserID string       `gorm:"index" json:"dibuatOlehUserId"`
	TahunAjaran      *TahunAjaran `gorm:"foreignKey:TahunAjaranID" json:"tahunAjaran,omitempty"`
}

// Modul O — Program (master, prd_fitur_simpkbm.md). Paket program kesetaraan (A/B/C).
// Relasi opsional ke Kelas & PesertaDidik (field nullable, backward compatible).
type Program struct {
	Base
	Kode          string `gorm:"uniqueIndex" json:"kode"` // "A"|"B"|"C"
	Nama          string `json:"nama"`
	JenjangSetara string `json:"jenjangSetara"`
	Keterangan    string `gorm:"type:text" json:"keterangan"`
}

// Modul N — Fase (master, opsional). Fase A..E; opsional, fallback ke jenjang lama.
type Fase struct {
	Base
	Kode          string `gorm:"uniqueIndex" json:"kode"` // "A".."E"
	Nama          string `json:"nama"`
	JenjangSetara string `json:"jenjangSetara"`
}

// Modul H — Sertifikat (prd_fitur_simpkbm.md). Terbit per peserta didik lulus; nomor
// unik PKBM-<tahun>-<program>-<seq>. QR verify via go-qrcode (publik, no auth).
type Sertifikat struct {
	Base
	PesertaDidikID        string       `gorm:"uniqueIndex" json:"pesertaDidikId"`
	ProgramID             string       `gorm:"index" json:"programId"`
	Nomor                 string       `gorm:"uniqueIndex" json:"nomor"`
	TanggalTerbit         time.Time    `json:"tanggalTerbit"`
	Status                string       `gorm:"default:draft" json:"status"` // "draft"|"terbit"
	DiterbitkanOlehUserID string       `gorm:"index" json:"diterbitkanOlehUserId"`
	PesertaDidik          PesertaDidik `json:"pesertaDidik"`
	Program               Program      `json:"program"`
}

// Modul G — Catatan Perilaku (prd_fitur_simpkbm.md). Catatan positif/negatif per
// peserta didik oleh tutor wali; diagregasi ke rapor (Modul I) sebagai kepribadian.
type CatatanPerilaku struct {
	Base
	PesertaDidikID    string       `gorm:"index" json:"pesertaDidikId"`
	KelasID           string       `gorm:"index" json:"kelasId"`
	Tanggal           time.Time    `json:"tanggal"`
	Kategori          string       `json:"kategori"` // "positif"|"negatif"
	Deskripsi         string       `gorm:"type:text" json:"deskripsi"`
	DicatatOlehUserID string       `gorm:"index" json:"dicatatOlehUserId"`
	PesertaDidik      PesertaDidik `json:"pesertaDidik"`
	Kelas             Kelas        `json:"kelas"`
}

// Modul I — Catatan Rapor (prd_fitur_simpkbm.md). Catatan wali + keputusan kenaikan
// per (peserta didik, tahun ajaran, semester). Rapor sendiri adalah agregasi (tidak
// ada tabel nilai baru) — lihat handler getRapor/printRapor.
type CatatanRapor struct {
	Base
	PesertaDidikID string       `gorm:"uniqueIndex:catatan_rapor_unique" json:"pesertaDidikId"`
	TahunAjaranID  string       `gorm:"uniqueIndex:catatan_rapor_unique" json:"tahunAjaranId"`
	Semester       string       `gorm:"uniqueIndex:catatan_rapor_unique" json:"semester"`
	CatatanWali    string       `gorm:"type:text" json:"catatanWali"`
	NaikKelas      *bool        `json:"naikKelas"`
	KenaikanKe     *string      `json:"kenaikanKe"`
	PesertaDidik   PesertaDidik `json:"pesertaDidik"`
	TahunAjaran    TahunAjaran  `json:"tahunAjaran"`
}

// Modul S — Sumber Nilai (prd_fitur_simpkbm.md). Generalisasi sumber nilai (UM,
// TUGAS, UJIAN, PRAKTIK) + bobot per mapel. Bila ada BobotSumberNilai untuk mapel,
// recomputeRekap menghitung NA gabungan berbobot; bila tidak ada → fallback (NA
// kosong, NP & NK terpisah seperti lama). Backward compatible.
type SumberNilai struct {
	Base
	Kode         string `gorm:"uniqueIndex" json:"kode"` // "UM"|"TUGAS"|"UJIAN"|"PRAKTIK"
	Nama         string `json:"nama"`
	BolehDipakai bool   `gorm:"default:true" json:"bolehDipakai"`
}
type BobotSumberNilai struct {
	Base
	MapelID  string        `gorm:"uniqueIndex:bobot_sumber_unique" json:"mapelId"`
	SumberID string        `gorm:"uniqueIndex:bobot_sumber_unique" json:"sumberId"`
	Bobot    float64       `gorm:"type:decimal(5,2)" json:"bobot"`
	Mapel    MataPelajaran `json:"mapel"`
	Sumber   SumberNilai   `json:"sumber"`
}

// Modul L — Modul Pembelajaran + Capaian (prd_fitur_simpkbm.md). Modul per mapel
// berisi urutan + deskripsi + daftar capaian (outcomes). Kaitkan opsional ke Materi
// (E) & Tugas (C) via field ModulID di tabel tsb (nullable, backward compatible).
type ModulBelajar struct {
	Base
	MapelID   string        `gorm:"index" json:"mapelId"`
	Judul     string        `gorm:"not null" json:"judul"`
	Urutan    int           `json:"urutan"`
	Deskripsi string        `gorm:"type:text" json:"deskripsi"`
	Mapel     MataPelajaran `json:"mapel"`
}
type CapaianModul struct {
	Base
	ModulID   string `gorm:"index" json:"modulId"`
	Kode      string `json:"kode"`
	Deskripsi string `gorm:"type:text" json:"deskripsi"`
}

// Modul M — Kompetensi + Capaian + Nilai + RombelKompetensi (prd_fitur_simpkbm.md).
// Kompetensi per mapel; CapaianKompetensi = outcomes; NilaiKompetensi = nilai per
// (peserta, kompetensi, kelas, semester); RombelKompetensi = kompetensi yg diberlakukan
// di suatu kelas (mendorong matriks penilaian).
type Kompetensi struct {
	Base
	MapelID string        `gorm:"index" json:"mapelId"`
	Nama    string        `gorm:"not null" json:"nama"`
	Mapel   MataPelajaran `json:"mapel"`
}
type CapaianKompetensi struct {
	Base
	KompetensiID string `gorm:"index" json:"kompetensiId"`
	Kode         string `json:"kode"`
	Deskripsi    string `gorm:"type:text" json:"deskripsi"`
}
type NilaiKompetensi struct {
	Base
	PesertaDidikID    string  `gorm:"uniqueIndex:nilai_kompetensi_unique" json:"pesertaDidikId"`
	KompetensiID      string  `gorm:"uniqueIndex:nilai_kompetensi_unique" json:"kompetensiId"`
	KelasID           string  `gorm:"uniqueIndex:nilai_kompetensi_unique;index" json:"kelasId"`
	Semester          string  `gorm:"uniqueIndex:nilai_kompetensi_unique" json:"semester"`
	Nilai             float64 `gorm:"type:decimal(5,2)" json:"nilai"`
	DicatatOlehUserID string  `gorm:"index" json:"dicatatOlehUserId"`
}
type RombelKompetensi struct {
	Base
	KelasID      string `gorm:"uniqueIndex:rombel_kompetensi_unique" json:"kelasId"`
	KompetensiID string `gorm:"uniqueIndex:rombel_kompetensi_unique" json:"kompetensiId"`
}

// Modul R — Import Terpusat (prd_fitur_simpkbm.md). Riwayat import terpusat:
// tipe (siswa/nilai-kompetensi/...), ringkasan baris, error per baris (JSON),
// status proses/selesai/gagal, user pelaksana.
type ImportLog struct {
	Base
	Tipe       string `gorm:"index" json:"tipe"`
	FileName   string `json:"fileName"`
	TotalBaris int    `json:"totalBaris"`
	Berhasil   int    `json:"berhasil"`
	Gagal      int    `json:"gagal"`
	ErrorJson  string `gorm:"type:text" json:"errorJson"`
	Status     string `json:"status"`
	UserID     string `gorm:"index" json:"userId"`
}

type Config struct {
	AccessSecret, RefreshSecret, Env, CookieDomain string
	AccessTTL, RefreshTTL                          time.Duration
}
type Server struct {
	db        *gorm.DB
	cfg       Config
	startedAt time.Time
	metrics   backupMetrics
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	cfg := Config{AccessSecret: env("JWT_ACCESS_SECRET", "development-access-secret-change-me-32-chars"), RefreshSecret: env("JWT_REFRESH_SECRET", "development-refresh-secret-change-me-32"), Env: env("APP_ENV", "development"), CookieDomain: os.Getenv("COOKIE_DOMAIN"), AccessTTL: duration("JWT_ACCESS_TTL", "15m"), RefreshTTL: duration("JWT_REFRESH_TTL", "168h")}
	if err := validateConfig(cfg); err != nil {
		panic(err)
	}
	// Apply any staged restore BEFORE opening the DB: a restore uploads a backup
	// file to a pending location; it is applied here (with an automatic safety
	// backup of the current DB) so the live file is never overwritten while open.
	// See backup.go. Returns nil if no restore is pending.
	if e := applyPendingRestore(); e != nil {
		fmt.Printf("applyPendingRestore FAILED (ignored, continuing with current DB): %v\n", e)
	}
	db, err := openDB()
	if err != nil {
		panic(err)
	}
	s := &Server{db: db, cfg: cfg, startedAt: time.Now()}
	if err = s.migrate(); err != nil {
		panic(err)
	}
	app := fiber.New(fiber.Config{
		ErrorHandler: apiError,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		// The restore endpoint accepts full database files; upload handlers still
		// validate their own file sizes before writing anything to disk.
		BodyLimit: backupUploadLimit(),
	})
	app.Use(logger.New())
	app.Use(helmet.New())
	app.Use(compress.New())
	app.Use(cors.New(cors.Config{AllowOrigins: env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), AllowHeaders: "Origin, Content-Type, Accept, Authorization", AllowCredentials: true}))
	health := func(c *fiber.Ctx) error {
		payload, healthy := s.healthPayload()
		if !healthy {
			return c.Status(fiber.StatusServiceUnavailable).JSON(payload)
		}
		return c.JSON(payload)
	}
	app.Get("/health", health)
	// Short public entry points. The page still calls its JSON endpoints under /api.
	// Register these before the production SPA fallback so they do not render the
	// administrator login page instead.
	app.Get("/ujian", s.serveUjianOnlinePage)
	app.Get("/orangtua", s.serveOrangTuaPortalPage)
	api := app.Group("/api")
	loginLimiterMax := 30
	if cfg.Env == "production" {
		loginLimiterMax = 5
	}
	api.Post("/auth/login", limiter.New(limiter.Config{
		Max:        loginLimiterMax,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Batas percobaan login terlampaui. Silakan tunggu 1 menit."})
		},
	}), s.login)
	api.Post("/auth/refresh", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), s.refresh)
	api.Post("/auth/logout", s.auth, s.logout)
	api.Get("/auth/me", s.auth, s.me)
	api.Put("/auth/account", s.auth, s.updateOwnAccount)
	api.Get("/auth/google/enabled", s.googleEnabled)
	api.Get("/auth/google", s.googleLogin)
	api.Get("/auth/google/callback", s.googleCallback)
	// Public verify endpoints (Modul H/P) — no auth. Hanya data non-sensitif untuk
	// verifikasi QR sertifikat & kartu pelajar. Terdaftar sebelum group protected.
	api.Get("/verify/sertifikat/:nomor", s.verifySertifikat)
	api.Get("/verify/siswa/:nisn", s.verifySiswa)
	// Keep the compose/reverse-proxy probe aligned with the public health probe.
	api.Get("/health", health)
	// Public materi share endpoints (Modul E) — no auth. Halaman share materi
	// untuk peserta didik (publik atau password). Terdaftar sebelum group
	// protected agar empty-prefix auth quirk tidak bocor ke sini.
	api.Get("/materi/share/:token", s.viewSharedMateri)
	api.Get("/materi/share/:token/file", s.downloadSharedMateri)
	// Backup read endpoints — n8n-friendly: admin JWT OR static BACKUP_API_KEY
	// (header X-Backup-Key or ?key=). Registered before the protected group so the
	// empty-prefix auth quirk doesn't leak JWT-only auth onto them.
	api.Get("/backup", s.backupReadAuth, s.listBackupsHandler)
	api.Get("/backup/download", s.backupReadAuth, s.downloadBackup)
	api.Get("/backup/file/:name", s.backupReadAuth, s.downloadBackupFile)
	// Public Ujian Online page & API — no auth. Siswa masuk via NISN + Kode Akses.
	api.Get("/ujian", s.serveUjianOnlinePage)
	api.Get("/ujian-online/page", s.serveUjianOnlinePage) // backward compat redirect
	publicExamLoginMax := 60
	publicParentLoginMax := 30
	if cfg.Env == "production" {
		publicExamLoginMax = 30
		publicParentLoginMax = 10
	}
	api.Post("/ujian-online/cek", limiter.New(limiter.Config{
		Max: publicExamLoginMax, Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Terlalu banyak percobaan. Silakan tunggu 1 menit."})
		},
	}), s.cekUjianOnline)
	api.Post("/ujian-online/:ujianId/mulai", s.mulaiUjianOnline)
	api.Get("/ujian-online/:ujianId/soal", s.getSoalUjianOnline)
	api.Post("/ujian-online/:ujianId/jawab", s.jawabSoal)
	api.Post("/ujian-online/:ujianId/selesai", s.selesaiUjianOnline)
	api.Post("/ujian-online/:ujianId/tab-switch", s.tabSwitchUjianOnline)
	// Public Orang Tua login endpoint — no JWT; login by NISN + Tanggal Lahir.
	api.Post("/orang-tua/login", limiter.New(limiter.Config{
		Max: publicParentLoginMax, Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Terlalu banyak percobaan. Silakan tunggu 1 menit."})
		},
	}), s.loginOrangTua)
	api.Get("/orangtua", s.serveOrangTuaPortalPage)
	api.Get("/orang-tua/portal", s.serveOrangTuaPortalPage) // backward compat redirect
	// SSE notification stream — accepts token via query param (EventSource can't set Authorization headers)
	api.Get("/notifikasi/stream", s.streamNotifikasi)
	protected := api.Group("", s.auth)
	protected.Get("/dashboard", s.dashboard)
	s.routes(protected)
	if cfg.Env == "production" {
		app.Static("/", "./public")
		app.Get("/*", func(c *fiber.Ctx) error { return c.SendFile("./public/index.html") })
	}
	s.startScheduler()
	serverErr := make(chan error, 1)
	go func() { serverErr <- app.Listen(":" + env("PORT", "8080")) }()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-serverErr:
		if err != nil {
			panic(err)
		}
	case <-signalCh:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			fmt.Printf("graceful shutdown failed: %v\n", err)
		}
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

func validateConfig(cfg Config) error {
	if cfg.Env != "production" {
		return nil
	}
	if unsafeSecret(cfg.AccessSecret) || unsafeSecret(cfg.RefreshSecret) {
		return errors.New("production requires JWT_ACCESS_SECRET and JWT_REFRESH_SECRET with unique random values of at least 32 characters")
	}
	databaseURL := strings.ToLower(strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if databaseURL == "" || strings.Contains(databaseURL, "your_password") || strings.Contains(databaseURL, "password123") {
		return errors.New("production requires DATABASE_URL; refusing to start with the SQLite fallback")
	}
	if _, err := parseDatabaseURL(os.Getenv("DATABASE_URL")); err != nil {
		return fmt.Errorf("invalid production DATABASE_URL: %w", err)
	}
	origins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	if len(origins) == 0 || strings.TrimSpace(origins[0]) == "" {
		return errors.New("production requires CORS_ALLOWED_ORIGINS")
	}
	for _, raw := range origins {
		origin := strings.TrimSpace(raw)
		lowerOrigin := strings.ToLower(origin)
		if strings.Contains(lowerOrigin, "ganti") || strings.Contains(lowerOrigin, "your_domain") {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS still contains a placeholder: %q", origin)
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("invalid CORS_ALLOWED_ORIGINS value %q", origin)
		}
	}
	if strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY")) == "" || strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY")) == "" {
		return errors.New("production requires TURNSTILE_SECRET_KEY and TURNSTILE_SITE_KEY")
	}
	if offsiteURL := strings.TrimSpace(os.Getenv("BACKUP_OFFSITE_URL")); offsiteURL != "" {
		u, err := url.Parse(offsiteURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return errors.New("BACKUP_OFFSITE_URL must be a valid HTTP(S) endpoint")
		}
		if u.Scheme != "https" {
			return errors.New("production BACKUP_OFFSITE_URL must use HTTPS")
		}
		if _, err := deriveBackupKey(os.Getenv("BACKUP_ENCRYPTION_KEY")); err != nil {
			return err
		}
		method := strings.ToUpper(strings.TrimSpace(env("BACKUP_OFFSITE_METHOD", "PUT")))
		if method != "PUT" && method != "POST" {
			return errors.New("BACKUP_OFFSITE_METHOD must be PUT or POST")
		}
		if _, err := time.ParseDuration(strings.TrimSpace(env("BACKUP_OFFSITE_TIMEOUT", "5m"))); err != nil {
			return errors.New("BACKUP_OFFSITE_TIMEOUT must be a valid duration")
		}
	}
	if drillURL := strings.TrimSpace(os.Getenv("BACKUP_DRILL_DATABASE_URL")); drillURL != "" {
		if _, err := parseDatabaseURL(drillURL); err != nil {
			return fmt.Errorf("invalid BACKUP_DRILL_DATABASE_URL: %w", err)
		}
	}
	return nil
}

func unsafeSecret(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return len(v) < 32 || strings.Contains(v, "development-") || strings.Contains(v, "change-me") || strings.Contains(v, "ganti_ini") || strings.Contains(v, "ubah-ini") || strings.Contains(v, "your_password") || strings.Contains(v, "your-secret")
}

func weakInitialPassword(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return len(v) < 12 || v == "admin123" || strings.Contains(v, "ganti") || strings.Contains(v, "ubah-ini") || strings.Contains(v, "password")
}

func intEnv(key string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
func duration(k, d string) time.Duration {
	v, e := time.ParseDuration(env(k, d))
	if e != nil {
		panic(e)
	}
	return v
}

// openDB opens the database. SQLite is the default; set DATABASE_URL to switch to
// PostgreSQL (the schema + isUniqueErr are portable). For SQLite we harden the
// connection to stay safe under concurrent writers (25+ tutors inputting together):
//   - WAL journal: readers don't block the single writer, commits are faster,
//     and crash recovery is stronger than the default DELETE journal.
//   - busy_timeout=5000ms: a writer that hits a lock WAITS up to 5s instead of
//     failing immediately with "database is locked" (SQLITE_BUSY).
//   - synchronous=NORMAL: the recommended pairing with WAL — much faster than
//     FULL, no corruption risk, at most the last transaction is lost on power loss.
//   - MaxOpenConns(1): the codebase is written assuming a single SQLite connection
//     (see the peminjaman precompute in routes.go and seedDummy's comment); capping
//     the pool avoids internal lock contention and the deferred-lock upgrade
//     deadlock. WAL still lets external readers (backup, sqlite3 CLI) work
//     concurrently. NOTE: foreign_keys is intentionally NOT enabled — the schema
//     relies on plain string ID columns (no declared FK constraints) and manual
//     orphan checks (e.g. deleteKelas's 16-table scan); enabling it now could
//     reject existing orphan rows or inserts the manual guards don't cover.
func openDB() (*gorm.DB, error) {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		db, err := gorm.Open(postgres.Open(url), &gorm.Config{TranslateError: true})
		if err != nil {
			return nil, err
		}
		configureDBPool(db, false)
		return db, nil
	}
	dsn := "file:pkbm-lms.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	configureDBPool(db, true)
	return db, nil
}

func configureDBPool(db *gorm.DB, sqliteDB bool) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	if sqliteDB {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxIdleTime(0)
		return
	}
	maxOpen := intEnv("DB_MAX_OPEN_CONNS", 25)
	if maxOpen > 200 {
		maxOpen = 200
	}
	maxIdle := intEnv("DB_MAX_IDLE_CONNS", 10)
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
}
func (s *Server) migrate() error {
	if e := s.migrateSchema(); e != nil {
		return e
	}
	// Auto-seed comprehensive dummy data on startup in non-production envs, so the
	// app is immediately explorable locally (go run → buka frontend). Idempotent:
	// guarded by the sentinel tahun ajaran, so re-runs are a no-op and never disturb
	// an operator's own active year once seeded. Disable with SEED_DUMMY_ON_START=false.
	// (e2e tests call migrateSchema() directly to skip this — dummy data would
	// collide with the records each test creates itself, e.g. mapel "Matematika".)
	if s.cfg.Env != "production" && env("SEED_DUMMY_ON_START", "true") == "true" && !pendingRestoreApplied {
		var dummyTA TahunAjaran
		if s.db.Where("nama_tahun_ajaran = ?", "2025/2026-DUMMY").First(&dummyTA).Error != nil {
			if msg, err := s.seedDummy(); err != nil {
				fmt.Printf("auto-seed dummy FAILED: %v\n", err)
			} else {
				fmt.Printf("auto-seed dummy: %s\n", msg)
			}
		}
	}
	return nil
}

// migrateSchema runs AutoMigrate plus the minimal master seed (pokjar, active
// tahun ajaran, jadwal, sumber nilai, admin account, default bobot/ambang). It
// does NOT seed comprehensive dummy data — used by e2e tests so their own
// fixtures are the sole source of data.
func (s *Server) migrateSchema() error {
	if e := s.db.AutoMigrate(&User{}, &RefreshToken{}, &AuditLog{}, &Tutor{}, &DokumenSistem{}, &SuratSiswa{}, &SuratSiswaFile{}, &OrangTua{}, &Pokjar{}, &TahunAjaran{}, &Semester{}, &Kelas{}, &RiwayatWaliKelas{}, &MataPelajaran{}, &KelasMapel{}, &PenugasanGuruMapel{}, &PesertaDidik{}, &RiwayatKelasPesertaDidik{}, &PengaturanJadwal{}, &Presensi{}, &PresensiDetail{}, &Tema{}, &CapaianPembelajaran{}, &NilaiCP{}, &NilaiUM{}, &PengaturanBobotNilai{}, &AmbangPredikat{}, &RekapNilaiAkhir{}, &Buku{}, &BukuKelas{}, &Peminjaman{}, &Pengembalian{}, &Pengumuman{}, &JurnalMengajar{}, &Tugas{}, &PengumpulanTugas{}, &Materi{}, &KomentarMateri{}, &RPP{}, &KelasVirtual{}, &BankSoal{}, &Ujian{}, &UjianSoal{}, &UjianPeserta{}, &UjianJawaban{}, &Notifikasi{}, &KalenderEvent{}, &Program{}, &Fase{}, &Sertifikat{}, &CatatanPerilaku{}, &CatatanRapor{}, &SumberNilai{}, &BobotSumberNilai{}, &ModulBelajar{}, &CapaianModul{}, &Kompetensi{}, &CapaianKompetensi{}, &NilaiKompetensi{}, &RombelKompetensi{}, &ImportLog{}, &ChatMessage{}); e != nil {
		return e
	}
	if e := s.ensureTemporaryNISNIndex(); e != nil {
		return e
	}
	if e := s.ensureOptionalUserEmailIndex(); e != nil {
		return e
	}
	if err := s.normalizeStoredRombelNames(); err != nil {
		return err
	}
	// Modul K — alur approve/reject jurnal dihapus; jurnal langsung final. Sekali
	// jalan: jurnal lama berstatus "pending" dianggap disetujui agar tidak macet.
	s.db.Model(&JurnalMengajar{}).Where("status = ?", "pending").Update("status", "disetujui")
	var n int64
	s.db.Model(&Pokjar{}).Count(&n)
	if n == 0 {
		for _, p := range []Pokjar{{NamaPokjar: "PKBM Tunas Ilmu Pusat", Tipe: "pusat"}, {NamaPokjar: "Nashirus Sunnah", Tipe: "binaan"}, {NamaPokjar: "Umar bin Khattab", Tipe: "binaan"}, {NamaPokjar: "Lentera Qalbu", Tipe: "binaan"}} {
			s.db.Create(&p)
		}
	}
	var active TahunAjaran
	if s.db.Where("is_aktif = ?", true).First(&active).Error != nil {
		now := time.Now()
		s.db.Create(&TahunAjaran{NamaTahunAjaran: fmt.Sprintf("%d/%d", now.Year(), now.Year()+1), TanggalMulai: now, TanggalSelesai: now.AddDate(1, 0, 0), IsAktif: true})
	}
	// Backfill Semester records untuk semua TahunAjaran yang belum punya.
	var allTA []TahunAjaran
	s.db.Find(&allTA)
	for i := range allTA {
		var cnt int64
		s.db.Model(&Semester{}).Where("tahun_ajaran_id = ?", allTA[i].ID).Count(&cnt)
		if cnt == 0 {
			s.syncSemesters(s.db, &allTA[i])
		}
	}
	var setting PengaturanJadwal
	if s.db.First(&setting).Error != nil {
		s.db.Create(&PengaturanJadwal{HariDefault: "Sabtu", JamGenerate: "00:05", ZonaWaktu: "Asia/Jakarta"})
	}
	// Modul S — seed the four standard sumber nilai on first run.
	var sn int64
	s.db.Model(&SumberNilai{}).Count(&sn)
	if sn == 0 {
		for _, su := range []SumberNilai{
			{Kode: "UM", Nama: "Ulangan / Ujian Harian", BolehDipakai: true},
			{Kode: "TUGAS", Nama: "Tugas", BolehDipakai: true},
			{Kode: "UJIAN", Nama: "Ujian Akhir", BolehDipakai: true},
			{Kode: "PRAKTIK", Nama: "Praktik / Kompetensi", BolehDipakai: true},
		} {
			s.db.Create(&su)
		}
	}
	// Seed the admin account only on the very first run. We intentionally do NOT
	// reset the password/lock on subsequent startups: doing so would revert any
	// password change and unlock the account behind the operator's back on every
	// restart. Override the initial password via ADMIN_DEFAULT_PASSWORD in prod.
	var admin User
	adminErr := s.db.Where("username = ?", "admin").First(&admin).Error
	if errors.Is(adminErr, gorm.ErrRecordNotFound) {
		initialPassword := env("ADMIN_DEFAULT_PASSWORD", "Admin123")
		if s.cfg.Env == "production" && weakInitialPassword(initialPassword) {
			return errors.New("ADMIN_DEFAULT_PASSWORD must be set to a strong password before the first production start")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := s.db.Create(&User{Username: "admin", PasswordHash: string(hash), Role: "admin", IsActive: true}).Error; err != nil {
			return err
		}
	} else if adminErr != nil {
		return adminErr
	}
	// Backfill default nilai settings (bobot + ambang predikat) for any pre-existing
	// mapel that predates the Modul Nilai migration.
	var mapels []MataPelajaran
	s.db.Find(&mapels)
	for _, m := range mapels {
		var n int64
		s.db.Model(&PengaturanBobotNilai{}).Where("mapel_id = ?", m.ID).Count(&n)
		if n == 0 {
			s.db.Create(&PengaturanBobotNilai{MapelID: m.ID, BobotKeterampilan: 60, BobotPengetahuan: 40})
		}
		s.db.Model(&AmbangPredikat{}).Where("mapel_id = ?", m.ID).Count(&n)
		if n == 0 {
			s.seedAmbang(s.db, m.ID, m.NamaMapel)
		}
	}
	return nil
}

// ensureTemporaryNISNIndex keeps official NISNs unique while allowing the
// shared temporary placeholder used by students awaiting NISN issuance.
func (s *Server) ensureTemporaryNISNIndex() error {
	if s.db.Dialector.Name() != "sqlite" && s.db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, name := range []string{"uni_peserta_didiks_nisn", "idx_peserta_didiks_nisn"} {
		if e := s.db.Exec(`DROP INDEX IF EXISTS "` + name + `"`).Error; e != nil {
			return e
		}
	}
	return s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "uniq_peserta_didiks_nisn_real" ON "peserta_didiks" ("nisn") WHERE "nisn" <> '0000000000'`).Error
}

// ensureOptionalUserEmailIndex allows multiple tutor accounts to start with
// an empty email while keeping every real email address unique.
func (s *Server) ensureOptionalUserEmailIndex() error {
	if s.db.Dialector.Name() != "sqlite" && s.db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, name := range []string{"uni_users_email", "idx_users_email", "users_email_key"} {
		if e := s.db.Exec(`DROP INDEX IF EXISTS "` + name + `"`).Error; e != nil {
			return e
		}
	}
	return s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "uniq_users_email_real" ON "users" ("email") WHERE "email" <> ''`).Error
}
func apiError(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}
	message := err.Error()
	if code >= fiber.StatusInternalServerError && env("APP_ENV", "development") == "production" {
		message = "Terjadi kesalahan internal. Silakan coba lagi."
	}
	return c.Status(code).JSON(fiber.Map{"error": message})
}
func (s *Server) token(user User, secret string, ttl time.Duration) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID, "role": user.Role, "exp": time.Now().Add(ttl).Unix()}).SignedString([]byte(secret))
}

func (s *Server) parseAccessToken(raw string) (jwt.MapClaims, string, string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if raw == "" {
		return nil, "", "", errors.New("missing access token")
	}
	t, err := jwt.ParseWithClaims(raw, jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.AccessSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || t == nil || !t.Valid {
		return nil, "", "", errors.New("invalid access token")
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, "", "", errors.New("invalid access token claims")
	}
	uid, uidOK := claims["sub"].(string)
	role, roleOK := claims["role"].(string)
	if !uidOK || strings.TrimSpace(uid) == "" || !roleOK || strings.TrimSpace(role) == "" {
		return nil, "", "", errors.New("invalid access token claims")
	}
	return claims, uid, role, nil
}

func hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func (s *Server) login(c *fiber.Ctx) error {
	var in struct {
		Login          string `json:"login"`
		Password       string `json:"password"`
		TurnstileToken string `json:"turnstileToken"`
	}
	if e := c.BodyParser(&in); e != nil || in.Login == "" || in.Password == "" {
		return fiber.NewError(400, "Username dan password wajib diisi")
	}
	if e := s.requireTurnstile(c, in.TurnstileToken); e != nil {
		return e
	}
	var u User
	if e := s.db.Where("username = ? OR email = ?", in.Login, in.Login).First(&u).Error; e != nil {
		return fiber.NewError(401, "Username/Email atau kata sandi tidak ditemukan.")
	}
	if !u.IsActive {
		return fiber.NewError(403, "Akun Anda nonaktif. Silakan hubungi Administrator.")
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		return fiber.NewError(403, "Akun terkunci sementara karena 5x salah kata sandi. Silakan coba beberapa saat lagi.")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		u.FailedLogins++
		if u.FailedLogins >= 5 {
			t := time.Now().Add(15 * time.Minute)
			u.LockedUntil = &t
			u.FailedLogins = 0
		}
		if err := s.db.Save(&u).Error; err != nil {
			return fiber.NewError(500, "gagal memperbarui status login")
		}
		return fiber.NewError(401, "Kata sandi yang Anda masukkan salah.")
	}
	u.FailedLogins = 0
	u.LockedUntil = nil
	if err := s.db.Save(&u).Error; err != nil {
		return fiber.NewError(500, "gagal memperbarui status login")
	}
	return s.issue(c, u)
}
func verifyTurnstile(token, ip string) bool {
	key := os.Getenv("TURNSTILE_SECRET_KEY")
	if key == "" {
		// No secret configured -> Turnstile disabled. Callers gate on this too.
		return true
	}
	client := &http.Client{Timeout: 8 * time.Second}
	r, e := client.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{"secret": {key}, "response": {token}, "remoteip": {ip}})
	if e != nil {
		return false
	}
	defer r.Body.Close()
	if r.StatusCode < http.StatusOK || r.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	var result struct {
		Success bool `json:"success"`
	}
	return json.NewDecoder(r.Body).Decode(&result) == nil && result.Success
}

// requireTurnstile keeps local/test environments usable while making the
// production login surface fail closed when Cloudflare is not configured.
func (s *Server) requireTurnstile(c *fiber.Ctx, token string) error {
	if s.cfg.Env != "production" {
		return nil
	}
	if os.Getenv("TURNSTILE_SECRET_KEY") == "" {
		return fiber.NewError(503, "Proteksi Turnstile belum dikonfigurasi oleh Administrator")
	}
	if token == "" || !verifyTurnstile(token, c.IP()) {
		return fiber.NewError(401, "Verifikasi Turnstile gagal")
	}
	return nil
}
func (s *Server) issue(c *fiber.Ctx, u User) error {
	if e := s.fillUserNames(&u); e != nil {
		return e
	}
	access, e := s.token(u, s.cfg.AccessSecret, s.cfg.AccessTTL)
	if e != nil {
		return e
	}
	raw := uuid.NewString() + uuid.NewString()
	if err := s.db.Create(&RefreshToken{UserID: u.ID, TokenHash: hash(raw), ExpiresAt: time.Now().Add(s.cfg.RefreshTTL)}).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan sesi")
	}
	// Keep password login aligned with Google OAuth. Lax allows the refresh
	// cookie to survive the application's cross-origin/proxy navigation while
	// still preventing it from being sent on ordinary cross-site subrequests.
	c.Cookie(&fiber.Cookie{Name: "refresh_token", Value: raw, HTTPOnly: true, Secure: s.cfg.Env == "production", SameSite: "Lax", Domain: s.cfg.CookieDomain, Expires: time.Now().Add(s.cfg.RefreshTTL), Path: "/api/auth"})
	s.audit(&u.ID, "login", "auth", "")
	return c.JSON(fiber.Map{"accessToken": access, "user": u})
}
func (s *Server) refresh(c *fiber.Ctx) error {
	raw := c.Cookies("refresh_token")
	var rt RefreshToken
	if raw == "" || s.db.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash(raw), time.Now()).First(&rt).Error != nil {
		return fiber.NewError(401, "invalid refresh token")
	}
	now := time.Now()
	result := s.db.Model(&RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", rt.ID).
		Updates(map[string]interface{}{"revoked_at": now})
	if result.Error != nil {
		// If we cannot revoke the old refresh token, do NOT issue a new pair:
		// the old (still-valid) token could then be replayed indefinitely.
		return fiber.NewError(500, "unable to rotate refresh token")
	}
	if result.RowsAffected != 1 {
		return fiber.NewError(401, "invalid refresh token")
	}
	var u User
	if s.db.First(&u, "id = ?", rt.UserID).Error != nil {
		return fiber.NewError(401, "user not found")
	}
	if !u.IsActive {
		return fiber.NewError(403, "Akun Anda nonaktif. Silakan hubungi Administrator.")
	}
	return s.issue(c, u)
}
func (s *Server) logout(c *fiber.Ctx) error {
	raw := c.Cookies("refresh_token")
	if raw != "" {
		s.db.Model(&RefreshToken{}).Where("token_hash = ?", hash(raw)).Update("revoked_at", time.Now())
	}
	c.ClearCookie("refresh_token")
	return c.SendStatus(204)
}
func (s *Server) auth(c *fiber.Ctx) error {
	_, uid, role, err := s.parseAccessToken(c.Get("Authorization"))
	if err != nil {
		return fiber.NewError(401, err.Error())
	}
	c.Locals("userID", uid)
	c.Locals("role", role)
	return c.Next()
}
func (s *Server) me(c *fiber.Ctx) error {
	var u User
	if e := s.db.First(&u, "id = ?", c.Locals("userID")).Error; e != nil {
		return e
	}
	if e := s.fillUserNames(&u); e != nil {
		return e
	}
	return c.JSON(u)
}

// ---------------------------------------------------------------------------
// Google OAuth login (prd: optional SSO).
//
// Flow: GET /auth/google -> redirect to Google consent (state in a short-lived
// oauth_state cookie) -> GET /auth/google/callback -> validate state, exchange
// code, fetch userinfo, match an existing User by verified email (admin must
// provision the account first; no auto-create), set the refresh_token cookie,
// then redirect to FRONTEND_URL. The SPA's on-load /auth/refresh call mints the
// access token, so no token is exposed in the URL.
//
// Configure via env: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL
// (e.g. http://localhost:5173/api/auth/google/callback in dev so the cookie is
// set on the same origin the SPA proxies through), and FRONTEND_URL.
// ---------------------------------------------------------------------------

func (s *Server) googleConfig() *oauth2.Config {
	cid := os.Getenv("GOOGLE_CLIENT_ID")
	if cid == "" || os.Getenv("GOOGLE_CLIENT_SECRET") == "" || os.Getenv("GOOGLE_REDIRECT_URL") == "" {
		return nil
	}
	return &oauth2.Config{
		ClientID:     cid,
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}
}

func (s *Server) googleEnabled(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"enabled": s.googleConfig() != nil})
}

func (s *Server) googleLogin(c *fiber.Ctx) error {
	front := os.Getenv("FRONTEND_URL")
	if front == "" {
		front = "http://localhost:5173"
	}
	cfg := s.googleConfig()
	if cfg == nil {
		return c.Redirect(front+"/?google_error="+url.QueryEscape("Login Google belum dikonfigurasi oleh Administrator."), fiber.StatusTemporaryRedirect)
	}
	state := uuid.NewString()
	c.Cookie(&fiber.Cookie{Name: "oauth_state", Value: state, HTTPOnly: true, SameSite: "Lax", Expires: time.Now().Add(10 * time.Minute), Path: "/"})
	return c.Redirect(cfg.AuthCodeURL(state), fiber.StatusTemporaryRedirect)
}

func (s *Server) googleCallback(c *fiber.Ctx) error {
	front := os.Getenv("FRONTEND_URL")
	if front == "" {
		front = "http://localhost:5173"
	}
	fail := func(msg string) error {
		return c.Redirect(front+"/?google_error="+url.QueryEscape(msg), fiber.StatusTemporaryRedirect)
	}
	cfg := s.googleConfig()
	if cfg == nil {
		return fail("Login Google belum dikonfigurasi.")
	}
	state := c.Query("state")
	if state == "" || state != c.Cookies("oauth_state") {
		return fail("State tidak valid (kemungkinan CSRF). Silakan coba lagi.")
	}
	c.ClearCookie("oauth_state")
	code := c.Query("code")
	if code == "" {
		return fail("Tidak ada kode otorisasi dari Google.")
	}
	tok, e := cfg.Exchange(c.Context(), code)
	if e != nil {
		return fail("Gagal menukar kode otorisasi: " + e.Error())
	}
	client := cfg.Client(c.Context(), tok)
	r, e := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if e != nil {
		return fail("Gagal mengambil info akun Google.")
	}
	defer r.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
	if r.StatusCode != 200 {
		return fail(fmt.Sprintf("Google userinfo gagal (HTTP %d): %s", r.StatusCode, string(respBody)))
	}
	var info struct {
		Sub           string      `json:"sub"`
		Email         string      `json:"email"`
		EmailVerified interface{} `json:"email_verified"` // Google returns bool true/false
		Name          string      `json:"name"`
	}
	if e := json.Unmarshal(respBody, &info); e != nil {
		return fail("Info akun Google tidak terbaca.")
	}
	if info.Email == "" {
		return fail("Akun Google tidak memiliki email.")
	}
	verified := false
	switch v := info.EmailVerified.(type) {
	case bool:
		verified = v
	case string:
		verified = v == "true"
	}
	if !verified {
		return fail("Email Google belum diverifikasi. Verifikasi email Anda di Google lalu coba lagi.")
	}
	var u User
	if e := s.db.Where("LOWER(email) = ?", strings.ToLower(info.Email)).First(&u).Error; e != nil {
		return fail("Akun Google \"" + info.Email + "\" belum terdaftar. Hubungi Administrator untuk mendaftarkan akun Anda.")
	}
	if !u.IsActive {
		return fail("Akun Anda nonaktif. Silakan hubungi Administrator.")
	}
	// Issue a refresh-token cookie (SameSite=Lax so it survives the cross-site
	// redirect from Google). The SPA mints the access token via /auth/refresh.
	raw := uuid.NewString() + uuid.NewString()
	if err := s.db.Create(&RefreshToken{UserID: u.ID, TokenHash: hash(raw), ExpiresAt: time.Now().Add(s.cfg.RefreshTTL)}).Error; err != nil {
		return fail("Gagal menyiapkan sesi Google. Silakan coba lagi.")
	}
	c.Cookie(&fiber.Cookie{
		Name: "refresh_token", Value: raw, HTTPOnly: true,
		Secure: s.cfg.Env == "production", SameSite: "Lax", Domain: s.cfg.CookieDomain,
		Expires: time.Now().Add(s.cfg.RefreshTTL), Path: "/api/auth",
	})
	s.audit(&u.ID, "google_login", "auth", info.Email)
	return c.Redirect(front+"/?google=ok", fiber.StatusTemporaryRedirect)
}
func (s *Server) audit(uid *string, a, r, d string) {
	s.db.Create(&AuditLog{UserID: uid, Action: a, Resource: r, Detail: d})
}
func (s *Server) auditTx(tx *gorm.DB, uid *string, a, r, d string) {
	tx.Create(&AuditLog{UserID: uid, Action: a, Resource: r, Detail: d})
}
func (s *Server) admin(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(403, "admin access required")
	}
	return c.Next()
}
func (s *Server) managementRead(c *fiber.Ctx) error {
	role := c.Locals("role")
	if role != "admin" && role != "kepala_sekolah" {
		return fiber.NewError(403, "management read access required")
	}
	return c.Next()
}
func (s *Server) writable(c *fiber.Ctx) error {
	if c.Locals("role") == "kepala_sekolah" {
		return fiber.NewError(403, "kepala sekolah has read-only access")
	}
	return c.Next()
}
