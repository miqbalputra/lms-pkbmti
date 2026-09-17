# Prioritas Pengembangan Jurnal Pembelajaran

Tanggal: 16 September 2026
Konteks: PKBM Tunas Ilmu Paket A. Jurnal diisi oleh admin/guru; orang tua hanya melihat perkembangan anak yang sudah dipublikasikan. Tidak ada akun peserta didik.

## Kondisi Awal

Fitur **Jurnal Mengajar** sudah tersedia untuk pencatatan per kelas pada hari Sabtu, dengan beberapa baris jam ke/mapel/materi, paraf tutor, foto dokumentasi opsional, sinkronisasi ketidakhadiran dari presensi, ekspor PDF/Word/JPG, pengingat jurnal tertunda, dan laporan kepatuhan.

Fitur ini belum menjadi jurnal pembelajaran lengkap: form belum mencatat tujuan, aktivitas, hasil, atau tindak lanjut secara terstruktur; belum tersambung ke RPP, materi, tugas, kompetensi, dan remedial; serta belum dapat ditampilkan sebagai ringkasan aman kepada orang tua.

## Prinsip Produk

- Jurnal internal guru berbeda dengan informasi yang dilihat orang tua.
- Tutor mengelola data akademik; orang tua hanya melihat data anaknya sendiri yang telah diterbitkan.
- Data cukup dicatat sekali lalu digunakan kembali untuk presensi, tugas, portofolio, laporan, dan rapor.
- Bukti foto/audio/video anak bersifat privat, memakai otorisasi akses dan penyimpanan persisten.

## P0 — Keamanan Akses Jurnal

### Tujuan

Menutup kemungkinan akun orang tua memperoleh jurnal semua kelas melalui endpoint internal.

### Pekerjaan

1. Batasi endpoint daftar jurnal hanya untuk `admin`, `kepala_sekolah`, dan `guru`.
2. Terapkan pemeriksaan peran di setiap endpoint jurnal, bukan hanya menyembunyikan menu pada antarmuka.
3. Guru hanya dapat membaca kelas/mapel yang menjadi penugasannya; admin dan kepala sekolah mengikuti kewenangan masing-masing.
4. Pastikan endpoint foto jurnal mengikuti otorisasi yang sama.
5. Tambahkan pengujian otomatis untuk memastikan token orang tua menerima `403` pada endpoint jurnal internal.

### Kriteria selesai

- Akun orang tua tidak dapat membaca daftar, lembar, ekspor, atau foto jurnal internal.
- Pengujian akses lintas peran lulus.

## P1 — Ringkasan Kegiatan Belajar untuk Orang Tua

### Tujuan

Memberi orang tua gambaran proses belajar anak tanpa membuka jurnal kerja internal tutor.

### Tampilan per anak

- kegiatan dan tujuan belajar minggu/pertemuan ini;
- materi atau tautan bahan pendampingan;
- kehadiran anak pada kegiatan terkait;
- tugas atau bukti belajar yang sudah/belum tercatat;
- catatan tutor yang relevan bagi keluarga;
- tindak lanjut atau dukungan yang diperlukan di rumah.

### Aturan publikasi

- Tutor menyusun jurnal sebagai **draf**.
- Data baru muncul pada portal orang tua setelah status **dipublikasikan**.
- Hanya ringkasan yang ditujukan kepada keluarga yang ditampilkan; catatan internal sekolah tidak ikut dipublikasikan.
- Orang tua hanya dapat melihat anak yang sah terhubung ke akunnya.

### Kriteria selesai

- Orang tua dapat membuka ringkasan kegiatan anak secara aman dari portal.
- Data draf atau catatan internal tidak pernah terlihat oleh orang tua.

## P1 — Form Jurnal Tutor yang Lengkap

### Tujuan

Mengubah jurnal dari daftar materi menjadi catatan pembelajaran yang bermanfaat untuk evaluasi dan pelaporan.

### Field tambahan

