// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import fs from 'node:fs';
import path from 'node:path';
import type {AllContent, LoadContext, Plugin} from '@docusaurus/types';
import {parse as parseYaml} from 'yaml';
import {validateEntry, validateRegistry} from './validate';
import {ECOSYSTEM_ICON_NAMES} from '../../src/components/Ecosystem/iconNames';
import type {EcosystemEntry, EcosystemGlobalData} from '../../src/types/ecosystem';

/** Docs version name Docusaurus uses for the unreleased `content/` tree. */
const CURRENT_VERSION = 'current';

const ENTRY_FILENAME = 'entry.yaml';

/**
 * Directory holding the SDKs & tools docs, under `content/` and under each
 * versioned snapshot. Only the `entry.yaml` files are read here; the `.mdx`
 * pages alongside them belong to the docs plugin.
 */
const REGISTRY_DIR = 'sdks-and-tools';
const CURRENT_ROOT = path.join('content', REGISTRY_DIR);
const VERSIONED_ROOT = 'versioned_docs';

interface EcosystemSource {
  /** Docs version name: `current`, `v1.0.x`, ... */
  version: string;
  /** Absolute path to the directory holding the per-entry folders. */
  dir: string;
}

/**
 * Lists the registry directory for every docs version.
 *
 * `versioned_docs/version-v1.0.x/sdks-and-tools` is a verbatim snapshot taken by
 * `docusaurus docs:version`, so a released version keeps the registry it
 * shipped with while `content/sdks` moves on. No extra wiring is needed when a
 * new version is cut.
 */
function findSources(siteDir: string): EcosystemSource[] {
  const sources: EcosystemSource[] = [];

  const currentDir = path.join(siteDir, CURRENT_ROOT);
  if (fs.existsSync(currentDir)) {
    sources.push({version: CURRENT_VERSION, dir: currentDir});
  }

  const versionedDir = path.join(siteDir, VERSIONED_ROOT);
  if (fs.existsSync(versionedDir)) {
    for (const name of fs.readdirSync(versionedDir)) {
      if (!name.startsWith('version-')) continue;
      const dir = path.join(versionedDir, name, REGISTRY_DIR);
      if (fs.existsSync(dir)) {
        sources.push({version: name.slice('version-'.length), dir});
      }
    }
  }

  return sources;
}

/**
 * Substitutes `{{ProductName}}` and `{{productSlug}}` through every string in a
 * parsed entry, matching what `docusaurus.config.ts` does for frontmatter and
 * code blocks. Doing it here keeps the placeholders out of every component that
 * reads the registry.
 */
function replacePlaceholders<T>(value: T, productName: string, productSlug: string): T {
  if (typeof value === 'string') {
    return value.replaceAll('{{ProductName}}', productName).replaceAll('{{productSlug}}', productSlug) as T;
  }
  if (Array.isArray(value)) {
    return (value as unknown[]).map((item) => replacePlaceholders(item, productName, productSlug)) as T;
  }
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([k, v]) => [
        k,
        replacePlaceholders(v, productName, productSlug),
      ]),
    ) as T;
  }
  return value;
}

function readEntries(source: EcosystemSource, productName: string, productSlug: string): {
  entries: EcosystemEntry[];
  problems: string[];
  warnings: string[];
} {
  const entries: EcosystemEntry[] = [];
  const problems: string[] = [];
  const warnings: string[] = [];

  for (const dirName of fs.readdirSync(source.dir).sort()) {
    const file = path.join(source.dir, dirName, ENTRY_FILENAME);
    if (!fs.existsSync(file)) continue;

    let raw: unknown;
    try {
      raw = parseYaml(fs.readFileSync(file, 'utf8'));
    } catch (error) {
      problems.push(`  ${file}: is not valid YAML (${(error as Error).message})`);
      continue;
    }

    const relative = path.relative(source.dir, file);
    const result = validateEntry(raw, relative);
    problems.push(...result.problems);
    warnings.push(...result.warnings);

    if (result.entry) {
      if (result.entry.id !== dirName) {
        problems.push(`  ${relative}: id "${result.entry.id}" does not match its directory name "${dirName}"`);
      }
      entries.push(replacePlaceholders(result.entry, productName, productSlug));
    }
  }

  return {entries, problems, warnings};
}

