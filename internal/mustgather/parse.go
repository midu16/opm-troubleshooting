package mustgather

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// ParseMustGather scans a must-gather directory and extracts operator state.
func ParseMustGather(ctx context.Context, mustGatherPath string) (*ParseResult, error) {
	result := &ParseResult{
		Operators:   make([]OperatorState, 0),
		ParseErrors: make([]error, 0),
	}

	// Try to find cluster-scoped subscriptions.yaml first
	subsPath, err := findSubscriptionsFile(mustGatherPath)
	if err != nil {
		return nil, fmt.Errorf("find subscriptions file: %w", err)
	}

	var subscriptions []map[string]interface{}

	if subsPath != "" {
		// Old format: cluster-scoped subscriptions.yaml
		subscriptions, err = parseSubscriptionsFile(subsPath)
		if err != nil {
			return nil, fmt.Errorf("parse subscriptions: %w", err)
		}
	} else {
		// New format: namespace-scoped subscription files
		subscriptions, err = findNamespaceScopedSubscriptions(mustGatherPath)
		if err != nil {
			return nil, fmt.Errorf("find namespace-scoped subscriptions: %w", err)
		}
		if len(subscriptions) == 0 {
			return nil, fmt.Errorf("no subscriptions found in must-gather (tried both cluster-scoped and namespace-scoped formats)")
		}
	}

	// For each subscription, parse corresponding CSV and InstallPlan
	for _, sub := range subscriptions {
		op := operatorFromSubscription(sub)

		// Extract installed version from CSV metadata
		if op.InstalledCSV != "" {
			csvPath := findCSVFile(mustGatherPath, op.Namespace, op.InstalledCSV)
			if csvPath != "" {
				csv, err := parseCSVFile(csvPath)
				if err == nil {
					enrichOperatorFromCSV(&op, csv)
				} else {
					result.ParseErrors = append(result.ParseErrors, fmt.Errorf("parse CSV %s: %w", csvPath, err))
				}
			}
		}

		// Detect faults by checking InstallPlan status
		op.Faulty = isFaulty(ctx, &op, mustGatherPath)
		if op.Faulty {
			op.FailureReason = buildFailureReason(ctx, &op, mustGatherPath)
			result.FaultyCount++
		}

		result.Operators = append(result.Operators, op)
	}

	return result, nil
}

