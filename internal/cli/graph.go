package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/applause/bachkator/internal/model"
)

type graphDocument struct {
	Profiles []string    `json:"profiles,omitempty"`
	Nodes    []graphNode `json:"nodes"`
	Edges    []graphEdge `json:"edges"`
}

type graphNode struct {
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Description          string   `json:"description,omitempty"`
	Cost                 string   `json:"cost,omitempty"`
	Lock                 string   `json:"lock,omitempty"`
	Remote               bool     `json:"remote,omitempty"`
	Destructive          bool     `json:"destructive,omitempty"`
	RequiresConfirmation bool     `json:"requires_confirmation,omitempty"`
	Risks                []string `json:"risks,omitempty"`
}

type graphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Type  string `json:"type"`
	Order int    `json:"order,omitempty"`
}

func runGraph(project *Project, format string, args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: bach graph [--format mermaid|json]")
	}
	doc := buildGraphDocument(project)
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

func buildGraphDocument(project *Project) graphDocument {
	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	sort.Strings(names)

	doc := graphDocument{Profiles: append([]string(nil), project.SelectedProfiles...)}
	for _, name := range names {
		target := project.Targets[name]
		spec := target.Spec
		risks := target.RiskLabels
		doc.Nodes = append(doc.Nodes, graphNode{
			Name:                 name,
			Kind:                 string(spec.TargetType()),
			Description:          spec.Metadata.Description,
			Cost:                 spec.Metadata.Cost,
			Lock:                 spec.Runtime.Lock,
			Remote:               containsRisk(risks, "remote"),
			Destructive:          containsRisk(risks, "destructive"),
			RequiresConfirmation: containsRisk(risks, "requires_confirmation"),
			Risks:                risks,
		})
		for _, dependency := range target.DependsOn {
			doc.Edges = append(doc.Edges, graphEdge{From: name, To: dependency, Type: "depends_on"})
		}
		pipeline, _ := spec.Body.(model.PipelineSpec)
		for index, step := range pipeline.Steps {
			doc.Edges = append(
				doc.Edges,
				graphEdge{From: name, To: step, Type: "pipeline_step", Order: index + 1},
			)
		}
	}
	sort.Slice(doc.Edges, func(i, j int) bool {
		left := doc.Edges[i]
		right := doc.Edges[j]
		if left.From != right.From {
			return left.From < right.From
		}
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return left.To < right.To
	})
	return doc
}

func containsRisk(risks []string, risk string) bool {
	for _, value := range risks {
		if value == risk {
			return true
		}
	}
	return false
}

func writeMermaidGraph(doc graphDocument, stdout io.Writer) error {
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

func mermaidLabel(node graphNode) string {
	parts := []string{node.Name, node.Kind}
	if node.Lock != "" {
		parts = append(parts, "lock="+node.Lock)
	}
	if len(node.Risks) > 0 {
		parts = append(parts, "risks="+strings.Join(node.Risks, ","))
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.Join(parts, "\\n"), `"`, `\"`), "|", "\\|")
}
