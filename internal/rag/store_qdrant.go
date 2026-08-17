package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// qdrantStore is a VectorStore backed by an external Qdrant service, reached
// over its HTTP REST API using only the standard library (no client SDK, so
// the pure-Go / CGO_ENABLED=0 build is preserved).
//
// Each RAG Collection maps to one Qdrant collection (optionally namespaced by
// CollectionPrefix). Embeddings are computed locally with the same EmbeddingFunc
// used by the chromem backend, then upserted as point vectors; document text and
// metadata are stored in the point payload.
type qdrantStore struct {
	baseURL    string
	apiKey     string
	prefix     string
	distance   string
	embed      EmbeddingFunc
	http       *http.Client
	contentKey string

	mu      sync.Mutex
	ensured map[string]bool // qdrant collection name -> created/verified
}

const qdrantContentKey = "content"

// NewQdrantStore verifies connectivity to Qdrant and returns a ready store.
func NewQdrantStore(cfg *Config, embedFunc EmbeddingFunc) (VectorStore, error) {
	qc := cfg.VectorStore.Qdrant
	if qc.URL == "" {
		return nil, fmt.Errorf("vector_store.qdrant.url is required when backend is %q", BackendQdrant)
	}
	distance := qc.Distance
	if distance == "" {
		distance = "Cosine"
	}
	timeout := qc.Timeout.Duration
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	s := &qdrantStore{
		baseURL:    strings.TrimRight(qc.URL, "/"),
		apiKey:     qc.APIKey,
		prefix:     qc.CollectionPrefix,
		distance:   distance,
		embed:      embedFunc,
		http:       &http.Client{Timeout: timeout},
		contentKey: qdrantContentKey,
		ensured:    make(map[string]bool),
	}

	// Fail fast if Qdrant is unreachable.
	if err := s.do(context.Background(), http.MethodGet, "/collections", nil, nil); err != nil {
		return nil, fmt.Errorf("connect to qdrant at %s: %w", s.baseURL, err)
	}
	return s, nil
}

func (s *qdrantStore) collName(coll Collection) string { return s.prefix + string(coll) }

// AddDocuments embeds and upserts documents, creating the collection on first use.
func (s *qdrantStore) AddDocuments(ctx context.Context, coll Collection, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	type embedded struct {
		doc Document
		vec []float32
		err error
	}
	results := make([]embedded, len(docs))

	// Embed with bounded concurrency (mirrors the chromem backend's batch size).
	const workers = 4
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, d := range docs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, doc Document) {
			defer wg.Done()
			defer func() { <-sem }()
			vec, err := s.embed(ctx, doc.Content)
			results[idx] = embedded{doc: doc, vec: vec, err: err}
		}(i, d)
	}
	wg.Wait()

	points := make([]map[string]any, 0, len(results))
	dim := 0
	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("embed document %s: %w", r.doc.ID, r.err)
		}
		if dim == 0 {
			dim = len(r.vec)
		}
		payload := make(map[string]any, len(r.doc.Metadata)+2)
		for k, v := range r.doc.Metadata {
			payload[k] = v
		}
		payload[s.contentKey] = r.doc.Content
		payload["_id"] = r.doc.ID
		points = append(points, map[string]any{
			"id":      pointID(r.doc.ID),
			"vector":  r.vec,
			"payload": payload,
		})
	}

	if dim == 0 {
		return fmt.Errorf("embedding produced zero-length vectors")
	}
	if err := s.ensureCollection(ctx, s.collName(coll), dim); err != nil {
		return err
	}

	body := map[string]any{"points": points}
	path := fmt.Sprintf("/collections/%s/points?wait=true", s.collName(coll))
	if err := s.do(ctx, http.MethodPut, path, body, nil); err != nil {
		return fmt.Errorf("upsert points into %s: %w", coll, err)
	}
	return nil
}

func (s *qdrantStore) Search(ctx context.Context, coll Collection, query string, topK int) ([]Document, error) {
	return s.search(ctx, coll, query, topK, nil)
}

func (s *qdrantStore) SearchWithFilter(ctx context.Context, coll Collection, query string, topK int, where map[string]string) ([]Document, error) {
	var filter map[string]any
	if len(where) > 0 {
		must := make([]map[string]any, 0, len(where))
		for k, v := range where {
			must = append(must, map[string]any{
				"key":   k,
				"match": map[string]any{"value": v},
			})
		}
		filter = map[string]any{"must": must}
	}
	return s.search(ctx, coll, query, topK, filter)
}

