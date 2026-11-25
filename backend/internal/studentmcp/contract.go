package studentmcp

import "encoding/base64"

const (
	APIPrefix    = "/integrations/student-mcp/v1"
	APIPath      = "/api" + APIPrefix
	MaxRows      = 100
	MaxBodyBytes = 80 * 1024
)

func ValidAPIKey(key string) bool {
	if len(key) != base64.RawURLEncoding.EncodedLen(32) {
		return false
	}
	_, err := base64.RawURLEncoding.Strict().DecodeString(key)
	return err == nil
}

type Student struct {
	ID      string `json:"id"`
	Nama    string `json:"nama"`
	KelasID string `json:"kelasId"`
	NIS     string `json:"nis"`
	NISN    string `json:"nisn"`
	Status  string `json:"status"`
}

type Class struct {
	ID          string `json:"id"`
	Jenjang     int    `json:"jenjang"`
	NamaRombel  string `json:"namaRombel"`
	Pokjar      string `json:"pokjar"`
	TahunAjaran string `json:"tahunAjaran"`
}

type Classes struct {
	Kelas []Class `json:"kelas"`
}

type Query struct {
	Nama    string `json:"nama,omitempty" jsonschema:"Nama lengkap persis, tanpa membedakan huruf besar dan kecil"`
	KelasID string `json:"kelasId,omitempty" jsonschema:"ID kelas dalam LMS untuk mempersempit pencarian"`
	NIS     string `json:"nis,omitempty" jsonschema:"NIS lama persis sebagai teks"`
	NISN    string `json:"nisn,omitempty" jsonschema:"NISN resmi persis sebagai teks, termasuk nol di depan"`
	NIK     string `json:"nik,omitempty" jsonschema:"NIK anak persis untuk verifikasi identitas, tidak dikembalikan dalam hasil"`
}

type SearchRequest struct {
	Queries []Query `json:"queries" jsonschema:"1 sampai 100 pencarian. Semua kriteria dalam satu pencarian harus cocok."`
}

type SearchMatch struct {
	Row       int       `json:"row"`
	Students  []Student `json:"students"`
	Ambiguous bool      `json:"ambiguous"`
}

type SearchResponse struct {
	Matches []SearchMatch `json:"matches"`
}

type Update struct {
	StudentID       string `json:"studentId" jsonschema:"ID internal hasil pencarian LMS"`
	ExpectedName    string `json:"expectedName" jsonschema:"Nama lengkap pada catatan yang dipilih"`
	ExpectedClassID string `json:"expectedClassId" jsonschema:"ID kelas pada catatan yang dipilih"`
	ExpectedNIK     string `json:"expectedNik,omitempty" jsonschema:"NIK terverifikasi dari sumber, wajib bila ada nama sama dalam satu kelas"`
	Nama            string `json:"nama,omitempty" jsonschema:"Nama resmi baru. Kosongkan bila tidak diubah."`
	NIS             string `json:"nis,omitempty" jsonschema:"NIS baru sebagai teks. Abaikan kolom ini bila tidak berubah."`
	NISN            string `json:"nisn,omitempty" jsonschema:"NISN resmi baru sebagai teks 10 digit. Abaikan kolom ini bila tidak berubah."`
}

type Create struct {
	Nama         string `json:"nama"`
	JenisKelamin string `json:"jenisKelamin" jsonschema:"L atau P"`
	NIK          string `json:"nik" jsonschema:"NIK anak sebagai teks 16 digit"`
	NIS          string `json:"nis" jsonschema:"NIS sekolah sebagai teks"`
	NISN         string `json:"nisn,omitempty" jsonschema:"NISN resmi sebagai teks 10 digit. Kosong memakai nomor sementara LMS."`
	TanggalLahir string `json:"tanggalLahir,omitempty" jsonschema:"Tanggal lahir dalam format YYYY-MM-DD"`
	KelasID      string `json:"kelasId" jsonschema:"ID kelas aktif dari LMS"`
}

type Batch struct {
	Updates []Update `json:"updates,omitempty" jsonschema:"Pembaruan nama, NIS, dan/atau NISN untuk ID internal yang sudah dikonfirmasi"`
	Creates []Create `json:"creates,omitempty" jsonschema:"Peserta baru yang sudah diverifikasi. Maksimum 100 baris gabungan dengan updates."`
}

type Issue struct {
	Section string `json:"section"`
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type Change struct {
	Row    int     `json:"row"`
	Before Student `json:"before"`
	After  Student `json:"after"`
}

type NewStudent struct {
	Row     int     `json:"row"`
	Student Student `json:"student"`
	Class   Class   `json:"class"`
}

type Preview struct {
	Changes      []Change     `json:"changes"`
	NewStudents  []NewStudent `json:"newStudents"`
	Issues       []Issue      `json:"issues"`
	Unchanged    int          `json:"unchanged"`
	WriteEnabled bool         `json:"writeEnabled"`
	PreviewToken string       `json:"previewToken,omitempty"`
	ExpiresAt    string       `json:"expiresAt,omitempty"`
}

type ApplyRequest struct {
	Changes      Batch  `json:"changes" jsonschema:"Kirim ulang batch yang sama persis dengan pratinjau yang disetujui"`
	PreviewToken string `json:"previewToken" jsonschema:"Token pratinjau yang masih aktif untuk batch ini"`
}

type Receipt struct {
	BatchID   string `json:"batchId"`
	Created   int    `json:"created"`
	Updated   int    `json:"updated"`
	Unchanged int    `json:"unchanged"`
}
