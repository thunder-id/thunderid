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

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import McpConnectComplete, {type McpConnectCompleteProps} from '../McpConnectComplete';

const mockCopy = vi.fn().mockResolvedValue(undefined);

vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(),
}));

const {mockUseThunderID} = vi.hoisted(() => ({
  mockUseThunderID: vi.fn(),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: mockUseThunderID,
}));

const {useCopyToClipboard} = await import('@thunderid/hooks');

describe('McpConnectComplete', () => {
  const mockOnContinue = vi.fn();

  const defaultProps: McpConnectCompleteProps = {
    clientId: 'mcp-client-id-12345',
    redirectUris: ['http://127.0.0.1:8080/callback'],
    clientType: 'userDelegated',
    onContinue: mockOnContinue,
  };

  const renderComponent = (props: Partial<McpConnectCompleteProps> = {}) =>
    render(<McpConnectComplete {...defaultProps} {...props} />);

  beforeEach(() => {
    vi.clearAllMocks();
    mockCopy.mockResolvedValue(undefined);
    vi.mocked(useCopyToClipboard).mockReturnValue({
      copied: false,
      copy: mockCopy,
    });
    mockUseThunderID.mockReturnValue({
      discovery: {
        wellKnown: {
          issuer: 'https://localhost:8090',
          authorization_endpoint: 'https://localhost:8090/oauth2/authorize',
          token_endpoint: 'https://localhost:8090/oauth2/token',
        },
      },
    });
  });

  describe('shared header', () => {
    it('should render the success icon and title for the user-delegated variant', () => {
      renderComponent();

      expect(screen.getByRole('heading', {level: 1, name: /your mcp client is ready/i})).toBeInTheDocument();
      expect(screen.getByTestId('application-mcp-connect-complete-subtitle')).toHaveTextContent(
        /pre-registered credentials and endpoints/i,
      );
    });

    it('should render the warning subtitle for the machine-to-machine variant', () => {
      renderComponent({clientType: 'm2m', clientSecret: 'super-secret-value'});

      expect(screen.getByTestId('application-mcp-connect-complete-subtitle')).toHaveTextContent(
        /save your client secret now/i,
      );
    });
  });

  describe('client id', () => {
    it('should render the client id in a read-only field', () => {
      renderComponent();

      const input = screen.getByDisplayValue('mcp-client-id-12345');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('readonly');
    });

    it('should copy the client id when its copy button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('button', {name: /copy client id/i}));

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('mcp-client-id-12345');
      });
    });
  });

  describe('endpoints', () => {
    it('should render endpoint rows derived from discovery.wellKnown', () => {
      renderComponent();

      expect(screen.getByDisplayValue('https://localhost:8090')).toBeInTheDocument();
      expect(
        screen.getByDisplayValue('https://localhost:8090/.well-known/oauth-authorization-server'),
      ).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://localhost:8090/.well-known/openid-configuration')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://localhost:8090/oauth2/token')).toBeInTheDocument();
    });

    it('should hide the issuer-derived rows when issuer is not available', () => {
      mockUseThunderID.mockReturnValue({
        discovery: {
          wellKnown: {
            authorization_endpoint: 'https://localhost:8090/oauth2/authorize',
            token_endpoint: 'https://localhost:8090/oauth2/token',
          },
        },
      });

      renderComponent();

      expect(screen.queryByDisplayValue(/well-known/)).not.toBeInTheDocument();
      expect(screen.getByDisplayValue('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    });

    it('should not render any endpoint rows when discovery.wellKnown is null', () => {
      mockUseThunderID.mockReturnValue({discovery: {wellKnown: null}});

      renderComponent();

      expect(screen.queryByText('Endpoints')).not.toBeInTheDocument();
    });
  });

  describe('user-delegated variant', () => {
    it('should render the registered redirect URIs', () => {
      renderComponent({redirectUris: ['http://127.0.0.1:8080/callback', 'https://agent.example.com/oauth/cb']});

      expect(screen.getByText('http://127.0.0.1:8080/callback')).toBeInTheDocument();
      expect(screen.getByText('https://agent.example.com/oauth/cb')).toBeInTheDocument();
    });

    it('should not render a client secret field', () => {
      renderComponent();

      expect(screen.queryByText('Client Secret')).not.toBeInTheDocument();
    });

    it('should not render the app name row, even when appName is provided', () => {
      renderComponent({appName: 'My MCP App'});

      expect(screen.queryByText('App Name')).not.toBeInTheDocument();
    });

    it('should render a single "Go to application" button that calls onContinue', async () => {
      const user = userEvent.setup();
      renderComponent();

      const continueButton = screen.getByRole('button', {name: /go to application/i});
      await user.click(continueButton);

      expect(mockOnContinue).toHaveBeenCalled();
      expect(screen.queryByRole('button', {name: /copy secret/i})).not.toBeInTheDocument();
    });
  });

  describe('machine-to-machine variant', () => {
    const m2mProps: Partial<McpConnectCompleteProps> = {
      appName: 'My MCP App',
      clientType: 'm2m',
      clientSecret: 'super-secret-value',
      redirectUris: [],
    };

    it('should render the app name', () => {
      renderComponent(m2mProps);

      expect(screen.getByText('App Name')).toBeInTheDocument();
      expect(screen.getByText('My MCP App')).toBeInTheDocument();
    });

    it('should not render an app name row when appName is not provided', () => {
      renderComponent({...m2mProps, appName: undefined});

      expect(screen.queryByText('App Name')).not.toBeInTheDocument();
    });

    it('should mask the client secret by default and reveal it on toggle', async () => {
      const user = userEvent.setup();
      renderComponent(m2mProps);

      const input = screen.getByDisplayValue('super-secret-value');
      expect(input).toHaveAttribute('type', 'password');

      await user.click(screen.getByRole('button', {name: 'Toggle secret visibility'}));

      expect(input).toHaveAttribute('type', 'text');
    });

    it('should copy the client secret when its copy button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent(m2mProps);

      await user.click(screen.getByRole('button', {name: /copy client secret/i}));

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('super-secret-value');
      });
    });

    it('should render the security warning', () => {
      renderComponent(m2mProps);

      expect(screen.getAllByText(/save your client secret now/i).length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText(/you'll need to regenerate it if it's lost/i)).toBeInTheDocument();
    });

    it('should render the client_credentials token hint with code-styled tokens', () => {
      renderComponent(m2mProps);

      expect(screen.getByText('grant_type=client_credentials')).toBeInTheDocument();
      expect(screen.getByText('resource', {selector: 'code'})).toBeInTheDocument();
    });

    it('should not render the redirect URIs card', () => {
      renderComponent(m2mProps);

      expect(screen.queryByText('Registered redirect URIs')).not.toBeInTheDocument();
    });

    it('should render "Copy secret" and "Go to application" footer buttons', async () => {
      const user = userEvent.setup();
      renderComponent(m2mProps);

      const copySecretButton = screen.getByRole('button', {name: /copy secret/i});
      await user.click(copySecretButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('super-secret-value');
      });

      const continueButton = screen.getByRole('button', {name: /go to application/i});
      await user.click(continueButton);
      expect(mockOnContinue).toHaveBeenCalled();
    });

    it('should disable the "Copy secret" button once copied', () => {
      vi.mocked(useCopyToClipboard).mockReturnValue({
        copied: true,
        copy: mockCopy,
      });

      renderComponent(m2mProps);

      expect(screen.getByRole('button', {name: /copied/i})).toBeDisabled();
    });
  });
});
