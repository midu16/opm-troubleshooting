package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HistoryEntry records one analysis run for continuity across redeployments.
type HistoryEntry struct {
	Timestamp       time.Time `json:"timestamp"`
	RedeploymentNum int       `json:"redeployment_num"`
	Summary         string    `json:"summary"`
	Status          string    `json:"status"` // healthy, degraded, failed
	RealIssues      int       `json:"real_issues"`
	CosmeticAlerts  int       `json:"cosmetic_alerts"`
	MustGatherPath  string    `json:"must_gather_path,omitempty"`
}

// Record persists analysis context across redeployments.
type Record struct {
	ClusterID       string         `json:"cluster_id"`
	ClusterName     string         `json:"cluster_name"`
	OperatorPackage string         `json:"operator_package"`
	Environment     string         `json:"environment"`
	RedeploymentCount int          `json:"redeployment_count"`
	FirstSeen       time.Time      `json:"first_seen"`
	LastSeen        time.Time      `json:"last_seen"`
	KnownCosmetic   []string       `json:"known_cosmetic,omitempty"`
	History         []HistoryEntry `json:"history"`
}

// Store manages persistent session files.
type Store struct {
	baseDir string
}

// NewStore creates a session store at the given directory.
func NewStore(baseDir string) (*Store, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		baseDir = filepath.Join(home, ".config", "opm-troubleshooting", "sessions")
	}
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

// Load retrieves an existing session or creates a new one.
func (s *Store) Load(clusterName, operatorPackage string) (*Record, error) {
	id := clusterID(clusterName, operatorPackage)
	path := s.filePath(id)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			now := time.Now().UTC()
			return &Record{
				ClusterID:       id,
				ClusterName:     clusterName,
				OperatorPackage: operatorPackage,
				FirstSeen:       now,
				LastSeen:        now,
				History:         make([]HistoryEntry, 0),
				KnownCosmetic:   make([]string, 0),
			}, nil
		}
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &record, nil
}

// Save persists the session record.
func (s *Store) Save(record *Record) error {
	record.LastSeen = time.Now().UTC()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(record.ClusterID), data, 0o600)
}

// RecordRun appends a new history entry and increments redeployment count.
func (s *Store) RecordRun(record *Record, entry HistoryEntry) {
	record.RedeploymentCount++
	entry.RedeploymentNum = record.RedeploymentCount
	entry.Timestamp = time.Now().UTC()
	record.History = append(record.History, entry)

	// Keep last 50 entries for production use
	if len(record.History) > 50 {
		record.History = record.History[len(record.History)-50:]
	}
}

// AddKnownCosmetic records a cosmetic alert to suppress in future runs.
func (s *Store) AddKnownCosmetic(record *Record, alert string) {
	for _, existing := range record.KnownCosmetic {
		if existing == alert {
			return
		}
	}
	record.KnownCosmetic = append(record.KnownCosmetic, alert)
}

func (s *Store) filePath(clusterID string) string {
	return filepath.Join(s.baseDir, clusterID+".json")
}

func clusterID(clusterName, operatorPackage string) string {
	key := clusterName + ":" + operatorPackage
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:16])
}

// DefaultClusterName derives a cluster identifier from must-gather path.
func DefaultClusterName(mustGatherPath string) string {
	base := filepath.Base(mustGatherPath)
	// must-gather.local.123456789 -> use as cluster fingerprint
	if base != "" && base != "." {
		return base
	}
	return "unknown-cluster"
}
