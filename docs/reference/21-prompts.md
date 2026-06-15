## Prompt Blocks

Top-level `prompt` blocks register prompt files that agent/provider workflows can reference by name:

```hcl
prompt "implementer" {
  path        = "prompts/agents/implementer.md"
  description = "Default implementation-agent prompt"
  version     = "v1"
}
```

`path` is required and must be project-relative. It must point to an existing regular file inside the project root after symlink resolution. `description` and `version` are passive metadata for operators and future provider integrations.

Prompt blocks do not execute anything and do not affect target fingerprints by themselves.
