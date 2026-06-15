CREATE TABLE IF NOT EXISTS factory_work_item_approvals (
  id TEXT PRIMARY KEY,
  factory TEXT NOT NULL,
  workflow TEXT NOT NULL,
  work_item_id TEXT NOT NULL,
  attempt_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  plan_path TEXT NOT NULL DEFAULT '',
  plan_hash TEXT NOT NULL DEFAULT '',
  approved_at TEXT NOT NULL,
  approver TEXT NOT NULL DEFAULT '',
  approver_source TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY(work_item_id) REFERENCES factory_work_items(id) ON DELETE CASCADE,
  FOREIGN KEY(attempt_id) REFERENCES factory_work_item_attempts(id) ON DELETE CASCADE,
  UNIQUE(work_item_id, attempt_id, phase)
);

CREATE INDEX IF NOT EXISTS idx_factory_work_item_approvals_item
  ON factory_work_item_approvals(work_item_id, approved_at);

CREATE INDEX IF NOT EXISTS idx_factory_work_item_approvals_unique
  ON factory_work_item_approvals(work_item_id, attempt_id, phase);