func (s *qdrantStore) search(ctx context.Context, coll Collection, query string, topK int, filter map[string]any) ([]Document, error) {
	if topK <= 0 {
		return nil, nil
	}
	vec, err := s.embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	body := map[string]any{
		"vector":       vec,
		"limit":        topK,
		"with_payload": true,
	}
	if filter != nil {
		body["filter"] = filter
	}

	var resp struct {
		Result []struct {
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	path := fmt.Sprintf("/collections/%s/points/search", s.collName(coll))
	if err := s.do(ctx, http.MethodPost, path, body, &resp); err != nil {
		if isNotFound(err) {
			return nil, nil // collection not created yet == empty, like chromem
		}
		return nil, fmt.Errorf("search %s: %w", coll, err)
	}

	docs := make([]Document, 0, len(resp.Result))
	for _, r := range resp.Result {
		docs = append(docs, s.payloadToDoc(r.Payload))
	}
	return docs, nil
}

// KeywordSearch returns documents whose content matches the keyword using
// Qdrant's full-text payload index (created on the content field at collection
// setup). This mirrors the chromem backend's $contains keyword supplement.
func (s *qdrantStore) KeywordSearch(ctx context.Context, coll Collection, keyword string, limit int) ([]Document, error) {
	if limit <= 0 || strings.TrimSpace(keyword) == "" {
		return nil, nil
	}
	body := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{{
				"key":   s.contentKey,
				"match": map[string]any{"text": keyword},
			}},
		},
		"limit":        limit,
		"with_payload": true,
	}

	var resp struct {
		Result struct {
			Points []struct {
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	path := fmt.Sprintf("/collections/%s/points/scroll", s.collName(coll))
	if err := s.do(ctx, http.MethodPost, path, body, &resp); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		// A missing text index (or unsupported match) should not break hybrid
		// retrieval — the vector results already stand on their own.
		return nil, nil
	}

	docs := make([]Document, 0, len(resp.Result.Points))
	for _, p := range resp.Result.Points {
		docs = append(docs, s.payloadToDoc(p.Payload))
	}
	return docs, nil
}

// Reset deletes the collection; it is recreated lazily on the next AddDocuments.
func (s *qdrantStore) Reset(coll Collection) error {
	name := s.collName(coll)
	err := s.do(context.Background(), http.MethodDelete, "/collections/"+name, nil, nil)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("delete collection %s: %w", coll, err)
	}
	s.mu.Lock()
	delete(s.ensured, name)
	s.mu.Unlock()
	return nil
}

func (s *qdrantStore) CollectionCount(coll Collection) int {
	var resp struct {
		Result struct {
			PointsCount int `json:"points_count"`
		} `json:"result"`
	}
	if err := s.do(context.Background(), http.MethodGet, "/collections/"+s.collName(coll), nil, &resp); err != nil {
		return 0
	}
	return resp.Result.PointsCount
}

// ensureCollection creates the Qdrant collection and a full-text index on the
// content field if they do not already exist.
func (s *qdrantStore) ensureCollection(ctx context.Context, name string, dim int) error {
	s.mu.Lock()
	if s.ensured[name] {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Already present?
	err := s.do(ctx, http.MethodGet, "/collections/"+name, nil, nil)
	if err == nil {
		s.markEnsured(name)
		return nil
	}
	if !isNotFound(err) {
		return fmt.Errorf("check collection %s: %w", name, err)
	}

	create := map[string]any{
		"vectors": map[string]any{
			"size":     dim,
			"distance": s.distance,
		},
	}
	if err := s.do(ctx, http.MethodPut, "/collections/"+name, create, nil); err != nil {
		return fmt.Errorf("create collection %s: %w", name, err)
	}

	// Full-text index on content enables KeywordSearch; best-effort.
	idx := map[string]any{
		"field_name":   s.contentKey,
		"field_schema": "text",
	}
	_ = s.do(ctx, http.MethodPut, "/collections/"+name+"/index?wait=true", idx, nil)

	s.markEnsured(name)
	return nil
}

func (s *qdrantStore) markEnsured(name string) {
	s.mu.Lock()
	s.ensured[name] = true
	s.mu.Unlock()
}

func (s *qdrantStore) payloadToDoc(payload map[string]any) Document {
	d := Document{Metadata: make(map[string]string)}
	for k, v := range payload {
		sv, ok := v.(string)
		if !ok {
			continue
		}
		switch k {
		case s.contentKey:
			d.Content = sv
		case "_id":
			d.ID = sv
		default:
			d.Metadata[k] = sv
		}
	}
	return d
}

// do performs a JSON request against Qdrant. On non-2xx it returns a qdrantError
// carrying the status code so callers can detect 404s.
func (s *qdrantStore) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &qdrantError{status: resp.StatusCode, body: strings.TrimSpace(string(data))}
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

type qdrantError struct {
	status int
	body   string
}

func (e *qdrantError) Error() string {
	return fmt.Sprintf("qdrant http %d: %s", e.status, e.body)
}

func isNotFound(err error) bool {
	var qe *qdrantError
	if errors.As(err, &qe) {
		return qe.status == http.StatusNotFound
	}
	return false
}

// pointID converts an arbitrary document ID into a deterministic UUID string,
// which Qdrant accepts as a point ID (upserts are idempotent across re-ingests).
// It is a name-based UUID: the first 16 bytes of SHA-256 over the document ID,
// with the RFC 4122 version and variant bits set. SHA-256 (not SHA-1) is used so
// the pure-Go build stays free of blocklisted-hash lint findings; the hash is
// only an identity derivation, never a security primitive.
func pointID(id string) string {
	sum := sha256.Sum256([]byte("ocp-rag:" + id))
	h := sum[:16]
	h[6] = (h[6] & 0x0f) | 0x40 // version 4 layout
	h[8] = (h[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
