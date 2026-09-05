package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
