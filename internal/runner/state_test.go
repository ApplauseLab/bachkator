package runner

import (
	"testing"

	statestore "github.com/applauselab/bachkator/internal/state"
)

func newTestStore(t testing.TB, path string) *statestore.Store {
	store, err := statestore.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
