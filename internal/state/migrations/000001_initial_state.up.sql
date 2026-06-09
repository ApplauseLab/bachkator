CREATE TABLE IF NOT EXISTS target_state (
  name TEXT PRIMARY KEY,
  fingerprint TEXT NOT NULL,
  completed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY,
  target TEXT NOT NULL,
  dry_run INTEGER NOT NULL DEFAULT 0,
  force INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT '',
  log_dir TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS target_runs (
  run_id TEXT NOT NULL,
  target TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT '',
  log_path TEXT NOT NULL DEFAULT '',
  command TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (run_id, target),
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at DESC);
