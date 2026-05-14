# Tutor MCP v0.3.0-alpha.2

Second public alpha of Tutor MCP. This release adds a Complementary Learning
Systems-inspired learner memory layer and switches consolidation to a
client-initiated MCP pattern: the server detects consolidation work, the
connected LLM client authors the archive, and no server-side LLM provider or
API key is required.

## Highlights

- Markdown-backed learner memory:
  - `MEMORY.md`
  - `MEMORY_pending.md`
  - `sessions/*.md`
  - `concepts/*.md`
  - `archives/*.md`
- New MCP tools:
  - `update_learner_memory`
  - `read_raw_session`
  - `get_memory_state`
- `get_next_activity` can now return:
  - `episodic_context`
  - `pedagogical_contract.reasoning_request`
  - `consolidation_request`
- Pedagogical snapshots now store `interpretation_brief` for replay/audit.
- Consolidation jobs are queued in `pending_consolidations`, delivered to the
  client, and completed when the client writes the archive.
- The release binary now supports:

```bash
tutor-mcp --version
```

## Operational Notes

- Install URL remains unchanged:

```bash
curl -fsSL https://tutor-mcp.dev/install.sh | sh
```

- `latest` release assets include stable names such as
  `tutor-mcp_linux_amd64.tar.gz`, so installers do not need to know the tag.
- Memory is enabled by default and can be disabled with:

```bash
TUTOR_MCP_MEMORY_ENABLED=false
```

- Memory root defaults to `~/.tutor-mcp/` and can be changed with:

```bash
TUTOR_MCP_MEMORY_ROOT=/path/to/memory
```

## Validation

- `go test ./...`
- `go build ./...`

Full changelog: [`CHANGELOG.md`](CHANGELOG.md).
