package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessTokenValidatesClaimsAndAlgorithm(t *testing.T) {
	s := &Server{cfg: Config{AccessSecret: "hardening-test-access-secret-32-characters"}}
	valid, err := s.token(User{Base: Base{ID: "user-1"}, Role: "admin"}, s.cfg.AccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("create valid token: %v", err)
	}
	_, uid, role, err := s.parseAccessToken(valid)
	if err != nil || uid != "user-1" || role != "admin" {
		t.Fatalf("valid token rejected: uid=%q role=%q err=%v", uid, role, err)
	}

	missingSubject := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	missingSubjectToken, err := missingSubject.SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		t.Fatalf("sign malformed token: %v", err)
	}
	if _, _, _, err := s.parseAccessToken(missingSubjectToken); err == nil {
		t.Fatal("token without subject was accepted")
	}

	wrongAlgorithm := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub":  "user-1",
		"role": "admin",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	wrongAlgorithmToken, err := wrongAlgorithm.SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		t.Fatalf("sign wrong algorithm token: %v", err)
	}
	if _, _, _, err := s.parseAccessToken(wrongAlgorithmToken); err == nil {
		t.Fatal("HS512 token was accepted by HS256 parser")
	}
}

func TestAcademicYearDateValidationAcceptsDateAndRFC3339(t *testing.T) {
	start, err := parseFlexibleDate("2030-07-01")
	if err != nil {
		t.Fatalf("date-only parse failed: %v", err)
	}
	end, err := parseFlexibleDate("2031-06-30T00:00:00Z")
	if err != nil {
		t.Fatalf("RFC3339 parse failed: %v", err)
	}
	if err := validateAcademicYearDates(&TahunAjaran{TanggalMulai: start, TanggalSelesai: end}); err != nil {
		t.Fatalf("valid academic year rejected: %v", err)
	}
	badGenap := start.AddDate(0, -1, 0)
	if err := validateAcademicYearDates(&TahunAjaran{TanggalMulai: start, TanggalSelesai: end, TanggalMulaiSemesterGenap: &badGenap}); err == nil {
		t.Fatal("semester start outside academic year was accepted")
	}
}

func TestWeakInitialPasswordRejectsPlaceholders(t *testing.T) {
	for _, password := range []string{
		"Admin123",
		"GANTI_DENGAN_PASSWORD_ADMIN_KUAT_MINIMAL_12_KARAKTER",
		"ubah-ini-sekarang-123",
	} {
		if !weakInitialPassword(password) {
			t.Fatalf("placeholder password %q was accepted", password)
		}
	}
	if weakInitialPassword("LautBiru!2026#Aman") {
		t.Fatal("strong initial password was rejected")
	}
}

func TestParseDatabaseURLPreservesSSLModeForCLIBackups(t *testing.T) {
	info, err := parseDatabaseURL("postgres://backup-user:p%40ss@db.example:5432/pkbm?sslmode=require")
	if err != nil {
		t.Fatalf("parse postgres URL: %v", err)
	}
	if info.Password != "p@ss" || info.SSLMode != "require" {
		t.Fatalf("connection options not preserved: %+v", info)
	}
	envVars := pgCommandEnv(info)
	joined := strings.Join(envVars, "\n")
	if !strings.Contains(joined, "PGSSLMODE=require") {
		t.Fatal("PGSSLMODE was not passed to PostgreSQL CLI")
	}
}

func TestProductionConfigRequiresPublicHTTPSOrigin(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:secret@db.example:5432/pkbm?sslmode=require")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://lms.example.test")
	t.Setenv("PUBLIC_BASE_URL", "https://lms.example.test")
	t.Setenv("TURNSTILE_SECRET_KEY", "turnstile-secret")
	t.Setenv("TURNSTILE_SITE_KEY", "turnstile-site")
	t.Setenv("BACKUP_R2_BUCKET", "")
	valid := Config{
		AccessSecret:  "access-secret-0123456789-0123456789",
		RefreshSecret: "refresh-secret-0123456789-0123456789",
		Env:           "production",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
	t.Setenv("DATABASE_URL", "postgres://app:ganti_dengan_password@db.example:5432/pkbm?sslmode=require")
	if err := validateConfig(valid); err == nil {
		t.Fatal("production config accepted the database password placeholder")
	}
	t.Setenv("PUBLIC_BASE_URL", "")
	if err := validateConfig(valid); err == nil {
		t.Fatal("production config without PUBLIC_BASE_URL was accepted")
	}
}

func TestProductionConfigRejectsUnencryptedDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:secret@db.example:5432/pkbm?sslmode=disable")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://lms.example.test")
	t.Setenv("PUBLIC_BASE_URL", "https://lms.example.test")
	t.Setenv("TURNSTILE_SECRET_KEY", "turnstile-secret")
	t.Setenv("TURNSTILE_SITE_KEY", "turnstile-site")
	valid := Config{
		AccessSecret:  "access-secret-0123456789-0123456789",
		RefreshSecret: "refresh-secret-0123456789-0123456789",
		Env:           "production",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
	}
	if err := validateConfig(valid); err == nil {
		t.Fatal("production config accepted a database connection without TLS")
	}
}

func TestProductionAPIErrorMasksDatabaseDetailsEvenOnBadRequest(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	app := fiber.New(fiber.Config{ErrorHandler: apiError})
	app.Get("/leak", func(c *fiber.Ctx) error {
		return fiber.NewError(http.StatusBadRequest, "duplicated key not allowed: users_email_key on database postgres://secret")
	})
	app.Get("/validation", func(c *fiber.Ctx) error {
		return fiber.NewError(http.StatusBadRequest, "judul wajib diisi")
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/leak", nil))
	if err != nil {
		t.Fatal(err)
	}
	body := readAndClose(t, res)
	if res.StatusCode != http.StatusBadRequest || strings.Contains(body, "postgres://") || strings.Contains(body, "users_email_key") {
		t.Fatalf("production response leaked internal database detail: status=%d body=%s", res.StatusCode, body)
	}

	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/validation", nil))
	if err != nil {
		t.Fatal(err)
	}
	body = readAndClose(t, res)
	if !strings.Contains(body, "judul wajib diisi") {
		t.Fatalf("safe validation message was unexpectedly masked: %s", body)
	}
}

func TestUploadHeaderMatchesRejectsRenamedWebpAndAcceptsValidWebp(t *testing.T) {
	valid := append([]byte("RIFF"), make([]byte, 4)...)
	valid = append(valid, []byte("WEBP")...)
	if !uploadHeaderMatches(".webp", valid) {
		t.Fatal("valid WEBP header was rejected")
	}
	if uploadHeaderMatches(".webp", []byte("not-an-image")) {
		t.Fatal("renamed non-image was accepted as WEBP")
	}
}

func TestSimulationImagesRequireAlternativeText(t *testing.T) {
	if err := validateSimulasiBahan(simulasiBahanInput{Judul: "Diagram", Jenis: "image", Konten: "uploads/diagram.webp"}); err == nil {
		t.Fatal("image material without alt text was accepted")
	}
	if err := validateStimulus(simulasiStimulusIn{Jenis: "image", Konten: "uploads/diagram.webp"}); err == nil {
		t.Fatal("image stimulus without alt text was accepted")
	}
}
