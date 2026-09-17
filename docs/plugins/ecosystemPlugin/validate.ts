// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {
  EcosystemCategory,
  EcosystemEntry,
  EcosystemOrigin,
  EcosystemPackageManager,
  EcosystemSection,
  EcosystemStatus,
  EcosystemType,
  EcosystemTone,
} from '../../src/types/ecosystem';

const CATEGORIES: EcosystemCategory[] = ['agent', 'spa', 'fullstack', 'backend', 'mobile'];

const TYPES: EcosystemType[] = ['sdk', 'plugin', 'integration', 'guide'];

const ORIGINS: EcosystemOrigin[] = ['official', 'community'];

const STATUSES: EcosystemStatus[] = ['ga', 'beta', 'soon'];

const MANAGERS: EcosystemPackageManager[] = ['npm', 'go', 'pip', 'gradle', 'maven', 'pod', 'spm', 'pub', 'plugin'];

const TONES: EcosystemTone[] = ['success', 'danger', 'warning', 'accent', 'primary', 'purple', 'neutral'];

const SECTION_TYPES = ['steps', 'table', 'commands', 'linkGrid', 'guides', 'quickstart', 'api', 'support'];

const META_ICONS = ['package', 'licence', 'clock', 'download', 'people'];

const GUIDE_ICONS = ['shield', 'terminal', 'server', 'pen', 'building', 'exchange', 'key', 'book'];

const API_SOURCES = ['typedoc', 'symbolgraph', 'manual'];

/**
 * Collects problems rather than throwing on the first one, so a single build
 * reports every broken entry at once instead of one per run.
 */
export class Problems {
  private readonly messages: string[] = [];

  constructor(private readonly scope: string) {}

  add(path: string, message: string): void {
    this.messages.push(`  ${this.scope}${path ? ` → ${path}` : ''}: ${message}`);
  }

  get list(): string[] {
    return this.messages;
  }
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requireString(p: Problems, obj: Record<string, unknown>, path: string, key: string): string | undefined {
  const value = obj[key];
  if (typeof value !== 'string' || value.trim() === '') {
    p.add(`${path}${key}`, 'is required and must be a non-empty string');
    return undefined;
  }
  return value;
}

function optionalString(p: Problems, obj: Record<string, unknown>, path: string, key: string): void {
  const value = obj[key];
  if (value !== undefined && typeof value !== 'string') {
    p.add(`${path}${key}`, 'must be a string when present');
  }
}

function requireEnum<T extends string>(
  p: Problems,
  obj: Record<string, unknown>,
  path: string,
  key: string,
  allowed: readonly T[],
): T | undefined {
  const value = obj[key];
  if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
    p.add(`${path}${key}`, `must be one of: ${allowed.join(', ')}`);
    return undefined;
  }
  return value as T;
}

function optionalEnum(
  p: Problems,
  obj: Record<string, unknown>,
  path: string,
  key: string,
  allowed: readonly string[],
): void {
  const value = obj[key];
  if (value !== undefined && (typeof value !== 'string' || !allowed.includes(value))) {
    p.add(`${path}${key}`, `must be one of: ${allowed.join(', ')}`);
  }
}

function validateCode(p: Problems, value: unknown, path: string): void {
  if (!isPlainObject(value)) {
    p.add(path, 'must be an object with a `content` field');
    return;
  }
  requireString(p, value, `${path}.`, 'content');
  optionalString(p, value, `${path}.`, 'file');
  optionalString(p, value, `${path}.`, 'lang');
}

function validateCtas(p: Problems, value: unknown, path: string): void {
  if (value === undefined) return;
  if (!Array.isArray(value)) {
    p.add(path, 'must be an array');
    return;
  }
  value.forEach((cta, i) => {
    if (!isPlainObject(cta)) {
      p.add(`${path}[${i}]`, 'must be an object');
      return;
    }
    requireString(p, cta, `${path}[${i}].`, 'label');
    requireString(p, cta, `${path}[${i}].`, 'href');
    optionalEnum(p, cta, `${path}[${i}].`, 'variant', ['primary', 'secondary']);
  });
}

