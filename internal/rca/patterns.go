package rca

// Package rca provides root cause analysis patterns for operator failures.
// Patterns are based on OpenShift RCA best practices.

import (
	"regexp"
	"strings"
)

// Pattern represents a root cause analysis pattern.
type Pattern string

const (
	PatternAsymmetry          Pattern = "ASYMMETRY"               // Different behavior in similar contexts
	PatternMissingGuard       Pattern = "MISSING_GUARD"           // Missing null/error checks
	PatternTypeEscalation     Pattern = "TYPE_ESCALATION"         // Error type changes breaking handling
	PatternStateDivergence    Pattern = "STATE_DIVERGENCE"        // Inconsistent state across components
	PatternDefaultInversion   Pattern = "DEFAULT_INVERSION"       // Default behavior inverted between versions
	PatternRaceCondition      Pattern = "RACE_CONDITION"          // Timing-dependent failures
	PatternErrorSwallowing    Pattern = "ERROR_SWALLOWING"        // Errors silently ignored
	PatternCertChainBreak     Pattern = "CERTIFICATE_CHAIN_BREAK" // Cert expiry or CA rotation cascade
	PatternEtcdPressure       Pattern = "ETCD_PRESSURE"           // etcd disk/memory pressure
	PatternNetworkPartition   Pattern = "NETWORK_PARTITION"       // Partial connectivity loss
	PatternResourceStarvation Pattern = "RESOURCE_STARVATION"     // Cascading OOM/CPU throttling
	PatternUpgradeStall       Pattern = "UPGRADE_STALL"           // MCP or ClusterVersion stuck
	PatternClockSkew          Pattern = "CLOCK_SKEW"              // Time drift causing auth failures
)

// PatternMatch holds pattern detection results.
type PatternMatch struct {
	Pattern     Pattern
	Confidence  float64 // 0.0 to 1.0
	Evidence    []string
	Description string
}

// PatternDetector analyzes failure symptoms to identify RCA patterns.
type PatternDetector struct {
	// Pattern detection rules
	rules map[Pattern][]DetectionRule
}

// DetectionRule defines how to detect a pattern.
type DetectionRule struct {
	Keywords   []string       // Keywords to search for
	Regex      *regexp.Regexp // Optional regex pattern
	Weight     float64        // Contribution to confidence (0.0-1.0)
	RequireAll bool           // All keywords must match
}

