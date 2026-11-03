// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {OAuth2GrantTypes, OAuth2ResponseTypes, TokenEndpointAuthMethods} from '../../models/oauth';
import type {OAuth2Config} from '../../models/oauth';
import {
  applyGrantTypesChange,
  applyPublicClientChange,
  applyTokenEndpointAuthMethodChange,
  CIMD_GRANT_TYPES,
  deriveOAuth2Flags,
  getCimdGrantRestriction,
  getPkceCaption,
  getPublicClientCaption,
  hasClientAccess,
  hasUserAccess,
  isGrantItemDisabled,
  isCimdGrantAllowed,
  isGrantLockedByIdJag,
  isIdJagEnabled,
  isIdJagIssuanceUnavailable,
  isOAuthTokenMode,
} from '../oauth2Rules';

const baseConfig = (overrides: Partial<OAuth2Config> = {}): OAuth2Config => ({
  grantTypes: [],
  responseTypes: [],
  ...overrides,
});

// Only the ID-JAG block matters to these rules, so the token config is asserted with just that populated.
const idJagToken = (enabled: boolean): OAuth2Config['token'] =>
  ({idJag: {enabled}}) as unknown as OAuth2Config['token'];

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

  it('disables public client when the JWT bearer grant is present', () => {
    const flags = deriveOAuth2Flags(
      baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER]}),
    );
    expect(flags.hasJwtBearerGrant).toBe(true);
    expect(flags.isPublicClientDisabledByGrants).toBe(true);
  });

  it('keeps public client available for an authorization_code-only client', () => {
    const flags = deriveOAuth2Flags(baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE]}));
    expect(flags.hasJwtBearerGrant).toBe(false);
    expect(flags.isPublicClientDisabledByGrants).toBe(false);
    expect(flags.isPublicClientDisabledByIdJag).toBe(false);
  });

  it('disables public client while ID-JAG is on', () => {
    const flags = deriveOAuth2Flags(
      baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], token: idJagToken(true)}),
    );
    expect(flags.isPublicClientDisabledByIdJag).toBe(true);
    expect(flags.isPublicClientDisabledByGrants).toBe(false);
  });

  it('keeps public client available once ID-JAG is off', () => {
    const flags = deriveOAuth2Flags(
      baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], token: idJagToken(false)}),
    );
    expect(flags.isPublicClientDisabledByIdJag).toBe(false);
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

  it('turns off public client and reverts token method when the JWT bearer grant is added', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        publicClient: true,
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.NONE,
      }),
      [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER],
    );
    expect(updates.publicClient).toBe(false);
    expect(updates.tokenEndpointAuthMethod).toBe(TokenEndpointAuthMethods.CLIENT_SECRET_BASIC);
  });

  it('keeps a non-none token method when the JWT bearer grant turns off public client', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        publicClient: true,
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
      }),
      [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER],
    );
    expect(updates.publicClient).toBe(false);
    expect(updates.tokenEndpointAuthMethod).toBeUndefined();
  });

  it('leaves a confidential client untouched when the JWT bearer grant is added', () => {
    const updates = applyGrantTypesChange(
      baseConfig({
        grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
        publicClient: false,
        tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
      }),
      [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER],
    );
    expect(updates.publicClient).toBeUndefined();
    expect(updates.tokenEndpointAuthMethod).toBeUndefined();
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

  it('disables a selected token exchange grant while ID-JAG is on', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.TOKEN_EXCHANGE, [OAuth2GrantTypes.TOKEN_EXCHANGE], true)).toBe(true);
  });

  it('keeps token exchange enabled once ID-JAG is off', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.TOKEN_EXCHANGE, [OAuth2GrantTypes.TOKEN_EXCHANGE], false)).toBe(false);
    expect(isGrantItemDisabled(OAuth2GrantTypes.TOKEN_EXCHANGE, [OAuth2GrantTypes.TOKEN_EXCHANGE])).toBe(false);
  });

  it('keeps an unselected token exchange grant enabled while ID-JAG is on, so it can be added', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.TOKEN_EXCHANGE, [], true)).toBe(false);
  });

  it('does not disable other grants while ID-JAG is on', () => {
    expect(
      isGrantItemDisabled(
        OAuth2GrantTypes.JWT_BEARER,
        [OAuth2GrantTypes.TOKEN_EXCHANGE, OAuth2GrantTypes.JWT_BEARER],
        true,
      ),
    ).toBe(false);
  });
});

