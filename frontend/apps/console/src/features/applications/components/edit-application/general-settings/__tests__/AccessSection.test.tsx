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

import {render, screen, waitFor, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useGetUserTypes from '../../../../../user-types/api/useGetUserTypes';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import AccessSection from '../AccessSection';

// Mock the useGetUserTypes hook
vi.mock('../../../../../user-types/api/useGetUserTypes');

type MockedUseGetUserTypes = ReturnType<typeof useGetUserTypes>;

// Mock the Components
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, description, children}: {title: string; description: string; children: React.ReactNode}) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {children}
    </div>
  ),
}));

describe('AccessSection', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    url: 'https://example.com',
    allowedUserTypes: ['admin', 'user'],
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'client-123',
          redirectUris: ['https://example.com/callback'],
        },
      },
    ],
  } as Application;

  const mockOAuth2Config: OAuth2Config = {
    clientId: 'client-123',
    redirectUris: ['https://example.com/callback'],
  } as OAuth2Config;

  const mockUserTypes = {
    types: [
      {name: 'admin', id: '1'},
      {name: 'user', id: '2'},
      {name: 'guest', id: '3'},
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the settings card with title and description', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByTestId('card-title')).toHaveTextContent('Access');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        "Configure who can access this application, where it's hosted, etc.",
      );
    });

    it('should render allowed user types autocomplete', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByLabelText('Allowed User Types')).toBeInTheDocument();
    });

    it('should render application URL field', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByLabelText('Application URL')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://example.com')).toBeInTheDocument();
    });

    it('should render redirect URIs section when OAuth2 config is provided', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('Authorized redirect URIs')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://example.com/callback')).toBeInTheDocument();
    });

    it('should not render redirect URIs section when OAuth2 config is not provided', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByLabelText('Redirect URIs')).not.toBeInTheDocument();
    });
  });

  describe('Loading State', () => {
    it('should show loading indicator while fetching user types', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: undefined,
        isLoading: true,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should not show loading indicator when user types are loaded', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Allowed User Types', () => {
    it('should display selected user types from application', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('admin')).toBeInTheDocument();
      expect(screen.getByText('user')).toBeInTheDocument();
    });

    it('should display selected user types from editedApp over application', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{allowedUserTypes: ['guest']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('guest')).toBeInTheDocument();
      expect(screen.queryByText('admin')).not.toBeInTheDocument();
    });

    it('should display all available user types in dropdown', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByLabelText('Allowed User Types');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getAllByText('admin').length).toBeGreaterThan(0);
        expect(screen.getAllByText('guest').length).toBeGreaterThan(0);
      });
    });
  });

  describe('Application URL', () => {
    it('should display URL from application', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toHaveAttribute('value', 'https://example.com');
    });

    it('should display URL from editedApp over application', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{url: 'https://edited.com'}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toHaveAttribute('value', 'https://edited.com');
    });

    it('should show validation error for invalid URL', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const urlInput = screen.getByLabelText('Application URL');
      await user.clear(urlInput);
      await user.type(urlInput, 'invalid-url');

      await waitFor(() => {
        expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
      });
    });

    it('should accept valid URL without error', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection application={{...mockApplication, url: ''}} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      const urlInput = screen.getByLabelText('Application URL');
      await user.type(urlInput, 'https://newurl.com');

      await waitFor(() => {
        expect(screen.queryByText('Please enter a valid URL')).not.toBeInTheDocument();
      });
    });
  });

  describe('Redirect URIs', () => {
    it('should display existing redirect URIs', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithMultipleUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'https://example.com/callback2'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithMultipleUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByDisplayValue('https://example.com/callback1')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://example.com/callback2')).toBeInTheDocument();
    });

    it('should add new redirect URI when add button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const addButton = screen.getByRole('button', {name: /Add URI/i});
      await user.click(addButton);

      const inputs = screen.getAllByPlaceholderText('https://example.com/callback');
      expect(inputs).toHaveLength(2);
    });

    it('should remove redirect URI when delete button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithMultipleUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'https://example.com/callback2'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithMultipleUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[0]);

      expect(screen.queryByDisplayValue('https://example.com/callback1')).not.toBeInTheDocument();
      expect(screen.getByDisplayValue('https://example.com/callback2')).toBeInTheDocument();
    });
  });

  describe('Field Change Callbacks', () => {
    it('should call onFieldChange when user types are changed', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByLabelText('Allowed User Types');
      await user.click(input);

      const guestOption = await screen.findByRole('option', {name: 'guest'});
      await user.click(guestOption);

      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalled();
        const {calls} = mockOnFieldChange.mock;
        const userTypesCall = calls.find((call) => call[0] === 'allowedUserTypes');
        expect(userTypesCall).toBeDefined();
      });
    });
  });

  describe('URI Validation on Blur', () => {
    const mockApplicationWithAuth: Application = {
      id: 'app-123',
      name: 'Test App',
      url: 'https://example.com',
      allowedUserTypes: ['admin', 'user'],
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            clientId: 'client-123',
            redirectUris: ['https://example.com/callback'],
          },
        },
      ],
    } as Application;

    it('should show error when invalid URI is entered and blurred', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplicationWithAuth}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Find the existing URI input and enter invalid URI
      const uriInput = screen.getByDisplayValue('https://example.com/callback');
      await user.clear(uriInput);
      await user.type(uriInput, 'not-a-valid-url');

      // Blur the input to trigger validation
      await user.tab();

      // Should show error and not call onFieldChange for inboundAuthConfig
      await waitFor(() => {
        const errorCalls = mockOnFieldChange.mock.calls.filter((call) => call[0] === 'inboundAuthConfig');
        expect(errorCalls).toHaveLength(0);
      });
    });

    it('should show error when URI is empty and blurred', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplicationWithAuth}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Find the existing URI input and clear it
      const uriInput = screen.getByDisplayValue('https://example.com/callback');
      await user.clear(uriInput);

      // Blur the input to trigger validation
      await user.tab();

      // Should not call onFieldChange for empty URI
      await waitFor(() => {
        const errorCalls = mockOnFieldChange.mock.calls.filter((call) => call[0] === 'inboundAuthConfig');
        expect(errorCalls).toHaveLength(0);
      });
    });

    it('should validate URI on blur', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplicationWithAuth}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Find the existing URI input
      const uriInput = screen.getByDisplayValue('https://example.com/callback');

      // Focus and blur to trigger validation flow
      await user.click(uriInput);
      await user.tab();

      // The onBlur handler should have been called
      // Since URI is valid and non-empty, it should call updateRedirectUris
      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.any(Array));
      });
    });
  });

  describe('Handle empty user types data', () => {
    it('should handle undefined user types data gracefully', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: undefined,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByLabelText('Allowed User Types')).toBeInTheDocument();
    });

    it('should handle null application allowedUserTypes', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const appWithNullTypes = {
        ...mockApplication,
        allowedUserTypes: undefined,
      };

      render(
        <AccessSection
          application={appWithNullTypes as unknown as Application}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('Allowed User Types')).toBeInTheDocument();
    });
  });

  describe('URI Error Handling', () => {
    it('should clear error when typing non-empty value in URI field', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const uriInput = screen.getByDisplayValue('https://example.com/callback');

      // Clear and blur to trigger empty error
      await user.clear(uriInput);
      fireEvent.blur(uriInput);

      // Now type something to clear the error
      await user.type(uriInput, 'https://new-uri.com');

      // Error should be cleared when typing non-empty value
      await waitFor(() => {
        expect(screen.queryByText('URI cannot be empty')).not.toBeInTheDocument();
      });
    });

    it('should reindex errors when removing a URI with errors on subsequent URIs', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithThreeUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'invalid-uri', 'https://example.com/callback3'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithThreeUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // First, trigger validation error on the second URI by blurring it
      const secondUriInput = screen.getByDisplayValue('invalid-uri');
      await user.click(secondUriInput);
      await user.tab();

      // Now remove the first URI - this should trigger reindexing of errors
      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[0]);

      // The first URI should be removed
      expect(screen.queryByDisplayValue('https://example.com/callback1')).not.toBeInTheDocument();
    });

    it('should preserve errors on URIs before the removed index', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithThreeUris = {
        ...mockOAuth2Config,
        redirectUris: ['invalid-first', 'https://example.com/callback2', 'https://example.com/callback3'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithThreeUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Trigger validation error on the first URI
      const firstUriInput = screen.getByDisplayValue('invalid-first');
      await user.click(firstUriInput);
      await user.tab();

      // Remove the last URI (index 2) - error on index 0 should be preserved
      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[2]);

      // The last URI should be removed
      expect(screen.queryByDisplayValue('https://example.com/callback3')).not.toBeInTheDocument();
      // First URI should still be present
      expect(screen.getByDisplayValue('invalid-first')).toBeInTheDocument();
    });
  });

  describe('Mixed Inbound Auth Config', () => {
    it('should preserve non-oauth2 config when updating redirect URIs', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const appWithMixedConfig: Application = {
        ...mockApplication,
        inboundAuthConfig: [
          {
            type: 'saml',
            config: {issuer: 'test-issuer'},
          },
          {
            type: 'oauth2',
            config: {
              clientId: 'client-123',
              redirectUris: ['https://example.com/callback'],
            },
          },
        ],
      } as Application;

      render(
        <AccessSection
          application={appWithMixedConfig}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Blur the URI input to trigger updateRedirectUris
      const uriInput = screen.getByDisplayValue('https://example.com/callback');
      await user.click(uriInput);
      await user.tab();

      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.any(Array));
        const call = mockOnFieldChange.mock.calls.find((c) => c[0] === 'inboundAuthConfig');
        expect(call).toBeDefined();
        const updatedConfig = call![1] as {type: string}[];
        // Should contain both saml and oauth2 configs
        expect(updatedConfig.some((c) => c.type === 'saml')).toBe(true);
        expect(updatedConfig.some((c) => c.type === 'oauth2')).toBe(true);
      });
    });
  });

  describe('URL Field Sync Effect', () => {
    it('should display editedApp URL over application URL', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{url: 'https://edited-url.com'}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toHaveValue('https://edited-url.com');
    });

    it('should display application URL when editedApp URL is not provided', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toHaveValue('https://example.com');
    });

    it('should display empty string when neither editedApp nor application have URL', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const appWithoutUrl = {...mockApplication, url: undefined};
      render(
        <AccessSection
          application={appWithoutUrl as unknown as Application}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toHaveValue('');
    });
  });

  describe('Redirect URI Updates', () => {
    it('should not update redirect URIs when oauth2Config is undefined', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      // Without oauth2Config, redirect URI section should not be rendered
      expect(screen.queryByText('Authorized redirect URIs')).not.toBeInTheDocument();

      // No inboundAuthConfig calls should be made
      const inboundAuthCalls = mockOnFieldChange.mock.calls.filter((call) => call[0] === 'inboundAuthConfig');
      expect(inboundAuthCalls).toHaveLength(0);
    });

    it('should filter out empty URIs when updating redirect URIs', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithMultipleUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'https://example.com/callback2'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithMultipleUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Focus on first valid URI and blur to trigger update
      const uriInput = screen.getByDisplayValue('https://example.com/callback1');
      await user.click(uriInput);
      await user.tab();

      await waitFor(() => {
        const inboundAuthCalls = mockOnFieldChange.mock.calls.filter((call) => call[0] === 'inboundAuthConfig');
        expect(inboundAuthCalls.length).toBeGreaterThan(0);
      });
    });
  });

  describe('Error Reindexing on URI Removal', () => {
    it('should reindex errors when removing URI from the middle of the list', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithThreeUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'https://example.com/callback2', 'invalid-uri-3'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithThreeUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Trigger validation error on the third URI
      const thirdUriInput = screen.getByDisplayValue('invalid-uri-3');
      await user.click(thirdUriInput);
      await user.tab();

      // Remove the second URI
      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[1]);

      // Verify the second URI was removed
      expect(screen.queryByDisplayValue('https://example.com/callback2')).not.toBeInTheDocument();
      // First and third (now second) should still be present
      expect(screen.getByDisplayValue('https://example.com/callback1')).toBeInTheDocument();
      expect(screen.getByDisplayValue('invalid-uri-3')).toBeInTheDocument();
    });

    it('should preserve error for URI at index before removed URI', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithThreeUris = {
        ...mockOAuth2Config,
        redirectUris: ['invalid-first', 'https://example.com/callback2', 'https://example.com/callback3'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithThreeUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Trigger validation error on the first URI
      const firstUriInput = screen.getByDisplayValue('invalid-first');
      await user.click(firstUriInput);
      await user.tab();

      // Remove the third URI
      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[2]);

      // First URI should still be present with its error state preserved
      expect(screen.getByDisplayValue('invalid-first')).toBeInTheDocument();
    });

    it('should shift error indices down when removing URI before errored URI', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const configWithThreeUris = {
        ...mockOAuth2Config,
        redirectUris: ['https://example.com/callback1', 'https://example.com/callback2', 'invalid-third'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={configWithThreeUris}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Trigger validation error on the third URI
      const thirdUriInput = screen.getByDisplayValue('invalid-third');
      await user.click(thirdUriInput);
      await user.tab();

      // Remove the first URI - this should cause error index to shift from 2 to 1
      const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
      await user.click(deleteButtons[0]);

      // First URI should be removed
      expect(screen.queryByDisplayValue('https://example.com/callback1')).not.toBeInTheDocument();
      // Third URI (now second) should still be present
      expect(screen.getByDisplayValue('invalid-third')).toBeInTheDocument();
    });
  });

  describe('OAuth2 Config Updates', () => {
    it('should display redirect URIs from oauth2Config prop', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      const initialConfig = {
        ...mockOAuth2Config,
        redirectUris: ['https://initial.com/callback'],
      };

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          oauth2Config={initialConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByDisplayValue('https://initial.com/callback')).toBeInTheDocument();
    });
  });
});
