# [FEATURE] Clarify REGULATION_ACTION/CONCEPT/GATE as prompt-only flags or implement real runtime switches

Suggested labels: `documentation`, `p2`, `todo`

## Description

> Priority: p2
> Classification: CONFIG

`REGULATION_ACTION`, `REGULATION_CONCEPT`, and `REGULATION_GATE` are used by `buildSystemPrompt` to include or omit explanatory appendices. They do not disable the runtime components: the normal `get_next_activity` path always calls the orchestrator, and the orchestrator always invokes Gate, ConceptSelector, and ActionSelector.

Affected files:

- `tools/prompt.go` — prompt appendix flags.
- `tools/activity.go` — always calls the orchestrator in auto mode.
- `engine/orchestrator.go` — always runs the regulation components.

## Proposed solution

Either document these as prompt-only flags or introduce clearer aliases:

- `REGULATION_ACTION_PROMPT`
- `REGULATION_CONCEPT_PROMPT`
- `REGULATION_GATE_PROMPT`

If real runtime kill switches are required, implement them explicitly as a separate design.

## Acceptance criteria

- [ ] Operator-facing docs/comments say the existing flags are prompt-only.
- [ ] Prompt tests still cover flag-gated appendices.
- [ ] If aliases are added, old names remain backward-compatible.
