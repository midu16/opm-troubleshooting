# OCP RAG MCP Server -- Tool Reference

## Overview

The OCP RAG MCP server provides AI-assisted troubleshooting for OpenShift Container Platform operators. It exposes 8 tools over the Model Context Protocol (MCP) stdio transport, giving LLM clients structured access to OCP documentation, operator source code, telco-reference configurations, known issues, and errata.

The server is built with `mark3labs/mcp-go` and backed by a local vector store. Each tool queries one or more collections using hybrid or vector retrieval, returning ranked results with source attribution.

Binary: `bin/ocp-rag-server`
Default config flag: `--config rag-config.yaml`

## Setup

### Claude Desktop / Claude Code

Add the server to your `.mcp.json` (project-level) or Claude Desktop configuration:

```json
{
  "mcpServers": {
    "ocp-rag": {
      "command": "/path/to/bin/ocp-rag-server",
      "args": ["--config", "/path/to/rag-config.yaml"]
    }
  }
}
```

### Environment Variables

| Variable | Description |
|---|---|
| `OCP_RAG_DATA_DIR` | Root directory for ingested data and vector store files |
| `OCP_RAG_EMBEDDING_URL` | URL of the embedding service endpoint |
| `OCP_RAG_EMBEDDING_MODEL` | Embedding model identifier to use for vectorization |
| `OCP_RAG_OCP_VERSION` | Default OCP version used when tools do not receive an explicit version parameter |

Environment variables override values set in `rag-config.yaml`.

## Tool Reference

### `search_docs`

Search OCP documentation.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search query for OCP documentation |

**Example usage:**

```
search_docs(query: "etcd backup and restore procedures")
```

---

### `search_operator_code`

Search operator Go source code.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search query for operator source code |
| `operator` | string | no | Filter by operator name (e.g. `cluster-etcd-operator`) |

**Example usage:**

```
search_operator_code(query: "pod disruption budget reconciliation", operator: "cluster-etcd-operator")
```

---

### `search_telco_configs`

Search telco-reference validated configurations.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search query for telco configurations |

**Example usage:**

```
search_telco_configs(query: "single-node OpenShift performance profile")
```

---

### `troubleshoot_operator`

Primary troubleshooting tool. Searches across all knowledge bases (docs, code, known issues, configs, manifests) to produce a consolidated troubleshooting response.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operator` | string | yes | Operator name to troubleshoot |
| `symptoms` | array of strings | no | List of observed symptoms or error messages |
| `ocp_version` | string | no | OCP version (defaults to configured version) |

**Example usage:**

```
troubleshoot_operator(
  operator: "cluster-etcd-operator",
  symptoms: ["EtcdMembersDegraded", "etcd leader changed 5 times in last hour"],
  ocp_version: "4.22"
)
```

---

### `get_operator_info`

Get comprehensive operator information including relevant documentation, known issues, and configurations.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operator` | string | yes | Operator name |

**Example usage:**

```
get_operator_info(operator: "cluster-monitoring-operator")
```

---

### `search_known_issues`

Search known issues and bugs for a specific operator.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operator` | string | yes | Operator name |
| `ocp_version` | string | no | OCP version filter |

**Example usage:**

```
search_known_issues(operator: "cluster-ingress-operator", ocp_version: "4.17")
```

---

### `search_errata`

Search errata advisories by OCP version.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `ocp_version` | string | yes | OCP version (e.g. `4.22`) |

**Example usage:**

```
search_errata(ocp_version: "4.22")
```

---

### `update_rag`

Re-ingest all data sources into the vector store. This operation fetches upstream content, re-embeds documents, and rebuilds collection indexes. It typically takes several minutes to complete.

| Parameter | Type | Required | Description |
|---|---|---|---|
| *(none)* | -- | -- | This tool accepts no parameters |

**Example usage:**

```
update_rag()
```

## Architecture Notes

### Collections

The vector store maintains 5 collections:

| Collection | Content | Retrieval Strategy |
|---|---|---|
| `ocp_docs` | OpenShift product documentation | Hybrid (keyword + vector) |
| `operator_code` | Operator Go source code | Hybrid (keyword + vector) |
| `telco_configs` | Telco-reference validated CR configurations | Plain vector |
| `known_issues` | Known issues and bugs from Bugzilla/Jira | Plain vector, version-aware filtering |
| `manifests` | Operator manifests and CRDs | Plain vector |

### Retrieval

- **Hybrid retrieval** (used by `ocp_docs` and `operator_code`) combines BM25 keyword matching with dense vector similarity, then fuses the ranked results. This improves recall for queries that mix natural language with exact identifiers such as Go function names or CRD field paths.
- **Plain vector retrieval** (used by `telco_configs`, `known_issues`, `manifests`) performs dense similarity search only.
- **Version-aware filtering** applies to the `known_issues` collection: when an `ocp_version` parameter is provided, results are filtered to issues that affect that version before ranking.

### Cross-collection queries

`troubleshoot_operator` and `get_operator_info` fan out queries to multiple collections in parallel and merge the results into a single response. Other tools target a single collection each.
