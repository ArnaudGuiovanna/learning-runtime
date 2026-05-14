# [FEATURE] Add semantic_observation_json to record_interaction

Suggested labels: `enhancement`, `p2`, `todo`

## Description

> Priority: p2
> Classification: LLM_DETECTION

`record_interaction` accepts `error_type`, `notes`, `misconception_type`, `misconception_detail`, `rubric_json`, and `rubric_score_json`. That captures some LLM-detected signals, but there is no structured field for richer semantic observations such as brittle success, correct answer with flawed reasoning, partial transfer, overconfidence, or suggested next move.

Affected files:

- `tools/interaction.go` — request schema and validation.
- `tools/interaction_apply.go` — interaction persistence and pedagogical snapshot observation.
- `db/schema.sql` / migrations — if persisted as a first-class interaction column.

## Proposed solution

Add optional `semantic_observation_json`, validated as a JSON object and included in pedagogical snapshots. Do not let it directly mutate BKT/FSRS in the first implementation.

Example:

```json
{
  "reasoning_quality": "brittle",
  "success_mode": "procedural_without_explanation",
  "transfer_quality": "near_with_scaffold",
  "confidence_alignment": "overconfident",
  "suggested_next_move": "contrastive_example",
  "evidence": "Learner produced the formula but could not justify why it applies."
}
```

## Acceptance criteria

- [ ] `record_interaction` accepts `semantic_observation_json`.
- [ ] Invalid JSON is rejected.
- [ ] Observation is persisted or included in pedagogical snapshots.
- [ ] Tests cover valid, invalid, and absent semantic observations.
- [ ] No probabilistic model update directly depends on this field in the first implementation.
