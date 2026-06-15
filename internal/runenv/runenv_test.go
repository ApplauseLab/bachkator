package runenv

import (
	"slices"
	"testing"
)

func TestExpand(t *testing.T) {
	env := map[string]string{
		"BACH_RUN_DIRECTORY": "/tmp/run",
		"NAME":               "bach",
	}

	got := Expand("$(RUN_DIRECTORY)/$NAME/${MISSING}", env)
	want := "/tmp/run/bach/"
	if got != want {
		t.Fatalf("Expand() = %q, want %q", got, want)
	}
}

func TestExpandSlice(t *testing.T) {
	env := map[string]string{"BACH_RUN_DIRECTORY": "/tmp/run", "NAME": "bach"}

	got := ExpandSlice([]string{"$NAME", "$(RUN_DIRECTORY)"}, env)
	want := []string{"bach", "/tmp/run"}
	if !slices.Equal(got, want) {
		t.Fatalf("ExpandSlice() = %#v, want %#v", got, want)
	}
}

func TestListSortsKeys(t *testing.T) {
	got := List(map[string]string{"B": "2", "A": "1"})
	want := []string{"A=1", "B=2"}
	if !slices.Equal(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
}
