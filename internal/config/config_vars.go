package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

func resolveVariables(
	body hcl.Body,
	overrides map[string]string,
	root string,
) (map[string]string, error) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "var", LabelNames: []string{"name"}}},
	}
	content, _, _ := body.PartialContent(schema)
	attrsByName := map[string]*hcl.Attribute{}
	variables := map[string]string{}
	for _, block := range content.Blocks {
		name := block.Labels[0]
		if _, exists := variables[name]; exists {
			return nil, fmt.Errorf("duplicate variable %q", name)
		}
		variables[name] = ""
		attrs, _ := block.Body.JustAttributes()
		if attr, ok := attrs["default"]; ok {
			attrsByName[name] = attr
		}
	}
	resolved := map[string]bool{}
	for name := range variables {
		if envValue, ok := os.LookupEnv("BACH_VAR_" + name); ok {
			variables[name] = envValue
			resolved[name] = true
		}
		if envValue, ok := os.LookupEnv("BACH_VAR_" + strings.ToUpper(name)); ok {
			variables[name] = envValue
			resolved[name] = true
		}
		if override, ok := overrides[name]; ok {
			variables[name] = override
			resolved[name] = true
		}
	}
	for name, value := range overrides {
		if _, exists := variables[name]; !exists {
			return nil, fmt.Errorf("override provided for undeclared variable %q", name)
		}
		variables[name] = value
		resolved[name] = true
	}
	for {
		progress := false
		for name, attr := range attrsByName {
			if resolved[name] {
				continue
			}
			value, ok, err := evalStringExpr(
				attr,
				evalContextForValues(resolvedVariables(variables, resolved), nil, root),
			)
			if err != nil {
				return nil, fmt.Errorf("variable %q default: %w", name, err)
			}
			if !ok {
				continue
			}
			variables[name] = value
			resolved[name] = true
			progress = true
		}
		if allAttrsResolved(attrsByName, resolved) || !progress {
			break
		}
	}
	for name := range attrsByName {
		if !resolved[name] {
			return nil, fmt.Errorf("variable %q default could not be resolved", name)
		}
	}
	return variables, nil
}

func allAttrsResolved(attrs map[string]*hcl.Attribute, resolved map[string]bool) bool {
	for name := range attrs {
		if !resolved[name] {
			return false
		}
	}
	return true
}

func resolvedVariables(variables map[string]string, resolved map[string]bool) map[string]string {
	values := map[string]string{}
	for name, value := range variables {
		if resolved[name] {
			values[name] = value
		}
	}
	return values
}
