import { existsSync, readFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";

type PluginContext = {
  inputs?: Record<string, string[]>;
  targets: Record<string, { inputs?: string[]; depends_on?: string[] }>;
};

type PackageJSON = {
  name?: string;
  main?: string;
  exports?: string | Record<string, string | { import?: string; default?: string }>;
};

const root = resolve(process.env.BACH_PROJECT_ROOT ?? process.cwd());
const targetEntries = readTargetEntries();
const packageByName = loadWorkspacePackages();

const output: PluginContext = { inputs: {}, targets: {} };

for (const [name, entryPatterns] of Object.entries(targetEntries)) {
  const entries = await expandEntries(entryPatterns);
  const seen = new Set<string>();
  for (const entry of entries) {
    await visit(entry, seen);
  }
  output.inputs![name] = [...seen].sort();
}

process.stdout.write(`${JSON.stringify(output)}\n`);

function readTargetEntries(): Record<string, string[]> {
  const configFile = process.env.BACH_TS_IMPORT_TARGETS_FILE;
  if (configFile) {
    return JSON.parse(readFileSync(resolve(root, configFile), "utf8")) as Record<string, string[]>;
  }
  if (process.env.BACH_PLUGIN_SOURCES) {
    return JSON.parse(process.env.BACH_PLUGIN_SOURCES) as Record<string, string[]>;
  }
  if (process.env.BACH_PLUGIN_TARGETS) {
    return JSON.parse(process.env.BACH_PLUGIN_TARGETS) as Record<string, string[]>;
  }
  return JSON.parse(process.env.BACH_TS_IMPORT_TARGETS ?? "{}") as Record<string, string[]>;
}

async function visit(projectPath: string, seen: Set<string>) {
  const resolved = await resolveProjectPath(projectPath);
  if (!resolved || seen.has(resolved)) return;
  seen.add(resolved);

  const absolute = resolve(root, resolved);
  const file = Bun.file(absolute);
  if (!(await file.exists())) return;
  if (resolved.endsWith(".json")) return;

  const text = stripShebang(await file.text());
  for (const imported of scannerFor(resolved).scanImports(text)) {
    if (!imported.path) continue;
    const importedPath = await resolveImport(resolved, imported.path);
    if (importedPath) await visit(importedPath, seen);
  }
}

function scannerFor(path: string) {
  if (/\.(tsx|jsx)$/.test(path)) return new Bun.Transpiler({ loader: "tsx" });
  return new Bun.Transpiler({ loader: "ts" });
}

function stripShebang(text: string) {
  return text.startsWith("#!") ? text.replace(/^#!.*(?:\r?\n|$)/, "") : text;
}

async function resolveImport(fromProjectPath: string, specifier: string) {
  if (specifier.startsWith(".")) {
    return resolveProjectPath(normalizeProjectPath(join(dirname(fromProjectPath), specifier)));
  }
  if (specifier.startsWith("@app/")) {
    return resolveWorkspaceImport(specifier);
  }
  return undefined;
}

async function resolveWorkspaceImport(specifier: string) {
  const [, packageName, ...rest] = specifier.split("/");
  const packageInfo = packageByName.get(`@app/${packageName}`);
  if (!packageInfo) return undefined;

  const exportKey = rest.length === 0 ? "." : `./${rest.join("/")}`;
  const exported = resolveExport(packageInfo.json, exportKey);
  if (exported) {
    return resolveProjectPath(normalizeProjectPath(join(packageInfo.root, exported)));
  }

  const fallback = rest.length === 0 ? "src/index" : rest.join("/");
  return resolveProjectPath(normalizeProjectPath(join(packageInfo.root, fallback)));
}

function resolveExport(json: PackageJSON, key: string) {
  const exports = json.exports;
  if (typeof exports === "string" && key === ".") return exports;
  if (!exports || typeof exports !== "object") return key === "." ? json.main : undefined;

  const value = exports[key];
  if (typeof value === "string") return value;
  if (value && typeof value === "object") return value.import ?? value.default;
  return undefined;
}

async function resolveProjectPath(path: string) {
  for (const candidate of candidates(path)) {
    if (await Bun.file(resolve(root, candidate)).exists()) return candidate;
  }
  return undefined;
}

async function expandEntries(patterns: string[]) {
  const entries = new Set<string>();
  for (const pattern of patterns) {
    if (hasGlob(pattern)) {
      const glob = new Bun.Glob(pattern);
      for await (const match of glob.scan({ cwd: root, absolute: false, onlyFiles: true })) {
        if (isSourceLike(match)) entries.add(match);
      }
      continue;
    }
    entries.add(pattern);
  }
  return [...entries].sort();
}

function loadWorkspacePackages() {
  const packages = new Map<string, { root: string; json: PackageJSON }>();
  for (const packageJSONPath of workspacePackageJSONPaths()) {
    const json = JSON.parse(readFileSync(resolve(root, packageJSONPath), "utf8")) as PackageJSON;
    if (!json.name) continue;
    packages.set(json.name, { root: dirname(packageJSONPath), json });
  }
  return packages;
}

function workspacePackageJSONPaths() {
  const rootPackageJSON = JSON.parse(readFileSync(resolve(root, "package.json"), "utf8")) as { workspaces?: string[] };
  const workspaces = rootPackageJSON.workspaces ?? ["packages/*"];
  const paths: string[] = [];
  for (const workspace of workspaces) {
    const glob = new Bun.Glob(`${workspace}/package.json`);
    for (const match of glob.scanSync({ cwd: root, absolute: false, onlyFiles: true })) {
      paths.push(match);
    }
  }
  return paths.sort();
}

function candidates(path: string) {
  if (/\.(js|jsx|mjs|cjs)$/.test(path)) {
    const withoutExtension = path.replace(/\.(js|jsx|mjs|cjs)$/, "");
    return [
      path,
      `${withoutExtension}.ts`,
      `${withoutExtension}.tsx`,
      `${withoutExtension}.mts`,
      `${withoutExtension}.cts`,
    ];
  }
  if (isSourceLike(path)) return [path];
  return [
    `${path}.ts`,
    `${path}.tsx`,
    `${path}.mts`,
    `${path}.cts`,
    `${path}.js`,
    `${path}.jsx`,
    `${path}.json`,
    `${path}/index.ts`,
    `${path}/index.tsx`,
    `${path}/index.mts`,
    `${path}/index.cts`,
    `${path}/index.js`,
    `${path}/index.jsx`,
  ];
}

function hasGlob(pattern: string) {
  return /[*?[\]{}]/.test(pattern);
}

function isSourceLike(path: string) {
  return /\.(ts|tsx|mts|cts|js|jsx|json)$/.test(path) && !path.endsWith(".d.ts");
}

function normalizeProjectPath(path: string) {
  return relative(root, resolve(root, path)).replaceAll("\\", "/");
}
