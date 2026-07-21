package metadata

import "time"

// PatternStat tracks frequency of RCA patterns.
type PatternStat struct {
	ID          int64
	Pattern     string
	Operator    string
	ClusterName string
	Confidence  float64
	Count       int
	LastSeen    time.Time
}

// RecordPattern upserts a pattern occurrence counter.
func (s *MetadataStore) RecordPattern(pattern, operator, clusterName string, confidence float64) error {
	_, err := s.db.Exec(`INSERT INTO pattern_stats (pattern, operator, cluster_name, confidence, count, last_seen)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(pattern, operator, cluster_name) DO UPDATE SET
			count = count + 1,
			confidence = excluded.confidence,
			last_seen = CURRENT_TIMESTAMP`,
		pattern, operator, clusterName, confidence)
	return err
}

// GetPatternFrequency returns patterns for an operator, sorted by frequency.
func (s *MetadataStore) GetPatternFrequency(operator string) ([]PatternStat, error) {
	rows, err := s.db.Query(`SELECT id, pattern, operator, COALESCE(cluster_name, ''),
		confidence, count, last_seen
		FROM pattern_stats WHERE operator = ? ORDER BY count DESC`, operator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PatternStat
	for rows.Next() {
		var ps PatternStat
		if err := rows.Scan(&ps.ID, &ps.Pattern, &ps.Operator, &ps.ClusterName,
			&ps.Confidence, &ps.Count, &ps.LastSeen); err != nil {
			continue
		}
		stats = append(stats, ps)
	}
	return stats, rows.Err()
}

// GetTopPatterns returns the most frequent patterns across all operators.
func (s *MetadataStore) GetTopPatterns(limit int) ([]PatternStat, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, pattern, COALESCE(operator, ''), COALESCE(cluster_name, ''),
		confidence, count, last_seen
		FROM pattern_stats ORDER BY count DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PatternStat
	for rows.Next() {
		var ps PatternStat
		if err := rows.Scan(&ps.ID, &ps.Pattern, &ps.Operator, &ps.ClusterName,
			&ps.Confidence, &ps.Count, &ps.LastSeen); err != nil {
			continue
		}
		stats = append(stats, ps)
	}
	return stats, rows.Err()
}

// RepoIssue caches a GitHub issue fetched from an OpenShift repo.
type RepoIssue struct {
	ID           int64
	Repo         string
	IssueNumber  int
	Title        string
	State        string
	Labels       string
	BodyExcerpt  string
	SymptomHash  string
	FetchedAt    time.Time
}

// SaveRepoIssue caches a GitHub issue.
func (s *MetadataStore) SaveRepoIssue(issue RepoIssue) error {
	_, err := s.db.Exec(`INSERT INTO repo_issues (repo, issue_number, title, state, labels, body_excerpt, symptom_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		issue.Repo, issue.IssueNumber, issue.Title, issue.State, issue.Labels, issue.BodyExcerpt, issue.SymptomHash)
	return err
}

// GetCachedIssues returns cached issues for a repo, optionally filtered by symptom hash.
func (s *MetadataStore) GetCachedIssues(repo string, symptomHash string) ([]RepoIssue, error) {
	var rows interface{ Next() bool; Scan(...interface{}) error; Close() error; Err() error }
	var err error
	if symptomHash != "" {
		rows, err = s.db.Query(`SELECT id, repo, issue_number, title, state, COALESCE(labels, ''),
			COALESCE(body_excerpt, ''), COALESCE(symptom_hash, ''), fetched_at
			FROM repo_issues WHERE repo = ? AND symptom_hash = ? ORDER BY fetched_at DESC`, repo, symptomHash)
	} else {
		rows, err = s.db.Query(`SELECT id, repo, issue_number, title, state, COALESCE(labels, ''),
			COALESCE(body_excerpt, ''), COALESCE(symptom_hash, ''), fetched_at
			FROM repo_issues WHERE repo = ? ORDER BY fetched_at DESC`, repo)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []RepoIssue
	for rows.Next() {
		var ri RepoIssue
		if err := rows.Scan(&ri.ID, &ri.Repo, &ri.IssueNumber, &ri.Title, &ri.State,
			&ri.Labels, &ri.BodyExcerpt, &ri.SymptomHash, &ri.FetchedAt); err != nil {
			continue
		}
		issues = append(issues, ri)
	}
	return issues, rows.Err()
}
