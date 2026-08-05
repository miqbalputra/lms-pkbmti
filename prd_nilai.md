# PRD — Modul Nilai (Lanjutan dari PRD Utama §4.15)

Dokumen ini melanjutkan **§4.15 PRD Utama** ("Nilai — Dibahas di PRD Terpisah"). Modul ini dibangun di atas fondasi yang sudah ada di PRD Utama: Mata Pelajaran (§4.7), Kelas↔Mapel (§4.8), Penugasan Guru per Kelas & Mapel (§4.9), Peserta Didik (§4.10), Tahun Ajaran (§4.4), dan bermuara ke Rapor (§4.16).

Rancangan mengacu pada file contoh `format_input_nilai.xlsx` yang berisi 5 sheet (satu per mapel: B.Indo, MTK, PKn, IPA, IPS) dengan format identik — jadi 1 skema generik berlaku untuk semua mapel, bukan hardcode per mapel.

## 1. Ringkasan Alur Penilaian

Berdasarkan format Excel contoh, alurnya berjenjang:

1. **Tutor membuat Tema** per kelas + mapel yang diampu (§2 tabel `tema`). Contoh dari file: Kelas 1A - Bahasa Indonesia punya Tema 1, 2, 3...; Kelas 3B - Matematika punya Tema 11, 12, 13... — penomoran/nama tema bebas ditulis tutor, tidak berurutan global.
2. **Setiap Tema punya beberapa slot Capaian Pembelajaran (CP)** — di contoh selalu 2 slot per tema (CP1, CP2), misal CP1 = "puisi", CP2 = "teks informasi".
3. **Tutor input nilai mentah per siswa** untuk setiap tema: nilai per slot CP (nilai keterampilan, 0–100) + 1 nilai UM Tema (nilai ulangan/pengetahuan, 0–100, dipakai bareng untuk semua slot CP di tema itu).
4. **Sistem menghitung otomatis** (tidak diinput manual):
   - Nilai Keterampilan Tema = rata-rata nilai semua CP dalam tema itu.
   - Nilai Akhir per CP (dipakai untuk narasi rapor) = bobot keterampilan × nilai CP + bobot pengetahuan × nilai UM tema itu (default 60:40, bisa diatur admin per mapel — §2.5).
   - Di akhir semester: NP Akhir (rata-rata nilai UM seluruh tema) & NK Akhir (rata-rata nilai keterampilan seluruh tema), masing-masing dengan Predikat (A/B/C berdasarkan ambang nilai).
5. NP Akhir, Predikat, NK Akhir, Predikat inilah yang jadi rujukan input ke Dapodik (aplikasi dapodik milik Kemendikbud, di luar sistem ini) dan sumber angka nilai di Rapor (§4.16 PRD Utama).

## 2. Data Model

### 2.1 Tema
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| kelas_id | FK → kelas | |
| mapel_id | FK → mata_pelajaran | |
| tahun_ajaran_id | FK → tahun_ajaran | |
| semester | enum(Ganjil/Genap) | |
| nama_tema | string | bebas diisi tutor, mis. "Tema 14" |
| urutan | int | urutan tampil di grid input & rapor |
| jumlah_cp | int | **fleksibel per tema**, tutor menentukan sendiri saat bikin tema (bisa 2, 3, atau lebih — beberapa mapel memang butuh lebih dari 2 CP per tema) |
| bobot_keterampilan | decimal(5,2) | disalin otomatis dari Pengaturan Bobot Nilai mapel (§2.5) saat tema dibuat — lihat §3 |
| bobot_pengetahuan | decimal(5,2) | disalin otomatis dari Pengaturan Bobot Nilai mapel (§2.5) saat tema dibuat — lihat §3 |
| created_at / updated_at | timestamp | |

Dibuat oleh Tutor yang punya penugasan aktif (§4.9 PRD Utama) untuk kombinasi kelas+mapel tsb. Rata-rata 3–5 tema per mapel per semester (sesuai catatan di file Excel), tapi jumlahnya tidak dibatasi di skema — cukup daftar dinamis per kelas+mapel+semester.

