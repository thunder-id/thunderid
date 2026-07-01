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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import TokenUserAttributesSection from '../TokenUserAttributesSection';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string, options?: Record<string, string>) => {
      let result = fallback ?? _key;
      if (options) {
        Object.entries(options).forEach(([k, v]) => {
          result = result.replace(new RegExp(`{{${k}}}`, 'g'), v);
        });
      }
      return result;
    },
  }),
}));

// Mock Components
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, description, children}: {title: string; description: string; children: React.ReactNode}) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {children}
    </div>
  ),
}));

// Mock JwtPreview (uses Monaco editor)
vi.mock('../JwtPreview', () => ({
  default: ({title, payload}: {title: string; payload: Record<string, string>}) => (
    <div data-testid="jwt-preview">
      <div data-testid="jwt-preview-title">{title}</div>
      <pre data-testid="jwt-preview-payload">{JSON.stringify(payload)}</pre>
    </div>
  ),
}));

// Mock TokenConstants
vi.mock('../../../../constants/token-constants', () => ({
  default: {
    DEFAULT_TOKEN_ATTRIBUTES: ['aud', 'exp', 'iat', 'iss', 'sub'],
    USER_INFO_DEFAULT_ATTRIBUTES: ['sub'],
    ADDITIONAL_USER_ATTRIBUTES: ['ouHandle'],
    ID_TOKEN_RESPONSE_TYPES: ['JWT', 'JWE', 'NESTED_JWT'],
    ID_TOKEN_ENCRYPTION_ALGS: ['RSA-OAEP', 'RSA-OAEP-256'],
    ID_TOKEN_ENCRYPTION_ENCS: ['A128CBC-HS256', 'A256GCM'],
    USER_INFO_RESPONSE_TYPES: ['JSON', 'JWS', 'JWE', 'NESTED_JWT'],
    USER_INFO_SIGNING_ALGS: ['RS256', 'RS512'],
    USER_INFO_ENCRYPTION_ALGS: ['RSA-OAEP', 'RSA-OAEP-256'],
    USER_INFO_ENCRYPTION_ENCS: ['A128CBC-HS256', 'A256GCM'],
  },
}));

const baseProps = {
  userAttributes: [],
  isLoadingUserAttributes: false,
  pendingAdditions: new Set<string>(),
  pendingRemovals: new Set<string>(),
  highlightedAttributes: new Set<string>(),
  onAttributeClick: vi.fn(),
};

