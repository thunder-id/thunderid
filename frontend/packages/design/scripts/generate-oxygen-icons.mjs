#!/usr/bin/env node

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @thunderid/copyright-header */

/**
 * Regenerates src/utils/oxygenIcons.json, the shapes of every icon in @wso2/oxygen-ui-icons-react,
 * which the console loads to draw icons it picks by a name only known at runtime (see
 * loadOxygenIcons). Run it after upgrading @wso2/oxygen-ui-icons-react or its lucide-react:
 *
 *   pnpm codegen:icons
 *
 * Each icon component the package exports is rendered once, and the elements it draws are recorded,
 * so the data matches what the components render, custom icons included. The OxygenIcon parity test
 * fails when the file is out of date.
 *
 * The generated JSON has two members:
 *   icons:   { [name]: { n: [[tag, attributes], ...], c?: class, dc?: default class, vb?: viewBox } }
 *            One entry per icon, under the name the picker lists. `c` is only present when the
 *            icon's own classes are not the ones loadOxygenIcons derives from its name, `dc` when
 *            the component adds a class of its own unless one is passed, and `vb` when its viewBox
 *            is not 0 0 24 24.
 *   aliases: { [name]: name }
 *            The other export names of each icon, except those loadOxygenIcons finds by dropping a
 *            `Lucide` prefix or an `Icon` suffix.
 *
 * The derivations here and in loadOxygenIcons must match.
 */

import {readFileSync, writeFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {dirname, resolve} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {createLogger} from '@thunderid/logger';
import {format, resolveConfig} from 'prettier';
import {createElement} from 'react';
import {renderToStaticMarkup} from 'react-dom/server';

const logger = createLogger({level: 'info'});

const __dirname = dirname(fileURLToPath(import.meta.url));
const OUTPUT_PATH = resolve(__dirname, '../src/utils/oxygenIcons.json');
const DEFAULT_VIEW_BOX = '0 0 24 24';
const PROBE_CLASS = 'oxygen-icons-probe';

/** Resolves the entry of the icon package, and lucide-react as the icon package resolves it. */
function resolvePackages() {
  const require = createRequire(import.meta.url);
  // require.resolve gives the CommonJS entry; the ES module entry next to it lists the custom icons
  // as imports.
  const iconsEntry = resolve(dirname(require.resolve('@wso2/oxygen-ui-icons-react')), 'index.js');
  const lucideEntry = createRequire(iconsEntry).resolve('lucide-react');
  return {iconsEntry, lucideEntry};
}

/** The icon components the package adds to lucide-react, keyed by export name. */
async function loadCustomIcons(iconsEntry) {
  const source = readFileSync(iconsEntry, 'utf8');
  const icons = {};
  for (const [, names, file] of source.matchAll(/import \{([^}]+)\} from "(\.\/icons\/[^"]+\.js)";/g)) {
    const module = await import(pathToFileURL(resolve(dirname(iconsEntry), file)).href);
    for (const name of names.split(',').map((part) => part.trim())) {
      icons[name] = module[name];
    }
  }
  return icons;
}

function isIconComponent(value) {
  return typeof value === 'object' && value !== null && '$$typeof' in value && typeof value.displayName === 'string';
}

/** lucide-react's toKebabCase. */
function toKebabCase(value) {
  return value.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

/** The file name lucide-react gives an icon of this name, as far as it can be told from the name. */
function toFileName(value) {
  return value
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1-$2')
    .replace(/([a-z])(\d)/g, '$1-$2')
    .toLowerCase();
}

/** The classes lucide-react gives an icon of this name, unless the icon is stored with its own. */
function deriveClassName(name) {
  return [...new Set([`lucide-${toKebabCase(name)}`, `lucide-${toFileName(name)}`])].join(' ');
}

/** Turns an attribute name as React renders it back into the prop name it came from. */
function toPropName(attribute) {
  if (attribute === 'class') return 'className';
  return attribute.replace(/[-:]([a-z])/g, (_match, letter) => letter.toUpperCase());
}

function unescapeHtml(value) {
  return value
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&');
}

function parseAttributes(source) {
  const attributes = {};
  for (const [, name, value] of source.matchAll(/([\w:-]+)="([^"]*)"/g)) {
    attributes[toPropName(name)] = unescapeHtml(value);
  }
  return attributes;
}

