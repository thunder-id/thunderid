// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The attributes the agent onboarding flow asks for by name, in the order it asks for them. A schema
 * reaches the console with its attributes in the order the server stored them, which is not always the
 * order they were declared in, so these are placed as the flow places them: the provider before its model.
 */
const KNOWN_ATTRIBUTE_ORDER: readonly string[] = ['modelProvider', 'model', 'function'];

/** Where a known attribute sits among the others of its group. An attribute not known comes after them. */
function knownPosition(name: string): number {
  const index = KNOWN_ATTRIBUTE_ORDER.indexOf(name);

  return index === -1 ? KNOWN_ATTRIBUTE_ORDER.length : index;
}

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
 * Orders the attributes of an agent type schema, as the user pages order a user type's. The display
 * attribute is the one the agent type names, if any. Within a group, the attributes the onboarding
 * flow knows come first, in its order, and the rest keep their schema order. None are dropped, since an agent's optional
 * attributes, such as its model, are the ones that describe it.
 */
export default function orderAgentAttributes<T extends object>(
  schema: Record<string, T>,
  displayAttribute?: string,
): [string, T][] {
  return Object.entries(schema)
    .map(([name, definition]) => ({name, definition, position: rank(name, definition, displayAttribute)}))
    .sort((a, b) => a.position - b.position || knownPosition(a.name) - knownPosition(b.name))
    .map(({name, definition}) => [name, definition]);
}
