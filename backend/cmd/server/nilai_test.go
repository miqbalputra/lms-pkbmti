package main

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// loginAs logs in as an arbitrary user (by username/password) and returns the access token.
func loginAs(t *testing.T, app *fiber.App, username, password string) string {
	t.Helper()
	res, err := makeRequest(app, "POST", "/api/auth/login", "", map[string]string{
		"login":    username,
		"password": password,
	}, "")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("login as %s failed: status %d, err %v", username, res.StatusCode, err)
	}
	defer res.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	tok, ok := result["accessToken"].(string)
	if !ok || tok == "" {
		t.Fatalf("accessToken missing for %s", username)
	}
	return tok
}

// TestE2E_NilaiFlow exercises the full Modul Nilai lifecycle: settings seed, tema create,
// input grid, nilai save, rekap computation, exports, RBAC, active-year guard, and
// tema-delete rekap cleanup. See prd_nilai.md.
func TestE2E_NilaiFlow(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	// --- 1. Mapel seed: Matematika (80/68/60) vs default (90/78/70) ---
	resMTK, _ := makeRequest(app, "POST", "/api/mapel", token, map[string]interface{}{"namaMapel": "Matematika", "kodeMapel": "MTK", "isActive": true}, "")
	if resMTK.StatusCode != 201 {
		t.Fatalf("mapel Matematika create returned %d", resMTK.StatusCode)
	}
	var mtk MataPelajaran
	json.NewDecoder(resMTK.Body).Decode(&mtk)

	resBIO, _ := makeRequest(app, "POST", "/api/mapel", token, map[string]interface{}{"namaMapel": "Bahasa Indonesia", "kodeMapel": "BIO", "isActive": true}, "")
	var bio MataPelajaran
	json.NewDecoder(resBIO.Body).Decode(&bio)

	resSet, _ := makeRequest(app, "GET", "/api/settings/nilai?mapelId="+mtk.ID, token, nil, "")
	if resSet.StatusCode != 200 {
		t.Fatalf("settings MTK GET returned %d", resSet.StatusCode)
	}
	var setM map[string]interface{}
	json.NewDecoder(resSet.Body).Decode(&setM)
	if setM["bobotKeterampilan"].(float64) != 60 || setM["bobotPengetahuan"].(float64) != 40 {
		t.Errorf("MTK bobot seed = %v/%v, want 60/40", setM["bobotKeterampilan"], setM["bobotPengetahuan"])
	}
	ambM := setM["ambang"].([]interface{})
	wantMin := map[string]float64{"A": 80, "B": 68, "C": 60}
	for _, a := range ambM {
		row := a.(map[string]interface{})
		p := row["predikat"].(string)
		min := row["nilaiMinimum"].(float64)
		if wantMin[p] != min {
			t.Errorf("MTK ambang %s = %v, want %v", p, min, wantMin[p])
		}
	}

	resSetB, _ := makeRequest(app, "GET", "/api/settings/nilai?mapelId="+bio.ID, token, nil, "")
	var setB map[string]interface{}
	json.NewDecoder(resSetB.Body).Decode(&setB)
	ambB := setB["ambang"].([]interface{})
	wantMinB := map[string]float64{"A": 90, "B": 78, "C": 70}
	for _, a := range ambB {
		row := a.(map[string]interface{})
		p := row["predikat"].(string)
		min := row["nilaiMinimum"].(float64)
		if wantMinB[p] != min {
			t.Errorf("BIO ambang %s = %v, want %v", p, min, wantMinB[p])
		}
	}

	// --- 2. Scaffolding: tutor, guru user, kelas, kelas-mapel, penugasan, 2 siswa ---
	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	resTutor, _ := makeRequest(app, "POST", "/api/tutor", token, map[string]interface{}{"nama": "Tutor Nilai", "jenisKelamin": "L"}, "")
	var tutor Tutor
	json.NewDecoder(resTutor.Body).Decode(&tutor)

	resUser, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "guru_nilai", "email": "guru.nilai@example.com", "password": "Guru12345", "role": "guru", "tutorId": tutor.ID, "isActive": true,
	}, "")
	if resUser.StatusCode != 201 {
		t.Fatalf("guru user create returned %d", resUser.StatusCode)
	}
	guruToken := loginAs(t, app, "guru_nilai", "Guru12345")

	resKelas, _ := makeRequest(app, "POST", "/api/kelas", token, map[string]interface{}{
		"jenjang": 1, "namaRombel": "N", "pokjarId": pokjar.ID, "tahunAjaranId": year.ID, "waliKelasId": tutor.ID,
	}, "")
	if resKelas.StatusCode != 201 {
		t.Fatalf("kelas create returned %d", resKelas.StatusCode)
	}
	var kelas Kelas
	json.NewDecoder(resKelas.Body).Decode(&kelas)

	makeRequest(app, "PUT", "/api/kelas/"+kelas.ID+"/mapel", token, map[string]interface{}{"mapelIds": []string{mtk.ID, bio.ID}}, "")
	resPg, _ := makeRequest(app, "POST", "/api/penugasan", token, map[string]interface{}{"tutorId": tutor.ID, "kelasId": kelas.ID, "mapelId": mtk.ID}, "")
	if resPg.StatusCode != 201 {
		t.Fatalf("penugasan create returned %d", resPg.StatusCode)
	}

	// Two students.
	for i, nis := range []string{"NIS-N1", "NIS-N2"} {
		r, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
			"nama": "Siswa " + nis, "jenisKelamin": "L", "nis": nis, "nisn": "NISN-" + nis, "nik": "NIK-" + nis,
			"kelasId": kelas.ID, "pokjarId": pokjar.ID,
		}, "")
		if r.StatusCode != 201 {
			t.Fatalf("siswa %d create returned %d", i, r.StatusCode)
		}
	}

	// --- 3. Create tema (admin) jumlahCp=2 ---
	resTema, _ := makeRequest(app, "POST", "/api/tema", token, map[string]interface{}{
		"kelasId": kelas.ID, "mapelId": mtk.ID, "tahunAjaranId": year.ID, "semester": "Ganjil",
		"namaTema": "Tema 1", "urutan": 1, "jumlahCp": 2, "labelDefaults": []string{"CP1 desc", "CP2 desc"},
	}, "")
	if resTema.StatusCode != 201 {
		t.Fatalf("tema create returned %d", resTema.StatusCode)
	}
	var tema Tema
	json.NewDecoder(resTema.Body).Decode(&tema)

	var cpCount, ncpCount, umCount int64
	s.db.Model(&CapaianPembelajaran{}).Where("tema_id = ?", tema.ID).Count(&cpCount)
	s.db.Model(&NilaiCP{}).Where("tema_id = ?", tema.ID).Count(&ncpCount)
	s.db.Model(&NilaiUM{}).Where("tema_id = ?", tema.ID).Count(&umCount)
	if cpCount != 2 || ncpCount != 4 || umCount != 2 {
		t.Errorf("tema seed counts cp=%d ncp=%d um=%d, want 2/4/2", cpCount, ncpCount, umCount)
	}

	// --- 4. Grid GET (guru) ---
	resGrid, _ := makeRequest(app, "GET", "/api/tema/"+tema.ID+"/grid", guruToken, nil, "")
	if resGrid.StatusCode != 200 {
		gb, _ := io.ReadAll(resGrid.Body)
		t.Fatalf("guru grid GET returned %d: %s", resGrid.StatusCode, string(gb))
	}
	var grid map[string]interface{}
	json.NewDecoder(resGrid.Body).Decode(&grid)
	students := grid["students"].([]interface{})
	if len(students) != 2 {
		t.Fatalf("grid students = %d, want 2", len(students))
	}
	first := students[0].(map[string]interface{})
	cells := first["cp"].([]interface{})
	if len(cells) != 2 {
		t.Errorf("grid cp = %d, want 2", len(cells))
	}
	if cells[0].(map[string]interface{})["nilaiKeterampilan"] != nil {
		t.Errorf("grid CP1 nilaiKeterampilan = %v, want nil (belum dinilai)", cells[0].(map[string]interface{})["nilaiKeterampilan"])
	}

	// --- 5. Save nilai (guru) ---
	var siswas []PesertaDidik
	s.db.Where("kelas_id = ? AND status = ?", kelas.ID, "aktif").Order("nama").Find(&siswas)
	if len(siswas) != 2 {
		t.Fatalf("expected 2 siswa, got %d", len(siswas))
	}
	s1 := siswas[0]
	s2 := siswas[1]

	saveBody := map[string]interface{}{
		"values": []map[string]interface{}{
			{
				"pesertaDidikId": s1.ID,
				"cp": []map[string]interface{}{
					{"urutanCp": 1, "deskripsiCp": "CP1 desc", "nilaiKeterampilan": 80.0},
					{"urutanCp": 2, "deskripsiCp": "CP2 desc", "nilaiKeterampilan": 90.0},
				},
				"nilaiUm": 85.0,
			},
			{
				"pesertaDidikId": s2.ID,
				"cp": []map[string]interface{}{
					{"urutanCp": 1, "deskripsiCp": "CP1 desc", "nilaiKeterampilan": 70.0},
					{"urutanCp": 2, "deskripsiCp": "CP2 desc", "nilaiKeterampilan": 75.0},
				},
				"nilaiUm": 80.0,
			},
		},
	}
	resSave, _ := makeRequest(app, "PUT", "/api/tema/"+tema.ID+"/nilai", guruToken, saveBody, "")
	if resSave.StatusCode != 200 {
		t.Fatalf("guru save nilai returned %d", resSave.StatusCode)
	}

	// --- 6. Rekap computation ---
	resRekap, _ := makeRequest(app, "GET", "/api/nilai/rekap?kelasId="+kelas.ID+"&mapelId="+mtk.ID+"&semester=Ganjil&tahunAjaranId="+year.ID, guruToken, nil, "")
	if resRekap.StatusCode != 200 {
		t.Fatalf("rekap GET returned %d", resRekap.StatusCode)
	}
	var rekaps []RekapNilaiAkhir
	json.NewDecoder(resRekap.Body).Decode(&rekaps)
	if len(rekaps) != 2 {
		t.Fatalf("rekap rows = %d, want 2", len(rekaps))
	}
	byID := map[string]RekapNilaiAkhir{}
	for _, r := range rekaps {
		byID[r.PesertaDidikID] = r
	}
	r1 := byID[s1.ID]
	if r1.NPAkhir == nil || *r1.NPAkhir != 85.0 {
		t.Errorf("siswa1 NP = %v, want 85", r1.NPAkhir)
	}
	if r1.NKAkhir == nil || *r1.NKAkhir != 85.0 {
		t.Errorf("siswa1 NK = %v, want 85", r1.NKAkhir)
	}
	if r1.PredikatNP != "A" || r1.PredikatNK != "A" {
		t.Errorf("siswa1 predikat NP/NK = %s/%s, want A/A", r1.PredikatNP, r1.PredikatNK)
	}
	r2 := byID[s2.ID]
	if r2.NPAkhir == nil || *r2.NPAkhir != 80.0 {
		t.Errorf("siswa2 NP = %v, want 80", r2.NPAkhir)
	}
	if r2.NKAkhir == nil || *r2.NKAkhir != 72.5 {
		t.Errorf("siswa2 NK = %v, want 72.5", r2.NKAkhir)
	}
	if r2.PredikatNP != "A" || r2.PredikatNK != "B" {
		t.Errorf("siswa2 predikat NP/NK = %s/%s, want A/B", r2.PredikatNP, r2.PredikatNK)
	}

	// --- 7. Exports ---
	resXlsx, _ := makeRequest(app, "GET", "/api/nilai/export?kelasId="+kelas.ID+"&mapelId="+mtk.ID+"&semester=Ganjil&tahunAjaranId="+year.ID+"&format=xlsx", guruToken, nil, "")
	if resXlsx.StatusCode != 200 {
		t.Fatalf("export xlsx returned %d", resXlsx.StatusCode)
	}
	if ct := resXlsx.Header.Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("export xlsx content-type = %s", ct)
	}
	body, _ := io.ReadAll(resXlsx.Body)
	if len(body) == 0 {
		t.Errorf("export xlsx body empty")
	}

	resPdf, _ := makeRequest(app, "GET", "/api/nilai/export?kelasId="+kelas.ID+"&mapelId="+mtk.ID+"&semester=Ganjil&tahunAjaranId="+year.ID+"&format=pdf", guruToken, nil, "")
	if resPdf.StatusCode != 200 {
		t.Fatalf("export pdf returned %d", resPdf.StatusCode)
	}
	if ct := resPdf.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("export pdf content-type = %s", ct)
	}
	pdfBody, _ := io.ReadAll(resPdf.Body)
	if len(pdfBody) == 0 {
		t.Errorf("export pdf body empty")
	}

	// --- 8. RBAC: guru not assigned to (kelas, bio) ---
	resTema2, _ := makeRequest(app, "POST", "/api/tema", token, map[string]interface{}{
		"kelasId": kelas.ID, "mapelId": bio.ID, "tahunAjaranId": year.ID, "semester": "Ganjil",
		"namaTema": "Tema BIO", "urutan": 1, "jumlahCp": 1, "labelDefaults": []string{"CP BIO"},
	}, "")
	var tema2 Tema
	json.NewDecoder(resTema2.Body).Decode(&tema2)

	resGridDeny, _ := makeRequest(app, "GET", "/api/tema/"+tema2.ID+"/grid", guruToken, nil, "")
	if resGridDeny.StatusCode != 403 {
		t.Errorf("guru grid GET for unassigned mapel = %d, want 403", resGridDeny.StatusCode)
	}
	resSaveDeny, _ := makeRequest(app, "PUT", "/api/tema/"+tema2.ID+"/nilai", guruToken, saveBody, "")
	if resSaveDeny.StatusCode != 403 {
		t.Errorf("guru save nilai for unassigned mapel = %d, want 403", resSaveDeny.StatusCode)
	}

	// --- 9. RBAC: kepala_sekolah (read-only) ---
	resKepUser, _ := makeRequest(app, "POST", "/api/users", token, map[string]interface{}{
		"username": "kepala_nilai", "email": "kepala.nilai@example.com", "password": "Kepala123", "role": "kepala_sekolah", "isActive": true,
	}, "")
	if resKepUser.StatusCode != 201 {
		t.Fatalf("kepala user create returned %d", resKepUser.StatusCode)
	}
	kepToken := loginAs(t, app, "kepala_nilai", "Kepala123")
	resKepGrid, _ := makeRequest(app, "GET", "/api/tema/"+tema.ID+"/grid", kepToken, nil, "")
	if resKepGrid.StatusCode != 200 {
		t.Errorf("kepala grid GET = %d, want 200", resKepGrid.StatusCode)
	}
	resKepSave, _ := makeRequest(app, "PUT", "/api/tema/"+tema.ID+"/nilai", kepToken, saveBody, "")
	if resKepSave.StatusCode != 403 {
		t.Errorf("kepala save nilai = %d, want 403", resKepSave.StatusCode)
	}
	resKepSet, _ := makeRequest(app, "PUT", "/api/settings/nilai", kepToken, map[string]interface{}{"mapelId": mtk.ID, "bobotKeterampilan": 70, "bobotPengetahuan": 30, "ambang": []map[string]interface{}{{"predikat": "A", "nilaiMinimum": 90}, {"predikat": "B", "nilaiMinimum": 78}, {"predikat": "C", "nilaiMinimum": 70}}}, "")
	if resKepSet.StatusCode != 403 {
		t.Errorf("kepala put settings = %d, want 403", resKepSet.StatusCode)
	}
	resKepRekap, _ := makeRequest(app, "GET", "/api/nilai/rekap?kelasId="+kelas.ID+"&mapelId="+mtk.ID+"&semester=Ganjil&tahunAjaranId="+year.ID, kepToken, nil, "")
	if resKepRekap.StatusCode != 200 {
		t.Errorf("kepala rekap GET = %d, want 200", resKepRekap.StatusCode)
	}
	resKepExport, _ := makeRequest(app, "GET", "/api/nilai/export?kelasId="+kelas.ID+"&mapelId="+mtk.ID+"&semester=Ganjil&tahunAjaranId="+year.ID+"&format=pdf", kepToken, nil, "")
	if resKepExport.StatusCode != 200 {
		t.Errorf("kepala export = %d, want 200", resKepExport.StatusCode)
	}

	// --- 10. Active-year guard ---
	archivedYear := TahunAjaran{NamaTahunAjaran: "2020/2021-Archive", TanggalMulai: time.Now(), TanggalSelesai: time.Now().AddDate(1, 0, 0), IsAktif: false}
	s.db.Create(&archivedYear)
	archivedKelas := Kelas{Jenjang: 2, NamaRombel: "ARC", PokjarID: pokjar.ID, TahunAjaranID: archivedYear.ID}
	s.db.Create(&archivedKelas)
	archTema := Tema{KelasID: archivedKelas.ID, MapelID: mtk.ID, TahunAjaranID: archivedYear.ID, Semester: "Ganjil", NamaTema: "Tema ARC", Urutan: 1, JumlahCP: 1, BobotKeterampilan: 60, BobotPengetahuan: 40}
	s.db.Create(&archTema)
	resArchSave, _ := makeRequest(app, "PUT", "/api/tema/"+archTema.ID+"/nilai", token, map[string]interface{}{
		"values": []map[string]interface{}{
			{"pesertaDidikId": s1.ID, "cp": []map[string]interface{}{{"urutanCp": 1, "deskripsiCp": "CP", "nilaiKeterampilan": 50.0}}, "nilaiUm": 60.0},
		},
	}, "")
	if resArchSave.StatusCode != 400 {
		t.Errorf("archived-year save nilai = %d, want 400", resArchSave.StatusCode)
	}

	// --- 11. Tema delete cleans up rekap ---
	resDel, _ := makeRequest(app, "DELETE", "/api/tema/"+tema.ID, token, nil, "")
	if resDel.StatusCode != 204 {
		t.Fatalf("tema delete returned %d, want 204", resDel.StatusCode)
	}
	var rekapAfter int64
	s.db.Model(&RekapNilaiAkhir{}).Where("kelas_id = ? AND mapel_id = ? AND semester = ? AND tahun_ajaran_id = ?", kelas.ID, mtk.ID, "Ganjil", year.ID).Count(&rekapAfter)
	if rekapAfter != 0 {
		t.Errorf("rekap rows after tema delete = %d, want 0 (cleanup)", rekapAfter)
	}
}