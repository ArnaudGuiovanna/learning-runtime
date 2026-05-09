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
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine de contexte (optionnel) ; le miroir est calculé au niveau apprenant et n'est pas filtré par domaine"`
}

func registerGetMetacognitiveMirror(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_metacognitive_mirror",
		Description: "Retourne un message miroir factuel si un pattern de dépendance est consolidé sur les 7 derniers jours, sinon mirror=null. Outil de réflexion métacognitive à la demande. " +
			"Quand appeler : UNIQUEMENT hors du cycle d'activité — par exemple lors d'une demande explicite de bilan métacognitif, ou si l'apprenant interroge ses propres patterns d'apprentissage. " +
			"Quand NE PAS appeler : si get_next_activity a déjà été appelé dans le même tour, le miroir est déjà présent dans sa clé metacognitive_mirror — un second appel ici fait du travail dupliqué (même calcul, même file webhook dédupliquée par jour). " +
			"Précondition : aucune ; si aucun pattern n'est détecté, mirror=null est renvoyé sans erreur. " +
			"Retour : {mirror: <objet ou null>}.",
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
