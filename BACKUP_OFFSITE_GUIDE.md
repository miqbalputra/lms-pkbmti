# Panduan Backup, Offsite Archive, dan Restore Drill

Panduan ini menjelaskan penggunaan:

```env
BACKUP_CRON=0 2 * * *
BACKUP_OFFSITE_URL=https://endpoint-arsip-anda
BACKUP_ENCRYPTION_KEY=random-secret-kuat
BACKUP_DRILL_DATABASE_URL=postgres://user:pass@database-drill:5432/pkbm_drill
```

Jangan memakai nilai contoh tersebut secara langsung. Ganti dengan endpoint, secret, dan database yang benar.

## 1. Alur kerja

Setiap hari pukul 02:00 WIB:

1. Aplikasi membuat backup penuh database.
2. Backup diverifikasi terlebih dahulu.
3. Jika `BACKUP_OFFSITE_URL` diaktifkan, backup dienkripsi AES-256-GCM.
4. File terenkripsi dikirim sebagai binary body ke endpoint offsite.
5. Jika `BACKUP_DRILL_DATABASE_URL` diaktifkan, dump PostgreSQL direstore ke database drill.
6. Backup baru dicatat berhasil hanya setelah verifikasi dan upload/drill selesai.

Backup lokal tetap disimpan. Jika endpoint offsite gagal, aplikasi tidak menghapus backup lokal dan mencatat error di log serta health endpoint.

## 2. Konfigurasi environment

Tambahkan nilai berikut pada environment production, Coolify, atau `.env` yang dipakai saat deploy:

```env
# Cron standar 5 kolom, memakai zona waktu Asia/Jakarta.
# Artinya setiap hari pukul 02:00 WIB.
BACKUP_CRON=0 2 * * *

# Endpoint yang menerima upload binary terenkripsi.
# Jangan biarkan literal "endpoint-arsip-anda".
BACKUP_OFFSITE_URL=https://backup-gateway.example.com/upload

# Minimal 16 karakter; gunakan random string panjang dan simpan di secret manager.
BACKUP_ENCRYPTION_KEY=GANTI_DENGAN_SECRET_RANDOM_PANJANG

# Database PostgreSQL disposable, bukan database production.
BACKUP_DRILL_DATABASE_URL=postgres://pkbm:DRILL_PASSWORD@db:5432/pkbm_drill?sslmode=disable
```

Variabel pendukung yang direkomendasikan:

```env
BACKUP_OFFSITE_METHOD=PUT
BACKUP_OFFSITE_TIMEOUT=5m
BACKUP_OFFSITE_TOKEN=
BACKUP_FORMAT=full
BACKUP_RETENTION=14
BACKUP_MAX_UPLOAD_MB=512
```

Production akan menolak start jika `BACKUP_OFFSITE_URL` diisi tetapi tidak menggunakan HTTPS atau `BACKUP_ENCRYPTION_KEY` belum diisi.

### Membuat encryption key

Linux/macOS:

```bash
openssl rand -base64 48
```

PowerShell:

```powershell
$bytes = New-Object byte[] 48
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
[Convert]::ToBase64String($bytes)
```

Simpan hasilnya sebagai secret environment. Jangan commit ke Git, jangan masukkan ke URL, dan jangan kirim ke log.

## 3. Menyiapkan endpoint offsite

`BACKUP_OFFSITE_URL` harus menerima:

- metode `PUT` secara default, atau `POST` jika `BACKUP_OFFSITE_METHOD=POST`;
- body berupa binary file terenkripsi;
- header `Content-Type: application/octet-stream`;
- header `X-Backup-Name`, misalnya `pkbm-lms-20260806-020000-auto.sql.enc`;
- header `Authorization: Bearer ...` jika `BACKUP_OFFSITE_TOKEN` diisi;
- response HTTP status `2xx` jika upload berhasil.

Endpoint dapat berupa:

- gateway n8n yang menerima binary lalu menyimpan ke Google Drive, S3, atau storage lain;
- service internal yang meneruskan body ke S3-compatible storage;
- endpoint upload yang dikelola sendiri.

URL presigned S3 yang kadaluarsa biasanya tidak cocok dipakai langsung untuk cron harian karena URL tersebut hanya berlaku sementara. Gunakan gateway yang dapat membuat signature baru, atau gunakan endpoint upload yang masa berlakunya panjang dan dibatasi dengan token.

