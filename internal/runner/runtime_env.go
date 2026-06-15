package runner

import (
	"os"
	"path/filepath"
	"strings"
)

func projectRuntimeEnv(project *Project) []string {
	env := append([]string(nil), project.Env...)
	env = append(env, project.ProfileEnv...)
	return env
}

func commandEnv(
	gitContext GitContext,
	dotenv map[string]string,
	projectEnv []string,
	envLayers ...[]string,
) map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	for key, value := range dotenv {
		env[key] = value
	}
	for _, entry := range projectEnv {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	for _, entry := range gitContext.Env() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	for _, envLayer := range envLayers {
		for _, entry := range envLayer {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				env[key] = value
			}
		}
	}
	return env
}

func absPath(root, path string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
