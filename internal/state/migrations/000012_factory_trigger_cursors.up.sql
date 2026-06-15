CREATE TABLE IF NOT EXISTS factory_trigger_cursors (
  factory TEXT NOT NULL,
  trigger TEXT NOT NULL,
  cursor TEXT NOT NULL DEFAULT '',
  last_poll_at TEXT NOT NULL DEFAULT '',
  last_ack_at TEXT NOT NULL DEFAULT '',
  last_nack_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  metadata TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY(factory, trigger)
);

CREATE INDEX IF NOT EXISTS idx_factory_trigger_cursors_factory
  ON factory_trigger_cursors(factory);