## 4. Pilihan praktis: n8n ke Google Drive

Gunakan satu workflow n8n sebagai gateway upload:

```text
Webhook (PUT) -> Google Drive: Upload -> Respond to Webhook
```

File workflow siap import tersedia di [n8n-backup-google-drive-workflow.json](n8n-backup-google-drive-workflow.json).
Setelah import, pilih credential Google Drive pada node **Upload ke Google Drive**, pilih credential **Header Auth** pada node Webhook, lalu pilih folder tujuan backup. Workflow sengaja tidak menyimpan credential, token, atau folder ID.

### Konfigurasi Webhook

1. Buat node **Webhook**.
2. Pilih **HTTP Method: PUT**.
3. Buat path khusus, misalnya `pkbm-backup`.
4. Aktifkan autentikasi webhook **Header Auth**. Buat credential dengan:
   - nama header: `Authorization`;
   - nilai: `Bearer TOKEN_RANDOM_YANG_SAMA_DENGAN_APLIKASI`.
5. Pada options, isi **Binary Property: `data`**.
6. Pilih response menggunakan node **Respond to Webhook** setelah proses selesai.

Salin **Production URL** dari node Webhook, lalu isi di aplikasi:

```env
BACKUP_OFFSITE_URL=https://n8n.example.com/webhook/pkbm-backup
BACKUP_OFFSITE_METHOD=PUT
BACKUP_OFFSITE_TOKEN=TOKEN_RANDOM_YANG_SAMA_DENGAN_CREDENTIAL_N8N
```

Jangan memakai Test URL untuk production. Webhook n8n memang mendukung method `PUT` dan menerima file melalui binary property; node Google Drive kemudian menggunakan nama binary property tersebut untuk upload.

### Konfigurasi Google Drive

1. Tambahkan node **Google Drive** setelah Webhook.
2. Pilih credential Google Drive yang hanya memiliki akses ke folder backup.
3. Pilih **Resource: File** dan **Operation: Upload**.
4. Isi **Input Data Field Name: `data`**.
5. Pilih **Parent Drive** dan **Parent Folder** tujuan.
6. Pada **File Name**, gunakan expression:

```text
{{ $json.headers['x-backup-name'] || $json.headers['X-Backup-Name'] || 'pkbm-backup.sql.enc' }}
```

Buat folder khusus, misalnya `PKBM Tunas Ilmu / Database Backups`, dan jangan aktifkan public sharing. File di Google Drive tetap berupa `.sql.enc`/`.db.enc`, bukan database plaintext.

### Konfigurasi response dan keamanan n8n

Tambahkan node **Respond to Webhook** setelah Google Drive. Kembalikan JSON sederhana:

```json
{
  "ok": true,
  "fileId": "={{ $json.id }}",
  "fileName": "={{ $json.name }}"
}
```

Atur n8n agar execution sukses tidak disimpan terlalu lama dan data binary execution dipangkas berkala. Jangan menulis body backup, encryption key, atau token ke log. Jika upload gagal, biarkan workflow menghasilkan HTTP `4xx`/`5xx`; aplikasi akan menandai offsite gagal tetapi tetap mempertahankan backup lokal.

### Uji workflow sebelum diaktifkan

Untuk uji dari server aplikasi, kirim salah satu file backup terenkripsi secara manual:

```bash
curl -X PUT \
  -H "Authorization: Bearer TOKEN_RANDOM_YANG_SAMA_DENGAN_APLIKASI" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Backup-Name: pkbm-test.sql.enc" \
  --data-binary @backups/pkbm-test.sql.enc \
  https://n8n.example.com/webhook/pkbm-backup
```

Pastikan response `2xx`, file muncul di folder Drive, ukurannya tidak nol, dan ekstensi tetap `.enc`. Setelah itu gunakan `BACKUP_CRON=*/5 * * * *` hanya di non-production untuk menguji satu siklus otomatis, lalu kembalikan ke `0 2 * * *`.

## 5. Menyiapkan database restore drill

Database drill harus terpisah dari production. Restore drill akan mengganti isi database drill dengan dump terbaru, sehingga database tersebut harus disposable.

Contoh jika PostgreSQL berjalan melalui Docker Compose:

```bash
docker compose exec db psql -U pkbm -d postgres -c "CREATE DATABASE pkbm_drill;"
```

Kemudian isi URL sesuai kredensial database drill:

