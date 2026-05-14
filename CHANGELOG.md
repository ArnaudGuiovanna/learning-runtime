# Changelog

All notable changes to Tutor MCP are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0-alpha.2] — 2026-05-14

### Added

- Learner memory package with markdown-backed episodic sessions, stable memory,
  pending observations, concept notes, archives, atomic writes, YAML session
  frontmatter parsing, and `TUTOR_MCP_MEMORY_ROOT` / `TUTOR_MCP_MEMORY_ENABLED`.
- Memory MCP tools: `update_learner_memory`, `read_raw_session`, and
  `get_memory_state`.
- `episodic_context` and `reasoning_request` in `get_next_activity` so the
  client LLM can produce an interpretation brief before generating the activity.
- `interpretation_brief` storage on pedagogical snapshots and replay summaries.
- Client-initiated consolidation: the server enqueues due monthly, quarterly,
  and annual jobs in `pending_consolidations`, attaches `consolidation_request`
  to `get_next_activity`, and marks jobs completed when the client writes the
  archive through `update_learner_memory`.
- `--version`, `-version`, and `version` CLI output for release binaries.

### Changed

- Consolidation no longer performs any server-side LLM or archive generation.
  The connected MCP client authors the archive using its own LLM session.
- OLM fallback focus reason and webhook/nudge runtime copy are normalized to
  English.
- Discord OLM push filtering now suppresses plain KST fallback pushes unless
  recent narrative memory gives the message real learning value.

## [0.3.0-alpha.1] — 2026-05-08

### Refreshed 2026-05-09 — QA hardening pass

Binaries re-cut from `0149a74` to ship 17 post-tag fixes from a focused QA
review. No API breakage, no migration required, drop-in replacement for the
2026-05-08 refresh.

#### Cross-domain leak fixes (the headline)

