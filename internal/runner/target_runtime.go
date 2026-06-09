package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/applause/bachkator/internal/model"
	targetpkg "github.com/applause/bachkator/internal/target"
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

func targetLabel(target *Target) string {
	return target.Spec().Label()
}

func targetOperation(
	ctx context.Context,
	target *Target,
	env map[string]string,
) (targetpkg.RunDescription, error) {
	spec := target.Spec()
	handler, err := targetpkg.BuiltinTargetRegistry().Handler(spec.TargetType())
	if err != nil {
		return targetpkg.RunDescription{}, err
	}
	return handler.Describe(ctx, targetpkg.DescribeRequest{Spec: spec, Env: env})
}

func executeTarget(ctx context.Context, target *Target, req targetpkg.ExecuteRequest) error {
	spec := target.Spec()
	handler, err := targetpkg.BuiltinTargetRegistry().Handler(spec.TargetType())
	if err != nil {
		return err
	}
	req.Spec = spec
	return handler.Execute(ctx, req)
}

func targetRunnable(target *Target) bool {
	spec := target.Spec()
	handler, err := targetpkg.BuiltinTargetRegistry().Handler(spec.TargetType())
	return err == nil && handler.Runnable(spec)
}

func targetCacheable(target *Target) bool {
	return target.Spec().Cacheable()
}

func targetFresh(target *Target, root string, record StateRecord, fingerprint string) bool {
	if record.Fingerprint == "" || record.Fingerprint != fingerprint {
		return false
	}
	for _, output := range target.Outputs {
		if _, err := os.Stat(absPath(root, output)); err != nil {
			return false
		}
	}
	return true
}

func targetStaleReasons(
	target *Target,
	root string,
	record StateRecord,
	fingerprint string,
	parts map[string]string,
	force bool,
) []string {
	var reasons []string
	if force {
		reasons = append(reasons, "forced run")
	}
	if record.Fingerprint == "" {
		reasons = append(reasons, "no cache record")
	} else if record.Fingerprint != fingerprint {
		if len(record.FingerprintParts) == 0 {
			reasons = append(reasons, "fingerprint changed")
		} else {
			if record.FingerprintParts["inputs"] != "" &&
				record.FingerprintParts["inputs"] != parts["inputs"] {
				reasons = append(reasons, "changed input")
			}
			if record.FingerprintParts["env"] != "" &&
				record.FingerprintParts["env"] != parts["env"] {
				reasons = append(reasons, "changed env var")
			}
			previousOperation := record.FingerprintParts["operation"]
			if previousOperation == "" {
				previousOperation = record.FingerprintParts["command"]
			}
			currentOperation := parts["operation"]
			if currentOperation == "" {
				currentOperation = parts["command"]
			}
			if previousOperation != "" && previousOperation != currentOperation {
				reasons = append(reasons, "changed operation")
			}
			if record.FingerprintParts["dependencies"] != "" &&
				record.FingerprintParts["dependencies"] != parts["dependencies"] {
				reasons = append(reasons, "dependency fingerprint change")
			}
			if record.FingerprintParts["git"] != "" &&
				record.FingerprintParts["git"] != parts["git"] {
				reasons = append(reasons, "dirty Git state")
			}
		}
	}
	for _, output := range target.Outputs {
		if _, err := os.Stat(absPath(root, output)); err != nil {
			reasons = append(reasons, "missing output")
			break
		}
	}
	if len(reasons) == 0 && record.Fingerprint != fingerprint {
		reasons = append(reasons, "fingerprint changed")
	}
	return dedupeStrings(reasons)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var deduped []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		deduped = append(deduped, value)
	}
	return deduped
}

