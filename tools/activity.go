// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"tutor-mcp/algorithms"
	"tutor-mcp/engine"
	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetNextActivityParams struct {
	DomainID string `json:"domain_id,omitempty" jsonschema:"target domain ID; if absent, the learner's last active domain is used"`
}

func registerGetNextActivity(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_next_activity",
		Description: "Determine the next optimal activity for the learner and aggregate all routing context: metacognitive_mirror, tutor_mode, motivation_brief, active misconceptions. Accounts for the current session to avoid repeating the same concept. " +
			"When to call: this is the main tool of the learning cycle; it already includes alert-aware routing, metacognitive_mirror, tutor_mode and motivation_brief. " +
			"When NOT to call: if another tool just returned needs_domain_setup=true (call init_domain first); do not call get_pending_alerts or get_metacognitive_mirror in the same turn unless the learner explicitly asks for those raw views. " +
			"Precondition: a domain must exist; otherwise needs_domain_setup=true is returned with a setup_domain activity. " +
			"Returns: {needs_domain_setup, domain_id, activity, session_concepts_done, metacognitive_mirror, tutor_mode, active_misconceptions, known_misconception_types, motivation_brief, mastery_evidence, mastery_uncertainty, transfer_profile, rasch_elo_calibration}.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetNextActivityParams) (*mcp.CallToolResult, any, error) {
		totalStart := time.Now()
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("get_next_activity: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// Check if domain exists
		domainStart := time.Now()
		domain, err := resolveDomain(deps.Store, learnerID, params.DomainID)
		domainMs := time.Since(domainStart).Milliseconds()
		if err != nil || domain == nil {
			r, _ := jsonResult(map[string]interface{}{
				"needs_domain_setup": true,
				"activity": models.Activity{
					Type:         models.ActivitySetupDomain,
					Rationale:    "no domain configured",
					PromptForLLM: "The learner has no domain yet. Analyse their objective, break it down into concepts, and call init_domain().",
				},
			})
			return r, nil, nil
		}

		prefetchStart := time.Now()
		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		interactions, _ := deps.Store.GetRecentInteractionsByLearner(learnerID, engine.DefaultRecentInteractionsWindow)
		sessionStart, _ := deps.Store.GetSessionStart(learnerID)

		// Get session interactions to track what was already practiced
		sessionInteractions, _ := deps.Store.GetSessionInteractions(learnerID)

		// Filter states to only those in the current domain
		domainConcepts := make(map[string]bool)
		for _, c := range domain.Graph.Concepts {
			domainConcepts[c] = true
		}
		var domainStates []*models.ConceptState
		for _, cs := range states {
			if domainConcepts[cs.Concept] {
				domainStates = append(domainStates, cs)
			}
		}

		// Filter interactions to domain concepts
		var domainInteractions []*models.Interaction
		for _, i := range interactions {
			if domainConcepts[i.Concept] {
				domainInteractions = append(domainInteractions, i)
			}
		}

		// Compute alerts (only for domain concepts)
		alerts := engine.ComputeAlerts(domainStates, domainInteractions, sessionStart)

		// Mastery map for downstream consumers (motivation brief, etc.).
		mastery := make(map[string]float64)
		for _, cs := range domainStates {
			mastery[cs.Concept] = cs.PMastery
		}

		// Build set of concepts already practiced in this session
		sessionConcepts := make(map[string]int)
		for _, i := range sessionInteractions {
			if domainConcepts[i.Concept] {
				sessionConcepts[i.Concept]++
			}
		}
		prefetchMs := time.Since(prefetchStart).Milliseconds()

		// Route to next activity through the regulation pipeline.
		// OrchestrateWithPhase returns the post-orchestrate phase so we
		// can audit-log it without re-reading the domain row (perf #91).
		orchestratorStart := time.Now()
		activity, orchPhase, orchErr := engine.OrchestrateWithPhase(deps.Store, engine.OrchestratorInput{
			LearnerID: learnerID,
			DomainID:  domain.ID,
			Now:       time.Now().UTC(),
			Config:    engine.NewDefaultPhaseConfig(),
		})
		orchestratorMs := time.Since(orchestratorStart).Milliseconds()
		if orchErr != nil {
			deps.Logger.Error("orchestrator failed", "err", orchErr, "learner", learnerID, "domain", domain.ID)
			r, _ := errorResult("could not compute next activity")
			return r, nil, nil
		}

		// Pipeline decision audit — one line per get_next_activity call.
		// Phase comes straight from the orchestrator (any FSM transition
		// or NoFringe fallback already applied).
		//
		// Divergence note (perf #91): the logged phase reflects the
		// orchestrator's in-memory currentPhase at the moment the
		// activity was returned. On the rare path where
		// store.UpdateDomainPhase fails inside the orchestrator (logged
		// at ERROR there — see engine/orchestrator.go around the
		// "failed to persist phase transition" / "failed to persist
		// NoFringe fallback transition" branches), the persisted DB
		// phase may lag this logged value by one transition. That's an
		// accepted trade-off for an audit log: the goal here is to
		// record the *phase decision* tied to the activity that was
		// actually returned, not the storage outcome. Storage failures
		// are surfaced via their own ERROR log line in the orchestrator
		// and don't block the live activity.
		loggedPhase := "INSTRUCTION" // orchestrator's NULL fallback
		if orchPhase != "" {
			loggedPhase = string(orchPhase)
		}
		deps.Logger.Info("pipeline decision",
			"learner", learnerID,
			"domain", domain.ID,
			"phase", loggedPhase,
			"activity_type", activity.Type,
			"concept", activity.Concept,
			"rationale", activity.Rationale,
		)

		// Metacognitive mirror
		mirrorStart := time.Now()
		since := time.Now().UTC().Add(-7 * 24 * time.Hour)
		allInteractions, _ := deps.Store.GetInteractionsSince(learnerID, since)
		calibBias, _ := deps.Store.GetCalibrationBias(learnerID, 20)
		affects, _ := deps.Store.GetRecentAffectStates(learnerID, 10)

		var autonomyScores []float64
		for _, a := range affects {
			autonomyScores = append(autonomyScores, a.AutonomyScore)
		}
		mirrorSessionCount := len(engine.GroupIntoSessionsExported(allInteractions, 2*time.Hour))

		mirror := engine.DetectMirrorPattern(engine.MirrorInput{
			Interactions:    allInteractions,
			ConceptStates:   domainStates,
			AutonomyScores:  autonomyScores,
			CalibrationBias: calibBias,
			SessionCount:    mirrorSessionCount,
		})

		// Persist & enqueue the mirror so it can be pushed proactively via
		// the webhook queue (#59). Per-day dedup lives in EnqueueMirrorWebhook
		// so a learner who hits get_next_activity multiple times in a day
		// only sees one queued nudge.
		if mirror != nil {
			if _, _, err := engine.EnqueueMirrorWebhook(deps.Store, learnerID, mirror, time.Now().UTC()); err != nil {
				deps.Logger.Warn("get_next_activity: mirror enqueue failed", "err", err, "learner", learnerID)
			}
		}
		mirrorMs := time.Since(mirrorStart).Milliseconds()

		// Tutor mode
		var currentAffect *models.AffectState
		if len(affects) > 0 {
			currentAffect = affects[0]
		}
		tutorMode := engine.ComputeTutorMode(currentAffect, alerts)

		// Apply tutor_mode adjustments to activity difficulty
		switch tutorMode {
		case "lighter":
			activity.DifficultyTarget *= 0.7
			activity.EstimatedMinutes = int(float64(activity.EstimatedMinutes) * 0.6)
			if activity.EstimatedMinutes < 5 {
				activity.EstimatedMinutes = 5
			}
		case "scaffolding":
			activity.DifficultyTarget *= 0.75
		}

		// Apply calibration bias adjustment
		// Positive bias = over-estimates → increase difficulty
		// Negative bias = under-estimates → decrease difficulty
		if calibBias != 0 {
			activity.DifficultyTarget += calibBias * 0.1
			if activity.DifficultyTarget < 0.3 {
				activity.DifficultyTarget = 0.3
			}
			if activity.DifficultyTarget > 0.85 {
				activity.DifficultyTarget = 0.85
			}
		}

		// Misconception enrichment for selected concept
		enrichmentStart := time.Now()
		var activeMisconceptions any = []any{}
		var knownMisconceptionTypes any = []string{}

		if activity.Concept != "" {
			if active, err := deps.Store.GetActiveMisconceptions(learnerID, activity.Concept); err == nil && len(active) > 0 {
				activeMisconceptions = active

				// Inject misconceptions into prompt
				misconceptionPrompt := fmt.Sprintf("\nATTENTION : l'apprenant a %d misconception(s) active(s) : ", len(active))
				for i, m := range active {
					if i > 0 {
						misconceptionPrompt += " ; "
					}
					misconceptionPrompt += m.MisconceptionType
					if m.LastErrorDetail != "" {
						misconceptionPrompt += " - " + m.LastErrorDetail
					}
				}
				misconceptionPrompt += ". Target these confusions in your explanation and exercise. Do not explicitly mention the misconceptions - design the exercise so the learner confronts them naturally."
				activity.PromptForLLM += misconceptionPrompt
			}

			if types, err := deps.Store.GetDistinctMisconceptionTypes(learnerID, activity.Concept); err == nil && len(types) > 0 {
				knownMisconceptionTypes = types
			}
		}
		enrichmentMs := time.Since(enrichmentStart).Milliseconds()

		// Motivation layer — compose a context-adaptive brief for this activity.
		// Detect plateau active on the chosen concept from the alerts list.
		motivationStart := time.Now()
		plateauActive := false
		for _, a := range alerts {
			if a.Type == models.AlertPlateau && a.Concept == activity.Concept {
				plateauActive = true
				break
			}
		}
		motivationEngine := engine.NewMotivationEngine(deps.Store)
		motivationBrief, _ := motivationEngine.Build(
			learnerID, domain, activity.Concept, activity.Type,
			plateauActive, len(sessionConcepts),
		)
		motivationMs := time.Since(motivationStart).Milliseconds()

		// [6] FadeController — post-decision module gated on
		// REGULATION_FADE=on (default OFF). Maps autonomy
		// score+trend to handover params (verbosity, webhook
		// cadence, ZPD aggressiveness, proactive review). When
		// the flag is OFF, the result JSON is byte-identical to
		// the pre-fade behaviour: no fade_params key, no mutation
		// of motivation_brief. See
		// docs/regulation-design/06-fade-controller.md.
		extra := map[string]any{}
		if regulationFadeEnabled() {
			autonomyMetrics := engine.ComputeAutonomyMetrics(engine.AutonomyInput{
				Interactions:    allInteractions,
				ConceptStates:   domainStates,
				CalibrationBias: calibBias,
				SessionGap:      2 * time.Hour,
			})
			trend := engine.AutonomyTrend(engine.ComputeAutonomyTrendExported(autonomyScores))
			fadeParams := engine.Decide(autonomyMetrics.Score, trend)
			motivationBrief = applyFadeToMotivation(motivationBrief, fadeParams.HintLevel)
			extra["fade_params"] = fadeParams
			extra["autonomy_score"] = autonomyMetrics.Score
			extra["autonomy_trend"] = string(trend)
		}

		diagnosticsStart := time.Now()
		var masteryEvidence any = map[string]any{}
		var masteryUncertainty any = map[string]any{}
		var transferProfile any = map[string]any{}
		var raschEloCalibration any = map[string]any{}
		if activity.Concept != "" {
			var selectedState *models.ConceptState
			for _, cs := range domainStates {
				if cs.Concept == activity.Concept {
					selectedState = cs
					break
				}
			}
			conceptInteractions, err := deps.Store.GetRecentInteractions(learnerID, activity.Concept, 50)
			if err != nil {
				deps.Logger.Warn("get_next_activity: mastery diagnostics fetch failed", "err", err, "learner", learnerID, "concept", activity.Concept)
			} else {
				conceptInteractions = filterInteractionsByDomainID(conceptInteractions, domain.ID)
				now := time.Now().UTC()
				profile := engine.BuildEvidenceProfile(learnerID, activity.Concept, conceptInteractions, now)
				masteryEvidence = map[string]any{
					"profile": profile,
					"quality": engine.MasteryEvidenceQuality(profile),
				}
				masteryUncertainty = engine.ComputeMasteryUncertainty(selectedState, conceptInteractions, engine.MasteryEvidenceProfile{Now: now})
			}
			if selectedState != nil {
				raschState := algorithms.NewRaschEloState(selectedState.Theta, algorithms.FSRSDifficultyToIRT(selectedState.Difficulty))
				raschEloCalibration = raschEloStateSnapshot(raschState)
			}
			if transferRecords, err := deps.Store.GetTransferScores(learnerID, activity.Concept); err != nil {
				deps.Logger.Warn("get_next_activity: transfer diagnostics fetch failed", "err", err, "learner", learnerID, "concept", activity.Concept)
			} else {
				transferProfile = engine.BuildTransferProfile(activity.Concept, transferRecords)
			}
		}
		diagnosticsMs := time.Since(diagnosticsStart).Milliseconds()

		out := map[string]any{
			"needs_domain_setup":        false,
			"domain_id":                 domain.ID,
			"activity":                  activity,
			"session_concepts_done":     len(sessionConcepts),
			"metacognitive_mirror":      mirror,
			"tutor_mode":                tutorMode,
			"active_misconceptions":     activeMisconceptions,
			"known_misconception_types": knownMisconceptionTypes,
			"motivation_brief":          motivationBrief,
			"mastery_evidence":          masteryEvidence,
			"mastery_uncertainty":       masteryUncertainty,
			"transfer_profile":          transferProfile,
			"rasch_elo_calibration":     raschEloCalibration,
		}
		for k, v := range extra {
			out[k] = v
		}
		jsonStart := time.Now()
		r, _ := jsonResult(out)
		jsonMs := time.Since(jsonStart).Milliseconds()
		if deps.Logger.Enabled(ctx, slog.LevelDebug) {
			deps.Logger.Debug("get_next_activity timings",
				"learner", learnerID,
				"domain", domain.ID,
				"concepts", len(domain.Graph.Concepts),
				"domain_ms", domainMs,
				"prefetch_ms", prefetchMs,
				"orchestrator_ms", orchestratorMs,
				"mirror_ms", mirrorMs,
				"enrichment_ms", enrichmentMs,
				"motivation_ms", motivationMs,
				"diagnostics_ms", diagnosticsMs,
				"json_ms", jsonMs,
				"total_ms", time.Since(totalStart).Milliseconds(),
			)
		}
		return r, nil, nil
	})
}

