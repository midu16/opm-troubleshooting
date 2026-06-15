// Batch validation: one catalog render, then resolve + inspect per operator (same logic as CLI).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/testfixture"
	"github.com/midu16/opm-troubleshooting/internal/workflow"
)

type result struct {
	packageName string
	channel     string
	status      string // OK, PARTIAL, FAIL
	detail      string
}

func main() {
	catalogRef := envOr("CATALOG", "registry.redhat.io/redhat/redhat-operator-index:v4.22")
	listPath := envOr("LIST", testfixture.OperatorsPath())
	timeout := 20 * time.Minute

	operators, err := testfixture.LoadOperatorsFromPath(listPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read list: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Rendering catalog %s ...\n", catalogRef)
	declCfg, err := catalog.RenderCatalog(ctx, catalogRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render catalog: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "Catalog rendered. Checking %d operators ...\n", len(operators))

	var ok, partial, fail int
	for _, op := range operators {
		r := checkOperator(ctx, declCfg, op.Package, op.Channel)
		switch r.status {
		case "OK":
			ok++
		case "PARTIAL":
			partial++
		default:
			fail++
		}
		fmt.Printf("%-6s %-40s %-25s %s\n", r.status, r.packageName, r.channel, r.detail)
	}
	fmt.Fprintf(os.Stderr, "\nTotal: %d OK, %d PARTIAL (missing commit or url), %d FAIL\n", ok, partial, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func checkOperator(ctx context.Context, cfg *declcfg.DeclarativeConfig, pkg, ch string) result {
	r := result{packageName: pkg, channel: ch}
	res, err := workflow.InspectChannelHeadFromConfig(ctx, cfg, pkg, ch)
	if err != nil {
		r.status = "FAIL"
		r.detail = err.Error()
		return r
	}
	info := res.Info
	if info.Package == "" || info.Bundle == "" || info.Version == "" {
		r.status = "FAIL"
		r.detail = "missing package/bundle/version in inspect output"
		return r
	}
	if info.Commit == "" || info.URL == "" {
		r.status = "PARTIAL"
		r.detail = fmt.Sprintf("version=%s commit=%q url=%q", info.Version, info.Commit, info.URL)
		return r
	}
	r.status = "OK"
	r.detail = fmt.Sprintf("version=%s commit=%s", info.Version, info.Commit)
	return r
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
