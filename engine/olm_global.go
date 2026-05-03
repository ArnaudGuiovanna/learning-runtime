// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

// Package engine — Open Learner Model global aggregator.
//
// BuildGlobalOLMSnapshot rolls up across all non-archived domains for a learner
// to power the cockpit's "Modèle global" tab.

package engine

import "time"

// TimePoint is a daily aggregate sample (for sparklines).
type TimePoint struct {
	Day   string  `json:"day"`   // YYYY-MM-DD UTC
	Value float64 `json:"value"`
}

// DomainSummary is one row of the Savoir column.
type DomainSummary struct {
	DomainID     string  `json:"domain_id"`
	DomainName   string  `json:"domain_name"`
	PersonalGoal string  `json:"personal_goal"`
	Solid        int     `json:"solid"`
	InProgress   int     `json:"in_progress"`
	Fragile      int     `json:"fragile"`
	NotStarted   int     `json:"not_started"`
	KSTProgress  float64 `json:"kst_progress"`
}

// GoalProgress is one row of the Objectifs column.
type GoalProgress struct {
	DomainID     string  `json:"domain_id"`
	PersonalGoal string  `json:"personal_goal"`
	Progress     float64 `json:"progress"` // mirror of KSTProgress
}

// LearnerEvent is one row of the recent events timeline.
type LearnerEvent struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"` // "mastery_threshold"|"calibration_threshold"|"retention_drop"|"streak_start"
	Message string    `json:"message"`
	Concept string    `json:"concept,omitempty"`
}

// GlobalOLMSnapshot is the aggregator payload for the cockpit's global tab.
type GlobalOLMSnapshot struct {
	Streak              int             `json:"streak"`
	TotalSolid          int             `json:"total_solid"`
	Domains             []DomainSummary `json:"domains"`
	CalibrationHistory  []TimePoint     `json:"calibration_history"`
	AutonomyHistory     []TimePoint     `json:"autonomy_history"`
	SatisfactionHistory []TimePoint     `json:"satisfaction_history"`
	Goals               []GoalProgress  `json:"goals"`
	RecentEvents        []LearnerEvent  `json:"recent_events"`
}