The orchestrator's concept-state list is learner-wide; two of the three phase
selectors and two metacognitive read tools were not re-filtering by the active
domain, so a concept mastered/in-progress in domain A could surface as the
suggested activity (or as input to the autonomy/mirror computation) for
domain B. Closed in [#131](https://github.com/ArnaudGuiovanna/tutor-mcp/pull/131):

- `selectDiagnostic` and `selectMaintenance` now restrict candidates to the
  active domain's `graph.Concepts` (closes #93, #130).
- `get_metacognitive_mirror` and `get_autonomy_metrics` now honor `domain_id`
  by filtering interactions and concept states (closes #95).
- Side benefit: `selectDiagnostic` now also respects the gate's anti-repeat
  window, which it was silently bypassing.
- `selectInstruction` was already structurally safe via `externalFringe`.

#### Auth, scheduler, migration, perf

- **OAuth confidential clients can complete `/token` without PKCE**
  ([#128](https://github.com/ArnaudGuiovanna/tutor-mcp/pull/128), closes #114).
  PKCE check at `/token` is now conditional on the auth code having been stored
  with a non-empty `code_challenge` — public clients still require PKCE
  (regression-tested), confidential clients authenticate via `client_secret`
  (bcrypt-verified, unchanged).
- **Scheduler shutdown drains cron jobs**
  ([#129](https://github.com/ArnaudGuiovanna/tutor-mcp/pull/129), closes #123,
  #124). `Scheduler.Stop()` now blocks on `cron.Stop()`'s drain context with a
  25 s deadline, so in-flight webhook retries finish before `database.Close()`
  on SIGTERM.
- **Migration runner is now atomic per migration**
  ([#127](https://github.com/ArnaudGuiovanna/tutor-mcp/pull/127), closes #118).
  `applyMigration` wraps the DDL body and the `schema_migrations` insert in a
  single transaction so a partial failure can no longer leave the DB and the
  bookkeeping table in disagreement. The `IgnoreExecErrors` legacy path still
  records its row.
- **`get_next_activity` drops a redundant `GetDomainByID` re-read**
  ([#113](https://github.com/ArnaudGuiovanna/tutor-mcp/pull/113), closes #91).
  New `engine.OrchestrateWithPhase` surfaces the resolved phase from
  in-memory; the legacy `engine.Orchestrate` is a thin wrapper so all existing
  callers are byte-identical.

#### Validation hardening

Twelve PRs landed earlier in the day to close the input-validation gaps QA had
flagged on chat-side LLM-driven tools:

- `actual_score`, `predicted_mastery`, `calibration_bias`, `autonomy_score`
  now reject NaN, Inf, and out-of-range values (closes #83, #85).
- `update_learner_profile` distinguishes "not provided" from "explicit 0"
  via `*float64`, so `calibration_bias=0` (perfect calibration) no longer
  vanishes through `omitempty` (closes #89).
- `activity_type` and `error_type` enforced as enums in `record_interaction`
  (closes #88).
- `learner_concept`, `concept_id`, and `context_type` validated against the
  active domain's graph in `learning_negotiation` and `record_transfer_result`
  (closes #92, #96).
- Unbounded string fields capped across remaining handlers (closes #82).
- `resolveDomain` now rejects archived domains (closes #94).
- `get_dashboard_state` aligned to the codebase's English error-string
  convention (closes #90).
- DB-layer ownership filter on calibration record helpers (closes #87).

#### Tests, docs, observability

- New end-to-end coherence regression suite covering scenarios 5-9 of #97.
- New `docs/mvp-checklist.md` MVP exit-criteria tracker.
- Disambiguated routing descriptions on `domain_id`-aware tools.
- 22 open issues bootstrapped with p0/p1/p2 priority labels (#81).

### Initial public alpha

Tutor MCP exposes a deterministic Intelligent Tutoring System runtime over the
Model Context Protocol so any compatible LLM (Claude, ChatGPT, Le Chat, Gemini)
can drive an adaptive learning loop without an editorial team.

#### Highlights

- **Cognitive engine** — BKT (mastery), FSRS (spaced repetition), IRT (ability),
  PFA (plateau detection), KST (prerequisite gating), plus a separate
  Rasch/Elo calibration signal for learner ability vs. exercise difficulty.
  The learner model updates on every interaction. The BKT → FSRS → IRT chain
  runs against a single read-only snapshot so order-of-evaluation no longer
  leaks cross-step state.
- **Regulation pipeline (v0.3)** — seven-stage pipeline: Threshold Resolver,
  Goal Decomposer, Action Selector, Concept Selector, Gate Controller, Phase
  Controller (DIAGNOSTIC ↔ INSTRUCTION ↔ MAINTENANCE FSM), Fade Controller.
  All shipped except FadeController which is opt-in via `REGULATION_FADE=on`.
- **Surface** — chat-only. 28 MCP tools across cognitive state, domain
  management, metacognition (mirror, calibration, autonomy), motivation
  (utility-value, growth-mindset, OLM narrative), and webhook nudges
  (Discord-targeted).
- **OAuth 2.1 + PKCE** — confidential and public client support, refresh-token
  rotation with client binding (per RFC 6749 §10.4 / RFC 9700 §2.2), per-IP
  and per-account rate limiting, bcrypt cost 12, password floor 12 chars.
- **Storage** — SQLite via modernc.org/sqlite (CGo-free), versioned migrations
  with SHA-256 checksum drift detection, idempotent CREATE TABLE / ALTER
  TABLE pipeline.
- **Observability** — structured slog, decision-trace logs through the
  regulation pipeline, scheduler with 6 cron jobs (OLM, motivation, recap,
  mirror, cleanup, metacognitive alerts).

#### Known limitations

Three algorithmic refinements deferred for a later release:

- **PFA fidelity** ([#48](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/48))
  — the plateau detector follows a project-specific convention rather than
  Pavlik (2009) verbatim (sign of ρ, β intercept, decay term).
- **IRT statistical robustness** ([#49](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/49))
  — pure MLE saturates θ on extreme response strings; no EAP/MAP prior.
- **FSRS sub-day intervals** ([#52](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/52))
  — Learning/Relearning steps are day-granular; hours not yet supported.

#### Operational notes

- Forward-only migrations. A schema body change after apply surfaces as
  `checksum mismatch` at startup; manual operator action is required to reset.
  This is intentional — drift requires intervention, not a silent retry.
- The per-IP rate limiter assumes `TRUSTED_PROXY_CIDRS` is set in any public
  deployment behind a reverse proxy. Tailscale Funnel users can ignore the
  startup warning — the funnel terminates TLS locally and the rate-limiter
  collapses to a single global bucket by design.
- No CI is configured at the repo level. `go build ./... && go test ./...` is
  the smoke test; contributors are expected to run it before opening a PR.

#### Compatibility

- Tested with Claude Desktop and Claude.ai (custom connectors).
- Go 1.25+ required.
- SQLite >= 3.35 (DROP COLUMN support).

[0.3.0-alpha.1]: https://github.com/ArnaudGuiovanna/tutor-mcp/releases/tag/v0.3.0-alpha.1
