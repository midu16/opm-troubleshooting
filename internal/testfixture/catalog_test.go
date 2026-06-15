package testfixture_test

import (
	"strings"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/testfixture"
)

func TestBuildDeclarativeConfig_allOperatorsResolveChannelHead(t *testing.T) {
	operators, err := testfixture.LoadOperators()
	if err != nil {
		t.Fatalf("LoadOperators: %v", err)
	}

	cfg, err := testfixture.BuildDeclarativeConfig("127.0.0.1:5000", operators)
	if err != nil {
		t.Fatalf("BuildDeclarativeConfig: %v", err)
	}

	for _, op := range operators {
		bundleName, imageRef, err := catalog.ResolveChannelHead(cfg, op.Package, op.Channel)
		if err != nil {
			t.Errorf("%s/%s: ResolveChannelHead: %v", op.Package, op.Channel, err)
			continue
		}
		if bundleName == "" || imageRef == "" {
			t.Errorf("%s/%s: empty bundle or image", op.Package, op.Channel)
		}
		vers := testfixture.VersionsForOperator(op)
		if !containsVersion(bundleName, vers.Newer) {
			t.Errorf("%s/%s: channel head %q want newer version %q", op.Package, op.Channel, bundleName, vers.Newer)
		}
	}
}

func TestBuildDeclarativeConfig_versionResolution_multiclusterEngine(t *testing.T) {
	operators, err := testfixture.LoadOperators()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := testfixture.BuildDeclarativeConfig("127.0.0.1:5000", operators)
	if err != nil {
		t.Fatal(err)
	}

	bundleName, _, err := catalog.ResolveChannelVersion(cfg, "multicluster-engine", "stable-2.11", "v2.11.1")
	if err != nil {
		t.Fatalf("ResolveChannelVersion: %v", err)
	}
	if !containsVersion(bundleName, "2.11.1-429") {
		t.Fatalf("bundle: %q", bundleName)
	}
}

func containsVersion(bundleName, version string) bool {
	v := version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return strings.Contains(bundleName, v)
}
