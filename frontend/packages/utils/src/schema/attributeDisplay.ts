// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The display settings a top-level schema attribute can set.
 *
 * @public
 */
export interface AttributeDisplay {
  /** Position of the attribute, lowest first. */
  order?: number;
  /** Settings for when an account is created. */
  onCreate?: {
    /** Whether an optional attribute is left out when an account is created. */
    hidden?: boolean;
  };
}

function getDisplay(definition: object): AttributeDisplay | undefined {
  return 'display' in definition && typeof definition.display === 'object' && definition.display !== null
    ? (definition.display as AttributeDisplay)
    : undefined;
}

function isFlagSet(definition: object, flag: 'required' | 'credential'): boolean {
  return flag in definition && (definition as Record<string, unknown>)[flag] === true;
}

/**
 * The display order a schema attribute sets, or undefined when it sets none.
 *
 * @param definition - The attribute's schema definition
 * @returns The attribute's display order
 *
 * @public
 */
export function getDisplayOrder(definition: object): number | undefined {
  const order = getDisplay(definition)?.order;
  return typeof order === 'number' ? order : undefined;
}

/**
 * Whether an attribute is left out when an account is created. A required attribute never is, and an
 * optional one is when its schema sets `display.onCreate.hidden`.
 *
 * @param definition - The attribute's schema definition
 * @returns True when the attribute is left out on create
 *
 * @public
 */
export function isHiddenOnCreate(definition: object): boolean {
  return !isFlagSet(definition, 'required') && getDisplay(definition)?.onCreate?.hidden === true;
}

/** Group of an attribute without a display order: required, optional, then the credentials. */
function group(definition: object): number {
  const required = isFlagSet(definition, 'required');
  if (isFlagSet(definition, 'credential')) return required ? 2 : 3;
  return required ? 0 : 1;
}

function compareNames(a: string, b: string): number {
  if (a === b) return 0;
  return a < b ? -1 : 1;
}

/**
 * Compares two attributes the way ThunderID orders them everywhere. Attributes that set a display order
 * come first, lowest first. The others follow: required attributes, optional ones, then the required and
 * optional credentials. Attributes with the same order, or in the same group, follow attribute name.
 *
 * @param nameA - The first attribute's name
 * @param a - The first attribute's schema definition
 * @param nameB - The second attribute's name
 * @param b - The second attribute's schema definition
 * @returns A negative number, zero or a positive number, as a sort comparator expects
 *
 * @public
 */
export function compareAttributes(nameA: string, a: object, nameB: string, b: object): number {
  const orderA = getDisplayOrder(a);
  const orderB = getDisplayOrder(b);
  if (orderA !== undefined && orderB !== undefined && orderA !== orderB) return orderA - orderB;
  if (orderA !== undefined && orderB === undefined) return -1;
  if (orderA === undefined && orderB !== undefined) return 1;
  if (orderA === undefined) {
    const groupDiff = group(a) - group(b);
    if (groupDiff !== 0) return groupDiff;
  }
  return compareNames(nameA, nameB);
}
