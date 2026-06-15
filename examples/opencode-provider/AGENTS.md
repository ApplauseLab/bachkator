# Agent Instructions

## Purpose

`examples/opencode-provider/` demonstrates the first-class OpenCode Agent Target provider with inspectable environment-dump, supplemental report, policy, and resume-session tasks.

## Ownership

- A sample Bachfile for `provider "opencode" { type = "opencode" }`, policy fan-out, improvement attempts, and resume-session behavior.
- Prompt and plan files that ask the agent to print Bach-provided environment variables, create simple workspace evidence, exercise `bach report` supplemental evidence, and resume prior OpenCode sessions.

## Local Contracts

- Keep the example safe to dry-run before launching OpenCode.
- Keep generated work inside the managed agent workspace under `.bach/agents/`.
- Do not require machine-specific paths beyond an installed and authenticated `opencode` CLI.

## Verification

- Use `go run ./cmd/bach -f examples/opencode-provider/Bachfile list` to verify the Bachfile loads.
- Use `go run ./cmd/bach -f examples/opencode-provider/Bachfile --dry-run run agent/print_environment` to inspect the basic provider invocation without launching OpenCode.
- Use `go run ./cmd/bach -f examples/opencode-provider/Bachfile --dry-run run agent/resume_session` to inspect the policy/improvement resume flow without launching OpenCode.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `examples/opencode-provider/`.
