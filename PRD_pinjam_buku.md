# PRD — Modul Peminjaman & Pengembalian Buku Modul
## PKBM Tunas Ilmu (Modul di Dalam Aplikasi Utama — Lampiran PRD Utama)

**Versi:** 4.0
**Tanggal:** 3 Agustus 2026
**Status:** Draft — Terintegrasi Penuh dengan PRD Utama (`PRD.md`), Semua Item Terbuka Sudah Diputuskan

---

## 0. Perubahan dari Draf Sebelumnya

**v1.1 → v2.0:** diubah dari aplikasi terpisah (Next.js + Prisma + SQLite standalone, auth PIN sendiri, tabel data sendiri) menjadi modul di dalam aplikasi utama (Go Fiber + React, §2 PRD Utama) — data siswa/kelas/guru/tahun ajaran reuse langsung, tidak disinkronkan 2 arah.

**v2.0 → v3.0:** menyelaraskan format & detail dengan `PRD.md` utama — model data ditulis ulang jadi format tabel Field/Tipe/Keterangan, ditambah field `semester` di `BukuKelas`/`Peminjaman`, snapshot `kelas_id` di `Peminjaman`, penomoran bagian disambung eksplisit dari PRD Utama, crosswalk keamanan ke §6.3/§6.5.

**v3.0 → v4.0 (revisi ini) — seluruh 4 item "Perlu Dikonfirmasi" sudah diputuskan:**
- **PIN cepat guru: ditiadakan.** Login guru murni pakai sesi JWT + Turnstile yang sama dengan modul lain, tanpa lapisan PIN tambahan. Semua referensi PIN dihapus dari dokumen.
- **Generate PDF: pakai teknologi yang sama dengan PRD Utama** — backend Go, lewat `utils/` (§2.4 PRD Utama menyebut eksplisit `utils/ # bcrypt, jwt helper, excel/pdf export`), bukan library terpisah di frontend. Berlaku konsisten untuk presensi, rapor, dan rekap buku.
- **`PenugasanPinjamBuku` sebagai tabel terpisah: dihapus.** Karena 1 tutor umumnya mengampu 1 kelas, akses mencatat pinjam-buku cukup mengikuti `wali_kelas_id` (§4.5 PRD Utama) langsung — tanpa tabel penugasan tambahan. Ini menyederhanakan skema (5 tabel baru → 4 tabel baru) dan alur guru (tidak perlu langkah "kelas mana yang ditugaskan tambahan").
- **Titik potong tanggal Semester Ganjil/Genap: dibuatkan fiturnya.** Ditambahkan field baru `tanggal_mulai_semester_genap` pada Tahun Ajaran (usulan perluasan §4.4 PRD Utama, lihat §4.2 di dokumen ini) — semester sekarang **dihitung otomatis dari tanggal**, bukan dipilih manual, konsisten dengan cara §4.12 PRD Utama menghitung semester presensi.

Penomoran bagian data model baru menjadi **§4.18–§4.21** (sebelumnya §4.18–4.22, berkurang 1 karena `PenugasanPinjamBuku` dihapus).

---

## 1. Tech Stack

Modul ini memakai **stack yang sama persis** dengan aplikasi utama — tidak ada dependency baru. Lihat §2 PRD Utama untuk detail lengkap:

| Kebutuhan Modul Buku | Dipenuhi Oleh (sudah ada di §2 PRD Utama) |
|---|---|
| Backend & DB | Go Fiber v2 + GORM, dual-driver SQLite (dev) / PostgreSQL (`DATABASE_URL`, prod) — §2.2, §2.3 |
| Auth & Role | JWT (access + refresh httpOnly cookie) + Cloudflare Turnstile + role guard — §2.2, §6 |
| Frontend | React 19 + Vite + TS + Tailwind v4 + shadcn/ui — §2.1 |
| Import/Export Excel | SheetJS/XLSX (sudah dipakai untuk import Peserta Didik, §4.10) — §2.1 |
| Tanda tangan jari | React Signature Canvas (sudah dipakai modul Presensi, §4.12) — §2.1 |
| Notifikasi terjadwal → n8n | Job cron `robfig/cron/v3`, timezone `Asia/Jakarta` (sudah dipakai auto-generate presensi, §4.13) — §2.2 |
| **Export PDF** | **Backend Go**, lewat `utils/` (§2.4 PRD Utama — folder yang sama dipakai untuk export presensi/rapor) — library spesifik (mis. `gofpdf`/`maroto`) mengikuti keputusan yang dipakai modul presensi/rapor, satu pendekatan untuk seluruh aplikasi |

## 2. Latar Belakang & Tujuan