/** Renders an icon and returns the attributes of its <svg> and the elements it draws. */
function renderIcon(component, props = {}) {
  const markup = renderToStaticMarkup(createElement(component, props));
  const root = /^<svg([^>]*)>([\s\S]*)<\/svg>$/.exec(markup);
  if (!root) {
    throw new Error(`Unexpected markup for ${component.displayName}: ${markup.slice(0, 80)}`);
  }
  const elements = [...root[2].matchAll(/<(\w+)([^>]*?)\/?>(?:<\/\1>)?/g)].map(([, tag, attributes]) => [
    tag,
    parseAttributes(attributes),
  ]);
  return {attributes: parseAttributes(root[1]), elements};
}

/** Describes one icon component: what it draws and the classes and viewBox of its <svg>. */
function describeIcon(component, name) {
  const plain = renderIcon(component);
  const probed = renderIcon(component, {className: PROBE_CLASS});
  // The <svg> carries `lucide`, then the icon's own classes, then the class passed to it. A class
  // the component adds itself only shows when none is passed.
  const ownClasses = probed.attributes.className.split(' ').filter((c) => c !== 'lucide' && c !== PROBE_CLASS);
  const defaultClasses = plain.attributes.className.split(' ').filter((c) => c !== 'lucide' && !ownClasses.includes(c));

  const icon = {n: plain.elements};
  const className = ownClasses.join(' ');
  if (className !== deriveClassName(name)) icon.c = className;
  if (defaultClasses.length > 0) icon.dc = defaultClasses.join(' ');
  if (plain.attributes.viewBox !== DEFAULT_VIEW_BOX) icon.vb = plain.attributes.viewBox;
  return icon;
}

/** The name an icon is listed under: its own name when it is exported as such. */
function pickName(component, names) {
  if (names.includes(component.displayName)) return component.displayName;
  const plain = names.filter((name) => !name.startsWith('Lucide') && !name.endsWith('Icon'));
  return [...(plain.length > 0 ? plain : names)].sort((a, b) => a.length - b.length || a.localeCompare(b))[0];
}

/** The name loadOxygenIcons tries when a name is neither an icon nor an alias. */
function stripAffixes(name) {
  return name.replace(/^Lucide/, '').replace(/Icon$/, '');
}

async function generate() {
  const {iconsEntry, lucideEntry} = resolvePackages();
  const lucide = await import(pathToFileURL(lucideEntry).href);
  // The package re-exports lucide-react and adds icons of its own, which win over lucide-react's
  // exports of the same name.
  const exportsByName = {...(lucide.default ?? lucide), ...(await loadCustomIcons(iconsEntry))};

  const namesByComponent = new Map();
  for (const [name, value] of Object.entries(exportsByName)) {
    if (!isIconComponent(value)) continue;
    namesByComponent.set(value, [...(namesByComponent.get(value) ?? []), name]);
  }

  const icons = {};
  const listedNameOf = new Map();
  for (const [component, names] of namesByComponent) {
    const name = pickName(component, names);
    icons[name] = describeIcon(component, name);
    listedNameOf.set(component, name);
  }

  // Names without affixes are listed in full. A name with a `Lucide` prefix or an `Icon` suffix is
  // left out when its stripped form already leads to the same icon.
  const allAliases = {};
  for (const [name, value] of Object.entries(exportsByName)) {
    if (isIconComponent(value) && name !== listedNameOf.get(value)) allAliases[name] = listedNameOf.get(value);
  }
  const aliases = {};
  for (const [name, listedName] of Object.entries(allAliases)) {
    const stripped = stripAffixes(name);
    const strippedLeadsTo = stripped in icons ? stripped : allAliases[stripped];
    if (stripped !== name && stripAffixes(stripped) === stripped && strippedLeadsTo === listedName) continue;
    aliases[name] = listedName;
  }

  return {icons: sortKeys(icons), aliases: sortKeys(aliases)};
}

function sortKeys(object) {
  return Object.fromEntries(Object.entries(object).sort(([a], [b]) => a.localeCompare(b)));
}

const data = await generate();
// Formatted as the repository formats JSON, so that the format check passes on it.
const prettierConfig = await resolveConfig(OUTPUT_PATH);
writeFileSync(OUTPUT_PATH, await format(JSON.stringify(data), {...prettierConfig, filepath: OUTPUT_PATH}));
logger.info(
  `Wrote ${Object.keys(data.icons).length} icons and ${Object.keys(data.aliases).length} aliases to ${OUTPUT_PATH}`,
);
