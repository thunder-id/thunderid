// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  OAuth2GrantTypes,
  OAuth2ResponseTypes,
  REFRESH_TOKEN_ISSUING_GRANTS,
  TokenEndpointAuthMethods,
} from '../models/oauth';
import type {OAuth2Config} from '../models/oauth';
/**
 * Derived boolean flags describing the current OAuth2 configuration state.
 * Used by the OAuth2 config UI to drive toggle/picker disabled states and captions.
 */
export interface OAuth2Flags {
  hasAuthorizationCodeGrant: boolean;
  hasClientCredentialsGrant: boolean;
  hasJwtBearerGrant: boolean;
  isPublicClient: boolean;
  isPkceDisabledByGrants: boolean;
  isPkceForcedByPublicClient: boolean;
  isPublicClientDisabledByGrants: boolean;
  isPublicClientDisabledByIdJag: boolean;
  isParDisabledByGrants: boolean;
}

/**
 * Returns whether the application issues identity assertions (ID-JAG).
 */
export function isIdJagEnabled(config: OAuth2Config | undefined): boolean {
  return config?.token?.idJag?.enabled === true;
}

/**
 * Returns whether the runtime is known not to issue identity assertions (ID-JAG), because its
 * authorization server metadata does not list the token exchange grant they are issued through.
 * Returns false while the metadata is unknown (`grantTypesSupported` undefined: loading, failed,
 * or a document without the field), so nothing is blocked on missing information.
 */
export function isIdJagIssuanceUnavailable(grantTypesSupported: string[] | undefined): boolean {
  return Array.isArray(grantTypesSupported) && !grantTypesSupported.includes(OAuth2GrantTypes.TOKEN_EXCHANGE);
}

/**
 * Derives state flags from an OAuth2 configuration.
 */
