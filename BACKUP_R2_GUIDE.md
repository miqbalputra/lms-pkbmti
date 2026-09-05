# Backup penuh Cloudflare R2

Backup R2 PKBM LMS berisi satu arsip terenkripsi yang mencakup database dan seluruh direktori `uploads/`. File `.env`, source code, serta secret server tidak pernah dimasukkan ke backup.

## Konfigurasi Cloudflare

1. Buat bucket R2 privat, misalnya `pkbm-lms-backups`. Jangan aktifkan public bucket/custom domain.
2. Buat **S3 API token** dengan izin Object Read & Write yang dibatasi hanya ke bucket tersebut. Catat Access Key ID dan Secret Access Key di secret manager deployment.
3. Buat lifecycle rule untuk prefix `${BACKUP_R2_PREFIX:-pkbm-lms}/archives/`: expire setelah nilai `BACKUP_R2_RETENTION_DAYS` (**36 hari** secara default), dan abort incomplete multipart upload setelah 1 hari.
4. Aktifkan Bucket Lock minimal 36 hari bila kebijakan operasional mengizinkan WORM protection. Ini melindungi backup dari penghapusan melalui kredensial aplikasi yang bocor.
5. Isi `BACKUP_R2_*` dan `BACKUP_ENCRYPTION_KEY` di environment server, lalu redeploy. Dashboard hanya menampilkan bucket/status dan tidak pernah menampilkan access key.

## Operasional

- Scheduler memeriksa setiap hari pukul 02.00 WIB dan membuat backup otomatis bila backup otomatis sukses terakhir berusia minimal 72 jam.
- Tombol **Backup Sekarang** menjalankan job penuh di background. Gunakan **Uji Koneksi** setelah konfigurasi atau rotasi kredensial.
- Restore dari daftar R2 membutuhkan konfirmasi, membuat backup pengaman penuh terlebih dahulu, lalu mengaktifkan mode pemeliharaan. Untuk SQLite, container restart otomatis diperlukan untuk menerapkan swap database yang aman.
- Arsip cloud sengaja tidak memiliki tombol hapus. Retensi dilakukan oleh lifecycle R2.