PKBM Tunas Ilmu meminjamkan buku modul kepada peserta didik setiap semester. Saat ini proses pencatatan peminjaman dan pengembalian buku dilakukan manual, rawan kehilangan data dan sulit direkap. Modul ini mendigitalkan proses tsb — penetapan buku per kelas per semester, pencatatan peminjaman oleh guru, hingga pencatatan pengembalian beserta kondisi buku — dengan memanfaatkan langsung data kelas, peserta didik, dan guru yang sudah ada di aplikasi utama.

Tujuan:
- Admin mengelola master data buku & penetapan buku per kelas per semester tanpa input ulang data kelas/siswa.
- Guru mencatat peminjaman & pengembalian buku per siswa secara digital, dengan tanda tangan jari sebagai bukti sah.
- Rekap peminjaman/pengembalian yang bisa diunduh (Excel/PDF) sebagai bukti fisik.
- Melacak kondisi buku saat dikembalikan (baik, rusak ringan, rusak berat, hilang).

## 3. Peran & Hak Akses

Mengikuti §3 PRD Utama — **tidak ada role atau mekanisme login baru**.

| Role | Akses Modul Buku |
|---|---|
| **Admin** | CRUD penuh: master buku, penetapan buku per kelas per semester, akses penuh ke seluruh rekap. |
| **Kepala Sekolah** | Read-only + download (Excel/PDF) ke seluruh rekap peminjaman/pengembalian, semua kelas — di-enforce oleh middleware yang sama yang menolak `POST/PUT/PATCH/DELETE` (§6.3 PRD Utama). |
| **Guru** | Input peminjaman & pengembalian buku, terbatas pada rombel di mana ia terdaftar sebagai wali kelas (`wali_kelas_id`, §4.5 PRD Utama) — lewat akun login yang sama seperti modul lain (username/password + Turnstile). |

Login guru pakai **akun yang sama** dengan modul lain: 1 akun bisa jadi wali kelas di 1 rombel (§4.5 PRD Utama) sekaligus guru mapel di rombel lain (§4.9 PRD Utama) — semua lewat 1 sesi login. Akses mencatat pinjam-buku otomatis mengikuti status wali kelas, tanpa tabel penugasan atau lapisan PIN tambahan.

## 4. Data Model

### 4.1 Data yang Dipakai Langsung dari Aplikasi Utama (Tidak Dibuat Ulang)
Modul ini **tidak** punya CRUD/tabel sendiri untuk hal-hal berikut — cukup relasi FK ke tabel yang sudah ada:
- Tahun Ajaran (§4.4 PRD Utama, dengan 1 field tambahan — lihat §4.2 di bawah).
- Kelas / Rombel (§4.5 PRD Utama).
- Peserta Didik (§4.10 PRD Utama), termasuk riwayat kelas/kenaikan kelas (§4.11) — kalau siswa naik/pindah kelas, data peminjaman lama tetap konsisten lewat snapshot `kelas_id` di tabel Peminjaman (§4.4).
- Guru/Tutor & Akun login (§4.1 & §4.17 PRD Utama).

### 4.2a Usulan Tambahan Field pada Tahun Ajaran (Perluasan §4.4 PRD Utama)

Supaya semester (Ganjil/Genap) bisa **dihitung otomatis dari tanggal** — bukan cuma disebut "dihitung otomatis" tanpa aturan jelas seperti di §4.12 PRD Utama — ditambahkan 1 field baru ke tabel Tahun Ajaran:

| Field | Tipe | Keterangan |
|---|---|---|
| tanggal_mulai_semester_genap | date | **field baru** — tanggal mulai Semester Genap dalam tahun ajaran ini |

Aturan pembagian: **Semester Ganjil** = `tanggal_mulai` s.d. 1 hari sebelum `tanggal_mulai_semester_genap`; **Semester Genap** = `tanggal_mulai_semester_genap` s.d. `tanggal_selesai`. Admin mengisi field ini saat membuat/mengedit Tahun Ajaran (halaman CRUD yang sama, §5.2 PRD Utama) — tidak perlu halaman baru. Field ini dipakai bersama oleh Presensi (§4.12 PRD Utama) dan modul Buku (§4.3–4.4 di dokumen ini) sebagai satu sumber kebenaran titik potong semester.

Tabel baru khusus modul buku melanjutkan penomoran data model PRD Utama sebagai **§4.18–§4.21**.

### 4.18 Buku (Master Data)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| judul | string | wajib |
| kode_buku | string | opsional |
| penerbit | string | opsional |
| created_at / updated_at | timestamp | |

Tidak ada field stok/jumlah eksemplar — sistem murni mencatat siapa pinjam apa, bukan manajemen inventaris (lihat §8 Di Luar Cakupan).

