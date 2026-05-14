# [BUG] Regulation prompt appendices still say components are not wired

Suggested labels: `documentation`, `p2`, `todo`

## Description

> Priority: p2
> Classification: DOCS

Some prompt appendix comments still imply that regulation components are not wired into the runtime, even though `get_next_activity` calls `engine.OrchestrateWithPhase` and the orchestrator runs Gate, ConceptSelector, and ActionSelector.

Affected file:

- `tools/prompt.go`

Examples:

- Action selector appendix references behavior “once the regulation orchestrator is wired”.
- Concept selector appendix says the function is “not yet wired into the runtime”.

## Expected behavior

Comments and prompt wording should match the current runtime state. The appendices should explain that the components are wired and that the corresponding flags only control whether explanatory prompt appendices are shown.

## Acceptance criteria

- [ ] Remove stale “not wired yet” / “once wired” language.
- [ ] Clarify that the runtime components are active in the orchestrator path.
- [ ] Prompt-related tests are updated if they assert old wording.
