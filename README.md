# LMS PKBM Tunas Ilmu

## Jalankan Lokal
1. Jalankan backend:
   ```powershell
   cd backend
   go run ./cmd/server
   ```
2. Jalankan frontend pada terminal lain:
   ```powershell
   cd frontend
   npm.cmd install
   npm.cmd run dev
   ```
3. Buka URL yang ditampilkan Vite, umumnya `http://localhost:5173`.

Backend memakai SQLite lokal secara default. Akun awal development: `admin` / `Admin123`.

## Verifikasi
```powershell
cd backend
go test ./...
go vet ./...
```

```powershell
cd frontend
npm.cmd run build
npm.cmd audit --omit=dev
```

## Backup dan keamanan

- Backup penuh dari menu **Backup & Restore** (lokal maupun R2) mencakup database
  serta seluruh isi `uploads` dalam arsip terenkripsi `.tar.gz.enc`.
- Untuk Google Drive atau S3-compatible melalui n8n, gunakan endpoint
  `GET /api/backup/offsite?format=full` dengan header `X-Backup-Key`; hasilnya
  adalah arsip lengkap `.tar.gz.enc`. Gunakan `format=db` atau `format=sql`
  hanya bila yang diperlukan memang dump database saja.
- Restore selalu divalidasi dan membuat backup pengaman `pre-restore-*`. Ikuti
  [DATA_SAFETY_RUNBOOK.md](DATA_SAFETY_RUNBOOK.md) sebelum operasi production.
- CI menjalankan test/vet backend, build frontend, dan audit dependency di
  `.github/workflows/ci.yml`.

## Production
Gunakan `deploy/.env.example` sebagai referensi environment server. Image
production berjalan sebagai user non-root dan entrypoint mempertahankan akses ke
volume backup/upload lama tanpa menghapus data. Docker belum tersedia pada
workspace ini, sehingga `docker compose` perlu divalidasi di mesin yang memasang
Docker sebelum deployment.
