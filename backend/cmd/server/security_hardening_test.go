package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestSafeUploadPathStaysInsideUploads(t *testing.T) {
	valid := []string{"uploads/file.pdf", "uploads/nested/image.jpg"}
	for _, path := range valid {
		if clean, ok := safeUploadPath(path); !ok || clean == "" {
			t.Errorf("safeUploadPath(%q) rejected a valid path: clean=%q ok=%v", path, clean, ok)
		}
	}

	invalid := []string{"", "uploads", "../secret", "uploads/../../secret", "/absolute/secret"}
	for _, path := range invalid {
		if clean, ok := safeUploadPath(path); ok {
			t.Errorf("safeUploadPath(%q) accepted traversal path %q", path, clean)
		}
	}
	customRoot := t.TempDir()
	t.Setenv("UPLOADS_DIR", customRoot)
	clean, ok := safeUploadPath("uploads/private/file.pdf")
	if !ok || clean != filepath.Join(customRoot, "private", "file.pdf") {
		t.Fatalf("safeUploadPath did not honor UPLOADS_DIR: clean=%q ok=%v", clean, ok)
	}
}

func TestGenericCRUDDoesNotAllowBaseMutation(t *testing.T) {
	db := isolatedTestDB(t, "generic-crud-base")
	if err := db.AutoMigrate(&Program{}, &AuditLog{}); err != nil {
		t.Fatal(err)
	}
	originalCreated := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	row := Program{Kode: "A", Nama: "Program Lama"}
	row.CreatedAt = originalCreated
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{db: db}
	app := fiber.New()
	app.Put("/program/:id", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return update[Program](s, c, "program")
	})
	body := `{"id":"attacker-id","createdAt":"2030-01-01T00:00:00Z","updatedAt":"2030-01-01T00:00:00Z","kode":"A2","nama":"Program Baru"}`
	req := httptest.NewRequest("PUT", "/program/"+row.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var updated Program
	if err := db.First(&updated, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.ID != row.ID || updated.CreatedAt.UTC() != originalCreated {
		t.Fatalf("generic update mutated immutable base fields: id=%q createdAt=%v", updated.ID, updated.CreatedAt)
	}
	var attacker Program
	if err := db.First(&attacker, "id = ?", "attacker-id").Error; err == nil {
		t.Fatal("generic update created an attacker-controlled record")
	}
}

func TestGenericCreateGeneratesServerOwnedID(t *testing.T) {
	db := isolatedTestDB(t, "generic-crud-create-base")
	if err := db.AutoMigrate(&Program{}); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	app := fiber.New()
	app.Post("/program", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return create[Program](s, c, "program")
	})
	req := httptest.NewRequest("POST", "/program", strings.NewReader(`{"id":"attacker-id","kode":"B","nama":"Program B"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	var created Program
	if err := db.Where("kode = ?", "B").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ID == "attacker-id" {
		t.Fatalf("generic create accepted a client-controlled id: %q", created.ID)
	}
}

func TestScheduleUpdateCannotRewriteSingletonIdentity(t *testing.T) {
	db := isolatedTestDB(t, "schedule-singleton-base")
	if err := db.AutoMigrate(&PengaturanJadwal{}, &AuditLog{}); err != nil {
		t.Fatal(err)
	}
	row := PengaturanJadwal{HariDefault: "Senin", JamGenerate: "07:00", ZonaWaktu: "Asia/Jakarta"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	app := fiber.New()
	app.Put("/settings/jadwal", func(c *fiber.Ctx) error {
		c.Locals("userID", "test-admin")
		return s.putJadwal(c)
	})
	req := httptest.NewRequest("PUT", "/settings/jadwal", strings.NewReader(`{"id":"attacker-id","createdAt":"2030-01-01T00:00:00Z","hariDefault":"Selasa","jamGenerate":"08:00","zonaWaktu":"Asia/Jakarta"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var updated PengaturanJadwal
	if err := db.First(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if updated.ID != row.ID || updated.HariDefault != "Selasa" || updated.JamGenerate != "08:00" {
		t.Fatalf("schedule update changed unexpected fields: %+v", updated)
	}
	var attacker PengaturanJadwal
	if err := db.First(&attacker, "id = ?", "attacker-id").Error; err == nil {
		t.Fatal("schedule update created an attacker-controlled record")
	}
}

func TestNormalizeAppEnvFailsClosedForUnknownValues(t *testing.T) {
	for _, raw := range []string{"production", "prod"} {
		got, err := normalizeAppEnv(raw)
		if err != nil || got != "production" {
			t.Fatalf("normalizeAppEnv(%q) = %q, %v", raw, got, err)
		}
	}
	if _, err := normalizeAppEnv("staging-without-hardening"); err == nil {
		t.Fatal("unknown APP_ENV must not silently run with development defaults")
	}
}

func TestExternalLinksAllowOnlyHTTPAndHTTPS(t *testing.T) {
	for _, raw := range []string{"https://meet.example.test/room", "http://localhost:8080/material"} {
		if err := validateExternalHTTPURL(raw, "link"); err != nil {
			t.Fatalf("valid URL %q rejected: %v", raw, err)
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "data:text/html,<script>alert(1)</script>", "//relative.example/path", "https://user:pass@example.test/room"} {
		if err := validateExternalHTTPURL(raw, "link"); err == nil {
			t.Fatalf("unsafe URL %q accepted", raw)
		}
	}
	production := &Server{cfg: Config{Env: "production"}}
	if err := production.validateExternalHTTPURL("http://insecure.example.test/room", "link"); err == nil {
		t.Fatal("production must reject insecure external links")
	}
	page := renderMateriShareHTML(&Materi{Judul: "Materi", LinkURL: "javascript:alert(1)"})
	if strings.Contains(page, "javascript:") {
		t.Fatal("shared material page rendered an unsafe link scheme")
	}
}

func TestProductionBackupKeyCannotBeSuppliedInURL(t *testing.T) {
	t.Setenv("BACKUP_API_KEY", "production-backup-key-with-32-chars!!")
	s := &Server{cfg: Config{Env: "production"}}
	app := fiber.New()
	app.Get("/backup", s.backupReadAuth, func(c *fiber.Ctx) error { return c.SendStatus(204) })

	queryReq := httptest.NewRequest("GET", "/backup?key=production-backup-key-with-32-chars!!", nil)
	queryRes, err := app.Test(queryReq)
	if err != nil {
		t.Fatal(err)
	}
	queryRes.Body.Close()
	if queryRes.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("production query-string backup key should be rejected, got %d", queryRes.StatusCode)
	}

	headerReq := httptest.NewRequest("GET", "/backup", nil)
	headerReq.Header.Set("X-Backup-Key", "production-backup-key-with-32-chars!!")
	headerRes, err := app.Test(headerReq)
	if err != nil {
		t.Fatal(err)
	}
	headerRes.Body.Close()
	if headerRes.StatusCode != http.StatusNoContent {
		t.Fatalf("production header backup key should be accepted, got %d", headerRes.StatusCode)
	}
}

func TestRequestBodyLimitAllowlistProtectsOrdinaryRoutes(t *testing.T) {
	if got := requestBodyLimitForPath("/api/presensi"); got != normalRequestBodyLimit {
		t.Fatalf("ordinary route body limit = %d, want %d", got, normalRequestBodyLimit)
	}
	if got := requestBodyLimitForPath("/api/backup/restore"); got <= normalRequestBodyLimit {
		t.Fatalf("restore route did not receive a larger allowlisted limit: %d", got)
	}
	if got := requestBodyLimitForPath("/api/identitas-siswa/zip"); got <= normalRequestBodyLimit {
		t.Fatalf("identity ZIP route did not receive a larger allowlisted limit: %d", got)
	}
}

func TestRequestIDRejectsLogInjectionAndPreservesSafeCorrelation(t *testing.T) {
	for _, raw := range []string{"", "bad value", "bad\nvalue", "bad/value", strings.Repeat("x", 129)} {
		got := requestIDOrNew(raw)
		if got == raw || got == "" || strings.ContainsAny(got, "\r\n") {
			t.Fatalf("requestIDOrNew(%q) returned unsafe value %q", raw, got)
		}
	}
	if got := requestIDOrNew("school-2026_09.14"); got != "school-2026_09.14" {
		t.Fatalf("safe request id was unexpectedly replaced: %q", got)
	}
}
