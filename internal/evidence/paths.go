package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const privateFileMode os.FileMode = 0o600

func ResolveProjectFile(root string, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("project file path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("project file %q must be relative to the project root", rel)
	}
	rootPath, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(rootPath, rel)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve project file %q: %w", rel, err)
	}
	if !withinDir(rootPath, resolved) {
		return "", fmt.Errorf("project file %q must stay within project root", rel)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("project file %q: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("project file %q must be a regular file", rel)
	}
	return resolved, nil
}

func ResolveWorkspace(root string, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("agent workspace path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("agent workspace %q must be relative to the project root", rel)
	}
	cleanRel := filepath.ToSlash(filepath.Clean(rel))
	if cleanRel != ".bach/agents" && !strings.HasPrefix(cleanRel, ".bach/agents/") {
		return "", fmt.Errorf("agent workspace %q must stay under .bach/agents", rel)
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	agentsRoot := filepath.Join(rootReal, ".bach", "agents")
	workspace := filepath.Join(rootPath, filepath.FromSlash(cleanRel))
	workspaceForCheck := filepath.Join(rootReal, filepath.FromSlash(cleanRel))
	if !withinDir(agentsRoot, workspaceForCheck) {
		return "", fmt.Errorf("agent workspace %q must stay under .bach/agents", rel)
	}
	if samePath(rootReal, workspaceForCheck) {
		return "", fmt.Errorf("agent workspace %q must not resolve to the project root", rel)
	}
	if err := rejectSymlinkComponentsBelow(rootPath, workspace); err != nil {
		return "", fmt.Errorf("agent workspace %q: %w", rel, err)
	}
	info, err := os.Lstat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return workspace, nil
		}
		return "", fmt.Errorf("agent workspace %q: %w", rel, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("agent workspace %q must not be a symlink", rel)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("agent workspace %q exists and is not a directory", rel)
	}
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve agent workspace %q: %w", rel, err)
	}
	if samePath(rootReal, resolved) {
		return "", fmt.Errorf("agent workspace %q must not resolve to the project root", rel)
	}
	if !withinDir(agentsRoot, resolved) {
		return "", fmt.Errorf("agent workspace %q must resolve under .bach/agents", rel)
	}
	return workspace, nil
}

func ResolveStatePath(root string, statePath string) (string, error) {
	if statePath == "" {
		return "", fmt.Errorf("state path is empty")
	}
	candidate := statePath
	if !filepath.IsAbs(candidate) && root != "" {
		rootPath, err := resolveRoot(root)
		if err != nil {
			return "", err
		}
		candidate = filepath.Join(rootPath, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("state path %q must not be a symlink", statePath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("state path %q: %w", statePath, err)
	}
	return abs, nil
}

func ResolveProjectStatePath(root string, statePath string) (string, error) {
	if statePath == "" {
		return "", fmt.Errorf("state path is empty")
	}
	if filepath.IsAbs(statePath) {
		return "", fmt.Errorf("state path %q must be relative to the project root", statePath)
	}
	cleanRel := filepath.Clean(statePath)
	if cleanRel == "." || cleanRel == ".." ||
		strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("state path %q must stay within project root", statePath)
	}
	rootReal, err := resolveRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(rootReal, cleanRel)
	if !withinDir(rootReal, candidate) {
		return "", fmt.Errorf("state path %q must stay within project root", statePath)
	}
	if err := rejectSymlinkComponentsBelow(rootReal, candidate); err != nil {
		return "", fmt.Errorf("state path %q: %w", statePath, err)
	}
	if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("state path %q must not be a symlink", statePath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("state path %q: %w", statePath, err)
	}
	return candidate, nil
}

func PrepareStatePath(path string) (string, error) {
	resolved, err := ResolveStatePath("", path)
	if err != nil {
		return "", err
	}
	if err := rejectBachSymlinkComponents(filepath.Dir(resolved)); err != nil {
		return "", fmt.Errorf("state path %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", err
	}
	if err := rejectBachSymlinkComponents(resolved); err != nil {
		return "", fmt.Errorf("state path %q: %w", path, err)
	}
	if info, err := os.Lstat(resolved); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("state path %q must not be a symlink", path)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("state path %q: %w", path, err)
	}
	file, err := os.OpenFile(resolved, os.O_CREATE|os.O_RDWR, privateFileMode)
	if err != nil {
		return "", err
	}
	chmodErr := file.Chmod(privateFileMode)
	if err := file.Close(); err != nil {
		return "", err
	}
	if chmodErr != nil {
		return "", chmodErr
	}
	return resolved, nil
}

func CreatePrivateFile(path string) (*os.File, error) {
	resolved, err := preparePrivatePath(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(resolved, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, privateFileMode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(privateFileMode); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func WritePrivateFile(path string, data []byte) error {
	file, err := CreatePrivateFile(path)
	if err != nil {
		return err
	}
	chmodErr := file.Chmod(privateFileMode)
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if chmodErr != nil {
		return chmodErr
	}
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func WriteJSONArtifact(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return WritePrivateFile(path, append(data, '\n'))
}

func preparePrivatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("private evidence path is empty")
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := rejectBachSymlinkComponents(filepath.Dir(resolved)); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", err
	}
	if err := rejectBachSymlinkComponents(resolved); err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("private evidence path %q must not be a symlink", path)
		}
		if info.IsDir() {
			return "", fmt.Errorf("private evidence path %q is a directory", path)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return resolved, nil
}

func rejectBachSymlinkComponents(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parts := pathParts(abs)
	start := -1
	for index, part := range parts {
		if part == ".bach" {
			start = index
			break
		}
	}
	if start == -1 {
		return nil
	}
	current := filepath.VolumeName(abs)
	if filepath.IsAbs(abs) {
		current += string(filepath.Separator)
	}
	for index, part := range parts {
		if current == "" || strings.HasSuffix(current, string(filepath.Separator)) {
			current += part
		} else {
			current = filepath.Join(current, part)
		}
		if index < start {
			continue
		}
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q must not be a symlink", current)
		}
	}
	return nil
}

func pathParts(path string) []string {
	volume := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, volume)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	if rest == "" {
		return nil
	}
	return strings.Split(rest, string(filepath.Separator))
}

func resolveRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("project root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("project root %q: %w", root, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("project root %q: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root %q must be a directory", root)
	}
	return resolved, nil
}

func rejectSymlinkComponentsBelow(root string, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("%q must stay under %q", path, root)
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q must not be a symlink", current)
		}
	}
	return nil
}

func withinDir(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
