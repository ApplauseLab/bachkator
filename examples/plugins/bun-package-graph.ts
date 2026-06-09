import { existsSync, readFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";

type PluginContext = {
  inputs: Record<string, string[]>;
  targets: Record<string, { inputs?: string[]; depends_on?: string[] }>;
};

type PackageJSON = {
  name?: string;
  main?: string;
  exports?: string | Record<string, string | { import?: string; default?: string }>;
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
};

type RootPackageJSON = {
  workspaces?: string[] | { packages?: string[] };
};

type WorkspacePackage = {
  name: string;
  root: string;
  json: PackageJSON;
};

const root = resolve(process.env.BACH_PROJECT_ROOT ?? process.cwd());
const sources = readSources();
const packages = loadWorkspacePackages();
const packageByName = new Map(packages.map((pkg) => [pkg.name, pkg]));

const output: PluginContext = { inputs: {}, targets: {} };

for (const key of Object.keys(sources).sort()) {
  const inputRoots = new Set<string>();
  for (const source of sources[key] ?? []) {
    const pkg = resolveSourcePackage(source);
    if (!pkg) continue;
    for (const dependency of dependencyClosure(pkg)) {
      inputRoots.add(dependency.root);
    }
  }
  output.inputs[key] = [...inputRoots].sort();
}

process.stdout.write(`${JSON.stringify(output)}\n`);

function readSources(): Record<string, string[]> {
  if (process.env.BACH_PLUGIN_SOURCES) {
    return JSON.parse(process.env.BACH_PLUGIN_SOURCES) as Record<string, string[]>;
  }
  return {};
}

function loadWorkspacePackages() {
  const loaded: WorkspacePackage[] = [];
  for (const packageJSONPath of workspacePackageJSONPaths()) {
    const json = JSON.parse(readFileSync(resolve(root, packageJSONPath), "utf8")) as PackageJSON;
    if (!json.name) continue;
    loaded.push({ name: json.name, root: dirname(packageJSONPath), json });
  }
  return loaded.sort((a, b) => a.root.localeCompare(b.root));
}

function workspacePackageJSONPaths() {
  const rootPackageJSONPath = resolve(root, "package.json");
  if (!existsSync(rootPackageJSONPath)) return [];

  const rootPackageJSON = JSON.parse(readFileSync(rootPackageJSONPath, "utf8")) as RootPackageJSON;
  const workspaces = workspacePatterns(rootPackageJSON.workspaces);
  const paths: string[] = [];
  for (const workspace of workspaces) {
    const glob = new Bun.Glob(`${workspace}/package.json`);
    for (const match of glob.scanSync({ cwd: root, absolute: false, onlyFiles: true })) {
      paths.push(normalizeProjectPath(match));
    }
  }
  return paths.sort();
}

function workspacePatterns(workspaces: RootPackageJSON["workspaces"]) {
  if (Array.isArray(workspaces)) return workspaces;
  if (workspaces?.packages) return workspaces.packages;
  return [];
}

function resolveSourcePackage(source: string) {
  const byName = packageByName.get(source);
  if (byName) return byName;

  const exportedPath = resolveWorkspaceImport(source);
  if (exportedPath) return packageForPath(exportedPath);

  return packageForPath(source);
}

function resolveWorkspaceImport(specifier: string) {
  if (!specifier.startsWith("@app/")) return undefined;

  const [, packageName, ...rest] = specifier.split("/");
  const pkg = packageByName.get(`@app/${packageName}`);
  if (!pkg) return undefined;

  const exportKey = rest.length === 0 ? "." : `./${rest.join("/")}`;
  const exported = resolveExport(pkg.json, exportKey);
  if (exported) return normalizeProjectPath(join(pkg.root, exported));

  const fallback = rest.length === 0 ? "src/index" : rest.join("/");
  return normalizeProjectPath(join(pkg.root, fallback));
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

function packageForPath(path: string) {
  const normalized = normalizeProjectPath(path);
  return packages
    .filter((pkg) => normalized === pkg.root || normalized.startsWith(`${pkg.root}/`))
    .sort((a, b) => b.root.length - a.root.length)[0];
}

function dependencyClosure(pkg: WorkspacePackage) {
  const seen = new Set<string>();
  const out: WorkspacePackage[] = [];

  function visit(current: WorkspacePackage) {
    if (seen.has(current.name)) return;
    seen.add(current.name);
    out.push(current);
    for (const dependencyName of dependencyNames(current).sort()) {
      const dependency = packageByName.get(dependencyName);
      if (dependency) visit(dependency);
    }
  }

  visit(pkg);
  return out.sort((a, b) => a.root.localeCompare(b.root));
}

function dependencyNames(pkg: WorkspacePackage) {
  return Object.keys({ ...(pkg.json.dependencies ?? {}), ...(pkg.json.devDependencies ?? {}) });
}

function normalizeProjectPath(path: string) {
  return relative(root, resolve(root, path)).replaceAll("\\", "/");
}
