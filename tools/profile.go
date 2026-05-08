// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// UpdateLearnerProfileParams enumerates the fields the chat-side LLM may push
// into a learner's persisted profile blob. Issue #61: `level`, `background`
// and `learning_style` were removed because no downstream component (motivation
// brief, concept selector, alerts, dashboard) consumes them — they were
// write-only metadata that bloated the profile_json blob without informing
// any decision. The forward-only migration in db/migrations.go scrubs the keys
// out of existing rows so a re-introduction won't silently inherit stale data.
type UpdateLearnerProfileParams struct {
	Device          string  `json:"device,omitempty" jsonschema:"Appareil principal (ex: laptop, phone, tablet)"`
	Objective       string  `json:"objective,omitempty" jsonschema:"Objectif d'apprentissage mis à jour"`
	Language        string  `json:"language,omitempty" jsonschema:"Langue préférée pour les exercices"`
	CalibrationBias float64 `json:"calibration_bias,omitempty" jsonschema:"Biais de calibration (positif=sur-estimé, négatif=sous-estimé)"`
	AffectBaseline  string  `json:"affect_baseline,omitempty" jsonschema:"Baseline émotionnelle de l'apprenant"`
	AutonomyScore   float64 `json:"autonomy_score,omitempty" jsonschema:"Score d'autonomie actuel (0-1)"`
}

func registerUpdateLearnerProfile(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_learner_profile",
		Description: "Met à jour les métadonnées persistantes de l'apprenant (device, objectif, langue, calibration, affect, autonomie). Seuls les champs fournis sont modifiés.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params UpdateLearnerProfileParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("update_learner_profile: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		learner, err := deps.Store.GetLearnerByID(learnerID)
		if err != nil {
			deps.Logger.Error("update_learner_profile: failed to get learner", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("learner not found: %v", err))
			return r, nil, nil
		}

		// Length caps (issue #31). Each field is a free-text input; without
		// these caps a misbehaving caller can inflate the profile_json blob
		// to multiple MB, slowing every subsequent read of this learner.
		stringFields := []struct {
			name  string
			value string
			max   int
		}{
			{"device", params.Device, maxShortLabelLen},
			{"objective", params.Objective, maxNoteLen},
			{"language", params.Language, maxShortLabelLen},
			{"affect_baseline", params.AffectBaseline, maxShortLabelLen},
		}
		for _, f := range stringFields {
			if err := validateString(f.name, f.value, f.max); err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
		}

		// Load existing profile
		profile := make(map[string]interface{})
		if learner.ProfileJSON != "" && learner.ProfileJSON != "{}" {
			_ = json.Unmarshal([]byte(learner.ProfileJSON), &profile)
		}

		// Merge only non-empty fields
		updated := 0
		if params.Device != "" {
			profile["device"] = params.Device
			updated++
		}
		if params.Language != "" {
			profile["language"] = params.Language
			updated++
		}
		if params.CalibrationBias != 0 {
			profile["calibration_bias"] = params.CalibrationBias
			updated++
		}
		if params.AffectBaseline != "" {
			profile["affect_baseline"] = params.AffectBaseline
			updated++
		}
		if params.AutonomyScore != 0 {
			profile["autonomy_score"] = params.AutonomyScore
			updated++
		}

		// Update objective on learner record if provided
		if params.Objective != "" {
			profile["objective"] = params.Objective
			updated++
		}

		profileJSON, _ := json.Marshal(profile)
		if err := deps.Store.UpdateLearnerProfile(learnerID, string(profileJSON)); err != nil {
			deps.Logger.Error("update_learner_profile: failed to update profile", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("failed to update profile: %v", err))
			return r, nil, nil
		}

		r, _ := jsonResult(map[string]interface{}{
			"updated":        true,
			"fields_changed": updated,
			"profile":        profile,
		})
		return r, nil, nil
	})
}
