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
import SignOutFlowSection from '../SignOutFlowSection';

// Mock the useGetFlows hook
vi.mock('../../../../../flows/api/useGetFlows');

type MockedUseGetFlows = ReturnType<typeof useGetFlows>;

// Mock the SettingsCard so the toggle is a simple button
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

describe('SignOutFlowSection', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    signOutFlowId: 'signout-flow-1',
    isSignOutFlowEnabled: true,
  } as Application;

  const mockSignOutFlows = [
    {id: 'signout-flow-1', name: 'Default SignOut Flow', handle: 'default-signout'},
    {id: 'signout-flow-2', name: 'Custom SignOut Flow', handle: 'custom-signout'},
  ];

  const mockFlows = (flows: unknown[], isLoading = false): void => {
    vi.mocked(useGetFlows).mockReturnValue({data: {flows}, isLoading} as unknown as MockedUseGetFlows);
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should query SIGNOUT flows', () => {
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
      </MemoryRouter>,
    );
    expect(useGetFlows).toHaveBeenCalledWith({flowType: 'SIGNOUT'});
  });

  it('should render the autocomplete and toggle', () => {
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
      </MemoryRouter>,
    );
    expect(screen.getByPlaceholderText('Select a signout flow')).toBeInTheDocument();
    expect(screen.getByTestId('toggle-button')).toHaveTextContent('Toggle: ON');
  });

  it('should show a loading indicator while fetching flows', () => {
    mockFlows([], true);
    render(
      <MemoryRouter>
        <SignOutFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
      </MemoryRouter>,
    );
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should display the selected flow, preferring editedApp over application', () => {
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection
          application={mockApplication}
          editedApp={{signOutFlowId: 'signout-flow-2'}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );
    expect(screen.getByPlaceholderText('Select a signout flow')).toHaveValue('Custom SignOut Flow');
  });

  it('should show the info alert only when a signout flow is selected', () => {
    mockFlows(mockSignOutFlows);
    const {rerender} = render(
      <MemoryRouter>
        <SignOutFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
      </MemoryRouter>,
    );
    expect(screen.getByRole('alert')).toBeInTheDocument();

    rerender(
      <MemoryRouter>
        <SignOutFlowSection
          application={{...mockApplication, signOutFlowId: undefined}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('should call onFieldChange when the toggle is clicked', async () => {
    const user = userEvent.setup();
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />
      </MemoryRouter>,
    );
    await user.click(screen.getByTestId('toggle-button'));
    expect(mockOnFieldChange).toHaveBeenCalledWith('isSignOutFlowEnabled', false);
  });

  it('should call onFieldChange with the selected signout flow id', async () => {
    const user = userEvent.setup();
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection
          application={{...mockApplication, signOutFlowId: undefined, isSignOutFlowEnabled: false}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByPlaceholderText('Select a signout flow'));
    await waitFor(() => {
      expect(screen.getByText('Custom SignOut Flow')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Custom SignOut Flow'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('signOutFlowId', 'signout-flow-2');
  });

  it('should disable the picker for a read-only application', () => {
    mockFlows(mockSignOutFlows);
    render(
      <MemoryRouter>
        <SignOutFlowSection
          application={{...mockApplication, isReadOnly: true}}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );
    expect(screen.getByPlaceholderText('Select a signout flow')).toBeDisabled();
  });
});
