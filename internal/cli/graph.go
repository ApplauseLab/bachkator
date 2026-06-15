package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/applauselab/bachkator/internal/graph"
)

func runGraph(
	project *Project,
	deps Dependencies,
	format string,
	args []string,
	stdout io.Writer,
) error {
	if len(args) > 0 {
		return UsageErrorf("usage: bach graph [--format mermaid|json]")
	}
	if deps.GraphDocument == nil {
		return fmt.Errorf("graph service is not configured")
	}
	doc, err := deps.GraphDocument(project)
	if err != nil {
		return err
	}
	switch format {
	case "", "mermaid":
		return writeMermaidGraph(doc, stdout)
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(doc)
	default:
		return fmt.Errorf("unsupported graph format %q", format)
	}
}

func writeMermaidGraph(doc graph.GraphDocument, stdout io.Writer) error {
	if _, err := fmt.Fprintln(stdout, "flowchart TD"); err != nil {
		return err
	}
	ids := make(map[string]string, len(doc.Nodes))
	for index, node := range doc.Nodes {
		id := fmt.Sprintf("n%d", index)
		ids[node.Name] = id
		if _, err := fmt.Fprintf(stdout, "  %s[\"%s\"]\n", id, mermaidLabel(node)); err != nil {
			return err
		}
	}
	for _, edge := range doc.Edges {
		fromID, fromOK := ids[edge.From]
		toID, toOK := ids[edge.To]
		if !fromOK || !toOK {
			continue
		}
		label := edge.Type
		if edge.Order > 0 {
			label = fmt.Sprintf("%s %d", edge.Type, edge.Order)
		}
		if _, err := fmt.Fprintf(stdout, "  %s -- %s --> %s\n", fromID, label, toID); err != nil {
			return err
		}
	}
	return nil
}

func mermaidLabel(node graph.GraphNode) string {
	parts := []string{node.Name, node.Kind}
	if node.Lock != "" {
		parts = append(parts, "lock="+node.Lock)
	}
	if len(node.Risks) > 0 {
		parts = append(parts, "risks="+strings.Join(node.Risks, ","))
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.Join(parts, "\\n"), `"`, `\"`), "|", "\\|")
}
