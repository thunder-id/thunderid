// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ValueRow} from '../components/ValuesTable';

/** One resource's values: the resource is known by its name, or its id when it has none. */
export interface ResourceGroup {
  key: string;
  label: string | null;
  rows: ValueRow[];
}

/** The resources of one type, or the values nothing refers to under a null type. */
export interface TypeGroup {
  resourceType: string | null;
  resources: ResourceGroup[];
}

// Words kept in capitals when a key is spelled out.
const ACRONYMS: Record<string, string> = {
  id: 'ID',
  ids: 'IDs',
  url: 'URL',
  urls: 'URLs',
  uri: 'URI',
  uris: 'URIs',
  api: 'API',
};

/** Spells a key out as words: `redirectUris` and `redirect_uris` both read "Redirect URIs". */
export function humanize(key: string): string {
  const words = key
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((word: string) => ACRONYMS[word.toLowerCase()] ?? word.toLowerCase());
  if (words.length === 0) return key;
  const [first, ...rest] = words;
  return [first.charAt(0).toUpperCase() + first.slice(1), ...rest].join(' ');
}

const byLabel = (a: string, b: string): number => a.localeCompare(b, undefined, {sensitivity: 'base'});

/**
 * Groups values by the type of resource that refers to them and then by the resource. A value referred
 * to from more than one resource goes with the first. Values nothing refers to come last, under a null
 * type.
 */
export default function groupByResource(rows: ValueRow[]): TypeGroup[] {
  const types = new Map<string | null, Map<string, ResourceGroup>>();
  [...rows]
    .sort((a: ValueRow, b: ValueRow) => a.name.localeCompare(b.name))
    .forEach((row: ValueRow) => {
      const reference = row.references[0];
      const resourceType = reference?.resourceType ?? null;
      const key = `${resourceType ?? ''}/${reference ? (reference.resourceId ?? reference.resourceName ?? '') : ''}`;
      const resources = types.get(resourceType) ?? new Map<string, ResourceGroup>();
      const resource = resources.get(key) ?? {
        key,
        label: reference ? (reference.resourceName ?? reference.resourceId ?? null) : null,
        rows: [],
      };
      resource.rows.push(row);
      resources.set(key, resource);
      types.set(resourceType, resources);
    });

  return [...types.entries()]
    .sort(([a], [b]) => {
      if (a === null) return 1;
      if (b === null) return -1;
      return byLabel(a, b);
    })
    .map(([resourceType, resources]) => ({
      resourceType,
      resources: [...resources.values()].sort((a, b) => byLabel(a.label ?? '', b.label ?? '')),
    }));
}
