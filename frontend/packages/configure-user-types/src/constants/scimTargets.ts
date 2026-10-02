// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Mirrors backend/internal/scim/discovery/schema_builder.go's rfcCoreUserAttributeTemplate()
// and rfcEnterpriseUserAttributeTemplate() (POC scope: flat only). Metadata-only sub-attributes
// (emails/phoneNumbers/photos' `.type` and `.primary`) are dropped from the mappable vocabulary —
// they are not normally sourced from a single user property — collapsing those three groups to
// one target each. `addresses` keeps its real data sub-fields but drops the same two metadata
// ones; `manager` keeps only its one mappable sub-attribute, so it collapses the same way.

export interface ScimTargetOption {
  value: string;
  label: string;
  group: string;
}

export const SCIM_TARGET_GROUP_ORDER = [
  'User Info',
  'Name',
  'Emails',
  'Phone Numbers',
  'Photos',
  'Address',
  'Enterprise',
] as const;

export const SCIM_CORE_TARGETS: ScimTargetOption[] = [
  {value: 'userName', label: 'userName', group: 'User Info'},
  {value: 'displayName', label: 'displayName', group: 'User Info'},
  {value: 'nickName', label: 'nickName', group: 'User Info'},
  {value: 'profileUrl', label: 'profileUrl', group: 'User Info'},
  {value: 'title', label: 'title', group: 'User Info'},
  {value: 'preferredLanguage', label: 'preferredLanguage', group: 'User Info'},
  {value: 'locale', label: 'locale', group: 'User Info'},
  {value: 'timezone', label: 'timezone', group: 'User Info'},
  {value: 'password', label: 'password', group: 'User Info'},
  {value: 'name.formatted', label: 'formatted', group: 'Name'},
  {value: 'name.givenName', label: 'givenName', group: 'Name'},
  {value: 'name.familyName', label: 'familyName', group: 'Name'},
  {value: 'name.middleName', label: 'middleName', group: 'Name'},
  {value: 'emails', label: 'emails', group: 'Emails'},
  {value: 'phoneNumbers', label: 'phoneNumbers', group: 'Phone Numbers'},
  {value: 'photos', label: 'photos', group: 'Photos'},
  {value: 'addresses.formatted', label: 'formatted', group: 'Address'},
  {value: 'addresses.streetAddress', label: 'streetAddress', group: 'Address'},
  {value: 'addresses.locality', label: 'locality', group: 'Address'},
  {value: 'addresses.region', label: 'region', group: 'Address'},
  {value: 'addresses.postalCode', label: 'postalCode', group: 'Address'},
  {value: 'addresses.country', label: 'country', group: 'Address'},
];

export const SCIM_ENTERPRISE_TARGETS: ScimTargetOption[] = [
  {value: 'employeeNumber', label: 'employeeNumber', group: 'Enterprise'},
  {value: 'costCenter', label: 'costCenter', group: 'Enterprise'},
  {value: 'organization', label: 'organization', group: 'Enterprise'},
  {value: 'division', label: 'division', group: 'Enterprise'},
  {value: 'department', label: 'department', group: 'Enterprise'},
  {value: 'manager', label: 'manager', group: 'Enterprise'},
];

export const SCIM_ENTERPRISE_EXTENSION_URN = 'urn:ietf:params:scim:schemas:extension:enterprise:2.0:User';

// The three targets that stay genuinely multi-valued: more than one ThunderID property can map
// to the same one (each becomes its own array entry), and each mapped property needs its own
// `type`/`primary` since ThunderID has no natural source for "which one is primary" or "is this
// a work or mobile number" — unlike addresses/name, which only ever have one meaningful entry.
export const SCIM_MULTI_VALUED_TARGETS = ['emails', 'phoneNumbers', 'photos'] as const;

// Canonical `type` values per RFC 7643's attribute descriptions in rfcCoreUserAttributeTemplate()
// (backend/internal/scim/discovery/schema_builder.go) — advisory, not enforced by the schema.
export const SCIM_MULTI_VALUED_TYPE_OPTIONS: Record<string, string[]> = {
  emails: ['work', 'home', 'other'],
  phoneNumbers: ['work', 'home', 'mobile', 'other'],
  photos: ['photo', 'thumbnail'],
};

// Mirrors backend/internal/scim/common/constants.go's ThunderIDURNPrefix/Version/UserURNResource
// and schema_shared.go's BuildSchemaURN — the custom extension schema a core user type's own
// unmapped attributes fall into (backend/internal/scim/discovery/schema_builder.go's
// mapUserTypeToSCIMSchema).
export function buildThunderIdExtensionUrn(userTypeName: string): string {
  return `urn:thunderid:params:scim:schemas:${userTypeName.toLowerCase()}:2.0:User`;
}
