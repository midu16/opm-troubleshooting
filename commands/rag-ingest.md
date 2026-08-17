---
description: Build or refresh the OpenShift RAG knowledge base (clone repos, scrape docs, rebuild vector store)
argument-hint: "[--config <path>]"
---

## Name
opm-troubleshooting:rag-ingest

## Synopsis
```
/opm-troubleshooting:rag-ingest [--config <path>]
```

## Description
The `opm-troubleshooting:rag-ingest` command populates the OpenShift RAG knowledge base
by wrapping the `ocp-rag-ingest` binary. It clones the operator repositories listed in
`rag-config.yaml`, scrapes the configured OCP documentation, embeds everything with the
configured embedding model, and writes the local vector store under `data_dir`
(default: `rag-data/`).

Run this **once before your first query**, and again whenever you want to refresh the
content. Ingestion takes several minutes.

## Implementation

Run the `ocp-rag-ingest` binary from the plugin/project root, building it from source
first if it is not present.

```bash
ROOT="${CLAUDE_PLUGIN_ROOT:-.}"
cd "$ROOT"

if [ ! -x ./bin/ocp-rag-ingest ]; then
  make ocp-rag-ingest || go build -o ./bin/ocp-rag-ingest ./cmd/ocp-rag-ingest
fi

./bin/ocp-rag-ingest --config "$ROOT/rag-config.yaml" $ARGUMENTS
```

Progress is written to stderr. When it finishes it prints `Ingestion complete.`

## Arguments

- `--config` (optional): RAG config path (defaults to the plugin's `rag-config.yaml`)

## Prerequisites

- The embedding backend configured under `embedding:` in `rag-config.yaml` must be
  reachable (URL and model).
- Network access to clone the operator repos and scrape the OCP documentation.

## Examples

### Example 1: Ingest with the default config
```bash
/opm-troubleshooting:rag-ingest
```

### Example 2: Ingest with a custom config
```bash
/opm-troubleshooting:rag-ingest --config /path/to/rag-config.yaml
```

## Notes

- Re-running re-ingests all sources; use it to pick up new operator versions or docs.
- After ingesting, verify with `/opm-troubleshooting:rag-query --freshness`.

## See Also

- `opm-troubleshooting:rag-query` — query the knowledge base
- `opm-troubleshooting:rag-server` — the MCP server that exposes search to Claude
- [docs/asking-questions-and-searching-docs.md](../docs/asking-questions-and-searching-docs.md)
