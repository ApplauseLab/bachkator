package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type checkstyle struct {
	XMLName xml.Name         `xml:"checkstyle"`
	Version string           `xml:"version,attr,omitempty"`
	Files   []checkstyleFile `xml:"file"`
}

type checkstyleFile struct {
	Name   string            `xml:"name,attr"`
	Errors []checkstyleError `xml:"error"`
}

type checkstyleError struct {
	Line     string `xml:"line,attr,omitempty"`
	Column   string `xml:"column,attr,omitempty"`
	Severity string `xml:"severity,attr,omitempty"`
	Message  string `xml:"message,attr,omitempty"`
	Source   string `xml:"source,attr,omitempty"`
}

func main() {
	inPath := flag.String("in", "", "input Checkstyle XML path")
	outPath := flag.String("out", "", "output Checkstyle XML path")
	limit := flag.Int("limit", 10, "maximum findings to keep")
	flag.Parse()

	if *inPath == "" || *outPath == "" {
		fatalf("usage: bach-lint-cap --in <path> --out <path> [--limit 10]")
	}
	if *limit < 0 {
		fatalf("limit must be >= 0")
	}

	data, err := os.ReadFile(*inPath)
	if err != nil {
		fatalf("read input: %v", err)
	}
	var doc checkstyle
	if err := xml.Unmarshal(data, &doc); err != nil {
		fatalf("parse input: %v", err)
	}
	total := countFindings(doc.Files)
	doc.Files = cappedFiles(doc.Files, *limit)
	if err := writeCheckstyle(*outPath, doc); err != nil {
		fatalf("write output: %v", err)
	}
	printSummary(doc.Files, total, *limit)
}

func countFindings(files []checkstyleFile) int {
	total := 0
	for _, file := range files {
		total += len(file.Errors)
	}
	return total
}

func cappedFiles(files []checkstyleFile, limit int) []checkstyleFile {
	if limit == 0 {
		return nil
	}
	kept := make([]checkstyleFile, 0, len(files))
	remaining := limit
	for _, file := range files {
		if remaining <= 0 {
			break
		}
		if len(file.Errors) == 0 {
			continue
		}
		if len(file.Errors) > remaining {
			file.Errors = append([]checkstyleError(nil), file.Errors[:remaining]...)
		} else {
			file.Errors = append([]checkstyleError(nil), file.Errors...)
		}
		kept = append(kept, file)
		remaining -= len(file.Errors)
	}
	return kept
}

func writeCheckstyle(path string, doc checkstyle) error {
	if doc.XMLName.Local == "" {
		doc.XMLName = xml.Name{Local: "checkstyle"}
	}
	if doc.Version == "" {
		doc.Version = "5.0"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append([]byte(xml.Header), data...)
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func printSummary(files []checkstyleFile, total int, limit int) {
	shown := countFindings(files)
	if total == 0 {
		return
	}
	if total > shown {
		fmt.Printf("showing first %d of %d golangci-lint issue(s)\n", shown, total)
	} else {
		fmt.Printf("%d issue(s)\n", shown)
	}
	for _, file := range files {
		for _, finding := range file.Errors {
			line := finding.Line
			if line == "" {
				line = "0"
			}
			message := finding.Message
			if finding.Source != "" {
				message += " (" + finding.Source + ")"
			}
			fmt.Printf("%s:%s: %s\n", file.Name, line, message)
		}
	}
	if total > limit {
		fmt.Println("lint output capped at " + strconv.Itoa(limit) + " total issue(s)")
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
