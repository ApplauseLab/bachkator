## Plugins

Plugins are external executables in any language. They run while loading the graph and emit JSON to stdout.

```hcl
plugin "ts_imports" {
  command = ["bun", "examples/plugins/ts-import-graph.ts"]
  sources = {
    api_tests = ["packages/api/tests/**/*.test.ts"]
  }
}

shell "test-api" {
  command = ["bun", "test", "packages/api/tests"]
  inputs  = [plugin.ts_imports.api_tests]
}
```

Plugin environment:

- `BACH_PLUGIN_NAME`: plugin name.
- `BACH_PROJECT_ROOT`: project root.
- `BACH_PLUGIN_INPUTS`: resolved plugin input paths, newline-separated.
- `BACH_PLUGIN_SOURCES`: JSON-encoded `sources` map.

Plugin stdout contract:

```json
{
  "inputs": {
    "api_tests": ["packages/api/src/main.ts"]
  },
  "targets": {
    "shell/test-api": {
      "depends_on": ["shell/generated"],
      "inputs": ["generated.ts"]
    }
  }
}
```

Bachkator merges plugin-provided `depends_on` and `inputs` into existing targets before validation, fingerprinting, and scheduling.

Plugins should not perform side effects. They run on graph load, so `bach list` also runs plugins.
