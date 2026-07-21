package metadata

import (
	"database/sql"
	"time"
)

// Run represents a single analysis execution.
type Run struct {
	ID             int64
	SessionID      string
	Timestamp      time.Time
	Status         string
	RealIssues     int
	CosmeticAlerts int
	HealthPassed   int
	HealthFailed   int
	InfraPassed    int
	InfraFailed    int
	ADHDBranches   int
	ADHDTraps      int
	MustGatherPath string
	RCAPath        string
	Classification string
}

// RecordRun inserts a new analysis run and returns its ID.
func (s *MetadataStore) RecordRun(run Run) (int64, error) {
	result, err := s.db.Exec(`INSERT INTO runs
		(session_id, status, real_issues, cosmetic_alerts, health_passed, health_failed,
		 infra_passed, infra_failed, adhd_branches, adhd_traps, must_gather_path, rca_path, classification)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.SessionID, run.Status, run.RealIssues, run.CosmeticAlerts,
		run.HealthPassed, run.HealthFailed, run.InfraPassed, run.InfraFailed,
		run.ADHDBranches, run.ADHDTraps, run.MustGatherPath, run.RCAPath, run.Classification)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetRunsForSession returns all runs for a session, most recent first.
func (s *MetadataStore) GetRunsForSession(sessionID string) ([]Run, error) {
	rows, err := s.db.Query(`SELECT id, session_id, timestamp, status, real_issues, cosmetic_alerts,
		COALESCE(health_passed, 0), COALESCE(health_failed, 0),
		COALESCE(infra_passed, 0), COALESCE(infra_failed, 0),
		COALESCE(adhd_branches, 0), COALESCE(adhd_traps, 0),
		COALESCE(must_gather_path, ''), COALESCE(rca_path, ''), COALESCE(classification, '')
		FROM runs WHERE session_id = ? ORDER BY timestamp DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

// GetRecentRuns returns the N most recent runs across all sessions.
func (s *MetadataStore) GetRecentRuns(limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, session_id, timestamp, status, real_issues, cosmetic_alerts,
		COALESCE(health_passed, 0), COALESCE(health_failed, 0),
		COALESCE(infra_passed, 0), COALESCE(infra_failed, 0),
		COALESCE(adhd_branches, 0), COALESCE(adhd_traps, 0),
		COALESCE(must_gather_path, ''), COALESCE(rca_path, ''), COALESCE(classification, '')
		FROM runs ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

func scanRuns(rows *sql.Rows) ([]Run, error) {
	var runs []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Timestamp, &r.Status,
			&r.RealIssues, &r.CosmeticAlerts,
			&r.HealthPassed, &r.HealthFailed,
			&r.InfraPassed, &r.InfraFailed,
			&r.ADHDBranches, &r.ADHDTraps,
			&r.MustGatherPath, &r.RCAPath, &r.Classification); err != nil {
			return runs, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