| Kelompok | Field |
| --- | --- |
| Perencanaan | tanggal pelaksanaan, rombel, mapel, jam/pertemuan, tujuan belajar, kompetensi/CP, referensi RPP/modul |
| Pelaksanaan | materi, aktivitas/metode, media atau bahan pembelajaran, keterlibatan peserta didik |
| Asesmen | cara cek pemahaman, hasil ringkas kelas, tugas/bukti yang diberikan |
| Refleksi | kendala, hal yang berhasil, kebutuhan penguatan/remedial |
| Komunikasi | ringkasan yang layak dipublikasikan kepada orang tua |

### Catatan desain

- Kolom `kegiatan` yang sudah ada perlu dimanfaatkan oleh form, bukan hanya tersimpan pada struktur lama.
- Gunakan pilihan dari data yang sudah ada untuk rombel, mapel, RPP, materi, tugas, dan kompetensi agar tidak terjadi duplikasi.
- Materi bebas tetap tersedia untuk kondisi khusus.

### Kriteria selesai

- Setiap jurnal dapat menjawab: tujuan apa, kegiatan apa, hasilnya bagaimana, dan tindak lanjutnya apa.

## P2 — Integrasi Data Pembelajaran

### Tujuan

Satu sesi belajar menjadi titik penghubung data operasional yang sudah tersedia.

### Integrasi

1. **Presensi:** tampilkan status kehadiran kelas dan anak tanpa input ulang.
2. **RPP/modul/kompetensi:** tautkan jurnal ke rencana dan capaian yang sedang dipelajari.
3. **Materi:** pilih materi yang sudah diunggah atau catat materi baru dari jurnal.
4. **Tugas:** hubungkan tugas yang diberikan dan status pengumpulan/pemeriksaannya.
5. **Kelas virtual:** kaitkan agenda, tautan, dan ringkasan kegiatan daring bila ada.
6. **Nilai formatif:** simpan hasil cek pemahaman singkat sebagai masukan penguatan, bukan semata nilai akhir.

### Kriteria selesai

- Tutor tidak perlu menulis ulang data yang sama di beberapa menu.
- Riwayat pembelajaran dapat ditelusuri dari sesi ke materi, tugas, presensi, dan capaian.

## P2 — Portofolio dan Bukti Belajar per Anak

### Tujuan

Mendokumentasikan proses belajar Paket A yang sering lebih tepat dibuktikan dengan karya dan praktik.

### Fitur

- Tutor mengunggah foto karya, audio membaca, video praktik pendek, atau berkas hasil kerja.
- Bukti ditautkan ke peserta didik, sesi jurnal, mapel, dan kompetensi.
- Tutor memberi komentar/rubrik sederhana serta status draf atau diterbitkan.
- Orang tua melihat atau mengunduh bukti milik anaknya sendiri.

### Keamanan berkas

- Berkas tidak memakai URL publik permanen.
- Akses berkas diperiksa oleh server setiap kali dibuka.
- Berkas tersimpan dalam volume persisten `/app/uploads` dan tercakup dalam backup terenkripsi.

### Kriteria selesai

- Bukti belajar anak dapat dibuka oleh keluarga yang berhak, tidak oleh keluarga lain.

## P2 — Ketuntasan, Penguatan, dan Remedial

### Tujuan

Membantu tutor menindaklanjuti anak yang perlu pendampingan, bukan hanya mencatat materi yang telah diberikan.

### Fitur

- Status per kompetensi: tuntas, perlu penguatan, atau perlu remedial.
- Rencana tindak lanjut: aktivitas, penanggung jawab, tenggat, dan hasil.
- Dasbor tutor/koordinator untuk anak yang belum memiliki bukti belajar, sering absen, atau belum tuntas.
- Ringkasan tindak lanjut yang sudah diterbitkan tampil bagi orang tua.

### Kriteria selesai

- Anak yang membutuhkan dukungan terdeteksi sebelum penerbitan rapor.
- Status remedial dan hasilnya dapat ditelusuri.

## P3 — Jadwal Fleksibel dan Penguncian Riwayat

