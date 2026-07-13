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

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import McpAccessSection from '../McpAccessSection';

vi.mock('@thunderid/configure-user-types');

type MockedUseGetUserTypes = ReturnType<typeof useGetUserTypes>;

describe('McpAccessSection', () => {
  const mockOnFieldChange = vi.fn();

  const mockApplication: Application = {
    id: 'app-mcp-123',
    name: 'My MCP Client',
    url: 'https://agent.example.com',
    allowedUserTypes: ['admin'],
  } as Application;

  const buildApplication = (oauth2Config: OAuth2Config, overrides: Partial<Application> = {}): Application =>
    ({
      id: 'app-mcp-123',
      name: 'My MCP Client',
      inboundAuthConfig: [{type: 'oauth2', config: oauth2Config}],
      ...overrides,
    }) as Application;

  const baseOAuth2Config: OAuth2Config = {
    clientId: 'mcp-client-id',
    grantTypes: ['authorization_code'],
    redirectUris: ['http://127.0.0.1:8080/callback'],
  } as OAuth2Config;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetUserTypes).mockReturnValue({
      data: {
        types: [
          {name: 'admin', id: '1'},
          {name: 'user', id: '2'},
        ],
      },
      isLoading: false,
    } as unknown as MockedUseGetUserTypes);
  });

  it('renders the card title and description', () => {
    render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly={false} />);

    expect(screen.getByText('Access')).toBeInTheDocument();
  });

  it('renders the allowed user types autocomplete with the current value', () => {
    render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly={false} />);

    expect(screen.getByText('admin')).toBeInTheDocument();
  });

  it('renders the client URI field mapped to application.url', () => {
    render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly={false} />);

    expect(screen.getByDisplayValue('https://agent.example.com')).toBeInTheDocument();
  });

  it('calls onFieldChange when the client URI is edited', () => {
    render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly={false} />);

    const urlInput = screen.getByDisplayValue('https://agent.example.com');
    fireEvent.change(urlInput, {target: {value: 'https://new.example.com'}});

    expect(mockOnFieldChange).toHaveBeenCalledWith('url', 'https://new.example.com');
  });

  it('disables inputs when read-only', () => {
    render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly />);

    expect(screen.getByDisplayValue('https://agent.example.com')).toBeDisabled();
  });

  describe('Authorized redirect URIs', () => {
    it('renders the initial redirect URIs from oauth2Config', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByDisplayValue('http://127.0.0.1:8080/callback')).toBeInTheDocument();
    });

    it('renders the redirect URIs section below the client URI field', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config, {url: 'https://agent.example.com', allowedUserTypes: []})}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const clientUriInput = screen.getByDisplayValue('https://agent.example.com');
      const redirectUriInput = screen.getByDisplayValue('http://127.0.0.1:8080/callback');
      expect(clientUriInput.compareDocumentPosition(redirectUriInput) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    it('adds a new empty row when "Add redirect URI" is clicked', async () => {
      const user = userEvent.setup();
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      await user.click(screen.getByRole('button', {name: /add redirect uri/i}));

      expect(screen.getAllByPlaceholderText('https://your-app.example.com/callback')).toHaveLength(2);
    });

    it('shows a validation error and does not update on blur for an invalid URI', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const uriInput = screen.getByDisplayValue('http://127.0.0.1:8080/callback');
      fireEvent.change(uriInput, {target: {value: 'http://example.com/cb'}});
      fireEvent.blur(uriInput);

      expect(screen.getByText(/enter a valid loopback/i)).toBeInTheDocument();
      expect(mockOnFieldChange).not.toHaveBeenCalled();
    });

    it('updates via onFieldChange on blur for a valid URI', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const uriInput = screen.getByDisplayValue('http://127.0.0.1:8080/callback');
      fireEvent.change(uriInput, {target: {value: 'https://agent.example.com/callback'}});
      fireEvent.blur(uriInput);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({redirectUris: ['https://agent.example.com/callback']}) as unknown,
          }),
        ]),
      );
    });

    it('trims whitespace from the URI before persisting it on blur', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const uriInput = screen.getByDisplayValue('http://127.0.0.1:8080/callback');
      fireEvent.change(uriInput, {target: {value: '  https://agent.example.com/callback  '}});
      fireEvent.blur(uriInput);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({redirectUris: ['https://agent.example.com/callback']}) as unknown,
          }),
        ]),
      );
    });

    it('removes a row and updates via onFieldChange', () => {
      const twoUriConfig: OAuth2Config = {
        ...baseOAuth2Config,
        redirectUris: ['http://127.0.0.1:8080/callback', 'https://agent.example.com/cb'],
      } as OAuth2Config;

      render(
        <McpAccessSection
          application={buildApplication(twoUriConfig)}
          oauth2Config={twoUriConfig}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      fireEvent.click(deleteButtons[0]);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({redirectUris: ['https://agent.example.com/cb']}) as unknown,
          }),
        ]),
      );
    });

    it('disables redirect URI inputs and buttons when read-only', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly
        />,
      );

      expect(screen.getByDisplayValue('http://127.0.0.1:8080/callback')).toBeDisabled();
      expect(screen.getByRole('button', {name: /add redirect uri/i})).toBeDisabled();
    });

    it('preserves unknown oauth2Config keys when updating redirect URIs', () => {
      const configWithUnknownKey = {
        ...baseOAuth2Config,
        dpopBoundAccessTokens: true,
      } as OAuth2Config;

      render(
        <McpAccessSection
          application={buildApplication(configWithUnknownKey)}
          oauth2Config={configWithUnknownKey}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      const uriInput = screen.getByDisplayValue('http://127.0.0.1:8080/callback');
      fireEvent.change(uriInput, {target: {value: 'https://agent.example.com/callback'}});
      fireEvent.blur(uriInput);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({
              dpopBoundAccessTokens: true,
              redirectUris: ['https://agent.example.com/callback'],
            }) as unknown,
          }),
        ]),
      );
    });
  });

  describe('onValidationChange', () => {
    const mockOnValidationChange = vi.fn();

    it('reports validation errors when the redirect URI list is empty', () => {
      render(
        <McpAccessSection
          application={buildApplication({...baseOAuth2Config, redirectUris: []})}
          oauth2Config={{...baseOAuth2Config, redirectUris: []}}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      expect(mockOnValidationChange).toHaveBeenCalledWith(true);
    });

    it('reports validation errors when a redirect URI is invalid', () => {
      const invalidUriConfig: OAuth2Config = {
        ...baseOAuth2Config,
        redirectUris: ['not a url'],
      } as OAuth2Config;

      render(
        <McpAccessSection
          application={buildApplication(invalidUriConfig)}
          oauth2Config={invalidUriConfig}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      expect(mockOnValidationChange).toHaveBeenCalledWith(true);
    });

    it('reports no validation errors when there is at least one valid loopback redirect URI', () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      expect(mockOnValidationChange).toHaveBeenCalledWith(false);
    });

    it('reports no validation errors when there is at least one valid HTTPS redirect URI', () => {
      const httpsUriConfig: OAuth2Config = {
        ...baseOAuth2Config,
        redirectUris: ['https://agent.example.com/callback'],
      } as OAuth2Config;

      render(
        <McpAccessSection
          application={buildApplication(httpsUriConfig)}
          oauth2Config={httpsUriConfig}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      expect(mockOnValidationChange).toHaveBeenCalledWith(false);
    });

    it('reports validation errors once the client URI becomes invalid', async () => {
      render(
        <McpAccessSection
          application={buildApplication(baseOAuth2Config)}
          oauth2Config={baseOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      const urlInput = screen.getByPlaceholderText('https://example.com');
      fireEvent.change(urlInput, {target: {value: 'not a url'}});

      await waitFor(() => {
        expect(mockOnValidationChange).toHaveBeenCalledWith(true);
      });
    });
  });

  describe('client URI validation message', () => {
    it('renders the i18n fallback message for an invalid client URI', async () => {
      render(<McpAccessSection application={mockApplication} onFieldChange={mockOnFieldChange} isReadOnly={false} />);

      const urlInput = screen.getByPlaceholderText('https://example.com');
      fireEvent.change(urlInput, {target: {value: 'not a url'}});

      await waitFor(() => {
        expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
      });
    });
  });
});
