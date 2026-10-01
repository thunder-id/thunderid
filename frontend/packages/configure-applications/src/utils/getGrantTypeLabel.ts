// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '../models/oauth';
/**
 * Returns a human-readable label for a given OAuth2 grant type value.
 * For known grant types with long URN identifiers a friendly name is returned.
 * All other grant type values are returned unchanged.
 */
export function getGrantTypeLabel(grant: string, t: (key: string, fallback: string) => string): string {
  if (grant === OAuth2GrantTypes.CIBA) {
    return t('applications:edit.advanced.grantTypes.labels.ciba', 'CIBA (Client-Initiated Backchannel Authentication)');
  }
  if (grant === OAuth2GrantTypes.TOKEN_EXCHANGE) {
    return t('applications:edit.advanced.grantTypes.labels.tokenExchange', 'Token Exchange');
  }
  if (grant === OAuth2GrantTypes.JWT_BEARER) {
    return t('applications:edit.advanced.grantTypes.labels.jwtBearer', 'JWT Bearer');
  }
  return grant;
}
