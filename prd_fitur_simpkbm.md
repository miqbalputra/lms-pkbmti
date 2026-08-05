# PRD + Rancangan Implementasi — Fitur Tambahan (Acuan SIMPKBM)

> **Dokumen:** `prd_fitur_simpkbm.md`
> **Acuan:** analisa `fitur_tambahan.md` (fitur SIMPKBM yang belum ada di aplikasi kita).
> **Stack:** Backend Go Fiber v2.52 + GORM (single package `backend/cmd/server`),
> Frontend React 19 + Vite + Tailwind v4 + shadcn/ui.
> **Cakupan:** Fitur **B–S** (18 modul; **A Portal Siswa & Q Presensi QR dihapus** —
> tidak ada akun/portal siswa). Fitur **T** (subscription SaaS + Tripay)
> **dilewati** — arsitektur multi-tenant yang tidak relevan untuk aplikasi internal
> single-institution.
> **Versi:** 1.1 (per 2026-08-03).

---

## 0. Konvensi & aturan implementasi (berlaku untuk semua modul)

Dipakai seragam agar konsisten dengan kode yang sudah berjalan:

### 0.1 Backend
- **Model** di `main.go` dengan `Base` (UUID auto `BeforeCreate`). Field opsional pakai
  pointer `*time.Time` / `*string` (nil = kosong). `json:"camelCase"`.
- **Routes & handler** di `routes.go`. Helper generik: `list[T]`, `create[T]`,
  `update[T]`, `deleteRow[T]`, `audit`/`auditTx`, `id(c)`.
- **RBAC** (urutan group wajib dijaga — quirk Fiber v2.52 empty-prefix group):
  - `api` (bare-api, **sebelum** `readAll` group) → endpoint yang diakses guru.
  - `readAll` group (middleware `s.managementRead`: admin+kepala, tolak guru) → baca
    lintas rombel.
  - `admin` group (middleware `s.admin`) → CRUD master/admin-only.
  - `s.writable` (tolak kepala_sekolah) untuk tulis data transaksional.
- **Guard wali kelas** via `canManageKelas(c, kelasID)` (admin bypass; guru wajib
  `kelas.WaliKelasID == user.TutorID`).
- **Aturan deadlock SQLite** (`SetMaxOpenConns(1)`): **jangan** panggil `s.db.*` atau
  `s.semester()` (yang baca `s.db`) **di dalam** `s.db.Transaction(...)`. Precompute/
  guard **di luar** tx, baru `tx.Create/Update` di dalam.
- **Semester** via `s.semester(t)` (sumber terpusat; fallback aman bila field genap kosong).
- **Signature** via `validSignature` + komponen `Signature` (canvas → base64 PNG).
- **Export** `gofpdf` (PDF) + `excelize` (XLSX) sudah ada. **`go-qrcode`** perlu
  ditambahkan ke `go.mod` untuk fitur QR (P kartu, H sertifikat).
- **Cron** `robfig/cron/v3` di `startScheduler`; webhook n8n via `N8N_WEBHOOK_URL`
  (no-op bila kosong).
- **Upload file** baru: simpan di `./uploads/<modul>/<uuid><ext>`, serve via
  `app.Static("/uploads", "./uploads")` (atau route Fiber). Validasi MIME+ukuran
  (mis. maks 10 MB, whitelist `pdf,docx,xlsx,png,jpg`). Simpan **path relatif** di DB,
  bukan blob.
- **Konvensi penamaan tabel** GORM: struct `FooBar` → tabel `foo_bars`. Hindari JOIN
  mentah dengan nama tabel diasumsikan; pakai subquery GORM
  `Where("kelas_id IN (?)", s.db.Model(&Kelas{}).Select("id").Where(...))`.

### 0.2 Frontend
- Tiap page punya helper `request(path, token, method, body)` sendiri (pola kodebase).
- `readOnly = user.role !== 'admin'`; untuk kepala, turunkan `readOnly` per konteks.
- Komponen reusable: `PageToolbar`, `FormCard`, `EmptyState`, `Field`, `Select`,
  `Table`, `Signature`, `Badge`, `AlertDialog`, `Dialog`, `sonner` toast.
- MasterData config-driven untuk master sederhana (schema registry + table block).
- View custom untuk alur kompleks (matrix, approval, cetak).
- Export = blob-download (`URL.createObjectURL(await r.blob())`).
- NavGroup baru di `AppSidebar.tsx`; routing di `App.tsx` (cabang sebelum fallback
  MasterData).

---

# MODUL B — Pengumuman (Announcements)

## B.1 Tujuan
Papan informasi internal: broadcast dari admin/tutor kepada staf (admin/tutor/kepala).
Sisi siswa dihapus (tidak ada portal siswa).

## B.2 Aktor
- Admin (semua), Tutor (rombel walinya), Kepala (lihat semua).

## B.3 Model
```go
type Pengumuman struct {
    Base
    Judul     string    `gorm:"not null" json:"judul"`
    Isi       string    `gorm:"type:text" json:"isi"`
    Target    string    `json:"target"`     // "semua" | "kelas"
    KelasID   *string   `gorm:"index" json:"kelasId"` // null bila semua
    Aktif     bool      `gorm:"default:true" json:"aktif"`
    TanggalMulai  *time.Time `json:"tanggalMulai"`
    TanggalSelesai *time.Time `json:"tanggalSelesai"`
    DibuatOlehUserID string `gorm:"index" json:"dibuatOlehUserId"`
    Kelas     Kelas  `json:"kelas"`
}
```

