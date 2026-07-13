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

import type {PropertyDefinition} from '@thunderid/configure-user-types';
import type {AttributeConfiguration, AttributeMapping} from '../models/connection';

/** Editable form state backing the attribute-mapping section. */
export interface AttributeMappingFormState {
  /** Default local user type federated identities are provisioned as. */
  userType: string;
  rows: AttributeMapping[];
}

/** A blank mapping row. */
export const emptyMappingRow = (): AttributeMapping => ({externalAttribute: '', localAttribute: ''});

/**
 * Build the API `attributeConfiguration` from the section state. Returns `undefined` when no
 * user type is selected (mappings cannot be attached to a user type). Incomplete rows (missing
 * either side) are dropped; the mappings entry is omitted entirely when no complete rows remain
 * (a default user type alone is still a valid configuration).
 */
export function toAttributeConfiguration(state: AttributeMappingFormState): AttributeConfiguration | undefined {
  const userType: string = state.userType.trim();
  if (!userType) {
    return undefined;
  }

  const attributes: AttributeMapping[] = state.rows
    .map((row) => ({externalAttribute: row.externalAttribute.trim(), localAttribute: row.localAttribute.trim()}))
    .filter((row) => row.externalAttribute !== '' && row.localAttribute !== '');

  return {
    userTypeResolution: {default: userType},
    ...(attributes.length > 0 ? {userTypeAttributeMappings: [{userType, attributes}]} : {}),
  };
}

/**
 * Derive editable section state from a fetched `attributeConfiguration` (edit prefill).
 */
export function fromAttributeConfiguration(config: AttributeConfiguration | undefined): AttributeMappingFormState {
  const userType: string = config?.userTypeResolution?.default ?? '';
  const entry =
    config?.userTypeAttributeMappings?.find((mapping) => mapping.userType === userType) ??
    config?.userTypeAttributeMappings?.[0];
  const rows: AttributeMapping[] = (entry?.attributes ?? []).map((attribute) => ({...attribute}));
  return {userType, rows};
}

/**
 * Flatten a user-type JSON schema into a flat list of assignable attribute names (dot-notation
 * for nested objects). Credential and array attributes are excluded. Mirrors the flattening used
 * by the applications token-settings attribute picker.
 */
export function flattenUserTypeAttributes(
  schema: Record<string, PropertyDefinition> | undefined,
  prefix = '',
): string[] {
  if (!schema) {
    return [];
  }

  const attributes: string[] = [];
  for (const [key, definition] of Object.entries(schema)) {
    const value = definition as PropertyDefinition & {
      type?: string;
      credential?: boolean;
      properties?: Record<string, PropertyDefinition>;
    };
    const fullKey = `${prefix}${key}`;

    if (value.credential) {
      continue;
    }
    if (value.type === 'object' && value.properties) {
      attributes.push(...flattenUserTypeAttributes(value.properties, `${fullKey}.`));
    } else if (value.type !== 'array') {
      attributes.push(fullKey);
    }
  }
  return attributes;
}
