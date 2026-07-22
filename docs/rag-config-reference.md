# rag-config.yaml Configuration Reference

This document describes every field accepted by `rag-config.yaml`, the
configuration file for the OPM Troubleshooting RAG pipeline.  When the file is
absent the tool runs with built-in defaults.  Fields present in the file are
merged on top of those defaults, so you only need to specify what you want to
change.

## Top-level structure

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| DataDir | `data_dir` | string | `"rag-data"` | Root directory for all cloned repos, docs, and vector-store data. Subdirectories `repos/`, `docs/`, `chromem/`, and `telco-reference/` are created beneath it automatically. |
| Embedding | `embedding` | object | see below | Embedding service connection settings. |
| OpenShift | `openshift` | object | see below | Target OpenShift version(s) and operator repository list. |
| Retrieval | `retrieval` | object | see below | Top-K limits for each retrieval category. |
| Chunking | `chunking` | object | see below | Reserved for future use. Controls text chunk sizing. |
| Ingestion | `ingestion` | object | see below | Timeouts and parallelism for git/HTTP operations during data ingestion. |
| Secret | `secret_filter` | object | see below | File-extension and filename patterns to exclude from ingestion to avoid leaking secrets. |
| Freshness | `freshness` | object | see below | Metadata file used to track ingestion freshness. |

---

## embedding

Settings for the embedding model endpoint (default: a local Ollama instance).

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| URL | `url` | string | `"http://localhost:11434"` | Base URL of the embedding service (Ollama-compatible API). |
| Model | `model` | string | `"all-minilm"` | Name of the embedding model to request from the service. |

---

## openshift

Defines which OpenShift version(s) and operator repositories are ingested.

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| Version | `version` | string | `"4.22"` | The active OpenShift version. Used to select the matching `VersionEntry` from `versions`, and as the basis for branch-name synthesis when `versions` is empty. |
| DocsBaseURL | `docs_base_url` | string | `"https://docs.redhat.com/..."` | **Deprecated.** Previously pointed at the hosted documentation. Docs are now cloned directly from GitHub. The field is retained only for backward compatibility and is ignored at runtime. |
| Repos | `repos` | []string | 27 operator repos (see full list below) | GitHub repository names under `openshift/` to clone and index. |
| Versions | `versions` | []VersionEntry | `[]` (empty) | Explicit per-version branch mappings. See the **VersionEntry** section and the multi-version example below. |

### Default repos list

The built-in default includes the following 27 repositories (all under the
`openshift` GitHub organization):

```
cluster-etcd-operator
cluster-kube-apiserver-operator
cluster-kube-controller-manager-operator
cluster-kube-scheduler-operator
cluster-version-operator
machine-config-operator
cluster-network-operator
cluster-ingress-operator
cluster-storage-operator
cluster-monitoring-operator
cluster-authentication-operator
local-storage-operator
sriov-network-operator
ptp-operator
cluster-nfd-operator
metallb-operator
cluster-node-tuning-operator
cluster-dns-operator
cluster-image-registry-operator
cluster-samples-operator
cluster-logging-operator
openshift-apiserver
console-operator
installer
oc
api
kubernetes-nmstate
```

### VersionEntry

Each entry in `openshift.versions` maps a version string to the branch names
used when cloning documentation, operator source, and telco-reference content.

| Field | YAML key | Type | Description |
|---|---|---|---|
| Version | `version` | string | OpenShift version identifier (e.g. `"4.22"`). |
| DocsBranch | `docs_branch` | string | Branch of the documentation repo to clone (e.g. `"enterprise-4.22"`). |
| OperatorBranch | `operator_branch` | string | Branch of each operator repo to clone (e.g. `"release-4.22"`). |
| TelcoBranch | `telco_branch` | string | Branch of the telco-reference repo to clone (e.g. `"release-4.22"`). |

#### Backward compatibility

If `versions` is empty (or omitted), a single `VersionEntry` is synthesized
automatically from the `version` field using the following convention:

```
docs_branch:     "enterprise-<version>"
operator_branch: "release-<version>"
telco_branch:    "release-<version>"
```

This means existing configurations that only set `openshift.version` continue to
work without modification.

---

## retrieval

Controls how many results each retrieval category returns.

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| DefaultTopK | `default_top_k` | int | `8` | Maximum number of results for general/untyped queries. |
| CodeTopK | `code_top_k` | int | `6` | Maximum number of results for code-related queries. |
| ConfigTopK | `config_top_k` | int | `4` | Maximum number of results for configuration-related queries. |
| IssuesTopK | `issues_top_k` | int | `5` | Maximum number of results for issue/bug-related queries. |
| KeywordSupplementMax | `keyword_supplement_max` | int | `4` | Maximum number of extra keyword-matched results appended after vector search. |
| KeywordMinLength | `keyword_min_length` | int | `4` | Minimum character length for a keyword to be considered in supplemental search. |

---

## chunking

Reserved for future use.  These values are read from config but chunking
behavior is not yet user-tunable.

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| ChunkSize | `chunk_size` | int | `1200` | Target size (in characters) for each text chunk. |
| ChunkOverlap | `chunk_overlap` | int | `200` | Number of overlapping characters between consecutive chunks. |

---

## ingestion

Timeouts and concurrency settings for the ingestion phase (git clone, HTTP
fetches).

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| GitTimeout | `git_timeout` | duration string | `"60s"` | Maximum time allowed for a single `git clone` or `git fetch` operation. Accepts Go duration syntax (e.g. `"90s"`, `"2m"`). |
| HTTPTimeout | `http_timeout` | duration string | `"30s"` | Maximum time allowed for a single HTTP request during ingestion. |
| MaxParallelClones | `max_parallel_clones` | int | `4` | Number of git clone operations to run concurrently. |

