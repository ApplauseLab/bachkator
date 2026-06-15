package model

import (
	"strings"
	"testing"
)

func TestParseTargetAddress(t *testing.T) {
	address, err := ParseTargetAddress("shell.test")
	if err != nil {
		t.Fatalf("ParseTargetAddress() error = %v", err)
	}

	want := TargetAddress{Type: TargetTypeShell, Name: "test"}
	if !address.Equal(want) {
		t.Fatalf("ParseTargetAddress() = %#v, want %#v", address, want)
	}
}

func TestTargetAddressString(t *testing.T) {
	address := TargetAddress{Type: TargetTypePipeline, Name: "release"}

	if got := address.String(); got != "pipeline.release" {
		t.Fatalf("TargetAddress.String() = %q, want %q", got, "pipeline.release")
	}
}

func TestTargetAddressLegacyName(t *testing.T) {
	address := TargetAddress{Type: TargetTypeAgent, Name: "implement"}

	if got := address.LegacyName(); got != "agent/implement" {
		t.Fatalf("TargetAddress.LegacyName() = %q, want %q", got, "agent/implement")
	}
}

func TestParsePolicyTargetAddress(t *testing.T) {
	address, err := ParseTargetAddress("policy.accept@agent.subject")
	if err != nil {
		t.Fatalf("ParseTargetAddress() error = %v", err)
	}

	want := TargetAddress{Type: TargetTypePolicy, Name: "accept@agent.subject"}
	if !address.Equal(want) {
		t.Fatalf("ParseTargetAddress() = %#v, want %#v", address, want)
	}
}

func TestGeneratedPolicyTargetAddress(t *testing.T) {
	address := GeneratedPolicyTargetAddress("accept", "agent/subject")

	if got := address.String(); got != "policy.accept@agent.subject" {
		t.Fatalf("address.String() = %q, want %q", got, "policy.accept@agent.subject")
	}
	if got := address.LegacyName(); got != "policy/accept@agent.subject" {
		t.Fatalf("address.LegacyName() = %q, want %q", got, "policy/accept@agent.subject")
	}
}

func TestTargetAddressEqual(t *testing.T) {
	left := TargetAddress{Type: TargetTypeImage, Name: "app"}
	right := TargetAddress{Type: TargetTypeImage, Name: "app"}
	other := TargetAddress{Type: TargetTypeShell, Name: "app"}

	if !left.Equal(right) {
		t.Fatal("Equal() returned false for identical target addresses")
	}
	if left.Equal(other) {
		t.Fatal("Equal() returned true for different target addresses")
	}
}

func TestParseTargetAddressRejectsSlashAddress(t *testing.T) {
	_, err := ParseTargetAddress("shell/test")
	if err == nil {
		t.Fatal("ParseTargetAddress() error = nil, want slash rejection")
	}
	if !strings.Contains(err.Error(), "obsolete target address") {
		t.Fatalf("ParseTargetAddress() error = %q, want obsolete guidance", err)
	}
}

func TestParseTargetAddressRejectsMalformedAddress(t *testing.T) {
	for _, raw := range []string{"", "test", "shell.", ".test", "shell.test.extra"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseTargetAddress(raw)
			if err == nil {
				t.Fatal("ParseTargetAddress() error = nil, want malformed rejection")
			}
			if !strings.Contains(err.Error(), "use type.name") {
				t.Fatalf("ParseTargetAddress() error = %q, want guidance", err)
			}
		})
	}
}

func TestParseTargetAddressRejectsUnknownType(t *testing.T) {
	_, err := ParseTargetAddress("task.test")
	if err == nil {
		t.Fatal("ParseTargetAddress() error = nil, want unknown type rejection")
	}
	if !strings.Contains(err.Error(), "unknown target type") {
		t.Fatalf("ParseTargetAddress() error = %q, want unknown type", err)
	}
}

func TestResolveTargetAddressAcceptsCanonicalAddress(t *testing.T) {
	address, err := ResolveTargetAddress("image.app", nil)
	if err != nil {
		t.Fatalf("ResolveTargetAddress() error = %v", err)
	}

	want := TargetAddress{Type: TargetTypeImage, Name: "app"}
	if !address.Equal(want) {
		t.Fatalf("ResolveTargetAddress() = %#v, want %#v", address, want)
	}
}

func TestResolveTargetAddressResolvesUniqueUnqualifiedName(t *testing.T) {
	targets := map[string]*Target{
		"test": {Name: "test", Body: ShellSpec{Command: []string{"go", "test", "./..."}}},
	}

	address, err := ResolveTargetAddress("test", targets)
	if err != nil {
		t.Fatalf("ResolveTargetAddress() error = %v", err)
	}

	want := TargetAddress{Type: TargetTypeShell, Name: "test"}
	if !address.Equal(want) {
		t.Fatalf("ResolveTargetAddress() = %#v, want %#v", address, want)
	}
}

func TestResolveTargetAddressReportsAmbiguousUnqualifiedName(t *testing.T) {
	targets := map[string]*Target{
		"image/app":    {Name: "image/app", Body: ImageSpec{Image: "example/app"}},
		"pipeline/app": {Name: "app", Body: PipelineSpec{Steps: []string{"shell/build"}}},
	}

	_, err := ResolveTargetAddress("app", targets)
	if err == nil {
		t.Fatal("ResolveTargetAddress() error = nil, want ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous target") {
		t.Fatalf("ResolveTargetAddress() error = %q, want ambiguity", err)
	}
	if !strings.Contains(err.Error(), "image.app") {
		t.Fatalf("ResolveTargetAddress() error = %q, want image option", err)
	}
	if !strings.Contains(err.Error(), "pipeline.app") {
		t.Fatalf("ResolveTargetAddress() error = %q, want pipeline option", err)
	}
}

func TestTargetAddressStripsLegacyTypePrefix(t *testing.T) {
	target := &Target{Name: "image/app", Body: ImageSpec{Image: "example/app"}}

	want := TargetAddress{Type: TargetTypeImage, Name: "app"}
	if got := target.Address(); !got.Equal(want) {
		t.Fatalf("Target.Address() = %#v, want %#v", got, want)
	}
}