`bobot_keterampilan`/`bobot_pengetahuan` disimpan sebagai **salinan (snapshot)** di level tema, bukan dibaca langsung dari §2.5 setiap kali dihitung — supaya kalau admin mengubah default bobot di kemudian hari, tema-tema lama yang sudah selesai/diarsipkan tidak ikut berubah nilainya secara retroaktif. Tutor tetap bisa override bobot khusus untuk 1 tema tertentu (kasus khusus) tanpa mempengaruhi default mapel.

### 2.2 Capaian Pembelajaran — Label Default per Tema
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| tema_id | FK → tema | |
| urutan_cp | int | 1, 2, ... sesuai `jumlah_cp` pada tema |
| label_default | string | label umum yang diisi tutor saat bikin/edit tema (mis. "puisi" untuk CP1) |
| created_at / updated_at | timestamp | |

### 2.3 Nilai CP per Siswa (realisasi, bisa custom per siswa)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| tema_id | FK → tema | |
| urutan_cp | int | mengacu ke §2.2 |
| peserta_didik_id | FK → peserta_didik | |
| deskripsi_cp | string | isi realisasi per siswa — lihat alur bulk-fill di bawah |
| nilai_keterampilan | decimal(5,2) | 0–100, input mentah tutor |
| created_at / updated_at | timestamp | |

Kombinasi (`tema_id`, `urutan_cp`, `peserta_didik_id`) harus unik.

**Alur pengisian `deskripsi_cp` (bulk lalu custom):**
1. Saat tutor mengisi/mengubah `label_default` di §2.2 untuk 1 slot CP, sistem otomatis **menerapkannya ke `deskripsi_cp` semua siswa** di kelas itu untuk slot CP tersebut (bulk apply) — jadi tutor cukup isi 1x di atas grid untuk kasus umum di mana seluruh siswa punya capaian yang sama.
2. Di baris grid tiap siswa, `deskripsi_cp` tetap bisa **diedit manual per siswa** untuk kasus siswa tertentu punya capaian berbeda dari yang lain — begitu diedit manual, baris siswa itu tidak lagi ikut ter-override otomatis kalau `label_default` diubah lagi nanti (nilai per-siswa yang sudah di-custom dikunci, hanya baris yang belum pernah diubah manual yang ikut update bulk).

### 2.4 Nilai UM (Pengetahuan) per Siswa per Tema
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| tema_id | FK → tema | |
| peserta_didik_id | FK → peserta_didik | |
| nilai_um | decimal(5,2) | 0–100, input mentah tutor — 1 nilai per siswa per tema (dipakai bareng untuk semua CP di tema itu, sesuai formula Excel) |
| created_at / updated_at | timestamp | |

Kombinasi (`tema_id`, `peserta_didik_id`) harus unik.

### 2.5 Pengaturan Bobot Nilai per Mapel (Configurable, Default 60:40)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| mapel_id | FK → mata_pelajaran | |
| bobot_keterampilan | decimal(5,2) | default **60** |
| bobot_pengetahuan | decimal(5,2) | default **40** |
| updated_at | timestamp | |

1 baris per mapel, diseed otomatis (60:40) saat mapel baru dibuat di §4.7 PRD Utama. Admin bisa mengubah kapan saja lewat halaman Pengaturan Nilai; `bobot_keterampilan + bobot_pengetahuan` wajib selalu berjumlah 100.

**Alur/jalur perhitungan saat bobot diubah admin:**
1. Admin mengubah bobot mapel tertentu di halaman pengaturan → baris §2.5 mapel itu ter-update.
2. Perubahan ini **tidak** langsung mengubah nilai tema yang sudah ada — setiap tema menyimpan salinan bobotnya sendiri di `bobot_keterampilan`/`bobot_pengetahuan` (§2.1), diisi dari §2.5 pada saat tema itu **dibuat**. Jadi rapor semester yang sudah diproses tidak berubah retroaktif hanya karena admin mengubah setting di kemudian hari.
3. Bobot baru dari §2.5 dipakai sebagai default untuk **tema-tema baru** yang dibuat setelah perubahan itu.
4. Nilai Akhir per CP (§3) selalu mengacu ke bobot yang tersimpan di tema terkait (§2.1), bukan ke §2.5 secara langsung.

### 2.6 Ambang Predikat per Mapel (Configurable — Matematika Beda dari Mapel Lain)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| mapel_id | FK → mata_pelajaran | |
| predikat | enum(A/B/C) | |
| nilai_minimum | int | batas bawah nilai untuk predikat ini |
| updated_at | timestamp | |

