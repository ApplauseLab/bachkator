package state

import (
	"strings"
	"time"
)

type ArtifactQuery struct {
	RunID  string
	Target string
	Status string
	Since  time.Time
	Path   string
	Limit  int
}

func ListArtifacts(path string, query ArtifactQuery) ([]ArtifactRecord, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	clauses := []string{"1 = 1"}
	args := []any{}
	if query.RunID != "" {
		clauses = append(clauses, "a.run_id = ?")
		args = append(args, query.RunID)
	}
	if query.Target != "" {
		clauses = append(clauses, "a.target = ?")
		args = append(args, query.Target)
	}
	if query.Status != "" {
		clauses = append(clauses, "r.status = ?")
		args = append(args, query.Status)
	}
	if !query.Since.IsZero() {
		clauses = append(clauses, "r.started_at >= ?")
		args = append(args, formatTime(query.Since))
	}
	if query.Path != "" {
		clauses = append(clauses, "a.path LIKE ?")
		args = append(args, "%"+query.Path+"%")
	}
	limit := ""
	if query.Limit > 0 {
		limit = " LIMIT ?"
		args = append(args, query.Limit)
	}

	rows, err := db.Query(`
		SELECT a.run_id, a.target, a.kind, a.path, a.value, a.created_at
		FROM artifacts a
		JOIN runs r ON r.id = a.run_id
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY r.started_at DESC, a.target ASC, a.kind ASC, a.path ASC`+limit, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	artifacts := []ArtifactRecord{}
	for rows.Next() {
		var artifact ArtifactRecord
		var createdAt string
		if err := rows.Scan(
			&artifact.RunID,
			&artifact.Target,
			&artifact.Kind,
			&artifact.Path,
			&artifact.Value,
			&createdAt,
		); err != nil {
			return nil, err
		}
		artifact.CreatedAt = parseTime(createdAt)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}