### Tujuan

Menyesuaikan jurnal dengan praktik pembelajaran PKBM serta menjaga integritas dokumen historis.

### Pekerjaan

1. Ubah aturan jurnal yang hanya mengizinkan hari Sabtu menjadi jadwal yang dikonfigurasi per rombel/semester, atau izinkan tanggal pelaksanaan dengan alasan perubahan jadwal.
2. Bedakan tanggal rencana dan tanggal pelaksanaan bila kegiatan bergeser.
3. Kunci jurnal setelah periode tertentu atau setelah laporan/rapor diterbitkan.
4. Perubahan setelah terkunci memerlukan alasan, jejak audit, dan bila perlu persetujuan admin.
5. Hapus jurnal historis hanya dengan aturan khusus; lebih aman memakai pembatalan/arsip daripada hapus permanen.

### Kriteria selesai

- Jurnal mengikuti jadwal nyata PKBM tanpa kehilangan kontrol.
- Riwayat periode lama tidak dapat diubah atau dihapus sembarangan.

## Urutan Implementasi

1. **P0:** perbaikan otorisasi endpoint jurnal dan pengujian akses orang tua.
2. **P1:** status draf/publikasi, form jurnal lengkap, dan ringkasan kegiatan untuk portal orang tua.
3. **P2:** integrasi presensi/RPP/materi/tugas/kompetensi, portofolio anak, serta tindak lanjut remedial.
4. **P3:** konfigurasi jadwal yang fleksibel, penguncian periode, dan arsip riwayat.

## Indikator Keberhasilan

- Tidak ada akun orang tua yang dapat mengakses jurnal internal atau data keluarga lain.
- Minimal 90% jurnal memiliki tujuan, aktivitas, hasil, dan tindak lanjut.
- Minimal 75% orang tua membuka ringkasan perkembangan anak pada tiap periode.
- Seluruh berkas bukti belajar berada di penyimpanan persisten dan dapat dipulihkan dari backup.
- Anak yang perlu penguatan/remedial teridentifikasi sebelum rapor diterbitkan.

## Status Implementasi — 17 September 2026

Seluruh prioritas di dokumen ini telah diimplementasikan pada aplikasi.

- **P0:** endpoint jurnal internal, lembar, ekspor, dan foto menolak akun orang tua; akses tutor dibatasi sampai pasangan penugasan **kelas–mapel**.
- **P1:** jurnal mendukung draf/publikasi, tujuan, kegiatan, metode, media, keterlibatan, asesmen, hasil ringkas, refleksi, kendala, tindak lanjut, dan ringkasan aman untuk keluarga. Portal orang tua hanya memuat data terbit milik anak yang terhubung.
- **P2:** satu sesi dapat ditautkan ke RPP, modul, materi, tugas, kompetensi, dan kelas virtual. Presensi serta status tugas ditampilkan kembali pada ringkasan anak. Portofolio privat dan tindak lanjut per mapel tersedia pada menu **Perkembangan Belajar**.
- **P2:** dasbor perkembangan menyoroti remedial/penguatan aktif, anak tanpa portofolio, dan anak dengan minimal tiga ketidakhadiran dalam 30 hari terakhir.
- **P3:** tanggal rencana dan pelaksanaan dapat dibedakan dengan alasan perubahan; jurnal dapat dikunci, dibuka kembali dengan alasan yang tercatat di audit, atau dibatalkan tanpa menghapus riwayat.

### Syarat deploy dan pemulihan

- Pertahankan volume Coolify `pkbm-uploads` pada `/app/uploads`; seluruh foto jurnal, bukti portofolio, RPP, materi, dan berkas unggahan berada di bawah lokasi ini.
- Pertahankan volume `pkbm-backups` pada `/app/backups`; pastikan proses backup offsite terenkripsi mencakup database dan kedua lokasi data tersebut.
- Jangan mengganti atau menghapus volume ketika redeploy. Perubahan skema tabel dijalankan otomatis saat backend versi ini pertama kali aktif.
