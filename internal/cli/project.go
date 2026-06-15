package cli

import "github.com/applauselab/bachkator/internal/model"

type LoadOptions struct {
	Variables map[string]string
	Profiles  []string
}

type Project struct {
	DefaultTarget    string
	Root             string
	StatePath        string
	SelectedProfiles []string
	Prompts          map[string]*model.Prompt
	Targets          map[string]*Target
	Aliases          map[string]*model.Alias
	Policies         map[string]*Policy
}

type Target struct {
	Name       string
	DependsOn  []string
	Spec       model.TargetSpec
	RiskLabels []string
}

type Policy struct {
	Name             string
	Address          string
	Subject          string
	SubjectWorkspace string
	SubjectCommit    string
	RequiredTargets  []string
}

type AffectedTarget struct {
	Name    string
	Matches []string
}

type PathProvenance struct {
	Path      string
	Generated bool
	Source    bool
	Producers []ProvenanceTarget
	Consumers []ProvenanceTarget
	Status    string
	Reasons   []string
}

type ProvenanceTarget struct {
	Target            string
	Operation         string
	RegenerateCommand string
	Outputs           []string
	Inputs            []string
}

func (p *Project) ResolveTargetName(name string) (string, *model.Alias) {
	if p == nil {
		return name, nil
	}
	if alias := p.Aliases[name]; alias != nil {
		return alias.Target, alias
	}
	return name, nil
}
