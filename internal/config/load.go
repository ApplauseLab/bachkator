package config

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func Load(path string) (*Project, error) {
	return LoadWithOptions(path, LoadOptions{})
}

func LoadWithOptions(path string, options LoadOptions) (*Project, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse %s: %s", path, diags.Error())
	}

	root, err := resolveProjectRoot(file.Body, filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	variables, err := resolveVariables(file.Body, options.Variables, root)
	if err != nil {
		return nil, err
	}
	projectEnv, err := resolveProjectEnv(file.Body, variables, root)
	if err != nil {
		return nil, err
	}
	profileEnv, err := resolveSelectedProfiles(
		file.Body,
		options.Profiles,
		variables,
		projectEnv,
		root,
	)
	if err != nil {
		return nil, err
	}
	evalContext := buildEvalContext(
		file.Body,
		variables,
		mergeEnvMaps(projectEnvMap(projectEnv), projectEnvMap(profileEnv)),
		root,
	)
	var cfg fileConfig
	diags = gohcl.DecodeBody(file.Body, evalContext, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decode %s: %s", path, diags.Error())
	}
	if cfg.Project == nil {
		return nil, fmt.Errorf("%s: missing project block", path)
	}

	statePath := cfg.Project.StatePath
	if statePath == "" {
		statePath = ".bach/state.db"
	}

	defaultTarget := ""
	if cfg.Project.DefaultTarget != "" {
		var err error
		defaultTarget, err = canonicalTargetOrAliasRef(cfg.Project.DefaultTarget)
		if err != nil {
			return nil, fmt.Errorf("project default: %w", err)
		}
	}

	project := &Project{
		DefaultTarget:    defaultTarget,
		Root:             root,
		StatePath:        absPath(root, statePath),
		Variables:        variables,
		Env:              projectEnv,
		SelectedProfiles: append([]string(nil), options.Profiles...),
		ProfileEnv:       profileEnv,
		Inputs:           make(map[string]*Input, len(cfg.Inputs)),
		Resources:        make(map[string]*Resource, len(cfg.Resources)),
		Producers:        map[string]string{},
		Targets: make(
			map[string]*Target,
			len(cfg.Shells)+len(cfg.Images)+len(cfg.Pipelines),
		),
		Aliases: make(map[string]*Alias, len(cfg.Aliases)),
	}
	if err := registerInputs(project, cfg.Inputs); err != nil {
		return nil, err
	}
	if err := registerResources(project, cfg.Resources); err != nil {
		return nil, err
	}
	if err := registerTargets(
		project,
		cfg.Shells,
		cfg.Images,
		cfg.Pipelines,
		variables,
	); err != nil {
		return nil, err
	}
	if err := registerQualityConfigs(project, cfg.Qualities); err != nil {
		return nil, err
	}
	if err := registerAliases(project, cfg.Aliases); err != nil {
		return nil, err
	}
	if err := runPlugins(context.Background(), project, cfg.Plugins); err != nil {
		return nil, err
	}
	if err := wireProducedInputs(project); err != nil {
		return nil, err
	}
	return project, nil
}

func resolveProjectRoot(body hcl.Body, fallback string) (string, error) {
	root := fallback
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "project", LabelNames: []string{"name"}}},
	}
	content, _, _ := body.PartialContent(schema)
	if len(content.Blocks) > 0 {
		attrs, _ := content.Blocks[0].Body.JustAttributes()
		if attr, ok := attrs["root"]; ok {
			value, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return "", fmt.Errorf("project root: %s", diags.Error())
			}
			if value.Type().FriendlyName() != "string" {
				return "", fmt.Errorf("project root: must be a string")
			}
			root = value.AsString()
		}
	}
	abs, err := filepath.Abs(expandHome(root))
	if err != nil {
		return "", err
	}
	return abs, nil
}
