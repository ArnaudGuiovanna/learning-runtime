# MVP — exit criteria checklist

Source-of-truth tracker for the five MVP categories defined in the agent routine. Each row is updated when an open PR or merged commit changes its evidence link.

> **Status convention.** ✅ = met today. 🟡 = partial / pending merge of named PRs. ❌ = not addressed.

## 1. Fonctionnel

| Item | Status | Evidence |
|------|--------|----------|
| Tous les tools renvoient une réponse valide sur cas nominaux | ✅ | `go test ./tools/` green; one happy-path e2e in `tools/interaction_e2e_test.go` |
| Entrées vides / malformées gérées proprement | 🟡 | `tools/length_validation_test.go`, jsonschema validation; PR #103 closes the unbounded-text gap (#82) |
| Schémas I/O cohérents avec descriptions | ✅ | Canonical output-shape tests cover `mastery`, `global_progress_percent`, and `concept` without legacy output aliases (#86); `concept_id` remains input-only compatibility |

## 2. Pédagogique

| Item | Status | Evidence |
|------|--------|----------|
| BKT / FSRS / IRT update after `record_interaction` | ✅ | `algorithms/*_test.go`, `tools/interaction_test.go` |
| `get_next_activity` cohérent avec l'état cognitif | 🟡 | `tools/activity_test.go`; cross-domain leakage in MAINTENANCE pending PR #102 (#93); negotiation hallucination pending PR #100 (#92) |
| E2E « créer un domaine → 5 exercices → mastery progresse » | ✅ | `tools/interaction_e2e_test.go::TestEndToEnd_TenSuccessesMoveMastery` |
| E2E coherence multi-tool (archived/deleted domain, multi-domain, calibration round-trip) | ❌ | Issue #97 — scenarios 5-9 to be added this cycle, scenarios 1-4 wait on the underlying fix PRs |

## 3. Signal LLM

| Item | Status | Evidence |
|------|--------|----------|
| Sorties JSON structurées (mastery, confidence, hints) | ✅ | `jsonResult` + `StructuredContent` in `tools/tools.go` |
| Descriptions de tools sans ambiguïté pour le routing | 🟡 | PR #104 covers the alerts/activity/mirror trio (#84); other tools still rely on legacy descriptions |
| Champs documentés et utilisés downstream | ✅ | `get_learner_context` exposes `priority_concept_domain_id` for routable priority concepts (#154); dashboard units document 0..1 vs 0..100 fields |

## 4. Sécurité

| Item | Status | Evidence |
|------|--------|----------|
| Validation des inputs sur tous les endpoints | 🟡 | PR #101 enums for activity_type/error_type (#88); PR #99 actual_score [0,1] (#83); PR #103 length caps (#82); profile NaN/Inf guard pending (#85) |
| Rate-limiting sur endpoints coûteux | ✅ | `auth.RateLimitMiddleware` on /authorize, /token, /register, /mcp (`main.go:113-121`) |
| Injection impossible via champs texte | ✅ | All DB writes use parameterised SQL; jsonschema enforces basic typing |
| Defence-in-depth: per-learner DB filters | 🟡 | Issue #87 — `CompleteCalibrationRecord` / `GetCalibrationRecord` lack `learner_id` filter at DB layer; PR target this cycle |
| Pas de secrets en clair dans les logs | ✅ | `auth/jwt.go` redacts secrets; `requestLogger` logs only method/path/status/UA |
| Domain ownership / archived enforcement | 🟡 | PR #98 closes the archived-domain leak in `resolveDomain` (#94); cross-domain MAINTENANCE leak in PR #102 (#93) |

## 5. UX

| Item | Status | Evidence |
|------|--------|----------|
| Messages d'erreur explicites et actionnables | 🟡 | `errorResult(...)` everywhere; mixed-language gap tracked by issue #90 (not addressed) |
| Latence p95 < 2 s (tools sans LLM) | ✅ | Opt-in CI/release gate in `tools/activity_pair_perf_test.go` covers `get_pending_alerts` + `get_next_activity`; observed p95 17.98 ms at 200 active learners with `MCP_PERF_BUDGET=1 MCP_PERF_ACTIVE_LEARNERS=200` |
| Latence p95 < 8 s (tools avec sampling) | ❌ | No measurement harness today |
| README à jour avec quickstart vérifié | 🟡 | `README.md` / `CHANGELOG.md` synced with the current `engine/orchestrator.go` regulation runtime and `docs/regulation-design/`; quickstart still needs a fresh sanity check |

## Decision gates

- The MVP is **NOT** reached this cycle. Remaining blockers:
  - The sampling/LLM latency gate and a fresh quickstart sanity-check are still pending.
- When all rows in this checklist are ✅ AND a documented quickstart sanity-check passes against `staging`, the MVP gate is open. The version bump and tag are humain-only steps after `staging` → `main`.
