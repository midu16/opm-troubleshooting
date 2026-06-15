package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
)

func TestWriteOutput_human(t *testing.T) {
	var buf bytes.Buffer
	info := &imageinspect.BundleInfo{
		Package: "pkg",
		Bundle:  "quay.io/example/bundle:1.0",
		Version: "1.0.0",
		Commit:  "deadbeef",
		URL:     "https://example/commit",
	}
	if err := writeOutput(&buf, info, false); err != nil {
		t.Fatalf("writeOutput: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"package: pkg",
		"bundle:  quay.io/example/bundle:1.0",
		"version: 1.0.0",
		"commit:  deadbeef",
		"url:     https://example/commit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteOutput_json(t *testing.T) {
	var buf bytes.Buffer
	info := &imageinspect.BundleInfo{
		Package: "pkg",
		Bundle:  "quay.io/example/bundle:1.0",
		Version: "1.0.0",
	}
	if err := writeOutput(&buf, info, true); err != nil {
		t.Fatalf("writeOutput: %v", err)
	}
	if !strings.Contains(buf.String(), `"Package":"pkg"`) && !strings.Contains(buf.String(), `"Package": "pkg"`) {
		t.Errorf("expected JSON output: %s", buf.String())
	}
}

func TestParseArgs_help(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "-help"} {
		_, err := parseArgs([]string{arg})
		if !errors.Is(err, errHelp) {
			t.Errorf("parseArgs(%q): got %v, want errHelp", arg, err)
		}
	}
}

func TestParseArgs_requiredFlags(t *testing.T) {
	_, err := parseArgs([]string{})
	if err == nil {
		t.Fatal("expected error when flags missing")
	}

	_, err = parseArgs([]string{"--catalog", "img"})
	if err == nil {
		t.Fatal("expected error when package missing")
	}

	cfg, err := parseArgs([]string{"--catalog", "img", "--package", "pkg"})
	if err != nil {
		t.Fatalf("channel optional: %v", err)
	}
	if cfg.channel != "" {
		t.Errorf("channel: %q", cfg.channel)
	}
}

func TestParseArgs_valid(t *testing.T) {
	cfg, err := parseArgs([]string{
		"-c", "quay.io/index:tag",
		"-p", "my-operator",
		"--channel", "stable",
		"--json",
	})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.catalog != "quay.io/index:tag" {
		t.Errorf("catalog: %q", cfg.catalog)
	}
	if cfg.packageName != "my-operator" {
		t.Errorf("package: %q", cfg.packageName)
	}
	if cfg.channel != "stable" {
		t.Errorf("channel: %q", cfg.channel)
	}
	if !cfg.jsonOut {
		t.Error("expected json output")
	}
}

func TestExitCode(t *testing.T) {
	if ExitCode(nil) != exitSuccess {
		t.Error("nil should be success")
	}
	if ExitCode(&CLIError{Code: exitUsage, Err: sentinelErr}) != exitUsage {
		t.Error("usage code")
	}
	if ExitCode(sentinelErr) != exitOperation {
		t.Error("generic error should be operation failure")
	}
}

type sentinelError struct{}

func (sentinelError) Error() string { return "sentinel" }

var sentinelErr = sentinelError{}
