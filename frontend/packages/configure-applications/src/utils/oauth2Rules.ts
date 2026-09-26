// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes, OAuth2ResponseTypes, REFRESH_TOKEN_ISSUING_GRANTS, TokenEndpointAuthMethods} from '../models/oauth';
import type {OAuth2Config} from '../models/oauth';
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
  isParDisabledByGrants: boolean;
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
    // PAR is an authorization-endpoint feature, so it only applies when the authorization code grant
    // is present (e.g. it has no effect on the token exchange or client credentials grants).
    isParDisabledByGrants: !hasAuthorizationCodeGrant,
  };
}

/**
 * Grant types that authenticate an end user, unlocking the user-facing tabs (User token, Flows,
 * Customization, and the general Access section).
 */
export const USER_ACCESS_GRANTS: readonly string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.CIBA,
  OAuth2GrantTypes.PASSWORD,
];

/**
 * Returns whether any granted flow acts on behalf of an end user.
 */
export function hasUserAccess(grantTypes: string[] | undefined): boolean {
  return (grantTypes ?? []).some((grant) => USER_ACCESS_GRANTS.includes(grant));
}

/**
 * Returns whether the client can obtain tokens as its own subject (client_credentials grant).
 */
export function hasClientAccess(grantTypes: string[] | undefined): boolean {
  return (grantTypes ?? []).includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
}

/**
 * Returns whether the application's tokens are governed by its OAuth2 token config rather than by
 * its assertion config. An application with no OAuth2 configuration (app-native sign-in) receives
 * only the flow assertion, whose validity and user attributes come from `application.assertion`.
 *
 * The backend always materializes `token.accessToken` and `token.idToken` whenever an OAuth profile
 * exists, so this is equivalent to "has an OAuth profile" for any config loaded from the API.
 */
export function isOAuthTokenMode(oauth2Config: OAuth2Config | undefined): boolean {
  return oauth2Config?.token?.accessToken !== undefined || oauth2Config?.token?.idToken !== undefined;
}

/**
 * Returns whether the grant list contains a token-issuing grant.
 */
function hasTokenIssuingGrant(grants: string[]): boolean {
  return grants.some((grant) => REFRESH_TOKEN_ISSUING_GRANTS.includes(grant));
}

/**
 * Computes the set of config updates triggered by a grant-types selection change.
 * Enforces cross-field invariants:
 * - refresh_token requires a token-issuing grant (authorization_code or ciba)
 * - PKCE requires authorization_code
 * - PAR (requirePushedAuthorizationRequests) requires authorization_code
 * - public client is incompatible with client_credentials and requires authorization_code
 * - response type 'code' is added/removed alongside the authorization_code grant
 */
export function applyGrantTypesChange(current: OAuth2Config, selected: string[]): Partial<OAuth2Config> {
  let nextGrantTypes = selected;
  if (nextGrantTypes.includes(OAuth2GrantTypes.REFRESH_TOKEN) && !hasTokenIssuingGrant(nextGrantTypes)) {
    nextGrantTypes = nextGrantTypes.filter((g) => g !== OAuth2GrantTypes.REFRESH_TOKEN);
  }

  const updates: Partial<OAuth2Config> = {grantTypes: nextGrantTypes};
  const nextHasAuthzCode = nextGrantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const nextHasCC = nextGrantTypes.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
  const currentResponseTypes = current.responseTypes ?? [];

  if (current.pkceRequired && !nextHasAuthzCode) {
    updates.pkceRequired = false;
  }

  if (current.requirePushedAuthorizationRequests && !nextHasAuthzCode) {
    updates.requirePushedAuthorizationRequests = false;
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
 * refresh_token requires a token-issuing grant.
 */
export function isGrantItemDisabled(grant: string, currentGrants: string[]): boolean {
  if (grant !== OAuth2GrantTypes.REFRESH_TOKEN) return false;
  if (currentGrants.includes(OAuth2GrantTypes.REFRESH_TOKEN)) return false;
  return !hasTokenIssuingGrant(currentGrants);
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
