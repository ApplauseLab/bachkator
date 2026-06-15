package config

import "testing"

func TestLoaderRegistryRejectsDuplicateFamily(t *testing.T) {
	registry := NewLoaderRegistry()
	loader := func(path string, options LoadOptions) (*Project, error) { return nil, nil }
	if err := registry.Register("bachfile", loader); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("bachfile", loader); err == nil {
		t.Fatal("duplicate config loader registered without error")
	}
}

func TestLoaderRegistryReportsMissingLoader(t *testing.T) {
	registry := NewLoaderRegistry()
	if _, err := registry.Loader("missing"); err == nil {
		t.Fatal("missing config loader returned no error")
	}
}

func TestLoaderRegistryZeroValue(t *testing.T) {
	var registry LoaderRegistry
	if _, err := registry.Loader("missing"); err == nil {
		t.Fatal("zero-value missing config loader returned no error")
	}
	loader := func(path string, options LoadOptions) (*Project, error) { return nil, nil }
	if err := registry.Register("bachfile", loader); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Loader("bachfile"); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinLoaderRegistryWiresBachfileLoader(t *testing.T) {
	registry := BuiltinLoaderRegistry()
	if _, err := registry.Loader("bachfile"); err != nil {
		t.Fatalf("bachfile loader: %v", err)
	}
}
