// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {
  applyGrantTypesChange,
  applyPublicClientChange,
  applyTokenEndpointAuthMethodChange,
  deriveOAuth2Flags,
  getPkceCaption,
  getPublicClientCaption,
  hasClientAccess,
  hasUserAccess,
  isGrantItemDisabled,
  isOAuthTokenMode,
} from '../oauth2Rules';
import {OAuth2GrantTypes, OAuth2ResponseTypes, TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import type {OAuth2Config} from '@thunderid/configure-applications';

const baseConfig = (overrides: Partial<OAuth2Config> = {}): OAuth2Config => ({
  grantTypes: [],
  responseTypes: [],
  ...overrides,
});

describe('deriveOAuth2Flags', () => {
  it('flags authorization_code grant presence', () => {
    const flags = deriveOAuth2Flags(baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE]}));
    expect(flags.hasAuthorizationCodeGrant).toBe(true);
    expect(flags.isPkceDisabledByGrants).toBe(false);
  });

  it('disables PKCE and public client when authorization_code is absent', () => {
    const flags = deriveOAuth2Flags(baseConfig({grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS]}));
    expect(flags.isPkceDisabledByGrants).toBe(true);
    expect(flags.isPublicClientDisabledByGrants).toBe(true);
  });

  it('disables PAR when authorization_code is absent', () => {
    expect(deriveOAuth2Flags(baseConfig({grantTypes: [OAuth2GrantTypes.TOKEN_EXCHANGE]})).isParDisabledByGrants).toBe(
      true,
    );
    expect(
      deriveOAuth2Flags(baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE]})).isParDisabledByGrants,
    ).toBe(false);
  });

  it('disables public client when client_credentials is present', () => {
    const flags = deriveOAuth2Flags(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.CLIENT_CREDENTIALS],
      }),
    );
    expect(flags.isPublicClientDisabledByGrants).toBe(true);
  });

  it('forces PKCE when public client is true', () => {
    const flags = deriveOAuth2Flags(baseConfig({publicClient: true}));
    expect(flags.isPkceForcedByPublicClient).toBe(true);
  });
});

describe('applyGrantTypesChange', () => {
  it('drops refresh_token if it would become the sole grant', () => {
    const updates = applyGrantTypesChange(baseConfig(), [OAuth2GrantTypes.REFRESH_TOKEN]);
    expect(updates.grantTypes).toEqual([]);
  });

  it('drops refresh_token when no token-issuing grant is present', () => {
    const updates = applyGrantTypesChange(baseConfig(), [
      OAuth2GrantTypes.CLIENT_CREDENTIALS,
      OAuth2GrantTypes.REFRESH_TOKEN,
    ]);
    expect(updates.grantTypes).toEqual([OAuth2GrantTypes.CLIENT_CREDENTIALS]);
  });

  it('allows refresh_token alongside authorization_code', () => {
    const updates = applyGrantTypesChange(baseConfig(), [
      OAuth2GrantTypes.AUTHORIZATION_CODE,
      OAuth2GrantTypes.REFRESH_TOKEN,
    ]);
    expect(updates.grantTypes).toEqual([OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN]);
  });

  it('allows refresh_token alongside ciba', () => {
    const updates = applyGrantTypesChange(baseConfig(), [OAuth2GrantTypes.CIBA, OAuth2GrantTypes.REFRESH_TOKEN]);
    expect(updates.grantTypes).toEqual([OAuth2GrantTypes.CIBA, OAuth2GrantTypes.REFRESH_TOKEN]);
  });

  it('turns off PKCE when authorization_code is removed', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        pkceRequired: true,
      }),
      [OAuth2GrantTypes.CLIENT_CREDENTIALS],
    );
    expect(updates.pkceRequired).toBe(false);
  });

  it('turns off PAR when authorization_code is removed', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        requirePushedAuthorizationRequests: true,
      }),
      [OAuth2GrantTypes.TOKEN_EXCHANGE],
    );
    expect(updates.requirePushedAuthorizationRequests).toBe(false);
  });

  it('keeps PAR when authorization_code remains', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        requirePushedAuthorizationRequests: true,
      }),
      [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.TOKEN_EXCHANGE],
    );
    expect(updates.requirePushedAuthorizationRequests).toBeUndefined();
  });

  it('turns off public client and reverts token method when grants become invalid', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        publicClient: true,
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.NONE,
      }),
      [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.CLIENT_CREDENTIALS],
    );
    expect(updates.publicClient).toBe(false);
    expect(updates.tokenEndpointAuthMethod).toBe(TokenEndpointAuthMethods.CLIENT_SECRET_BASIC);
  });

  it("adds 'code' response type when authorization_code is added", () => {
    const updates = applyGrantTypesChange(baseConfig({responseTypes: []}), [OAuth2GrantTypes.AUTHORIZATION_CODE]);
    expect(updates.responseTypes).toEqual([OAuth2ResponseTypes.CODE]);
  });

  it('clears response types when authorization_code leaves grants', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        responseTypes: [OAuth2ResponseTypes.CODE],
      }),
      [OAuth2GrantTypes.CLIENT_CREDENTIALS],
    );
    expect(updates.responseTypes).toEqual([]);
  });
});

