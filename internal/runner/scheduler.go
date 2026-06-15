package runner

import (
	"context"
	"sync"

	"github.com/applauselab/bachkator/internal/dag"
	"github.com/applauselab/bachkator/internal/model"
)

func (r *Runner) runTargets(ctx context.Context, s *Session) error {
	return r.runExecutionGraph(ctx, s)
}

func (r *Runner) runExecutionGraph(ctx context.Context, s *Session) error {
	status := model.RunStatusFailed
	defer func() { s.closeActiveScopes(status) }()
	err := dag.Walk(ctx, s.execGraph.dag, dag.WalkOptions[string, execEdgeKind]{
		Parallelism:    r.Parallelism,
		Less:           s.execGraph.less,
		TryReserve:     s.tryReserveExecutionNode,
		BlockedChanged: s.locks.changed,
		Run: func(ctx context.Context, name string) error {
			return r.runExecutionNode(ctx, s, name)
		},
	})
	if err == nil {
		status = model.RunStatusSuccess
	}
	return err
}

func (r *Runner) runExecutionNode(ctx context.Context, s *Session, name string) error {
	if target := s.execGraph.scopeStartTarget(name); target != nil {
		runtimeEnv := commandEnv(s.gitContext, s.dotenv, projectRuntimeEnv(s.project), target.Env)
		description, err := targetOperation(ctx, s.targets, target, runtimeEnv)
		if err != nil {
			s.releaseExecutionNode(name)
			return err
		}
		s.startScope(ctx, target, description.Operation)
		s.forgetExecutionNodeReservation(name)
		return nil
	}
	if s.execGraph.isBarrier(name) {
		return nil
	}
	if target := s.execGraph.scopeEndTarget(name); target != nil {
		defer s.finishScope(target)
		err := r.runOne(s.scopedContext(ctx, name), s, s.plan, target)
		return s.scopeRuntimeError(name, err)
	}
	target := s.execGraph.target(name)
	if target == nil {
		return nil
	}
	defer s.releaseExecutionNode(name)
	err := r.runOne(s.scopedContext(ctx, name), s, s.plan, target)
	return s.scopeRuntimeError(name, err)
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
