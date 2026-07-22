package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/analysis"
	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/workflow"
)

const defaultTimeout = 10 * time.Minute

// Exit codes.
const (
	exitSuccess   = 0
	exitUsage     = 1
	exitOperation = 2
)

// CLIError carries an explicit exit code for the main package.
type CLIError struct { //nolint:revive // widely used name
	Code int
	Err  error
}

func (e *CLIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "error"
}

func (e *CLIError) Unwrap() error {
	return e.Err
}

// ExitCode maps errors to process exit codes.
func ExitCode(err error) int {
	if err == nil {
		return exitSuccess
	}
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.Code
	}
	return exitOperation
}

type config struct {
	catalog        string
	packageName    string
	channel        string
	version        string
	jsonOut        bool
	timeout        time.Duration
	mustGatherPath string

	// Telco production options
	telcoSuite  bool
	healthCheck bool
	environment string
	sourceRepo  string
	generateRCA bool
	rcaFile     string
	clusterName string
	stateDir    string

	// ADHD divergent analysis
	adhdEnabled bool
	adhdFrames  int
	adhdDepth   string

	// Metadata, learning, and repo correlation
	metadataDir    string
	enableLearning bool
	enableRepoCorr bool
	githubToken    string

	// RAG knowledge base
	ragEnabled    bool
	ragConfigPath string
	ragDataDir    string
}

var errHelp = errors.New("help requested")

// Run executes the CLI with the given arguments.
func Run(args []string) error {
	cfg, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			return nil
		}
		return &CLIError{Code: exitUsage, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	// Branch execution based on must-gather flag
	if cfg.mustGatherPath != "" {
		return runMustGatherAnalysis(ctx, cfg)
	}

	declCfg, err := catalog.RenderCatalog(ctx, cfg.catalog)
	if err != nil {
		return &CLIError{Code: exitOperation, Err: err}
	}

	channel := cfg.channel
	if channel == "" {
		channel, err = workflow.ResolveDefaultChannel(declCfg, cfg.packageName)
		if err != nil {
			return &CLIError{Code: exitUsage, Err: err}
		}
	}

	result, err := workflow.InspectBundleFromConfig(ctx, declCfg, cfg.packageName, channel, cfg.version)
	if err != nil {
		code := exitOperation
		if isUsageError(err) {
			code = exitUsage
		}
		return &CLIError{Code: code, Err: err}
	}

	return writeOutput(os.Stdout, result.Info, cfg.jsonOut)
}

func isUsageError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no bundle image found") ||
		strings.Contains(msg, "default channel not found") ||
		strings.Contains(msg, "channel ") && strings.Contains(msg, "not found for package") ||
		strings.Contains(msg, "package ") && strings.Contains(msg, "not found in catalog") ||
		strings.Contains(msg, "version ") && strings.Contains(msg, "not found for package")
}

