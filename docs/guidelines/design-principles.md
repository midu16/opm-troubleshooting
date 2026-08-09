# Design Principles

## Using AI Reasoning

Agents should apply AI analysis for:

- **Pattern Recognition**: Identify common failure modes (e.g., "channel X.Y always missing commit labels")
- **Context Synthesis**: Combine catalog metadata with external docs (operator repos, Red Hat docs)
- **Error Interpretation**: Translate technical errors into actionable user guidance
- **Recommendation Ranking**: Prioritize fixes by impact and effort

## Avoiding Pure Automation

Do NOT create agents for simple command wrappers. These should be direct commands:

- ❌ **Bad**: `/opm-troubleshooting:inspect` that just calls `inspect-bundle` with no analysis
- ✅ **Good**: `/opm-troubleshooting:analyze-metadata` that interprets missing fields and suggests fixes

**Guideline**: An agent is justified when it does at least one of the following beyond a single tool call:
1. Synthesizes data from multiple sources (catalog + upstream repo + docs)
2. Applies conditional branching based on inspection results
3. Generates actionable recommendations with specific commands or YAML changes

## Error Handling

Agents must gracefully handle:

- **Network failures**: Suggest retry with different registry mirrors
- **Authentication errors**: Guide user to set `DOCKER_CONFIG`
- **Missing channels**: List available channels and suggest alternatives
- **Timeout issues**: Recommend increasing `--timeout` or splitting work
