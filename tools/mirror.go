// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"time"

	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetMetacognitiveMirrorParams struct {
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel). Vide = analyse apprenant-large; non-vide = restreint aux concepts de ce domaine (issue #95)."`
}

func registerGetMetacognitiveMirror(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_metacognitive_mirror",
		Description: "Retourne un message miroir factuel si un pattern de dépendance est consolidé. Null sinon.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetMetacognitiveMirrorParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("get_metacognitive_mirror: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		since := time.Now().UTC().Add(-7 * 24 * time.Hour)
		interactions, _ := deps.Store.GetInteractionsSince(learnerID, since)
		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		calibBias, _ := deps.Store.GetCalibrationBias(learnerID, 20)
		affects, _ := deps.Store.GetRecentAffectStates(learnerID, 10)

		// Issue #95: when an explicit domain_id is given, scope the
		// concept-keyed inputs (interactions + states) to that domain's
		// concept set. An empty domain_id keeps the existing learner-wide
		// behaviour so callers that pass nothing don't see a regression.
		// Affects + autonomy scores are session-keyed (not concept-keyed),
		// so leaving them learner-wide is intentional — they drive the
		// dependency_increasing trend, which is a cross-session signal.
		if params.DomainID != "" {
			domain, domainErr := resolveDomain(deps.Store, learnerID, params.DomainID)
			if domainErr != nil || domain == nil {
				deps.Logger.Error("get_metacognitive_mirror: domain not found", "err", domainErr, "learner", learnerID, "domain_id", params.DomainID)
				r, _ := errorResult("domain not found")
				return r, nil, nil
			}
			domainConcepts := make(map[string]bool, len(domain.Graph.Concepts))
			for _, c := range domain.Graph.Concepts {
				domainConcepts[c] = true
			}
			interactions = filterInteractionsByConcepts(interactions, domainConcepts)
			states = filterStatesByConcepts(states, domainConcepts)
		}

		var autonomyScores []float64
		for _, a := range affects {
			autonomyScores = append(autonomyScores, a.AutonomyScore)
		}

		sessionCount := len(engine.GroupIntoSessionsExported(interactions, 2*time.Hour))

		mirror := engine.DetectMirrorPattern(engine.MirrorInput{
			Interactions:    interactions,
			ConceptStates:   states,
			AutonomyScores:  autonomyScores,
			CalibrationBias: calibBias,
			SessionCount:    sessionCount,
		})

		if mirror == nil {
			r, _ := jsonResult(map[string]interface{}{
				"mirror": nil,
			})
			return r, nil, nil
		}

		// Persist & enqueue for proactive push (#59). Best-effort: a queue
		// failure must not block the in-session pull response — Claude can
		// still surface the mirror text even if the webhook lane is offline.
		if _, _, err := engine.EnqueueMirrorWebhook(deps.Store, learnerID, mirror, time.Now().UTC()); err != nil {
			deps.Logger.Warn("get_metacognitive_mirror: enqueue failed", "err", err, "learner", learnerID)
		}

		r, _ := jsonResult(map[string]interface{}{
			"mirror": mirror,
		})
		return r, nil, nil
	})
}
