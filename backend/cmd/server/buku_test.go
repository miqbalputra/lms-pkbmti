package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Minimal valid 1x1 PNG as a data URL — exercises validSignature end-to-end
// (PNG magic bytes + base64 prefix + non-trivial length).
const validPngSignature = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// loginRole creates a user of the given role (optionally linked to a tutor) and
// returns an access token by logging in. Admin token is needed to create users.
func loginRole(t *testing.T, app *fiber.App, adminToken, username, role string, tutorID *string) string {
	t.Helper()
	body := map[string]interface{}{
		"username": username, "email": username + "@example.com", "password": "Password123",
		"role": role, "isActive": true,
	}
	if tutorID != nil {
		body["tutorId"] = *tutorID
	}
	res, _ := makeRequest(app, "POST", "/api/users", adminToken, body, "")
	if res.StatusCode != 201 {
		t.Fatalf("create user %s returned %d", username, res.StatusCode)
	}
	resLogin, _ := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login": username, "password": "Password123",
	}, "")
	if resLogin.StatusCode != 200 {
		t.Fatalf("%s %s login returned %d", role, username, resLogin.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resLogin.Body).Decode(&result)
	return result["accessToken"].(string)
}

func TestE2E_BukuLending(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	// 1. Admin creates a Buku → 201.
	resBuku, _ := makeRequest(app, "POST", "/api/buku", token, map[string]interface{}{
		"judul": "Matematika Paket C", "kodeBuku": "MTC-1", "penerbit": "Penerbit X",
	}, "")
	if resBuku.StatusCode != 201 {
		t.Fatalf("buku create returned %d", resBuku.StatusCode)
	}
	var buku Buku
	json.NewDecoder(resBuku.Body).Decode(&buku)

	// 2. Tutor (wali), Kelas, then BukuKelas assignment → 201 (semester auto).
	tutor := Tutor{Nama: "Wali Buku", JenisKelamin: "L"}
	if e := s.db.Create(&tutor).Error; e != nil {
		t.Fatalf("tutor create: %v", e)
	}
	resKelas, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang": 3, "namaRombel": "BUKU", "pokjarId": pokjar.ID, "tahunAjaranId": year.ID, "waliKelasId": tutor.ID,
	}, "")
	if resKelas.StatusCode != 201 {
		t.Fatalf("kelas create returned %d", resKelas.StatusCode)
	}
	var kelas Kelas
	json.NewDecoder(resKelas.Body).Decode(&kelas)
	kelasID := kelas.ID

	resBK, _ := makeRequest(app, "POST", "/api/buku-kelas", token, map[string]interface{}{
		"kelasId": kelasID, "bukuId": buku.ID,
	}, "")
	if resBK.StatusCode != 201 {
		t.Fatalf("buku-kelas create returned %d", resBK.StatusCode)
	}
	var bk BukuKelas
	json.NewDecoder(resBK.Body).Decode(&bk)
	if bk.Semester != "Ganjil" && bk.Semester != "Genap" {
		t.Errorf("buku-kelas semester not set, got %q", bk.Semester)
	}
	// Duplicate assignment → 400.
	resDup, _ := makeRequest(app, "POST", "/api/buku-kelas", token, map[string]interface{}{
		"kelasId": kelasID, "bukuId": buku.ID,
	}, "")
	if resDup.StatusCode != 400 {
		t.Errorf("expected 400 for duplicate buku-kelas, got %d", resDup.StatusCode)
	}

	// 3. Guru (wali) + a second tutor (not wali).
	guruToken := loginRole(t, app, token, "guru-buku-1", "guru", &tutor.ID)
	tutor2 := Tutor{Nama: "Lain", JenisKelamin: "P"}
	s.db.Create(&tutor2)
	guru2Token := loginRole(t, app, token, "guru-buku-2", "guru", &tutor2.ID)

	// 4. Guru lists kelas they walikan via /kelas → includes their rombel.
	resMyKelas, _ := makeRequest(app, "GET", "/api/kelas", guruToken, nil, "")
	var myKelas []Kelas
	json.NewDecoder(resMyKelas.Body).Decode(&myKelas)
	foundMine := false
	for _, k := range myKelas {
		if k.ID == kelasID {
			foundMine = true
		}
	}
	if !foundMine {
		t.Errorf("guru /kelas should include wali kelas %s", kelasID)
	}

	// 5. Student in the kelas (direct DB).
	siswa := PesertaDidik{Nama: "Siswa Buku", JenisKelamin: "L", NIS: "NIS-BK", NISN: "NISN-BK", NIK: "NIK-BK-1", KelasID: kelasID, PokjarID: pokjar.ID, Status: "aktif"}
	if e := s.db.Create(&siswa).Error; e != nil {
		t.Fatalf("siswa create: %v", e)
	}

	// 6. Guru records peminjaman for their wali kelas → 201.
	resPinjam, _ := makeRequest(app, "POST", "/api/peminjaman-buku", guruToken, map[string]interface{}{
		"kelasId": kelasID,
		"items": []map[string]interface{}{
			{"pesertaDidikId": siswa.ID, "bukuId": buku.ID, "tanggalPinjam": time.Now().Format("2006-01-02")},
		},
		"tandaTangan": validPngSignature,
	}, "")
	if resPinjam.StatusCode != 201 {
		t.Fatalf("peminjaman create returned %d (want 201)", resPinjam.StatusCode)
	}
	var created []Peminjaman
	json.NewDecoder(resPinjam.Body).Decode(&created)
	if len(created) != 1 || created[0].Status != "Dipinjam" {
		t.Errorf("peminjaman not recorded as Dipinjam, got %+v", created)
	}
	peminjamanID := created[0].ID

	// 6b. Missing signature → 400.
	resNoSig, _ := makeRequest(app, "POST", "/api/peminjaman-buku", guruToken, map[string]interface{}{
		"kelasId": kelasID,
		"items":   []map[string]interface{}{{"pesertaDidikId": siswa.ID, "bukuId": buku.ID}},
		"tandaTangan": "",
	}, "")
	if resNoSig.StatusCode != 400 {
		t.Errorf("expected 400 for missing signature, got %d", resNoSig.StatusCode)
	}

	// 6c. Unassigned book → 400.
	resBuku2, _ := makeRequest(app, "POST", "/api/buku", token, map[string]interface{}{"judul": "Tidak Ditetapkan"}, "")
	var buku2 Buku
	json.NewDecoder(resBuku2.Body).Decode(&buku2)
	resUnassigned, _ := makeRequest(app, "POST", "/api/peminjaman-buku", guruToken, map[string]interface{}{
		"kelasId": kelasID,
		"items":   []map[string]interface{}{{"pesertaDidikId": siswa.ID, "bukuId": buku2.ID}},
		"tandaTangan": validPngSignature,
	}, "")
	if resUnassigned.StatusCode != 400 {
		t.Errorf("expected 400 for unassigned buku, got %d", resUnassigned.StatusCode)
	}

	// 7. Non-wali guru → 403.
	resForbidden, _ := makeRequest(app, "POST", "/api/peminjaman-buku", guru2Token, map[string]interface{}{
		"kelasId": kelasID,
		"items":   []map[string]interface{}{{"pesertaDidikId": siswa.ID, "bukuId": buku.ID}},
		"tandaTangan": validPngSignature,
	}, "")
	if resForbidden.StatusCode != 403 {
		t.Errorf("expected 403 for non-wali guru, got %d", resForbidden.StatusCode)
	}

	// 8. Active peminjaman list → the loan shows.
	resAktif, _ := makeRequest(app, "GET", "/api/peminjaman-buku/aktif?kelasId="+kelasID, guruToken, nil, "")
	if resAktif.StatusCode != 200 {
		t.Fatalf("aktif list returned %d", resAktif.StatusCode)
	}
	var aktif []Peminjaman
	json.NewDecoder(resAktif.Body).Decode(&aktif)
	if len(aktif) != 1 || aktif[0].ID != peminjamanID {
		t.Errorf("aktif list should contain the open loan, got %+v", aktif)
	}

	// 9. Pengembalian "Hilang" without catatan → 400.
	resKembaliNoNote, _ := makeRequest(app, "POST", "/api/peminjaman-buku/kembali", guruToken, map[string]interface{}{
		"items": []map[string]interface{}{
			{"peminjamanId": peminjamanID, "kondisiBuku": "Hilang", "catatan": ""},
		},
		"tandaTangan": validPngSignature,
	}, "")
	if resKembaliNoNote.StatusCode != 400 {
		t.Errorf("expected 400 for Hilang without catatan, got %d", resKembaliNoNote.StatusCode)
	}

	// 10. Pengembalian with catatan → 201, status flips to Dikembalikan.
	resKembali, _ := makeRequest(app, "POST", "/api/peminjaman-buku/kembali", guruToken, map[string]interface{}{
		"items": []map[string]interface{}{
			{"peminjamanId": peminjamanID, "kondisiBuku": "Hilang", "catatan": "ganti buku, info ortu"},
		},
		"tandaTangan": validPngSignature,
	}, "")
	if resKembali.StatusCode != 201 {
		t.Fatalf("pengembalian create returned %d (want 201)", resKembali.StatusCode)
	}
	var p Peminjaman
	s.db.First(&p, "id = ?", peminjamanID)
	if p.Status != "Dikembalikan" {
		t.Errorf("peminjaman status = %q, want Dikembalikan", p.Status)
	}
	// 10b. Double return → 400.
	resDouble, _ := makeRequest(app, "POST", "/api/peminjaman-buku/kembali", guruToken, map[string]interface{}{
		"items": []map[string]interface{}{
			{"peminjamanId": peminjamanID, "kondisiBuku": "Baik", "catatan": ""},
		},
		"tandaTangan": validPngSignature,
	}, "")
	if resDouble.StatusCode != 400 {
		t.Errorf("expected 400 for double return, got %d", resDouble.StatusCode)
	}

	// 11. Admin rekap → has the row.
	resRekap, _ := makeRequest(app, "GET", "/api/buku/rekap?kelasId="+kelasID, token, nil, "")
	if resRekap.StatusCode != 200 {
		t.Fatalf("rekap returned %d", resRekap.StatusCode)
	}
	var rekaps []map[string]json.RawMessage
	json.NewDecoder(resRekap.Body).Decode(&rekaps)
	if len(rekaps) != 1 {
		t.Errorf("rekap should have 1 row, got %d", len(rekaps))
	}

	// 12. Export XLSX and PDF → 200.
	resXlsx, _ := makeRequest(app, "GET", "/api/buku/export?kelasId="+kelasID+"&format=xlsx", token, nil, "")
	if resXlsx.StatusCode != 200 {
		t.Errorf("export xlsx returned %d", resXlsx.StatusCode)
	}
	resPdf, _ := makeRequest(app, "GET", "/api/buku/export?kelasId="+kelasID+"&format=pdf", token, nil, "")
	if resPdf.StatusCode != 200 {
		t.Errorf("export pdf returned %d", resPdf.StatusCode)
	}

	// 13. Kepala_sekolah: rekap 200; POST peminjaman → 403.
	kepalaToken := loginRole(t, app, token, "kepala-buku", "kepala_sekolah", nil)
	resRekapK, _ := makeRequest(app, "GET", "/api/buku/rekap", kepalaToken, nil, "")
	if resRekapK.StatusCode != 200 {
		t.Errorf("kepala rekap returned %d", resRekapK.StatusCode)
	}
	resKepalaPost, _ := makeRequest(app, "POST", "/api/peminjaman-buku", kepalaToken, map[string]interface{}{
		"kelasId": kelasID, "items": []map[string]interface{}{{"pesertaDidikId": siswa.ID, "bukuId": buku.ID}}, "tandaTangan": validPngSignature,
	}, "")
	if resKepalaPost.StatusCode != 403 {
		t.Errorf("expected 403 for kepala_sekolah write, got %d", resKepalaPost.StatusCode)
	}

	// 14. semester fallback: genap field unset → month-based (identical to legacy).
	jan, _ := time.Parse("2006-01-02", "2026-01-15")
	if got := s.semester(jan); got != "Genap" {
		t.Errorf("semester(January) = %q, want Genap (fallback)", got)
	}
	jul, _ := time.Parse("2006-01-02", "2026-07-15")
	if got := s.semester(jul); got != "Ganjil" {
		t.Errorf("semester(July) = %q, want Ganjil (fallback)", got)
	}
	// 14b. Set genap start to Feb 1 → January is Ganjil, March is Genap.
	genapStart := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	year.TanggalMulaiSemesterGenap = &genapStart
	s.db.Save(&year)
	if got := s.semester(jan); got != "Ganjil" {
		t.Errorf("semester(January) with genap=Feb1 = %q, want Ganjil", got)
	}
	mar, _ := time.Parse("2006-01-02", "2026-03-15")
	if got := s.semester(mar); got != "Genap" {
		t.Errorf("semester(March) with genap=Feb1 = %q, want Genap", got)
	}
}