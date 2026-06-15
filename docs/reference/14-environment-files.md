## Environment Files

Bachkator loads `.env` from the project root into target operation environments when the file exists. Use `--env-file .env.local` to overlay another file on top of `.env`.

Environment precedence for operations is:

- parent process environment.
- project `.env` values.
- `--env-file` values.
- top-level Bachfile `env` values.
- selected `profile` env values, in CLI order.
- Bachkator runtime values such as `BACH_GIT_COMMIT`.
- target `env` entries.

Environment files support blank lines, `#` comments, optional `export`, `KEY=value`, and single- or double-quoted values.
