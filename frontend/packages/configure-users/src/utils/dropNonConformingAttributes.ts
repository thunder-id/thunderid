// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isContactAttributeType, resolveAttributeRegex} from '@thunderid/utils';
import type {PropertyDefinition} from '../models/users';

/** Whether a stored value still matches its schema definition (type, enum, regex). */
export function attributeConformsToSchema(value: unknown, fieldDef: PropertyDefinition): boolean {
  switch (fieldDef.type) {
    case 'email':
    case 'mobile':
    case 'string': {
      if (typeof value !== 'string') return false;
      if (isContactAttributeType(fieldDef.type) && value === '') return false;
      if (fieldDef.enum && fieldDef.enum.length > 0 && !fieldDef.enum.includes(value)) return false;
      const regex = resolveAttributeRegex(fieldDef.type, fieldDef.regex);
      if (regex) {
        try {
          return new RegExp(regex).test(value);
        } catch {
          return true; // unparseable schema regex can't judge the value
        }
      }
      return true;
    }
    case 'number':
      return typeof value === 'number' && Number.isFinite(value);
    case 'boolean':
      return typeof value === 'boolean';
    case 'array':
      return Array.isArray(value);
    case 'object':
      return typeof value === 'object' && value !== null && !Array.isArray(value);
    default:
      return true;
  }
}

/**
 * Drop stale values for optional declared attributes; keep required ones (backend rejects, user fixes)
 * and undeclared keys (backend strips those).
 */
export function dropNonConformingOptionalAttributes(
  attributes: Record<string, unknown>,
  schema: Record<string, PropertyDefinition> | undefined,
): Record<string, unknown> {
  if (!schema) return attributes;

  const result: Record<string, unknown> = {};
  Object.entries(attributes).forEach(([key, value]) => {
    const fieldDef = schema[key];
    if (fieldDef && !fieldDef.required && !attributeConformsToSchema(value, fieldDef)) {
      return;
    }
    result[key] = value;
  });

  return result;
}
