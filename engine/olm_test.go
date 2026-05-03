// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"

	"tutor-mcp/db"
)

var olmTestDSNCounter int64

// newOLMTestStore opens a fresh in-memory SQLite database with migrations applied
// and returns the wrapped Store + raw *sql.DB. Reused across olm_test.go.
func newOLMTestStore(t *testing.T) (*db.Store, *sql.DB) {
	t.Helper()
	n := atomic.AddInt64(&olmTestDSNCounter, 1)
	dsn := fmt.Sprintf("file:olm_%s_%d?mode=memory&cache=shared", t.Name(), n)
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(raw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	return db.NewStore(raw), raw
}

func TestBuildOLMSnapshot_NoDomain_ReturnsError(t *testing.T) {
	store, _ := newOLMTestStore(t)

	snap, err := BuildOLMSnapshot(store, "nonexistent_learner", "")
	if err == nil {
		t.Fatalf("expected error for learner with no active domain, got snap=%+v", snap)
	}
}
