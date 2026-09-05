# Hasil Audit LMS PKBM Tunas Ilmu

Tanggal audit: 5 September 2026 (WIB)
Ruang lingkup: source backend Go, frontend React/Vite, konfigurasi deployment, test otomatis, dependensi frontend, dan fitur backup R2 yang ada di worktree.

## Ringkasan Eksekutif

Build aplikasi dan seluruh test backend yang tersedia **lulus**. Fitur utama yang dicakup test mencakup autentikasi, RBAC, master data, kelas, peserta didik, presensi, kenaikan kelas, nilai, jurnal, perpustakaan, ujian, impor, dan audit log.

Namun, pernyataan bahwa *seluruh fitur sudah berfungsi di production* belum dapat dibuktikan sepenuhnya dari mesin audit ini. Docker tidak terpasang; tidak ada kredensial R2/Turnstile/database production untuk pengujian integrasi; dan cakupan statement backend masih 42,6%. Bagian tersebut dicatat sebagai pekerjaan verifikasi sebelum rilis.

## Bukti Verifikasi

| Area | Bukti | Status |
| --- | --- | --- |
| Backend | `go test ./...` di `backend` selesai sukses | Lulus |
| Kualitas Go | `go vet ./...` selesai sukses | Lulus |
| Cakupan backend | `go test -cover ./cmd/server` menghasilkan 42,6% statement coverage | Perlu ditingkatkan |
| Suite API/E2E | Ditemukan 48 entri `TestE2E*` dan 97 entri `Test*`; seluruhnya ikut lulus pada `go test ./...` | Lulus otomatis |
| Frontend | `npm run build` di `frontend` (TypeScript + Vite) selesai sukses | Lulus |
| Dependensi frontend produksi | `npm audit --omit=dev --json`: 0 low/moderate/high/critical | Lulus |
| Konfigurasi Docker | `docker compose config --quiet` tidak dapat dijalankan karena Docker CLI tidak ada pada mesin audit | Belum diverifikasi |
| Layanan eksternal | R2, Turnstile, PostgreSQL production, dan lifecycle R2 tidak dapat diakses tanpa environment/secret production | Belum diverifikasi |

### Fitur yang terbukti lewat test otomatis

- Autentikasi JWT, refresh-token, logout/revocation, lockout, dan batas akses per peran.
- CRUD tutor, orang tua, pokjar, tahun ajaran, kelas, mapel, peserta didik, akun, serta penugasan guru.
- Presensi: validasi tanda tangan, detail kehadiran, rekap, ekspor, batas peran, dan skenario jadwal.
- Kenaikan kelas, riwayat kelas/wali kelas, arsip akademik, impor data atomik, dan audit log.
- Fitur pembelajaran yang memiliki test backend terkait: jurnal, materi, RPP, nilai/rapor, buku, ujian, sertifikat, notifikasi, dan kepatuhan pembelajaran.
- Arsip R2 lokal: database dan lampiran masuk ke arsip terenkripsi; manifest/checksum serta pengaman path diuji.

### Perbaikan laporan presensi yang telah diverifikasi secara build/test

- Kop PDF memakai karakter ASCII aman agar pemisah tidak lagi tampil sebagai `Â`.
- PDF mencantumkan PKBM, periode, waktu cetak, panel **Filter dan Hasil Data**, serta jumlah kelas dan presensi hasil filter.
- Test backend dan build frontend tetap lulus setelah perubahan.

## Temuan dan Optimasi Prioritas

### P0 — wajib sebelum mengklaim siap production

1. **Lakukan smoke test deployment yang nyata.** Docker tidak tersedia pada mesin audit, sehingga image, volume `uploads`, healthcheck, dan koneksi PostgreSQL belum tervalidasi. Jalankan `docker compose config`, build image, start stack, login admin, upload lampiran, restart container, lalu pastikan data dan lampiran masih tersedia.
2. **Uji R2 dengan bucket non-produksi.** Test saat ini memverifikasi format arsip lokal, bukan `PutObject`, `ListObjectsV2`, download, restore, timeout, maupun permission pada bucket Cloudflare. Tambahkan integration test dengan bucket test atau MinIO dan jalankan restore drill terjadwal.
3. **Verifikasi lifecycle dan Bucket Lock di Cloudflare.** `BACKUP_R2_RETENTION_DAYS=36` ditampilkan aplikasi, tetapi penghapusan 36 hari hanya terjadi bila lifecycle rule benar-benar dipasang di R2. Simpan bukti konfigurasi lifecycle/abort multipart/Bucket Lock pada runbook deployment.

### P1 — keamanan dan pemulihan data

