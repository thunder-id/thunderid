// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CONTACT_ATTRIBUTE_TYPES, isContactAttributeType, type ContactAttributeType} from '@thunderid/utils';
import type {SchemaPropertyInput, SystemAttributes} from '../types/user-types';

/**
 * Keeps one primary per contact type among named attributes: the sole attribute is primary,
 * otherwise the first selected one is kept. Unnamed and non-contact properties are never primary.
 */
export function normalizePrimaryContacts(properties: SchemaPropertyInput[]): SchemaPropertyInput[] {
  return properties.map((property) => {
    const contact = isContactAttributeType(property.type) && Boolean(property.name.trim());
    const contacts = properties.filter((candidate) => candidate.type === property.type && candidate.name.trim());
    const primary =
      contact && (contacts.length === 1 || contacts.find((candidate) => candidate.primary)?.id === property.id);
    if (!contact && property.primary === undefined) return property;
    return {...property, primary};
  });
}

/** Contact types that have attributes but no named primary. Expects normalized properties. */
export function getMissingPrimaryContactTypes(properties: SchemaPropertyInput[]): ContactAttributeType[] {
  return CONTACT_ATTRIBUTE_TYPES.filter(
    (type) =>
      properties.some((property) => property.type === type && property.name.trim()) &&
      !properties.some((property) => property.type === type && property.primary && property.name.trim()),
  );
}

/** Primary contact system attributes. Expects normalized properties. */
export function primaryContactMappings(properties: SchemaPropertyInput[]): SystemAttributes {
  const email = properties.find((property) => property.type === 'email' && property.primary)?.name.trim();
  const mobile = properties.find((property) => property.type === 'mobile' && property.primary)?.name.trim();
  return {...(email ? {primaryEmail: email} : {}), ...(mobile ? {primaryMobile: mobile} : {})};
}
