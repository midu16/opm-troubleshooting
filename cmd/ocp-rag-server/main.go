package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midu16/opm-troubleshooting/internal/rag"
	ragmcp "github.com/midu16/opm-troubleshooting/internal/rag/mcp"
)

func main() {
	configPath := flag.String("config", "rag-config.yaml", "RAG config file path")
	flag.Parse()

	cfg, err := rag.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	engine, err := rag.NewEngine(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating RAG engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	srv := ragmcp.NewMCPServer(engine)
	if err := srv.RunStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
