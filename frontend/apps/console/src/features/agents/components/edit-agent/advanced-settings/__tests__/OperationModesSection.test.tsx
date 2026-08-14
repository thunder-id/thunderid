// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {OAuth2Config} from '@thunderid/configure-applications';
import {describe, it, expect, vi} from 'vitest';
import OperationModesSection from '../OperationModesSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
  Trans: ({defaults = ''}: {defaults?: string}) => <span>{defaults}</span>,
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    discovery: {
      wellKnown: {
        token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
      },
    },
  }),
}));

describe('OperationModesSection', () => {
  const autonomousOnlyConfig: OAuth2Config = {
    grantTypes: ['client_credentials'],
    responseTypes: [],
  };

  const delegatedConfig: OAuth2Config = {
    grantTypes: ['client_credentials', 'authorization_code'],
    responseTypes: ['code'],
    redirectUris: ['https://example.com/cb'],
  };

  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(<OperationModesSection />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the card title and description', () => {
    render(<OperationModesSection oauth2Config={autonomousOnlyConfig} />);

    expect(screen.getByText('OAuth 2 Configuration')).toBeInTheDocument();
  });

  it('shows an info note explaining the Delegated-mode dependency, before the grant types selector', () => {
    render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={vi.fn()} />);

    expect(
      screen.getByText(/greyed-out grants unlock once you turn on Delegated mode at the top of this tab/i),
    ).toBeInTheDocument();
  });

  describe('Grant type selection', () => {
    it('lists every grant type in Autonomous-only mode, but locks the delegated-only ones', async () => {
      const user = userEvent.setup();
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={vi.fn()} />);

      await user.click(document.getElementById('agent-grant-types')!);

      const listbox = await screen.findByRole('listbox');
      expect(within(listbox).getByText('client_credentials')).toBeInTheDocument();
      expect(within(listbox).getByText('Token Exchange')).toBeInTheDocument();
      expect(within(listbox).getByText('authorization_code')).toBeInTheDocument();
      expect(within(listbox).getByText('CIBA (Client-Initiated Backchannel Authentication)')).toBeInTheDocument();
      expect(within(listbox).getByText('refresh_token')).toBeInTheDocument();

      expect(within(listbox).getByText('authorization_code').closest('li')).toHaveAttribute('aria-disabled', 'true');
      expect(
        within(listbox).getByText('CIBA (Client-Initiated Backchannel Authentication)').closest('li'),
      ).toHaveAttribute('aria-disabled', 'true');
      expect(within(listbox).getByText('refresh_token').closest('li')).toHaveAttribute('aria-disabled', 'true');
    });

    it('unlocks ciba and refresh_token (but keeps authorization_code locked) once Delegated mode is on', async () => {
      const user = userEvent.setup();
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={vi.fn()} />);

      await user.click(document.getElementById('agent-grant-types')!);

      const listbox = await screen.findByRole('listbox');
      expect(within(listbox).getByText('authorization_code').closest('li')).toHaveAttribute('aria-disabled', 'true');
      expect(
        within(listbox).getByText('CIBA (Client-Initiated Backchannel Authentication)').closest('li'),
      ).not.toHaveAttribute('aria-disabled', 'true');
      expect(within(listbox).getByText('refresh_token').closest('li')).not.toHaveAttribute('aria-disabled', 'true');
    });

    it('locks client_credentials so it cannot be toggled off', async () => {
      const user = userEvent.setup();
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={vi.fn()} />);

      await user.click(document.getElementById('agent-grant-types')!);
      const listbox = await screen.findByRole('listbox');

      expect(within(listbox).getByText('client_credentials').closest('li')).toHaveAttribute('aria-disabled', 'true');
    });

    it('toggles token_exchange on when selected', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.click(document.getElementById('agent-grant-types')!);
      const listbox = await screen.findByRole('listbox');
      await user.click(within(listbox).getByText('Token Exchange'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          grantTypes: expect.arrayContaining([
            'client_credentials',
            'urn:ietf:params:oauth:grant-type:token-exchange',
          ]) as string[],
        }),
      );
    });

    it('toggles ciba on within delegated grants', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.click(document.getElementById('agent-grant-types')!);
      const listbox = await screen.findByRole('listbox');
      await user.click(within(listbox).getByText('CIBA (Client-Initiated Backchannel Authentication)'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          grantTypes: expect.arrayContaining(['authorization_code', 'urn:openid:params:grant-type:ciba']) as string[],
        }),
      );
    });
  });

  describe('Redirect URIs', () => {
    it('renders the redirect URI section once authorization_code is selected', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={vi.fn()} />);

      expect(screen.getByText('Authorized redirect URIs')).toBeInTheDocument();
    });

    it('hides the redirect URI section in Autonomous-only mode', () => {
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={vi.fn()} />);

      expect(screen.queryByText('Authorized redirect URIs')).not.toBeInTheDocument();
    });
  });

  describe('read-only', () => {
    it('disables the grant types input when there is no onOAuth2ConfigChange handler', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} />);

      expect(document.getElementById('agent-grant-types')).toHaveAttribute('aria-disabled', 'true');
    });

    it('disables the grant types input when disabled is true', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={vi.fn()} disabled />);

      expect(document.getElementById('agent-grant-types')).toHaveAttribute('aria-disabled', 'true');
    });
  });

  describe('Client Authentication Method', () => {
    it('shows the placeholder when no method is set', () => {
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} />);

      expect(screen.getByText('Select an auth method')).toBeInTheDocument();
    });

    it('locks the token method when the client is public', () => {
      render(<OperationModesSection oauth2Config={{...autonomousOnlyConfig, publicClient: true}} />);

      expect(screen.getByText('Set to "none" because this agent is a public client.')).toBeInTheDocument();
      expect(document.getElementById('agent_token_endpoint_auth_method')).toHaveClass('Mui-disabled');
    });

    it('renders the currently selected method', () => {
      render(
        <OperationModesSection
          oauth2Config={{...autonomousOnlyConfig, tokenEndpointAuthMethod: 'client_secret_basic'}}
        />,
      );

      expect(screen.getByText('client_secret_basic')).toBeInTheDocument();
    });

    it('calls onOAuth2ConfigChange when a new method is selected', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(
        <OperationModesSection
          oauth2Config={{...autonomousOnlyConfig, tokenEndpointAuthMethod: 'client_secret_basic'}}
          onOAuth2ConfigChange={onOAuth2ConfigChange}
        />,
      );

      await user.click(document.getElementById('agent_token_endpoint_auth_method')!);
      await user.click(screen.getByRole('option', {name: 'client_secret_post'}));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({tokenEndpointAuthMethod: 'client_secret_post'}) as Partial<OAuth2Config>,
      );
    });
  });

  describe('Require PKCE', () => {
    it('checks PKCE automatically when authorization_code is selected', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} />);

      expect(screen.getByLabelText('Require PKCE')).toBeChecked();
    });

    it('unchecks PKCE when authorization_code is not selected', () => {
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} />);

      expect(screen.getByLabelText('Require PKCE')).not.toBeChecked();
    });

    it('checks PKCE when the client is public even without authorization_code', () => {
      render(<OperationModesSection oauth2Config={{...autonomousOnlyConfig, publicClient: true}} />);

      expect(screen.getByLabelText('Require PKCE')).toBeChecked();
    });

    it('is never directly editable by the user', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={vi.fn()} />);

      expect(screen.getByLabelText('Require PKCE')).toBeDisabled();
    });

    it('shows the not-applicable caption when authorization_code is off', () => {
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} />);

      expect(screen.getByText(/PKCE only applies to the/)).toBeInTheDocument();
    });
  });

  describe('Require Pushed Authorization Requests', () => {
    it('reflects the configured value', () => {
      render(<OperationModesSection oauth2Config={{...delegatedConfig, requirePushedAuthorizationRequests: true}} />);

      expect(screen.getByLabelText('Require Pushed Authorization Requests')).toBeChecked();
    });

    it('treats an unset value as unchecked', () => {
      render(<OperationModesSection oauth2Config={delegatedConfig} />);

      expect(screen.getByLabelText('Require Pushed Authorization Requests')).not.toBeChecked();
    });

    it('calls onOAuth2ConfigChange when toggled', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(<OperationModesSection oauth2Config={delegatedConfig} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.click(screen.getByLabelText('Require Pushed Authorization Requests'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith({requirePushedAuthorizationRequests: true});
    });

    it('is disabled and unchecked when the authorization_code grant is off', () => {
      render(
        <OperationModesSection
          oauth2Config={{...autonomousOnlyConfig, requirePushedAuthorizationRequests: true}}
          onOAuth2ConfigChange={vi.fn()}
        />,
      );

      const parSwitch = screen.getByLabelText('Require Pushed Authorization Requests');
      expect(parSwitch).toBeDisabled();
      expect(parSwitch).not.toBeChecked();
    });
  });

  describe('Default audience', () => {
    it('renders the current default audience', () => {
      render(
        <OperationModesSection
          oauth2Config={{
            ...autonomousOnlyConfig,
            token: {accessToken: {defaultAudience: 'https://api.example.com'}} as OAuth2Config['token'],
          }}
        />,
      );

      expect(screen.getByDisplayValue('https://api.example.com')).toBeInTheDocument();
    });

    it('calls onOAuth2ConfigChange when the audience changes', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(<OperationModesSection oauth2Config={autonomousOnlyConfig} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.type(screen.getByLabelText('Default audience (aud)'), 'a');

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          token: expect.objectContaining({
            accessToken: expect.objectContaining({defaultAudience: 'a'}) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      );
    });
  });
});
