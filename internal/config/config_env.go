package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

func resolveProjectEnv(body hcl.Body, variables map[string]string, root string) ([]string, error) {
	schema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "env"}}}
	content, _, _ := body.PartialContent(schema)
	blocks := make([]*EnvBlock, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		blocks = append(blocks, &EnvBlock{Remain: block.Body})
	}
	return resolveEnvBlocks(blocks, variables, nil, "env", root)
}

func resolveTargetEnv(
	target *Target,
	variables map[string]string,
	projectEnv []string,
	root string,
) error {
	resolved, err := resolveEnvBlocks(
		target.EnvBlocks,
		variables,
		projectEnvMap(projectEnv),
		target.Name+" env",
		root,
	)
	if err != nil {
		return err
	}
	values := projectEnvMap(resolved)
	for _, entry := range target.Env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	target.Env = envListFromMap(values)
	return nil
}

func resolveEnvBlocks(
	blocks []*EnvBlock,
	variables map[string]string,
	baseEnv map[string]string,
	context string,
	root string,
) ([]string, error) {
	attrsByName := map[string]*hcl.Attribute{}
	for _, block := range blocks {
		attrs, _ := block.Remain.JustAttributes()
		for name, attr := range attrs {
			if _, exists := attrsByName[name]; exists {
				return nil, fmt.Errorf("duplicate %s %q", context, name)
			}
			attrsByName[name] = attr
		}
	}
	values := map[string]string{}
	for name, value := range baseEnv {
		if _, overridden := attrsByName[name]; overridden {
			continue
		}
		values[name] = value
	}
	resolvedValues := map[string]string{}
	for {
		progress := false
		for name, attr := range attrsByName {
			if _, exists := resolvedValues[name]; exists {
				continue
			}
			value, ok, err := evalStringExpr(attr, evalContextForValues(variables, values, root))
			if err != nil {
				return nil, fmt.Errorf("%s %q: %w", context, name, err)
			}
			if !ok {
				continue
			}
			values[name] = value
			resolvedValues[name] = value
			progress = true
		}
		if len(resolvedValues) == len(attrsByName) || !progress {
			break
		}
	}
	for name := range attrsByName {
		if _, exists := resolvedValues[name]; !exists {
			return nil, fmt.Errorf("%s %q could not be resolved", context, name)
		}
	}
	return envListFromMap(resolvedValues), nil
}

func envListFromMap(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func projectEnvMap(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}
