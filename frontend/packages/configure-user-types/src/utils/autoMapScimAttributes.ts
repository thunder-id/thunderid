// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import SCIM_LIBRARY_MAPPING from '../constants/scimLibraryMapping';
import {SCIM_MULTI_VALUED_TARGETS} from '../constants/scimTargets';
import type {SchemaPropertyInput} from '../types/user-types';

const MAPPABLE_TYPES = new Set(['string', 'number', 'boolean', 'enum']);
const MULTI_VALUED_TARGET_SET = new Set<string>(SCIM_MULTI_VALUED_TARGETS);

/**
 * Maps schema properties that came from the predefined attribute library to their SCIM targets
 * (for example `given_name` to `name.givenName`). Existing mappings are never changed: only
 * unmapped scalar properties are matched, and a single-valued target that is already taken is
 * skipped. A credential property maps to the write-only SCIM `password`.
 */
export default function autoMapScimAttributes(
  properties: SchemaPropertyInput[],
  existing: Record<string, string>,
): Record<string, string> {
  const next = {...existing};
  const usedTargets = new Set(Object.values(existing).filter(Boolean));

  properties.forEach((property) => {
    const propertyName = property.name.trim();
    const target = SCIM_LIBRARY_MAPPING[propertyName];
    if (!target || next[propertyName] || !MAPPABLE_TYPES.has(property.type)) return;
    if (!MULTI_VALUED_TARGET_SET.has(target) && usedTargets.has(target)) return;

    next[propertyName] = target;
    usedTargets.add(target);
  });

  return next;
}
