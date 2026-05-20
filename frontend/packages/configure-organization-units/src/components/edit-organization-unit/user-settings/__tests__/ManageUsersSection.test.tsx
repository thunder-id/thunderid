/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {screen, renderWithProviders} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ManageUsersSection from '../ManageUsersSection';

// Mock the useGetOrganizationUnitUsers hook
const mockUseGetOrganizationUnitUsers = vi.fn();
vi.mock('@/api/useGetOrganizationUnitUsers', () => ({
  default: (id: string): unknown => mockUseGetOrganizationUnitUsers(id),
}));

// Mock useDataGridLocaleText hook
vi.mock('@thunderid/hooks', async (importOriginal) => {
  const actual = await importOriginal();
  return {...(actual as object), useDataGridLocaleText: () => ({})};
});

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'organizationUnits:edit.users.sections.manage.title': 'Manage Users',
        'organizationUnits:edit.users.sections.manage.description': 'View and manage users in this organization unit',
        'organizationUnits:edit.users.sections.manage.listing.columns.name': 'Display Name',
        'organizationUnits:edit.users.sections.manage.listing.columns.id': 'User ID',
        'organizationUnits:edit.users.sections.manage.listing.columns.type': 'Type',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('ManageUsersSection', () => {
  const mockUsers: User[] = [
    {id: 'user-1', type: 'internal', ouId: 'ou-123', display: 'John Doe'},
    {id: 'user-2', type: 'external', ouId: 'ou-123', display: 'Jane Smith'},
    {id: 'user-3', type: 'internal', ouId: 'ou-123'},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the manage users section', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByText('Manage Users')).toBeInTheDocument();
    expect(screen.getByText('View and manage users in this organization unit')).toBeInTheDocument();
  });

  it('should render data grid with users showing display names', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.getByText('John Doe')).toBeInTheDocument();
    expect(screen.getByText('Jane Smith')).toBeInTheDocument();
    // user-3 has no display, should fall back to id
    expect(screen.getAllByText('user-3').length).toBeGreaterThanOrEqual(1);
  });

  it('should show initials in avatar', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByText('JD')).toBeInTheDocument();
    expect(screen.getByText('JS')).toBeInTheDocument();
    // user-3 falls back to id 'user-3', initials would be 'US'
    expect(screen.getByText('US')).toBeInTheDocument();
  });

  it('should render column headers', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByText('Display Name')).toBeInTheDocument();
    expect(screen.getByText('User ID')).toBeInTheDocument();
    expect(screen.getByText('Type')).toBeInTheDocument();
  });

  it('should show loading state', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: null,
      isLoading: true,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
  });

  it('should handle empty users list', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: []},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should handle null users data', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should call useGetOrganizationUnitUsers with correct ID', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-456" />);

    expect(mockUseGetOrganizationUnitUsers).toHaveBeenCalledWith('ou-456');
  });

  it('should render user type correctly', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    const internalCells = screen.getAllByText('internal');
    const externalCells = screen.getAllByText('external');

    expect(internalCells.length).toBeGreaterThan(0);
    expect(externalCells.length).toBeGreaterThan(0);
  });

  it('should show user IDs in the grid', () => {
    mockUseGetOrganizationUnitUsers.mockReturnValue({
      data: {users: mockUsers},
      isLoading: false,
    });

    renderWithProviders(<ManageUsersSection organizationUnitId="ou-123" />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
    expect(screen.getByText('user-2')).toBeInTheDocument();
  });
});
