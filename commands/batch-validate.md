---
description: Batch validate multiple OLM operators from a catalog index with parallel inspection
argument-hint: "--catalog <image> --list <file>"
---

## Name
opm-troubleshooting:batch-validate

## Synopsis
```
/opm-troubleshooting:batch-validate --catalog <catalog-image> --list <operator-list-file>
```

## Description
The `opm-troubleshooting:batch-validate` command validates multiple OLM operators from a single catalog index in batch mode. It renders the catalog once, then resolves and inspects each operator's channel-head bundle in parallel. This is optimized for validating large operator lists (e.g., 125+ Red Hat operators) efficiently.

The command is designed for:
- **Catalog Quality Assurance**: Validate all operators in a catalog build before release
- **Release Validation**: Ensure operator bundles meet metadata requirements (commit SHA, repository URL)
- **Regression Detection**: Detect missing metadata or broken bundle images across catalog updates
- **Catalog Migration Testing**: Verify operators after catalog format migrations

The tool outputs a structured report showing which operators pass validation (`OK`), have partial metadata (`PARTIAL`), or fail inspection (`FAIL`).

## Implementation

The command executes an optimized workflow:

1. **Single Catalog Render**: Renders the catalog index **once** and stores the `declcfg.DeclarativeConfig` in memory
2. **Operator List Parsing**: Reads operator specifications from a text file (format: `package-name channel-name`)
3. **Parallel Validation**: For each operator:
   - Resolves channel head bundle from in-memory catalog config (no re-render)
   - Inspects bundle image metadata (version, commit, URL)
   - Classifies result as OK / PARTIAL / FAIL
4. **Status Reporting**: Outputs results in columnar format with summary statistics

Validation criteria:
- **OK**: Bundle has package, bundle name, version, commit SHA, and repository URL
- **PARTIAL**: Bundle resolves but is missing commit SHA or repository URL
- **FAIL**: Bundle resolution fails, or package/bundle/version metadata is missing

Performance characteristics:
- Single catalog render (1-2 minutes for large catalogs)
- Parallel bundle inspections (~2-5 seconds per operator)
- Total runtime for 125 operators: ~5-8 minutes

## Return Value
- **Claude agent text**: Validation report with status per operator
- **Exit codes**:
  - `0`: All operators validated successfully (OK or PARTIAL)
  - `1`: One or more operators failed validation
  - `2`: Catalog render failed or operator list unreadable

Output format:
```
STATUS PACKAGE                                  CHANNEL                   DETAIL
OK     odf-operator                             stable-4.22               version=v4.22.5 commit=a1b2c3d4
PARTIAL kubernetes-nmstate-operator            stable                    version=v4.22.0 commit="" url=""
FAIL   cluster-logging                          stable                    channel "stable" not found; available: stable-5.9, stable-6.0
```

## Examples

### Example 1: Validate Red Hat v4.22 operator catalog
```bash
/opm-troubleshooting:batch-validate \
  --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
  --list testdata/catalog/operators.json
```

Sample `testdata/catalog/operators.json`:
```
odf-operator stable-4.22
advanced-cluster-management release-2.11
kubernetes-nmstate-operator stable
cluster-logging stable-5.9
local-storage-operator stable
```

Output:
```
Rendering catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 ...
Catalog rendered. Checking 5 operators ...
OK     odf-operator                             stable-4.22               version=v4.22.5 commit=5d1e5bd5
OK     advanced-cluster-management              release-2.11              version=v2.11.2 commit=abc12345
PARTIAL kubernetes-nmstate-operator            stable                    version=v4.22.0 commit="" url=""
OK     cluster-logging                          stable-5.9                version=v5.9.8 commit=def67890
OK     local-storage-operator                   stable                    version=v4.22.0 commit=789abcde

Total: 4 OK, 1 PARTIAL (missing commit or url), 0 FAIL
```

### Example 2: Validate internal Prega catalog
```bash
/opm-troubleshooting:batch-validate \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --list prega-operators.txt
```

### Example 3: Validate with custom timeout
```bash
/opm-troubleshooting:batch-validate \
  --catalog registry.redhat.io/redhat/certified-operator-index:v4.22 \
  --list certified-operators.txt \
  --timeout 30m
```

### Example 4: Export results to file
```bash
/opm-troubleshooting:batch-validate \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --list all-operators.txt > validation-report-$(date +%Y%m%d).txt
```

### Example 5: Filter for failures only
```bash
/opm-troubleshooting:batch-validate \
  --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
  --list operators.txt | grep -E '^(FAIL|PARTIAL)'
```

## Arguments

- `--catalog` (required): OLM catalog index image reference to validate
- `--list` (required): Path to text file containing operator specifications (one per line: `package-name channel-name`)
- `--timeout` (optional): Overall operation timeout (default: 20m)
- `--json` (optional): Output results in JSON format for automation
- `--fail-on-partial` (optional): Exit with code 1 if any operator has PARTIAL status (stricter validation)

## Operator List File Format

The operator list file uses a simple text format:
```
# Lines starting with # are comments
# Format: package-name channel-name

odf-operator stable-4.22
advanced-cluster-management release-2.11
kubernetes-nmstate-operator stable

# Empty lines are ignored
cluster-logging stable-5.9
```

Each line contains:
1. Package name (required)
2. Channel name (required)
3. Fields separated by whitespace

## Environment Variables

- `DOCKER_CONFIG`: Path to directory containing `config.json` for registry authentication
- `REGISTRY_AUTH_FILE`: Alternative registry credentials file
- `CATALOG` (fallback): Default catalog image if `--catalog` is not provided
- `LIST` (fallback): Default operator list file if `--list` is not provided

## Notes

- The catalog is rendered **once** and reused for all operators, significantly reducing runtime
- Operators are validated sequentially (future enhancement: parallel inspection)
- PARTIAL status indicates missing commit/URL metadata, which is common for some operator builds
- Bundle images are pulled on-demand for inspection (requires registry access)
- For private catalogs, ensure `DOCKER_CONFIG` contains valid credentials

## Use Cases

1. **Pre-Release Validation**: Validate catalog builds before promoting to production
2. **Continuous Integration**: Run as part of catalog build CI pipelines
3. **Metadata Auditing**: Identify operators with incomplete source metadata
4. **Catalog Comparison**: Diff validation reports between catalog versions
5. **Regression Testing**: Detect broken operators after catalog regeneration

## Performance Tips

- Use `--timeout` based on operator count (125 operators ≈ 15-20 minutes)
- Run validation from a machine with good network connectivity to the registry
- For very large catalogs (500+ operators), consider splitting the operator list
- Cache Docker credentials in `DOCKER_CONFIG` to avoid auth overhead per bundle

## Troubleshooting

**Common FAIL reasons:**
- Channel not found: Channel name doesn't exist for package (check with `resolve-channel`)
- Package not found: Package name misspelled or not in catalog
- Bundle image pull failure: Network issue or missing registry credentials
- Missing metadata: Bundle image lacks required labels

**PARTIAL status:**
- Bundle image built without Git metadata labels
- CSV repository annotation missing or incomplete
- Labels present but not in expected format

## See Also

- `opm-troubleshooting:inspect-bundle` - Inspect individual operator bundles
- `opm-troubleshooting:resolve-channel` - List available channels for a package
