package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func TestLoginRateLimitIsScopedToNormalizedAccount(t *testing.T) {
	app := fiber.New()
	app.Post("/login", limiter.New(limiter.Config{
		Max:                    1,
		Expiration:             time.Minute,
		KeyGenerator:           loginRateLimitKey,
		SkipSuccessfulRequests: true,
	}), func(c *fiber.Ctx) error { return c.SendStatus(http.StatusUnauthorized) })

	post := func(login string) int {
		body := `{"login":"` + login + `"}`
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatalf("send login request: %v", err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}

	if got := post("guru@example.test"); got != http.StatusUnauthorized {
		t.Fatalf("first account attempt status = %d, want 401", got)
	}
	if got := post("other@example.test"); got != http.StatusUnauthorized {
		t.Fatalf("different account should have an independent budget, got %d", got)
	}
	if got := post(" GURU@example.test "); got != http.StatusTooManyRequests {
		t.Fatalf("normalized repeated account status = %d, want 429", got)
	}
}

func TestRefreshRateLimitIsScopedToRefreshTokenAndSkipsMissingCookie(t *testing.T) {
	app := fiber.New()
	app.Post("/refresh", limiter.New(limiter.Config{
		Max:          1,
		Expiration:   time.Minute,
		Next:         func(c *fiber.Ctx) bool { return strings.TrimSpace(c.Cookies("refresh_token")) == "" },
		KeyGenerator: refreshRateLimitKey,
	}), func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	post := func(token string) int {
		request := httptest.NewRequest(http.MethodPost, "/refresh", nil)
		if token != "" {
			request.AddCookie(&http.Cookie{Name: "refresh_token", Value: token})
		}
		response, err := app.Test(request, -1)
		if err != nil {
			t.Fatalf("send refresh request: %v", err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}

	for i := 0; i < 2; i++ {
		if got := post(""); got != http.StatusNoContent {
			t.Fatalf("missing-cookie request %d status = %d, want 204", i+1, got)
		}
	}
	if got := post("refresh-token-a"); got != http.StatusNoContent {
		t.Fatalf("first token A request status = %d, want 204", got)
	}
	if got := post("refresh-token-b"); got != http.StatusNoContent {
		t.Fatalf("different refresh token should have an independent budget, got %d", got)
	}
	if got := post("refresh-token-a"); got != http.StatusTooManyRequests {
		t.Fatalf("repeated token A request status = %d, want 429", got)
	}
}
