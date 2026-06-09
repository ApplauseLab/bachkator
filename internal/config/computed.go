package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func computedFunctions(root string) map[string]function.Function {
	ctx := context.Background()
	return map[string]function.Function{
		"git_short_sha": function.New(&function.Spec{
			Params: []function.Parameter{},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				value, err := computeGitShortSHA(ctx, root)
				if err != nil {
					return cty.NilVal, err
				}
				return cty.StringVal(value), nil
			},
		}),
		"git_dirty_suffix": function.New(&function.Spec{
			Params: []function.Parameter{},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				value, err := computeGitDirtySuffix(ctx, root)
				if err != nil {
					return cty.NilVal, err
				}
				return cty.StringVal(value), nil
			},
		}),
		"file_hash": function.New(&function.Spec{
			VarParam: &function.Parameter{Type: cty.String},
			Type:     function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				paths := make([]string, 0, len(args))
				for _, arg := range args {
					paths = append(paths, arg.AsString())
				}
				value, err := computeFileHash(root, paths)
				if err != nil {
					return cty.NilVal, err
				}
				return cty.StringVal(value), nil
			},
		}),
	}
}

func computeGitShortSHA(ctx context.Context, root string) (string, error) {
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git_short_sha: %w", err)
	}
	if len(commit) <= 12 {
		return commit, nil
	}
	return commit[:12], nil
}

func computeGitDirtySuffix(ctx context.Context, root string) (string, error) {
	status, err := gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git_dirty_suffix: %w", err)
	}
	if status == "" {
		return "", nil
	}
	return "-dirty", nil
}

func computeFileHash(root string, paths []string) (string, error) {
	h := sha256.New()
	files, err := expandFiles(root, paths)
	if err != nil {
		return "", err
	}
	for _, path := range files {
		hashPath(h, root, path)
	}
	return shortHash(h), nil
}

func shortHash(h hash.Hash) string {
	return hex.EncodeToString(h.Sum(nil))[:12]
}
