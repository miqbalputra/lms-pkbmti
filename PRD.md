# PRD — Sistem Informasi LMS PKBM Tunas Ilmu

## 1. Latar Belakang & Tujuan

PKBM Tunas Ilmu membutuhkan sistem informasi manajemen sekolah (LMS) untuk mengelola data tutor, orang tua, peserta didik, kelompok belajar (pokjar), presensi pertemuan, nilai, hingga penerbitan rapor. Kegiatan Belajar Mengajar (KBM) berlangsung setiap hari Sabtu di 4 pokjar (1 pusat + 3 binaan/cabang).

Tujuan aplikasi:
- Mendigitalkan pencatatan data tutor, orang tua, peserta didik, dan pokjar.
- Mendigitalkan presensi pertemuan mingguan (Sabtu) dengan tanda tangan tutor.
- Mendigitalkan pencatatan nilai dan penerbitan rapor.
- Menyediakan akses berjenjang: Admin, Kepala Sekolah, dan Guru (per kelas).

## 2. Tech Stack

### 2.1 Frontend
- TypeScript 5.7 + Node.js, bundler Vite 6.1
- React 19 (`react`, `react-dom` v19.0.0)
- React Router DOM v7
- Axios (REST API + JWT Bearer token)
- Tailwind CSS v4 (`@tailwindcss/vite` 4.0.6) dengan `@theme inline`
- shadcn/ui (OKLCH color space, semantic design tokens: `--background`, `--foreground`, `--primary`, `--card`, `--muted`, `--border`, dll.)
- Lucide React (icon)
- Recharts v2.15.1 (grafik/statistik)
- Sonner v2.0.1 (toast/notifikasi)
- SheetJS/XLSX v0.18.5 (import/export Excel)
- React Signature Canvas / HTML5 Canvas (tanda tangan jari tutor)
- `clsx` & `tailwind-merge`
- **Cloudflare Turnstile** (`react-turnstile` atau embed script resmi) — widget CAPTCHA di halaman login

### 2.2 Backend
- Go (Golang) v1.22
- Go Fiber v2 (`github.com/gofiber/fiber/v2`)
- GORM (`gorm.io/gorm` v1.25.11)
- JWT: `github.com/golang-jwt/jwt/v5`
- Password hashing: `golang.org/x/crypto/bcrypt`
- Rate limiting: `github.com/gofiber/fiber/v2/middleware/limiter`
- Security headers: `github.com/gofiber/fiber/v2/middleware/helmet`
- CORS: `github.com/gofiber/fiber/v2/middleware/cors` (whitelist origin, bukan wildcard `*`)
- Request logging: `github.com/gofiber/fiber/v2/middleware/logger`
- Validasi input: `github.com/go-playground/validator/v10`
- Verifikasi Cloudflare Turnstile: HTTP POST server-side ke `https://challenges.cloudflare.com/turnstile/v0/siteverify` (tanpa SDK tambahan, cukup `net/http`)
- Scheduler/cron: `github.com/robfig/cron/v3` — untuk job auto-generate jadwal presensi mingguan, dijalankan dengan zona waktu `Asia/Jakarta` (WIB) via `time.LoadLocation("Asia/Jakarta")`

### 2.3 Database (Dual-Driver)
- **Development (lokal):** SQLite CGO-free (`github.com/glebarez/sqlite` v1.11.0 / `modernc.org/sqlite`)
- **Production (VPS):** PostgreSQL (`gorm.io/driver/postgres` + `pgx/v5`), aktif otomatis saat env var `DATABASE_URL` terdeteksi

### 2.4 Struktur Proyek (disarankan)
```
pkbm-lms/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/       # koneksi DB dual-driver, env
│   │   ├── models/       # GORM models
│   │   ├── handlers/     # Fiber handlers per modul
│   │   ├── middleware/   # JWT auth, role guard
│   │   ├── routes/
│   │   └── utils/        # bcrypt, jwt helper, excel/pdf export
│   ├── migrations/
│   └── go.mod
└── frontend/
    ├── src/
    │   ├── components/
    │   │   ├── ui/        # shadcn/ui components
    │   │   └── shared/
    │   ├── pages/
    │   ├── layouts/
    │   ├── routes/
    │   ├── lib/            # axios instance, utils
    │   ├── hooks/
    │   └── types/
    └── package.json
```

### 2.5 Environment Variables (Wajib, Jangan Hardcode)

**Backend (`.env`, tidak di-commit ke Git):**
```
DATABASE_URL=                # kosong = pakai SQLite lokal, terisi = pakai PostgreSQL
JWT_ACCESS_SECRET=           # secret random panjang (min 32 karakter), khusus access token
JWT_REFRESH_SECRET=          # secret random terpisah, khusus refresh token
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=7d
TURNSTILE_SECRET_KEY=        # dari dashboard Cloudflare Turnstile
CORS_ALLOWED_ORIGINS=        # domain frontend yang diizinkan, dipisah koma
APP_ENV=development|production
COOKIE_DOMAIN=
```

**Frontend (`.env`):**
```
VITE_API_BASE_URL=
VITE_TURNSTILE_SITE_KEY=     # site key publik Turnstile (aman untuk terekspos di client)
```

`JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, dan `TURNSTILE_SECRET_KEY` **tidak boleh** pernah ada di kode sumber, commit history, atau dikirim ke frontend. File `.env` wajib masuk `.gitignore`. Untuk deployment VPS, secret disimpan lewat environment variable server atau secret manager, bukan file `.env` yang ikut ter-deploy dari repo publik.

## 3. Role & Hak Akses

| Role | Hak Akses |
|---|---|
| **Admin** | Full access (CRUD semua modul: tutor, orang tua, peserta didik, pokjar, presensi, nilai, rapor, akun) |
| **Kepala Sekolah** | Full read-only akses semua modul + download (Excel/PDF), tanpa hak edit/hapus |
| **Guru** | Dua peran yang bisa dipegang bersamaan oleh 1 akun: **(a) Wali Kelas** — akses penuh administratif ke rombel yang dipegang (presensi + tanda tangan, lihat data peserta didik), berdasarkan `wali_kelas_id` di §4.5; **(b) Guru Mapel** — akses input nilai hanya untuk kombinasi kelas+mapel yang ditugaskan admin lewat tabel penugasan (§4.9), bisa mencakup 1 kelas tertentu atau banyak kelas sekaligus (mis. guru Olahraga lintas kelas) |

Autentikasi: login dengan username/email + password + **verifikasi Cloudflare Turnstile** (wajib untuk semua role — Admin, Kepala Sekolah, Guru), password di-hash dengan bcrypt, sesi dikelola dengan pasangan access token (JWT, umur pendek) + refresh token (umur panjang, disimpan sebagai httpOnly cookie). Guru login dengan 1 akun yang bisa terhubung ke banyak rombel dengan peran berbeda-beda (wali kelas di satu rombel, guru mapel di rombel lain) — lihat §4.5 dan §4.9. Detail lengkap alur keamanan ada di §6.

## 4. Data Model

### 4.1 Tutor (Guru)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama | string | wajib |
| jenis_kelamin | enum(L/P) | wajib |
| tempat_lahir | string | |
| tanggal_lahir | date | |
| tanggal_bertugas | date | |
| no_hp | string | |
| alamat | text | |
| user_id | FK → users | relasi ke akun login (nullable, tidak semua tutor punya akun) |
| created_at / updated_at | timestamp | |

### 4.2 Orang Tua
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama_bapak | string | tidak wajib |
| nama_ibu | string | wajib |
| created_at / updated_at | timestamp | |

Relasi: 1 data orang tua → banyak peserta didik (1 orang tua bisa punya beberapa anak terdaftar).

### 4.3 Pokjar (Kelompok Belajar)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama_pokjar | string | wajib |
| tipe | enum(pusat/binaan) | Pusat / Binaan-Cabang |
| alamat | text | opsional |
| created_at / updated_at | timestamp | |

Data awal (seed):
- PKBM Tunas Ilmu Pusat — tipe: Pusat
- Nashirus Sunnah — tipe: Binaan/Cabang
- Umar bin Khattab — tipe: Binaan/Cabang
- Lentera Qalbu — tipe: Binaan/Cabang

### 4.4 Tahun Ajaran / Periode
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama_tahun_ajaran | string | wajib, unik, mis. "2026/2027" |
| tanggal_mulai | date | |
| tanggal_selesai | date | |
| is_aktif | bool | hanya boleh **1 tahun ajaran aktif** pada satu waktu; mengubah salah satu jadi aktif otomatis menonaktifkan yang lain |
| created_at / updated_at | timestamp | |

Konsep "periode aktif" ini dipakai di seluruh modul (Kelas, Presensi, Dashboard) agar data selalu bisa difilter per tahun ajaran, sekaligus jadi basis untuk histori/arsip multi-tahun.

### 4.5 Kelas (Rombel)

Jenjang tetap 1–6 (Kelas 1 s.d. Kelas 6). Setiap jenjang bisa punya beberapa rombongan belajar paralel (rombel), mis. Kelas 1A, Kelas 1B, Kelas 2A, dst. **Setiap rombel terikat ke 1 tahun ajaran tertentu** — rombel baru dibuat tiap tahun ajaran baru (bisa cukup dengan aksi "duplikasi dari tahun ajaran sebelumnya" di UI supaya admin tidak input ulang dari nol).

| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| jenjang | int (1–6) | wajib |
| nama_rombel | string | mis. "A", "B", "C" — digabung tampilkan sebagai "Kelas 1A" |
| pokjar_id | FK → pokjar | |
| tahun_ajaran_id | FK → tahun_ajaran | wajib |
| wali_kelas_id | FK → tutor | nullable, **bisa diubah kapan saja oleh Admin**; pointer cepat ke wali kelas yang sedang aktif |
| created_at / updated_at | timestamp | |

Kombinasi (`jenjang`, `nama_rombel`, `pokjar_id`, `tahun_ajaran_id`) harus unik. Wali kelas adalah tutor utama yang bertanggung jawab administratif atas 1 rombel (presensi mingguan, koordinasi rapor). Histori lengkap pergantian wali kelas (bukan cuma nilai terakhir) disimpan di §4.6.

### 4.6 Riwayat Wali Kelas
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| kelas_id | FK → kelas | |
| tutor_id | FK → tutor | |
| tanggal_mulai | date | |
| tanggal_selesai | date | nullable — kosong berarti masih menjabat |
| created_at | timestamp | |

Setiap kali Admin mengganti `wali_kelas_id` pada Kelas (§4.5), backend otomatis: (1) mengisi `tanggal_selesai` pada baris riwayat wali kelas lama yang masih aktif, (2) membuat baris baru untuk wali kelas pengganti. Dengan begitu histori "siapa wali kelas 1A tahun ajaran 2025/2026" tetap tersimpan meski sudah diganti berkali-kali.

### 4.7 Mata Pelajaran (CRUD, Master Data)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama_mapel | string | wajib, unik, mis. "Matematika", "Olahraga", "Tahfidz" |
| kode_mapel | string | opsional, untuk keperluan rapor/singkatan |
| is_active | bool | default true, agar mapel lama bisa dinonaktifkan tanpa menghapus data historis |
| created_at / updated_at | timestamp | |

Mata pelajaran dikelola bebas oleh Admin (tambah/ubah/nonaktifkan) — tidak di-hardcode di kode program, karena daftar mapel per PKBM/jenjang bisa berbeda dan berubah dari waktu ke waktu.

### 4.8 Kelas ↔ Mata Pelajaran (Pivot, Scalable)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| kelas_id | FK → kelas | |
| mapel_id | FK → mata_pelajaran | |
| created_at | timestamp | |

Menentukan mapel apa saja yang berlaku di sebuah rombel. Karena sebagian kelas mengikuti kurikulum lengkap dan sebagian lain hanya beberapa mapel, admin memilih mapel per kelas secara bebas lewat tabel ini — bukan daftar mapel yang sama untuk semua kelas.

### 4.9 Penugasan Guru per Kelas & Mapel (Pivot)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| tutor_id | FK → tutor | |
| kelas_id | FK → kelas | |
| mapel_id | FK → mata_pelajaran | |
| created_at / updated_at | timestamp | |

Ini yang mengatur akses guru mapel (mis. guru Olahraga) ke kelas tertentu untuk keperluan input nilai:
- 1 baris = izin 1 guru mengajar/menilai 1 mapel di 1 kelas tertentu (kelas sudah terikat 1 tahun ajaran via §4.5, jadi penugasan otomatis per-tahun-ajaran tanpa perlu field tambahan).
- Guru Olahraga yang mengajar di banyak kelas cukup punya banyak baris (1 per kelas) dengan `mapel_id` = Olahraga — Admin bisa memakai aksi "tugaskan ke semua kelas" di UI yang otomatis membuat baris untuk tiap kelas yang memang punya mapel Olahraga (lihat §4.8), tanpa perlu field khusus "berlaku untuk semua kelas" di skema.
- Penugasan ini terpisah dari `wali_kelas_id` di §4.5 — wali kelas otomatis berhak atas administrasi rombelnya (presensi), sedangkan hak *input nilai per mapel* selalu lewat tabel penugasan ini, termasuk untuk wali kelas sendiri jika dia juga mengajar mapel tertentu di kelasnya.
- Kombinasi (`tutor_id`, `kelas_id`, `mapel_id`) harus unik.

### 4.10 Peserta Didik
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| nama | string | wajib |
| jenis_kelamin | enum(L/P) | wajib |
| nis | string | unik |
| nisn | string | unik |
| kelas_id | FK → kelas | rombel **saat ini** (tahun ajaran aktif) |
| pokjar_id | FK → pokjar | asal pokjar |
| orang_tua_id | FK → orang_tua | |
| status | enum(aktif/lulus/pindah/keluar) | default aktif |
| created_at / updated_at | timestamp | |

Import Excel untuk peserta didik disarankan (mengikuti pola aplikasi buku modul PKBM yang sudah ada).

### 4.11 Riwayat Kelas Peserta Didik (Kenaikan Kelas & Arsip)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| peserta_didik_id | FK → peserta_didik | |
| kelas_id | FK → kelas | rombel pada tahun ajaran terkait |
| tahun_ajaran_id | FK → tahun_ajaran | |
| status | enum(aktif/naik/tinggal/lulus/pindah/keluar) | status peserta didik pada tahun ajaran tsb |
| catatan | string | opsional |
| created_at | timestamp | |

Setiap kenaikan kelas, peserta didik pindah rombel, lulus, atau keluar, sistem menambah 1 baris baru di sini (append-only, tidak menimpa data lama) sekaligus memperbarui `kelas_id`/`status` **saat ini** pada §4.10. Tabel ini yang menjadi dasar: riwayat kelas per peserta didik dari tahun ke tahun, laporan kenaikan kelas per jenjang, dan arsip peserta didik yang sudah lulus/pindah (datanya tidak hilang, tetap bisa ditelusuri per tahun ajaran).

**Navigasi halaman Arsip:** baris pada tabel ini bersifat tahunan (1 baris per tahun ajaran per peserta didik, ditulis saat proses kenaikan kelas §5.3 dijalankan di akhir tahun ajaran), sedangkan Presensi (§4.12) dan Rapor (§4.16) memang sudah tercatat per semester. Agar konsisten, halaman Arsip di frontend memakai alur navigasi bertingkat **Tahun Ajaran → Semester (Ganjil/Genap)** untuk seluruh data historis (riwayat kelas, presensi, rapor): Admin/Kepala Sekolah memilih tahun ajaran (lama/non-aktif), lalu memilih Semester Ganjil atau Semester Genap, baru sistem menampilkan data periode tsb. Karena `kelas_id` pada tabel ini berlaku untuk 1 tahun ajaran penuh (bukan per semester), 1 baris riwayat kelas yang sama ditampilkan konsisten baik saat Semester Ganjil maupun Semester Genap dari tahun ajaran itu dipilih — tidak perlu baris terpisah per semester, cukup ikut mengisi konteks kelas pada kedua tampilan semester tsb.

**Dikonfirmasi:** keputusan "tinggal kelas" hanya terjadi pada satu titik, yaitu saat proses kenaikan kelas di pergantian tahun ajaran (§5.3, lewat opsi override per peserta didik), dan biasanya hanya menyangkut sebagian kecil peserta didik — bukan peristiwa yang bisa terjadi di tengah semester. Karena itu tabel ini **tidak** memerlukan field `semester` tambahan; cukup `tahun_ajaran_id` seperti skema di atas.

### 4.12 Presensi Pertemuan (Auto-Generate, WIB)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| kelas_id | FK → kelas | |
| tanggal | date | tanggal efektif pertemuan — default Sabtu (hasil auto-generate), **bisa diubah** wali kelas/admin kalau Sabtu digeser |
| tanggal_rencana | date | tanggal asli hasil auto-generate sebelum diubah; nullable untuk pertemuan yang dibuat manual |
| semester | enum(Ganjil/Genap) | dihitung otomatis dari tanggal & periode tahun ajaran aktif |
| status_pertemuan | enum(berlangsung/libur/dipindah) | "dipindah" dipakai saat `tanggal` diubah dari `tanggal_rencana` |
| dibuat_otomatis | bool | true = hasil job auto-generate; false = dibuat manual oleh admin/wali kelas (pertemuan tambahan/susulan) |
| keterangan | string | opsional, alasan libur/pindah |
| tutor_id | FK → tutor | wali kelas yang bertugas & tanda tangan pada pertemuan itu |
| tanda_tangan | text (base64 image) | wajib diisi setiap pertemuan (dari signature canvas) |
| created_at / updated_at | timestamp | |

**Mekanisme auto-generate:**
- Job terjadwal (cron, `robfig/cron`) berjalan dengan zona waktu **WIB (Asia/Jakarta)**, bukan UTC server — supaya perhitungan "Sabtu berikutnya" selalu tepat sesuai waktu Indonesia.
- Setiap awal pekan, job otomatis membuat 1 baris presensi untuk **setiap rombel di tahun ajaran aktif** (§4.5) yang punya wali kelas terisi, dengan `tanggal` = `tanggal_rencana` = Sabtu mendatang, `status_pertemuan = berlangsung`, `dibuat_otomatis = true`.
- Hari default = **Sabtu**, dikonfigurasi lewat §4.13 Pengaturan Jadwal — bisa diganti admin bila kebijakan hari KBM berubah di masa depan.
- Wali kelas/Admin bisa mengubah `tanggal` pertemuan yang sudah dibuat otomatis (mis. Sabtu libur nasional → digeser ke hari lain); `tanggal_rencana` tetap tersimpan sebagai jejak rencana awal, `status_pertemuan` otomatis berubah jadi "dipindah". Jika pertemuan ditiadakan sama sekali (tanpa pengganti), cukup ubah `status_pertemuan` jadi "libur".
- Admin/wali kelas tetap bisa membuat pertemuan tambahan secara manual (`dibuat_otomatis = false`) di luar jadwal reguler bila diperlukan (mis. kelas susulan).

### 4.13 Pengaturan Jadwal (Settings)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | baris tunggal (single-row config) |
| hari_default | enum(Senin..Minggu) | default **Sabtu** |
| jam_generate | time | jam job cron dijalankan, dalam WIB (mis. 00:05) |
| zona_waktu | string | default `"Asia/Jakarta"`, tetap eksplisit di skema untuk kejelasan meski saat ini fixed WIB |
| updated_at | timestamp | |

Satu baris konfigurasi global yang dipakai job auto-generate presensi (§4.12). Admin bisa mengubah `hari_default` lewat halaman pengaturan bila kebijakan KBM berubah (mis. dari Sabtu ke hari lain) tanpa perlu ubah kode program.

### 4.14 Presensi Detail (per siswa)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| presensi_id | FK → presensi | |
| peserta_didik_id | FK → peserta_didik | |
| status_kehadiran | enum(Hadir/Sakit/Izin/Alpa) | checklist oleh guru |
| catatan | string | opsional |

Export: Excel (rekap per kelas/per tanggal) dan PDF (format presensi cetak, termasuk kolom tanda tangan tutor).

### 4.15 Nilai — *Dibahas di PRD Terpisah*

Komponen penilaian (jenis nilai, bobot, KKM, cara agregasi ke rapor, dsb.) cukup kompleks dan akan dirancang di PRD tersendiri. Untuk PRD ini, struktur di §4.7–4.9 (Mata Pelajaran, Kelas↔Mapel, Penugasan Guru) sudah disiapkan sebagai fondasi yang akan dipakai modul nilai nanti — sehingga modul nilai tinggal mengacu ke `kelas_id` + `mapel_id` + `tutor_id` yang sudah ada, tanpa perlu merombak struktur kelas/mapel/guru saat PRD nilai dibuat.

### 4.16 Rapor
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| peserta_didik_id | FK → peserta_didik | |
| kelas_id | FK → kelas | |
| semester | enum(Ganjil/Genap) | |
| tahun_ajaran_id | FK → tahun_ajaran | |
| ringkasan_nilai | JSON/relasi ke nilai | menyusul — bergantung desain modul nilai (§4.15) |
| catatan_wali_kelas | text | opsional |
| rekap_presensi | JSON/relasi | ringkasan hadir/sakit/izin/alpa selama semester |
| status_kenaikan | enum(naik/tinggal/lulus) | opsional, tersinkron dengan §4.11 saat kenaikan kelas diproses |
| created_at / updated_at | timestamp | |

Rapor dihasilkan (generate) dari agregasi data nilai + presensi per peserta didik per semester, dicetak sebagai PDF. Detail agregasi nilai menyusul bersamaan dengan PRD modul nilai.

### 4.17 Users (Akun)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| username / email | string | unik |
| password_hash | string | bcrypt |
| role | enum(admin/kepala_sekolah/guru) | |
| tutor_id | FK → tutor | nullable, hanya untuk role guru |
| is_active | bool | |
| created_at / updated_at | timestamp | |

## 5. Modul & Fitur Utama

### 5.1 Autentikasi & Manajemen Akun
- Halaman login menampilkan widget **Cloudflare Turnstile**; permintaan login ditolak backend jika token Turnstile tidak ada/tidak valid, sebelum kredensial diperiksa.
- Login menghasilkan access token (JWT, umur pendek) + refresh token (httpOnly cookie, umur panjang); logout menghapus refresh token (server + cookie).
- Admin: CRUD akun (admin, kepala sekolah, guru), termasuk menghubungkan akun guru ke data tutor, menetapkan sebagai wali kelas (§4.5), dan menugaskan sebagai guru mapel di kelas-kelas tertentu (§4.9).
- Guru hanya bisa mengakses data & fitur sesuai perannya: administrasi rombel (jika wali kelas) dan/atau input nilai per kelas+mapel (jika ditugaskan sebagai guru mapel) — role guard di middleware backend + route protection di frontend (**enforcement utama selalu di backend**, guard di frontend hanya untuk UX).
- Percobaan login gagal berulang kali dari akun/IP yang sama dibatasi (rate limiting + lockout sementara).

### 5.2 Master Data
- CRUD Tutor
- CRUD Orang Tua (terhubung ke peserta didik saat input/edit peserta didik)
- CRUD Pokjar
- CRUD Tahun Ajaran/Periode — buat tahun ajaran baru, atur tanggal mulai/selesai, tetapkan sebagai periode aktif.
- CRUD Kelas/Rombel — pilih jenjang (1–6), nama rombel (A/B/C, dst.), pokjar, tahun ajaran, dan wali kelas (bisa diganti kapan saja oleh Admin, otomatis tercatat di riwayat wali kelas §4.6); tersedia aksi "duplikasi rombel dari tahun ajaran sebelumnya" untuk mempercepat setup tahun ajaran baru.
- CRUD Mata Pelajaran (nama, kode, status aktif) — bebas ditambah/diubah admin, tidak hardcode di kode program.
- Pengaturan mapel per kelas: admin memilih mapel apa saja yang berlaku di tiap rombel (mendukung kelas dengan kurikulum lengkap maupun kelas dengan mapel terbatas).
- Penugasan guru mapel per kelas: admin menghubungkan tutor ke kombinasi kelas+mapel tertentu, termasuk aksi cepat "tugaskan ke semua kelas yang punya mapel ini" untuk guru lintas kelas (mis. Olahraga).
- CRUD Peserta Didik (dengan pilihan orang tua, kelas, pokjar asal; dukung import Excel)
- Pengaturan Jadwal (§4.13): admin mengatur hari default pertemuan (default Sabtu) dan jam job auto-generate presensi.

### 5.3 Kenaikan Kelas & Arsip Peserta Didik
- Wizard "Proses Kenaikan Kelas" di akhir tahun ajaran: admin memilih tahun ajaran tujuan, lalu memindahkan peserta didik per rombel ke rombel jenjang berikutnya secara **massal**, dengan opsi override per peserta didik individual (mis. tinggal kelas, pindah rombel berbeda, lulus, atau keluar).
- Setiap peserta didik yang diproses otomatis mendapat baris baru di Riwayat Kelas Peserta Didik (§4.11); status "lulus"/"pindah"/"keluar" membuat peserta didik masuk kategori arsip (tetap tersimpan, tidak dihapus, bisa ditelusuri lewat riwayatnya).
- Halaman riwayat per peserta didik: menampilkan rombel & status di setiap tahun ajaran yang pernah diikuti.
- Halaman riwayat wali kelas per rombel: menampilkan daftar wali kelas dari tahun ke tahun (§4.6).
- Halaman **Arsip** (riwayat multi-tahun): navigasi bertingkat **pilih Tahun Ajaran → pilih Semester (Ganjil/Genap)** sebelum data riwayat ditampilkan — lihat detail alur & catatan skema di §4.11.

### 5.4 Presensi
- Presensi mingguan **dibuat otomatis** oleh job terjadwal (WIB) untuk setiap rombel aktif, default hari **Sabtu** — lihat mekanisme lengkap di §4.12.
- Wali kelas membuka rombel yang dipegang → pertemuan minggu berjalan sudah tersedia (hasil auto-generate); tinggal isi checklist kehadiran & tanda tangan.
- Jika Sabtu diliburkan/digeser, wali kelas atau admin bisa mengubah tanggal pertemuan itu langsung (tercatat sebagai "dipindah") atau menandainya "libur" tanpa presensi kehadiran.
- Jika "berlangsung": checklist kehadiran per siswa (Hadir/Sakit/Izin/Alpa).
- Wajib tanda tangan wali kelas (signature canvas) sebelum presensi bisa disimpan.
- Export ke Excel dan PDF (per pertemuan atau rekap per periode).
- Presensi tahun ajaran lampau diakses lewat halaman Arsip (§5.3) dengan navigasi Tahun Ajaran → Semester, direkap sesuai field `semester` yang sudah tersimpan otomatis di §4.12.

### 5.5 Nilai — *Menyusul di PRD Terpisah*
- Struktur akses sudah disiapkan lewat §4.7–4.9 (mapel, kelas↔mapel, penugasan guru), sehingga modul nilai nanti tinggal memakai kombinasi kelas+mapel+guru yang sudah ada.
- Detail komponen penilaian, bobot, dan alur input akan dirancang di PRD khusus menyusul.

### 5.6 Rapor
- Generate rapor otomatis dari agregasi nilai (menyusul, §4.15) + rekap presensi per peserta didik per semester.
- Cetak/export rapor sebagai PDF.
- Admin & Kepala Sekolah dapat mengakses rapor seluruh peserta didik; wali kelas hanya untuk rombel yang dipegang.
- Rapor tahun ajaran lampau juga diakses lewat halaman Arsip (§5.3) dengan navigasi Tahun Ajaran → Semester, sesuai field `semester` pada §4.16.

### 5.7 Dashboard
- Ringkasan statistik (jumlah peserta didik per pokjar/kelas, rekap kehadiran, distribusi nilai) menggunakan Recharts, difilter per tahun ajaran aktif.
- Beda tampilan dashboard sesuai role (Admin/Kepala Sekolah: global; Guru: rombel/mapel yang diampu).

## 6. Keamanan (Security)

Karena aplikasi menyimpan data pribadi anak (peserta didik, NIS/NISN, orang tua), keamanan wajib diperlakukan sebagai kebutuhan inti, bukan tambahan di akhir.

### 6.1 Cloudflare Turnstile di Setiap Login
- Widget Turnstile (`VITE_TURNSTILE_SITE_KEY`) dipasang di form login untuk **semua role** (Admin, Kepala Sekolah, Guru) — bukan hanya salah satu role.
- Frontend mengirim token Turnstile (`cf-turnstile-response`) bersama username/password ke endpoint login.
- Backend **wajib** memverifikasi token ke Cloudflare (`POST https://challenges.cloudflare.com/turnstile/v0/siteverify` dengan `secret`, `response`, dan `remoteip`) **sebelum** memeriksa kredensial. Jika verifikasi gagal/kedaluwarsa → tolak dengan 400/401, jangan lanjut cek password.
- Token Turnstile hanya berlaku sekali pakai; jangan cache/reuse token di backend.
- Mode "Managed" (bukan "Invisible") direkomendasikan agar tetap terlihat sebagai lapisan verifikasi standar, sekaligus mengurangi gesekan bagi pengguna sah.

