package functional_test

import (
	"context"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/testfixture"
	"github.com/midu16/opm-troubleshooting/internal/workflow"
)

func TestRedHatOperatorFixture_mockRegistryInspectAll(t *testing.T) {
	operators, err := testfixture.LoadOperators()
	if err != nil {
		t.Fatalf("LoadOperators: %v", err)
	}

	specs := testfixture.BundleImageSpecs(operators)
	registryHost := testfixture.StartMockRegistry(t, specs)

	cfg, err := testfixture.BuildDeclarativeConfig(registryHost, operators)
	if err != nil {
		t.Fatalf("BuildDeclarativeConfig: %v", err)
	}

	ctx := context.Background()
	for _, op := range operators {
		result, err := workflow.InspectChannelHeadFromConfig(ctx, cfg, op.Package, op.Channel)
		if err != nil {
			t.Errorf("%s/%s: inspect: %v", op.Package, op.Channel, err)
			continue
		}
		info := result.Info
		if info.Package != op.Package {
			t.Errorf("%s: package %q", op.Package, info.Package)
		}
		if info.Version == "" || info.Commit == "" || info.URL == "" {
			t.Errorf("%s: incomplete info version=%q commit=%q url=%q", op.Package, info.Version, info.Commit, info.URL)
		}
	}
}

func TestRedHatOperatorFixture_versionInspect_multiclusterEngine(t *testing.T) {
	operators, err := testfixture.LoadOperators()
	if err != nil {
		t.Fatal(err)
	}
	var mce []testfixture.Operator
	for _, op := range operators {
		if op.Package == "multicluster-engine" {
			mce = append(mce, op)
		}
	}
	if len(mce) != 1 {
		t.Fatalf("multicluster-engine operators: %d", len(mce))
	}

	specs := testfixture.BundleImageSpecs(mce)
	registryHost := testfixture.StartMockRegistry(t, specs)
	cfg, err := testfixture.BuildDeclarativeConfig(registryHost, mce)
	if err != nil {
		t.Fatal(err)
	}

	result, err := workflow.InspectBundleFromConfig(context.Background(), cfg, "multicluster-engine", "stable-2.11", "v2.11.1")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if result.Info.Version != "2.11.1-429" {
		t.Fatalf("version: %q", result.Info.Version)
	}

	_, imageRef, err := catalog.ResolveChannelVersion(cfg, "multicluster-engine", "stable-2.11", "v2.11.2")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if imageRef == "" {
		t.Fatal("empty image ref")
	}
}
