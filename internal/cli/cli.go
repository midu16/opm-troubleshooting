package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
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
type CLIError struct {
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
	catalog     string
	packageName string
	channel     string
	version     string
	jsonOut     bool
	timeout     time.Duration
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
		jsonOut         bool
		timeoutDuration time.Duration
	)

	fs.StringVar(&catalogFlag, "catalog", "", "OLM catalog index image reference")
	fs.StringVar(&catalogFlag, "c", "", "OLM catalog index image reference (shorthand)")
	fs.StringVar(&packageFlag, "package", "", "OLM package name")
	fs.StringVar(&packageFlag, "p", "", "OLM package name (shorthand)")
	fs.StringVar(&channelFlag, "channel", "", "OLM channel name (optional; uses package defaultChannel when omitted)")
	fs.StringVar(&versionFlag, "version", "", "Bundle version to resolve on the channel (optional; channel head when omitted)")
	fs.BoolVar(&jsonOut, "json", false, "Output JSON instead of human-readable lines")
	fs.DurationVar(&timeoutDuration, "timeout", defaultTimeout, "Overall timeout for catalog render and bundle inspect")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	if timeoutDuration <= 0 {
		timeoutDuration = defaultTimeout
	}

	if catalogFlag == "" {
		return nil, errors.New("--catalog is required")
	}
	if packageFlag == "" {
		return nil, errors.New("--package is required")
	}

	return &config{
		catalog:     catalogFlag,
		packageName: packageFlag,
		channel:     channelFlag,
		version:     versionFlag,
		jsonOut:     jsonOut,
		timeout:     timeoutDuration,
	}, nil
}

func printUsage(w io.Writer) {
	const usage = `catalog-bundle-inspect resolves the channel-head OLM bundle from a catalog index image
and prints bundle image metadata (package, version, commit, URL).

Implemented in pure Go (operator-registry + go-containerregistry). No opm, jq, or skopeo.

Registry authentication uses DOCKER_CONFIG (directory containing config.json)
or REGISTRY_AUTH_FILE from the environment.

Usage:
  catalog-bundle-inspect --catalog <index-image> --package <name> [--channel <name>]

Flags:
  -c, --catalog string   OLM catalog index image reference (required)
  -p, --package string   OLM package name (required)
      --channel string   OLM channel name (optional; package defaultChannel when omitted)
      --version string   Bundle version on the channel (optional; channel head when omitted)
      --json             Output JSON instead of human-readable lines
      --timeout duration Overall timeout for catalog render and bundle inspect (default 10m)
  -h, --help             Show this help message

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
