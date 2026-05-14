# [FEATURE] Return a structured pedagogical_contract from get_next_activity

Suggested labels: `enhancement`, `p1`, `todo`, `next-up`

## Description

> Priority: p1
> Classification: ARCHITECTURE

The runtime currently returns a thin `Activity` object: type, concept, difficulty, format, minutes, rationale, and `PromptForLLM`. That is useful for routing, but it is not LLM-native enough for adaptive tutoring. The LLM receives a directive to generate an activity, not a policy contract explaining intent, constraints, allowed variation, evidence requirements, or negotiability.

Affected files:

- `models/domain.go` — `Activity` has no structured pedagogical affordances.
- `engine/orchestrator.go` — `composeActivity` creates a minimal prompt.
- `tools/activity.go` — `get_next_activity` already collects tutor mode, motivation, misconceptions, evidence, uncertainty, transfer, and Rasch/Elo signals that can feed the contract.

## Proposed solution

Add a non-breaking sibling object to `get_next_activity`, for example:

```json
{
  "pedagogical_contract": {
    "intent": "stabilize_near_mastery",
    "target_concept": "goroutines",
    "recommended_activity_type": "PRACTICE",
    "constraints": {
      "must_collect": ["learner_answer", "rubric_score"],
      "avoid": ["introducing_new_prerequisite", "long_explanation_first"]
    },
    "allowed_variants": ["socratic_prompt", "contrastive_example", "worked_example_completion", "micro_debug"],
    "llm_discretion": {
      "can_change_format": true,
      "can_request_clarification": true,
      "can_propose_negotiation": true,
      "cannot_mark_mastery_without_evidence": true
    },
    "learner_explanation": "On stabilise ce point avant un transfert plus ouvert.",
    "audit_rationale": "phase=INSTRUCTION; selected by relevance × mastery gap"
  }
}
```

## Acceptance criteria

- [ ] `get_next_activity` returns `pedagogical_contract` without breaking existing clients.
- [ ] Contract includes `intent`, `constraints`, `allowed_variants`, `llm_discretion`, `learner_explanation`, and `audit_rationale`.
- [ ] Tests cover at least `PRACTICE`, `RECALL_EXERCISE`, `DEBUG_MISCONCEPTION`, `FEYNMAN_PROMPT`, `TRANSFER_PROBE`, and `CLOSE_SESSION`.
- [ ] The system prompt tells the LLM to follow the contract rather than only `PromptForLLM` prose.