// NewPatternDetector creates a new pattern detector with built-in rules.
func NewPatternDetector() *PatternDetector {
	detector := &PatternDetector{
		rules: make(map[Pattern][]DetectionRule),
	}

	// ASYMMETRY pattern rules
	detector.rules[PatternAsymmetry] = []DetectionRule{
		{
			Keywords:   []string{"phase", "mirror", "archive", "tolerates", "fatal"},
			Weight:     0.7,
			RequireAll: false,
		},
		{
			Keywords:   []string{"succeeds", "fails", "same", "different"},
			Weight:     0.5,
			RequireAll: false,
		},
	}

	// MISSING GUARD pattern rules
	detector.rules[PatternMissingGuard] = []DetectionRule{
		{
			Keywords: []string{"nil", "null", "NullPointerException", "AttributeError", "NoneType"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"panic", "nil pointer dereference"},
			Weight:   0.9,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(missing|no)\s+(check|guard|validation)`),
			Weight: 0.6,
		},
	}

	// TYPE ESCALATION pattern rules
	detector.rules[PatternTypeEscalation] = []DetectionRule{
		{
			Keywords: []string{"error", "type", "changed", "incompatible"},
			Weight:   0.6,
		},
		{
			Regex:  regexp.MustCompile(`(?i)expected\s+\w+\s+got\s+\w+`),
			Weight: 0.7,
		},
	}

	// STATE DIVERGENCE pattern rules
	detector.rules[PatternStateDivergence] = []DetectionRule{
		{
			Keywords: []string{"state", "inconsistent", "mismatch", "diverged"},
			Weight:   0.7,
		},
		{
			Keywords: []string{"stale", "outdated", "not synchronized"},
			Weight:   0.6,
		},
	}

	// DEFAULT INVERSION pattern rules
	detector.rules[PatternDefaultInversion] = []DetectionRule{
		{
			Keywords: []string{"default", "changed", "inverted", "opposite"},
			Weight:   0.6,
		},
		{
			Keywords: []string{"worked", "now fails", "regression"},
			Weight:   0.5,
		},
	}

	// RACE CONDITION pattern rules
	detector.rules[PatternRaceCondition] = []DetectionRule{
		{
			Keywords: []string{"race", "concurrent", "timing", "thread"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"intermittent", "sometimes", "randomly"},
			Weight:   0.5,
		},
		{
			Regex:  regexp.MustCompile(`(?i)\d+ms\s+(later|after|before)`),
			Weight: 0.6,
		},
		{
			Keywords: []string{"started", "stopped", "modified", "accessed"},
			Weight:   0.4,
		},
	}

	// ERROR SWALLOWING pattern rules
	detector.rules[PatternErrorSwallowing] = []DetectionRule{
		{
			Keywords: []string{"ignored", "silently", "suppressed", "swallowed"},
			Weight:   0.7,
		},
		{
			Keywords: []string{"error", "not logged", "not returned"},
			Weight:   0.6,
		},
	}

	// CERTIFICATE CHAIN BREAK pattern rules
	detector.rules[PatternCertChainBreak] = []DetectionRule{
		{
			Keywords: []string{"certificate", "expired", "x509"},
			Weight:   0.9,
		},
		{
			Keywords: []string{"tls", "handshake", "unknown authority"},
			Weight:   0.8,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(cert|certificate)\s+(expir|rotat|renew|invalid)`),
			Weight: 0.8,
		},
		{
			Keywords: []string{"ca", "bundle", "trust", "signing"},
			Weight:   0.5,
		},
	}

	// ETCD PRESSURE pattern rules
	detector.rules[PatternEtcdPressure] = []DetectionRule{
		{
			Keywords: []string{"etcd", "slow", "apply", "leader"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"etcd", "quota", "backend", "exceeded"},
			Weight:   0.9,
		},
		{
			Keywords: []string{"etcd", "compaction", "snapshot", "wal"},
			Weight:   0.7,
		},
		{
			Regex:  regexp.MustCompile(`(?i)etcd.*\b(disk|io|latency|timeout)\b`),
			Weight: 0.7,
		},
	}

	// NETWORK PARTITION pattern rules
	detector.rules[PatternNetworkPartition] = []DetectionRule{
		{
			Keywords: []string{"connection refused", "connection timed out", "no route to host"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"network", "unreachable", "partition"},
			Weight:   0.7,
		},
		{
			Keywords: []string{"dns", "resolve", "nxdomain", "lookup"},
			Weight:   0.6,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(ovn|sdn|geneve|vxlan)\s+(error|fail|down)`),
			Weight: 0.8,
		},
	}

	// RESOURCE STARVATION pattern rules
	detector.rules[PatternResourceStarvation] = []DetectionRule{
		{
			Keywords: []string{"oomkilled", "oom", "out of memory"},
			Weight:   0.9,
		},
		{
			Keywords: []string{"cpu", "throttl", "limit", "exceeded"},
			Weight:   0.7,
		},
		{
			Keywords: []string{"evicted", "preempted", "insufficient"},
			Weight:   0.8,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(memory|cpu)\s+(pressure|exhausted|exceeded)`),
			Weight: 0.8,
		},
	}

	// UPGRADE STALL pattern rules
	detector.rules[PatternUpgradeStall] = []DetectionRule{
		{
			Keywords: []string{"upgrade", "stuck", "progressing", "stall"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"machineconfigpool", "degraded", "render"},
			Weight:   0.7,
		},
		{
			Keywords: []string{"clusterversion", "failing", "unable to apply"},
			Weight:   0.8,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(mcp|machineconfig)\s+(degraded|render|fail|stuck)`),
			Weight: 0.8,
		},
	}

	// CLOCK SKEW pattern rules
	detector.rules[PatternClockSkew] = []DetectionRule{
		{
			Keywords: []string{"clock", "skew", "drift", "ntp"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"token", "expired", "future", "not yet valid"},
			Weight:   0.6,
		},
		{
			Regex:  regexp.MustCompile(`(?i)(time|clock)\s+(out of sync|drift|skew)`),
			Weight: 0.8,
		},
	}

	return detector
}

// DetectPatterns analyzes failure symptoms and returns matching patterns.
func (d *PatternDetector) DetectPatterns(failureSymptoms string) []PatternMatch {
	matches := make([]PatternMatch, 0)

	symptomsLower := strings.ToLower(failureSymptoms)

	for pattern, rules := range d.rules {
		confidence := 0.0
		evidence := make([]string, 0)

		for _, rule := range rules {
			ruleMatched := false
			ruleEvidence := make([]string, 0)

			// Check keywords
			if len(rule.Keywords) > 0 {
				matchedKeywords := 0
				for _, keyword := range rule.Keywords {
					if strings.Contains(symptomsLower, strings.ToLower(keyword)) {
						matchedKeywords++
						ruleEvidence = append(ruleEvidence, keyword)
					}
				}

				if rule.RequireAll {
					ruleMatched = matchedKeywords == len(rule.Keywords)
				} else {
					ruleMatched = matchedKeywords > 0
				}
			}

			// Check regex
			if rule.Regex != nil && rule.Regex.MatchString(failureSymptoms) {
				ruleMatched = true
				regexMatch := rule.Regex.FindString(failureSymptoms)
				if regexMatch != "" {
					ruleEvidence = append(ruleEvidence, regexMatch)
				}
			}

			if ruleMatched {
				confidence += rule.Weight
				evidence = append(evidence, ruleEvidence...)
			}
		}

		// Normalize confidence to 0.0-1.0 range
		if confidence > 1.0 {
			confidence = 1.0
		}

		// Only include patterns with meaningful confidence
		if confidence >= 0.3 {
			matches = append(matches, PatternMatch{
				Pattern:     pattern,
				Confidence:  confidence,
				Evidence:    evidence,
				Description: getPatternDescription(pattern),
			})
		}
	}

	// Sort by confidence (highest first)
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Confidence > matches[i].Confidence {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// getPatternDescription returns a human-readable description of the pattern.
func getPatternDescription(p Pattern) string {
	descriptions := map[Pattern]string{
		PatternAsymmetry:          "Different behavior in similar contexts (e.g., phase A tolerates errors, phase B fails)",
		PatternMissingGuard:       "Missing null/error checks causing crashes",
		PatternTypeEscalation:     "Error type changes breaking error handling logic",
		PatternStateDivergence:    "Inconsistent state across components or replicas",
		PatternDefaultInversion:   "Default behavior changed between versions",
		PatternRaceCondition:      "Timing-dependent failures due to concurrent execution",
		PatternErrorSwallowing:    "Errors silently ignored or suppressed",
		PatternCertChainBreak:     "Certificate expiry or CA rotation causing cascading authentication failures",
		PatternEtcdPressure:       "etcd disk/memory pressure causing API server instability and slow responses",
		PatternNetworkPartition:   "Partial network connectivity loss between nodes or to external services",
		PatternResourceStarvation: "Cascading OOM kills or CPU throttling across dependent workloads",
		PatternUpgradeStall:       "MachineConfigPool or ClusterVersion stuck in progressing/degraded state",
		PatternClockSkew:          "Time drift causing certificate validation failures or token expiry",
	}

	if desc, ok := descriptions[p]; ok {
		return desc
	}
	return string(p)
}

// AnalysisRecommendation provides actionable fix recommendations.
type AnalysisRecommendation struct {
	Priority    int // 1=Critical, 2=High, 3=Medium
	Title       string
	Description string
	CodeExample string // Optional code snippet
}

// GetRecommendations returns fix recommendations for a detected pattern.
func GetRecommendations(pattern Pattern, context string) []AnalysisRecommendation {
	switch pattern {
	case PatternMissingGuard:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Add nil/null guard checks",
				Description: "Validate objects are not nil before accessing their members",
				CodeExample: "if obj != nil && obj.Field != \"\" {\n    // safe to use\n}",
			},
			{
				Priority:    2,
				Title:       "Use safe navigation operators",
				Description: "Consider using safe navigation or optional chaining where available",
			},
		}

	case PatternRaceCondition:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Add synchronization primitives",
				Description: "Use mutex/lock to serialize access to shared state",
				CodeExample: "mutex.Lock()\ndefer mutex.Unlock()\n// access shared state",
			},
			{
				Priority:    1,
				Title:       "Add startup guard flag",
				Description: "Track initialization state to prevent config changes during startup",
				CodeExample: "if !self._engine_started {\n    return  // skip during startup\n}",
			},
			{
				Priority:    2,
				Title:       "Introduce message queue",
				Description: "Serialize state changes through a channel/queue to avoid races",
			},
		}

	case PatternAsymmetry:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Unify error handling across phases",
				Description: "Make error handling consistent between similar code paths",
			},
			{
				Priority:    2,
				Title:       "Extract common error handler",
				Description: "Create shared error handling function used by all phases",
			},
		}

	case PatternStateDivergence:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Implement state reconciliation",
				Description: "Add periodic checks to ensure state consistency across components",
			},
			{
				Priority:    2,
				Title:       "Add state version tracking",
				Description: "Use version numbers or timestamps to detect stale state",
			},
		}

	case PatternErrorSwallowing:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Log all errors",
				Description: "Ensure errors are logged even if recovered",
				CodeExample: "if err != nil {\n    log.Error(err, \"operation failed\")\n    return err\n}",
			},
			{
				Priority:    2,
				Title:       "Add error metrics",
				Description: "Export error counts as Prometheus metrics for visibility",
			},
		}

	case PatternCertChainBreak:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Check certificate expiry dates",
				Description: "Run `oc get secret -A -o json | jq '.items[] | select(.type==\"kubernetes.io/tls\")' | openssl x509 -noout -dates` to find expired certs",
			},
			{
				Priority:    1,
				Title:       "Force certificate rotation",
				Description: "For OpenShift managed certs, delete the secret to trigger automatic regeneration. For custom CA, rotate and update the CA bundle ConfigMap in openshift-config",
			},
		}

	case PatternEtcdPressure:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Check etcd disk performance",
				Description: "Run `etcdctl endpoint status` and check WAL fsync latency. Ensure etcd data dir is on fast SSD with <10ms p99 fsync",
			},
			{
				Priority:    1,
				Title:       "Defragment etcd",
				Description: "Run `etcdctl defrag` on each member sequentially. Check db size vs quota with `etcdctl endpoint status -w table`",
			},
			{
				Priority:    2,
				Title:       "Review etcd compaction",
				Description: "Ensure automatic compaction is running. Check `etcd_mvcc_db_total_size_in_bytes` vs `etcd_server_quota_backend_bytes`",
			},
		}

	case PatternNetworkPartition:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Verify node-to-node connectivity",
				Description: "Check OVN-Kubernetes or SDN pod health on all nodes. Verify geneve/VXLAN overlay connectivity between nodes",
			},
			{
				Priority:    1,
				Title:       "Check DNS resolution",
				Description: "Verify CoreDNS pods are running and responding. Test `nslookup kubernetes.default.svc.cluster.local` from affected pods",
			},
			{
				Priority:    2,
				Title:       "Review network policies",
				Description: "Check if NetworkPolicies or EgressFirewall rules are blocking required traffic paths",
			},
		}

	case PatternResourceStarvation:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Review resource limits and requests",
				Description: "Check if pods have appropriate resource requests/limits. OOM kills indicate limits too low or memory leaks",
			},
			{
				Priority:    1,
				Title:       "Check node resource pressure",
				Description: "Run `oc describe node` and check for MemoryPressure, DiskPressure, PIDPressure conditions",
			},
			{
				Priority:    2,
				Title:       "Review LimitRange and ResourceQuota",
				Description: "Check namespace LimitRange and ResourceQuota objects for overly restrictive constraints",
			},
		}

	case PatternUpgradeStall:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Check MachineConfigPool status",
				Description: "Run `oc get mcp` and check for degraded machines. Review `oc describe mcp <pool>` for render errors",
			},
			{
				Priority:    1,
				Title:       "Review ClusterVersion conditions",
				Description: "Run `oc get clusterversion -o yaml` and check Failing/Progressing conditions for specific error messages",
			},
			{
				Priority:    2,
				Title:       "Check operator compatibility",
				Description: "Verify all installed operators support the target OpenShift version. Check CSV installModes and API compatibility",
			},
		}

	case PatternClockSkew:
		return []AnalysisRecommendation{
			{
				Priority:    1,
				Title:       "Check NTP synchronization",
				Description: "Run `chronyc tracking` on each node. Ensure chrony/NTP is configured and syncing to a reliable time source",
			},
			{
				Priority:    2,
				Title:       "Review certificate validity windows",
				Description: "Clock skew can make valid certificates appear expired or not-yet-valid. Fix time sync before investigating cert issues",
			},
		}

	default:
		return []AnalysisRecommendation{
			{
				Priority:    3,
				Title:       "Review code changes",
				Description: "Examine git diff for suspicious changes related to the failure",
			},
		}
	}
}