function validateSteps(p: Problems, value: unknown, path: string): void {
  if (!Array.isArray(value) || value.length === 0) {
    p.add(path, 'must be a non-empty array');
    return;
  }
  value.forEach((step, i) => {
    const at = `${path}[${i}]`;
    if (!isPlainObject(step)) {
      p.add(at, 'must be an object');
      return;
    }
    requireString(p, step, `${at}.`, 'title');
    optionalString(p, step, `${at}.`, 'description');
    optionalString(p, step, `${at}.`, 'note');
    if (step.code !== undefined) validateCode(p, step.code, `${at}.code`);
    if (step.uiPath !== undefined) {
      if (!Array.isArray(step.uiPath) || step.uiPath.some((s) => typeof s !== 'string')) {
        p.add(`${at}.uiPath`, 'must be an array of strings');
      }
    }
  });
}

function validateSection(p: Problems, section: unknown, path: string): void {
  if (!isPlainObject(section)) {
    p.add(path, 'must be an object');
    return;
  }

  const type = requireEnum(p, section, `${path}.`, 'type', SECTION_TYPES);
  requireString(p, section, `${path}.`, 'id');
  requireString(p, section, `${path}.`, 'title');
  optionalString(p, section, `${path}.`, 'intro');
  optionalString(p, section, `${path}.`, 'meta');

  switch (type) {
    case 'api': {
      if (section.kindTones !== undefined && !isPlainObject(section.kindTones)) {
        p.add(`${path}.kindTones`, 'must be an object mapping kind to tone');
      }
      if (!Array.isArray(section.exports) || section.exports.length === 0) {
        p.add(`${path}.exports`, 'must be a non-empty array');
        return;
      }
      section.exports.forEach((item, i) => {
        const at = `${path}.exports[${i}]`;
        if (!isPlainObject(item)) {
          p.add(at, 'must be an object');
          return;
        }
        for (const key of ['group', 'kind', 'name', 'description']) {
          requireString(p, item, `${at}.`, key);
        }
        optionalString(p, item, `${at}.`, 'signature');
        optionalString(p, item, `${at}.`, 'rowsLabel');
        optionalString(p, item, `${at}.`, 'example');
        // An export earns its place in the explorer by showing something
        // concrete: a declaration, its props, or how it is used.
        if (item.signature === undefined && !Array.isArray(item.rows) && item.example === undefined) {
          p.add(at, 'must set at least one of `signature`, `rows`, or `example`');
        }
        if (item.rows !== undefined) {
          if (!Array.isArray(item.rows)) {
            p.add(`${at}.rows`, 'must be an array');
          } else {
            item.rows.forEach((row, k) => {
              if (!isPlainObject(row)) {
                p.add(`${at}.rows[${k}]`, 'must be an object');
                return;
              }
              requireString(p, row, `${at}.rows[${k}].`, 'name');
              requireString(p, row, `${at}.rows[${k}].`, 'type');
              optionalString(p, row, `${at}.rows[${k}].`, 'description');
              optionalString(p, row, `${at}.rows[${k}].`, 'tag');
            });
          }
        }
        const tones = isPlainObject(section.kindTones) ? section.kindTones : {};
        if (typeof item.kind === 'string' && !(item.kind in tones)) {
          p.add(`${at}.kind`, `"${item.kind}" has no entry in ${path}.kindTones`);
        }
      });
      return;
    }

    case 'steps': {
      const hasSteps = section.steps !== undefined;
      const hasTabs = section.tabs !== undefined;
      if (hasSteps === hasTabs) {
        p.add(path, 'must set exactly one of `steps` or `tabs`');
        return;
      }
      if (hasSteps) {
        validateSteps(p, section.steps, `${path}.steps`);
        return;
      }
      if (!Array.isArray(section.tabs) || section.tabs.length === 0) {
        p.add(`${path}.tabs`, 'must be a non-empty array');
        return;
      }
      section.tabs.forEach((tab, i) => {
        const at = `${path}.tabs[${i}]`;
        if (!isPlainObject(tab)) {
          p.add(at, 'must be an object');
          return;
        }
        requireString(p, tab, `${at}.`, 'label');
        optionalString(p, tab, `${at}.`, 'blurb');
        validateSteps(p, tab.steps, `${at}.steps`);
      });
      return;
    }

    case 'table': {
      if (!Array.isArray(section.columns) || section.columns.length === 0) {
        p.add(`${path}.columns`, 'must be a non-empty array');
        return;
      }
      const width = section.columns.length;
      section.columns.forEach((col, i) => {
        if (!isPlainObject(col)) {
          p.add(`${path}.columns[${i}]`, 'must be an object');
          return;
        }
        optionalString(p, col, `${path}.columns[${i}].`, 'label');
        optionalEnum(p, col, `${path}.columns[${i}].`, 'style', ['text', 'code', 'accent', 'muted']);
      });
      if (!Array.isArray(section.rows) || section.rows.length === 0) {
        p.add(`${path}.rows`, 'must be a non-empty array');
        return;
      }
      section.rows.forEach((row, i) => {
        const at = `${path}.rows[${i}]`;
        if (!isPlainObject(row)) {
          p.add(at, 'must be an object');
          return;
        }
        if (!Array.isArray(row.cells) || row.cells.some((c) => typeof c !== 'string')) {
          p.add(`${at}.cells`, 'must be an array of strings');
          return;
        }
        if (row.cells.length !== width) {
          p.add(`${at}.cells`, `has ${row.cells.length} cells but the table declares ${width} columns`);
        }
        if (row.badge !== undefined) {
          if (!isPlainObject(row.badge)) {
            p.add(`${at}.badge`, 'must be an object');
          } else {
            requireString(p, row.badge, `${at}.badge.`, 'label');
            optionalEnum(p, row.badge, `${at}.badge.`, 'tone', TONES);
          }
        }
      });
      return;
    }

    case 'commands': {
      if (section.groupTones !== undefined) {
        if (!isPlainObject(section.groupTones)) {
          p.add(`${path}.groupTones`, 'must be an object mapping group name to tone');
        } else {
          for (const [group, tone] of Object.entries(section.groupTones)) {
            if (typeof tone !== 'string' || !(TONES as string[]).includes(tone)) {
              p.add(`${path}.groupTones.${group}`, `must be one of: ${TONES.join(', ')}`);
            }
          }
        }
      }
      if (!Array.isArray(section.items) || section.items.length === 0) {
        p.add(`${path}.items`, 'must be a non-empty array');
        return;
      }
      section.items.forEach((item, i) => {
        const at = `${path}.items[${i}]`;
        if (!isPlainObject(item)) {
          p.add(at, 'must be an object');
          return;
        }
        requireString(p, item, `${at}.`, 'command');
        requireString(p, item, `${at}.`, 'description');
        optionalString(p, item, `${at}.`, 'group');
        const groupTones = isPlainObject(section.groupTones) ? section.groupTones : {};
        if (typeof item.group === 'string' && !(item.group in groupTones)) {
          p.add(`${at}.group`, `"${item.group}" has no entry in ${path}.groupTones`);
        }
      });
      return;
    }

    case 'guides': {
      if (section.items === undefined) return;
      if (!Array.isArray(section.items) || section.items.some((i) => typeof i !== 'string')) {
        p.add(`${path}.items`, 'must be an array of guide ids');
      }
      return;
    }

    case 'linkGrid': {
      if (!Array.isArray(section.items) || section.items.length === 0) {
        p.add(`${path}.items`, 'must be a non-empty array');
        return;
      }
      section.items.forEach((item, i) => {
        const at = `${path}.items[${i}]`;
        if (!isPlainObject(item)) {
          p.add(at, 'must be an object');
          return;
        }
        requireString(p, item, `${at}.`, 'label');
        requireString(p, item, `${at}.`, 'href');
      });
      return;
    }

    case 'quickstart': {
      requireString(p, section, `${path}.`, 'label');
      requireString(p, section, `${path}.`, 'path');
      optionalString(p, section, `${path}.`, 'outro');
      if (section.minutes !== undefined && typeof section.minutes !== 'number') {
        p.add(`${path}.minutes`, 'must be a number when present');
      }
      return;
    }

    case 'support': {
      const variant = requireEnum(p, section, `${path}.`, 'variant', ['official', 'split']);
      optionalString(p, section, `${path}.`, 'eyebrow');
      optionalString(p, section, `${path}.`, 'body');
      validateCtas(p, section.ctas, `${path}.ctas`);
      if (variant === 'split') {
        if (!Array.isArray(section.panels) || section.panels.length !== 2) {
          p.add(`${path}.panels`, 'must be an array of exactly two panels for the `split` variant');
          return;
        }
        section.panels.forEach((panel, i) => {
          const at = `${path}.panels[${i}]`;
          if (!isPlainObject(panel)) {
            p.add(at, 'must be an object');
            return;
          }
          requireString(p, panel, `${at}.`, 'title');
          requireString(p, panel, `${at}.`, 'body');
          validateCtas(p, panel.ctas, `${at}.ctas`);
        });
      }
    }
  }
}

