# Output Format

Agent outputs should follow this structure:

```markdown
## Summary
[1-2 sentence overview]

## Analysis
[Detailed findings with data]

## Root Cause
[Specific diagnosis with references]

## Recommendations
1. [Action item with command/YAML]
2. [Action item with command/YAML]

## References
- [Commit URL]
- [Documentation link]
- [GitHub issue]
```

### Section Guidelines

| Section | Purpose | Length |
|---------|---------|--------|
| **Summary** | What happened, in one breath | 1-2 sentences |
| **Analysis** | Evidence and data that led to diagnosis | As detailed as needed |
| **Root Cause** | The specific thing to fix, with proof | 3-5 sentences |
| **Recommendations** | Actionable next steps, numbered | 2-5 items |
| **References** | Links for verification | Bullet list |
