package livecluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/midu16/opm-troubleshooting/internal/datasource"
)

var (
	gvrSubscriptions = schema.GroupVersionResource{
		Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions",
	}
	gvrCSVs = schema.GroupVersionResource{
		Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions",
	}
	gvrInstallPlans = schema.GroupVersionResource{
		Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans",
	}
	gvrCatalogSources = schema.GroupVersionResource{
		Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources",
	}
	gvrOperatorGroups = schema.GroupVersionResource{
		Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups",
	}
	gvrClusterOperators = schema.GroupVersionResource{
		Group: "config.openshift.io", Version: "v1", Resource: "clusteroperators",
	}
	gvrClusterVersions = schema.GroupVersionResource{
		Group: "config.openshift.io", Version: "v1", Resource: "clusterversions",
	}
	gvrMCPs = schema.GroupVersionResource{
		Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigpools",
	}
	gvrNetworks = schema.GroupVersionResource{
		Group: "config.openshift.io", Version: "v1", Resource: "networks",
	}
	gvrRoutes = schema.GroupVersionResource{
		Group: "route.openshift.io", Version: "v1", Resource: "routes",
	}
	gvrIDMS = schema.GroupVersionResource{
		Group: "config.openshift.io", Version: "v1", Resource: "imagedigestmirrorsets",
	}
)

type LiveClusterSource struct {
	client *Client
}

