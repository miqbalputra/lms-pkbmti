package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestNotificationStreamTicketIsSingleUse(t *testing.T) {
	s := &Server{}
	app := fiber.New()
	app.Get("/ticket", func(c *fiber.Ctx) error {
		c.Locals("userID", "user-1")
		return s.issueNotificationStreamTicket(c)
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/ticket", nil))
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var payload struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode ticket response: %v", err)
	}
	_ = res.Body.Close()
	if payload.Ticket == "" {
		t.Fatal("ticket response is empty")
	}
	if uid, ok := s.consumeNotificationStreamTicket(payload.Ticket); !ok || uid != "user-1" {
		t.Fatalf("first consume = (%q, %v), want (user-1, true)", uid, ok)
	}
	if _, ok := s.consumeNotificationStreamTicket(payload.Ticket); ok {
		t.Fatal("ticket was reusable")
	}
}
