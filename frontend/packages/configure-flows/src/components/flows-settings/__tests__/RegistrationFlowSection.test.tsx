// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Application} from '@thunderid/configure-applications';
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useGetFlows from '../../../api/useGetFlows';
import RegistrationFlowSection from '../RegistrationFlowSection';

// Mock the useGetFlows hook
vi.mock('../../../api/useGetFlows', () => ({
  default: vi.fn(),
}));

type MockedUseGetFlows = ReturnType<typeof useGetFlows>;

// Mock the Components
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  SettingsCard: ({
    title,
    description,
    enabled = false,
    onToggle = undefined,
    children,
  }: {
    title: string;
    description: string;
    enabled?: boolean;
    onToggle?: (enabled: boolean) => void;
    children: React.ReactNode;
  }) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {onToggle && (
        <button type="button" data-testid="toggle-button" onClick={() => onToggle(!enabled)}>
          Toggle: {enabled ? 'ON' : 'OFF'}
        </button>
      )}
      {children}
    </div>
  ),
}));

describe('RegistrationFlowSection', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    registrationFlowId: 'reg-flow-1',
    isRegistrationFlowEnabled: true,
  } as Application;

  const mockRegFlows = [
    {id: 'reg-flow-1', name: 'Default Registration Flow', handle: 'default-reg'},
    {id: 'reg-flow-2', name: 'Custom Registration Flow', handle: 'custom-reg'},
    {id: 'reg-flow-3', name: 'SSO Registration Flow', handle: 'sso-reg'},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the settings card with title and description', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: []},
        isLoading: false,
      } as unknown as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Sign-up Flow');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Choose the flow that handles user sign-up and account creation.',
      );
    });

    it('should render autocomplete field', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a registration flow')).toBeInTheDocument();
      expect(
        screen.getByText('Select the flow that handles user registration for this application.'),
      ).toBeInTheDocument();
    });

    it('should render toggle button', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toBeInTheDocument();
    });
  });

  describe('Loading State', () => {
    it('should show loading indicator while fetching flows', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: undefined,
        isLoading: true,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should not show loading indicator when flows are loaded', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Enable/Disable Toggle', () => {
    it('should pass enabled state from application to SettingsCard', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: ON');
    });

    it('should pass enabled state from editedApp to SettingsCard', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection
            application={mockApplication}
            editedApp={{isRegistrationFlowEnabled: false}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: OFF');
    });

    it('should default to false when isRegistrationFlowEnabled is undefined', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const appWithoutEnabled = {...mockApplication, isRegistrationFlowEnabled: undefined};

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={appWithoutEnabled} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: OFF');
    });

    it('should call onFieldChange when toggle is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      await user.click(screen.getByTestId('toggle-button'));

      expect(mockOnFieldChange).toHaveBeenCalledWith('isRegistrationFlowEnabled', false);
    });
  });

  describe('Flow Selection', () => {
    it('should display selected flow from application', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a registration flow');
      expect(input).toHaveValue('Default Registration Flow');
    });

    it('should display selected flow from editedApp over application', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection
            application={mockApplication}
            editedApp={{registrationFlowId: 'reg-flow-2'}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a registration flow');
      expect(input).toHaveValue('Custom Registration Flow');
    });

    it('should handle flow selection', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a registration flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('SSO Registration Flow')).toBeInTheDocument();
      });

      await user.click(screen.getByText('SSO Registration Flow'));

      expect(mockOnFieldChange).toHaveBeenCalledWith('registrationFlowId', 'reg-flow-3');
    });

    it('should handle clearing selection', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const clearButton = screen.getByTitle('Clear');
      await user.click(clearButton);

      expect(mockOnFieldChange).toHaveBeenCalledWith('registrationFlowId', '');
    });
  });

  describe('Flow Options Display', () => {
    it('should display flow name and handle in options', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a registration flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('Custom Registration Flow')).toBeInTheDocument();
        expect(screen.getByText('custom-reg')).toBeInTheDocument();
      });
    });

    it('should display all available flows in dropdown', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRegFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a registration flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('Default Registration Flow')).toBeInTheDocument();
        expect(screen.getByText('Custom Registration Flow')).toBeInTheDocument();
        expect(screen.getByText('SSO Registration Flow')).toBeInTheDocument();
      });
    });
  });

  describe('Empty State', () => {
    it('should handle empty flows array', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: []},
        isLoading: false,
      } as unknown as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a registration flow')).toBeInTheDocument();
    });

    it('should handle undefined flows data', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: undefined,
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RegistrationFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a registration flow')).toBeInTheDocument();
    });
  });
});
