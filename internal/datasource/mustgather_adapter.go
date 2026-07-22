package datasource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/midu16/opm-troubleshooting/internal/mustgather"
)

// MustGatherSource implements ClusterDataSource by reading a must-gather dump.
type MustGatherSource struct {
	root string
}

// NewMustGatherSource creates a ClusterDataSource backed by a must-gather directory.
func NewMustGatherSource(path string) *MustGatherSource {
	return &MustGatherSource{root: path}
}

func (s *MustGatherSource) SourceType() string { return "must-gather" }

func (s *MustGatherSource) GetSubscriptions(ns string) ([]OperatorState, error) {
	ctx := context.Background()
	mgResult, err := mustgather.ParseMustGather(ctx, s.root)
	if err != nil {
		return nil, err
	}
	operators := make([]OperatorState, 0)
	for _, op := range mgResult.Operators {
		if ns != "" && op.Namespace != ns {
			continue
		}
		operators = append(operators, OperatorFromMustGather(op))
	}
	return operators, nil
}

func (s *MustGatherSource) GetCSVs(ns string) ([]CSVState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "operators.coreos.com", "clusterserviceversions", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	csvs := make([]CSVState, 0, len(matches))
	for _, path := range matches {
		csv, err := s.parseCSVState(path)
		if err != nil {
			continue
		}
		csvs = append(csvs, csv)
	}
	return csvs, nil
}

func (s *MustGatherSource) parseCSVState(path string) (CSVState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CSVState{}, err
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return CSVState{}, err
	}
	metadata := yamlMap(doc, "metadata")
	status := yamlMap(doc, "status")
	spec := yamlMap(doc, "spec")
	return CSVState{
		Name:      yamlStr(metadata, "name"),
		Namespace: yamlStr(metadata, "namespace"),
		Phase:     yamlStr(status, "phase"),
		Reason:    yamlStr(status, "reason"),
		Message:   yamlStr(status, "message"),
		Version:   yamlStr(spec, "version"),
	}, nil
}

func (s *MustGatherSource) GetInstallPlans(ns string) ([]InstallPlanState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "operators.coreos.com", "installplans", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	plans := make([]InstallPlanState, 0, len(matches))
	for _, path := range matches {
		ctx := context.Background()
		plan, err := mustgather.ParseInstallPlan(ctx, path)
		if err != nil {
			continue
		}
		ds := InstallPlanState{
			Name:      plan.Metadata.Name,
			Namespace: plan.Metadata.Namespace,
			Phase:     plan.Status.Phase,
			Approved:  plan.Spec.Approved,
			Failed:    plan.IsFailed(),
		}
		for _, step := range plan.Status.Plan {
			ds.Steps = append(ds.Steps, InstallPlanStep{
				Group:     step.Resource.Group,
				Kind:      step.Resource.Kind,
				Name:      step.Resource.Name,
				Version:   step.Resource.Version,
				Status:    step.Status,
				Resolving: step.Resolving,
			})
		}
		plans = append(plans, ds)
	}
	return plans, nil
}

func (s *MustGatherSource) GetCatalogSources(ns string) ([]CatalogSourceState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "operators.coreos.com", "catalogsources", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	sources := make([]CatalogSourceState, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")
		status := yamlMap(doc, "status")
		cs := CatalogSourceState{
			Name:      yamlStr(metadata, "name"),
			Namespace: yamlStr(metadata, "namespace"),
			Image:     yamlStr(spec, "image"),
			Status:    yamlStr(status, "connectionState.lastObservedState"),
		}
		sources = append(sources, cs)
	}
	return sources, nil
}

func (s *MustGatherSource) GetOperatorGroups(ns string) ([]OperatorGroupState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "operators.coreos.com", "operatorgroups", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	groups := make([]OperatorGroupState, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")
		og := OperatorGroupState{
			Name:      yamlStr(metadata, "name"),
			Namespace: yamlStr(metadata, "namespace"),
		}
		if targets := yamlSlice(spec, "targetNamespaces"); targets != nil {
			for _, t := range targets {
				if s, ok := t.(string); ok {
					og.TargetNamespaces = append(og.TargetNamespaces, s)
				}
			}
		}
		groups = append(groups, og)
	}
	return groups, nil
}