## B.4 Endpoint
- Admin group: CRUD `/pengumuman` (admin). Tutor: `POST /pengumuman` scoped kelas
  walinya (bare-api), `GET /pengumuman` (readAll).
- `PUT /pengumuman/:id` (admin / pembuat).

## B.5 RBAC
- Tutor buat hanya untuk `target=kelas` & `kelasId` walinya (`canManageKelas`).
- Admin bebas semua target.

## B.6 UI
- Admin/tutor: MasterData-style + pilih target. `pages/PengumumanView.tsx`.

## B.7 Aturan bisnis
- Pengumuman tampil bila `aktif && now ∈ [mulai, selesai]` (tanggal opsional = selalu).
- Admin bisa nonaktifkan.

---

# MODUL C — Tugas Siswa (Assignments)

## C.1 Tujuan
Tutor menugaskan (rombel+mapel walinya); tutor mencatat pengumpulan & menilai (siswa
kumpul **offline/luring**, tidak ada portal siswa). **Bukan** Penugasan Tutor.

## C.2 Aktor
- Tutor (buat untuk rombel+mapel walinya, catat pengumpulan & nilai), Admin (semua),
  Kepala (lihat).

## C.3 Model
```go
type Tugas struct {
    Base
    MapelID   string    `gorm:"index" json:"mapelId"`
    KelasID   string    `gorm:"index" json:"kelasId"`
    Judul     string    `gorm:"not null" json:"judul"`
    Deskripsi string    `gorm:"type:text" json:"deskripsi"`
    Deadline  time.Time `json:"deadline"`
    Semester  string    `json:"semester"`
    BolehUpload bool    `gorm:"default:true" json:"bolehUpload"`
    FilePath  *string   `json:"filePath"`     // lampiran tugas (opsional)
    DibuatOlehUserID string `gorm:"index" json:"dibuatOlehUserId"`
}
type PengumpulanTugas struct {
    Base
    TugasID        string `gorm:"uniqueIndex:tugas_siswa_uniq" json:"tugasId"`
    PesertaDidikID string `gorm:"uniqueIndex:tugas_siswa_uniq" json:"pesertaDidikId"`
    TanggalKumpul  time.Time `json:"tanggalKumpul"`
    JawabanTeks    string `gorm:"type:text" json:"jawabanTeks"`
    FilePath       *string `json:"filePath"`  // file siswa (diunggah tutor)
    Status         string `json:"status"`     // "Terkumpul"|"Terlambat"|"Dinilai"
    Nilai          *float64 `json:"nilai"`
    CatatanTutor   string `json:"catatanTutor"`
    DinilaiOlehUserID *string `json:"dinilaiOlehUserId"`
}
```

## C.4 Endpoint
- Tutor (bare-api): `POST /tugas` (guard wali), `PUT/DELETE /tugas/:id`.
- `GET /tugas?kelasId=&mapelId=` (readAll).
- Tutor (bare-api): `POST /tugas/:id/pengumpulan` → catat pengumpulan siswa (teks +
  file lampiran) atas nama siswa; anti-double via `uniqueIndex` (update bila status
  belum `Dinilai`).
- `POST /tugas/:id/nilai` (tutor/admin) → `{pengumpulanId, nilai, catatan}`; set
  status `Dinilai`; guard wali via tugas.kelasId.
- `GET /tugas/:id/pengumpulan` (tutor) → daftar + status.
- Download lampiran: `GET /uploads/tugas/:file` (scoped tutor/admin).

## C.5 RBAC
- Tutor hanya kelas walinya (guard via `tugas.KelasID` → `canManageKelas`).
- Pengumpulan dicatat tutor; satu siswa–satu tugas–satu record (`uniqueIndex`); update
  sebelum `Dinilai`.
- Terlambat: `TanggalKumpul > Deadline` → status `Terlambat` (flag, tetap dicatat).

## C.6 UI
- `pages/TugasView.tsx` (tutor/admin): list + FormCard (mapel, kelas, deadline,
  deskripsi, lampiran). Tab "Pengumpulan" per tugas → tutor catat pengumpulan siswa +
  grade inline.

## C.7 Aturan bisnis
- Deadline lewat: tutor tetap bisa catat pengumpulan (flag Terlambat), atau kunci (opsional).
- Nilai 0–100; opsional masuk komponen nilai (S) bila dipetakan.

## C.8 Edge case
- Tutor catat pengumpulan 2x sebelum dinilai → update row (`uniqueIndex` mencegah
  duplikat; gunakan upsert/select-then-update, bukan Create baru).
- File lampiran > 10MB / MIME tidak diizinkan → 400.

---

# MODUL D — Bank Soal + Ujian (Luring)

## D.1 Tujuan
Bank soal + penjadwalan ujian per rombel (tutor/admin). **Tanpa pengerjaan online
siswa** (tidak ada portal); ujian dikerjakan offline/luring, naskah & kunci dicetak.

## D.2 Aktor
- Tutor (buat ujian + pilih soal), Admin, Kepala (lihat).

