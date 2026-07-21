package rag

import (
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"
)

type Config struct {
	DataDir   string          `json:"data_dir"`
	Embedding EmbeddingConfig `json:"embedding"`
	OpenShift OpenShiftConfig `json:"openshift"`
	Retrieval RetrievalConfig `json:"retrieval"`
	Chunking  ChunkingConfig  `json:"chunking"`
	Ingestion IngestionConfig `json:"ingestion"`
	Secret    SecretConfig    `json:"secret_filter"`
	Freshness FreshnessConfig `json:"freshness"`
}

type EmbeddingConfig struct {
	URL   string `json:"url"`
	Model string `json:"model"`
}

type OpenShiftConfig struct {
	Version     string   `json:"version"`
	DocsBaseURL string   `json:"docs_base_url"`
	Repos       []string `json:"repos"`
}

type RetrievalConfig struct {
	DefaultTopK        int `json:"default_top_k"`
	CodeTopK           int `json:"code_top_k"`
	ConfigTopK         int `json:"config_top_k"`
	IssuesTopK         int `json:"issues_top_k"`
	KeywordSupplementMax int `json:"keyword_supplement_max"`
	KeywordMinLength   int `json:"keyword_min_length"`
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
		Embedding: EmbeddingConfig{
			URL:   "http://localhost:11434",
			Model: "all-minilm",
		},
		OpenShift: OpenShiftConfig{
			Version:     "4.22",
			DocsBaseURL: "https://docs.redhat.com/en/documentation/openshift_container_platform/4.22",
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
			DefaultTopK:        8,
			CodeTopK:           6,
			ConfigTopK:         4,
			IssuesTopK:         5,
			KeywordSupplementMax: 4,
			KeywordMinLength:   4,
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
}

func (c *Config) ReposDir() string   { return filepath.Join(c.DataDir, "repos") }
func (c *Config) DocsDir() string    { return filepath.Join(c.DataDir, "docs") }
func (c *Config) ChromemDir() string { return filepath.Join(c.DataDir, "chromem") }
func (c *Config) TelcoDir() string   { return filepath.Join(c.DataDir, "telco-reference") }
