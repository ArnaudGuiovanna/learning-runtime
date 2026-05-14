# [FEATURE] Split audit rationale, LLM instruction, and learner explanation

Suggested labels: `enhancement`, `p2`, `todo`

## Description

> Priority: p2
> Classification: LLM_UX

The current `Activity` contains a single `Rationale`. The orchestrator fills it with technical details combining phase, selection rationale, and action rationale. The system prompt also tells the LLM not to explain algorithmic reasoning to the learner. This creates an avoidable ambiguity: the same field is useful for audit but not learner-facing communication.

Affected files:

- `models/domain.go` — single `Activity.Rationale` field.
- `engine/orchestrator.go` — technical rationale construction.
- `tools/prompt.go` — instructs the LLM not to expose algorithmic reasoning.

## Proposed solution

Add separate outputs, either in `pedagogical_contract` or as a sibling object:

```json
{
  "audit_rationale": "phase=INSTRUCTION; selected by relevance × mastery gap",
  "llm_instruction": "Use a short contrastive practice prompt and collect a rubric score.",
  "learner_explanation": "On consolide ce point car il débloque la suite de ton objectif."
}
```

## Acceptance criteria

- [ ] Technical rationale remains available for audit.
- [ ] LLM receives a concise operational instruction.
- [ ] Learner-safe explanation is available without raw thresholds unless intentionally included.
- [ ] Tests verify the learner explanation does not leak internal algorithm details by default.
