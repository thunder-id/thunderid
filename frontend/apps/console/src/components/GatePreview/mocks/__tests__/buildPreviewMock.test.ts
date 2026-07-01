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
import buildPreviewMock from '../buildPreviewMock';
import {AuthenticatorTypes} from '@/features/connections/models/authenticators';
import {IdentityProviderTypes} from '@/features/connections/models/identity-provider';

type MockComponent = Record<string, unknown>;

const getComponentById = (components: MockComponent[], id: string): MockComponent | undefined =>
  components.find((c) => c.id === id);

describe('buildPreviewMock', () => {
  describe('Always-present components', () => {
    it('should always include an app_logo component', () => {
      const result = buildPreviewMock() as unknown as MockComponent[];

      const logo = getComponentById(result, 'app_logo');
      expect(logo).toBeDefined();
      expect(logo!.type).toBe('IMAGE');
      expect(logo!.category).toBe('DISPLAY');
    });

    it('should always include a text_heading component', () => {
      const result = buildPreviewMock() as unknown as MockComponent[];

      const heading = getComponentById(result, 'text_heading');
      expect(heading).toBeDefined();
      expect(heading!.type).toBe('TEXT');
      expect(heading!.variant).toBe('HEADING_1');
    });

    it('should return at least 2 components with empty integrations', () => {
      const result = buildPreviewMock({}, [], {}) as unknown as MockComponent[];

      expect(result.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('App logo meta', () => {
    it('should use application logoUrl from meta when provided', () => {
      const meta = {application: {logoUrl: 'https://example.com/logo.png'}};
      const result = buildPreviewMock({}, [], meta) as unknown as MockComponent[];

      const logo = getComponentById(result, 'app_logo');
      expect(logo!.src).toBe('https://example.com/logo.png');
    });

    it('should use empty string for app logo src when meta has no logoUrl', () => {
      const result = buildPreviewMock({}, [], {}) as unknown as MockComponent[];

      const logo = getComponentById(result, 'app_logo');
      expect(logo!.src).toBe('');
    });
  });

  describe('Basic auth block', () => {
    it('should include basic auth block when basic_auth integration is enabled', () => {
      const integrations = {[AuthenticatorTypes.CREDENTIALS_AUTH]: true};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      const block = getComponentById(result, 'block_credentials_auth');
      expect(block).toBeDefined();
      expect(block!.type).toBe('BLOCK');
    });

    it('should not include basic auth block when basic_auth integration is disabled', () => {
      const integrations = {[AuthenticatorTypes.CREDENTIALS_AUTH]: false};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      const block = getComponentById(result, 'block_credentials_auth');
      expect(block).toBeUndefined();
    });

    it('should not include basic auth block when no integrations are provided', () => {
      const result = buildPreviewMock({}, []) as unknown as MockComponent[];

      const block = getComponentById(result, 'block_credentials_auth');
      expect(block).toBeUndefined();
    });

    it('basic auth block should contain username and password field components', () => {
      const integrations = {[AuthenticatorTypes.CREDENTIALS_AUTH]: true};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      const block = getComponentById(result, 'block_credentials_auth')!;
      const subComponents = block.components as MockComponent[];

      const usernameField = subComponents.find((c) => c.id === 'text_input_username');
      const passwordField = subComponents.find((c) => c.id === 'password_input');
      const submitAction = subComponents.find((c) => c.id === 'action_submit');

      expect(usernameField).toBeDefined();
      expect(passwordField).toBeDefined();
      expect(submitAction).toBeDefined();
    });
  });

  describe('Passkey component', () => {
    it('should include passkey action when passkey integration is enabled', () => {
      const integrations = {[AuthenticatorTypes.PASSKEY]: true};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      const passkey = getComponentById(result, 'action_passkey');
      expect(passkey).toBeDefined();
      expect(passkey!.label).toBe('{{t(signin:passkey.button.use)}}');
      expect(passkey!.variant).toBe('SOCIAL');
    });

    it('should not include passkey action when passkey integration is disabled', () => {
      const integrations = {[AuthenticatorTypes.PASSKEY]: false};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      const passkey = getComponentById(result, 'action_passkey');
      expect(passkey).toBeUndefined();
    });
  });

  describe('Social provider blocks', () => {
    it('should include a social block for each selected identity provider', () => {
      const integrations = {google: true, github: true};
      const providers = [
        {id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE},
        {id: 'github', name: 'GitHub', type: IdentityProviderTypes.GITHUB},
      ];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      expect(getComponentById(result, 'block_google')).toBeDefined();
      expect(getComponentById(result, 'block_github')).toBeDefined();
    });

    it('should only include social blocks for providers that have a truthy integration flag', () => {
      const integrations = {google: true, github: false};
      const providers = [
        {id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE},
        {id: 'github', name: 'GitHub', type: IdentityProviderTypes.GITHUB},
      ];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      expect(getComponentById(result, 'block_google')).toBeDefined();
      expect(getComponentById(result, 'block_github')).toBeUndefined();
    });

    it('should not include any social blocks when no providers are given', () => {
      const result = buildPreviewMock({}, []) as unknown as MockComponent[];

      const socialBlocks = result.filter((c) => String(c.id).startsWith('block_'));
      expect(socialBlocks).toHaveLength(0);
    });

    it('should include the provider name in the social action label', () => {
      const integrations = {google: true};
      const providers = [{id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE}];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      const googleBlock = getComponentById(result, 'block_google')!;
      const subComponents = googleBlock.components as MockComponent[];
      const action = subComponents.find((c) => c.id === 'action_google');
      expect(action!.label).toBe('{{t(elements:buttons.google.text)}}');
    });
  });

  describe('Divider visibility', () => {
    it('should show divider when basic auth and passkey are both enabled', () => {
      const integrations = {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        [AuthenticatorTypes.PASSKEY]: true,
      };
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeDefined();
    });

    it('should show divider when basic auth and social providers are present', () => {
      const integrations = {[AuthenticatorTypes.CREDENTIALS_AUTH]: true, google: true};
      const providers = [{id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE}];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeDefined();
    });

    it('should show divider when passkey and social providers are present', () => {
      const integrations = {[AuthenticatorTypes.PASSKEY]: true, google: true};
      const providers = [{id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE}];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeDefined();
    });

    it('should not show divider when only basic auth is enabled with no social', () => {
      const integrations = {[AuthenticatorTypes.CREDENTIALS_AUTH]: true};
      const result = buildPreviewMock(integrations, []) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeUndefined();
    });

    it('should not show divider when only social providers are enabled with no basic/passkey', () => {
      const integrations = {google: true};
      const providers = [{id: 'google', name: 'Google', type: IdentityProviderTypes.GOOGLE}];
      const result = buildPreviewMock(integrations, providers) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeUndefined();
    });

    it('should not show divider when no integrations are enabled', () => {
      const result = buildPreviewMock({}, []) as unknown as MockComponent[];

      expect(getComponentById(result, 'divider_or')).toBeUndefined();
    });
  });

  describe('Default parameters', () => {
    it('should use defaults and return a non-empty list', () => {
      const result = buildPreviewMock() as unknown as MockComponent[];

      expect(result.length).toBeGreaterThan(0);
    });

    it('should include basic auth, passkey, and social blocks in the default output', () => {
      const result = buildPreviewMock() as unknown as MockComponent[];

      expect(getComponentById(result, 'block_credentials_auth')).toBeDefined();
      expect(getComponentById(result, 'action_passkey')).toBeDefined();
      expect(getComponentById(result, 'block_google')).toBeDefined();
      expect(getComponentById(result, 'block_github')).toBeDefined();
    });
  });
});
