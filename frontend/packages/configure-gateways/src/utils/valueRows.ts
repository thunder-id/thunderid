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
