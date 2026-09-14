# Hasil Audit LMS PKBM Tunas Ilmu

> **Addendum 14 September 2026:** tindak lanjut hardening pada worktree ini telah
> diterapkan. Status terbaru berada pada bagian [Pembaruan Audit](#pembaruan-audit-14-september-2026); bagian di bawahnya menyimpan baseline audit
> 5 September untuk histori dan tidak boleh dibaca sebagai status terbaru.

## Pembaruan Audit 14 September 2026

Perubahan yang sudah diterapkan tanpa menghapus atau memigrasikan data existing:

- **Security:** validasi status aktif dan role dari database pada setiap JWT request,
  hardening header HTTP, HSTS production, validasi konfigurasi secret/domain, serta
  pencegahan JWT pada URL SSE memakai credential satu kali dengan TTL 60 detik.
- **Backup/restore:** folder backup dipersempit ke permission private, path filesystem
  tidak lagi dikirim ke browser, endpoint `GET /api/backup/offsite` menghasilkan
  `.db.enc`/`.sql.enc` AES-256-GCM untuk Google Drive/S3-compatible, dan upload restore
  terenkripsi dapat langsung didekripsi serta divalidasi sebelum diterapkan.
- **Scalability:** daftar arsip R2 memakai `ListObjectsV2` pagination dengan
  continuation token; UI tetap menampilkan backup lokal meski R2 sedang unavailable.
- **UX/accessibility:** skip link keyboard, landmark semantik, label kontrol, fallback
  polling notifikasi, error state dashboard dengan aksi retry, dan panduan offsite yang
  dapat diikuti operator.
- **Operational quality:** CI mempertahankan backend test/vet, frontend lint/build,
  MinIO R2 integration, Docker smoke/Playwright, lalu menambah audit dependency,
  concurrency, dan artefak coverage.
- **Deployment hardening:** image production memakai user non-root, ownership volume
  lama dialihkan secara aman saat startup, Compose memakai `no-new-privileges`, dan
  `.dockerignore` mencegah secret/data lokal masuk ke build context.
- **Deployment configuration:** Compose sekarang fail-fast bila `DATABASE_URL`, secret
  auth, CORS, atau password PostgreSQL belum diisi; contoh environment juga mencakup
  kredensial service database bawaan secara eksplisit. `PUBLIC_BASE_URL`, TTL JWT,
  dan endpoint R2 juga diteruskan ke container agar validasi production dan konfigurasi
  backup tidak berbeda antara file `.env` dan runtime. Versi builder Go Docker kini
  selaras dengan Go version pada CI dan `go.mod`.
- **Public-session hardening:** sesi ujian online dan share materi memakai cookie
  HttpOnly bertanda tangan; kredensial/password tidak lagi diteruskan oleh halaman
  baru melalui URL. Link share materi lama dengan `?pwd=` tetap diterima sekali untuk
  kompatibilitas lalu di-upgrade ke sesi cookie.
- **Production privacy/configuration:** koneksi PostgreSQL production wajib
  `sslmode=require`, `verify-ca`, atau `verify-full`; logger SQL GORM dinonaktifkan di
  production agar NISN/email tidak masuk ke log query.
- **Request resilience:** request normal dibatasi 16 MiB, sedangkan restore/ZIP besar
  tetap memakai allowlist route eksplisit; endpoint unlock share dibatasi 10 percobaan
  per menit; ekstraksi R2 memiliki batas total dan jumlah file untuk mencegah archive bomb.
- **Privacy/observability:** QR kartu pelajar baru memakai token verifikasi siswa
  bertanda tangan dan URL NISN lama tetap kompatibel; endpoint verifikasi diberi rate
  limit. Request ID disanitasi sebelum masuk access log, dikembalikan pada error, dan
  error 5xx hanya mencatat metadata operasional yang aman.
- **Restore hygiene:** workspace dekripsi/ekstraksi R2 dibersihkan jika gagal sebelum
  journal durable tersimpan; setelah journal tersimpan, artefak tetap dipertahankan
  untuk recovery lintas restart. Swap file/folder idempotent terhadap crash setelah
  rename selesai dan safety copy tidak ditimpa.
- **Upload lifecycle:** penggantian lampiran materi, tugas, RPP, jurnal, dan foto siswa
  sekarang menyimpan file baru serta data DB terlebih dahulu, menghapus file lama hanya
  setelah commit berhasil, dan membersihkan file baru bila penyimpanan DB gagal.
- **Metadata minimization:** respons restore PostgreSQL, audit manual backup, dan log
  safety restore hanya menyebut nama file relatif, bukan path filesystem host.
- **Public exam contract:** daftar ujian publik dan hasil ujian orang tua kini memakai
  DTO allowlist sehingga `aksesKode` serta metadata internal tidak ikut terkirim;
  jawaban tersimpan juga dipetakan ke ID `UjianSoal` yang digunakan frontend.
- **Parent privacy contract:** daftar anak, presensi, tugas, materi, peminjaman, dan
  perilaku pada portal orang tua memakai DTO allowlist; NIK, path file, share token,
  operator ID, dan payload tanda tangan tidak ikut keluar lewat JSON.
- **Backup operations:** kegagalan setiap tahap backup terjadwal dicatat sebagai
  metrik/audit dan dikirim ke webhook operasional; webhook hanya mendinginkan alert
  setelah respons 2xx, dapat mencoba ulang setelah gagal, dan wajib HTTPS di production.
  Health/monitoring juga memeriksa umur backup lokal otomatis saat `BACKUP_CRON` aktif;
  artefak yang gagal diverifikasi tidak dihitung sebagai backup sehat.
- **Parent session lifecycle:** tombol keluar portal orang tua sekarang memanggil endpoint
  logout server untuk mencabut sesi dan cookie refresh, lalu membersihkan state lokal.
- **CSP compatibility:** halaman ujian publik dan portal orang tua tidak lagi memakai
  inline event handler; interaksi memakai `data-action` dengan event delegation sehingga
  dapat berjalan di bawah CSP production tanpa `unsafe-inline`.

### Bukti verifikasi terbaru

| Area | Bukti | Status |
| --- | --- | --- |
| Backend regression | `go test ./... -count=1` | Lulus |
| Go static checks | `go vet ./...` dan `git diff --check` | Lulus |
| Concurrency regression | `go test -race ./...` | Gate CI Linux; belum dapat dijalankan lokal karena compiler C (`gcc`) tidak tersedia |
| Kualitas Go | `go vet ./...` | Lulus |
| Offsite backup | Test membuat `.sql.enc`, memastikan plaintext tidak bocor, decrypt, dan validasi SQL | Lulus |
| SSE credential | Test ticket satu kali pakai | Lulus |
| Auth session lifecycle | Logout mencabut refresh token dan menghapus cookie pada path scoped | Lulus |
| Frontend | `npm.cmd run lint` dan `npm.cmd run build` | Lulus |
| UI visual/accessibility spot check | Preview frontend terbaru; login layout, heading, label, field, toggle password, dan tombol terdeteksi | Lulus |
| Dependency production | `npm.cmd audit --omit=dev --json` | 0 vulnerability low/moderate/high/critical |
| Public exam/share/privacy regression | Cookie session, URL bersih, grading unanswered, handler JS, signed QR siswa | Lulus |
| Backend coverage | `go test -cover ./cmd/server` | 46,8%; masih di bawah target 70% jalur kritis |
| Docker/cloud nyata | Docker CLI, kredensial R2/Google Drive, dan production DB tidak tersedia di mesin audit | Wajib staging |

### Gate rilis yang masih terbuka

1. Jalankan `docker compose config`, build/start stack, login, upload lampiran, restart,
   dan verifikasi volume pada staging yang memasang Docker.
2. Uji real R2 dan Google Drive/n8n: upload, list pagination, download, decrypt,
   restore database + lampiran, rollback, lifecycle, serta Bucket Lock.
3. Aktifkan alert ketika backup gagal atau umur backup melewati SLA; bukti konfigurasi
   lifecycle/retensi storage harus disimpan operator.
4. Naikkan coverage jalur kritis dan jalankan Playwright lintas role pada environment
   terisolasi sebelum multi-sekolah production.
5. **Tetapkan batas tenancy.** Skema saat ini single-school per database; multi-sekolah
   dalam satu database belum aman tanpa penambahan tenant boundary, backfill, dan
   pengujian isolasi per role. Gunakan stack/database terpisah per sekolah untuk
   rilis sekarang.

Tanggal audit baseline: 5 September 2026 (WIB)
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
