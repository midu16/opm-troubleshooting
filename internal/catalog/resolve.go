package catalog

import (
	"fmt"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// ResolveChannelHead returns the bundle name and image reference for the last entry
// on the given package channel (FBC channel order is authoritative).
// This replaces jq queries over opm render output to find the channel head bundle.
func ResolveChannelHead(cfg *declcfg.DeclarativeConfig, packageName, channelName string) (bundleName, imageRef string, err error) {
	if cfg == nil {
		return "", "", fmt.Errorf("no bundle image found for package=%s channel=%s", packageName, channelName)
	}

	var channel *declcfg.Channel
	for i := range cfg.Channels {
		ch := &cfg.Channels[i]
		if ch.Package == packageName && ch.Name == channelName {
			channel = ch
			break
		}
	}

	if channel == nil || len(channel.Entries) == 0 {
		return "", "", channelNotFoundError(cfg, packageName, channelName)
	}

	bundleName = channel.Entries[len(channel.Entries)-1].Name
	if bundleName == "" {
		return "", "", fmt.Errorf("no bundle image found for package=%s channel=%s", packageName, channelName)
	}

	b, err := findBundleRecord(cfg, packageName, bundleName)
	if err != nil {
		return bundleName, "", fmt.Errorf("no bundle image found for package=%s channel=%s", packageName, channelName)
	}
	return bundleName, b.Image, nil
}

// ChannelsForPackage lists channel names defined for a package in catalog order.
func ChannelsForPackage(cfg *declcfg.DeclarativeConfig, packageName string) []string {
	if cfg == nil {
		return nil
	}
	var channels []string
	for i := range cfg.Channels {
		ch := &cfg.Channels[i]
		if ch.Package == packageName {
			channels = append(channels, ch.Name)
		}
	}
	return channels
}

func defaultChannelForPackage(cfg *declcfg.DeclarativeConfig, packageName string) string {
	if cfg == nil {
		return ""
	}
	for i := range cfg.Packages {
		p := &cfg.Packages[i]
		if p.Name == packageName {
			return p.DefaultChannel
		}
	}
	return ""
}

func channelNotFoundError(cfg *declcfg.DeclarativeConfig, packageName, channelName string) error {
	channels := ChannelsForPackage(cfg, packageName)
	if len(channels) == 0 {
		return fmt.Errorf("package %q not found in catalog", packageName)
	}

	msg := fmt.Sprintf("channel %q not found for package %q", channelName, packageName)
	if def := defaultChannelForPackage(cfg, packageName); def != "" {
		msg += fmt.Sprintf(" (defaultChannel: %s)", def)
	}
	msg += fmt.Sprintf("; available channels: %s", strings.Join(channels, ", "))
	return fmt.Errorf("%s", msg)
}
