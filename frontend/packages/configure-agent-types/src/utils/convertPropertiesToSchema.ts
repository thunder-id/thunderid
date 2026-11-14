// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isContactAttributeType} from '@thunderid/utils';
import type {
  AgentTypeDefinition,
  PropertyDefinition,
  PropertyType,
  SchemaPropertyInput,
} from '../models/property-definition';

/**
 * Convert editable property inputs to the API schema format. Properties without a name are
 * skipped, and each property's position is saved as its display order.
 *
 * @param properties - The schema properties being edited
 * @returns The agent type schema the API accepts
 *
 * @example
 * ```ts
 * const schema = convertPropertiesToSchema(properties);
 * ```
 *
 * @public
 */
export default function convertPropertiesToSchema(properties: SchemaPropertyInput[]): AgentTypeDefinition {
  const schema: AgentTypeDefinition = {};

  properties
    .filter((prop) => prop.name.trim())
    .forEach((prop, index) => {
      const actualType: PropertyType = prop.type === 'enum' ? 'string' : prop.type;

      const propDef: Partial<PropertyDefinition> = {
        type: actualType,
        required: prop.required,
        ...(prop.displayName.trim() ? {displayName: prop.displayName.trim()} : {}),
        display: {order: index, ...(!prop.required && prop.hiddenOnCreate ? {onCreate: {hidden: true}} : {})},
      };

      if (prop.unique) {
        (propDef as {unique?: boolean}).unique = true;
      }

      if ((prop.type === 'string' || prop.type === 'number' || prop.type === 'enum') && prop.credential) {
        (propDef as {credential?: boolean}).credential = true;
      }

      if (prop.type === 'string' || isContactAttributeType(prop.type) || prop.type === 'enum') {
        if (prop.enum.length > 0) {
          (propDef as {enum?: string[]}).enum = prop.enum;
        }
        if (prop.regex.trim() || isContactAttributeType(prop.type)) {
          (propDef as {regex?: string}).regex = prop.regex;
        }
      }

      if (prop.type === 'array') {
        (propDef as {items?: {type: string}}).items = prop.items ?? {type: 'string'};
      } else if (prop.type === 'object') {
        (propDef as {properties?: Record<string, PropertyDefinition>}).properties = prop.properties ?? {};
      }

      schema[prop.name.trim()] = propDef as PropertyDefinition;
    });

  return schema;
}
