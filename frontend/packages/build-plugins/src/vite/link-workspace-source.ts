// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {existsSync, readFileSync, realpathSync} from 'fs';
import {join} from 'path';
import {normalizePath, type Plugin} from 'vite';

// ThunderID npm scope / plugin-name prefix.
const ORG = 'thunderid';

// normalizePath only converts `\` to `/`; it doesn't normalize Windows drive-letter casing
// (e.g. `C:` vs `c:`), which can still differ between paths sourced from different APIs
// (Vite's resolved config root vs realpathSync). Compare case-insensitively on Windows only,
// since the underlying filesystem there is case-insensitive too.
const isWindows = process.platform === 'win32';

function comparablePath(path: string): string {
  return isWindows ? path.toLowerCase() : path;
}

interface AppPackageJson {
  dependencies?: Record<string, string>;
}

interface ExportConditions {
  types?: string;
  import?: string;
  require?: string;
}

type ExportsField = Record<string, string | ExportConditions>;

interface WorkspacePackageJson {
  exports?: ExportsField;
}

export interface LinkWorkspaceSourceOptions {
  root?: string;
}

// A wildcard subpath export (`"./*"`, `"./pages/*"`): specifiers that start with `prefix` and end
// with `suffix` resolve through `distTarget`, with the part in between substituted for its `*`.
interface WildcardLink {
  prefix: string;
  suffix: string;
  packageDir: string;
  distTarget: string;
}

interface WorkspaceLinks {
  specifierMap: Map<string, string>;
  wildcardLinks: WildcardLink[];
  packageSrcRoots: string[];
}

// Redirects @thunderid/* workspace imports to package source during `vite serve` only
// (never build, and never vitest, which also drives Vite in serve mode), so edits
// under packages/*/src hot-update in the consuming app instead of resolving to the
// prebuilt dist/ output that ships in the package's exports map.
export function linkWorkspaceSource(options: LinkWorkspaceSourceOptions = {}): Plugin {
  let links: WorkspaceLinks | undefined;
  let appSrcRoot: string | undefined;

  return {
    name: `${ORG}:link-workspace-source`,
    enforce: 'pre',
    configResolved(config) {
      if (config.command !== 'serve' || process.env['VITEST']) {
        return;
      }
      const appRoot = options.root ?? config.root;
      appSrcRoot = normalizePath(join(appRoot, 'src'));
      links = buildWorkspaceLinks(appRoot);
    },
    resolveId(source, importer) {
      if (!links) {
        return undefined;
      }

      const direct = links.specifierMap.get(source);
      if (direct) {
        return direct;
      }

      const wildcard = resolveWildcard(links.wildcardLinks, source);
      if (wildcard) {
        return wildcard;
      }

      // The app's own `@`/`@/*` aliases (resolved by Vite before this plugin runs) assume
      // every import lives under the app's `src`. When a linked package's own source uses
      // the same alias convention internally, Vite mis-substitutes it onto the app's `src`
      // before this plugin ever sees the original specifier. Detect that mis-substitution
      // (importer lives inside a linked package's `src`, but the substituted id is rooted at
      // the app's `src`) and re-root it onto the package the import actually came from.
      const normalizedSource = normalizePath(source);
      if (importer && appSrcRoot && comparablePath(normalizedSource).startsWith(comparablePath(appSrcRoot) + '/')) {
        const normalizedImporter = normalizePath(importer);
        const packageSrcRoot = links.packageSrcRoots.find((root) =>
          comparablePath(normalizedImporter).startsWith(comparablePath(root) + '/'),
        );
        if (packageSrcRoot) {
          return resolveExistingFile(join(packageSrcRoot, normalizedSource.slice(appSrcRoot.length)));
        }
      }

      return undefined;
    },
  };
}

function buildWorkspaceLinks(appRoot: string): WorkspaceLinks {
  const specifierMap = new Map<string, string>();
  const wildcardLinks: WildcardLink[] = [];
  const packageSrcRoots: string[] = [];
  const appPackageJson = readJson<AppPackageJson>(join(appRoot, 'package.json'));

  for (const [name, spec] of Object.entries(appPackageJson.dependencies ?? {})) {
    if (!name.startsWith(`@${ORG}/`) || !spec.startsWith('workspace:')) {
      continue;
    }

    let packageDir: string;
    try {
      packageDir = realpathSync(join(appRoot, 'node_modules', name));
    } catch {
      continue;
    }

    let packageJson: WorkspacePackageJson;
    try {
      packageJson = readJson<WorkspacePackageJson>(join(packageDir, 'package.json'));
    } catch {
      continue;
    }
    let linked = false;

    for (const [subpath, target] of Object.entries(packageJson.exports ?? {})) {
      const distTarget = typeof target === 'string' ? target : target.import;
      if (distTarget && subpath.includes('*') && distTarget.includes('*')) {
        const star = subpath.indexOf('*');
        wildcardLinks.push({
          prefix: `${name}/${subpath.slice(2, star)}`,
          suffix: subpath.slice(star + 1),
          packageDir,
          distTarget,
        });
        linked = true;
        continue;
      }
      const sourcePath = distTarget && resolveDistAsSource(packageDir, distTarget);
      if (!sourcePath) {
        continue;
      }
      specifierMap.set(subpath === '.' ? name : `${name}/${subpath.slice(2)}`, sourcePath);
      linked = true;
    }

    if (linked) {
      packageSrcRoots.push(normalizePath(join(packageDir, 'src')));
    }
  }

  return {specifierMap, wildcardLinks, packageSrcRoots};
}

// Resolves a specifier matched by a wildcard subpath export to the package source it was built
// from. A match with a `..` segment is ignored, so a subpath cannot reach outside the package.
function resolveWildcard(wildcardLinks: WildcardLink[], source: string): string | undefined {
  for (const {prefix, suffix, packageDir, distTarget} of wildcardLinks) {
    if (!source.startsWith(prefix) || !source.endsWith(suffix) || source.length < prefix.length + suffix.length) {
      continue;
    }
    const match = source.slice(prefix.length, source.length - suffix.length);
    if (!match || match.split('/').includes('..')) {
      continue;
    }
    const sourcePath = resolveDistAsSource(packageDir, distTarget.replace('*', match));
    if (sourcePath) {
      return sourcePath;
    }
  }
  return undefined;
}

function resolveDistAsSource(packageDir: string, distTarget: string): string | undefined {
  const sourceBase = distTarget.replace(/^\.\/dist\//, './src/').replace(/\.(m|c)?js$/, '');
  return resolveExistingFile(join(packageDir, sourceBase));
}

// Tries the source as a file first (`foo/bar.ts`), then as a directory barrel
// (`foo/bar/index.ts`), matching how the `@`-alias imports inside packages reference
// both individual modules and folder barrels.
function resolveExistingFile(base: string): string | undefined {
  const candidates = [`${base}.ts`, `${base}.tsx`, join(base, 'index.ts'), join(base, 'index.tsx')];
  const found = candidates.find((candidate) => existsSync(candidate));
  return found ? normalizePath(found) : undefined;
}

function readJson<T>(path: string): T {
  return JSON.parse(readFileSync(path, 'utf-8')) as T;
}