func parseArgs(args []string) (*config, error) {
	// Pre-process args to handle shell-expanded globs
	// When user types: --must-gather /path/* --package foo
	// Shell expands to: --must-gather /path/dir1 /path/dir2 /path/dir3 --package foo
	// We need to collect all paths between --must-gather and the next flag
	args = preprocessMustGatherArgs(args)

	fs := flag.NewFlagSet("catalog-bundle-inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		printUsage(os.Stdout)
	}

	var (
		catalogFlag     string
		packageFlag     string
		channelFlag     string
		versionFlag     string
		mustGatherFlag  string
		jsonOut         bool
		timeoutDuration time.Duration
		telcoSuite      bool
		healthCheck     bool
		environment     string
		sourceRepo      string
		generateRCA     bool
		rcaFile         string
		clusterName     string
		stateDir        string
	)

	fs.StringVar(&catalogFlag, "catalog", "", "OLM catalog index image reference")
	fs.StringVar(&catalogFlag, "c", "", "OLM catalog index image reference (shorthand)")
	fs.StringVar(&packageFlag, "package", "", "OLM package name")
	fs.StringVar(&packageFlag, "p", "", "OLM package name (shorthand)")
	fs.StringVar(&channelFlag, "channel", "", "OLM channel name (optional; uses package defaultChannel when omitted)")
	fs.StringVar(&versionFlag, "version", "", "Bundle version to resolve on the channel (optional; channel head when omitted)")
	fs.StringVar(&mustGatherFlag, "must-gather", "", "Path or glob pattern to must-gather directory(ies) for fault analysis (optional)")
	fs.BoolVar(&jsonOut, "json", false, "Output JSON instead of human-readable lines")
	fs.DurationVar(&timeoutDuration, "timeout", defaultTimeout, "Overall timeout for catalog render and bundle inspect")
	fs.BoolVar(&telcoSuite, "telco-suite", false, "Analyze full telco suite: OADP, TALM, IDMS, MCH")
	fs.BoolVar(&healthCheck, "health-check", true, "Run 20-dimension systematic health checks (must-gather mode)")
	fs.StringVar(&environment, "environment", "production", "Deployment environment for noise filtering: production, disconnected, lab, kvm")
	fs.StringVar(&sourceRepo, "source-repo", "", "Local path to operator source repo for code-level analysis")
	fs.BoolVar(&generateRCA, "rca", false, "Generate professional RCA markdown report")
	fs.StringVar(&rcaFile, "rca-file", "", "Write RCA markdown to file (implies --rca)")
	fs.StringVar(&clusterName, "cluster-name", "", "Cluster identifier for session persistence across redeployments")
	fs.StringVar(&stateDir, "state-dir", "", "Directory for session state (default: ~/.config/opm-troubleshooting/sessions)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	if timeoutDuration <= 0 {
		timeoutDuration = defaultTimeout
	}

	// Validation for must-gather mode
	if mustGatherFlag != "" {
		if packageFlag == "" && !telcoSuite {
			return nil, errors.New("--package is required when using --must-gather (or use --telco-suite)")
		}
		// catalog is optional in must-gather mode
	} else {
		// Normal bundle inspection mode requires catalog
		if catalogFlag == "" {
			return nil, errors.New("--catalog is required")
		}
		if packageFlag == "" {
			return nil, errors.New("--package is required")
		}
	}

	if rcaFile != "" {
		generateRCA = true
	}

	return &config{
		catalog:        catalogFlag,
		packageName:    packageFlag,
		channel:        channelFlag,
		version:        versionFlag,
		jsonOut:        jsonOut,
		timeout:        timeoutDuration,
		mustGatherPath: mustGatherFlag,
		telcoSuite:     telcoSuite,
		healthCheck:    healthCheck,
		environment:    environment,
		sourceRepo:     sourceRepo,
		generateRCA:    generateRCA,
		rcaFile:        rcaFile,
		clusterName:    clusterName,
		stateDir:       stateDir,
	}, nil
}

// preprocessMustGatherArgs handles shell-expanded globs for --must-gather.
// Converts: [--must-gather, /path/dir1, /path/dir2, /path/dir3, --package, foo]
// To:       [--must-gather, /path/dir1,/path/dir2,/path/dir3, --package, foo]
func preprocessMustGatherArgs(args []string) []string {
	result := make([]string, 0, len(args))
	i := 0

	for i < len(args) {
		if args[i] == "--must-gather" && i+1 < len(args) {
			result = append(result, args[i])
			i++

			// Collect all non-flag arguments that follow --must-gather
			var paths []string
			for i < len(args) && !strings.HasPrefix(args[i], "-") {
				paths = append(paths, args[i])
				i++
			}

			// Join paths with comma separator
			if len(paths) > 0 {
				result = append(result, strings.Join(paths, ","))
			}
		} else {
			result = append(result, args[i])
			i++
		}
	}

	return result
}

