package main

import (
	"bytes"
	"encoding/base64"
	"testing"

	"pkbm-lms/backend/internal/studentmcp"
)

func TestStudentMCPCreateWithoutParentPreservesForeignKeys(t *testing.T) {
	f := setupStudentMCP(t, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{39}, 32)), "true", "test")
	// Production PostgreSQL enforces the optional parent relationship. Match
	// that behavior here instead of relying on SQLite's default FK setting.
	if err := f.db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	batch := studentmcp.Batch{Creates: []studentmcp.Create{{Nama: "Peserta Tanpa Relasi Orang Tua", JenisKelamin: "P", NIK: "2222222222222222", NIS: "NO-PARENT-01", NISN: "0000000456", KelasID: f.student.KelasID}}}
	var preview studentmcp.Preview
	f.request(t, "/preview", batch, 200, &preview)
	if preview.PreviewToken == "" || len(preview.Issues) != 0 {
		t.Fatal("verified student could not be previewed")
	}
	var receipt studentmcp.Receipt
	f.request(t, "/apply", studentmcp.ApplyRequest{Changes: batch, PreviewToken: preview.PreviewToken}, 200, &receipt)
	if receipt.Created != 1 || receipt.Updated != 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	var created PesertaDidik
	if err := f.db.Where("nis = ?", "NO-PARENT-01").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	var withoutParent int64
	if err := f.db.Model(&PesertaDidik{}).Where("id = ? AND orang_tua_id IS NULL", created.ID).Count(&withoutParent).Error; err != nil || withoutParent != 1 {
		t.Fatal("missing parent must remain SQL NULL, never a fabricated or invalid parent ID")
	}
	var histories int64
	if err := f.db.Model(&RiwayatKelasPesertaDidik{}).Where("peserta_didik_id = ? AND kelas_id = ?", created.ID, f.student.KelasID).Count(&histories).Error; err != nil || histories != 1 {
		t.Fatal("new student's class history was not created")
	}
	studentMCPAssertRowUnchanged(t, f.db, f.student, f.student.ID)
	var enforced int
	if err := f.db.Raw("PRAGMA foreign_keys").Scan(&enforced).Error; err != nil || enforced != 1 {
		t.Fatal("foreign key enforcement was weakened")
	}
}
