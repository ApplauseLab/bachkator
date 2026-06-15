package state

import (
	"database/sql"
	"encoding/json"
	"strings"
)

type FindingQuery struct {
	Fingerprint string
	Status      string
	Limit       int
}

type FindingLocation struct {
	Path        string
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}

type NormalizedFinding struct {
	ID                   string
	Fingerprint          string
	SourceType           string
	SourceID             string
	Severity             string
	Category             string
	Message              string
	Location             *FindingLocation
	SuggestedFingerprint string
	ObservedAt           string
	Status               string
	Metadata             map[string]string
}

func (s *Store) RecordFindingObservation(finding NormalizedFinding) error {
	if s.db == nil {
		return errReadOnlyStore()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	metadata, err := json.Marshal(finding.Metadata)
	if err != nil {
		return err
	}
	location := FindingLocation{}
	if finding.Location != nil {
		location = *finding.Location
	}
	if _, err := tx.Exec(`
		INSERT INTO normalized_finding_events (
			id, fingerprint, source_type, source_id, severity, category, message,
			path, start_line, start_column, end_line, end_column,
			suggested_fingerprint, observed_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		finding.ID,
		finding.Fingerprint,
		finding.SourceType,
		finding.SourceID,
		finding.Severity,
		finding.Category,
		finding.Message,
		location.Path,
		location.StartLine,
		location.StartColumn,
		location.EndLine,
		location.EndColumn,
		finding.SuggestedFingerprint,
		finding.ObservedAt,
		string(metadata),
	); err != nil {
		return err
	}
	if finding.Status == "" {
		finding.Status = "open"
	}
	if _, err := tx.Exec(`
		INSERT INTO normalized_findings_current (
			fingerprint, latest_event_id, source_type, source_id, severity, category,
			message, path, start_line, start_column, end_line, end_column, status,
			first_observed_at, last_observed_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			latest_event_id = excluded.latest_event_id,
			source_type = excluded.source_type,
			source_id = excluded.source_id,
			severity = excluded.severity,
			category = excluded.category,
			message = excluded.message,
			path = excluded.path,
			start_line = excluded.start_line,
			start_column = excluded.start_column,
			end_line = excluded.end_line,
			end_column = excluded.end_column,
			status = excluded.status,
			last_observed_at = excluded.last_observed_at,
			metadata = excluded.metadata
	`,
		finding.Fingerprint,
		finding.ID,
		finding.SourceType,
		finding.SourceID,
		finding.Severity,
		finding.Category,
		finding.Message,
		location.Path,
		location.StartLine,
		location.StartColumn,
		location.EndLine,
		location.EndColumn,
		finding.Status,
		finding.ObservedAt,
		finding.ObservedAt,
		string(metadata),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetFinding(fingerprint string) (NormalizedFinding, bool, error) {
	findings, err := s.ListCurrentFindings(FindingQuery{Fingerprint: fingerprint, Limit: 1})
	if err != nil || len(findings) == 0 {
		return NormalizedFinding{}, false, err
	}
	return findings[0], true, nil
}

func (s *Store) ListCurrentFindings(query FindingQuery) ([]NormalizedFinding, error) {
	if s.db == nil {
		return []NormalizedFinding{}, nil
	}
	clauses := []string{"1 = 1"}
	args := []any{}
	if query.Fingerprint != "" {
		clauses = append(clauses, "fingerprint = ?")
		args = append(args, query.Fingerprint)
	}
	if query.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, query.Status)
	}
	limit := ""
	if query.Limit > 0 {
		limit = " LIMIT ?"
		args = append(args, query.Limit)
	}
	return s.queryFindings(`
		SELECT latest_event_id, fingerprint, source_type, source_id, severity, category, message,
			path, start_line, start_column, end_line, end_column, '', last_observed_at, status, metadata
		FROM normalized_findings_current
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY last_observed_at DESC`+limit, args...)
}

func (s *Store) ListFindingEvents(query FindingQuery) ([]NormalizedFinding, error) {
	if s.db == nil {
		return []NormalizedFinding{}, nil
	}
	clauses := []string{"1 = 1"}
	args := []any{}
	if query.Fingerprint != "" {
		clauses = append(clauses, "fingerprint = ?")
		args = append(args, query.Fingerprint)
	}
	limit := ""
	if query.Limit > 0 {
		limit = " LIMIT ?"
		args = append(args, query.Limit)
	}
	return s.queryFindings(`
		SELECT id, fingerprint, source_type, source_id, severity, category, message,
			path, start_line, start_column, end_line, end_column, suggested_fingerprint, observed_at, '', metadata
		FROM normalized_finding_events
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY observed_at DESC`+limit, args...)
}

func (s *Store) queryFindings(query string, args ...any) ([]NormalizedFinding, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	findings := []NormalizedFinding{}
	for rows.Next() {
		finding, err := s.scanFinding(rows)
		if err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

func (s *Store) scanFinding(rows *sql.Rows) (NormalizedFinding, error) {
	var finding NormalizedFinding
	var path sql.NullString
	var suggestedFingerprint sql.NullString
	var startLine, startColumn, endLine, endColumn sql.NullInt64
	var metadata string
	if err := rows.Scan(
		&finding.ID,
		&finding.Fingerprint,
		&finding.SourceType,
		&finding.SourceID,
		&finding.Severity,
		&finding.Category,
		&finding.Message,
		&path,
		&startLine,
		&startColumn,
		&endLine,
		&endColumn,
		&suggestedFingerprint,
		&finding.ObservedAt,
		&finding.Status,
		&metadata,
	); err != nil {
		return NormalizedFinding{}, err
	}
	if suggestedFingerprint.Valid {
		finding.SuggestedFingerprint = suggestedFingerprint.String
	}
	if path.Valid && path.String != "" {
		finding.Location = &FindingLocation{Path: path.String}
		if startLine.Valid {
			finding.Location.StartLine = int(startLine.Int64)
		}
		if startColumn.Valid {
			finding.Location.StartColumn = int(startColumn.Int64)
		}
		if endLine.Valid {
			finding.Location.EndLine = int(endLine.Int64)
		}
		if endColumn.Valid {
			finding.Location.EndColumn = int(endColumn.Int64)
		}
	}
	finding.Metadata = map[string]string{}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &finding.Metadata); err != nil {
			return NormalizedFinding{}, err
		}
	}
	return finding, nil
}
