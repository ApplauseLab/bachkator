# Agent Instructions

## Purpose

`.github/` owns GitHub-hosted project automation: Actions workflows, local composite actions, issue forms, and pull request templates.

## Ownership

- `workflows/`: GitHub Actions workflows that call Bach targets for project operations.
- `actions/`: local composite actions shared by workflows in this repository.
- `ISSUE_TEMPLATE/`: GitHub issue forms and issue-template chooser configuration.
- `pull_request_template.md`: default pull request checklist for Bachkator changes.

## Local Contracts

- Use Bach targets for repository operations instead of duplicating test, build, release, lint, or docs commands in workflow YAML.
- PR validation must run with the pull request base SHA `Bachfile`; do not trust a PR-modified Bachfile for required gates.
- Keep workflow permissions minimal. PR gates default to read-only permissions; release workflows may request `contents: write` only when publishing a GitHub release.
- Release workflows must run a Bach dry-run before creating tags or releases.
- Keep composite actions generic and pinned to explicit inputs; do not hard-code secrets.

## Work Guidance

- Prefer local composite actions for repeated workflow setup that belongs to this repository.
- Keep issue forms short, triage-oriented, and aligned with Bachkator terms from `CONTEXT.md`.
- Update this file when adding workflow families, local action contracts, or template responsibilities.

## Verification

- Use `go run ./cmd/bach run --dry-run group/gate` after workflow changes that affect CI gates.
- Use `go run ./cmd/bach --var release_version=vX.Y.Z run --dry-run shell/github-release` and `go run ./cmd/bach --var release_version=vX.Y.Z run --dry-run shell/github-release-publish` after release workflow changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `.github/`.