### 6.2 Autentikasi & Sesi
- Access token JWT umur pendek (± 15 menit), refresh token umur panjang (± 7 hari) disimpan sebagai **httpOnly, Secure, SameSite=Strict cookie** — bukan di `localStorage`, untuk mengurangi risiko pencurian token lewat XSS.
- Refresh token disimpan juga di DB (hash-nya, bukan plaintext) agar bisa di-revoke (mis. saat logout, ganti password, atau akun dinonaktifkan admin).
- Endpoint `refresh` memvalidasi refresh token dari cookie, menerbitkan access token baru; refresh token lama di-rotate (one-time use) untuk mendeteksi pencurian token.
- Password minimal 8 karakter, kombinasi huruf & angka; validasi di frontend (UX) **dan** backend (source of truth).
- Rate limiting endpoint login & refresh (mis. maks 5 percobaan/menit per IP) memakai Fiber `limiter` middleware, ditambah lockout sementara per akun setelah beberapa kali gagal berturut-turut.
- Endpoint `logout` menghapus refresh token dari DB dan cookie di browser.

### 6.3 Otorisasi (RBAC)
- Semua endpoint API diverifikasi di **backend** (bukan hanya disembunyikan di UI): middleware role guard mengecek klaim role di JWT untuk setiap request.
- Untuk endpoint presensi, middleware memastikan `kelas_id` yang diakses memang rombel di mana user adalah `wali_kelas_id` (§4.5) — cegah IDOR (wali kelas mengakses/isi presensi rombel lain).
- Untuk endpoint nilai (menyusul, §4.15), middleware memastikan kombinasi `kelas_id`+`mapel_id` yang diakses memang ada di tabel penugasan guru (§4.9) untuk user yang login — guru mapel lintas kelas (mis. Olahraga) tetap hanya bisa mengakses kelas & mapel yang benar-benar ditugaskan padanya, tidak otomatis akses ke seluruh kelas.
- Kepala Sekolah: middleware khusus yang menolak semua request `POST/PUT/PATCH/DELETE`, hanya izinkan `GET` dan endpoint export/download.

