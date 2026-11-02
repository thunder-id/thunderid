// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EMAIL_REGEX} from './emailRegex';

/**
 * Default regex patterns for contact attribute types.
 * Keep in sync with `backend/internal/entitytype/model/string.go`.
 */
export const CONTACT_ATTRIBUTE_REGEX = {
  email: EMAIL_REGEX.source,
  mobile: '^\\+[1-9]\\d{1,14}$',
} as const;

export type ContactAttributeType = keyof typeof CONTACT_ATTRIBUTE_REGEX;

export const CONTACT_ATTRIBUTE_TYPES = Object.keys(CONTACT_ATTRIBUTE_REGEX) as ContactAttributeType[];

/** Whether the attribute type is a contact type (`email` or `mobile`). */
export function isContactAttributeType(type: unknown): type is ContactAttributeType {
  return CONTACT_ATTRIBUTE_TYPES.some((contactType) => contactType === type);
}

/** Contact types that have a named attribute without a regex. */
export function getContactTypesMissingRegex(
  properties: {name: string; type: unknown; regex: string}[],
): ContactAttributeType[] {
  return CONTACT_ATTRIBUTE_TYPES.filter((type) =>
    properties.some((property) => property.name.trim() && property.type === type && !property.regex.trim()),
  );
}

/** The saved regex, or the default regex of a contact attribute type when none is saved. */
export function resolveAttributeRegex(type: unknown, regex?: string): string {
  if (regex) return regex;
  return isContactAttributeType(type) ? CONTACT_ATTRIBUTE_REGEX[type] : '';
}
