# [FEATURE] Make learning_negotiation support structured, one-shot activity overrides

Suggested labels: `enhancement`, `p1`, `todo`

## Description

> Priority: p1
> Classification: ARCHITECTURE

`learning_negotiation` currently negotiates mostly a concept. It accepts `learner_concept` and `learner_rationale`, computes the system plan, then either rejects the proposal or returns an accepted plan. When accepted, the plan is always rewritten as `RECALL_EXERCISE` with `format=mixed` and `estimated_minutes=15`.

The accepted plan is returned in JSON but is not persisted as a one-shot override for the next `get_next_activity` call.

Affected files:

- `tools/negotiation.go` — narrow params and non-persistent accepted plan.
- `tools/activity.go` — no consumption path for negotiated one-shot plans.

## Proposed solution

Extend negotiation beyond concept changes:

- `concept_change`
- `format_change`
- `activity_type_change`
- `scaffold_change`
- `micro_diagnostic`
- `defer_activity`

Persist accepted overrides with one-shot semantics and consume them in the next `get_next_activity`, without allowing them to bypass hard constraints such as overload, unknown concept, invalid domain, or unsatisfied hard prerequisites.

## Acceptance criteria

- [ ] `learning_negotiation` supports at least concept, format, and activity-type changes.
- [ ] Accepted negotiation creates a persisted one-shot override.
- [ ] Next `get_next_activity` consumes the override exactly once.
- [ ] Rejected negotiations explain tradeoffs.
- [ ] Tests cover accepted, rejected, expired, and consumed override paths.
