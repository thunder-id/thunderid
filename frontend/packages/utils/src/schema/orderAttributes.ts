// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {compareAttributes, isHiddenOnCreate} from './attributeDisplay';

/**
 * Orders the attributes of a user type or agent type schema, as `compareAttributes` does.
 *
 * @param schema - The entity type schema
 * @param options - Set `onCreate` to leave out the optional attributes hidden on create
 * @returns The attributes in order, each with its definition
 *
 * @public
 */
export default function orderAttributes<T extends object>(
  schema: Record<string, T>,
  {onCreate = false}: {onCreate?: boolean} = {},
): [string, T][] {
  return Object.entries(schema)
    .filter(([, definition]) => !onCreate || !isHiddenOnCreate(definition))
    .sort(([nameA, a], [nameB, b]) => compareAttributes(nameA, a, nameB, b));
}
