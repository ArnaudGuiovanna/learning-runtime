# [FEATURE] Use mastery evidence, uncertainty, and transfer before final activity selection

Suggested labels: `enhancement`, `p1`, `todo`, `next-up`

## Description

> Priority: p1
> Classification: PEDAGOGY

`get_next_activity` computes `mastery_evidence`, `mastery_uncertainty`, `transfer_profile`, and `rasch_elo_calibration` after the orchestrator has already selected the activity. The LLM sees those diagnostics, but the runtime selection itself does not use them to adjust the final activity.

Affected files:

- `tools/activity.go` — evidence, uncertainty, transfer, and Rasch/Elo diagnostics are computed post-selection.
- `engine/orchestrator.go` — `SelectAction` result is composed before those diagnostics are available.
- `engine/evidence.go` and `engine/uncertainty.go` — existing signals can feed the selection path.

## Proposed solution

Introduce an `EvidenceController` or extend `ActionSelector` input so the runtime can adjust the final action before returning it.

Examples:

- High BKT + weak evidence → avoid `MASTERY_CHALLENGE`; request varied proof.
- High mastery + unobserved transfer → prefer `TRANSFER_PROBE`.
- Transfer blocked → prefer `FEYNMAN_PROMPT`.
- Recall-only evidence → prefer practice/Feynman/transfer before mastery claim.

## Acceptance criteria

- [ ] Runtime can redirect high-mastery actions when evidence is weak.
- [ ] Runtime can prioritize transfer when transfer is missing or weak.
- [ ] Runtime rationale includes the evidence-based adjustment.
- [ ] Tests cover high BKT + weak evidence, high BKT + no transfer, transfer blocked, and strong evidence.