**Dikonfirmasi:** hanya mapel **Matematika** yang punya ambang predikat lebih rendah (KKM lebih longgar karena mapel ini memang lebih sulit dapat nilai tinggi), mapel lain memakai ambang yang sama satu sama lain. Nilai default yang diseed sesuai legenda file contoh:
- **Matematika**: A ≥ 80, B ≥ 68, C ≥ 60
- **Mapel lain (default)**: A ≥ 90, B ≥ 78, C ≥ 70

Tetap disimpan per-mapel (bukan hardcode 2 kategori tetap) supaya admin bisa menyesuaikan mapel mana pun di kemudian hari lewat halaman pengaturan, tanpa perlu ubah kode.

### 2.7 Rekap Nilai Akhir Mapel per Semester (Cache, untuk Rapor & Performa)
| Field | Tipe | Keterangan |
|---|---|---|
| id | UUID/PK | |
| peserta_didik_id | FK → peserta_didik | |
| kelas_id | FK → kelas | |
| mapel_id | FK → mata_pelajaran | |
| tahun_ajaran_id | FK → tahun_ajaran | |
| semester | enum(Ganjil/Genap) | |
| np_akhir | decimal(5,2) | hasil hitung, lihat §3 |
| predikat_np | enum(A/B/C) | |
| nk_akhir | decimal(5,2) | hasil hitung, lihat §3 |
| predikat_nk | enum(A/B/C) | |
| updated_at | timestamp | dihitung ulang otomatis setiap ada perubahan nilai CP/UM di tema-tema terkait |

Tabel ini adalah **cache hasil hitung** (bukan sumber input manual) — dipakai supaya generate Rapor (§4.16 PRD Utama) dan tampilan rekap tidak perlu agregasi ulang dari ratusan baris nilai CP/UM setiap kali dibuka. Kombinasi (`peserta_didik_id`, `mapel_id`, `semester`, `tahun_ajaran_id`) harus unik.

## 3. Formula & Perhitungan

Mengikuti persis formula pada file Excel contoh (semua dihitung otomatis oleh backend, tutor hanya input nilai mentah warna putih):

| Hasil Hitung | Formula | Sumber |
|---|---|---|
| Nilai Keterampilan Tema (per siswa, per tema) | `AVERAGE(nilai_keterampilan semua CP dalam tema itu)` | §2.3 |
| Nilai Akhir per CP ("nilai input rapot") | `(bobot_keterampilan/100) × nilai_keterampilan_cp + (bobot_pengetahuan/100) × nilai_um tema itu` — bobot diambil dari tema (§2.1), **default 60:40** tapi bisa beda per tema jika di-override | §2.1 + §2.3 + §2.4 |
| NP Akhir (semester) | `AVERAGE(nilai_um seluruh tema dalam mapel+semester itu)` | §2.4 |
| NK Akhir (semester) | `AVERAGE(nilai_keterampilan_tema seluruh tema dalam mapel+semester itu)` | turunan §2.3 |
| Predikat NP / Predikat NK | lookup `nilai_minimum` tertinggi yang terpenuhi di §2.6, berdasarkan `mapel_id` terkait | §2.6 |

**Warna cell di Excel contoh** (dipakai sebagai acuan desain UI grid input, bukan bagian skema):
- Putih = input manual tutor (nilai CP mentah, nilai UM tema).
- Biru = hasil hitung, dipakai untuk redaksi nilai di Rapor (nilai akhir per CP).
- Kuning = hasil hitung akhir semester, dipakai admin untuk keperluan input ke Dapodik (NP Akhir, NK Akhir, & predikat masing-masing).

## 4. Modul & Fitur Utama

