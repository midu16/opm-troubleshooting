package ingest

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// defaultSkipExtensions is the fallback list when SecretConfig is nil.
var defaultSkipExtensions = []string{
	".pem", ".crt", ".key", ".p12", ".jks", ".pfx", ".enc", ".gpg",
	".keystore", ".truststore",
}

// defaultSkipFilenames is the fallback list when SecretConfig is nil.
var defaultSkipFilenames = []string{
	".env", "pull-secret", "kubeconfig", "credentials", "htpasswd",
	"id_rsa", "id_ed25519", "cloud-credentials", "auth-token",
}

// Regex patterns for detecting sensitive values in content.
var (
	rePassword   = regexp.MustCompile(`(?i)(password|passwd|secret)\s*[:=]\s*\S+`)
	reToken      = regexp.MustCompile(`(?i)(token|api[_-]?key)\s*[:=]\s*\S+`)
	reBearer     = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	reBase64Blob = regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`)
	reSSHKey     = regexp.MustCompile(`(?s)-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----.*?-----END (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)
	reK8sSecret  = regexp.MustCompile(`(?m)^kind:\s*(?:Secret|SealedSecret)\s*$`)
)

// ShouldSkipFile returns true if the file at the given path should be
// entirely skipped based on its extension or filename. When cfg is nil
// the built-in defaults are used.
func ShouldSkipFile(path string, cfg *rag.SecretConfig) bool {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	skipExts := defaultSkipExtensions
	skipNames := defaultSkipFilenames
	if cfg != nil {
		if len(cfg.SkipExtensions) > 0 {
			skipExts = cfg.SkipExtensions
		}
		if len(cfg.SkipFilenames) > 0 {
			skipNames = cfg.SkipFilenames
		}
	}

	for _, e := range skipExts {
		if ext == strings.ToLower(e) {
			return true
		}
	}
	for _, n := range skipNames {
		if base == strings.ToLower(n) {
			return true
		}
	}
	return false
}

// FilterAndRedact inspects content for secrets and credentials. It returns
// the (possibly redacted) content and a boolean indicating whether the file
// should be skipped entirely.
//
// Three tiers:
//  1. Skip: K8s Secret or SealedSecret resources are dropped completely.
//  2. Redact: passwords, tokens, Bearer strings, base64 blobs >= 40 chars,
//     and SSH private key blocks are replaced with placeholder text.
//  3. Patterns are applied via compiled regular expressions.
func FilterAndRedact(path, content string) (string, bool) {
	// Tier 1 — skip K8s Secret / SealedSecret resources entirely.
	if reK8sSecret.MatchString(content) {
		return "", true
	}

	redacted := content

	// Tier 2 — redact SSH private keys first (multi-line).
	redacted = reSSHKey.ReplaceAllString(redacted, "[REDACTED-SSH-KEY]")

	// Tier 3 — single-line regex redactions.
	redacted = rePassword.ReplaceAllString(redacted, "[REDACTED-PASSWORD]")
	redacted = reToken.ReplaceAllString(redacted, "[REDACTED-TOKEN]")
	redacted = reBearer.ReplaceAllString(redacted, "Bearer [REDACTED]")
	redacted = reBase64Blob.ReplaceAllString(redacted, "[REDACTED-BASE64]")

	return redacted, false
}
