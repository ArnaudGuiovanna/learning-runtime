# [BUG] Regulation Gate never receives real sessionStart for OVERLOAD

Suggested labels: `bug`, `p0`, `todo`, `next-up`

## Description

> Priority: p0
> Classification: BUG

`OVERLOAD` should force the regulation Gate to emit `CLOSE_SESSION`, but the orchestrator recomputes alerts with `input.Now` where `ComputeAlerts` expects the session start timestamp.

`tools/activity.go` computes alerts correctly with `sessionStart`, but `engine/orchestrator.go` recomputes them inside `fetchPipelineFixtures` as `ComputeAlerts(states, recentInteractions, input.Now)`. Since `engine/alert.go` emits `OVERLOAD` only when `time.Since(sessionStart) > 45*time.Minute`, passing `input.Now` makes the condition almost always false.

Affected files:

- `tools/activity.go` — correct `sessionStart` is already available before the orchestrator call.
- `engine/orchestrator.go` — `fetchPipelineFixtures` passes `input.Now` as the session timestamp.
- `engine/alert.go` — `ComputeAlerts` interprets the third argument as `sessionStart`.
- `engine/gate.go` — `ApplyGate` only emits `CLOSE_SESSION` if it receives `AlertOverload`.

## Expected behavior

When the active session is older than the overload threshold, `get_next_activity` should return a `CLOSE_SESSION` activity with `session_overload` format.

## Suggested fix

Add `SessionStart time.Time` or precomputed `Alerts []models.Alert` to `engine.OrchestratorInput`, then ensure `fetchPipelineFixtures` uses the real session start instead of `input.Now`.

## Acceptance criteria

- [ ] `fetchPipelineFixtures` no longer calls `ComputeAlerts(..., input.Now)` as if `input.Now` were `sessionStart`.
- [ ] `get_next_activity` propagates the real session start or precomputed alerts into the orchestrator.
- [ ] A test proves a session older than 45 minutes returns `CLOSE_SESSION`.
- [ ] A test proves a fresh session does not return `CLOSE_SESSION`.
- [ ] Existing Gate unit tests still pass.