describe('isGrantLockedByIdJag', () => {
  it('locks only a selected token exchange grant while ID-JAG is on', () => {
    expect(isGrantLockedByIdJag(OAuth2GrantTypes.TOKEN_EXCHANGE, [OAuth2GrantTypes.TOKEN_EXCHANGE], true)).toBe(true);
    expect(isGrantLockedByIdJag(OAuth2GrantTypes.TOKEN_EXCHANGE, [OAuth2GrantTypes.TOKEN_EXCHANGE], false)).toBe(false);
    expect(isGrantLockedByIdJag(OAuth2GrantTypes.TOKEN_EXCHANGE, [], true)).toBe(false);
    expect(isGrantLockedByIdJag(OAuth2GrantTypes.AUTHORIZATION_CODE, [OAuth2GrantTypes.AUTHORIZATION_CODE], true)).toBe(
      false,
    );
  });
});

describe('isIdJagEnabled', () => {
  it('is true only when the ID-JAG config is enabled', () => {
    expect(isIdJagEnabled(baseConfig({token: idJagToken(true)}))).toBe(true);
    expect(isIdJagEnabled(baseConfig({token: idJagToken(false)}))).toBe(false);
    expect(isIdJagEnabled(baseConfig())).toBe(false);
    expect(isIdJagEnabled(undefined)).toBe(false);
  });
});

describe('isIdJagIssuanceUnavailable', () => {
  it('should be true when discovery lists grant types without token exchange', () => {
    expect(isIdJagIssuanceUnavailable([OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER])).toBe(true);
  });

  it('should be true when discovery lists no grant types', () => {
    expect(isIdJagIssuanceUnavailable([])).toBe(true);
  });

  it('should be false when discovery lists the token exchange grant', () => {
    expect(isIdJagIssuanceUnavailable([OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.TOKEN_EXCHANGE])).toBe(
      false,
    );
  });

  it('should be false when discovery has not loaded', () => {
    expect(isIdJagIssuanceUnavailable(undefined)).toBe(false);
  });
});

