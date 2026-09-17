package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func journalParentToken(t *testing.T, s *Server, app *fiber.App, adminToken string, student *PesertaDidik) string {
	t.Helper()
	parent := OrangTua{NamaBapak: "Orang Tua Jurnal"}
	if err := s.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	student.OrangTuaID = parent.ID
	if err := s.db.Save(student).Error; err != nil {
		t.Fatal(err)
	}
	username := "ortu-jurnal-" + student.ID[:8]
	response, err := makeRequest(app, http.MethodPost, "/api/users", adminToken, map[string]any{
		"username": username, "email": username + "@example.com", "password": "Password123", "role": "orang_tua", "orangTuaId": parent.ID, "isActive": true,
	}, "")
	if err != nil || response.StatusCode != http.StatusCreated {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("buat akun orang tua: status=%v err=%v", response.StatusCode, err)
	}
	response.Body.Close()
	login, err := makeRequest(app, http.MethodPost, "/api/auth/login", "", map[string]string{"login": username, "password": "Password123"}, "")
	if err != nil || login.StatusCode != http.StatusOK {
		if login != nil {
			login.Body.Close()
		}
		t.Fatalf("login orang tua: status=%v err=%v", login.StatusCode, err)
	}
	defer login.Body.Close()
	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(login.Body).Decode(&payload); err != nil || payload.AccessToken == "" {
		t.Fatalf("token orang tua tidak tersedia: %v", err)
	}
	return payload.AccessToken
}

func TestJournalInternalEndpointsRejectParent(t *testing.T) {
	s, app, adminToken, guruToken, kelas, mapel, _, day := setupJournalBatchFixture(t)
	var student PesertaDidik
	if err := s.db.Where("kelas_id = ?", kelas.ID).First(&student).Error; err != nil {
		t.Fatal(err)
	}
	parentToken := journalParentToken(t, s, app.(*fiber.App), adminToken, &student)
	lines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapel.ID, Materi: "Materi aman"}})
	created, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(lines)}}), -1)
	if err != nil || created.StatusCode != http.StatusCreated {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("buat jurnal: status=%v err=%v", created.StatusCode, err)
	}
	created.Body.Close()
	var journal JurnalMengajar
	if err := s.db.Where("kelas_id = ?", kelas.ID).First(&journal).Error; err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"/api/jurnal",
		"/api/jurnal/sheet?kelasId=" + url.QueryEscape(kelas.ID) + "&tanggal=" + wibTimeFormat(day, "2006-01-02"),
		"/api/jurnal/export?kelasId=" + url.QueryEscape(kelas.ID) + "&tanggal=" + wibTimeFormat(day, "2006-01-02") + "&format=pdf",
		"/api/jurnal/" + journal.ID + "/foto",
	}
	for _, path := range paths {
		request := journalBatchRequest(http.MethodGet, path, parentToken, nil)
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusForbidden {
			response.Body.Close()
			t.Fatalf("parent access %s: want 403, got %d", path, response.StatusCode)
		}
		response.Body.Close()
	}
}

func TestPublishedJournalVisibleOnlyToLinkedParent(t *testing.T) {
	s, app, adminToken, guruToken, kelas, mapel, secondMapel, day := setupJournalBatchFixture(t)
	var student PesertaDidik
	if err := s.db.Where("kelas_id = ?", kelas.ID).First(&student).Error; err != nil {
		t.Fatal(err)
	}
	parentToken := journalParentToken(t, s, app.(*fiber.App), adminToken, &student)
	lines, _ := json.Marshal([]journalLineInput{
		{JamKe: 1, MapelID: mapel.ID, Materi: "Bilangan", Tujuan: "Mengenal penjumlahan", Kegiatan: "Menghitung kartu", RingkasanOrangTua: "Anak berlatih penjumlahan sederhana.", StatusPublikasi: statusPublikasiDipublikasikan},
		{JamKe: 2, MapelID: secondMapel.ID, Materi: "Ekosistem", RingkasanOrangTua: "Belum terbit", StatusPublikasi: statusPublikasiDraf},
	})
	created, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(lines)}}), -1)
	if err != nil || created.StatusCode != http.StatusCreated {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("buat jurnal: status=%v err=%v", created.StatusCode, err)
	}
	created.Body.Close()
	response, err := app.Test(journalBatchRequest(http.MethodGet, "/api/orang-tua/anak/"+student.ID+"/jurnal", parentToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ringkasan orang tua: want 200, got %d", response.StatusCode)
	}
	var journals []parentJournalItem
	if err := json.NewDecoder(response.Body).Decode(&journals); err != nil {
		t.Fatal(err)
	}
	if len(journals) != 1 || journals[0].Tujuan != "Mengenal penjumlahan" || journals[0].Ringkasan != "Anak berlatih penjumlahan sederhana." {
		t.Fatalf("jurnal terbit tidak sesuai: %+v", journals)
	}
}

