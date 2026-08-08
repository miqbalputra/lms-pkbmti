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

## 4. Configure Environment Variables

Add these environment variables in your application:

| Variable | Description | Example |
|----------|-------------|---------|
| `APP_ENV` | Environment | `production` |
| `PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection | `postgres://user:pass@db-host:5432/pkbm` |
| `DB_MAX_OPEN_CONNS` | Batas koneksi database aktif | `25` |
| `DB_MAX_IDLE_CONNS` | Batas koneksi idle database | `10` |
| `JWT_ACCESS_SECRET` | Random 32+ chars | `your-secret-key-here` |
| `JWT_REFRESH_SECRET` | Random 32+ chars | `another-secret-key` |
| `ADMIN_DEFAULT_PASSWORD` | Password admin saat first start | Wajib kuat, minimal 12 karakter |
| `CORS_ALLOWED_ORIGINS` | Your domain | `https://lms.example.com` |
| `COOKIE_DOMAIN` | Your domain | `lms.example.com` |
| `TURNSTILE_SECRET_KEY` | Cloudflare Turnstile secret | Required in production |
| `TURNSTILE_SITE_KEY` | Cloudflare Turnstile site key for public pages | Required in production |
| `VITE_TURNSTILE_SITE_KEY` | Same public site key for the main login build | Required in production |
| `BACKUP_CRON` | Jadwal backup otomatis | `0 2 * * *` |
| `BACKUP_FORMAT` | Format backup penuh | `full` |
| `BACKUP_RETENTION` | Jumlah backup otomatis yang disimpan | `14` |
| `BACKUP_AUTO_RESTART` | Restart otomatis setelah upload restore | `true` |
| `BACKUP_MAX_UPLOAD_MB` | Batas upload file restore | `512` |
| `BACKUP_API_KEY` | Key untuk download backup via n8n | Optional |
| `N8N_PRESENSI_API_KEY` | API key khusus workflow pengingat presensi tutor | Wajib jika workflow n8n presensi digunakan |
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

### Backup Offsite dan Restore Drill

Jika `BACKUP_OFFSITE_URL` diisi, setiap backup terjadwal dienkripsi AES-256-GCM lalu dikirim sebagai body binary ke endpoint tersebut. Endpoint dapat berupa gateway n8n, storage service internal, atau URL S3 yang memang dikelola untuk upload berulang. Simpan `BACKUP_ENCRYPTION_KEY` di secret manager; file `.enc` tidak dapat dipulihkan tanpa kunci itu.

Isi `BACKUP_DRILL_DATABASE_URL` dengan database PostgreSQL disposable yang terpisah dari production. Backup terjadwal akan direstore ke database tersebut dengan `psql` sebelum dicatat sebagai backup berhasil.

Panduan lengkap konfigurasi, endpoint, pengujian, dan restore tersedia di [BACKUP_OFFSITE_GUIDE.md](BACKUP_OFFSITE_GUIDE.md).

## 5. Configure Domain

1. Go to **Application** → **Networking**
2. Add your domain (e.g., `lms.example.com`)
3. Enable SSL (Let's Encrypt or your own certificate)

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
