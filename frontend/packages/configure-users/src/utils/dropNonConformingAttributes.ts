/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {PropertyDefinition} from '../models/users';

/** Whether a stored value still matches its schema definition (type, enum, regex). */
export function attributeConformsToSchema(value: unknown, fieldDef: PropertyDefinition): boolean {
  switch (fieldDef.type) {
    case 'string': {
      if (typeof value !== 'string') return false;
      if (fieldDef.enum && fieldDef.enum.length > 0 && !fieldDef.enum.includes(value)) return false;
      if (fieldDef.regex) {
        try {
          return new RegExp(fieldDef.regex).test(value);
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
