package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

func resolveSelectedProfiles(
	body hcl.Body,
	selected []string,
	variables map[string]string,
	projectEnv []string,
	root string,
) ([]string, error) {
	if len(selected) == 0 {
		return nil, nil
	}
	profiles, err := profileBlocks(body)
	if err != nil {
		return nil, err
	}
	base := projectEnvMap(projectEnv)
	profileValues := map[string]string{}
	for _, name := range selected {
		blocks, exists := profiles[name]
		if !exists {
			return nil, fmt.Errorf("unknown profile %q", name)
		}
		resolved, err := resolveEnvBlocks(
			blocks,
			variables,
			mergeEnvMaps(base, profileValues),
			"profile \""+name+"\" env",
			root,
		)
		if err != nil {
			return nil, err
		}
		for key, value := range projectEnvMap(resolved) {
			profileValues[key] = value
		}
	}
	return envListFromMap(profileValues), nil
}

func profileBlocks(body hcl.Body) (map[string][]*EnvBlock, error) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "profile", LabelNames: []string{"name"}}},
	}
	content, _, _ := body.PartialContent(schema)
	profiles := map[string][]*EnvBlock{}
	for _, profile := range content.Blocks {
		name := profile.Labels[0]
		if _, exists := profiles[name]; exists {
			return nil, fmt.Errorf("duplicate profile %q", name)
		}
		envSchema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "env"}}}
		envContent, _, _ := profile.Body.PartialContent(envSchema)
		blocks := make([]*EnvBlock, 0, len(envContent.Blocks))
		for _, block := range envContent.Blocks {
			blocks = append(blocks, &EnvBlock{Remain: block.Body})
		}
		profiles[name] = blocks
	}
	return profiles, nil
}

func mergeEnvMaps(layers ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, layer := range layers {
		for key, value := range layer {
			merged[key] = value
		}
	}
	return merged
}

func projectRuntimeEnv(project *Project) []string {
	return envListFromMap(
		mergeEnvMaps(projectEnvMap(project.Env), projectEnvMap(project.ProfileEnv)),
	)
}
