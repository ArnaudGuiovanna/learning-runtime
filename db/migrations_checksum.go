// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna/tutor-mcp
// SPDX-License-Identifier: MIT

package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

// migration is a single versioned schema change. Body is the SQL executed when
// the migration is first applied; Checksum is computed from Body and persisted
// in schema_migrations to detect drift on subsequent startups.
//
// IgnoreExecErrors is set for ALTER-style migrations whose statements may
// already have been applied to a pre-existing database (e.g. "duplicate column"
// from sqlite). For those, errors during Exec are intentionally swallowed so
// the migration can still be recorded as applied.
type migration struct {
	Version          string
	Body             string
	IgnoreExecErrors bool
}

// checksum returns the lowercase hex SHA-256 of the migration body. Whitespace
// is preserved deliberately so editing a body — even cosmetically — surfaces as
// a checksum mismatch on the next startup. Operators who knowingly want to
// rewrite history must update the row in schema_migrations themselves.
func (m migration) checksum() string {
	sum := sha256.Sum256([]byte(m.Body))
	return hex.EncodeToString(sum[:])
}

// ensureSchemaMigrationsTable creates the bookkeeping table used by Migrate to
// track which migrations have been applied and the checksum they were applied
// with. Called unconditionally before any other migration runs.
func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

// applyMigration executes one migration if it has not been applied yet, or
// verifies its stored checksum if it has. A mismatch returns an error
// containing "checksum mismatch" so callers (and tests) can detect drift.
//
// Issue #118: the body Exec and the bookkeeping INSERT are wrapped in a
// single transaction so a crash (or any error) between them cannot leave
// the schema mutated but unrecorded. The IgnoreExecErrors path is a special
// case: a failed Exec inside a sqlite tx aborts that tx, so we have to roll
// it back and start a fresh tx for the INSERT — see recordIgnoredMigration.
func applyMigration(db *sql.DB, m migration) error {
	var storedChecksum string
	// The checksum-stored check is read-only and stays outside the tx.
	row := db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version = ?`, m.Version)
	switch err := row.Scan(&storedChecksum); err {
	case nil:
		if storedChecksum != m.checksum() {
			return fmt.Errorf(
				"schema_migrations: checksum mismatch for version %q: stored=%s current=%s "+
					"(migration body changed since it was applied; manual intervention required)",
				m.Version, storedChecksum, m.checksum(),
			)
		}
		return nil
	case sql.ErrNoRows:
		// fall through to apply
	default:
		return fmt.Errorf("schema_migrations: read version %q: %w", m.Version, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx %q: %w", m.Version, err)
	}
	// Rollback is a no-op once Commit succeeds; if the IgnoreExecErrors
	// branch already rolled back manually, this returns sql.ErrTxDone
	// which we intentionally ignore.
	defer func() { _ = tx.Rollback() }()

	if _, execErr := tx.Exec(m.Body); execErr != nil {
		if !m.IgnoreExecErrors {
			return fmt.Errorf("apply migration %q: %w", m.Version, execErr)
		}
		// IgnoreExecErrors covers ALTERs already applied on legacy DBs
		// ("duplicate column name", "no such column" on DROP COLUMN,
		// etc.). The failed statement aborted this tx, so roll it back
		// and record the bookkeeping row on a fresh tx — subsequent
		// runs then take the "checksum already matches, skip" branch.
		_ = tx.Rollback()
		return recordIgnoredMigration(db, m)
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`,
		m.Version, m.checksum(),
	); err != nil {
		return fmt.Errorf("record migration %q: %w", m.Version, err)
	}
	return tx.Commit()
}

// recordIgnoredMigration inserts the schema_migrations bookkeeping row for
// an IgnoreExecErrors migration whose body Exec failed. It runs on its own
// transaction because the original tx was aborted by the failing body
// statement (sqlite refuses further writes on an aborted tx).
func recordIgnoredMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin replacement tx %q: %w", m.Version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`,
		m.Version, m.checksum(),
	); err != nil {
		return fmt.Errorf("record migration %q: %w", m.Version, err)
	}
	return tx.Commit()
}

// buildMigrations assembles the ordered migration list from the embedded
// schema.sql, the historical ALTER list, the BKT data migration, and the
// idempotent CREATE TABLE/INDEX list. The order here is the canonical apply
// order; appending new entries is safe, reordering is not.
func buildMigrations() []migration {
	out := make([]migration, 0, 2+len(alterMigrations)+1+len(idempotentMigrations))
	out = append(out, migration{
		Version: "0001_base_schema",
		Body:    schemaSQL,
	})
	for i, body := range alterMigrations {
		out = append(out, migration{
			Version: fmt.Sprintf("0002_alter_%03d_%s", i+1, alterShortName(body)),
			Body:    body,
			// ALTERs may already be present on legacy DBs that ran the old
			// "ignore errors" migrator. Swallow per-statement errors so the
			// row still gets recorded.
			IgnoreExecErrors: true,
		})
	}
	out = append(out, migration{
		Version: "0003_data_plearn_default_0_15",
		Body:    `UPDATE concept_states SET p_learn = 0.15 WHERE p_learn = 0.3`,
	})
	// Issue #61: scrub the legacy `level`, `background`, `learning_style`
	// keys out of profile_json. The tool no longer accepts them so the
	// re-introduction surface is closed; json_remove is idempotent.
	out = append(out, migration{
		Version: "0003_data_drop_legacy_learner_profile_fields",
		Body: `UPDATE learners
		 SET profile_json = json_remove(profile_json, '$.level', '$.background', '$.learning_style')
		 WHERE profile_json IS NOT NULL
		   AND profile_json != ''
		   AND (json_extract(profile_json, '$.level') IS NOT NULL
		     OR json_extract(profile_json, '$.background') IS NOT NULL
		     OR json_extract(profile_json, '$.learning_style') IS NOT NULL)`,
	})
	for i, body := range idempotentMigrations {
		out = append(out, migration{
			Version: fmt.Sprintf("0004_idempotent_%03d_%s", i+1, alterShortName(body)),
			Body:    body,
		})
	}
	return out
}

// alterShortName extracts a short, stable token from a SQL statement to make
// the version key human-readable. It is purely cosmetic — the checksum, not the
// version string, is what guards integrity.
func alterShortName(sql string) string {
	// Take the first significant identifier-ish run of characters.
	const maxLen = 40
	var b strings.Builder
	prevSpace := true
	for _, r := range sql {
		switch {
		case r == '\n' || r == '\t':
			if !prevSpace {
				b.WriteByte('_')
				prevSpace = true
			}
		case r == ' ':
			if !prevSpace {
				b.WriteByte('_')
				prevSpace = true
			}
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
			prevSpace = false
		default:
			if !prevSpace {
				b.WriteByte('_')
				prevSpace = true
			}
		}
		if b.Len() >= maxLen {
			break
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "stmt"
	}
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.ToLower(s)
}
