// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {MissingValues, ValueKind, ValueReference} from '../models/deploy';

export interface MissingField {
  kind: ValueKind;
  name: string;
  /** Whether the value is a list, entered one item per line. */
  list: boolean;
  /** The resource and field that refer to the value, for the label. */
  references: ValueReference[];
}

export const fieldKey = (field: MissingField): string => `${field.kind}:${field.name}`;

/** The values a dry run found missing, as fields to fill, each knowing what refers to it. */
export function missingFields(missing: MissingValues | undefined): MissingField[] {
  if (!missing) return [];
  const references = missing.references ?? [];
  const of = (kind: ValueKind, name: string): MissingField => {
    const own = references.filter((reference: ValueReference) => reference.kind === kind && reference.name === name);
    return {kind, name, list: own.some((reference: ValueReference) => reference.list === true), references: own};
  };
  return [
    ...(missing.secrets ?? []).map((name: string) => of('secret', name)),
    ...(missing.variables ?? []).map((name: string) => of('variable', name)),
  ];
}
