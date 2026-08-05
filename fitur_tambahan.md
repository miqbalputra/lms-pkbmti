# Fitur Tambahan — Hasil Analisa SIMPKBM (simpkbm.id)

> **Sumber & metode:** Fitur diturunkan dari manifest rute **Ziggy** yang tertanam di
> HTML `https://simpkbm.id/` (aplikasi Laravel Inertia + multi-tenant SaaS, frontend
> Figtree, antrean Horizon, payment Tripay). Karena halamannya SPA murni, daftar modul
> dibaca dari daftar rute `tenant.*` (sisi pengelola) dan `portal.*` (sisi siswa), lalu
> dibandingkan dengan modul yang **sudah ada** di Tunas Ilmu Learn. Yang ditulis di
> bawah adalah **fitur SIMPKBM yang belum ada / belum seutuhnya ada di aplikasi kita**,
> beserta penjelasan dan rekomendasi penyesuaian ke stack kita (Go Fiber + GORM backend,
> React 19 + Vite + Tailwind frontend).

## Ringkasan modul yang SUDAH ada di aplikasi kita (untuk pembanding)
Tutor, Orang Tua (+ relasi NIK), Pokjar, Tahun Ajaran (+ tanggal mulai semester genap),
Mata Pelajaran, Mapel per Kelas, Penugasan Tutor, Kelas Rombel, Peserta Didik,
Relasi Orang Tua, Presensi Mingguan (+ tanda tangan), Modul Nilai (NP/NK/predikat),
Kenaikan Kelas, Arsip Data, Akun/Users, Pengaturan Jadwal, Pengaturan Nilai (KKM),
Audit Log, Buku + Penetapan Buku + Peminjaman/Pengembalian Buku + Rekap (export
Excel/PDF) + cron reminder n8n.

---

## A. Portal Siswa (belum ada sama sekali)
SIMPKBM punya sisi siswa terpisah (`portal.*`): login PIN, dashboard siswa,
pengumuman, tugas, ujian, materi, kelas virtual, ganti PIN.

- **Status kita:** Tidak ada autentikasi siswa. Peserta didik hanya dikelola admin/guru;
  tidak ada login siswa.
- **Yang perlu dibangun:**
  - Autentikasi siswa (PIN / login) — mirip JWT guru tapi role `peserta_didik`,
    scoped ke data sendiri.
  - Dashboard siswa ringkas (jadwal, tugas aktif, materi, pengumuman).
  - Guard IDOR ketat: siswa hanya boleh melihat data miliknya.
- **Catatan:** Ini prasyarat hampir semua fitur di bawah (Tugas, Ujian, Materi, Kelas
  Virtual, Pengumuman) karena semua punya sisi "siswa mengakses/mengerjakan".

## B. Pengumuman / Announcements (belum ada)
SIMPKBM: `tenant.announcement.*` (admin/tutor buat) → `portal.announcement.index` (siswa lihat).

- **Status kita:** Tidak ada modul broadcast/pengumuman.
- **Yang perlu dibangun:** Tabel `Pengumuman` (judul, isi, target: rombel/semua, tanggal,
  aktif). Endpoint guru/admin CRUD (guru scoped ke rombel walinya), endpoint read-only
  untuk siswa di portal. Bisa pakai cron n8n yang sudah ada untuk reminder.

## C. Tugas (Assignments) — buat, kumpulkan, nilai (belum ada)
SIMPKBM: `tenant.assignment.*` + `tenant.assignment.grade` + `portal.assignment.*`
(submit). **Bukan** "Penugasan Tutor" kita (itu penempatan tutor ke kelas/mapel).

- **Status kita:** "Penugasan" kita = assignment tutor ke kelas. Tidak ada tugas siswa
  (kumpul + nilai).
- **Yang perlu dibangun:**
  - Tabel `Tugas` (mapelId, kelasId, judul, deskripsi, deadline, lampiran?).
  - Tabel `PengumpulanTugas` (tugasId, pesertaDidikId, file/teks, tanggalKumpul, nilai,
    catatan). Anti-double kumpul.
  - Guru upload/tetapkan tugas per kelas+mapel; siswa kumpul di portal; guru nilai
    (grade) per pengumpulan. Masuk ke komponen nilai bila diinginkan.