func TestJournalLockAndCancellationProtectHistory(t *testing.T) {
	_, app, adminToken, guruToken, kelas, mapel, _, day := setupJournalBatchFixture(t)
	lines, _ := json.Marshal([]journalLineInput{{JamKe: 1, MapelID: mapel.ID, Materi: "Catatan sejarah"}})
	created, err := app.Test(journalBatchRequest(http.MethodPost, "/api/jurnal/batches", guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "tandaTangan": {validPngSignature}, "lines": {string(lines)}}), -1)
	if err != nil || created.StatusCode != http.StatusCreated {
		if created != nil {
			created.Body.Close()
		}
		t.Fatalf("buat jurnal: status=%v err=%v", created.StatusCode, err)
	}
	var batch JurnalBatch
	if err := json.NewDecoder(created.Body).Decode(&batch); err != nil {
		created.Body.Close()
		t.Fatal(err)
	}
	created.Body.Close()
	adminApp := app.(*fiber.App)
	locked, err := makeRequest(adminApp, http.MethodPost, "/api/jurnal/batches/"+batch.ID+"/kunci", adminToken, nil, "")
	if err != nil || locked.StatusCode != http.StatusOK {
		if locked != nil {
			locked.Body.Close()
		}
		t.Fatalf("kunci jurnal: status=%v err=%v", locked.StatusCode, err)
	}
	locked.Body.Close()
	blocked, err := app.Test(journalBatchRequest(http.MethodPut, "/api/jurnal/batches/"+batch.ID, guruToken, url.Values{"kelasId": {kelas.ID}, "tanggal": {wibTimeFormat(day, "2006-01-02")}, "lines": {string(lines)}}), -1)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.StatusCode != http.StatusLocked {
		blocked.Body.Close()
		t.Fatalf("edit locked journal: want 423, got %d", blocked.StatusCode)
	}
	blocked.Body.Close()
	unlocked, err := makeRequest(adminApp, http.MethodPost, "/api/jurnal/batches/"+batch.ID+"/buka-kunci", adminToken, map[string]string{"alasan": "Koreksi yang disetujui"}, "")
	if err != nil || unlocked.StatusCode != http.StatusOK {
		if unlocked != nil {
			unlocked.Body.Close()
		}
		t.Fatalf("buka kunci: status=%v err=%v", unlocked.StatusCode, err)
	}
	unlocked.Body.Close()
	cancelled, err := makeRequest(adminApp, http.MethodDelete, "/api/jurnal/batches/"+batch.ID, adminToken, map[string]string{"alasan": "Duplikasi catatan"}, "")
	if err != nil || cancelled.StatusCode != http.StatusOK {
		if cancelled != nil {
			cancelled.Body.Close()
		}
		t.Fatalf("batalkan jurnal: status=%v err=%v", cancelled.StatusCode, err)
	}
	cancelled.Body.Close()
}

