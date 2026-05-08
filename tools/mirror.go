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
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel)"`
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
