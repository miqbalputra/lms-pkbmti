package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// An opt-in browser harness: no connection to the application's data database,
// no integrations, temporary storage, and a server-side write barrier.
func TestMenuAuditServer(t *testing.T) {
	if os.Getenv("MENU_AUDIT_SERVER") != "1" {
		t.Skip("isolated menu audit only")
	}
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		for _, prefix := range []string{"R2_", "CLOUDFLARE_", "BACKUP_", "SMTP_", "RESEND_", "OPENAI_", "GEMINI_", "N8N_", "CBT_"} {
			if strings.HasPrefix(key, prefix) {
				t.Setenv(key, "")
				break
			}
		}
	}
	root := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	staticDir := filepath.Clean(filepath.Join(originalDir, "../../../frontend/dist"))
	if err = os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	t.Setenv("DATABASE_URL", "")
	t.Setenv("UPLOADS_DIR", filepath.Join(root, "uploads"))
	t.Setenv("BACKUP_DIR", filepath.Join(root, "backups"))
	s, inner, _, k, _, mp := reportFixture(t)
	for _, account := range []struct {
		login string
		wali  bool
	}{{"guru-audit", true}, {"guru-mapel-audit", false}} {
		var tutorID string
		if account.wali {
			tutorID = *k.WaliKelasID
		} else {
			tutor := Tutor{Nama: "Guru Mapel Uji", JenisKelamin: "P"}
			if err := s.db.Create(&tutor).Error; err != nil {
				t.Fatal(err)
			}
			tutorID = tutor.ID
		}
		hash, err := bcryptHash("Guru12345")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.db.Create(&User{Username: account.login, Email: account.login + "@gmail.com", Role: "guru", TutorID: &tutorID, PasswordHash: hash, IsActive: true}).Error; err != nil {
			t.Fatal(err)
		}
		if err = s.db.Create(&PenugasanGuruMapel{TutorID: tutorID, KelasID: k.ID, MapelID: mp.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	inner.Static("/", staticDir)
	inner.Get("/*", func(c *fiber.Ctx) error { return c.SendFile(filepath.Join(staticDir, "index.html")) })
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	productionCSP := os.Getenv("MENU_AUDIT_PRODUCTION_CSP") == "1"
	if productionCSP {
		app.Use(func(c *fiber.Ctx) error {
			nonce := uuid.NewString()
			c.Locals("cspNonce", nonce)
			c.Set("Content-Security-Policy", productionContentSecurityPolicy(nonce))
			return c.Next()
		})
	}
	app.Get("/orangtua", s.serveOrangTuaPortalPage)
	app.Get("/__menu-audit/exam", s.serveUjianOnlinePage)
	app.Get("/api/backup", s.backupReadAuth, s.listBackupsHandler)
	app.Get("/api/notifikasi/stream-ticket", s.auth, s.issueNotificationStreamTicket)
	app.Get("/api/notifikasi/stream", s.streamNotifikasi)
	app.Get("/__menu-audit", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"isolated": true, "storage": "temporary", "writes": "blocked", "classId": k.ID, "productionCSP": productionCSP})
	})
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead && c.Method() != fiber.MethodOptions && c.Path() != "/api/auth/login" && c.Path() != "/api/auth/refresh" && c.Path() != "/api/auth/logout" {
			return fiber.NewError(405, "Audit mode: changes are disabled")
		}
		return c.Next()
	})
	app.Mount("/", inner)
	if err := app.Listen("127.0.0.1:19101"); err != nil {
		t.Fatal(err)
	}
}
