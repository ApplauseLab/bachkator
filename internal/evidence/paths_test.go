package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkspaceRejectsSymlinkToProjectRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	agents := filepath.Join(root, ".bach", "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(agents, "escape")); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveWorkspace(root, ".bach/agents/escape")
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("ResolveWorkspace() error = %v, want symlink rejection", err)
	}
}

func TestResolveWorkspaceRejectsSymlinkOutsideProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	agents := filepath.Join(root, ".bach", "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(agents, "outside")); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveWorkspace(root, ".bach/agents/outside")
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("ResolveWorkspace() error = %v, want symlink rejection", err)
	}
}

func TestResolveProjectFileRejectsSymlinkOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsidePlan := filepath.Join(outside, "plan.md")
	if err := os.WriteFile(outsidePlan, []byte("ship it\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePlan, filepath.Join(root, "plan.md")); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveProjectFile(root, "plan.md")
	if err == nil || !strings.Contains(err.Error(), "must stay within project root") {
		t.Fatalf("ResolveProjectFile() error = %v, want root containment rejection", err)
	}
}

func TestWritePrivateFileRejectsSymlinkTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "artifact.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := WritePrivateFile(link, []byte("new\n")); err == nil {
		t.Fatal("WritePrivateFile() error = nil, want symlink rejection")
	}
}

func TestWritePrivateFileRejectsBachSymlinkParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".bach")); err != nil {
		t.Fatal(err)
	}

	err := WritePrivateFile(
		filepath.Join(root, ".bach", "artifacts", "secret.json"),
		[]byte("{}\n"),
	)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("WritePrivateFile() error = %v, want .bach symlink rejection", err)
	}
}

func TestCreatePrivateFileChmodsBeforeCallerWrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".bach", "runs", "provider-events.raw.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := CreatePrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		_ = file.Close()
		t.Fatalf("open file mode = %o, want 600", got)
	}
	if _, err := file.Write([]byte("new\n")); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareStatePathRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "real.db")
	if err := os.WriteFile(target, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "state.db")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareStatePath(link); err == nil {
		t.Fatal("PrepareStatePath() error = nil, want symlink rejection")
	}
}

func TestWritePrivateFileUsesPrivatePermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := WritePrivateFile(path, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want 600", got)
	}
}
