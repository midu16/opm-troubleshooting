package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/midu16/opm-troubleshooting/internal/rag"
	"github.com/midu16/opm-troubleshooting/internal/rag/ingest"
)

func main() {
	configPath := flag.String("config", "rag-config.yaml", "RAG config file path")
	flag.Parse()

	cfg, err := rag.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "  Config:    %s\n", *configPath)
	fmt.Fprintf(os.Stderr, "  Data dir:  %s\n", cfg.DataDir)
	fmt.Fprintf(os.Stderr, "  Ollama:    %s (model: %s)\n", cfg.Embedding.URL, cfg.Embedding.Model)
	fmt.Fprintf(os.Stderr, "  OCP:       %s (%d repos)\n\n", cfg.OpenShift.Version, len(cfg.OpenShift.Repos))

	engine, err := rag.NewEngine(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating RAG engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	ctx := context.Background()
	if err := ingest.RunIngestion(ctx, cfg, engine.Store()); err != nil {
		fmt.Fprintf(os.Stderr, "Ingestion failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Ingestion complete.")
}
