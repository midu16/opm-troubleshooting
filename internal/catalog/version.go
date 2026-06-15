package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// ResolveChannelVersion finds the bundle on a channel whose package version matches versionQuery.
// versionQuery may be a full bundle version (2.11.2-509) or a prefix (v2.11.2, 2.11.1).
func ResolveChannelVersion(cfg *declcfg.DeclarativeConfig, packageName, channelName, versionQuery string) (bundleName, imageRef string, err error) {
	if cfg == nil {
		return "", "", fmt.Errorf("no bundle image found for package=%s channel=%s version=%s", packageName, channelName, versionQuery)
	}

	channel, err := channelForPackage(cfg, packageName, channelName)
	if err != nil {
		return "", "", err
	}

	var (
		matchedName string
		matchCount  int
	)
	for _, entry := range channel.Entries {
		if entry.Name == "" {
			continue
		}
		b, err := findBundleRecord(cfg, packageName, entry.Name)
		if err != nil {
			continue
		}
		ver := bundlePackageVersion(b)
		if !versionMatches(versionQuery, ver) {
			continue
		}
		matchedName = entry.Name
		matchCount++
	}

	if matchCount == 0 {
		return "", "", versionNotFoundError(cfg, packageName, channelName, versionQuery)
	}

	b, err := findBundleRecord(cfg, packageName, matchedName)
	if err != nil {
		return matchedName, "", err
	}
	return matchedName, b.Image, nil
}

// VersionsInChannel returns distinct bundle versions listed on a package channel.
func VersionsInChannel(cfg *declcfg.DeclarativeConfig, packageName, channelName string) []string {
	channel, err := channelForPackage(cfg, packageName, channelName)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var versions []string
	for _, entry := range channel.Entries {
		if entry.Name == "" {
			continue
		}
		b, err := findBundleRecord(cfg, packageName, entry.Name)
		if err != nil {
			continue
		}
		ver := bundlePackageVersion(b)
		if ver == "" {
			continue
		}
		if _, ok := seen[ver]; ok {
			continue
		}
		seen[ver] = struct{}{}
		versions = append(versions, ver)
	}
	sort.Strings(versions)
	return versions
}

func channelForPackage(cfg *declcfg.DeclarativeConfig, packageName, channelName string) (*declcfg.Channel, error) {
	for i := range cfg.Channels {
		ch := &cfg.Channels[i]
		if ch.Package == packageName && ch.Name == channelName {
			if len(ch.Entries) == 0 {
				return nil, fmt.Errorf("channel %q for package %q has no entries", channelName, packageName)
			}
			return ch, nil
		}
	}
	return nil, channelNotFoundError(cfg, packageName, channelName)
}

func findBundleRecord(cfg *declcfg.DeclarativeConfig, packageName, bundleName string) (*declcfg.Bundle, error) {
	for i := range cfg.Bundles {
		b := &cfg.Bundles[i]
		if b.Package == packageName && b.Name == bundleName {
			if b.Image == "" {
				return nil, fmt.Errorf("bundle %q has no image", bundleName)
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("bundle %q not found for package %q", bundleName, packageName)
}

func bundlePackageVersion(b *declcfg.Bundle) string {
	if b == nil {
		return ""
	}
	props, err := property.Parse(b.Properties)
	if err == nil && len(props.Packages) > 0 && props.Packages[0].Version != "" {
		return props.Packages[0].Version
	}
	return versionFromBundleName(b.Package, b.Name)
}

func versionFromBundleName(packageName, bundleName string) string {
	prefix := packageName + "."
	if strings.HasPrefix(bundleName, prefix) {
		return bundleName[len(prefix):]
	}
	return ""
}

func normalizeVersionQuery(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	return v
}

func versionMatches(query, bundleVersion string) bool {
	q := normalizeVersionQuery(query)
	if q == "" {
		return false
	}
	bv := normalizeVersionQuery(bundleVersion)
	if bv == q {
		return true
	}
	// Prefix match: 2.11.2 matches 2.11.2-509
	if strings.HasPrefix(bv, q+"-") {
		return true
	}
	if strings.HasPrefix(bv, q+".") {
		return true
	}
	return false
}

func versionNotFoundError(cfg *declcfg.DeclarativeConfig, packageName, channelName, versionQuery string) error {
	versions := VersionsInChannel(cfg, packageName, channelName)
	msg := fmt.Sprintf("version %q not found for package %q on channel %q", versionQuery, packageName, channelName)
	if len(versions) > 0 {
		msg += fmt.Sprintf("; versions on channel: %s", strings.Join(versions, ", "))
	}
	return fmt.Errorf("%s", msg)
}
