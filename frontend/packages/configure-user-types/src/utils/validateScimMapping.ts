// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SCIM_MULTI_VALUED_TARGETS} from '../constants/scimTargets';
import type {ScimMultiValuedMeta} from '../types/user-types';

const MULTI_VALUED_TARGET_SET = new Set<string>(SCIM_MULTI_VALUED_TARGETS);

export type ScimMappingValidationError =
  | {type: 'duplicateTarget'; target: string; properties: string[]}
  | {type: 'multiplePrimary'; target: string; properties: string[]};

/**
 * Blocking validation only. userName completeness is a non-blocking warning shown separately
 * (SCIM requires it, but ThunderID doesn't require any particular attribute to fill that role,
 * so it doesn't stop a save).
 */
export default function validateScimMapping(
  map: Record<string, string>,
  meta: Record<string, ScimMultiValuedMeta>,
): ScimMappingValidationError | null {
  const mappedEntries = Object.entries(map).filter(([, target]) => target.trim().length > 0);

  const propertiesByTarget = new Map<string, string[]>();
  mappedEntries.forEach(([property, target]) => {
    propertiesByTarget.set(target, [...(propertiesByTarget.get(target) ?? []), property]);
  });

  // Multi-valued targets (emails/phoneNumbers/photos) may legitimately have several properties
  // mapped to them — each becomes its own array entry — so only single-valued targets with more
  // than one property are a real conflict.
  const duplicate = [...propertiesByTarget.entries()].find(
    ([target, properties]) => !MULTI_VALUED_TARGET_SET.has(target) && properties.length > 1,
  );
  if (duplicate) {
    return {type: 'duplicateTarget', target: duplicate[0], properties: duplicate[1]};
  }

  const multiplePrimary = [...propertiesByTarget.entries()]
    .filter(([target]) => MULTI_VALUED_TARGET_SET.has(target))
    .map(([target, properties]): [string, string[]] => [
      target,
      properties.filter((property) => meta[property]?.primary),
    ])
    .find(([, primaryProperties]) => primaryProperties.length > 1);
  if (multiplePrimary) {
    return {type: 'multiplePrimary', target: multiplePrimary[0], properties: multiplePrimary[1]};
  }

  return null;
}
