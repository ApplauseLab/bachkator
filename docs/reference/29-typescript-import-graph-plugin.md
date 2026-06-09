## TypeScript Import Graph Plugin

`examples/plugins/ts-import-graph.ts` is self-contained and only needs Bun. It does not require the `typescript` package.

It supports:

- glob source entries.
- relative imports.
- `.js` specifiers resolving to `.ts` and `.tsx` files.
- JSON leaf inputs.
- `@app/*` workspace imports through `package.json` exports.
- TSX parsing via Bun's `tsx` loader.
- shebang stripping before scanning imports.
