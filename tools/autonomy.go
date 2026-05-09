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

type GetAutonomyMetricsParams struct {
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel). Vide = score apprenant-large; non-vide = restreint aux concepts de ce domaine (issue #95). La tendance reste cross-session."`
}

func registerGetAutonomyMetrics(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_autonomy_metrics",
		Description: "Score d'autonomie courant avec ses 4 composantes et la tendance. Consultable par l'apprenant et le système.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetAutonomyMetricsParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("get_autonomy_metrics: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		since := time.Now().UTC().Add(-30 * 24 * time.Hour)
		interactions, _ := deps.Store.GetInteractionsSince(learnerID, since)
		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		calibBias, _ := deps.Store.GetCalibrationBias(learnerID, 20)

		// Issue #95: when an explicit domain_id is given, scope the
		// concept-keyed inputs (interactions + states) to that domain's
		// concept set. The affects-driven Trend below is session-keyed
		// (not concept-keyed) so it stays learner-wide — that's the
		// intentional cross-session signal documented in the schema.
		if params.DomainID != "" {
			domain, domainErr := resolveDomain(deps.Store, learnerID, params.DomainID)
			if domainErr != nil || domain == nil {
				deps.Logger.Error("get_autonomy_metrics: domain not found", "err", domainErr, "learner", learnerID, "domain_id", params.DomainID)
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

		metrics := engine.ComputeAutonomyMetrics(engine.AutonomyInput{
			Interactions:    interactions,
			ConceptStates:   states,
			CalibrationBias: calibBias,
			SessionGap:      2 * time.Hour,
		})

		affects, _ := deps.Store.GetRecentAffectStates(learnerID, 10)
		var scores []float64
		for _, a := range affects {
			scores = append(scores, a.AutonomyScore)
		}
		metrics.Trend = engine.ComputeAutonomyTrendExported(scores)

		r, _ := jsonResult(metrics)
		return r, nil, nil
	})
}
