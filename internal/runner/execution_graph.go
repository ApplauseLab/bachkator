package runner

import (
	"fmt"

	"github.com/applause/bachkator/internal/dag"
	"github.com/applause/bachkator/internal/model"
)

const (
	execNodeTarget     execNodeKind = "target"
	execNodeBarrier    execNodeKind = "barrier"
	execNodeScopeStart execNodeKind = "scope_start"

	execEdgeDependency  execEdgeKind = "dependency"
	execEdgeScopeStart  execEdgeKind = "scope_start"
	execEdgeSequence    execEdgeKind = "sequence"
	execEdgeGroupMember execEdgeKind = "group_member"
	execEdgeFinalize    execEdgeKind = "finalize"
)

type execNodeKind string
type execEdgeKind string

type execNode struct {
	ID     string
	Kind   execNodeKind
	Target *Target
	Depth  int
}

type executionScope struct {
	Name  string
	Depth int
}

type executionGraph struct {
	dag     *dag.Graph[string, execEdgeKind]
	nodes   map[string]execNode
	targets map[string]*Target
	members map[string][]executionScope
	order   map[string]int
}

func buildExecutionGraph(plan *Plan) (*executionGraph, error) {
	b := executionGraphBuilder{
		plan:     plan,
		graph:    dag.New[string, execEdgeKind](),
		nodes:    map[string]execNode{},
		members:  map[string][]executionScope{},
		starts:   map[string]string{},
		visiting: map[string]bool{},
		closures: map[string]map[string]bool{},
		order:    map[string]int{},
	}
	for index, name := range plan.Order {
		b.order[name] = index * 1000
	}
	if _, err := b.addClosure(plan.TargetName); err != nil {
		return nil, err
	}
	if err := b.graph.ValidateAcyclic(); err != nil {
		return nil, err
	}
	return &executionGraph{
		dag:     b.graph,
		nodes:   b.nodes,
		targets: b.scheduledTargets(),
		members: b.members,
		order:   b.order,
	}, nil
}

type executionGraphBuilder struct {
	plan     *Plan
	graph    *dag.Graph[string, execEdgeKind]
	nodes    map[string]execNode
	members  map[string][]executionScope
	starts   map[string]string
	visiting map[string]bool
	closures map[string]map[string]bool
	order    map[string]int
	stack    []string
}

func (b *executionGraphBuilder) addClosure(name string) (map[string]bool, error) {
	if closure := b.closures[name]; closure != nil {
		return closure, nil
	}
	if b.visiting[name] {
		return nil, fmt.Errorf("execution graph cycle includes %q", name)
	}
	target := b.plan.Target(name)
	if target == nil {
		return nil, fmt.Errorf("unknown target %q", name)
	}
	b.visiting[name] = true
	defer func() { b.visiting[name] = false }()

	b.addTarget(name, target)
	closure := map[string]bool{name: true}
	for _, dep := range target.DependsOn {
		depClosure, err := b.addClosure(dep)
		if err != nil {
			return nil, err
		}
		b.graph.AddEdge(dep, name, execEdgeDependency)
		mergeClosure(closure, depClosure)
	}
	if pipeline, ok := target.Spec().Body.(model.PipelineSpec); ok {
		b.stack = append(b.stack, name)
		if err := b.addPipeline(
			name,
			target.DependsOn,
			pipeline.Steps,
			closure,
			len(b.stack),
		); err != nil {
			return nil, err
		}
		b.stack = b.stack[:len(b.stack)-1]
	}
	if group, ok := target.Spec().Body.(model.GroupSpec); ok {
		b.stack = append(b.stack, name)
		if err := b.addGroup(
			name,
			target.DependsOn,
			group.Targets,
			closure,
			len(b.stack),
		); err != nil {
			return nil, err
		}
		b.stack = b.stack[:len(b.stack)-1]
	}
	b.closures[name] = closure
	return closure, nil
}

func (b *executionGraphBuilder) addPipeline(
	pipelineName string,
	dependsOn []string,
	steps []string,
	closure map[string]bool,
	depth int,
) error {
	scopeStart := fmt.Sprintf("@scope/%s/start", pipelineName)
	pipelineOrder := b.order[pipelineName]
	b.addScopeStart(scopeStart, b.plan.Target(pipelineName), pipelineOrder-1, depth)
	b.starts[pipelineName] = scopeStart
	for _, dep := range dependsOn {
		b.graph.AddEdge(dep, scopeStart, execEdgeDependency)
	}
	completedBeforeScope := b.closuresFor(dependsOn)
	previousBarrier := ""
	previousClosures := copyClosure(completedBeforeScope)
	for index, step := range steps {
		stepClosure, err := b.addClosure(step)
		if err != nil {
			return err
		}
		mergeClosure(closure, stepClosure)
		stepRoot := b.startOrTarget(step)
		if !previousClosures[stepRoot] {
			b.addExecutionEdge(scopeStart, stepRoot, execEdgeScopeStart)
			b.addMemberScope(stepRoot, pipelineName, depth)
		}
		for node := range stepClosure {
			if node == pipelineName || previousClosures[node] {
				continue
			}
			b.addExecutionEdge(scopeStart, node, execEdgeScopeStart)
			b.addMemberScope(node, pipelineName, depth)
		}
		if previousBarrier != "" {
			b.addExecutionEdge(previousBarrier, stepRoot, execEdgeSequence)
			for node := range stepClosure {
				if previousClosures[node] {
					continue
				}
				b.addExecutionEdge(previousBarrier, node, execEdgeSequence)
			}
		}
		barrier := fmt.Sprintf("@barrier/%s/%d", pipelineName, index)
		b.addBarrier(barrier, pipelineOrder+index+1)
		for node := range stepClosure {
			b.addExecutionEdge(node, barrier, execEdgeFinalize)
		}
		previousBarrier = barrier
		mergeClosure(previousClosures, stepClosure)
	}
	if previousBarrier != "" {
		b.addExecutionEdge(previousBarrier, pipelineName, execEdgeFinalize)
	}
	b.addExecutionEdge(scopeStart, pipelineName, execEdgeFinalize)
	return nil
}

