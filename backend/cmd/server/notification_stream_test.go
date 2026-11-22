package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestNotificationStreamConnectsOverHTTP(t *testing.T) {
	db := isolatedTestDB(t, "notification-stream")
	if err := db.AutoMigrate(&Notifikasi{}); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, streamTickets: map[string]notificationStreamTicket{
		"test-ticket": {UserID: "stream-user", ExpiresAt: time.Now().Add(time.Minute)},
	}}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/stream", s.streamNotifikasi)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(listener) }()
	t.Cleanup(func() { _ = app.ShutdownWithTimeout(12 * time.Second) })
	client := &http.Client{Timeout: 10 * time.Second}
	url := "http://" + listener.Addr().String() + "/stream?ticket=test-ticket"
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected SSE response: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(res.Body)
	for _, want := range []string{"event: connected\n", "data: ok\n", "\n", "event: unread\n", "data: 0\n", "\n"} {
		line, err := reader.ReadString('\n')
		if err != nil || line != want {
			t.Fatalf("stream line = %q, err = %v; want %q", line, err, want)
		}
	}
	reused, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer reused.Body.Close()
	if reused.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused ticket status = %d; want 401", reused.StatusCode)
	}
}

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
