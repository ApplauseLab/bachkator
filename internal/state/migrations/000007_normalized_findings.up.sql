CREATE TABLE IF NOT EXISTS normalized_finding_events (
  id TEXT PRIMARY KEY,
  fingerprint TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  severity TEXT NOT NULL,
  category TEXT NOT NULL,
  message TEXT NOT NULL,
  path TEXT,
  start_line INTEGER,
  start_column INTEGER,
  end_line INTEGER,
  end_column INTEGER,
  suggested_fingerprint TEXT,
  observed_at TEXT NOT NULL,
  metadata TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS normalized_findings_current (
  fingerprint TEXT PRIMARY KEY,
  latest_event_id TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  severity TEXT NOT NULL,
  category TEXT NOT NULL,
  message TEXT NOT NULL,
  path TEXT,
  start_line INTEGER,
  start_column INTEGER,
  end_line INTEGER,
  end_column INTEGER,
  status TEXT NOT NULL,
  first_observed_at TEXT NOT NULL,
  last_observed_at TEXT NOT NULL,
  metadata TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY(latest_event_id) REFERENCES normalized_finding_events(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_normalized_finding_events_fingerprint_observed
  ON normalized_finding_events(fingerprint, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_normalized_findings_current_status
  ON normalized_findings_current(status, last_observed_at DESC);
