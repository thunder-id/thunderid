// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {
  EcosystemCategory,
  EcosystemEntry,
  EcosystemOrigin,
  EcosystemType,
} from '@site/src/types/ecosystem';

/** Second-level label: the platform an entry targets. */
export const CATEGORY_LABELS: Record<EcosystemCategory, string> = {
  agent: 'Agent tooling',
  spa: 'SPA',
  fullstack: 'Fullstack',
  backend: 'Backend',
  mobile: 'Mobile',
};

/** Singular label for what an entry is, shown in a card footer. */
export const TYPE_LABELS: Record<EcosystemType, string> = {
  sdk: 'SDK',
  plugin: 'Plugin',
  integration: 'Integration',
  guide: 'Guide',
};

/**
 * Types whose group is split by who maintains it.
 *
 * A product decision, not a property of the data: choosing between an official
 * and a community SDK matters to a reader, whereas plugins and guides are
 * evaluated the same way whoever wrote them. A type left out of this set gets
 * one combined group.
 */
const SPLIT_BY_ORIGIN: ReadonlySet<EcosystemType> = new Set<EcosystemType>(['sdk', 'integration']);

/** Top-level grouping key: a type, optionally prefixed by its origin. */
export type EcosystemGroup = string;

export function groupOf(entry: EcosystemEntry): EcosystemGroup {
  return SPLIT_BY_ORIGIN.has(entry.type) ? `${entry.origin}-${entry.type}` : entry.type;
}

const ORIGIN_LABELS: Record<EcosystemOrigin, string> = {official: 'Official', community: 'Community'};

/**
 * Human label for a group key, derived rather than listed.
 *
 * Adding a member to `EcosystemType` only requires a `TYPE_LABELS` entry, which
 * the compiler already demands, and the group label follows from it.
 */
function groupLabel(key: EcosystemGroup): string {
  const [head, ...rest] = key.split('-');
  if (rest.length === 0) return `${TYPE_LABELS[head as EcosystemType]}s`;
  return `${ORIGIN_LABELS[head as EcosystemOrigin]} ${TYPE_LABELS[rest.join('-') as EcosystemType]}s`;
}

/** Packages that ship inside someone else's library rather than on their own. */
const BUILT_IN = '(built-in)';

/** Availability as the filter panel presents it, collapsing `ga` and `beta`. */
export type EcosystemAvailability = 'available' | 'soon';

export function originOf(entry: EcosystemEntry): EcosystemOrigin {
  return entry.origin;
}

export function artifactOf(entry: EcosystemEntry): string {
  return TYPE_LABELS[entry.type];
}

export function availabilityOf(entry: EcosystemEntry): EcosystemAvailability {
  return entry.status === 'soon' ? 'soon' : 'available';
}

/** `soon` entries sit in their own grid and never link anywhere. */
export function isAvailable(entry: EcosystemEntry): boolean {
  return availabilityOf(entry) === 'available';
}

/**
 * The short label beside a card's origin badge: how to get the thing.
 *
 * Empty for a guide, which has no version of its own to report, and for
 * anything whose version could not be resolved. `version` arrives from the npm
 * registry lookup where the registry pins no value.
 */
export function versionLabel(entry: EcosystemEntry, version?: string): string {
  if (!isAvailable(entry)) return 'Soon';
  if (entry.package.name.includes(BUILT_IN)) return 'Built-in';
  if (entry.type === 'guide') return '';
  return version ?? '';
}

/**
 * Whether to show the category alongside the artifact label in a card footer.
 *
 * "Integration" beside "Integration" is noise; every other pairing is additive.
 */
export function showsCategory(entry: EcosystemEntry): boolean {
  return CATEGORY_LABELS[entry.category].toLowerCase() !== artifactOf(entry).toLowerCase();
}

/**
 * Where a listing card points.
 *
 * An entry with authored sections has a detail page of its own, which is the
 * better landing spot. Everything else goes straight to its docs, and an entry
 * with neither is not a link at all.
 */
export function entryHref(entry: EcosystemEntry): string | undefined {
  if (!isAvailable(entry)) return undefined;
  // Matches what the plugin registers a route for: an entry with guides but no
  // sections still gets a page, so its card still has somewhere to point.
  if (entry.sections?.length || entry.guides?.length) return `/sdks/${entry.id}`;
  return entry.docs?.overview;
}

/** Package coordinate as shown on a card, without the built-in suffix. */
export function packageLabel(entry: EcosystemEntry): string {
  return entry.package.name.replace(BUILT_IN, '').trim();
}

/** The three facet groups, plus the free-text query. */
export interface EcosystemFilters {
  query: string;
  groups: EcosystemGroup[];
  categories: EcosystemCategory[];
  availability: EcosystemAvailability[];
}

export const EMPTY_FILTERS: EcosystemFilters = {query: '', groups: [], categories: [], availability: []};

/** Which facet group a predicate belongs to, so counting can exclude it. */
export type EcosystemFacetGroup = 'groups' | 'categories' | 'availability';

