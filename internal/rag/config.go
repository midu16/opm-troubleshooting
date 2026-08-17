package rag

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

type Config struct {
	DataDir     string            `json:"data_dir"`
	VectorStore VectorStoreConfig `json:"vector_store"`
	Embedding   EmbeddingConfig   `json:"embedding"`
	OpenShift   OpenShiftConfig   `json:"openshift"`
	Retrieval   RetrievalConfig   `json:"retrieval"`
	Chunking    ChunkingConfig    `json:"chunking"`
	Ingestion   IngestionConfig   `json:"ingestion"`
	Secret      SecretConfig      `json:"secret_filter"`
	Freshness   FreshnessConfig   `json:"freshness"`
}

// VectorStoreConfig selects and configures the vector database backend.
type VectorStoreConfig struct {
	// Backend is "chromem" (default, embedded/local) or "qdrant" (external service).
	Backend string       `json:"backend"`
	Qdrant  QdrantConfig `json:"qdrant"`
}

// QdrantConfig holds connection settings for the external Qdrant backend.
type QdrantConfig struct {
	// URL is the Qdrant HTTP endpoint, e.g. http://qdrant:6333
	URL string `json:"url"`
	// APIKey is sent as the api-key header when set (Qdrant Cloud / secured deployments).
	APIKey string `json:"api_key"`
	// CollectionPrefix namespaces this instance's collections (optional).
	CollectionPrefix string `json:"collection_prefix"`
	// Distance metric for vector similarity: Cosine (default), Dot, or Euclid.
	Distance string `json:"distance"`
	// Timeout for individual HTTP requests to Qdrant.
	Timeout duration `json:"timeout"`
}

type EmbeddingConfig struct {
	URL   string `json:"url"`
	Model string `json:"model"`
}

type VersionEntry struct {
	Version        string `json:"version"`
	DocsBranch     string `json:"docs_branch"`
	OperatorBranch string `json:"operator_branch"`
	TelcoBranch    string `json:"telco_branch"`
	ACMDocsBranch  string `json:"acm_docs_branch"`
}

type ACMDocsConfig struct {
	Repo    string `json:"repo"`
	Enabled bool   `json:"enabled"`
}

type TelcoRefConfig struct {
	Repo    string `json:"repo"`
	Enabled bool   `json:"enabled"`
}

type OpenShiftConfig struct {
	Version        string         `json:"version"`
	DocsBaseURL    string         `json:"docs_base_url"` // Deprecated: docs are cloned from GitHub. Kept for config backward compatibility.
	GitBaseURL     string         `json:"git_base_url"`  // Base URL for cloning repos (default https://github.com); set for a GitHub Enterprise host or mirror.
	Repos          []string       `json:"repos"`
	Versions       []VersionEntry `json:"versions"`
	ACMDocs        ACMDocsConfig  `json:"acm_docs"`
	TelcoReference TelcoRefConfig `json:"telco_reference"`
}

type RetrievalConfig struct {
	DefaultTopK          int `json:"default_top_k"`
	CodeTopK             int `json:"code_top_k"`
	ConfigTopK           int `json:"config_top_k"`
	IssuesTopK           int `json:"issues_top_k"`
	KeywordSupplementMax int `json:"keyword_supplement_max"`
	KeywordMinLength     int `json:"keyword_min_length"`
}

type ChunkingConfig struct {
	ChunkSize    int `json:"chunk_size"`
	ChunkOverlap int `json:"chunk_overlap"`
}

type IngestionConfig struct {
	GitTimeout        duration `json:"git_timeout"`
	HTTPTimeout       duration `json:"http_timeout"`
	MaxParallelClones int      `json:"max_parallel_clones"`
}

type SecretConfig struct {
	SkipExtensions []string `json:"skip_extensions"`
	SkipFilenames  []string `json:"skip_filenames"`
}

type FreshnessConfig struct {
	MetaFile string `json:"meta_file"`
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	var err error
	d.Duration, err = time.ParseDuration(s)
	return err
}

