package metadata

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Session represents a troubleshooting session row.
type Session struct {
	ID           string
	ClusterName  string
	Operator     string
	Environment  string
	SourceType   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MetadataJSON string
}

// SaveSession upserts a session record.
func (s *MetadataStore) SaveSession(sess Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions (id, cluster_name, operator, environment, source_type, metadata_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			cluster_name = excluded.cluster_name,
			operator = excluded.operator,
			environment = excluded.environment,
			source_type = excluded.source_type,
			metadata_json = excluded.metadata_json,
			updated_at = CURRENT_TIMESTAMP`,
		sess.ID, sess.ClusterName, sess.Operator, sess.Environment, sess.SourceType, sess.MetadataJSON)
	return err
}

// LoadSession retrieves a session by ID.
func (s *MetadataStore) LoadSession(id string) (*Session, error) {
	row := s.db.QueryRow(`SELECT id, cluster_name, operator, environment, source_type, created_at, updated_at, COALESCE(metadata_json, '')
		FROM sessions WHERE id = ?`, id)

	var sess Session
	err := row.Scan(&sess.ID, &sess.ClusterName, &sess.Operator, &sess.Environment, &sess.SourceType, &sess.CreatedAt, &sess.UpdatedAt, &sess.MetadataJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &sess, err
}

// ListSessions returns all sessions ordered by last update.
func (s *MetadataStore) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, cluster_name, operator, environment, source_type, created_at, updated_at
		FROM sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.ClusterName, &sess.Operator, &sess.Environment, &sess.SourceType, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return sessions, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// SearchSessions finds sessions matching a query on cluster name or operator.
func (s *MetadataStore) SearchSessions(query string) ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, cluster_name, operator, environment, source_type, created_at, updated_at
		FROM sessions WHERE cluster_name LIKE ? OR operator LIKE ? ORDER BY updated_at DESC`,
		"%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.ClusterName, &sess.Operator, &sess.Environment, &sess.SourceType, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return sessions, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// SessionStats returns aggregate statistics for a session.
type SessionStats struct {
	TotalRuns     int
	FailedRuns    int
	LastStatus    string
	LastTimestamp time.Time
	TopPatterns   []string
}

// GetSessionStats returns statistics for a given session.
func (s *MetadataStore) GetSessionStats(sessionID string) (*SessionStats, error) {
	stats := &SessionStats{}

	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM runs WHERE session_id = ?`, sessionID)
	if err := row.Scan(&stats.TotalRuns, &stats.FailedRuns); err != nil {
		return nil, err
	}

	row = s.db.QueryRow(`SELECT COALESCE(status, ''), COALESCE(timestamp, '')
		FROM runs WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`, sessionID)
	_ = row.Scan(&stats.LastStatus, &stats.LastTimestamp)

	rows, err := s.db.Query(`SELECT pattern FROM pattern_stats
		WHERE operator = (SELECT operator FROM sessions WHERE id = ?)
		ORDER BY count DESC LIMIT 5`, sessionID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil {
				stats.TopPatterns = append(stats.TopPatterns, p)
			}
		}
	}

	return stats, nil
}

// SessionToJSON serializes a Session to JSON for storage.
func SessionToJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
