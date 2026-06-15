package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func withMigrationLock(dbPath string, fn func() error) error {
	lockPath := dbPath + ".migrate.lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create migration lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return fmt.Errorf("open migration lock: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
