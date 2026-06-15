package runner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadDotenv(root, path string) (map[string]string, error) {
	values := map[string]string{}
	defaultPath := filepath.Join(root, ".env")
	if defaultValues, err := parseOptionalEnvFile(defaultPath); err != nil {
		return nil, err
	} else {
		mergeEnv(values, defaultValues)
	}

	if path != "" {
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		overrideValues, err := parseEnvFile(path)
		if err != nil {
			return nil, err
		}
		mergeEnv(values, overrideValues)
	}
	return values, nil
}

func parseOptionalEnvFile(path string) (map[string]string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseEnvFile(path)
}

func mergeEnv(dst, src map[string]string) {
	for key, value := range src {
		dst[key] = value
	}
}

func parseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, lineNumber)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNumber)
		}
		value = strings.TrimSpace(value)
		unquoted, err := unquoteEnvValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		values[key] = unquoted
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func unquoteEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	quote := value[0]
	if quote != '\'' && quote != '"' {
		return stripEnvComment(value), nil
	}
	if len(value) < 2 || value[len(value)-1] != quote {
		return "", fmt.Errorf("unterminated quoted value")
	}
	return value[1 : len(value)-1], nil
}

func stripEnvComment(value string) string {
	for index := 0; index < len(value); index++ {
		if value[index] == '#' && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
			return strings.TrimSpace(value[:index])
		}
	}
	return value
}
