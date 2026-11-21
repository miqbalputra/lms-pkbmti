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

## Rapor sumatif semester

Menu **Nilai** dan **Rapor** sekarang membuka alur sumatif CP. Pilih tahun,
kelas, dan semester; atur urutan mapel serta kelompoknya di tab **Pengaturan**.
Mapel mengikuti relasi kelas–mapel, sehingga jenjang atas dapat menambah mapel
tanpa mengubah kode. Tutor mapel membuat CP, menyimpan, lalu mengisi angka
0–100. Blok angka dari Excel dapat ditempel ke grid. Nilai rapor adalah rata-rata
CP terisi yang dibulatkan; angka nol tetap dihitung dan sel kosong bukan nol.

Deskripsi otomatis mengambil maksimal tiga capaian tertinggi dan tiga capaian
lain untuk penguatan. Tutor dapat mengeditnya atau memilih **Bantu AI**,
meninjau saran, lalu menyimpannya. Jika nilai berubah, deskripsi manual ditandai
untuk ditinjau kembali. Konfigurasikan `GEMINI_API_KEY` atau `OPENAI_API_KEY`
pada server, kemudian pilih penyedia dan model yang tersedia pada akun melalui
Pengaturan. AI tidak diperlukan untuk pengisian manual dan tidak menghitung nilai.

Wali kelas/admin melengkapi catatan, ekstrakurikuler, identitas cetak, tanggal,
penandatangan, serta keputusan kenaikan semester Genap. Presensi terintegrasi
per hari; konflik status pada satu tanggal harus diselesaikan sebelum finalisasi.
Alurnya **draf → siap diperiksa → final → diterbitkan**. Rapor final menyimpan
snapshot dan riwayat versi. Pembukaan revisi wajib disertai alasan. Portal orang
tua hanya menampilkan rapor sumatif yang sudah diterbitkan; grafik nilai juga
menggunakan nilai rapor terbit tersebut.

Cetak PDF tersedia untuk satu siswa atau satu kelas, dengan pilihan cover,
identitas, rapor, dan paket lengkap. Template rapor menggunakan geometri cetak,
font, kop, serta empat halaman dari workbook referensi. Mapel tambahan memakai
halaman tabel lanjutan sebelum ringkasan dan tanda tangan. Leger dapat diunduh
sebagai Excel/PDF. Ukuran kotak deskripsi mengikuti blangko standar; hanya kotak
yang teksnya tidak muat yang bertambah tinggi. Baris berikutnya ikut bergeser,
dan halaman lanjutan ditambahkan bila perlu tanpa mengecilkan huruf. Deskripsi
manual tetap dibatasi 2.000 karakter per bidang; saran AI tetap dibuat ringkas.
Isi tabel memakai Arial 11 pt dengan jarak dalam kotak; deskripsi, catatan,
dan keterangan ekstrakurikuler rata kiri-kanan. Baris akhir paragraf tetap rata
kiri. Tinggi baris memperhitungkan jarak dalam kotak agar teks tidak terpotong.

Pada **Pengaturan → Ekstrakurikuler kelas ini**, wali/admin mengaktifkan atau
menonaktifkan ekstrakurikuler dan mengisi daftar kegiatan per kelas/semester.
Pada **Pemeriksaan & cetak**, pilih siswa, tambahkan kegiatan yang diikuti, isi
keterangannya (maksimal 1.000 karakter per kegiatan), lalu **Simpan catatan**.
Setiap siswa boleh memiliki maksimal 10 kegiatan; siswa yang tidak ikut boleh
dikosongkan. Cetak memakai satu baris per kegiatan yang membesar sesuai isi.
Saat dinonaktifkan, tabel tidak dicetak dan isian lama tetap tersimpan. Data
semester lama tanpa pengaturan ini tetap dapat dibaca seperti sebelumnya.

Gambar tanda tangan kepala PKBM bersifat opsional dan diunggah pada Pengaturan;
tanda tangan dari workbook lama tidak digunakan otomatis.

Tab **Impor workbook** khusus admin menerima `.xlsm`/`.xlsx` berstruktur referensi.
Pilih periode tujuan, unggah, cocokkan NIS/NISN dan mapel, lalu konfirmasi.
Siswa/sheet yang tidak dipakai harus ditandai **Lewati**. Pratinjau tidak menyimpan
nilai; konfirmasi bersifat transaksional, tidak menimpa data, dan mencegah impor
ulang berkas yang sama. Macro tidak dijalankan. Data kosong tetap kosong.
Alur tema 60:40 dan rapor lama tersedia melalui tombol arsip, terpisah dari CP baru.

## Pemeriksaan kode
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
