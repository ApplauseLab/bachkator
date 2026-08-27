package target

import (
	"context"
	"os"
	"os/exec"
	"testing"

	gitpkg "github.com/applauselab/bachkator/internal/git"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func checkoutFixture(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.email", "t@example.com")
	gitRun(t, root, "config", "user.name", "t")
	gitRun(t, root, "commit", "--allow-empty", "-q", "-m", "base")
	base, err := gitpkg.Head(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, base
}

func snapshotValidate(
	t *testing.T,
	ctx context.Context,
	root string,
	before projectCheckoutState,
) error {
	t.Helper()
	snapshot := projectCheckoutState{
		Branch: before.Branch,
		Head:   before.Head,
		Status: before.Status,
	}
	return validateProjectCheckoutSnapshot(ctx, root, snapshot)
}

// Relaxed mode lets operators fast-forward main while a provider runs; a
// non-fast-forward move is still provider damage.
func TestRelaxedCheckoutGuardAllowsFastForwardOnly(t *testing.T) {
	ctx := context.Background()
	root, base := checkoutFixture(t)
	before := projectCheckoutState{Branch: "main", Head: base}

	t.Setenv("BACH_CHECKOUT_GUARD", "relaxed")

	gitRun(t, root, "commit", "--allow-empty", "-q", "-m", "operator commit")
	if err := snapshotValidate(t, ctx, root, before); err != nil {
		t.Fatalf("fast-forward commit should pass relaxed guard: %v", err)
	}

	gitRun(t, root, "checkout", "-q", "-b", "divergent")
	gitRun(t, root, "commit", "--allow-empty", "-q", "-m", "divergence")
	if err := snapshotValidate(t, ctx, root, before); err == nil {
		t.Fatal("non-fast-forward HEAD move should fail the guard")
	}
}

// Strict mode (default) keeps the old behavior: any HEAD move fails.
func TestStrictCheckoutGuardFailsOnAnyHeadMove(t *testing.T) {
	ctx := context.Background()
	root, base := checkoutFixture(t)
	before := projectCheckoutState{Branch: "main", Head: base}

	os.Unsetenv("BACH_CHECKOUT_GUARD")

	gitRun(t, root, "commit", "--allow-empty", "-q", "-m", "operator commit")
	if err := snapshotValidate(t, ctx, root, before); err == nil {
		t.Fatal("strict guard should fail on any HEAD move")
	}
}
