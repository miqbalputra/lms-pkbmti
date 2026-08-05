# Progress Pengembangan LMS PKBM Tunas Ilmu

## Selesai
- Fondasi monorepo React/Vite dan Go Fiber/GORM dengan SQLite lokal serta dukungan PostgreSQL melalui `DATABASE_URL`.
- Migrasi dan seed Pokjar, tahun ajaran aktif, pengaturan jadwal, dan akun admin development.
- Autentikasi JWT, refresh-token cookie, bcrypt, CORS, security headers, rate limit, RBAC, dan audit log.
- Master data Tutor, Orang Tua, Pokjar, Tahun Ajaran, Kelas, Mapel, Peserta Didik, Akun, dan penugasan guru.
- Presensi scheduler WIB, checklist kehadiran, tanda tangan canvas, edit pertemuan otomatis, dan export CSV.
- Kenaikan kelas massal dengan override per peserta didik dan riwayat kelas.
- Import peserta didik `.xlsx` dengan validasi atomik per baris.
- Pengaturan jadwal KBM: hari default, jam generate, dan zona waktu `Asia/Jakarta`.
- Dashboard statistik per pokjar dan rombel memakai Recharts, dengan data guru dibatasi ke rombel wali kelasnya.
- Export PDF presensi per pertemuan, termasuk detail kehadiran dan tanda tangan wali kelas.
- Audit log Admin dengan filter aksi/modul untuk menelusuri aktivitas sensitif.
- Grafik dashboard dimuat secara lazy-load agar Recharts tidak masuk bundle awal.
- Pengaturan mata pelajaran per rombel untuk menjadi dasar penugasan guru mapel.
- Kepala Sekolah dapat membaca seluruh master data dan audit log, tetap tanpa akses tulis.
- Validasi akun mewajibkan email agar constraint unik konsisten dan akun tanpa email tidak dapat dibuat.
- Riwayat wali kelas per rombel dapat dilihat dari halaman Kelas.
- Admin dapat mengganti wali kelas dari halaman Kelas; backend otomatis menjaga riwayat pergantian.
- Test backend terisolasi memverifikasi pergantian wali kelas: pointer kelas berubah, riwayat lama ditutup, dan riwayat baru aktif dibuat.
- Constraint unik rombel diberlakukan untuk kombinasi jenjang, nama rombel, pokjar, dan tahun ajaran.
- UI master data Tutor, Orang Tua, Pokjar, Tahun Ajaran, dan Mapel mendukung tambah, ubah, dan hapus.
- Arsip mengikuti navigasi Tahun Ajaran lalu Semester dan menampilkan konteks riwayat kelas serta presensi periode.
- Rekap presensi per rombel dan semester menampilkan Hadir/Sakit/Izin/Alpa per peserta didik.
- Hardening presensi: tanda tangan wajib berupa PNG base64 valid dan dibatasi ukuran maksimal 1 MB.
- Access token frontend hanya disimpan di memori; sesi dipulihkan lewat refresh-token httpOnly cookie.
- Test backend mencakup rotasi refresh token, penolakan reuse refresh token, serta guard Kepala Sekolah read-only.
- Artefak deployment Docker Compose dan Nginx HTTPS ditambahkan; secret hanya disediakan melalui environment server.
- Test integritas/security mencakup IDOR guru lintas rombel, import Excel atomik, dan promosi yang menolak rombel dari tahun ajaran lain.
- Template import peserta didik tersedia sebagai file `.xlsx` asli dan penugasan guru mendukung aksi ke semua rombel yang memiliki mapel.
- Panduan menjalankan dan memverifikasi aplikasi lokal tersedia di `README.md`.
- Rekap presensi per semester dapat diexport sebagai PDF.
- Navigasi Guru dibatasi ke dashboard, rombel wali kelas, peserta didik rombel, dan presensi.
- Peserta didik kini mendukung tambah, ubah, dan hapus dari UI Admin.
- UI dimigrasikan ke fondasi shadcn/ui: Tailwind v4, token OKLCH, Button/Card/Dialog primitives, sidebar responsif, login dan tabel/form konsisten.
- Admin dapat menghapus rombel/penugasan dan mengubah akun termasuk reset password.
- Perombakan UI menyeluruh ke komponen shadcn/ui resmi lengkap dengan AlertDialog modal konfirmasi hapus, Sonner toast notifications, penyesuaian layout responsif, dan konsistensi token OKLCH.

## Status Implementasi Saat Ini
- Semua fitur yang memiliki aturan lengkap pada PRD utama telah diimplementasikan dan diverifikasi secara lokal.
- Modul Nilai dan Rapor tetap ditunda hingga PRD khusus menyediakan komponen, bobot, KKM, agregasi, serta template cetak resmi.
- Test backend mencakup rotasi refresh token, penolakan reuse refresh token, serta guard Kepala Sekolah read-only.
- Artefak deployment Docker Compose dan Nginx HTTPS ditambahkan; secret hanya disediakan melalui environment server.

## Verifikasi Terakhir
- Backend: `go test ./...`
- Frontend: `npm.cmd run build`

## Belum Dikerjakan
- Template rapor dan export PDF rapor.
- Modul nilai dan rapor: menunggu PRD khusus.
- Validasi deployment Docker/VPS, HTTPS domain nyata, PostgreSQL production, dan backup terjadwal.

## Catatan Batas PRD
- Nilai dan rapor sengaja belum diimplementasikan karena PRD utama menyatakan komponen nilai, bobot, KKM, agregasi, serta format rapor akan disediakan dalam PRD terpisah.
- Aplikasi telah divalidasi berjalan lokal melalui backend health endpoint dan build frontend. Docker tidak tersedia pada workspace ini sehingga validasi Compose dilakukan saat environment Docker/VPS tersedia.
