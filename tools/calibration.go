// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CalibrationCheckParams struct {
	ConceptID        string  `json:"concept_id" jsonschema:"the concept to assess"`
	PredictedMastery float64 `json:"predicted_mastery" jsonschema:"learner self-assessment: 1=no mastery, 5=perfect mastery"`
	DomainID         string  `json:"domain_id,omitempty" jsonschema:"domain ID (optional)"`
}

func registerCalibrationCheck(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "calibration_check",
		Description: "Record the learner's self-assessment on a concept before an exercise. Returns a prediction_id for post-exercise comparison.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params CalibrationCheckParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("calibration_check: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.ConceptID == "" {
			r, _ := errorResult("concept_id is required")
			return r, nil, nil
		}

		// String length caps (issue #82). concept_id is persisted into
		// calibration_records and echoed into prompt_text; domain_id is
		// resolved against the learner's domains. Without these guards a
		// misbehaving caller could push multi-MB strings into either path.
		stringFields := []struct {
			name  string
			value string
			max   int
		}{
			{"concept_id", params.ConceptID, maxShortLabelLen},
			{"domain_id", params.DomainID, maxShortLabelLen},
		}
		for _, f := range stringFields {
			if err := validateString(f.name, f.value, f.max); err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
		}

		// 1-5 Likert self-assessment. Reject NaN/Inf and out-of-range values
		// rather than silently clamping — clamping a hallucinated 100.0 to
		// 1.0 would corrupt the calibration record. See issue #25.
		if err := validateLikertFloat("predicted_mastery", params.PredictedMastery, 1, 5); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		predicted := (params.PredictedMastery - 1.0) / 4.0

		predictionID := generatePredictionID()
		record := &models.CalibrationRecord{
			PredictionID: predictionID,
			LearnerID:    learnerID,
			ConceptID:    params.ConceptID,
			Predicted:    predicted,
		}

		if err := deps.Store.CreateCalibrationPrediction(record); err != nil {
			deps.Logger.Error("calibration_check: failed to create calibration prediction", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("failed to create calibration: %v", err))
			return r, nil, nil
		}

		promptText := fmt.Sprintf(
			"You estimated your mastery of '%s' at %.0f/5. Let's check that with an exercise.",
			params.ConceptID, params.PredictedMastery,
		)

		r, _ := jsonResult(map[string]interface{}{
			"prediction_id": predictionID,
			"prompt_text":   promptText,
		})
		return r, nil, nil
	})
}

type RecordCalibrationResultParams struct {
	PredictionID string  `json:"prediction_id" jsonschema:"prediction ID returned by calibration_check"`
	ActualScore  float64 `json:"actual_score" jsonschema:"actual score between 0 and 1 (0=total failure, 1=perfect success)"`
}

func registerRecordCalibrationResult(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_calibration_result",
		Description: "Compare the learner's prediction with the actual result. Updates the calibration bias.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params RecordCalibrationResultParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("record_calibration_result: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.PredictionID == "" {
			r, _ := errorResult("prediction_id is required")
			return r, nil, nil
		}

		// Reject NaN/Inf and out-of-range scores rather than silently
		// persisting them. The bias estimator (GetCalibrationBias) averages
		// `predicted - actual`, so a single hallucinated 100.0 corrupts the
		// rolling estimate for the learner. See issue #83 (gap left from
		// the #25/#50 numeric-validation pass).
		if err := validateUnitInterval("actual_score", params.ActualScore); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// Ownership is enforced at the DB layer: GetCalibrationRecord returns
		// "not found" if the prediction belongs to another learner (issue #87).
		record, err := deps.Store.GetCalibrationRecord(params.PredictionID, learnerID)
		if err != nil {
			deps.Logger.Error("record_calibration_result: calibration record not found", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("prediction not found: %v", err))
			return r, nil, nil
		}

		delta := record.Predicted - params.ActualScore

		if err := deps.Store.CompleteCalibrationRecord(params.PredictionID, learnerID, params.ActualScore, delta); err != nil {
			deps.Logger.Error("record_calibration_result: failed to complete calibration record", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("failed to record result: %v", err))
			return r, nil, nil
		}

		bias, _ := deps.Store.GetCalibrationBias(learnerID, 20)

		r, _ := jsonResult(map[string]interface{}{
			"delta":                    delta,
			"calibration_bias_updated": bias,
		})
		return r, nil, nil
	})
}

func generatePredictionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "cal_" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}
