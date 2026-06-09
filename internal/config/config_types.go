package config

import (
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/hashicorp/hcl/v2"
)

type Project struct {
	DefaultTarget    string
	Root             string
	StatePath        string
	Variables        map[string]string
	Env              []string
	SelectedProfiles []string
	ProfileEnv       []string
	Inputs           map[string]*Input
	Resources        map[string]*Resource
	Producers        map[string]string
	Targets          map[string]*Target
	Aliases          map[string]*Alias
}

type LoadOptions struct {
	Variables map[string]string
	Profiles  []string
}

type PluginContext = model.PluginContext
type TargetContext = model.TargetContext
type ToolRequirement = model.ToolRequirement
type QualityReportDeclaration = model.QualityReportDeclaration

type QualityReportBlock struct {
	Path   string `hcl:"path"`
	Format string `hcl:"format,optional"`
}

type Variable struct {
	Name    string `hcl:"name,label"`
	Default string `hcl:"default,optional"`
}

type Resource struct {
	Name string `hcl:"name,label"`
}

type Alias struct {
	Name       string `hcl:"name,label"`
	Target     string `hcl:"target"`
	Deprecated string `hcl:"deprecated,optional"`
}

type Plugin struct {
	Name    string              `hcl:"name,label"`
	Command []string            `hcl:"command,optional"`
	Shell   string              `hcl:"shell,optional"`
	WorkDir string              `hcl:"workdir,optional"`
	Inputs  []string            `hcl:"inputs,optional"`
	Env     []string            `hcl:"env,optional"`
	Sources map[string][]string `hcl:"sources,optional"`
}

type CompletionCheck struct {
	OutputContains string   `hcl:"output_contains,optional"`
	FileExists     string   `hcl:"file_exists,optional"`
	Command        []string `hcl:"command,optional"`
}

type RetryBlock struct {
	Attempts        int    `hcl:"attempts,optional"`
	Backoff         string `hcl:"backoff,optional"`
	BackoffDuration time.Duration
}

type QualityConfig struct {
	Target       string `hcl:"target,label"`
	Reports      []QualityReportDeclaration
	JUnit        []*QualityReportBlock `hcl:"junit,block"`
	Coverage     []*QualityReportBlock `hcl:"cov,block"`
	Lint         []*QualityReportBlock `hcl:"lint,block"`
	Complexity   []*QualityReportBlock `hcl:"complexity,block"`
	QualityGates []*QualityGate        `hcl:"quality_gate,block"`
	Remain       hcl.Body              `hcl:",remain"`
}

type QualityGate struct {
	Metric string   `hcl:"metric"`
	Min    *float64 `hcl:"min,optional"`
	Max    *float64 `hcl:"max,optional"`
}

type EnvBlock struct {
	Remain hcl.Body `hcl:",remain"`
}

type Profile struct {
	Name      string      `hcl:"name,label"`
	EnvBlocks []*EnvBlock `hcl:"env,block"`
}

type Input struct {
	Kind string   `hcl:"kind,label"`
	Name string   `hcl:"name,label"`
	Src  string   `hcl:"src,optional"`
	Srcs []string `hcl:"srcs,optional"`
}

type Target struct {
	Name                 string `hcl:"name,label"`
	Description          string `hcl:"description,optional"`
	When                 string `hcl:"when,optional"`
	Cost                 string `hcl:"cost,optional"`
	Remote               bool   `hcl:"remote,optional"`
	Destructive          bool   `hcl:"destructive,optional"`
	RequiresConfirmation bool   `hcl:"requires_confirmation,optional"`
	Lock                 string `hcl:"lock,optional"`
	Timeout              string `hcl:"timeout,optional"`
	TimeoutDuration      time.Duration
	Retry                []*RetryBlock `hcl:"retry,block"`
	RetryPolicy          model.RetryPolicy
	DependsOn            []string
	Command              []string `hcl:"command,optional"`
	Shell                string   `hcl:"shell,optional"`
	Tools                []ToolRequirement
	Preflights           []PreflightCheck
	Reports              []QualityReportDeclaration
	Quiet                bool     `hcl:"quiet,optional"`
	WorkDir              string   `hcl:"workdir,optional"`
	Inputs               []string `hcl:"inputs,optional"`
	Outputs              []string
	OutputMap            map[string]string
	Produces             []string `hcl:"produces,optional"`
	Steps                []string
	Env                  []string           `hcl:"env,optional"`
	EnvBlocks            []*EnvBlock        `hcl:"env,block"`
	SuccessWhen          []*CompletionCheck `hcl:"success_when,block"`
	FailWhen             []*CompletionCheck `hcl:"fail_when,block"`
	QualityGates         []*QualityGate
	Builder              string            `hcl:"builder,optional"`
	Image                string            `hcl:"image,optional"`
	Tags                 []string          `hcl:"tags,optional"`
	Dockerfile           string            `hcl:"dockerfile,optional"`
	Context              string            `hcl:"context,optional"`
	Platform             string            `hcl:"platform,optional"`
	Push                 bool              `hcl:"push,optional"`
	BuildArgs            []string          `hcl:"build_args_list,optional"`
	BuildArgMap          map[string]string `hcl:"build_args,optional"`
	Remain               hcl.Body          `hcl:",remain"`
}

type PreflightCheck struct {
	Name    string
	Kind    string
	Command []string
	Fix     string
}

type fileConfig struct {
	Project   *projectBlock    `hcl:"project,block"`
	Variables []*Variable      `hcl:"var,block"`
	Envs      []*EnvBlock      `hcl:"env,block"`
	Profiles  []*Profile       `hcl:"profile,block"`
	Inputs    []*Input         `hcl:"input,block"`
	Resources []*Resource      `hcl:"resource,block"`
	Plugins   []*Plugin        `hcl:"plugin,block"`
	Aliases   []*Alias         `hcl:"alias,block"`
	Qualities []*QualityConfig `hcl:"quality,block"`
	Shells    []*Target        `hcl:"shell,block"`
	Images    []*Target        `hcl:"image,block"`
	Pipelines []*Target        `hcl:"pipeline,block"`
}

type projectBlock struct {
	Name          string `hcl:"name,label"`
	DefaultTarget string `hcl:"default,optional"`
	Root          string `hcl:"root,optional"`
	StatePath     string `hcl:"state,optional"`
}

func hasInputOrResource(p *Project, key string) bool {
	if _, exists := p.Inputs[key]; exists {
		return true
	}
	_, exists := p.Resources[key]
	return exists
}

func (i *Input) Key() string {
	return i.Kind + "/" + i.Name
}

func (r *Resource) Key() string {
	return "resource/" + r.Name
}
