# Backup penuh Cloudflare R2

Backup R2 PKBM LMS berisi satu arsip terenkripsi yang mencakup database dan seluruh direktori `uploads/`. File `.env`, source code, serta secret server tidak pernah dimasukkan ke backup.

## Konfigurasi Cloudflare

1. Buat bucket R2 privat, misalnya `pkbm-lms-backups`. Jangan aktifkan public bucket/custom domain.
2. Buat **S3 API token** dengan izin Object Read & Write yang dibatasi hanya ke bucket tersebut. Catat Access Key ID dan Secret Access Key di secret manager deployment.
3. Buat lifecycle rule untuk prefix `pkbm-lms/archives/` (sesuaikan bila `BACKUP_R2_PREFIX` diubah): expire setelah **36 hari**, dan abort incomplete multipart upload setelah 1 hari.
4. Aktifkan Bucket Lock minimal 36 hari bila kebijakan operasional mengizinkan WORM protection. Ini melindungi backup dari penghapusan melalui kredensial aplikasi yang bocor.
5. Isi `BACKUP_R2_*` dan `BACKUP_ENCRYPTION_KEY` di environment server, lalu redeploy. Dashboard hanya menampilkan bucket/status dan tidak pernah menampilkan access key.

## Operasional

- Scheduler memeriksa setiap hari pukul 02.00 WIB dan membuat backup otomatis bila backup otomatis sukses terakhir berusia minimal 72 jam.
- `BACKUP_R2_TIMEOUT` membatasi seluruh request R2 (default 2 menit), sehingga job gagal secara terukur jika storage tidak dapat dijangkau.
- Tombol **Backup Sekarang** menjalankan job penuh di background. Gunakan **Uji Koneksi** setelah konfigurasi atau rotasi kredensial.
- Restore dari daftar R2 membutuhkan konfirmasi, membuat backup pengaman penuh terlebih dahulu, lalu mengaktifkan mode pemeliharaan. Untuk SQLite, container restart otomatis diperlukan untuk menerapkan swap database yang aman.
- Restore penuh memiliki jurnal persisten di volume backup. Bila container berhenti pada saat swap database/lampiran, startup berikutnya menyelesaikan SQLite secara atomik atau mengembalikan PostgreSQL dari snapshot pengaman lokal terenkripsi.
- Arsip cloud sengaja tidak memiliki tombol hapus. Retensi dilakukan oleh lifecycle R2.

## Alert dan drill pemulihan

- Isi `OPERATIONS_WEBHOOK_URL` untuk menerima status backup/restore gagal, backup terlambat, restore selesai, dan perubahan health. Payload tidak memuat data siswa, kredensial, URL database, atau isi request. Jika `OPERATIONS_WEBHOOK_SECRET` diisi, verifikasi header `X-PKBM-Signature-256` dengan HMAC-SHA256 payload mentah.
- Default alert keterlambatan adalah 78 jam (`BACKUP_ALERT_MAX_AGE_HOURS`) untuk jadwal backup 72 jam. Alert untuk insiden yang sama dibatasi satu kali per 24 jam.
- Lakukan drill minimal mingguan pada bucket **non-produksi**: buat backup, unduh, dekripsi/validasi manifest, restore ke PostgreSQL/volume lampiran terisolasi, lalu verifikasi login dan sebuah lampiran. Jangan gunakan token atau bucket produksi pada CI.
- Workflow GitHub `R2 Test Bucket Drill` menjalankan drill mingguan dan dapat dipicu manual. Buat GitHub Environment `r2-drill` dengan secret `BACKUP_R2_ACCOUNT_ID`, `BACKUP_R2_BUCKET`, `BACKUP_R2_ACCESS_KEY_ID`, `BACKUP_R2_SECRET_ACCESS_KEY`, dan `BACKUP_ENCRYPTION_KEY`; gunakan bucket/prefix khusus uji.
- Rotasi S3 access key sedikitnya setiap 90 hari, uji tombol **Uji Koneksi**, lalu cabut key lama. Audit akses admin melalui audit log aplikasi dan Cloudflare audit log.

## Checklist staging

1. Jalankan `docker compose config`, build image, lalu `docker compose up -d` pada environment staging.
2. Login admin, unggah satu lampiran, restart container, dan pastikan lampiran tetap ada.
3. Jalankan backup R2 dan drill restore dari bucket uji. Pastikan mode pemeliharaan aktif selama restore dan job berakhir `completed` atau `rolled-back`.
4. Pastikan lifecycle 36 hari, abort multipart 1 hari, serta Bucket Lock prefix backup minimal 36 hari sudah aktif di dashboard Cloudflare.
