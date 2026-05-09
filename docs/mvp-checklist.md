# MVP — exit criteria checklist

Source-of-truth tracker for the five MVP categories defined in the agent routine. Each row is updated when an open PR or merged commit changes its evidence link.

> **Status convention.** ✅ = met today. 🟡 = partial / pending merge of named PRs. ❌ = not addressed.

## 1. Fonctionnel

| Item | Status | Evidence |
|------|--------|----------|
| Tous les tools renvoient une réponse valide sur cas nominaux | ✅ | `go test ./tools/` green; one happy-path e2e in `tools/interaction_e2e_test.go` |
| Entrées vides / malformées gérées proprement | 🟡 | `tools/length_validation_test.go`, jsonschema validation; PR #103 closes the unbounded-text gap (#82) |
| Schémas I/O cohérents avec descriptions | 🟡 | PR #104 disambiguates the read-tool routing trio (#84). API-shape consistency (concept vs concept_id) tracked by issue #86 — not yet addressed |

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
| Champs documentés et utilisés downstream | 🟡 | Issue #86 (output-shape inconsistency: concept vs concept_id, mastery 0..1 vs 0..100) — not yet addressed |

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
| Latence p95 < 2 s (tools sans LLM) | ❌ | No measurement harness today; perf risk on `get_next_activity` (8+ DB calls per turn) tracked by issue #91 |
| Latence p95 < 8 s (tools avec sampling) | ❌ | No measurement harness today |
| README à jour avec quickstart vérifié | 🟡 | `README.md` exists; not re-validated against current `main.go` and tool surface |

## Decision gates

- The MVP is **NOT** reached this cycle. Major blockers:
  - Issues #86, #90, #91 are unaddressed (output-shape consistency, mixed-language errors, perf budget).
  - 9 PRs still pending human merge.
- When all rows in this checklist are ✅ AND a documented quickstart sanity-check passes against `staging`, the MVP gate is open. The version bump and tag are humain-only steps after `staging` → `main`.
