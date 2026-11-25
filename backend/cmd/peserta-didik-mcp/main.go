package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"pkbm-lms/backend/internal/studentmcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type studentAPI struct {
	base *url.URL
	key  string
	http *http.Client
}

func newStudentAPI(origin, key string) (*studentAPI, error) {
	base, err := url.Parse(origin)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" ||
		(base.EscapedPath() != "" && base.EscapedPath() != "/") {
		return nil, errors.New("LMS_STUDENT_MCP_URL harus berupa origin LMS, tanpa kredensial, path, atau parameter")
	}
	if base.Scheme != "https" && (base.Scheme != "http" || (base.Hostname() != "127.0.0.1" && base.Hostname() != "::1")) {
		return nil, errors.New("gunakan HTTPS untuk LMS, atau alamat loopback untuk pengujian lokal")
	}
	return &studentAPI{base: base, key: key, http: &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

func (a *studentAPI) request(ctx context.Context, method, endpoint string, input, output any) error {
	if !studentmcp.ValidAPIKey(a.key) {
		return errors.New("kunci STUDENT_MCP_API_KEY untuk proses Codex belum tersedia atau formatnya tidak valid")
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil || len(data) > studentmcp.MaxBodyBytes {
			return errors.New("permintaan tidak valid atau melebihi batas ukuran batch")
		}
		body = bytes.NewReader(data)
	}
	endpointURL := a.base.JoinPath(studentmcp.APIPath, endpoint)
	request, err := http.NewRequestWithContext(ctx, method, endpointURL.String(), body)
	if err != nil {
		return errors.New("permintaan ke LMS tidak valid")
	}
	request.Header.Set("Authorization", "Bearer "+a.key)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := a.http.Do(request)
	if err != nil {
		return errors.New("gagal menghubungi API LMS")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return studentAPIError(response.StatusCode)
	}
	const maxResponse = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil || len(data) > maxResponse || json.Unmarshal(data, output) != nil {
		return errors.New("respons API LMS tidak valid")
	}
	return nil
}

func studentAPIError(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("autentikasi API ditolak. Periksa kunci integrasi dan status deploy LMS")
	case http.StatusForbidden:
		return errors.New("akses API ditolak. Periksa HTTPS dan apakah penulisan MCP diaktifkan pada LMS")
	case http.StatusNotFound:
		return errors.New("API peserta didik MCP belum tersedia di LMS tujuan")
	case http.StatusConflict:
		return errors.New("batch ditolak karena pratinjau kedaluwarsa, sudah digunakan, atau data berubah. Periksa data LMS dan buat pratinjau baru")
	case http.StatusUnprocessableEntity:
		return errors.New("API menolak kriteria, ukuran batch, atau token pratinjau. Periksa argumen tool")
	default:
		return fmt.Errorf("API LMS mengembalikan status HTTP %d", status)
	}
}

func newStudentMCPServer(api *studentAPI) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "pkbm-lms-peserta-didik", Version: "1.0.0"}, &mcp.ServerOptions{
		Instructions: "Cocokkan peserta dengan sumber resmi dan ID internal. Untuk batch tulis, jalankan peserta_didik_preview terlebih dahulu. Tampilkan ringkasan nama, NIS, dan NISN yang dapat ditinjau tanpa menampilkan NIK atau tanggal lahir. Panggil peserta_didik_apply hanya setelah pengguna secara eksplisit menyetujui pratinjau yang masih berlaku.",
	})
	read := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{Name: "peserta_didik_kelas_aktif", Description: "Cari ID kelas yang tersedia untuk peserta baru.", Annotations: read},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, studentmcp.Classes, error) {
			var out studentmcp.Classes
			err := api.request(ctx, http.MethodGet, "classes", nil, &out)
			return nil, out, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "peserta_didik_cari", Description: "Cocokkan hingga 100 peserta didik dengan kriteria persis. Periksa hasil ambigu sebelum memilih ID.", Annotations: read},
		func(ctx context.Context, _ *mcp.CallToolRequest, input studentmcp.SearchRequest) (*mcp.CallToolResult, studentmcp.SearchResponse, error) {
			var out studentmcp.SearchResponse
			err := api.request(ctx, http.MethodPost, "search", input, &out)
			return nil, out, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "peserta_didik_preview", Description: "Tinjau penambahan peserta baru dan perubahan nama/NIS/NISN untuk maksimum 100 baris. Pratinjau tidak menyimpan data, menjaga ID serta riwayat terkait, dan melaporkan konflik.", Annotations: read},
		func(ctx context.Context, _ *mcp.CallToolRequest, input studentmcp.Batch) (*mcp.CallToolResult, studentmcp.Preview, error) {
			var out studentmcp.Preview
			err := api.request(ctx, http.MethodPost, "preview", input, &out)
			return nil, out, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "peserta_didik_apply", Description: "Simpan perubahan yang sama persis dengan pratinjau. Gunakan hanya setelah pengguna menyetujui pratinjau yang valid. Batch disimpan secara atomik.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true}},
		func(ctx context.Context, _ *mcp.CallToolRequest, input studentmcp.ApplyRequest) (*mcp.CallToolResult, studentmcp.Receipt, error) {
			var out studentmcp.Receipt
			err := api.request(ctx, http.MethodPost, "apply", input, &out)
			return nil, out, err
		})
	return server
}

func main() {
	api, err := newStudentAPI(os.Getenv("LMS_STUDENT_MCP_URL"), os.Getenv("STUDENT_MCP_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	if err := newStudentMCPServer(api).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal("sesi MCP peserta didik berakhir dengan kesalahan")
	}
}
