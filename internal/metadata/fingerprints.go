package metadata

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Fingerprint represents an issue symptom signature for pattern matching.
type Fingerprint struct {
	ID             int64
	RunID          int64
	SymptomHash    string
	Operator       string
	Symptoms       []string
	RootCause      string
	Classification string
	Resolution     string
	Confidence     float64
	HitCount       int
	CreatedAt      time.Time
	LastSeen       time.Time
}

// ComputeSymptomHash produces a stable SHA256 hash from a set of symptoms.
func ComputeSymptomHash(symptoms []string) string {
	normalized := make([]string, 0, len(symptoms))
	for _, s := range symptoms {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			normalized = append(normalized, s)
		}
	}
	sort.Strings(normalized)
	joined := strings.Join(normalized, "\n")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

// SaveFingerprint inserts or updates a fingerprint. On hash collision, increments hit_count.
func (s *MetadataStore) SaveFingerprint(fp Fingerprint) (int64, error) {
	symptomsJSON, _ := json.Marshal(fp.Symptoms)

	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM fingerprints WHERE symptom_hash = ? AND operator = ?`,
		fp.SymptomHash, fp.Operator).Scan(&existingID)
	if err == nil {
		_, err = s.db.Exec(`UPDATE fingerprints SET hit_count = hit_count + 1, last_seen = CURRENT_TIMESTAMP,
			run_id = ?, confidence = ? WHERE id = ?`, fp.RunID, fp.Confidence, existingID)
		return existingID, err
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := s.db.Exec(`INSERT INTO fingerprints
		(run_id, symptom_hash, operator, symptoms, root_cause, classification, resolution, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fp.RunID, fp.SymptomHash, fp.Operator, string(symptomsJSON),
		fp.RootCause, fp.Classification, fp.Resolution, fp.Confidence)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// FindByHash looks up an exact fingerprint match.
func (s *MetadataStore) FindByHash(hash string) (*Fingerprint, error) {
	row := s.db.QueryRow(`SELECT id, run_id, symptom_hash, operator, symptoms,
		COALESCE(root_cause, ''), COALESCE(classification, ''), COALESCE(resolution, ''),
		confidence, hit_count, created_at, last_seen
		FROM fingerprints WHERE symptom_hash = ? ORDER BY hit_count DESC LIMIT 1`, hash)
	return scanFingerprint(row)
}

// FindSimilar returns fingerprints with Jaccard similarity above the threshold.
func (s *MetadataStore) FindSimilar(symptoms []string, threshold float64) ([]SimilarFingerprint, error) {
	rows, err := s.db.Query(`SELECT id, run_id, symptom_hash, operator, symptoms,
		COALESCE(root_cause, ''), COALESCE(classification, ''), COALESCE(resolution, ''),
		confidence, hit_count, created_at, last_seen
		FROM fingerprints ORDER BY hit_count DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SimilarFingerprint
	for rows.Next() {
		fp, err := scanFingerprintRow(rows)
		if err != nil {
			continue
		}
		sim := JaccardSimilarity(symptoms, fp.Symptoms)
		if sim >= threshold {
			results = append(results, SimilarFingerprint{
				Fingerprint: *fp,
				Similarity:  sim,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	return results, rows.Err()
}

// FindSimilarForOperator searches fingerprints scoped to a specific operator.
func (s *MetadataStore) FindSimilarForOperator(operator string, symptoms []string, threshold float64) ([]SimilarFingerprint, error) {
	rows, err := s.db.Query(`SELECT id, run_id, symptom_hash, operator, symptoms,
		COALESCE(root_cause, ''), COALESCE(classification, ''), COALESCE(resolution, ''),
		confidence, hit_count, created_at, last_seen
		FROM fingerprints WHERE operator = ? ORDER BY hit_count DESC LIMIT 200`, operator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SimilarFingerprint
	for rows.Next() {
		fp, err := scanFingerprintRow(rows)
		if err != nil {
			continue
		}
		sim := JaccardSimilarity(symptoms, fp.Symptoms)
		if sim >= threshold {
			results = append(results, SimilarFingerprint{
				Fingerprint: *fp,
				Similarity:  sim,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	return results, rows.Err()
}

// UpdateResolution sets the root cause, classification, and resolution for a fingerprint.
func (s *MetadataStore) UpdateResolution(id int64, rootCause, classification, resolution string) error {
	_, err := s.db.Exec(`UPDATE fingerprints SET root_cause = ?, classification = ?, resolution = ? WHERE id = ?`,
		rootCause, classification, resolution, id)
	return err
}

// SimilarFingerprint pairs a fingerprint with its similarity score.
type SimilarFingerprint struct {
	Fingerprint Fingerprint
	Similarity  float64
}

// JaccardSimilarity computes J(A,B) = |A∩B| / |A∪B| for two string sets.
func JaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[strings.ToLower(strings.TrimSpace(s))] = true
	}
	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[strings.ToLower(strings.TrimSpace(s))] = true
	}

	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}

	union := len(setA)
	for k := range setB {
		if !setA[k] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func scanFingerprint(row *sql.Row) (*Fingerprint, error) {
	var fp Fingerprint
	var symptomsJSON string
	err := row.Scan(&fp.ID, &fp.RunID, &fp.SymptomHash, &fp.Operator, &symptomsJSON,
		&fp.RootCause, &fp.Classification, &fp.Resolution,
		&fp.Confidence, &fp.HitCount, &fp.CreatedAt, &fp.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(symptomsJSON), &fp.Symptoms)
	return &fp, nil
}

func scanFingerprintRow(rows *sql.Rows) (*Fingerprint, error) {
	var fp Fingerprint
	var symptomsJSON string
	err := rows.Scan(&fp.ID, &fp.RunID, &fp.SymptomHash, &fp.Operator, &symptomsJSON,
		&fp.RootCause, &fp.Classification, &fp.Resolution,
		&fp.Confidence, &fp.HitCount, &fp.CreatedAt, &fp.LastSeen)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(symptomsJSON), &fp.Symptoms)
	return &fp, nil
}