func DefaultConfig() *Config {
	return &Config{
		DataDir: "rag-data",
		VectorStore: VectorStoreConfig{
			Backend: BackendChromem,
			Qdrant: QdrantConfig{
				URL:      "http://localhost:6333",
				Distance: "Cosine",
				Timeout:  duration{30 * time.Second},
			},
		},
		Embedding: EmbeddingConfig{
			URL:   "http://localhost:11434",
			Model: "all-minilm",
		},
		OpenShift: OpenShiftConfig{
			Version:     "4.22",
			DocsBaseURL: "https://docs.redhat.com/en/documentation/openshift_container_platform/4.22",
			GitBaseURL:  "https://github.com",
			ACMDocs: ACMDocsConfig{
				Repo:    "stolostron/rhacm-docs",
				Enabled: true,
			},
			TelcoReference: TelcoRefConfig{
				Repo:    "openshift-kni/telco-reference",
				Enabled: true,
			},
			Repos: []string{
				"cluster-etcd-operator",
				"cluster-kube-apiserver-operator",
				"cluster-kube-controller-manager-operator",
				"cluster-kube-scheduler-operator",
				"cluster-version-operator",
				"machine-config-operator",
				"cluster-network-operator",
				"cluster-ingress-operator",
				"cluster-storage-operator",
				"cluster-monitoring-operator",
				"cluster-authentication-operator",
				"local-storage-operator",
				"sriov-network-operator",
				"ptp-operator",
				"cluster-nfd-operator",
				"metallb-operator",
				"cluster-node-tuning-operator",
				"cluster-dns-operator",
				"cluster-image-registry-operator",
				"cluster-samples-operator",
				"cluster-logging-operator",
				"openshift-apiserver",
				"console-operator",
				"installer",
				"oc",
				"api",
				"kubernetes-nmstate",
			},
		},
		Retrieval: RetrievalConfig{
			DefaultTopK:          8,
			CodeTopK:             6,
			ConfigTopK:           4,
			IssuesTopK:           5,
			KeywordSupplementMax: 4,
			KeywordMinLength:     4,
		},
		Chunking: ChunkingConfig{
			ChunkSize:    1200,
			ChunkOverlap: 200,
		},
		Ingestion: IngestionConfig{
			GitTimeout:        duration{60 * time.Second},
			HTTPTimeout:       duration{30 * time.Second},
			MaxParallelClones: 4,
		},
		Secret: SecretConfig{
			SkipExtensions: []string{".pem", ".crt", ".key", ".p12", ".jks", ".pfx", ".enc", ".gpg"},
			SkipFilenames:  []string{".env", "pull-secret", "kubeconfig", "credentials", "htpasswd", "id_rsa", "id_ed25519"},
		},
		Freshness: FreshnessConfig{
			MetaFile: ".ingest_meta.json",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("OCP_RAG_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("OCP_RAG_EMBEDDING_URL"); v != "" {
		cfg.Embedding.URL = v
	}
	if v := os.Getenv("OCP_RAG_EMBEDDING_MODEL"); v != "" {
		cfg.Embedding.Model = v
	}
	if v := os.Getenv("OCP_RAG_OCP_VERSION"); v != "" {
		cfg.OpenShift.Version = v
	}
	if v := os.Getenv("OCP_RAG_GIT_BASE_URL"); v != "" {
		cfg.OpenShift.GitBaseURL = v
	}
	if v := os.Getenv("OCP_RAG_VECTORSTORE_BACKEND"); v != "" {
		cfg.VectorStore.Backend = v
	}
	if v := os.Getenv("OCP_RAG_QDRANT_URL"); v != "" {
		cfg.VectorStore.Qdrant.URL = v
	}
	if v := os.Getenv("OCP_RAG_QDRANT_API_KEY"); v != "" {
		cfg.VectorStore.Qdrant.APIKey = v
	}
}

// GitBase returns the configured git host base URL for cloning repositories,
// with a trailing slash trimmed. Defaults to https://github.com when unset so
// existing configs keep working; override it for a GitHub Enterprise host or an
// internal mirror.
func (c *Config) GitBase() string {
	base := strings.TrimRight(c.OpenShift.GitBaseURL, "/")
	if base == "" {
		return "https://github.com"
	}
	return base
}

// RepoURL resolves an "org/repo" slug to a full clone URL under the configured
// git host (see GitBase).
func (c *Config) RepoURL(slug string) string {
	return c.GitBase() + "/" + strings.TrimPrefix(slug, "/")
}

func (c *Config) ReposDir() string   { return filepath.Join(c.DataDir, "repos") }
func (c *Config) DocsDir() string    { return filepath.Join(c.DataDir, "docs") }
func (c *Config) ChromemDir() string { return filepath.Join(c.DataDir, "chromem") }
func (c *Config) TelcoDir() string {
	repo := c.OpenShift.TelcoReference.Repo
	if repo == "" {
		return filepath.Join(c.DataDir, "telco-reference")
	}
	parts := strings.SplitN(repo, "/", 2)
	name := parts[len(parts)-1]
	return filepath.Join(c.DataDir, name)
}
func (c *Config) ACMDocsDir() string { return filepath.Join(c.DataDir, "docs", "rhacm-docs") }

func synthesizeVersionEntry(version string) VersionEntry {
	return VersionEntry{
		Version:        version,
		DocsBranch:     "enterprise-" + version,
		OperatorBranch: "release-" + version,
		TelcoBranch:    "release-" + version,
		ACMDocsBranch:  "2.17_stage",
	}
}

func (c *Config) ActiveVersionEntry() VersionEntry {
	for _, ve := range c.OpenShift.Versions {
		if ve.Version == c.OpenShift.Version {
			return ve
		}
	}
	return synthesizeVersionEntry(c.OpenShift.Version)
}

func (c *Config) AllVersionEntries() []VersionEntry {
	if len(c.OpenShift.Versions) > 0 {
		return c.OpenShift.Versions
	}
	return []VersionEntry{synthesizeVersionEntry(c.OpenShift.Version)}
}
