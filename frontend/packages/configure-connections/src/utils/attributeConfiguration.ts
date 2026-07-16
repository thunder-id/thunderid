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

/** A single external-value → local-user-type entry in the claim-driven resolution table. */
export interface ValueMappingEntry {
  value: string;
  userType: string;
}

/** An attribute-mapping profile for one user type, as edited in the form. */
export interface MappingGroup {
  userType: string;
  rows: AttributeMapping[];
}

/** Editable form state backing the attribute-configuration section (all three sub-sections). */
export interface AttributeMappingFormState {
  /** Default local user type an external identity resolves to (fallback when dynamic). */
  defaultUserType: string;
  /** Whether the user type is resolved from an external attribute's value. */
  resolveDynamic: boolean;
  /** External attribute whose value drives dynamic resolution. */
  externalAttribute: string;
  /** External-value → local-user-type mappings for dynamic resolution. */
  valueMapping: ValueMappingEntry[];
  /** Per-user-type attribute-mapping profiles. */
  groups: MappingGroup[];
  /** External attributes combined (AND) to link a returning identity to an existing account. */
  linking: string[];
}

/**
 * Build the API `attributeConfiguration` from the section state. Returns `undefined` when the whole
 * configuration is empty (no default type, no dynamic resolution, no complete mappings, no linking).
 * Incomplete mapping rows (missing either side) are dropped and groups without a user type or
 * without complete rows are omitted; the external attribute is included whenever dynamic resolution
 * is enabled (value mappings are optional — every identity resolves to the default until they're
 * added); account linking is included only when it has non-empty attributes.
 */
export function toAttributeConfiguration(state: AttributeMappingFormState): AttributeConfiguration | undefined {
  const defaultUserType: string = state.defaultUserType.trim();

  const valueMapping: Record<string, string> = {};
  if (state.resolveDynamic) {
    for (const entry of state.valueMapping) {
      const value: string = entry.value.trim();
      const userType: string = entry.userType.trim();
      if (value !== '' && userType !== '') {
        valueMapping[value] = userType;
      }
    }
  }
  const externalAttribute: string = state.externalAttribute.trim();
  // An external attribute alone is enough — every identity resolves to the default until value
  // mappings are added, matching the backend (which only rejects a value mapping with no attribute).
  const hasDynamic: boolean = state.resolveDynamic && externalAttribute !== '';
  const hasValueMapping: boolean = hasDynamic && Object.keys(valueMapping).length > 0;

  const userTypeAttributeMappings = state.groups
    .map((group) => ({
      userType: group.userType.trim(),
      attributes: group.rows
        .map((row) => ({externalAttribute: row.externalAttribute.trim(), localAttribute: row.localAttribute.trim()}))
        .filter((row) => row.externalAttribute !== '' && row.localAttribute !== ''),
    }))
    .filter((group) => group.userType !== '' && group.attributes.length > 0);

  const linking: string[] = state.linking.map((attribute) => attribute.trim()).filter((attribute) => attribute !== '');

  if (defaultUserType === '' && !hasDynamic && userTypeAttributeMappings.length === 0 && linking.length === 0) {
    return undefined;
  }

  return {
    userTypeResolution: {
      default: defaultUserType,
      ...(hasDynamic ? {externalAttribute} : {}),
      ...(hasValueMapping ? {valueMapping} : {}),
    },
    ...(userTypeAttributeMappings.length > 0 ? {userTypeAttributeMappings} : {}),
    ...(linking.length > 0 ? {accountLinking: {attributes: linking}} : {}),
  };
}

/**
 * Derive editable section state from a fetched `attributeConfiguration` (edit prefill).
 */
export function fromAttributeConfiguration(config: AttributeConfiguration | undefined): AttributeMappingFormState {
  const resolution = config?.userTypeResolution;
  const valueMapping: ValueMappingEntry[] = Object.entries(resolution?.valueMapping ?? {}).map(([value, userType]) => ({
    value,
    userType,
  }));
  const resolveDynamic: boolean = Boolean(resolution?.externalAttribute) || valueMapping.length > 0;

  const groups: MappingGroup[] = (config?.userTypeAttributeMappings ?? []).map((mapping) => ({
    userType: mapping.userType,
    rows: mapping.attributes.map((attribute) => ({...attribute})),
  }));

  return {
    defaultUserType: resolution?.default ?? '',
    resolveDynamic,
    externalAttribute: resolution?.externalAttribute ?? '',
    valueMapping,
    groups,
    linking: [...(config?.accountLinking?.attributes ?? [])],
  };
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
