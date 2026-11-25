// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render as rtlRender, screen, type RenderOptions, type RenderResult} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import type {JSX, ReactElement, ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {OAuthAgentConfig} from '../../../../models/agent';
import ClientSecretSection from '../ClientSecretSection';

// The admin may do everything, so the component's actions show; a test nests its own provider to restrict them.
function AllowAll({children}: {children: ReactNode}): JSX.Element {
  return <CapabilitiesProvider capabilities={{can: () => true, isLoading: false}}>{children}</CapabilitiesProvider>;
}

const render = (ui: ReactElement, options?: RenderOptions): RenderResult =>
  rtlRender(ui, {wrapper: AllowAll, ...options});

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

vi.mock('../../../RegenerateSecretDialog', () => ({
  default: ({open, onSuccess}: {open: boolean; onSuccess: (clientSecret: string) => void}) =>
    open ? (
      <div data-testid="regenerate-dialog">
        <button type="button" onClick={() => onSuccess('new-secret-value')}>
          Confirm regenerate
        </button>
      </div>
    ) : null,
}));

vi.mock('../../../ClientSecretSuccessDialog', () => ({
  default: ({open, clientSecret, onClose}: {open: boolean; clientSecret: string; onClose: () => void}) =>
    open ? (
      <div data-testid="secret-success-dialog">
        <span data-testid="new-secret-value">{clientSecret}</span>
        <button type="button" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

describe('ClientSecretSection', () => {
  it('shows the regenerate button for confidential clients', () => {
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    expect(screen.getByRole('button', {name: /regenerate client secret/i})).toBeInTheDocument();
  });

  it('renders nothing for public clients', () => {
    const oauth2Config = {clientId: 'client-123', tokenEndpointAuthMethod: 'none'} as OAuthAgentConfig;
    const {container} = render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    expect(container.firstChild).toBeNull();
  });

  it('opens the regenerate dialog when the button is clicked', async () => {
    const user = userEvent.setup();
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    await user.click(screen.getByRole('button', {name: /regenerate client secret/i}));

    expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();
  });

  it('shows the new secret dialog after regeneration succeeds', async () => {
    const user = userEvent.setup();
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    await user.click(screen.getByRole('button', {name: /regenerate client secret/i}));
    await user.click(screen.getByRole('button', {name: /confirm regenerate/i}));

    expect(screen.getByTestId('secret-success-dialog')).toBeInTheDocument();
    expect(screen.getByTestId('new-secret-value')).toHaveTextContent('new-secret-value');
  });

  it('closes and clears the new secret when the success dialog is dismissed', async () => {
    const user = userEvent.setup();
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    await user.click(screen.getByRole('button', {name: /regenerate client secret/i}));
    await user.click(screen.getByRole('button', {name: /confirm regenerate/i}));
    await user.click(screen.getByRole('button', {name: /close/i}));

    expect(screen.queryByTestId('secret-success-dialog')).not.toBeInTheDocument();
  });

  it('disables the regenerate button when the admin may not update agents', () => {
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />
      </CapabilitiesProvider>,
    );

    expect(screen.getByRole('button', {name: /regenerate client secret/i})).toBeDisabled();
  });

  it('disables the regenerate button when disabled', () => {
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} disabled />);

    expect(screen.getByRole('button', {name: /regenerate client secret/i})).toBeDisabled();
  });
});
