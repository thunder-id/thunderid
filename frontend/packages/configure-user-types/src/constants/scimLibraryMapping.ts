// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * SCIM target for each attribute of the global attribute list (the predefined library in
 * constants/attributes.ts plus mobile_number, which the default user types and flows use),
 * used to auto-map a user type's schema properties when SCIM mapping is enabled. Attributes with
 * no SCIM counterpart (birthdate, gender, website, the verified flags, active) are left out and
 * stay unmapped.
 */
const SCIM_LIBRARY_MAPPING: Record<string, string> = {
  username: 'userName',
  password: 'password',
  mobile_number: 'phoneNumbers',
  email: 'emails',
  given_name: 'name.givenName',
  family_name: 'name.familyName',
  display_name: 'displayName',
  name: 'name.formatted',
  middle_name: 'name.middleName',
  nickname: 'nickName',
  picture: 'photos',
  locale: 'locale',
  zoneinfo: 'timezone',
  preferred_language: 'preferredLanguage',
  profile: 'profileUrl',
  title: 'title',
  street_address: 'addresses.streetAddress',
  city: 'addresses.locality',
  region: 'addresses.region',
  postal_code: 'addresses.postalCode',
  country: 'addresses.country',
  employee_number: 'employeeNumber',
  department: 'department',
  division: 'division',
  organization: 'organization',
  cost_center: 'costCenter',
  manager: 'manager',
};

export default SCIM_LIBRARY_MAPPING;
