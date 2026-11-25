// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ValueRow} from '../components/ValuesTable';
import type {StoredValue, ValueKind, ValueReference} from '../models/deploy';

/**
 * Every name of one kind a target holds or the version it runs refers to, unset rows first. `current`
 * is where the configuration as it stands refers to values: a name the running version does not refer
 * to is still said to belong to the resource that the current configuration has refer to it, so a value
 * is not left as a bare name.
 */
export default function valueRowsOf(
  kind: ValueKind,
  stored: StoredValue[],
  references: ValueReference[],
  current: ValueReference[] = [],
): ValueRow[] {
  const byName = new Map<string, ValueRow>();
  references
    .filter((reference: ValueReference) => reference.kind === kind)
    .forEach((reference: ValueReference) => {
      const row = byName.get(reference.name) ?? {name: reference.name, references: [], current: []};
      row.references.push(reference);
      byName.set(reference.name, row);
    });
  stored.forEach((value: StoredValue) => {
    const row = byName.get(value.name) ?? {name: value.name, references: [], current: []};
    row.stored = value;
    byName.set(value.name, row);
  });
  current
    .filter((reference: ValueReference) => reference.kind === kind)
    .forEach((reference: ValueReference) => {
      const row = byName.get(reference.name);
      if (row?.references.length === 0) row.current.push(reference);
    });
  return [...byName.values()].sort((a: ValueRow, b: ValueRow) => {
    const unset = (row: ValueRow): number => (row.stored ? 1 : 0);
    return unset(a) - unset(b) || a.name.localeCompare(b.name);
  });
}

/**
 * The rows with the hidden resources left out: a value only hidden resources use is dropped, and a value
 * other resources use too is kept without them. A value no resource uses is kept.
 */
export function withoutHiddenResources(
  rows: ValueRow[],
  isHiddenResource: (resourceType: string, id: string | undefined) => boolean,
): ValueRow[] {
  const isShown = (reference: ValueReference): boolean =>
    !isHiddenResource(reference.resourceType, reference.resourceId);
  return rows.flatMap((row: ValueRow) => {
    const uses = [...row.references, ...row.current];
    if (uses.length > 0 && !uses.some(isShown)) return [];
    return [{...row, references: row.references.filter(isShown), current: row.current.filter(isShown)}];
  });
}

/**
 * The rows of a page of stored values with the hidden resources' uses taken off. Unlike
 * `withoutHiddenResources`, a value only hidden resources use is not dropped but kept as `hiddenUse`, so a
 * page shows every value the server counted and its paging holds. Such a value is left as it is: it is
 * neither set nor removed from here.
 */
export function maskHiddenResources(
  rows: ValueRow[],
  isHiddenResource: (resourceType: string, id: string | undefined) => boolean,
): ValueRow[] {
  const isShown = (reference: ValueReference): boolean =>
    !isHiddenResource(reference.resourceType, reference.resourceId);
  return rows.map((row: ValueRow) => {
    const uses = [...row.references, ...row.current];
    if (uses.length > 0 && !uses.some(isShown)) return {...row, references: [], current: [], hiddenUse: true};
    return {...row, references: row.references.filter(isShown), current: row.current.filter(isShown)};
  });
}

/** The names of one kind the references refer to, each once. */
export function referencedNames(references: ValueReference[], kind: ValueKind): string[] {
  return [
    ...new Set(
      references
        .filter((reference: ValueReference) => reference.kind === kind)
        .map((reference: ValueReference) => reference.name),
    ),
  ];
}

/**
 * The rows of one page of stored values, in the order the page holds them, each with where the running
 * version refers to it, or, when it does not, where the configuration as it stands does.
 */
export function pageRowsOf(
  kind: ValueKind,
  stored: StoredValue[],
  references: ValueReference[],
  current: ValueReference[] = [],
): ValueRow[] {
  const ofKind = (reference: ValueReference): boolean => reference.kind === kind;
  return stored.map((value: StoredValue) => {
    const own = references.filter((reference: ValueReference) => ofKind(reference) && reference.name === value.name);
    return {
      name: value.name,
      stored: value,
      references: own,
      current:
        own.length > 0
          ? []
          : current.filter((reference: ValueReference) => ofKind(reference) && reference.name === value.name),
    };
  });
}

/**
 * The names of one kind the running version refers to that the target does not hold, given what it holds
 * of them, with the hidden resources left out as `withoutHiddenResources` leaves them out.
 */
export function missingRowsOf(
  kind: ValueKind,
  stored: StoredValue[],
  references: ValueReference[],
  isHiddenResource: (resourceType: string, id: string | undefined) => boolean,
): ValueRow[] {
  return withoutHiddenResources(valueRowsOf(kind, stored, references), isHiddenResource).filter(
    (row: ValueRow) => !row.stored && row.references.length > 0,
  );
}

/** Where a row's value is used: by the running version if it is, otherwise by the current configuration. */
export const placesOf = (row: ValueRow): ValueReference[] => (row.references.length > 0 ? row.references : row.current);

/** Whether a row's value is held: a variable's value, or a secret that exists. */
export const isHeld = (row: ValueRow, secret: boolean): boolean =>
  secret ? Boolean(row.stored) && row.stored?.exists !== false : row.stored?.value !== undefined;

/** Whether a row's value is a list, entered one item per line. */
export const isListRow = (row: ValueRow): boolean =>
  placesOf(row).some((reference: ValueReference) => reference.list === true);
