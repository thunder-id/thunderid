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
  const mockOnCopyToClipboard = vi.fn();

  it('renders all sections', () => {
    render(
      <EditCredentialsSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: [], responseTypes: []}}
        copiedField={null}
        onCopyToClipboard={mockOnCopyToClipboard}
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
        copiedField={null}
        onCopyToClipboard={mockOnCopyToClipboard}
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
