package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pkbm-lms/backend/internal/studentmcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStudentMCPToolsUseScopedAPI(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{25}, 32))
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key || r.URL.RawQuery != "" {
			t.Error("API did not receive a scoped authorization header")
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET " + studentmcp.APIPath + "/classes":
			_ = json.NewEncoder(w).Encode(studentmcp.Classes{Kelas: []studentmcp.Class{{ID: "uji", NamaRombel: "A"}}})
		case "POST " + studentmcp.APIPath + "/preview":
			var batch studentmcp.Batch
			if err := json.NewDecoder(r.Body).Decode(&batch); err != nil || len(batch.Creates) != 1 || batch.Creates[0].NIS != "00012" || batch.Creates[0].NISN != "0010000012" ||
				len(batch.Updates) != 1 || batch.Updates[0].Nama != "Nama Resmi" {
				t.Error("preview did not preserve the name and identifiers as strings")
				http.Error(w, "bad request", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(studentmcp.Preview{Changes: []studentmcp.Change{}, NewStudents: []studentmcp.NewStudent{}, Issues: []studentmcp.Issue{}, PreviewToken: "sample"})
		default:
			t.Error("unexpected API path or method")
			http.Error(w, "not found", 404)
		}
	}))
	defer apiServer.Close()
	api, err := newStudentAPI(apiServer.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server, err := newStudentMCPServer(api).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 4 {
		t.Fatalf("unexpected MCP tool list: %+v, %v", tools, err)
	}
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "peserta_didik_kelas_aktif", Arguments: struct{}{}})
	if err != nil || result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, `"id":"uji"`) {
		t.Fatalf("classes tool did not return the response: %+v, %v", result, err)
	}
	batch := studentmcp.Batch{
		Updates: []studentmcp.Update{{StudentID: "siswa-uji", ExpectedName: "Nama Lama", ExpectedClassID: "kelas-uji", Nama: "Nama Resmi"}},
		Creates: []studentmcp.Create{{Nama: "Peserta Uji", JenisKelamin: "P", NIK: "3333333333333333", NIS: "00012", NISN: "0010000012", KelasID: "uji"}},
	}
	result, err = client.CallTool(ctx, &mcp.CallToolParams{Name: "peserta_didik_preview", Arguments: batch})
	if err != nil || result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, `"previewToken":"sample"`) {
		t.Fatalf("preview tool did not return the response: %+v, %v", result, err)
	}
}

func TestStudentAPIRejectsInsecureOriginAndRedirect(t *testing.T) {
	for _, origin := range []string{"http://lms.example", "http://localhost:8080", "https://user:password@lms.example", "https://lms.example/api", "https://lms.example/?key=value"} {
		if _, err := newStudentAPI(origin, ""); err == nil {
			t.Errorf("accepted unsupported origin %q", origin)
		}
	}
	if _, err := newStudentAPI("https://lms.example", ""); err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{26}, 32))
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("client followed a redirect")
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	api, err := newStudentAPI(origin.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	var output studentmcp.Classes
	if err := api.request(context.Background(), http.MethodGet, "classes", nil, &output); err == nil {
		t.Fatal("client treated a redirect as success")
	}
}
