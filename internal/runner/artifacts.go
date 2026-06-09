package runner

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/applause/bachkator/internal/model"
	statestore "github.com/applause/bachkator/internal/state"
)

func indexRunArtifacts(
	project *Project,
	run RunRecord,
	targets map[string]*Target,
) []statestore.ArtifactRecord {
	indexed := map[string]statestore.ArtifactRecord{}
	now := time.Now().UTC()
	add := func(target, kind, path, value string) {
		key := target + "\x00" + kind + "\x00" + path + "\x00" + value
		indexed[key] = statestore.ArtifactRecord{
			RunID:     run.ID,
			Target:    target,
			Kind:      kind,
			Path:      path,
			Value:     value,
			CreatedAt: now,
		}
	}

	for name, targetRun := range run.Targets {
		if targetRun.LogPath != "" {
			add(name, "log", targetRun.LogPath, "")
		}
		runDir := targetRunDirectory(&run, name)
		if _, err := os.Stat(absPath(project.Root, runDir)); err == nil {
			add(name, "run-directory", runDir, "")
			indexTargetRunDirectory(project, runDir, name, add)
		}

		target := targets[name]
		if target == nil {
			continue
		}
		for _, output := range target.Outputs {
			if outputExists(project.Root, output) {
				add(name, artifactKind(output), output, "")
			}
		}
		for _, output := range target.OutputMap {
			if outputExists(project.Root, output) {
				add(name, artifactKind(output), output, "")
			}
		}
		if body, ok := target.Spec().Body.(model.ImageSpec); ok {
			for _, tag := range imageTags(target.Name, body) {
				add(name, "image-tag", "", tag)
			}
		}
	}

	keys := make([]string, 0, len(indexed))
	for key := range indexed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	artifacts := make([]statestore.ArtifactRecord, 0, len(keys))
	for _, key := range keys {
		artifacts = append(artifacts, indexed[key])
	}
	return artifacts
}

func indexTargetRunDirectory(
	project *Project,
	runDir, target string,
	add func(target, kind, path, value string),
) {
	absRunDir := absPath(project.Root, runDir)
	_ = filepath.WalkDir(absRunDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absPath(project.Root, "."), path)
		if err != nil {
			rel = path
		}
		add(target, artifactKind(rel), rel, "")
		return nil
	})
}

func outputExists(root, path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(absPath(root, path))
	return err == nil
}

func artifactKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml", ".json":
		return "manifest"
	case ".log":
		return "log"
	default:
		return "artifact"
	}
}

func imageTags(name string, body model.ImageSpec) []string {
	image := body.Image
	if image == "" {
		image = strings.TrimPrefix(name, "image/")
	}
	if len(body.Tags) == 0 {
		return []string{image}
	}
	tags := make([]string, 0, len(body.Tags))
	for _, tag := range body.Tags {
		if strings.Contains(tag, ":") || strings.Contains(tag, "/") {
			tags = append(tags, tag)
			continue
		}
		tags = append(tags, image+":"+tag)
	}
	return tags
}
