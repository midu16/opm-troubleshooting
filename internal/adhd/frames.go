package adhd

import (
	"math/rand"
	"sort"
)

// allFrames holds the 13 OpenShift-specific diagnostic frames.
// Each frame is a cognitive vantage point that re-poses the problem.
var allFrames = []Frame{
	{
		ID:   "network-engineer",
		Name: "Network Engineer",
		Tags: []string{"infrastructure", "networking"},
		VantagePrompt: "You are a network engineer investigating an OpenShift cluster issue. " +
			"Forget everything except the packet path: think about DNS resolution chains, OVN-Kubernetes " +
			"flow tables, pod-to-pod connectivity across nodes, service ClusterIP routing, and network " +
			"policy enforcement. Consider MTU mismatches between overlay and underlay, conntrack table " +
			"exhaustion, kube-proxy/OVN iptables rule ordering, and whether egress traffic from pods " +
			"can actually reach external registries or API endpoints.",
	},
	{
		ID:   "storage-admin",
		Name: "Storage Admin",
		Tags: []string{"infrastructure", "storage"},
		VantagePrompt: "You are a storage administrator looking at an OpenShift cluster problem through " +
			"the lens of I/O and persistence. Focus on PersistentVolume lifecycle states, CSI driver " +
			"health and socket availability, etcd disk-pressure symptoms (fdatasync latency, WAL write " +
			"durations), and volume attachment/detachment races. Consider whether the storage class " +
			"provisioner is responsive, whether IOPS throttling on the underlying infrastructure is " +
			"causing cascading timeouts, and whether node-local storage is silently full.",
	},
	{
		ID:   "security-auditor",
		Name: "Security Auditor",
		Tags: []string{"security"},
		VantagePrompt: "You are a security auditor examining an OpenShift cluster failure. " +
			"Think about RBAC misconfigurations where a ServiceAccount lacks a required ClusterRole " +
			"binding, SCC violations that prevent pods from running with the right capabilities, and " +
			"certificate chain breaks where an intermediate CA has expired or been rotated without " +
			"updating trust bundles. Consider OAuth token expiry, image pull secret scoping errors, " +
			"network policy rules that silently block required control-plane traffic, and whether " +
			"admission webhooks are rejecting resources that the operator needs to create.",
	},
	{
		ID:   "capacity-planner",
		Name: "Capacity Planner",
		Tags: []string{"infrastructure"},
		VantagePrompt: "You are a capacity planner analyzing an OpenShift cluster problem from the " +
			"perspective of resource exhaustion. Examine the gap between resource requests, limits, " +
			"and actual utilization on every node. Consider whether ResourceQuota or LimitRange objects " +
			"in the namespace are silently blocking pod creation, whether the scheduler cannot place " +
			"pods due to topology constraints or taints, whether PodDisruptionBudgets are blocking " +
			"evictions, and whether pod priority and preemption are causing unexpected cascading failures.",
	},
	{
		ID:   "upgrade-specialist",
		Name: "Upgrade Specialist",
		Tags: []string{"infrastructure", "lifecycle"},
		VantagePrompt: "You are an upgrade specialist focused on version transitions in OpenShift. " +
			"Think about version skew between the kubelet and the API server, operator compatibility " +
			"matrices where a specific operator version is only certified for certain OCP ranges, and " +
			"MachineConfigPool rollout stalls where nodes are stuck in a rendered config transition. " +
			"Consider whether the cluster's update channel graph has a gap, whether deprecated or " +
			"removed APIs are being called by operators compiled against an older SDK, and whether " +
			"a partial upgrade left the cluster in an inconsistent mixed-version state.",
	},
	{
		ID:   "3am-oncall-sre",
		Name: "3AM On-Call SRE",
		Tags: []string{"infrastructure", "wild"},
		VantagePrompt: "You are an SRE paged at 3AM for this OpenShift cluster issue. You need to " +
			"determine the blast radius RIGHT NOW: how many customers or workloads are affected, is " +
			"the damage spreading, and what is the fastest mitigation that will not cause further harm. " +
			"Think about what is actively paging, what will page next if you do nothing, and what " +
			"well-intentioned fix would actually make things worse. Prioritize stabilization over " +
			"root cause, but note what evidence you need to preserve before it is lost.",
	},
	{
		ID:   "etcd-specialist",
		Name: "etcd Specialist",
		Tags: []string{"infrastructure"},
		VantagePrompt: "You are an etcd specialist diagnosing an OpenShift cluster problem at the " +
			"consensus layer. Focus on leader election stability, WAL corruption indicators, compaction " +
			"backlog size, slow-apply warnings, and the quota backend bytes threshold. Consider whether " +
			"the etcd snapshot size is abnormally large (indicating a leak of Kubernetes objects), " +
			"whether member health checks show a partitioned member, whether clock skew between etcd " +
			"peers is causing Raft term confusion, and whether disk I/O latency is causing heartbeat " +
			"timeouts that trigger unnecessary leader elections.",
	},
	{
		ID:   "adversarial",
		Name: "Adversarial Thinker",
		Tags: []string{"wild"},
		VantagePrompt: "You are an adversarial thinker: if you wanted to DELIBERATELY cause exactly " +
			"the symptoms being observed in this OpenShift cluster, what sequence of actions would you " +
			"take? Think about what misconfiguration, what race condition, what resource you would " +
			"delete or corrupt to produce these exact failure modes. Work backwards from the symptoms " +
			"to the simplest action that reproduces them. This reverse-engineering often reveals the " +
			"accidental root cause that forward analysis misses.",
	},
	{
		ID:   "assumption-remover",
		Name: "Assumption Remover",
		Tags: []string{"infrastructure", "wild"},
		VantagePrompt: "You are an assumption remover. Take every load-bearing assumption in the " +
			"current diagnosis and ask: what if this is wrong? What if DNS is returning stale or " +
			"incorrect records? What if node clocks are skewed by more than the certificate tolerance? " +
			"What if the node reported as 'Ready' is actually network-partitioned from the others? " +
			"What if the container image being pulled is not the one you think it is due to a tag " +
			"mutation? Systematically remove each assumption and see which removal best explains " +
			"the observed symptoms.",
	},
	{
		ID:   "dependency-walker",
		Name: "Dependency Walker",
		Tags: []string{"lifecycle"},
		VantagePrompt: "You are a dependency walker tracing the full chain from symptom to root cause. " +
			"Start at the failing pod and walk upward: pod -> ReplicaSet -> Deployment -> operator " +
			"controller -> ClusterServiceVersion -> Subscription -> CatalogSource -> registry image " +
			"-> network connectivity to registry. At each link, check whether the resource exists, " +
			"is in the expected state, and has the right owner references. The first broken link in " +
			"this chain is almost always the actual root cause, not the symptom at the leaf.",
	},
	{
		ID:   "timeline-reconstructor",
		Name: "Timeline Reconstructor",
		Tags: []string{"infrastructure", "lifecycle"},
		VantagePrompt: "You are a timeline reconstructor. Collect ALL timestamped events: pod " +
			"transitions, node condition changes, operator status updates, MCP rollouts, certificate " +
			"rotations, image pulls, and API server audit entries. Order them chronologically and find " +
			"the first domino -- the earliest event that deviates from normal. What changed in the " +
			"cluster, its configuration, or its environment in the minutes or hours before the failure " +
			"started? Correlation in time is the strongest signal for causation in distributed systems.",
	},
	{
		ID:   "platform-architect",
		Name: "Platform Architect",
		Tags: []string{"wild"},
		VantagePrompt: "You are a platform architect stepping back from the immediate symptoms to " +
			"ask whether this failure is a symptom of a deeper architectural mismatch. Is this cluster " +
			"running day-2 operations on a day-1 configuration? Is the network topology wrong for this " +
			"workload pattern? Is there design debt from shortcuts taken during initial deployment that " +
			"is now manifesting as operational failures? Consider whether the problem will keep recurring " +
			"even after a fix because the fundamental design assumptions are wrong.",
	},
	{
		ID:   "source-code-forensics",
		Name: "Source Code Forensics Analyst",
		Tags: []string{"lifecycle", "wild"},
		VantagePrompt: "You are a source code forensics analyst. Given the operator's failure, " +
			"you examine the deployed commit, the git history around it, and the diff between " +
			"installed and target versions. You look for recently introduced bugs, regression " +
			"patterns, misconfigured defaults in code, and whether the failure signature matches " +
			"error handling paths in the source. You distinguish between issues caused by code " +
			"changes (new bugs, regressions, incorrect error returns) and issues caused by " +
			"environmental configuration (missing CRDs, wrong RBAC, resource limits, network " +
			"policy). You consider whether the failure string appears in the source as a " +
			"hardcoded error message (suggesting a code path that was intentionally coded) " +
			"versus being generated by a framework (suggesting a runtime configuration issue).",
	},
}

