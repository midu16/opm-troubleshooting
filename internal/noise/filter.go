package noise

import (
	"regexp"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
)

// Environment classifies the deployment context for noise filtering.
type Environment string

const (
	EnvProduction   Environment = "production"
	EnvDisconnected Environment = "disconnected"
	EnvLab          Environment = "lab"
	EnvKVM          Environment = "kvm"
)

// Classification indicates whether a finding is real or cosmetic noise.
type Classification string

const (
	ClassReal      Classification = "real"
	ClassCosmetic  Classification = "cosmetic"
	ClassAmbiguous Classification = "ambiguous"
)

// FilteredFinding wraps a health dimension result with noise classification.
type FilteredFinding struct {
	Dimension      healthcheck.DimensionResult `json:"dimension"`
	Classification Classification              `json:"classification"`
	NoiseReason    string                      `json:"noise_reason,omitempty"`
	ActionRequired bool                        `json:"action_required"`
}

// FilterReport summarizes noise-filtered health results.
type FilterReport struct {
	Environment     Environment       `json:"environment"`
	TotalFindings   int               `json:"total_findings"`
	RealIssues      int               `json:"real_issues"`
	CosmeticAlerts  int               `json:"cosmetic_alerts"`
	AmbiguousAlerts int               `json:"ambiguous_alerts"`
	Findings        []FilteredFinding `json:"findings"`
}

// noiseRule defines a pattern that indicates cosmetic noise in a given environment.
type noiseRule struct {
	Environments []Environment
	Patterns     []*regexp.Regexp
	Keywords     []string
	Reason       string
}

var noiseRules = []noiseRule{
	{
		Environments: []Environment{EnvKVM, EnvLab},
		Keywords:     []string{"kvm", "libvirt", "no route to host", "connection reset"},
		Reason:       "KVM/lab networking: node connectivity alerts are often transient in virtualized lab environments",
	},
	{
		Environments: []Environment{EnvKVM, EnvLab},
		Keywords:     []string{"nodedisruption", "machinehealthcheck", "unhealthy node"},
		Reason:       "MachineHealthCheck may trigger on lab nodes with intentional power management",
	},
	{
		Environments: []Environment{EnvDisconnected, EnvLab},
		Keywords:     []string{"registry.redhat.io", "imagepullbackoff", "errimagepull", "unauthorized"},
		Reason:       "Disconnected cluster: direct pulls from registry.redhat.io expected to fail; verify IDMS mirror path",
	},
	{
		Environments: []Environment{EnvDisconnected},
		Keywords:     []string{"catalogsource", "catalogsourcesunhealthy", "connection refused"},
		Reason:       "Disconnected cluster: catalog pod may need mirrored index image",
	},
	{
		Environments: []Environment{EnvLab},
		Keywords:     []string{"x509", "certificate", "self-signed", "tls", "unknown authority"},
		Reason:       "Lab environment: self-signed certificates are common and may not indicate production issues",
	},
	{
		Environments: []Environment{EnvLab, EnvKVM},
		Keywords:     []string{"dns", "no such host", "name resolution", "lookup failed"},
		Reason:       "Lab/KVM: external DNS resolution failures are often expected in isolated networks",
	},
	{
		Environments: []Environment{EnvLab},
		Keywords:     []string{"upgrade pending", "requiresapproval"},
		Reason:       "Lab redeployment: InstallPlan awaiting approval is an operational step, not a fault",
	},
	{
		Environments: []Environment{EnvDisconnected, EnvLab},
		Patterns:     []*regexp.Regexp{regexp.MustCompile(`(?i)back.?off.*pull`)},
		Reason:       "Mirror fallback in progress: image pull backoff may resolve once IDMS routes to local registry",
	},
	// Infrastructure noise rules
	{
		Environments: []Environment{EnvLab, EnvKVM},
		Keywords:     []string{"etcd", "leader", "election", "changed leader"},
		Reason:       "etcd leader elections during lab cluster restarts are expected and transient",
	},
	{
		Environments: []Environment{EnvLab},
		Keywords:     []string{"machineconfigpool", "paused", "mcp paused"},
		Reason:       "MCP paused state in lab is an operational choice, not a fault",
	},
	{
		Environments: []Environment{EnvKVM, EnvLab},
		Keywords:     []string{"etcd", "slow", "heartbeat", "clock drift"},
		Reason:       "KVM virtualization introduces latency that can trigger etcd slow heartbeat warnings",
	},
	{
		Environments: []Environment{EnvLab, EnvKVM},
		Keywords:     []string{"node_health", "notready", "not ready"},
		Reason:       "Lab/KVM nodes may be intentionally powered off or in maintenance mode",
	},
	{
		Environments: []Environment{EnvLab},
		Keywords:     []string{"cluster version", "progressing", "upgrade"},
		Reason:       "Cluster upgrades in lab are often intentional and may take time to complete",
	},
	{
		Environments: []Environment{EnvDisconnected},
		Keywords:     []string{"monitoring", "prometheus", "thanos", "alertmanager", "telemetry"},
		Reason:       "Monitoring components may have reduced functionality in disconnected environments due to telemetry restrictions",
	},
}