---

## secret_filter

File patterns excluded from ingestion to prevent accidental indexing of
credentials or private keys.

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| SkipExtensions | `skip_extensions` | []string | `[".pem", ".crt", ".key", ".p12", ".jks", ".pfx", ".enc", ".gpg"]` | File extensions to skip during ingestion. |
| SkipFilenames | `skip_filenames` | []string | `[".env", "pull-secret", "kubeconfig", "credentials", "htpasswd", "id_rsa", "id_ed25519"]` | Exact filenames to skip during ingestion. |

---

## freshness

Controls the metadata file that tracks when each source was last ingested.

| Field | YAML key | Type | Default | Description |
|---|---|---|---|---|
| MetaFile | `meta_file` | string | `".ingest_meta.json"` | Filename (relative to `data_dir`) written after each successful ingestion run. |

---

## Environment variable overrides

The following environment variables, when set, override their corresponding
config-file values.  They are applied after the YAML file is loaded.

| Variable | Overrides | Example |
|---|---|---|
| `OCP_RAG_DATA_DIR` | `data_dir` | `OCP_RAG_DATA_DIR=/var/lib/rag-data` |
| `OCP_RAG_EMBEDDING_URL` | `embedding.url` | `OCP_RAG_EMBEDDING_URL=http://gpu-host:11434` |
| `OCP_RAG_EMBEDDING_MODEL` | `embedding.model` | `OCP_RAG_EMBEDDING_MODEL=nomic-embed-text` |
| `OCP_RAG_OCP_VERSION` | `openshift.version` | `OCP_RAG_OCP_VERSION=4.21` |

---

## Full annotated example

The following YAML shows every field with its default value.

```yaml
# Root directory for cloned repos, docs, and vector DB files.
data_dir: "rag-data"

# Embedding service settings (Ollama-compatible endpoint).
embedding:
  url: "http://localhost:11434"
  model: "all-minilm"

# OpenShift target configuration.
openshift:
  version: "4.22"

  # Deprecated -- docs are cloned from GitHub; this field is ignored.
  # docs_base_url: "https://docs.redhat.com/..."

  # Operator repos to clone (under github.com/openshift/).
  repos:
    - cluster-etcd-operator
    - cluster-kube-apiserver-operator
    - cluster-kube-controller-manager-operator
    - cluster-kube-scheduler-operator
    - cluster-version-operator
    - machine-config-operator
    - cluster-network-operator
    - cluster-ingress-operator
    - cluster-storage-operator
    - cluster-monitoring-operator
    - cluster-authentication-operator
    - local-storage-operator
    - sriov-network-operator
    - ptp-operator
    - cluster-nfd-operator
    - metallb-operator
    - cluster-node-tuning-operator
    - cluster-dns-operator
    - cluster-image-registry-operator
    - cluster-samples-operator
    - cluster-logging-operator
    - openshift-apiserver
    - console-operator
    - installer
    - oc
    - api
    - kubernetes-nmstate

# Retrieval top-K limits per category.
retrieval:
  default_top_k: 8
  code_top_k: 6
  config_top_k: 4
  issues_top_k: 5
  keyword_supplement_max: 4
  keyword_min_length: 4

# Chunking (reserved for future use).
chunking:
  chunk_size: 1200
  chunk_overlap: 200

# Ingestion timeouts and parallelism.
ingestion:
  git_timeout: "60s"
  http_timeout: "30s"
  max_parallel_clones: 4

# Files excluded from ingestion to avoid indexing secrets.
secret_filter:
  skip_extensions:
    - ".pem"
    - ".crt"
    - ".key"
    - ".p12"
    - ".jks"
    - ".pfx"
    - ".enc"
    - ".gpg"
  skip_filenames:
    - ".env"
    - "pull-secret"
    - "kubeconfig"
    - "credentials"
    - "htpasswd"
    - "id_rsa"
    - "id_ed25519"

# Freshness tracking metadata file.
freshness:
  meta_file: ".ingest_meta.json"
```

---

## Multi-version example

To ingest and query across multiple OpenShift versions, populate the `versions`
list.  The `version` field selects which entry is the "active" version used at
query time.

```yaml
data_dir: "rag-data"

openshift:
  # The active version -- retrieval queries target this version by default.
  version: "4.22"

  repos:
    - cluster-etcd-operator
    - cluster-kube-apiserver-operator
    - machine-config-operator
    - cluster-network-operator

  # Explicit branch mappings per version.
  versions:
    - version: "4.21"
      docs_branch: "enterprise-4.21"
      operator_branch: "release-4.21"
      telco_branch: "release-4.21"

    - version: "4.22"
      docs_branch: "enterprise-4.22"
      operator_branch: "release-4.22"
      telco_branch: "release-4.22"

    - version: "4.23"
      docs_branch: "enterprise-4.23"
      operator_branch: "release-4.23"
      telco_branch: "release-4.23"
```

When `versions` is populated, all listed versions are ingested.  The entry
whose `version` matches `openshift.version` is returned by `ActiveVersionEntry()`
and used for default retrieval.  If no entry matches, a version entry is
synthesized from the `version` field using the standard branch-name convention.

When `versions` is empty or omitted entirely, the tool synthesizes a single
version entry from `openshift.version`, preserving full backward compatibility
with configurations written before multi-version support was added.
