package rag

import (
	"context"
	"fmt"
	"os"
	"sync"

	chromem "github.com/philippgille/chromem-go"
)

type Store struct {
	db          *chromem.DB
	collections map[Collection]*chromem.Collection
	mu          sync.RWMutex
	embedFunc   chromem.EmbeddingFunc
}

func NewStore(dataDir string, embedFunc EmbeddingFunc) (*Store, error) {
	db, err := chromem.NewPersistentDB(dataDir, false)
	if err != nil {
		// If the DB is corrupted (e.g. partial deletion), wipe and retry.
		if removeErr := os.RemoveAll(dataDir); removeErr != nil {
			return nil, fmt.Errorf("open chromem db: %w (cleanup also failed: %v)", err, removeErr)
		}
		db, err = chromem.NewPersistentDB(dataDir, false)
		if err != nil {
			return nil, fmt.Errorf("open chromem db after reset: %w", err)
		}
	}

	chromemEmbed := chromem.EmbeddingFunc(embedFunc)

	s := &Store{
		db:          db,
		collections: make(map[Collection]*chromem.Collection),
		embedFunc:   chromemEmbed,
	}

	for _, name := range AllCollections {
		coll, err := db.GetOrCreateCollection(string(name), nil, chromemEmbed)
		if err != nil {
			return nil, fmt.Errorf("get/create collection %s: %w", name, err)
		}
		s.collections[name] = coll
	}

	return s, nil
}

func (s *Store) AddDocuments(ctx context.Context, coll Collection, docs []Document) error {
	c := s.getCollection(coll)
	if c == nil {
		return fmt.Errorf("collection %s not found", coll)
	}

	if len(docs) == 0 {
		return nil
	}

	chromemDocs := make([]chromem.Document, 0, len(docs))
	for _, d := range docs {
		chromemDocs = append(chromemDocs, chromem.Document{
			ID:       d.ID,
			Content:  d.Content,
			Metadata: d.Metadata,
		})
	}

	return c.AddDocuments(ctx, chromemDocs, 4)
}

func (s *Store) Search(ctx context.Context, coll Collection, query string, topK int) ([]Document, error) {
	c := s.getCollection(coll)
	if c == nil {
		return nil, fmt.Errorf("collection %s not found", coll)
	}

	if c.Count() == 0 {
		return nil, nil
	}

	nResults := topK
	if nResults > c.Count() {
		nResults = c.Count()
	}

	results, err := c.Query(ctx, query, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query collection %s: %w", coll, err)
	}

	docs := make([]Document, 0, len(results))
	for _, r := range results {
		docs = append(docs, Document{
			ID:       r.ID,
			Content:  r.Content,
			Metadata: r.Metadata,
		})
	}
	return docs, nil
}

func (s *Store) SearchWithFilter(ctx context.Context, coll Collection, query string, topK int, where map[string]string) ([]Document, error) {
	c := s.getCollection(coll)
	if c == nil {
		return nil, fmt.Errorf("collection %s not found", coll)
	}

	if c.Count() == 0 {
		return nil, nil
	}

	nResults := topK
	if nResults > c.Count() {
		nResults = c.Count()
	}

	results, err := c.Query(ctx, query, nResults, where, nil)
	if err != nil {
		return nil, fmt.Errorf("query collection %s: %w", coll, err)
	}

	docs := make([]Document, 0, len(results))
	for _, r := range results {
		docs = append(docs, Document{
			ID:       r.ID,
			Content:  r.Content,
			Metadata: r.Metadata,
		})
	}
	return docs, nil
}

func (s *Store) KeywordSearch(ctx context.Context, coll Collection, keyword string, limit int) ([]Document, error) {
	c := s.getCollection(coll)
	if c == nil {
		return nil, fmt.Errorf("collection %s not found", coll)
	}

	if c.Count() == 0 {
		return nil, nil
	}

	nResults := limit
	if nResults > c.Count() {
		nResults = c.Count()
	}

	results, err := c.Query(ctx, keyword, nResults, nil, map[string]string{
		"$contains": keyword,
	})
	if err != nil {
		return nil, nil
	}

	docs := make([]Document, 0, len(results))
	for _, r := range results {
		docs = append(docs, Document{
			ID:       r.ID,
			Content:  r.Content,
			Metadata: r.Metadata,
		})
	}
	return docs, nil
}

func (s *Store) Reset(coll Collection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.db.DeleteCollection(string(coll)); err != nil {
		return fmt.Errorf("delete collection %s: %w", coll, err)
	}

	c, err := s.db.CreateCollection(string(coll), nil, s.embedFunc)
	if err != nil {
		return fmt.Errorf("recreate collection %s: %w", coll, err)
	}
	s.collections[coll] = c
	return nil
}

func (s *Store) CollectionCount(coll Collection) int {
	c := s.getCollection(coll)
	if c == nil {
		return 0
	}
	return c.Count()
}

func (s *Store) getCollection(coll Collection) *chromem.Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collections[coll]
}