## D.3 Model
```go
type BankSoal struct {
    Base
    MapelID   string `gorm:"index" json:"mapelId"`
    Tipe      string `json:"tipe"`        // "pg" | "essay"
    Pertanyaan string `gorm:"type:text" json:"pertanyaan"`
    Opsi      string `gorm:"type:text" json:"opsi"`     // JSON array string ["a","b","c","d"]
    Kunci     string `json:"kunci"`      // huruf kunci untuk pg
    Poin      float64 `json:"poin"`
    DibuatOlehUserID string `gorm:"index" json:"dibuatOlehUserId"`
}
type Ujian struct {
    Base
    MapelID   string `gorm:"index" json:"mapelId"`
    KelasID   string `gorm:"index" json:"kelasId"`
    Judul     string `gorm:"not null" json:"judul"`
    WaktuMulai  time.Time `json:"waktuMulai"`
    WaktuSelesai time.Time `json:"waktuSelesai"`
    DurasiMenit int `json:"durasiMenit"`
    AcakSoal  bool `gorm:"default:false" json:"acakSoal"`
    Semester  string `json:"semester"`
}
type UjianSoal struct {
    Base
    UjianID  string `gorm:"uniqueIndex:ujian_soal_uniq" json:"ujianId"`
    SoalID   string `gorm:"uniqueIndex:ujian_soal_uniq" json:"soalId"`
    Bobot    float64 `json:"bobot"`
}
```

## D.4 Endpoint
- Bank soal (admin/tutor bare-api): CRUD `/bank-soal` (tutor scoped mapel yang dia ampu?).
- Ujian: `POST /ujian` (tutor, guard wali), CRUD `/ujian`.
- `POST /ujian/:id/soal` → pilih soal dari bank + bobot.
- `GET /ujian/:id/soal` (readAll/tutor) → daftar soal terpilih + bobot (untuk cetak
  naskah soal luring).
- Cetak naskah ujian: `GET /ujian/:id/print` (PDF gofpdf) — opsional, untuk ujian luring.

## D.5 RBAC & guard
- Tutor hanya kelas walinya (guard via `ujian.KelasID` → `canManageKelas`).
- `WaktuMulai`/`WaktuSelesai`/`DurasiMenit` jadi metadata jadwal ujian luring (tidak
  ada timer sisi-server).

## D.6 UI
- `pages/BankSoalView.tsx`: tabel soal + editor (pg/essay, opsi, kunci, poin).
- `pages/UjianView.tsx`: list ujian + FormCard (mapel, kelas, jadwal, durasi) +
  pemilih soal (checkbox dari bank) + tombol cetak naskah PDF (opsional).

## D.7 Aturan bisnis
- `AcakSoal` dipakai saat cetak naskah: urutan soal & opsi diacak per ujian (seed dari
  `ujianID`, deterministik) untuk menghasilkan varian naskah.
- Kunci jawaban (PG) & poin dipakai tutor untuk koreksi manual luring.

## D.8 Edge case
- Soal dihapus dari bank setelah dipakai ujian → soft: jangan hapus `BankSoal` yang
  terpakai `UjianSoal` (cek sebelum delete).

---

# MODUL E — Materi Pembelajaran

## E.1 Tujuan
Tutor berbagi materi/file antar staf; diskusi/komentar internal (tutor/admin). Sisi
siswa dihapus (tidak ada portal siswa).

## E.2 Aktor
- Tutor (upload per mapel+kelas walinya), Admin, Kepala.

## E.3 Model
```go
type Materi struct {
    Base
    MapelID   string `gorm:"index" json:"mapelId"`
    KelasID   string `gorm:"index" json:"kelasId"`
    Judul     string `gorm:"not null" json:"judul"`
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
    FilePath  string `gorm:"not null" json:"filePath"`   // relatif ke ./uploads/materi
    Tipe      string `json:"tipe"`     // ekstensi/MIME
    Ukuran    int64  `json:"ukuran"`
    Semester  string `json:"semester"`
    DibuatOlehUserID string `gorm:"index" json:"dibuatOlehUserId"`
}
type KomentarMateri struct {
    Base
    MateriID  string `gorm:"index" json:"materiId"`
    UserID    *string `gorm:"index" json:"userId"` // tutor/admin
    Isi       string `gorm:"type:text" json:"isi"`
}
```

## E.4 Endpoint
- Tutor (bare-api): `POST /materi` (upload multipart, guard wali), `PUT/DELETE`.
- `GET /materi?kelasId=&mapelId=` (readAll).
- `GET /materi/:id` (detail + komentar).
- `GET /materi/:id/download` (scoped tutor/admin, stream file).
- `POST /materi/:id/komentar` (tutor/admin).

## E.5 RBAC
- Akses/download tutor/admin (readAll lintas rombel untuk admin+kepala; guru scoped
  walinya via `canManageKelas`).
- Tutor hapus hanya materi miliknya (`DibuatOlehUserID`) atau admin.

## E.6 UI
- `pages/MateriView.tsx`: list + upload (drag-drop) + pilih mapel/kelas + unduh +
  kolom komentar internal.

## E.7 Edge case
- File dihapus dari disk tapi record ada → 404 dengan pesan jelas.
- MIME berbahaya (exe, dll) → tolak di whitelist upload.

---

# MODUL F — Kelas Virtual

