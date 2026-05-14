# [BUG] PromptForLLM can contain stale difficulty after tutor-mode and calibration adjustments

Suggested labels: `bug`, `p1`, `todo`

## Description

> Priority: p1
> Classification: BUG

`engine.composeActivity` embeds the numeric target difficulty directly in `PromptForLLM`. Later, `tools/get_next_activity` mutates `activity.DifficultyTarget` for tutor mode (`lighter`, `scaffolding`) and calibration bias. The structured field can therefore disagree with the textual prompt.

Affected files:

- `engine/orchestrator.go` — `composeActivity` writes `Target difficulty: %.2f` into the prompt.
- `tools/activity.go` — tutor mode and calibration bias mutate `activity.DifficultyTarget` after composition.

## Expected behavior

The LLM should receive one final, consistent difficulty signal.

## Suggested fix

Prefer removing the numeric difficulty from `PromptForLLM` and instruct the LLM to use the final structured `activity.difficulty_target`. Alternatively, recompute `PromptForLLM` after all post-orchestrator adjustments.

## Acceptance criteria

- [ ] `PromptForLLM` cannot contradict `activity.DifficultyTarget`.
- [ ] A test covers `tutor_mode=lighter` and verifies prompt consistency.
- [ ] A test covers `tutor_mode=scaffolding` and verifies prompt consistency.
- [ ] A test covers calibration-bias clamping and verifies prompt consistency.
