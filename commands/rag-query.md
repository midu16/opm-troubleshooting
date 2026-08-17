---
description: Query the OpenShift RAG knowledge base (docs, code, telco, issues, manifests) from the terminal
argument-hint: "[--collection docs|code|telco|issues|manifests] [--operator <name>] [--json] [--freshness] <query>"
---

## Name
opm-troubleshooting:rag-query

## Synopsis
```
/opm-troubleshooting:rag-query [--collection <name>] [--operator <name>] [--json] <query>
/opm-troubleshooting:rag-query --freshness
```

## Description
The `opm-troubleshooting:rag-query` command searches the OpenShift Container Platform
RAG (Retrieval-Augmented Generation) knowledge base built by `ocp-rag-ingest`. It wraps
the `ocp-rag-query` binary so you can ask documentation, source-code, telco-config, and
known-issue questions directly from a slash command.

This is the CLI counterpart to the `ocp-rag` MCP server: the MCP server lets Claude
search on your behalf during a conversation, while this command gives you precise,
scriptable control over the collection, the operator filter, and the output format.

**The knowledge base must be populated first** — run `/opm-troubleshooting:rag-ingest`
(or `./bin/ocp-rag-ingest`) at least once, otherwise every query returns no results.

## Implementation

Run the `ocp-rag-query` binary from the plugin/project root, passing the user's
arguments through verbatim. Build the binary from source first if it is not present
(the `bin/` directory is not shipped with the plugin).

```bash
ROOT="${CLAUDE_PLUGIN_ROOT:-.}"
cd "$ROOT"

# Build on first use if the binary is missing
if [ ! -x ./bin/ocp-rag-query ]; then
  make ocp-rag-query || go build -o ./bin/ocp-rag-query ./cmd/ocp-rag-query
fi

# Run the query. $ARGUMENTS is whatever the user typed after the command.
./bin/ocp-rag-query --config "$ROOT/rag-config.yaml" $ARGUMENTS
```

Report the tool's output back to the user. If the result is empty, remind them to run
`/opm-troubleshooting:rag-ingest` first and to verify freshness with `--freshness`.

## Arguments

- `<query>` (required unless `--freshness`): natural-language search text
- `--collection` (optional): which collection to search
  - *(omitted)* — troubleshoot across everything (summary + known issues + doc refs + config advice)
  - `docs` — OpenShift Container Platform documentation
  - `code` — operator Go source code
  - `telco` — telco-reference validated configurations
  - `issues` — known issues/bugs (use with `--operator`)
  - `manifests` — manifest/config snippets
- `--operator` (optional): scope `code`/`issues` (and the default troubleshoot) to one operator (e.g. `cluster-etcd-operator`)
- `--json` (optional): emit raw JSON instead of formatted text
- `--freshness` (optional): report knowledge-base freshness and exit
- `--config` (optional): RAG config path (defaults to the plugin's `rag-config.yaml`)

## Examples

### Example 1: Search the docs (the canonical case)
```bash
/opm-troubleshooting:rag-query --collection docs --json "OADP backup failure"
```

### Example 2: Ask across all knowledge bases
```bash
/opm-troubleshooting:rag-query "etcd leader election timeout"
```

### Example 3: Search operator source code, scoped to one operator
```bash
/opm-troubleshooting:rag-query --collection code --operator cluster-etcd-operator "reconcile loop error handling"
```

### Example 4: Check whether the knowledge base is up to date
```bash
/opm-troubleshooting:rag-query --freshness
```

## Notes

- Requires the RAG knowledge base to be ingested first (`/opm-troubleshooting:rag-ingest`).
- The embedding backend configured under `embedding:` in `rag-config.yaml` must be reachable.
- The same knowledge base powers the `ocp-rag` MCP server tools (`search_docs`, `search_operator_code`, …).

## See Also

- `opm-troubleshooting:rag-ingest` — build/refresh the knowledge base
- `opm-troubleshooting:rag-server` — the MCP server that lets Claude search for you
- [docs/asking-questions-and-searching-docs.md](../docs/asking-questions-and-searching-docs.md)