function validateHero(p: Problems, hero: unknown): void {
  if (!isPlainObject(hero)) {
    p.add('hero', 'must be an object');
    return;
  }
  optionalString(p, hero, 'hero.', 'description');

  if (hero.badges !== undefined) {
    if (!Array.isArray(hero.badges)) {
      p.add('hero.badges', 'must be an array');
    } else {
      hero.badges.forEach((badge, i) => {
        if (!isPlainObject(badge)) {
          p.add(`hero.badges[${i}]`, 'must be an object');
          return;
        }
        requireString(p, badge, `hero.badges[${i}].`, 'label');
        optionalEnum(p, badge, `hero.badges[${i}].`, 'tone', TONES);
      });
    }
  }

  if (hero.meta !== undefined) {
    if (!Array.isArray(hero.meta)) {
      p.add('hero.meta', 'must be an array');
    } else {
      hero.meta.forEach((item, i) => {
        if (!isPlainObject(item)) {
          p.add(`hero.meta[${i}]`, 'must be an object');
          return;
        }
        requireEnum(p, item, `hero.meta[${i}].`, 'icon', META_ICONS);
        requireString(p, item, `hero.meta[${i}].`, 'label');
      });
    }
  }

  validateCtas(p, hero.ctas, 'hero.ctas');

  if (hero.aside !== undefined) {
    const aside = hero.aside;
    if (!isPlainObject(aside)) {
      p.add('hero.aside', 'must be an object');
      return;
    }
    const asideType = requireEnum(p, aside, 'hero.aside.', 'type', ['install', 'snippet']);
    optionalString(p, aside, 'hero.aside.', 'title');
    optionalString(p, aside, 'hero.aside.', 'footnote');
    if (asideType === 'install') {
      if (!Array.isArray(aside.tabs) || aside.tabs.length === 0) {
        p.add('hero.aside.tabs', 'must be a non-empty array for the `install` type');
      } else {
        aside.tabs.forEach((tab, i) => {
          if (!isPlainObject(tab)) {
            p.add(`hero.aside.tabs[${i}]`, 'must be an object');
            return;
          }
          requireString(p, tab, `hero.aside.tabs[${i}].`, 'label');
          validateCode(p, tab.code, `hero.aside.tabs[${i}].code`);
        });
      }
    } else if (asideType === 'snippet') {
      validateCode(p, aside.code, 'hero.aside.code');
    }
  }
}

