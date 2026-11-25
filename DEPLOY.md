# Deployment Guide - Coolify

## Prerequisites

- Coolify instance running
- Domain name configured
- Cloudflare Turnstile (required in production, optional in local development)

## 1. Create New Project in Coolify

1. Go to **Applications** → **New Application**
2. Select **Git Repository**
3. Repository: `https://github.com/miqbalputra/lms-pkbmti.git`
4. Branch: `main`

## 2. Configure Build

- **Build Pack**: Dockerfile
- **Dockerfile Location**: `/Dockerfile`

## 3. Add PostgreSQL Database

1. Go to **Databases** → **New Database**
2. Select **PostgreSQL**
3. Set name: `pkbm-db`
4. Note the internal connection details

The production image runs the application as an unprivileged `lms` user
(UID/GID 10001). On startup, `deploy/entrypoint.sh` transfers ownership of the
existing `backupdata` and `uploadsdata` volumes before dropping privileges. This
is an ownership migration only: it does not delete, rename, or rewrite data.
Never remove those volumes during a redeploy.

Until an explicit tenant-boundary migration is completed, one production stack
and one PostgreSQL database must serve one school/PKBM. The schema intentionally
retains its existing single-school data model; do not place multiple schools in
the same database and rely on UI filtering. Serving many schools is supported
through isolated stacks/databases, or requires a separately planned tenant
migration with row-level isolation and data backfill.

## 4. Configure Environment Variables

Add these environment variables in your application:

| Variable | Description | Example |
|----------|-------------|---------|
| `APP_ENV` | Environment (`development`, `test`, or `production`) | `production` |
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection; production wajib TLS eksplisit | `postgres://user:pass@db-host:5432/pkbm?sslmode=require` |
| `DB_MAX_OPEN_CONNS` | Batas koneksi database aktif | `25` |
| `DB_MAX_IDLE_CONNS` | Batas koneksi idle database | `10` |
| `JWT_ACCESS_SECRET` | Random 32+ chars | `your-secret-key-here` |
| `JWT_REFRESH_SECRET` | Random 32+ chars | `another-secret-key` |
| `JWT_ACCESS_TTL` | TTL access token (maks. 24 jam) | `15m` |
| `JWT_REFRESH_TTL` | TTL refresh token (lebih lama dari access, maks. 365 hari) | `168h` |
| `ADMIN_DEFAULT_PASSWORD` | Password admin saat first start | Wajib kuat, minimal 12 karakter |
| `CORS_ALLOWED_ORIGINS` | Your domain | `https://lms.example.com` |
| `COOKIE_DOMAIN` | Your domain | `lms.example.com` |
| `PUBLIC_BASE_URL` | Origin publik untuk QR/link verifikasi | `https://lms.example.com` |
| `TURNSTILE_SECRET_KEY` | Cloudflare Turnstile secret | Required in production |
| `TURNSTILE_SITE_KEY` | Cloudflare Turnstile site key for public pages | Required in production |
| `VITE_TURNSTILE_SITE_KEY` | Same public site key for the main login build | Required in production |
| `BACKUP_CRON` | Jadwal backup otomatis | `0 2 * * *` |
| `BACKUP_FORMAT` | Format backup penuh | `full` |
| `BACKUP_RETENTION` | Jumlah backup otomatis yang disimpan | `14` |
| `BACKUP_AUTO_RESTART` | Restart otomatis setelah upload restore | `true` |
| `BACKUP_MAX_UPLOAD_MB` | Batas upload file restore | `512` |
| `BACKUP_MAX_ARCHIVE_MB` | Batas total data hasil ekstraksi arsip R2 | `4096` |
| `BACKUP_API_KEY` | Key untuk download backup via n8n | Optional |
| `N8N_PRESENSI_API_KEY` | API key khusus workflow pengingat presensi tutor | Wajib jika workflow n8n presensi digunakan |
| `STUDENT_MCP_API_KEY` | Kunci khusus MCP peserta didik, 32 byte acak dalam base64url tanpa padding | Kosong menonaktifkan integrasi |
| `STUDENT_MCP_ALLOW_WRITE` | Izinkan penerapan batch yang sudah ditinjau | `false` secara bawaan |
| `BACKUP_OFFSITE_URL` | Endpoint S3 presigned/n8n untuk arsip terenkripsi | Optional |
| `BACKUP_OFFSITE_METHOD` | Metode upload offsite | `PUT` |
| `BACKUP_OFFSITE_TOKEN` | Token gateway offsite | Optional |
| `BACKUP_OFFSITE_TIMEOUT` | Timeout upload offsite | `5m` |
| `BACKUP_ENCRYPTION_KEY` | Kunci enkripsi backup offsite | Wajib jika offsite aktif |
| `BACKUP_DRILL_DATABASE_URL` | Database PostgreSQL disposable untuk restore drill | Optional, sangat disarankan |

