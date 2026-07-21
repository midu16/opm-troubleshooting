package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/noise"
)

// telcoConfig holds telco-diagnose specific CLI options.
type telcoConfig struct {
	mustGatherPath string
	catalog        string
	packageName    string
	version        string
	jsonOut        bool
	timeout        time.Duration
	environment    string
	sourceRepo     string
	rcaFile        string
	clusterName    string
	stateDir       string
	skipRCA        bool
	skipHealth     bool
}

// RunTelcoDiagnose executes the dedicated telco production diagnosis workflow.
// Defaults: full OADP/TALM/IDMS/MCH suite, 20-dimension health checks, RCA generation.
func RunTelcoDiagnose(args []string) error {
	cfg, err := parseTelcoArgs(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			return nil
		}
		return &CLIError{Code: exitUsage, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	mgCfg := &config{
		mustGatherPath: cfg.mustGatherPath,
		catalog:        cfg.catalog,
		packageName:    cfg.packageName,
		version:        cfg.version,
		jsonOut:        cfg.jsonOut,
		timeout:        cfg.timeout,
		telcoSuite:     cfg.packageName == "",
		healthCheck:    !cfg.skipHealth,
		environment:    cfg.environment,
		sourceRepo:     cfg.sourceRepo,
		generateRCA:    !cfg.skipRCA,
		rcaFile:        cfg.rcaFile,
		clusterName:    cfg.clusterName,
		stateDir:       cfg.stateDir,
	}

	return runMustGatherAnalysis(ctx, mgCfg)
}

func parseTelcoArgs(args []string) (*telcoConfig, error) {
	args = preprocessMustGatherArgs(args)

	fs := flag.NewFlagSet("telco-diagnose", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		printTelcoUsage(os.Stdout)
	}

	var (
		mustGatherFlag  string
		catalogFlag     string
		packageFlag     string
		versionFlag     string
		jsonOut         bool
		timeoutDuration time.Duration
		environment     string
		sourceRepo      string
		rcaFile         string
		clusterName     string
		stateDir        string
		skipRCA         bool
		skipHealth      bool
	)

	fs.StringVar(&mustGatherFlag, "must-gather", "", "Path or glob to must-gather directory (required)")
	fs.StringVar(&mustGatherFlag, "m", "", "Must-gather path (shorthand)")
	fs.StringVar(&catalogFlag, "catalog", "", "OLM catalog index image for bundle/code delta analysis")
	fs.StringVar(&catalogFlag, "c", "", "Catalog index image (shorthand)")
	fs.StringVar(&packageFlag, "package", "", "Single telco operator to diagnose (default: full OADP/TALM/IDMS/MCH suite)")
	fs.StringVar(&packageFlag, "p", "", "Operator package name (shorthand)")
	fs.StringVar(&versionFlag, "version", "", "Target bundle version for code delta comparison")
	fs.StringVar(&environment, "environment", "production", "Noise filter profile: production, disconnected, lab, kvm")
	fs.StringVar(&sourceRepo, "source-repo", "", "Local operator source repo for code-level correlation")
	fs.StringVar(&rcaFile, "rca-file", "", "Write RCA markdown report to file")
	fs.StringVar(&clusterName, "cluster-name", "", "Cluster ID for session persistence across redeployments")
	fs.StringVar(&stateDir, "state-dir", "", "Session state directory")
	fs.BoolVar(&jsonOut, "json", false, "Output JSON")
	fs.BoolVar(&skipRCA, "no-rca", false, "Skip RCA markdown generation")
	fs.BoolVar(&skipHealth, "no-health-check", false, "Skip 20-dimension health checks")
	fs.DurationVar(&timeoutDuration, "timeout", defaultTimeout, "Overall timeout")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	if mustGatherFlag == "" {
		return nil, errors.New("--must-gather is required")
	}
	if timeoutDuration <= 0 {
		timeoutDuration = defaultTimeout
	}

	return &telcoConfig{
		mustGatherPath: mustGatherFlag,
		catalog:        catalogFlag,
		packageName:    packageFlag,
		version:        versionFlag,
		jsonOut:        jsonOut,
		timeout:        timeoutDuration,
		environment:    environment,
		sourceRepo:     sourceRepo,
		rcaFile:        rcaFile,
		clusterName:    clusterName,
		stateDir:       stateDir,
		skipRCA:        skipRCA,
		skipHealth:     skipHealth,
	}, nil
}

func printTelcoUsage(w io.Writer) {
	const usage = `telco-diagnose — production-grade telco operator diagnosis

Fast root-cause analysis for the telco production operator suite (27 OLM operators + IDMS):

  Cluster:  ACM, MCE
  Lifecycle: TALM, Lifecycle Agent, GitOps
  Logging:  Cluster Logging
  Network:  NMState, MetalLB, SR-IOV, PTP, NUMA Resources
  Storage:  Local Storage, LVMS
  ODF:      ODF, Rook Ceph, CSI, OCS, MCG, CSI Addons, Dependencies, Snapshotter, Prometheus, Recipe
  Backup:   OADP
  Config:   IDMS (mirror), O-Cloud Manager, Cert Manager

Runs 20 systematic health dimensions, filters cosmetic lab/KVM/disconnected noise,
correlates operator source code, and generates shareable RCA documentation.
Maintains context across redeployments via session store.

Usage:
  telco-diagnose --must-gather <path> [options]

Required:
  -m, --must-gather string   Must-gather directory or glob pattern

Optional:
  -c, --catalog string       Catalog index image for bundle/code delta analysis
  -p, --package string       Single operator (default: full telco suite)
      --version string       Target bundle version for code comparison
      --environment string   Noise filter: production, disconnected, lab, kvm (default production)
      --source-repo string   Local operator source repo for code-level analysis
      --rca-file string      Write RCA markdown to file
      --cluster-name string  Cluster ID for redeployment session tracking
      --state-dir string     Session store directory
      --no-rca               Skip RCA report generation
      --no-health-check      Skip 20-dimension health checks
      --json                 JSON output
      --timeout duration     Timeout (default 10m)

Environment:
  DOCKER_CONFIG              Registry credentials for catalog/bundle pulls
  ANTHROPIC_API_KEY          Optional: AI code-change correlation

Examples:
  # Full telco suite on lab cluster
  telco-diagnose \
    --must-gather /path/to/must-gather.local.123456 \
    --environment lab \
    --cluster-name edge-lab-01 \
    --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
    --rca-file /tmp/telco-rca.md

  # Single operator with local source repo
  telco-diagnose \
    --must-gather /path/to/must-gather \
    --package redhat-oadp-operator \
    --source-repo ~/src/oadp-operator \
    --environment disconnected \
    --rca-file /tmp/oadp-rca.md

  # Redeployment iteration (session auto-tracked)
  telco-diagnose \
    --must-gather /path/to/must-gather-redeploy-3 \
    --cluster-name production-hub-01 \
    --state-dir ~/.config/opm-troubleshooting/sessions
`
	_, _ = fmt.Fprint(w, usage)
}

// ValidateTelcoEnvironment checks environment string is recognized.
func ValidateTelcoEnvironment(env string) bool {
	switch noise.ParseEnvironment(env) {
	case noise.EnvProduction, noise.EnvDisconnected, noise.EnvLab, noise.EnvKVM:
		return strings.TrimSpace(env) != "" || env == ""
	default:
		return false
	}
}
