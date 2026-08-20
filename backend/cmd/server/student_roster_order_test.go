package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestStudentRosterOrderKeepsExistingRecordsAttached verifies the Dapodik
// reordering contract end-to-end. The order belongs to the student roster,
// not to individual nilai, presensi, or peminjaman records: changing it must
// only change presentation order while every existing record stays attached to
// its original peserta didik.
func TestStudentRosterOrderKeepsExistingRecordsAttached(t *testing.T) {
	s, app := setupE2EServer(t)
	adminToken, _ := getAdminToken(t, app)

	var pokjar Pokjar
	var tahun TahunAjaran
	if err := s.db.First(&pokjar).Error; err != nil {
		t.Fatalf("load pokjar: %v", err)
	}
	if err := s.db.Where("is_aktif = ?", true).First(&tahun).Error; err != nil {
		t.Fatalf("load active academic year: %v", err)
	}

	tutor := Tutor{Nama: "Wali Urutan", JenisKelamin: "P"}
	if err := s.db.Create(&tutor).Error; err != nil {
		t.Fatalf("create tutor: %v", err)
	}
	kelas := Kelas{Jenjang: 2, NamaRombel: "URUT", PokjarID: pokjar.ID, TahunAjaranID: tahun.ID, WaliKelasID: &tutor.ID}
	if err := s.db.Create(&kelas).Error; err != nil {
		t.Fatalf("create class: %v", err)
	}
	// Deliberately create names in the opposite order from the intended
	// Dapodik order to make the presentation change observable.
	siswaA := PesertaDidik{Nama: "Aisyah", NIS: "URUT-1", NISN: "9100000001", JenisKelamin: "P", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"}
	siswaB := PesertaDidik{Nama: "Zain", NIS: "URUT-2", NISN: "9100000002", JenisKelamin: "L", KelasID: kelas.ID, PokjarID: pokjar.ID, Status: "aktif"}
	if err := s.db.Create(&siswaA).Error; err != nil {
		t.Fatalf("create first student: %v", err)
	}
	if err := s.db.Create(&siswaB).Error; err != nil {
		t.Fatalf("create second student: %v", err)
	}

	mapel := MataPelajaran{NamaMapel: "Matematika", KodeMapel: "MTK-URUT", IsActive: true}
	if err := s.db.Create(&mapel).Error; err != nil {
		t.Fatalf("create subject: %v", err)
	}
	if err := s.db.Create(&PenugasanGuruMapel{TutorID: tutor.ID, KelasID: kelas.ID, MapelID: mapel.ID}).Error; err != nil {
		t.Fatalf("assign tutor subject: %v", err)
	}
	tema := Tema{KelasID: kelas.ID, MapelID: mapel.ID, TahunAjaranID: tahun.ID, Semester: "Ganjil", NamaTema: "Urutan", BobotKeterampilan: 50, BobotPengetahuan: 50}
	if err := s.db.Create(&tema).Error; err != nil {
		t.Fatalf("create tema: %v", err)
	}
	if err := s.db.Create(&CapaianPembelajaran{TemaID: tema.ID, UrutanCP: 1, LabelDefault: "CP 1"}).Error; err != nil {
		t.Fatalf("create capaian: %v", err)
	}
	nilaiA, nilaiB := 71.0, 93.0
	if err := s.db.Create(&NilaiUM{TemaID: tema.ID, PesertaDidikID: siswaA.ID, NilaiUM: &nilaiA}).Error; err != nil {
		t.Fatalf("create first score: %v", err)
	}
	if err := s.db.Create(&NilaiUM{TemaID: tema.ID, PesertaDidikID: siswaB.ID, NilaiUM: &nilaiB}).Error; err != nil {
		t.Fatalf("create second score: %v", err)
	}

	meeting := Presensi{KelasID: kelas.ID, Tanggal: testSaturday(), Semester: "Ganjil", StatusPertemuan: "berlangsung", TandaTangan: validPngSignature, TutorID: &tutor.ID}
	if err := s.db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: siswaA.ID, StatusKehadiran: "Sakit"}).Error; err != nil {
		t.Fatalf("create first attendance detail: %v", err)
	}
	if err := s.db.Create(&PresensiDetail{PresensiID: meeting.ID, PesertaDidikID: siswaB.ID, StatusKehadiran: "Hadir"}).Error; err != nil {
		t.Fatalf("create second attendance detail: %v", err)
	}

	bukuA := Buku{Judul: "Buku A", KodeBuku: "BU-A"}
	bukuB := Buku{Judul: "Buku B", KodeBuku: "BU-B"}
	if err := s.db.Create(&bukuA).Error; err != nil {
		t.Fatalf("create first book: %v", err)
	}
	if err := s.db.Create(&bukuB).Error; err != nil {
		t.Fatalf("create second book: %v", err)
	}
	if err := s.db.Create(&Peminjaman{PesertaDidikID: siswaA.ID, BukuID: bukuA.ID, KelasID: kelas.ID, Semester: "Ganjil", TanggalPinjam: time.Now().Add(-time.Hour), Status: "Dipinjam"}).Error; err != nil {
		t.Fatalf("create first loan: %v", err)
	}
	if err := s.db.Create(&Peminjaman{PesertaDidikID: siswaB.ID, BukuID: bukuB.ID, KelasID: kelas.ID, Semester: "Ganjil", TanggalPinjam: time.Now(), Status: "Dipinjam"}).Error; err != nil {
		t.Fatalf("create second loan: %v", err)
	}

	guruToken := loginRole(t, app, adminToken, "guru-urutan", "guru", &tutor.ID)

	// Admin cannot change the tutor-owned Dapodik order.
	resAdmin, _ := makeRequest(app, http.MethodPut, "/api/kelas/"+kelas.ID+"/peserta-didik-order", adminToken, map[string]interface{}{
		"pesertaDidikIds": []string{siswaB.ID, siswaA.ID},
	}, "")
	if resAdmin.StatusCode != http.StatusForbidden {
		t.Fatalf("admin must not change student order, got %d", resAdmin.StatusCode)
	}

	// A partial roster must be rejected, so it cannot accidentally erase a
	// student's saved position.
	resPartial, _ := makeRequest(app, http.MethodPut, "/api/kelas/"+kelas.ID+"/peserta-didik-order", guruToken, map[string]interface{}{
		"pesertaDidikIds": []string{siswaB.ID},
	}, "")
	if resPartial.StatusCode != http.StatusBadRequest {
		t.Fatalf("partial roster must be rejected, got %d", resPartial.StatusCode)
	}

	resSave, _ := makeRequest(app, http.MethodPut, "/api/kelas/"+kelas.ID+"/peserta-didik-order", guruToken, map[string]interface{}{
		"pesertaDidikIds": []string{siswaB.ID, siswaA.ID},
	}, "")
	if resSave.StatusCode != http.StatusOK {
		t.Fatalf("save roster order returned %d", resSave.StatusCode)
	}

	resStudents, _ := makeRequest(app, http.MethodGet, "/api/peserta-didik?kelasId="+kelas.ID, guruToken, nil, "")
	if resStudents.StatusCode != http.StatusOK {
		t.Fatalf("student list returned %d", resStudents.StatusCode)
	}
	var students []PesertaDidik
	if err := json.NewDecoder(resStudents.Body).Decode(&students); err != nil {
		t.Fatalf("decode student list: %v", err)
	}
	if len(students) != 2 || students[0].ID != siswaB.ID || students[1].ID != siswaA.ID {
		t.Fatalf("student list must use saved Dapodik order, got %+v", students)
	}

	resGrid, _ := makeRequest(app, http.MethodGet, "/api/tema/"+tema.ID+"/grid", guruToken, nil, "")
	if resGrid.StatusCode != http.StatusOK {
		t.Fatalf("nilai grid returned %d", resGrid.StatusCode)
	}
	var grid struct {
		Students []struct {
			PesertaDidik PesertaDidik `json:"pesertaDidik"`
			NilaiUM      *float64     `json:"nilaiUm"`
		} `json:"students"`
	}
	if err := json.NewDecoder(resGrid.Body).Decode(&grid); err != nil {
		t.Fatalf("decode nilai grid: %v", err)
	}
	if len(grid.Students) != 2 || grid.Students[0].PesertaDidik.ID != siswaB.ID || grid.Students[0].NilaiUM == nil || *grid.Students[0].NilaiUM != nilaiB || grid.Students[1].PesertaDidik.ID != siswaA.ID || grid.Students[1].NilaiUM == nil || *grid.Students[1].NilaiUM != nilaiA {
		t.Fatalf("nilai must remain attached while grid follows saved order, got %+v", grid.Students)
	}

	resPresensi, _ := makeRequest(app, http.MethodGet, "/api/presensi/"+meeting.ID, guruToken, nil, "")
	if resPresensi.StatusCode != http.StatusOK {
		t.Fatalf("attendance detail returned %d", resPresensi.StatusCode)
	}
	var savedMeeting Presensi
	if err := json.NewDecoder(resPresensi.Body).Decode(&savedMeeting); err != nil {
		t.Fatalf("decode attendance detail: %v", err)
	}
	if len(savedMeeting.Details) != 2 || savedMeeting.Details[0].PesertaDidikID != siswaB.ID || savedMeeting.Details[0].StatusKehadiran != "Hadir" || savedMeeting.Details[1].PesertaDidikID != siswaA.ID || savedMeeting.Details[1].StatusKehadiran != "Sakit" {
		t.Fatalf("attendance data must remain attached while details follow saved order, got %+v", savedMeeting.Details)
	}

	resLoans, _ := makeRequest(app, http.MethodGet, "/api/peminjaman-buku/aktif?kelasId="+kelas.ID, guruToken, nil, "")
	if resLoans.StatusCode != http.StatusOK {
		t.Fatalf("active loan list returned %d", resLoans.StatusCode)
	}
	var loans []Peminjaman
	if err := json.NewDecoder(resLoans.Body).Decode(&loans); err != nil {
		t.Fatalf("decode loan list: %v", err)
	}
	if len(loans) != 2 || loans[0].PesertaDidikID != siswaB.ID || loans[0].BukuID != bukuB.ID || loans[1].PesertaDidikID != siswaA.ID || loans[1].BukuID != bukuA.ID {
		t.Fatalf("loans must remain attached while list follows saved order, got %+v", loans)
	}
}