func NewLiveClusterSource(kubeconfigPath, contextName string) (*LiveClusterSource, error) {
	c, err := NewClient(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	return &LiveClusterSource{client: c}, nil
}

func (s *LiveClusterSource) SourceType() string { return "live-cluster" }

func (s *LiveClusterSource) GetSubscriptions(ns string) ([]datasource.OperatorState, error) {
	list, err := s.client.dynamicClient.Resource(gvrSubscriptions).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	operators := make([]datasource.OperatorState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")
		status := nestedMap(item.Object, "status")

		op := datasource.OperatorState{
			PackageName:      nestedStr(spec, "name"),
			Namespace:        item.GetNamespace(),
			Channel:          nestedStr(spec, "channel"),
			InstalledCSV:     nestedStr(status, "installedCSV"),
			CurrentCSV:       nestedStr(status, "currentCSV"),
			State:            nestedStr(status, "state"),
			InstallPlanRef:   nestedStr(nestedMap(status, "installPlanRef"), "name"),
			Conditions:       extractConditions(status),
		}
		operators = append(operators, op)
	}
	return operators, nil
}

func (s *LiveClusterSource) GetCSVs(ns string) ([]datasource.CSVState, error) {
	list, err := s.client.dynamicClient.Resource(gvrCSVs).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	csvs := make([]datasource.CSVState, 0, len(list.Items))
	for _, item := range list.Items {
		status := nestedMap(item.Object, "status")
		spec := nestedMap(item.Object, "spec")

		csvs = append(csvs, datasource.CSVState{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Phase:      nestedStr(status, "phase"),
			Reason:     nestedStr(status, "reason"),
			Message:    nestedStr(status, "message"),
			Version:    nestedStr(spec, "version"),
			Conditions: extractConditions(status),
		})
	}
	return csvs, nil
}

func (s *LiveClusterSource) GetInstallPlans(ns string) ([]datasource.InstallPlanState, error) {
	list, err := s.client.dynamicClient.Resource(gvrInstallPlans).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	plans := make([]datasource.InstallPlanState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")
		status := nestedMap(item.Object, "status")

		ip := datasource.InstallPlanState{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Phase:      nestedStr(status, "phase"),
			Approved:   nestedBool(spec, "approved"),
			Conditions: extractConditions(status),
		}
		ip.Failed = ip.Phase == "Failed"

		if planSteps, ok := status["plan"].([]interface{}); ok {
			for _, step := range planSteps {
				stepMap, ok := step.(map[string]interface{})
				if !ok {
					continue
				}
				res := nestedMap(stepMap, "resource")
				ip.Steps = append(ip.Steps, datasource.InstallPlanStep{
					Group:     nestedStr(res, "group"),
					Kind:      nestedStr(res, "kind"),
					Name:      nestedStr(res, "name"),
					Version:   nestedStr(res, "version"),
					Status:    nestedStr(stepMap, "status"),
					Resolving: nestedStr(stepMap, "resolving"),
				})
			}
		}
		plans = append(plans, ip)
	}
	return plans, nil
}

func (s *LiveClusterSource) GetCatalogSources(ns string) ([]datasource.CatalogSourceState, error) {
	list, err := s.client.dynamicClient.Resource(gvrCatalogSources).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	sources := make([]datasource.CatalogSourceState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")
		status := nestedMap(item.Object, "status")
		connState := nestedMap(status, "connectionState")

		sources = append(sources, datasource.CatalogSourceState{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Image:      nestedStr(spec, "image"),
			Status:     nestedStr(connState, "lastObservedState"),
			Conditions: extractConditions(status),
		})
	}
	return sources, nil
}

func (s *LiveClusterSource) GetOperatorGroups(ns string) ([]datasource.OperatorGroupState, error) {
	list, err := s.client.dynamicClient.Resource(gvrOperatorGroups).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	groups := make([]datasource.OperatorGroupState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")

		og := datasource.OperatorGroupState{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		}

		if targets, ok := spec["targetNamespaces"].([]interface{}); ok {
			for _, t := range targets {
				if tStr, ok := t.(string); ok {
					og.TargetNamespaces = append(og.TargetNamespaces, tStr)
				}
			}
		}
		groups = append(groups, og)
	}
	return groups, nil
}

func (s *LiveClusterSource) GetDeployments(ns string) ([]datasource.DeploymentState, error) {
	list, err := s.client.clientset.AppsV1().Deployments(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	deployments := make([]datasource.DeploymentState, 0, len(list.Items))
	for _, d := range list.Items {
		ds := datasource.DeploymentState{
			Name:          d.Name,
			Namespace:     d.Namespace,
			Replicas:      ptrInt32(d.Spec.Replicas),
			ReadyReplicas: d.Status.ReadyReplicas,
		}
		for _, cond := range d.Status.Conditions {
			switch cond.Type {
			case "Available":
				ds.Available = cond.Status == "True"
				if cond.Status != "True" {
					ds.UnavailableMsg = cond.Message
				}
			case "Progressing":
				ds.Progressing = cond.Status == "True"
				ds.ProgressingMsg = cond.Message
			}
		}
		deployments = append(deployments, ds)
	}
	return deployments, nil
}

func (s *LiveClusterSource) GetPods(ns string) ([]datasource.PodState, error) {
	list, err := s.client.clientset.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]datasource.PodState, 0, len(list.Items))
	for _, p := range list.Items {
		ps := datasource.PodState{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
			Ready:     true,
		}

		var totalRestarts int32
		for _, cs := range p.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
			if !cs.Ready {
				ps.Ready = false
			}
			if cs.State.Waiting != nil {
				ps.WaitingReason = cs.State.Waiting.Reason
				ps.WaitingMessage = cs.State.Waiting.Message
			}
			if cs.State.Terminated != nil {
				ps.TerminatedReason = cs.State.Terminated.Reason
			}
		}
		ps.RestartCount = totalRestarts
		pods = append(pods, ps)
	}
	return pods, nil
}

func (s *LiveClusterSource) GetEvents(ns string) ([]datasource.EventState, error) {
	list, err := s.client.clientset.CoreV1().Events(ns).List(context.TODO(), metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		return nil, err
	}

	events := make([]datasource.EventState, 0, len(list.Items))
	for _, e := range list.Items {
		events = append(events, datasource.EventState{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Object:  fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
		})
	}
	return events, nil
}

