package graph

import "github.com/applauselab/bachkator/internal/model"

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

type ExplainRecord struct {
	Alias           string
	CanonicalTarget string
	Deprecated      string
	Target          string
	Description     string
	When            string
	Cost            string
	Risks           []string
	DependsOn       []string
	Steps           []string
	Inputs          []string
	Outputs         []string
	Produces        []string
	RequiredTools   []string
	Preflights      []string

	GeneratedPolicy  bool
	Subject          string
	SubjectWorkspace string
	SubjectCommit    string
	RequiredTargets  []string
}

type GraphDocument struct {
	Profiles []string    `json:"profiles,omitempty"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
}

type GraphNode struct {
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Description          string   `json:"description,omitempty"`
	Cost                 string   `json:"cost,omitempty"`
	Lock                 string   `json:"lock,omitempty"`
	Remote               bool     `json:"remote,omitempty"`
	Destructive          bool     `json:"destructive,omitempty"`
	RequiresConfirmation bool     `json:"requires_confirmation,omitempty"`
	Risks                []string `json:"risks,omitempty"`
}

type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Type  string `json:"type"`
	Order int    `json:"order,omitempty"`
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
