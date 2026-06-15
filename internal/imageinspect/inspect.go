package imageinspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/midu16/opm-troubleshooting/internal/bundlecsv"
)

const (
	labelBundlePackage = "operators.operatorframework.io.bundle.package.v1"
	labelVersion       = "version"

	// OpenShift build-system labels (common on in-house / Prega Quay builds).
	labelBuildCommitID       = "io.openshift.build.commit.id"
	labelBuildCommitURL      = "io.openshift.build.commit.url"
	labelBuildSourceLocation = "io.openshift.build.source-location"
	labelSourceLocation      = "source-location"

	// OCI / Red Hat Konflux labels (common on registry.redhat.io operator bundles).
	labelOCIRevision = "org.opencontainers.image.revision"
	labelOCISource   = "org.opencontainers.image.source"
	labelVCSRef      = "vcs-ref"
	labelUpstreamRef = "upstream-vcs-ref"
	labelVCSType     = "vcs-type"
	labelGenericURL  = "url"
)

// commitLabelKeys is ordered: prefer OpenShift build metadata, then OCI/VCS fallbacks.
var commitLabelKeys = []string{
	labelBuildCommitID,
	labelOCIRevision,
	labelVCSRef,
	labelUpstreamRef,
}

// urlLabelKeys holds direct commit/source URL labels (repo URLs are resolved separately).
var urlLabelKeys = []string{
	labelBuildCommitURL,
}

// BundleInfo holds bundle image metadata extracted from registry labels.
type BundleInfo struct {
	Package string
	Bundle  string
	Version string
	Commit  string
	URL     string
}

// InspectBundle fetches image config labels for the given image reference.
// This is the native Go equivalent of: skopeo inspect --format '{{json .Labels}}' <image>
func InspectBundle(ctx context.Context, imageRef string) (*BundleInfo, error) {
	return inspectBundle(ctx, imageRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func inspectBundle(ctx context.Context, imageRef string, opts ...remote.Option) (*BundleInfo, error) {
	ref, err := name.ParseReference(imageRef, name.Insecure)
	if err != nil {
		return nil, fmt.Errorf("parse image reference %q: %w", imageRef, err)
	}

	allOpts := append(opts, remote.WithContext(ctx))
	img, err := remote.Image(ref, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("pull image %q: %w", imageRef, err)
	}

	cfgFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("read image config %q: %w", imageRef, err)
	}

	bundleName, err := bundleDisplayName(ref, img)
	if err != nil {
		return nil, err
	}

	labels := cfgFile.Config.Labels
	commit := firstLabel(labels, commitLabelKeys)
	url := resolveSourceURL(labels, commit, img)

	return &BundleInfo{
		Package: labelValue(labels, labelBundlePackage),
		Bundle:  bundleName,
		Version: labelValue(labels, labelVersion),
		Commit:  commit,
		URL:     url,
	}, nil
}

func bundleDisplayName(ref name.Reference, img v1.Image) (string, error) {
	if tagged, ok := ref.(name.Tag); ok {
		return tagged.String(), nil
	}

	digest, err := img.Digest()
	if err != nil {
		return ref.String(), nil
	}
	return ref.Context().String() + "@" + digest.String(), nil
}

func labelValue(labels map[string]string, key string) string {
	if labels == nil {
		return ""
	}
	return labels[key]
}

func firstLabel(labels map[string]string, keys []string) string {
	for _, key := range keys {
		if v := labelValue(labels, key); v != "" {
			return v
		}
	}
	return ""
}

func isLikelyGitRepoURL(u string) bool {
	u = strings.TrimSpace(strings.ToLower(u))
	if u == "" {
		return false
	}
	if strings.Contains(u, "catalog.redhat.com") || strings.Contains(u, "access.redhat.com") {
		return false
	}
	return strings.Contains(u, "github.com") ||
		strings.Contains(u, "gitlab.com") ||
		strings.HasPrefix(u, "git@") ||
		strings.HasSuffix(u, ".git") ||
		strings.HasPrefix(u, "github.com/")
}

func deriveGitCommitURL(labels map[string]string, commit string) string {
	return deriveGitCommitURLFromRepo(firstRepoURL(labels), commit, labelValue(labels, labelVCSType))
}

func resolveSourceURL(labels map[string]string, commit string, img v1.Image) string {
	if u := firstLabel(labels, urlLabelKeys); u != "" && strings.Contains(u, "/commit/") {
		return u
	}

	if u := deriveGitCommitURL(labels, commit); u != "" {
		return u
	}

	if repos, err := bundlecsv.RepositoryURLs(img, labelValue(labels, labelBundlePackage)); err == nil {
		for _, repo := range repos {
			if u := deriveGitCommitURLFromRepo(repo, commit, labelValue(labels, labelVCSType)); u != "" {
				return u
			}
		}
	}

	if u := firstLabel(labels, urlLabelKeys); u != "" && isLikelyGitRepoURL(u) {
		return u
	}
	if u := labelValue(labels, labelOCISource); isLikelyGitRepoURL(u) {
		return u
	}
	if u := labelValue(labels, labelBuildSourceLocation); isLikelyGitRepoURL(u) {
		return u
	}
	if u := labelValue(labels, labelSourceLocation); isLikelyGitRepoURL(u) {
		return u
	}
	if u := labelValue(labels, labelGenericURL); isLikelyGitRepoURL(u) {
		return normalizeGitRepoURLForCompose(u)
	}

	return ""
}

func firstRepoURL(labels map[string]string) string {
	for _, key := range []string{labelOCISource, labelBuildSourceLocation, labelSourceLocation, labelGenericURL} {
		if u := labelValue(labels, key); isLikelyGitRepoURL(u) {
			return normalizeGitRepoURLForCompose(u)
		}
	}
	return ""
}

func normalizeGitRepoURLForCompose(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(strings.ToLower(u), "github.com/") {
		u = "https://" + u
	}
	return strings.TrimSuffix(u, "/")
}

func deriveGitCommitURLFromRepo(repo, commit, vcsType string) string {
	if commit == "" {
		return ""
	}
	if vcsType != "" && vcsType != "git" {
		return ""
	}
	if !isLikelyGitRepoURL(repo) {
		return ""
	}

	repo = normalizeGitRepoURLForCompose(repo)
	lower := strings.ToLower(repo)
	if strings.Contains(lower, "github.com") {
		return repo + "/commit/" + commit
	}
	if strings.Contains(lower, "gitlab.com") {
		return repo + "/-/commit/" + commit
	}
	return ""
}
