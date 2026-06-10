package state

import "database/sql"

func SaveQualityReports(path string, reports []QualityReport, gates []QualityGateResult) error {
	if len(reports) == 0 && len(gates) == 0 {
		return nil
	}
	db, err := OpenDB(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
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

func ListQualityReports(path string, limit int) ([]QualityReport, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	query := `SELECT id, run_id, target, kind, format, path, status, message, created_at FROM quality_reports ORDER BY created_at DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.Query(query, args...)
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

func ListQualityMetrics(path string, limit int) ([]QualityMetric, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	query := `SELECT m.name, m.scope, m.value, m.unit FROM quality_metrics m JOIN quality_reports r ON r.id = m.report_id ORDER BY r.created_at DESC, m.name ASC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanQualityMetrics(rows)
}

func ListQualityFindings(path string, limit int) ([]QualityFinding, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	query := `SELECT f.kind, f.file, f.line, f.severity, f.rule, f.message, f.duration_ms FROM quality_findings f JOIN quality_reports r ON r.id = f.report_id ORDER BY r.created_at DESC, f.duration_ms DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.Query(query, args...)
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

func ListQualityGateResults(path string, limit int) ([]QualityGateResult, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	query := `SELECT run_id, target, metric, op, threshold, actual, status, message, created_at FROM quality_gate_results ORDER BY created_at DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.Query(query, args...)
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

func QualityReportsForRun(path string, runID string) ([]QualityReport, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(
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
		metrics, err := qualityMetricsForReport(db, reports[index].ID)
		if err != nil {
			return nil, err
		}
		findings, err := qualityFindingsForReport(db, reports[index].ID)
		if err != nil {
			return nil, err
		}
		reports[index].Metrics = metrics
		reports[index].Findings = findings
	}
	return reports, nil
}

func QualityGateResultsForRun(path string, runID string) ([]QualityGateResult, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(
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

func qualityMetricsForReport(db *sql.DB, reportID int64) ([]QualityMetric, error) {
	rows, err := db.Query(
		`SELECT name, scope, value, unit FROM quality_metrics WHERE report_id = ? ORDER BY name ASC`,
		reportID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanQualityMetrics(rows)
}

func qualityFindingsForReport(db *sql.DB, reportID int64) ([]QualityFinding, error) {
	rows, err := db.Query(
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

func scanQualityMetrics(rows *sql.Rows) ([]QualityMetric, error) {
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
