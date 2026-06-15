## File Provenance

Use `bach provenance <path> [path ...]` to explain which declared targets generate or consume files.

```sh
bach provenance docs/reference.md
bach provenance --json internal/runner/plan.go
```

Paths are interpreted relative to the project root unless they are absolute. Missing files can still report provenance when they match declared target outputs.

Human output is optimized for agents deciding whether to edit a file directly or regenerate it from sources. For broad source declarations, generated files may also list consumers; this example is abridged:

```text
docs/reference.md
generated: true
source: false
generated_by:
  - shell/docs-generate
    operation: go run ./cmd/bach-docs-gen
    regenerate: bach run shell/docs-generate
consumed_by:
  - shell/build
    operation: sh -c mkdir -p dist && go build -ldflags '-X main.version=dev' -o dist/bach ./cmd/bach
status: unknown
```

JSON output emits one record per queried path:

```json
{
  "paths": [
    {
      "path": "docs/reference.md",
      "generated": true,
      "source": false,
      "producers": [
        {
          "target": "shell/docs-generate",
          "operation": "go run ./cmd/bach-docs-gen",
          "regenerate_command": "bach run shell/docs-generate",
          "outputs": ["docs/reference.md"],
          "inputs": ["cmd/bach-docs-gen", "docs/reference"]
        }
      ],
      "consumers": [
        {
          "target": "shell/build",
          "operation": "sh -c mkdir -p dist && go build -ldflags '-X main.version=dev' -o dist/bach ./cmd/bach",
          "regenerate_command": "bach run shell/build",
          "outputs": ["dist/bach"],
          "inputs": ["cmd", "docs", "internal"]
        }
      ],
      "status": "unknown",
      "reasons": []
    }
  ]
}
```

Directory inputs and outputs match files beneath them. Unknown paths return success with empty `producers` and `consumers`.
