package docs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.md schemas/*.json
var content embed.FS

type Heading struct {
	File  string
	Level int
	Title string
}

type Section struct {
	File    string
	Heading Heading
	Body    string
}

type Asset struct {
	File  string
	Title string
	Body  string
}

func Headings() ([]Heading, error) {
	sections, err := sections()
	if err != nil {
		return nil, err
	}
	headings := make([]Heading, 0, len(sections))
	for _, section := range sections {
		headings = append(headings, section.Heading)
	}
	return headings, nil
}

func Search(query string) ([]Section, error) {
	sections, err := sections()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return sections, nil
	}

	exact := []Section{}
	matches := []Section{}
	for _, section := range sections {
		heading := strings.ToLower(section.Heading.Title)
		headingSlug := slug(section.Heading.Title)
		if heading == query || headingSlug == query ||
			strings.Contains(query, "-") && strings.HasPrefix(headingSlug, query+"-") {
			exact = append(exact, section)
			continue
		}
		if strings.Contains(heading, query) ||
			strings.Contains(strings.ToLower(section.Body), query) {
			matches = append(matches, section)
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	return matches, nil
}

func Assets() ([]Asset, error) {
	var files []string
	if err := fs.WalkDir(content, "schemas", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)

	out := make([]Asset, 0, len(files))
	for _, file := range files {
		text, err := content.ReadFile(file)
		if err != nil {
			return nil, err
		}
		out = append(out, Asset{File: file, Title: assetTitle(file), Body: string(text)})
	}
	return out, nil
}

func SearchAssets(query string) ([]Asset, error) {
	assets, err := Assets()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return assets, nil
	}

	exact := []Asset{}
	matches := []Asset{}
	for _, asset := range assets {
		file := strings.ToLower(asset.File)
		title := strings.ToLower(asset.Title)
		titleSlug := slug(asset.Title)
		fileSlug := assetFileSlug(asset.File)
		if query == file || query == title || query == titleSlug || query == fileSlug {
			exact = append(exact, asset)
			continue
		}
		if strings.Contains(file, query) ||
			strings.Contains(title, query) ||
			strings.Contains(fileSlug, query) {
			matches = append(matches, asset)
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	return matches, nil
}

func sections() ([]Section, error) {
	var files []string
	if err := fs.WalkDir(content, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") || path == "AGENTS.md" {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)

	var out []Section
	for _, file := range files {
		text, err := content.ReadFile(file)
		if err != nil {
			return nil, err
		}
		out = append(out, parseSections(file, string(text))...)
	}
	return out, nil
}

func parseSections(file string, text string) []Section {
	lines := strings.Split(text, "\n")
	var sections []Section
	var current *Section
	inFence := false

	flush := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimSpace(current.Body)
		sections = append(sections, *current)
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			if current != nil {
				current.Body += line + "\n"
			}
			continue
		}
		if inFence {
			if current != nil {
				current.Body += line + "\n"
			}
			continue
		}
		level, title, ok := parseHeading(line)
		if ok {
			flush()
			current = &Section{File: file, Heading: Heading{File: file, Level: level, Title: title}}
			continue
		}
		if current != nil {
			current.Body += line + "\n"
		}
	}
	flush()
	return sections
}

func parseHeading(line string) (int, string, bool) {
	if strings.HasPrefix(line, "# vim:") {
		return 0, "", false
	}
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(line[level+1:])
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func FormatHeadings(headings []Heading) string {
	var builder strings.Builder
	builder.WriteString("Reference topics:\n")
	lastFile := ""
	for _, heading := range headings {
		if heading.File != lastFile {
			if lastFile != "" {
				builder.WriteString("\n")
			}
			fmt.Fprintf(&builder, "%s\n", heading.File)
			lastFile = heading.File
		}
		indent := strings.Repeat("  ", heading.Level-1)
		fmt.Fprintf(&builder, "  %s%s\n", indent, slug(heading.Title))
	}
	assets, err := Assets()
	if err == nil && len(assets) > 0 {
		builder.WriteString("\nEmbedded assets:\n")
		for _, asset := range assets {
			fmt.Fprintf(&builder, "  %s\n", assetFileSlug(asset.File))
		}
	}
	return builder.String()
}

func FormatAssets(assets []Asset) string {
	var builder strings.Builder
	for index, asset := range assets {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		fmt.Fprintf(&builder, "# %s\n\n", asset.Title)
		builder.WriteString("```json\n")
		builder.WriteString(strings.TrimSpace(asset.Body))
		builder.WriteString("\n```\n")
	}
	return builder.String()
}

func assetTitle(file string) string {
	name := strings.TrimPrefix(file, "schemas/")
	name = strings.TrimSuffix(name, ".json")
	name = strings.TrimSuffix(name, ".schema")
	words := strings.Fields(strings.ReplaceAll(name, "-", " ") + " schema")
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func assetFileSlug(file string) string {
	name := strings.TrimPrefix(file, "schemas/")
	name = strings.TrimSuffix(name, ".json")
	name = strings.TrimSuffix(name, ".schema")
	return slug(name + " schema")
}

func slug(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	return strings.TrimSuffix(builder.String(), "-")
}

func FormatSections(sections []Section) string {
	var builder strings.Builder
	for index, section := range sections {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		fmt.Fprintf(
			&builder,
			"%s %s\n\n",
			strings.Repeat("#", section.Heading.Level),
			section.Heading.Title,
		)
		if section.Body != "" {
			builder.WriteString(section.Body)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
