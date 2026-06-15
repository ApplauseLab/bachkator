package quality

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

const defaultQualityPluginTimeout = 30 * time.Second

func parsePluginReport(req ParseRequest) (Report, error) {
	plugin, ok := req.Plugins[req.Declaration.Parser]
	if !ok {
		return Report{}, fmt.Errorf("unknown quality parser plugin %q", req.Declaration.Parser)
	}
	if plugin.Type != "quality" {
		return Report{}, fmt.Errorf("plugin %q is not a quality plugin", plugin.Name)
	}
	if plugin.Shell != "" && len(plugin.Command) > 0 {
		return Report{}, fmt.Errorf("plugin %q must use command or shell, not both", plugin.Name)
	}
	if plugin.Shell == "" && len(plugin.Command) == 0 {
		return Report{}, fmt.Errorf("plugin %q must set command or shell", plugin.Name)
	}
	timeout := plugin.Timeout
	if timeout == 0 {
		timeout = defaultQualityPluginTimeout
	}
	baseCtx := req.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()
	cmd := pluginCommand(ctx, req, plugin)
	cmd.Dir = req.Workdir
	cmd.Env = pluginEnv(req, plugin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Report{}, pluginRunError(ctx, plugin, timeout, &stderr, err)
	}
	parsed, err := decodePluginQuality(stdout.Bytes())
	if err != nil {
		return Report{}, fmt.Errorf("plugin %q emitted invalid quality JSON: %w", plugin.Name, err)
	}
	return parsed, nil
}

func pluginCommand(ctx context.Context, req ParseRequest, plugin *model.Plugin) *exec.Cmd {
	if plugin.Shell != "" {
		return exec.CommandContext(
			ctx,
			"/bin/sh",
			"-c",
			plugin.Shell,
			"bach-quality-plugin",
			req.Path,
		)
	}
	args := append([]string(nil), plugin.Command[1:]...)
	args = append(args, req.Path)
	return exec.CommandContext(ctx, plugin.Command[0], args...)
}

func pluginRunError(
	ctx context.Context,
	plugin *model.Plugin,
	timeout time.Duration,
	stderr *bytes.Buffer,
	err error,
) error {
	if ctx.Err() != nil {
		return fmt.Errorf("plugin %q timed out after %s", plugin.Name, timeout)
	}
	message := strings.TrimSpace(stderr.String())
	if message != "" {
		return fmt.Errorf("plugin %q failed: %w: %s", plugin.Name, err, message)
	}
	return fmt.Errorf("plugin %q failed: %w", plugin.Name, err)
}

func pluginEnv(req ParseRequest, plugin *model.Plugin) []string {
	env := os.Environ()
	for key, value := range req.Env {
		env = append(env, key+"="+value)
	}
	env = append(env,
		"BACH_PLUGIN_NAME="+plugin.Name,
		"BACH_PLUGIN_TYPE="+plugin.Type,
		"BACH_PROJECT_ROOT="+req.ProjectRoot,
		"BACH_RUN_ID="+req.RunID,
		"BACH_RUN_DIRECTORY="+req.Env["BACH_RUN_DIRECTORY"],
		"BACH_TARGET="+req.TargetName,
		"BACH_QUALITY_KIND="+req.Declaration.Kind,
		"BACH_QUALITY_REPORT_PATH="+req.DisplayPath,
		"BACH_QUALITY_REPORT_ABS_PATH="+req.Path,
	)
	env = append(env, plugin.Env...)
	return env
}

func decodePluginQuality(data []byte) (Report, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Report{}, err
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return Report{}, err
	}
	for key := range raw {
		if key != "metrics" && key != "findings" {
			return Report{}, fmt.Errorf("unknown top-level field %q", key)
		}
	}
	if len(raw) == 0 {
		return Report{}, fmt.Errorf("must include metrics or findings")
	}
	var report Report
	if value, ok := raw["metrics"]; ok {
		if err := json.Unmarshal(value, &report.Metrics); err != nil {
			return Report{}, fmt.Errorf("metrics: %w", err)
		}
	}
	if value, ok := raw["findings"]; ok {
		if err := json.Unmarshal(value, &report.Findings); err != nil {
			return Report{}, fmt.Errorf("findings: %w", err)
		}
	}
	for _, metric := range report.Metrics {
		if metric.Name == "" {
			return Report{}, fmt.Errorf("metric name is required")
		}
	}
	for _, finding := range report.Findings {
		if finding.Kind == "" {
			return Report{}, fmt.Errorf("finding kind is required")
		}
	}
	return report, nil
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
