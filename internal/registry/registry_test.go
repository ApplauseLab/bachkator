package registry

import (
	"fmt"
	"testing"
)

func TestRegistryRejectsDuplicateKey(t *testing.T) {
	registry := New[string, int]()
	if err := registry.Register("alpha", 1, duplicateStringKey); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("alpha", 2, duplicateStringKey); err == nil {
		t.Fatal("duplicate key registered without error")
	}
}

func TestRegistryGetsRegisteredValue(t *testing.T) {
	registry := New[string, int]()
	if err := registry.Register("alpha", 42, duplicateStringKey); err != nil {
		t.Fatal(err)
	}
	value, err := registry.Get("alpha", missingStringKey)
	if err != nil {
		t.Fatal(err)
	}
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}
}

func TestRegistryReportsMissingKey(t *testing.T) {
	registry := New[string, int]()
	if _, err := registry.Get("missing", missingStringKey); err == nil {
		t.Fatal("missing key returned no error")
	}
}

func TestRegistryZeroValue(t *testing.T) {
	var registry Registry[string, int]
	if _, err := registry.Get("missing", missingStringKey); err == nil {
		t.Fatal("zero-value missing key returned no error")
	}
	if err := registry.Register("alpha", 42, duplicateStringKey); err != nil {
		t.Fatal(err)
	}
	value, err := registry.Get("alpha", missingStringKey)
	if err != nil {
		t.Fatal(err)
	}
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}
}

func duplicateStringKey(key string) error {
	return fmt.Errorf("duplicate %q", key)
}

func missingStringKey(key string) error {
	return fmt.Errorf("missing %q", key)
}
