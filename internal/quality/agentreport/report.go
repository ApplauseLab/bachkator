package agentreport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const Schema = "bach.agent_report.v1"

type Defaults struct {
	Path              string
	Role              string
	Name              string
	Summary           string
	Env               map[string]string
	AllowExternalPath bool
}

type Report struct {
	Schema   string            `json:"schema"`
	Agent    Actor             `json:"agent"`
	Subject  map[string]string `json:"subject,omitempty"`
	Status   string            `json:"status"`
	Summary  string            `json:"summary"`
	Metrics  []Metric          `json:"metrics"`
	Findings []Finding         `json:"findings"`
}

type Actor struct {
	Role string `json:"role"`
	Name string `json:"name,omitempty"`
}

type Metric struct {
	Name  string  `json:"name"`
	Scope string  `json:"scope,omitempty"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type Finding struct {
	Kind       string  `json:"kind"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Severity   string  `json:"severity,omitempty"`
	Rule       string  `json:"rule,omitempty"`
	Message    string  `json:"message,omitempty"`
	DurationMS float64 `json:"duration_ms,omitempty"`
}

func ResolvePath(explicit string, env map[string]string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env["BACH_AGENT_QUALITY_REPORT_PATH"] != "" {
		return env["BACH_AGENT_QUALITY_REPORT_PATH"], nil
	}
	if env["BACH_RUN_DIRECTORY"] != "" {
		return filepath.Join(env["BACH_RUN_DIRECTORY"], "agent-report-v1.json"), nil
	}
	return "", fmt.Errorf(
		"report destination is required: pass --path, " +
			"set BACH_AGENT_QUALITY_REPORT_PATH, or set BACH_RUN_DIRECTORY",
	)
}

func New(defaults Defaults) Report {
	role := firstNonEmpty(
		defaults.Role,
		defaults.Env["BACH_REPORT_AGENT_ROLE"],
		defaults.Env["BACH_AGENT_ROLE"],
		"reporter",
	)
	name := firstNonEmpty(defaults.Name, defaults.Env["BACH_REPORT_AGENT_NAME"])
	summary := firstNonEmpty(defaults.Summary, "Report initialized by bach report.")
	report := Report{
		Schema:   Schema,
		Agent:    Actor{Role: role, Name: name},
		Subject:  subjectFromEnv(defaults.Env),
		Status:   "success",
		Summary:  summary,
		Metrics:  []Metric{},
		Findings: []Finding{},
	}
	if len(report.Subject) == 0 {
		report.Subject = nil
	}
	return report
}

func WriteInit(defaults Defaults) (string, error) {
	path, err := ResolvePath(defaults.Path, defaults.Env)
	if err != nil {
		return "", err
	}
	if err := validateDestination(path, defaults); err != nil {
		return "", err
	}
	defaults.Path = path
	return path, lockedWrite(path, func(_ Report, _ bool) (Report, error) {
		return New(defaults), nil
	})
}

func AppendFinding(defaults Defaults, finding Finding) (string, error) {
	path, err := ResolvePath(defaults.Path, defaults.Env)
	if err != nil {
		return "", err
	}
	if err := validateDestination(path, defaults); err != nil {
		return "", err
	}
	defaults.Path = path
	return path, lockedWrite(path, func(report Report, exists bool) (Report, error) {
		if !exists {
			report = New(defaults)
		}
		if err := validateFinding(finding); err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, finding)
		return report, nil
	})
}

func AppendMetric(defaults Defaults, metric Metric) (string, error) {
	path, err := ResolvePath(defaults.Path, defaults.Env)
	if err != nil {
		return "", err
	}
	if err := validateDestination(path, defaults); err != nil {
		return "", err
	}
	defaults.Path = path
	return path, lockedWrite(path, func(report Report, exists bool) (Report, error) {
		if !exists {
			report = New(defaults)
		}
		if err := validateMetric(metric); err != nil {
			return Report{}, err
		}
		report.Metrics = append(report.Metrics, metric)
		return report, nil
	})
}

