package main

import (
	"encoding/json"
	"testing"
)

// relasiResponse mirrors the relasiOrtu shape returned by GET /orang-tua/relasi.
type relasiResponse struct {
	OrangTua  OrangTua `json:"orangTua"`
	Children  []struct {
		ID           string `json:"id"`
		Nama         string `json:"nama"`
		NIK          string `json:"nik"`
		KelasLabel   string `json:"kelasLabel"`
	} `json:"children"`
	AnakCount int `json:"anakCount"`
}

func TestE2E_OrangTuaRelasi(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	s.db.First(&pokjar)
	var year TahunAjaran
	s.db.Where("is_aktif = ?", true).First(&year)

	// 1. Create an Orang Tua with NIKAyah + NIKIbu (opsional).
	resOrtu, _ := makeRequest(app, "POST", "/api/orang-tua", token, map[string]interface{}{
		"namaBapak": "Budi", "namaIbu": "Siti", "nikAyah": "3201000101010001", "nikIbu": "3201000202020002",
	}, "")
	if resOrtu.StatusCode != 201 {
		t.Fatalf("orang tua create returned %d", resOrtu.StatusCode)
	}
	var ortu OrangTua
	json.NewDecoder(resOrtu.Body).Decode(&ortu)
	if ortu.NIKAyah != "3201000101010001" || ortu.NIKIbu != "3201000202020002" {
		t.Errorf("orang tua NIK not persisted, got ayah=%q ibu=%q", ortu.NIKAyah, ortu.NIKIbu)
	}

	// Kelas for the students (direct DB create bypasses handler; not under test here).
	class := Kelas{Jenjang: 1, NamaRombel: "REL", PokjarID: pokjar.ID, TahunAjaranID: year.ID}
	if err := s.db.Create(&class).Error; err != nil {
		t.Fatalf("kelas create failed: %v", err)
	}

	// 2. Create a student WITHOUT nik → 400.
	resNoNIK, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama": "Tanpa NIK", "jenisKelamin": "L", "nis": "NIS-R-0", "nisn": "NISN-R-0",
		"kelasId": class.ID, "pokjarId": pokjar.ID, "orangTuaId": ortu.ID,
	}, "")
	if resNoNIK.StatusCode != 400 {
		t.Fatalf("expected 400 for missing nik, got %d", resNoNIK.StatusCode)
	}

	// 3. Create first child with nik → 201.
	resS1, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama": "Anak Pertama", "jenisKelamin": "L", "nis": "NIS-R-1", "nisn": "NISN-R-1", "nik": "NIK-R-1",
		"kelasId": class.ID, "pokjarId": pokjar.ID, "orangTuaId": ortu.ID,
	}, "")
	if resS1.StatusCode != 201 {
		t.Fatalf("first siswa create returned %d", resS1.StatusCode)
	}
	var s1 PesertaDidik
	json.NewDecoder(resS1.Body).Decode(&s1)

	// 4. Create second child (saudara) with same orangTuaId → 201.
	resS2, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama": "Anak Kedua", "jenisKelamin": "P", "nis": "NIS-R-2", "nisn": "NISN-R-2", "nik": "NIK-R-2",
		"kelasId": class.ID, "pokjarId": pokjar.ID, "orangTuaId": ortu.ID,
	}, "")
	if resS2.StatusCode != 201 {
		t.Fatalf("second siswa create returned %d", resS2.StatusCode)
	}

	// 5. Duplicate NIK → 400.
	resDup, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama": "Dup NIK", "jenisKelamin": "L", "nis": "NIS-R-3", "nisn": "NISN-R-3", "nik": "NIK-R-1",
		"kelasId": class.ID, "pokjarId": pokjar.ID, "orangTuaId": ortu.ID,
	}, "")
	if resDup.StatusCode != 400 {
		t.Fatalf("expected 400 for duplicate nik, got %d", resDup.StatusCode)
	}

	// 6. GET /orang-tua/relasi → parent has anakCount==2 and 2 children.
	resRel, _ := makeRequest(app, "GET", "/api/orang-tua/relasi", token, nil, "")
	if resRel.StatusCode != 200 {
		t.Fatalf("relasi GET returned %d", resRel.StatusCode)
	}
	var groups []relasiResponse
	json.NewDecoder(resRel.Body).Decode(&groups)
	var found *relasiResponse
	for i := range groups {
		if groups[i].OrangTua.ID == ortu.ID {
			found = &groups[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("relasi: orang tua %s not found in response", ortu.ID)
	}
	if found.AnakCount != 2 {
		t.Errorf("relasi: anakCount = %d, want 2", found.AnakCount)
	}
	if len(found.Children) != 2 {
		t.Errorf("relasi: children len = %d, want 2", len(found.Children))
	}
	for _, ch := range found.Children {
		if ch.NIK == "" {
			t.Errorf("relasi: child %s has empty NIK", ch.Nama)
		}
		if ch.KelasLabel != "Kelas 1REL" {
			t.Errorf("relasi: child %s kelasLabel = %q, want %q", ch.Nama, ch.KelasLabel, "Kelas 1REL")
		}
	}

	// 7. Search by parent name (ibu=Siti) → matches; nonsense q → empty.
	resQ, _ := makeRequest(app, "GET", "/api/orang-tua/relasi?q=Siti", token, nil, "")
	var qGroups []relasiResponse
	json.NewDecoder(resQ.Body).Decode(&qGroups)
	if len(qGroups) == 0 {
		t.Errorf("relasi?q=Siti: expected at least one group, got 0")
	}
	resQ2, _ := makeRequest(app, "GET", "/api/orang-tua/relasi?q=zzznope", token, nil, "")
	var qGroups2 []relasiResponse
	json.NewDecoder(resQ2.Body).Decode(&qGroups2)
	if len(qGroups2) != 0 {
		t.Errorf("relasi?q=zzznope: expected 0 groups, got %d", len(qGroups2))
	}

	// 8. PUT /peserta-didik/:id with empty nik → 400.
	resUpd, _ := makeRequest(app, "PUT", "/api/peserta-didik/"+s1.ID, token, map[string]interface{}{
		"nama": s1.Nama, "jenisKelamin": s1.JenisKelamin, "nis": s1.NIS, "nisn": s1.NISN, "nik": "",
		"kelasId": s1.KelasID, "pokjarId": s1.PokjarID, "orangTuaId": s1.OrangTuaID, "status": s1.Status,
	}, "")
	if resUpd.StatusCode != 400 {
		t.Fatalf("expected 400 for update with empty nik, got %d", resUpd.StatusCode)
	}

	// 9. Student without orangTuaId → virtual orphan group in relasi.
	resS3, _ := makeRequest(app, "POST", "/api/peserta-didik", token, map[string]interface{}{
		"nama": "Anak Yatim Data", "jenisKelamin": "L", "nis": "NIS-R-9", "nisn": "NISN-R-9", "nik": "NIK-R-9",
		"kelasId": class.ID, "pokjarId": pokjar.ID,
	}, "")
	if resS3.StatusCode != 201 {
		t.Fatalf("orphan siswa create returned %d", resS3.StatusCode)
	}
	resRel2, _ := makeRequest(app, "GET", "/api/orang-tua/relasi", token, nil, "")
	var groups2 []relasiResponse
	json.NewDecoder(resRel2.Body).Decode(&groups2)
	var orphan *relasiResponse
	for i := range groups2 {
		if groups2[i].OrangTua.ID == "" {
			orphan = &groups2[i]
			break
		}
	}
	if orphan == nil {
		t.Fatalf("relasi: virtual orphan group not found")
	}
	if orphan.AnakCount < 1 {
		t.Errorf("relasi: orphan anakCount = %d, want >=1", orphan.AnakCount)
	}
}