package model

import "time"

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
	Plugins          map[string]*Plugin
	Targets          map[string]*Target
	Aliases          map[string]*Alias
}

type RunProject struct {
	DefaultTarget    string
	Root             string
	StatePath        string
	Env              []string
	SelectedProfiles []string
	ProfileEnv       []string
	Inputs           map[string]*Input
	Resources        map[string]*Resource
	Producers        map[string]string
	Plugins          map[string]*Plugin
	Targets          map[string]*RunTarget
	Aliases          map[string]*Alias
}

type RunTarget struct {
	Name      string
	DependsOn []string
	Env       []string
	Outputs   []string
	OutputMap map[string]string
	SpecValue TargetSpec
}

func (p *RunProject) ResolveTargetName(name string) (string, *Alias) {
	if p == nil {
		return name, nil
	}
	if alias := p.Aliases[name]; alias != nil {
		return alias.Target, alias
	}
	return name, nil
}

func (t *RunTarget) Spec() TargetSpec {
	if t == nil {
		return TargetSpec{}
	}
	return t.SpecValue
}

type Target struct {
	Name string
	Body TargetBody
}

type Variable struct {
	Name    string
	Default string
}

type Resource struct {
	Name string
}

type Alias struct {
	Name       string
	Target     string
	Deprecated string
}

type Plugin struct {
	Name    string
	Type    string
	Command []string
	Shell   string
	WorkDir string
	Inputs  []string
	Env     []string
	Sources map[string][]string
	Timeout time.Duration
}

type PluginContext struct {
	Inputs  map[string][]string      `json:"inputs"`
	Targets map[string]TargetContext `json:"targets"`
}

type TargetContext struct {
	DependsOn []string `json:"depends_on"`
	Inputs    []string `json:"inputs"`
}

type Input struct {
	Kind string
	Name string
	Src  string
	Srcs []string
}

type CompletionCheck struct {
	OutputContains string
	FileExists     string
	Command        []string
}

type ToolRequirement struct {
	Name    string
	Command []string
	Version string
	Fix     string
}

type PreflightCheck struct {
	Name    string
	Kind    string
	Command []string
	Fix     string
}

func (p PreflightCheck) Label() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Kind
}

type QualityReportDeclaration struct {
	Kind   string
	Format string
	Parser string
	Path   string
}

type QualityConfig struct {
	Target       string
	Reports      []QualityReportDeclaration
	JUnit        []*QualityReportBlock
	Coverage     []*QualityReportBlock
	Lint         []*QualityReportBlock
	Complexity   []*QualityReportBlock
	QualityGates []*QualityGate
}

type QualityReportBlock struct {
	Path   string
	Format string
}

type QualityGate struct {
	Metric string
	Min    *float64
	Max    *float64
}

type EnvBlock struct {
}

type Profile struct {
	Name      string
	EnvBlocks []*EnvBlock
}

func (i *Input) Key() string {
	return i.Kind + "/" + i.Name
}

func (i *Input) Paths() []string {
	if i.Src != "" {
		return []string{i.Src}
	}
	return append([]string(nil), i.Srcs...)
}

func (r *Resource) Key() string {
	return "resource/" + r.Name
}

type TargetType string

const (
	TargetTypeShell    TargetType = "shell"
	TargetTypeImage    TargetType = "image"
	TargetTypePipeline TargetType = "pipeline"
	TargetTypeGroup    TargetType = "group"
)

type TargetSpec struct {
	Name     string
	Metadata TargetMetadata
	Runtime  TargetRuntime
	Quality  TargetQuality
	Cache    TargetCache
	Contract TargetContract
	Body     TargetBody
}

type TargetBody interface {
	TargetType() TargetType
}

type TargetMetadata struct {
	Description          string
	When                 string
	Cost                 string
	Remote               bool
	Destructive          bool
	RequiresConfirmation bool
}

type TargetRuntime struct {
	Quiet      bool
	Lock       string
	Timeout    time.Duration
	Retry      RetryPolicy
	Env        []string
	Tools      []ToolRequirement
	Preflights []PreflightCheck
}

type RetryPolicy struct {
	Attempts                  int
	Backoff                   time.Duration
	RetryOnQualityGateFailure bool
}

type TargetQuality struct {
	Reports []QualityReportDeclaration
	Gates   []QualityGateSpec
}

type QualityGateSpec struct {
	Metric string
	Min    *float64
	Max    *float64
}

type TargetCache struct {
	Inputs       []string
	Outputs      []string
	NamedOutputs map[string]string
	Produces     []string
}

type ShellSpec struct {
	Command []string
	Shell   string
	WorkDir string
}

func (ShellSpec) TargetType() TargetType { return TargetTypeShell }

type ImageSpec struct {
	Builder    string
	Image      string
	Tags       []string
	Dockerfile string
	Context    string
	Platform   string
	Push       bool
	BuildArgs  []string
}

func (ImageSpec) TargetType() TargetType { return TargetTypeImage }

type PipelineSpec struct {
	Steps []string
}

func (PipelineSpec) TargetType() TargetType { return TargetTypePipeline }

type GroupSpec struct {
	Targets []string
}

func (GroupSpec) TargetType() TargetType { return TargetTypeGroup }

type TargetContract struct {
	SuccessWhen []CompletionCheckSpec
	FailWhen    []CompletionCheckSpec
}

type CompletionCheckSpec struct {
	OutputContains string
	FileExists     string
	Command        []string
}

func (s TargetSpec) RiskLabels() []string {
	risks := []string{}
	if s.Metadata.Remote {
		risks = append(risks, "remote")
	}
	if s.Metadata.Destructive {
		risks = append(risks, "destructive")
	}
	if s.Metadata.RequiresConfirmation {
		risks = append(risks, "requires_confirmation")
	}
	return risks
}

func (s TargetSpec) Label() string {
	if s.Runtime.Lock == "" {
		return s.Name
	}
	return s.Name + " lock=" + s.Runtime.Lock
}

func (s TargetSpec) TargetType() TargetType {
	if s.Body == nil {
		return ""
	}
	return s.Body.TargetType()
}

func (s TargetSpec) Cacheable() bool {
	return len(s.Cache.Inputs) > 0 || len(s.Cache.Outputs) > 0
}