func TestPortfolioAndFollowUpAreScopedByAssignmentAndParentPublication(t *testing.T) {
	s, app, adminToken, guruToken, kelas, mapel, secondMapel, day := setupJournalBatchFixture(t)
	var student PesertaDidik
	if err := s.db.Where("kelas_id = ?", kelas.ID).First(&student).Error; err != nil {
		t.Fatal(err)
	}
	parentToken := journalParentToken(t, s, app.(*fiber.App), adminToken, &student)

	// This tutor remains assigned only to mapel. A portfolio for secondMapel in
	// the same class must not leak into their staff list.
	var assignment PenugasanGuruMapel
	if err := s.db.Where("kelas_id = ? AND mapel_id = ?", kelas.ID, secondMapel.ID).First(&assignment).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Delete(&assignment).Error; err != nil {
		t.Fatal(err)
	}
	visibleJournal := JurnalMengajar{JamKe: 1, TutorID: assignment.TutorID, KelasID: kelas.ID, MapelID: mapel.ID, Tanggal: day, Materi: "Materi penugasan", Status: "disetujui"}
	hiddenJournal := JurnalMengajar{JamKe: 2, TutorID: assignment.TutorID, KelasID: kelas.ID, MapelID: secondMapel.ID, Tanggal: day, Materi: "Materi bukan penugasan", Status: "disetujui"}
	if err := s.db.Create(&visibleJournal).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&hiddenJournal).Error; err != nil {
		t.Fatal(err)
	}
	journalResponse, err := app.Test(journalBatchRequest(http.MethodGet, "/api/jurnal", guruToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer journalResponse.Body.Close()
	if journalResponse.StatusCode != http.StatusOK {
		t.Fatalf("daftar jurnal tutor: want 200, got %d", journalResponse.StatusCode)
	}
	var tutorJournals []JurnalMengajar
	if err := json.NewDecoder(journalResponse.Body).Decode(&tutorJournals); err != nil {
		t.Fatal(err)
	}
	if len(tutorJournals) != 1 || tutorJournals[0].MapelID != mapel.ID {
		t.Fatalf("jurnal mapel bukan penugasan bocor ke tutor: %+v", tutorJournals)
	}
	createdFollow, err := makeRequest(app.(*fiber.App), http.MethodPost, "/api/tindak-lanjut-belajar", guruToken, map[string]any{
		"pesertaDidikId": student.ID, "mapelId": mapel.ID, "statusKetuntasan": "penguatan", "rencana": "Latihan tambahan",
	}, "")
	if err != nil || createdFollow.StatusCode != http.StatusCreated {
		status := 0
		if createdFollow != nil {
			status = createdFollow.StatusCode
			createdFollow.Body.Close()
		}
		t.Fatalf("tindak lanjut sesuai penugasan tutor: status=%v err=%v", status, err)
	}
	var storedFollow TindakLanjutBelajar
	if err := json.NewDecoder(createdFollow.Body).Decode(&storedFollow); err != nil || storedFollow.MapelID != mapel.ID {
		createdFollow.Body.Close()
		t.Fatalf("mapel tindak lanjut tidak tersimpan: %+v err=%v", storedFollow, err)
	}
	createdFollow.Body.Close()
	published := PortofolioBelajar{PesertaDidikID: student.ID, MapelID: mapel.ID, Judul: "Hasil karya matematika", TipeBukti: "pdf", FilePath: "uploads/portofolio/aman.pdf", FileName: "aman.pdf", StatusPublikasi: statusPublikasiDipublikasikan, DibuatOlehUserID: "seed"}
	hiddenMapel := PortofolioBelajar{PesertaDidikID: student.ID, MapelID: secondMapel.ID, Judul: "Hasil karya IPA", TipeBukti: "pdf", FilePath: "uploads/portofolio/ipa.pdf", FileName: "ipa.pdf", StatusPublikasi: statusPublikasiDipublikasikan, DibuatOlehUserID: "seed"}
	draft := PortofolioBelajar{PesertaDidikID: student.ID, MapelID: mapel.ID, Judul: "Masih draf", TipeBukti: "pdf", FilePath: "uploads/portofolio/draf.pdf", FileName: "draf.pdf", StatusPublikasi: statusPublikasiDraf, DibuatOlehUserID: "seed"}
	for _, row := range []*PortofolioBelajar{&published, &hiddenMapel, &draft} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	followPublished := TindakLanjutBelajar{PesertaDidikID: student.ID, StatusKetuntasan: "remedial", Rencana: "Latihan penjumlahan", RingkasanOrangTua: "Dampingi latihan 10 menit.", StatusPublikasi: statusPublikasiDipublikasikan, DibuatOlehUserID: "seed"}
	followDraft := TindakLanjutBelajar{PesertaDidikID: student.ID, StatusKetuntasan: "penguatan", Rencana: "Latihan membaca", RingkasanOrangTua: "Belum terbit", StatusPublikasi: statusPublikasiDraf, DibuatOlehUserID: "seed"}
	if err := s.db.Create(&followPublished).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&followDraft).Error; err != nil {
		t.Fatal(err)
	}

	staffResponse, err := app.Test(journalBatchRequest(http.MethodGet, "/api/portofolio?kelasId="+url.QueryEscape(kelas.ID), guruToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer staffResponse.Body.Close()
	if staffResponse.StatusCode != http.StatusOK {
		t.Fatalf("daftar portofolio tutor: want 200, got %d", staffResponse.StatusCode)
	}
	var staffItems []PortofolioBelajar
	if err := json.NewDecoder(staffResponse.Body).Decode(&staffItems); err != nil {
		t.Fatal(err)
	}
	if len(staffItems) != 2 {
		t.Fatalf("portofolio tutor harus hanya melihat mapel penugasan: %+v", staffItems)
	}
	for _, item := range staffItems {
		if item.MapelID != mapel.ID {
			t.Fatalf("portofolio mapel lain bocor ke tutor: %+v", item)
		}
	}

	portfolioResponse, err := app.Test(journalBatchRequest(http.MethodGet, "/api/orang-tua/anak/"+student.ID+"/portofolio", parentToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer portfolioResponse.Body.Close()
	if portfolioResponse.StatusCode != http.StatusOK {
		t.Fatalf("portofolio orang tua: want 200, got %d", portfolioResponse.StatusCode)
	}
	var parentPortfolio []parentPortfolioItem
	if err := json.NewDecoder(portfolioResponse.Body).Decode(&parentPortfolio); err != nil {
		t.Fatal(err)
	}
	if len(parentPortfolio) != 2 {
		t.Fatalf("orang tua hanya boleh melihat portofolio terbit: %+v", parentPortfolio)
	}
	for _, item := range parentPortfolio {
		if item.Judul == draft.Judul {
			t.Fatalf("portofolio draf terlihat oleh orang tua: %+v", item)
		}
	}

	followResponse, err := app.Test(journalBatchRequest(http.MethodGet, "/api/orang-tua/anak/"+student.ID+"/tindak-lanjut", parentToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer followResponse.Body.Close()
	if followResponse.StatusCode != http.StatusOK {
		t.Fatalf("tindak lanjut orang tua: want 200, got %d", followResponse.StatusCode)
	}
	var parentFollow []parentFollowUpItem
	if err := json.NewDecoder(followResponse.Body).Decode(&parentFollow); err != nil {
		t.Fatal(err)
	}
	if len(parentFollow) != 1 || parentFollow[0].Ringkasan != followPublished.RingkasanOrangTua {
		t.Fatalf("orang tua hanya boleh melihat tindak lanjut terbit: %+v", parentFollow)
	}

	other := PesertaDidik{Nama: "Anak Lain", NIS: "L-" + day.Format("150405"), KelasID: kelas.ID}
	if err := s.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	forbidden, err := app.Test(journalBatchRequest(http.MethodGet, "/api/orang-tua/anak/"+other.ID+"/portofolio", parentToken, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("orang tua tidak boleh melihat anak lain: want 403, got %d", forbidden.StatusCode)
	}
}