### 4.19 Buku ↔ Kelas (Pivot) — Penetapan Buku Wajib per Kelas per Semester
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| kelas_id | FK → kelas (§4.5 PRD Utama) | |
| buku_id | FK → buku (§4.18) | |
| semester | enum(Ganjil/Genap) | **dihitung otomatis** dari tanggal hari ini vs `tanggal_mulai_semester_genap` tahun ajaran aktif (§4.2a) saat admin menetapkan buku — sistem otomatis mengisi semester berjalan, admin tidak perlu pilih manual |
| created_at | timestamp | |

Kombinasi (`kelas_id`, `buku_id`, `semester`) harus unik. Daftar ini jadi acuan checklist yang muncul di form peminjaman guru untuk kelas & semester terkait. Seorang siswa bisa meminjam lebih dari satu judul buku.

### 4.20 Peminjaman
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| peserta_didik_id | FK → peserta_didik (§4.10 PRD Utama) | |
| buku_id | FK → buku (§4.18) | |
| kelas_id | FK → kelas (§4.5 PRD Utama) | **snapshot** kelas siswa pada saat transaksi dicatat — menjaga rekap per kelas tetap akurat meski siswa kemudian naik/pindah kelas (§4.11 PRD Utama) |
| semester | enum(Ganjil/Genap) | **dihitung otomatis** dari `tanggal_pinjam` vs `tanggal_mulai_semester_genap` tahun ajaran aktif (§4.2a) — sama seperti §4.12 PRD Utama menghitung semester presensi, guru tidak perlu pilih manual |
| tanggal_pinjam | date | |
| status | enum(Dipinjam/Dikembalikan) | |
| dicatat_oleh_user_id | FK → users (§4.17 PRD Utama) | |
| tanda_tangan | text (base64 image) | wajib, dari signature canvas |
| created_at / updated_at | timestamp | |

### 4.21 Pengembalian
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| peminjaman_id | FK → peminjaman (§4.20) | |
| tanggal_kembali | date | |
| kondisi_buku | enum(Baik/Rusak Ringan/Rusak Berat/Hilang) | |
| catatan | text | opsional; disarankan wajib diisi di sisi UX kalau kondisi Rusak Berat/Hilang (mis. rencana penggantian, info ke orang tua) — murni informasi, tanpa modul denda/pembayaran otomatis (§8) |
| dicatat_oleh_user_id | FK → users (§4.17 PRD Utama) | |
| tanda_tangan | text (base64 image) | wajib, dari signature canvas |
| created_at / updated_at | timestamp | |

**Dihapus dari draf sebelumnya:** `TahunAjaran`, `Semester` (tabel tersendiri), `Kelas`, `PesertaDidik`, `GuruPin`, `GuruPinKelas`, `AkunAdmin`, `AkunKepalaSekolah` (duplikat PRD Utama), dan `PenugasanPinjamBuku` (v3.0 → v4.0, digantikan `wali_kelas_id` langsung).

## 5. Modul & Fitur (§5.8 PRD Utama)

### 5.8 Peminjaman & Pengembalian Buku
- Reuse penuh data Tahun Ajaran (+1 field baru §4.2a), Kelas, Peserta Didik, Tutor, dan Users dari PRD Utama — hanya §4.18–4.21 yang baru.
- **Admin**: CRUD Buku (§4.18), Penetapan Buku per Kelas per Semester (§4.19, semester terisi otomatis).
- **Guru**: setelah login, pilih kelas dari rombel yang diampu sebagai wali kelas (`wali_kelas_id`, §4.5 PRD Utama) → sistem otomatis menampilkan semester berjalan (§4.2a) → form **Peminjaman** (checklist siswa × buku sesuai §4.19 + tanda tangan jari, reuse komponen signature canvas dari modul Presensi §4.12) atau form **Pengembalian** (checklist buku yang sedang dipinjam + kondisi buku per buku + tanda tangan). Simpan → hasil bisa diunduh sebagai PDF (generate di backend Go, §1).
- Kondisi Rusak Berat/Hilang memunculkan kolom catatan bebas — murni informasi, tanpa modul denda/pembayaran (§8, di luar cakupan v1).
- **Rekap** per kelas per tahun ajaran/semester untuk Admin & Kepala Sekolah (read-only + download sesuai §3), export Excel & PDF.
- Riwayat tahun ajaran/semester lampau otomatis mengikuti navigasi halaman **Arsip** (Tahun Ajaran → Semester) yang sudah ada di §5.3 PRD Utama — tidak perlu mekanisme arsip terpisah.
- **Reminder n8n**: job cron Go (`robfig/cron`, scheduler sama dengan §4.13 PRD Utama tapi job terpisah) mengirim webhook ke n8n (env var `N8N_WEBHOOK_URL`, payload daftar siswa/kelas/buku belum kembali) pada jadwal tertentu menjelang akhir semester (mis. H-14/H-7/H-1) atau berdasar `tanggal_selesai`/`tanggal_mulai_semester_genap` tahun ajaran aktif (§4.2a); n8n meneruskan ke saluran notifikasi (WhatsApp/email dsb, di luar cakupan aplikasi ini).

