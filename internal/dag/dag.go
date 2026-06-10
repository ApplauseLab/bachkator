package dag

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

type Graph[V comparable, K comparable] struct {
	vertices map[V]Vertex[V]
	edges    map[V]map[V]map[K]Edge[V, K]
}

type Vertex[V comparable] struct {
	ID   V
	Kind string
}

type Edge[V comparable, K comparable] struct {
	From V
	To   V
	Kind K
}

type EdgeFilter[V comparable, K comparable] func(Edge[V, K]) bool

type WalkOptions[V comparable, K comparable] struct {
	Parallelism int
	Less        func(a, b V) bool
	EdgeFilter  EdgeFilter[V, K]
	// TryReserve may reserve external resources for a ready vertex. When it
	// returns true, Walk will call Run exactly once for that vertex; Run is then
	// responsible for releasing any reservation on success, error, or cancellation.
	TryReserve     func(V) bool
	BlockedChanged func() <-chan struct{}
	Run            func(context.Context, V) error
}

func New[V comparable, K comparable]() *Graph[V, K] {
	return &Graph[V, K]{
		vertices: map[V]Vertex[V]{},
		edges:    map[V]map[V]map[K]Edge[V, K]{},
	}
}

func (g *Graph[V, K]) AddVertex(vertex Vertex[V]) {
	var zero V
	if vertex.ID == zero {
		return
	}
	if existing, exists := g.vertices[vertex.ID]; exists {
		if vertex.Kind == "" {
			return
		}
		if existing.Kind != "" && existing.Kind != vertex.Kind {
			return
		}
	}
	g.vertices[vertex.ID] = vertex
}

func (g *Graph[V, K]) AddEdge(from, to V, kind K) {
	var zero V
	if from == zero || to == zero || from == to {
		return
	}
	g.AddVertex(Vertex[V]{ID: from})
	g.AddVertex(Vertex[V]{ID: to})
	if g.edges[from] == nil {
		g.edges[from] = map[V]map[K]Edge[V, K]{}
	}
	if g.edges[from][to] == nil {
		g.edges[from][to] = map[K]Edge[V, K]{}
	}
	g.edges[from][to][kind] = Edge[V, K]{From: from, To: to, Kind: kind}
}

func (g *Graph[V, K]) Vertex(id V) (Vertex[V], bool) {
	vertex, ok := g.vertices[id]
	return vertex, ok
}

func (g *Graph[V, K]) Vertices() []Vertex[V] {
	vertices := make([]Vertex[V], 0, len(g.vertices))
	for _, vertex := range g.vertices {
		vertices = append(vertices, vertex)
	}
	sort.SliceStable(vertices, func(i, j int) bool {
		return fmt.Sprint(vertices[i].ID) < fmt.Sprint(vertices[j].ID)
	})
	return vertices
}

func (g *Graph[V, K]) Edges(filters ...EdgeFilter[V, K]) []Edge[V, K] {
	var filter EdgeFilter[V, K]
	if len(filters) > 0 {
		filter = filters[0]
	}
	edges := []Edge[V, K]{}
	for _, byTo := range g.edges {
		for _, byKind := range byTo {
			for _, edge := range byKind {
				if filter == nil || filter(edge) {
					edges = append(edges, edge)
				}
			}
		}
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if fmt.Sprint(edges[i].From) == fmt.Sprint(edges[j].From) {
			if fmt.Sprint(edges[i].To) == fmt.Sprint(edges[j].To) {
				return fmt.Sprint(edges[i].Kind) < fmt.Sprint(edges[j].Kind)
			}
			return fmt.Sprint(edges[i].To) < fmt.Sprint(edges[j].To)
		}
		return fmt.Sprint(edges[i].From) < fmt.Sprint(edges[j].From)
	})
	return edges
}

func (g *Graph[V, K]) Successors(id V, filters ...EdgeFilter[V, K]) []V {
	var filter EdgeFilter[V, K]
	if len(filters) > 0 {
		filter = filters[0]
	}
	seen := map[V]bool{}
	ids := []V{}
	for to, byKind := range g.edges[id] {
		for _, edge := range byKind {
			if filter != nil && !filter(edge) {
				continue
			}
			if !seen[to] {
				seen[to] = true
				ids = append(ids, to)
			}
		}
	}
	sortByString(ids)
	return ids
}

func (g *Graph[V, K]) Predecessors(id V, filters ...EdgeFilter[V, K]) []V {
	var filter EdgeFilter[V, K]
	if len(filters) > 0 {
		filter = filters[0]
	}
	seen := map[V]bool{}
	ids := []V{}
	for from, byTo := range g.edges {
		for to, byKind := range byTo {
			if to != id {
				continue
			}
			for _, edge := range byKind {
				if filter != nil && !filter(edge) {
					continue
				}
				if !seen[from] {
					seen[from] = true
					ids = append(ids, from)
				}
			}
		}
	}
	sortByString(ids)
	return ids
}