func UpdateStatus(defaults Defaults, status string) (string, error) {
	path, err := ResolvePath(defaults.Path, defaults.Env)
	if err != nil {
		return "", err
	}
	if err := validateDestination(path, defaults); err != nil {
		return "", err
	}
	defaults.Path = path
	return path, lockedWrite(path, func(report Report, exists bool) (Report, error) {
		if !exists {
			report = New(defaults)
		}
		if status != "success" && status != "failed" {
			return Report{}, fmt.Errorf("status must be success or failed")
		}
		report.Status = status
		if defaults.Summary != "" {
			report.Summary = defaults.Summary
		}
		return report, nil
	})
}

func DecodeFindingStrict(data []byte) (Finding, error) {
	var finding Finding
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&finding); err != nil {
		return Finding{}, err
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return Finding{}, err
	}
	return finding, validateFinding(finding)
}

func validateDestination(path string, defaults Defaults) error {
	if defaults.AllowExternalPath {
		return nil
	}
	absPath, err := resolvedDestinationPath(path)
	if err != nil {
		return err
	}
	allowedRoots := []string{}
	if cwd, err := os.Getwd(); err == nil {
		if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
			allowedRoots = append(allowedRoots, resolved)
		}
	}
	if runDir := defaults.Env["BACH_RUN_DIRECTORY"]; runDir != "" {
		if absRunDir, err := filepath.Abs(runDir); err == nil {
			if resolved, err := filepath.EvalSymlinks(absRunDir); err == nil {
				allowedRoots = append(allowedRoots, resolved)
			}
		}
	}
	for _, root := range allowedRoots {
		if pathWithin(absPath, root) {
			return nil
		}
	}
	return fmt.Errorf(
		"report path %q is outside the workspace; pass --allow-external-path to allow it",
		path,
	)
}

func resolvedDestinationPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(absPath)
	missing := []string{filepath.Base(absPath)}
	for {
		if _, err := os.Stat(dir); err == nil {
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return absPath, nil
		}
		missing = append(missing, filepath.Base(dir))
		dir = parent
	}
}

func pathWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func lockedWrite(path string, mutate func(Report, bool) (Report, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()

	report, exists, err := read(path)
	if err != nil {
		return err
	}
	report, err = mutate(report, exists)
	if err != nil {
		return err
	}
	if err := validateReport(report); err != nil {
		return err
	}
	return atomicWrite(path, report)
}

func read(path string) (Report, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Report{}, false, nil
	}
	if err != nil {
		return Report{}, false, err
	}
	var report Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return Report{}, false, err
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return Report{}, false, err
	}
	if report.Status == "" {
		report.Status = "success"
	}
	return report, true, validateReport(report)
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON input must contain one object")
		}
		return err
	}
	return nil
}

func atomicWrite(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-report-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func validateReport(report Report) error {
	if report.Schema != Schema {
		return fmt.Errorf("agent report schema must be %s", Schema)
	}
	if report.Agent.Role == "" {
		return fmt.Errorf("agent report agent.role is required")
	}
	if report.Status != "success" && report.Status != "failed" {
		return fmt.Errorf("agent report status must be success or failed")
	}
	for _, finding := range report.Findings {
		if err := validateFinding(finding); err != nil {
			return err
		}
	}
	for _, metric := range report.Metrics {
		if err := validateMetric(metric); err != nil {
			return err
		}
	}
	return nil
}

func validateFinding(finding Finding) error {
	if finding.Kind == "" {
		return fmt.Errorf("finding kind is required")
	}
	return nil
}

func validateMetric(metric Metric) error {
	if metric.Name == "" {
		return fmt.Errorf("metric name is required")
	}
	if strings.HasPrefix(metric.Name, "agent.") || strings.HasPrefix(metric.Name, "policy.") {
		return fmt.Errorf("metric %q uses reserved agent/policy namespace", metric.Name)
	}
	return nil
}

func subjectFromEnv(env map[string]string) map[string]string {
	subject := map[string]string{}
	addSubject(subject, "target", env["BACH_AGENT_TARGET"])
	addSubject(subject, "workspace", env["BACH_AGENT_SUBJECT_WORKSPACE"])
	addSubject(subject, "commit", env["BACH_AGENT_SUBJECT_COMMIT"])
	return subject
}

func addSubject(subject map[string]string, key, value string) {
	if value != "" {
		subject[key] = value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
