// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"fmt"
	"math"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"
)

// PhaseObservables is the *snapshot* of state the FSM evaluates
// against. Pre-fetched by the orchestrator from the store; the FSM
// itself is pure (no DB, no time, no logging).
type PhaseObservables struct {
	// MeanEntropy is the current mean binary entropy of P(L) across
	// all concepts in the domain graph. Used for DIAGNOSTIC →
	// INSTRUCTION transition (compared against PhaseEntryEntropy).
	MeanEntropy float64

	// PhaseEntryEntropy is the entropy snapshot taken at the moment
	// the phase was last set to DIAGNOSTIC. Zero (or NaN) means "no
	// snapshot available" — the relative criterion does not fire
	// and only NDiagnosticMax governs DIAGNOSTIC exit.
	PhaseEntryEntropy float64

	// DiagnosticItemsCount is the number of interactions on the
	// domain since PhaseChangedAt, derived by the orchestrator via
	// db.CountInteractionsSince. Used for the NDiagnosticMax cap.
	DiagnosticItemsCount int

	// MasteredGoalRelevant is the count of concepts that are
	// *goal-relevant* (per cfg.GoalRelevantCutoff) AND have
	// PMastery >= MasteryBKT(). Used for INSTRUCTION → MAINTENANCE.
	MasteredGoalRelevant int

	// TotalGoalRelevant is the total count of goal-relevant concepts
	// (denominator of the mastery check). Equality with
	// MasteredGoalRelevant fires the INSTRUCTION → MAINTENANCE
	// transition.
	TotalGoalRelevant int

	// GoalRelevantBelowRetention is true when at least one
	// goal-relevant concept has FSRS retrievability strictly below
	// cfg.RetentionRecallThreshold. Triggers MAINTENANCE →
	// INSTRUCTION.
	GoalRelevantBelowRetention bool
}

// PhaseEvaluation is the FSM output: from-state, to-state, whether a
// transition occurred, plus a human-readable rationale used in audit
// logs and the E2E artifact.
type PhaseEvaluation struct {
	From         models.Phase
	To           models.Phase
	Transitioned bool
	Rationale    string
}

// EvaluatePhase computes the next phase given the current phase and
// a snapshot of observables. Pure function — same input always
// yields same output.
//
// Transitions (cf. docs/regulation-design/02-phase-controller.md §3) :
//
//   - DIAGNOSTIC → INSTRUCTION :
//     (entropy_entry - entropy_now >= cfg.DeltaHThreshold)
//     OR (n_items >= cfg.NDiagnosticMax)
//
//   - INSTRUCTION → MAINTENANCE :
//     all goal-relevant concepts have PMastery >= MasteryBKT()
//     (encoded via MasteredGoalRelevant == TotalGoalRelevant > 0)
//
//   - MAINTENANCE → INSTRUCTION :
//     at least one goal-relevant concept has retention <
//     cfg.RetentionRecallThreshold
//
// Unknown current phase → no transition, returns the unknown phase
// echoed back with Transitioned=false. The caller (Orchestrate) is
// responsible for the *legacy* fallback (NULL phase → INSTRUCTION) —
// the FSM itself only processes recognised phases.
func EvaluatePhase(current models.Phase, obs PhaseObservables, cfg PhaseConfig) PhaseEvaluation {
	switch current {
	case models.PhaseDiagnostic:
		// Relative entropy criterion: only fire when we have a valid
		// snapshot AND the reduction reaches the threshold.
		hasSnapshot := obs.PhaseEntryEntropy > 0 && !math.IsNaN(obs.PhaseEntryEntropy)
		entropyDelta := obs.PhaseEntryEntropy - obs.MeanEntropy
		entropyExit := hasSnapshot && entropyDelta >= cfg.DeltaHThreshold
		countExit := obs.DiagnosticItemsCount >= cfg.NDiagnosticMax

		if entropyExit {
			return PhaseEvaluation{
				From: current, To: models.PhaseInstruction, Transitioned: true,
				Rationale: fmt.Sprintf(
					"DIAGNOSTIC→INSTRUCTION: entropy reduction reached (%.3f >= %.3f bits)",
					entropyDelta, cfg.DeltaHThreshold),
			}
		}
		if countExit {
			return PhaseEvaluation{
				From: current, To: models.PhaseInstruction, Transitioned: true,
				Rationale: fmt.Sprintf(
					"DIAGNOSTIC→INSTRUCTION: N_max reached (%d >= %d items)",
					obs.DiagnosticItemsCount, cfg.NDiagnosticMax),
			}
		}
		return PhaseEvaluation{
			From: current, To: current, Transitioned: false,
			Rationale: fmt.Sprintf(
				"DIAGNOSTIC: delta=%.3f bits (threshold %.3f), items=%d/%d — stay",
				entropyDelta, cfg.DeltaHThreshold,
				obs.DiagnosticItemsCount, cfg.NDiagnosticMax),
		}

	case models.PhaseInstruction:
		// All goal-relevant concepts mastered.
		if obs.TotalGoalRelevant > 0 && obs.MasteredGoalRelevant == obs.TotalGoalRelevant {
			return PhaseEvaluation{
				From: current, To: models.PhaseMaintenance, Transitioned: true,
				Rationale: fmt.Sprintf(
					"INSTRUCTION→MAINTENANCE: %d/%d goal-relevant concepts mastered",
					obs.MasteredGoalRelevant, obs.TotalGoalRelevant),
			}
		}
		return PhaseEvaluation{
			From: current, To: current, Transitioned: false,
			Rationale: fmt.Sprintf(
				"INSTRUCTION: %d/%d goal-relevant concepts mastered — stay",
				obs.MasteredGoalRelevant, obs.TotalGoalRelevant),
		}

	case models.PhaseMaintenance:
		if obs.GoalRelevantBelowRetention {
			return PhaseEvaluation{
				From: current, To: models.PhaseInstruction, Transitioned: true,
				Rationale: "MAINTENANCE→INSTRUCTION: one goal-relevant concept below the retention threshold",
			}
		}
		return PhaseEvaluation{
			From: current, To: current, Transitioned: false,
			Rationale: "MAINTENANCE: retention OK on all goal-relevants — stay",
		}

	default:
		// Unrecognised phase: no transition. The orchestrator decides
		// the fallback (typically INSTRUCTION for rows pre-flagged with
		// phase NULL — already mapped to an empty string).
		return PhaseEvaluation{
			From: current, To: current, Transitioned: false,
			Rationale: fmt.Sprintf("unrecognised phase %q — no transition", string(current)),
		}
	}
}

// MeanBinaryEntropyOverGraph computes the mean of H(P(L_c)) over all
// concepts in graph. Concepts absent from the states map are treated
// as having P(L) = 0 → H = 0 (uninformative — they contribute zero
// entropy to the mean, biasing it down). NaN P(L) values are skipped
// (defensive). Returns 0 for an empty graph.
//
// Used by the orchestrator to feed PhaseObservables.MeanEntropy and
// to snapshot PhaseEntryEntropy on transition INTO DIAGNOSTIC.
func MeanBinaryEntropyOverGraph(graph models.KnowledgeSpace, states map[string]*models.ConceptState) float64 {
	if len(graph.Concepts) == 0 {
		return 0
	}
	var sum float64
	var n int
	for _, c := range graph.Concepts {
		cs := states[c]
		var p float64
		if cs != nil && !math.IsNaN(cs.PMastery) {
			p = cs.PMastery
		}
		sum += algorithms.BinaryEntropy(p)
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
