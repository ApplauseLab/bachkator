package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const baselineUsage = "optional baseline file with '<path> <limit>' lines"

func main() {
	limit := flag.Int("limit", 500, "maximum lines per Go file")
	baselinePath := flag.String("baseline", "", baselineUsage)
	flag.Parse()
	if *limit < 1 {
		fatalf("limit must be greater than zero")
	}
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"cmd", "internal"}
	}
	baseline, err := readBaseline(*baselinePath)
	if err != nil {
		fatalf("read baseline: %v", err)
	}

	violations := []string{}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if shouldSkipDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			lines, err := countLines(path)
			if err != nil {
				return err
			}
			allowed := *limit
			baselineLimit, ok := baseline[filepath.ToSlash(path)]
			if ok && baselineLimit > allowed {
				allowed = baselineLimit
			}
			if lines > allowed {
				violations = append(
					violations,
					fmt.Sprintf("%s: %d lines, max %d", path, lines, allowed),
				)
			}
			return nil
		}); err != nil {
			fatalf("check %s: %v", root, err)
		}
	}

	if len(violations) == 0 {
		fmt.Printf("Go file length check passed: new files <= %d lines\n", *limit)
		return
	}
	fmt.Printf("Go file length violations:\n")
	for _, violation := range violations {
		fmt.Println("- " + violation)
	}
	os.Exit(1)
}

func readBaseline(path string) (baseline map[string]int, err error) {
	baseline = map[string]int{}
	if path == "" {
		return baseline, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: expected '<path> <limit>'", path, lineNumber)
		}
		limit, err := strconv.Atoi(fields[1])
		if err != nil || limit < 1 {
			return nil, fmt.Errorf("%s:%d: invalid limit %q", path, lineNumber, fields[1])
		}
		baseline[filepath.ToSlash(fields[0])] = limit
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return baseline, nil
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", ".bach", "dist", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func countLines(path string) (lines int, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
	}
	return lines, scanner.Err()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
