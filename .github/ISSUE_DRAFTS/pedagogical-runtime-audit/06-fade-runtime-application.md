# [FEATURE] Apply FadeController parameters beyond motivation text

Suggested labels: `enhancement`, `p2`, `todo`

## Description

> Priority: p2
> Classification: ADAPTIVITY

`FadeController` returns `HintLevel`, `WebhookFrequency`, `ZPDAggressiveness`, and `ProactiveReviewEnabled`. In `get_next_activity`, the current integration mainly applies `HintLevel` to the motivation brief and exposes `fade_params` in JSON. The other fade parameters are not yet applied deeply to runtime behavior.

Affected files:

- `engine/fade_controller.go` — returns four autonomy-dependent parameters.
- `tools/activity.go` — applies fade only to motivation brief and exposes the bundle.

## Proposed solution

Short term: inject fade into `pedagogical_contract` so the LLM adapts hinting, autonomy handoff, and challenge framing.

Medium term:

- Use `ZPDAggressiveness` in difficulty targeting.
- Use `WebhookFrequency` in scheduler dispatch.
- Use `ProactiveReviewEnabled` in proactive review scheduling.

## Acceptance criteria

- [ ] `fade_params` influence LLM-facing scaffolding guidance, not only motivation text.
- [ ] Low, mid, and high autonomy produce distinct pedagogical contract outputs.
- [ ] Follow-up issues or TODOs exist for scheduler-level `WebhookFrequency` and `ProactiveReviewEnabled` integration.
