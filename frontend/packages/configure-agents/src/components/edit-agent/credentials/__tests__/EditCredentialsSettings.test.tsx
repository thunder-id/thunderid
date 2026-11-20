// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {CredentialResetSectionProps} from '@thunderid/components';
import {render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, it, expect, vi} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import EditCredentialsSettings from '../EditCredentialsSettings';

const {mockMutateAsync, mockCan} = vi.hoisted(() => ({mockMutateAsync: vi.fn(), mockCan: vi.fn()}));

vi.mock('../../../../api/useUpdateAgentCredentials', () => ({default: () => ({mutateAsync: mockMutateAsync})}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useCapabilitiesContext: () => ({can: mockCan}),
}));

// The shared section is tested in @thunderid/components. This stand-in exposes what the agent hands it.
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  CredentialResetSection: ({fields, disabled, onReset, getErrorMessage}: CredentialResetSectionProps) => (
    <div data-testid="credential-reset" data-disabled={String(disabled)}>
      <button type="button" onClick={() => void onReset(fields[0], 'new-pass')}>
        reset
      </button>
      <span data-testid="reset-error">{getErrorMessage(new Error('boom'))}</span>
    </div>
  ),
}));

vi.mock('../ClientIdSection', () => ({default: () => <div data-testid="client-id" />}));
vi.mock('../ClientSecretSection', () => ({default: () => <div data-testid="client-secret" />}));
vi.mock('../CertificateSection', () => ({
  default: ({onCertificateChange}: {onCertificateChange: (cert: unknown) => void}) => (
    <button type="button" data-testid="certificate" onClick={() => onCertificateChange({type: 'jwks', value: '{}'})}>
      cert
    </button>
  ),
}));

describe('EditCredentialsSettings', () => {
  const mockAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
    inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: [], responseTypes: []} as OAuthAgentConfig}],
  };
  const mockOnFieldChange = vi.fn();
  const password = [{fieldName: 'password', label: 'Password'}];

  beforeEach(() => {
    vi.clearAllMocks();
    mockCan.mockReturnValue(true);
    mockMutateAsync.mockResolvedValue(undefined);
  });

  it('renders all sections', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: [], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('client-id')).toBeInTheDocument();
    expect(screen.getByTestId('client-secret')).toBeInTheDocument();
    expect(screen.getByTestId('certificate')).toBeInTheDocument();
  });

  it('merges certificate updates into inboundAuthConfig on field change', async () => {
    const user = userEvent.setup();
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: [], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByTestId('certificate'));

    expect(mockOnFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({
            certificate: {type: 'jwks', value: '{}'},
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('shows the credential reset when the agent type declares a credential', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: [], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.getByTestId('credential-reset')).toBeInTheDocument();
  });

  it('resets the credential of this agent through the agent update call', async () => {
    const user = userEvent.setup();
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    await user.click(screen.getByRole('button', {name: 'reset'}));

    expect(mockMutateAsync).toHaveBeenCalledWith({agentId: 'agent-1', data: {credentials: {password: 'new-pass'}}});
  });

  it('shows the agent catalog message for a failed reset', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.getByTestId('reset-error')).toHaveTextContent('Failed to update credentials. Please try again.');
  });

  it('turns the reset off when read-only', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        readOnly
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.getByTestId('credential-reset')).toHaveAttribute('data-disabled', 'true');
  });

  it('leaves the reset on when not read-only', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.getByTestId('credential-reset')).toHaveAttribute('data-disabled', 'false');
  });

  it('does not offer the administrator reset or secret regeneration for a personal agent', () => {
    render(
      <EditCredentialsSettings
        agent={{...mockAgent, kind: 'personal-assistant'}}
        editedAgent={{}}
        oauth2Config={{grantTypes: [], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.queryByTestId('credential-reset')).not.toBeInTheDocument();
    expect(screen.queryByTestId('client-secret')).not.toBeInTheDocument();
    expect(screen.getByTestId('client-id')).toBeInTheDocument();
    expect(screen.getByTestId('certificate')).toBeInTheDocument();
  });

  it('shows only the reset for an agent with no OAuth configuration', () => {
    render(
      <EditCredentialsSettings
        agent={{...mockAgent, inboundAuthConfig: []}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
        credentialFields={password}
      />,
    );

    expect(screen.getByTestId('credential-reset')).toBeInTheDocument();
    expect(screen.queryByTestId('client-id')).not.toBeInTheDocument();
    expect(screen.queryByTestId('certificate')).not.toBeInTheDocument();
  });
});