## F.1 Tujuan
Jadwalkan sesi belajar online (link Zoom/Meet) untuk rombel — link dibagikan tutor
secara eksternal (tidak ada portal siswa).

## F.2 Aktor
- Tutor (buat per mapel+kelas), Admin, Kepala.

## F.3 Model
```go
type KelasVirtual struct {
    Base
    MapelID   string `gorm:"index" json:"mapelId"`
    KelasID   string `gorm:"index" json:"kelasId"`
    Judul     string `gorm:"not null" json:"judul"`
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
    LinkMeeting string `gorm:"not null" json:"linkMeeting"` // Zoom/Meet/Room
    WaktuMulai  time.Time `json:"waktuMulai"`
    WaktuSelesai time.Time `json:"waktuSelesai"`
    Semester  string `json:"semester"`
    DibuatOlehUserID string `gorm:"index" json:"dibuatOlehUserId"`
}
```

## F.4 Endpoint
- Tutor (bare-api): CRUD `/kelas-virtual` (guard wali).
- `GET /kelas-virtual?kelasId=` (readAll).

## F.5 RBAC
- Tutor hanya rombel walinya.

## F.6 UI
- `pages/KelasVirtualView.tsx`: list + form (link, jadwal) + tombol salin/buka link.

## F.7 Catatan
- Tidak perlu video server sendiri; cukup simpan link eksternal.
- Cron n8n reminder H-1/H-jam bisa dipakai untuk pengingat sesi.

---

# MODUL G — Catatan Perilaku / Kepribadian

## G.1 Tujuan
Catatan karakter/perilaku siswa (positif/negatif) untuk rapor & monitoring.

## G.2 Aktor
- Tutor (catat untuk siswa di rombelnya), Wali (khusus), Admin (semua), Kepala (lihat).

## G.3 Model
```go
type CatatanPerilaku struct {
    Base
    PesertaDidikID string `gorm:"index" json:"pesertaDidikId"`
    KelasID   string `gorm:"index" json:"kelasId"`
    Tanggal   time.Time `json:"tanggal"`
    Kategori  string `json:"kategori"` // "positif" | "negatif"
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
    DicatatOlehUserID string `gorm:"index" json:"dicatatOlehUserId"`
}
```

## G.4 Endpoint
- Tutor (bare-api): `POST /perilaku` (guard wali via `KelasID`), `GET /perilaku?kelasId=&pesertaDidikId=`.
- readAll: `GET /perilaku` lintas rombel (admin+kepala).

## G.5 RBAC
- Tutor catat hanya siswa di rombel walinya.

## G.6 UI
- `pages/PerilakuView.tsx`: pilih siswa → timeline catatan + form tambah.

## G.7 Integrasi
- Dimasukkan ke rapor (Modul I) sebagai "Catatan Kepribadian".

---

# MODUL H — Ijazah / Sertifikat + Cetak

## H.1 Tujuan
Cetak ijazah/sertifikat kelulusan dengan nomor unik + QR verifikasi.

## H.2 Aktor
- Admin (terbit & cetak), Kepala (lihat).

## H.3 Model
```go
type Sertifikat struct {
    Base
    PesertaDidikID string `gorm:"uniqueIndex" json:"pesertaDidikId"` // 1 siswa 1 sertifikat aktif
    ProgramID string `gorm:"index" json:"programId"` // Paket A/B/C (Modul O)
    Nomor     string `gorm:"uniqueIndex" json:"nomor"` // unik, format generated
    TanggalTerbit time.Time `json:"tanggalTerbit"`
    Status    string `json:"status"` // "draft" | "terbit"
    DiterbitkanOlehUserID string `gorm:"index" json:"diterbitkanOlehUserId"`
    PesertaDidik PesertaDidik `json:"pesertaDidik"`
    Program   Program `json:"program"`
}
```

## H.4 Endpoint
- Admin: `POST /sertifikat` (terbit, generate nomor), `GET /sertifikat` (readAll),
  `GET /sertifikat/:id/print` (PDF via gofpdf + go-qrcode QR verifikasi).
- Public verify: `GET /verify/sertifikat/:nomor` → status valid/tidak (tanpa auth,
  untuk pihak ketiga memverifikasi).

## H.5 RBAC
- Hanya admin terbit. Kepala lihat.
- Verify endpoint publik (no auth) — hanya return nomor/nama/program/status, **bukan** data sensitif.

## H.6 UI
- `pages/SertifikatView.tsx`: list + tombol "Terbit" per siswa lulus + "Cetak PDF".
- PDF: layout ijazah (kop, nama, NISN, program, nomor, tanggal, ttd kepala, QR link verify).

## H.7 Aturan bisnis
- Nomor format: `PKBM-<tahun>-<program>-<seq>` unik.
- 1 siswa 1 sertifikat aktif (uniqueIndex `PesertaDidikID`); bila perlu re-issue,
  soft-delete lama + buat baru (atau tambah field `versi`).

## H.8 Edge case
- Siswa belum lulus → tolak terbit.
- QR rusak di PDF → pastikan go-qrcode generate level error-correction M.

## H.9 Dependensi
- **`go-qrcode`** baru. **Modul O (Program)** harus ada dulu untuk field `ProgramID`.

---

# MODUL I — Rapor (Report Card) + Cetak

## I.1 Tujuan
Rapor formal per siswa: identitas + nilai akhir per mapel + kepribadian + kenaikan, cetak PDF.