## 6. Keamanan (Tambahan §6.7 PRD Utama)

Modul ini tidak menambah mekanisme keamanan baru — sepenuhnya reuse §6 PRD Utama, dengan 2 poin yang perlu eksplisit diterapkan ke endpoint modul buku:

### 6.7 Guard IDOR & Audit Log — Modul Buku
- **Guard kepemilikan kelas** (pola §6.3): endpoint simpan Peminjaman/Pengembalian wajib memverifikasi `kelas_id` yang diakses memang rombel di mana user yang login adalah `wali_kelas_id`-nya (§4.5) — sebelum request diproses. Guru tidak boleh bisa mencatat transaksi untuk kelas di luar rombel yang diampunya.
- **Audit log** (pola §6.5): pembuatan/perubahan Peminjaman & Pengembalian dicatat ke tabel `audit_log` yang sama (user_id, waktu, aksi), selain field `dicatat_oleh_user_id` bawaan tabel — konsisten dengan pencatatan aktivitas sensitif modul lain (login, perubahan nilai, export data).
- Endpoint export (Excel/PDF) rekap buku mengikuti aturan Kepala Sekolah read-only yang sama (§6.3): hanya `GET`/download, tanpa akses tulis.

## 7. Alur Implementasi — Tahap 13 PRD Utama

Melanjutkan §7 PRD Utama (Tahap 1–12), modul ini ditambahkan sebagai:

**Tahap 13 — Modul Peminjaman & Pengembalian Buku**
- Tambah field `tanggal_mulai_semester_genap` ke model `TahunAjaran` (§4.2a) — migrasi kolom baru ke tabel yang sudah ada.
- Tambah model GORM baru: `Buku`, `BukuKelas`, `Peminjaman`, `Pengembalian` (§4.18–4.21) ke schema yang sudah berjalan; AutoMigrate menyusul tabel existing, tanpa setup project/database dari nol.
- Backend: handler & routes API baru — CRUD Buku/BukuKelas, endpoint simpan Peminjaman & Pengembalian (dengan guard IDOR §6.7 berbasis `wali_kelas_id`, dan logika hitung `semester` otomatis dari tanggal), endpoint rekap per kelas/tahun ajaran/semester, endpoint export PDF (reuse pendekatan backend Go dari modul presensi/rapor, §1) — pakai middleware role-guard JWT yang sudah ada (§2.2, §6.3 PRD Utama).
- Frontend: halaman Admin (Buku, BukuKelas), halaman Guru (pilih kelas dari rombel yang diampu → form Peminjaman/Pengembalian + signature canvas, semester tampil otomatis), halaman Rekap (Admin & Kepala Sekolah) — mengikuti struktur folder & komponen shadcn/ui yang sudah ada (§2.4 PRD Utama).
- Export: reuse util Excel (SheetJS) dan util PDF backend Go yang sama dengan modul presensi/rapor.
- Integrasi n8n: job cron Go baru (scheduler sama dengan §4.13, job terpisah dari auto-generate presensi) + endpoint pengirim payload webhook (`N8N_WEBHOOK_URL`).
- Testing & polish fokus di device tablet/HP untuk form guru & signature pad, sesuai kebutuhan lapangan (diisi langsung di kelas).

## 8. Di Luar Cakupan (Out of Scope) — v1

- Manajemen inventaris buku secara detail (pengadaan, pembelian, stok/eksemplar).
- Sistem denda/pembayaran otomatis atas buku rusak/hilang.
- Aplikasi mobile native (tetap web responsif, bagian dari app utama).
- PIN cepat guru (ditiadakan, §0).
- Tabel penugasan pencatat pinjam-buku terpisah dari wali kelas (ditiadakan, §0).

## 9. Keputusan yang Sudah Dikonfirmasi

- PIN cepat guru tidak dipakai — cukup sesi JWT + Turnstile yang sama dengan modul lain.
- Generate PDF pakai backend Go (`utils/`, §2.4 PRD Utama), satu pendekatan konsisten untuk presensi, rapor, dan rekap buku.
- Akses mencatat pinjam-buku mengikuti `wali_kelas_id` (§4.5 PRD Utama) langsung, tanpa tabel penugasan terpisah.
- Semester (Ganjil/Genap) dihitung otomatis dari tanggal transaksi vs field baru `tanggal_mulai_semester_genap` pada Tahun Ajaran (§4.2a), bukan dipilih manual.

Tidak ada lagi item terbuka untuk modul ini — seluruh poin di "Perlu Dikonfirmasi" versi sebelumnya sudah diputuskan di atas.
