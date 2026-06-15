package testfixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Operator is a package and channel pair from the Red Hat v4.22 operator index list.
type Operator struct {
	Package string `json:"package"`
	Channel string `json:"channel"`
}

// OperatorsPath returns the path to the embedded operator list JSON fixture.
func OperatorsPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", "catalog", "operators.json")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "catalog", "operators.json")
}

// LoadOperators reads the operator list JSON fixture used for mock-catalog tests.
func LoadOperators() ([]Operator, error) {
	return LoadOperatorsFromPath(OperatorsPath())
}

// LoadOperatorsFromPath reads operators from JSON or whitespace-separated text.
func LoadOperatorsFromPath(path string) ([]Operator, error) {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var ops []Operator
		if err := json.Unmarshal(data, &ops); err != nil {
			return nil, fmt.Errorf("parse operators json: %w", err)
		}
		if len(ops) == 0 {
			return nil, fmt.Errorf("operators list is empty")
		}
		return ops, nil
	}
	return LoadOperatorsFromFile(path)
}

// LoadOperatorsFromFile reads package/channel pairs from a whitespace-separated text file.
func LoadOperatorsFromFile(path string) ([]Operator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ops []Operator
	for _, line := range stringLines(data) {
		fields := splitFields(line)
		if len(fields) < 2 {
			continue
		}
		ops = append(ops, Operator{Package: fields[0], Channel: fields[1]})
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operators in %s", path)
	}
	return ops, nil
}

func stringLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

func splitFields(line string) []string {
	line = trimSpace(line)
	if line == "" || line[0] == '#' {
		return nil
	}
	return splitSpace(line)
}

func trimSpace(s string) string {
	return string(trimSpaceBytes([]byte(s)))
}

func trimSpaceBytes(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}

func splitSpace(s string) []string {
	var out []string
	b := []byte(s)
	start := -1
	for i := 0; i < len(b); i++ {
		if b[i] == ' ' || b[i] == '\t' {
			if start >= 0 {
				out = append(out, string(b[start:i]))
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, string(b[start:]))
	}
	return out
}
