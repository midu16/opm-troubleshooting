# RAG Ingestion Pipeline Guide

This guide covers the RAG ingestion pipeline that populates the vector store collections used by the MCP server and CLI query tool.

## Prerequisites

- **Go 1.26+** for building the ingestion binary
- **Ollama** running locally with an embedding model pulled (e.g. `qwen3-embedding:latest` or `all-minilm`)
- **git** for cloning upstream repositories
- **Sufficient disk space** -- approximately 2GB or more for 27 operator repos plus documentation

## Running

```bash
# Build
make build

# Run ingestion
bin/ocp-rag-ingest --config rag-config.yaml

# Check freshness
bin/ocp-rag-query --freshness
```

## Pipeline Steps

The ingestion is orchestrated by `RunIngestion` in `internal/rag/ingest/ingest.go`. It executes the following steps in order.

### 1. Create data directories

Creates `rag-data/repos/` and `rag-data/docs/` if they do not already exist.

### 2. Clone/update operator repos

Clones or updates 27 OpenShift operator repositories from GitHub concurrently. Concurrency is bounded by `max_parallel_clones` in the config (default 4). Each repo is cloned with `--depth=1` (shallow clone) and, when a branch is configured, with `--branch <operator_branch>`. Already-cloned repos are updated with `git pull --ff-only`.

The default repo list includes operators such as `cluster-etcd-operator`, `machine-config-operator`, `cluster-network-operator`, `sriov-network-operator`, `ptp-operator`, `cluster-node-tuning-operator`, and 21 others. The full list is defined in the config defaults in `internal/rag/config.go`.

After cloning, the HEAD commit hash of each repo is recorded for freshness tracking.

### 3. Load telco-reference configs

Clones or updates `openshift-kni/telco-reference` at the configured branch (e.g. `release-4.22`). Loads all YAML manifests and Markdown files from the `telco-core/` and `telco-ran/` subdirectories. Each document is tagged with its telco profile (`telco-core` or `telco-ran`).

### 4. Scrape OCP docs

Clones `openshift/openshift-docs` using a sparse checkout to limit download size. Only 28 topic directories are checked out:

`installing`, `post_installation_configuration`, `networking`, `storage`, `security`, `authentication`, `cicd`, `builds`, `operators`, `monitoring`, `observability`, `support`, `backup_and_restore`, `updating`, `nodes`, `registry`, `machine_management`, `rest_api`, `architecture`, `web_console`, `cli_reference`, `images`, `applications`, `scalability_and_performance`, `virt`, `windows_containers`, `service_mesh`, `serverless`, `edge_computing`

AsciiDoc files in these directories are split into chunks by heading boundaries (= through ====). Comment blocks, include directives, and conditional preprocessor lines are stripped. Each chunk becomes a document with section and breadcrumb metadata.

### 5. Build and index collections

Five vector store collections are populated:

| Collection | Source | Chunking strategy |
|---|---|---|
| `ocp_docs` | AsciiDoc files from `openshift/openshift-docs` | Split by heading (= through ====) |
| `operator_code` | Go source from 27 operator repos | One document per top-level declaration (function, type, const, var) parsed with `go/parser`; scans `internal/`, `api/`, `pkg/`, `cmd/`, `controllers/` directories; skips test files, generated files (`zz_generated*`), and vendor directories |
| `telco_configs` | YAML and Markdown from `openshift-kni/telco-reference` | YAML files kept as single documents; Markdown split by heading (# through ####) |
| `known_issues` | 12 hardcoded known operator issues | One document per issue, covering etcd, MCO, CNO, storage, SR-IOV, PTP, monitoring, authentication, ingress, local-storage, NTO, and CVO |
| `manifests` | YAML files from telco-reference plus CRD files from operator repos (`manifests/`, `config/crd/`, `deploy/` directories) | One document per YAML file |

Each existing collection is reset before documents are added.

### 6. Save freshness metadata

Writes ingestion metadata to `.ingest_meta.json`, including timestamps, collection names, and the HEAD commit hash of each cloned repo. This is used by the freshness checking system.

## Secret Filtering

The pipeline filters sensitive content at two levels.

**File-level exclusion:** Files matching configured `skip_extensions` or `skip_filenames` are excluded entirely. Defaults:

- Extensions: `.pem`, `.crt`, `.key`, `.p12`, `.jks`, `.pfx`, `.enc`, `.gpg`
- Filenames: `.env`, `pull-secret`, `kubeconfig`, `credentials`, `htpasswd`, `id_rsa`, `id_ed25519`

**Content-level filtering (YAML files):**

- Kubernetes `Secret` and `SealedSecret` resources are skipped entirely
- Passwords, tokens, and API keys are redacted (`[REDACTED-PASSWORD]`, `[REDACTED-TOKEN]`)
- Bearer tokens are redacted (`Bearer [REDACTED]`)
- Base64 blobs of 40+ characters are redacted (`[REDACTED-BASE64]`)
- SSH private key blocks are redacted (`[REDACTED-SSH-KEY]`)

## Multi-Version Ingestion

When `versions` is configured in `rag-config.yaml`, the pipeline uses branch-specific cloning for each version entry:

- Operator repos are cloned with `--branch <operator_branch>`
- The docs repo is cloned with `--branch <docs_branch>`
- Telco reference is cloned with `--branch <telco_branch>`

All documents are tagged with `ocp_version` metadata, enabling version-aware retrieval at query time.

## Freshness Checking

Run the freshness check to verify the knowledge base is up to date:

```bash
bin/ocp-rag-query --freshness
```

This compares the commit hashes stored in `.ingest_meta.json` against the current HEAD of each upstream repository. If any repo has advanced, the knowledge base is reported as stale.

## Configuration Reference

The ingestion pipeline is configured through `rag-config.yaml`. Key settings:

- `data_dir` -- base directory for cloned repos and docs (default: `rag-data`)
- `embedding.url` -- Ollama API endpoint (default: `http://localhost:11434`)
- `embedding.model` -- embedding model name (default: `all-minilm`)
- `openshift.version` -- OCP version string used in metadata
- `openshift.repos` -- list of operator repository names to clone
- `ingestion.max_parallel_clones` -- concurrent clone limit (default: 4)
- `ingestion.git_timeout` -- per-repo git operation timeout (default: 60s)
- `secret.skip_extensions` -- file extensions to exclude
- `secret.skip_filenames` -- filenames to exclude
- `versions` -- list of version entries for multi-version ingestion, each with `operator_branch`, `docs_branch`, and `telco_branch`
