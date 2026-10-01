// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useGetFlows from '../../api/useGetFlows';
import OrganizationUnitDefaultFlowsSettings from '../OrganizationUnitDefaultFlowsSettings';

vi.mock('../../api/useGetFlows');

type MockedUseGetFlows = ReturnType<typeof useGetFlows>;

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
        <input
          type="checkbox"
          aria-label={`toggle-${title}`}
          checked={enabled}
          onChange={(e) => onToggle(e.target.checked)}
        />
      )}
      {(!onToggle || enabled) && children}
    </div>
  ),
}));

describe('OrganizationUnitDefaultFlowsSettings', () => {
  const mockOnFieldChange = vi.fn();
  const mockOrganizationUnit: OrganizationUnit = {
    id: 'ou-123',
    name: 'Test OU',
    authFlowId: 'auth-flow-1',
  } as OrganizationUnit;

  const mockFlows = [
    {id: 'auth-flow-1', name: 'Default Auth Flow', handle: 'default-auth'},
    {id: 'auth-flow-2', name: 'Custom Auth Flow', handle: 'custom-auth'},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetFlows).mockReturnValue({
      data: {flows: mockFlows},
      isLoading: false,
    } as MockedUseGetFlows);
  });

  it('should render all four flow sections', () => {
    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={mockOrganizationUnit}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('Sign-in Flow')).toBeInTheDocument();
    expect(screen.getByText('Sign-up Flow')).toBeInTheDocument();
    expect(screen.getByText('Recovery Flow')).toBeInTheDocument();
    expect(screen.getByText('Sign Out Flow')).toBeInTheDocument();
  });

  it('should not render a toggle for Sign In or Sign Out', () => {
    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={mockOrganizationUnit}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    expect(screen.queryByLabelText('toggle-Sign-in Flow')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('toggle-Sign Out Flow')).not.toBeInTheDocument();
  });

  it('should render a toggle for Sign Up and Recovery bound to their enabled fields', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={{...mockOrganizationUnit, isRegistrationFlowEnabled: false}}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    const registrationToggle = screen.getByLabelText('toggle-Sign-up Flow');
    await user.click(registrationToggle);

    expect(mockOnFieldChange).toHaveBeenCalledWith('isRegistrationFlowEnabled', true);
  });

  it('should display the info banner when a flow is selected', () => {
    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={mockOrganizationUnit}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    expect(screen.getAllByRole('alert').length).toBeGreaterThan(0);
  });

  it('should not display the info banner when no flow is selected', () => {
    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={{...mockOrganizationUnit, authFlowId: undefined}}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('should link the info banner to the selected flow and the flows list', () => {
    const {container} = render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={mockOrganizationUnit}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    const links = container.querySelectorAll('a');
    const editLink = Array.from(links).find((link) => link.getAttribute('href') === '/flows/auth-flow-1');
    const createLink = Array.from(links).find((link) => link.getAttribute('href') === '/flows');

    expect(editLink).toBeInTheDocument();
    expect(createLink).toBeInTheDocument();
  });

  it('should update the sign-in flow selection', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <OrganizationUnitDefaultFlowsSettings
          organizationUnit={mockOrganizationUnit}
          editedOU={{}}
          onFieldChange={mockOnFieldChange}
        />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText('Select an authentication flow');
    await user.click(input);
    await user.click(screen.getByText('Custom Auth Flow'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('authFlowId', 'auth-flow-2');
  });
});
