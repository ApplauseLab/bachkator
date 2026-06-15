package config

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type evalNamespaceObjects struct {
	inputs    map[string]map[string]cty.Value
	shells    map[string]cty.Value
	agents    map[string]cty.Value
	images    map[string]cty.Value
	pipelines map[string]cty.Value
	groups    map[string]cty.Value
	plugins   map[string]cty.Value
	providers map[string]cty.Value
	prompts   map[string]cty.Value
	policies  map[string]cty.Value
	templates map[string]cty.Value
	resources map[string]cty.Value
	variables map[string]cty.Value
}

func buildEvalContext(
	body hcl.Body,
	variables map[string]string,
	env map[string]string,
	root string,
) *hcl.EvalContext {
	namespaces := newEvalNamespaceObjects(variables)
	content, _, _ := body.PartialContent(evalNamespaceSchema())
	valueContext := evalContextForValues(variables, env, root)
	namespaces.addImageObjects(content.Blocks, valueContext)
	namespaces.addPrimitiveObjects(content.Blocks)
	namespaces.addAgentRefs(content.Blocks)
	namespaces.addPipelineRefs(content.Blocks)
	namespaces.addGroupRefs(content.Blocks)
	namespaces.addShellOutputRefs(content.Blocks)
	namespaces.addAgentOutputRefs(content.Blocks)
	namespaces.addResourceRefs(content.Blocks)
	namespaces.addPluginRefs(content.Blocks)
	namespaces.addInputRefs(content.Blocks)
	return &hcl.EvalContext{Variables: namespaces.values(env), Functions: computedFunctions(root)}
}

func newEvalNamespaceObjects(variables map[string]string) evalNamespaceObjects {
	variableObjects := map[string]cty.Value{}
	for name, value := range variables {
		variableObjects[name] = cty.StringVal(value)
	}
	return evalNamespaceObjects{
		inputs:    map[string]map[string]cty.Value{},
		shells:    map[string]cty.Value{},
		agents:    map[string]cty.Value{},
		images:    map[string]cty.Value{},
		pipelines: map[string]cty.Value{},
		groups:    map[string]cty.Value{},
		plugins:   map[string]cty.Value{},
		providers: map[string]cty.Value{},
		prompts:   map[string]cty.Value{},
		policies:  map[string]cty.Value{},
		templates: map[string]cty.Value{},
		resources: map[string]cty.Value{},
		variables: variableObjects,
	}
}

func evalNamespaceSchema() *hcl.BodySchema {
	return &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "var", LabelNames: []string{"name"}},
			{Type: "input", LabelNames: []string{"kind", "name"}},
			{Type: "resource", LabelNames: []string{"name"}},
			{Type: "plugin", LabelNames: []string{"name"}},
			{Type: "provider", LabelNames: []string{"name"}},
			{Type: "prompt", LabelNames: []string{"name"}},
			{Type: "policy", LabelNames: []string{"name"}},
			{Type: "agent_template", LabelNames: []string{"name"}},
			{Type: "shell", LabelNames: []string{"name"}},
			{Type: "agent", LabelNames: []string{"name"}},
			{Type: "image", LabelNames: []string{"name"}},
			{Type: "pipeline", LabelNames: []string{"name"}},
			{Type: "group", LabelNames: []string{"name"}},
		},
	}
}

func (n evalNamespaceObjects) addImageObjects(blocks hcl.Blocks, valueContext *hcl.EvalContext) {
	for _, block := range blocks {
		switch block.Type {
		case "input":
			kind := block.Labels[0]
			if _, ok := n.inputs[kind]; !ok {
				n.inputs[kind] = map[string]cty.Value{}
			}
		case "image":
			name := block.Labels[0]
			imageName := imageAttribute(block.Body, "image", name, valueContext)
			tag := firstTag(block.Body, valueContext)
			if tag == "" {
				tag = "latest"
			}
			if strings.Contains(tag, ":") || strings.Contains(tag, "/") {
				n.images[name] = cty.ObjectVal(map[string]cty.Value{"tag": cty.StringVal(tag)})
			} else {
				n.images[name] = cty.ObjectVal(
					map[string]cty.Value{"tag": cty.StringVal(imageName + ":" + tag)},
				)
			}
		}
	}
}

func (n evalNamespaceObjects) addPrimitiveObjects(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type == "provider" {
			name := block.Labels[0]
			n.providers[name] = cty.StringVal("provider/" + name)
		}
		if block.Type == "prompt" {
			name := block.Labels[0]
			n.prompts[name] = cty.StringVal("prompt/" + name)
		}
		if block.Type == "policy" {
			name := block.Labels[0]
			n.policies[name] = cty.StringVal("policy/" + name)
		}
		if block.Type == "agent_template" {
			name := block.Labels[0]
			n.templates[name] = cty.StringVal("agent_template/" + name)
		}
	}
}

func (n evalNamespaceObjects) addAgentRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "agent" {
			continue
		}
		name := block.Labels[0]
		n.agents[name] = cty.StringVal("agent." + name)
	}
}

func (n evalNamespaceObjects) addPipelineRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "pipeline" {
			continue
		}
		name := block.Labels[0]
		n.pipelines[name] = cty.StringVal("pipeline." + name)
	}
}