### 6.4 Perlindungan Data & Input
- Semua query database lewat GORM (parameterized query) — dilarang membangun query dengan string concatenation manual (`Raw()`/`Exec()` dengan input mentah user).
- Validasi & sanitasi seluruh input request (`validator/v10` di backend) — tipe data, panjang string, format (NIS/NISN numerik, tanggal valid, dsb.), termasuk saat import Excel (validasi baris per baris, tolak file dengan struktur tak sesuai, batasi ukuran file).
- Output yang dirender di frontend React di-escape otomatis oleh React (default aman dari XSS) — hindari `dangerouslySetInnerHTML` kecuali benar-benar perlu dan sudah disanitasi.
- Data tanda tangan (base64 image) divalidasi ukuran & tipe MIME sebelum disimpan, untuk mencegah penyalahgunaan sebagai vektor upload file besar/berbahaya.
- Header keamanan HTTP standar (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy) diaktifkan lewat middleware `helmet` di Fiber.
- CORS dibatasi ke origin frontend resmi saja (`CORS_ALLOWED_ORIGINS`), tidak memakai wildcard `*` khususnya untuk endpoint yang memakai cookie.

### 6.5 Audit & Monitoring
- Log setiap aktivitas sensitif (login berhasil/gagal, perubahan data nilai, perubahan akun, export data) dengan user_id, waktu, dan aksi — tabel `audit_log` terpisah.
- Log request HTTP (via Fiber `logger` middleware) untuk kebutuhan investigasi, tanpa mencatat password/token secara penuh.

