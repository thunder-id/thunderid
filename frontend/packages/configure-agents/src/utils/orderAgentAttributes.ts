// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Position of an attribute: the agent type's display attribute first, then the other required
 * attributes, then the optional ones, then the credentials, required before optional.
 */
function rank(name: string, definition: object, displayAttribute: string | undefined): number {
  if (displayAttribute !== undefined && name === displayAttribute) return 0;

  const required = 'required' in definition && definition.required === true;
  const credential = 'credential' in definition && definition.credential === true;
  if (credential) return required ? 3 : 4;

  return required ? 1 : 2;
}

/**
 * Orders the attributes of an agent type schema, as the user pages order a user type's. Everything
 * comes from the schema: the display attribute is the one the agent type names, if any, and
 * attributes in the same group keep their schema order. None are dropped, since an agent's optional
 * attributes, such as its model, are the ones that describe it.
 */
export default function orderAgentAttributes<T extends object>(
  schema: Record<string, T>,
  displayAttribute?: string,
): [string, T][] {
  return Object.entries(schema)
    .map(([name, definition]) => ({name, definition, position: rank(name, definition, displayAttribute)}))
    .sort((a, b) => a.position - b.position)
    .map(({name, definition}) => [name, definition]);
}