// AllFrames returns all 13 diagnostic frames.
func AllFrames() []Frame {
	out := make([]Frame, len(allFrames))
	copy(out, allFrames)
	return out
}

// SelectFrames picks frames biased toward those matching the given tags,
// always including at least one frame tagged "wild". If count exceeds the
// total number of frames, all frames are returned.
func SelectFrames(tags []string, count int) []Frame {
	all := AllFrames()
	if count <= 0 || count >= len(all) {
		return all
	}

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	// Score each frame by how many of the requested tags it matches.
	type scored struct {
		frame Frame
		score int
	}
	scored_frames := make([]scored, len(all))
	for i, f := range all {
		s := 0
		for _, ft := range f.Tags {
			if tagSet[ft] {
				s++
			}
		}
		scored_frames[i] = scored{frame: f, score: s}
	}

	// Stable sort: highest match score first, then preserve definition order.
	sort.SliceStable(scored_frames, func(i, j int) bool {
		return scored_frames[i].score > scored_frames[j].score
	})

	// Shuffle within same-score tiers to add variety across runs.
	start := 0
	for start < len(scored_frames) {
		end := start + 1
		for end < len(scored_frames) && scored_frames[end].score == scored_frames[start].score {
			end++
		}
		if end-start > 1 {
			tier := scored_frames[start:end]
			rand.Shuffle(len(tier), func(i, j int) {
				tier[i], tier[j] = tier[j], tier[i]
			})
		}
		start = end
	}

	// Take top-count, ensuring at least one "wild" frame.
	selected := make([]Frame, 0, count)
	hasWild := false
	for i := 0; i < count && i < len(scored_frames); i++ {
		selected = append(selected, scored_frames[i].frame)
		if frameHasTag(scored_frames[i].frame, "wild") {
			hasWild = true
		}
	}

	// If no wild frame was selected, swap the last selected frame with the
	// highest-ranked wild frame that was not selected.
	if !hasWild && len(selected) > 0 {
		for i := count; i < len(scored_frames); i++ {
			if frameHasTag(scored_frames[i].frame, "wild") {
				selected[len(selected)-1] = scored_frames[i].frame
				break
			}
		}
	}

	return selected
}

// frameHasTag returns true if the frame has the given tag.
func frameHasTag(f Frame, tag string) bool {
	for _, t := range f.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
