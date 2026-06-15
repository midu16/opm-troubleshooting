package catalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

// LoadDeclarativeConfigFromNDJSON reads newline-delimited FBC JSON objects into a DeclarativeConfig.
func LoadDeclarativeConfigFromNDJSON(path string) (*declcfg.DeclarativeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &declcfg.DeclarativeConfig{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := appendNDJSONObject(cfg, line); err != nil {
			return nil, fmt.Errorf("parse line: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func appendNDJSONObject(cfg *declcfg.DeclarativeConfig, line string) error {
	var meta struct {
		Schema  string `json:"schema"`
		Name    string `json:"name"`
		Package string `json:"package"`
	}
	if err := json.Unmarshal([]byte(line), &meta); err != nil {
		return err
	}

	switch meta.Schema {
	case declcfg.SchemaPackage:
		var pkg declcfg.Package
		if err := json.Unmarshal([]byte(line), &pkg); err != nil {
			return err
		}
		cfg.Packages = append(cfg.Packages, pkg)
	case declcfg.SchemaChannel:
		var ch declcfg.Channel
		if err := json.Unmarshal([]byte(line), &ch); err != nil {
			return err
		}
		cfg.Channels = append(cfg.Channels, ch)
	case declcfg.SchemaBundle:
		var b declcfg.Bundle
		if err := json.Unmarshal([]byte(line), &b); err != nil {
			return err
		}
		cfg.Bundles = append(cfg.Bundles, b)
	default:
		var m declcfg.Meta
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return err
		}
		cfg.Others = append(cfg.Others, m)
	}
	return nil
}