func (g *Graph[V, K]) TopologicalOrder(
	less func(a, b V) bool,
	filters ...EdgeFilter[V, K],
) ([]V, error) {
	var filter EdgeFilter[V, K]
	if len(filters) > 0 {
		filter = filters[0]
	}
	if err := g.ValidateAcyclic(filter); err != nil {
		return nil, err
	}
	remaining, dependents := dependencyState(g, filter)
	ready := zeroRemaining(remaining)
	sortReady(ready, less)
	order := make([]V, 0, len(g.vertices))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, dependent := range dependents[id] {
			remaining[dependent]--
			if remaining[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sortReady(ready, less)
	}
	return order, nil
}

func (g *Graph[V, K]) ValidateAcyclic(filters ...EdgeFilter[V, K]) error {
	var filter EdgeFilter[V, K]
	if len(filters) > 0 {
		filter = filters[0]
	}
	visiting := map[V]bool{}
	visited := map[V]bool{}
	stack := []V{}
	var visit func(V) error
	visit = func(id V) error {
		if visited[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("cycle includes %s", cyclePath(stack, id))
		}
		visiting[id] = true
		stack = append(stack, id)
		for _, next := range g.Successors(id, filter) {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, vertex := range g.Vertices() {
		if err := visit(vertex.ID); err != nil {
			return err
		}
	}
	return nil
}

func Walk[V comparable, K comparable](
	ctx context.Context,
	g *Graph[V, K],
	opts WalkOptions[V, K],
) error {
	if opts.Parallelism <= 0 {
		opts.Parallelism = 1
	}
	if opts.Run == nil {
		return errors.New("dag walk requires Run callback")
	}
	if err := g.ValidateAcyclic(opts.EdgeFilter); err != nil {
		return err
	}
	walkCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	remaining, dependents := dependencyState(g, opts.EdgeFilter)
	ready := zeroRemaining(remaining)
	sortReady(ready, opts.Less)
	results := make(chan walkResult[V])
	running := 0
	completed := 0
	var firstErr error
	var blockedChanged <-chan struct{}
	if opts.BlockedChanged != nil {
		blockedChanged = opts.BlockedChanged()
	}

	startReady := func() {
		for firstErr == nil && walkCtx.Err() == nil && running < opts.Parallelism && len(ready) > 0 {
			index := firstStartable(ready, opts.TryReserve)
			if index < 0 {
				return
			}
			id := ready[index]
			ready = append(ready[:index], ready[index+1:]...)
			running++
			go func() {
				results <- walkResult[V]{id: id, err: opts.Run(walkCtx, id)}
			}()
		}
	}

	for completed < len(remaining) {
		startReady()
		if running == 0 {
			if firstErr != nil {
				return firstErr
			}
			if walkCtx.Err() != nil {
				return walkCtx.Err()
			}
			if len(ready) == 0 {
				return errors.New("dag walk made no progress")
			}
			if opts.BlockedChanged == nil {
				return errors.New("dag walk blocked by external constraints")
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-blockedChanged:
				if opts.BlockedChanged != nil {
					blockedChanged = opts.BlockedChanged()
				}
				continue
			}
		}
		select {
		case <-blockedChanged:
			if opts.BlockedChanged != nil {
				blockedChanged = opts.BlockedChanged()
			}
			continue
		case <-ctx.Done():
			cancel()
			for running > 0 {
				result := <-results
				running--
				if result.err != nil && firstErr == nil {
					firstErr = result.err
				}
			}
			if firstErr != nil {
				return firstErr
			}
			return ctx.Err()
		case result := <-results:
			running--
			completed++
			if result.err != nil {
				if firstErr == nil {
					firstErr = result.err
					cancel()
				}
				continue
			}
			if firstErr != nil {
				continue
			}
			for _, dependent := range dependents[result.id] {
				remaining[dependent]--
				if remaining[dependent] == 0 {
					ready = append(ready, dependent)
				}
			}
			sortReady(ready, opts.Less)
		}
	}
	return firstErr
}

type walkResult[V comparable] struct {
	id  V
	err error
}

func dependencyState[V comparable, K comparable](
	g *Graph[V, K],
	filter EdgeFilter[V, K],
) (map[V]int, map[V][]V) {
	remaining := map[V]int{}
	dependents := map[V][]V{}
	for _, vertex := range g.Vertices() {
		remaining[vertex.ID] = 0
	}
	seenPairs := map[[2]V]bool{}
	for _, edge := range g.Edges(filter) {
		pair := [2]V{edge.From, edge.To}
		if seenPairs[pair] {
			continue
		}
		seenPairs[pair] = true
		remaining[edge.To]++
		dependents[edge.From] = append(dependents[edge.From], edge.To)
	}
	for from := range dependents {
		sortByString(dependents[from])
	}
	return remaining, dependents
}

func zeroRemaining[V comparable](remaining map[V]int) []V {
	ready := []V{}
	for id, count := range remaining {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	return ready
}

func firstStartable[V comparable](ready []V, canStart func(V) bool) int {
	for index, id := range ready {
		if canStart == nil || canStart(id) {
			return index
		}
	}
	return -1
}

func sortReady[V comparable](ids []V, less func(a, b V) bool) {
	sort.SliceStable(ids, func(i, j int) bool {
		if less != nil {
			return less(ids[i], ids[j])
		}
		return fmt.Sprint(ids[i]) < fmt.Sprint(ids[j])
	})
}

func sortByString[V comparable](ids []V) {
	sort.SliceStable(ids, func(i, j int) bool {
		return fmt.Sprint(ids[i]) < fmt.Sprint(ids[j])
	})
}

func cyclePath[V comparable](stack []V, id V) string {
	start := 0
	for index, value := range stack {
		if value == id {
			start = index
			break
		}
	}
	cycle := append([]V(nil), stack[start:]...)
	cycle = append(cycle, id)
	return fmt.Sprintf("%v", cycle)
}