function validateRail(p: Problems, rail: unknown): void {
  if (!isPlainObject(rail)) {
    p.add('rail', 'must be an object');
    return;
  }
  optionalString(p, rail, 'rail.', 'statsTitle');
  optionalString(p, rail, 'rail.', 'relatedTitle');

  if (rail.stats !== undefined) {
    if (!Array.isArray(rail.stats)) {
      p.add('rail.stats', 'must be an array');
    } else {
      rail.stats.forEach((stat, i) => {
        if (!isPlainObject(stat)) {
          p.add(`rail.stats[${i}]`, 'must be an object');
          return;
        }
        requireString(p, stat, `rail.stats[${i}].`, 'label');
        requireString(p, stat, `rail.stats[${i}].`, 'value');
      });
    }
  }

  if (rail.related !== undefined) {
    if (!Array.isArray(rail.related) || rail.related.some((r) => typeof r !== 'string')) {
      p.add('rail.related', 'must be an array of entry ids');
    }
  }
}

/**
 * Validates one parsed `sdk.yaml`. Returns the entry when it is structurally
 * sound so the caller can keep going, or undefined when it is not usable at
 * all. Cross-entry checks (duplicate ids, `rail.related` targets) happen in
 * `validateRegistry` once every file has been read.
 */
export function validateEntry(
  raw: unknown,
  sourcePath: string,
): {entry?: EcosystemEntry; problems: string[]; warnings: string[]} {
  const p = new Problems(sourcePath);
  const w = new Problems(sourcePath);

  if (!isPlainObject(raw)) {
    p.add('', 'must be a YAML mapping');
    return {problems: p.list, warnings: w.list};
  }

  const id = requireString(p, raw, '', 'id');
  requireString(p, raw, '', 'name');
  requireString(p, raw, '', 'icon');
  requireString(p, raw, '', 'description');
  requireEnum(p, raw, '', 'category', CATEGORIES);
  requireEnum(p, raw, '', 'type', TYPES);
  requireEnum(p, raw, '', 'origin', ORIGINS);
  requireEnum(p, raw, '', 'status', STATUSES);
  optionalString(p, raw, '', 'accent');
  optionalString(p, raw, '', 'author');
  optionalString(p, raw, '', 'version');

  if (!isPlainObject(raw.package)) {
    p.add('package', 'is required and must be an object');
  } else {
    requireString(p, raw.package, 'package.', 'name');
    optionalEnum(p, raw.package, 'package.', 'manager', MANAGERS);
    optionalString(p, raw.package, 'package.', 'url');
  }

  const docs = isPlainObject(raw.docs) ? raw.docs : undefined;
  if (raw.docs !== undefined) {
    if (!docs) {
      p.add('docs', 'must be an object');
    } else {
      for (const key of ['overview', 'apiReference', 'quickstart']) {
        optionalString(p, docs, 'docs.', key);
        const value = docs[key];
        if (typeof value === 'string' && !value.startsWith('/')) {
          p.add(`docs.${key}`, 'must be a site-absolute route starting with "/"');
        }
      }
    }
  }

  // An unreleased entry has nowhere to send a reader, so claiming a page is an
  // error. The reverse is only a warning: a package can ship before its docs
  // page lands, and the card simply renders without a link until it does.
  if (raw.status === 'soon') {
    if (docs?.overview !== undefined) {
      p.add('docs.overview', 'must be omitted while status is "soon"');
    }
  } else if (docs?.overview === undefined) {
    w.add('docs.overview', 'is not set, so this entry has no detail page to link to');
  }

  if (raw.links !== undefined) {
    if (!isPlainObject(raw.links)) {
      p.add('links', 'must be an object');
    } else {
      for (const key of ['github', 'package', 'changelog', 'samples', 'external']) {
        optionalString(p, raw.links, 'links.', key);
      }
    }
  }

  if (raw.hero !== undefined) validateHero(p, raw.hero);
  if (raw.rail !== undefined) validateRail(p, raw.rail);

  if (raw.apiReference !== undefined) {
    if (!isPlainObject(raw.apiReference)) {
      p.add('apiReference', 'must be an object');
    } else {
      requireEnum(p, raw.apiReference, 'apiReference.', 'source', API_SOURCES);
      optionalString(p, raw.apiReference, 'apiReference.', 'file');
    }
  }

  if (raw.guides !== undefined) {
    if (!Array.isArray(raw.guides)) {
      p.add('guides', 'must be an array');
    } else {
      const seenGuides = new Set<string>();
      raw.guides.forEach((guide, i) => {
        const at = `guides[${i}]`;
        if (!isPlainObject(guide)) {
          p.add(at, 'must be an object');
          return;
        }
        const guideId = requireString(p, guide, `${at}.`, 'id');
        requireString(p, guide, `${at}.`, 'title');
        optionalString(p, guide, `${at}.`, 'description');
        optionalString(p, guide, `${at}.`, 'intro');
        optionalEnum(p, guide, `${at}.`, 'icon', GUIDE_ICONS);
        if (guideId !== undefined) {
          if (seenGuides.has(guideId)) p.add(`${at}.id`, `"${guideId}" is used by more than one guide`);
          seenGuides.add(guideId);
        }
        if (!Array.isArray(guide.sections) || guide.sections.length === 0) {
          p.add(`${at}.sections`, 'must be a non-empty array');
          return;
        }
        guide.sections.forEach((s2, j) => validateSection(p, s2, `${at}.sections[${j}]`));
      });

      // A guides card section may only name guides that exist.
      const guideIds = new Set(
        raw.guides.filter(isPlainObject).map((g) => g.id).filter((id): id is string => typeof id === 'string'),
      );
      for (const section of Array.isArray(raw.sections) ? raw.sections : []) {
        if (!isPlainObject(section) || section.type !== 'guides' || !Array.isArray(section.items)) continue;
        for (const id of section.items) {
          if (typeof id === 'string' && !guideIds.has(id)) {
            p.add('sections', `guides card "${id}" does not match any entry in \`guides\``);
          }
        }
      }
    }
  }

  if (raw.sections !== undefined) {
    if (!Array.isArray(raw.sections)) {
      p.add('sections', 'must be an array');
    } else {
      const seen = new Set<string>();
      raw.sections.forEach((section, i) => {
        validateSection(p, section, `sections[${i}]`);
        if (!isPlainObject(section)) return;
        if (typeof section.id === 'string') {
          if (seen.has(section.id)) {
            p.add(`sections[${i}].id`, `"${section.id}" is used by more than one section`);
          }
          seen.add(section.id);
        }
        if (section.type === 'quickstart' && docs?.quickstart === undefined) {
          p.add(`sections[${i}]`, 'a `quickstart` section requires `docs.quickstart`');
        }
      });
    }
  }

  // An entry missing its id is unusable downstream; anything else can still be
  // surfaced so one bad field does not hide the rest of the report.
  if (id === undefined) return {problems: p.list, warnings: w.list};

  return {entry: raw as unknown as EcosystemEntry, problems: p.list, warnings: w.list};
}

/**
 * Cross-entry checks that can only run once the whole version has been read:
 * duplicate ids, `rail.related` pointing at entries that do not exist, and
 * icons that are not declared in `src/components/Ecosystem/iconNames.ts`.
 */
export function validateRegistry(
  entries: EcosystemEntry[],
  availableIcons: ReadonlySet<string>,
  scope: string,
): string[] {
  const p = new Problems(scope);
  const ids = new Set<string>();

  for (const entry of entries) {
    if (ids.has(entry.id)) {
      p.add(entry.id, 'is declared by more than one sdk.yaml');
    }
    ids.add(entry.id);
  }

  for (const entry of entries) {
    if (entry.icon && !availableIcons.has(entry.icon)) {
      p.add(`${entry.id}.icon`, `"${entry.icon}" is not listed in src/components/Ecosystem/iconNames.ts`);
    }
    for (const related of entry.rail?.related ?? []) {
      if (!ids.has(related)) {
        p.add(`${entry.id}.rail.related`, `"${related}" is not a known entry id`);
      }
      if (related === entry.id) {
        p.add(`${entry.id}.rail.related`, 'an entry cannot be related to itself');
      }
    }
  }

  return p.list;
}

export type {EcosystemEntry, EcosystemSection};
