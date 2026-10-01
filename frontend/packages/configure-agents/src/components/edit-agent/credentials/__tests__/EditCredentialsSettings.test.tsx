// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import EditCredentialsSettings from '../EditCredentialsSettings';

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
    name: 'Test Agent',
    inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: [], responseTypes: []} as OAuthAgentConfig}],
  };
  const mockOnFieldChange = vi.fn();

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
});
