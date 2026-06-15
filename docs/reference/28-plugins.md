## Plugins

Plugins are typed external executables in any language. The plugin `type` determines when Bachkator runs the executable and which stdout contract it must satisfy.

Existing plugins default to `type = "graph"`. Graph plugins run while loading the Project and emit graph evidence JSON to stdout.

```hcl
plugin "ts_imports" {
  type    = "graph"
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

Graph plugin environment:

- `BACH_PLUGIN_NAME`: plugin name.
- `BACH_PROJECT_ROOT`: project root.
- `BACH_PLUGIN_INPUTS`: resolved plugin input paths, newline-separated.
- `BACH_PLUGIN_SOURCES`: JSON-encoded `sources` map.

Graph plugin stdout contract:

```json
{
  "inputs": {
    "api_tests": ["packages/api/src/main.ts"]
  },
  "targets": {
    "shell.test-api": {
      "depends_on": ["shell.generated"],
      "inputs": ["generated.ts"]
    }
  }
}
```

Bachkator merges graph plugin-provided `depends_on` and `inputs` into existing targets before validation, fingerprinting, and scheduling.

Graph plugins should not perform side effects. They run on graph load, so `bach list` also runs graph plugins.

Quality plugins use `type = "quality"`. They do not run while loading the graph. They run only after a target command succeeds and a quality report declaration references them with `parser = plugin.<name>`.

```hcl
plugin "eslint_quality" {
  type    = "quality"
  command = ["bun", "scripts/bach/parse-eslint-quality.ts"]
  timeout = "10s"
  env     = ["MODE=strict"]
}
```

Quality plugins receive the report path as the first command argument and through environment metadata such as `BACH_QUALITY_REPORT_ABS_PATH`, `BACH_QUALITY_KIND`, `BACH_TARGET`, and `BACH_RUN_ID`. Stdout must contain only normalized quality JSON; stderr is diagnostics and is copied to the target log.

See `examples/plugins/quality-parser` for a complete quality parser plugin example. The normalized stdout schema lives at `docs/schemas/quality-plugin-report.schema.json`.

Quality plugin fields:

- `command` or `shell`: parser executable. One is required; they are mutually exclusive.
- `timeout`: optional parser timeout. Defaults to `30s`.
- `env`: optional environment entries layered onto Bach's runtime environment.

Quality plugins do not support `sources` or `inputs`. Graph plugins do not support `timeout`.

Quality plugin environment:

- `BACH_PLUGIN_NAME`: plugin name.
- `BACH_PLUGIN_TYPE`: `quality`.
- `BACH_PROJECT_ROOT`: project root.
- `BACH_RUN_ID`: current run ID.
- `BACH_RUN_DIRECTORY`: target run directory.
- `BACH_TARGET`: target address being parsed, such as `shell/lint`.
- `BACH_QUALITY_KIND`: report kind, such as `lint` or `tests`.
- `BACH_QUALITY_REPORT_PATH`: report path as declared in the Bachfile after env expansion.
- `BACH_QUALITY_REPORT_ABS_PATH`: absolute report path.
