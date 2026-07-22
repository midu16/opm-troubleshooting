package ragmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/midu16/opm-troubleshooting/internal/rag"
	"github.com/midu16/opm-troubleshooting/internal/rag/ingest"
)

type mcpServer struct {
	engine *rag.Engine
	srv    *server.MCPServer
}

func NewMCPServer(engine *rag.Engine) *mcpServer {
	srv := server.NewMCPServer(
		"ocp-rag-server",
		"1.0.0",
	)

	s := &mcpServer{engine: engine, srv: srv}
	s.registerTools()
	return s
}

func (s *mcpServer) RunStdio() error {
	stdio := server.NewStdioServer(s.srv)
	return stdio.Listen(context.Background(), os.Stdin, os.Stdout)
}

func (s *mcpServer) registerTools() {
	s.srv.AddTool(
		mcp.NewTool("search_docs",
			mcp.WithDescription("Search OpenShift Container Platform documentation for troubleshooting, configuration, and operational guidance"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query for OCP documentation")),
		),
		s.handleSearchDocs,
	)

	s.srv.AddTool(
		mcp.NewTool("search_operator_code",
			mcp.WithDescription("Search OpenShift operator Go source code for implementation details, error handling, and reconcile logic"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query for operator source code")),
			mcp.WithString("operator", mcp.Description("Filter by operator name (e.g. cluster-etcd-operator)")),
		),
		s.handleSearchCode,
	)

	s.srv.AddTool(
		mcp.NewTool("search_telco_configs",
			mcp.WithDescription("Search telco-reference validated configurations for telco-core and telco-ran deployments"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query for telco configurations")),
		),
		s.handleSearchTelco,
	)

	s.srv.AddTool(
		mcp.NewTool("troubleshoot_operator",
			mcp.WithDescription("Primary troubleshooting tool: searches all knowledge bases (docs, code, configs, known issues) for comprehensive operator diagnosis"),
			mcp.WithString("operator", mcp.Required(), mcp.Description("Operator name to troubleshoot")),
			mcp.WithArray("symptoms", mcp.Description("List of observed symptoms or error messages"), mcp.WithStringItems()),
			mcp.WithString("ocp_version", mcp.Description("OCP version (default: 4.22)")),
		),
		s.handleTroubleshoot,
	)

	s.srv.AddTool(
		mcp.NewTool("get_operator_info",
			mcp.WithDescription("Get comprehensive information about an operator: documentation, known issues, and reference configurations"),
			mcp.WithString("operator", mcp.Required(), mcp.Description("Operator name")),
		),
		s.handleGetOperatorInfo,
	)

	s.srv.AddTool(
		mcp.NewTool("search_known_issues",
			mcp.WithDescription("Search known issues and bugs for OpenShift operators"),
			mcp.WithString("operator", mcp.Required(), mcp.Description("Operator name")),
			mcp.WithString("ocp_version", mcp.Description("OCP version filter")),
		),
		s.handleSearchKnownIssues,
	)

	s.srv.AddTool(
		mcp.NewTool("search_errata",
			mcp.WithDescription("Search errata and known issues by OCP version"),
			mcp.WithString("ocp_version", mcp.Required(), mcp.Description("OCP version (e.g. 4.22)")),
		),
		s.handleSearchErrata,
	)

	s.srv.AddTool(
		mcp.NewTool("update_rag",
			mcp.WithDescription("Re-ingest all data sources (clone repos, scrape docs, rebuild vector store). Takes several minutes."),
		),
		s.handleUpdateRAG,
	)
}

func (s *mcpServer) handleSearchDocs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return errResult("query is required"), nil
	}

	result, err := s.engine.SearchDocs(ctx, query)
	if err != nil {
		return errResult("search failed: " + err.Error()), nil
	}

	return textResult(formatSearchResult("OCP Documentation", result)), nil
}

func (s *mcpServer) handleSearchCode(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return errResult("query is required"), nil
	}
	operator := req.GetString("operator", "")

	result, err := s.engine.SearchCode(ctx, query, operator)
	if err != nil {
		return errResult("search failed: " + err.Error()), nil
	}

	return textResult(formatSearchResult("Operator Source Code", result)), nil
}

func (s *mcpServer) handleSearchTelco(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return errResult("query is required"), nil
	}

	result, err := s.engine.SearchTelcoConfigs(ctx, query)
	if err != nil {
		return errResult("search failed: " + err.Error()), nil
	}

	return textResult(formatSearchResult("Telco Reference Configs", result)), nil
}

