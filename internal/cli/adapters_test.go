package cli

import "testing"

func TestCommandAdaptersRegisterKnownCommandsBeforeFallback(t *testing.T) {
	adapters := commandAdapters()
	if len(adapters) == 0 {
		t.Fatal("commandAdapters returned no adapters")
	}

	commands := []string{
		"list",
		"runs",
		"artifacts",
		"affected",
		"explain",
		"graph",
		"quality",
		"reference",
		"run",
	}
	for _, command := range commands {
		matched := false
		for _, adapter := range adapters {
			if adapter.name == command {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("command %q did not match an adapter before fallback", command)
		}
	}

	if adapters[len(adapters)-1].name != "run" {
		t.Fatal("last adapter should be the run command")
	}
}

func TestCommandRegistryRejectsDuplicateAdapter(t *testing.T) {
	registry := newCommandRegistry()
	adapter := commandAdapter{
		use:   "example",
		short: "Example command",
		run:   func(ctx commandContext, args []string) error { return nil },
	}
	if err := registry.Register("example", adapter); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("example", adapter); err == nil {
		t.Fatal("duplicate command adapter registered without error")
	}
}

func TestCommandRegistryRejectsIncompleteAdapter(t *testing.T) {
	registry := newCommandRegistry()
	if err := registry.Register("broken", commandAdapter{}); err == nil {
		t.Fatal("incomplete command adapter registered without error")
	}
}