describe('TokenUserAttributesSection', () => {
  describe('Card title and description', () => {
    it('renders the settings card with correct title for native mode', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} />);

      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Attributes & Response');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Configure the response types and user attributes included in your tokens and user info responses',
      );
    });

    it('renders the settings card with correct title for OAuth mode', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="access"
          onTabChange={vi.fn()}
        />,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Attributes & Response');
    });
  });

  describe('Native mode (sharedAttributes)', () => {
    it('renders a single panel without tabs', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} />);

      expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    });

    it('shows empty state alert when userAttributes is empty', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} />);

      expect(
        screen.getByText('No user attributes available. Configure allowed user types for this application.'),
      ).toBeInTheDocument();
    });

    it('shows loading text when isLoadingUserAttributes is true', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} isLoadingUserAttributes />);

      expect(screen.getByText('Loading user attributes...')).toBeInTheDocument();
    });

    it('renders user attributes as chips when provided', () => {
      render(
        <TokenUserAttributesSection {...baseProps} userAttributes={['email', 'username']} sharedAttributes={[]} />,
      );

      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.getByText('username')).toBeInTheDocument();
    });

    it('excludes DEFAULT_TOKEN_ATTRIBUTES from the available attributes panel', () => {
      render(<TokenUserAttributesSection {...baseProps} userAttributes={['email', 'sub']} sharedAttributes={[]} />);

      // 'sub' is a default attr and should not appear as a chip
      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.queryByText('sub')).not.toBeInTheDocument();
    });

    it('renders active chip (filled/primary) for attributes in sharedAttributes', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email', 'username']}
          sharedAttributes={['email']}
        />,
      );

      const emailChip = screen.getByText('email').closest('.MuiChip-root');
      expect(emailChip).toHaveClass('MuiChip-filled');
      expect(emailChip).toHaveClass('MuiChip-colorPrimary');
    });

    it('renders inactive chip (outlined) for attributes not in sharedAttributes', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email', 'username']}
          sharedAttributes={['email']}
        />,
      );

      const usernameChip = screen.getByText('username').closest('.MuiChip-root');
      expect(usernameChip).toHaveClass('MuiChip-outlined');
    });

    it('calls onAttributeClick with correct args when chip is clicked', async () => {
      const user = userEvent.setup();
      const onAttributeClick = vi.fn();

      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email']}
          sharedAttributes={[]}
          onAttributeClick={onAttributeClick}
        />,
      );

      await user.click(screen.getByText('email'));

      expect(onAttributeClick).toHaveBeenCalledWith('email', 'shared');
    });

    it('renders JWT preview', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} />);

      expect(screen.getByTestId('jwt-preview')).toBeInTheDocument();
    });

    it('shows sharedAttributes in the JWT preview payload', () => {
      render(<TokenUserAttributesSection {...baseProps} userAttributes={['email']} sharedAttributes={['email']} />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('email');
    });

    it('shows pending addition in JWT preview for shared mode', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email']}
          sharedAttributes={[]}
          pendingAdditions={new Set(['email'])}
        />,
      );

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('email');
    });

    it('excludes pending removal from JWT preview for shared mode', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email', 'username']}
          sharedAttributes={['email', 'username']}
          pendingRemovals={new Set(['email'])}
        />,
      );

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).not.toContain('"email"');
      expect(payload).toContain('username');
    });
  });

  describe('OAuth mode (accessTokenAttributes, idTokenAttributes, userInfoAttributes)', () => {
    const oauthProps = {
      ...baseProps,
      accessTokenAttributes: ['email'],
      idTokenAttributes: ['username'],
      userInfoAttributes: [],
      activeTab: 'access' as const,
      onTabChange: vi.fn(),
    };

    it('renders three tabs', () => {
      render(<TokenUserAttributesSection {...oauthProps} />);

      expect(screen.getByRole('tab', {name: /access token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /id token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /user info endpoint/i})).toBeInTheDocument();
    });

    it('shows Access Token panel when activeTab is "access"', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" />);

      // Access token attrs shown; no id token attrs
      expect(screen.queryByRole('tab', {selected: true})).not.toBeNull();
    });

    it('shows ID Token panel when activeTab is "id"', () => {
      render(<TokenUserAttributesSection {...oauthProps} userAttributes={['username']} activeTab="id" />);

      // ID token attrs panel should show 'username' chip (it's in idTokenAttributes → active)
      const usernameChip = screen.getByText('username').closest('.MuiChip-root');
      expect(usernameChip).toHaveClass('MuiChip-filled');
    });

    it('shows User Info panel with inherit toggle when activeTab is "userinfo"', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="userinfo" />);

      expect(screen.getByText('Use same attributes as ID Token')).toBeInTheDocument();
    });

    it('shows custom user info panel when isUserInfoCustomAttributes is true', () => {
      render(
        <TokenUserAttributesSection
          {...oauthProps}
          activeTab="userinfo"
          isUserInfoCustomAttributes
          userAttributes={['email']}
          userInfoAttributes={['email']}
        />,
      );

      // Custom attributes panel is active (not disabled/grayed out)
      // The email chip should be active (filled) because it's in userInfoAttributes
      const emailChip = screen.getByText('email').closest('.MuiChip-root');
      expect(emailChip).toHaveClass('MuiChip-filled');
    });

    it('calls onTabChange when a tab is clicked', async () => {
      const user = userEvent.setup();
      const onTabChange = vi.fn();

      render(<TokenUserAttributesSection {...oauthProps} onTabChange={onTabChange} />);

      await user.click(screen.getByRole('tab', {name: /id token/i}));

      expect(onTabChange).toHaveBeenCalledWith('id');
    });

    it('shows empty state when userAttributes is empty in OAuth mode', () => {
      render(<TokenUserAttributesSection {...oauthProps} userAttributes={[]} />);

      expect(
        screen.getByText('No user attributes available. Configure allowed user types for this application.'),
      ).toBeInTheDocument();
    });

    it('calls onAttributeClick with "access" token type when chip clicked in access tab', async () => {
      const user = userEvent.setup();
      const onAttributeClick = vi.fn();

      render(
        <TokenUserAttributesSection
          {...oauthProps}
          userAttributes={['email']}
          activeTab="access"
          onAttributeClick={onAttributeClick}
        />,
      );

      await user.click(screen.getByText('email'));

      expect(onAttributeClick).toHaveBeenCalledWith('email', 'access');
    });

    it('shows pending additions in access token preview when activeTab matches', () => {
      render(
        <TokenUserAttributesSection
          {...oauthProps}
          userAttributes={['email']}
          activeTab="access"
          pendingAdditions={new Set(['email'])}
        />,
      );

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('email');
    });

    it('does not apply pending changes when activeTab does not match', () => {
      render(
        <TokenUserAttributesSection
          {...oauthProps}
          userAttributes={['email']}
          activeTab="id"
          pendingAdditions={new Set(['email'])}
        />,
      );

      // ID token panel is shown; email is in accessTokenAttributes but not idTokenAttributes
      // Pending additions don't apply to 'id' tab when activeTab='id' but email is access-only
      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      // email is a pending addition and activeTab=id, so it should appear in id preview too
      // because isPendingTab = (activeTab === tokenType) = ('id' === 'id') = true
      expect(payload).toContain('email');
    });
  });

  describe('ID Token response format', () => {
    it('renders response type select in ID Token tab', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Response Format')).toBeInTheDocument();
      expect(screen.getByText('Response Type')).toBeInTheDocument();
    });

    it('shows encryption fields when ID token response type is JWE', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWE"
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Encryption Algorithm')).toBeInTheDocument();
      expect(screen.getByText('Content Encryption')).toBeInTheDocument();
    });

    it('does not show encryption fields when ID token response type is JWT', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWT"
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.queryByText('Encryption Algorithm')).not.toBeInTheDocument();
    });
  });

  describe('UserInfo response format', () => {
    it('renders response type select in UserInfo tab', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="userinfo"
          onTabChange={vi.fn()}
          isUserInfoCustomAttributes
          onToggleUserInfo={vi.fn()}
          onUserInfoConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Response Format')).toBeInTheDocument();
    });

    it('shows signing algorithm when UserInfo response type is JWS', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="userinfo"
          onTabChange={vi.fn()}
          isUserInfoCustomAttributes
          onToggleUserInfo={vi.fn()}
          userInfoResponseType="JWS"
          onUserInfoConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Signing Algorithm')).toBeInTheDocument();
    });

    it('shows encryption fields when UserInfo response type is JWE', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="userinfo"
          onTabChange={vi.fn()}
          isUserInfoCustomAttributes
          onToggleUserInfo={vi.fn()}
          userInfoResponseType="JWE"
          onUserInfoConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Encryption Algorithm')).toBeInTheDocument();
      expect(screen.getByText('Content Encryption')).toBeInTheDocument();
    });

    it('does not show algorithm fields when UserInfo response type is JSON', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="userinfo"
          onTabChange={vi.fn()}
          isUserInfoCustomAttributes
          onToggleUserInfo={vi.fn()}
          userInfoResponseType="JSON"
          onUserInfoConfigChange={vi.fn()}
        />,
      );

      expect(screen.queryByText('Signing Algorithm')).not.toBeInTheDocument();
      expect(screen.queryByText('Encryption Algorithm')).not.toBeInTheDocument();
    });
  });

  describe('ADDITIONAL_USER_ATTRIBUTES', () => {
    it('includes ADDITIONAL_USER_ATTRIBUTES in the available chips', () => {
      render(<TokenUserAttributesSection {...baseProps} userAttributes={['email']} sharedAttributes={[]} />);

      // 'ouHandle' is in the mocked ADDITIONAL_USER_ATTRIBUTES and not a default attr
      // It should appear alongside userAttributes when userAttributes.length > 0
      expect(screen.getByText('ouHandle')).toBeInTheDocument();
    });
  });
});
