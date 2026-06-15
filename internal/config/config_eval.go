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

func targetRefEvalContext(
	shells []*Target,
	images []*Target,
	pipelines []*Target,
	groups []*Target,
	agents []*Target,
) *hcl.EvalContext {
	values := map[string]cty.Value{
		"shell":    targetRefObjects("shell", shells),
		"agent":    targetRefObjects("agent", agents),
		"image":    targetRefObjects("image", images),
		"pipeline": targetRefObjects("pipeline", pipelines),
		"group":    targetRefObjects("group", groups),
	}
	return &hcl.EvalContext{Variables: values}
}

func targetRefObjects(kind string, targets []*Target) cty.Value {
	objects := map[string]cty.Value{}
	for _, target := range targets {
		name := strings.TrimPrefix(target.Name, kind+"/")
		objects[name] = cty.StringVal(kind + "." + name)
	}
	if len(objects) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(objects)
}