interface LoadedDoc {
  permalink: string;
  /** `@site/`-prefixed path to the source file. */
  source: string;
}

interface LoadedVersion {
  docs: LoadedDoc[];
}

/**
 * Every doc permalink across every version, with the version prefix stripped.
 *
 * `docs:` values are written as `/docs/next/...` and rewritten to the reader's
 * version at render time, so they are compared against the version-agnostic
 * tail rather than any one version's literal path.
 */
function collectDocPaths(allContent: AllContent): Map<string, string> {
  const docsContent = allContent['docusaurus-plugin-content-docs']?.default as
    | {loadedVersions?: LoadedVersion[]}
    | undefined;

  const paths = new Map<string, string>();
  for (const version of docsContent?.loadedVersions ?? []) {
    for (const doc of version.docs) {
      paths.set(doc.permalink.replace(/^\/docs\/[^/]+\//, '').replace(/\/$/, ''), doc.source.replace('@site/', ''));
    }
  }
  return paths;
}

/** Average adult reading speed, in words per minute. */
const WORDS_PER_MINUTE = 200;

/**
 * Lines of code a reader gets through in a minute.
 *
 * Far slower than prose: a code block in a guide is read, compared against your
 * own file, and often typed out. Counting it at reading speed, or excluding it
 * as prose-only estimates do, puts "1 min" on a guide that takes ten.
 */
const CODE_LINES_PER_MINUTE = 20;

/** Every string in a value, so a guide can be measured whatever its shape. */
function collectText(value: unknown, prose: string[], code: string[], inCode = false): void {
  if (typeof value === 'string') {
    (inCode ? code : prose).push(value);
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) collectText(item, prose, code, inCode);
    return;
  }
  if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) {
      collectText(child, prose, code, inCode || key === 'code' || key === 'signature' || key === 'example');
    }
  }
}

/**
 * Fills in each guide's reading time from its own sections.
 *
 * Derived rather than authored, so it cannot drift from the guide, and so the
 * card on the entry's page and the guide page itself always agree.
 */
function fillReadingTimes(byVersion: Record<string, EcosystemEntry[]>): void {
  for (const entries of Object.values(byVersion)) {
    for (const entry of entries) {
      for (const guide of entry.guides ?? []) {
        if (guide.minutes !== undefined) continue;
        const prose: string[] = [];
        const code: string[] = [];
        collectText(guide.sections, prose, code);
        const words = prose.join(' ').split(/\s+/).filter(Boolean).length;
        const codeLines = code.join('\n').split('\n').length;
        guide.minutes = Math.max(1, Math.round(words / WORDS_PER_MINUTE + codeLines / CODE_LINES_PER_MINUTE));
      }
    }
  }
}

/**
 * Drops every `docs:` route that does not match a page the docs plugin built,
 * and reports what it dropped.
 *
 * A registry entry may name a page before that page exists: an API reference
 * waiting on its generator, say. Left alone, the first component to render such
 * a pointer fails the build with a broken link on a page that did not cause it.
 * Removing the route means the UI simply omits that link, while the warning
 * still names the entry so the gap stays visible.
 */