func (s *mcpServer) handleTroubleshoot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	operator := req.GetString("operator", "")
	if operator == "" {
		return errResult("operator is required"), nil
	}
	symptoms := req.GetStringSlice("symptoms", nil)
	version := req.GetString("ocp_version", "")

	result, err := s.engine.Troubleshoot(ctx, operator, symptoms, version)
	if err != nil {
		return errResult("troubleshoot failed: " + err.Error()), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Troubleshooting: %s\n\n", operator))
	sb.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n\n", result.Confidence*100))
	sb.WriteString(result.Summary + "\n\n")

	if len(result.KnownIssues) > 0 {
		sb.WriteString("## Known Issues\n\n")
		for _, ki := range result.KnownIssues {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", ki.ID, ki.Summary))
			if ki.Workaround != "" {
				sb.WriteString(fmt.Sprintf("  Workaround: %s\n", ki.Workaround))
			}
		}
		sb.WriteString("\n")
	}

	if len(result.DocumentationRefs) > 0 {
		sb.WriteString("## Documentation References\n\n")
		for _, ref := range result.DocumentationRefs {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n  %s\n", ref.Title, ref.Source, ref.Excerpt))
			if ref.URL != "" {
				sb.WriteString(fmt.Sprintf("  URL: %s\n", ref.URL))
			}
		}
		sb.WriteString("\n")
	}

	if len(result.ConfigAdvice) > 0 {
		sb.WriteString("## Configuration Advice\n\n")
		for _, ca := range result.ConfigAdvice {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", ca.Component, ca.Reference, ca.Advice))
		}
		sb.WriteString("\n")
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sb.WriteString("```json\n")
	sb.Write(jsonBytes)
	sb.WriteString("\n```\n")

	return textResult(sb.String()), nil
}

func (s *mcpServer) handleGetOperatorInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	operator := req.GetString("operator", "")
	if operator == "" {
		return errResult("operator is required"), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Operator Info: %s\n\n", operator))

	docs, err := s.engine.SearchDocs(ctx, operator+" operator troubleshooting configuration")
	if err == nil && len(docs.Documents) > 0 {
		sb.WriteString("## Documentation\n\n")
		for _, d := range docs.Documents {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", d.Title, d.Excerpt))
		}
		sb.WriteString("\n")
	}

	issues, err := s.engine.SearchKnownIssues(ctx, operator, "")
	if err == nil && len(issues.Documents) > 0 {
		sb.WriteString("## Known Issues\n\n")
		for _, d := range issues.Documents {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", d.Title, d.Excerpt))
		}
		sb.WriteString("\n")
	}

	configs, err := s.engine.SearchTelcoConfigs(ctx, operator)
	if err == nil && len(configs.Documents) > 0 {
		sb.WriteString("## Reference Configurations\n\n")
		for _, d := range configs.Documents {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", d.Title, d.Excerpt))
		}
		sb.WriteString("\n")
	}

	return textResult(sb.String()), nil
}

func (s *mcpServer) handleSearchKnownIssues(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	operator := req.GetString("operator", "")
	if operator == "" {
		return errResult("operator is required"), nil
	}
	version := req.GetString("ocp_version", "")

	result, err := s.engine.SearchKnownIssues(ctx, operator, version)
	if err != nil {
		return errResult("search failed: " + err.Error()), nil
	}

	return textResult(formatSearchResult("Known Issues", result)), nil
}

func (s *mcpServer) handleSearchErrata(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	version := req.GetString("ocp_version", "")
	if version == "" {
		return errResult("ocp_version is required"), nil
	}

	result, err := s.engine.SearchKnownIssues(ctx, "", version)
	if err != nil {
		return errResult("search failed: " + err.Error()), nil
	}

	return textResult(formatSearchResult("Errata for OCP "+version, result)), nil
}

func (s *mcpServer) handleUpdateRAG(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ingest.RunIngestion(ctx, s.engine.Config(), s.engine.Store()); err != nil {
		return errResult("ingestion failed: " + err.Error()), nil
	}
	return textResult("RAG knowledge base updated successfully."), nil
}

func formatSearchResult(title string, sr *rag.SearchResult) string {
	if sr == nil {
		return fmt.Sprintf("No results found for %s", title)
	}
	if len(sr.Documents) == 0 {
		return fmt.Sprintf("No results found for %s query: %s", title, sr.Query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s Results\n\n", title))
	sb.WriteString(fmt.Sprintf("Query: %s\n\n", sr.Query))

	for i, doc := range sr.Documents {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, doc.Title))
		sb.WriteString(fmt.Sprintf("Source: %s\n", doc.Source))
		if doc.URL != "" {
			sb.WriteString(fmt.Sprintf("URL: %s\n", doc.URL))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n\n", doc.Excerpt))
	}

	return sb.String()
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}