- **Kelola Tema** (Tutor, sesuai kelas+mapel yang ditugaskan §4.9 PRD Utama): tambah/ubah/hapus tema, atur `jumlah_cp` & label default per tema.
- **Input Nilai** (Tutor): grid per tema — baris = siswa di kelas itu, kolom = tiap CP (deskripsi + nilai keterampilan) + 1 kolom nilai UM tema, mengikuti tata letak file Excel contoh. Nilai akhir per CP, nilai keterampilan tema, dst. tampil live (read-only, auto-hitung) di grid yang sama.
- **Pengaturan Nilai** (Admin): halaman untuk mengubah bobot keterampilan:pengetahuan default per mapel (§2.5) dan ambang predikat per mapel (§2.6).
- **Rekap Nilai Semester** (Tutor kelasnya sendiri; Admin & Kepala Sekolah semua kelas): tabel NP Akhir/Predikat/NK Akhir/Predikat per siswa per mapel, sumbernya §2.7.
- **Export Excel & PDF**: layout mengikuti format file contoh (grup kolom per tema), bisa diunduh Admin (semua kelas/mapel) dan Tutor (kelas & mapel yang diampu) — sesuai §3 & §4.9 PRD Utama. Format khusus untuk import Dapodik **menyusul** begitu ada informasi/template resminya; untuk saat ini export cukup Excel & PDF standar.
- **Integrasi ke Rapor** (§4.16 PRD Utama): field `ringkasan_nilai` pada Rapor diisi dari agregasi §2.7 (semua mapel di kelas itu) + narasi Nilai Akhir per CP (§3) sebagai deskripsi capaian di rapor cetak.

## 5. Hak Akses

Mengikuti §3 PRD Utama:
- **Admin**: CRUD penuh semua tema & nilai, semua kelas/mapel.
- **Kepala Sekolah**: read-only + download semua tema & nilai.
- **Guru (Guru Mapel)**: kelola tema & input nilai **hanya** untuk kombinasi kelas+mapel yang ada di tabel penugasan §4.9 PRD Utama — bukan berdasarkan `wali_kelas_id`, sehingga wali kelas yang tidak ditugaskan mengajar mapel tertentu di kelasnya tetap tidak bisa input nilai mapel itu. Guru mapel **tidak** bisa mengubah Pengaturan Nilai (§2.5, §2.6) — itu khusus Admin.

## 6. Validasi & Aturan Bisnis

- `nilai_keterampilan` dan `nilai_um`: wajib 0–100, desimal 2 angka di belakang koma (sesuai contoh: 88.75, 91.7, dst.).
- `bobot_keterampilan + bobot_pengetahuan` (§2.1, §2.5) wajib selalu berjumlah 100.
- Tema, CP, dan nilai UM hanya bisa diinput/diubah untuk tahun ajaran & semester yang masih aktif (tahun ajaran lama bersifat arsip, read-only — konsisten dengan §4.11 & §8 PRD Utama soal halaman Arsip).
- Rekap §2.7 dihitung ulang otomatis setiap ada perubahan pada nilai CP/UM di tema-tema terkait siswa itu (bukan proses manual terpisah).

## 7. Keputusan yang Sudah Dikonfirmasi

- **Jumlah CP per tema**: dibuat **fleksibel** per tema (§2.1, `jumlah_cp`) — beberapa mapel memang butuh lebih dari 2 CP per tema, tidak dikunci mengikuti contoh Excel.
- **Deskripsi CP per siswa**: alurnya **bulk dulu, baru custom** (§2.3) — tutor isi 1x label di atas grid (§2.2), otomatis diterapkan ke semua siswa, lalu bisa diedit manual per siswa untuk kasus yang berbeda.
- **Bobot 60:40**: dijadikan **default yang bisa diubah admin per mapel** (§2.5), dengan bobot di-snapshot ke tiap tema saat tema dibuat (§2.1) supaya perubahan setting admin tidak mengubah nilai tema/rapor yang sudah selesai secara retroaktif — alur lengkapnya di §2.5.
- **Ambang predikat**: **hanya Matematika** yang punya ambang lebih rendah (A ≥ 80, B ≥ 68, C ≥ 60) karena mapel ini lebih sulit dapat nilai tinggi; mapel lain memakai ambang yang sama (A ≥ 90, B ≥ 78, C ≥ 70) — tetap disimpan per-mapel (§2.6) supaya admin bisa menyesuaikan mapel lain di kemudian hari kalau perlu.
- **Format ekspor Dapodik**: untuk saat ini cukup **Excel & PDF standar** (§4) mengikuti layout file contoh; template resmi untuk import Dapodik menyusul begitu tersedia informasinya.

## 8. Masih Perlu Didetailkan

- Template/format resmi import Dapodik (menyusul, lihat §7).
- Format & tata letak PDF Rapor cetak final (kop surat, tanda tangan) — dibahas bersamaan dengan finalisasi §4.16 PRD Utama.