function matchesQuery(entry: EcosystemEntry, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    entry.name.toLowerCase().includes(q) ||
    entry.package.name.toLowerCase().includes(q) ||
    entry.description.toLowerCase().includes(q) ||
    CATEGORY_LABELS[entry.category].toLowerCase().includes(q)
  );
}

/**
 * Applies the filters, optionally ignoring one group.
 *
 * An empty group means "no constraint" rather than "match nothing", so clearing
 * every box shows everything. `skip` exists for facet counting: a group's own
 * selections must not constrain the counts shown next to its boxes, otherwise
 * ticking one option drives its siblings to zero.
 */
function matches(entry: EcosystemEntry, filters: EcosystemFilters, skip?: EcosystemFacetGroup): boolean {
  if (!matchesQuery(entry, filters.query)) return false;
  if (skip !== 'groups' && filters.groups.length > 0 && !filters.groups.includes(groupOf(entry))) {
    return false;
  }
  if (skip !== 'categories' && filters.categories.length > 0 && !filters.categories.includes(entry.category)) {
    return false;
  }
  if (skip !== 'availability' && filters.availability.length > 0 && !filters.availability.includes(availabilityOf(entry))) {
    return false;
  }
  return true;
}

export function selectEntries(entries: EcosystemEntry[], filters: EcosystemFilters): EcosystemEntry[] {
  return entries.filter((entry) => matches(entry, filters));
}

export interface EcosystemFacet<T extends string> {
  key: T;
  label: string;
  count: number;
  selected: boolean;
}

/**
 * Builds one facet group from whatever the registry actually contains.
 *
 * Membership comes from the full entry set, never the filtered one: an option
 * that vanished as you narrowed the results would strand its own chip and make
 * the panel jump around. Counts, by contrast, do reflect the other groups'
 * selections, with this group's own ignored (see `matches`).
 *
 * `order` only sorts. A key it does not mention still appears, at the end, so a
 * newly used type or platform shows up without touching this file.
 */
function buildFacets<T extends string>(
  entries: EcosystemEntry[],
  filters: EcosystemFilters,
  group: EcosystemFacetGroup,
  selected: readonly T[],
  order: readonly T[],
  keyOf: (entry: EcosystemEntry) => T,
  labelOf: (key: T) => string,
): EcosystemFacet<T>[] {
  const rank = (key: T): number => {
    const index = order.indexOf(key);
    return index === -1 ? order.length : index;
  };

  return [...new Set(entries.map(keyOf))]
    .sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))
    .map((key) => ({
      key,
      label: labelOf(key),
      count: entries.filter((entry) => matches(entry, filters, group) && keyOf(entry) === key).length,
      selected: selected.includes(key),
    }));
}

/** Display order only. Keys absent from a list still render, sorted last. */
const GROUP_ORDER: readonly EcosystemGroup[] = [
  'official-sdk',
  'community-sdk',
  'plugin',
  'official-integration',
  'community-integration',
  'guide',
];

const CATEGORY_ORDER: readonly EcosystemCategory[] = ['spa', 'fullstack', 'backend', 'mobile', 'agent'];

const AVAILABILITY_ORDER: readonly EcosystemAvailability[] = ['available', 'soon'];

const AVAILABILITY_LABELS: Record<EcosystemAvailability, string> = {
  available: 'Available now',
  soon: 'Coming soon',
};

/** The filter panel's three groups, with counts that react to the other two. */
export function buildFilterPanel(
  entries: EcosystemEntry[],
  filters: EcosystemFilters,
): {
  groups: EcosystemFacet<EcosystemGroup>[];
  categories: EcosystemFacet<EcosystemCategory>[];
  availability: EcosystemFacet<EcosystemAvailability>[];
} {
  return {
    groups: buildFacets(entries, filters, 'groups', filters.groups, GROUP_ORDER, groupOf, groupLabel),
    categories: buildFacets(
      entries,
      filters,
      'categories',
      filters.categories,
      CATEGORY_ORDER,
      (e) => e.category,
      (key) => CATEGORY_LABELS[key],
    ),
    availability: buildFacets(
      entries,
      filters,
      'availability',
      filters.availability,
      AVAILABILITY_ORDER,
      availabilityOf,
      (key) => AVAILABILITY_LABELS[key],
    ),
  };
}

/** Every selected facet, flattened for the removable chips above the results. */
export function activeChips(filters: EcosystemFilters): {group: EcosystemFacetGroup; key: string; label: string}[] {
  const labelFor: Record<EcosystemFacetGroup, (key: string) => string> = {
    groups: groupLabel,
    categories: (key) => CATEGORY_LABELS[key as EcosystemCategory],
    availability: (key) => AVAILABILITY_LABELS[key as EcosystemAvailability],
  };
  const groups: EcosystemFacetGroup[] = ['groups', 'categories', 'availability'];
  return groups.flatMap((group) => filters[group].map((key: string) => ({group, key, label: labelFor[group](key)})));
}

/** Adds or removes one facet key, leaving the other groups untouched. */
export function toggleFacet(filters: EcosystemFilters, group: EcosystemFacetGroup, key: string): EcosystemFilters {
  const current: string[] = filters[group];
  const next = current.includes(key) ? current.filter((k) => k !== key) : [...current, key];
  return {...filters, [group]: next};
}
