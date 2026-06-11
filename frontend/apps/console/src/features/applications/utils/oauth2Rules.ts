/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {OAuth2GrantTypes, OAuth2ResponseTypes, TokenEndpointAuthMethods, type OAuth2Config} from '../models/oauth';

/**
 * Derived boolean flags describing the current OAuth2 configuration state.
 * Used by the OAuth2 config UI to drive toggle/picker disabled states and captions.
 */
export interface OAuth2Flags {
  hasAuthorizationCodeGrant: boolean;
  hasClientCredentialsGrant: boolean;
  isPublicClient: boolean;
  isPkceDisabledByGrants: boolean;
  isPkceForcedByPublicClient: boolean;
  isPublicClientDisabledByGrants: boolean;
}

/**
 * Derives state flags from an OAuth2 configuration.
 */
export function deriveOAuth2Flags(config: OAuth2Config): OAuth2Flags {
  const grantTypes = config.grantTypes ?? [];
  const hasAuthorizationCodeGrant = grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const hasClientCredentialsGrant = grantTypes.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
  const isPublicClient = config.publicClient === true;

  return {
    hasAuthorizationCodeGrant,
    hasClientCredentialsGrant,
    isPublicClient,
    isPkceDisabledByGrants: !hasAuthorizationCodeGrant,
    isPkceForcedByPublicClient: isPublicClient,
    isPublicClientDisabledByGrants: hasClientCredentialsGrant || !hasAuthorizationCodeGrant,
  };
}

/**
 * Computes the set of config updates triggered by a grant-types selection change.
 * Enforces cross-field invariants:
 * - refresh_token cannot be the sole grant
 * - PKCE requires authorization_code
 * - public client is incompatible with client_credentials and requires authorization_code
 * - response type 'code' is added/removed alongside the authorization_code grant
 */
export function applyGrantTypesChange(current: OAuth2Config, selected: string[]): Partial<OAuth2Config> {
  let nextGrantTypes = selected;
  if (nextGrantTypes.length === 1 && nextGrantTypes[0] === OAuth2GrantTypes.REFRESH_TOKEN) {
    nextGrantTypes = [];
  }

  const updates: Partial<OAuth2Config> = {grantTypes: nextGrantTypes};
  const nextHasAuthzCode = nextGrantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const nextHasCC = nextGrantTypes.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
  const currentResponseTypes = current.responseTypes ?? [];

  if (current.pkceRequired && !nextHasAuthzCode) {
    updates.pkceRequired = false;
  }

  if (current.publicClient && (nextHasCC || !nextHasAuthzCode)) {
    updates.publicClient = false;
    if (current.tokenEndpointAuthMethod === TokenEndpointAuthMethods.NONE) {
      updates.tokenEndpointAuthMethod = TokenEndpointAuthMethods.CLIENT_SECRET_BASIC;
    }
  }

  if (nextHasAuthzCode && !currentResponseTypes.includes(OAuth2ResponseTypes.CODE)) {
    updates.responseTypes = [...currentResponseTypes, OAuth2ResponseTypes.CODE];
  } else if (!nextHasAuthzCode && currentResponseTypes.length > 0) {
    updates.responseTypes = [];
  }

  return updates;
}

/**
 * Computes the set of config updates triggered by toggling the public client switch.
 * Turning on public client forces tokenEndpointAuthMethod='none' and pkceRequired=true.
 * Turning it off restores tokenEndpointAuthMethod to client_secret_basic if it was 'none'.
 */
export function applyPublicClientChange(current: OAuth2Config, checked: boolean): Partial<OAuth2Config> {
  const updates: Partial<OAuth2Config> = {publicClient: checked};
  if (checked) {
    updates.tokenEndpointAuthMethod = TokenEndpointAuthMethods.NONE;
    updates.pkceRequired = true;
  } else if (current.tokenEndpointAuthMethod === TokenEndpointAuthMethods.NONE) {
    updates.tokenEndpointAuthMethod = TokenEndpointAuthMethods.CLIENT_SECRET_BASIC;
  }
  return updates;
}

/**
 * Computes the set of config updates triggered by changing the token endpoint auth method.
 * Selecting 'none' promotes the client to public and forces PKCE on; switching away
 * from 'none' demotes it to confidential.
 * Switching away from 'private_key_jwt' clears the certificate since the cert is only
 * valid for that auth method in the current console configuration.
 */
export function applyTokenEndpointAuthMethodChange(current: OAuth2Config, method: string): Partial<OAuth2Config> {
  const updates: Partial<OAuth2Config> = {tokenEndpointAuthMethod: method};
  if (method === TokenEndpointAuthMethods.NONE) {
    updates.publicClient = true;
    updates.pkceRequired = true;
  } else if (current.publicClient) {
    updates.publicClient = false;
  }
  if (
    current.tokenEndpointAuthMethod === TokenEndpointAuthMethods.PRIVATE_KEY_JWT &&
    method !== TokenEndpointAuthMethods.PRIVATE_KEY_JWT
  ) {
    updates.certificate = null;
  }
  return updates;
}

/**
 * Returns whether a grant-type MenuItem should be disabled in the grants picker.
 * refresh_token cannot be picked as the first grant since it has no companion yet.
 */
export function isGrantItemDisabled(grant: string, currentGrants: string[]): boolean {
  if (grant !== OAuth2GrantTypes.REFRESH_TOKEN) return false;
  if (currentGrants.includes(OAuth2GrantTypes.REFRESH_TOKEN)) return false;
  return currentGrants.length === 0;
}

/** i18n key paired with its English fallback, suitable for spreading into `t(key, fallback)`. */
export type CaptionTuple = readonly [key: string, fallback: string];

/** Picks the public-client toggle caption for the current config state. */
export function getPublicClientCaption(flags: OAuth2Flags, config: OAuth2Config): CaptionTuple {
  if (flags.isPublicClientDisabledByGrants) {
    return flags.hasClientCredentialsGrant
      ? [
          'applications:edit.advanced.publicClient.incompatibleWithClientCredentials',
          'Not available for machine-to-machine clients.',
        ]
      : [
          'applications:edit.advanced.publicClient.requiresAuthorizationCode',
          'Available only for clients using the authorization code flow.',
        ];
  }
  return config.publicClient
    ? ['applications:edit.advanced.publicClient.public', '']
    : ['applications:edit.advanced.publicClient.confidential', ''];
}

/** Picks the PKCE toggle caption for the current config state. */
export function getPkceCaption(flags: OAuth2Flags, config: OAuth2Config): CaptionTuple {
  if (flags.isPkceForcedByPublicClient) {
    return ['applications:edit.advanced.pkce.requiredForPublicClient', 'Always required for public clients.'];
  }
  if (flags.isPkceDisabledByGrants) {
    return [
      'applications:edit.advanced.pkce.requiresAuthorizationCode',
      'PKCE applies only to the authorization code flow.',
    ];
  }
  return config.pkceRequired
    ? ['applications:edit.advanced.pkce.enabled', '']
    : ['applications:edit.advanced.pkce.disabled', ''];
}
