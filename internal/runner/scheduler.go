package runner

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type runResult struct {
	name string
	err  error
}

func (r Runner) runTargets(ctx context.Context, s *Session) error {
	return r.runTargetsWithLocks(ctx, s, s.plan)
}

func (r Runner) runTargetsWithLocks(ctx context.Context, s *Session, plan *Plan) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	scheduledTargets := plan.ScheduledTargets()
	scheduledOrder := plan.ScheduledOrder()
	remainingDeps, dependents, orderIndex := planDependencyGraph(
		plan,
		scheduledTargets,
		scheduledOrder,
	)
	ready := readyTargets(scheduledOrder, remainingDeps)
	results := make(chan runResult)
	running := 0
	completed := 0
	var firstErr error

	start := func(name string) {
		running++
		go func() {
			results <- runResult{name: name, err: r.runOne(ctx, s, plan, scheduledTargets[name])}
		}()
	}

	for completed < len(scheduledTargets) {
		for firstErr == nil && running < r.Parallelism {
			index := nextLockableTarget(ready, scheduledTargets, s.locks)
			if index < 0 {
				break
			}
			name := ready[index]
			ready = append(ready[:index], ready[index+1:]...)
			start(name)
		}
		if running == 0 {
			if firstErr != nil {
				return firstErr
			}
			if len(ready) > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-s.locks.changed():
					continue
				}
			}
			return fmt.Errorf("dependency scheduler stalled")
		}
		result := <-results
		running--
		completed++
		s.locks.release(scheduledTargets[result.name].Spec().Runtime.Lock)
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			cancel()
			continue
		}
		if result.err != nil {
			continue
		}
		for _, dependent := range dependents[result.name] {
			remainingDeps[dependent]--
			if remainingDeps[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sortByOrder(ready, orderIndex)
	}
	return firstErr
}

type lockManager struct {
	mu   sync.Mutex
	held map[string]bool
	wait chan struct{}
}

func newLockManager() *lockManager {
	return &lockManager{held: map[string]bool{}, wait: make(chan struct{})}
}

func (m *lockManager) tryAcquire(lock string) bool {
	if lock == "" {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.held[lock] {
		return false
	}
	m.held[lock] = true
	return true
}

func (m *lockManager) release(lock string) {
	if lock == "" {
		return
	}
	m.mu.Lock()
	delete(m.held, lock)
	close(m.wait)
	m.wait = make(chan struct{})
	m.mu.Unlock()
}

func (m *lockManager) changed() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wait
}

func nextLockableTarget(ready []string, targets map[string]*Target, locks *lockManager) int {
	for index, name := range ready {
		if locks.tryAcquire(targets[name].Spec().Runtime.Lock) {
			return index
		}
	}
	return -1
}

func planDependencyGraph(
	plan *Plan,
	targets map[string]*Target,
	order []string,
) (map[string]int, map[string][]string, map[string]int) {
	remainingDeps := map[string]int{}
	dependents := map[string][]string{}
	orderIndex := make(map[string]int, len(order))
	for index, name := range order {
		orderIndex[name] = index
	}
	for _, edge := range plan.DependencyEdges {
		if _, ok := targets[edge.To]; !ok {
			continue
		}
		if _, ok := targets[edge.From]; !ok {
			continue
		}
		remainingDeps[edge.To]++
		dependents[edge.From] = append(dependents[edge.From], edge.To)
	}
	for name := range dependents {
		sortByOrder(dependents[name], orderIndex)
	}
	return remainingDeps, dependents, orderIndex
}

func readyTargets(order []string, remainingDeps map[string]int) []string {
	ready := make([]string, 0)
	for _, name := range order {
		if remainingDeps[name] == 0 {
			ready = append(ready, name)
		}
	}
	return ready
}

func sortByOrder(names []string, orderIndex map[string]int) {
	sort.SliceStable(names, func(i, j int) bool {
		return orderIndex[names[i]] < orderIndex[names[j]]
	})
}
