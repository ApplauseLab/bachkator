package graph

import (
	"sort"

	"github.com/applauselab/bachkator/internal/model"
)

func BuildDocument(project *Project) GraphDocument {
	if project == nil {
		return GraphDocument{}
	}
	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	sort.Strings(names)

	doc := GraphDocument{Profiles: append([]string(nil), project.SelectedProfiles...)}
	for _, name := range names {
		target := project.Targets[name]
		if target == nil {
			continue
		}
		spec := target.Spec()
		risk, err := TargetRisk(project, name)
		if err != nil {
			continue
		}
		risks := risk.Labels()
		doc.Nodes = append(doc.Nodes, GraphNode{
			Name:                 name,
			Kind:                 string(spec.TargetType()),
			Description:          spec.Metadata.Description,
			Cost:                 spec.Metadata.Cost,
			Lock:                 spec.Runtime.Lock,
			Remote:               risk.Remote,
			Destructive:          risk.Destructive,
			RequiresConfirmation: risk.RequiresConfirmation,
			Risks:                risks,
		})
		for _, dependency := range target.DependsOn {
			doc.Edges = append(doc.Edges, GraphEdge{From: name, To: dependency, Type: "depends_on"})
		}
		pipeline, _ := spec.Body.(model.PipelineSpec)
		for index, step := range pipeline.Steps {
			doc.Edges = append(
				doc.Edges,
				GraphEdge{From: name, To: step, Type: "pipeline_step", Order: index + 1},
			)
		}
	}
	sortGraphEdges(doc.Edges)
	return doc
}

func sortGraphEdges(edges []GraphEdge) {
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i]
		right := edges[j]
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
}