**Note**: Coolify can reference database variables automatically using Coolify's variable syntax.

### Workflow n8n Pengingat Presensi Tutor

1. Buat nilai acak yang panjang (minimal 32 karakter), lalu isi nilai yang sama sebagai `N8N_PRESENSI_API_KEY` pada service LMS dan service n8n.
2. Redeploy/restart LMS agar endpoint `GET /api/automation/presensi-reminders` aktif dengan key tersebut.
3. Impor `n8n-presensi-tutor-reminder-workflow.json` ke n8n.
4. Pada n8n, isi `LMS_API_BASE_URL` (opsional; default `https://edu.pkbmtunasilmu.web.id/api`) dan `LMS_WEB_URL` (opsional).
5. Pilih credential GOWA pada node **Kirim WhatsApp via GOWA**, jalankan manual sekali, lalu aktifkan workflow.

Workflow memakai header `X-Automation-Key`; jangan memakai akun/password admin karena login production membutuhkan Turnstile. Endpoint hanya mengirim ringkasan kelengkapan per tutor dan tidak mengirim foto Base64, tanda tangan, atau identitas peserta didik.

### MCP Peserta Didik

1. Sebelum rilis, buat backup baru dan periksa ukurannya. Pertahankan database serta volume yang sama. Ikuti [Runbook Keamanan Data](DATA_SAFETY_RUNBOOK.md).
2. Buat kunci khusus dari 32 byte acak dengan format base64url tanpa padding. Simpan `STUDENT_MCP_API_KEY` sebagai secret runtime pada LMS dan berikan nilai yang sama ke proses Codex setempat melalui penyimpanan secret operator. Kunci dikirim hanya dalam header `Authorization` lewat HTTPS. Jaga port Go agar hanya dapat dijangkau proxy yang menetapkan `X-Forwarded-Proto` dari koneksi HTTPS masuk.
3. Buka Codex dari akar proyek tepercaya dengan Go 1.25 atau lebih baru. Konfigurasi proyek mendaftarkan `peserta_didik_lms` dan meneruskan nama variabel secret dari lingkungan proses. Atur `LMS_STUDENT_MCP_URL` di konfigurasi proyek bila targetnya staging. Percobaan lewat HTTP dibatasi ke alamat loopback pada klien.
4. Dengan `STUDENT_MCP_ALLOW_WRITE=false`, gunakan `peserta_didik_kelas_aktif`, `peserta_didik_cari`, dan `peserta_didik_preview` untuk memeriksa alur terhadap target. Simpan secret di luar Git, URL, dan chat.
5. Untuk mengizinkan impor, isi `STUDENT_MCP_ALLOW_WRITE=true` dan deploy ulang. Tinjau pratinjau beserta konflik. Kirim ulang batch yang identik dengan token pratinjau dalam 15 menit memakai `peserta_didik_apply`. Konfigurasi Codex proyek meminta persetujuan untuk tool tulis. ID batch dan jumlah perubahan dicatat pada Audit Log dengan aksi `mcp_apply`.

