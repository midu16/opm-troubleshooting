package metadata

import "database/sql"

// HypothesisRecord stores an ADHD hypothesis with its outcome.
type HypothesisRecord struct {
	ID             int64
	RunID          int64
	FrameID        string
	HypothesisText string
	ScoreTotal     float64
	WasTrap        bool
	WasConfirmed   *bool
	Operator       string
	ClusterName    string
}

// RecordHypotheses inserts a batch of hypotheses for a single run.
func (s *MetadataStore) RecordHypotheses(runID int64, hypotheses []HypothesisRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT INTO hypotheses
		(run_id, frame_id, hypothesis_text, score_total, was_trap, was_confirmed, operator, cluster_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, h := range hypotheses {
		_, err = stmt.Exec(h.RunID, h.FrameID, h.HypothesisText, h.ScoreTotal,
			h.WasTrap, h.WasConfirmed, h.Operator, h.ClusterName)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FrameAccuracy returns the confirmation rate for a given ADHD frame.
func (s *MetadataStore) FrameAccuracy(frameID string) (float64, error) {
	var total, confirmed int
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN was_confirmed = 1 THEN 1 ELSE 0 END), 0)
		FROM hypotheses WHERE frame_id = ? AND was_confirmed IS NOT NULL`, frameID)
	if err := row.Scan(&total, &confirmed); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0.5, nil
	}
	return float64(confirmed) / float64(total), nil
}

// GetBoostFactors returns per-frame accuracy-based boost factors for an operator.
func (s *MetadataStore) GetBoostFactors(operator string) (map[string]float64, error) {
	rows, err := s.db.Query(`SELECT frame_id, COUNT(*),
		COALESCE(SUM(CASE WHEN was_confirmed = 1 THEN 1 ELSE 0 END), 0)
		FROM hypotheses WHERE operator = ? AND was_confirmed IS NOT NULL
		GROUP BY frame_id`, operator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boosts := make(map[string]float64)
	for rows.Next() {
		var frameID string
		var total, confirmed int
		if err := rows.Scan(&frameID, &total, &confirmed); err != nil {
			continue
		}
		if total > 0 {
			accuracy := float64(confirmed) / float64(total)
			boosts[frameID] = 1 + 0.2*(accuracy-0.5)
		}
	}
	return boosts, rows.Err()
}

// GetFrameStats returns per-frame hypothesis statistics.
type FrameStats struct {
	FrameID   string
	Total     int
	Confirmed int
	Rejected  int
	TrapCount int
	AvgScore  float64
}

// GetFrameStatsForOperator returns frame-level stats for an operator.
func (s *MetadataStore) GetFrameStatsForOperator(operator string) ([]FrameStats, error) {
	rows, err := s.db.Query(`SELECT frame_id,
		COUNT(*),
		COALESCE(SUM(CASE WHEN was_confirmed = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN was_confirmed = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN was_trap = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(score_total), 0)
		FROM hypotheses WHERE operator = ? GROUP BY frame_id ORDER BY COUNT(*) DESC`, operator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []FrameStats
	for rows.Next() {
		var fs FrameStats
		if err := rows.Scan(&fs.FrameID, &fs.Total, &fs.Confirmed, &fs.Rejected, &fs.TrapCount, &fs.AvgScore); err != nil {
			continue
		}
		stats = append(stats, fs)
	}
	return stats, rows.Err()
}

// ConfirmHypothesis marks a hypothesis as confirmed or rejected.
func (s *MetadataStore) ConfirmHypothesis(id int64, confirmed bool) error {
	_, err := s.db.Exec(`UPDATE hypotheses SET was_confirmed = ? WHERE id = ?`, confirmed, id)
	return err
}

// GetHypothesesForRun returns all hypotheses recorded for a specific run.
func (s *MetadataStore) GetHypothesesForRun(runID int64) ([]HypothesisRecord, error) {
	rows, err := s.db.Query(`SELECT id, run_id, frame_id, hypothesis_text, score_total,
		was_trap, was_confirmed, COALESCE(operator, ''), COALESCE(cluster_name, '')
		FROM hypotheses WHERE run_id = ?`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HypothesisRecord
	for rows.Next() {
		var h HypothesisRecord
		var confirmed sql.NullBool
		if err := rows.Scan(&h.ID, &h.RunID, &h.FrameID, &h.HypothesisText,
			&h.ScoreTotal, &h.WasTrap, &confirmed, &h.Operator, &h.ClusterName); err != nil {
			continue
		}
		if confirmed.Valid {
			h.WasConfirmed = &confirmed.Bool
		}
		results = append(results, h)
	}
	return results, rows.Err()
}
