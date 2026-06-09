package cli

import "github.com/applause/bachkator/internal/model"

type LoadOptions struct {
	Variables map[string]string
	Profiles  []string
}

type Project struct {
	Backing          any
	DefaultTarget    string
	StatePath        string
	SelectedProfiles []string
	Targets          map[string]*Target
	Aliases          map[string]*model.Alias
}

type Target struct {
	Name       string
	DependsOn  []string
	Spec       model.TargetSpec
	RiskLabels []string
}

type AffectedTarget struct {
	Name    string
	Matches []string
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
