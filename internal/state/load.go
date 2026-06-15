package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

func (s *Store) loadFromDB() (*State, error) {
	state := newEmptyState()
	if err := s.loadTargetState(state); err != nil {
		return nil, err
	}
	if err := s.loadRuns(state); err != nil {
		return nil, err
	}
	return state, nil
}

func newEmptyState() *State {
	return &State{Version: 3, Targets: map[string]Record{}, Runs: []RunRecord{}}
}

func (s *Store) loadTargetState(state *State) error {
	rows, err := s.db.Query(
		`SELECT name, fingerprint, fingerprint_parts, completed_at FROM target_state`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, fingerprint, fingerprintPartsJSON, completedAt string
		if err := rows.Scan(&name, &fingerprint, &fingerprintPartsJSON, &completedAt); err != nil {
			return err
		}
		fingerprintParts := map[string]string{}
		if fingerprintPartsJSON != "" {
			if err := json.Unmarshal([]byte(fingerprintPartsJSON), &fingerprintParts); err != nil {
				return err
			}
		}
		state.Targets[name] = Record{
			Fingerprint:      fingerprint,
			FingerprintParts: fingerprintParts,
			CompletedAt:      parseTime(completedAt),
		}
	}
	return rows.Err()
}

func (s *Store) loadRuns(state *State) error {
	rows, err := s.db.Query(
		`SELECT id, target, dry_run, force, status, started_at, finished_at, log_dir FROM runs ORDER BY started_at ASC`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	runsByID := map[string]int{}
	for rows.Next() {
		var run RunRecord
		var dryRun, force int
		var startedAt, finishedAt string
		if err := rows.Scan(
			&run.ID,
			&run.Target,
			&dryRun,
			&force,
			&run.Status,
			&startedAt,
			&finishedAt,
			&run.LogDir,
		); err != nil {
			return err
		}
		run.DryRun = dryRun != 0
		run.Force = force != 0
		run.StartedAt = parseTime(startedAt)
		run.FinishedAt = parseTime(finishedAt)
		run.Targets = map[string]TargetRunRecord{}
		state.Runs = append(state.Runs, run)
		runsByID[run.ID] = len(state.Runs) - 1
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := s.loadTargetRuns(state, runsByID); err != nil {
		return err
	}
	return s.loadArtifacts(state, runsByID)
}

func (s *Store) loadTargetRuns(state *State, runsByID map[string]int) error {
	operationColumn, err := s.targetRunOperationColumn()
	if err != nil {
		return err
	}
	exitCodeColumn, err := s.targetRunExitCodeColumn()
	if err != nil {
		return err
	}
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT run_id, target, status, started_at, finished_at, log_path, %s, %s FROM target_runs ORDER BY started_at ASC`,
		operationColumn,
		exitCodeColumn,
	))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var runID, target, startedAt, finishedAt string
		var targetRun TargetRunRecord
		var exitCode sql.NullInt64
		if err := rows.Scan(
			&runID,
			&target,
			&targetRun.Status,
			&startedAt,
			&finishedAt,
			&targetRun.LogPath,
			&targetRun.Operation,
			&exitCode,
		); err != nil {
			return err
		}
		if exitCode.Valid {
			value := int(exitCode.Int64)
			targetRun.ExitCode = &value
		}
		runIndex, ok := runsByID[runID]
		if !ok {
			return fmt.Errorf("target run references unknown run %q", runID)
		}
		targetRun.StartedAt = parseTime(startedAt)
		targetRun.FinishedAt = parseTime(finishedAt)
		state.Runs[runIndex].Targets[target] = targetRun
	}
	return rows.Err()
}

func (s *Store) targetRunExitCodeColumn() (string, error) {
	rows, err := s.db.Query(`PRAGMA table_info(target_runs)`)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return "", err
		}
		if name == "exit_code" {
			return "exit_code", nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "NULL", nil
}

func (s *Store) targetRunOperationColumn() (string, error) {
	rows, err := s.db.Query(`PRAGMA table_info(target_runs)`)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	fallback := ""
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return "", err
		}
		if name == "operation" {
			return "operation", nil
		}
		if name == "command" {
			fallback = "command"
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("target_runs has no operation column")
}

func (s *Store) loadArtifacts(state *State, runsByID map[string]int) error {
	rows, err := s.db.Query(
		`SELECT run_id, target, kind, path, value, created_at FROM artifacts ORDER BY created_at ASC`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var artifact ArtifactRecord
		var createdAt string
		if err := rows.Scan(
			&artifact.RunID,
			&artifact.Target,
			&artifact.Kind,
			&artifact.Path,
			&artifact.Value,
			&createdAt,
		); err != nil {
			return err
		}
		runIndex, ok := runsByID[artifact.RunID]
		if !ok {
			return fmt.Errorf("artifact references unknown run %q", artifact.RunID)
		}
		artifact.CreatedAt = parseTime(createdAt)
		state.Runs[runIndex].Artifacts = append(state.Runs[runIndex].Artifacts, artifact)
	}
	return rows.Err()
}
