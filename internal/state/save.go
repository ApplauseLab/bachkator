package state

import "encoding/json"

func (s *Store) SaveSnapshot(targets map[string]Record, runs []RunRecord) error {
	if s.db == nil {
		return errReadOnlyStore()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for name, record := range targets {
		fingerprintParts, err := json.Marshal(record.FingerprintParts)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO target_state (name, fingerprint, fingerprint_parts, completed_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				fingerprint = excluded.fingerprint,
				fingerprint_parts = excluded.fingerprint_parts,
				completed_at = excluded.completed_at
		`, name, record.Fingerprint, string(fingerprintParts), formatTime(record.CompletedAt)); err != nil {
			return err
		}
	}

	for _, run := range runs {
		if _, err := tx.Exec(`
			INSERT INTO runs (id, target, dry_run, force, status, started_at, finished_at, log_dir)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				target = excluded.target,
				dry_run = excluded.dry_run,
				force = excluded.force,
				status = excluded.status,
				started_at = excluded.started_at,
				finished_at = excluded.finished_at,
				log_dir = excluded.log_dir
		`, run.ID, run.Target, boolInt(run.DryRun), boolInt(run.Force), run.Status, formatTime(run.StartedAt), formatTime(run.FinishedAt), run.LogDir); err != nil {
			return err
		}
		for target, targetRun := range run.Targets {
			var exitCode any
			if targetRun.ExitCode != nil {
				exitCode = *targetRun.ExitCode
			}
			if _, err := tx.Exec(`
				INSERT INTO target_runs (run_id, target, status, started_at, finished_at, log_path, operation, exit_code)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(run_id, target) DO UPDATE SET
					status = excluded.status,
					started_at = excluded.started_at,
					finished_at = excluded.finished_at,
					log_path = excluded.log_path,
					operation = excluded.operation,
					exit_code = excluded.exit_code
			`, run.ID, target, targetRun.Status, formatTime(targetRun.StartedAt), formatTime(targetRun.FinishedAt), targetRun.LogPath, targetRun.Operation, exitCode); err != nil {
				return err
			}
		}
		for _, artifact := range run.Artifacts {
			if _, err := tx.Exec(`
				INSERT INTO artifacts (run_id, target, kind, path, value, created_at)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT(run_id, target, kind, path, value) DO UPDATE SET
					created_at = excluded.created_at
			`, run.ID, artifact.Target, artifact.Kind, artifact.Path, artifact.Value, formatTime(artifact.CreatedAt)); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
