package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var slugInvalid = regexp.MustCompile(`[^a-z0-9._/-]+`)

type frontmatter struct {
	Schema          string            `yaml:"schema"`
	ID              string            `yaml:"id"`
	Title           string            `yaml:"title"`
	Description     string            `yaml:"description"`
	DependsOn       []string          `yaml:"depends_on"`
	AgentTemplate   string            `yaml:"agent_template"`
	Policy          string            `yaml:"policy"`
	RequiredTargets []string          `yaml:"required_targets"`
	Labels          []string          `yaml:"labels"`
	Metadata        map[string]string `yaml:"metadata"`
}

func Parse(path string, data []byte) (Document, []Diagnostic) {
	body := string(data)
	doc := Document{
		Path:     filepath.ToSlash(path),
		ID:       inferID(path),
		Title:    firstHeading(body),
		Metadata: map[string]string{},
	}
	if doc.Title == "" {
		doc.Title = doc.ID
	}
	diagnostics := []Diagnostic{}
	front, strippedBody, hasFront, err := splitFrontmatter(body)
	if err != nil {
		diagnostics = append(diagnostics, diag(path, "malformed-frontmatter", err.Error()))
	} else if hasFront {
		metadata, metadataDiagnostics := parseFrontmatter(path, front)
		diagnostics = append(diagnostics, metadataDiagnostics...)
		applyFrontmatter(&doc, metadata)
		body = strippedBody
	}
	diagnostics = append(diagnostics, validateDocument(doc)...)
	doc.Hash = hashDocument(doc, body)
	return doc, diagnostics
}

func splitFrontmatter(body string) (string, string, bool, error) {
	if body != "---" && !strings.HasPrefix(body, "---\n") && !strings.HasPrefix(body, "---\r\n") {
		return "", body, false, nil
	}
	lines := strings.SplitAfter(body, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], ""), strings.Join(lines[i+1:], ""), true, nil
		}
	}
	return "", body, true, fmt.Errorf("frontmatter is not closed with ---")
}

func parseFrontmatter(path string, front string) (frontmatter, []Diagnostic) {
	diagnostics := []Diagnostic{}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(front), &node); err != nil {
		return frontmatter{}, []Diagnostic{diag(path, "malformed-frontmatter", err.Error())}
	}
	allowed := map[string]bool{
		"schema": true, "id": true, "title": true, "description": true,
		"depends_on": true, "agent_template": true, "policy": true,
		"required_targets": true, "labels": true, "metadata": true,
	}
	if len(node.Content) > 0 && node.Content[0].Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content[0].Content); i += 2 {
			key := node.Content[0].Content[i].Value
			if key == "workstreams" {
				diagnostics = append(
					diagnostics,
					diag(
						path,
						"unsupported-workstreams",
						"workstreams are not supported in bach.plan.v1",
					),
				)
				continue
			}
			if !allowed[key] {
				diagnostics = append(
					diagnostics,
					diag(path, "unknown-frontmatter-field", "unknown Plan frontmatter field "+key),
				)
			}
		}
	}
	var metadata frontmatter
	if err := node.Decode(&metadata); err != nil {
		diagnostics = append(diagnostics, diag(path, "malformed-frontmatter", err.Error()))
	}
	if metadata.Schema != "" && metadata.Schema != SchemaVersion {
		diagnostics = append(
			diagnostics,
			diag(path, "unsupported-schema", "Plan schema must be "+SchemaVersion),
		)
	}
	return metadata, diagnostics
}

func applyFrontmatter(doc *Document, metadata frontmatter) {
	if metadata.ID != "" {
		doc.ID = metadata.ID
	}
	if metadata.Title != "" {
		doc.Title = metadata.Title
	}
	doc.Description = metadata.Description
	doc.DependsOn = append([]string(nil), metadata.DependsOn...)
	doc.AgentTemplate = metadata.AgentTemplate
	doc.Policy = metadata.Policy
	doc.RequiredTargets = append([]string(nil), metadata.RequiredTargets...)
	doc.Labels = append([]string(nil), metadata.Labels...)
	if len(metadata.Metadata) > 0 {
		doc.Metadata = cloneMap(metadata.Metadata)
	}
}

func validateDocument(doc Document) []Diagnostic {
	diagnostics := []Diagnostic{}
	if doc.ID == "" {
		diagnostics = append(diagnostics, diag(doc.Path, "missing-plan-id", "Plan ID is empty"))
	} else if slugify(doc.ID) != doc.ID {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "invalid-plan-id", "Plan ID must be slug-like"),
		)
	}
	if strings.TrimSpace(doc.Title) == "" {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "missing-plan-title", "Plan title is empty"),
		)
	}
	for _, dep := range doc.DependsOn {
		if strings.TrimSpace(dep) == "" {
			diagnostics = append(
				diagnostics,
				diag(doc.Path, "empty-plan-dependency", "Plan dependency must not be empty"),
			)
		}
	}
	return diagnostics
}

func inferID(path string) string {
	path = filepath.ToSlash(path)
	path = strings.TrimSuffix(path, filepath.Ext(path))
	return slugify(path)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(filepath.ToSlash(value)))
	value = slugInvalid.ReplaceAllString(value, "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.Trim(value, "-._")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		text := strings.TrimLeft(trimmed, "#")
		if len(text) == len(trimmed) || !strings.HasPrefix(text, " ") {
			continue
		}
		return strings.TrimSpace(text)
	}
	return ""
}

func hashDocument(doc Document, body string) string {
	builder := strings.Builder{}
	builder.WriteString("id=" + doc.ID + "\n")
	builder.WriteString("title=" + doc.Title + "\n")
	builder.WriteString("description=" + doc.Description + "\n")
	writeList(&builder, "depends_on", doc.DependsOn)
	builder.WriteString("agent_template=" + doc.AgentTemplate + "\n")
	builder.WriteString("policy=" + doc.Policy + "\n")
	writeList(&builder, "required_targets", doc.RequiredTargets)
	writeList(&builder, "labels", doc.Labels)
	keys := make([]string, 0, len(doc.Metadata))
	for key := range doc.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString("metadata." + key + "=" + doc.Metadata[key] + "\n")
	}
	builder.WriteString("body\n")
	builder.WriteString(normalizeBody(body))
	sum := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeList(builder *strings.Builder, key string, values []string) {
	builder.WriteString(key + "=")
	builder.WriteString(strings.Join(values, ","))
	builder.WriteString("\n")
}

func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.TrimSpace(body) + "\n"
}

func diag(path string, code string, message string) Diagnostic {
	return Diagnostic{Severity: "error", File: filepath.ToSlash(path), Code: code, Message: message}
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
