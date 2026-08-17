---
description: Build and manage the ocp-rag MCP server that exposes RAG search to Claude
argument-hint: "[--config <path>]"
---

## Name
opm-troubleshooting:rag-server

## Synopsis
```
/opm-troubleshooting:rag-server [--config <path>]
```

## Description
The `opm-troubleshooting:rag-server` command manages the `ocp-rag-server` binary — the
MCP (Model Context Protocol) server that exposes the RAG knowledge base to Claude as
tools. Unlike `rag-query`/`rag-ingest`, this server is **long-running** and is normally
launched automatically by Claude Code via the plugin's `.mcp.json`, not invoked
per-question.

Use this command to **build the server binary** and to check that the `ocp-rag` MCP
server is wired up. Once the binary exists and the MCP server is registered, just ask
Claude in natural language and it will call the right tool.

### Tools the MCP server exposes

| Tool | Purpose |
|------|---------|
| `search_docs` | Search OCP documentation |
| `search_operator_code` | Search operator Go source (optional `operator` filter) |
| `search_telco_configs` | Search telco-reference configs |
| `troubleshoot_operator` | Full diagnosis across all knowledge bases |
| `get_operator_info` | Docs + issues + reference configs for one operator |
| `search_known_issues` | Known issues/bugs for an operator |
| `search_errata` | Errata / known issues by OCP version |
| `update_rag` | Re-ingest everything (slow) |

## Implementation

Build the binary from source if missing, then confirm the MCP wiring. Do **not** start
the server in the foreground for interactive use — Claude Code launches it over stdio
from `.mcp.json`.

```bash
ROOT="${CLAUDE_PLUGIN_ROOT:-.}"
cd "$ROOT"

# Build on first use if the binary is missing
if [ ! -x ./bin/ocp-rag-server ]; then
  make ocp-rag-server || go build -o ./bin/ocp-rag-server ./cmd/ocp-rag-server
fi

echo "ocp-rag-server binary: $ROOT/bin/ocp-rag-server"
echo "MCP registration (.mcp.json):"
cat "$ROOT/.mcp.json"
```

After building, run `/reload-plugins` (or restart Claude Code) so the `ocp-rag` MCP
server picks up the freshly built binary. Then ask Claude a question like
*"Search the OCP docs for SR-IOV configuration."*

## Arguments

- `--config` (optional): RAG config path (defaults to the plugin's `rag-config.yaml`)

## Notes

- The knowledge base must be ingested first (`/opm-troubleshooting:rag-ingest`).
- `.mcp.json` uses an absolute path to the binary — adjust it if you build elsewhere.
- To run the server manually (for debugging over stdio):
  `./bin/ocp-rag-server --config rag-config.yaml`

## See Also

- `opm-troubleshooting:rag-query` — query the knowledge base from the CLI
- `opm-troubleshooting:rag-ingest` — build/refresh the knowledge base
- [docs/asking-questions-and-searching-docs.md](../docs/asking-questions-and-searching-docs.md)
