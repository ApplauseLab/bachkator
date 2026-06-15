CREATE TABLE IF NOT EXISTS factory_work_items (
  id TEXT PRIMARY KEY,
  factory TEXT NOT NULL,
  workflow TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  current_phase TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  body_hash TEXT NOT NULL DEFAULT '',
  priority TEXT NOT NULL,
  labels TEXT NOT NULL DEFAULT '[]',
  source_type TEXT NOT NULL,
  dedupe_key TEXT NOT NULL DEFAULT '',
  submitted_plan_path TEXT NOT NULL DEFAULT '',
  submitted_plan_hash TEXT NOT NULL DEFAULT '',
  intake_evidence_id TEXT NOT NULL DEFAULT '',
  intake_evidence_uri TEXT NOT NULL DEFAULT '',
  intake_evidence_hash TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  cancelled_at TEXT NOT NULL DEFAULT '',
  cancel_reason TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_factory_work_items_open_dedupe
  ON factory_work_items(factory, workflow, dedupe_key)
  WHERE dedupe_key <> '' AND lifecycle = 'pending';

CREATE INDEX IF NOT EXISTS idx_factory_work_items_factory_status
  ON factory_work_items(factory, lifecycle, created_at DESC);

CREATE TABLE IF NOT EXISTS factory_work_item_attempts (
  id TEXT PRIMARY KEY,
  work_item_id TEXT NOT NULL,
  attempt_number INTEGER NOT NULL,
  status TEXT NOT NULL,
  start_phase TEXT NOT NULL,
  submitted_plan_path TEXT NOT NULL DEFAULT '',
  submitted_plan_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(work_item_id) REFERENCES factory_work_items(id) ON DELETE CASCADE,
  UNIQUE(work_item_id, attempt_number)
);

CREATE INDEX IF NOT EXISTS idx_factory_work_item_attempts_item
  ON factory_work_item_attempts(work_item_id, attempt_number);

CREATE TABLE IF NOT EXISTS factory_work_item_events (
  id TEXT PRIMARY KEY,
  work_item_id TEXT NOT NULL,
  attempt_id TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  FOREIGN KEY(work_item_id) REFERENCES factory_work_items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_factory_work_item_events_item_time
  ON factory_work_item_events(work_item_id, created_at);