function pruneDocPaths(byVersion: Record<string, EcosystemEntry[]>, docPaths: Map<string, string>): string[] {
  const warnings: string[] = [];
  const seen = new Set<string>();

  for (const entries of Object.values(byVersion)) {
    for (const entry of entries) {
      if (!entry.docs) continue;
      for (const [key, route] of Object.entries(entry.docs)) {
        if (typeof route !== 'string') continue;
        const tail = route.replace(/^\/docs\/[^/]+\//, '').replace(/\/$/, '');
        if (docPaths.has(tail)) continue;

        delete entry.docs[key as keyof typeof entry.docs];
        const message = `  ${entry.id}/${ENTRY_FILENAME} → docs.${key}: "${route}" does not match any docs page, so it is not linked`;
        if (seen.has(message)) continue;
        seen.add(message);
        warnings.push(message);
      }
    }
  }
  return warnings;
}

/**
 * Loads the SDKs & tools registry from `entry.yaml` files and exposes it as
 * plugin global data, keyed by docs version.
 *
 * The listing page, the detail pages, and the API reference pages all read from
 * this one source, so adding an SDK, plugin, or integration is a matter of
 * adding a directory with an `entry.yaml` in it. Parsing and validation happen
 * here, at build time, which means a malformed entry fails the build rather
 * than blanking a card in production.
 *
 * Read it with `usePluginData('ecosystem-plugin')`, or through the
 * `useEcosystem()` hook which picks the right version for the current page.
 */
export default function ecosystemPlugin(context: LoadContext): Plugin {
  const {siteDir, siteConfig} = context;
  const productName = (siteConfig.customFields?.product as {project: {name: string}} | undefined)?.project.name ?? '';
  const productSlug = productName.toLowerCase();
  let loaded: EcosystemGlobalData | undefined;

  return {
    name: 'ecosystem-plugin',

    getPathsToWatch(): string[] {
      return findSources(siteDir).map((source) => path.join(source.dir, '*', ENTRY_FILENAME));
    },

    loadContent(): EcosystemGlobalData {
      const availableIcons = new Set<string>(ECOSYSTEM_ICON_NAMES);
      const byVersion: Record<string, EcosystemEntry[]> = {};
      const problems: string[] = [];
      const warnings: string[] = [];

      for (const source of findSources(siteDir)) {
        const result = readEntries(source, productName, productSlug);
        problems.push(...result.problems);
        warnings.push(...result.warnings);
        problems.push(...validateRegistry(result.entries, availableIcons, path.relative(siteDir, source.dir)));
        byVersion[source.version] = result.entries;
      }

      if (problems.length > 0) {
        throw new Error(`Invalid SDKs & tools registry (${ENTRY_FILENAME}):\n${problems.join('\n')}`);
      }

      if (warnings.length > 0) {
        // `@docusaurus/logger` is not a direct dependency of the site, so this
        // build-time notice goes through the console the build already writes to.
        // eslint-disable-next-line no-console
        console.warn(`[WARNING] SDKs & tools registry (${ENTRY_FILENAME}):\n${warnings.join('\n')}`);
      }

      return {byVersion};
    },

    // `Plugin` is generic over its content type, but the `plugins` array in
    // docusaurus.config.ts is typed against `Plugin<unknown>`, so the loaded
    // content arrives back here untyped and is narrowed on the way out.
    contentLoaded({content, actions}): void {
      loaded = content as EcosystemGlobalData;
      actions.setGlobalData(loaded);
    },

    // Route checking waits until every plugin has loaded, since it needs the
    // docs plugin's own list of built pages.
    async allContentLoaded({allContent, actions}): Promise<void> {
      if (!loaded) return;
      fillReadingTimes(loaded.byVersion);
      const warnings = pruneDocPaths(loaded.byVersion, collectDocPaths(allContent));
      actions.setGlobalData(loaded);

      // Routes are created here, not in contentLoaded, so each page is
      // serialized from the entry *after* reading times are filled in and
      // unresolvable `docs:` routes are dropped.
      for (const entry of loaded.byVersion[CURRENT_VERSION] ?? []) {
        if (!entry.sections?.length && !entry.guides?.length) continue;
        const data = await actions.createData(`ecosystem-${entry.id}.json`, JSON.stringify(entry));
        actions.addRoute({
          path: `/sdks/${entry.id}`,
          component: '@site/src/components/Ecosystem/Detail/DetailPage',
          modules: {entry: data},
          exact: true,
        });

        // Each guide is its own page in the same shell, so a reader moving from
        // the entry into a guide stays in one layout.
        for (const guide of entry.guides ?? []) {
          actions.addRoute({
            path: `/sdks/${entry.id}/guides/${guide.id}`,
            component: '@site/src/components/Ecosystem/Detail/GuidePage',
            modules: {entry: data},
            exact: true,
            customData: {guideId: guide.id},
          });
        }
      }

      if (warnings.length > 0) {
        // eslint-disable-next-line no-console
        console.warn(`[WARNING] SDKs & tools registry (${ENTRY_FILENAME}):\n${warnings.join('\n')}`);
      }
    },
  };
}
