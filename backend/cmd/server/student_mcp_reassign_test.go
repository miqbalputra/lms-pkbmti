package main

import (
	"bytes"
	"encoding/base64"
	"testing"

	"pkbm-lms/backend/internal/studentmcp"
)

func TestStudentMCPNISReassignmentIsAtomicAndPreservesHistory(t *testing.T) {
	for _, failAudit := range []bool{false, true} {
		name := "commit"
		if failAudit {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			f := setupStudentMCP(t, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{34}, 32)), "true", "test")
			other := PesertaDidik{Nama: "Peserta Kedua", NIS: "SWAP-02", NISN: "0000000456", NIK: "2222222222222222", KelasID: f.student.KelasID, PokjarID: f.student.PokjarID, Status: "aktif"}
			studentMCPSeed(t, f.db, &other)
			attendance := PresensiDetail{PesertaDidikID: f.student.ID, PresensiID: "presensi-swap", StatusKehadiran: "hadir"}
			score := 91.0
			grade := NilaiUM{PesertaDidikID: other.ID, TemaID: "tema-swap", NilaiUM: &score}
			history := RiwayatKelasPesertaDidik{PesertaDidikID: f.student.ID, KelasID: f.student.KelasID, TahunAjaranID: f.year.ID, Status: "aktif"}
			studentMCPSeed(t, f.db, &attendance, &grade, &history)
			batch := studentmcp.Batch{Updates: []studentmcp.Update{
				{StudentID: f.student.ID, ExpectedName: f.student.Nama, ExpectedClassID: f.student.KelasID, NIS: other.NIS},
				{StudentID: other.ID, ExpectedName: other.Nama, ExpectedClassID: other.KelasID, NIS: f.student.NIS},
			}}
			// A NIS owner outside the reviewed batch must still block the write.
			var blocked studentmcp.Preview
			f.request(t, "/preview", studentmcp.Batch{Updates: batch.Updates[:1]}, 200, &blocked)
			if blocked.PreviewToken != "" || len(blocked.Issues) != 1 {
				t.Fatal("an unreviewed owner was allowed to release its NIS")
			}
			invalid := batch
			invalid.Updates = append([]studentmcp.Update{}, batch.Updates...)
			invalid.Updates[1].ExpectedName = "Identitas Salah"
			blocked = studentmcp.Preview{}
			f.request(t, "/preview", invalid, 200, &blocked)
			if blocked.PreviewToken != "" || len(blocked.Issues) == 0 {
				t.Fatal("an owner with invalid verification was allowed to release its NIS")
			}
			var preview studentmcp.Preview
			f.request(t, "/preview", batch, 200, &preview)
			if preview.PreviewToken == "" || len(preview.Issues) != 0 || len(preview.Changes) != 2 {
				t.Fatalf("reviewed swap was rejected: %+v", preview)
			}
			wantStatus := 200
			if failAudit {
				if err := f.db.Exec(`CREATE TRIGGER student_mcp_swap_failure BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'audit failed'); END`).Error; err != nil {
					t.Fatal(err)
				}
				wantStatus = 500
			}
			f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}, wantStatus, nil)
			var first, second PesertaDidik
			if err := f.db.First(&first, "id = ?", f.student.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.First(&second, "id = ?", other.ID).Error; err != nil {
				t.Fatal(err)
			}
			wantFirst, wantSecond := other.NIS, f.student.NIS
			if failAudit {
				wantFirst, wantSecond = f.student.NIS, other.NIS
			}
			if first.NIS != wantFirst || second.NIS != wantSecond || first.NISN != f.student.NISN || second.NISN != other.NISN {
				t.Fatal("NIS swap or rollback left an incorrect identity")
			}
			studentMCPAssertRowUnchanged(t, f.db, attendance, attendance.ID)
			studentMCPAssertRowUnchanged(t, f.db, grade, grade.ID)
			studentMCPAssertRowUnchanged(t, f.db, history, history.ID)
		})
	}
}

func TestStudentMCPAllowsVerifiedSharedNamesOnlyAcrossClasses(t *testing.T) {
	f := setupStudentMCP(t, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{35}, 32)), "true", "test")
	class := Kelas{Jenjang: 1, NamaRombel: "A", PokjarID: f.student.PokjarID, TahunAjaranID: f.year.ID}
	studentMCPSeed(t, f.db, &class)
	row := studentmcp.Create{Nama: f.student.Nama, JenisKelamin: "L", NIK: "2222222222222222", NIS: "NAME-02", NISN: "0000000456", KelasID: class.ID}
	for _, mode := range []string{"same-class", "missing-official-NISN", "same-NIK", "same-NISN"} {
		in := row
		switch mode {
		case "same-class":
			in.KelasID = f.student.KelasID
		case "missing-official-NISN":
			in.NISN = ""
		case "same-NIK":
			in.NIK = f.student.NIK
		case "same-NISN":
			in.NISN = f.student.NISN
		}
		var preview studentmcp.Preview
		f.request(t, "/preview", studentmcp.Batch{Creates: []studentmcp.Create{in}}, 200, &preview)
		if preview.PreviewToken != "" || len(preview.Issues) != 1 {
			t.Fatalf("unsafe shared name accepted (%s): %+v", mode, preview)
		}
	}
	batch := studentmcp.Batch{Creates: []studentmcp.Create{row}}
	var preview studentmcp.Preview
	f.request(t, "/preview", batch, 200, &preview)
	if preview.PreviewToken == "" || len(preview.Issues) != 0 {
		t.Fatalf("distinct verified student rejected: %+v", preview)
	}
	var receipt studentmcp.Receipt
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}, 200, &receipt)
	if receipt.Created != 1 || receipt.Updated != 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	var original PesertaDidik
	if err := f.db.First(&original, "id = ?", f.student.ID).Error; err != nil || original.NIS != f.student.NIS || original.KelasID != f.student.KelasID {
		t.Fatal("existing student with shared name was modified")
	}
}
