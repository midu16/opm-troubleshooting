# Vector Store Persistence

This document explains how the OPM Troubleshooting RAG knowledge base is
persisted, and how to move from the default single-machine store to a shared,
always-on vector database that many clients can query at once.

## The persistence seam

All retrieval and ingestion code talks to a single Go interface,
`rag.VectorStore` (`internal/rag/vectorstore.go`):

```go
type VectorStore interface {
    AddDocuments(ctx, coll, docs) error
    Search(ctx, coll, query, topK) ([]Document, error)
    SearchWithFilter(ctx, coll, query, topK, where) ([]Document, error)
    KeywordSearch(ctx, coll, keyword, limit) ([]Document, error)
    Reset(coll) error
    CollectionCount(coll) int
}
```

`NewVectorStore(cfg, embedFunc)` picks the concrete backend from
`vector_store.backend` in `rag-config.yaml`. Both the query engine and the
ingestion pipeline depend only on the interface, so the storage backend can be
swapped without touching retrieval logic.

## Two backends

| | `chromem` (default) | `qdrant` |
|---|---|---|
| Type | Embedded, in-process, pure Go | External network service |
| Persistence | Files under `data_dir/chromem/` | Qdrant's own storage (disk/volume) |
| Sharing | One process at a time | Many concurrent readers |
| Ops overhead | None | Run/operate a Qdrant instance |
| Best for | Laptops, CI, single user | Teams, servers, always-on MCP |

### chromem — embedded and local

The default. `NewStore(cfg.ChromemDir(), embedFunc)` opens a persistent
chromem-go database on local disk. The index survives restarts, but it lives
inside one process's data directory and is not meant for concurrent writers.
This is the zero-dependency path and requires no configuration.

### qdrant — external and shared

When `vector_store.backend: qdrant`, `NewQdrantStore` connects to an external
Qdrant service over its HTTP REST API using only the Go standard library (no
client SDK — the pure-Go / `CGO_ENABLED=0` build is preserved).

Mapping:

- Each RAG collection (`docs`, `code`, `telco`, `issues`, `manifests`,
  `acm_docs`) becomes one Qdrant collection named
  `<collection_prefix><collection>`.
- Collections are created lazily on first ingestion with the configured
  `distance` metric and the embedding vector size discovered at runtime.
- A full-text payload index is created on the document `content` field so
  `KeywordSearch` (the keyword half of hybrid retrieval) works server-side.
- Document text and metadata are stored in each point's payload; the original
  document ID is preserved as a deterministic UUIDv5 point ID, making
  re-ingestion idempotent.

Embeddings are still computed locally with the same `EmbeddingFunc` used by the
chromem backend, then upserted as vectors — Qdrant stores and searches vectors;
it does not embed text itself.

## Recommended architecture: one writer, many readers

This is the deployment the project targets for shared use:

```
                         embeddings
   ┌──────────────┐   (Ollama-compatible)   ┌───────────────┐
   │ ocp-rag-     │◄───────────────────────►│  Embedding    │
   │ ingest       │                          │  service      │
   │ (WRITER)     │──── upsert vectors ─┐    └───────────────┘
   └──────────────┘                     │
                                        ▼
                                 ┌─────────────┐
                                 │   Qdrant    │  ← persistent volume
                                 │  (shared)   │
                                 └─────────────┘
                                        ▲
        ┌───────────────────────────────┼───────────────────────────┐
        │                                │                           │
  ┌───────────┐                   ┌───────────┐               ┌───────────┐
  │ ocp-rag-  │                   │ ocp-rag-  │               │  MCP      │
  │ query     │                   │ server    │               │  clients  │
  │ (READER)  │                   │ (READER)  │               │ (READERS) │
  └───────────┘                   └───────────┘               └───────────┘
```

- **One writer.** A single scheduled `ocp-rag-ingest` run refreshes the index
  (clone repos → chunk → embed → upsert). `Reset` + re-`AddDocuments` per
  collection keeps ingestion idempotent.
- **Many readers.** Any number of `ocp-rag-query` invocations, `ocp-rag-server`
  MCP servers, and their clients read concurrently from the same Qdrant
  collections. Readers never mutate the store.
- **Persistence.** The index outlives every client process — it lives in
  Qdrant's storage volume, not in any one client's working directory.

## Running Qdrant

A minimal local instance for testing:

```bash
podman run -p 6333:6333 -p 6334:6334 \
  -v "$PWD/qdrant_storage:/qdrant/storage:z" \
  docker.io/qdrant/qdrant
```

Mount a persistent volume (as above) or use a managed Qdrant so the data
survives container restarts. For production, secure it with an API key and TLS.

## Switching a deployment to Qdrant

1. Start/point at a Qdrant service (persistent volume, optional API key).
2. In `rag-config.yaml`:

   ```yaml
   vector_store:
     backend: "qdrant"
     qdrant:
       url: "http://qdrant.internal:6333"   # your FQDN
       api_key: ""                           # set if secured
       collection_prefix: "ocp422_"          # optional namespacing
       distance: "Cosine"
       timeout: "30s"
   ```

   Or set it entirely from the environment (nothing is hard-coded):

   ```bash
   export OCP_RAG_VECTORSTORE_BACKEND=qdrant
   export OCP_RAG_QDRANT_URL=http://qdrant.internal:6333
   export OCP_RAG_QDRANT_API_KEY=***          # if secured
   ```

3. Run ingestion **once** (the writer) to populate Qdrant:

   ```bash
   ./bin/ocp-rag-ingest --config rag-config.yaml
   ```

4. Point every reader (`ocp-rag-query`, `ocp-rag-server`, MCP clients) at the
   same config. They immediately share the persisted index.

> **Consistency note.** Keep the embedding model identical across the writer and
> all readers. Vectors are only comparable when produced by the same model, so
> changing `embedding.model` requires a full re-ingestion.

## Configurable endpoints

Every FQDN the pipeline touches is configurable — nothing is hard-coded:

| Endpoint | Config key | Env override |
|---|---|---|
| Vector DB (Qdrant) | `vector_store.qdrant.url` | `OCP_RAG_QDRANT_URL` |
| Embedding service | `embedding.url` | `OCP_RAG_EMBEDDING_URL` |
| Git host for cloning | `openshift.git_base_url` | `OCP_RAG_GIT_BASE_URL` |

See [rag-config-reference.md](rag-config-reference.md) for the full field list.