## I.2 Aktor
- Admin (isi/cetak semua), Tutor (rombelnya), Kepala (lihat/cetak).

## I.3 Model
Tidak butuh tabel baru utama — rapor adalah **agregasi** data yang sudah ada:
- `PesertaDidik` (identitas), `Kelas`, `TahunAjaran`, `Nilai` (per mapel), `CatatanPerilaku` (G).
- Tambah `CatatanRapor` opsional:
```go
type CatatanRapor struct {
    Base
    PesertaDidikID string `gorm:"uniqueIndex:rapor_pd_ta_uniq" json:"pesertaDidikId"`
    TahunAjaranID  string `gorm:"uniqueIndex:rapor_pd_ta_uniq" json:"tahunAjaranId"`
    Semester       string `gorm:"uniqueIndex:rapor_pd_ta_uniq" json:"semester"`
    CatatanWali    string `gorm:"type:text" json:"catatanWali"`
    NaikKelas      *bool `json:"naikKelas"`
    KenaikanKe     *string `json:"kenaikanKe"`
}
```

## I.4 Endpoint
- `GET /rapor/:pesertaDidikId?tahunAjaranId=&semester=` (readAll + tutor guard wali via
  kelas siswa) → JSON rapor lengkap.
- `GET /rapor/:pesertaDidikId/print?...` (PDF gofpdf).
- `PUT /rapor/:pesertaDidikId/catatan` (admin/tutor guard) → simpan CatatanRapor.

## I.5 RBAC
- Tutor hanya siswa di rombel walinya.
- Admin/kepala semua (readAll).

## I.6 UI
- `pages/RaporView.tsx`: pilih siswa (atau cetak massal per rombel) → preview +
  tombol "Cetak PDF" + edit catatan wali.
- PDF: kop sekolah, identitas, tabel nilai (mapel, NP, NK, NA, predikat, keterangan),
  catatan kepribadian (dari G), catatan wali, kenaikan, ttd kepala + wali.

## I.7 Aturan bisnis
- Nilai akhir (NA) = sesuai pengaturan nilai (S) yang sudah ada.
- Cetak massal: loop per siswa, satu PDF multi-halaman.

## I.8 Edge case
- Siswa belum punya nilai lengkap → tampilkan "—" bukan 0.
- Rapor semester ganjil vs genap: pakai `semester` field.

---

# MODUL J — Pusat Laporan (Report Center)

## J.1 Tujuan
Satu halaman terpusat: pilih jenis laporan + filter + export Excel/PDF.

## J.2 Aktor
- Admin, Kepala (lihat + export), Tutor (rombelnya).

## J.3 Model
Tidak ada tabel baru. **Aggregator** yang memanggil handler export yang sudah ada
(nilai, presensi, rekap buku, siswa per pokjar, dll).

## J.4 Endpoint
- `GET /laporan/jenis` → daftar jenis laporan tersedia.
- `GET /laporan/export?jenis=&...&format=xlsx|pdf` → dispatch ke handler terkait
  (reuse `exportNilaiXLSX`, `exportPresensiPDF`, `exportBuku`, dll).

## J.5 RBAC
- readAll: admin+kepala semua; guru scoped rombel (pas `kelasId` walinya).

## J.6 UI
- `pages/LaporanView.tsx`: card pilih jenis → filter dinamis per jenis → tombol
  Export Excel/PDF (blob-download).

## J.7 Catatan
- Mendaftarkan jenis laporan = konfigurasi array di frontend + dispatch backend.
- Mencegah duplikasi: handler export lama tetap, pusat laporan hanya membungkus.

---

# MODUL K — Jurnal Mengajar + Approval + Foto

## K.1 Tujuan
Tutor mencatat kegiatan mengajar harian (dengan foto bukti); kepala approve/reject.

## K.2 Aktor
- Tutor (buat/edit miliknya), Kepala (approve/reject), Admin (semua).

## K.3 Model
```go
type JurnalMengajar struct {
    Base
    TutorID  string `gorm:"index" json:"tutorId"`
    MapelID  string `gorm:"index" json:"mapelId"`
    KelasID  string `gorm:"index" json:"kelasId"`
    Tanggal  time.Time `json:"tanggal"`
    Materi   string `gorm:"type:text" json:"materi"`
    Kegiatan string `gorm:"type:text" json:"kegiatan"`
    FotoPath *string `json:"fotoPath"` // bukti, opsional
    Status   string `json:"status"`   // "pending" | "disetujui" | "ditolak"
    CatatanReviewer string `gorm:"type:text" json:"catatanReviewer"`
    ReviewedBy *string `gorm:"index" json:"reviewedBy"`
    ReviewedAt *time.Time `json:"reviewedAt"`
}
```

## K.4 Endpoint
- Tutor (bare-api): `POST /jurnal` (guard wali via kelasId), `PUT/DELETE /jurnal/:id`
  (hanya miliknya & status pending).
- `GET /jurnal?tutorId=&kelasId=&status=` (readAll).
- Kepala (managementRead + writable): `POST /jurnal/:id/approve`,
  `POST /jurnal/:id/reject` (body catatan).
- `GET /jurnal/:id/foto` (scoped, stream).

## K.5 RBAC
- Tutor hanya lihat/edit miliknya (`TutorID == user.TutorID`).
- Approve/reject: kepala_sekolah & admin (bukan guru lain).
- Edit hanya saat `status=pending`.

