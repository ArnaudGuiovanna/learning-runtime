// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"fmt"
	"time"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"
)

// interactionInput carries the minimal fields needed to persist an
// interaction and drive the BKT/FSRS/IRT/PFA update chain.  Fields not
// relevant to a given call site (e.g. HintsRequested for submit_answer)
// should be left at their zero values.
type interactionInput struct {
	Concept             string
	ActivityType        string
	Success             bool
	ResponseTimeSeconds float64
	Confidence          float64 // 0 if not provided
	ErrorType           string  // "" if not applicable
	Notes               string
	HintsRequested      int
	SelfInitiated       bool
	CalibrationID       string
	MisconceptionType   string
	MisconceptionDetail string
	DomainID            string // persisted on the interaction row (issue #24)
}

// applyInteraction persists the interaction and updates the learner's
// cognitive state (BKT, FSRS, IRT, PFA) for the concept.  Returns the
// resulting ConceptState (post-update) and an error if any persistence
// step failed.
func applyInteraction(
	deps *Deps,
	learnerID string,
	input interactionInput,
	now time.Time,
) (*models.ConceptState, error) {
	// Load or bootstrap concept state.
	cs, err := deps.Store.GetConceptState(learnerID, input.Concept)
	if err != nil {
		cs = models.NewConceptState(learnerID, input.Concept)
	}

	// Build and persist the interaction row.
	interaction := &models.Interaction{
		LearnerID:      learnerID,
		Concept:        input.Concept,
		ActivityType:   input.ActivityType,
		Success:        input.Success,
		ResponseTime:   int(input.ResponseTimeSeconds),
		Confidence:     input.Confidence,
		ErrorType:      input.ErrorType,
		Notes:          input.Notes,
		HintsRequested: input.HintsRequested,
		SelfInitiated:  input.SelfInitiated,
		CalibrationID:  input.CalibrationID,
		DomainID:       input.DomainID,
		CreatedAt:      now,
	}

	// Misconception fields — only stored on failures.
	if !input.Success && input.MisconceptionType != "" {
		interaction.MisconceptionType = input.MisconceptionType
		interaction.MisconceptionDetail = input.MisconceptionDetail
	}

	// Proactive review flag.
	if cs.NextReview != nil && cs.NextReview.After(now) && cs.CardState != "new" {
		interaction.IsProactiveReview = true
	}

	if err := deps.Store.CreateInteraction(interaction); err != nil {
		return nil, fmt.Errorf("applyInteraction: create interaction: %w", err)
	}

	// BKT update — error-type-aware.
	bktState := algorithms.BKTState{
		PMastery: cs.PMastery,
		PLearn:   cs.PLearn,
		PForget:  cs.PForget,
		PSlip:    cs.PSlip,
		PGuess:   cs.PGuess,
	}
	bktState = algorithms.BKTUpdateWithErrorType(bktState, input.Success, input.ErrorType)
	cs.PMastery = bktState.PMastery

	// FSRS ReviewCard.
	rating := algorithms.Good
	if !input.Success {
		rating = algorithms.Again
	} else if input.Confidence >= 0.9 {
		rating = algorithms.Easy
	} else if input.Confidence < 0.5 {
		rating = algorithms.Hard
	}

	var lastReview time.Time
	if cs.LastReview != nil {
		lastReview = *cs.LastReview
	}
	fsrsCard := algorithms.FSRSCard{
		Stability:     cs.Stability,
		Difficulty:    cs.Difficulty,
		ElapsedDays:   cs.ElapsedDays,
		ScheduledDays: cs.ScheduledDays,
		Reps:          cs.Reps,
		Lapses:        cs.Lapses,
		State:         algorithms.CardState(cs.CardState),
		LastReview:    lastReview,
	}
	fsrsCard = algorithms.ReviewCard(fsrsCard, rating, now)
	cs.Stability = fsrsCard.Stability
	cs.Difficulty = fsrsCard.Difficulty
	cs.ElapsedDays = fsrsCard.ElapsedDays
	cs.ScheduledDays = fsrsCard.ScheduledDays
	cs.Reps = fsrsCard.Reps
	cs.Lapses = fsrsCard.Lapses
	cs.CardState = string(fsrsCard.State)
	cs.LastReview = &now
	nextReview := now.Add(time.Duration(fsrsCard.ScheduledDays) * 24 * time.Hour)
	cs.NextReview = &nextReview

	// IRT UpdateTheta.
	item := algorithms.IRTItem{
		Difficulty:     algorithms.FSRSDifficultyToIRT(cs.Difficulty),
		Discrimination: 1.0,
	}
	cs.Theta = algorithms.IRTUpdateTheta(cs.Theta, []algorithms.IRTItem{item}, []bool{input.Success})

	// PFA Update.
	pfaState := algorithms.PFAState{
		Successes: cs.PFASuccesses,
		Failures:  cs.PFAFailures,
	}
	pfaState = algorithms.PFAUpdate(pfaState, input.Success)
	cs.PFASuccesses = pfaState.Successes
	cs.PFAFailures = pfaState.Failures

	// Persist updated concept state.
	if err := deps.Store.UpsertConceptState(cs); err != nil {
		return nil, fmt.Errorf("applyInteraction: upsert concept state: %w", err)
	}

	return cs, nil
}
