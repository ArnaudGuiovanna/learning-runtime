# Tutor MCP — first alpha (v0.3.0-alpha.1)

First public alpha of **Tutor MCP**, a deterministic Intelligent Tutoring
System runtime that any MCP-compatible LLM can drive (Claude, ChatGPT, Le Chat,
Gemini).

Generic LLM chat covers the *interface* of an ITS. Tutor MCP supplies the
three other pillars as pure runtime, calibrated in real time:

| ITS pillar | Provided by | How |
|---|---|---|
| **Domain model** | Tutor MCP runtime | KST-validated concept graph with prerequisites, cycle detection at `init_domain` |
| **Learner model** | Tutor MCP runtime | BKT × IRT × PFA, snapshot pattern keeps the update chain order-invariant |
| **Pedagogical model** | Tutor MCP runtime | FSRS spaced repetition, 7-stage regulation pipeline, alert engine, motivation engine, metacognitive loop |
| **Interface & content** | Any MCP-compatible LLM | 28 tools, chat-only surface |

## What works today

- Five complementary cognitive algorithms (BKT, FSRS, IRT, PFA, KST) updating
  the learner model on every interaction
- Seven-stage regulation pipeline driving activity selection through an
  explicit phase FSM (DIAGNOSTIC ↔ INSTRUCTION ↔ MAINTENANCE)
- 28 MCP tools, OAuth 2.1 + PKCE, refresh-token rotation with client binding,
  bcrypt cost 12, per-account login lockout, length caps on free-text params
- Versioned schema migrations with SHA-256 checksum drift detection
- Structured slog observability + 6 cron jobs (OLM, motivation, recap, mirror,
  cleanup, metacognitive alerts)

## Known limitations

Three RESEARCH items deferred — none block daily use:

- [#48 PFA fidelity to Pavlik 2009](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/48)
- [#49 IRT statistical robustness (EAP/MAP prior)](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/49)
- [#52 FSRS sub-day intervals](https://github.com/ArnaudGuiovanna/tutor-mcp/issues/52)

## Quickstart

```bash
git clone https://github.com/ArnaudGuiovanna/tutor-mcp
cd tutor-mcp
export JWT_SECRET="$(openssl rand -base64 32)"
go build -o tutor-mcp && ./tutor-mcp
```

See the README §[Setup workflow](https://github.com/ArnaudGuiovanna/tutor-mcp#setup-workflow)
for OAuth client registration and host-side connector setup (Claude Desktop,
Claude.ai, etc.).

## Caveats

- **Alpha** quality: the API surface may break between alphas. Schema
  migrations are forward-only — manual intervention required on body drift.
- **No CI** runs in this repo. `go build ./... && go test ./...` is the
  contract for contributors.
- **Go 1.25+** and **SQLite >= 3.35** required.

## Compatibility

- Tested with Claude Desktop and Claude.ai custom connectors.
- Stateless per-request; SQLite as the only persistent store.

---

Feedback welcome via [issues](https://github.com/ArnaudGuiovanna/tutor-mcp/issues).
Full changelog: [`CHANGELOG.md`](CHANGELOG.md).
