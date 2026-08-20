package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var isolatedTestDatabaseSequence atomic.Uint64

// isolatedTestDB provides a new SQLite database for every test invocation.
// Closing the pool is important: shared in-memory SQLite databases otherwise
// survive a repeated go test run while a connection remains open.
func isolatedTestDB(t *testing.T, label string) *gorm.DB {
	t.Helper()
	safeLabel := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(label)
	dsn := fmt.Sprintf(
		"file:pkbm-test-%s-%d-%d?mode=memory&cache=shared",
		safeLabel,
		time.Now().UnixNano(),
		isolatedTestDatabaseSequence.Add(1),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get isolated test database connection: %v", err)
	}
	// A single connection also avoids SQLite in-memory transaction deadlocks.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
