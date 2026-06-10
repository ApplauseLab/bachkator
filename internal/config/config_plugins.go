package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	pluginTypeGraph   = "graph"
	pluginTypeQuality = "quality"
)

func registerPlugins(project *Project, plugins []*Plugin) error {
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		if plugin.Type == "" {
			plugin.Type = pluginTypeGraph
		}
		switch plugin.Type {
		case pluginTypeGraph:
			if plugin.Timeout != "" {
				return fmt.Errorf("plugin %q graph plugins do not support timeout", plugin.Name)
			}
		case pluginTypeQuality:
			if len(plugin.Sources) > 0 {
				return fmt.Errorf("plugin %q quality plugins do not support sources", plugin.Name)
			}
			if len(plugin.Inputs) > 0 {
				return fmt.Errorf("plugin %q quality plugins do not support inputs", plugin.Name)
			}
			if plugin.Timeout != "" {
				timeout, err := time.ParseDuration(plugin.Timeout)
				if err != nil {
					return fmt.Errorf(
						"plugin %q timeout %q is invalid: %w",
						plugin.Name,
						plugin.Timeout,
						err,
					)
				}
				if timeout <= 0 {
					return fmt.Errorf("plugin %q timeout must be greater than zero", plugin.Name)
				}
				plugin.TimeoutDuration = timeout
			}
		default:
			return fmt.Errorf("plugin %q has unsupported type %q", plugin.Name, plugin.Type)
		}
		if _, exists := project.Plugins[plugin.Name]; exists {
			return fmt.Errorf("duplicate plugin %q", plugin.Name)
		}
		project.Plugins[plugin.Name] = plugin
	}
	return nil
}

func runPlugins(ctx context.Context, project *Project, plugins []*Plugin) error {
	for _, plugin := range plugins {
		if plugin == nil || plugin.Type == pluginTypeQuality {
			continue
		}
		pluginContext, err := runPlugin(ctx, project, plugin)
		if err != nil {
			return err
		}
		for name, paths := range pluginContext.Inputs {
			key := "plugin/" + plugin.Name + "/" + name
			project.Inputs[key] = &Input{
				Kind: "plugin",
				Name: plugin.Name + "/" + name,
				Srcs: paths,
			}
		}
		for name, patch := range pluginContext.Targets {
			canonicalName, err := canonicalTargetRef(name)
			if err != nil {
				return fmt.Errorf("plugin %q target reference: %w", plugin.Name, err)
			}
			target, ok := project.Targets[name]
			if !ok {
				target, ok = project.Targets[canonicalName]
			}
			if !ok {
				return fmt.Errorf("plugin %q referenced missing target %q", plugin.Name, name)
			}
			dependsOn, err := canonicalTargetRefs(patch.DependsOn)
			if err != nil {
				return fmt.Errorf("plugin %q target %q depends_on: %w", plugin.Name, name, err)
			}
			target.DependsOn = appendUnique(target.DependsOn, dependsOn...)
			target.Inputs = appendUnique(target.Inputs, patch.Inputs...)
		}
	}
	return nil
}

func runPlugin(ctx context.Context, project *Project, plugin *Plugin) (PluginContext, error) {
	if plugin.Shell != "" && len(plugin.Command) > 0 {
		return PluginContext{}, fmt.Errorf(
			"plugin %q must use command or shell, not both",
			plugin.Name,
		)
	}
	if plugin.Shell == "" && len(plugin.Command) == 0 {
		return PluginContext{}, nil
	}

	workdir := project.Root
	if plugin.WorkDir != "" {
		workdir = absPath(project.Root, plugin.WorkDir)
	}

	var cmd *exec.Cmd
	if plugin.Shell != "" {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", plugin.Shell)
	} else {
		cmd = exec.CommandContext(ctx, plugin.Command[0], plugin.Command[1:]...)
	}
	cmd.Dir = workdir
	sourcesJSON, err := json.Marshal(plugin.Sources)
	if err != nil {
		return PluginContext{}, fmt.Errorf(
			"plugin %q sources could not be encoded: %w",
			plugin.Name,
			err,
		)
	}
	cmd.Env = append(os.Environ(),
		"BACH_PLUGIN_NAME="+plugin.Name,
		"BACH_PROJECT_ROOT="+project.Root,
		"BACH_PLUGIN_INPUTS="+strings.Join(project.resolveInputs(plugin.Inputs), "\n"),
		"BACH_PLUGIN_SOURCES="+string(sourcesJSON),
	)
	cmd.Env = append(cmd.Env, plugin.Env...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return PluginContext{}, fmt.Errorf(
				"plugin %q failed: %w\n%s",
				plugin.Name,
				err,
				string(exitErr.Stderr),
			)
		}
		return PluginContext{}, fmt.Errorf("plugin %q failed: %w", plugin.Name, err)
	}
	if strings.TrimSpace(string(output)) == "" {
		return PluginContext{}, nil
	}
	var pluginContext PluginContext
	if err := json.Unmarshal(output, &pluginContext); err != nil {
		return PluginContext{}, fmt.Errorf("plugin %q emitted invalid JSON: %w", plugin.Name, err)
	}
	return pluginContext, nil
}