```env
BACKUP_DRILL_DATABASE_URL=postgres://pkbm:DRILL_PASSWORD@db:5432/pkbm_drill?sslmode=disable
```

Untuk production yang lebih aman, gunakan instance PostgreSQL terpisah. Jangan pernah mengarahkan `BACKUP_DRILL_DATABASE_URL` ke database production.

Saat backup terjadwal berjalan, aplikasi menggunakan `psql` dengan transaksi dan `ON_ERROR_STOP=1`. Jika restore drill gagal, backup lokal tetap ada tetapi job ditandai gagal.

## 6. Cara memverifikasi konfigurasi

### Cek environment di container

```bash
docker compose exec app printenv BACKUP_CRON
docker compose exec app printenv BACKUP_OFFSITE_URL
docker compose exec app printenv BACKUP_DRILL_DATABASE_URL
```

Jangan mencetak `BACKUP_ENCRYPTION_KEY` atau `BACKUP_OFFSITE_TOKEN` ke terminal/log production.

### Cek health endpoint

```bash
curl https://domain-anda.example/api/health
```

Perhatikan bagian `backup`:

```json
{
  "backup": {
    "offsiteConfigured": true,
    "restoreDrillConfigured": true,
    "lastSuccessAt": "2026-08-06T19:00:00Z",
    "lastOffsiteAt": "2026-08-06T19:00:00Z",
    "lastRestoreDrillAt": "2026-08-06T19:00:00Z",
    "totalFailure": 0
  }
}
```

Waktu pada response menggunakan UTC. Konfigurasi cron tetap menggunakan Asia/Jakarta.

### Cek log aplikasi

```bash
docker compose logs -f app
```

Indikator berhasil:

```text
scheduled backup written: backups/pkbm-lms-...-auto.sql
```

Indikator yang harus ditindaklanjuti:

```text
scheduled backup: verification failed: ...
scheduled backup: offsite upload failed: ...
scheduled backup: pg_dump failed: ...
```

## 7. Uji pertama tanpa menunggu pukul 02:00

Untuk uji sementara, gunakan cron setiap 5 menit di environment non-production:

```env
BACKUP_CRON=*/5 * * * *
```

Setelah satu job berhasil diverifikasi, kembalikan ke:

```env
BACKUP_CRON=0 2 * * *
```

Pastikan:

1. file backup muncul di folder `BACKUP_DIR`;
2. endpoint offsite menerima file `.enc`;
3. file dapat didekripsi oleh aplikasi;
4. `lastOffsiteAt` terisi;
5. `lastRestoreDrillAt` terisi jika database drill dikonfigurasi;
6. `totalFailure` tidak bertambah.

## 8. Prosedur restore

### Restore PostgreSQL biasa

1. Buka menu **Backup & Restore**.
2. Pilih file `.sql`.
3. Klik **Restore Sekarang**.
4. Aplikasi membuat `pre-restore-*.sql` terlebih dahulu.
5. Dump diterapkan langsung melalui `psql` dalam satu transaksi.

### Restore backup offsite terenkripsi

1. Download file dengan ekstensi `.sql.enc` atau `.db.enc` dari storage offsite.
2. Pastikan `BACKUP_ENCRYPTION_KEY` yang benar masih terpasang di aplikasi.
3. Buka menu **Backup & Restore**.
4. Upload file terenkripsi tersebut.
5. Klik **Restore Sekarang**.

Untuk PostgreSQL, file `.sql.enc` diterapkan langsung. Untuk SQLite, file `.db.enc` atau `.sql.enc` diproses saat restart berikutnya.

Jika encryption key sudah diganti, gunakan key lama untuk restore backup lama. Sistem tidak dapat mendekripsi backup lama dengan key baru.

## 9. Retensi dan keamanan

- `BACKUP_RETENTION` hanya memangkas backup otomatis lokal.
- Retensi file offsite diatur oleh storage/gateway offsite, bukan aplikasi ini.
- Simpan encryption key dan token offsite di secret manager.
- Batasi endpoint offsite agar hanya menerima request dari server/gateway yang diperlukan.
- Gunakan HTTPS untuk endpoint offsite.
- Jangan gunakan database production sebagai database drill.
- Lakukan simulasi restore minimal sebulan sekali.
- Simpan setidaknya satu salinan backup di lokasi atau provider yang berbeda dari server aplikasi.
