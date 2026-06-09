package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func evalStringExpr(attr *hcl.Attribute, ctx *hcl.EvalContext) (string, bool, error) {
	value, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return "", false, nil
	}
	if value.Type() != cty.String {
		return "", false, fmt.Errorf("must be a string")
	}
	return value.AsString(), true, nil
}

func evalContextForValues(
	variables map[string]string,
	env map[string]string,
	root string,
) *hcl.EvalContext {
	variableObjects := map[string]cty.Value{}
	for name, value := range variables {
		variableObjects[name] = cty.StringVal(value)
	}
	values := map[string]cty.Value{"var": cty.ObjectVal(variableObjects)}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = cty.StringVal(value)
		}
	}
	for name, value := range env {
		values[name] = cty.StringVal(value)
	}
	return &hcl.EvalContext{Variables: values, Functions: computedFunctions(root)}
}

func buildEvalContext(
	body hcl.Body,
	variables map[string]string,
	env map[string]string,
	root string,
) *hcl.EvalContext {
	inputKinds := map[string]map[string]cty.Value{}
	shellObjects := map[string]cty.Value{}
	imageObjects := map[string]cty.Value{}
	pipelineObjects := map[string]cty.Value{}
	pluginObjects := map[string]cty.Value{}
	resourceObjects := map[string]cty.Value{}
	variableObjects := map[string]cty.Value{}
	for name, value := range variables {
		variableObjects[name] = cty.StringVal(value)
	}
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "var", LabelNames: []string{"name"}},
			{Type: "input", LabelNames: []string{"kind", "name"}},
			{Type: "resource", LabelNames: []string{"name"}},
			{Type: "plugin", LabelNames: []string{"name"}},
			{Type: "shell", LabelNames: []string{"name"}},
			{Type: "image", LabelNames: []string{"name"}},
			{Type: "pipeline", LabelNames: []string{"name"}},
		},
	}
	content, _, _ := body.PartialContent(schema)
	valueContext := evalContextForValues(variables, env, root)
	for _, block := range content.Blocks {
		switch block.Type {
		case "input":
			kind := block.Labels[0]
			if _, ok := inputKinds[kind]; !ok {
				inputKinds[kind] = map[string]cty.Value{}
			}
		case "image":
			name := block.Labels[0]
			imageName := imageAttribute(block.Body, "image", name, valueContext)
			tag := firstTag(block.Body, valueContext)
			if tag == "" {
				tag = "latest"
			}
			if strings.Contains(tag, ":") || strings.Contains(tag, "/") {
				imageObjects[name] = cty.ObjectVal(map[string]cty.Value{"tag": cty.StringVal(tag)})
			} else {
				imageObjects[name] = cty.ObjectVal(
					map[string]cty.Value{"tag": cty.StringVal(imageName + ":" + tag)},
				)
			}
		}
	}
	for _, block := range content.Blocks {
		if block.Type != "pipeline" {
			continue
		}
		name := block.Labels[0]
		pipelineObjects[name] = cty.StringVal("pipeline." + name)
	}
	for _, block := range content.Blocks {
		if block.Type != "shell" {
			continue
		}
		name := block.Labels[0]
		shellObjects[name] = targetOutputReferenceObject(block.Body)
	}
	for _, block := range content.Blocks {
		if block.Type == "resource" {
			name := block.Labels[0]
			resourceObjects[name] = cty.StringVal("resource/" + name)
		}
	}
	for _, block := range content.Blocks {
		if block.Type != "plugin" {
			continue
		}
		name := block.Labels[0]
		attrs, _ := block.Body.JustAttributes()
		attr, ok := attrs["sources"]
		if !ok {
			pluginObjects[name] = cty.EmptyObjectVal
			continue
		}
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !value.Type().IsObjectType() {
			pluginObjects[name] = cty.EmptyObjectVal
			continue
		}
		outputs := map[string]cty.Value{}
		for outputName := range value.AsValueMap() {
			outputs[outputName] = cty.StringVal("plugin/" + name + "/" + outputName)
		}
		pluginObjects[name] = cty.ObjectVal(outputs)
	}
	for _, block := range content.Blocks {
		if block.Type != "input" {
			continue
		}
		kind := block.Labels[0]
		name := block.Labels[1]
		inputKinds[kind][name] = cty.StringVal(kind + "/" + name)
	}
	inputObjects := map[string]cty.Value{}
	for kind, values := range inputKinds {
		inputObjects[kind] = cty.ObjectVal(values)
	}
	values := map[string]cty.Value{
		"input":    cty.ObjectVal(inputObjects),
		"shell":    cty.ObjectVal(shellObjects),
		"image":    cty.ObjectVal(imageObjects),
		"pipeline": cty.ObjectVal(pipelineObjects),
		"plugin":   cty.ObjectVal(pluginObjects),
		"resource": cty.ObjectVal(resourceObjects),
		"var":      cty.ObjectVal(variableObjects),
	}
	for name, value := range env {
		values[name] = cty.StringVal(value)
	}
	return &hcl.EvalContext{Variables: values, Functions: computedFunctions(root)}
}

func targetRefEvalContext(
	shells []*Target,
	images []*Target,
	pipelines []*Target,
) *hcl.EvalContext {
	values := map[string]cty.Value{
		"shell":    targetRefObjects("shell", shells),
		"image":    targetRefObjects("image", images),
		"pipeline": targetRefObjects("pipeline", pipelines),
	}
	return &hcl.EvalContext{Variables: values}
}

func targetRefObjects(kind string, targets []*Target) cty.Value {
	objects := map[string]cty.Value{}
	for _, target := range targets {
		objects[target.Name] = cty.StringVal(kind + "." + target.Name)
	}
	if len(objects) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(objects)
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
