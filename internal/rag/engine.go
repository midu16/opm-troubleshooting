package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Engine struct {
	store  *Store
	config *Config
}

func NewEngine(cfg *Config) (*Engine, error) {
	embedFunc := NewOllamaEmbedder(cfg.Embedding.URL, cfg.Embedding.Model)

	store, err := NewStore(cfg.ChromemDir(), embedFunc)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return &Engine{store: store, config: cfg}, nil
}

func (e *Engine) Close() {}

func (e *Engine) Store() *Store { return e.store }

func (e *Engine) Config() *Config { return e.config }

func (e *Engine) Troubleshoot(ctx context.Context, operator string, symptoms []string, ocpVersion string) (*TroubleshootResult, error) {
	query := buildTroubleshootQuery(operator, symptoms)

	type collResult struct {
		coll Collection
		docs []Document
		err  error
	}

	searches := []struct {
		coll   Collection
		topK   int
		hybrid bool
	}{
		{CollDocs, e.config.Retrieval.DefaultTopK, true},
		{CollCode, e.config.Retrieval.CodeTopK, true},
		{CollTelco, e.config.Retrieval.ConfigTopK, false},
		{CollKnownIssues, e.config.Retrieval.IssuesTopK, false},
	}

	results := make([]collResult, len(searches))
	var wg sync.WaitGroup

	for i, s := range searches {
		wg.Add(1)
		go func(idx int, coll Collection, topK int, hybrid bool) {
			defer wg.Done()
			var docs []Document
			var err error
			if hybrid {
				docs, err = e.hybridRetrieve(ctx, coll, query, topK)
			} else {
				docs, err = e.store.Search(ctx, coll, query, topK)
			}
			results[idx] = collResult{coll: coll, docs: docs, err: err}
		}(i, s.coll, s.topK, s.hybrid)
	}
	wg.Wait()

	tr := &TroubleshootResult{}
	var summaryParts []string
	totalDocs := 0

	for _, r := range results {
		if r.err != nil || len(r.docs) == 0 {
			continue
		}
		totalDocs += len(r.docs)

		switch r.coll {
		case CollDocs:
			for _, d := range r.docs {
				tr.DocumentationRefs = append(tr.DocumentationRefs, DocReference{
					Title:   metaOrDefault(d.Metadata, "section", "OCP Documentation"),
					Source:  metaOrDefault(d.Metadata, "source", "docs.redhat.com"),
					Excerpt: truncate(d.Content, 300),
					URL:     metaOrDefault(d.Metadata, "url", ""),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d documentation references", len(r.docs)))

		case CollCode:
			for _, d := range r.docs {
				tr.DocumentationRefs = append(tr.DocumentationRefs, DocReference{
					Title:   metaOrDefault(d.Metadata, "declaration", "Source code"),
					Source:  metaOrDefault(d.Metadata, "source", "openshift source"),
					Excerpt: truncate(d.Content, 300),
					URL:     metaOrDefault(d.Metadata, "repo_url", ""),
				})
			}

		case CollTelco:
			for _, d := range r.docs {
				tr.ConfigAdvice = append(tr.ConfigAdvice, ConfigAdvice{
					Component: metaOrDefault(d.Metadata, "k8s_kind", "Configuration"),
					Reference: metaOrDefault(d.Metadata, "source", "telco-reference"),
					Advice:    truncate(d.Content, 500),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d configuration references", len(r.docs)))

		case CollKnownIssues:
			for _, d := range r.docs {
				tr.KnownIssues = append(tr.KnownIssues, KnownIssue{
					ID:         metaOrDefault(d.Metadata, "issue_id", ""),
					Summary:    truncate(d.Content, 300),
					Workaround: metaOrDefault(d.Metadata, "workaround", ""),
					FixVersion: metaOrDefault(d.Metadata, "fix_version", ""),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d known issues", len(r.docs)))
		}
	}

	if totalDocs > 0 {
		tr.Summary = fmt.Sprintf("RAG analysis for %s: found %s", operator, strings.Join(summaryParts, ", "))
		tr.Confidence = computeConfidence(totalDocs, len(tr.KnownIssues))
	} else {
		tr.Summary = fmt.Sprintf("No RAG data found for %s — run ocp-rag-ingest to populate the knowledge base", operator)
		tr.Confidence = 0
	}

	return tr, nil
}

func (e *Engine) SearchDocs(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.hybridRetrieve(ctx, CollDocs, query, e.config.Retrieval.DefaultTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchCode(ctx context.Context, query, operator string) (*SearchResult, error) {
	q := query
	if operator != "" {
		q = operator + " " + query
	}
	docs, err := e.hybridRetrieve(ctx, CollCode, q, e.config.Retrieval.CodeTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchTelcoConfigs(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.store.Search(ctx, CollTelco, query, e.config.Retrieval.ConfigTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchKnownIssues(ctx context.Context, operator, version string) (*SearchResult, error) {
	query := operator
	if version != "" {
		query += " " + version
	}
	docs, err := e.store.Search(ctx, CollKnownIssues, query, e.config.Retrieval.IssuesTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchManifests(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.store.Search(ctx, CollManifests, query, e.config.Retrieval.ConfigTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) CheckFreshness() (*FreshnessStatus, error) {
	return CheckFreshness(e.config.DataDir, e.config.Freshness.MetaFile)
}

func buildTroubleshootQuery(operator string, symptoms []string) string {
	parts := []string{"OpenShift operator " + operator + " troubleshooting"}
	for i, s := range symptoms {
		if i >= 5 {
			break
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

func computeConfidence(totalDocs, knownIssueCount int) float64 {
	c := 0.0
	if totalDocs > 0 {
		c = 0.3
	}
	if totalDocs > 3 {
		c = 0.5
	}
	if totalDocs > 6 {
		c = 0.65
	}
	if knownIssueCount > 0 {
		c += 0.2
	}
	if c > 1.0 {
		c = 1.0
	}
	return c
}

func docsToSearchResult(query string, docs []Document) *SearchResult {
	sr := &SearchResult{Query: query}
	for _, d := range docs {
		sr.Documents = append(sr.Documents, DocReference{
			Title:   metaOrDefault(d.Metadata, "section", metaOrDefault(d.Metadata, "declaration", "Document")),
			Source:  metaOrDefault(d.Metadata, "source", "unknown"),
			Excerpt: truncate(d.Content, 500),
			URL:     metaOrDefault(d.Metadata, "url", ""),
		})
	}
	return sr
}

func metaOrDefault(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
