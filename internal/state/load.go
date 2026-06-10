package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

func Load(path string) (*State, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	return loadFromDB(db)
}

func LoadReadOnly(path string) (*State, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return newEmptyState(), nil
		}
		return nil, err
	}
	db, err := OpenReadOnlyDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	return loadFromDB(db)
}

func loadFromDB(db *sql.DB) (*State, error) {
	state := newEmptyState()
	if err := loadTargetState(db, state); err != nil {
		return nil, err
	}
	if err := loadRuns(db, state); err != nil {
		return nil, err
	}
	return state, nil
}

func newEmptyState() *State {
	return &State{Version: 3, Targets: map[string]Record{}, Runs: []RunRecord{}}
}

func loadTargetState(db *sql.DB, state *State) error {
	rows, err := db.Query(
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

func loadRuns(db *sql.DB, state *State) error {
	rows, err := db.Query(
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
	if err := loadTargetRuns(db, state, runsByID); err != nil {
		return err
	}
	return loadArtifacts(db, state, runsByID)
}

func loadTargetRuns(db *sql.DB, state *State, runsByID map[string]int) error {
	operationColumn, err := targetRunOperationColumn(db)
	if err != nil {
		return err
	}
	exitCodeColumn, err := targetRunExitCodeColumn(db)
	if err != nil {
		return err
	}
	rows, err := db.Query(fmt.Sprintf(
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

func targetRunExitCodeColumn(db *sql.DB) (string, error) {
	rows, err := db.Query(`PRAGMA table_info(target_runs)`)
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

func targetRunOperationColumn(db *sql.DB) (string, error) {
	rows, err := db.Query(`PRAGMA table_info(target_runs)`)
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

func loadArtifacts(db *sql.DB, state *State, runsByID map[string]int) error {
	rows, err := db.Query(
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
