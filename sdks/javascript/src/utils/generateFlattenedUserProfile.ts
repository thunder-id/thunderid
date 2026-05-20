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

import get from './get';
import {User} from '../models/user';

/**
 * Generates a flattened user profile from a response object and schema definitions.
 *
 * This function processes user data according to schema specifications, creating
 * a flat object with dot notation keys instead of nested objects. Multi-valued
 * properties and type-specific defaults are handled appropriately.
 *
 * Additionally, any fields present in the response but not defined in the schema
 * will be included to ensure no user data is lost during flattening.
 *
 * @param meResponse - The response object containing user data
 * @param processedSchemas - Array of schema objects defining field properties
 * @param processedSchemas[].name - The field name/path for the property
 * @param processedSchemas[].type - The data type of the field (e.g., 'STRING')
 * @param processedSchemas[].multiValued - Whether the field can contain multiple values
 *
 * @returns A flattened user profile object with dot notation keys
 *
 * @example
 * ```typescript
 * const schemas = [
 *   { name: 'name.givenName', type: 'STRING', multiValued: false },
 *   { name: 'emails', type: 'STRING', multiValued: true }
 * ];
 * const response = {
 *   name: { givenName: 'John' },
 *   emails: 'john@example.com',
 *   country: 'US' // This will be included even if not in schema
 * };
 * const profile = generateFlattenedUserProfile(response, schemas);
 * // Result: { "name.givenName": 'John', emails: ['john@example.com'], country: 'US' }
 * ```
 */
const generateFlattenedUserProfile = (meResponse: any, processedSchemas: any[]): User => {
  const profile: User = {};

  const allSchemaNames: string[] = processedSchemas.map((schema: any) => schema.name).filter(Boolean);

  // First, process all schema-defined fields
  processedSchemas.forEach((schema: any) => {
    const {name, type, multiValued} = schema;

    if (!name) return;

    // Skip this property if it's a parent of other flattened properties
    // e.g., skip "name" if "name.givenName" or "name.familyName" exists
    // skip "roles" if "roles.default" exists
    const hasChildProperties: boolean = allSchemaNames.some(
      (schemaName: string) => schemaName !== name && schemaName.startsWith(`${name}.`),
    );

    if (hasChildProperties) {
      // Skip this parent property
      return;
    }

    let value: any = get(meResponse, name);

    // SCIM2 multi-valued complex attributes (phoneNumbers, emails, ims, ...)
    // are stored as arrays of typed objects: [{type: "mobile", value: "..."}].
    // The schema flattens them as `<attr>.<type>` (e.g. "phoneNumbers.mobile"),
    // so when the dotted lookup misses, fall back to filtering the array by
    // `type` and reading `.value`.
    if (value === undefined) {
      const dotIndex: number = name.indexOf('.');
      if (dotIndex > 0) {
        const head: string = name.slice(0, dotIndex);
        const tail: string = name.slice(dotIndex + 1);
        const arr: any = meResponse[head];
        if (Array.isArray(arr)) {
          const match: any = arr.find((item: any) => item?.type === tail);
          if (match?.value !== undefined) {
            value = match.value;
          }
        }
      }
    }

    // If value not found at top level, check within schema namespaces
    if (value === undefined) {
      const schemaNamespaces: string[] = [
        'urn:ietf:params:scim:schemas:core:2.0:User',
        'urn:ietf:params:scim:schemas:extension:enterprise:2.0:User',
        'urn:scim:wso2:schema',
        'urn:scim:schemas:extension:custom:User',
      ];

      schemaNamespaces.some((namespace: string) => {
        if (meResponse[namespace]) {
          // Try the field name directly within the namespace
          if (meResponse[namespace][name] !== undefined) {
            value = meResponse[namespace][name];
            return true; // Break out of some()
          }
          // Also try using get() for nested paths within the namespace
          const nestedValue: any = get(meResponse[namespace], name);
          if (nestedValue !== undefined) {
            value = nestedValue;
            return true; // Break out of some()
          }
        }
        return false;
      });
    }

    if (value !== undefined) {
      if (multiValued && !Array.isArray(value)) {
        value = [value];
      }
    } else if (multiValued) {
      value = undefined;
    } else if (type === 'STRING') {
      value = '';
    } else {
      value = undefined;
    }

    profile[name] = value;
  });

  // Then, include any additional fields from meResponse that aren't in the schema
  // This ensures fields like 'country' are not missed if they exist in the response
  const flattenObject = (obj: any, prefix = ''): void => {
    if (obj && typeof obj === 'object' && !Array.isArray(obj)) {
      Object.keys(obj).forEach((key: string) => {
        const fullKey: string = prefix ? `${prefix}.${key}` : key;
        const value: any = obj[key];

        // Skip if this field is already processed by schema
        if (Object.prototype.hasOwnProperty.call(profile, fullKey)) {
          return;
        }

        // Skip if this is a parent of schema-defined fields
        const hasSchemaChildProperties: boolean = allSchemaNames.some((schemaName: string) =>
          schemaName.startsWith(`${fullKey}.`),
        );

        if (hasSchemaChildProperties) {
          // Recursively process child properties
          flattenObject(value, fullKey);
        } else {
          // Include the field as-is
          profile[fullKey] = value;
        }
      });
    }
  };

  flattenObject(meResponse);

  return profile;
};

export default generateFlattenedUserProfile;
