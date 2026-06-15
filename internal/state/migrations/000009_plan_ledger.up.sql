CREATE TABLE IF NOT EXISTS plan_ledgers (
  ledger_id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL,
  status TEXT NOT NULL,
  hash TEXT NOT NULL,
  run_id TEXT NOT NULL DEFAULT '',
  commit_sha TEXT NOT NULL DEFAULT '',
  recorded_at TEXT NOT NULL,
  implemented_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_plan_ledgers_plan_latest
  ON plan_ledgers(plan_id, recorded_at DESC, ledger_id DESC);

CREATE TABLE IF NOT EXISTS plan_evidence (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  hash TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '{}',
  metadata TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY(ledger_id) REFERENCES plan_ledgers(ledger_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_plan_evidence_ledger
  ON plan_evidence(ledger_id);
