// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package db

import "database/sql"

// GetChatModeEnabled reads the per-learner chat_mode_enabled preference.
// Returns false (and nil error) when the learner row does not exist —
// the caller treats unknown learners as iframe-mode by default.
func (s *Store) GetChatModeEnabled(learnerID string) (bool, error) {
	var v int
	err := s.db.QueryRow(
		`SELECT chat_mode_enabled FROM learners WHERE id = ?`, learnerID,
	).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// SetChatModeEnabled toggles the per-learner chat_mode_enabled flag.
// No-op (no error) if the learner row does not exist.
func (s *Store) SetChatModeEnabled(learnerID string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.Exec(
		`UPDATE learners SET chat_mode_enabled = ? WHERE id = ?`, v, learnerID,
	)
	return err
}
