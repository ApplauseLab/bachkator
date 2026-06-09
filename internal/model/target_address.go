package model

import (
	"fmt"
	"sort"
	"strings"
)

type TargetAddress struct {
	Type TargetType
	Name string
}

func ParseTargetAddress(raw string) (TargetAddress, error) {
	if strings.Contains(raw, "/") {
		return TargetAddress{}, fmt.Errorf(
			"obsolete target address %q: use type.name, for example shell.test",
			raw,
		)
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return TargetAddress{}, fmt.Errorf(
			"invalid target address %q: use type.name, for example shell.test",
			raw,
		)
	}

	targetType := TargetType(parts[0])
	if !knownTargetType(targetType) {
		return TargetAddress{}, fmt.Errorf(
			"invalid target address %q: unknown target type %q",
			raw,
			parts[0],
		)
	}

	return TargetAddress{Type: targetType, Name: parts[1]}, nil
}

func (a TargetAddress) String() string {
	if a.Type == "" {
		return a.Name
	}
	return string(a.Type) + "." + a.Name
}

func (a TargetAddress) Equal(other TargetAddress) bool {
	return a.Type == other.Type && a.Name == other.Name
}

func ResolveTargetAddress(raw string, targets map[string]*Target) (TargetAddress, error) {
	if strings.Contains(raw, ".") || strings.Contains(raw, "/") {
		return ParseTargetAddress(raw)
	}
	if raw == "" {
		return TargetAddress{}, fmt.Errorf(
			"invalid target address %q: use type.name, for example shell.test",
			raw,
		)
	}

	matches := []TargetAddress{}
	for _, target := range targets {
		if target == nil || targetAddressName(target) != raw {
			continue
		}
		matches = append(matches, target.Address())
	}

	sortTargetAddresses(matches)
	switch len(matches) {
	case 0:
		return TargetAddress{}, fmt.Errorf("unknown target %q", raw)
	case 1:
		return matches[0], nil
	default:
		return TargetAddress{}, fmt.Errorf(
			"ambiguous target %q: use one of %s",
			raw,
			joinTargetAddresses(matches),
		)
	}
}

func (t *Target) Address() TargetAddress {
	if t == nil {
		return TargetAddress{}
	}
	targetType := TargetTypeShell
	switch {
	case t.Body != nil:
		targetType = t.Body.TargetType()
	case strings.HasPrefix(t.Name, "image/"):
		targetType = TargetTypeImage
	case strings.HasPrefix(t.Name, "pipeline/"):
		targetType = TargetTypePipeline
	}
	return TargetAddress{Type: targetType, Name: targetAddressName(t)}
}

func targetAddressName(t *Target) string {
	name := t.Name
	for _, targetType := range []TargetType{TargetTypeShell, TargetTypeImage, TargetTypePipeline} {
		prefix := string(targetType) + "/"
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

func knownTargetType(targetType TargetType) bool {
	switch targetType {
	case TargetTypeShell, TargetTypeImage, TargetTypePipeline:
		return true
	default:
		return false
	}
}

func sortTargetAddresses(addresses []TargetAddress) {
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].String() < addresses[j].String()
	})
}

func joinTargetAddresses(addresses []TargetAddress) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		formatted = append(formatted, address.String())
	}
	return strings.Join(formatted, ", ")
}