func (n evalNamespaceObjects) addGroupRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "group" {
			continue
		}
		name := block.Labels[0]
		n.groups[name] = cty.StringVal("group." + name)
	}
}

func (n evalNamespaceObjects) addShellOutputRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "shell" {
			continue
		}
		name := block.Labels[0]
		n.shells[name] = targetOutputReferenceObject(block.Body)
	}
}

func (n evalNamespaceObjects) addAgentOutputRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "agent" {
			continue
		}
		name := block.Labels[0]
		n.agents[name] = targetOutputReferenceObject(block.Body)
	}
}

func (n evalNamespaceObjects) addResourceRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type == "resource" {
			name := block.Labels[0]
			n.resources[name] = cty.StringVal("resource/" + name)
		}
	}
}

func (n evalNamespaceObjects) addPluginRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "plugin" {
			continue
		}
		name := block.Labels[0]
		attrs, _ := block.Body.JustAttributes()
		attr, ok := attrs["sources"]
		if !ok {
			n.plugins[name] = cty.EmptyObjectVal
			continue
		}
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !value.Type().IsObjectType() {
			n.plugins[name] = cty.EmptyObjectVal
			continue
		}
		outputs := map[string]cty.Value{}
		for outputName := range value.AsValueMap() {
			outputs[outputName] = cty.StringVal("plugin/" + name + "/" + outputName)
		}
		n.plugins[name] = cty.ObjectVal(outputs)
	}
}

func (n evalNamespaceObjects) addInputRefs(blocks hcl.Blocks) {
	for _, block := range blocks {
		if block.Type != "input" {
			continue
		}
		kind := block.Labels[0]
		name := block.Labels[1]
		n.inputs[kind][name] = cty.StringVal(kind + "/" + name)
	}
}

func (n evalNamespaceObjects) values(env map[string]string) map[string]cty.Value {
	inputObjects := map[string]cty.Value{}
	for kind, values := range n.inputs {
		inputObjects[kind] = cty.ObjectVal(values)
	}
	values := map[string]cty.Value{
		"input":          cty.ObjectVal(inputObjects),
		"shell":          cty.ObjectVal(n.shells),
		"agent":          cty.ObjectVal(n.agents),
		"image":          cty.ObjectVal(n.images),
		"pipeline":       cty.ObjectVal(n.pipelines),
		"group":          cty.ObjectVal(n.groups),
		"plugin":         cty.ObjectVal(n.plugins),
		"provider":       cty.ObjectVal(n.providers),
		"prompt":         cty.ObjectVal(n.prompts),
		"policy":         cty.ObjectVal(n.policies),
		"agent_template": cty.ObjectVal(n.templates),
		"resource":       cty.ObjectVal(n.resources),
		"var":            cty.ObjectVal(n.variables),
	}
	for name, value := range env {
		values[name] = cty.StringVal(value)
	}
	for name, value := range agentTemplatePlaceholderObjects() {
		values[name] = value
	}
	return values
}

func agentTemplatePlaceholderObjects() map[string]cty.Value {
	return map[string]cty.Value{
		"work_item": cty.ObjectVal(map[string]cty.Value{
			"id":   cty.StringVal("${work_item.id}"),
			"slug": cty.StringVal("${work_item.slug}"),
		}),
		"plan": cty.ObjectVal(map[string]cty.Value{
			"id": cty.StringVal("${plan.id}"),
		}),
		"workstream": cty.ObjectVal(map[string]cty.Value{
			"id": cty.StringVal("${workstream.id}"),
		}),
		"factory": cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("${factory.name}"),
		}),
		"workflow": cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("${workflow.name}"),
		}),
	}
}

func targetOutputReferenceObject(body hcl.Body) cty.Value {
	attrs, _ := body.JustAttributes()
	attr, ok := attrs["outputs"]
	if !ok {
		return cty.ObjectVal(map[string]cty.Value{"outputs": cty.EmptyObjectVal})
	}
	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || !value.Type().IsObjectType() && !value.Type().IsMapType() {
		return cty.ObjectVal(map[string]cty.Value{"outputs": cty.EmptyObjectVal})
	}
	outputs := map[string]cty.Value{}
	for name, output := range value.AsValueMap() {
		if output.Type() == cty.String {
			outputs[name] = output
		}
	}
	return cty.ObjectVal(map[string]cty.Value{"outputs": cty.ObjectVal(outputs)})
}

func imageAttribute(body hcl.Body, name string, fallback string, ctx *hcl.EvalContext) string {
	attrs, _ := body.JustAttributes()
	attr, ok := attrs[name]
	if !ok {
		return fallback
	}
	value, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() || value.Type() != cty.String {
		return fallback
	}
	return value.AsString()
}

func firstTag(body hcl.Body, ctx *hcl.EvalContext) string {
	attrs, _ := body.JustAttributes()
	attr, ok := attrs["tags"]
	if !ok {
		return ""
	}
	value, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() || !value.Type().IsTupleType() && !value.Type().IsListType() {
		return ""
	}
	values := value.AsValueSlice()
	if len(values) == 0 || values[0].Type() != cty.String {
		return ""
	}
	return values[0].AsString()
}
