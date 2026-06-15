package testfixture

import (
	"fmt"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// BundleVersions holds synthetic older and newer bundle versions for mock catalogs.
type BundleVersions struct {
	Older string
	Newer string
}

// BundleImageSpec describes a bundle image pushed to the mock registry.
type BundleImageSpec struct {
	RepoPath string
	Package  string
	Version  string
	Commit   string
	URL      string
}

// VersionsForOperator returns bundle versions for mock catalog entries.
func VersionsForOperator(op Operator) BundleVersions {
	switch op.Package {
	case "multicluster-engine":
		return BundleVersions{Older: "2.11.1-429", Newer: "2.11.2-509"}
	case "advanced-cluster-management":
		return BundleVersions{Older: "2.16.1-400", Newer: "2.16.2-454"}
	default:
		return BundleVersions{Older: "1.0.0", Newer: "1.0.1"}
	}
}

// BuildDeclarativeConfig builds an FBC catalog for all operators pointing at a mock registry host.
func BuildDeclarativeConfig(registryHost string, operators []Operator) (*declcfg.DeclarativeConfig, error) {
	if registryHost == "" {
		return nil, fmt.Errorf("registry host is required")
	}
	cfg := &declcfg.DeclarativeConfig{}
	for _, op := range operators {
		vers := VersionsForOperator(op)
		olderBundle := bundleName(op.Package, vers.Older)
		newerBundle := bundleName(op.Package, vers.Newer)

		cfg.Packages = append(cfg.Packages, declcfg.Package{
			Schema:         declcfg.SchemaPackage,
			Name:           op.Package,
			DefaultChannel: op.Channel,
		})
		cfg.Channels = append(cfg.Channels, declcfg.Channel{
			Schema:  declcfg.SchemaChannel,
			Package: op.Package,
			Name:    op.Channel,
			Entries: []declcfg.ChannelEntry{
				{Name: olderBundle},
				{Name: newerBundle},
			},
		})

		for _, spec := range bundleImageSpecs(op, vers) {
			b, err := bundleRecord(registryHost, op.Package, spec)
			if err != nil {
				return nil, err
			}
			cfg.Bundles = append(cfg.Bundles, b)
		}
	}
	return cfg, nil
}

// BundleImageSpecs returns image specs for all bundles in the mock catalog.
func BundleImageSpecs(operators []Operator) []BundleImageSpec {
	var specs []BundleImageSpec
	for _, op := range operators {
		vers := VersionsForOperator(op)
		specs = append(specs, bundleImageSpecs(op, vers)...)
	}
	return specs
}

func bundleImageSpecs(op Operator, vers BundleVersions) []BundleImageSpec {
	return []BundleImageSpec{
		newBundleImageSpec(op, vers.Older),
		newBundleImageSpec(op, vers.Newer),
	}
}

func newBundleImageSpec(op Operator, version string) BundleImageSpec {
	commit := mockCommit(op.Package, version)
	return BundleImageSpec{
		RepoPath: repoPath(op.Package, version),
		Package:  op.Package,
		Version:  version,
		Commit:   commit,
		URL:      mockCommitURL(op.Package, commit),
	}
}

func bundleRecord(registryHost, packageName string, spec BundleImageSpec) (declcfg.Bundle, error) {
	pkgProp, err := property.Build(&property.Package{
		PackageName: packageName,
		Version:     spec.Version,
	})
	if err != nil {
		return declcfg.Bundle{}, err
	}
	return declcfg.Bundle{
		Schema:  declcfg.SchemaBundle,
		Name:    bundleName(packageName, spec.Version),
		Package: packageName,
		Image:   fmt.Sprintf("%s/%s", registryHost, spec.RepoPath),
		Properties: []property.Property{
			*pkgProp,
		},
	}, nil
}

func bundleName(packageName, version string) string {
	return packageName + "." + normalizeBundleVersion(version)
}

func normalizeBundleVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func repoPath(packageName, version string) string {
	return fmt.Sprintf("%s/bundle:%s", packageName, sanitizeTag(version))
}

func sanitizeTag(version string) string {
	return strings.ReplaceAll(version, "/", "_")
}

func mockCommit(packageName, version string) string {
	h := fnv32(packageName + "/" + version)
	return fmt.Sprintf("%040x", h)
}

func mockCommitURL(packageName, commit string) string {
	repo := inferredRepo(packageName)
	return fmt.Sprintf("https://github.com/%s/commit/%s", repo, commit)
}

func inferredRepo(packageName string) string {
	switch packageName {
	case "amq-streams-proxy":
		return "kroxylicious/kroxylicious"
	case "odf-csi-addons-operator":
		return "csi-addons/kubernetes"
	case "ansible-automation-platform-operator", "ansible-cloud-addons-operator":
		return "ansible/ansible-rulebook"
	case "advanced-cluster-management":
		return "stolostron/acm-operator-bundle"
	case "multicluster-engine":
		return "stolostron/mce-operator-bundle"
	default:
		return "example/" + packageName
	}
}

func fnv32(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
