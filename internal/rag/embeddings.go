package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

func NewOllamaEmbedder(baseURL, model string) EmbeddingFunc {
	client := &http.Client{Timeout: 120 * time.Second}
	baseURL = strings.TrimRight(baseURL, "/")

	// Track which endpoint works so we don't probe on every call.
	var resolvedEndpoint string

	return func(ctx context.Context, text string) ([]float32, error) {
		if resolvedEndpoint != "" {
			return callEmbedEndpoint(ctx, client, baseURL+resolvedEndpoint, model, text)
		}

		// Try /api/embed first (Ollama >= 0.4), then /api/embeddings (legacy).
		for _, ep := range []string{"/api/embed", "/api/embeddings"} {
			result, err := callEmbedEndpoint(ctx, client, baseURL+ep, model, text)
			if err == nil {
				resolvedEndpoint = ep
				return result, nil
			}
			if isEmbeddingNotSupported(err) {
				continue
			}
			return nil, err
		}

		return nil, fmt.Errorf(
			"ollama at %s does not support embeddings for model %q.\n"+
				"  Fix: pull an embedding model and use it in rag-config.yaml:\n"+
				"    ollama pull all-minilm        # 384-dim, fast, recommended\n"+
				"    ollama pull nomic-embed-text   # 768-dim, higher quality\n"+
				"    ollama pull mxbai-embed-large  # 1024-dim, best quality\n"+
				"  Then set embedding.model in rag-config.yaml to the model name.\n"+
				"  Note: generative LLMs (llama3, qwen3, etc.) cannot produce embeddings.",
			baseURL, model,
		)
	}
}

func callEmbedEndpoint(ctx context.Context, client *http.Client, url, model, text string) ([]float32, error) {
	var reqBody []byte
	var err error

	if strings.HasSuffix(url, "/api/embed") {
		reqBody, err = json.Marshal(map[string]any{"model": model, "input": text})
	} else {
		reqBody, err = json.Marshal(map[string]any{"model": model, "prompt": text})
	}
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		// /api/embed returns {"embeddings": [[...]]}
		Embeddings [][]float32 `json:"embeddings"`
		// /api/embeddings returns {"embedding": [...]}
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}

	if len(result.Embeddings) > 0 && len(result.Embeddings[0]) > 0 {
		return result.Embeddings[0], nil
	}
	if len(result.Embedding) > 0 {
		return result.Embedding, nil
	}

	return nil, fmt.Errorf("ollama embed: empty embedding returned for model %q — ensure it is an embedding model, not a generative LLM", model)
}

func isEmbeddingNotSupported(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not support embeddings") ||
		strings.Contains(msg, "status 501") ||
		strings.Contains(msg, "status 404") ||
		strings.Contains(msg, "not found")
}
