// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {mkdirSync, realpathSync, rmSync, writeFileSync} from 'fs';
import {tmpdir} from 'os';
import {join} from 'path';
import {normalizePath, type Plugin} from 'vite';
import {afterEach, beforeEach, describe, expect, it} from 'vitest';
import {linkWorkspaceSource} from '../link-workspace-source';

interface FakeResolvedConfig {
  command: 'build' | 'serve';
  root: string;
}

type ConfigResolvedHook = (config: FakeResolvedConfig) => void;
type ResolveIdHook = (source: string, importer?: string) => string | undefined;

function callConfigResolved(plugin: Plugin, config: FakeResolvedConfig): void {
  (plugin.configResolved as unknown as ConfigResolvedHook).call(plugin, config);
}

function callResolveId(plugin: Plugin, source: string, importer?: string): string | undefined {
  return (plugin.resolveId as unknown as ResolveIdHook).call(plugin, source, importer);
}

function writeJson(path: string, value: unknown): void {
  writeFileSync(path, JSON.stringify(value));
}

function createLinkedPackage(appRoot: string, name: string, exports: Record<string, unknown>): string {
  const packageDir = join(appRoot, 'node_modules', name);
  mkdirSync(packageDir, {recursive: true});
  writeJson(join(packageDir, 'package.json'), {exports});
  return packageDir;
}

