package rca

import (
	"strings"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
)

func TestGenerateDocument(t *testing.T) {
	doc := GenerateDocument(ReportInput{
		ClusterName:    "lab-cluster-01",
		Environment:    noise.EnvLab,
		MustGatherPath: "/tmp/mg",
		Operator:       "redhat-oadp-operator",
		Namespace:      "openshift-adp",
		OperatorState: mustgather.OperatorState{
			PackageName:   "redhat-oadp-operator",
			Namespace:     "openshift-adp",
			State:         "Failed",
			Faulty:        true,
			FailureReason: "Missing CRDs: foo.example.com",
		},
		HealthReport: &healthcheck.Report{
			TotalDimensions: 20,
			Passed:          15,
			Failed:          2,
			Warnings:        2,
			Skipped:         1,
			Dimensions: []healthcheck.DimensionResult{
				{Name: "CSV Phase", Category: "OLM", Status: healthcheck.StatusFail, Severity: healthcheck.SeverityCritical, Summary: "CSV not succeeded"},
			},
		},
		NoiseReport: &noise.FilterReport{
			Environment:    noise.EnvLab,
			TotalFindings:  2,
			RealIssues:     1,
			CosmeticAlerts: 1,
		},
	})

	if doc.Title == "" {
		t.Fatal("empty title")
	}
	if !strings.Contains(doc.Markdown, "## Executive Summary") {
		t.Error("missing executive summary")
	}
	if !strings.Contains(doc.Markdown, "## OLM Health Check") {
		t.Error("missing health check section")
	}
	if !strings.Contains(doc.Markdown, "## Root Cause Analysis") {
		t.Error("missing root cause section")
	}
	if !strings.Contains(doc.Markdown, "## Recommended Actions") {
		t.Error("missing recommendations section")
	}
	if !strings.Contains(doc.Markdown, "lab-cluster-01") {
		t.Error("missing cluster name")
	}
}