## D. Ujian Online + Bank Soal (belum ada)
SIMPKBM: `tenant.exam.*` + `tenant.exam.questions` + `portal.exam.*` (submit) +
`tenant.question-bank.*`.

- **Status kita:** Tidak ada ujian online maupun bank soal.
- **Yang perlu dibangun:**
  - `BankSoal` (mapelId, tipe: pg/essay, pertanyaan, opsi, kunci, poin) — reuse oleh
    banyak ujian.
  - `Ujian` (mapelId, kelasId, judul, waktu mulai/selesai, durasi, acak soal).
  - `UjianSoal` (relasi ujian↔soal, bobot).
  - `JawabanUjian` (ujianId, pesertaDidikId, soalId, jawaban, skor, status: sedang/selesai).
  - Siswa kerjakan di portal dengan timer sisi-klien + validasi deadline sisi-server.
  - Auto-koreksi PG, manual essay; hasil bisa masuk komponen nilai.

## E. Materi Pembelajaran (belum ada)
SIMPKBM: `tenant.material.*` (CRUD + download + komentar) + `portal.material.*`.

- **Status kita:** Tidak ada modul materi/file pembelajaran.
- **Yang perlu dibangun:**
  - `Materi` (mapelId, kelasId, judul, deskripsi, filePath, tipe, ukuran).
  - Upload file (penyimpanan lokal / static serve via Fiber), download scoped,
  - Komentar per materi (diskusi). Bisa jadi lampiran di kelas virtual.

## F. Kelas Virtual (belum ada)
SIMPKBM: `tenant.virtual-class.*` + `portal.virtual-class.index`.

- **Status kita:** Tidak ada sesi belajar online terjadwal.
- **Yang perlu dibangun:** `KelasVirtual` (mapelId, kelasId, judul, linkMeeting,
  jadwalMulai/selesai, deskripsi). Siswa lihat & ikut dari portal. Opsional integrasi
  link Zoom/Meet (tidak perlu video server sendiri).

## G. Catatan Perilaku / Kepribadian (belum ada)
SIMPKBM: `tenant.behavior.*` (index, store).

- **Status kita:** Tidak ada catatan perilaku/karakter siswa.
- **Yang perlu dibangun:** `CatatanPerilaku` (pesertaDidikId, tanggal, kategori:
  positif/negatif, deskripsi, dicatatOleh). Tampil di rapor / portal ortu bila nanti ada.

## H. Ijazah / Sertifikat + Cetak (belum ada)
SIMPKBM: `tenant.certificate.*` (index, store, print).

- **Status kita:** Tidak ada cetak ijazah/sertifikat kelulusan.
- **Yang perlu dibangun:** `Sertifikat` (pesertaDidikId, program, nomor, tanggalTerbit,
  status). Generate PDF berstempel/QR verifikasi (gofpdf sudah ada; bisa tambah
  go-qrcode). Nomor sertifikat unik anti-duplikat.

## I. Rapor (Report Card) — isi & cetak (belum utuh)
SIMPKBM: `tenant.report-card.*` (fill, update, print per siswa).

- **Status kita:** Ada Modul Nilai (NP/NK/predikat) tapi belum ada **rapor formal**
  (halaman rapor lengkap + cetak PDF per siswa).
- **Yang perlu dibangun:** View rapor per siswa: data identitas, kelas, tahun ajaran,
  daftar mapel + nilai akhir + predikat + keterangan, catatan kepribadian (butuh G),
  kenaikan. Cetak PDF (gofpdf) per siswa. Reuse data nilai yang sudah ada.

## J. Pusat Laporan (Report Center) terpusat (belum ada)
SIMPKBM: `tenant.report.*` (index, export, print, download per report_export).

- **Status kita:** Export berserak per modul (nilai, presensi, rekap buku). Tidak ada
  satu pusat laporan.