// applyFadeToMotivation modulates a MotivationBrief based on the fade
// HintLevel. The contract:
//
//   - HintLevelFull    : brief returned unchanged (legacy behaviour).
//   - HintLevelPartial : Instruction is collapsed to a one-line
//     concise form; structured fields (ValueFraming, ProgressDelta,
//     etc.) are preserved so the LLM still has the context to weave
//     in if it chooses, but the explicit phrasing guidance is shorter.
//   - HintLevelNone    : Kind is cleared and Instruction is emptied.
//     Per the system prompt, kind == "" means "no motivational
//     preamble". Structured fields are also cleared to make the
//     suppression unambiguous on the wire.
//
// Returns brief unchanged if it's nil or already silent (kind == "").
func applyFadeToMotivation(brief *models.MotivationBrief, level engine.HintLevel) *models.MotivationBrief {
	if brief == nil {
		return brief
	}
	switch level {
	case engine.HintLevelNone:
		return &models.MotivationBrief{Kind: "", Instruction: ""}
	case engine.HintLevelPartial:
		if brief.Kind == "" {
			return brief
		}
		// Replace the verbose tutor-direction Instruction with a
		// terse one-liner. The kind + structured fields stay so
		// downstream context is preserved.
		clone := *brief
		clone.Instruction = "Bref. Reste minimal."
		return &clone
	default: // HintLevelFull
		return brief
	}
}
