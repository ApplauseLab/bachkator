package config

import (
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/hashicorp/hcl/v2"
)

type Project struct {
	DefaultTarget    string
	Root             string
	StatePath        string
	Backend          Backend
	Variables        map[string]string
	ProfileCount     int
	Env              []string
	SelectedProfiles []string
	ProfileEnv       []string
	Inputs           map[string]*Input
	Resources        map[string]*Resource
	Producers        map[string]string
	Plugins          map[string]*Plugin
	Providers        map[string]*Provider
	Prompts          map[string]*Prompt
	Policies         map[string]*Policy
	AgentTemplates   map[string]*AgentTemplate
	Factories        map[string]*Factory
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
	Path   string         `hcl:"path"`
	Format string         `hcl:"format,optional"`
	Parser hcl.Expression `hcl:"parser,optional"`
}

type RegoPolicyBlock struct {
	Path     string `hcl:"path,optional"`
	Package  string `hcl:"package,optional"`
	Allow    string `hcl:"allow,optional"`
	Findings string `hcl:"findings,optional"`
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

type Policy struct {
	Name             string `hcl:"name,label"`
	Subject          string `hcl:"subject,optional"`
	SubjectWorkspace string `hcl:"subject_workspace,optional"`
	SubjectCommit    string `hcl:"subject_commit,optional"`
	RequiredTargets  []string
	Reviewers        []string
	QualityGates     []*QualityGate
	Remain           hcl.Body `hcl:",remain"`
}

type Plugin struct {
	Name            string              `hcl:"name,label"`
	Type            string              `hcl:"type,optional"`
	Command         []string            `hcl:"command,optional"`
	Shell           string              `hcl:"shell,optional"`
	WorkDir         string              `hcl:"workdir,optional"`
	Inputs          []string            `hcl:"inputs,optional"`
	Env             []string            `hcl:"env,optional"`
	Sources         map[string][]string `hcl:"sources,optional"`
	Timeout         string              `hcl:"timeout,optional"`
	TimeoutDuration time.Duration
}

type Provider struct {
	Name    string   `hcl:"name,label"`
	Type    string   `hcl:"type"`
	Command []string `hcl:"command,optional"`
}

type Backend struct {
	Type    string            `hcl:"type"`
	Command []string          `hcl:"command"`
	Config  map[string]string `hcl:"config,optional"`
}

type Prompt struct {
	Name        string `hcl:"name,label"`
	Path        string `hcl:"path"`
	Description string `hcl:"description,optional"`
	Version     string `hcl:"version,optional"`
}

type CompletionCheck struct {
	OutputContains string   `hcl:"output_contains,optional"`
	FileExists     string   `hcl:"file_exists,optional"`
	Command        []string `hcl:"command,optional"`
}

type RetryBlock struct {
	Attempts                  int    `hcl:"attempts,optional"`
	Backoff                   string `hcl:"backoff,optional"`
	RetryOnQualityGateFailure bool   `hcl:"retry_on_quality_gate_failure,optional"`
	BackoffDuration           time.Duration
}

type ImproveBlock struct {
	MaxAttempts int    `hcl:"max_attempts,optional"`
	Until       string `hcl:"until,optional"`
}

type QualityConfig struct {
	Target       string `hcl:"target,label"`
	Reports      []QualityReportDeclaration
	RegoPolicies []*RegoPolicyBlock    `hcl:"rego_policy,block"`
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

type AgentWorkspaceBlock struct {
	Mode string `hcl:"mode,optional"`
	Path string `hcl:"path,optional"`
}

type AgentGitBlock struct {
	Branch string `hcl:"branch,optional"`
	Commit string `hcl:"commit,optional"`
}

type AgentTemplate struct {
	Name           string `hcl:"name,label"`
	Mode           string `hcl:"mode,optional"`
	Provider       string `hcl:"provider,optional"`
	ProviderConfig *Provider
	Role           string `hcl:"role,optional"`
	Prompt         string `hcl:"prompt,optional"`
	PromptConfig   *Prompt
	Policy         string `hcl:"policy,optional"`
	AgentPolicy    model.Policy
	Workspace      []*AgentWorkspaceBlock `hcl:"workspace,block"`
	Git            []*AgentGitBlock       `hcl:"git,block"`
}

type Factory struct {
	Name      string             `hcl:"name,label"`
	Workflows []*FactoryWorkflow `hcl:"workflow,block"`
	Triggers  []*FactoryTriggers `hcl:"triggers,block"`
}

type FactoryWorkflow struct {
	Name      string                     `hcl:"name,label"`
	Plan      []*FactoryPlanPhase        `hcl:"plan,block"`
	Implement []*FactoryImplementPhase   `hcl:"implement,block"`
	Merge     []*FactoryTargetPhase      `hcl:"merge,block"`
	Deploy    []*FactoryNamedTargetPhase `hcl:"deploy,block"`
	Verify    []*FactoryNamedTargetPhase `hcl:"verify,block"`
}

type FactoryPlanPhase struct {
	AgentTemplate    string `hcl:"agent_template"`
	Path             string `hcl:"path"`
	RequiresApproval *bool  `hcl:"requires_approval,optional"`
}

type FactoryImplementPhase struct {
	AgentTemplate    string `hcl:"agent_template"`
	RequiresApproval *bool  `hcl:"requires_approval,optional"`
}

type FactoryTargetPhase struct {
	Target           string `hcl:"target"`
	RequiresApproval *bool  `hcl:"requires_approval,optional"`
}

type FactoryNamedTargetPhase struct {
	Name             string `hcl:"name,label"`
	Target           string `hcl:"target"`
	RequiresApproval *bool  `hcl:"requires_approval,optional"`
}

type FactoryTriggers struct {
	Manual   []*FactoryManualTrigger   `hcl:"manual,block"`
	Provider []*FactoryProviderTrigger `hcl:"provider,block"`
}

type FactoryProviderTrigger struct {
	Name         string            `hcl:"name,label"`
	Command      []string          `hcl:"command,optional"`
	PollInterval string            `hcl:"poll_interval,optional"`
	Config       map[string]string `hcl:"config,optional"`
	Route        []*ProviderRoute  `hcl:"route,block"`
}

type ProviderRoute struct {
	Label    string `hcl:"label"`
	Workflow string `hcl:"workflow"`
}

type FactoryManualTrigger struct{}

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
	Command              []string        `hcl:"command,optional"`
	Shell                string          `hcl:"shell,optional"`
	Improve              []*ImproveBlock `hcl:"improve,block"`
	Tools                []ToolRequirement
	Preflights           []PreflightCheck
	Reports              []QualityReportDeclaration
	Quiet                bool     `hcl:"quiet,optional"`
	WorkDir              string   `hcl:"workdir,optional"`
	Inputs               []string `hcl:"inputs,optional"`
	Outputs              []string
	OutputMap            map[string]string
	Produces             []string `hcl:"produces,optional"`
	RegoPolicies         []*RegoPolicyBlock
	Template             string `hcl:"template,optional"`
	Mode                 string `hcl:"mode,optional"`
	Provider             string `hcl:"provider,optional"`
	ProviderConfig       *Provider
	Role                 string `hcl:"role,optional"`
	Prompt               string `hcl:"prompt,optional"`
	PromptConfig         *Prompt
	Plan                 string `hcl:"plan,optional"`
	Subject              string
	AgentSubject         model.AgentSubject
	Policy               string `hcl:"policy,optional"`
	AgentPolicy          model.Policy
	Workspace            []*AgentWorkspaceBlock `hcl:"workspace,block"`
	Git                  []*AgentGitBlock       `hcl:"git,block"`
	Steps                []string
	Targets              []string
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
	Providers []*Provider      `hcl:"provider,block"`
	Prompts   []*Prompt        `hcl:"prompt,block"`
	Policies  []*Policy        `hcl:"policy,block"`
	Templates []*AgentTemplate `hcl:"agent_template,block"`
	Factories []*Factory       `hcl:"factory,block"`
	Aliases   []*Alias         `hcl:"alias,block"`
	Qualities []*QualityConfig `hcl:"quality,block"`
	Shells    []*Target        `hcl:"shell,block"`
	Agents    []*Target        `hcl:"agent,block"`
	Images    []*Target        `hcl:"image,block"`
	Pipelines []*Target        `hcl:"pipeline,block"`
	Groups    []*Target        `hcl:"group,block"`
}

type projectBlock struct {
	Name          string     `hcl:"name,label"`
	DefaultTarget string     `hcl:"default,optional"`
	Root          string     `hcl:"root,optional"`
	StatePath     string     `hcl:"state,optional"`
	Backends      []*Backend `hcl:"backend,block"`
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
