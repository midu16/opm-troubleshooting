package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

func main() {
	configPath := flag.String("config", "rag-config.yaml", "RAG config file path")
	collection := flag.String("collection", "", "Collection to search: docs, code, telco, issues, manifests (default: troubleshoot all)")
	operator := flag.String("operator", "", "Operator name (for troubleshoot and code search)")
	jsonOut := flag.Bool("json", false, "Output raw JSON instead of formatted text")
	freshness := flag.Bool("freshness", false, "Check knowledge base freshness and exit")
	flag.Parse()

	query := strings.Join(flag.Args(), " ")

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

	ctx := context.Background()

	if *freshness {
		status, err := engine.CheckFreshness()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking freshness: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(status)
		} else {
			fmt.Printf("Fresh: %v\n", status.Fresh)
			fmt.Printf("Message: %s\n", status.Message)
			if status.IngestedAt != "" {
				fmt.Printf("Ingested at: %s\n", status.IngestedAt)
			}
		}
		return
	}

	if query == "" {
		fmt.Fprintln(os.Stderr, "Usage: ocp-rag-query [flags] <query>")
		fmt.Fprintln(os.Stderr, "       ocp-rag-query --freshness")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  ocp-rag-query etcd leader election timeout")
		fmt.Fprintln(os.Stderr, "  ocp-rag-query --collection docs SR-IOV configuration")
		fmt.Fprintln(os.Stderr, "  ocp-rag-query --operator cluster-etcd-operator --json pod crash")
		fmt.Fprintln(os.Stderr, "  ocp-rag-query --freshness")
		flag.PrintDefaults()
		os.Exit(1)
	}

	switch *collection {
	case "docs":
		result, err := engine.SearchDocs(ctx, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		printSearchResult(result, *jsonOut)

	case "code":
		result, err := engine.SearchCode(ctx, query, *operator)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		printSearchResult(result, *jsonOut)

	case "telco":
		result, err := engine.SearchTelcoConfigs(ctx, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		printSearchResult(result, *jsonOut)

	case "issues":
		result, err := engine.SearchKnownIssues(ctx, *operator, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		printSearchResult(result, *jsonOut)

	case "manifests":
		result, err := engine.SearchManifests(ctx, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		printSearchResult(result, *jsonOut)

	case "":
		op := *operator
		if op == "" {
			op = "cluster"
		}
		result, err := engine.Troubleshoot(ctx, op, []string{query}, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Troubleshoot error: %v\n", err)
			os.Exit(1)
		}
		printTroubleshootResult(result, *jsonOut)

	default:
		fmt.Fprintf(os.Stderr, "Unknown collection: %s (valid: docs, code, telco, issues, manifests)\n", *collection)
		os.Exit(1)
	}
}

func printSearchResult(result *rag.SearchResult, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		return
	}

	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Results: %d\n\n", len(result.Documents))

	for i, doc := range result.Documents {
		fmt.Printf("--- %d. %s ---\n", i+1, doc.Title)
		fmt.Printf("Source: %s\n", doc.Source)
		if doc.URL != "" {
			fmt.Printf("URL: %s\n", doc.URL)
		}
		fmt.Printf("\n%s\n\n", doc.Excerpt)
	}
}

func printTroubleshootResult(result *rag.TroubleshootResult, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		return
	}

	fmt.Printf("Summary: %s\n", result.Summary)
	fmt.Printf("Confidence: %.0f%%\n\n", result.Confidence*100)

	if len(result.KnownIssues) > 0 {
		fmt.Println("== Known Issues ==")
		for _, ki := range result.KnownIssues {
			fmt.Printf("  [%s] %s\n", ki.ID, ki.Summary)
			if ki.Workaround != "" {
				fmt.Printf("    Workaround: %s\n", ki.Workaround)
			}
		}
		fmt.Println()
	}

	if len(result.DocumentationRefs) > 0 {
		fmt.Println("== Documentation ==")
		for _, ref := range result.DocumentationRefs {
			fmt.Printf("  - %s (%s)\n", ref.Title, ref.Source)
			if ref.URL != "" {
				fmt.Printf("    %s\n", ref.URL)
			}
		}
		fmt.Println()
	}

	if len(result.ConfigAdvice) > 0 {
		fmt.Println("== Config Advice ==")
		for _, ca := range result.ConfigAdvice {
			fmt.Printf("  - %s (%s): %s\n", ca.Component, ca.Reference, ca.Advice)
		}
		fmt.Println()
	}
}
