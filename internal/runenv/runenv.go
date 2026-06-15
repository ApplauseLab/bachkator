package runenv

import (
	"os"
	"sort"
	"strings"
)

// Expand applies Bach runtime command environment expansion to value.
func Expand(value string, env map[string]string) string {
	value = strings.ReplaceAll(value, "$(RUN_DIRECTORY)", env["BACH_RUN_DIRECTORY"])
	return os.Expand(value, func(key string) string { return env[key] })
}

// ExpandSlice applies Expand to each value while preserving order.
func ExpandSlice(values []string, env map[string]string) []string {
	expanded := make([]string, len(values))
	for index, value := range values {
		expanded[index] = Expand(value, env)
	}
	return expanded
}

// List converts env to a deterministic KEY=value process environment list.
func List(env map[string]string) []string {
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
