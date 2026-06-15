package catalog

import (
	"context"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/image/containersimageregistry"
)

// RenderCatalog pulls and renders an OLM catalog index image into declarative config.
// This is the native Go equivalent of: opm render <index-image>
func RenderCatalog(ctx context.Context, catalogRef string) (*declcfg.DeclarativeConfig, error) {
	reg, err := containersimageregistry.NewDefault()
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}
	defer func() {
		_ = reg.Destroy()
	}()

	render := action.Render{
		Refs:     []string{catalogRef},
		Registry: reg,
	}

	cfg, err := render.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("render catalog %q: %w", catalogRef, err)
	}
	return cfg, nil
}
