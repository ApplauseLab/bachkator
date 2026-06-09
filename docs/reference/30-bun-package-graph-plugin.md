## Bun Package Graph Plugin

`examples/plugins/bun-package-graph.ts` is an example workspace-level plugin for Bun monorepos that use `@app/*` packages.

```hcl
plugin "bun_packages" {
  command = ["bun", "examples/plugins/bun-package-graph.ts"]
  sources = {
    api = ["packages/api"]
  }
}

shell "test-api" {
  command = ["bun", "test", "packages/api"]
  inputs  = [plugin.bun_packages.api]
}
```

The plugin reads root `package.json` workspaces, package names, exports, dependencies, and dev dependencies. For each configured source package, it emits a deterministic plugin input containing that package and its transitive workspace dependency closure, so changes in shared packages affect dependent target suggestions through `bach affected`.