func printUsage(w io.Writer) {
	const usage = `catalog-bundle-inspect resolves the channel-head OLM bundle from a catalog index image
and prints bundle image metadata (package, version, commit, URL).

With --must-gather, analyzes faulty operators and correlates code changes with failure symptoms
using AI-powered fault isolation.

Implemented in pure Go (operator-registry + go-containerregistry). No opm, jq, or skopeo.

Registry authentication uses DOCKER_CONFIG (directory containing config.json)
or REGISTRY_AUTH_FILE from the environment.

Usage:
  catalog-bundle-inspect --catalog <index-image> --package <name> [--channel <name>]
  catalog-bundle-inspect --must-gather <path> --package <name> [--catalog <index>] [--version <target>]

Flags:
  -c, --catalog string      OLM catalog index image reference (required for bundle inspection)
  -p, --package string      OLM package name (required)
      --channel string      OLM channel name (optional; package defaultChannel when omitted)
      --version string      Bundle version on the channel (optional; channel head when omitted)
      --must-gather string  Path or glob pattern to must-gather directory(ies) for fault analysis (optional)
      --telco-suite         Analyze full telco suite: OADP, TALM, IDMS, MCH
      --health-check        Run 20-dimension systematic health checks (default true)
      --environment string  Environment for noise filtering: production, disconnected, lab, kvm (default production)
      --source-repo string  Local path to operator source repo for code-level analysis
      --rca                 Generate professional RCA markdown report
      --rca-file string     Write RCA markdown to file (implies --rca)
      --cluster-name string Cluster identifier for session persistence across redeployments
      --state-dir string    Directory for session state (default ~/.config/opm-troubleshooting/sessions)
      --json                Output JSON instead of human-readable lines
      --timeout duration    Overall timeout for catalog render and bundle inspect (default 10m)
  -h, --help                Show this help message

Must-Gather Mode:
  Analyzes faulty operators from one or more must-gather dumps and uses Claude API to correlate
  code changes with failure symptoms.

  The --must-gather flag supports:
    - Direct path: /path/to/must-gather.local.123456
    - Glob pattern: /path/to/must-gathers/*
    - Multiple directories will be analyzed and results merged

  Requires: ANTHROPIC_API_KEY environment variable

  Example (single directory):
    export ANTHROPIC_API_KEY=sk-ant-...
    export DOCKER_CONFIG=~/.docker
    catalog-bundle-inspect \
      --must-gather /path/to/must-gather.local.123456 \
      --package kubernetes-nmstate-operator \
      --catalog quay.io/prega/prega-operator-index:v4.22-latest \
      --version v4.22.0

  Example (multiple directories with glob):
    catalog-bundle-inspect \
      --must-gather '/path/to/must-gathers/*' \
      --package advanced-cluster-management

The channel head is the last entry in the catalog channel (FBC order), not semver-sorted.
With --version, the tool selects a bundle on the channel whose version matches (e.g. v2.11.2 matches 2.11.2-509).

Example:
  export DOCKER_CONFIG=/path/to/dir_with_config_json
  catalog-bundle-inspect \
    --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
    --package kubernetes-nmstate-operator \
    --channel stable
`
	_, _ = fmt.Fprint(w, usage)
}

