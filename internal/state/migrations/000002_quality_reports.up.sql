CREATE TABLE IF NOT EXISTS quality_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  target TEXT NOT NULL,
  kind TEXT NOT NULL,
  format TEXT NOT NULL,
  path TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS quality_metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT '',
  value REAL NOT NULL,
  unit TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (report_id) REFERENCES quality_reports(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS quality_findings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  report_id INTEGER NOT NULL,
  kind TEXT NOT NULL,
  file TEXT NOT NULL DEFAULT '',
  line INTEGER NOT NULL DEFAULT 0,
  severity TEXT NOT NULL DEFAULT '',
  rule TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  duration_ms REAL NOT NULL DEFAULT 0,
  FOREIGN KEY (report_id) REFERENCES quality_reports(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS quality_gate_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  target TEXT NOT NULL,
  metric TEXT NOT NULL,
  op TEXT NOT NULL,
  threshold REAL NOT NULL,
  actual REAL NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_quality_reports_run_target ON quality_reports(run_id, target);
CREATE INDEX IF NOT EXISTS idx_quality_metrics_name ON quality_metrics(name);
CREATE INDEX IF NOT EXISTS idx_quality_findings_kind ON quality_findings(kind);
CREATE INDEX IF NOT EXISTS idx_quality_gate_results_status ON quality_gate_results(status);
