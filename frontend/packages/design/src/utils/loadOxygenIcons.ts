// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {IconNode} from '@wso2/oxygen-ui-icons-react';

/**
 * An icon as stored in oxygenIcons.json. See scripts/generate-oxygen-icons.mjs, whose derivations
 * the ones here must match.
 */
interface StoredIcon {
  n: [string, Record<string, string>][];
  c?: string;
  dc?: string;
  vb?: string;
}

interface StoredIcons {
  icons: Record<string, StoredIcon>;
  aliases: Record<string, string>;
}

/**
 * An icon of `@wso2/oxygen-ui-icons-react`, as {@link OxygenIcon} draws it.
 */
export interface OxygenIconDefinition {
  /** The name the icon is listed under. */
  name: string;
  /** The elements the icon draws. */
  iconNode: IconNode;
  /** The classes of the icon itself. */
  className: string;
  /** A class the icon carries unless another one is passed. */
  defaultClassName?: string;
  /** The viewBox of the icon, when it is not 0 0 24 24. */
  viewBox?: string;
}

/**
 * The icons of `@wso2/oxygen-ui-icons-react`, as {@link loadOxygenIcons} loads them.
 */
export interface OxygenIconSet {
  /** The names of the icons, one per icon, sorted. */
  names: readonly string[];
  /**
   * Resolves any name the icon package exports an icon under, such as `UserIcon` or `LucideUser`,
   * to the name that icon is listed under.
   *
   * @param name - An export name of the icon package.
   * @returns The listed name, or undefined when the package exports no icon of that name.
   */
  resolveName(name: string): string | undefined;
  /**
   * Gets an icon by any name the icon package exports it under.
   *
   * @param name - An export name of the icon package.
   * @returns The icon, or undefined when the package exports no icon of that name.
   */
  get(name: string): OxygenIconDefinition | undefined;
}

function stripAffixes(name: string): string {
  return name.replace(/^Lucide/, '').replace(/Icon$/, '');
}

/** The classes lucide-react gives an icon of this name, unless the icon is stored with its own. */
function deriveClassName(name: string): string {
  const kebabCase = name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
  const fileName = name
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1-$2')
    .replace(/([a-z])(\d)/g, '$1-$2')
    .toLowerCase();
  return [...new Set([`lucide-${kebabCase}`, `lucide-${fileName}`])].join(' ');
}

function createOxygenIconSet({icons, aliases}: StoredIcons): OxygenIconSet {
  const definitions = new Map<string, OxygenIconDefinition>();

  const resolveName = (name: string): string | undefined => {
    if (Object.hasOwn(icons, name)) return name;
    if (Object.hasOwn(aliases, name)) return aliases[name];
    const stripped = stripAffixes(name);
    if (stripped === name) return undefined;
    if (Object.hasOwn(icons, stripped)) return stripped;
    return Object.hasOwn(aliases, stripped) ? aliases[stripped] : undefined;
  };

  const get = (name: string): OxygenIconDefinition | undefined => {
    const listedName = resolveName(name);
    if (!listedName) return undefined;

    let definition = definitions.get(listedName);
    if (!definition) {
      const stored = icons[listedName];
      definition = {
        name: listedName,
        // React needs a key on each element; lucide-react's own icons carry one in their data.
        iconNode: stored.n.map(([tag, attributes], index) => [
          tag,
          {...attributes, key: `${listedName}-${index}`},
        ]) as IconNode,
        className: stored.c ?? deriveClassName(listedName),
        ...(stored.dc ? {defaultClassName: stored.dc} : {}),
        ...(stored.vb ? {viewBox: stored.vb} : {}),
      };
      definitions.set(listedName, definition);
    }
    return definition;
  };

  return {names: Object.keys(icons), resolveName, get};
}

let loadedIcons: OxygenIconSet | undefined;
let pendingIcons: Promise<OxygenIconSet> | undefined;
const listeners = new Set<() => void>();

/**
 * Subscribes to the icon set being loaded, by any caller of {@link loadOxygenIcons}: a component
 * whose own load failed still gets the set when a later load succeeds.
 *
 * @param listener - Called once the icon set has loaded.
 * @returns A function that ends the subscription.
 */
export function subscribeToOxygenIcons(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Returns the icon set if {@link loadOxygenIcons} has already loaded it, without loading it.
 *
 * @returns The icon set, or undefined when it has not been loaded yet.
 */
export function getLoadedOxygenIcons(): OxygenIconSet | undefined {
  return loadedIcons;
}

/**
 * Loads the icons of `@wso2/oxygen-ui-icons-react`, for code that picks an icon by a name it only
 * knows at runtime. Draw them with {@link OxygenIcon}.
 *
 * The icons are loaded as data, the shapes they draw, from oxygenIcons.json, rather than as the
 * components of the icon package: that is a fraction of the size, and it keeps the package's full
 * set of components out of the console's bundles. The data is loaded on demand, once.
 *
 * Code that renders icons it names in its source should import them from the icon package instead.
 *
 * @returns The icon set.
 */
export default function loadOxygenIcons(): Promise<OxygenIconSet> {
  pendingIcons ??= import('./oxygenIconData')
    .then((module) => {
      // The generator writes this shape; the type TypeScript infers for the file is the literal data.
      loadedIcons = createOxygenIconSet(module.default as unknown as StoredIcons);
      listeners.forEach((listener) => listener());
      return loadedIcons;
    })
    .catch((error: unknown) => {
      // Let a later call retry, after a failed chunk download for instance. Catching after the
      // .then covers a failure in reading the data too, not only in importing it.
      pendingIcons = undefined;
      throw error;
    });
  return pendingIcons;
}
