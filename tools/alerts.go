// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"

	"tutor-mcp/engine"
	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetPendingAlertsParams struct {
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel, utilisé le dernier domaine si absent)"`
}

func registerGetPendingAlerts(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_pending_alerts",
		Description: "Récupère les alertes en attente pour l'apprenant. Appeler en premier à chaque réponse.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetPendingAlertsParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("get_pending_alerts: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		interactions, _ := deps.Store.GetRecentInteractionsByLearner(learnerID, engine.DefaultRecentInteractionsWindow)
		sessionStart, _ := deps.Store.GetSessionStart(learnerID)

		// Resolve which domain(s) constrain this alert computation. The
		// README contract for the Alert Engine is that orphan concept
		// history (states/interactions on concepts no longer in any
		// active domain — e.g. survivors of a deleted domain) must NEVER
		// surface as alerts.
		if params.DomainID != "" {
			// Single-domain branch: explicit domain_id given, scope to
			// that domain's concepts (or refuse if the lookup fails or
			// the domain doesn't belong to this learner).
			domain, domainErr := resolveDomain(deps.Store, learnerID, params.DomainID)
			if domainErr != nil || domain == nil {
				deps.Logger.Error("get_pending_alerts: domain not found", "err", domainErr, "learner", learnerID, "domain_id", params.DomainID)
				r, _ := errorResult("domain not found")
				return r, nil, nil
			}
			domainConcepts := make(map[string]bool, len(domain.Graph.Concepts))
			for _, c := range domain.Graph.Concepts {
				domainConcepts[c] = true
			}
			states = filterStatesByConcepts(states, domainConcepts)
			interactions = filterInteractionsByConcepts(interactions, domainConcepts)
		} else {
			// No domain_id given: compute alerts over the union of
			// concepts across all non-archived domains. If the learner
			// has zero active domains, return a clean empty payload with
			// needs_domain_setup so the LLM can self-correct.
			activeDomains, _ := deps.Store.GetDomainsByLearner(learnerID, false)
			if len(activeDomains) == 0 {
				r, _ := jsonResult(map[string]interface{}{
					"alerts":             []models.Alert{},
					"has_critical":       false,
					"needs_domain_setup": true,
				})
				return r, nil, nil
			}
			activeConcepts := make(map[string]bool)
			for _, d := range activeDomains {
				for _, c := range d.Graph.Concepts {
					activeConcepts[c] = true
				}
			}
			states = filterStatesByConcepts(states, activeConcepts)
			interactions = filterInteractionsByConcepts(interactions, activeConcepts)
		}

		alerts := engine.ComputeAlerts(states, interactions, sessionStart)

		hasCritical := false
		for _, a := range alerts {
			if a.Urgency == models.UrgencyCritical {
				hasCritical = true
				break
			}
		}
		if alerts == nil {
			alerts = []models.Alert{}
		}

		r, _ := jsonResult(map[string]interface{}{
			"alerts":             alerts,
			"has_critical":       hasCritical,
			"needs_domain_setup": false,
		})
		return r, nil, nil
	})
}
