// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Application} from '@thunderid/configure-applications';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AccessSection from '../AccessSection';

// Mock the useGetUserTypes hook
vi.mock('@thunderid/configure-user-types');

type MockedUseGetUserTypes = ReturnType<typeof useGetUserTypes>;

// Mock the Components
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({
    title,
    description,
    children,
    headerAction = undefined,
  }: {
    title: string;
    description: string;
    children: React.ReactNode;
    headerAction?: React.ReactNode;
  }) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {headerAction}
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
    it('should render the Allowed User Types and Application Access cards', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const titles = screen.getAllByTestId('card-title').map((el) => el.textContent);
      expect(titles).toEqual(['Allowed User Types', 'Agent Sign-In', 'Application Access']);
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

    it('hides the Allowed User Types card when user access config is disabled', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          showUserAccessConfig={false}
        />,
      );

      expect(screen.queryByLabelText('Allowed User Types')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Enable Agent Sign-In')).not.toBeInTheDocument();
      // The generic application URL field stays visible for every client type.
      expect(screen.getByLabelText('Application URL')).toBeInTheDocument();
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

  describe('Agent Sign-In', () => {
    const agentSignInToggle = (): HTMLElement => screen.getByLabelText('Enable Agent Sign-In');

    beforeEach(() => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);
    });

    it('should render the toggle off when no agent type is allowed', () => {
      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(agentSignInToggle()).not.toBeChecked();
    });

    it('should render the toggle on when an agent type is allowed', () => {
      render(
        <AccessSection
          application={{...mockApplication, allowedAgentTypes: ['default']}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(agentSignInToggle()).toBeChecked();
    });

    it('should prefer the agent types from editedApp over application', () => {
      render(
        <AccessSection
          application={{...mockApplication, allowedAgentTypes: ['default']}}
          editedApp={{allowedAgentTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(agentSignInToggle()).not.toBeChecked();
    });

    it('should allow the default agent type when enabled', async () => {
      const user = userEvent.setup();

      render(<AccessSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      await user.click(agentSignInToggle());

      expect(mockOnFieldChange).toHaveBeenCalledWith('allowedAgentTypes', ['default']);
    });

    it('should clear every allowed agent type when disabled', async () => {
      const user = userEvent.setup();

      render(
        <AccessSection
          application={{...mockApplication, allowedAgentTypes: ['default']}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(agentSignInToggle());

      expect(mockOnFieldChange).toHaveBeenCalledWith('allowedAgentTypes', []);
    });

    it('should disable the toggle for a read-only application', () => {
      render(
        <AccessSection
          application={{...mockApplication, isReadOnly: true}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(agentSignInToggle()).toBeDisabled();
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

  describe('Validation Change Callback', () => {
    it('should notify parent with no errors for a valid URL', async () => {
      const mockOnValidationChange = vi.fn();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      await waitFor(() => {
        expect(mockOnValidationChange).toHaveBeenCalledWith(false);
      });
    });

    it('should notify parent with errors when the URL becomes invalid', async () => {
      const user = userEvent.setup();
      const mockOnValidationChange = vi.fn();
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(
        <AccessSection
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      const urlInput = screen.getByLabelText('Application URL');
      await user.clear(urlInput);
      await user.type(urlInput, 'invalid-url');

      await waitFor(() => {
        expect(mockOnValidationChange).toHaveBeenCalledWith(true);
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

  describe('Read-Only State', () => {
    const readOnlyApplication: Application = {
      ...mockApplication,
      isReadOnly: true,
    } as Application;

    it('should disable all inputs when application.isReadOnly is true', () => {
      vi.mocked(useGetUserTypes).mockReturnValue({
        data: mockUserTypes,
        isLoading: false,
      } as unknown as MockedUseGetUserTypes);

      render(<AccessSection application={readOnlyApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      // Application URL input should be disabled
      const urlInput = screen.getByLabelText('Application URL');
      expect(urlInput).toBeDisabled();

      // Allowed User Types autocomplete input should be disabled
      const autocompleteInput = screen.getByLabelText('Allowed User Types');
      expect(autocompleteInput).toBeDisabled();
    });
  });
});