func TestGenerateDocumentWithADHD(t *testing.T) {
	doc := GenerateDocument(ReportInput{
		Operator: "sriov-network-operator",
		OperatorState: mustgather.OperatorState{
			PackageName: "sriov-network-operator",
			State:       "Failed",
			Faulty:      true,
		},
		ADHDResult: &adhd.DiagnosisResult{
			Problem: "SR-IOV operator failing to reconcile after upgrade",
			Branches: []adhd.Branch{
				{
					FrameID:   "network-engineer",
					FrameName: "Network Engineer",
					Hypotheses: []adhd.Hypothesis{
						{
							ID:        "h1",
							FrameID:   "network-engineer",
							Text:      "NIC firmware incompatible with new driver version",
							Rationale: "Firmware mismatch is common after operator upgrades",
							Evidence:  []string{"dmesg shows firmware timeout", "lspci reports unsupported PF"},
							Score:     adhd.Score{Likelihood: 8.0, Impact: 9.0, Evidence: 7.0, Total: 7.95},
						},
					},
				},
				{
					FrameID:   "etcd-specialist",
					FrameName: "etcd Specialist",
					Hypotheses: []adhd.Hypothesis{
						{
							ID:        "h2",
							FrameID:   "etcd-specialist",
							Text:      "etcd latency causing webhook timeout for config CRD",
							Rationale: "Webhook calls depend on etcd-backed API server responsiveness",
							Evidence:  []string{"etcd slow apply warnings in logs"},
							Score:     adhd.Score{Likelihood: 4.0, Impact: 6.0, Evidence: 3.0, Total: 4.15},
						},
					},
				},
			},
			Shortlist: []adhd.Hypothesis{
				{
					ID:        "h1",
					FrameID:   "network-engineer",
					Text:      "NIC firmware incompatible with new driver version",
					Rationale: "Firmware mismatch is common after operator upgrades",
					Evidence:  []string{"dmesg shows firmware timeout", "lspci reports unsupported PF"},
					Score:     adhd.Score{Likelihood: 8.0, Impact: 9.0, Evidence: 7.0, Total: 7.95},
				},
				{
					ID:        "h2",
					FrameID:   "etcd-specialist",
					Text:      "etcd latency causing webhook timeout for config CRD",
					Rationale: "Webhook calls depend on etcd-backed API server responsiveness",
					Evidence:  []string{"etcd slow apply warnings in logs"},
					Score:     adhd.Score{Likelihood: 4.0, Impact: 6.0, Evidence: 3.0, Total: 4.15},
				},
			},
			Traps: []adhd.Hypothesis{
				{
					ID:      "h3",
					FrameID: "network-engineer",
					Text:    "Pod restarts suggest OOM kill",
					Score:   adhd.Score{Likelihood: 3.0, Impact: 2.0, Evidence: 1.0, Total: 2.05, Trap: "OOM is a symptom not a cause"},
				},
			},
			NonObvious: &adhd.Hypothesis{
				ID:        "h4",
				FrameID:   "etcd-specialist",
				Text:      "Clock skew between nodes causes certificate validation failure",
				Rationale: "NTP drift can silently break mTLS in SR-IOV webhook chain",
				Evidence:  []string{"journalctl shows ntp sync failures on worker-2"},
				Score:     adhd.Score{Likelihood: 3.0, Impact: 8.0, Evidence: 2.0, Total: 3.60},
			},
			Deepened: []adhd.DeepenedHypothesis{
				{
					HypothesisID:    "h1",
					Sketch:          "Compare firmware versions across nodes, correlate with driver compat matrix",
					LoadBearingRisk: "If firmware rollback is needed, it requires node drain and reboot",
					FirstStep:       "Run ethtool -i on all SR-IOV capable NICs and collect firmware versions",
					ChildHypotheses: []adhd.Hypothesis{
						{
							ID:      "h1-1",
							FrameID: "network-engineer",
							Text:    "Only i40e NICs affected, Mellanox NICs work fine",
							Score:   adhd.Score{Likelihood: 6.0},
						},
					},
				},
			},
			Provocation: "What if the SR-IOV operator is working correctly and the real problem is that the cluster network policy changed underneath it?",
		},
	})

	expectedSections := []string{
		"## Divergent Analysis",
		"### Hypothesis Scoring",
		"### Trap Identification",
		"### Evidence Chains",
		"### Non-Obvious Finding",
		"### Technical Deep Dives",
		"### Provocation",
	}

	for _, section := range expectedSections {
		if !strings.Contains(doc.Markdown, section) {
			t.Errorf("missing section: %s", section)
		}
	}
}

func TestGenerateDocumentWithInfraReport(t *testing.T) {
	doc := GenerateDocument(ReportInput{
		Operator: "local-storage-operator",
		OperatorState: mustgather.OperatorState{
			PackageName: "local-storage-operator",
			State:       "AtLatestKnown",
		},
		InfraReport: &healthcheck.Report{
			TotalDimensions: 5,
			Passed:          4,
			Failed:          1,
			Warnings:        0,
			Skipped:         0,
			Dimensions: []healthcheck.DimensionResult{
				{
					Name:     "Disk Pressure",
					Category: "Node",
					Status:   healthcheck.StatusFail,
					Severity: healthcheck.SeverityCritical,
					Summary:  "Node worker-3 has disk pressure",
				},
			},
		},
	})

	if !strings.Contains(doc.Markdown, "## Infrastructure Health Check") {
		t.Error("missing Infrastructure Health Check section")
	}
}

