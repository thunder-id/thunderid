// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PropertyDefinition} from '../models/users';

/** Attributes shown first, in this order, whether required or optional. */
const LEADING_ATTRIBUTES = ['username', 'given_name', 'family_name', 'email', 'mobile_number'];

const PASSWORD_ATTRIBUTE = 'password';

/**
 * Position of an attribute in the form, or null when it is not shown. Leading attributes come
 * first, then other required attributes, then other optional attributes when included, then
 * required credentials and the password last. A required password leads the required
 * credentials. Other optional attributes are not shown unless included.
 */
function rank(name: string, definition: PropertyDefinition, includeOptional: boolean): number | null {
  const leadingIndex = LEADING_ATTRIBUTES.indexOf(name);
  if (leadingIndex >= 0) return leadingIndex;

  const base = LEADING_ATTRIBUTES.length;
  const required = definition.required === true;
  const credential = 'credential' in definition && definition.credential === true;
  if (name === PASSWORD_ATTRIBUTE) return required ? base + 2 : base + 4;
  if (!required) {
    if (!includeOptional) return null;
    return credential ? base + 5 : base + 1;
  }
  return credential ? base + 3 : base;
}

/**
 * Orders the attributes of a user type schema. The optional ones that are not commonly needed are
 * dropped, as on user creation, unless includeOptional is set.
 */
export default function orderSchemaAttributes(
  schema: Record<string, PropertyDefinition>,
  {includeOptional = false}: {includeOptional?: boolean} = {},
): [string, PropertyDefinition][] {
  return Object.entries(schema)
    .map(([name, definition]) => ({name, definition, position: rank(name, definition, includeOptional)}))
    .filter(
      (entry): entry is {name: string; definition: PropertyDefinition; position: number} => entry.position !== null,
    )
    .sort((a, b) => a.position - b.position)
    .map(({name, definition}) => [name, definition]);
}
