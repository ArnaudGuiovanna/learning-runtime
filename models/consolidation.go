// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package models

import "time"

type PendingConsolidation struct {
	ID          int64      `json:"id"`
	LearnerID   string     `json:"learner_id"`
	PeriodType  string     `json:"period_type"`
	PeriodKey   string     `json:"period_key"`
	Status      string     `json:"status"`
	DetectedAt  time.Time  `json:"detected_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