func writeOutput(w io.Writer, info *imageinspect.BundleInfo, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(info)
	}

	lines := []string{
		fmt.Sprintf("package: %s", info.Package),
		fmt.Sprintf("bundle:  %s", info.Bundle),
		fmt.Sprintf("version: %s", info.Version),
		fmt.Sprintf("commit:  %s", info.Commit),
		fmt.Sprintf("url:     %s", info.URL),
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// runMustGatherAnalysis executes the must-gather fault analysis workflow.
func runMustGatherAnalysis(ctx context.Context, cfg *config) error {
	// Expand glob pattern to find all matching directories
	paths, err := expandMustGatherPaths(cfg.mustGatherPath)
	if err != nil {
		return &CLIError{Code: exitUsage, Err: fmt.Errorf("expand must-gather path: %w", err)}
	}

	if len(paths) == 0 {
		return &CLIError{Code: exitUsage, Err: fmt.Errorf("no must-gather directories found matching: %s", cfg.mustGatherPath)}
	}

	// Analyze each must-gather directory
	allResults := make([]*analysis.AnalysisResult, 0, len(paths))
	for _, path := range paths {
		analysisConfig := analysis.AnalysisConfig{
			MustGatherPath:        path,
			CatalogRef:            cfg.catalog,
			TargetVersion:         cfg.version,
			PackageName:           cfg.packageName,
			TelcoSuite:            cfg.telcoSuite,
			HealthCheck:           cfg.healthCheck,
			Environment:           noise.ParseEnvironment(cfg.environment),
			SourceRepo:            cfg.sourceRepo,
			GenerateRCA:           cfg.generateRCA,
			ClusterName:           cfg.clusterName,
			StateDir:              cfg.stateDir,
			ADHDEnabled:           cfg.adhdEnabled,
			ADHDFrames:            cfg.adhdFrames,
			ADHDDepth:             cfg.adhdDepth,
			MetadataDir:           cfg.metadataDir,
			EnableLearning:        cfg.enableLearning,
			EnableRepoCorrelation: cfg.enableRepoCorr,
			GitHubToken:           cfg.githubToken,
			RAGEnabled:            cfg.ragEnabled,
			RAGConfigPath:         cfg.ragConfigPath,
			RAGDataDir:            cfg.ragDataDir,
		}

		result, err := analysis.AnalyzeMustGather(ctx, analysisConfig)
		if err != nil {
			// Log error but continue with other directories
			fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", path, err)
			continue
		}

		allResults = append(allResults, result)
	}

	if len(allResults) == 0 {
		return &CLIError{Code: exitOperation, Err: errors.New("no must-gather directories were successfully analyzed")}
	}

	// Merge results if multiple directories
	finalResult := mergeAnalysisResults(allResults)

	if err := writeRCAFiles(finalResult, cfg); err != nil {
		return &CLIError{Code: exitOperation, Err: err}
	}

	return writeMustGatherOutput(os.Stdout, finalResult, cfg.jsonOut)
}

// writeRCAFiles writes RCA markdown documents to files if requested.
func writeRCAFiles(result *analysis.AnalysisResult, cfg *config) error {
	if len(result.RCADocuments) == 0 {
		return nil
	}

	if cfg.rcaFile != "" {
		// Write combined RCA or first document
		content := result.RCADocuments[0].Markdown
		if len(result.RCADocuments) > 1 {
			var combined strings.Builder
			for _, doc := range result.RCADocuments {
				combined.WriteString(doc.Markdown)
				combined.WriteString("\n---\n\n")
			}
			content = combined.String()
		}
		return os.WriteFile(cfg.rcaFile, []byte(content), 0o600)
	}

	// Auto-write to state dir if configured
	if cfg.stateDir != "" && result.ClusterName != "" {
		for i, doc := range result.RCADocuments {
			filename := fmt.Sprintf("rca-%s-%d.md", result.ClusterName, i+1)
			path := filepath.Join(cfg.stateDir, filename)
			if err := os.WriteFile(path, []byte(doc.Markdown), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write RCA to %s: %v\n", path, err)
			}
		}
	}

	return nil
}

// expandMustGatherPaths expands glob patterns and returns all matching directories.
func expandMustGatherPaths(pattern string) ([]string, error) {
	var allDirs []string

	// Handle comma-separated paths (from shell-expanded globs)
	patterns := strings.Split(pattern, ",")

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Check if it's a direct path first (no glob characters)
		if !strings.ContainsAny(p, "*?[]") {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				allDirs = append(allDirs, p)
				continue
			}
		}

		// Try glob expansion
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("glob pattern failed for %s: %w", p, err)
		}

		// Filter to only include directories
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.IsDir() {
				allDirs = append(allDirs, match)
			}
		}
	}

	return allDirs, nil
}

// mergeAnalysisResults combines multiple analysis results into one.
func mergeAnalysisResults(results []*analysis.AnalysisResult) *analysis.AnalysisResult {
	if len(results) == 1 {
		return results[0]
	}

	merged := &analysis.AnalysisResult{
		MustGatherPath: fmt.Sprintf("%d directories", len(results)),
		Operators:      make([]mustgather.OperatorState, 0),
		FaultyReports:  make([]analysis.FaultReport, 0),
		Errors:         make([]error, 0),
	}

	for _, result := range results {
		merged.Operators = append(merged.Operators, result.Operators...)
		merged.FaultyReports = append(merged.FaultyReports, result.FaultyReports...)
		merged.RCADocuments = append(merged.RCADocuments, result.RCADocuments...)
		merged.TotalOperators += result.TotalOperators
		merged.FaultyCount += result.FaultyCount
		merged.AnalyzedCount += result.AnalyzedCount
		merged.Errors = append(merged.Errors, result.Errors...)
		if result.Environment != "" {
			merged.Environment = result.Environment
		}
		if result.ClusterName != "" {
			merged.ClusterName = result.ClusterName
		}
		if result.Session != nil {
			merged.Session = result.Session
		}
	}

	return merged
}