// findSubscriptionsFile searches for subscriptions in the must-gather directory.
// Supports two formats:
// 1. Cluster-scoped: cluster-scoped-resources/operators.coreos.com/subscriptions.yaml (OLM v0.19 and earlier)
// 2. Namespace-scoped: namespaces/{ns}/operators.coreos.com/subscriptions/*.yaml (OLM v0.20+)
func findSubscriptionsFile(mustGatherPath string) (string, error) {
	// Try cluster-scoped format first
	var clusterScopedPath string
	err := filepath.Walk(mustGatherPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "subscriptions.yaml" &&
			strings.Contains(path, "cluster-scoped-resources/operators.coreos.com") {
			clusterScopedPath = path
			return filepath.SkipDir // Stop searching after first match
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if clusterScopedPath != "" {
		return clusterScopedPath, nil
	}

	// No cluster-scoped subscriptions found - this is OK for newer must-gathers
	// Return empty string to signal we should use namespace-scoped parsing instead
	return "", nil
}

// findNamespaceScopedSubscriptions finds all subscription files in namespace directories.
func findNamespaceScopedSubscriptions(mustGatherPath string) ([]map[string]interface{}, error) {
	var allSubscriptions []map[string]interface{}

	// Pattern: */namespaces/*/operators.coreos.com/subscriptions/*.yaml
	pattern := filepath.Join(mustGatherPath, "*", "namespaces", "*", "operators.coreos.com", "subscriptions", "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern failed for %s: %w", pattern, err)
	}

	for _, subFile := range matches {
		subs, err := parseSubscriptionsFile(subFile)
		if err != nil {
			// Log error but continue with other files
			continue
		}
		allSubscriptions = append(allSubscriptions, subs...)
	}

	return allSubscriptions, nil
}

// parseSubscriptionsFile reads YAML list of subscriptions.
func parseSubscriptionsFile(path string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	// Handle both single Subscription and SubscriptionList
	if items, ok := doc["items"].([]interface{}); ok {
		// SubscriptionList
		return convertItems(items), nil
	}
	// Single Subscription
	return []map[string]interface{}{doc}, nil
}

// convertItems converts []interface{} to []map[string]interface{}.
func convertItems(items []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// findCSVFile looks for namespaces/{namespace}/operators.coreos.com/clusterserviceversions/{csvName}.yaml.
func findCSVFile(mustGatherPath, namespace, csvName string) string {
	// Search pattern: */namespaces/{namespace}/operators.coreos.com/clusterserviceversions/{csvName}.yaml
	pattern := filepath.Join(mustGatherPath, "*", "namespaces", namespace, "operators.coreos.com", "clusterserviceversions", csvName+".yaml")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// parseCSVFile reads a ClusterServiceVersion YAML file.
func parseCSVFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var csv map[string]interface{}
	if err := yaml.Unmarshal(data, &csv); err != nil {
		return nil, err
	}
	return csv, nil
}

// operatorFromSubscription extracts OperatorState from a Subscription YAML map.
func operatorFromSubscription(sub map[string]interface{}) OperatorState {
	spec := getMap(sub, "spec")
	status := getMap(sub, "status")
	metadata := getMap(sub, "metadata")

	// Extract InstallPlan reference
	installPlanRef := ""
	if ipRef := getMap(status, "installPlanRef"); len(ipRef) > 0 {
		installPlanRef = getString(ipRef, "name")
	}

	return OperatorState{
		PackageName:    getString(spec, "name"),
		Namespace:      getString(metadata, "namespace"),
		Channel:        getString(spec, "channel"),
		InstalledCSV:   getString(status, "installedCSV"),
		CurrentCSV:     getString(status, "currentCSV"),
		State:          getString(status, "state"),
		Conditions:     parseConditions(getSlice(status, "conditions")),
		InstallPlanRef: installPlanRef,
	}
}

// enrichOperatorFromCSV adds version and conditions from ClusterServiceVersion.
func enrichOperatorFromCSV(op *OperatorState, csv map[string]interface{}) {
	// Extract version from CSV spec.version
	spec := getMap(csv, "spec")
	if version := getString(spec, "version"); version != "" {
		op.InstalledVersion = version
	} else {
		// Fallback: extract version from CSV name (e.g., "operator.v1.2.3" -> "1.2.3")
		op.InstalledVersion = extractVersionFromCSVName(op.InstalledCSV)
	}

	// Extract CSV phase and conditions
	status := getMap(csv, "status")
	csvConditions := parseConditions(getSlice(status, "conditions"))
	op.Conditions = append(op.Conditions, csvConditions...)
}

// extractVersionFromCSVName extracts version from CSV name (e.g., "operator.v1.2.3" -> "1.2.3").
func extractVersionFromCSVName(csvName string) string {
	// CSV name format: {package}.v{version}
	if idx := strings.LastIndex(csvName, ".v"); idx != -1 {
		return csvName[idx+2:] // Skip ".v"
	}
	return csvName
}

// isFaulty determines if an operator is in a faulty state by checking the actual InstallPlan.
func isFaulty(ctx context.Context, op *OperatorState, mustGatherRoot string) bool {
	// First check subscription-level faults
	if op.State == "Failed" {
		return true
	}

	// Check conditions for catalog health issues
	for _, cond := range op.Conditions {
		if cond.Type == "CatalogSourcesUnhealthy" && cond.Status == "True" {
			return true
		}
	}

	// For UpgradePending or RequirementsNotMet, we need to check the actual InstallPlan
	// Don't trust subscription conditions alone - they can be misleading
	needsInstallPlanCheck := false
	if op.State == "UpgradePending" {
		needsInstallPlanCheck = true
	}
	for _, cond := range op.Conditions {
		if cond.Reason == "RequirementsNotMet" || cond.Reason == "InstallPlanFailed" {
			needsInstallPlanCheck = true
			break
		}
	}

	// If we need to check the InstallPlan, try to load it
	if needsInstallPlanCheck && op.InstallPlanRef != "" {
		planPath, err := FindInstallPlan(mustGatherRoot, op.Namespace, op.InstallPlanRef)
		if err == nil {
			plan, err := ParseInstallPlan(ctx, planPath)
			if err == nil {
				// Only mark as faulty if InstallPlan actually failed
				// Ignore "RequiresApproval" or "Complete" plans
				if plan.IsFailed() {
					// Extract detailed root cause
					op.RootCause = ExtractRootCause(plan)
					return true
				}
				// If plan is waiting approval or complete, this is NOT a fault
				if plan.IsWaitingApproval() || plan.IsComplete() {
					return false
				}
			}
		}
	}

	// Detect ResolutionFailed when operator has no successful install
	for _, cond := range op.Conditions {
		if cond.Type == "ResolutionFailed" && cond.Status == "True" {
			if op.InstalledCSV == "" || op.State == "" {
				if op.FailureReason == "" {
					op.FailureReason = fmt.Sprintf("ResolutionFailed: %s", cond.Message)
				}
				return true
			}
		}
	}

	return false
}

// buildFailureReason constructs a human-readable failure reason with code-level details.
func buildFailureReason(_ context.Context, op *OperatorState, _ string) string {
	reasons := []string{}

	// Check for catalog health issues first
	for _, cond := range op.Conditions {
		if cond.Status == "True" && cond.Type == "CatalogSourcesUnhealthy" {
			reasons = append(reasons, fmt.Sprintf("CatalogSourcesUnhealthy: %s", cond.Message))
		}
	}

	// If we have detailed root cause from InstallPlan analysis, use that
	if op.RootCause != nil {
		if len(op.RootCause.MissingCRDs) > 0 {
			reasons = append(reasons, fmt.Sprintf("Missing CRDs: %s", strings.Join(op.RootCause.MissingCRDs, ", ")))
		}
		if len(op.RootCause.UnknownResources) > 0 {
			reasons = append(reasons, fmt.Sprintf("Unknown resources: %s", strings.Join(op.RootCause.UnknownResources, ", ")))
		}
		if len(op.RootCause.NotPresentResources) > 0 {
			reasons = append(reasons, fmt.Sprintf("Not present: %s", strings.Join(op.RootCause.NotPresentResources, ", ")))
		}
		if op.RootCause.RawFailureMessage != "" {
			reasons = append(reasons, op.RootCause.RawFailureMessage)
		}
	} else {
		// Fallback to generic messages if no InstallPlan analysis available
		if op.State == "UpgradePending" {
			reasons = append(reasons, "Upgrade pending")
		} else if op.State == "Failed" {
			reasons = append(reasons, "Subscription failed")
		}

		for _, cond := range op.Conditions {
			if cond.Reason == "RequirementsNotMet" {
				reasons = append(reasons, fmt.Sprintf("RequirementsNotMet: %s", cond.Message))
			}
		}
	}

	if len(reasons) == 0 {
		return "Unknown failure"
	}
	return strings.Join(reasons, "; ")
}

// parseConditions parses a slice of condition maps.
func parseConditions(items []interface{}) []Condition {
	conditions := make([]Condition, 0, len(items))
	for _, item := range items {
		if cMap, ok := item.(map[string]interface{}); ok {
			conditions = append(conditions, Condition{
				Type:    getString(cMap, "type"),
				Status:  getString(cMap, "status"),
				Reason:  getString(cMap, "reason"),
				Message: getString(cMap, "message"),
			})
		}
	}
	return conditions
}

// Helper functions for safe map/slice access.

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return make(map[string]interface{})
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key].([]interface{}); ok {
		return v
	}
	return nil
}
