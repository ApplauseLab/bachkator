package factorydaemon

import (
	"strings"
	"testing"
)

// Provider child processes must inherit the daemon environment so provider
// config like token_env can resolve credentials exported by the operator.
func TestProviderEnvironmentInheritsDaemonEnv(t *testing.T) {
	const key = "BACH_FACTORDAEMON_ENV_PROBE"
	t.Setenv(key, "probe-value")

	env := providerEnvironment()
	found := false
	for _, entry := range env {
		if entry == key+"=probe-value" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("provider environment dropped %s; entries=%d sample=%v",
			key, len(env), env[:min(len(env), 3)])
	}
	if len(env) == 0 {
		t.Fatal("provider environment is empty")
	}
	if !strings.Contains(strings.Join(env, "\n"), "PATH=") {
		t.Fatal("provider environment lost PATH")
	}
}
