package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Engine struct {
	store  VectorStore
	config *Config
}

func NewEngine(cfg *Config) (*Engine, error) {
	embedFunc := NewOllamaEmbedder(cfg.Embedding.URL, cfg.Embedding.Model)

	store, err := NewVectorStore(cfg, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return &Engine{store: store, config: cfg}, nil
}

func (e *Engine) Close() {}

func (e *Engine) Store() VectorStore { return e.store }

func (e *Engine) Config() *Config { return e.config }

func (e *Engine) Troubleshoot(ctx context.Context, operator string, symptoms []string, ocpVersion string) (*TroubleshootResult, error) {
	if ocpVersion == "" {
		ocpVersion = e.config.OpenShift.Version
	}
	query := buildTroubleshootQuery(operator, symptoms, ocpVersion)

	type collResult struct {
		coll Collection
		docs []Document
		err  error
	}

	searches := []struct {
		coll   Collection
		topK   int
		hybrid bool
	}{
		{CollDocs, e.config.Retrieval.DefaultTopK, true},
		{CollCode, e.config.Retrieval.CodeTopK, true},
		{CollTelco, e.config.Retrieval.ConfigTopK, false},
		{CollKnownIssues, e.config.Retrieval.IssuesTopK, false},
		{CollACMDocs, e.config.Retrieval.DefaultTopK, true},
	}

	results := make([]collResult, len(searches))
	var wg sync.WaitGroup

	for i, s := range searches {
		wg.Add(1)
		go func(idx int, coll Collection, topK int, hybrid bool) {
			defer wg.Done()
			var docs []Document
			var err error
			switch {
			case hybrid:
				docs, err = e.hybridRetrieve(ctx, coll, query, topK)
			case coll == CollKnownIssues && ocpVersion != "":
				docs, err = e.store.SearchWithFilter(ctx, coll, query, topK,
					map[string]string{"ocp_version": ocpVersion})
			default:
				docs, err = e.store.Search(ctx, coll, query, topK)
			}
			results[idx] = collResult{coll: coll, docs: docs, err: err}
		}(i, s.coll, s.topK, s.hybrid)
	}
	wg.Wait()

	tr := &TroubleshootResult{}
	var summaryParts []string
	totalDocs := 0

	for _, r := range results {
		if r.err != nil || len(r.docs) == 0 {
			continue
		}
		totalDocs += len(r.docs)

		switch r.coll {
		case CollDocs:
			for _, d := range r.docs {
				tr.DocumentationRefs = append(tr.DocumentationRefs, DocReference{
					Title:   metaOrDefault(d.Metadata, "section", "OCP Documentation"),
					Source:  metaOrDefault(d.Metadata, "source", "docs.redhat.com"),
					Excerpt: truncate(d.Content, 300),
					URL:     metaOrDefault(d.Metadata, "url", ""),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d documentation references", len(r.docs)))

		case CollCode:
			for _, d := range r.docs {
				tr.DocumentationRefs = append(tr.DocumentationRefs, DocReference{
					Title:   metaOrDefault(d.Metadata, "declaration", "Source code"),
					Source:  metaOrDefault(d.Metadata, "source", "openshift source"),
					Excerpt: truncate(d.Content, 300),
					URL:     metaOrDefault(d.Metadata, "repo_url", ""),
				})
			}

		case CollTelco:
			for _, d := range r.docs {
				tr.ConfigAdvice = append(tr.ConfigAdvice, ConfigAdvice{
					Component: metaOrDefault(d.Metadata, "k8s_kind", "Configuration"),
					Reference: metaOrDefault(d.Metadata, "source", "telco-reference"),
					Advice:    truncate(d.Content, 500),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d configuration references", len(r.docs)))

		case CollKnownIssues:
			for _, d := range r.docs {
				tr.KnownIssues = append(tr.KnownIssues, KnownIssue{
					ID:         metaOrDefault(d.Metadata, "issue_id", ""),
					Summary:    truncate(d.Content, 300),
					Workaround: metaOrDefault(d.Metadata, "workaround", ""),
					FixVersion: metaOrDefault(d.Metadata, "fix_version", ""),
				})
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%d known issues", len(r.docs)))

		case CollACMDocs:
			for _, d := range r.docs {
				tr.DocumentationRefs = append(tr.DocumentationRefs, DocReference{
					Title:   metaOrDefault(d.Metadata, "section", "ACM/MCE Documentation"),
					Source:  metaOrDefault(d.Metadata, "source", "rhacm-docs"),
					Excerpt: truncate(d.Content, 300),
				})
			}
			if len(r.docs) > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("%d ACM/MCE documentation references", len(r.docs)))
			}
		}
	}

	if totalDocs > 0 {
		tr.Summary = fmt.Sprintf("RAG analysis for %s: found %s", operator, strings.Join(summaryParts, ", "))
		tr.Confidence = computeConfidence(totalDocs, len(tr.KnownIssues))
	} else {
		tr.Summary = fmt.Sprintf("No RAG data found for %s — run ocp-rag-ingest to populate the knowledge base", operator)
		tr.Confidence = 0
	}

	return tr, nil
}

func (e *Engine) SearchDocs(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.hybridRetrieve(ctx, CollDocs, query, e.config.Retrieval.DefaultTopK)
	if err != nil {
		return nil, err
	}
	acmDocs, err := e.hybridRetrieve(ctx, CollACMDocs, query, e.config.Retrieval.DefaultTopK/2)
	if err == nil {
		docs = append(docs, acmDocs...)
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchACMDocs(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.hybridRetrieve(ctx, CollACMDocs, query, e.config.Retrieval.DefaultTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchCode(ctx context.Context, query, operator string) (*SearchResult, error) {
	q := query
	if operator != "" {
		q = operator + " " + query
	}
	docs, err := e.hybridRetrieve(ctx, CollCode, q, e.config.Retrieval.CodeTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchTelcoConfigs(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.store.Search(ctx, CollTelco, query, e.config.Retrieval.ConfigTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchKnownIssues(ctx context.Context, operator, version string) (*SearchResult, error) {
	query := operator
	if query == "" {
		query = "OpenShift known issues"
	}

	var docs []Document
	var err error
	if version != "" {
		docs, err = e.store.SearchWithFilter(ctx, CollKnownIssues, query, e.config.Retrieval.IssuesTopK,
			map[string]string{"ocp_version": version})
	} else {
		docs, err = e.store.Search(ctx, CollKnownIssues, query, e.config.Retrieval.IssuesTopK)
	}
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) SearchManifests(ctx context.Context, query string) (*SearchResult, error) {
	docs, err := e.store.Search(ctx, CollManifests, query, e.config.Retrieval.ConfigTopK)
	if err != nil {
		return nil, err
	}
	return docsToSearchResult(query, docs), nil
}

func (e *Engine) CheckFreshness() (*FreshnessStatus, error) {
	return CheckFreshness(e.config.DataDir, e.config.Freshness.MetaFile)
}

func (e *Engine) DeepTroubleshoot(ctx context.Context, input DeepTroubleshootInput) (*TroubleshootResult, error) {
	if input.OCPVersion == "" {
		input.OCPVersion = e.config.OpenShift.Version
	}

	tr := &TroubleshootResult{}

	// Phase 1: symptom-targeted search — one per failed/warned dimension, capped at 8.
	dims := input.FailedDimensions
	if len(dims) > 8 {
		dims = dims[:8]
	}

	type symptomResult struct {
		idx int
		se  SymptomEvidence
	}
	symCh := make(chan symptomResult, len(dims))
	var wg sync.WaitGroup
	for i, dim := range dims {
		wg.Add(1)
		go func(idx int, d DimensionSymptom) {
			defer wg.Done()
			se := e.searchForSymptom(ctx, d, input.Operator, input.OCPVersion)
			symCh <- symptomResult{idx: idx, se: se}
		}(i, dim)
	}
	wg.Wait()
	close(symCh)

	symResults := make([]SymptomEvidence, len(dims))
	for sr := range symCh {
		symResults[sr.idx] = sr.se
	}
	tr.SymptomAnalysis = symResults

	// Phase 2: operator-specific code search from pod symptoms.
	tr.RelevantCodePaths = e.searchOperatorCode(ctx, input)

	// Phase 3: configuration validation from telco configs + manifests.
	configAdvice := e.searchConfigEvidence(ctx, input.Operator)
	tr.ConfigAdvice = configAdvice

	// Collect all doc refs and known issues from symptom results for the summary.
	seen := make(map[string]bool)
	for _, se := range symResults {
		for _, d := range se.DocMatches {
			key := d.Source + "|" + d.Title
			if !seen[key] {
				tr.DocumentationRefs = append(tr.DocumentationRefs, d)
				seen[key] = true
			}
		}
		for _, d := range se.CodeMatches {
			key := d.Source + "|" + d.Title
			if !seen[key] {
				tr.DocumentationRefs = append(tr.DocumentationRefs, d)
				seen[key] = true
			}
		}
		for _, ki := range se.KnownIssues {
			if !seen["ki:"+ki.ID] {
				tr.KnownIssues = append(tr.KnownIssues, ki)
				seen["ki:"+ki.ID] = true
			}
		}
	}

	// Phase 4: classify the issue.
	tr.IssueClassification, tr.ClassificationEvidence = classifyIssue(input, symResults, tr.RelevantCodePaths, configAdvice)

	// Phase 5: build remediation steps.
	tr.RemediationSteps = buildRemediationSteps(symResults, tr.KnownIssues, configAdvice)

	// Compute confidence and summary.
	tr.Confidence = computeDeepConfidence(tr)

	var summaryParts []string
	if len(tr.DocumentationRefs) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d documentation references", len(tr.DocumentationRefs)))
	}
	if len(tr.KnownIssues) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d known issues", len(tr.KnownIssues)))
	}
	if len(tr.ConfigAdvice) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d configuration references", len(tr.ConfigAdvice)))
	}
	if len(tr.RelevantCodePaths) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d code paths", len(tr.RelevantCodePaths)))
	}
	if len(summaryParts) > 0 {
		if tr.IssueClassification != "" && tr.IssueClassification != "unknown" {
			topEvidence := ""
			if len(input.FailedDimensions) > 0 {
				topEvidence = fmt.Sprintf(" Primary symptom: %s.", input.FailedDimensions[0].Summary)
			}
			tr.Summary = fmt.Sprintf("Analysis of %s identified a %s issue with %s.%s",
				input.Operator, tr.IssueClassification, strings.Join(summaryParts, ", "), topEvidence)
		} else {
			tr.Summary = fmt.Sprintf("Analysis of %s found %s but could not determine a definitive root cause classification.",
				input.Operator, strings.Join(summaryParts, ", "))
		}
	} else {
		tr.Summary = fmt.Sprintf("No RAG data found for %s — run ocp-rag-ingest to populate the knowledge base.", input.Operator)
	}

	return tr, nil
}

func (e *Engine) searchForSymptom(ctx context.Context, dim DimensionSymptom, operator, version string) SymptomEvidence {
	query := buildDimensionQuery(dim, operator, version)
	se := SymptomEvidence{
		Symptom:     dim.Summary,
		DimensionID: dim.DimensionID,
		Query:       query,
	}

	type searchResult struct {
		kind string
		docs []Document
	}
	ch := make(chan searchResult, 4)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		docs, err := e.hybridRetrieve(ctx, CollDocs, query, e.config.Retrieval.DefaultTopK)
		if err == nil {
			ch <- searchResult{kind: "docs", docs: docs}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		codeQuery := operator + " " + query
		docs, err := e.hybridRetrieve(ctx, CollCode, codeQuery, e.config.Retrieval.CodeTopK)
		if err == nil {
			ch <- searchResult{kind: "code", docs: docs}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var docs []Document
		var err error
		if version != "" {
			docs, err = e.store.SearchWithFilter(ctx, CollKnownIssues, query,
				e.config.Retrieval.IssuesTopK, map[string]string{"ocp_version": version})
		} else {
			docs, err = e.store.Search(ctx, CollKnownIssues, query, e.config.Retrieval.IssuesTopK)
		}
		if err == nil {
			ch <- searchResult{kind: "issues", docs: docs}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		docs, err := e.hybridRetrieve(ctx, CollACMDocs, query, e.config.Retrieval.DefaultTopK/2)
		if err == nil {
			ch <- searchResult{kind: "docs", docs: docs}
		}
	}()

	wg.Wait()
	close(ch)

	totalMatches := 0
	for sr := range ch {
		switch sr.kind {
		case "docs":
			for _, d := range sr.docs {
				se.DocMatches = append(se.DocMatches, DocReference{
					Title:   metaOrDefault(d.Metadata, "section", metaOrDefault(d.Metadata, "breadcrumb", "OCP Documentation")),
					Source:  metaOrDefault(d.Metadata, "source", "docs.redhat.com"),
					Excerpt: truncate(d.Content, 500),
					URL:     metaOrDefault(d.Metadata, "url", ""),
				})
			}
			totalMatches += len(sr.docs)
		case "code":
			for _, d := range sr.docs {
				se.CodeMatches = append(se.CodeMatches, DocReference{
					Title:   metaOrDefault(d.Metadata, "declaration", "Source code"),
					Source:  metaOrDefault(d.Metadata, "source", "openshift source"),
					Excerpt: truncate(d.Content, 500),
					URL:     metaOrDefault(d.Metadata, "repo_url", ""),
				})
			}
			totalMatches += len(sr.docs)
		case "issues":
			for _, d := range sr.docs {
				se.KnownIssues = append(se.KnownIssues, KnownIssue{
					ID:         metaOrDefault(d.Metadata, "issue_id", ""),
					Summary:    truncate(d.Content, 500),
					Workaround: metaOrDefault(d.Metadata, "workaround", ""),
					FixVersion: metaOrDefault(d.Metadata, "fix_version", ""),
				})
			}
			totalMatches += len(sr.docs)
		}
	}

	if totalMatches > 0 {
		channels := 0
		if len(se.DocMatches) > 0 {
			channels++
		}
		if len(se.CodeMatches) > 0 {
			channels++
		}
		if len(se.KnownIssues) > 0 {
			channels++
		}
		se.Relevance = float64(channels) / 3.0
		docBoost := float64(totalMatches) / 10.0
		if docBoost > 0.3 {
			docBoost = 0.3
		}
		se.Relevance = se.Relevance*0.7 + docBoost
		if se.Relevance > 1.0 {
			se.Relevance = 1.0
		}
	}

	return se
}

func (e *Engine) searchOperatorCode(ctx context.Context, input DeepTroubleshootInput) []CodePathEvidence {
	var queries []string

	for _, pod := range input.UnhealthyPods {
		switch {
		case pod.TerminatedReason == "OOMKilled":
			queries = append(queries, input.Operator+" memory resource limits OOMKilled")
		case pod.WaitingReason == "CrashLoopBackOff":
			queries = append(queries, input.Operator+" reconcile error handling crash recovery")
		case pod.WaitingReason == "CreateContainerConfigError":
			queries = append(queries, input.Operator+" container configuration secret configmap mount")
		case pod.WaitingReason == "ImagePullBackOff" || pod.WaitingReason == "ErrImagePull":
			queries = append(queries, input.Operator+" image registry pull mirror IDMS")
		case pod.WaitingReason != "":
			queries = append(queries, input.Operator+" "+pod.WaitingReason+" error handling")
		case pod.Phase == "Pending":
			queries = append(queries, input.Operator+" scheduling node affinity tolerations")
		}
	}

	if input.FailureReason != "" {
		queries = append(queries, input.Operator+" "+input.FailureReason)
	}

	if len(queries) == 0 {
		queries = append(queries, input.Operator+" reconcile controller operator lifecycle")
	}

	if len(queries) > 4 {
		queries = queries[:4]
	}

	seen := make(map[string]bool)
	var results []CodePathEvidence
	for _, q := range queries {
		docs, err := e.hybridRetrieve(ctx, CollCode, q, 3)
		if err != nil {
			continue
		}
		for _, d := range docs {
			key := d.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, CodePathEvidence{
				Declaration: metaOrDefault(d.Metadata, "declaration", ""),
				FilePath:    metaOrDefault(d.Metadata, "source", ""),
				Repo:        metaOrDefault(d.Metadata, "repo", ""),
				RepoURL:     metaOrDefault(d.Metadata, "repo_url", ""),
				Excerpt:     truncate(d.Content, 400),
				Relevance:   q,
			})
		}
	}

	if len(results) > 10 {
		results = results[:10]
	}
	return results
}

func (e *Engine) searchConfigEvidence(ctx context.Context, operator string) []ConfigAdvice {
	telcoDocs, err := e.store.Search(ctx, CollTelco, operator+" configuration", e.config.Retrieval.ConfigTopK)
	if err != nil {
		telcoDocs = nil
	}

	manifestDocs, err := e.store.Search(ctx, CollManifests, operator+" CRD deployment", e.config.Retrieval.ConfigTopK)
	if err != nil {
		manifestDocs = nil
	}

	advice := make([]ConfigAdvice, 0, len(telcoDocs)+len(manifestDocs))
	for _, d := range telcoDocs {
		advice = append(advice, ConfigAdvice{
			Component: metaOrDefault(d.Metadata, "k8s_kind", "Configuration"),
			Reference: metaOrDefault(d.Metadata, "source", "telco-reference"),
			Advice:    truncate(d.Content, 500),
		})
	}
	for _, d := range manifestDocs {
		advice = append(advice, ConfigAdvice{
			Component: metaOrDefault(d.Metadata, "k8s_kind", "Manifest"),
			Reference: metaOrDefault(d.Metadata, "source", "operator-manifest"),
			Advice:    truncate(d.Content, 500),
		})
	}
	return advice
}

func buildDimensionQuery(dim DimensionSymptom, operator, version string) string {
	base := "OpenShift " + version + " " + operator + " "

	evidenceHint := ""
	if len(dim.Evidence) > 0 {
		hints := dim.Evidence
		if len(hints) > 2 {
			hints = hints[:2]
		}
		evidenceHint = " " + strings.Join(hints, " ")
	}

	switch dim.DimensionID {
	case "catalog_source":
		return base + "CatalogSource connectivity health GRPC" + evidenceHint
	case "catalog_subscription":
		return base + "OLM Subscription ResolutionFailed dependency resolution" + evidenceHint
	case "install_plan":
		return base + "InstallPlan phase failed approval" + evidenceHint
	case "operator_group":
		return base + "OperatorGroup configuration namespace target" + evidenceHint
	case "csv_phase":
		return base + "ClusterServiceVersion CSV phase installing failed" + evidenceHint
	case "csv_requirements":
		return base + "CSV requirements CRD dependency unmet" + evidenceHint
	case "deployment_ready":
		return base + "Deployment replicas unavailable progressing timeout" + evidenceHint
	case "pod_health":
		return base + "pod unhealthy CrashLoopBackOff Pending scheduling" + evidenceHint
	case "container_restarts":
		return base + "container restart backoff OOMKilled error" + evidenceHint
	case "image_pull":
		return base + "ImagePullBackOff ErrImagePull registry mirror disconnected" + evidenceHint
	case "warning_events":
		return base + "warning events FailedScheduling BackOff" + evidenceHint
	case "crd_established":
		return base + "CRD CustomResourceDefinition missing established" + evidenceHint
	case "node_scheduling":
		return base + "node scheduling constraints affinity toleration taint" + evidenceHint
	case "node_health":
		return base + "node NotReady MemoryPressure DiskPressure conditions" + evidenceHint
	case "etcd_health":
		return base + "etcd cluster health leader election degraded" + evidenceHint
	case "pv_health":
		return base + "PersistentVolume PV failed released storage" + evidenceHint
	case "mcp_health":
		return base + "MachineConfigPool degraded updating machine config" + evidenceHint
	default:
		return base + dim.Name + " troubleshooting" + evidenceHint
	}
}

func classifyIssue(input DeepTroubleshootInput, symptoms []SymptomEvidence, codePaths []CodePathEvidence, configAdvice []ConfigAdvice) (cls string, ev []string) {
	var evidence []string

	olmFailures := 0
	infraFailures := 0
	workloadFailures := 0
	hasCodeBugKI := false

	for _, dim := range input.FailedDimensions {
		switch dim.Category {
		case "OLM":
			olmFailures++
		case "Workload":
			workloadFailures++
		}
	}
	for _, dim := range input.InfraFailures {
		if dim.Status == "Fail" {
			infraFailures++
		}
	}

	for _, se := range symptoms {
		for _, ki := range se.KnownIssues {
			if ki.ID != "" {
				hasCodeBugKI = true
				evidence = append(evidence, fmt.Sprintf("Known issue %s matches symptoms: %s", ki.ID, truncate(ki.Summary, 80)))
			}
		}
	}

	if infraFailures > 0 {
		for _, dim := range input.InfraFailures {
			if dim.Status == "Fail" {
				evidence = append(evidence, fmt.Sprintf("Infrastructure failure: %s — %s", dim.Name, dim.Summary))
			}
		}
		if workloadFailures > 0 && olmFailures == 0 {
			evidence = append(evidence, "Workload failures correlate with infrastructure degradation")
			return "infrastructure", evidence
		}
		if workloadFailures > 0 {
			evidence = append(evidence, "Both infrastructure and OLM issues present — infrastructure may be contributing factor")
		}
	}

	if hasCodeBugKI {
		evidence = append(evidence, "Known issue database contains matching code-level bug")
		if len(codePaths) > 0 {
			evidence = append(evidence, fmt.Sprintf("Found %d relevant operator code paths", len(codePaths)))
		}
		return "code", evidence
	}

	if olmFailures > 0 {
		evidence = append(evidence, fmt.Sprintf("%d OLM-layer failures detected (subscription/CSV/installplan)", olmFailures))
		if len(configAdvice) > 0 {
			evidence = append(evidence, fmt.Sprintf("%d telco/manifest configuration references available", len(configAdvice)))
		}
		if input.SubscriptionState != "" && input.SubscriptionState != "AtLatestKnown" {
			evidence = append(evidence, fmt.Sprintf("Subscription state '%s' indicates OLM resolution problem", input.SubscriptionState))
		}
		if input.Channel != "" {
			evidence = append(evidence, fmt.Sprintf("Subscription channel: %s", input.Channel))
		}
		return "configuration", evidence
	}

	if workloadFailures > 0 && len(codePaths) > 0 {
		evidence = append(evidence, fmt.Sprintf("%d workload failures with %d relevant code paths identified", workloadFailures, len(codePaths)))
		return "code", evidence
	}

	if len(configAdvice) > 0 {
		evidence = append(evidence, "Configuration references found but no clear failure pattern match")
		return "configuration", evidence
	}

	evidence = append(evidence, "Insufficient evidence to determine root cause category")
	return "unknown", evidence
}

func buildRemediationSteps(symptoms []SymptomEvidence, knownIssues []KnownIssue, configAdvice []ConfigAdvice) []RemediationStep {
	steps := make([]RemediationStep, 0, len(knownIssues)+len(symptoms)+len(configAdvice))
	stepNum := 1

	for _, ki := range knownIssues {
		if ki.Workaround != "" {
			steps = append(steps, RemediationStep{
				Step:       stepNum,
				Priority:   "Critical",
				Action:     fmt.Sprintf("[%s] %s", ki.ID, ki.Workaround),
				Source:     fmt.Sprintf("Known Issue %s (fix in %s)", ki.ID, ki.FixVersion),
				Confidence: 0.95,
			})
			stepNum++
		}
	}

	for _, se := range symptoms {
		if len(se.DocMatches) > 0 && se.Relevance > 0.3 {
			best := se.DocMatches[0]
			steps = append(steps, RemediationStep{
				Step:       stepNum,
				Priority:   "High",
				Action:     fmt.Sprintf("Review: %s — %s", best.Title, truncate(best.Excerpt, 200)),
				Source:     best.Source,
				Confidence: se.Relevance,
			})
			stepNum++
		}
	}

	for _, ca := range configAdvice {
		steps = append(steps, RemediationStep{
			Step:       stepNum,
			Priority:   "Medium",
			Action:     fmt.Sprintf("Validate %s configuration: %s", ca.Component, truncate(ca.Advice, 200)),
			Source:     ca.Reference,
			Confidence: 0.7,
		})
		stepNum++
		if stepNum > 10 {
			break
		}
	}

	return steps
}

func computeDeepConfidence(result *TroubleshootResult) float64 {
	c := 0.0

	// Factor 1: base document coverage (max 0.20)
	docCount := len(result.DocumentationRefs)
	switch {
	case docCount > 10:
		c += 0.20
	case docCount > 6:
		c += 0.18
	case docCount > 3:
		c += 0.14
	case docCount > 0:
		c += 0.08
	}

	// Factor 2: known issues matched (max 0.20)
	if len(result.KnownIssues) > 0 {
		c += 0.20
	}

	// Factor 3: symptom-specific match quality (max 0.25)
	if len(result.SymptomAnalysis) > 0 {
		totalRelevance := 0.0
		matchCount := 0
		for _, se := range result.SymptomAnalysis {
			if len(se.DocMatches) > 0 || len(se.CodeMatches) > 0 || len(se.KnownIssues) > 0 {
				totalRelevance += se.Relevance
				matchCount++
			}
		}
		if matchCount > 0 {
			avgRelevance := totalRelevance / float64(matchCount)
			matchRatio := float64(matchCount) / float64(len(result.SymptomAnalysis))
			symptomScore := 0.25 * (0.6*avgRelevance + 0.4*matchRatio)
			c += symptomScore
		}
	}

	// Factor 4: code path relevance (max 0.15)
	switch {
	case len(result.RelevantCodePaths) >= 3:
		c += 0.15
	case len(result.RelevantCodePaths) > 0:
		c += 0.08
	}

	// Factor 5: config/manifest match (max 0.10)
	switch {
	case len(result.ConfigAdvice) >= 3:
		c += 0.10
	case len(result.ConfigAdvice) > 0:
		c += 0.06
	}

	// Factor 6: classification strength (max 0.10)
	if result.IssueClassification != "" && result.IssueClassification != "unknown" {
		c += 0.06
		if len(result.ClassificationEvidence) >= 2 {
			c += 0.04
		}
	}

	if c > 1.0 {
		c = 1.0
	}
	return c
}

func buildTroubleshootQuery(operator string, symptoms []string, ocpVersion string) string {
	parts := []string{"OpenShift " + ocpVersion + " operator " + operator + " troubleshooting"}
	for i, s := range symptoms {
		if i >= 5 {
			break
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

func computeConfidence(totalDocs, knownIssueCount int) float64 {
	c := 0.0
	if totalDocs > 0 {
		c = 0.3
	}
	if totalDocs > 3 {
		c = 0.5
	}
	if totalDocs > 6 {
		c = 0.65
	}
	if knownIssueCount > 0 {
		c += 0.2
	}
	if c > 1.0 {
		c = 1.0
	}
	return c
}

func docsToSearchResult(query string, docs []Document) *SearchResult {
	sr := &SearchResult{Query: query}
	for _, d := range docs {
		sr.Documents = append(sr.Documents, DocReference{
			Title:   metaOrDefault(d.Metadata, "section", metaOrDefault(d.Metadata, "declaration", "Document")),
			Source:  metaOrDefault(d.Metadata, "source", "unknown"),
			Excerpt: truncate(d.Content, 500),
			URL:     metaOrDefault(d.Metadata, "url", ""),
		})
	}
	return sr
}

func metaOrDefault(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