### 6.6 Transport & Deployment
- HTTPS wajib di production (lewat reverse proxy/nginx + sertifikat, mis. Let's Encrypt); redirect otomatis HTTP → HTTPS.
- Semua secret (`JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `TURNSTILE_SECRET_KEY`, kredensial `DATABASE_URL`) hanya lewat environment variable server, tidak pernah masuk repo/log/response API (lihat §2.5).
- Backup database PostgreSQL production terjadwal (mis. harian) mengingat data ini menyangkut data anak & institusi.
- Dependency (Go modules & npm packages) diperbarui berkala untuk menutup celah keamanan yang diketahui (`go list -u`, `npm audit`).

## 7. Alur Implementasi (Bertahap — untuk Claude Code)

**Tahap 1 — Setup Proyek**
- Inisialisasi backend Go Fiber + GORM dengan dual-driver DB (SQLite lokal, PostgreSQL via `DATABASE_URL`).
- Inisialisasi frontend Vite + React 19 + TypeScript + Tailwind v4 + shadcn/ui.
- Setup struktur folder sesuai §2.4.

**Tahap 2 — Skema Database & Migrasi**
- Buat model GORM: User, Tutor, OrangTua, Pokjar, TahunAjaran, Kelas (jenjang+rombel+wali_kelas_id+tahun_ajaran_id), RiwayatWaliKelas, MataPelajaran, KelasMapel (pivot), PenugasanGuruMapel (pivot), PesertaDidik, RiwayatKelasPesertaDidik, PengaturanJadwal, Presensi, PresensiDetail, Rapor.
- Migrasi otomatis (AutoMigrate) + seed data pokjar awal (4 pokjar) + seed 1 tahun ajaran aktif awal + seed PengaturanJadwal default (hari_default = Sabtu).

**Tahap 3 — Autentikasi, Turnstile & Role**
- Setup env vars (§2.5): JWT secrets, `TURNSTILE_SECRET_KEY`/`VITE_TURNSTILE_SITE_KEY`, CORS origin.
- Pasang widget Turnstile di halaman login (frontend) + endpoint verifikasi token ke Cloudflare (backend) sebelum cek kredensial.
- Endpoint login (access token JWT + refresh token httpOnly cookie), refresh, logout (revoke refresh token).
- Middleware: role guard (admin/kepala_sekolah/guru), guard kepemilikan kelas untuk guru, rate limiting login/refresh, CORS whitelist, security headers (helmet).
- Password hashing bcrypt, validasi input (validator/v10) di semua endpoint auth.

**Tahap 4 — Modul Master Data (Admin)**
- CRUD Tutor, Orang Tua, Pokjar, Mata Pelajaran, Peserta Didik (+ import Excel peserta didik).
- CRUD Tahun Ajaran (buat baru, atur tanggal, tetapkan aktif) dan CRUD Kelas/Rombel (jenjang 1–6 + wali kelas + tahun ajaran), termasuk aksi "duplikasi rombel dari tahun ajaran sebelumnya".
- CRUD pengaturan mapel per kelas dan penugasan guru mapel per kelas (§4.8–4.9).
- CRUD Akun (Admin, Kepala Sekolah, Guru + hubungkan ke tutor/wali kelas/penugasan mapel).
- Halaman Pengaturan Jadwal (§4.13): ubah hari default & jam job auto-generate.

**Tahap 5 — Modul Presensi & Scheduler Otomatis (WIB)**
- Setup job cron (`robfig/cron`, timezone `Asia/Jakarta`) yang membuat baris presensi otomatis tiap awal pekan untuk seluruh rombel aktif sesuai §4.12; baca hari default dari §4.13.
- Form presensi per rombel per tanggal — mendukung ubah tanggal (status "dipindah") atau tandai "libur", hanya bisa diakses wali kelas rombel terkait.
- Checklist kehadiran + signature canvas wali kelas.
- Export Excel & PDF.

**Tahap 6 — Kenaikan Kelas & Arsip Peserta Didik**
- Wizard proses kenaikan kelas massal per rombel + override per peserta didik (§5.3), menulis ke Riwayat Kelas Peserta Didik dan memperbarui `kelas_id`/status di Peserta Didik.
- Halaman riwayat kelas per peserta didik dan riwayat wali kelas per rombel.
- Halaman Arsip dengan navigasi bertingkat Tahun Ajaran → Semester (§4.11, §5.3), sebagai pintu masuk ke rekap presensi & rapor tahun ajaran lampau.

**Tahap 7 — Modul Nilai — Menyusul**
- Ditunda ke PRD terpisah (§4.15); struktur kelas/mapel/penugasan guru di tahap ini sudah disiapkan sebagai fondasinya.

**Tahap 8 — Modul Rapor**
- Generate rapor (agregasi nilai + presensi), cetak PDF.

**Tahap 9 — Dashboard & Laporan**
- Statistik ringkas per role, grafik dengan Recharts.

**Tahap 10 — Kepala Sekolah (Read-only + Download)**
- Pastikan semua endpoint read hanya dan tombol download aktif, tanpa akses tulis/hapus.

**Tahap 11 — Audit Log & Hardening Tambahan**
- Tabel & pencatatan `audit_log` untuk aktivitas sensitif (§6.5).
- Review menyeluruh: rate limiting sudah aktif di semua endpoint publik/auth, CORS whitelist, header keamanan (helmet), validasi input di semua endpoint tulis, cek IDOR di endpoint guru.

**Tahap 12 — Deployment**
- Build frontend (Vite) sebagai static assets.
- Build backend Go sebagai binary.
- Konfigurasi `DATABASE_URL` di VPS untuk beralih ke PostgreSQL.
- Reverse proxy (nginx) + HTTPS (wajib, bukan opsional — lihat §6.6) + redirect HTTP→HTTPS.
- Set seluruh environment variable secret di server (bukan file `.env` dari repo), termasuk `TURNSTILE_SECRET_KEY` dan `JWT_*_SECRET` yang berbeda dari environment development.
- Setup jadwal backup database production.

## 8. Keputusan yang Sudah Dikonfirmasi
- **Kenaikan kelas**: diproses **massal per rombel** dengan opsi override per peserta didik individual (§5.3) — bukan satu-satu manual dari awal. Data riwayatnya diakses lewat halaman **Arsip** dengan navigasi bertingkat Tahun Ajaran → Semester (pilih Semester Ganjil atau Semester Genap) — lihat detail alur & catatan skema di §4.11, dan penerapannya di §5.3, §5.4, §5.6.
- **Struktur data Riwayat Kelas Peserta Didik (§4.11)** tetap 1 baris per tahun ajaran (tanpa field `semester` tambahan): keputusan "tinggal kelas" hanya terjadi pada momen kenaikan kelas/pergantian tahun ajaran (lewat opsi override di §5.3) dan biasanya hanya menyangkut sebagian kecil peserta didik, bukan peristiwa per semester. Di halaman Arsip, baris riwayat yang sama tetap tampil konsisten baik saat Semester Ganjil maupun Semester Genap dari tahun ajaran tsb dipilih.
- **Jam job auto-generate presensi** (§4.13 `jam_generate`): PRD ini memakai contoh dini hari WIB (mis. 00:05) di awal pekan sekadar sebagai default/contoh — nilai sebenarnya diatur bebas oleh admin lewat halaman Pengaturan Jadwal, tanpa perlu ubah kode.

## 9. Hal yang Perlu Dikonfirmasi/Didetailkan Lebih Lanjut
- Komponen nilai (jenis nilai, bobot, KKM, dsb.) — **akan dibahas di PRD terpisah**.
- Format rapor cetak (template resmi PKBM, kop surat, tanda tangan siapa saja) — menyusul bersamaan dengan PRD nilai.
