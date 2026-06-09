CREATE TABLE IF NOT EXISTS artifacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  target TEXT NOT NULL,
  kind TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE,
  UNIQUE(run_id, target, kind, path, value)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_run_target ON artifacts(run_id, target);
CREATE INDEX IF NOT EXISTS idx_artifacts_path ON artifacts(path);
CREATE INDEX IF NOT EXISTS idx_artifacts_kind ON artifacts(kind);
