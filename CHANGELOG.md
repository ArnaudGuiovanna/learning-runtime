# Changelog

All notable changes to Tutor MCP are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0-alpha.1] — 2026-05-08

### Initial public alpha

Tutor MCP exposes a deterministic Intelligent Tutoring System runtime over the
Model Context Protocol so any compatible LLM (Claude, ChatGPT, Le Chat, Gemini)
can drive an adaptive learning loop without an editorial team.

#### Highlights

- **Cognitive engine** — BKT (mastery), FSRS (spaced repetition), IRT (ability),
  PFA (plateau detection), KST (prerequisite gating). Five algorithms updating
  the learner model on every interaction. The BKT → FSRS → IRT chain runs
  against a single read-only snapshot so order-of-evaluation no longer leaks
  cross-step state.
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