export function deriveOAuth2Flags(config: OAuth2Config): OAuth2Flags {
  const grantTypes = config.grantTypes ?? [];
  const hasAuthorizationCodeGrant = grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const hasClientCredentialsGrant = grantTypes.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
  const hasJwtBearerGrant = grantTypes.includes(OAuth2GrantTypes.JWT_BEARER);
  const isPublicClient = config.publicClient === true;

  return {
    hasAuthorizationCodeGrant,
    hasClientCredentialsGrant,
    hasJwtBearerGrant,
    isPublicClient,
    isPkceDisabledByGrants: !hasAuthorizationCodeGrant,
    isPkceForcedByPublicClient: isPublicClient,
    // client_credentials and the JWT bearer grant both require a confidential client.
    isPublicClientDisabledByGrants: hasClientCredentialsGrant || hasJwtBearerGrant || !hasAuthorizationCodeGrant,
    // Issuing identity assertions (ID-JAG) requires a confidential client.
    isPublicClientDisabledByIdJag: isIdJagEnabled(config),
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
 * - public client is incompatible with client_credentials and jwt-bearer, and requires authorization_code
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
  const nextHasJwtBearer = nextGrantTypes.includes(OAuth2GrantTypes.JWT_BEARER);
  const currentResponseTypes = current.responseTypes ?? [];

  if (current.pkceRequired && !nextHasAuthzCode) {
    updates.pkceRequired = false;
  }

  if (current.requirePushedAuthorizationRequests && !nextHasAuthzCode) {
    updates.requirePushedAuthorizationRequests = false;
  }

  if (current.publicClient && (nextHasCC || nextHasJwtBearer || !nextHasAuthzCode)) {
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
 * Returns whether a selected grant is held in place by identity assertions (ID-JAG), which are
 * issued through the token exchange grant, so it cannot be removed while ID-JAG is on.
 */
export function isGrantLockedByIdJag(grant: string, currentGrants: string[], idJagEnabled: boolean): boolean {
  return idJagEnabled && grant === OAuth2GrantTypes.TOKEN_EXCHANGE && currentGrants.includes(grant);
}

/**
 * Returns whether a grant-type MenuItem should be disabled in the grants picker.
 * refresh_token requires a token-issuing grant, and token exchange cannot be removed while
 * identity assertions (ID-JAG) are on.
 */
export function isGrantItemDisabled(grant: string, currentGrants: string[], idJagEnabled = false): boolean {
  if (isGrantLockedByIdJag(grant, currentGrants, idJagEnabled)) return true;
  if (grant !== OAuth2GrantTypes.REFRESH_TOKEN) return false;
  if (currentGrants.includes(OAuth2GrantTypes.REFRESH_TOKEN)) return false;
  return !hasTokenIssuingGrant(currentGrants);
}

/** i18n key paired with its English fallback, suitable for spreading into `t(key, fallback)`. */
export type CaptionTuple = readonly [key: string, fallback: string];

/**
 * Grant types a client registered from a client ID metadata document can hold. Token exchange and JWT
 * bearer are among them only for a client that authenticates with private_key_jwt.
 */
export const CIMD_GRANT_TYPES: readonly string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.REFRESH_TOKEN,
  OAuth2GrantTypes.TOKEN_EXCHANGE,
  OAuth2GrantTypes.JWT_BEARER,
];

/**
 * Returns whether a client registered from a client ID metadata document may hold a grant when it
 * authenticates with the given method, following the server's document rules.
 */
export function isCimdGrantAllowed(grant: string, tokenEndpointAuthMethod: string | undefined): boolean {
  switch (grant) {
    case OAuth2GrantTypes.AUTHORIZATION_CODE:
    case OAuth2GrantTypes.REFRESH_TOKEN:
      return true;
    case OAuth2GrantTypes.TOKEN_EXCHANGE:
    case OAuth2GrantTypes.JWT_BEARER:
      return tokenEndpointAuthMethod === TokenEndpointAuthMethods.PRIVATE_KEY_JWT;
    default:
      return false;
  }
}

/**
 * Returns why a grant cannot be changed for a client registered from a client ID metadata document, or
 * undefined when it can. The authorization code grant is always required, token exchange and JWT bearer
 * need a client whose document declares private_key_jwt, and token exchange serves such a client only for
 * identity assertions, so it comes and goes with the ID-JAG toggle.
 */
export function getCimdGrantRestriction(grant: string, config: OAuth2Config): CaptionTuple | undefined {
  if (!config.clientIdMetadataDocument) return undefined;
  if (grant === OAuth2GrantTypes.AUTHORIZATION_CODE) {
    return [
      'applications:edit.advanced.grantTypes.requiredByMetadataDocument',
      'Required for a client registered from a metadata document',
    ];
  }
  if (!isCimdGrantAllowed(grant, config.tokenEndpointAuthMethod)) {
    return [
      'applications:edit.advanced.grantTypes.requiresPrivateKeyJwt',
      'Requires a metadata document that declares private key JWT authentication',
    ];
  }
  if (grant === OAuth2GrantTypes.TOKEN_EXCHANGE) {
    return [
      'applications:edit.advanced.grantTypes.followsIdJag',
      'Turned on and off with identity assertions (ID-JAG)',
    ];
  }
  return undefined;
}

/** Picks the public-client toggle caption for the current config state. */
export function getPublicClientCaption(flags: OAuth2Flags, config: OAuth2Config): CaptionTuple {
  if (config.clientIdMetadataDocument) {
    return ['applications:edit.advanced.publicClient.setByMetadataDocument', "Set by the client's metadata document."];
  }
  if (flags.hasClientCredentialsGrant) {
    return [
      'applications:edit.advanced.publicClient.incompatibleWithClientCredentials',
      'Not available for machine-to-machine clients.',
    ];
  }
  if (flags.hasJwtBearerGrant) {
    return [
      'applications:edit.advanced.publicClient.incompatibleWithJwtBearer',
      'Not available with the JWT bearer grant, which requires a confidential client.',
    ];
  }
  if (flags.isPublicClientDisabledByIdJag) {
    return [
      'applications:edit.advanced.publicClient.incompatibleWithIdJag',
      'Not available while identity assertions (ID-JAG) are on, since they require a confidential client.',
    ];
  }
  if (flags.isPublicClientDisabledByGrants) {
    return [
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
  if (config.clientIdMetadataDocument) {
    return [
      'applications:edit.advanced.pkce.requiredForMetadataDocument',
      'Always required for clients registered from a metadata document.',
    ];
  }
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
