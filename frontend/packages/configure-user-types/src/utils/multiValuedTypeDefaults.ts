// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SCIM_MULTI_VALUED_TYPE_OPTIONS} from '../constants/scimTargets';
import type {ScimMultiValuedMeta} from '../types/user-types';

/**
 * Types already taken by the other properties mapped to the same multi-valued SCIM target.
 */
export function getUsedMultiValuedTypes(
  target: string,
  propertyName: string,
  mapping: Record<string, string>,
  meta: Record<string, ScimMultiValuedMeta>,
): Set<string> {
  const used = new Set<string>();
  Object.entries(mapping).forEach(([name, mappedTarget]) => {
    const type = meta[name]?.type;
    if (name !== propertyName && mappedTarget === target && type) {
      used.add(type);
    }
  });
  return used;
}

/**
 * Picks the first predefined type of a multi-valued SCIM target that no other property mapped to it
 * uses, or an empty string when the target has no predefined types or all of them are taken.
 */
export function pickDefaultMultiValuedType(
  target: string,
  propertyName: string,
  mapping: Record<string, string>,
  meta: Record<string, ScimMultiValuedMeta>,
): string {
  const used = getUsedMultiValuedTypes(target, propertyName, mapping, meta);
  return (SCIM_MULTI_VALUED_TYPE_OPTIONS[target] ?? []).find((option) => !used.has(option)) ?? '';
}

/**
 * Gives each of the named properties that is mapped to a multi-valued SCIM target a default type,
 * in order, so that properties sharing a target receive different types. Properties that already
 * have metadata are left as they are.
 */
export function addDefaultMultiValuedTypes(
  mapping: Record<string, string>,
  meta: Record<string, ScimMultiValuedMeta>,
  propertyNames: string[],
): Record<string, ScimMultiValuedMeta> {
  const next = {...meta};
  propertyNames.forEach((propertyName) => {
    const target = mapping[propertyName];
    if (!target || next[propertyName]) return;
    const type = pickDefaultMultiValuedType(target, propertyName, mapping, next);
    if (type) {
      next[propertyName] = {type, primary: false};
    }
  });
  return next;
}
