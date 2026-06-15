package config

import (
	"io"
	"os"
	"path/filepath"
	"sort"
)

func (p *Project) resolveInputs(inputs []string) []string {
	return p.resolveInputsSeen(inputs, map[string]bool{})
}

func (p *Project) resolveInputsSeen(inputs []string, seen map[string]bool) []string {
	var paths []string
	for _, input := range inputs {
		if named, ok := p.Inputs[input]; ok {
			if seen[input] {
				continue
			}
			seen[input] = true
			paths = append(paths, p.resolveInputsSeen(named.Paths(), seen)...)
			continue
		}
		if _, ok := p.Resources[input]; ok {
			continue
		}
		paths = append(paths, input)
	}
	return paths
}

func (i *Input) Paths() []string {
	if i.Src != "" {
		return []string{i.Src}
	}
	return append([]string(nil), i.Srcs...)
}

func expandFiles(root string, patterns []string) ([]string, error) {
	seen := map[string]bool{}
	var paths []string
	for _, pattern := range patterns {
		absPattern := absPath(root, pattern)
		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			matches = []string{absPattern}
		}
		for _, match := range matches {
			if err := collectPath(root, match, seen, &paths); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func collectPath(root string, path string, seen map[string]bool, paths *[]string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return filepath.WalkDir(path, func(child string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			return addPath(root, child, seen, paths)
		})
	}
	return addPath(root, path, seen, paths)
}

func addPath(root string, path string, seen map[string]bool, paths *[]string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	if seen[rel] {
		return nil
	}
	seen[rel] = true
	*paths = append(*paths, rel)
	return nil
}

func hashPath(h io.Writer, root string, path string) {
	data, err := os.ReadFile(absPath(root, path))
	if err != nil {
		return
	}
	writeHash(h, path)
	_, _ = h.Write(data)
	_, _ = io.WriteString(h, "\x00")
}

func writeHash(w io.Writer, parts ...string) {
	for _, part := range parts {
		_, _ = io.WriteString(w, part)
		_, _ = io.WriteString(w, "\x00")
	}
}
