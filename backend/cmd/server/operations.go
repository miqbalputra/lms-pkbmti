package main

// Operational notifications deliberately contain only service metadata. They
// are useful for an administrator's automation (n8n/Slack/etc.) but must
// never turn a backup failure into a path for leaking credentials or student
// data.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const operationAlertCooldown = 24 * time.Hour

type operationAlertState struct {
	Base
	Fingerprint string     `gorm:"uniqueIndex;not null"`
	Event       string     `gorm:"index;not null"`
	LastSentAt  *time.Time `json:"lastSentAt,omitempty"`
}

type operationNotifier struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func operationWebhookURL() string { return strings.TrimSpace(os.Getenv("OPERATIONS_WEBHOOK_URL")) }

func operationWebhookTimeout() time.Duration {
	v, err := time.ParseDuration(env("OPERATIONS_WEBHOOK_TIMEOUT", "5s"))
	if err != nil || v <= 0 || v > time.Minute {
		return 5 * time.Second
	}
	return v
}

func backupAlertMaxAge() time.Duration {
	hours, err := strconv.Atoi(strings.TrimSpace(env("BACKUP_ALERT_MAX_AGE_HOURS", "78")))
	if err != nil || hours < 72 || hours > 24*30 {
		hours = 78
	}
	return time.Duration(hours) * time.Hour
}

func safeOperationError(err error) string {
	if err == nil {
		return ""
	}
	// The original error can contain a DSN, object endpoint, or a filesystem
	// location. Preserve enough context for the dashboard without exposing it.
	return "Operasi tidak dapat diselesaikan. Periksa log server untuk detail aman."
}

func operationLog(event string, fields map[string]any) {
	payload := map[string]any{"event": event, "at": time.Now().UTC().Format(time.RFC3339)}
	for k, v := range fields {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err == nil {
		log.Print(string(b))
	}
}

func (s *Server) notifyOperation(event, fingerprint, message string, job *R2BackupJob) {
	endpoint := operationWebhookURL()
	if endpoint == "" {
		return
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		operationLog("operation_webhook_invalid", map[string]any{"operation": event})
		return
	}
	now := time.Now()
	s.notifier.mu.Lock()
	if s.notifier.last == nil {
		s.notifier.last = make(map[string]time.Time)
	}
	if previous, ok := s.notifier.last[fingerprint]; ok && now.Sub(previous) < operationAlertCooldown {
		s.notifier.mu.Unlock()
		return
	}
	s.notifier.last[fingerprint] = now
	s.notifier.mu.Unlock()
	if s.db != nil {
		var state operationAlertState
		if err := s.db.Where("fingerprint = ?", fingerprint).First(&state).Error; err == nil && state.LastSentAt != nil && now.Sub(*state.LastSentAt) < operationAlertCooldown {
			return
		}
		if state.ID == "" {
			state = operationAlertState{Fingerprint: fingerprint, Event: event, LastSentAt: &now}
			if err := s.db.Create(&state).Error; err != nil {
				operationLog("operation_alert_state_failed", map[string]any{"operation": event})
			}
		} else {
			s.db.Model(&state).Updates(map[string]any{"event": event, "last_sent_at": &now})
		}
	}

	go func() {
		payload := map[string]any{
			"event": event, "occurredAt": now.UTC().Format(time.RFC3339),
			"application": "pkbm-lms", "message": message,
		}
		if job != nil {
			payload["job"] = map[string]any{"id": job.ID, "kind": job.Kind, "status": job.Status, "phase": job.Phase}
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), operationWebhookTimeout())
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if secret := strings.TrimSpace(os.Getenv("OPERATIONS_WEBHOOK_SECRET")); secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(body)
			req.Header.Set("X-PKBM-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		resp, err := (&http.Client{}).Do(req)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			operationLog("operation_webhook_failed", map[string]any{"operation": event})
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
}

func (s *Server) monitorBackupHealth() {
	var previousHealth *bool
	check := func() {
		_, healthy := s.healthPayload()
		if previousHealth != nil && *previousHealth != healthy {
			event, message := "health_recovered", "Health aplikasi kembali normal."
			if !healthy {
				event, message = "health_degraded", "Health aplikasi terdegradasi; periksa database dan deployment."
			}
			s.notifyOperation(event, event, message, nil)
		}
		previousHealth = &healthy
		if !r2Enabled() {
			return
		}
		var last R2BackupJob
		err := s.db.Where("kind = ? AND status = ?", "scheduled", "succeeded").Order("finished_at desc").First(&last).Error
		if err != nil || last.FinishedAt == nil || time.Since(*last.FinishedAt) > backupAlertMaxAge() {
			s.notifyOperation("backup_stale", "backup_stale", "Backup otomatis R2 belum berhasil dalam rentang yang diizinkan.", nil)
		}
	}
	check()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		check()
	}
}