- **Yang perlu dibangun:** Halaman "Laporan" terpusat: pilih jenis laporan
  (rekap nilai, presensi, peminjaman buku, siswa per pokjar, dll) + filter periode +
  export Excel/PDF. Bisa jadi aggregator yang memanggil handler export yang sudah ada.

## K. Jurnal Mengajar + Approval + Foto (belum ada)
SIMPKBM: `tenant.journal.*` (create/edit/destroy + **approve/reject** + **photo**).

- **Status kita:** Tidak ada jurnal mengajar tutor.
- **Yang perlu dibangun:** `JurnalMengajar` (tutorId, mapelId, kelasId, tanggal,
  materi/kegiatan, foto bukti, status: pending/disetujui/ditolak, catatanReviewer).
  Tutor isi (upload foto), kepala_sekolah approve/reject. Bisa jadi dasar evaluasi tutor.
  Cron reminder n8n bisa dipakai untuk pengingat tutor mengisi jurnal.

## L. Modul Pembelajaran + Outcomes (belum ada)
SIMPKBM: `tenant.module.*` + `tenant.module.outcome.*`.

- **Status kita:** "Modul Nilai" kita = modul penilaian, **bukan** modul pembelajaran
  dengan capaian/outcomes.
- **Yang perlu dibangun:** `ModulBelajar` (mapelId, judul, urutan) +
  `CapaianModul` (outcome/kriteria ketercapaian). Bisa dikaitkan ke Materi (E) dan
  Tugas (C).

## M. Kompetensi/Skill + Outcomes + Nilai Keterampilan terstruktur (belum utuh)
SIMPKBM: `tenant.skill.*` + `skill.outcome.*` + `tenant.skill-score.*` +
`study-group-skill.*`.

- **Status kita:** Ada NK (nilai keterampilan) dari rata-rata per tema, tapi tidak ada
  manajemen **kompetensi + outcomes** eksplisit yang bisa ditugaskan per rombel.
- **Yang perlu dibangun:** `Kompetensi` (mapelId, nama) + `CapaianKompetensi`, relasi
  ke rombel, dan `NilaiKompetensi` per siswa per kompetensi. Memperkaya modul nilai
  yang sudah ada tanpa mengubah struktur lama.

## N. Fase Kurikulum (A–E) (belum ada)
SIMPKBM: `tenant.phase.*`.

- **Status kita:** Jenjang kita 1–6 (model SD reguler). PKBM biasanya pakai **Paket
  A/B/C** yang berkorelasi dengan Fase Kurikulum Merdeka (A–E), bukan jenjang 1–6.
- **Yang perlu dibangun:** Tabel `Fase` (A–E) + relasi ke jenjang/program. Pertimbangan
  apakah perlu mengganti model jenjang 1–6 → Paket/Fase (lihat O). **Perlu keputusan
  bisnis** karena menyentuh data kelas & kenaikan yang sudah berjalan.

## O. Program Pendidikan (Paket A/B/C) (belum ada)
SIMPKBM: `tenant.program.*`.

- **Status kita:** Tidak ada konsep Program (Paket A/B/C). Hanya jenjang rombel.
- **Yang perlu dibangun:** `Program` (nama: Paket A/B/C, jenjang setara, keterangan).
  Relasi ke kelas & peserta didik. Penting untuk keabsahan PKBM (ijazah Paket).
  Berkaitan dengan N (Fase) dan H (Sertifikat).

## P. Kartu Pelajar (Student Card) + cetak (belum ada)
SIMPKBM: `tenant.student-card.*` (per siswa, per group/rombel).

- **Status kita:** Tidak ada cetak kartu pelajar.
- **Yang perlu dibangun:** Generate kartu pelajar PDF (foto, NIS/NISN, nama, kelas,
  tahun ajaran, QR verifikasi) per siswa maupun massal per rombel. gofpdf + go-qrcode.

## Q. Presensi berbasis QR (scan + qr-sheet) (belum ada — alternatif model)
SIMPKBM: `tenant.attendance.qr-sheet` + `tenant.attendance.scan` + `close/reopen`.

