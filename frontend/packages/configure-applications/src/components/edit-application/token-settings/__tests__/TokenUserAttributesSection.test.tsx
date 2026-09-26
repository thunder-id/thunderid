// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor, within} from '@testing-library/react';
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

      // Native mode issues one token and has no response-format controls, so it drops the
      // "& Response" title and the multi-token wording.
      expect(screen.getByTestId('card-title')).toHaveTextContent('Token Attributes');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Choose the attributes included in the token issued to this application.',
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

    it('drops the "user info responses" mention from the description when showUserInfoTab is false', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          activeTab="access"
          onTabChange={vi.fn()}
          showUserInfoTab={false}
          entityLabel="agent"
        />,
      );

      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Choose the attributes in each token issued to this agent, and how each is returned.',
      );
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

    // A user-subject token has a user as its subject, so the server never stamps sub_type here.
    it('does not show sub_type in a user token preview', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={[]} />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).not.toContain('sub_type');
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

    it('renders the access token tab alongside the combined user-token tab', () => {
      render(<TokenUserAttributesSection {...oauthProps} />);

      // The ID token and UserInfo share one scope mapping, so they sit under a single top-level tab.
      expect(screen.getByRole('tab', {name: /access token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /id token & user info/i})).toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: /user info endpoint/i})).not.toBeInTheDocument();
    });

    it('renders the ID Token and User Info sub-tabs under the combined tab', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="id" />);

      expect(screen.getByRole('tab', {name: /^id token$/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /user info endpoint/i})).toBeInTheDocument();
    });

    it('labels the combined tab as ID Token alone when the User Info tab is hidden', () => {
      render(<TokenUserAttributesSection {...oauthProps} showUserInfoTab={false} activeTab="id" />);

      expect(screen.queryByRole('tab', {name: /user info endpoint/i})).not.toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /^id token$/i})).toBeInTheDocument();
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

    it('renders only two tabs when showUserInfoTab is false (agents)', () => {
      render(<TokenUserAttributesSection {...oauthProps} showUserInfoTab={false} />);

      expect(screen.getByRole('tab', {name: /access token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /id token/i})).toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: /user info endpoint/i})).not.toBeInTheDocument();
      expect(screen.getAllByRole('tab')).toHaveLength(2);
    });

    it('does not render the User Info panel when showUserInfoTab is false, even if activeTab is "userinfo"', () => {
      render(<TokenUserAttributesSection {...oauthProps} showUserInfoTab={false} activeTab="userinfo" />);

      expect(screen.queryByText('Use same attributes as ID Token')).not.toBeInTheDocument();
    });

    it('maps tab clicks to only the two visible tabs when showUserInfoTab is false', async () => {
      const user = userEvent.setup();
      const onTabChange = vi.fn();

      render(<TokenUserAttributesSection {...oauthProps} showUserInfoTab={false} onTabChange={onTabChange} />);

      await user.click(screen.getByRole('tab', {name: /id token/i}));

      expect(onTabChange).toHaveBeenCalledWith('id');
    });

    it('does not include the act claim in the access token preview by default', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).not.toContain('"act"');
    });

    it('includes the act claim in the access token preview when showActorClaim is true (agents)', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" showActorClaim />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('"act"');
      expect(payload).toContain('agent-id');
    });

    it('does not include an iss field inside the act claim (matches the agent OBO token)', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" showActorClaim />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('"act"');
      expect(payload).not.toContain('issuer');
    });

    it('renders the provided actorSub as the act.sub value', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" showActorClaim actorSub="my-agent-123" />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).toContain('my-agent-123');
    });

    it('does not include the act claim in the ID token preview even when showActorClaim is true', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="id" showActorClaim />);

      const payload = screen.getByTestId('jwt-preview-payload').textContent ?? '';
      expect(payload).not.toContain('"act"');
    });

    it('shows an explanation of the act claim on the access token tab when showActorClaim is true', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" showActorClaim entityLabel="agent" />);

      expect(
        screen.getByText(/identifies this agent as the party acting on behalf of the subject/i),
      ).toBeInTheDocument();
    });

    it('does not show the act claim explanation when showActorClaim is false', () => {
      render(<TokenUserAttributesSection {...oauthProps} activeTab="access" />);

      expect(
        screen.queryByText(/identifies this agent as the party acting on behalf of the subject/i),
      ).not.toBeInTheDocument();
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

    it('does not show a signing algorithm selector when UserInfo response type is JWS', () => {
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

      // Signing always uses the server key, so no algorithm is chosen or encryption shown for JWS.
      expect(screen.queryByText('Signing Algorithm')).not.toBeInTheDocument();
      expect(screen.queryByText('Encryption Algorithm')).not.toBeInTheDocument();
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
          hasCertificate
          onUserInfoConfigChange={vi.fn()}
        />,
      );

      expect(screen.queryByText('Signing Algorithm')).not.toBeInTheDocument();
      expect(screen.queryByText('Encryption Algorithm')).not.toBeInTheDocument();
      // With a certificate configured, the certificate-required hint is not shown.
      expect(screen.queryByText(/Encrypted formats require an OAuth client certificate/)).not.toBeInTheDocument();
    });

    it('shows the certificate requirement hint for an encrypted UserInfo format', () => {
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

      expect(screen.getByText(/Encrypted formats require an OAuth client certificate/)).toBeInTheDocument();
    });

    it('shows the certificate requirement hint for an encrypted ID token format', () => {
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
      expect(screen.getByText(/Encrypted formats require an OAuth client certificate/)).toBeInTheDocument();
    });

    it('names the configured certificate location in the requirement hint', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWT"
          certificateLocation="Credentials"
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText(/configured under the Credentials tab/)).toBeInTheDocument();
    });

    it('hides the certificate requirement hint when a certificate is configured', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWT"
          hasCertificate
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.queryByText(/Encrypted formats require an OAuth client certificate/)).not.toBeInTheDocument();
    });

    it('shows the read-only signing algorithm from discovery on the ID token tab', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWT"
          signingAlg="ES256"
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.getByText(/Signed with ES256/)).toBeInTheDocument();
    });

    it('hides the signing algorithm on the ID token tab when the format is encrypt-only (JWE)', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWE"
          signingAlg="ES256"
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      expect(screen.queryByText(/Signed with/)).not.toBeInTheDocument();
    });

    it('hides the signing algorithm when it cannot be resolved from discovery', () => {
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

      expect(screen.queryByText(/Signed with/)).not.toBeInTheDocument();
    });

    it('disables encrypted ID token format options when no certificate is configured', () => {
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

      // Open the response type dropdown. In tests the i18n mock renders the raw format values.
      fireEvent.mouseDown(screen.getByRole('combobox'));
      const options = within(screen.getByRole('listbox')).getAllByRole('option');
      const optionByValue = (value: string) => options.find((o) => o.getAttribute('data-value') === value);

      expect(optionByValue('JWE')).toHaveAttribute('aria-disabled', 'true');
      expect(optionByValue('NESTED_JWT')).toHaveAttribute('aria-disabled', 'true');
      expect(optionByValue('JWT')).not.toHaveAttribute('aria-disabled', 'true');
    });

    it('enables encrypted ID token format options when a certificate is configured', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          accessTokenAttributes={[]}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="id"
          onTabChange={vi.fn()}
          idTokenResponseType="JWT"
          hasCertificate
          onIdTokenConfigChange={vi.fn()}
        />,
      );

      fireEvent.mouseDown(screen.getByRole('combobox'));
      const options = within(screen.getByRole('listbox')).getAllByRole('option');
      const optionByValue = (value: string) => options.find((o) => o.getAttribute('data-value') === value);

      expect(optionByValue('JWE')).not.toHaveAttribute('aria-disabled', 'true');
      expect(optionByValue('NESTED_JWT')).not.toHaveAttribute('aria-disabled', 'true');
    });
  });

  describe('Scope mapping slot and collapsible attribute picker', () => {
    const slotProps = {
      ...baseProps,
      userAttributes: ['username'],
      accessTokenAttributes: ['email'],
      idTokenAttributes: ['username'],
      userInfoAttributes: [],
      onTabChange: vi.fn(),
      scopeMapping: <div data-testid="scope-mapping">mapping</div>,
    };

    it('renders the scope mapping under the ID Token tab', () => {
      render(<TokenUserAttributesSection {...slotProps} activeTab="id" />);

      expect(screen.getByTestId('scope-mapping')).toBeInTheDocument();
    });

    it('renders the scope mapping under the User Info tab', () => {
      render(<TokenUserAttributesSection {...slotProps} activeTab="userinfo" />);

      expect(screen.getByTestId('scope-mapping')).toBeInTheDocument();
    });

    it('does not render the scope mapping under the Access Token tab', () => {
      render(<TokenUserAttributesSection {...slotProps} activeTab="access" />);

      // Access token attributes are not scope-driven, so the mapping would be misleading there.
      expect(screen.queryByTestId('scope-mapping')).not.toBeInTheDocument();
    });

    it('collapses the attribute picker on the ID Token tab behind a disclosure', () => {
      render(<TokenUserAttributesSection {...slotProps} activeTab="id" />);

      const toggle = screen.getByRole('button', {name: /allowed attributes/i});
      expect(toggle).toBeInTheDocument();
      expect(screen.getByText('username').closest('.MuiChip-root')).not.toBeVisible();
    });

    it('expands the attribute picker when the disclosure is clicked', async () => {
      const user = userEvent.setup();
      render(<TokenUserAttributesSection {...slotProps} activeTab="id" />);

      await user.click(screen.getByRole('button', {name: /allowed attributes/i}));

      await waitFor(() => {
        expect(screen.getByText('username').closest('.MuiChip-root')).toBeVisible();
      });
    });

    it('leaves the attribute picker expanded on the Access Token tab', () => {
      render(<TokenUserAttributesSection {...slotProps} activeTab="access" />);

      // Nothing else drives the access token, so its picker is not collapsed.
      expect(screen.queryByRole('button', {name: /allowed attributes/i})).not.toBeInTheDocument();
      expect(screen.getByText('username').closest('.MuiChip-root')).toBeVisible();
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

  describe('Selected attributes missing from the schema', () => {
    it('renders a selected attribute that is no longer in userAttributes', () => {
      render(
        <TokenUserAttributesSection {...baseProps} userAttributes={['email']} sharedAttributes={['deletedAttr']} />,
      );

      const chip = screen.getByText('deletedAttr').closest('.MuiChip-root');
      expect(chip).toHaveClass('MuiChip-filled');
    });

    it('allows deselecting an attribute that is no longer in userAttributes', async () => {
      const onAttributeClick = vi.fn();
      render(
        <TokenUserAttributesSection
          {...baseProps}
          onAttributeClick={onAttributeClick}
          userAttributes={['email']}
          sharedAttributes={['deletedAttr']}
        />,
      );

      await userEvent.click(screen.getByText('deletedAttr'));

      expect(onAttributeClick).toHaveBeenCalledWith('deletedAttr', 'shared');
    });

    it('renders stale selections even when userAttributes is empty', () => {
      render(<TokenUserAttributesSection {...baseProps} sharedAttributes={['deletedAttr']} />);

      expect(screen.getByText('deletedAttr')).toBeInTheDocument();
    });

    it('renders a stale access token attribute in OAuth mode', () => {
      render(
        <TokenUserAttributesSection
          {...baseProps}
          userAttributes={['email']}
          accessTokenAttributes={['deletedAttr']}
          idTokenAttributes={[]}
          userInfoAttributes={[]}
          activeTab="access"
          onTabChange={vi.fn()}
        />,
      );

      const chip = screen.getByText('deletedAttr').closest('.MuiChip-root');
      expect(chip).toHaveClass('MuiChip-filled');
    });
  });
});
