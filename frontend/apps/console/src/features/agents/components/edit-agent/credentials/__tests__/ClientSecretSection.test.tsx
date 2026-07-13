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
import type {OAuthAgentConfig} from '../../../../models/agent';
import ClientSecretSection from '../ClientSecretSection';

vi.mock('react-i18next', () => ({
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

describe('ClientSecretSection', () => {
  it('shows the regenerate button for confidential clients', () => {
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    expect(screen.getByRole('button', {name: /regenerate secret/i})).toBeInTheDocument();
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

    await user.click(screen.getByRole('button', {name: /regenerate secret/i}));

    expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();
  });

  it('shows the new secret dialog after regeneration succeeds', async () => {
    const user = userEvent.setup();
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} />);

    await user.click(screen.getByRole('button', {name: /regenerate secret/i}));
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

    await user.click(screen.getByRole('button', {name: /regenerate secret/i}));
    await user.click(screen.getByRole('button', {name: /confirm regenerate/i}));
    await user.click(screen.getByRole('button', {name: /close/i}));

    expect(screen.queryByTestId('secret-success-dialog')).not.toBeInTheDocument();
  });

  it('disables the regenerate button when disabled', () => {
    const oauth2Config = {
      clientId: 'client-123',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuthAgentConfig;
    render(<ClientSecretSection agentId="agent-1" oauth2Config={oauth2Config} disabled />);

    expect(screen.getByRole('button', {name: /regenerate secret/i})).toBeDisabled();
  });
});
