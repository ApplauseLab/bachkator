package agentprovider

import (
	"context"
	"path/filepath"

	gitpkg "github.com/applauselab/bachkator/internal/git"
)

func workspaceDirty(ctx context.Context, workspace string) bool {
	dirty, err := gitpkg.Dirty(ctx, workspace)
	return err != nil || dirty
}

func samePath(a string, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
