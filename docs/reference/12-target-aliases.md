## Target Aliases

Use top-level `alias` blocks to preserve old command names while directing users and agents to canonical targets.

```hcl
alias "staging-kristiyan-deploy" {
  target      = "pipeline.deploy-kristiyan"
  deprecated = "Use pipeline.deploy-kristiyan."
}
```

Aliases resolve before planning and execution, so dry-runs, locks, cache keys, logs, and run history use the canonical target name. Aliases do not create executable target nodes.

`deprecated` is optional. When present, Bachkator prints the message when the alias is used and includes it in `bach explain <alias>`.

List aliases with:

```sh
bach list --aliases
```

Alias targets must point directly to real targets. Alias-to-alias chains are rejected at load time.