- **Status kita:** Presensi Mingguan berbasis **tanda tangan digital** guru per
  pertemuan. Tidak ada self-service scan QR oleh siswa.
- **Yang perlu dibangun (opsional):** QR sheet per rombel (kode unik per sesi, kedaluwarsa),
  endpoint scan (siswa/portal tandatangan digital via kamera), close/reopen sesi.
  Bisa **berdampingan** dengan presensi tanda tangan yang sudah ada (tidak menggantikan).

## R. Import Data terpusat + template (belum utuh)
SIMPKBM: `tenant.import.*` (index, store, template per type).

- **Status kita:** Hanya import peserta didik.
- **Yang perlu dibangun:** Pusat import multi-tipe (tutor, mapel, nilai, kelas) dengan
  unduh template per tipe (excelize). Validasi baris + laporan error per baris.

## S. Pengaturan Sumber Nilai (Grade Sources) (belum utuh)
SIMPKBM: `tenant.grade-source.*`.

- **Status kita:** "Pengaturan Nilai" mengatur KKM/komponen. Mungkin sudah tercakup
  sebagian, tapi SIMPKBM memisahkan "sumber nilai" sebagai entitas yang dapat dikustom.
- **Yang perlu dibangun (bila perlu):** Konfigurasi sumber/komponen nilai (UM, tugas,
  ujian, praktik) dengan bobot dipetakan per mapel. Cek apakah pengaturan-nilai kita
  sudah cukup fleksibel; bila ya, cukup di-_flag_ sebagai sudah ada.

## T. Pembayaran Langganan + Tripay (SaaS) — di luar cakupan
SIMPKBM: `tenant.subscription.*` + `webhook.tripay` + `operator.subscription-plan.*`
+ `operator.invoice.*` (sisi operator multi-tenant).

- **Status kita:** Aplikasi single-institution, gratis, tanpa subscription.
- **Catatan:** Ini fitur **multi-tenant SaaS** (penyedia menjual langganan ke banyak
  PKBM + payment gateway). **Tidak relevan** untuk aplikasi internal satu PKBM seperti
  kita, kecuali suatu hari ingin dikomersialkan. **Direkomendasikan dilewati** untuk
  implementasi internal.

---

## Prioritas implementasi (usulan)
1. **A Portal Siswa + autentikasi PIN** — fondasi; banyak fitur lain bergantung.
2. **B Pengumuman** — kecil, cepat, langsung berguna setelah portal ada.
3. **C Tugas + G Pengembalian/nilai** — inti KBM daring setelah portal.
4. **D Ujian + Bank Soal** — nilai tinggi, kompleks (timer + auto-koreksi).
5. **K Jurnal Mengajar** — mandiri, tidak butuh portal siswa, cepat berdampak.
6. **I Rapor cetak** — reuse data nilai yang ada, langsung berguna.
7. **H Sertifikat + P Kartu Pelajar** — administratif, butuh O Program dulu.
8. **O Program (Paket A/B/C) + N Fase** — keputusan bisnis; sentuh data lama.
9. **E Materi, F Kelas Virtual, L Modul, M Kompetensi** — pengayaan pembelajaran.
10. **J Pusat Laporan, R Import terpusat** — konsolidasi setelah modul bertambah.
11. **G Perilaku, Q Presensi QR, S Grade Sources** — opsional/percaya diri.

## Catatan akhir
- Analisa ini berdasarkan **nama rute** (endpoint) SIMPKBM, bukan tampilan layar —
  jadi ada kemungkinan detail UI/alur sedikit berbeda dari implementasi sesungguhnya.
  Nama rute cukup andal untuk memetakan **modul dan kapabilitas** yang ditawarkan.
- Beberapa fitur SIMPKBM sudah tercakup sebagian di aplikasi kita (KKM via
  pengaturan-nilai, study-group-subject via kelas-mapel, teaching via penugasan+jadwal,
  import siswa). Itu ditandai "belum utuh" bila hanya sebagian.
- Fitur multi-tenant/subscription (T) sengaja dipisah karena arsitektur beda
  (SaaS vs internal single-tenant).