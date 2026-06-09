/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import IndividualMethodsToggleView, {type IndividualMethodsToggleViewProps} from '../IndividualMethodsToggleView';
import {AuthenticatorTypes} from '@/features/integrations/models/authenticators';
import {IdentityProviderTypes, type IdentityProvider} from '@/features/integrations/models/identity-provider';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:onboarding.configure.SignInOptions.usernamePassword': 'Username & Password',
        'applications:onboarding.configure.SignInOptions.passkey': 'Passkey',
        'applications:onboarding.configure.SignInOptions.google': 'Google',
        'applications:onboarding.configure.SignInOptions.github': 'GitHub',
        'applications:onboarding.configure.SignInOptions.notConfigured': 'Not configured',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock the integration icon utility
vi.mock('@/features/integrations/utils/getIntegrationIcon', () => ({
  default: vi.fn((type: string) => <div data-testid={`icon-${type}`}>Mock Icon</div>),
}));

describe('IndividualMethodsToggleView', () => {
  const mockOnIntegrationToggle = vi.fn();

  const mockIdentityProviders: IdentityProvider[] = [
    {
      id: 'google-idp',
      name: 'Google',
      type: IdentityProviderTypes.GOOGLE,
      description: 'Sign in with Google',
    },
    {
      id: 'github-idp',
      name: 'GitHub',
      type: IdentityProviderTypes.GITHUB,
      description: 'Sign in with GitHub',
    },
    {
      id: 'oauth-idp-1',
      name: 'OAuth Provider',
      type: IdentityProviderTypes.OAUTH,
      description: 'Sign in with OAuth',
    },
    {
      id: 'oidc-idp-1',
      name: 'OIDC Provider',
      type: IdentityProviderTypes.OIDC,
      description: 'Sign in with OIDC',
    },
  ];

  const defaultProps: IndividualMethodsToggleViewProps = {
    integrations: {
      [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
    },
    availableIntegrations: mockIdentityProviders,
    onIntegrationToggle: mockOnIntegrationToggle,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderComponent = (props: Partial<IndividualMethodsToggleViewProps> = {}) =>
    render(<IndividualMethodsToggleView {...defaultProps} {...props} />);

  describe('core authentication methods', () => {
    it('should render Username & Password option', () => {
      renderComponent();

      expect(screen.getByText('Username & Password')).toBeInTheDocument();
    });

    it('should render Passkey option', () => {
      renderComponent();

      expect(screen.getByText('Passkey')).toBeInTheDocument();
    });

    it('should render Google option', () => {
      renderComponent();

      expect(screen.getByText('Google')).toBeInTheDocument();
    });

    it('should render GitHub option', () => {
      renderComponent();

      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    it('should render in correct order: Username & Password, Google, GitHub', () => {
      renderComponent();

      const listItems = screen.getAllByRole('button');
      // First item should be Username & Password
      expect(listItems[0]).toHaveTextContent('Username & Password');
    });
  });

  describe('integration states', () => {
    it('should show Username & Password as enabled when basic_auth is true', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      const switches = screen.getAllByRole('switch');
      // First switch should be for Username & Password
      expect(switches[0]).toBeChecked();
    });

    it('should show Username & Password as disabled when basic_auth is false', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        },
      });

      const switches = screen.getAllByRole('switch');
      expect(switches[0]).not.toBeChecked();
    });

    it('should show Passkey as enabled when passkey is true', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          [AuthenticatorTypes.PASSKEY]: true,
        },
      });

      const switches = screen.getAllByRole('switch');
      // Second switch should be for Passkey
      expect(switches[1]).toBeChecked();
    });

    it('should show Passkey as disabled when passkey is false', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          [AuthenticatorTypes.PASSKEY]: false,
        },
      });

      const switches = screen.getAllByRole('switch');
      // Second switch should be for Passkey
      expect(switches[1]).not.toBeChecked();
    });

    it('should show Passkey as disabled when passkey is undefined', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          [AuthenticatorTypes.PASSKEY]: undefined as unknown as boolean,
        },
      });

      const switches = screen.getAllByRole('switch');
      expect(switches[1]).not.toBeChecked();
    });

    it('should show Google as enabled when its ID is in integrations', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'google-idp': true,
        },
      });

      const switches = screen.getAllByRole('switch');
      // Second switch should be for Google
      expect(switches[2]).toBeChecked();
    });

    it('should show GitHub as enabled when its ID is in integrations', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'github-idp': true,
        },
      });

      const switches = screen.getAllByRole('switch');
      // Third switch should be for GitHub
      expect(switches[3]).toBeChecked();
    });
  });

  describe('toggle interactions', () => {
    it('should call onIntegrationToggle with basic_auth when Username & Password is toggled', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switches = screen.getAllByRole('switch');
      await user.click(switches[0]);

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.CREDENTIALS_AUTH);
    });

    it('should call onIntegrationToggle with passkey when Passkey is toggled', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switches = screen.getAllByRole('switch');
      await user.click(switches[1]); // Passkey switch

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.PASSKEY);
    });

    it('should call onIntegrationToggle with google provider ID when Google is toggled', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switches = screen.getAllByRole('switch');
      await user.click(switches[2]); // Google switch

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith('google-idp');
    });

    it('should call onIntegrationToggle with github provider ID when GitHub is toggled', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switches = screen.getAllByRole('switch');
      await user.click(switches[3]); // GitHub switch

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith('github-idp');
    });

    it('should call onIntegrationToggle when clicking on list item button', async () => {
      const user = userEvent.setup();
      renderComponent();

      const buttons = screen.getAllByRole('button');
      await user.click(buttons[0]); // Username & Password button

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.CREDENTIALS_AUTH);
    });
  });

  describe('other social providers', () => {
    it('should render other providers (non-Google, non-GitHub)', () => {
      renderComponent();

      expect(screen.getByText('OAuth Provider')).toBeInTheDocument();
      expect(screen.getByText('OIDC Provider')).toBeInTheDocument();
    });

    it('should call onIntegrationToggle with provider ID when other provider is toggled', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Find the OAuth Provider item and its switch
      const oauthItem = screen.getByText('OAuth Provider').closest('li');
      const oauthSwitch = oauthItem?.querySelector('input[type="checkbox"]');

      expect(oauthSwitch).not.toBeNull();
      await user.click(oauthSwitch!);
      expect(mockOnIntegrationToggle).toHaveBeenCalledWith('oauth-idp-1');
    });

    it('should show other provider as enabled when its ID is in integrations', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'oauth-idp-1': true,
        },
      });

      const oauthItem = screen.getByText('OAuth Provider').closest('li');
      const oauthSwitch = oauthItem?.querySelector('input[type="checkbox"]');

      expect(oauthSwitch).toBeChecked();
    });
  });

  describe('unavailable providers', () => {
    it('should show Google as unavailable when not in availableIntegrations', () => {
      renderComponent({
        availableIntegrations: mockIdentityProviders.filter((idp) => idp.type !== IdentityProviderTypes.GOOGLE),
      });

      expect(screen.getByText('Google')).toBeInTheDocument();
      expect(screen.getByText('Not configured')).toBeInTheDocument();
    });

    it('should show GitHub as unavailable when not in availableIntegrations', () => {
      renderComponent({
        availableIntegrations: mockIdentityProviders.filter((idp) => idp.type !== IdentityProviderTypes.GITHUB),
      });

      expect(screen.getByText('GitHub')).toBeInTheDocument();
      expect(screen.getByText('Not configured')).toBeInTheDocument();
    });

    it('should show both Google and GitHub as unavailable when empty integrations', () => {
      renderComponent({
        availableIntegrations: [],
      });

      expect(screen.getByText('Google')).toBeInTheDocument();
      expect(screen.getByText('GitHub')).toBeInTheDocument();
      expect(screen.getAllByText('Not configured')).toHaveLength(2);
    });

    it('should disable unavailable provider buttons', () => {
      renderComponent({
        availableIntegrations: [],
      });

      // Find all buttons - Google and GitHub should be disabled (aria-disabled)
      const buttons = screen.getAllByRole('button');
      // The unavailable providers will have buttons with aria-disabled attribute
      const disabledButtons = buttons.filter((btn) => btn.getAttribute('aria-disabled') === 'true');

      // Should have 2 disabled buttons (Google and GitHub)
      expect(disabledButtons).toHaveLength(2);
    });

    it('should not render switch for unavailable providers', () => {
      renderComponent({
        availableIntegrations: [],
      });

      // Only Username & Password and Passkey should have a switch
      const switches = screen.getAllByRole('switch');
      expect(switches).toHaveLength(2);
    });
  });

  describe('list structure', () => {
    it('should render a list element', () => {
      renderComponent();

      const list = screen.getByRole('list');
      expect(list).toBeInTheDocument();
    });

    it('should render dividers between items', () => {
      renderComponent();

      const list = screen.getByRole('list');
      const dividers = list.querySelectorAll('hr, .MuiDivider-root');
      expect(dividers.length).toBeGreaterThan(0);
    });
  });

  describe('edge cases', () => {
    it('should handle empty integrations object', () => {
      renderComponent({
        integrations: {},
      });

      // Should not crash and should render all options as unchecked
      expect(screen.getByText('Username & Password')).toBeInTheDocument();
      const switches = screen.getAllByRole('switch');
      switches.forEach((switchEl) => {
        expect(switchEl).not.toBeChecked();
      });
    });

    it('should handle undefined integration value for basic_auth', () => {
      renderComponent({
        integrations: {
          // basic_auth not defined
        },
      });

      const switches = screen.getAllByRole('switch');
      // Username & Password switch should be unchecked when undefined
      expect(switches[0]).not.toBeChecked();
    });

    it('should handle undefined integration value for Google provider', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'google-idp': undefined as unknown as boolean,
        },
      });

      const switches = screen.getAllByRole('switch');
      // Google switch should be unchecked when undefined
      expect(switches[1]).not.toBeChecked();
    });

    it('should handle undefined integration value for GitHub provider', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'github-idp': undefined as unknown as boolean,
        },
      });

      const switches = screen.getAllByRole('switch');
      // GitHub switch should be unchecked when undefined
      expect(switches[2]).not.toBeChecked();
    });

    it('should handle undefined integration value for other providers', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'oauth-idp-1': undefined as unknown as boolean,
        },
      });

      const oauthItem = screen.getByText('OAuth Provider').closest('li');
      const oauthSwitch = oauthItem?.querySelector('input[type="checkbox"]');

      // OAuth provider switch should be unchecked when undefined
      expect(oauthSwitch).not.toBeChecked();
    });

    it('should use default ID for Google when provider not found', () => {
      renderComponent({
        availableIntegrations: [],
      });

      // Google should still render with fallback ID 'google'
      expect(screen.getByText('Google')).toBeInTheDocument();
    });

    it('should use default ID for GitHub when provider not found', () => {
      renderComponent({
        availableIntegrations: [],
      });

      // GitHub should still render with fallback ID 'github'
      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    it('should handle providers with special characters in names', () => {
      const specialProviders: IdentityProvider[] = [
        {
          id: 'special-provider',
          name: 'Provider with Special & Characters',
          type: IdentityProviderTypes.OAUTH,
          description: 'Special provider',
        },
      ];

      renderComponent({
        availableIntegrations: [...mockIdentityProviders, ...specialProviders],
      });

      expect(screen.getByText('Provider with Special & Characters')).toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('should have proper list structure for screen readers', () => {
      renderComponent();

      const list = screen.getByRole('list');
      expect(list).toBeInTheDocument();
    });

    it('should have switches with proper roles', () => {
      renderComponent();

      const switches = screen.getAllByRole('switch');
      expect(switches.length).toBeGreaterThan(0);
    });

    it('should have buttons with proper roles', () => {
      renderComponent();

      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should be keyboard navigable', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Tab to first focusable element
      await user.tab();

      const firstButton = screen.getAllByRole('button')[0];
      expect(firstButton).toHaveFocus();
    });
  });
});
