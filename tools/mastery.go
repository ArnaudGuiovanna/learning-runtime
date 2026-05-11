// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"time"

	"tutor-mcp/algorithms"
	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CheckMasteryParams struct {
	Concept   string `json:"concept,omitempty" jsonschema:"the concept to check for mastery; canonical key for concept-targeting tools; required unless concept_id is used"`
	ConceptID string `json:"concept_id,omitempty" jsonschema:"deprecated compatibility alias for concept; prefer concept"`
	DomainID  string `json:"domain_id,omitempty" jsonschema:"domain ID (optional)"`
}

func registerCheckMastery(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_mastery",
		Description: "Check whether a concept is ready for the mastery challenge using BKT plus evidence diversity and uncertainty.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params CheckMasteryParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("check_mastery: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		concept, err := normalizeConceptParam(params.Concept, params.ConceptID)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}
		if concept == "" {
			r, _ := errorResult("concept is required")
			return r, nil, nil
		}
		if err := validateString("domain_id", params.DomainID, maxShortLabelLen); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		domainID := params.DomainID
		if domainID != "" {
			domain, err := resolveDomain(deps.Store, learnerID, domainID)
			if err != nil || domain == nil {
				deps.Logger.Error("check_mastery: domain not found", "err", err, "learner", learnerID, "domain_id", params.DomainID)
				r, _ := errorResult("domain not found")
				return r, nil, nil
			}
			if err := validateConceptInDomain(domain, concept); err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
			domainID = domain.ID
		}

		cs, err := deps.Store.GetConceptState(learnerID, concept)
		if err != nil {
			deps.Logger.Error("check_mastery: failed to get concept state", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("concept state not found: %v", err))
			return r, nil, nil
		}

		bktState := algorithms.BKTState{PMastery: cs.PMastery}
		bktMastered := algorithms.BKTIsMastered(bktState)
		recent, err := deps.Store.GetRecentInteractions(learnerID, concept, 50)
		if err != nil {
			deps.Logger.Error("check_mastery: failed to get recent interactions", "err", err, "learner", learnerID, "concept", concept)
			r, _ := errorResult(fmt.Sprintf("failed to compute mastery evidence: %v", err))
			return r, nil, nil
		}
		recent = filterInteractionsByDomainID(recent, domainID)
		now := time.Now().UTC()
		evidenceProfile := engine.BuildEvidenceProfile(learnerID, concept, recent, now)
		evidenceQuality := engine.MasteryEvidenceQuality(evidenceProfile)
		uncertainty := engine.ComputeMasteryUncertainty(cs, recent, engine.MasteryEvidenceProfile{Now: now})
		transferRecords, err := deps.Store.GetTransferScores(learnerID, concept)
		if err != nil {
			deps.Logger.Warn("check_mastery: transfer profile fetch failed", "err", err, "learner", learnerID, "concept", concept)
			transferRecords = nil
		}
		transferProfile := engine.BuildTransferProfile(concept, transferRecords)
		evidenceOK := evidenceQuality.Quality != engine.EvidenceQualityWeak
		uncertaintyOK := uncertainty.ConfidenceLabel != engine.MasteryConfidenceLow
		transferOK := transferProfile.ReadinessLabel != engine.TransferReadinessBlocked
		isMastered := bktMastered && evidenceOK && uncertaintyOK && transferOK

		if !isMastered {
			message := fmt.Sprintf("Pas encore pret. Maitrise actuelle: %.0f%%, seuil: %.0f%%", cs.PMastery*100, algorithms.MasteryBKT()*100)
			if bktMastered && !evidenceOK {
				message = "BKT est au-dessus du seuil, mais les preuves sont encore trop peu variees pour une mastery challenge."
			}
			if bktMastered && evidenceOK && !uncertaintyOK {
				message = "BKT est au-dessus du seuil, mais l'incertitude du modele reste trop elevee pour une mastery challenge."
			}
			if bktMastered && evidenceOK && uncertaintyOK && !transferOK {
				message = "BKT est au-dessus du seuil, mais un transfert recent est bloque: retravailler le concept dans un autre contexte avant la mastery challenge."
			}
			r, _ := jsonResult(map[string]interface{}{
				"mastery_ready":       false,
				"bkt_mastery_ready":   bktMastered,
				"mastery":             cs.PMastery,
				"current_mastery":     cs.PMastery,
				"threshold":           algorithms.MasteryBKT(),
				"evidence_profile":    evidenceProfile,
				"evidence_quality":    evidenceQuality,
				"mastery_uncertainty": uncertainty,
				"transfer_profile":    transferProfile,
				"message":             message,
			})
			return r, nil, nil
		}

		r, _ := jsonResult(map[string]interface{}{
			"mastery_ready":       true,
			"bkt_mastery_ready":   bktMastered,
			"mastery":             cs.PMastery,
			"current_mastery":     cs.PMastery,
			"evidence_profile":    evidenceProfile,
			"evidence_quality":    evidenceQuality,
			"mastery_uncertainty": uncertainty,
			"transfer_profile":    transferProfile,
			"challenge": map[string]interface{}{
				"prompt_for_llm": fmt.Sprintf(
					"Generate a mastery challenge on %s. "+
						"The learner must build something complete that demonstrates transfer. "+
						"Evaluate: autonomous application, edge-case handling, code quality. "+
						"Do not guide - observe whether the learner can apply the concept alone.",
					concept,
				),
				"evaluation_criteria": []string{
					"Application autonome sans aide",
					"Gestion correcte des cas limites",
					"Code propre et idiomatique",
					"Explication claire du raisonnement",
				},
			},
		})
		return r, nil, nil
	})
}
