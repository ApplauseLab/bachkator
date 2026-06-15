package state

import "database/sql"

func (s *Store) SaveQualityReports(reports []QualityReport, gates []QualityGateResult) error {
	if len(reports) == 0 && len(gates) == 0 {
		return nil
	}
	if s.db == nil {
		return errReadOnlyStore()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, report := range reports {
		result, err := tx.Exec(`
			INSERT INTO quality_reports (run_id, target, kind, format, path, status, message, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, report.RunID, report.Target, report.Kind, report.Format, report.Path, report.Status, report.Message, formatTime(report.CreatedAt))
		if err != nil {
			return err
		}
		reportID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		for _, metric := range report.Metrics {
			if _, err := tx.Exec(`
				INSERT INTO quality_metrics (report_id, name, scope, value, unit)
				VALUES (?, ?, ?, ?, ?)
			`, reportID, metric.Name, metric.Scope, metric.Value, metric.Unit); err != nil {
				return err
			}
		}
		for _, finding := range report.Findings {
			if _, err := tx.Exec(`
				INSERT INTO quality_findings (report_id, kind, file, line, severity, rule, message, duration_ms)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, reportID, finding.Kind, finding.File, finding.Line, finding.Severity, finding.Rule, finding.Message, finding.DurationMS); err != nil {
				return err
			}
		}
	}
	for _, gate := range gates {
		if _, err := tx.Exec(`
			INSERT INTO quality_gate_results (run_id, target, metric, op, threshold, actual, status, message, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, gate.RunID, gate.Target, gate.Metric, gate.Op, gate.Threshold, gate.Actual, gate.Status, gate.Message, formatTime(gate.CreatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListQualityReports(limit int) ([]QualityReport, error) {
	if s.db == nil {
		return []QualityReport{}, nil
	}
	query := `SELECT id, run_id, target, kind, format, path, status, message, created_at FROM quality_reports ORDER BY created_at DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var reports []QualityReport
	for rows.Next() {
		var report QualityReport
		var createdAt string
		if err := rows.Scan(
			&report.ID,
			&report.RunID,
			&report.Target,
			&report.Kind,
			&report.Format,
			&report.Path,
			&report.Status,
			&report.Message,
			&createdAt,
		); err != nil {
			return nil, err
		}
		report.CreatedAt = parseTime(createdAt)
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (s *Store) ListQualityMetrics(limit int) ([]QualityMetric, error) {
	if s.db == nil {
		return []QualityMetric{}, nil
	}
	query := `SELECT m.name, m.scope, m.value, m.unit FROM quality_metrics m JOIN quality_reports r ON r.id = m.report_id ORDER BY r.created_at DESC, m.name ASC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return s.scanQualityMetrics(rows)
}

func (s *Store) ListQualityFindings(limit int) ([]QualityFinding, error) {
	if s.db == nil {
		return []QualityFinding{}, nil
	}
	query := `SELECT f.kind, f.file, f.line, f.severity, f.rule, f.message, f.duration_ms FROM quality_findings f JOIN quality_reports r ON r.id = f.report_id ORDER BY r.created_at DESC, f.duration_ms DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var findings []QualityFinding
	for rows.Next() {
		var finding QualityFinding
		if err := rows.Scan(
			&finding.Kind,
			&finding.File,
			&finding.Line,
			&finding.Severity,
			&finding.Rule,
			&finding.Message,
			&finding.DurationMS,
		); err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

func (s *Store) ListQualityGateResults(limit int) ([]QualityGateResult, error) {
	if s.db == nil {
		return []QualityGateResult{}, nil
	}
	query := `SELECT run_id, target, metric, op, threshold, actual, status, message, created_at FROM quality_gate_results ORDER BY created_at DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var gates []QualityGateResult
	for rows.Next() {
		var gate QualityGateResult
		var createdAt string
		if err := rows.Scan(
			&gate.RunID,
			&gate.Target,
			&gate.Metric,
			&gate.Op,
			&gate.Threshold,
			&gate.Actual,
			&gate.Status,
			&gate.Message,
			&createdAt,
		); err != nil {
			return nil, err
		}
		gate.CreatedAt = parseTime(createdAt)
		gates = append(gates, gate)
	}
	return gates, rows.Err()
}

func (s *Store) QualityReportsForRun(runID string) ([]QualityReport, error) {
	if s.db == nil {
		return []QualityReport{}, nil
	}
	rows, err := s.db.Query(
		`SELECT id, run_id, target, kind, format, path, status, message, created_at FROM quality_reports WHERE run_id = ? ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var reports []QualityReport
	for rows.Next() {
		var report QualityReport
		var createdAt string
		if err := rows.Scan(
			&report.ID,
			&report.RunID,
			&report.Target,
			&report.Kind,
			&report.Format,
			&report.Path,
			&report.Status,
			&report.Message,
			&createdAt,
		); err != nil {
			return nil, err
		}
		report.CreatedAt = parseTime(createdAt)
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range reports {
		metrics, err := s.qualityMetricsForReport(reports[index].ID)
		if err != nil {
			return nil, err
		}
		findings, err := s.qualityFindingsForReport(reports[index].ID)
		if err != nil {
			return nil, err
		}
		reports[index].Metrics = metrics
		reports[index].Findings = findings
	}
	return reports, nil
}

func (s *Store) QualityGateResultsForRun(runID string) ([]QualityGateResult, error) {
	if s.db == nil {
		return []QualityGateResult{}, nil
	}
	rows, err := s.db.Query(
		`SELECT run_id, target, metric, op, threshold, actual, status, message, created_at FROM quality_gate_results WHERE run_id = ? ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var gates []QualityGateResult
	for rows.Next() {
		var gate QualityGateResult
		var createdAt string
		if err := rows.Scan(
			&gate.RunID,
			&gate.Target,
			&gate.Metric,
			&gate.Op,
			&gate.Threshold,
			&gate.Actual,
			&gate.Status,
			&gate.Message,
			&createdAt,
		); err != nil {
			return nil, err
		}
		gate.CreatedAt = parseTime(createdAt)
		gates = append(gates, gate)
	}
	return gates, rows.Err()
}

func (s *Store) qualityMetricsForReport(reportID int64) ([]QualityMetric, error) {
	rows, err := s.db.Query(
		`SELECT name, scope, value, unit FROM quality_metrics WHERE report_id = ? ORDER BY name ASC`,
		reportID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return s.scanQualityMetrics(rows)
}

func (s *Store) qualityFindingsForReport(reportID int64) ([]QualityFinding, error) {
	rows, err := s.db.Query(
		`SELECT kind, file, line, severity, rule, message, duration_ms FROM quality_findings WHERE report_id = ? ORDER BY id ASC`,
		reportID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var findings []QualityFinding
	for rows.Next() {
		var finding QualityFinding
		if err := rows.Scan(
			&finding.Kind,
			&finding.File,
			&finding.Line,
			&finding.Severity,
			&finding.Rule,
			&finding.Message,
			&finding.DurationMS,
		); err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

func (s *Store) scanQualityMetrics(rows *sql.Rows) ([]QualityMetric, error) {
	var metrics []QualityMetric
	for rows.Next() {
		var metric QualityMetric
		if err := rows.Scan(&metric.Name, &metric.Scope, &metric.Value, &metric.Unit); err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}
