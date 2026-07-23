/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {existsSync, readFileSync, realpathSync} from 'fs';
import {join, sep} from 'path';
import type {Plugin} from 'vite';

// ThunderID npm scope / plugin-name prefix.
const ORG = 'thunderid';

// prismjs language files reference `Prism` as a global with no import — add one so
// Rollup sees the dependency edge and evaluates the core before any language file.
export function prismjsInjectCore(): Plugin {
  return {
    name: 'prismjs-inject-core',
    transform(code: string, id: string) {
      if (/[/\\]prismjs[/\\]components[/\\]prism-(?!core)/.test(id)) {
        // map: null intentionally omitted — prepending a line shifts devtools line numbers by 1.
        return {code: `import Prism from 'prismjs';\n${code}`, map: null};
      }
      return null;
    },
  };
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

interface WorkspaceLinks {
  specifierMap: Map<string, string>;
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
      appSrcRoot = join(appRoot, 'src');
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

      // The app's own `@`/`@/*` aliases (resolved by Vite before this plugin runs) assume
      // every import lives under the app's `src`. When a linked package's own source uses
      // the same alias convention internally, Vite mis-substitutes it onto the app's `src`
      // before this plugin ever sees the original specifier. Detect that mis-substitution
      // (importer lives inside a linked package's `src`, but the substituted id is rooted at
      // the app's `src`) and re-root it onto the package the import actually came from.
      if (importer && appSrcRoot && source.startsWith(appSrcRoot + sep)) {
        const packageSrcRoot = links.packageSrcRoots.find((root) => importer.startsWith(root + sep));
        if (packageSrcRoot) {
          return resolveExistingFile(join(packageSrcRoot, source.slice(appSrcRoot.length)));
        }
      }

      return undefined;
    },
  };
}

function buildWorkspaceLinks(appRoot: string): WorkspaceLinks {
  const specifierMap = new Map<string, string>();
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

    const packageJson = readJson<WorkspacePackageJson>(join(packageDir, 'package.json'));
    let linked = false;

    for (const [subpath, target] of Object.entries(packageJson.exports ?? {})) {
      const distTarget = typeof target === 'string' ? target : target.import;
      const sourcePath = distTarget && resolveDistAsSource(packageDir, distTarget);
      if (!sourcePath) {
        continue;
      }
      specifierMap.set(subpath === '.' ? name : `${name}/${subpath.slice(2)}`, sourcePath);
      linked = true;
    }

    if (linked) {
      packageSrcRoots.push(join(packageDir, 'src'));
    }
  }

  return {specifierMap, packageSrcRoots};
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
  return candidates.find((candidate) => existsSync(candidate));
}

function readJson<T>(path: string): T {
  return JSON.parse(readFileSync(path, 'utf-8')) as T;
}
