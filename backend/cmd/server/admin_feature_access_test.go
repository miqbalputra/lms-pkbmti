package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func adminFormRequest(method, appURL string, token string, values url.Values) *http.Request {
	req := httptest.NewRequest(method, appURL, bytes.NewBufferString(values.Encode()))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// TestAdminCanCreateTeachingContentWithoutTutorProfile guards a previously
// shared regression: the UI lets admins create teaching content, while several
// handlers rejected the administrator because the account is not a tutor.
func TestAdminCanCreateTeachingContentWithoutTutorProfile(t *testing.T) {
	s, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)

	var pokjar Pokjar
	var year TahunAjaran
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Where("is_aktif = ?", true).First(&year).Error; err != nil {
		t.Fatal(err)
	}
	tutor := Tutor{Nama: "Tutor Pemilik Konten", JenisKelamin: "P"}
	mapel := MataPelajaran{NamaMapel: "Mapel Konten", KodeMapel: "MK"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&mapel).Error; err != nil {
		t.Fatal(err)
	}
	kelas := Kelas{Jenjang: 1, NamaRombel: "Konten", PokjarID: pokjar.ID, TahunAjaranID: year.ID, WaliKelasID: &tutor.ID}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatal(err)
	}

	postForm := func(path string, values url.Values) *http.Response {
		res, err := app.Test(adminFormRequest(http.MethodPost, path, token, values))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		return res
	}

	resJurnal := postForm("/api/jurnal", url.Values{
		"tutorId": {tutor.ID}, "mapelId": {mapel.ID}, "kelasId": {kelas.ID}, "tanggal": {"2026-08-22"}, "materi": {"Pecahan"}, "kegiatan": {"Diskusi"},
	})
	if resJurnal.StatusCode != http.StatusCreated {
		resJurnal.Body.Close()
		t.Fatalf("admin create jurnal: want 201, got %d", resJurnal.StatusCode)
	}
	var jurnal JurnalMengajar
	if err := json.NewDecoder(resJurnal.Body).Decode(&jurnal); err != nil {
		resJurnal.Body.Close()
		t.Fatal(err)
	}
	resJurnal.Body.Close()
	if jurnal.TutorID != tutor.ID {
		t.Fatalf("jurnal owner = %q, want selected tutor %q", jurnal.TutorID, tutor.ID)
	}

	resJurnalUpdate, err := app.Test(adminFormRequest(http.MethodPut, "/api/jurnal/"+jurnal.ID, token, url.Values{"materi": {"Pecahan Lanjutan"}}))
	if err != nil {
		t.Fatal(err)
	}
	if resJurnalUpdate.StatusCode != http.StatusOK {
		resJurnalUpdate.Body.Close()
		t.Fatalf("admin update jurnal: want 200, got %d", resJurnalUpdate.StatusCode)
	}
	resJurnalUpdate.Body.Close()

	resTugas := postForm("/api/tugas", url.Values{
		"mapelId": {mapel.ID}, "kelasId": {kelas.ID}, "judul": {"Latihan pecahan"}, "deadline": {"2026-08-30"},
	})
	if resTugas.StatusCode != http.StatusCreated {
		resTugas.Body.Close()
		t.Fatalf("admin create tugas: want 201, got %d", resTugas.StatusCode)
	}
	resTugas.Body.Close()

	resMateri := postForm("/api/materi", url.Values{
		"mapelId": {mapel.ID}, "kelasId": {kelas.ID}, "judul": {"Video pecahan"}, "linkUrl": {"https://example.test/pecahan"},
	})
	if resMateri.StatusCode != http.StatusCreated {
		resMateri.Body.Close()
		t.Fatalf("admin create materi: want 201, got %d", resMateri.StatusCode)
	}
	resMateri.Body.Close()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	resVirtual, err := makeRequest(app, http.MethodPost, "/api/kelas-virtual", token, map[string]any{
		"mapelId": mapel.ID, "kelasId": kelas.ID, "judul": "Kelas daring", "linkMeeting": "https://meet.example.test/kelas", "waktuMulai": now, "waktuSelesai": now.Add(time.Hour),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resVirtual.StatusCode != http.StatusCreated {
		resVirtual.Body.Close()
		t.Fatalf("admin create kelas virtual: want 201, got %d", resVirtual.StatusCode)
	}
	resVirtual.Body.Close()

	resUjian, err := makeRequest(app, http.MethodPost, "/api/ujian", token, map[string]any{
		"mapelId": mapel.ID, "kelasId": kelas.ID, "judul": "Ujian pecahan", "waktuMulai": now, "waktuSelesai": now.Add(time.Hour), "durasiMenit": 60,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resUjian.StatusCode != http.StatusCreated {
		resUjian.Body.Close()
		t.Fatalf("admin create ujian: want 201, got %d", resUjian.StatusCode)
	}
	resUjian.Body.Close()

	var rppBody bytes.Buffer
	writer := multipart.NewWriter(&rppBody)
	for key, value := range map[string]string{
		"tutorId": tutor.ID, "mapelId": mapel.ID, "jenjang": "1", "tahunAjaranId": year.ID, "judul": "RPP Pecahan",
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("file", "rpp.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("%PDF-1.4\nminimal")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reqRPP := httptest.NewRequest(http.MethodPost, "/api/rpp", &rppBody)
	reqRPP.Header.Set("Authorization", "Bearer "+token)
	reqRPP.Header.Set("Content-Type", writer.FormDataContentType())
	resRPP, err := app.Test(reqRPP)
	if err != nil {
		t.Fatal(err)
	}
	if resRPP.StatusCode != http.StatusCreated {
		resRPP.Body.Close()
		t.Fatalf("admin create RPP: want 201, got %d", resRPP.StatusCode)
	}
	var rpp RPP
	if err := json.NewDecoder(resRPP.Body).Decode(&rpp); err != nil {
		resRPP.Body.Close()
		t.Fatal(err)
	}
	resRPP.Body.Close()
	t.Cleanup(func() { removeUpload(rpp.FilePath) })
}

func TestAdminJournalRequiresSelectedTutor(t *testing.T) {
	_, app := setupE2EServer(t)
	token, _ := getAdminToken(t, app)
	res, err := app.Test(adminFormRequest(http.MethodPost, "/api/jurnal", token, url.Values{
		"mapelId": {"missing-mapel"}, "kelasId": {"missing-kelas"}, "tanggal": {"2026-08-22"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("admin journal without tutor: want 400, got %d", res.StatusCode)
	}
}
