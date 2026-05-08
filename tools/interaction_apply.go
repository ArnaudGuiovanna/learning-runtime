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
// interaction and drive the BKT/FSRS/IRT update chain.  Fields not
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
// cognitive state (BKT, FSRS, IRT) for the concept.  Returns the
// resulting ConceptState (post-update) and an error if any persistence
// step failed.
//
// The BKT → FSRS → IRT update chain is *non-commutative* on
// `cs.Difficulty` (and in spirit on `cs.Reps` / `cs.Stability` /
// `cs.PMastery`): IRT consumes the FSRS difficulty to compute the θ
// step, but FSRS itself rewrites that field. Reading `cs.*` directly
// after running each step would silently mix prior- and post-update
// values across the chain (issue #53). To keep the chain
// order-independent we snapshot the read-only prior values at the top,
// run all three updates against the snapshot, and write the merged
// result back to `cs` exactly once at the end.
//
// Note: PFA is intentionally NOT persisted on the concept state. The
// PLATEAU alert in engine/alert.go recomputes a fresh PFAState from the
// recent-interactions list on each call, so storing rolling counts
// per-concept added schema weight without any reader (issue #55).
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

	// ── Snapshot read-only prior state ──────────────────────────────
	// All downstream algorithm steps read from this snapshot, never
	// from `cs` directly, to keep the BKT → FSRS → IRT chain
	// commutative. See doc comment above and issue #53.
	priorPMastery := cs.PMastery
	priorPLearn := cs.PLearn
	priorPForget := cs.PForget
	priorPSlip := cs.PSlip
	priorPGuess := cs.PGuess
	priorStability := cs.Stability
	priorDifficulty := cs.Difficulty
	priorElapsedDays := cs.ElapsedDays
	priorScheduledDays := cs.ScheduledDays
	priorReps := cs.Reps
	priorLapses := cs.Lapses
	priorCardState := cs.CardState
	priorTheta := cs.Theta
	var priorLastReview time.Time
	if cs.LastReview != nil {
		priorLastReview = *cs.LastReview
	}
	priorNextReview := cs.NextReview

	// ── BKT update — non-canonical, project-specific heuristic that
	// ramps slip/guess by error_type. Computed up front (it only reads
	// from the prior snapshot) so the (slipUsed, guessUsed) values can
	// be persisted on the interaction row below for deterministic
	// replay (issue #51 / #8). The PMastery write-back to `cs` happens
	// in the merged-result block at the end of the function.
	bktState := algorithms.BKTState{
		PMastery: priorPMastery,
		PLearn:   priorPLearn,
		PForget:  priorPForget,
		PSlip:    priorPSlip,
		PGuess:   priorPGuess,
	}
	bktState, slipUsed, guessUsed := algorithms.BKTUpdateHeuristicSlipByErrorType(bktState, input.Success, input.ErrorType)

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
		BKTSlip:        &slipUsed,
		BKTGuess:       &guessUsed,
		CreatedAt:      now,
	}

	// Misconception fields — only stored on failures.
	if !input.Success && input.MisconceptionType != "" {
		interaction.MisconceptionType = input.MisconceptionType
		interaction.MisconceptionDetail = input.MisconceptionDetail
	}

	// Proactive review flag — derived from the prior schedule.
	if priorNextReview != nil && priorNextReview.After(now) && priorCardState != "new" {
		interaction.IsProactiveReview = true
	}

	if err := deps.Store.CreateInteraction(interaction); err != nil {
		return nil, fmt.Errorf("applyInteraction: create interaction: %w", err)
	}

	// ── FSRS update — reads from prior snapshot. ───────────────────
	rating := algorithms.Good
	if !input.Success {
		rating = algorithms.Again
	} else if input.Confidence >= 0.9 {
		rating = algorithms.Easy
	} else if input.Confidence < 0.5 {
		rating = algorithms.Hard
	}

	fsrsCard := algorithms.FSRSCard{
		Stability:     priorStability,
		Difficulty:    priorDifficulty,
		ElapsedDays:   priorElapsedDays,
		ScheduledDays: priorScheduledDays,
		Reps:          priorReps,
		Lapses:        priorLapses,
		State:         algorithms.CardState(priorCardState),
		LastReview:    priorLastReview,
	}
	fsrsCard = algorithms.ReviewCard(fsrsCard, rating, now)

	// ── IRT update — reads PRIOR difficulty from the snapshot, not
	// the FSRS-rewritten value. This is the issue #53 fix: previously
	// IRT consumed `cs.Difficulty` after FSRS had overwritten it. ──
	item := algorithms.IRTItem{
		Difficulty:     algorithms.FSRSDifficultyToIRT(priorDifficulty),
		Discrimination: 1.0,
	}
	newTheta := algorithms.IRTUpdateTheta(priorTheta, []algorithms.IRTItem{item}, []bool{input.Success})

	// ── Single write-back of the merged result. ────────────────────
	cs.PMastery = bktState.PMastery
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
	cs.Theta = newTheta

	// Persist updated concept state.
	if err := deps.Store.UpsertConceptState(cs); err != nil {
		return nil, fmt.Errorf("applyInteraction: upsert concept state: %w", err)
	}

	return cs, nil
}
