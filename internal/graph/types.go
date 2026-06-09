package graph

import "github.com/applause/bachkator/internal/model"

type Project = model.RunProject
type Target = model.RunTarget

type AffectedTarget struct {
	Name    string
	Matches []string
}

type Risk struct {
	Remote               bool
	Destructive          bool
	RequiresConfirmation bool
}

func (r Risk) Labels() []string {
	risks := []string{}
	if r.Remote {
		risks = append(risks, "remote")
	}
	if r.Destructive {
		risks = append(risks, "destructive")
	}
	if r.RequiresConfirmation {
		risks = append(risks, "requires_confirmation")
	}
	return risks
}
