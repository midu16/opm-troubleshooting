// Package workflow implements the catalog-bundle-inspect pipeline in pure Go:
// catalog index render (operator-registry), channel-head resolution (declcfg),
// and bundle image inspection (go-containerregistry) — no opm, jq, or skopeo.
package workflow

import (
	"context"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
)

// Result is the resolved channel-head bundle and its inspected metadata.
type Result struct {
	BundleName string
	ImageRef   string
	Info       *imageinspect.BundleInfo
}

// InspectChannelHead renders a catalog index image, resolves the channel head,
// and inspects the bundle image labels.
func InspectChannelHead(ctx context.Context, catalogRef, packageName, channelName string) (*Result, error) {
	cfg, err := catalog.RenderCatalog(ctx, catalogRef)
	if err != nil {
		return nil, err
	}
	return InspectChannelHeadFromConfig(ctx, cfg, packageName, channelName)
}

// InspectChannelHeadFromConfig resolves the channel head and inspects the bundle image.
func InspectChannelHeadFromConfig(ctx context.Context, cfg *declcfg.DeclarativeConfig, packageName, channelName string) (*Result, error) {
	return inspectBundleFromConfig(ctx, cfg, packageName, channelName, "")
}

// InspectBundleFromConfig resolves a bundle on a channel (channel head or --version) and inspects it.
func InspectBundleFromConfig(ctx context.Context, cfg *declcfg.DeclarativeConfig, packageName, channelName, version string) (*Result, error) {
	return inspectBundleFromConfig(ctx, cfg, packageName, channelName, version)
}

func inspectBundleFromConfig(ctx context.Context, cfg *declcfg.DeclarativeConfig, packageName, channelName, version string) (*Result, error) {
	var bundleName, imageRef string
	var err error
	if version == "" {
		bundleName, imageRef, err = catalog.ResolveChannelHead(cfg, packageName, channelName)
	} else {
		bundleName, imageRef, err = catalog.ResolveChannelVersion(cfg, packageName, channelName, version)
	}
	if err != nil {
		return nil, err
	}

	info, err := imageinspect.InspectBundle(ctx, imageRef)
	if err != nil {
		return nil, err
	}
	if info.Bundle == "" {
		info.Bundle = bundleName
	}

	return &Result{
		BundleName: bundleName,
		ImageRef:   imageRef,
		Info:       info,
	}, nil
}

// ResolveDefaultChannel returns the package defaultChannel from rendered catalog metadata.
func ResolveDefaultChannel(cfg *declcfg.DeclarativeConfig, packageName string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("default channel not found for package=%s", packageName)
	}
	for i := range cfg.Packages {
		p := &cfg.Packages[i]
		if p.Name == packageName && p.DefaultChannel != "" {
			return p.DefaultChannel, nil
		}
	}
	return "", fmt.Errorf("default channel not found for package=%s", packageName)
}