describe('applyPublicClientChange', () => {
  it('forces token method to none and PKCE on when toggled on', () => {
    const updates = applyPublicClientChange(baseConfig(), true);
    expect(updates.tokenEndpointAuthMethod).toBe(TokenEndpointAuthMethods.NONE);
    expect(updates.pkceRequired).toBe(true);
  });

  it('reverts token method from none when toggled off', () => {
    const updates = applyPublicClientChange(
      baseConfig({tokenEndpointAuthMethod: TokenEndpointAuthMethods.NONE}),
      false,
    );
    expect(updates.tokenEndpointAuthMethod).toBe(TokenEndpointAuthMethods.CLIENT_SECRET_BASIC);
  });

  it('leaves token method untouched when toggled off and method is not none', () => {
    const updates = applyPublicClientChange(
      baseConfig({tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_POST}),
      false,
    );
    expect(updates.tokenEndpointAuthMethod).toBeUndefined();
  });
});

describe('applyTokenEndpointAuthMethodChange', () => {
  it('promotes to public client and forces PKCE when switching to none', () => {
    const updates = applyTokenEndpointAuthMethodChange(baseConfig(), TokenEndpointAuthMethods.NONE);
    expect(updates.publicClient).toBe(true);
    expect(updates.pkceRequired).toBe(true);
  });

  it('demotes public client when switching away from none', () => {
    const updates = applyTokenEndpointAuthMethodChange(
      baseConfig({publicClient: true}),
      TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
    );
    expect(updates.publicClient).toBe(false);
  });

  it('leaves public client alone when already confidential', () => {
    const updates = applyTokenEndpointAuthMethodChange(baseConfig(), TokenEndpointAuthMethods.CLIENT_SECRET_POST);
    expect(updates.publicClient).toBeUndefined();
  });

  it('clears certificate when switching away from private_key_jwt', () => {
    const updates = applyTokenEndpointAuthMethodChange(
      baseConfig({
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
        certificate: {type: 'JWKS_URI', value: 'https://example.com/jwks'},
      }),
      TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
    );
    expect(updates.certificate).toBeNull();
  });

  it('does not clear certificate when staying on private_key_jwt', () => {
    const updates = applyTokenEndpointAuthMethodChange(
      baseConfig({
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
        certificate: {type: 'JWKS_URI', value: 'https://example.com/jwks'},
      }),
      TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
    );
    expect(updates.certificate).toBeUndefined();
  });

  it('does not clear certificate when switching between non-private_key_jwt methods', () => {
    const updates = applyTokenEndpointAuthMethodChange(
      baseConfig({tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC}),
      TokenEndpointAuthMethods.CLIENT_SECRET_POST,
    );
    expect(updates.certificate).toBeUndefined();
  });
});

describe('isGrantItemDisabled', () => {
  it('disables refresh_token when no other grant is selected', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [])).toBe(true);
  });

  it('enables refresh_token once a token-issuing grant is selected', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(false);
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [OAuth2GrantTypes.CIBA])).toBe(false);
  });

  it('keeps refresh_token disabled when only a non-token-issuing grant is selected', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [OAuth2GrantTypes.CLIENT_CREDENTIALS])).toBe(true);
  });

  it('keeps refresh_token enabled when already selected so it can be unchecked', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [OAuth2GrantTypes.REFRESH_TOKEN])).toBe(false);
  });

  it('never disables non-refresh grants', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.AUTHORIZATION_CODE, [])).toBe(false);
    expect(isGrantItemDisabled(OAuth2GrantTypes.CLIENT_CREDENTIALS, [])).toBe(false);
  });
});