describe('getPublicClientCaption', () => {
  it('points to the cc-incompatible key when client_credentials is selected', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS]});
    const [key] = getPublicClientCaption(deriveOAuth2Flags(config), config);
    expect(key).toBe('applications:edit.advanced.publicClient.incompatibleWithClientCredentials');
  });

  it('points to the JWT bearer key when the JWT bearer grant is selected', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER]});
    const [key, fallback] = getPublicClientCaption(deriveOAuth2Flags(config), config);
    expect(key).toBe('applications:edit.advanced.publicClient.incompatibleWithJwtBearer');
    expect(fallback).toBe('Not available with the JWT bearer grant, which requires a confidential client.');
  });

  it('points to the JWT bearer key for a JWT bearer-only client', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.JWT_BEARER]});
    expect(getPublicClientCaption(deriveOAuth2Flags(config), config)[0]).toBe(
      'applications:edit.advanced.publicClient.incompatibleWithJwtBearer',
    );
  });

  it('keeps the cc-incompatible key when both client_credentials and JWT bearer are selected', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS, OAuth2GrantTypes.JWT_BEARER]});
    expect(getPublicClientCaption(deriveOAuth2Flags(config), config)[0]).toBe(
      'applications:edit.advanced.publicClient.incompatibleWithClientCredentials',
    );
  });

  it('points to the ID-JAG key while ID-JAG is on', () => {
    const config = baseConfig({grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE], token: idJagToken(true)});
    const [key, fallback] = getPublicClientCaption(deriveOAuth2Flags(config), config);
    expect(key).toBe('applications:edit.advanced.publicClient.incompatibleWithIdJag');
    expect(fallback).toBe(
      'Not available while identity assertions (ID-JAG) are on, since they require a confidential client.',
    );
  });

  it('prefers the JWT bearer key over the ID-JAG key when both apply', () => {
    const config = baseConfig({
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER],
      token: idJagToken(true),
    });
    expect(getPublicClientCaption(deriveOAuth2Flags(config), config)[0]).toBe(
      'applications:edit.advanced.publicClient.incompatibleWithJwtBearer',
    );
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

describe('isCimdGrantAllowed', () => {
  const {NONE, PRIVATE_KEY_JWT, CLIENT_SECRET_BASIC} = TokenEndpointAuthMethods;
  it.each([
    [OAuth2GrantTypes.AUTHORIZATION_CODE, true, true, true],
    [OAuth2GrantTypes.REFRESH_TOKEN, true, true, true],
    [OAuth2GrantTypes.TOKEN_EXCHANGE, false, true, false],
    [OAuth2GrantTypes.JWT_BEARER, false, true, false],
    [OAuth2GrantTypes.CLIENT_CREDENTIALS, false, false, false],
    [OAuth2GrantTypes.CIBA, false, false, false],
    [OAuth2GrantTypes.PASSWORD, false, false, false],
  ])('%s: none=%s, private_key_jwt=%s, client_secret_basic=%s', (grant, none, privateKeyJwt, secretBasic) => {
    expect(isCimdGrantAllowed(grant, NONE)).toBe(none);
    expect(isCimdGrantAllowed(grant, PRIVATE_KEY_JWT)).toBe(privateKeyJwt);
    expect(isCimdGrantAllowed(grant, CLIENT_SECRET_BASIC)).toBe(secretBasic);
  });

  it('lists exactly the grants a private key JWT client may hold', () => {
    expect(CIMD_GRANT_TYPES.every((grant) => isCimdGrantAllowed(grant, PRIVATE_KEY_JWT))).toBe(true);
    expect(CIMD_GRANT_TYPES).toHaveLength(4);
  });
});

describe('getCimdGrantRestriction', () => {
  const cimdConfig = (tokenEndpointAuthMethod: string): OAuth2Config =>
    baseConfig({clientIdMetadataDocument: true, tokenEndpointAuthMethod});

  it('returns nothing for a client that is not registered from a metadata document', () => {
    expect(getCimdGrantRestriction(OAuth2GrantTypes.AUTHORIZATION_CODE, baseConfig())).toBeUndefined();
    expect(getCimdGrantRestriction(OAuth2GrantTypes.JWT_BEARER, baseConfig())).toBeUndefined();
  });

  it('keeps the authorization code grant for every metadata document client', () => {
    for (const method of [TokenEndpointAuthMethods.NONE, TokenEndpointAuthMethods.PRIVATE_KEY_JWT]) {
      expect(getCimdGrantRestriction(OAuth2GrantTypes.AUTHORIZATION_CODE, cimdConfig(method))?.[0]).toBe(
        'applications:edit.advanced.grantTypes.requiredByMetadataDocument',
      );
    }
  });

  it('restricts the identity assertion grants to private key JWT clients', () => {
    const publicClient = cimdConfig(TokenEndpointAuthMethods.NONE);
    for (const grant of [OAuth2GrantTypes.TOKEN_EXCHANGE, OAuth2GrantTypes.JWT_BEARER]) {
      expect(getCimdGrantRestriction(grant, publicClient)?.[0]).toBe(
        'applications:edit.advanced.grantTypes.requiresPrivateKeyJwt',
      );
    }
    expect(getCimdGrantRestriction(OAuth2GrantTypes.REFRESH_TOKEN, publicClient)).toBeUndefined();
  });

  it('lets a private key JWT client pick JWT bearer but ties token exchange to identity assertions', () => {
    const confidentialClient = cimdConfig(TokenEndpointAuthMethods.PRIVATE_KEY_JWT);
    expect(getCimdGrantRestriction(OAuth2GrantTypes.JWT_BEARER, confidentialClient)).toBeUndefined();
    expect(getCimdGrantRestriction(OAuth2GrantTypes.TOKEN_EXCHANGE, confidentialClient)?.[0]).toBe(
      'applications:edit.advanced.grantTypes.followsIdJag',
    );
  });
});

describe('metadata document captions', () => {
  it('attributes the public client and PKCE settings to the metadata document', () => {
    const config = baseConfig({
      clientIdMetadataDocument: true,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.JWT_BEARER],
      tokenEndpointAuthMethod: TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
    });
    const flags = deriveOAuth2Flags(config);
    expect(getPublicClientCaption(flags, config)[0]).toBe(
      'applications:edit.advanced.publicClient.setByMetadataDocument',
    );
    expect(getPkceCaption(flags, config)[0]).toBe('applications:edit.advanced.pkce.requiredForMetadataDocument');
  });
});
