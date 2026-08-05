# Deployment Guide - Coolify

## Prerequisites

- Coolify instance running
- Domain name configured
- Cloudflare Turnstile (optional, for login protection)

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
| `JWT_ACCESS_SECRET` | Random 32+ chars | `your-secret-key-here` |
| `JWT_REFRESH_SECRET` | Random 32+ chars | `another-secret-key` |
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

**Note**: Coolify can reference database variables automatically using Coolify's variable syntax.

## 5. Configure Domain

1. Go to **Application** → **Networking**
2. Add your domain (e.g., `lms.example.com`)
3. Enable SSL (Let's Encrypt or your own certificate)

## 6. Deploy

1. Click **Deploy** in Coolify
2. Monitor build logs
3. First run will auto-migrate database and seed default admin

## Default Credentials

After first deploy, login with:
- **Username**: `admin`
- **Password**: `Admin123`

**Change this password immediately after first login!**

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