Pencarian dan perubahan menerima hingga 100 baris per permintaan. Untuk peserta lama, gunakan ID internal hasil pencarian, nama dan kelas yang sudah dicocokkan, serta NIK dari sumber bila ada nama yang sama dalam kelas. Pertahankan NIS/NISN sebagai teks agar nol di depan tetap utuh. Pembaruan hanya menulis NIS dan NISN pada ID yang sama. Peserta baru harus masuk ke kelas aktif dan mendapatkan ID serta riwayat kelas baru. Kelompok belajar diturunkan dari kelas. Nomor NISN sementara dipakai bila peserta baru belum memiliki NISN resmi. NISN baru dipakai saat login Portal Orang Tua berikutnya.

API memakai transaksi untuk seluruh batch dan memeriksa ulang data sebelum menyimpan. Konflik identitas, perubahan sejak pratinjau, atau kegagalan pencatatan audit membatalkan batch. Untuk mencabut kemampuan tulis, set `STUDENT_MCP_ALLOW_WRITE=false` dan deploy ulang. Kosongkan atau ganti `STUDENT_MCP_API_KEY` bila akses perlu dicabut atau kunci perlu dirotasi.

### Backup Offsite dan Restore Drill

Jika `BACKUP_OFFSITE_URL` diisi, setiap backup terjadwal dienkripsi AES-256-GCM lalu dikirim sebagai body binary ke endpoint tersebut. Endpoint dapat berupa gateway n8n, storage service internal, atau URL S3 yang memang dikelola untuk upload berulang. Simpan `BACKUP_ENCRYPTION_KEY` di secret manager; file `.enc` tidak dapat dipulihkan tanpa kunci itu.

Isi `BACKUP_DRILL_DATABASE_URL` dengan database PostgreSQL disposable yang terpisah dari production. Backup terjadwal akan direstore ke database tersebut dengan `psql` sebelum dicatat sebagai backup berhasil.

Panduan lengkap konfigurasi, endpoint, pengujian, dan restore tersedia di [BACKUP_OFFSITE_GUIDE.md](BACKUP_OFFSITE_GUIDE.md).

## 5. Configure Domain

