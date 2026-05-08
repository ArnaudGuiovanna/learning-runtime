package db

import (
	"database/sql"
	"fmt"
	"testing"
)

// TestMigrate_Idempotent runs Migrate twice on a fresh in-memory database;
// the second invocation must be a no-op (no error, no duplicate-table or
// duplicate-column errors propagated). Then we assert that all expected
// tables and indexes exist by querying sqlite_master.
func TestMigrate_Idempotent(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_idempo_%d?mode=memory&cache=shared", testDBCounter+10000)
	testDBCounter++
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate (must be idempotent): %v", err)
	}

	expectedTables := []string{
		"learners",
		"refresh_tokens",
		"domains",
		"concept_states",
		"interactions",
		"availability",
		"scheduled_alerts",
		"oauth_codes",
		"oauth_clients",
		"affect_states",
		"calibration_records",
		"transfer_records",
		"implementation_intentions",
		"webhook_message_queue",
	}
	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist: %v", table, err)
		}
	}

	expectedIndexes := []string{
		"idx_concept_states_learner",
		"idx_concept_states_review",
		"idx_interactions_learner_created",
		"idx_interactions_learner_concept",
		"idx_scheduled_alerts_learner_type",
		"idx_oauth_codes_expires",
		"idx_affect_states_learner",
		"idx_calibration_records_learner",
		"idx_transfer_records_learner_concept",
		"idx_interactions_self_initiated",
		"idx_interactions_misconception",
		"idx_impl_intent_learner",
		"idx_wmq_dispatch",
	}
	for _, idx := range expectedIndexes {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name = ?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("expected index %q to exist: %v", idx, err)
		}
	}

	// Sanity: all migrated columns are queryable.
	if _, err := db.Exec(
		`INSERT INTO learners (id, email, password_hash, objective, profile_json) VALUES ('m1','m@m','h','o','{}')`,
	); err != nil {
		t.Fatalf("insert with profile_json: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO interactions (learner_id, concept, activity_type, success, error_type, hints_requested, self_initiated, calibration_id, is_proactive_review, misconception_type, misconception_detail) VALUES ('m1','C','RECALL_EXERCISE',1,'',0,0,'',0,NULL,NULL)`,
	); err != nil {
		t.Fatalf("insert with v0.9/v0.10 columns: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO domains (id, learner_id, name, graph_json, personal_goal, archived, value_framings_json, last_value_axis) VALUES ('d1','m1','dn','{}','goal',0,'','')`,
	); err != nil {
		t.Fatalf("insert with domain framing columns: %v", err)
	}
}

// TestMigrate_DropsPFAColumns is the regression guard for issue #55.
// Two scenarios are exercised in the same test so that the assertion
// holds regardless of whether a deployed DB started fresh (post-#55) or
// upgraded from a pre-#55 schema that still had the columns:
//
//  1. Fresh DB: schema.sql no longer declares pfa_successes / pfa_failures.
//     After Migrate(), table_info(concept_states) must not list them.
//  2. Upgrade DB: the columns are inserted manually before Migrate(),
//     simulating a pre-#55 database. Migrate() must drop them via the
//     incremental ALTER TABLE ... DROP COLUMN entries (idempotent).
//
// If either column reappears in either scenario the test fails — that
// catches accidental re-introduction in schema.sql and accidental
// removal of the DROP COLUMN migration entries.
func TestMigrate_DropsPFAColumns(t *testing.T) {
	pfaCols := []string{"pfa_successes", "pfa_failures"}

	hasColumn := func(t *testing.T, db *sql.DB, table, col string) bool {
		t.Helper()
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			t.Fatalf("PRAGMA table_info(%s): %v", table, err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				cid     int
				name    string
				ctype   string
				notnull int
				dflt    sql.NullString
				pk      int
			)
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				t.Fatalf("scan table_info row: %v", err)
			}
			if name == col {
				return true
			}
		}
		return rows.Err() == nil && false
	}

	// Scenario 1: fresh DB.
	t.Run("fresh", func(t *testing.T) {
		dsn := fmt.Sprintf("file:migrate_pfa_fresh_%d?mode=memory&cache=shared", testDBCounter+10100)
		testDBCounter++
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		if err := Migrate(db); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		for _, col := range pfaCols {
			if hasColumn(t, db, "concept_states", col) {
				t.Errorf("concept_states.%s must not exist in fresh schema", col)
			}
		}
	})

	// Scenario 2: pre-#55 DB upgraded.
	t.Run("upgrade_from_pre_55", func(t *testing.T) {
		dsn := fmt.Sprintf("file:migrate_pfa_upgrade_%d?mode=memory&cache=shared", testDBCounter+10200)
		testDBCounter++
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { db.Close() })

		// First migration brings the schema up to current.
		if err := Migrate(db); err != nil {
			t.Fatalf("first Migrate: %v", err)
		}
		// Re-introduce the legacy columns to simulate a pre-#55 DB.
		for _, col := range pfaCols {
			if _, err := db.Exec(fmt.Sprintf(
				"ALTER TABLE concept_states ADD COLUMN %s REAL DEFAULT 0.0", col,
			)); err != nil {
				t.Fatalf("seed legacy column %s: %v", col, err)
			}
			if !hasColumn(t, db, "concept_states", col) {
				t.Fatalf("seed column %s should exist", col)
			}
		}
		// Second migration must drop them again — idempotent.
		if err := Migrate(db); err != nil {
			t.Fatalf("second Migrate: %v", err)
		}
		for _, col := range pfaCols {
			if hasColumn(t, db, "concept_states", col) {
				t.Errorf("concept_states.%s should have been dropped by Migrate", col)
			}
		}
		// Third migration must remain a no-op (no panic, no error).
		if err := Migrate(db); err != nil {
			t.Fatalf("third Migrate (idempotent): %v", err)
		}
	})
}

// TestOpenDB_Memory exercises the OpenDB helper. OpenDB appends `?_pragma=...`
// to the path before opening, so we use a file-backed temp DB to keep the DSN
// shape simple.
func TestOpenDB_Memory(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir + "/open.db")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

// TestOpenDB_BadPath ensures the error path is exercised when ping fails.
func TestOpenDB_BadPath(t *testing.T) {
	// "/proc/0/forbidden" is not openable as a file-backed sqlite db on Linux.
	_, err := OpenDB("/proc/0/forbidden-not-a-real-sqlite-file")
	if err == nil {
		t.Fatal("expected error for unreachable path")
	}
}
