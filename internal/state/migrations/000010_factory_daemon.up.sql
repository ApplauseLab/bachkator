ALTER TABLE factory_work_items ADD COLUMN claimed_by_daemon_id TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN claimed_at TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN claim_expires_at TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN completed_at TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN failed_at TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN failure_phase TEXT NOT NULL DEFAULT '';
ALTER TABLE factory_work_items ADD COLUMN failure_message TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_factory_work_items_claim_order
  ON factory_work_items(factory, lifecycle, priority, created_at, id);

CREATE TABLE IF NOT EXISTS factory_daemon_leases (
  daemon_id TEXT PRIMARY KEY,
  factory TEXT NOT NULL,
  hostname TEXT NOT NULL DEFAULT '',
  pid INTEGER NOT NULL DEFAULT 0,
  acquired_at TEXT NOT NULL,
  renewed_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  released_at TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_factory_daemon_leases_factory_status
  ON factory_daemon_leases(factory, status, expires_at);

CREATE TABLE IF NOT EXISTS factory_work_item_phases (
  work_item_id TEXT NOT NULL,
  attempt_id TEXT NOT NULL,
  phase_key TEXT NOT NULL,
  status TEXT NOT NULL,
  target TEXT NOT NULL DEFAULT '',
  run_id TEXT NOT NULL DEFAULT '',
  plan_path TEXT NOT NULL DEFAULT '',
  ledger_id TEXT NOT NULL DEFAULT '',
  evidence TEXT NOT NULL DEFAULT '{}',
  started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY(work_item_id, attempt_id, phase_key),
  FOREIGN KEY(work_item_id) REFERENCES factory_work_items(id) ON DELETE CASCADE,
  FOREIGN KEY(attempt_id) REFERENCES factory_work_item_attempts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_factory_work_item_phases_item
  ON factory_work_item_phases(work_item_id, attempt_id, phase_key);
