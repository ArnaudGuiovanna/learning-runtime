// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"time"

	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RecordInteractionParams struct {
	Concept             string  `json:"concept" jsonschema:"Le concept concerné"`
	ActivityType        string  `json:"activity_type" jsonschema:"Type d'activité (RECALL_EXERCISE, NEW_CONCEPT, etc.)"`
	Success             bool    `json:"success" jsonschema:"L'exercice a été réussi"`
	ResponseTimeSeconds float64 `json:"response_time_seconds" jsonschema:"Temps de réponse en secondes"`
	Confidence          float64 `json:"confidence" jsonschema:"Confiance estimée entre 0 et 1"`
	ErrorType           string  `json:"error_type,omitempty" jsonschema:"Type d'erreur si échec: SYNTAX_ERROR, LOGIC_ERROR, KNOWLEDGE_GAP (optionnel)"`
	Notes               string  `json:"notes" jsonschema:"Notes optionnelles sur l'interaction"`
	DomainID            string  `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel)"`
	HintsRequested      int     `json:"hints_requested,omitempty" jsonschema:"Nombre d'indices demandés pendant l'échange (optionnel, défaut 0)"`
	SelfInitiated       bool    `json:"self_initiated,omitempty" jsonschema:"true si la session a démarré sans alerte webhook"`
	CalibrationID       string  `json:"calibration_id,omitempty" jsonschema:"ID de la prédiction de calibration associée (optionnel)"`
	MisconceptionType   string  `json:"misconception_type,omitempty" jsonschema:"Label libre de la misconception détectée (optionnel, ignoré si success=true)"`
	MisconceptionDetail string  `json:"misconception_detail,omitempty" jsonschema:"Description de la misconception en une phrase (optionnel)"`
}

func registerRecordInteraction(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_interaction",
		Description: "Enregistre le résultat d'un exercice et met à jour l'état cognitif de l'apprenant. Supporte error_type pour ajuster le BKT selon le type d'erreur.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params RecordInteractionParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("record_interaction: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.Concept == "" {
			r, _ := errorResult("concept is required")
			return r, nil, nil
		}

		// Resolve the active domain (honoring the optional domain_id) and
		// validate the concept against its concept list. Without this guard
		// the BKT/FSRS chain silently inserts orphan concept_states for
		// hallucinated or stale concept names — see issue #23.
		domain, err := resolveDomain(deps.Store, learnerID, params.DomainID)
		if err != nil || domain == nil {
			deps.Logger.Error("record_interaction: resolve domain", "err", err, "learner", learnerID)
			r, _ := errorResult("no active domain — call init_domain first")
			return r, nil, nil
		}
		if err := validateConceptInDomain(domain, params.Concept); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		cs, err := applyInteraction(deps, learnerID, interactionInput{
			Concept:             params.Concept,
			ActivityType:        params.ActivityType,
			Success:             params.Success,
			ResponseTimeSeconds: params.ResponseTimeSeconds,
			Confidence:          params.Confidence,
			ErrorType:           params.ErrorType,
			Notes:               params.Notes,
			HintsRequested:      params.HintsRequested,
			SelfInitiated:       params.SelfInitiated,
			CalibrationID:       params.CalibrationID,
			MisconceptionType:   params.MisconceptionType,
			MisconceptionDetail: params.MisconceptionDetail,
			DomainID:            domain.ID,
		}, time.Now().UTC())
		if err != nil {
			deps.Logger.Error("record_interaction: applyInteraction failed", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("failed to record interaction: %v", err))
			return r, nil, nil
		}

		// Update last active
		_ = deps.Store.UpdateLastActive(learnerID)

		deps.Logger.Info("interaction recorded",
			"learner", learnerID,
			"concept", params.Concept,
			"activity_type", params.ActivityType,
			"success", params.Success,
			"hints_requested", params.HintsRequested,
			"self_initiated", params.SelfInitiated,
			"new_mastery", cs.PMastery,
			"new_theta", cs.Theta,
			"reps", cs.Reps,
		)

		// Compute engagement signal
		engagementSignal := "stable"
		if params.Confidence >= 0.8 && params.Success {
			engagementSignal = "positive"
		} else if !params.Success && params.Confidence < 0.3 {
			engagementSignal = "declining"
		}

		// Compute cognitive signals from session patterns
		sessionInteractions, _ := deps.Store.GetSessionInteractions(learnerID)
		fatigueSignal, frustrationSignal := computeCognitiveSignals(sessionInteractions)

		nextReviewHours := float64(cs.ScheduledDays) * 24.0

		r, _ := jsonResult(map[string]interface{}{
			"updated":              true,
			"new_mastery":          cs.PMastery,
			"next_review_in_hours": nextReviewHours,
			"engagement_signal":    engagementSignal,
			"fatigue_signal":       fatigueSignal,
			"frustration_signal":   frustrationSignal,
		})
		return r, nil, nil
	})
}

// computeCognitiveSignals analyzes session interaction patterns for fatigue and frustration.
func computeCognitiveSignals(sessionInteractions []*models.Interaction) (fatigue string, frustration string) {
	fatigue = "none"
	frustration = "none"

	if len(sessionInteractions) < 3 {
		return
	}

	// Fatigue: declining accuracy + increasing response time in last N interactions
	// Look at the most recent 5 interactions (they're sorted newest-first)
	window := sessionInteractions
	if len(window) > 5 {
		window = window[:5]
	}

	recentSuccesses := 0
	recentTotalTime := 0
	for _, i := range window {
		if i.Success {
			recentSuccesses++
		}
		recentTotalTime += i.ResponseTime
	}
	recentRate := float64(recentSuccesses) / float64(len(window))
	avgRecentTime := float64(recentTotalTime) / float64(len(window))

	// Compare with earlier interactions if available
	if len(sessionInteractions) >= 6 {
		earlier := sessionInteractions[len(window):]
		if len(earlier) > 5 {
			earlier = earlier[:5]
		}
		earlySuccesses := 0
		earlyTotalTime := 0
		for _, i := range earlier {
			if i.Success {
				earlySuccesses++
			}
			earlyTotalTime += i.ResponseTime
		}
		earlyRate := float64(earlySuccesses) / float64(len(earlier))
		avgEarlyTime := float64(earlyTotalTime) / float64(len(earlier))

		// Fatigue: accuracy drops AND response time increases
		if recentRate < earlyRate-0.2 && avgRecentTime > avgEarlyTime*1.3 {
			fatigue = "high"
		} else if recentRate < earlyRate-0.1 || avgRecentTime > avgEarlyTime*1.2 {
			fatigue = "moderate"
		}
	}

	// Frustration: consecutive failures + low confidence
	consecutiveFailures := 0
	lowConfidenceCount := 0
	for _, i := range window {
		if !i.Success {
			consecutiveFailures++
			if i.Confidence < 0.3 {
				lowConfidenceCount++
			}
		} else {
			break
		}
	}

	if consecutiveFailures >= 3 && lowConfidenceCount >= 2 {
		frustration = "high"
	} else if consecutiveFailures >= 2 && lowConfidenceCount >= 1 {
		frustration = "moderate"
	}

	return
}
