package mustgather

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// InstallPlanStep represents a single resource in an InstallPlan.
type InstallPlanStep struct {
	Resolving string                 `yaml:"resolving"`
	Resource  InstallPlanStepResource `yaml:"resource"`
	Status    string                 `yaml:"status"` // Unknown, Present, Created, NotPresent
}

// InstallPlanStepResource describes the resource being installed.
type InstallPlanStepResource struct {
	Group   string `yaml:"group"`
	Kind    string `yaml:"kind"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// InstallPlanStatus represents the status section of an InstallPlan.
type InstallPlanStatus struct {
	Phase      string                   `yaml:"phase"`      // Planning, RequiresApproval, Installing, Complete, Failed
	Conditions []InstallPlanCondition   `yaml:"conditions"`
	Plan       []InstallPlanStep        `yaml:"plan"`
}

// InstallPlanCondition represents a condition in the InstallPlan status.
type InstallPlanCondition struct {
	Type    string `yaml:"type"`    // Installed
	Status  string `yaml:"status"`  // True, False
	Message string `yaml:"message"`
	Reason  string `yaml:"reason"`
}

// InstallPlanSpec represents the spec section of an InstallPlan.
type InstallPlanSpec struct {
	Approved bool `yaml:"approved"`
}

// InstallPlanMetadata represents the metadata section of an InstallPlan.
type InstallPlanMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// InstallPlan represents an OLM InstallPlan resource.
type InstallPlan struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       string              `yaml:"kind"`
	Metadata   InstallPlanMetadata `yaml:"metadata"`
	Spec       InstallPlanSpec     `yaml:"spec"`
	Status     InstallPlanStatus   `yaml:"status"`
}

// InstallPlanRef represents a reference to an InstallPlan from a Subscription.
type InstallPlanRef struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// RootCauseDetail contains specific details about missing dependencies.
type RootCauseDetail struct {
	MissingCRDs       []string // CRD names that are required but not present
	MissingAPIs       []string // API versions required but not available
	UnknownResources  []string // Resources with status "Unknown"
	NotPresentResources []string // Resources with status "NotPresent"
	PodErrors         []string // Errors from operator pod logs
	RawFailureMessage string   // Raw failure message from conditions
}

// ParseInstallPlan reads and parses an InstallPlan YAML file.
func ParseInstallPlan(ctx context.Context, filePath string) (*InstallPlan, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read InstallPlan file %s: %w", filePath, err)
	}

	// Remove YAML document separator if present
	yamlContent := strings.TrimPrefix(string(data), "---\n")

	var plan InstallPlan
	if err := yaml.Unmarshal([]byte(yamlContent), &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal InstallPlan from %s: %w", filePath, err)
	}

	return &plan, nil
}

// FindInstallPlan locates the InstallPlan file for a given subscription.
func FindInstallPlan(mustGatherRoot, namespace, installPlanName string) (string, error) {
	// Pattern: {must-gather-root}/*/namespaces/{namespace}/operators.coreos.com/installplans/{name}.yaml
	pattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "operators.coreos.com", "installplans", installPlanName+".yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob pattern failed for %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no InstallPlan file found for %s in namespace %s", installPlanName, namespace)
	}

	// Return first match (should only be one)
	return matches[0], nil
}

// ExtractRootCause analyzes an InstallPlan and extracts detailed root cause information.
func ExtractRootCause(plan *InstallPlan) *RootCauseDetail {
	detail := &RootCauseDetail{
		MissingCRDs:       make([]string, 0),
		MissingAPIs:       make([]string, 0),
		UnknownResources:  make([]string, 0),
		NotPresentResources: make([]string, 0),
		PodErrors:         make([]string, 0),
	}

	// Check for failure conditions
	for _, cond := range plan.Status.Conditions {
		if cond.Type == "Installed" && cond.Status == "False" {
			detail.RawFailureMessage = cond.Message
			if cond.Reason != "" {
				detail.RawFailureMessage = cond.Reason + ": " + cond.Message
			}
		}
	}

	// Analyze plan steps for missing resources
	for _, step := range plan.Status.Plan {
		switch step.Status {
		case "Unknown":
			// Resource cannot be installed - likely missing dependency
			resName := fmt.Sprintf("%s/%s (%s)", step.Resource.Group, step.Resource.Kind, step.Resource.Name)
			detail.UnknownResources = append(detail.UnknownResources, resName)

			// If it's a CRD, add to MissingCRDs
			if step.Resource.Kind == "CustomResourceDefinition" {
				detail.MissingCRDs = append(detail.MissingCRDs, step.Resource.Name)
			}

		case "NotPresent":
			// Resource explicitly not present
			resName := fmt.Sprintf("%s/%s (%s)", step.Resource.Group, step.Resource.Kind, step.Resource.Name)
			detail.NotPresentResources = append(detail.NotPresentResources, resName)

			if step.Resource.Kind == "CustomResourceDefinition" {
				detail.MissingCRDs = append(detail.MissingCRDs, step.Resource.Name)
			}
		}
	}

	return detail
}

// IsFailed returns true if the InstallPlan is in a failed state.
func (p *InstallPlan) IsFailed() bool {
	// Check phase
	if p.Status.Phase == "Failed" {
		return true
	}

	// Check conditions
	for _, cond := range p.Status.Conditions {
		if cond.Type == "Installed" && cond.Status == "False" {
			return true
		}
	}

	// Check for resources that couldn't be installed
	for _, step := range p.Status.Plan {
		if step.Status == "Unknown" || step.Status == "NotPresent" {
			return true
		}
	}

	return false
}

// IsWaitingApproval returns true if the InstallPlan is waiting for manual approval.
func (p *InstallPlan) IsWaitingApproval() bool {
	return p.Status.Phase == "RequiresApproval" && !p.Spec.Approved
}

// IsComplete returns true if the InstallPlan completed successfully.
func (p *InstallPlan) IsComplete() bool {
	if p.Status.Phase != "Complete" {
		return false
	}

	// Verify Installed condition is True
	for _, cond := range p.Status.Conditions {
		if cond.Type == "Installed" && cond.Status == "True" {
			return true
		}
	}

	return false
}
