package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOperationalWebhookSignsSanitizedPayload(t *testing.T) {
	received := make(chan *http.Request, 1)
	bodyReceived := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodyReceived <- b
		received <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("OPERATIONS_WEBHOOK_URL", server.URL)
	t.Setenv("OPERATIONS_WEBHOOK_SECRET", "webhook-test-secret")
	s := &Server{notifier: operationNotifier{last: make(map[string]time.Time)}}
	job := &R2BackupJob{Kind: "restore", Status: "failed", Phase: "validating", SourceKey: "must-not-appear"}
	s.notifyOperation("restore_failed", "restore_failed:test", "Restore R2 gagal.", job)
	var req *http.Request
	select {
	case req = <-received:
	case <-time.After(time.Second):
		t.Fatal("webhook not called")
	}
	body := <-bodyReceived
	if string(body) == "" || string(body) == job.SourceKey {
		t.Fatalf("unexpected payload %q", body)
	}
	if string(body) != "" && strings.Contains(string(body), "must-not-appear") {
		t.Fatalf("payload leaks archive key: %s", body)
	}
	mac := hmac.New(sha256.New, []byte("webhook-test-secret"))
	_, _ = mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if req.Header.Get("X-PKBM-Signature-256") != want {
		t.Fatalf("signature = %q, want %q", req.Header.Get("X-PKBM-Signature-256"), want)
	}
}

func TestOperationalWebhookRetriesAfterDeliveryFailure(t *testing.T) {
	var attempts atomic.Int32
	completed := make(chan int, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := int(attempts.Add(1))
		defer func() { completed <- attempt }()
		if attempt == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("APP_ENV", "development")
	t.Setenv("OPERATIONS_WEBHOOK_URL", server.URL)
	s := &Server{notifier: operationNotifier{last: make(map[string]time.Time)}}

	s.notifyOperation("backup_failed", "backup_failed:retry-test", "Backup gagal.", nil)
	select {
	case attempt := <-completed:
		if attempt != 1 {
			t.Fatalf("first webhook attempt = %d, want 1", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("first webhook attempt did not finish")
	}
	deadline := time.Now().Add(time.Second)
	for {
		s.notifier.mu.Lock()
		inFlight := s.notifier.inFlight["backup_failed:retry-test"]
		s.notifier.mu.Unlock()
		if !inFlight {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed webhook attempt remained in flight")
		}
		time.Sleep(time.Millisecond)
	}

	// A failed delivery must not consume the 24-hour cooldown.
	s.notifyOperation("backup_failed", "backup_failed:retry-test", "Backup gagal.", nil)
	select {
	case attempt := <-completed:
		if attempt != 2 {
			t.Fatalf("retry webhook attempt = %d, want 2", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("failed webhook was not retried")
	}
}

func TestOperationalWebhookRequiresHTTPSInProduction(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("APP_ENV", "production")
	t.Setenv("OPERATIONS_WEBHOOK_URL", server.URL)
	s := &Server{notifier: operationNotifier{last: make(map[string]time.Time)}}
	s.notifyOperation("backup_failed", "backup_failed:https-test", "Backup gagal.", nil)
	select {
	case <-called:
		t.Fatal("production webhook must not use plain HTTP")
	case <-time.After(100 * time.Millisecond):
	}
}