func (s *MustGatherSource) GetDeployments(ns string) ([]DeploymentState, error) {
	wl, err := mustgather.ParseWorkloads(s.root, ns)
	if err != nil {
		return nil, err
	}
	deployments := make([]DeploymentState, 0, len(wl.Deployments))
	for _, d := range wl.Deployments {
		deployments = append(deployments, DeploymentFromMustGather(d))
	}
	return deployments, nil
}

func (s *MustGatherSource) GetPods(ns string) ([]PodState, error) {
	wl, err := mustgather.ParseWorkloads(s.root, ns)
	if err != nil {
		return nil, err
	}
	pods := make([]PodState, 0, len(wl.Pods))
	for _, p := range wl.Pods {
		pods = append(pods, PodFromMustGather(p))
	}
	return pods, nil
}

func (s *MustGatherSource) GetEvents(ns string) ([]EventState, error) {
	wl, err := mustgather.ParseWorkloads(s.root, ns)
	if err != nil {
		return nil, err
	}
	events := make([]EventState, 0, len(wl.Events))
	for _, e := range wl.Events {
		events = append(events, EventFromMustGather(e))
	}
	return events, nil
}

func (s *MustGatherSource) GetNodes() ([]NodeState, error) {
	resources, err := s.findClusterScopedResources("nodes")
	if err != nil {
		return nil, err
	}
	nodes := make([]NodeState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		status := yamlMap(doc, "status")
		spec := yamlMap(doc, "spec")
		nodeInfo := yamlMap(status, "nodeInfo")
		capacity := yamlMap(status, "capacity")
		allocatable := yamlMap(status, "allocatable")

		n := NodeState{
			Name:             yamlStr(metadata, "name"),
			Unschedulable:    yamlBool(spec, "unschedulable"),
			KubeletVersion:   yamlStr(nodeInfo, "kubeletVersion"),
			OSImage:          yamlStr(nodeInfo, "osImage"),
			ContainerRuntime: yamlStr(nodeInfo, "containerRuntimeVersion"),
			Capacity: ResourceList{
				CPU:    yamlStr(capacity, "cpu"),
				Memory: yamlStr(capacity, "memory"),
				Pods:   yamlStr(capacity, "pods"),
			},
			Allocatable: ResourceList{
				CPU:    yamlStr(allocatable, "cpu"),
				Memory: yamlStr(allocatable, "memory"),
				Pods:   yamlStr(allocatable, "pods"),
			},
		}

		for _, item := range yamlSlice(status, "conditions") {
			cMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cond := Condition{
				Type:    yamlStr(cMap, "type"),
				Status:  yamlStr(cMap, "status"),
				Reason:  yamlStr(cMap, "reason"),
				Message: yamlStr(cMap, "message"),
			}
			n.Conditions = append(n.Conditions, cond)
			if cond.Type == "Ready" && cond.Status == "True" {
				n.Ready = true
			}
		}

		labels := yamlMap(metadata, "labels")
		for key := range labels {
			if strings.HasPrefix(key, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
				n.Roles = append(n.Roles, role)
			}
		}

		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (s *MustGatherSource) GetNamespaces() ([]string, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", "*")
	matches, _ := filepath.Glob(pattern)
	seen := make(map[string]bool)
	namespaces := make([]string, 0)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		ns := filepath.Base(m)
		if !seen[ns] {
			seen[ns] = true
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces, nil
}

func (s *MustGatherSource) GetClusterOperators() ([]ClusterOperatorState, error) {
	resources, err := s.findClusterScopedResources("clusteroperators")
	if err != nil {
		return nil, err
	}
	cos := make([]ClusterOperatorState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		status := yamlMap(doc, "status")

		co := ClusterOperatorState{
			Name: yamlStr(metadata, "name"),
		}

		for _, v := range yamlSlice(status, "versions") {
			if vMap, ok := v.(map[string]interface{}); ok {
				if yamlStr(vMap, "name") == "operator" {
					co.Version = yamlStr(vMap, "version")
				}
			}
		}

		for _, item := range yamlSlice(status, "conditions") {
			cMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cond := Condition{
				Type:    yamlStr(cMap, "type"),
				Status:  yamlStr(cMap, "status"),
				Reason:  yamlStr(cMap, "reason"),
				Message: yamlStr(cMap, "message"),
			}
			co.Conditions = append(co.Conditions, cond)
			switch cond.Type {
			case "Available":
				co.Available = cond.Status == "True"
			case "Progressing":
				co.Progressing = cond.Status == "True"
			case "Degraded":
				co.Degraded = cond.Status == "True"
			}
		}

		cos = append(cos, co)
	}
	return cos, nil
}

func (s *MustGatherSource) GetClusterVersion() (*ClusterVersionState, error) {
	resources, err := s.findClusterScopedResources("clusterversions")
	if err != nil || len(resources) == 0 {
		return nil, fmt.Errorf("no ClusterVersion found in must-gather")
	}
	doc := resources[0]
	spec := yamlMap(doc, "spec")
	status := yamlMap(doc, "status")

	cv := &ClusterVersionState{
		Channel:   yamlStr(spec, "channel"),
		ClusterID: yamlStr(spec, "clusterID"),
	}

	// Extract current version from history
	for _, h := range yamlSlice(status, "history") {
		if hMap, ok := h.(map[string]interface{}); ok {
			if yamlStr(hMap, "state") == "Completed" && cv.Version == "" {
				cv.Version = yamlStr(hMap, "version")
			}
			cv.History = append(cv.History, UpdateHistory{
				Version: yamlStr(hMap, "version"),
				State:   yamlStr(hMap, "state"),
			})
		}
	}

	for _, item := range yamlSlice(status, "conditions") {
		cMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cond := Condition{
			Type:    yamlStr(cMap, "type"),
			Status:  yamlStr(cMap, "status"),
			Reason:  yamlStr(cMap, "reason"),
			Message: yamlStr(cMap, "message"),
		}
		cv.Conditions = append(cv.Conditions, cond)
		switch cond.Type {
		case "Available":
			cv.Available = cond.Status == "True"
		case "Progressing":
			cv.Progressing = cond.Status == "True"
		case "Failing":
			cv.Failing = cond.Status == "True"
		}
	}

	return cv, nil
}

func (s *MustGatherSource) GetMachineConfigPools() ([]MCPState, error) {
	resources, err := s.findClusterScopedResources("machineconfigpools")
	if err != nil {
		return nil, err
	}
	mcps := make([]MCPState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		status := yamlMap(doc, "status")
		spec := yamlMap(doc, "spec")

		mcp := MCPState{
			Name:                 yamlStr(metadata, "name"),
			MachineCount:         int32(yamlInt(status, "machineCount")),         //nolint:gosec // values are small cluster counts
			ReadyMachineCount:    int32(yamlInt(status, "readyMachineCount")),    //nolint:gosec // values are small cluster counts
			UpdatedMachineCount:  int32(yamlInt(status, "updatedMachineCount")),  //nolint:gosec // values are small cluster counts
			DegradedMachineCount: int32(yamlInt(status, "degradedMachineCount")), //nolint:gosec // values are small cluster counts
			Paused:               yamlBool(spec, "paused"),
		}

		for _, item := range yamlSlice(status, "conditions") {
			cMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cond := Condition{
				Type:    yamlStr(cMap, "type"),
				Status:  yamlStr(cMap, "status"),
				Reason:  yamlStr(cMap, "reason"),
				Message: yamlStr(cMap, "message"),
			}
			mcp.Conditions = append(mcp.Conditions, cond)
			switch cond.Type {
			case "Updating":
				mcp.Updating = cond.Status == "True"
			case "Degraded":
				mcp.Degraded = cond.Status == "True"
			}
		}

		mcps = append(mcps, mcp)
	}
	return mcps, nil
}

func (s *MustGatherSource) GetNetworkConfig() (*NetworkConfigState, error) {
	resources, err := s.findClusterScopedResources("networks")
	if err != nil || len(resources) == 0 {
		return nil, fmt.Errorf("no network config found in must-gather")
	}
	doc := resources[0]
	spec := yamlMap(doc, "spec")
	status := yamlMap(doc, "status")

	nc := &NetworkConfigState{
		NetworkType: yamlStr(status, "networkType"),
	}
	if nc.NetworkType == "" {
		nc.NetworkType = yamlStr(spec, "networkType")
	}

	for _, cn := range yamlSlice(spec, "clusterNetwork") {
		if cnMap, ok := cn.(map[string]interface{}); ok {
			nc.ClusterNetwork = append(nc.ClusterNetwork, NetworkRange{
				CIDR:       yamlStr(cnMap, "cidr"),
				HostPrefix: int32(yamlInt(cnMap, "hostPrefix")), //nolint:gosec // hostPrefix is a small network value
			})
		}
	}

	for _, sn := range yamlSlice(spec, "serviceNetwork") {
		if snStr, ok := sn.(string); ok {
			nc.ServiceNetwork = append(nc.ServiceNetwork, snStr)
		}
	}

	return nc, nil
}

func (s *MustGatherSource) GetRoutes(ns string) ([]RouteState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "route.openshift.io", "routes", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	routes := make([]RouteState, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")
		status := yamlMap(doc, "status")

		r := RouteState{
			Name:      yamlStr(metadata, "name"),
			Namespace: yamlStr(metadata, "namespace"),
			Host:      yamlStr(spec, "host"),
			TLS:       len(yamlMap(spec, "tls")) > 0,
		}

		for _, ingress := range yamlSlice(status, "ingress") {
			if iMap, ok := ingress.(map[string]interface{}); ok {
				for _, cond := range yamlSlice(iMap, "conditions") {
					if cMap, ok := cond.(map[string]interface{}); ok {
						if yamlStr(cMap, "type") == "Admitted" && yamlStr(cMap, "status") == "True" {
							r.Admitted = true
						}
					}
				}
			}
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func (s *MustGatherSource) GetPVs() ([]PVState, error) {
	resources, err := s.findClusterScopedResources("persistentvolumes")
	if err != nil {
		return nil, err
	}
	pvs := make([]PVState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")
		status := yamlMap(doc, "status")
		capacity := yamlMap(spec, "capacity")

		pv := PVState{
			Name:          yamlStr(metadata, "name"),
			Capacity:      yamlStr(capacity, "storage"),
			Phase:         yamlStr(status, "phase"),
			StorageClass:  yamlStr(spec, "storageClassName"),
			ReclaimPolicy: yamlStr(spec, "persistentVolumeReclaimPolicy"),
		}

		claimRef := yamlMap(spec, "claimRef")
		if len(claimRef) > 0 {
			pv.ClaimRef = fmt.Sprintf("%s/%s", yamlStr(claimRef, "namespace"), yamlStr(claimRef, "name"))
		}

		pvs = append(pvs, pv)
	}
	return pvs, nil
}

func (s *MustGatherSource) GetPVCs(ns string) ([]PVCState, error) {
	pattern := filepath.Join(s.root, "*", "namespaces", ns, "core", "persistentvolumeclaims", "*.yaml")
	matches, _ := filepath.Glob(pattern)
	pvcs := make([]PVCState, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")
		status := yamlMap(doc, "status")
		statusCapacity := yamlMap(status, "capacity")

		pvc := PVCState{
			Name:         yamlStr(metadata, "name"),
			Namespace:    yamlStr(metadata, "namespace"),
			Phase:        yamlStr(status, "phase"),
			VolumeName:   yamlStr(spec, "volumeName"),
			StorageClass: yamlStr(spec, "storageClassName"),
			Capacity:     yamlStr(statusCapacity, "storage"),
		}
		pvcs = append(pvcs, pvc)
	}
	return pvcs, nil
}

func (s *MustGatherSource) GetStorageClasses() ([]StorageClassState, error) {
	resources, err := s.findClusterScopedResources("storageclasses")
	if err != nil {
		return nil, err
	}
	scs := make([]StorageClassState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		annotations := yamlMap(metadata, "annotations")

		sc := StorageClassState{
			Name:        yamlStr(metadata, "name"),
			Provisioner: yamlStr(doc, "provisioner"),
			IsDefault:   yamlStr(annotations, "storageclass.kubernetes.io/is-default-class") == "true",
			Parameters:  make(map[string]string),
		}

		if params := yamlMap(doc, "parameters"); len(params) > 0 {
			for k, v := range params {
				if vStr, ok := v.(string); ok {
					sc.Parameters[k] = vStr
				}
			}
		}

		scs = append(scs, sc)
	}
	return scs, nil
}

func (s *MustGatherSource) GetCertificates() ([]CertificateState, error) {
	// Must-gather doesn't directly expose parsed certificates
	return []CertificateState{}, nil
}

func (s *MustGatherSource) GetIDMS() ([]IDMSState, error) {
	resources, err := mustgather.ParseClusterResources(s.root, "imagedigestmirrorsets")
	if err != nil || len(resources) == 0 {
		resources, _ = mustgather.ParseClusterResources(s.root, "imagedigestmirrorset")
	}

	idmsList := make([]IDMSState, 0, len(resources))
	for _, doc := range resources {
		metadata := yamlMap(doc, "metadata")
		spec := yamlMap(doc, "spec")

		idms := IDMSState{
			Name: yamlStr(metadata, "name"),
		}

		for _, mirror := range yamlSlice(spec, "imageDigestMirrors") {
			if mMap, ok := mirror.(map[string]interface{}); ok {
				entry := MirrorEntry{
					Source: yamlStr(mMap, "source"),
				}
				for _, m := range yamlSlice(mMap, "mirrors") {
					if mStr, ok := m.(string); ok {
						entry.Mirrors = append(entry.Mirrors, mStr)
					}
				}
				idms.Mirrors = append(idms.Mirrors, entry)
			}
		}

		idmsList = append(idmsList, idms)
	}
	return idmsList, nil
}

func (s *MustGatherSource) GetEtcdMembers() ([]EtcdMemberState, error) {
	// etcd member details are not directly available from must-gather filesystem
	// We can infer health from the etcd cluster operator status
	return []EtcdMemberState{}, nil
}

func (s *MustGatherSource) GetAPIServerStatus() (*APIServerState, error) {
	cos, err := s.GetClusterOperators()
	if err != nil {
		return nil, err
	}
	for _, co := range cos {
		if co.Name == "kube-apiserver" || co.Name == "openshift-apiserver" {
			return &APIServerState{
				Available:  co.Available,
				Conditions: co.Conditions,
			}, nil
		}
	}
	return &APIServerState{Available: true}, nil
}

// findClusterScopedResources finds YAML files for cluster-scoped resources.
//
//nolint:unparam // error return kept for interface consistency
func (s *MustGatherSource) findClusterScopedResources(kind string) ([]map[string]interface{}, error) {
	// Try multiple patterns: direct files, list files, nested API group dirs
	patterns := []string{
		filepath.Join(s.root, "*", "cluster-scoped-resources", kind, "*.yaml"),
		filepath.Join(s.root, "*", "cluster-scoped-resources", "*", kind, "*.yaml"),
		filepath.Join(s.root, "*", "cluster-scoped-resources", "*", "*", kind, "*.yaml"),
	}

	resources := make([]map[string]interface{}, 0)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := strings.TrimPrefix(string(data), "---\n")
			var doc map[string]interface{}
			if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
				continue
			}

			// Handle list resources
			if items := yamlSlice(doc, "items"); items != nil {
				for _, item := range items {
					if iMap, ok := item.(map[string]interface{}); ok {
						resources = append(resources, iMap)
					}
				}
			} else {
				resources = append(resources, doc)
			}
		}
	}
	return resources, nil
}

// YAML helper functions (scoped to datasource package to avoid conflicts with mustgather package helpers).
func yamlMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return make(map[string]interface{})
}

func yamlStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func yamlSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key].([]interface{}); ok {
		return v
	}
	return nil
}

func yamlInt(m map[string]interface{}, key string) int {
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

func yamlBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
