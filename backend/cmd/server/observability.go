package main

import (
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

type backupMetrics struct {
	lastSuccessUnix       atomic.Int64
	lastFailureUnix       atomic.Int64
	lastOffsiteUnix       atomic.Int64
	lastDrillUnix         atomic.Int64
	totalSuccess          atomic.Uint64
	totalFailure          atomic.Uint64
	offsiteSuccesses      atomic.Uint64
	restoreDrillSuccesses atomic.Uint64
}

func (m *backupMetrics) recordFailure() {
	m.lastFailureUnix.Store(time.Now().Unix())
	m.totalFailure.Add(1)
}

func (m *backupMetrics) recordSuccess(offsite, drill bool) {
	now := time.Now().Unix()
	m.lastSuccessUnix.Store(now)
	m.totalSuccess.Add(1)
	if offsite {
		m.lastOffsiteUnix.Store(now)
		m.offsiteSuccesses.Add(1)
	}
	if drill {
		m.lastDrillUnix.Store(now)
		m.restoreDrillSuccesses.Add(1)
	}
}

func unixTimeOrNil(value int64) any {
	if value <= 0 {
		return nil
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func (s *Server) healthPayload() (fiber.Map, bool) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fiber.Map{"status": "unhealthy", "database": dialect()}, false
	}
	if err := sqlDB.Ping(); err != nil {
		return fiber.Map{"status": "unhealthy", "database": dialect()}, false
	}
	return fiber.Map{
		"status":        "ok",
		"database":      dialect(),
		"uptimeSeconds": int64(time.Since(s.startedAt).Seconds()),
		"backup": fiber.Map{
			"lastSuccessAt":          unixTimeOrNil(s.metrics.lastSuccessUnix.Load()),
			"lastFailureAt":          unixTimeOrNil(s.metrics.lastFailureUnix.Load()),
			"lastOffsiteAt":          unixTimeOrNil(s.metrics.lastOffsiteUnix.Load()),
			"lastRestoreDrillAt":     unixTimeOrNil(s.metrics.lastDrillUnix.Load()),
			"totalSuccess":           s.metrics.totalSuccess.Load(),
			"totalFailure":           s.metrics.totalFailure.Load(),
			"offsiteConfigured":      strings.TrimSpace(os.Getenv("BACKUP_OFFSITE_URL")) != "",
			"restoreDrillConfigured": strings.TrimSpace(os.Getenv("BACKUP_DRILL_DATABASE_URL")) != "",
		},
	}, true
}
