package config

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

func Load(path string) (*Project, error) {
	return LoadWithOptions(path, LoadOptions{})
}

func LoadWithOptions(path string, options LoadOptions) (*Project, error) {
	return loadWithOptions(path, options, true)
}

func loadWithOptions(path string, options LoadOptions, executePlugins bool) (*Project, error) {
	file, err := loadBachfile(path)
	if err != nil {
		return nil, err
	}
	body := file.Body
	root, err := resolveProjectRoot(body, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	variables, err := resolveVariables(body, options.Variables, root)
	if err != nil {
		return nil, err
	}
	projectEnv, err := resolveProjectEnv(body, variables, root)
	if err != nil {
		return nil, err
	}
	profileEnv, err := resolveSelectedProfiles(
		body,
		options.Profiles,
		variables,
		projectEnv,
		root,
	)
	if err != nil {
		return nil, err
	}
	evalContext := buildEvalContext(
		body,
		variables,
		mergeEnvMaps(projectEnvMap(projectEnv), projectEnvMap(profileEnv)),
		root,
	)
	var cfg fileConfig
	diags := gohcl.DecodeBody(body, evalContext, &cfg)
	if diags.HasErrors() {
		return nil, diagnosticError(
			path,
			fmt.Errorf("decode %s: %s", path, diags.Error()),
			diags,
			"hcl-decode-error",
		)
	}
	if cfg.Project == nil {
		return nil, fmt.Errorf("%s: missing project block", path)
	}

	backend, err := resolveBackend(cfg.Project)
	if err != nil {
		return nil, err
	}
	statePath := backend.Config["path"]

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
		Backend:          backend,
		Variables:        variables,
		ProfileCount:     len(cfg.Profiles),
		Env:              projectEnv,
		SelectedProfiles: append([]string(nil), options.Profiles...),
		ProfileEnv:       profileEnv,
		Inputs:           make(map[string]*Input, len(cfg.Inputs)),
		Resources:        make(map[string]*Resource, len(cfg.Resources)),
		Producers:        map[string]string{},
		Plugins:          make(map[string]*Plugin, len(cfg.Plugins)),
		Providers:        make(map[string]*Provider, len(cfg.Providers)),
		Prompts:          make(map[string]*Prompt, len(cfg.Prompts)),
		Policies:         make(map[string]*Policy, len(cfg.Policies)),
		AgentTemplates:   make(map[string]*AgentTemplate, len(cfg.Templates)),
		Factories:        make(map[string]*Factory, len(cfg.Factories)),
		Targets: make(
			map[string]*Target,
			len(cfg.Shells)+len(cfg.Images)+len(cfg.Pipelines)+len(cfg.Groups)+len(cfg.Agents),
		),
		Aliases: make(map[string]*Alias, len(cfg.Aliases)),
	}
	if err := registerInputs(project, cfg.Inputs); err != nil {
		return nil, err
	}
	if err := registerResources(project, cfg.Resources); err != nil {
		return nil, err
	}
	if err := registerProviders(project, cfg.Providers); err != nil {
		return nil, err
	}
	if err := registerPrompts(project, cfg.Prompts); err != nil {
		return nil, err
	}
	if err := registerPolicies(
		project,
		cfg.Policies,
		cfg.Shells,
		cfg.Agents,
		cfg.Images,
		cfg.Pipelines,
		cfg.Groups,
	); err != nil {
		return nil, err
	}
	if err := registerAgentTemplates(project, cfg.Templates); err != nil {
		return nil, err
	}
	if err := registerFactories(project, cfg.Factories); err != nil {
		return nil, err
	}
	if err := registerTargets(
		project,
		cfg.Shells,
		cfg.Agents,
		cfg.Images,
		cfg.Pipelines,
		cfg.Groups,
		variables,
	); err != nil {
		return nil, err
	}
	if err := validateFactoryPhaseReferences(project); err != nil {
		return nil, err
	}
	if err := registerPlugins(project, cfg.Plugins); err != nil {
		return nil, err
	}
	if err := registerQualityConfigs(project, cfg.Qualities); err != nil {
		return nil, err
	}
	if err := registerAliases(project, cfg.Aliases); err != nil {
		return nil, err
	}
	if executePlugins {
		if err := runPlugins(context.Background(), project, cfg.Plugins); err != nil {
			return nil, err
		}
	}
	if err := wireProducedInputs(project); err != nil {
		return nil, err
	}
	return project, nil
}

func resolveBackend(project *projectBlock) (Backend, error) {
	if project.StatePath != "" {
		return Backend{}, fmt.Errorf(
			"project state is no longer supported; omit it to use the default backend or configure project backend",
		)
	}
	if len(project.Backends) > 1 {
		return Backend{}, fmt.Errorf("project backend: only one backend block is allowed")
	}
	if len(project.Backends) == 0 {
		return defaultBackend(), nil
	}
	backend := *project.Backends[0]
	if backend.Type != "stdio" {
		return Backend{}, fmt.Errorf(
			"project backend: unsupported type %q (only stdio is supported)",
			backend.Type,
		)
	}
	if !slices.Equal(backend.Command, defaultBackendCommand()) {
		return Backend{}, fmt.Errorf(
			"project backend: unsupported command %q (only [\"bach\", \"backend\", \"sqlite\"] is supported)",
			backend.Command,
		)
	}
	if backend.Config == nil {
		backend.Config = map[string]string{}
	}
	if backend.Config["path"] == "" {
		backend.Config["path"] = ".bach/state.db"
	}
	if err := validateProjectRelativePath(
		"backend config.path",
		backend.Config["path"],
	); err != nil {
		return Backend{}, fmt.Errorf("project backend: %w", err)
	}
	return backend, nil
}

func defaultBackend() Backend {
	return Backend{
		Type:    "stdio",
		Command: defaultBackendCommand(),
		Config:  map[string]string{"path": ".bach/state.db"},
	}
}

func defaultBackendCommand() []string {
	return []string{"bach", "backend", "sqlite"}
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
