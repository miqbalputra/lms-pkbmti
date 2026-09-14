# Runbook Keamanan Data dan Pemulihan LMS

Runbook ini menjadi prosedur operasional sebelum deploy, migrasi, atau restore.
Tujuannya menjaga data peserta didik, nilai, presensi, dan lampiran tetap dapat
diakses jika perubahan atau layanan cloud bermasalah.

## Prinsip wajib

- Jangan menghapus database, volume `pgdata`, `uploadsdata`, atau `backupdata`
  untuk menyelesaikan masalah deploy.
- Selalu buat satu backup baru dan pastikan ukurannya tidak nol sebelum migrasi.
- Simpan minimal dua salinan: volume backup lokal dan provider berbeda (R2 atau
  Google Drive melalui n8n).
- Restore selalu dilakukan ke staging/disposable database terlebih dahulu jika
  memungkinkan. Jangan gunakan `BACKUP_DRILL_DATABASE_URL` yang menunjuk ke
  production.
- Kunci `BACKUP_ENCRYPTION_KEY`, `BACKUP_API_KEY`, dan kredensial R2 adalah
  secret; jangan dimasukkan ke Git, URL, screenshot, atau log.
- Satu deployment/database saat ini adalah satu sekolah/PKBM. Untuk banyak
  sekolah, gunakan database dan volume terisolasi per sekolah sampai migrasi
  tenant dengan row-level isolation selesai dan diaudit.

## Backup aman

1. Buka **Backup & Restore** sebagai Administrator.
2. Jalankan **Backup Sekarang** untuk R2 dan pastikan job berstatus `succeeded`.
3. Untuk Google Drive/S3-compatible melalui n8n, panggil:

   ```text
   GET /api/backup/offsite?format=full
   X-Backup-Key: <BACKUP_API_KEY>
   ```

   Respons berupa `.db.enc` untuk SQLite atau `.sql.enc` untuk PostgreSQL.
   Endpoint `/api/backup/download` dipertahankan untuk kompatibilitas lama,
   tetapi menghasilkan plaintext dan tidak direkomendasikan untuk cloud.

4. Pastikan objek cloud privat, ukuran file masuk akal, dan file tidak tersimpan
   sebagai execution binary jangka panjang di n8n.

## Sebelum deploy atau migrasi

- Catat commit/release yang sedang berjalan.
- Pastikan `DATABASE_URL`, `UPLOADS_DIR`, `BACKUP_DIR`, dan volume Docker tetap
  menunjuk ke lokasi yang sama.
- Pastikan `CORS_ALLOWED_ORIGINS` hanya berisi origin final, HTTPS, dan bukan `*`.
- Pastikan secret JWT, Turnstile, backup, dan OAuth terisi dari secret manager.
- Jalankan `go test ./...`, `go vet ./...`, `npm ci`, dan `npm run build`.
- Di staging, login minimal sebagai Admin, Kepala Sekolah, dan Tutor; buka
  dashboard, peserta didik, presensi, nilai, dan Backup & Restore.

## Restore SQLite

1. Unggah `.db`, `.sql`, `.db.enc`, atau `.sql.enc` dari halaman Backup.
2. Aplikasi memvalidasi format dan integritas sebelum file dipentaskan.
3. Di production, restart terkontrol menerapkan restore sebelum database dibuka.
4. Aplikasi membuat `pre-restore-*.db`; jangan hapus file ini sampai verifikasi
   selesai.
5. Verifikasi jumlah peserta didik, kelas, nilai, presensi, dan beberapa lampiran.
6. Jika hasil salah, hentikan service secara terkontrol, simpan log/jurnal
   restore, lalu ikuti prosedur rollback dengan backup `pre-restore`.

## Restore PostgreSQL

1. Restore selalu membuat dump pengaman `pre-restore-*.sql` sebelum `psql`.
2. Pastikan dump berasal dari instance/versi yang kompatibel dan telah diuji ke
   database disposable.
3. Selama restore, aplikasi menolak write agar tidak ada perubahan parsial dari
   pengguna.
4. Verifikasi data bisnis dan login semua peran utama setelah selesai.
5. Jika restore gagal, pertahankan dump pengaman dan gunakan prosedur rollback
   PostgreSQL yang disetujui operator database.

## Bukti yang disimpan

Untuk setiap restore drill bulanan, simpan tanggal, commit aplikasi, ukuran arsip,
checksum SHA-256, durasi, status validasi database, dan status akses lampiran.
Jangan menyimpan isi backup, password, token, atau data peserta didik di laporan.
