package metadata

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/midu16/opm-troubleshooting/internal/session"
)

// MetadataStore wraps a SQLite database for persistent troubleshooting metadata.
type MetadataStore struct {
	db      *sql.DB
	baseDir string
}

// Open creates or opens a SQLite metadata database in the given directory.
func Open(dir string) (*MetadataStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(home, ".config", "opm-troubleshooting")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create metadata dir: %w", err)
	}

	dbPath := filepath.Join(dir, "metadata.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}

	store := &MetadataStore{db: db, baseDir: dir}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate metadata db: %w", err)
	}
	return store, nil
}

// Close releases the database connection.
func (s *MetadataStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database for advanced queries.
func (s *MetadataStore) DB() *sql.DB {
	return s.db
}

// BaseDir returns the metadata store base directory.
func (s *MetadataStore) BaseDir() string {
	return s.baseDir
}

func (s *MetadataStore) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS sessions (
		id              TEXT PRIMARY KEY,
		cluster_name    TEXT NOT NULL,
		operator        TEXT NOT NULL,
		environment     TEXT,
		source_type     TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata_json   TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_cluster ON sessions(cluster_name);
	CREATE INDEX IF NOT EXISTS idx_sessions_operator ON sessions(operator);

	CREATE TABLE IF NOT EXISTS runs (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id      TEXT REFERENCES sessions(id),
		timestamp       DATETIME DEFAULT CURRENT_TIMESTAMP,
		status          TEXT,
		real_issues     INTEGER DEFAULT 0,
		cosmetic_alerts INTEGER DEFAULT 0,
		health_passed   INTEGER,
		health_failed   INTEGER,
		infra_passed    INTEGER,
		infra_failed    INTEGER,
		adhd_branches   INTEGER,
		adhd_traps      INTEGER,
		must_gather_path TEXT,
		rca_path        TEXT,
		classification  TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_runs_session ON runs(session_id);
	CREATE INDEX IF NOT EXISTS idx_runs_timestamp ON runs(timestamp);

	CREATE TABLE IF NOT EXISTS fingerprints (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id          INTEGER REFERENCES runs(id),
		symptom_hash    TEXT NOT NULL,
		operator        TEXT,
		symptoms        TEXT,
		root_cause      TEXT,
		classification  TEXT,
		resolution      TEXT,
		confidence      REAL DEFAULT 0.0,
		hit_count       INTEGER DEFAULT 1,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen       DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_fingerprints_hash ON fingerprints(symptom_hash);
	CREATE INDEX IF NOT EXISTS idx_fingerprints_operator ON fingerprints(operator);

	CREATE TABLE IF NOT EXISTS hypotheses (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id          INTEGER REFERENCES runs(id),
		frame_id        TEXT,
		hypothesis_text TEXT,
		score_total     REAL,
		was_trap        BOOLEAN DEFAULT FALSE,
		was_confirmed   BOOLEAN,
		operator        TEXT,
		cluster_name    TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_hypotheses_frame ON hypotheses(frame_id);
	CREATE INDEX IF NOT EXISTS idx_hypotheses_operator ON hypotheses(operator);

	CREATE TABLE IF NOT EXISTS pattern_stats (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		pattern         TEXT NOT NULL,
		operator        TEXT,
		cluster_name    TEXT,
		confidence      REAL,
		count           INTEGER DEFAULT 1,
		last_seen       DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(pattern, operator, cluster_name)
	);

	CREATE TABLE IF NOT EXISTS repo_issues (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		repo            TEXT NOT NULL,
		issue_number    INTEGER,
		title           TEXT,
		state           TEXT,
		labels          TEXT,
		body_excerpt    TEXT,
		symptom_hash    TEXT,
		fetched_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_repo_issues_repo ON repo_issues(repo);
	CREATE INDEX IF NOT EXISTS idx_repo_issues_symptom ON repo_issues(symptom_hash);

	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	var count int
	row := s.db.QueryRow("SELECT COUNT(*) FROM schema_version")
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err = s.db.Exec("INSERT INTO schema_version (version) VALUES (1)")
	}
	return err
}

// MigrateFromJSON imports legacy JSON session files into the SQLite store.
func (s *MetadataStore) MigrateFromJSON(sessionsDir string) (int, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var record session.Record
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		metaJSON, _ := json.Marshal(record)
		_, err = s.db.Exec(`INSERT OR IGNORE INTO sessions (id, cluster_name, operator, environment, source_type, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?)`,
			record.ClusterID, record.ClusterName, record.OperatorPackage, record.Environment, "must-gather", string(metaJSON))
		if err != nil {
			continue
		}

		for _, entry := range record.History {
			_, err = s.db.Exec(`INSERT INTO runs (session_id, timestamp, status, real_issues, cosmetic_alerts, must_gather_path)
				VALUES (?, ?, ?, ?, ?, ?)`,
				record.ClusterID, entry.Timestamp, entry.Status, entry.RealIssues, entry.CosmeticAlerts, entry.MustGatherPath)
			if err != nil {
				continue
			}
		}
		migrated++
	}
	return migrated, nil
}
