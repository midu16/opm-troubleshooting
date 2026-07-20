package claudeapi

import (
	"fmt"
	"strings"
)

func buildFaultAnalysisPrompt(req AnalysisRequest) string {
	template := `You are an expert in Kubernetes operators and OLM (Operator Lifecycle Manager).

An operator has failed to install or upgrade in an OpenShift cluster. Your task is to analyze the code changes between two versions and identify which specific changes may have caused the observed failure.

## Operator Information
- **Package**: %s
- **Installed Version**: %s
- **Target Version**: %s

## Observed Failure Symptoms
%s

## Code Changes (Git Diff)
The following code changes occurred between the installed version and the target version:

**Files Changed**: %s

**Diff Summary**:
` + "```diff\n%s\n```" + `

## Your Task
1. Analyze the code changes in the context of the failure symptoms
2. Identify specific code changes that could have caused the observed failure
3. Provide concrete evidence linking code changes to symptoms
4. Rate your confidence (Low/Medium/High) in each potential cause
5. Suggest troubleshooting steps or workarounds

## Output Format
Provide your analysis in the following structure:

**SUMMARY**: [One paragraph summary]

**LIKELY CAUSES**:
- [Cause 1 with evidence from diff]
- [Cause 2 with evidence from diff]
...

**RECOMMENDED ACTIONS**:
- [Action 1]
- [Action 2]
...

**CONFIDENCE**: [Low/Medium/High]
`

	filesChangedStr := strings.Join(req.FilesChanged, ", ")
	if filesChangedStr == "" {
		filesChangedStr = "No files changed (commits may be identical)"
	}

	// Truncate diff if too long (Claude has context limits)
	diff := req.CommitDelta
	if len(diff) > 50000 {
		diff = diff[:50000] + "\n\n[... diff truncated due to length ...]"
	}

	return fmt.Sprintf(template,
		req.OperatorName,
		req.InstalledVersion,
		req.TargetVersion,
		req.FailureSymptoms,
		filesChangedStr,
		diff,
	)
}

func parseAnalysisResponse(text string) *AnalysisResponse {
	// Simple parsing of structured response
	result := &AnalysisResponse{
		RawResponse: text,
	}

	// Extract sections using markers
	if idx := strings.Index(text, "**SUMMARY**:"); idx != -1 {
		end := strings.Index(text[idx:], "\n\n")
		if end != -1 {
			result.Summary = strings.TrimSpace(text[idx+len("**SUMMARY**:") : idx+end])
		}
	}

	if idx := strings.Index(text, "**LIKELY CAUSES**:"); idx != -1 {
		end := strings.Index(text[idx:], "**RECOMMENDED ACTIONS**:")
		if end == -1 {
			end = len(text) - idx
		}
		causeText := text[idx+len("**LIKELY CAUSES**:") : idx+end]
		result.LikelyCauses = parseListItems(causeText)
	}

	if idx := strings.Index(text, "**RECOMMENDED ACTIONS**:"); idx != -1 {
		end := strings.Index(text[idx:], "**CONFIDENCE**:")
		if end == -1 {
			end = len(text) - idx
		}
		actionText := text[idx+len("**RECOMMENDED ACTIONS**:") : idx+end]
		result.RecommendedActions = parseListItems(actionText)
	}

	if idx := strings.Index(text, "**CONFIDENCE**:"); idx != -1 {
		rest := text[idx+len("**CONFIDENCE**:"):]
		lines := strings.Split(rest, "\n")
		if len(lines) > 0 {
			result.Confidence = strings.TrimSpace(lines[0])
		}
	}

	return result
}

func parseListItems(text string) []string {
	lines := strings.Split(text, "\n")
	items := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			items = append(items, strings.TrimSpace(line[1:]))
		}
	}
	return items
}