func (s *LiveClusterSource) GetNodes() ([]datasource.NodeState, error) {
	list, err := s.client.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodes := make([]datasource.NodeState, 0, len(list.Items))
	for _, n := range list.Items {
		ns := datasource.NodeState{
			Name:             n.Name,
			Unschedulable:    n.Spec.Unschedulable,
			KubeletVersion:   n.Status.NodeInfo.KubeletVersion,
			OSImage:          n.Status.NodeInfo.OSImage,
			ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
			Capacity: datasource.ResourceList{
				CPU:    n.Status.Capacity.Cpu().String(),
				Memory: n.Status.Capacity.Memory().String(),
				Pods:   n.Status.Capacity.Pods().String(),
			},
			Allocatable: datasource.ResourceList{
				CPU:    n.Status.Allocatable.Cpu().String(),
				Memory: n.Status.Allocatable.Memory().String(),
				Pods:   n.Status.Allocatable.Pods().String(),
			},
		}

		for _, cond := range n.Status.Conditions {
			ns.Conditions = append(ns.Conditions, datasource.Condition{
				Type:               string(cond.Type),
				Status:             string(cond.Status),
				Reason:             cond.Reason,
				Message:            cond.Message,
				LastTransitionTime: cond.LastTransitionTime.Time,
			})
			if cond.Type == "Ready" && cond.Status == "True" {
				ns.Ready = true
			}
		}

		for key := range n.Labels {
			if strings.HasPrefix(key, "node-role.kubernetes.io/") {
				ns.Roles = append(ns.Roles, strings.TrimPrefix(key, "node-role.kubernetes.io/"))
			}
		}

		nodes = append(nodes, ns)
	}
	return nodes, nil
}