## K.6 UI
- `pages/JurnalMengajarView.tsx` (tutor): list + form (mapel, kelas, tanggal, materi,
  kegiatan, upload foto). Status badge.
- `pages/JurnalApprovalView.tsx` (kepala/admin): tabel pending → approve/reject + catatan.

## K.7 Aturan bisnis
- Reject wajib catatan reviewer.
- Cron n8n reminder tutor belum isi jurnal minggu ini (opsional).

## K.8 Edge case
- Foto besar → kompres/limit 5 MB.
- Tutor diganti walinya → jurnal lama tetap milik tutor asli.

---

# MODUL L — Modul Pembelajaran + Outcomes

## L.1 Tujuan
Struktur kurikulum: Modul per mapel dengan capaian/outcomes. (Bukan "Modul Nilai".)

## L.2 Aktor
- Admin (CRUD master), Tutor (lihat + pakai), Kepala (lihat).

## L.3 Model
```go
type ModulBelajar struct {
    Base
    MapelID  string `gorm:"index" json:"mapelId"`
    Judul    string `gorm:"not null" json:"judul"`
    Urutan   int `json:"urutan"`
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
}
type CapaianModul struct {
    Base
    ModulID  string `gorm:"index" json:"modulId"`
    Kode     string `json:"kode"`     // mis. "CP-1"
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
}
```

## L.4 Endpoint
- Admin: CRUD `/modul-belajar`, CRUD `/modul-belajar/:id/outcomes`.
- readAll: `GET /modul-belajar?mapelId=`.
- Dapat dikaitkan ke Materi (E) & Tugas (C) via `modulId` opsional di tabel tsb.

## L.5 UI
- MasterData-style + nested outcomes editor (expand row → list outcomes).

## L.6 Catatan
- Tidak mengganggu "Modul Nilai" yang ada (penamaan berbeda: `ModulBelajar` vs
  struktur nilai lama).

---

# MODUL M — Kompetensi/Skill + Outcomes + Nilai

## M.1 Tujuan
Kelola kompetensi + outcomes; nilai keterampilan terstruktur per siswa. Memperkaya NK.

## M.2 Aktor
- Admin (CRUD), Tutor (nilai, rombelnya), Kepala (lihat).

## M.3 Model
```go
type Kompetensi struct {
    Base
    MapelID  string `gorm:"index" json:"mapelId"`
    Nama     string `gorm:"not null" json:"nama"`
}
type CapaianKompetensi struct {
    Base
    KompetensiID string `gorm:"index" json:"kompetensiId"`
    Kode     string `json:"kode"`
    Deskripsi string `gorm:"type:text" json:"deskripsi"`
}
type NilaiKompetensi struct {
    Base
    PesertaDidikID string `gorm:"index" json:"pesertaDidikId"`
    KompetensiID  string `gorm:"index" json:"kompetensiId"`
    KelasID   string `gorm:"index" json:"kelasId"`
    Semester  string `json:"semester"`
    Nilai     float64 `json:"nilai"`
    DicatatOlehUserID string `gorm:"index" json:"dicatatOlehUserId"`
}
// Relasi rombel↔kompetensi (skill per study-group)
type RombelKompetensi struct {
    Base
    KelasID  string `gorm:"uniqueIndex:rombel_komp_uniq" json:"kelasId"`
    KompetensiID string `gorm:"uniqueIndex:rombel_komp_uniq" json:"kompetensiId"`
}
```

## M.4 Endpoint
- Admin: CRUD `/kompetensi`, outcomes, `rombel-kompetensi`.
- Tutor (bare-api): `POST /nilai-kompetensi` (guard wali, bulk per rombel).
- readAll: `GET /nilai-kompetensi?kelasId=&semester=`.
- Integrasi: rata-rata NilaiKompetensi bisa jadi sumber NK (Modul Nilai) bila dipetakan (S).

## M.5 UI
- MasterData untuk kompetensi + outcomes.
- `pages/NilaiKompetensiView.tsx`: matrix siswa×kompetensi (mirip Peminjaman matrix).

## M.6 Catatan
- **Tidak menggantikan** NK lama; jadi sumber tambahan. Bila dipakai, pengaturan nilai (S)
  yang memilih sumber.

---

# MODUL N — Fase Kurikulum Merdeka (A–E)

## N.1 Tujuan
Mendukung fase Kurikulum Merdeka (A–E) yang berkorelasi dengan jenjang/Paket.

## N.2 Aktor
- Admin (CRUD master fase + relasi).

## N.3 Model
```go
type Fase struct {
    Base
    Kode     string `gorm:"uniqueIndex" json:"kode"` // "A".."E"
    Nama     string `json:"nama"`
    JenjangSetara string `json:"jenjangSetara"` // mis. "1-2" untuk Fase A
}
```
Relasi: `Kelas.FaseID *string` (opsional — jangan ganggu `Jenjang` lama).

## N.4 Endpoint
- Admin: CRUD `/fase`.
- `GET /fase` (readAll).
- `Kelas` update: tambah field `FaseID` (opsional).

