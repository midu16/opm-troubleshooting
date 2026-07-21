package rag

import (
	"context"
	"strings"
	"unicode"
)

func (e *Engine) hybridRetrieve(ctx context.Context, coll Collection, query string, topK int) ([]Document, error) {
	docs, err := e.store.Search(ctx, coll, query, topK)
	if err != nil {
		return nil, err
	}

	keywords := extractKeywords(query, e.config.Retrieval.KeywordMinLength)
	if len(keywords) == 0 {
		return docs, nil
	}

	seen := make(map[string]bool, len(docs))
	for _, d := range docs {
		prefix := d.Content
		if len(prefix) > 100 {
			prefix = prefix[:100]
		}
		seen[prefix] = true
	}

	supplementMax := e.config.Retrieval.KeywordSupplementMax
	added := 0

	limit := 2
	if len(keywords) < limit {
		limit = len(keywords)
	}
	for i := 0; i < limit && added < supplementMax; i++ {
		kwDocs, err := e.store.KeywordSearch(ctx, coll, keywords[i], 5)
		if err != nil {
			continue
		}
		for _, d := range kwDocs {
			if added >= supplementMax {
				break
			}
			prefix := d.Content
			if len(prefix) > 100 {
				prefix = prefix[:100]
			}
			if seen[prefix] {
				continue
			}
			seen[prefix] = true
			docs = append(docs, d)
			added++
		}
	}

	return docs, nil
}

func extractKeywords(text string, minLength int) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	})

	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true,
		"this": true, "from": true, "what": true, "when": true, "where": true,
		"which": true, "have": true, "does": true, "should": true, "could": true,
		"would": true, "about": true, "into": true, "than": true, "then": true,
		"them": true, "they": true, "there": true, "these": true, "those": true,
		"been": true, "being": true, "were": true, "will": true, "your": true,
		"more": true, "some": true, "also": true, "each": true, "other": true,
	}

	var keywords []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) >= minLength && !stopWords[w] && !seen[w] {
			seen[w] = true
			keywords = append(keywords, w)
		}
	}
	return keywords
}
