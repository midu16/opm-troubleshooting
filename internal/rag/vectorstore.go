package rag

import (
	"context"
	"fmt"
	"strings"
)

// VectorStore is the persistence seam for the RAG knowledge base. Both the
// embedded chromem-go backend (*Store) and external backends (e.g. Qdrant)
// implement it, so the engine and ingestion pipeline are backend-agnostic.
//
// The method set matches the original *Store exactly, so existing code keeps
// working when the concrete type is replaced by this interface.
type VectorStore interface {
	AddDocuments(ctx context.Context, coll Collection, docs []Document) error
	Search(ctx context.Context, coll Collection, query string, topK int) ([]Document, error)
	SearchWithFilter(ctx context.Context, coll Collection, query string, topK int, where map[string]string) ([]Document, error)
	KeywordSearch(ctx context.Context, coll Collection, keyword string, limit int) ([]Document, error)
	Reset(coll Collection) error
	CollectionCount(coll Collection) int
}

// Backend identifiers accepted in vector_store.backend.
const (
	BackendChromem = "chromem"
	BackendQdrant  = "qdrant"
)

// NewVectorStore constructs the vector store backend selected in the config.
// It defaults to the embedded chromem-go store (local, zero-dependency) and
// returns a Qdrant-backed store when vector_store.backend is "qdrant".
func NewVectorStore(cfg *Config, embedFunc EmbeddingFunc) (VectorStore, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.VectorStore.Backend)) {
	case BackendQdrant:
		return NewQdrantStore(cfg, embedFunc)
	case "", BackendChromem:
		return NewStore(cfg.ChromemDir(), embedFunc)
	default:
		return nil, fmt.Errorf("unknown vector_store.backend %q (valid: %s, %s)",
			cfg.VectorStore.Backend, BackendChromem, BackendQdrant)
	}
}