func (b *executionGraphBuilder) addGroup(
	groupName string,
	dependsOn []string,
	targets []string,
	closure map[string]bool,
	depth int,
) error {
	scopeStart := fmt.Sprintf("@scope/%s/start", groupName)
	groupOrder := b.order[groupName]
	b.addScopeStart(scopeStart, b.plan.Target(groupName), groupOrder-1, depth)
	b.starts[groupName] = scopeStart
	for _, dep := range dependsOn {
		b.graph.AddEdge(dep, scopeStart, execEdgeDependency)
	}
	completedBeforeScope := b.closuresFor(dependsOn)
	for _, member := range targets {
		memberClosure, err := b.addClosure(member)
		if err != nil {
			return err
		}
		mergeClosure(closure, memberClosure)
		memberRoot := b.startOrTarget(member)
		if !completedBeforeScope[memberRoot] {
			b.addExecutionEdge(scopeStart, memberRoot, execEdgeScopeStart)
			b.addMemberScope(memberRoot, groupName, depth)
		}
		for node := range memberClosure {
			if node != groupName {
				if !completedBeforeScope[node] {
					b.addExecutionEdge(scopeStart, node, execEdgeScopeStart)
					b.addMemberScope(node, groupName, depth)
				}
				b.addExecutionEdge(node, groupName, execEdgeGroupMember)
			}
		}
	}
	b.addExecutionEdge(scopeStart, groupName, execEdgeFinalize)
	return nil
}

func (b *executionGraphBuilder) addExecutionEdge(from, to string, kind execEdgeKind) {
	if from == "" || to == "" || b.hasPath(to, from, map[string]bool{}) {
		return
	}
	b.graph.AddEdge(from, to, kind)
}

func (b *executionGraphBuilder) hasPath(from, to string, seen map[string]bool) bool {
	if from == to {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, next := range b.graph.Successors(from) {
		if b.hasPath(next, to, seen) {
			return true
		}
	}
	return false
}

func (b *executionGraphBuilder) startOrTarget(name string) string {
	if start := b.starts[name]; start != "" {
		return start
	}
	return name
}

func (b *executionGraphBuilder) closuresFor(names []string) map[string]bool {
	closure := map[string]bool{}
	for _, name := range names {
		mergeClosure(closure, b.closures[name])
	}
	return closure
}

func copyClosure(src map[string]bool) map[string]bool {
	dst := map[string]bool{}
	mergeClosure(dst, src)
	return dst
}

func (b *executionGraphBuilder) addTarget(name string, target *Target) {
	b.nodes[name] = execNode{ID: name, Kind: execNodeTarget, Target: target}
	b.graph.AddVertex(dag.Vertex[string]{ID: name, Kind: string(execNodeTarget)})
}

func (b *executionGraphBuilder) addBarrier(name string, order int) {
	b.nodes[name] = execNode{ID: name, Kind: execNodeBarrier}
	b.order[name] = order
	b.graph.AddVertex(dag.Vertex[string]{ID: name, Kind: string(execNodeBarrier)})
}

func (b *executionGraphBuilder) addScopeStart(name string, target *Target, order int, depth int) {
	b.nodes[name] = execNode{ID: name, Kind: execNodeScopeStart, Target: target, Depth: depth}
	b.order[name] = order
	b.graph.AddVertex(dag.Vertex[string]{ID: name, Kind: string(execNodeScopeStart)})
}

func (b *executionGraphBuilder) addMemberScope(node string, scope string, depth int) {
	b.members[node] = appendExecutionScope(
		b.members[node],
		executionScope{Name: scope, Depth: depth},
	)
}

func appendExecutionScope(values []executionScope, value executionScope) []executionScope {
	for _, existing := range values {
		if existing.Name == value.Name {
			return values
		}
	}
	return append(values, value)
}

func mergeClosure(dst, src map[string]bool) {
	for key := range src {
		dst[key] = true
	}
}

func (b *executionGraphBuilder) scheduledTargets() map[string]*Target {
	targets := map[string]*Target{}
	for name, node := range b.nodes {
		if node.Kind == execNodeTarget {
			targets[name] = node.Target
		}
	}
	return targets
}

func (g *executionGraph) target(name string) *Target {
	return g.targets[name]
}

func (g *executionGraph) less(leftName, rightName string) bool {
	left := g.order[leftName]
	right := g.order[rightName]
	if left == right {
		return leftName < rightName
	}
	return left < right
}

func (g *executionGraph) isBarrier(name string) bool {
	return g.nodes[name].Kind == execNodeBarrier
}

func (g *executionGraph) scopeStartTarget(name string) *Target {
	node := g.nodes[name]
	if node.Kind != execNodeScopeStart {
		return nil
	}
	return node.Target
}

func (g *executionGraph) scopeEndTarget(name string) *Target {
	node := g.nodes[name]
	if node.Kind != execNodeTarget {
		return nil
	}
	if _, ok := node.Target.Spec().Body.(model.PipelineSpec); ok {
		return node.Target
	}
	if _, ok := node.Target.Spec().Body.(model.GroupSpec); !ok {
		return nil
	}
	return node.Target
}

func (g *executionGraph) memberScopes(name string) []executionScope {
	return append([]executionScope(nil), g.members[name]...)
}
