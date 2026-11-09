// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {compareAttributes, getDisplayOrder, isHiddenOnCreate} from './attributeDisplay';

/**
 * Orders the attributes of a user type or agent type schema, as `compareAttributes` does. A `leading`
 * attribute without a display order, such as an agent type's display attribute, comes first among the
 * attributes without one.
 *
 * @param schema - The entity type schema
 * @param options - Set `onCreate` to leave out the optional attributes hidden on create, and `leading`
 * to name an attribute that leads the ones without a display order
 * @returns The attributes in order, each with its definition
 *
 * @public
 */
export default function orderAttributes<T extends object>(
  schema: Record<string, T>,
  {onCreate = false, leading}: {onCreate?: boolean; leading?: string} = {},
): [string, T][] {
  const leads = (name: string, definition: T): boolean => name === leading && getDisplayOrder(definition) === undefined;

  return Object.entries(schema)
    .filter(([, definition]) => !onCreate || !isHiddenOnCreate(definition))
    .sort(([nameA, a], [nameB, b]) => {
      const leadDiff = Number(leads(nameB, b)) - Number(leads(nameA, a));
      if (leadDiff !== 0 && getDisplayOrder(a) === undefined && getDisplayOrder(b) === undefined) return leadDiff;
      return compareAttributes(nameA, a, nameB, b);
    });
}
