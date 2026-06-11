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

import {describe, it, expect} from 'vitest';
import {OAuth2GrantTypes, OAuth2ResponseTypes, TokenEndpointAuthMethods, type OAuth2Config} from '../../models/oauth';
import {
  applyGrantTypesChange,
  applyPublicClientChange,
  applyTokenEndpointAuthMethodChange,
  deriveOAuth2Flags,
  getPkceCaption,
  getPublicClientCaption,
  isGrantItemDisabled,
} from '../oauth2Rules';

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

  it('allows refresh_token alongside another grant', () => {
    const updates = applyGrantTypesChange(baseConfig(), [
      OAuth2GrantTypes.AUTHORIZATION_CODE,
      OAuth2GrantTypes.REFRESH_TOKEN,
    ]);
    expect(updates.grantTypes).toEqual([OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN]);
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

  it('enables refresh_token once another grant is selected', () => {
    expect(isGrantItemDisabled(OAuth2GrantTypes.REFRESH_TOKEN, [OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(false);
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
