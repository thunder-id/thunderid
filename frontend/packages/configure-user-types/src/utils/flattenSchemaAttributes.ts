// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PropertyDefinition, UserTypeDefinition} from '../types/user-types';

/**
 * A single assignable attribute flattened out of a user type schema.
 */
export interface FlattenedAttribute {
  /** Attribute name, dot-notated for attributes nested inside an object property. */
  attribute: string;
  /** Whether the attribute is a credential (password and similar). */
  credential: boolean;
}

/** Credentials only exist on string and number properties; read the flag off the union safely. */
function isCredential(definition: PropertyDefinition): boolean {
  return 'credential' in definition && Boolean(definition.credential);
}

/**
 * Flatten a user type schema into a list of assignable attribute names, using dot notation for
 * attributes nested inside object properties. Array properties are skipped because they are not
 * individually assignable. Credential attributes are tagged rather than dropped, so callers can
 * either include or exclude them.
 *
 * @param schema - The user type schema to flatten.
 * @param prefix - Dot-notation prefix applied while recursing into nested objects.
 * @returns The flattened attributes.
 */
export default function flattenSchemaAttributes(
  schema: UserTypeDefinition | undefined,
  prefix = '',
): FlattenedAttribute[] {
  if (!schema) {
    return [];
  }

  const attributes: FlattenedAttribute[] = [];

  for (const [key, definition] of Object.entries(schema)) {
    const fullKey = `${prefix}${key}`;

    if (definition.type === 'object' && definition.properties) {
      attributes.push(...flattenSchemaAttributes(definition.properties, `${fullKey}.`));
    } else if (definition.type !== 'array') {
      attributes.push({attribute: fullKey, credential: isCredential(definition)});
    }
  }

  return attributes;
}
