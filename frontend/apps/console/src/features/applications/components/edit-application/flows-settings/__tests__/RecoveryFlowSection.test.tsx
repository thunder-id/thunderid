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
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useGetFlows from '../../../../../flows/api/useGetFlows';
import type {Application} from '../../../../models/application';
import RecoveryFlowSection from '../RecoveryFlowSection';

// Mock the useGetFlows hook
vi.mock('../../../../../flows/api/useGetFlows');

type MockedUseGetFlows = ReturnType<typeof useGetFlows>;

// Mock the Components
vi.mock('@thunderid/components', () => ({
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

describe('RecoveryFlowSection', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    recoveryFlowId: 'rec-flow-1',
    isRecoveryFlowEnabled: true,
  } as Application;

  const mockRecoveryFlows = [
    {id: 'rec-flow-1', name: 'Default Recovery Flow', handle: 'default-rec'},
    {id: 'rec-flow-2', name: 'Custom Recovery Flow', handle: 'custom-rec'},
    {id: 'rec-flow-3', name: 'SMS Recovery Flow', handle: 'sms-rec'},
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
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('card-title')).toHaveTextContent('Recovery Flow');
      expect(screen.getByTestId('card-description')).toHaveTextContent(
        'Choose the flow that handles password and account recovery.',
      );
    });

    it('should render autocomplete field', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a recovery flow')).toBeInTheDocument();
      expect(
        screen.getByText('Select the flow that handles account recovery for this application.'),
      ).toBeInTheDocument();
    });

    it('should render toggle button', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toBeInTheDocument();
    });

    it('should display alert when recovery flow is selected', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('should not display alert when no recovery flow is selected', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const appWithoutFlow = {...mockApplication, recoveryFlowId: undefined};

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={appWithoutFlow} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should display alert when recovery flow is in editedApp', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const appWithoutFlow = {...mockApplication, recoveryFlowId: undefined};

      render(
        <MemoryRouter>
          <RecoveryFlowSection
            application={appWithoutFlow}
            editedApp={{recoveryFlowId: 'rec-flow-2'}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('should use custom entityLabel in hint text', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection
            application={mockApplication}
            editedApp={{}}
            onFieldChange={mockOnFieldChange}
            entityLabel="organization"
          />
        </MemoryRouter>,
      );

      expect(
        screen.getByText('Select the flow that handles account recovery for this organization.'),
      ).toBeInTheDocument();
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
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should not show loading indicator when flows are loaded', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Enable/Disable Toggle', () => {
    it('should pass enabled state from application to SettingsCard', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: ON');
    });

    it('should pass enabled state from editedApp to SettingsCard', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection
            application={mockApplication}
            editedApp={{isRecoveryFlowEnabled: false}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: OFF');
    });

    it('should default to false when isRecoveryFlowEnabled is undefined', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const appWithoutEnabled = {...mockApplication, isRecoveryFlowEnabled: undefined};

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={appWithoutEnabled} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: OFF');
    });

    it('should call onFieldChange when toggle is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      await user.click(screen.getByTestId('toggle-button'));

      expect(mockOnFieldChange).toHaveBeenCalledWith('isRecoveryFlowEnabled', false);
    });
  });

  describe('Flow Selection', () => {
    it('should display selected flow from application', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a recovery flow');
      expect(input).toHaveValue('Default Recovery Flow');
    });

    it('should display selected flow from editedApp over application', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection
            application={mockApplication}
            editedApp={{recoveryFlowId: 'rec-flow-2'}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a recovery flow');
      expect(input).toHaveValue('Custom Recovery Flow');
    });

    it('should handle flow selection', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a recovery flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('SMS Recovery Flow')).toBeInTheDocument();
      });

      await user.click(screen.getByText('SMS Recovery Flow'));

      expect(mockOnFieldChange).toHaveBeenCalledWith('recoveryFlowId', 'rec-flow-3');
    });

    it('should handle clearing selection', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const clearButton = screen.getByTitle('Clear');
      await user.click(clearButton);

      expect(mockOnFieldChange).toHaveBeenCalledWith('recoveryFlowId', '');
    });
  });

  describe('Flow Options Display', () => {
    it('should display flow name and handle in options', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a recovery flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('Custom Recovery Flow')).toBeInTheDocument();
        expect(screen.getByText('custom-rec')).toBeInTheDocument();
      });
    });

    it('should display all available flows in dropdown', async () => {
      const user = userEvent.setup();
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const input = screen.getByPlaceholderText('Select a recovery flow');
      await user.click(input);

      await waitFor(() => {
        expect(screen.getByText('Default Recovery Flow')).toBeInTheDocument();
        expect(screen.getByText('Custom Recovery Flow')).toBeInTheDocument();
        expect(screen.getByText('SMS Recovery Flow')).toBeInTheDocument();
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
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a recovery flow')).toBeInTheDocument();
    });

    it('should handle undefined flows data', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: undefined,
        isLoading: false,
      } as MockedUseGetFlows);

      render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      expect(screen.getByPlaceholderText('Select a recovery flow')).toBeInTheDocument();
    });
  });

  describe('Alert Links', () => {
    it('should display edit link with correct flow ID from application', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const {container} = render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const links = container.querySelectorAll('a');
      const editLink = Array.from(links).find((link) => link.getAttribute('href')?.includes('/flows/recovery/'));
      expect(editLink).toHaveAttribute('href', '/flows/recovery/rec-flow-1');
    });

    it('should display edit link with correct flow ID from editedApp', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const {container} = render(
        <MemoryRouter>
          <RecoveryFlowSection
            application={mockApplication}
            editedApp={{recoveryFlowId: 'rec-flow-2'}}
            onFieldChange={mockOnFieldChange}
          />
        </MemoryRouter>,
      );

      const links = container.querySelectorAll('a');
      const editLink = Array.from(links).find((link) => link.getAttribute('href')?.includes('/flows/recovery/'));
      expect(editLink).toHaveAttribute('href', '/flows/recovery/rec-flow-2');
    });

    it('should display create link pointing to flows page', () => {
      vi.mocked(useGetFlows).mockReturnValue({
        data: {flows: mockRecoveryFlows},
        isLoading: false,
      } as MockedUseGetFlows);

      const {container} = render(
        <MemoryRouter>
          <RecoveryFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
        </MemoryRouter>,
      );

      const links = container.querySelectorAll('a');
      const createLink = Array.from(links).find((link) => link.getAttribute('href') === '/flows');
      expect(createLink).toHaveAttribute('href', '/flows');
    });
  });
});