func (s *LiveClusterSource) GetNamespaces() ([]string, error) {
	list, err := s.client.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	namespaces := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

func (s *LiveClusterSource) GetClusterOperators() ([]datasource.ClusterOperatorState, error) {
	list, err := s.client.dynamicClient.Resource(gvrClusterOperators).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	cos := make([]datasource.ClusterOperatorState, 0, len(list.Items))
	for _, item := range list.Items {
		status := nestedMap(item.Object, "status")

		co := datasource.ClusterOperatorState{
			Name: item.GetName(),
		}

		if versions, ok := status["versions"].([]interface{}); ok {
			for _, v := range versions {
				if vMap, ok := v.(map[string]interface{}); ok {
					if nestedStr(vMap, "name") == "operator" {
						co.Version = nestedStr(vMap, "version")
					}
				}
			}
		}

		co.Conditions = extractConditions(status)
		for _, cond := range co.Conditions {
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

func (s *LiveClusterSource) GetClusterVersion() (*datasource.ClusterVersionState, error) {
	list, err := s.client.dynamicClient.Resource(gvrClusterVersions).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no ClusterVersion found")
	}

	item := list.Items[0]
	spec := nestedMap(item.Object, "spec")
	status := nestedMap(item.Object, "status")

	cv := &datasource.ClusterVersionState{
		Channel:   nestedStr(spec, "channel"),
		ClusterID: nestedStr(spec, "clusterID"),
	}

	if history, ok := status["history"].([]interface{}); ok {
		for _, h := range history {
			if hMap, ok := h.(map[string]interface{}); ok {
				uh := datasource.UpdateHistory{
					Version: nestedStr(hMap, "version"),
					State:   nestedStr(hMap, "state"),
				}
				if t, err := time.Parse(time.RFC3339, nestedStr(hMap, "startedTime")); err == nil {
					uh.StartedAt = t
				}
				if t, err := time.Parse(time.RFC3339, nestedStr(hMap, "completionTime")); err == nil {
					uh.CompletedAt = t
				}
				cv.History = append(cv.History, uh)
				if uh.State == "Completed" && cv.Version == "" {
					cv.Version = uh.Version
				}
			}
		}
	}

	cv.Conditions = extractConditions(status)
	for _, cond := range cv.Conditions {
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

func (s *LiveClusterSource) GetMachineConfigPools() ([]datasource.MCPState, error) {
	list, err := s.client.dynamicClient.Resource(gvrMCPs).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	mcps := make([]datasource.MCPState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")
		status := nestedMap(item.Object, "status")

		mcp := datasource.MCPState{
			Name:                 item.GetName(),
			MachineCount:         int32(nestedInt(status, "machineCount")),
			ReadyMachineCount:    int32(nestedInt(status, "readyMachineCount")),
			UpdatedMachineCount:  int32(nestedInt(status, "updatedMachineCount")),
			DegradedMachineCount: int32(nestedInt(status, "degradedMachineCount")),
			Paused:               nestedBool(spec, "paused"),
		}

		mcp.Conditions = extractConditions(status)
		for _, cond := range mcp.Conditions {
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

func (s *LiveClusterSource) GetNetworkConfig() (*datasource.NetworkConfigState, error) {
	obj, err := s.client.dynamicClient.Resource(gvrNetworks).Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	spec := nestedMap(obj.Object, "spec")
	status := nestedMap(obj.Object, "status")

	nc := &datasource.NetworkConfigState{
		NetworkType: nestedStr(status, "networkType"),
	}
	if nc.NetworkType == "" {
		nc.NetworkType = nestedStr(spec, "networkType")
	}

	if clusterNets, ok := spec["clusterNetwork"].([]interface{}); ok {
		for _, cn := range clusterNets {
			if cnMap, ok := cn.(map[string]interface{}); ok {
				nc.ClusterNetwork = append(nc.ClusterNetwork, datasource.NetworkRange{
					CIDR:       nestedStr(cnMap, "cidr"),
					HostPrefix: int32(nestedInt(cnMap, "hostPrefix")),
				})
			}
		}
	}

	if svcNets, ok := spec["serviceNetwork"].([]interface{}); ok {
		for _, sn := range svcNets {
			if snStr, ok := sn.(string); ok {
				nc.ServiceNetwork = append(nc.ServiceNetwork, snStr)
			}
		}
	}

	return nc, nil
}

func (s *LiveClusterSource) GetRoutes(ns string) ([]datasource.RouteState, error) {
	list, err := s.client.dynamicClient.Resource(gvrRoutes).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	routes := make([]datasource.RouteState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")
		status := nestedMap(item.Object, "status")

		r := datasource.RouteState{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Host:      nestedStr(spec, "host"),
			TLS:       len(nestedMap(spec, "tls")) > 0,
		}

		if ingresses, ok := status["ingress"].([]interface{}); ok {
			for _, ingress := range ingresses {
				if iMap, ok := ingress.(map[string]interface{}); ok {
					if conds, ok := iMap["conditions"].([]interface{}); ok {
						for _, c := range conds {
							if cMap, ok := c.(map[string]interface{}); ok {
								if nestedStr(cMap, "type") == "Admitted" && nestedStr(cMap, "status") == "True" {
									r.Admitted = true
								}
							}
						}
					}
				}
			}
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func (s *LiveClusterSource) GetPVs() ([]datasource.PVState, error) {
	list, err := s.client.clientset.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pvs := make([]datasource.PVState, 0, len(list.Items))
	for _, pv := range list.Items {
		p := datasource.PVState{
			Name:          pv.Name,
			Phase:         string(pv.Status.Phase),
			StorageClass:  pv.Spec.StorageClassName,
			ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		}
		if storage, ok := pv.Spec.Capacity["storage"]; ok {
			p.Capacity = storage.String()
		}
		if pv.Spec.ClaimRef != nil {
			p.ClaimRef = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		pvs = append(pvs, p)
	}
	return pvs, nil
}

func (s *LiveClusterSource) GetPVCs(ns string) ([]datasource.PVCState, error) {
	list, err := s.client.clientset.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pvcs := make([]datasource.PVCState, 0, len(list.Items))
	for _, pvc := range list.Items {
		p := datasource.PVCState{
			Name:       pvc.Name,
			Namespace:  pvc.Namespace,
			Phase:      string(pvc.Status.Phase),
			VolumeName: pvc.Spec.VolumeName,
		}
		if pvc.Spec.StorageClassName != nil {
			p.StorageClass = *pvc.Spec.StorageClassName
		}
		if storage, ok := pvc.Status.Capacity["storage"]; ok {
			p.Capacity = storage.String()
		}
		pvcs = append(pvcs, p)
	}
	return pvcs, nil
}

func (s *LiveClusterSource) GetStorageClasses() ([]datasource.StorageClassState, error) {
	list, err := s.client.clientset.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	scs := make([]datasource.StorageClassState, 0, len(list.Items))
	for _, sc := range list.Items {
		s := datasource.StorageClassState{
			Name:        sc.Name,
			Provisioner: sc.Provisioner,
			IsDefault:   sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true",
			Parameters:  make(map[string]string),
		}
		for k, v := range sc.Parameters {
			s.Parameters[k] = v
		}
		scs = append(scs, s)
	}
	return scs, nil
}

func (s *LiveClusterSource) GetCertificates() ([]datasource.CertificateState, error) {
	return []datasource.CertificateState{}, nil
}

func (s *LiveClusterSource) GetIDMS() ([]datasource.IDMSState, error) {
	list, err := s.client.dynamicClient.Resource(gvrIDMS).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// Resource may not exist on non-OpenShift clusters
		return []datasource.IDMSState{}, nil
	}

	idmsList := make([]datasource.IDMSState, 0, len(list.Items))
	for _, item := range list.Items {
		spec := nestedMap(item.Object, "spec")

		idms := datasource.IDMSState{
			Name: item.GetName(),
		}

		if mirrors, ok := spec["imageDigestMirrors"].([]interface{}); ok {
			for _, m := range mirrors {
				if mMap, ok := m.(map[string]interface{}); ok {
					entry := datasource.MirrorEntry{
						Source: nestedStr(mMap, "source"),
					}
					if mirrorList, ok := mMap["mirrors"].([]interface{}); ok {
						for _, mirror := range mirrorList {
							if mStr, ok := mirror.(string); ok {
								entry.Mirrors = append(entry.Mirrors, mStr)
							}
						}
					}
					idms.Mirrors = append(idms.Mirrors, entry)
				}
			}
		}
		idmsList = append(idmsList, idms)
	}
	return idmsList, nil
}

func (s *LiveClusterSource) GetEtcdMembers() ([]datasource.EtcdMemberState, error) {
	return []datasource.EtcdMemberState{}, nil
}

func (s *LiveClusterSource) GetAPIServerStatus() (*datasource.APIServerState, error) {
	obj, err := s.client.dynamicClient.Resource(gvrClusterOperators).Get(context.TODO(), "kube-apiserver", metav1.GetOptions{})
	if err != nil {
		return &datasource.APIServerState{Available: true}, nil
	}

	status := nestedMap(obj.Object, "status")
	as := &datasource.APIServerState{
		Conditions: extractConditions(status),
	}
	for _, cond := range as.Conditions {
		if cond.Type == "Available" {
			as.Available = cond.Status == "True"
		}
	}
	return as, nil
}

// --- helpers ---

func nestedMap(obj map[string]interface{}, key string) map[string]interface{} {
	if v, ok := obj[key].(map[string]interface{}); ok {
		return v
	}
	return map[string]interface{}{}
}

func nestedStr(obj map[string]interface{}, key string) string {
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}

func nestedBool(obj map[string]interface{}, key string) bool {
	if v, ok := obj[key].(bool); ok {
		return v
	}
	return false
}

func nestedInt(obj map[string]interface{}, key string) int64 {
	switch v := obj[key].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}

func extractConditions(status map[string]interface{}) []datasource.Condition {
	raw, ok := status["conditions"].([]interface{})
	if !ok {
		return nil
	}

	conds := make([]datasource.Condition, 0, len(raw))
	for _, item := range raw {
		cMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		c := datasource.Condition{
			Type:    nestedStr(cMap, "type"),
			Status:  nestedStr(cMap, "status"),
			Reason:  nestedStr(cMap, "reason"),
			Message: nestedStr(cMap, "message"),
		}
		if t, err := time.Parse(time.RFC3339, nestedStr(cMap, "lastTransitionTime")); err == nil {
			c.LastTransitionTime = t
		}
		conds = append(conds, c)
	}
	return conds
}

func ptrInt32(p *int32) int32 {
	if p == nil {
		return 1
	}
	return *p
}

// Compile-time interface check.
var _ datasource.ClusterDataSource = (*LiveClusterSource)(nil)