describe('getPublicClientCaption', () => {
  it('points to the cc-incompatible key when client_credentials is selected', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS]});
    const [key] = getPublicClientCaption(deriveOAuth2Flags(config), config);
    expect(key).toBe('applications:edit.advanced.publicClient.incompatibleWithClientCredentials');
  });

  it('points to the requires-authz-code key when authorization_code is absent', () => {
    const config = baseConfig({grantTypes: []});
    const [key] = getPublicClientCaption(deriveOAuth2Flags(config), config);
    expect(key).toBe('applications:edit.advanced.publicClient.requiresAuthorizationCode');
  });

  it('points to public/confidential keys in valid states', () => {
    const publicConfig = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], publicClient: true});
    expect(getPublicClientCaption(deriveOAuth2Flags(publicConfig), publicConfig)[0]).toBe(
      'applications:edit.advanced.publicClient.public',
    );
    const confidentialConfig = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], publicClient: false});
    expect(getPublicClientCaption(deriveOAuth2Flags(confidentialConfig), confidentialConfig)[0]).toBe(
      'applications:edit.advanced.publicClient.confidential',
    );
  });
});

describe('hasUserAccess', () => {
  it('is true for authorization_code, CIBA, and password grants', () => {
    expect(hasUserAccess([OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(true);
    expect(hasUserAccess([OAuth2GrantTypes.CIBA])).toBe(true);
    expect(hasUserAccess([OAuth2GrantTypes.PASSWORD])).toBe(true);
  });

  it('is false for client_credentials only and for no grants', () => {
    expect(hasUserAccess([OAuth2GrantTypes.CLIENT_CREDENTIALS])).toBe(false);
    expect(hasUserAccess([])).toBe(false);
    expect(hasUserAccess(undefined)).toBe(false);
  });
});

describe('hasClientAccess', () => {
  it('is true only when client_credentials is granted', () => {
    expect(hasClientAccess([OAuth2GrantTypes.CLIENT_CREDENTIALS])).toBe(true);
    expect(hasClientAccess([OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(false);
    expect(hasClientAccess(undefined)).toBe(false);
  });
});

describe('isOAuthTokenMode', () => {
  // Either block alone flips the flag, so each case is asserted with only one of them populated.
  const partialToken = (block: Record<string, unknown>): OAuth2Config['token'] =>
    block as unknown as OAuth2Config['token'];

  it('is true when the access token config is present', () => {
    expect(
      isOAuthTokenMode(baseConfig({token: partialToken({accessToken: {userConfig: {validityPeriod: 3600}}})})),
    ).toBe(true);
  });

  it('is true when only the ID token config is present', () => {
    expect(
      isOAuthTokenMode(baseConfig({token: partialToken({idToken: {validityPeriod: 3600, userAttributes: []}})})),
    ).toBe(true);
  });

  it('is false for an app-native application with no OAuth config', () => {
    expect(isOAuthTokenMode(undefined)).toBe(false);
  });

  it('is false when an OAuth config carries no token block', () => {
    expect(isOAuthTokenMode(baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE]}))).toBe(false);
  });
});

describe('getPkceCaption', () => {
  it('points to the required-for-public key when the client is public', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], publicClient: true});
    expect(getPkceCaption(deriveOAuth2Flags(config), config)[0]).toBe(
      'applications:edit.advanced.pkce.requiredForPublicClient',
    );
  });

  it('points to the requires-authz-code key when no authorization_code grant', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS]});
    expect(getPkceCaption(deriveOAuth2Flags(config), config)[0]).toBe(
      'applications:edit.advanced.pkce.requiresAuthorizationCode',
    );
  });

  it('points to enabled/disabled keys in the regular states', () => {
    const enabled = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], pkceRequired: true});
    expect(getPkceCaption(deriveOAuth2Flags(enabled), enabled)[0]).toBe('applications:edit.advanced.pkce.enabled');
    const disabled = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], pkceRequired: false});
    expect(getPkceCaption(deriveOAuth2Flags(disabled), disabled)[0]).toBe('applications:edit.advanced.pkce.disabled');
  });
});