// Filter applies environment-aware noise classification to health check results.
func Filter(env Environment, dimensions []healthcheck.DimensionResult) *FilterReport {
	if env == "" {
		env = EnvProduction
	}

	report := &FilterReport{
		Environment: env,
		Findings:    make([]FilteredFinding, 0),
	}

	for _, dim := range dimensions {
		// Skip healthy, skipped, and not-installed operators
		if dim.Status == healthcheck.StatusPass || dim.Status == healthcheck.StatusSkip {
			continue
		}

		report.TotalFindings++
		ff := FilteredFinding{
			Dimension: dim,
		}

		text := collectText(dim)
		if reason, isNoise := matchNoise(env, text); isNoise {
			ff.Classification = ClassCosmetic
			ff.NoiseReason = reason
			ff.ActionRequired = false
			report.CosmeticAlerts++
		} else if dim.Severity == healthcheck.SeverityWarning && env != EnvProduction {
			ff.Classification = ClassAmbiguous
			ff.NoiseReason = "Warning in non-production environment; verify before escalating"
			ff.ActionRequired = false
			report.AmbiguousAlerts++
		} else {
			ff.Classification = ClassReal
			ff.ActionRequired = true
			report.RealIssues++
		}

		report.Findings = append(report.Findings, ff)
	}

	return report
}

func collectText(dim healthcheck.DimensionResult) string {
	parts := []string{dim.Summary, dim.Recommendation}
	parts = append(parts, dim.Evidence...)
	return strings.ToLower(strings.Join(parts, " "))
}

func matchNoise(env Environment, text string) (string, bool) {
	for _, rule := range noiseRules {
		if !envMatches(env, rule.Environments) {
			continue
		}
		for _, kw := range rule.Keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				return rule.Reason, true
			}
		}
		for _, pat := range rule.Patterns {
			if pat.MatchString(text) {
				return rule.Reason, true
			}
		}
	}
	return "", false
}

func envMatches(env Environment, targets []Environment) bool {
	for _, t := range targets {
		if t == env {
			return true
		}
	}
	// Lab inherits KVM noise rules
	if env == EnvLab {
		for _, t := range targets {
			if t == EnvKVM {
				return true
			}
		}
	}
	return false
}

// ParseEnvironment normalizes user input to a known environment.
func ParseEnvironment(s string) Environment {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "disconnected", "disconnected-cluster", "mirror":
		return EnvDisconnected
	case "lab", "dev", "development":
		return EnvLab
	case "kvm", "virtualized", "virtual":
		return EnvKVM
	default:
		return EnvProduction
	}
}
