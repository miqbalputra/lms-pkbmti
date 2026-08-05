package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// seedDummy fills comprehensive representative data across ALL modules (B-S) so the
// whole app can be exercised end-to-end. Idempotent: guarded by a sentinel Tahun
// Ajaran name — re-running is a no-op. Invoked via admin-only POST /seed/dummy so it
// runs in-process (same DB pool — no SQLite lock conflict with SetMaxOpenConns(1)).
func (s *Server) seedDummy() (string, error) {
	const sentinel = "2025/2026-DUMMY"
	// Ensure the sentinel year is the SOLE active tahun ajaran: deactivate any other
	// active year so activeYearID() resolves to the sentinel (where our kelas/rekap
	// live). Runs every call, including re-runs that hit the idempotency guard below.
	s.db.Model(&TahunAjaran{}).Where("is_aktif = ? AND nama_tahun_ajaran <> ?", true, sentinel).Update("is_aktif", false)
	var existing TahunAjaran
	if s.db.Where("nama_tahun_ajaran = ?", sentinel).First(&existing).Error == nil {
		// Re-assert sentinel active (in case a re-run found it inactive).
		s.db.Model(&TahunAjaran{}).Where("id = ?", existing.ID).Update("is_aktif", true)
		return "already seeded (sentinel tahun ajaran ditemukan) — hapus DB untuk re-seed", nil
	}

	strPtr := func(v string) *string { return &v }
	floatPtr := func(v float64) *float64 { return &v }
	boolPtr := func(v bool) *bool { return &v }
	timePtr := func(t time.Time) *time.Time { return &t }
	pwd := func(p string) string {
		h, _ := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
		return string(h)
	}

	// --- Users + Tutors ---
	type seedUser struct {
		username, role, password, nama string
	}
	users := []seedUser{
		{"kepala", "kepala_sekolah", "Kepala123", "Drs. Kepala Sekolah"},
		{"guru1", "guru", "Guru1234", "Budi Santoso"},
		{"guru2", "guru", "Guru1234", "Siti Aminah"},
	}
	userIDs := map[string]string{}
	tutorIDs := map[string]string{}
	for _, u := range users {
		var user User
		if s.db.Where("username = ?", u.username).First(&user).Error != nil {
			// Email has a uniqueIndex; admin (created at startup) already occupies the
			// empty-string slot, so each seeded user needs a distinct email or the
			// insert silently fails on the UNIQUE constraint and login breaks.
			user = User{Username: u.username, Email: u.username + "@pkbm.test", Role: u.role, IsActive: true, PasswordHash: pwd(u.password)}
			s.db.Create(&user)
		}
		userIDs[u.username] = user.ID
		if u.role == "guru" {
			var t Tutor
			if s.db.Where("user_id = ?", user.ID).First(&t).Error != nil {
				t = Tutor{Nama: u.nama, JenisKelamin: "L", NoHP: "08120000" + u.username, UserID: strPtr(user.ID), TanggalBertugas: timePtr(time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC))}
				s.db.Create(&t)
			}
			tutorIDs[u.username] = t.ID
			s.db.Model(&User{}).Where("id = ?", user.ID).Update("tutor_id", t.ID)
		}
	}
	var adminUser User
	s.db.Where("username = ?", "admin").First(&adminUser)
	adminID := adminUser.ID

	// --- Masters: Program, Fase, Pokjar ---
	programs := map[string]string{}
	for _, p := range []struct{ kode, nama, setara string }{
		{"A", "Paket A", "Setara SD"},
		{"B", "Paket B", "Setara SMP"},
		{"C", "Paket C", "Setara SMA"},
	} {
		var pr Program
		if s.db.Where("kode = ?", p.kode).First(&pr).Error != nil {
			pr = Program{Kode: p.kode, Nama: p.nama, JenjangSetara: p.setara}
			s.db.Create(&pr)
		}
		programs[p.kode] = pr.ID
	}
	fases := map[string]string{}
	for _, f := range []struct{ kode, nama, setara string }{
		{"A", "Fase A", "Kelas 1-2 SD"},
		{"B", "Fase B", "Kelas 3-4 SD"},
		{"C", "Fase C", "Kelas 5-6 SD"},
		{"D", "Fase D", "Kelas 7-9 SMP"},
		{"E", "Fase E", "Kelas 10-12 SMA"},
	} {
		var fz Fase
		if s.db.Where("kode = ?", f.kode).First(&fz).Error != nil {
			fz = Fase{Kode: f.kode, Nama: f.nama, JenjangSetara: f.setara}
			s.db.Create(&fz)
		}
		fases[f.kode] = fz.ID
	}
	pokjars := map[string]string{}
	for _, pk := range []struct{ nama, alamat string }{
		{"Pokjar Tunas Ilmu Pusat", "Jl. Merdeka No. 1"},
		{"Pokjar Tunas Ilmu Cabang", "Jl. Sudirman No. 25"},
	} {
		var p Pokjar
		if s.db.Where("nama_pokjar = ?", pk.nama).First(&p).Error != nil {
			p = Pokjar{NamaPokjar: pk.nama, Alamat: pk.alamat, Tipe: "PKBM"}
			s.db.Create(&p)
		}
		pokjars[pk.nama] = p.ID
	}

	// --- Tahun Ajaran (active) ---
	ta := TahunAjaran{
		NamaTahunAjaran:           sentinel,
		TanggalMulai:              time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		TanggalSelesai:            time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		TanggalMulaiSemesterGenap: timePtr(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
		IsAktif:                   true,
	}
	s.db.Create(&ta)
	taID := ta.ID

	// --- Mapel + bobot + ambang predikat ---
	mapelIDs := map[string]string{}
	mapelNames := []string{"Matematika", "Bahasa Indonesia", "IPA", "IPS", "Bahasa Inggris", "PPKn"}
	for _, name := range mapelNames {
		var m MataPelajaran
		if s.db.Where("nama_mapel = ?", name).First(&m).Error != nil {
			m = MataPelajaran{NamaMapel: name, KodeMapel: name, IsActive: true}
			s.db.Create(&m)
		}
		mapelIDs[name] = m.ID
		// PengaturanBobotNilai (bobot default: ket 40, pengetahuan 60)
		var pb PengaturanBobotNilai
		if s.db.Where("mapel_id = ?", m.ID).First(&pb).Error != nil {
			s.db.Create(&PengaturanBobotNilai{MapelID: m.ID, BobotKeterampilan: 40, BobotPengetahuan: 60})
		}
		// AmbangPredikat (A>=90,B>=80,C>=70,D>=60,E<60)
		for _, a := range []struct {
			pred string
			min  float64
		}{{"A", 90}, {"B", 80}, {"C", 70}, {"D", 60}, {"E", 0}} {
			var ex AmbangPredikat
			if s.db.Where("mapel_id = ? AND predikat = ?", m.ID, a.pred).First(&ex).Error != nil {
				s.db.Create(&AmbangPredikat{MapelID: m.ID, Predikat: a.pred, NilaiMinimum: a.min})
			}
		}
	}

	// --- SumberNilai lookup (seeded at startup) + BobotSumberNilai for Matematika ---
	sumbers := map[string]string{} // kode -> id
	var snRows []SumberNilai
	s.db.Find(&snRows)
	for _, sn := range snRows {
		sumbers[sn.Kode] = sn.ID
	}
	// Matematika: UM 30, TUGAS 30, PRAKTIK 40 (UJIAN 0/dilewati)
	if mid, ok := mapelIDs["Matematika"]; ok {
		for kode, bobot := range map[string]float64{"UM": 30, "TUGAS": 30, "PRAKTIK": 40} {
			if sid, ok2 := sumbers[kode]; ok2 {
				var ex BobotSumberNilai
				if s.db.Where("mapel_id = ? AND sumber_id = ?", mid, sid).First(&ex).Error != nil {
					s.db.Create(&BobotSumberNilai{MapelID: mid, SumberID: sid, Bobot: bobot})
				}
			}
		}
	}

	// --- Kelas (2) ---
	kelasIDs := map[string]string{}
	type kelasDef struct {
		key     string
		jenjang int
		rombel  string
		pokjar  string
		wali    string
	}
	for _, kd := range []kelasDef{
		{"7A", 7, "A", "Pokjar Tunas Ilmu Pusat", "guru1"},
		{"8B", 8, "B", "Pokjar Tunas Ilmu Cabang", "guru2"},
	} {
		wali := strPtr(tutorIDs[kd.wali])
		k := Kelas{
			Jenjang: kd.jenjang, NamaRombel: kd.rombel, PokjarID: pokjars[kd.pokjar],
			TahunAjaranID: taID, WaliKelasID: wali, ProgramID: strPtr(programs["B"]), FaseID: strPtr(fases["D"]),
		}
		s.db.Create(&k)
		kelasIDs[kd.key] = k.ID
		// Riwayat wali
		s.db.Create(&RiwayatWaliKelas{KelasID: k.ID, TutorID: *wali, TanggalMulai: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)})
		// KelasMapel: assign all 6 mapels
		for _, name := range mapelNames {
			s.db.Create(&KelasMapel{KelasID: k.ID, MapelID: mapelIDs[name]})
		}
	}
	kelas7A := kelasIDs["7A"]
	kelas8B := kelasIDs["8B"]

	// --- PenugasanGuruMapel ---
	assign := []struct {
		kelas, guru, mapel string
	}{
		{"7A", "guru1", "Matematika"}, {"7A", "guru1", "IPA"}, {"7A", "guru1", "IPS"},
		{"8B", "guru2", "Bahasa Indonesia"}, {"8B", "guru2", "Bahasa Inggris"}, {"8B", "guru2", "PPKn"},
	}
	for _, a := range assign {
		s.db.Create(&PenugasanGuruMapel{TutorID: tutorIDs[a.guru], KelasID: kelasIDs[a.kelas], MapelID: mapelIDs[a.mapel]})
	}

	// --- OrangTua + PesertaDidik ---
	type siswaDef struct {
		kelas, nama, jk, nis, nisn, nik, program string
		ortu                                     OrangTua
	}
	birth := time.Date(2012, 3, 15, 0, 0, 0, 0, time.UTC)
	siswaList := []siswaDef{
		{"7A", "Andi Pratama", "L", "2025001", "902025001", "3201010103000001", "B", OrangTua{NamaBapak: "Ayah Andi", NamaIbu: "Ibu Andi"}},
		{"7A", "Bunga Lestari", "P", "2025002", "902025002", "3201010103000002", "B", OrangTua{NamaBapak: "Ayah Bunga", NamaIbu: "Ibu Bunga"}},
		{"7A", "Citra Dewi", "P", "2025003", "902025003", "3201010103000003", "B", OrangTua{NamaBapak: "Ayah Citra", NamaIbu: "Ibu Citra"}},
		{"7A", "Dimas Putra", "L", "2025004", "902025004", "3201010103000004", "B", OrangTua{NamaBapak: "Ayah Dimas", NamaIbu: "Ibu Dimas"}},
		{"7A", "Eka Nurhaliza", "P", "2025005", "902025005", "3201010103000005", "B", OrangTua{NamaBapak: "Ayah Eka", NamaIbu: "Ibu Eka"}},
		{"7A", "Fajar Hidayat", "L", "2025006", "902025006", "3201010103000006", "B", OrangTua{NamaBapak: "Ayah Fajar", NamaIbu: "Ibu Fajar"}},
		{"8B", "Galih Saputra", "L", "2025007", "902025007", "3201010103000007", "C", OrangTua{NamaBapak: "Ayah Galih", NamaIbu: "Ibu Galih"}},
		{"8B", "Hana Marliana", "P", "2025008", "902025008", "3201010103000008", "C", OrangTua{NamaBapak: "Ayah Hana", NamaIbu: "Ibu Hana"}},
		{"8B", "Irfan Maulana", "L", "2025009", "902025009", "3201010103000009", "C", OrangTua{NamaBapak: "Ayah Irfan", NamaIbu: "Ibu Irfan"}},
		{"8B", "Jihan Aulia", "P", "2025010", "902025010", "3201010103000010", "C", OrangTua{NamaBapak: "Ayah Jihan", NamaIbu: "Ibu Jihan"}},
		{"8B", "Krisna Wibowo", "L", "2025011", "902025011", "3201010103000011", "C", OrangTua{NamaBapak: "Ayah Krisna", NamaIbu: "Ibu Krisna"}},
	}
	pdByKelas := map[string][]string{} // kelas -> []pdID
	for _, sd := range siswaList {
		ot := sd.ortu
		s.db.Create(&ot)
		pd := PesertaDidik{
			Nama: sd.nama, JenisKelamin: sd.jk, NIS: sd.nis, NISN: sd.nisn, NIK: sd.nik,
			KelasID: kelasIDs[sd.kelas], PokjarID: pokjars["Pokjar Tunas Ilmu Pusat"], OrangTuaID: ot.ID,
			ProgramID: strPtr(programs[sd.program]), Status: "aktif",
		}
		s.db.Create(&pd)
		s.db.Create(&RiwayatKelasPesertaDidik{PesertaDidikID: pd.ID, KelasID: kelasIDs[sd.kelas], TahunAjaranID: taID, Status: "aktif"})
		pdByKelas[sd.kelas] = append(pdByKelas[sd.kelas], pd.ID)
	}
	_ = birth

	// --- Modul L: ModulBelajar + CapaianModul ---
	modulIDs := map[string]string{}
	for _, md := range []struct {
		mapel, judul string
		urutan       int
		capaian      []string
	}{
		{"Matematika", "Modul 1: Bilangan Bulat", 1, []string{"Menjelaskan konsep bilangan bulat", "Melakukan operasi penjumlahan/pengurangan"}},
		{"Matematika", "Modul 2: Pengukuran", 2, []string{"Mengukur panjang dan berat", "Mengonversi satuan"}},
		{"Bahasa Indonesia", "Modul 1: Teks Narasi", 1, []string{"Mengidentifikasi struktur teks narasi", "Menulis teks narasi sederhana"}},
	} {
		mb := ModulBelajar{MapelID: mapelIDs[md.mapel], Judul: md.judul, Urutan: md.urutan, Deskripsi: "Modul pembelajaran " + md.judul}
		s.db.Create(&mb)
		modulIDs[md.judul] = mb.ID
		for i, c := range md.capaian {
			s.db.Create(&CapaianModul{ModulID: mb.ID, Kode: fmt.Sprintf("CM%d.%d", md.urutan, i+1), Deskripsi: c})
		}
	}

	// --- Modul M: Kompetensi + Capaian + RombelKompetensi + NilaiKompetensi ---
	kompetensiIDs := map[string]string{}
	for _, kd := range []struct {
		mapel, nama string
		capaian     []string
	}{
		{"Matematika", "Penjumlahan & Pengurangan", []string{"Operasi dasar penjumlahan", "Operasi dasar pengurangan", "Pemecahan masalah"}},
		{"Matematika", "Pengukuran", []string{"Mengukur panjang", "Mengukur berat"}},
		{"IPA", "Ekosistem", []string{"Komponen ekosistem", "Rantai makanan"}},
	} {
		kom := Kompetensi{MapelID: mapelIDs[kd.mapel], Nama: kd.nama}
		s.db.Create(&kom)
		kompetensiIDs[kd.mapel+"|"+kd.nama] = kom.ID
		for i, c := range kd.capaian {
			s.db.Create(&CapaianKompetensi{KompetensiID: kom.ID, Kode: fmt.Sprintf("CK%d", i+1), Deskripsi: c})
		}
	}
	// Assign kompetensi to kelas 7A (Matematika x2 + IPA) and kelas 8B (Bahasa Indonesia? no kompetensi for it; assign Matematika ones too for cross-sample)
	for _, key := range []string{"Matematika|Penjumlahan & Pengurangan", "Matematika|Pengukuran", "IPA|Ekosistem"} {
		s.db.Create(&RombelKompetensi{KelasID: kelas7A, KompetensiID: kompetensiIDs[key]})
	}
	// NilaiKompetensi for kelas 7A Ganjil (all siswa x 3 kompetensi)
	for i, pid := range pdByKelas["7A"] {
		for j, key := range []string{"Matematika|Penjumlahan & Pengurangan", "Matematika|Pengukuran", "IPA|Ekosistem"} {
			n := 70 + float64((i+j)%5)*5
			s.db.Create(&NilaiKompetensi{PesertaDidikID: pid, KompetensiID: kompetensiIDs[key], KelasID: kelas7A, Semester: "Ganjil", Nilai: n, DicatatOlehUserID: userIDs["guru1"]})
		}
	}

	// --- Modul B: Pengumuman ---
	allKelas := strPtr(kelas7A)
	s.db.Create(&Pengumuman{Judul: "Selamat Datang Semester Ganjil 2025/2026", Isi: "Pengumuman untuk seluruh staf PKBM Tunas Ilmu. Mohon hadir tepat waktu.", Target: "semua", Aktif: true, DibuatOlehUserID: adminID})
	s.db.Create(&Pengumuman{Judul: "Jadwal Ulangan Kelas 7A", Isi: "Ulangan Matematika akan dilaksanakan minggu depan.", Target: "kelas", KelasID: allKelas, Aktif: true, DibuatOlehUserID: userIDs["guru1"]})

	// --- Modul K: JurnalMengajar ---
	s.db.Create(&JurnalMengajar{TutorID: tutorIDs["guru1"], MapelID: mapelIDs["Matematika"], KelasID: kelas7A, Tanggal: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), Materi: "Bilangan Bulat", Kegiatan: "Penjelasan konsep + latihan soal", Status: "pending"})
	s.db.Create(&JurnalMengajar{TutorID: tutorIDs["guru1"], MapelID: mapelIDs["IPA"], KelasID: kelas7A, Tanggal: time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC), Materi: "Ekosistem", Kegiatan: "Diskusi kelompok", Status: "disetujui", ReviewedBy: strPtr(adminID), ReviewedAt: timePtr(time.Date(2025, 9, 10, 0, 0, 0, 0, time.UTC))})
	s.db.Create(&JurnalMengajar{TutorID: tutorIDs["guru2"], MapelID: mapelIDs["Bahasa Indonesia"], KelasID: kelas8B, Tanggal: time.Date(2025, 9, 5, 0, 0, 0, 0, time.UTC), Materi: "Teks Narasi", Kegiatan: "Menulis teks", Status: "ditolak", CatatanReviewer: "Kegiatan belum sesuai materi.", ReviewedBy: strPtr(adminID), ReviewedAt: timePtr(time.Date(2025, 9, 7, 0, 0, 0, 0, time.UTC))})

	// --- Modul C: Tugas + PengumpulanTugas (dinilai agar TUGAS source ada data NA) ---
	// Ensure uploads dir exists for any optional lampiran (not strictly needed; lampiran nullable).
	_ = os.MkdirAll("./uploads/tugas", 0o755)
	tugasMat := Tugas{MapelID: mapelIDs["Matematika"], KelasID: kelas7A, Judul: "Latihan Bilangan Bulat", Deskripsi: "Kerjakan soal halaman 12", Deadline: time.Date(2025, 9, 20, 23, 59, 0, 0, time.UTC), Semester: "Ganjil", BolehUpload: true, DibuatOlehUserID: userIDs["guru1"], ModulID: strPtr(modulIDs["Modul 1: Bilangan Bulat"])}
	s.db.Create(&tugasMat)
	tugasBindo := Tugas{MapelID: mapelIDs["Bahasa Indonesia"], KelasID: kelas8B, Judul: "Tulis Teks Narasi", Deskripsi: "Tulis narasi 200 kata", Deadline: time.Date(2025, 9, 25, 23, 59, 0, 0, time.UTC), Semester: "Ganjil", BolehUpload: true, DibuatOlehUserID: userIDs["guru2"]}
	s.db.Create(&tugasBindo)
	for i, pid := range pdByKelas["7A"] {
		nilai := 75 + float64(i%4)*5
		s.db.Create(&PengumpulanTugas{TugasID: tugasMat.ID, PesertaDidikID: pid, TanggalKumpul: time.Date(2025, 9, 18, 10, 0, 0, 0, time.UTC), JawabanTeks: "Jawaban " + pid, Status: "Dinilai", Nilai: floatPtr(nilai), CatatanTutor: "Bagus", DinilaiOlehUserID: strPtr(userIDs["guru1"])})
	}

	// --- Modul E: Materi + KomentarMateri (FilePath required; write placeholder file) ---
	_ = os.MkdirAll("./uploads/materi", 0o755)
	materiPaths := []string{}
	for _, mt := range []struct {
		mapel, judul, kelas, modul string
	}{
		{"Matematika", "Materi Bilangan Bulat", "7A", "Modul 1: Bilangan Bulat"},
		{"Bahasa Indonesia", "Materi Teks Narasi", "8B", "Modul 1: Teks Narasi"},
	} {
		fname := "dummy-" + uuid.NewString() + ".txt"
		rel := filepath.ToSlash(filepath.Join("uploads", "materi", fname))
		_ = os.WriteFile("./"+rel, []byte("Konten materi dummy untuk "+mt.judul), 0o644)
		materiPaths = append(materiPaths, rel)
		m := Materi{MapelID: mapelIDs[mt.mapel], KelasID: kelasIDs[mt.kelas], Judul: mt.judul, Deskripsi: "Materi pembelajaran " + mt.judul, FilePath: rel, Tipe: ".txt", Ukuran: 64, Semester: "Ganjil", DibuatOlehUserID: userIDs["guru1"], ModulID: strPtr(modulIDs[mt.modul])}
		s.db.Create(&m)
		s.db.Create(&KomentarMateri{MateriID: m.ID, UserID: strPtr(userIDs["guru2"]), Isi: "Materi ini cukup jelas."})
	}

	// --- Modul F: KelasVirtual ---
	s.db.Create(&KelasVirtual{MapelID: mapelIDs["Matematika"], KelasID: kelas7A, Judul: "Kelas Daring Matematika", Deskripsi: "Via Zoom", LinkMeeting: "https://zoom.us/j/1234567890", WaktuMulai: time.Date(2025, 9, 15, 9, 0, 0, 0, time.UTC), WaktuSelesai: time.Date(2025, 9, 15, 10, 30, 0, 0, time.UTC), Semester: "Ganjil", DibuatOlehUserID: userIDs["guru1"]})
	s.db.Create(&KelasVirtual{MapelID: mapelIDs["Bahasa Indonesia"], KelasID: kelas8B, Judul: "Kelas Daring Bahasa Indonesia", Deskripsi: "Via Google Meet", LinkMeeting: "https://meet.google.com/abc-defg-hij", WaktuMulai: time.Date(2025, 9, 16, 10, 0, 0, 0, time.UTC), WaktuSelesai: time.Date(2025, 9, 16, 11, 0, 0, 0, time.UTC), Semester: "Ganjil", DibuatOlehUserID: userIDs["guru2"]})

	// --- Modul D: BankSoal + Ujian + UjianSoal ---
	soalIDs := []string{}
	pgOpsi := `["15","20","25","30"]`
	for _, so := range []struct {
		mapel, tipe, soal, opsi, kunci string
		poin                           float64
	}{
		{"Matematika", "pg", "Hasil dari 12 + 8 adalah ...", pgOpsi, "0", 10},
		{"Matematika", "pg", "Hasil dari 50 - 17 adalah ...", `["23","33","43","53"]`, "1", 10},
		{"Matematika", "essay", "Jelaskan pengertian bilangan bulat!", "", "Bilangan bulat adalah bilangan yang terdiri dari bilangan bulat positif, nol, dan negatif.", 20},
		{"Matematika", "essay", "Sebutkan 3 contoh bilangan bulat negatif!", "", "Contoh: -1, -5, -100", 20},
	} {
		bs := BankSoal{MapelID: mapelIDs[so.mapel], Tipe: so.tipe, Pertanyaan: so.soal, Opsi: so.opsi, Kunci: so.kunci, Poin: so.poin, DibuatOlehUserID: userIDs["guru1"]}
		s.db.Create(&bs)
		soalIDs = append(soalIDs, bs.ID)
	}
	ujian := Ujian{MapelID: mapelIDs["Matematika"], KelasID: kelas7A, Judul: "Ulangan Harian Bilangan Bulat", WaktuMulai: time.Date(2025, 10, 1, 8, 0, 0, 0, time.UTC), WaktuSelesai: time.Date(2025, 10, 1, 9, 30, 0, 0, time.UTC), DurasiMenit: 90, AcakSoal: true, Semester: "Ganjil", DibuatOlehUserID: userIDs["guru1"]}
	s.db.Create(&ujian)
	for i, sid := range soalIDs {
		bobot := 25.0
		if i >= 2 {
			bobot = 50
		}
		s.db.Create(&UjianSoal{UjianID: ujian.ID, SoalID: sid, Bobot: bobot})
	}

	// --- Modul G: CatatanPerilaku ---
	for i, pid := range pdByKelas["7A"] {
		kategori := "positif"
		if i%2 == 1 {
			kategori = "negatif"
		}
		s.db.Create(&CatatanPerilaku{PesertaDidikID: pid, KelasID: kelas7A, Tanggal: time.Date(2025, 9, 10+i, 0, 0, 0, 0, time.UTC), Kategori: kategori, Deskripsi: "Catatan " + kategori + " contoh.", DicatatOlehUserID: userIDs["guru1"]})
	}

	// --- Modul H: Sertifikat (untuk siswa kelas 8B program C) ---
	for i, pid := range pdByKelas["8B"] {
		if i > 1 {
			break // 2 sertifikat
		}
		nomor := fmt.Sprintf("PKBM-2025-C-%03d", i+1)
		s.db.Create(&Sertifikat{PesertaDidikID: pid, ProgramID: programs["C"], Nomor: nomor, TanggalTerbit: time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC), Status: "terbit", DiterbitkanOlehUserID: adminID})
	}

	// --- Modul I: CatatanRapor ---
	for _, pid := range pdByKelas["7A"][:3] {
		naik := true
		s.db.Create(&CatatanRapor{PesertaDidikID: pid, TahunAjaranID: taID, Semester: "Ganjil", CatatanWali: "Semangat belajar, pertahankan prestasi.", NaikKelas: boolPtr(naik), KenaikanKe: strPtr("Kelas 8A")})
	}

	// --- Perpustakaan: Buku + BukuKelas + Peminjaman + Pengembalian ---
	bukuIDs := map[string]string{}
	for _, b := range []struct {
		judul, kode, penerbit string
	}{
		{"Matematika SMP Kelas 7", "BK-MAT-01", "Erlangga"},
		{"Bahasa Indonesia SMP", "BK-BIN-01", "Gramedia"},
		{"IPA Terpadu", "BK-IPA-01", "Yudhistira"},
	} {
		bk := Buku{Judul: b.judul, KodeBuku: b.kode, Penerbit: b.penerbit}
		s.db.Create(&bk)
		bukuIDs[b.judul] = bk.ID
	}
	s.db.Create(&BukuKelas{KelasID: kelas7A, BukuID: bukuIDs["Matematika SMP Kelas 7"], Semester: "Ganjil"})
	s.db.Create(&BukuKelas{KelasID: kelas7A, BukuID: bukuIDs["IPA Terpadu"], Semester: "Ganjil"})
	s.db.Create(&BukuKelas{KelasID: kelas8B, BukuID: bukuIDs["Bahasa Indonesia SMP"], Semester: "Ganjil"})
	// Peminjaman: 1 masih dipinjam, 1 dikembalikan
	if len(pdByKelas["7A"]) > 0 {
		pjm := Peminjaman{PesertaDidikID: pdByKelas["7A"][0], BukuID: bukuIDs["Matematika SMP Kelas 7"], KelasID: kelas7A, Semester: "Ganjil", TanggalPinjam: time.Date(2025, 9, 5, 0, 0, 0, 0, time.UTC), Status: "Dipinjam", DicatatOlehUserID: userIDs["guru1"]}
		s.db.Create(&pjm)
		if len(pdByKelas["7A"]) > 1 {
			pjm2 := Peminjaman{PesertaDidikID: pdByKelas["7A"][1], BukuID: bukuIDs["IPA Terpadu"], KelasID: kelas7A, Semester: "Ganjil", TanggalPinjam: time.Date(2025, 9, 3, 0, 0, 0, 0, time.UTC), Status: "Dikembalikan", DicatatOlehUserID: userIDs["guru1"]}
			s.db.Create(&pjm2)
			s.db.Create(&Pengembalian{PeminjamanID: pjm2.ID, TanggalKembali: time.Date(2025, 9, 20, 0, 0, 0, 0, time.UTC), KondisiBuku: "Baik", Catatan: "Kembali lengkap", DicatatOlehUserID: userIDs["guru1"]})
		}
	}

	// --- Presensi + PresensiDetail (1 meeting per kelas, Ganjil) ---
	for _, kkey := range []string{"7A", "8B"} {
		kid := kelasIDs[kkey]
		p := Presensi{KelasID: kid, Tanggal: time.Date(2025, 9, 10, 8, 0, 0, 0, time.UTC), Semester: "Ganjil", StatusPertemuan: "selesai", TutorID: strPtr(tutorIDs[map[string]string{"7A": "guru1", "8B": "guru2"}[kkey]])}
		s.db.Create(&p)
		statuses := []string{"Hadir", "Hadir", "Hadir", "Sakit", "Izin", "Alpa"}
		for i, pid := range pdByKelas[kkey] {
			st := statuses[i%len(statuses)]
			s.db.Create(&PresensiDetail{PresensiID: p.ID, PesertaDidikID: pid, StatusKehadiran: st})
		}
	}

	// --- Modul Nilai: Tema + CapaianPembelajaran + NilaiCP + NilaiUM, lalu recompute ---
	// Seed tema for: 7A x Matematika, 7A x IPA, 8B x Bahasa Indonesia (Ganjil).
	temaConfigs := []struct {
		kelas, mapel string
		jumlahCP     int
	}{
		{"7A", "Matematika", 3},
		{"7A", "IPA", 2},
		{"8B", "Bahasa Indonesia", 2},
	}
	for _, tc := range temaConfigs {
		kid := kelasIDs[tc.kelas]
		mid := mapelIDs[tc.mapel]
		tema := Tema{KelasID: kid, MapelID: mid, TahunAjaranID: taID, Semester: "Ganjil", NamaTema: "Tema 1 " + tc.mapel, Urutan: 1, JumlahCP: tc.jumlahCP, BobotKeterampilan: 40, BobotPengetahuan: 60}
		if e := s.db.Create(&tema).Error; e != nil {
			continue
		}
		for cp := 1; cp <= tc.jumlahCP; cp++ {
			s.db.Create(&CapaianPembelajaran{TemaID: tema.ID, UrutanCP: cp, LabelDefault: fmt.Sprintf("CP %d", cp)})
		}
		for _, pid := range pdByKelas[tc.kelas] {
			for cp := 1; cp <= tc.jumlahCP; cp++ {
				nk := 70 + float64((cp*3)%4)*5
				s.db.Create(&NilaiCP{TemaID: tema.ID, UrutanCP: cp, PesertaDidikID: pid, DeskripsiCP: fmt.Sprintf("CP %d", cp), NilaiKeterampilan: floatPtr(nk)})
			}
			um := 75 + float64(5)
			s.db.Create(&NilaiUM{TemaID: tema.ID, PesertaDidikID: pid, NilaiUM: floatPtr(um)})
		}
		// Recompute rekap per (peserta, mapel, kelas, tahun, semester)
		for _, pid := range pdByKelas[tc.kelas] {
			_ = s.recomputeRekap(s.db, pid, mid, kid, taID, "Ganjil")
		}
	}

	// --- Audit trail marker ---
	s.audit(&adminID, "seed", "dummy", "comprehensive dummy data seeded")

	// --- Kalender Akademik events ---
	events := []KalenderEvent{
		{Judul: "Libur Nasional - Hari Pancasila", TanggalMulai: time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local), Tipe: "libur", Warna: "#dc2626", DibuatOlehUserID: adminID},
		{Judul: "Ujian Tengah Semester Ganjil", TanggalMulai: time.Date(2025, 10, 6, 0, 0, 0, 0, time.Local), TanggalSelesai: timePtr(time.Date(2025, 10, 17, 0, 0, 0, 0, time.Local)), Tipe: "ujian", Warna: "#2563eb", DibuatOlehUserID: adminID},
		{Judul: "Hari Guru Nasional", TanggalMulai: time.Date(2025, 11, 25, 0, 0, 0, 0, time.Local), Tipe: "kegiatan", Warna: "#16a34a", DibuatOlehUserID: adminID},
		{Judul: "Ujian Akhir Semester Ganjil", TanggalMulai: time.Date(2025, 12, 1, 0, 0, 0, 0, time.Local), TanggalSelesai: timePtr(time.Date(2025, 12, 19, 0, 0, 0, 0, time.Local)), Tipe: "ujian", Warna: "#2563eb", DibuatOlehUserID: adminID},
		{Judul: "Libur Semester", TanggalMulai: time.Date(2025, 12, 22, 0, 0, 0, 0, time.Local), TanggalSelesai: timePtr(time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local)), Tipe: "libur", Warna: "#dc2626", DibuatOlehUserID: adminID},
	}
	for _, ev := range events {
		s.db.Create(&ev)
	}

	// --- Notifikasi for admin ---
	notifs := []Notifikasi{
		{UserID: adminID, Judul: "Selamat Datang", Isi: "Sistem LMS PKBM Tunas Ilmu telah berhasil diinisialisasi dengan data dummy.", Tipe: "umum"},
		{UserID: adminID, Judul: "Fitur Ujian Online Aktif", Isi: "Ujian Online kini tersedia. Buat ujian dengan kode akses untuk siswa mengerjakan tanpa login.", Tipe: "ujian"},
		{UserID: adminID, Judul: "Portal Orang Tua Aktif", Isi: "Orang tua dapat login dengan NIK + NISN anak untuk melihat nilai dan presensi.", Tipe: "umum"},
	}
	for _, n := range notifs {
		s.db.Create(&n)
	}

	// --- Parent account demo ---
	var ortu OrangTua
	if s.db.First(&ortu).Error == nil {
		var existing User
		if s.db.Where("username = ?", "orangtua1").First(&existing).Error != nil {
			hash, _ := bcryptHash("OrangTua123")
			s.db.Create(&User{Username: "orangtua1", Email: "orangtua1@pkbm.local", PasswordHash: hash, Role: "orang_tua", OrangTuaID: &ortu.ID, IsActive: true})
		}
	}

	return "OK: dummy data lengkap diisi (user: kepala/Kepala123, guru1 & guru2/Guru1234, orangtua1/OrangTua123). Login admin/Admin123 untuk akses penuh.", nil
}

// seedDummyHandler is the admin-only endpoint that triggers seedDummy.
func (s *Server) seedDummyHandler(c *fiber.Ctx) error {
	if c.Locals("role") != "admin" {
		return fiber.NewError(403, "admin access required")
	}
	msg, err := s.seedDummy()
	if err != nil {
		return fiber.NewError(500, err.Error())
	}
	return c.JSON(fiber.Map{"message": msg})
}
