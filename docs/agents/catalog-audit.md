# Catalog Quality Auditor

**Purpose**: Audit entire catalog for metadata quality, missing bundles, and broken channels.

## Workflow

```
1. Run batch-validate on operator list
2. Classify failures: FAIL vs PARTIAL
3. For FAIL: identify root cause (channel missing, bundle pull failure)
4. For PARTIAL: suggest metadata improvements
5. Generate quality report with priority ranking
```

## Invocation

```bash
/opm-troubleshooting:audit-catalog \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --list operators.txt
```

## Agent Prompt Template

```
You are auditing an OLM catalog for quality and completeness.

Context:
- Catalog: {catalog}
- Operator count: {count}

Task:
1. Run batch-validate on all operators
2. Categorize results:
   - Critical failures: bundle resolution failed
   - Metadata gaps: missing commit/url (PARTIAL)
   - Success: complete metadata (OK)
3. For failures, run individual inspect-bundle to get detailed errors
4. Prioritize fixes:
   - P0: FAIL status (blocks subscription)
   - P1: PARTIAL status with missing commit (affects debugging)
   - P2: PARTIAL status with missing URL (cosmetic)

Output:
- Summary: X OK, Y PARTIAL, Z FAIL
- Critical failures: [list with error details]
- Metadata gaps: [list with recommended labels]
- Priority ranking: [P0/P1/P2 breakdown]
```
