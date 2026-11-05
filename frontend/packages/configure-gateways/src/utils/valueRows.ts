// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ValueRow} from '../components/ValuesTable';
import type {StoredValue, ValueKind, ValueReference} from '../models/deploy';

/** Every name of one kind a target holds or the version it runs refers to, unset rows first. */
export default function valueRowsOf(kind: ValueKind, stored: StoredValue[], references: ValueReference[]): ValueRow[] {
  const byName = new Map<string, ValueRow>();
  references
    .filter((reference: ValueReference) => reference.kind === kind)
    .forEach((reference: ValueReference) => {
      const row = byName.get(reference.name) ?? {name: reference.name, references: []};
      row.references.push(reference);
      byName.set(reference.name, row);
    });
  stored.forEach((value: StoredValue) => {
    const row = byName.get(value.name) ?? {name: value.name, references: []};
    row.stored = value;
    byName.set(value.name, row);
  });
  return [...byName.values()].sort((a: ValueRow, b: ValueRow) => {
    const unset = (row: ValueRow): number => (row.stored ? 1 : 0);
    return unset(a) - unset(b) || a.name.localeCompare(b.name);
  });
}
