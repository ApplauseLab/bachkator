package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

func targetFingerprintParts(
	targets TargetHandlers,
	project *Project,
	target *Target,
	dotenv map[string]string,
	dependencyFingerprints map[string]string,
) (string, map[string]string, error) {
	spec := target.Spec()
	parts := map[string]string{}

	operationHash := sha256.New()
	writeHash(operationHash, "name", spec.Name)
	writeHash(operationHash, "type", string(spec.TargetType()))
	writeHash(operationHash, "lock", spec.Runtime.Lock)
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return "", nil, err
	}
	writeOperationParts(operationHash, handler.FingerprintParts(spec.Body))
	writeCompletionChecks(operationHash, "success-when", spec.Contract.SuccessWhen)
	writeCompletionChecks(operationHash, "fail-when", spec.Contract.FailWhen)
	parts["operation"] = hex.EncodeToString(operationHash.Sum(nil))

	parts["env"] = envFingerprintPart(project, spec.Runtime.Env, dotenv)
	inputHash := sha256.New()
	inputRefs := inputReferences(spec)
	writeHash(inputHash, "inputs", strings.Join(inputRefs, "\x00"))
	parts["dependencies"] = dependencyFingerprintPart(target, dependencyFingerprints)

	inputFiles, err := expandFiles(project.Root, resolveInputs(project, inputRefs))
	if err != nil {
		return "", nil, err
	}
	for _, path := range inputFiles {
		hashPath(inputHash, project.Root, path)
	}
	parts["inputs"] = hex.EncodeToString(inputHash.Sum(nil))
	parts["outputs"] = outputFingerprintPart(project.Root, spec.Cache.Outputs)

	h := sha256.New()
	partNames := make([]string, 0, len(parts))
	for name := range parts {
		partNames = append(partNames, name)
	}
	sort.Strings(partNames)
	for _, name := range partNames {
		writeHash(h, name, parts[name])
	}
	return hex.EncodeToString(h.Sum(nil)), parts, nil
}

func writeOperationParts(h io.Writer, values map[string]string) {
	names := make([]string, 0, len(values))
	for key := range values {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		writeHash(h, key, values[key])
	}
}

func envFingerprintPart(project *Project, targetEnv []string, dotenv map[string]string) string {
	envHash := sha256.New()
	writeHash(envHash, "project-env", strings.Join(project.Env, "\x00"))
	writeHash(envHash, "profiles", strings.Join(project.SelectedProfiles, "\x00"))
	writeHash(envHash, "profile-env", strings.Join(project.ProfileEnv, "\x00"))
	writeHash(envHash, "env", strings.Join(targetEnv, "\x00"))
	dotenvKeys := make([]string, 0, len(dotenv))
	for key := range dotenv {
		dotenvKeys = append(dotenvKeys, key)
	}
	sort.Strings(dotenvKeys)
	for _, key := range dotenvKeys {
		writeHash(envHash, "dotenv", key+"="+dotenv[key])
	}
	return hex.EncodeToString(envHash.Sum(nil))
}

func inputReferences(spec model.TargetSpec) []string {
	inputRefs := append([]string(nil), spec.Cache.Inputs...)
	inputHash := sha256.New()
	for _, policy := range spec.Quality.RegoPolicies {
		inputRefs = append(inputRefs, policy.Path)
		writeHash(inputHash, "rego-package", policy.Path+"="+policy.Package)
		writeHash(inputHash, "rego-allow", policy.Path+"="+policy.Allow)
		writeHash(inputHash, "rego-findings", policy.Path+"="+policy.Findings)
	}
	return inputRefs
}

func dependencyFingerprintPart(target *Target, dependencyFingerprints map[string]string) string {
	dependencyHash := sha256.New()
	deps := append([]string(nil), target.DependsOn...)
	sort.Strings(deps)
	for _, dep := range deps {
		writeHash(dependencyHash, "dep", dep)
		writeHash(dependencyHash, "dep-fingerprint", dependencyFingerprints[dep])
	}
	return hex.EncodeToString(dependencyHash.Sum(nil))
}

func outputFingerprintPart(root string, outputs []string) string {
	outputHash := sha256.New()
	writeHash(outputHash, "outputs", strings.Join(outputs, "\x00"))
	for _, output := range outputs {
		if _, err := os.Stat(absPath(root, output)); err != nil {
			writeHash(outputHash, "missing-output", output)
		}
	}
	return hex.EncodeToString(outputHash.Sum(nil))
}

func writeCompletionChecks(h io.Writer, prefix string, checks []model.CompletionCheckSpec) {
	for _, check := range checks {
		writeHash(h, prefix, check.OutputContains)
		writeHash(h, prefix, check.FileExists)
		writeHash(h, prefix, strings.Join(check.Command, "\x00"))
	}
}

func writeHash(w io.Writer, parts ...string) {
	for _, part := range parts {
		_, _ = io.WriteString(w, part)
		_, _ = io.WriteString(w, "\x00")
	}
}

func resolveInputs(project *Project, inputs []string) []string {
	return resolveInputsSeen(project, inputs, map[string]bool{})
}

func resolveInputsSeen(project *Project, inputs []string, seen map[string]bool) []string {
	var paths []string
	for _, input := range inputs {
		if named, ok := project.Inputs[input]; ok {
			if seen[input] {
				continue
			}
			seen[input] = true
			paths = append(paths, resolveInputsSeen(project, inputPaths(named), seen)...)
			continue
		}
		if _, ok := project.Resources[input]; ok {
			continue
		}
		paths = append(paths, input)
	}
	return paths
}

func inputPaths(input *Input) []string {
	if input.Src != "" {
		return []string{input.Src}
	}
	return append([]string(nil), input.Srcs...)
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
	abs := absPath(root, path)
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	writeHash(h, path)
	_, _ = h.Write(data)
	_, _ = io.WriteString(h, "\x00")
}