## N.5 Aturan
- Fase opsional; bila kosong, sistem pakai jenjang lama (backward compatible).
- **Keputusan bisnis**: apakah jenjang 1–6 diganti Paket/Fase? Bila ya, ini
  **migrasi data** besar — pisahkan jadi tahap tersendiri, jangan dipaksakan di sini.

## N.6 UI
- MasterData `fase` (schema sederhana). Dropdown fase di editor Kelas.

---

# MODUL O — Program Pendidikan (Paket A/B/C)

## O.1 Tujuan
PKBM menjalankan Program Paket A (setara SD), B (SMP), C (SMA). Penting untuk
keabsahan ijazah (H) & rapor (I).

## O.2 Aktor
- Admin (CRUD master program).

## O.3 Model
```go
type Program struct {
    Base
    Kode     string `gorm:"uniqueIndex" json:"kode"` // "A" | "B" | "C"
    Nama     string `gorm:"not null" json:"nama"`    // "Paket A"
    JenjangSetara string `json:"jenjangSetara"` // "SD" | "SMP" | "SMA"
    Keterangan string `gorm:"type:text" json:"keterangan"`
}
```
Relasi: `Kelas.ProgramID *string`, `PesertaDidik.ProgramID *string` (opsional).

## O.4 Endpoint
- Admin: CRUD `/program`.
- `GET /program` (readAll, dipakai dropdown kelas/siswa/sertifikat).

## O.5 Aturan
- Opsional di kelas/siswa; bila kosong, default Paket A (atau pakai jenjang lama).
- **Prasyarat Modul H** (sertifikat) & **Modul I** (rapor) memakai program.

## O.6 UI
- MasterData `program`. Dropdown di editor Kelas & Siswa.

## O.7 Catatan keputusan
- Bila user tetap dengan jenjang 1–6 (model SD), program bisa tetap ditambah sebagai
  metadata ("Paket A") tanpa mengubah jenjang. **Konfirmasi user** sebelum migrasi
  besar jenjang → paket.

---

# MODUL P — Kartu Pelajar + Cetak

## P.1 Tujuan
Cetak kartu pelajar (ID card) per siswa & massal per rombel, dengan QR verifikasi.

## P.2 Aktor
- Admin (cetak), Kepala (cetak), Tutor (cetak rombelnya).

## P.3 Model
Tidak perlu tabel baru — kartu = cetak data `PesertaDidik` + `Kelas` + `TahunAjaran`.
QR berisi URL verify `https://<domain>/verify/siswa/<nisn>` (publik, return nama/kelas/aktif).

## P.4 Endpoint
- `GET /kartu-pelajar/:pesertaDidikId/print` (PDF, guard wali/role).
- `GET /kartu-pelajar/group/:kelasId/print` (PDF multi-kartu per rombel).
- Public: `GET /verify/siswa/:nisn` → `{nama, kelas, status, tahunAjaran}` (no auth).

## P.5 RBAC
- Tutor cetak hanya rombelnya. Admin/kepala semua. Verify publik.

## P.6 UI
- `pages/KartuPelajarView.tsx`: pilih rombel → preview grid kartu → cetak massal,
  atau per siswa.

## P.7 PDF
- Layout ID card (depan: foto, nama, NISN, kelas, tahun ajaran, QR; belakang: kop,
  alamat, tanda tangan kepala). gofpdf + go-qrcode.
- Bila foto siswa belum ada → placeholder.

## P.8 Dependensi
- **`go-qrcode`** baru. Bila foto siswa belum ada di master, perlu tambah field
  `FotoPath *string` di `PesertaDidik` + upload (bisa pakai upload umum E.7).

---

# MODUL R — Import Data Terpusat + Template

## R.1 Tujuan
Pusat import multi-tipe (tutor, mapel, kelas, nilai, siswa) dengan unduh template +
laporan error per baris.

## R.2 Aktor
- Admin (import semua), Tutor (import nilai rombelnya).

## R.3 Model
```go
type ImportLog struct {
    Base
    Tipe     string `json:"tipe"`     // "siswa" | "tutor" | "mapel" | "kelas" | "nilai"
    FileName string `json:"fileName"`
    TotalBaris int `json:"totalBaris"`
    Berhasil  int `json:"berhasil"`
    Gagal     int `json:"gagal"`
    ErrorJson string `gorm:"type:text" json:"errorJson"` // [{baris,msg}]
    Status    string `json:"status"` // "proses" | "selesai" | "gagal"
    UserID    string `gorm:"index" json:"userId"`
}
```

## R.4 Endpoint
- `GET /import/template/:tipe` (admin) → XLSX template per tipe (excelize).
- `POST /import` (admin/tutor) multipart → parse XLSX, validasi per baris, insert
  via transaction batch, simpan ImportLog + errorJson.

## R.5 RBAC
- Tutor hanya import nilai untuk rombelnya (guard wali via kelasId di baris).
- Admin semua tipe.

## R.6 UI
- `pages/ImportView.tsx`: pilih tipe → unduh template → upload → tampilkan ringkasan
  (berhasil/gagal) + tabel error per baris + riwayat import (ImportLog).

## R.7 Aturan
- Transaksi per batch (mis. 100 baris) untuk hindari rollback besar.
- Validasi: FK wajib (kelasId/mapelId ada), duplikat (NISN unik), tipe data.
- Bila 1 baris gagal → skip + catat, lanjut baris berikutnya (partial success).

