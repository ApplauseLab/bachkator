package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunnerLoadsDefaultDotenv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("APP_ENV=local\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	project := envProject(dir)

	if err := (Runner{Stdout: os.Stdout, Stderr: os.Stderr}).Run(
		context.Background(),
		project,
		"shell/write-env",
	); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dir, "env.txt"), "local")
}

func TestRunnerEnvFileOverlaysDefaultDotenvFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("APP_ENV=default\nSHARED=from-default\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".env.local"),
		[]byte("APP_ENV=override\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	project := envProject(dir)
	project.Targets["shell/write-env"] = shellTarget(
		"shell/write-env",
		"printf '%s:%s' \"$APP_ENV\" \"$SHARED\" > env.txt",
	)

	r := Runner{EnvFile: ".env.local", Stdout: os.Stdout, Stderr: os.Stderr}
	if err := r.Run(context.Background(), project, "shell/write-env"); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dir, "env.txt"), "override:from-default")
}

func TestTargetEnvOverridesDotenv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("APP_ENV=dotenv\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	project := envProject(dir)
	project.Targets["shell/write-env"] = shellTarget(
		"shell/write-env",
		"printf '%s' \"$APP_ENV\" > env.txt",
		withEnv("APP_ENV=target"),
	)

	if err := (Runner{Stdout: os.Stdout, Stderr: os.Stderr}).Run(
		context.Background(),
		project,
		"shell/write-env",
	); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dir, "env.txt"), "target")
}

func TestProjectEnvOverridesDotenvAndTargetEnvOverridesProjectEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("APP_ENV=dotenv\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	project := envProject(dir)
	project.Env = []string{"APP_ENV=project"}

	if err := (Runner{Stdout: os.Stdout, Stderr: os.Stderr}).Run(
		context.Background(),
		project,
		"shell/write-env",
	); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dir, "env.txt"), "project")

	project.Targets["shell/write-env"] = shellTarget(
		"shell/write-env",
		"printf '%s' \"$APP_ENV\" > env.txt",
		withEnv("APP_ENV=target"),
	)
	if err := (Runner{Force: true, Stdout: os.Stdout, Stderr: os.Stderr}).Run(
		context.Background(),
		project,
		"shell/write-env",
	); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dir, "env.txt"), "target")
}

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\nPLAIN=value # trailing comment\nexport EXPORTED=yes\nSINGLE='quoted value'\nDOUBLE=\"quoted # value\"\nEMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := parseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "yes",
		"SINGLE":   "quoted value",
		"DOUBLE":   "quoted # value",
		"EMPTY":    "",
	}
	for key, value := range want {
		if values[key] != value {
			t.Fatalf("%s = %q, want %q", key, values[key], value)
		}
	}
}

func envProject(dir string) *Project {
	return &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/write-env": shellTarget("shell/write-env", "printf '%s' \"$APP_ENV\" > env.txt"),
		},
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, string(got), want)
	}
}
