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

// TestMigrate_DropsLegacyLearnerProfileFields is the issue #61 regression
// guard. The data migration in db/migrations.go must scrub the `level`,
// `background` and `learning_style` keys out of pre-existing profile_json
// blobs because no production component reads them and leaving them in the
// blob causes an unbounded write-only key surface.
//
// We seed two rows that mirror the historical shape (one with all three
// keys plus an unrelated key that must survive, one with a partial subset),
// run Migrate, then assert json_extract returns NULL for the dropped keys
// and the unrelated key is intact.
func TestMigrate_DropsLegacyLearnerProfileFields(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_drop_legacy_%d?mode=memory&cache=shared", testDBCounter+20000)
	testDBCounter++
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run an initial migration so the learners table (and profile_json
	// column) exists, then seed legacy rows BEFORE the second migration
	// runs the data scrub. The migration is idempotent so the seed runs
	// after a no-op second call would also work — but seeding between
	// two Migrate() calls makes the assertion order obvious.
	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	seed := []struct {
		id      string
		profile string
	}{
		{
			id: "legacy_full",
			profile: `{"level":"intermediate","background":"engineer",` +
				`"learning_style":"visual","language":"fr","device":"laptop"}`,
		},
		{
			id:      "legacy_partial",
			profile: `{"level":"beginner","autonomy_score":0.6}`,
		},
		{
			id:      "clean",
			profile: `{"language":"en"}`,
		},
	}
	for _, s := range seed {
		_, err := db.Exec(
			`INSERT INTO learners (id, email, password_hash, objective, profile_json) VALUES (?, ?, 'h', 'o', ?)`,
			s.id, s.id+"@x", s.profile,
		)
		if err != nil {
			t.Fatalf("seed %s: %v", s.id, err)
		}
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate (must scrub legacy keys): %v", err)
	}

	// Each dropped key must be absent (json_extract returns NULL) on
	// every row.
	droppedKeys := []string{"$.level", "$.background", "$.learning_style"}
	for _, s := range seed {
		for _, key := range droppedKeys {
			var v sql.NullString
			err := db.QueryRow(
				`SELECT json_extract(profile_json, ?) FROM learners WHERE id = ?`,
				key, s.id,
			).Scan(&v)
			if err != nil {
				t.Fatalf("query %s %s: %v", s.id, key, err)
			}
			if v.Valid {
				t.Errorf("learner %s: key %s should have been scrubbed, got %q", s.id, key, v.String)
			}
		}
	}

	// Unrelated keys must survive on the rows that originally carried them.
	preserved := []struct {
		id   string
		key  string
		want string
	}{
		{"legacy_full", "$.language", "fr"},
		{"legacy_full", "$.device", "laptop"},
		{"clean", "$.language", "en"},
	}
	for _, p := range preserved {
		var v sql.NullString
		err := db.QueryRow(
			`SELECT json_extract(profile_json, ?) FROM learners WHERE id = ?`,
			p.key, p.id,
		).Scan(&v)
		if err != nil {
			t.Fatalf("query %s %s: %v", p.id, p.key, err)
		}
		if !v.Valid || v.String != p.want {
			t.Errorf("learner %s: key %s should equal %q, got valid=%v value=%q", p.id, p.key, p.want, v.Valid, v.String)
		}
	}

	// legacy_partial: autonomy_score must survive, level must be gone.
	var auto sql.NullFloat64
	if err := db.QueryRow(
		`SELECT json_extract(profile_json, '$.autonomy_score') FROM learners WHERE id = 'legacy_partial'`,
	).Scan(&auto); err != nil {
		t.Fatalf("query autonomy_score: %v", err)
	}
	if !auto.Valid || auto.Float64 != 0.6 {
		t.Errorf("legacy_partial autonomy_score should equal 0.6, got valid=%v value=%v", auto.Valid, auto.Float64)
	}

	// Idempotence: running Migrate again on already-scrubbed rows is a
	// no-op (no error, no spurious changes).
	if err := Migrate(db); err != nil {
		t.Fatalf("third Migrate (idempotent on scrubbed rows): %v", err)
	}
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
