package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func buildArgList(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, key+"="+values[key])
	}
	return args
}

func appendUnique(values []string, additions ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, addition := range additions {
		if addition == "" || seen[addition] {
			continue
		}
		seen[addition] = true
		values = append(values, addition)
	}
	return values
}

func canonicalTargetRef(ref string) (string, error) {
	if strings.Contains(ref, "/") {
		return "", fmt.Errorf(
			"obsolete target reference %q: use type.name, for example shell.test",
			ref,
		)
	}
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf(
			"invalid target reference %q: use type.name, for example shell.test",
			ref,
		)
	}
	switch parts[0] {
	case "shell", "image", "pipeline":
		return parts[0] + "/" + parts[1], nil
	default:
		return "", fmt.Errorf("invalid target reference %q: unknown target type %q", ref, parts[0])
	}
}

func canonicalTargetOrAliasRef(ref string) (string, error) {
	if strings.Contains(ref, "/") {
		return "", fmt.Errorf(
			"obsolete target reference %q: use type.name, for example shell.test",
			ref,
		)
	}
	if strings.Contains(ref, ".") {
		return canonicalTargetRef(ref)
	}
	return ref, nil
}

func canonicalTargetRefs(refs []string) ([]string, error) {
	translated := make([]string, 0, len(refs))
	for _, ref := range refs {
		canonical, err := canonicalTargetRef(ref)
		if err != nil {
			return nil, err
		}
		translated = append(translated, canonical)
	}
	return translated, nil
}

func decodeTargetRefList(attr *hcl.Attribute, ctx *hcl.EvalContext) ([]string, error) {
	value, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s", diags.Error())
	}
	if !value.CanIterateElements() {
		return nil, fmt.Errorf("must be a list")
	}
	refs := []string{}
	for _, item := range value.AsValueSlice() {
		if item.Type() != cty.String {
			return nil, fmt.Errorf("entries must be target references")
		}
		refs = append(refs, item.AsString())
	}
	return canonicalTargetRefs(refs)
}
