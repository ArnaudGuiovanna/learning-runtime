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
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel). Si fourni, les métriques d'autonomie computées sur les interactions et états sont restreintes à ce domaine. La tendance reste learner-wide (signal cross-session)."`
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

		// Domain filter (#95): if domain_id is supplied, restrict the
		// concept-keyed inputs (interactions, states) to that domain's
		// concept set. resolveDomain enforces learner ownership and
		// rejects archived/foreign IDs. Trend stays learner-wide
		// because it's computed from affect rows (session-keyed, not
		// concept-keyed) and represents a cross-session learner signal.
		if params.DomainID != "" {
			domain, err := resolveDomain(deps.Store, learnerID, params.DomainID)
			if err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
			conceptSet := make(map[string]bool, len(domain.Graph.Concepts))
			for _, c := range domain.Graph.Concepts {
				conceptSet[c] = true
			}
			interactions = filterInteractionsByConcepts(interactions, conceptSet)
			states = filterStatesByConcepts(states, conceptSet)
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
