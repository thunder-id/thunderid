/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Schema, FlattenedSchema} from '../models/scim2-schema';

/**
 * Flattens nested schema attributes into a flat structure for easier processing
 *
 * This function processes SCIM2 schemas and creates a flattened representation by:
 * - Processing sub-attributes and creating dot-notation names (e.g., 'name.givenName')
 * - Adding schema ID reference to each flattened attribute
 * - Preserving all original attribute properties while adding schema context
 * - Only including leaf-level attributes (sub-attributes) and top-level simple attributes
 *
 * @param schemas - Array of SCIM2 schemas containing nested attribute structures
 * @returns Array of flattened schema attributes with dot-notation names and schema references
 *
 * @example
 * ```typescript
 * const schemas = [
 *   {
 *     id: 'urn:ietf:params:scim:schemas:core:2.0:User',
 *     attributes: [
 *       {
 *         name: 'userName',
 *         type: 'string',
 *         multiValued: false
 *       },
 *       {
 *         name: 'name',
 *         type: 'complex',
 *         multiValued: false,
 *         subAttributes: [
 *           { name: 'givenName', type: 'string', multiValued: false },
 *           { name: 'familyName', type: 'string', multiValued: false }
 *         ]
 *       }
 *     ]
 *   }
 * ];
 *
 * const flattened = flattenUserSchema(schemas);
 * // Result: [
 * //   { name: 'userName', type: 'string', multiValued: false, schemaId: 'urn:ietf:params:scim:schemas:core:2.0:User' },
 * //   { name: 'name.givenName', type: 'string', multiValued: false, schemaId: 'urn:ietf:params:scim:schemas:core:2.0:User' },
 * //   { name: 'name.familyName', type: 'string', multiValued: false, schemaId: 'urn:ietf:params:scim:schemas:core:2.0:User' }
 * // ]
 * ```
 */
const flattenUserSchema = (schemas: Schema[]): FlattenedSchema[] => {
  const flattenedAttributes: FlattenedSchema[] = [];

  schemas.forEach((schema: Schema) => {
    if (schema.attributes && Array.isArray(schema.attributes)) {
      schema.attributes.forEach((attribute: any) => {
        // If the attribute has sub-attributes, only add the flattened sub-attributes
        if (attribute.subAttributes && Array.isArray(attribute.subAttributes)) {
          attribute.subAttributes.forEach((subAttribute: any) => {
            flattenedAttributes.push({
              ...subAttribute,
              name: `${attribute.name}.${subAttribute.name}`,
              schemaId: schema.id,
            } as unknown as FlattenedSchema);
          });
        } else {
          // If it's a simple attribute (no sub-attributes), add it directly
          flattenedAttributes.push({
            ...attribute,
            schemaId: schema.id,
          } as unknown as FlattenedSchema);
        }
      });
    }
  });

  return flattenedAttributes;
};

export default flattenUserSchema;
