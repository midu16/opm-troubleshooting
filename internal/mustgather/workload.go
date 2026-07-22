package mustgather

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// WorkloadState captures deployment and pod health from must-gather.
type WorkloadState struct {
	Namespace   string
	Deployments []DeploymentState
	Pods        []PodState
	Events      []EventState
}

// DeploymentState represents a Deployment resource snapshot.
type DeploymentState struct {
	Name           string
	Replicas       int32
	ReadyReplicas  int32
	Available      bool
	Progressing    bool
	ProgressingMsg string
	UnavailableMsg string
}

// PodState represents a Pod resource snapshot.
type PodState struct {
	Name             string
	Phase            string
	Ready            bool
	RestartCount     int32
	WaitingReason    string
	WaitingMessage   string
	TerminatedReason string
}

// EventState represents a Warning event.
type EventState struct {
	Type    string
	Reason  string
	Message string
	Object  string
}

// ParseWorkloads extracts deployment, pod, and event state for a namespace.
func ParseWorkloads(mustGatherRoot, namespace string) (*WorkloadState, error) {
	state := &WorkloadState{
		Namespace:   namespace,
		Deployments: make([]DeploymentState, 0),
		Pods:        make([]PodState, 0),
		Events:      make([]EventState, 0),
	}

	deployPattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "apps", "deployments", "*.yaml")
	deployFiles, _ := filepath.Glob(deployPattern)
	for _, f := range deployFiles {
		dep, err := parseDeploymentFile(f)
		if err == nil {
			state.Deployments = append(state.Deployments, dep)
		}
	}

	podPattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "core", "pods", "*.yaml")
	podFiles, _ := filepath.Glob(podPattern)
	// Newer must-gather format: namespaces/{ns}/pods/{pod}/{pod}.yaml
	if len(podFiles) == 0 {
		podDirPattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "pods", "*", "*.yaml")
		podFiles, _ = filepath.Glob(podDirPattern)
	}
	for _, f := range podFiles {
		pod, err := parsePodFile(f)
		if err == nil {
			state.Pods = append(state.Pods, pod)
		}
	}

	eventPattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "events", "*.yaml")
	eventFiles, _ := filepath.Glob(eventPattern)
	for _, f := range eventFiles {
		events, err := parseEventsFile(f)
		if err == nil {
			state.Events = append(state.Events, events...)
		}
	}

	// Also try events.yaml list format
	eventsListPath := findEventsList(mustGatherRoot, namespace)
	if eventsListPath != "" {
		events, err := parseEventsFile(eventsListPath)
		if err == nil {
			state.Events = append(state.Events, events...)
		}
	}

	return state, nil
}

func findEventsList(mustGatherRoot, namespace string) string {
	pattern := filepath.Join(mustGatherRoot, "*", "namespaces", namespace, "events.yaml")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func parseDeploymentFile(path string) (DeploymentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DeploymentState{}, err
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return DeploymentState{}, err
	}

	metadata := getMap(doc, "metadata")
	status := getMap(doc, "status")

	dep := DeploymentState{
		Name:          getString(metadata, "name"),
		Replicas:      int32(getInt(status, "replicas")),      //nolint:gosec // values are small replica counts
		ReadyReplicas: int32(getInt(status, "readyReplicas")), //nolint:gosec // values are small replica counts
	}

	for _, item := range getSlice(status, "conditions") {
		cMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		condType := getString(cMap, "type")
		condStatus := getString(cMap, "status")
		msg := getString(cMap, "message")

		switch condType {
		case "Available":
			dep.Available = condStatus == "True"
			if condStatus == "False" {
				dep.UnavailableMsg = msg
			}
		case "Progressing":
			dep.Progressing = condStatus == "True"
			if condStatus == "False" {
				dep.ProgressingMsg = msg
			}
		}
	}

	return dep, nil
}

func parsePodFile(path string) (PodState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PodState{}, err
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return PodState{}, err
	}

	metadata := getMap(doc, "metadata")
	status := getMap(doc, "status")

	pod := PodState{
		Name:  getString(metadata, "name"),
		Phase: getString(status, "phase"),
	}

	for _, item := range getSlice(status, "conditions") {
		cMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(cMap, "type") == "Ready" && getString(cMap, "status") == "True" {
			pod.Ready = true
		}
	}

	for _, item := range getSlice(status, "containerStatuses") {
		cMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		pod.RestartCount += int32(getInt(cMap, "restartCount")) //nolint:gosec // values are small replica counts

		state := getMap(cMap, "state")
		if waiting := getMap(state, "waiting"); len(waiting) > 0 {
			pod.WaitingReason = getString(waiting, "reason")
			pod.WaitingMessage = getString(waiting, "message")
		}
		if terminated := getMap(state, "terminated"); len(terminated) > 0 {
			pod.TerminatedReason = getString(terminated, "reason")
		}
	}

	return pod, nil
}

func parseEventsFile(path string) ([]EventState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlContent := strings.TrimPrefix(string(data), "---\n")
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil, err
	}

	items := getSlice(doc, "items")
	if len(items) == 0 {
		// Single event
		if getString(doc, "kind") == "Event" {
			return []EventState{eventFromMap(doc)}, nil
		}
		return nil, nil
	}

	events := make([]EventState, 0, len(items))
	for _, item := range items {
		if eMap, ok := item.(map[string]interface{}); ok {
			ev := eventFromMap(eMap)
			if ev.Type == "Warning" {
				events = append(events, ev)
			}
		}
	}
	return events, nil
}

func eventFromMap(m map[string]interface{}) EventState {
	involved := getMap(m, "involvedObject")
	obj := fmt.Sprintf("%s/%s", getString(involved, "kind"), getString(involved, "name"))
	return EventState{
		Type:    getString(m, "type"),
		Reason:  getString(m, "reason"),
		Message: getString(m, "message"),
		Object:  obj,
	}
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// ParseClusterResources scans cluster-scoped resources like IDMS.
func ParseClusterResources(mustGatherRoot, kind string) ([]map[string]interface{}, error) {
	pattern := filepath.Join(mustGatherRoot, "*", "cluster-scoped-resources", "*", "*", kind, "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	resources := make([]map[string]interface{}, 0)
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		resources = append(resources, doc)
	}
	return resources, nil
}
