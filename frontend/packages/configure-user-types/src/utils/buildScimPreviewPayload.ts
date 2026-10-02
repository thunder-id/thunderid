// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SCIM_ENTERPRISE_EXTENSION_URN, SCIM_MULTI_VALUED_TARGETS, buildThunderIdExtensionUrn} from '../constants/scimTargets';
import type {ScimMultiValuedMeta} from '../types/user-types';

const MULTI_VALUED_TARGET_SET = new Set<string>(SCIM_MULTI_VALUED_TARGETS);
const ENTERPRISE_TARGETS = new Set(['employeeNumber', 'costCenter', 'organization', 'division', 'department']);

/**
 * Builds a placeholder SCIM User payload preview from the current property-to-target mapping.
 * Each mapped property is shown as `<propertyName>` in the position its SCIM target occupies,
 * matching how the JWT/token attribute previews show `<attr>` placeholders. Multi-valued targets
 * (emails/phoneNumbers/photos) collect every property mapped to them into one array, each entry
 * carrying that property's own `type`/`primary` from `meta`. Properties present in
 * `allPropertyNames` but not mapped to any SCIM target are shown under this user type's own
 * custom extension URN, matching the backend's real core-type dedup behavior: unmapped
 * attributes fall into the custom extension schema, not the core/enterprise ones.
 */
export default function buildScimPreviewPayload(
  mapping: Record<string, string>,
  meta: Record<string, ScimMultiValuedMeta>,
  allPropertyNames: string[],
  userTypeName: string,
): Record<string, unknown> {
  const payload: Record<string, unknown> = {};
  const name: Record<string, string> = {};
  const addresses: Record<string, string> = {};
  const enterprise: Record<string, unknown> = {};
  const multiValued: Record<string, Record<string, unknown>[]> = {};

  Object.entries(mapping).forEach(([propertyName, target]) => {
    if (!target) return;
    const placeholder = `<${propertyName}>`;

    if (target === 'manager') {
      enterprise['manager'] = {value: placeholder};
      return;
    }
    if (ENTERPRISE_TARGETS.has(target)) {
      enterprise[target] = placeholder;
      return;
    }
    if (MULTI_VALUED_TARGET_SET.has(target)) {
      const entryMeta = meta[propertyName];
      const entry: Record<string, unknown> = {value: placeholder};
      if (entryMeta?.type) {
        entry['type'] = entryMeta.type;
      }
      if (entryMeta?.primary) {
        entry['primary'] = true;
      }
      multiValued[target] = [...(multiValued[target] ?? []), entry];
      return;
    }
    if (target.startsWith('name.')) {
      name[target.slice('name.'.length)] = placeholder;
      return;
    }
    if (target.startsWith('addresses.')) {
      addresses[target.slice('addresses.'.length)] = placeholder;
      return;
    }
    payload[target] = placeholder;
  });

  if (Object.keys(name).length > 0) {
    payload['name'] = name;
  }
  if (Object.keys(addresses).length > 0) {
    payload['addresses'] = [addresses];
  }
  Object.entries(multiValued).forEach(([target, entries]) => {
    payload[target] = entries.length === 1 ? [{...entries[0], primary: true}] : entries;
  });
  if (Object.keys(enterprise).length > 0) {
    payload[SCIM_ENTERPRISE_EXTENSION_URN] = enterprise;
  }

  const unmapped = allPropertyNames.filter((propertyName) => !mapping[propertyName]);
  if (unmapped.length > 0) {
    const custom: Record<string, string> = {};
    unmapped.forEach((propertyName) => {
      custom[propertyName] = `<${propertyName}>`;
    });
    payload[buildThunderIdExtensionUrn(userTypeName)] = custom;
  }

  return payload;
}
