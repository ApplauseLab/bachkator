package docs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.md
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

func sections() ([]Section, error) {
	var files []string
	if err := fs.WalkDir(content, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
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
	return builder.String()
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