1. Go to **Application** → **Networking**
2. Add your domain (e.g., `lms.example.com`)
3. Enable SSL (Let's Encrypt or your own certificate)

### HTTPS, TLS, dan aset lokal

Untuk domain yang diproksi Cloudflare, pengaturan TLS publik ada di Cloudflare,
bukan di container Go. Pada **SSL/TLS → Edge Certificates**, aktifkan
**Always Use HTTPS**, pilih **Minimum TLS Version: TLS 1.2**, dan pertahankan
**TLS 1.3** aktif. Pengaturan ini berlaku pada seluruh zona domain, termasuk
subdomain. HTTP harus mengembalikan `301` dengan tujuan HTTPS; TLS 1.0 dan 1.1
harus ditolak pada handshake, sementara TLS 1.2 dan 1.3 tetap berhasil.

Pastikan HTTPS tidak mengalami redirect loop. Jangan menambahkan redirect di
aplikasi hanya berdasarkan koneksi internal HTTP antara proxy dan container.
Untuk pemasangan Nginx langsung, `deploy/nginx.conf` menyertakan redirect dan
`ssl_protocols TLSv1.2 TLSv1.3` pada konteks `http`; sesuaikan sertifikat/domain
dan jalankan `nginx -t` sebelum reload. Pengaturan Nginx origin tidak menggantikan
pengaturan TLS di Cloudflare.

Outfit, Inter, dan Chart.js disertakan di `frontend/public/vendor`, lengkap dengan
versi, lisensi, dan checksum. Build menolak aset yang berubah tanpa pembaruan
manifest serta referensi Google Fonts/CDN yang diperkenalkan kembali. Docker
menyalin aset ke `/app/public`; jangan menghapus folder `vendor` atau `fonts.css`
dari hasil build. CSP hanya mengizinkan font lokal. Turnstile tetap menggunakan
endpoint resmi Cloudflare yang dinamis.

Perubahan HTTPS/TLS dan aset ini tidak memerlukan migrasi database, impor,
penggantian volume, pemulihan backup, atau perubahan kredensial pengguna.

## CBT mandiri dan cutover asesmen

CBT dipasang sebagai aplikasi dan PostgreSQL **terpisah** dari LMS, dari
repository `https://github.com/miqbalputra/lms-pkbmti-ujian.git`. Jangan
menggunakan database, volume unggahan, atau credential database LMS pada
stack CBT.

1. Deploy repo CBT terlebih dahulu dengan domain `ujian.pkbmtunasilmu.sch.id`,
   PostgreSQL sendiri, volume unggahan, dan backup harian.
2. Buat satu secret HMAC acak minimal 32 karakter. Isi nilai dan key ID yang
   sama di LMS (`CBT_INTEGRATION_*`) dan CBT (`LMS_INTEGRATION_*`).
3. Buat secret acak minimal 32 karakter lain khusus SSO—jangan gunakan ulang
   secret integrasi. Isi `CBT_SSO_HMAC_SECRET` di LMS dan nilai identik sebagai
   `LMS_SSO_HMAC_SECRET` di CBT. Pada CBT, set `LMS_PUBLIC_URL` ke
   `https://edu.pkbmtunasilmu.sch.id`. Kredensial ini hanya berada di server.
4. Jalankan sinkronisasi master serta import salin-data dari panel CBT. Cocokkan
   jumlah data, snapshot, nilai, relasi siswa-kelas, dan berkas. Sumber LMS
   tidak dihapus atau ditulis ulang oleh proses ini.
5. Setelah backup kedua database dan verifikasi selesai, isi
   `CBT_PUBLIC_URL=https://ujian.pkbmtunasilmu.sch.id` pada LMS lalu redeploy
   kedua aplikasi. Tutor membuka menu **CBT & Asesmen** di LMS; dari halaman CBT,
   pilih **Masuk melalui akun LMS**. Siswa dengan akun LMS juga dapat memakai
   tombol tersebut. Identitas dan peran selalu diperiksa LMS,
   sementara password LMS tidak pernah disalin ke CBT.
   Sebelum variabel itu diisi, seluruh rute asesmen LMS tetap bekerja seperti
   semula; sesudahnya `/ujian`, `/simulasi`, `/bank-soal`, `/ujian-online`, dan
   `/ujian-monitor` mengarahkan pengguna ke CBT.

Jangan mengaktifkan `CBT_PUBLIC_URL` jika probe HMAC, import, atau pemulihan
percobaan lama belum lulus. Data presensi dan modul LMS lain tidak disentuh.

## 6. Deploy

1. Click **Deploy** in Coolify
2. Monitor build logs
3. First run will auto-migrate database and seed the `admin` account using `ADMIN_DEFAULT_PASSWORD`

## Default Credentials

Before first deploy, set `ADMIN_DEFAULT_PASSWORD` to a unique strong password (minimum 12 characters).
Use that password with username `admin` for the first login, then rotate it through the application.

## Database Migration

The app uses GORM AutoMigrate on startup - no manual migration needed.

## Troubleshooting

### Build fails
- Check Coolify build logs
- Ensure Dockerfile is at root

### Database connection error
- Verify `DATABASE_URL` format
- Ensure PostgreSQL is running in Coolify
- Check internal network connectivity

### App starts but no data
- First run auto-seeds default admin
- Check logs for migration errors

## Konfigurasi bantuan AI rapor

Tambahkan `GEMINI_API_KEY` dan/atau `OPENAI_API_KEY` ke environment server.
`docker-compose.yml` meneruskan kedua variabel ke backend. Jangan memasukkan
API key ke variabel `VITE_*`, kode frontend, atau repository. Setelah restart,
admin memilih penyedia dan ID model melalui **Rapor → Pengaturan → Bantuan AI**.
Gunakan model yang mendukung keluaran JSON terstruktur. Tanpa kredensial,
seluruh pengisian nilai, deskripsi manual, finalisasi, dan cetak tetap tersedia.

Migrasi menambah tabel `report_*` dan tidak mengonversi atau menimpa nilai tema
lama. Lakukan backup penuh sebelum rilis sesuai prosedur di dokumen ini. Font,
logo, dan layout cetak dibundel dalam binary; server tidak memerlukan Microsoft
Excel untuk membuat PDF. Workbook sumber hanya diperlukan untuk regenerasi aset
oleh `backend/tools/rapor-template`, bukan saat aplikasi berjalan.
