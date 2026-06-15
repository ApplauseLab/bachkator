package quality

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/model"
)

type AppliedPolicyArtifact struct {
	Schema           string                        `json:"schema"`
	RunID            string                        `json:"run_id"`
	Target           string                        `json:"target"`
	PolicyTarget     string                        `json:"policy_target,omitempty"`
	SubjectWorkspace string                        `json:"subject_workspace,omitempty"`
	SubjectCommit    string                        `json:"subject_commit,omitempty"`
	Verdict          string                        `json:"verdict"`
	Reports          []AppliedPolicyReportEvidence `json:"reports"`
	Gates            []GateResult                  `json:"gates"`
	CreatedAt        time.Time                     `json:"created_at"`
}

type AppliedPolicyReportEvidence struct {
	Kind                  string          `json:"kind"`
	Format                string          `json:"format"`
	Path                  string          `json:"path"`
	Status                model.RunStatus `json:"status"`
	MetricCount           int             `json:"metric_count"`
	FindingCount          int             `json:"finding_count"`
	BlockingFindingCount  int             `json:"blocking_finding_count"`
	PolicyMetricCollision bool            `json:"policy_metric_collision"`
}

func WriteAppliedPolicyArtifact(
	req IngestRequest,
	reports []Report,
	gates []GateResult,
	passed bool,
) error {
	if req.ProjectRoot == "" || req.RunID == "" || req.TargetName == "" {
		return nil
	}
	verdict := "failed"
	if passed {
		verdict = "passed"
	}
	targetName := req.TargetName
	if subjectTarget := req.Env["BACH_POLICY_SUBJECT"]; subjectTarget != "" {
		targetName = subjectTarget
	}
	policyTarget := ""
	if strings.HasPrefix(req.TargetName, "policy/") {
		policyTarget = req.TargetName
	}
	subjectWorkspace := req.Env["BACH_POLICY_SUBJECT_WORKSPACE"]
	subjectCommit := req.Env["BACH_POLICY_SUBJECT_COMMIT"]
	if subjectCommit == "" {
		subjectWorkspace, subjectCommit = appliedPolicySubject(reports)
	}
	if subjectCommit == "" {
		subjectCommit = appliedPolicyAttemptCommit(req)
	}
	artifact := AppliedPolicyArtifact{
		Schema:           "bach.applied_policy.v1",
		RunID:            req.RunID,
		Target:           targetName,
		PolicyTarget:     policyTarget,
		SubjectWorkspace: subjectWorkspace,
		SubjectCommit:    subjectCommit,
		Verdict:          verdict,
		Reports:          appliedPolicyReportEvidence(reports),
		Gates:            gates,
		CreatedAt:        req.now(),
	}
	dir := filepath.Join(req.ProjectRoot, ".bach", "artifacts", "policies", req.RunID)
	name := strings.NewReplacer("/", "-", ":", "-", " ", "-").Replace(targetName)
	return evidence.WriteJSONArtifact(filepath.Join(dir, name+".json"), artifact)
}

func appliedPolicyAttemptCommit(req IngestRequest) string {
	if req.ProjectRoot == "" || req.RunID == "" {
		return ""
	}
	path := ""
	_ = filepath.Walk(
		filepath.Join(req.ProjectRoot, ".bach", "runs", req.RunID),
		func(candidate string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && info.Name() == "attempt-history.json" {
				path = candidate
			}
			return nil
		},
	)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var attempts []struct {
		Commit       string `json:"commit"`
		PolicyPassed bool   `json:"policy_passed"`
	}
	if err := json.Unmarshal(data, &attempts); err != nil {
		return ""
	}
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].PolicyPassed && attempts[index].Commit != "" {
			return attempts[index].Commit
		}
	}
	return ""
}

func appliedPolicySubject(reports []Report) (string, string) {
	for _, report := range reports {
		if report.Kind != "policy" || report.Path == "" {
			continue
		}
		data, err := os.ReadFile(report.Path)
		if err != nil {
			continue
		}
		var payload struct {
			Subject struct {
				Workspace string `json:"workspace"`
				Commit    string `json:"commit"`
			} `json:"subject"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		if payload.Subject.Commit != "" {
			return payload.Subject.Workspace, payload.Subject.Commit
		}
	}
	return "", ""
}

func appliedPolicyReportEvidence(reports []Report) []AppliedPolicyReportEvidence {
	evidence := make([]AppliedPolicyReportEvidence, 0, len(reports))
	for _, report := range reports {
		item := AppliedPolicyReportEvidence{
			Kind:                  report.Kind,
			Format:                report.Format,
			Path:                  report.Path,
			Status:                report.Status,
			MetricCount:           len(report.Metrics),
			FindingCount:          len(report.Findings),
			PolicyMetricCollision: strings.Contains(report.Message, "policy-metric-collision"),
		}
		for _, finding := range report.Findings {
			switch strings.ToLower(finding.Severity) {
			case "blocker", "blocking", "critical", "error", "failure", "failed":
				item.BlockingFindingCount++
			}
		}
		evidence = append(evidence, item)
	}
	return evidence
}
