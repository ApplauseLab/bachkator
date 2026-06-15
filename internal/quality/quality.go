package quality

import (
	"context"

	"github.com/applauselab/bachkator/internal/model"
)

type ParseRequest struct {
	Context     context.Context
	Path        string
	DisplayPath string
	Declaration model.QualityReportDeclaration
	Parsers     ReportParsers
	ProjectRoot string
	Workdir     string
	Env         map[string]string
	Plugins     map[string]*model.Plugin
	RunID       string
	TargetName  string
}

func ParseReport(req ParseRequest) (Report, error) {
	if req.Declaration.Parser != "" {
		return parsePluginReport(req)
	}
	parsers := req.Parsers
	if parsers == nil {
		parsers = BuiltinReportParserRegistry()
	}
	parser, err := parsers.Parser(req.Declaration.Format)
	if err != nil {
		return Report{}, err
	}
	return parser.Parse(req.Path)
}
