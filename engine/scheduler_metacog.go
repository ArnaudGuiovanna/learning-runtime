// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"time"

	"tutor-mcp/models"
)

// metacogKindToWebhookKind maps an AlertType to the webhook_message_queue
// `kind` slot under which the dispatch enqueues a nudge. Each kind is
// dispatched independently — a learner with both AFFECT_NEGATIVE and
// CALIBRATION_DIVERGING firing on the same tick will get two webhook
// rows, deduped per-day via scheduled_alerts (WasAlertSentToday).
func metacogKindToWebhookKind(t models.AlertType) string {
	switch t {
	case models.AlertDependencyIncreasing:
		return "metacog_dependency"
	case models.AlertCalibrationDiverging:
		return "metacog_calibration"
	case models.AlertAffectNegative:
		return "metacog_affect"
	case models.AlertTransferBlocked:
		return "metacog_transfer"
	}
	return ""
}

// metacogFallbackContent returns the sober Go-side fallback nudge body for
// each metacognitive alert kind. The content is intentionally short and
// non-prescriptive — it tells the learner *what surfaced* and points to
// the corresponding tool, leaving the conversational handling to Claude
// when the learner returns.
func metacogFallbackContent(a models.Alert) string {
	switch a.Type {
	case models.AlertDependencyIncreasing:
		return "Your autonomy score has dropped over the last 3 sessions. " +
			"When you come back, we can talk about it - call get_metacognitive_mirror."
	case models.AlertCalibrationDiverging:
		return "Your calibration has drifted from reality (" + a.RecommendedAction + "). " +
			"A short calibration session can help - calibration_check."
	case models.AlertAffectNegative:
		return "The last two sessions have been hard. " +
			"We can adjust tutor_mode when you return."
	case models.AlertTransferBlocked:
		concept := a.Concept
		if concept == "" {
			concept = "a mastered concept"
		}
		return "Transfer on \"" + concept + "\" remains weak despite mastery. " +
			"A Feynman challenge would likely unblock the situation."
	}
	return "A new metacognitive observation is available."
}

// dispatchMetacognitiveAlerts iterates over every active learner, computes
// metacognitive alerts (DEPENDENCY / CALIBRATION / AFFECT / TRANSFER) from
// their current state, and pushes a webhook for each newly-firing alert
// kind. Per-learner per-day dedup is enforced by dispatchQueued via
// scheduled_alerts (WasAlertSentToday + CreateScheduledAlert) — the cron
// cadence only affects detection latency, not delivery frequency.
//
// Two-stage flow on each tick:
//
//  1. Compute alerts → enqueue one webhook_message_queue row per *kind*
//     that just went hot (kind = metacog_dependency / metacog_calibration
//     / metacog_affect / metacog_transfer). Skip kinds already fired today
//     (WasAlertSentToday) so we don't enqueue stale duplicates.
//  2. Drain each enqueued kind via dispatchQueued, which posts the embed
//     and stamps scheduled_alerts so the next tick deduplicates.
//
// TRANSFER_BLOCKED is per-concept in the alert payload but is collapsed to
// a single per-day kind here — one nudge per day is the product contract,
// and the embed mentions the most-recent concept that fired.
func (s *Scheduler) dispatchMetacognitiveAlerts() {
	if s.store == nil {
		return
	}
	learners, err := s.store.GetActiveLearners()
	if err != nil {
		s.logger.Error("scheduler: metacog get learners", "err", err)
		return
	}
	now := time.Now().UTC()

	// Track which kinds were enqueued this tick so we know what to drain.
	enqueuedKinds := make(map[string]bool)

	for _, learner := range learners {
		if learner.WebhookURL == "" {
			continue
		}
		avail, _ := s.store.GetAvailability(learner.ID)
		if avail != nil && avail.DoNotDisturb {
			continue
		}

		// Gather inputs. Errors on any one source are tolerated — the
		// missing input simply skips its branch in
		// ComputeMetacognitiveAlerts (no alert is better than panicking
		// the cron tick on a single learner with corrupt rows).
		states, _ := s.store.GetConceptStatesByLearner(learner.ID)
		interactions, _ := s.store.GetRecentInteractionsByLearner(learner.ID, 20)
		affects, _ := s.store.GetRecentAffectStates(learner.ID, 10)
		var autonomyScores []float64
		for _, a := range affects {
			autonomyScores = append(autonomyScores, a.AutonomyScore)
		}
		calibBias, _ := s.store.GetCalibrationBias(learner.ID, 20)
		transfers, _ := s.store.GetTransferRecordsByLearner(learner.ID)

		alerts := ComputeMetacognitiveAlerts(
			autonomyScores,
			calibBias,
			affects,
			interactions,
			WithTransferData(states, transfers),
		)

		// Collapse per-concept alerts down to one row per kind. Keep the
		// first alert seen for each kind (deterministic enough — alerts
		// from ComputeMetacognitiveAlerts are produced in a fixed order).
		seenKind := make(map[string]bool)
		for _, a := range alerts {
			kind := metacogKindToWebhookKind(a.Type)
			if kind == "" || seenKind[kind] {
				continue
			}
			seenKind[kind] = true

			// kind is the dedup tag too — dispatchQueued stamps
			// scheduled_alerts(alert_type=kind) on success, so a re-tick
			// today will WasAlertSentToday-skip before re-enqueueing.
			alreadySent, _ := s.store.WasAlertSentToday(learner.ID, kind)
			if alreadySent {
				continue
			}

			content := metacogFallbackContent(a)
			if _, err := s.store.EnqueueWebhookMessage(
				learner.ID, kind, content, now, now.Add(2*time.Hour), 5,
			); err != nil {
				s.logger.Error("scheduler: metacog enqueue",
					"err", err, "learner", learner.ID, "kind", kind)
				continue
			}
			enqueuedKinds[kind] = true
			s.logger.Info("scheduler: metacog enqueued",
				"learner", learner.ID, "kind", kind, "type", a.Type)
		}
	}

	// Drain every kind we enqueued so the webhook actually leaves the
	// process this tick. dispatchQueued handles the WasAlertSentToday
	// dedup + CreateScheduledAlert stamping so the next tick is a no-op.
	for kind := range enqueuedKinds {
		s.dispatchQueued(kind, kind, nil)
	}
}