// writeMustGatherOutput writes must-gather analysis results in human-readable or JSON format.
func writeMustGatherOutput(w io.Writer, result *analysis.AnalysisResult, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Human-readable output
	fmt.Fprintf(w, "Must-Gather Analysis: %s\n", result.MustGatherPath)
	if result.ClusterName != "" {
		fmt.Fprintf(w, "Cluster: %s\n", result.ClusterName)
	}
	if result.Environment != "" {
		fmt.Fprintf(w, "Environment: %s\n", result.Environment)
	}
	if result.Session != nil {
		fmt.Fprintf(w, "Redeployment #%d (session since %s)\n",
			result.Session.RedeploymentCount,
			result.Session.FirstSeen.Format("2006-01-02"))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(w, "Parse Errors: %d\n", len(result.Errors))
	}
	fmt.Fprintf(w, "Total Operators: %d\n", result.TotalOperators)
	fmt.Fprintf(w, "Faulty Operators: %d\n", result.FaultyCount)
	fmt.Fprintf(w, "Analyzed: %d\n\n", result.AnalyzedCount)

	for i, report := range result.FaultyReports {
		fmt.Fprintf(w, "=== Fault Report #%d: %s ===\n", i+1, report.Operator.PackageName)
		if report.TelcoProfile != nil {
			fmt.Fprintf(w, "Telco Suite: %s (%s)\n", report.TelcoProfile.ID, report.TelcoProfile.DisplayName)
		}
		fmt.Fprintf(w, "Namespace: %s\n", report.Operator.Namespace)
		fmt.Fprintf(w, "Channel: %s\n", report.Operator.Channel)
		fmt.Fprintf(w, "Installed CSV: %s (%s)\n", report.Operator.InstalledCSV, report.Operator.InstalledVersion)
		fmt.Fprintf(w, "State: %s\n", report.Operator.State)
		fmt.Fprintf(w, "Failure Reason: %s\n\n", report.Operator.FailureReason)

		if report.HealthReport != nil {
			hr := report.HealthReport
			fmt.Fprintf(w, "Health Check (20 dimensions): %d pass, %d fail, %d warn, %d skip\n",
				hr.Passed, hr.Failed, hr.Warnings, hr.Skipped)
			for _, dim := range hr.Dimensions {
				if dim.Status == "Fail" || dim.Status == "Warn" {
					fmt.Fprintf(w, "  [%s] %s: %s\n", dim.Status, dim.Name, dim.Summary)
				}
			}
			fmt.Fprintln(w)
		}

		if report.NoiseReport != nil && report.NoiseReport.TotalFindings > 0 {
			nr := report.NoiseReport
			fmt.Fprintf(w, "Noise Filter: %d real, %d cosmetic, %d ambiguous\n",
				nr.RealIssues, nr.CosmeticAlerts, nr.AmbiguousAlerts)
			for _, f := range nr.Findings {
				if f.Classification == "cosmetic" {
					fmt.Fprintf(w, "  [COSMETIC] %s: %s\n", f.Dimension.Name, f.NoiseReason)
				} else if f.ActionRequired {
					fmt.Fprintf(w, "  [REAL] %s: %s\n", f.Dimension.Name, f.Dimension.Summary)
				}
			}
			fmt.Fprintln(w)
		}

		if report.CodeAnalysis != nil && len(report.CodeAnalysis.Matches) > 0 {
			fmt.Fprintf(w, "Code-Level Evidence: %s\n", report.CodeAnalysis.Summary)
			for j, m := range report.CodeAnalysis.Matches {
				if j >= 5 {
					fmt.Fprintf(w, "  ... and %d more matches\n", len(report.CodeAnalysis.Matches)-5)
					break
				}
				fmt.Fprintf(w, "  %s:%d %s\n", m.FilePath, m.LineNumber, m.LineContent)
			}
			fmt.Fprintln(w)
		}

		if report.InstalledBundle != nil {
			fmt.Fprintf(w, "Installed Bundle:\n")
			fmt.Fprintf(w, "  Version: %s\n", report.InstalledBundle.Version)
			fmt.Fprintf(w, "  Commit:  %s\n", report.InstalledBundle.Commit)
			fmt.Fprintf(w, "  URL:     %s\n\n", report.InstalledBundle.URL)
		}

		if report.TargetBundle != nil {
			fmt.Fprintf(w, "Target Bundle:\n")
			fmt.Fprintf(w, "  Version: %s\n", report.TargetBundle.Version)
			fmt.Fprintf(w, "  Commit:  %s\n", report.TargetBundle.Commit)
			fmt.Fprintf(w, "  URL:     %s\n\n", report.TargetBundle.URL)
		}

		if report.CommitDelta != nil {
			fmt.Fprintf(w, "Code Changes:\n")
			fmt.Fprintf(w, "  Files Changed: %d\n", len(report.CommitDelta.FilesChanged))
			fmt.Fprintf(w, "  Additions: +%d\n", report.CommitDelta.Additions)
			fmt.Fprintf(w, "  Deletions: -%d\n\n", report.CommitDelta.Deletions)
		}

		if len(report.RCAPatterns) > 0 {
			fmt.Fprintf(w, "RCA Pattern Analysis:\n")
			for _, pattern := range report.RCAPatterns {
				fmt.Fprintf(w, "  Pattern: %s (Confidence: %.0f%%)\n", pattern.Pattern, pattern.Confidence*100)
				fmt.Fprintf(w, "  Description: %s\n", pattern.Description)
				if len(pattern.Evidence) > 0 {
					fmt.Fprintf(w, "  Evidence: %s\n", strings.Join(pattern.Evidence, ", "))
				}
			}
			fmt.Fprintln(w)
		}

		if len(report.Recommendations) > 0 {
			fmt.Fprintf(w, "Recommended Fixes:\n")
			for i, rec := range report.Recommendations {
				priority := "Medium"
				if rec.Priority == 1 {
					priority = "Critical"
				} else if rec.Priority == 2 {
					priority = "High"
				}
				fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, priority, rec.Title)
				fmt.Fprintf(w, "     %s\n", rec.Description)
				if rec.CodeExample != "" {
					fmt.Fprintf(w, "     Example:\n")
					for _, line := range strings.Split(rec.CodeExample, "\n") {
						fmt.Fprintf(w, "       %s\n", line)
					}
				}
			}
			fmt.Fprintln(w)
		}

		if report.ClaudeAnalysis != nil {
			fmt.Fprintf(w, "AI Fault Analysis:\n")
			fmt.Fprintf(w, "  Summary: %s\n\n", report.ClaudeAnalysis.Summary)

			if len(report.ClaudeAnalysis.LikelyCauses) > 0 {
				fmt.Fprintf(w, "  Likely Causes:\n")
				for _, cause := range report.ClaudeAnalysis.LikelyCauses {
					fmt.Fprintf(w, "    - %s\n", cause)
				}
				fmt.Fprintln(w)
			}

			if len(report.ClaudeAnalysis.RecommendedActions) > 0 {
				fmt.Fprintf(w, "  Recommended Actions:\n")
				for _, action := range report.ClaudeAnalysis.RecommendedActions {
					fmt.Fprintf(w, "    - %s\n", action)
				}
				fmt.Fprintln(w)
			}

			fmt.Fprintf(w, "  Confidence: %s\n\n", report.ClaudeAnalysis.Confidence)
		}

		if len(report.Errors) > 0 {
			fmt.Fprintf(w, "  Errors during analysis:\n")
			for _, err := range report.Errors {
				fmt.Fprintf(w, "    - %s\n", err)
			}
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w, "---")
	}

	if len(result.RCADocuments) > 0 {
		fmt.Fprintf(w, "\n=== RCA Reports Generated: %d ===\n", len(result.RCADocuments))
		for i, doc := range result.RCADocuments {
			fmt.Fprintf(w, "RCA #%d: %s (%s)\n", i+1, doc.Title, doc.GeneratedAt.Format(time.RFC3339))
		}
		fmt.Fprintln(w, "Use --rca-file to write full markdown report")
	}

	return nil
}
