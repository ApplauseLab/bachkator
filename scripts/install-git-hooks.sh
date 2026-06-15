#!/bin/sh
set -eu

existing="$(git config --local --get core.hooksPath || true)"
hook_dir="$(git rev-parse --git-path hooks)"

case "$existing" in
  "")
    for hook in "$hook_dir"/*; do
      [ -e "$hook" ] || continue
      case "${hook##*/}" in
        *.sample) continue ;;
      esac
      printf 'refusing to replace existing local Git hook: %s\n' "$hook" >&2
      printf 'move it into .githooks/ or remove it before installing tracked hooks\n' >&2
      exit 1
    done
    ;;
  .githooks)
    ;;
  *)
    printf 'refusing to replace existing core.hooksPath: %s\n' "$existing" >&2
    printf 'unset it first if switching to .githooks is intentional\n' >&2
    exit 1
    ;;
esac

git config --local core.hooksPath .githooks
printf 'core.hooksPath=.githooks\n'