func TestGenerateDocumentDeepAnalysis(t *testing.T) {
	doc := GenerateDocument(ReportInput{
		ClusterName:    "lab-cluster-01",
		Environment:    noise.EnvLab,
		MustGatherPath: "/tmp/mg",
		Operator:       "odf-dependencies",
		Namespace:      "openshift-storage",
		OperatorState: mustgather.OperatorState{
			PackageName:   "odf-dependencies",
			Namespace:     "openshift-storage",
			Channel:       "stable-4.18",
			State:         "",
			Faulty:        true,
			FailureReason: "OLM resolution failed",
		},
		HealthReport: &healthcheck.Report{
			TotalDimensions: 20,
			Passed:          11,
			Failed:          1,
			Warnings:        1,
			Skipped:         7,
			Dimensions: []healthcheck.DimensionResult{
				{ID: healthcheck.DimSubscription, Name: "Subscription State", Category: "OLM", Status: healthcheck.StatusFail, Severity: healthcheck.SeverityCritical, Summary: "OLM resolution failed", Evidence: []string{"constraints not satisfiable: no operators found in package odf-dependencies"}, Recommendation: "Verify package exists in catalog and dependency subscriptions resolve"},
				{ID: healthcheck.DimCSVPhase, Name: "ClusterServiceVersion Phase", Category: "OLM", Status: healthcheck.StatusWarn, Summary: "No installed CSV found"},
			},
		},
		InfraReport: &healthcheck.Report{
			TotalDimensions: 13,
			Passed:          12,
			Failed:          1,
			Dimensions: []healthcheck.DimensionResult{
				{ID: healthcheck.DimClusterVersion, Name: "Cluster Version Status", Category: "Infrastructure", Status: healthcheck.StatusPass, Severity: healthcheck.SeverityHealthy, Summary: "Cluster version 4.18.22 (channel: stable-4.18)"},
				{ID: healthcheck.DimNodeHealth, Name: "Node Health", Category: "Infrastructure", Status: healthcheck.StatusFail, Severity: healthcheck.SeverityCritical, Summary: "1/6 nodes NotReady", Evidence: []string{"worker-3 NotReady"}},
			},
		},
		NoiseReport: &noise.FilterReport{
			Environment:   noise.EnvLab,
			TotalFindings: 2,
			RealIssues:    2,
		},
		RAGContext: &RAGContextData{
			Summary:    "Analysis of odf-dependencies identified a configuration issue",
			Confidence: 0.46,
			SymptomAnalysis: []RAGSymptomEvidence{
				{Symptom: "OLM resolution failed", DimensionID: "catalog_subscription", Relevance: 0.53,
					DocMatches: []RAGDocRef{{Title: "OLM Troubleshooting", Source: "openshift-docs/operators/olm-troubleshooting.adoc", Excerpt: "When OLM cannot resolve operator dependencies, check the CatalogSource and ensure the package exists."}},
				},
			},
			IssueClassification:    "configuration",
			ClassificationEvidence: []string{"2 OLM-layer failures detected", "Subscription channel: stable-4.18"},
			RemediationSteps: []RAGRemediationStep{
				{Step: 1, Priority: "High", Action: "Verify CatalogSource contains odf-dependencies package", Source: "olm-troubleshooting.adoc", Confidence: 0.53},
			},
		},
	})

	sections := []string{
		"## Executive Summary",
		"## Environment",
		"## Observed Behavior",
		"### Failure Chain Analysis",
		"### Root Cause Determination",
		"### Impact Assessment",
		"### Suggested Fix",
		"## Recommended Actions",
	}
	for _, section := range sections {
		if !strings.Contains(doc.Markdown, section) {
			t.Errorf("missing section: %s", section)
		}
	}

	if !strings.Contains(doc.Markdown, "4.18.22") {
		t.Error("missing OCP version in report")
	}
	if !strings.Contains(doc.Markdown, "CONFIGURATION") {
		t.Error("missing classification verdict")
	}
	if !strings.Contains(doc.Markdown, "constraints not satisfiable") {
		t.Error("missing evidence in report")
	}
	if !strings.Contains(doc.Markdown, "configuration") {
		t.Error("missing classification in executive summary")
	}
}

func TestGenerateDocumentNoADHD(t *testing.T) {
	doc := GenerateDocument(ReportInput{
		Operator: "compliance-operator",
		OperatorState: mustgather.OperatorState{
			PackageName: "compliance-operator",
			State:       "AtLatestKnown",
		},
	})

	if strings.Contains(doc.Markdown, "## Divergent Analysis") {
		t.Error("Divergent Analysis section should not appear when ADHDResult is nil")
	}
}