func targetFingerprintParts(
	project *Project,
	target *Target,
	gitContext GitContext,
	dotenv map[string]string,
	dependencyFingerprints map[string]string,
) (string, map[string]string, error) {
	spec := target.Spec()
	parts := map[string]string{}

	operationHash := sha256.New()
	writeHash(operationHash, "name", spec.Name)
	writeHash(operationHash, "type", string(spec.TargetType()))
	writeHash(operationHash, "lock", spec.Runtime.Lock)
	operationPartValues := targetOperationFingerprintParts(spec)
	operationPartNames := make([]string, 0, len(operationPartValues))
	for key := range operationPartValues {
		operationPartNames = append(operationPartNames, key)
	}
	sort.Strings(operationPartNames)
	for _, key := range operationPartNames {
		writeHash(operationHash, key, operationPartValues[key])
	}
	writeCompletionChecks(operationHash, "success-when", spec.Contract.SuccessWhen)
	writeCompletionChecks(operationHash, "fail-when", spec.Contract.FailWhen)
	parts["operation"] = hex.EncodeToString(operationHash.Sum(nil))

	envHash := sha256.New()
	writeHash(envHash, "project-env", strings.Join(project.Env, "\x00"))
	writeHash(envHash, "profiles", strings.Join(project.SelectedProfiles, "\x00"))
	writeHash(envHash, "profile-env", strings.Join(project.ProfileEnv, "\x00"))
	writeHash(envHash, "env", strings.Join(spec.Runtime.Env, "\x00"))
	dotenvKeys := make([]string, 0, len(dotenv))
	for key := range dotenv {
		dotenvKeys = append(dotenvKeys, key)
	}
	sort.Strings(dotenvKeys)
	for _, key := range dotenvKeys {
		writeHash(envHash, "dotenv", key+"="+dotenv[key])
	}
	parts["env"] = hex.EncodeToString(envHash.Sum(nil))

	gitHash := sha256.New()
	writeHash(gitHash, "git-branch", gitContext.Branch)
	writeHash(gitHash, "git-commit", gitContext.Commit)
	writeHash(gitHash, "git-dirty", fmt.Sprint(gitContext.Dirty))
	writeHash(gitHash, "git-staged", strings.Join(gitContext.StagedFiles, "\x00"))
	writeHash(gitHash, "git-unstaged", strings.Join(gitContext.UnstagedFiles, "\x00"))
	writeHash(gitHash, "git-untracked", strings.Join(gitContext.Untracked, "\x00"))
	parts["git"] = hex.EncodeToString(gitHash.Sum(nil))

	inputHash := sha256.New()
	writeHash(inputHash, "inputs", strings.Join(spec.Cache.Inputs, "\x00"))

	dependencyHash := sha256.New()
	deps := append([]string(nil), target.DependsOn...)
	sort.Strings(deps)
	for _, dep := range deps {
		writeHash(dependencyHash, "dep", dep)
		writeHash(dependencyHash, "dep-fingerprint", dependencyFingerprints[dep])
	}
	parts["dependencies"] = hex.EncodeToString(dependencyHash.Sum(nil))

	inputFiles, err := expandFiles(project.Root, resolveInputs(project, spec.Cache.Inputs))
	if err != nil {
		return "", nil, err
	}
	for _, path := range inputFiles {
		hashPath(inputHash, project.Root, path)
	}
	parts["inputs"] = hex.EncodeToString(inputHash.Sum(nil))

	outputHash := sha256.New()
	writeHash(outputHash, "outputs", strings.Join(spec.Cache.Outputs, "\x00"))
	for _, output := range spec.Cache.Outputs {
		if _, err := os.Stat(absPath(project.Root, output)); err != nil {
			writeHash(outputHash, "missing-output", output)
		}
	}
	parts["outputs"] = hex.EncodeToString(outputHash.Sum(nil))

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

func targetOperationFingerprintParts(spec model.TargetSpec) map[string]string {
	switch body := spec.Body.(type) {
	case model.ShellSpec:
		return map[string]string{
			"command": strings.Join(body.Command, "\x00"),
			"shell":   body.Shell,
			"workdir": body.WorkDir,
		}
	case model.ImageSpec:
		return map[string]string{
			"builder":    body.Builder,
			"image":      body.Image,
			"tags":       strings.Join(body.Tags, "\x00"),
			"dockerfile": body.Dockerfile,
			"context":    body.Context,
			"platform":   body.Platform,
			"push":       fmt.Sprint(body.Push),
			"build-args": strings.Join(body.BuildArgs, "\x00"),
		}
	case model.PipelineSpec:
		return map[string]string{
			"steps": strings.Join(body.Steps, "\x00"),
		}
	default:
		return nil
	}
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

func evaluateCompletionContract(
	ctx context.Context,
	project *Project,
	target *Target,
	workdir string,
	env map[string]string,
	logPath string,
	logFile io.Writer,
) error {
	spec := target.Spec()
	for _, check := range spec.Contract.FailWhen {
		matched, detail, err := evaluateCompletionCheck(
			ctx,
			project,
			check,
			workdir,
			env,
			logPath,
			logFile,
		)
		if err != nil {
			return err
		}
		if matched {
			return fmt.Errorf("target %q fail_when matched: %s", target.Name, detail)
		}
	}
	for _, check := range spec.Contract.SuccessWhen {
		matched, detail, err := evaluateCompletionCheck(
			ctx,
			project,
			check,
			workdir,
			env,
			logPath,
			logFile,
		)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("target %q success_when not satisfied: %s", target.Name, detail)
		}
	}
	return nil
}

func evaluateCompletionCheck(
	ctx context.Context,
	project *Project,
	check model.CompletionCheckSpec,
	workdir string,
	env map[string]string,
	logPath string,
	logFile io.Writer,
) (bool, string, error) {
	if check.OutputContains != "" {
		matched, err := logContains(logPath, check.OutputContains)
		return matched, "output_contains " + strconvQuote(check.OutputContains), err
	}
	if check.FileExists != "" {
		path := absPath(project.Root, check.FileExists)
		_, err := os.Stat(path)
		if err == nil {
			return true, "file_exists " + strconvQuote(check.FileExists), nil
		}
		if os.IsNotExist(err) {
			return false, "file_exists " + strconvQuote(check.FileExists), nil
		}
		return false, "", err
	}
	return runCompletionCommand(ctx, check.Command, workdir, env, logFile)
}

func logContains(logPath string, needle string) (bool, error) {
	contents, err := os.ReadFile(logPath)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(contents), needle), nil
}

func runCompletionCommand(
	ctx context.Context,
	command []string,
	workdir string,
	env map[string]string,
	logFile io.Writer,
) (bool, string, error) {
	if len(command) == 0 {
		return false, "command []", nil
	}
	expanded := expandEnvSlice(command, env)
	cmd := exec.CommandContext(ctx, expanded[0], expanded[1:]...)
	cmd.Dir = workdir
	cmd.Env = envList(env)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	logf(logFile, "[contract] %s\n", strings.Join(expanded, " "))
	if err := cmd.Run(); err != nil {
		return false, "command " + strings.Join(expanded, " "), nil
	}
	return true, "command " + strings.Join(expanded, " "), nil
}

func expandEnvSlice(values []string, env map[string]string) []string {
	expanded := make([]string, len(values))
	for index, value := range values {
		expanded[index] = expandEnv(value, env)
	}
	return expanded
}

func expandEnv(value string, env map[string]string) string {
	value = strings.ReplaceAll(value, "$(RUN_DIRECTORY)", env["BACH_RUN_DIRECTORY"])
	return os.Expand(value, func(key string) string { return env[key] })
}

func envList(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+env[key])
	}
	return values
}

func strconvQuote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}
