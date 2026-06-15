package dag

import (
	"context"
	"testing"
)

func TestTopologicalOrderUsesDependenciesBeforeDependents(t *testing.T) {
	g := New[string, string]()
	g.AddEdge("a", "c", "dependency")
	g.AddEdge("b", "c", "dependency")

	order, err := g.TopologicalOrder(nil)
	if err != nil {
		t.Fatal(err)
	}
	assertBefore(t, order, "a", "c")
	assertBefore(t, order, "b", "c")
}

func TestDuplicateEdgesAreIdempotent(t *testing.T) {
	g := New[string, string]()
	g.AddEdge("a", "b", "dependency")
	g.AddEdge("a", "b", "dependency")

	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("edges = %#v, want one edge", edges)
	}
}

func TestAddEdgePreservesVertexMetadata(t *testing.T) {
	g := New[string, string]()
	g.AddVertex(Vertex[string]{ID: "a", Kind: "target"})
	g.AddEdge("a", "b", "dependency")

	vertex, ok := g.Vertex("a")
	if !ok || vertex.Kind != "target" {
		t.Fatalf("vertex = %#v, ok=%v, want target metadata preserved", vertex, ok)
	}
}

func TestValidateAcyclicRejectsCycle(t *testing.T) {
	g := New[string, string]()
	g.AddEdge("a", "b", "dependency")
	g.AddEdge("b", "a", "dependency")

	if err := g.ValidateAcyclic(); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestMultipleEdgeKindsBetweenSameVerticesArePreserved(t *testing.T) {
	g := New[string, string]()
	g.AddEdge("a", "b", "dependency")
	g.AddEdge("a", "b", "pipeline_order")

	edges := g.Edges()
	if len(edges) != 2 {
		t.Fatalf("edges = %#v, want two edge kinds", edges)
	}
}

func TestWalkDedupesMultipleKindsForReadiness(t *testing.T) {
	g := New[string, string]()
	g.AddEdge("a", "b", "dependency")
	g.AddEdge("a", "b", "pipeline_order")

	order := []string{}
	err := Walk(context.Background(), g, WalkOptions[string, string]{
		Parallelism: 1,
		Run: func(_ context.Context, id string) error {
			order = append(order, id)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertBefore(t, order, "a", "b")
}

func assertBefore(t *testing.T, order []string, first string, second string) {
	t.Helper()
	positions := map[string]int{}
	for index, value := range order {
		positions[value] = index
	}
	if positions[first] >= positions[second] {
		t.Fatalf("order = %#v, want %s before %s", order, first, second)
	}
}