describe('linkWorkspaceSource', () => {
  const testDir = join(realpathSync(tmpdir()), 'build-plugins-link-workspace-source');
  const appRoot = join(testDir, 'app');
  let originalVitestEnv: string | undefined;

  beforeEach(() => {
    originalVitestEnv = process.env['VITEST'];
    mkdirSync(appRoot, {recursive: true});
  });

  afterEach(() => {
    if (originalVitestEnv === undefined) {
      delete process.env['VITEST'];
    } else {
      process.env['VITEST'] = originalVitestEnv;
    }
    rmSync(testDir, {recursive: true, force: true});
  });

  it('does nothing when the Vite command is not "serve"', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {}});

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'build', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo')).toBeUndefined();
  });

  it('does nothing under vitest, which also drives Vite in serve mode', () => {
    // process.env.VITEST is already set by the vitest test runner itself here.
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {'.': {import: './dist/index.js'}});
    mkdirSync(join(packageDir, 'src'), {recursive: true});
    writeFileSync(join(packageDir, 'src', 'index.ts'), 'export default 1;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo')).toBeUndefined();
  });

  it('redirects a workspace package specifier to its source file', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {
      '.': {import: './dist/index.js'},
      './sub': {import: './dist/sub.js'},
    });
    mkdirSync(join(packageDir, 'src'), {recursive: true});
    writeFileSync(join(packageDir, 'src', 'index.ts'), 'export default 1;');
    writeFileSync(join(packageDir, 'src', 'sub.ts'), 'export default 2;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo')).toBe(normalizePath(join(packageDir, 'src', 'index.ts')));
    expect(callResolveId(plugin, '@thunderid/foo/sub')).toBe(normalizePath(join(packageDir, 'src', 'sub.ts')));
  });

  it('resolves a subpath export to its index barrel when no matching file exists', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {
      './widgets': {import: './dist/widgets/index.js'},
    });
    mkdirSync(join(packageDir, 'src', 'widgets'), {recursive: true});
    writeFileSync(join(packageDir, 'src', 'widgets', 'index.ts'), 'export default 1;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo/widgets')).toBe(
      normalizePath(join(packageDir, 'src', 'widgets', 'index.ts')),
    );
  });

  it('redirects a wildcard subpath export to the matching source file', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {
      '.': {import: './dist/index.js'},
      './*': {import: './dist/*.js'},
    });
    mkdirSync(join(packageDir, 'src', 'pages'), {recursive: true});
    writeFileSync(join(packageDir, 'src', 'index.ts'), 'export default 1;');
    writeFileSync(join(packageDir, 'src', 'pages', 'ListPage.tsx'), 'export default 2;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo')).toBe(normalizePath(join(packageDir, 'src', 'index.ts')));
    expect(callResolveId(plugin, '@thunderid/foo/pages/ListPage')).toBe(
      normalizePath(join(packageDir, 'src', 'pages', 'ListPage.tsx')),
    );
    expect(callResolveId(plugin, '@thunderid/foo/pages/Missing')).toBeUndefined();
  });

  it('does not resolve a wildcard subpath export outside the package source', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {'./*': {import: './dist/*.js'}});
    mkdirSync(join(packageDir, 'src'), {recursive: true});
    writeFileSync(join(packageDir, 'secret.ts'), 'export default 1;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo/../secret')).toBeUndefined();
  });

  it('ignores dependencies that are not @thunderid workspace packages', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {
      dependencies: {
        '@thunderid/pinned': '^1.0.0',
        'other-scope': 'workspace:*',
      },
    });
    createLinkedPackage(appRoot, '@thunderid/pinned', {'.': {import: './dist/index.js'}});

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/pinned')).toBeUndefined();
  });

  it('skips a workspace dependency whose node_modules entry is missing', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/missing': 'workspace:*'}});
    // Intentionally do not create node_modules/@thunderid/missing.

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/missing')).toBeUndefined();
  });

  it('skips a linked package with an unreadable package.json instead of throwing', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/broken': 'workspace:*'}});
    const packageDir = join(appRoot, 'node_modules', '@thunderid/broken');
    mkdirSync(packageDir, {recursive: true});
    writeFileSync(join(packageDir, 'package.json'), '{ not valid json');

    const plugin = linkWorkspaceSource();

    expect(() => callConfigResolved(plugin, {command: 'serve', root: appRoot})).not.toThrow();
    expect(callResolveId(plugin, '@thunderid/broken')).toBeUndefined();
  });

  it('re-roots an alias mis-substitution from a linked package back onto its own source', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    const packageDir = createLinkedPackage(appRoot, '@thunderid/foo', {'.': {import: './dist/index.js'}});
    mkdirSync(join(packageDir, 'src', 'utils'), {recursive: true});
    writeFileSync(join(packageDir, 'src', 'index.ts'), 'export default 1;');
    writeFileSync(join(packageDir, 'src', 'utils', 'helper.ts'), 'export const helper = 1;');

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    // Simulates Vite having already mis-substituted the package's own `@/utils/helper`
    // import onto the app's `src`, because the importer (inside the package) uses the
    // same alias convention as the app.
    const misSubstitutedSource = join(appRoot, 'src', 'utils', 'helper');
    const importerInsidePackage = join(packageDir, 'src', 'index.ts');

    expect(callResolveId(plugin, misSubstitutedSource, importerInsidePackage)).toBe(
      normalizePath(join(packageDir, 'src', 'utils', 'helper.ts')),
    );
  });

  it('leaves an app-src-rooted import alone when there is no importer', () => {
    delete process.env['VITEST'];
    writeJson(join(appRoot, 'package.json'), {dependencies: {}});

    const plugin = linkWorkspaceSource();
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, join(appRoot, 'src', 'App.tsx'))).toBeUndefined();
  });

  it('honors an explicit root option instead of the resolved config root', () => {
    delete process.env['VITEST'];
    const explicitRoot = join(testDir, 'explicit-root');
    const packageDir = join(explicitRoot, 'node_modules', '@thunderid/foo');
    mkdirSync(join(packageDir, 'src'), {recursive: true});
    writeJson(join(explicitRoot, 'package.json'), {dependencies: {'@thunderid/foo': 'workspace:*'}});
    writeJson(join(packageDir, 'package.json'), {exports: {'.': {import: './dist/index.js'}}});
    writeFileSync(join(packageDir, 'src', 'index.ts'), 'export default 1;');

    const plugin = linkWorkspaceSource({root: explicitRoot});
    // config.root points elsewhere; the plugin should use `options.root` instead.
    callConfigResolved(plugin, {command: 'serve', root: appRoot});

    expect(callResolveId(plugin, '@thunderid/foo')).toBe(normalizePath(join(packageDir, 'src', 'index.ts')));
  });
});