1. **Jurnal restore yang durable belum terbukti ada.** Restore database dan swap folder lampiran adalah dua sumber daya berbeda; kegagalan proses di tengah langkah dapat menghasilkan keadaan tidak konsisten. Tambahkan restore journal persisten pada volume backup, fase rollback eksplisit, dan test simulasi crash untuk SQLite serta PostgreSQL.
2. **Tambahkan notifikasi kegagalan backup.** Metrik backup dan job status sudah ada, tetapi belum ada notifikasi aktif saat backup R2 tiga-harian gagal/terlambat. Kirim alert ke email/WhatsApp/Telegram atau webhook admin saat gagal dan saat usia backup terakhir melewati ambang.
3. **Tambahkan drill restore PostgreSQL terisolasi.** `BACKUP_DRILL_DATABASE_URL` tersedia untuk dump database lama, tetapi backup R2 lengkap juga perlu drill database + lampiran agar prosedur pemulihan benar-benar terbukti.
4. **Batasi dan audit akses backup cloud.** Pastikan token S3 R2 hanya untuk satu bucket dan secret disimpan di secret manager. Tetapkan proses rotasi key serta audit akses admin ke endpoint backup/restore.

### P1 — kualitas dan observabilitas

1. **Naikkan coverage backend dari 42,6% ke minimal 70% untuk jalur kritis.** Prioritas: R2 upload/download/restore, report PDF, impor Excel, upload berkas, ekspor, dan error handling PostgreSQL.
2. **Tambahkan observabilitas production.** Health endpoint dan counter internal ada, tetapi belum terlihat error tracking/tracing terpusat. Tambahkan structured logs, alert berbasis health/backup age, dan pelaporan error frontend/backend.
3. **Tambahkan browser E2E.** Build TypeScript membuktikan aplikasi terkompilasi, bukan bahwa tombol, dialog, download, filter, upload, dan RBAC UI bekerja di browser. Gunakan Playwright untuk alur admin, guru, kepala sekolah, orang tua, dan ujian online.

### P2 — performa dan pemeliharaan

1. **Optimalkan bundle frontend.** Bundle utama sekitar 403 kB gzip dan chunk chart sekitar 103 kB gzip. Lazy-load chart berat, ukur dengan Lighthouse, dan tetapkan performance budget.
2. **Tambahkan pagination pada daftar arsip R2.** `ListObjectsV2` perlu continuation token untuk skala arsip di atas 1.000 objek, terutama bila arsip manual atau pre-restore bertambah.
3. **Buat kontrak API terdokumentasi.** Generate OpenAPI untuk endpoint utama termasuk respons error, upload, ekspor, dan R2. Ini mengurangi regresi antara frontend/backend.
4. **Pisahkan dokumentasi status test dari bukti saat ini.** `TEST_READY.md` menyebut angka hasil lama. Jadikan laporan test di CI sebagai artefak otomatis agar angka dan status tidak basi.

## Saran Fitur Berikutnya

1. **Pusat notifikasi operasional:** reminder presensi/jurnal/tugas, escalation berjenjang ke wali kelas lalu admin, kanal WhatsApp/email/Telegram, dan riwayat pengiriman.
2. **Dashboard tindakan harian:** daftar prioritas “perlu diisi hari ini”, filter per pokjar/tutor, tautan langsung ke record, serta SLA keterlambatan.
3. **Approval workflow:** persetujuan jurnal, RPP, nilai, surat, dan perubahan data peserta didik dengan komentar, status, dan audit trail.
4. **Rapor dan ekspor regulasi:** template rapor yang dapat dikonfigurasi, tanda tangan digital/QR, serta ekspor data sesuai kebutuhan Dapodik/PKBM.
5. **Komunikasi wali murid:** portal orang tua dengan notifikasi presensi, nilai, tugas, surat, dan konfirmasi baca.
6. **Analitik risiko peserta didik:** deteksi dini berdasarkan presensi, tugas tertunda, nilai, dan catatan perilaku; tampilkan rekomendasi tindak lanjut.
7. **Mode offline terbatas:** cache jadwal/daftar siswa dan antrean presensi untuk koneksi tidak stabil, lalu sinkronisasi aman saat online.
8. **Manajemen dokumen terpusat:** versi dokumen, masa berlaku, pengingat pembaruan, hak akses granular, dan retensi arsip.

## Checklist Rilis yang Direkomendasikan

- [ ] Docker compose, build image, healthcheck, dan persistence volume diuji pada environment staging.
- [ ] R2 test bucket lulus: backup, list, download, decrypt, restore, rollback, dan lifecycle 36 hari.
- [ ] Turnstile production dan CORS/cookie domain diverifikasi pada domain final.
- [ ] Backup pengaman dan restore drill dibuktikan secara berkala.
- [ ] Alert backup gagal/terlambat aktif dan diuji.
- [ ] Playwright browser E2E untuk alur peran utama lulus di CI.
- [ ] Coverage jalur kritis mencapai target yang disepakati.

## Kesimpulan

Kode saat ini berada pada kondisi **lulus build dan test otomatis yang tersedia**, dengan dependensi frontend produksi bersih. Rilis production sebaiknya menunggu penyelesaian checklist P0, khususnya verifikasi deployment dan restore R2 nyata. Setelah itu, fokus optimasi terbaik adalah ketahanan restore, alert operasional, browser E2E, dan observabilitas.
