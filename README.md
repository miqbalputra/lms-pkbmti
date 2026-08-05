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
```

```powershell
cd frontend
npm.cmd run build
```

## Production
Gunakan `deploy/.env.example` sebagai referensi environment server. Docker belum tersedia pada workspace ini, sehingga `docker compose` perlu divalidasi di mesin yang memasang Docker sebelum deployment.