## R.8 Catatan
- Reuse `importSiswa` yang sudah ada sebagai salah satu tipe; generalisasi ke framework
  import (interface per tipe: `Validate(row)`, `Insert(rows)`).

---

# MODUL S — Pengaturan Sumber Nilai (Grade Sources)

## S.1 Tujuan
Konfigurasi sumber/komponen nilai (UM, Tugas, Ujian, Praktik) + bobot per mapel,
dipakai oleh Modul Nilai untuk hitung NA.

## S.2 Aktor
- Admin (CRUD master sumber + bobot global), Tutor (lihat), Kepala (lihat).

## S.3 Model
```go
type SumberNilai struct {
    Base
    Kode     string `gorm:"uniqueIndex" json:"kode"` // "UM" | "TUGAS" | "UJIAN" | "PRAKTIK"
    Nama     string `gorm:"not null" json:"nama"`
    BolehDipakai bool `gorm:"default:true" json:"bolehDipakai"`
}
type BobotSumberNilai struct {
    Base
    MapelID  string `gorm:"uniqueIndex:bobot_uniq" json:"mapelId"`
    SumberID string `gorm:"uniqueIndex:bobot_uniq" json:"sumberId"`
    Bobot    float64 `json:"bobot"` // 0-100
}
```

## S.4 Endpoint
- Admin: CRUD `/sumber-nilai`, CRUD `/bobot-sumber-nilai`.
- readAll: `GET /sumber-nilai`, `GET /bobot-sumber-nilai?mapelId=`.
- Modul Nilai: saat hitung NA, baca bobot per mapel; bila tidak ada → fallback
  konfigurasi lama (backward compatible).

## S.5 RBAC
- Admin only untuk ubah. Tutor/kepala read.

## S.6 UI
- `pages/SumberNilaiView.tsx`: master sumber + matrix bobot per mapel (mapel×sumber).

## S.7 Aturan
- Total bobot per mapel idealnya 100; bila ≠ 100 → warning (bukan error) & normalisasi.
- Sumber terkait: TUGAS (Modul C), UJIAN (Modul D), PRAKTIK (Modul M Kompetensi).

## S.8 Catatan
- **Cek dulu** apakah `PengaturanNilai` yang ada sudah mencakup ini. Bila ya, modul
  ini cukup jadi generalisasi/penyempurnaan, bukan tabel baru. Konfirmasi user.

---

# Ringkasan dependensi & urutan

```
O (Program) ──┬──> H (Sertifikat)
              └──> I (Rapor) [field program]

G (Perilaku) ──> I (Rapor) [catatan kepribadian]
L (Modul Belajar) ──> E/C [relasi opsional]
M (Kompetensi) ──> S (Sumber Nilai) ──> Modul Nilai
N (Fase) [opsional, keputusan bisnis]
K (Jurnal Mengajar) [mandiri, butuh go upload foto]
J (Pusat Laporan) [agregator, setelah modul bertambah]
R (Import) [agregator, generalisasi import siswa]
P (Kartu Pelajar) [butuh go-qrcode + foto siswa]
S (Sumber Nilai) [cek dulu pengaturan-nilai]
B (Pengumuman), C (Tugas), D (Ujian+Bank Soal), E (Materi), F (Kelas Virtual)
  [mandiri, sisi tutor/admin]
```

## Urutan rekomendasi (tahapan)
1. **Tahap 14** — B Pengumuman + K Jurnal Mengajar (mandiri, cepat berdampak).
2. **Tahap 15** — C Tugas + E Materi + F Kelas Virtual (KBM, sisi tutor/admin).
3. **Tahap 16** — D Ujian + Bank Soal (penjadwalan + pemilihan soal; tanpa pengerjaan
   online).
4. **Tahap 17** — O Program + N Fase (keputusan bisnis) + H Sertifikat + P Kartu
   Pelajar (administratif, butuh go-qrcode + foto siswa).
5. **Tahap 18** — I Rapor + G Perilaku + S Sumber Nilai (rapor formal & penyempurnaan
   nilai).
6. **Tahap 19** — L Modul Belajar + M Kompetensi (pengayaan kurikulum).
7. **Tahap 20** — J Pusat Laporan + R Import terpusat (konsolidasi).

## Dependensi baru (go.mod / npm)
- `github.com/skip2/go-qrcode` — backend (H, P).
- Upload file: tidak perlu dep baru (stdlib Fiber multipart + `os`).

## Catatan akhir
- **Portal/akun siswa dihapus dari cakupan** (Modul A & Q); modul B/C/D/E/F/I direduksi
  menjadi sisi tutor/admin. Endpoint publik `/verify/*` (H, P) tetap.
- Setiap modul di atas **mandiri & backward compatible**: tidak mengubah modul yang
  sudah berjalan (Presensi, Nilai, Buku) kecuali dinyatakan eksplisit (mis. S
  menyentuh hitung NA — dengan fallback ke konfigurasi lama).
- **Keputusan bisnis yang menunggu konfirmasi user**: N (ganti jenjang→Fase?) &
  O (Paket A/B/C vs jenjang 1–6). Keduanya menyentuh data historis; jangan
  dipaksakan tanpa migrasi terencana.
- Dokumen ini adalah **rancangan (PRD + desain)**; tiap tahap tetap perlu plan
  implementasi terpisah sebelum coding, mengikuti konvensi di Bagian 0.