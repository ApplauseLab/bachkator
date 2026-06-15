package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	files, err := filepath.Glob("docs/reference/*.md")
	if err != nil {
		fatal(err)
	}
	sort.Strings(files)
	var out bytes.Buffer
	out.WriteString(
		"<!-- Generated from docs/reference/*.md. Edit fragments, then run shell/docs-generate. -->\n\n",
	)
	for _, file := range files {
		if filepath.Base(file) == "AGENTS.md" {
			continue
		}
		contents, err := os.ReadFile(file)
		if err != nil {
			fatal(err)
		}
		out.WriteString(strings.TrimSpace(string(contents)))
		out.WriteString("\n\n")
	}
	if err := os.WriteFile("docs/reference.md", out.Bytes(), 0o600); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
