// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '../models/oauth';

/**
 * Reports whether the JWT bearer grant is the only configured grant type, optionally alongside
 * `refresh_token`. Such a client only redeems assertions (e.g. an ID-JAG) at the token endpoint on
 * behalf of a user and never signs users in itself.
 *
 * @param grantTypes - The OAuth2 grant types currently configured for the client
 * @returns Whether the grant set is JWT bearer only
 *
 * @example
 * ```ts
 * isJwtBearerOnlyGrantSet(['urn:ietf:params:oauth:grant-type:jwt-bearer']); // true
 * isJwtBearerOnlyGrantSet(['urn:ietf:params:oauth:grant-type:jwt-bearer', 'authorization_code']); // false
 * ```
 *
 * @public
 */
export default function isJwtBearerOnlyGrantSet(grantTypes: readonly string[] | undefined): boolean {
  const grants = grantTypes ?? [];

  return (
    grants.includes(OAuth2GrantTypes.JWT_BEARER) &&
    grants.every((grant) => grant === OAuth2GrantTypes.JWT_BEARER || grant === OAuth2GrantTypes.REFRESH_TOKEN)
  );
}
