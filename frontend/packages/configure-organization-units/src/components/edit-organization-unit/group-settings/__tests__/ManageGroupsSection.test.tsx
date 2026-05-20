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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Group} from '../../../../models/group';
import ManageGroupsSection from '../ManageGroupsSection';

// Mock the useGetOrganizationUnitGroups hook
const mockUseGetOrganizationUnitGroups = vi.fn();
vi.mock('@/api/useGetOrganizationUnitGroups', () => ({
  default: (id: string): unknown => mockUseGetOrganizationUnitGroups(id),
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
        'organizationUnits:edit.groups.sections.manage.title': 'Manage Groups',
        'organizationUnits:edit.groups.sections.manage.description': 'View and manage groups in this organization unit',
        'organizationUnits:edit.groups.sections.manage.listing.columns.name': 'Group Name',
        'organizationUnits:edit.groups.sections.manage.listing.columns.id': 'Group ID',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('ManageGroupsSection', () => {
  const mockGroups: Group[] = [
    {id: 'group-1', name: 'Developers', ouId: 'ou-123'},
    {id: 'group-2', name: 'Designers', ouId: 'ou-123'},
    {id: 'group-3', name: 'Product Managers', ouId: 'ou-123'},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the manage groups section', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: mockGroups},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getByText('Manage Groups')).toBeInTheDocument();
    expect(screen.getByText('View and manage groups in this organization unit')).toBeInTheDocument();
  });

  it('should render data grid with groups', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: mockGroups},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.getByText('Developers')).toBeInTheDocument();
    expect(screen.getByText('Designers')).toBeInTheDocument();
    expect(screen.getByText('Product Managers')).toBeInTheDocument();
  });

  it('should render column headers', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: mockGroups},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getByText('Group Name')).toBeInTheDocument();
    expect(screen.getByText('Group ID')).toBeInTheDocument();
  });

  it('should show loading state', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: null,
      isLoading: true,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
    // DataGrid shows loading overlay when isLoading is true
  });

  it('should handle empty groups list', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: []},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
    // Grid should show "No rows" message
  });

  it('should handle null groups data', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should call useGetOrganizationUnitGroups with correct ID', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: mockGroups},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-456" />);

    expect(mockUseGetOrganizationUnitGroups).toHaveBeenCalledWith('ou-456');
  });

  it('should render group IDs correctly', () => {
    mockUseGetOrganizationUnitGroups.mockReturnValue({
      data: {groups: mockGroups},
      isLoading: false,
    });

    renderWithProviders(<ManageGroupsSection organizationUnitId="ou-123" />);

    expect(screen.getAllByText('group-1').length).toBeGreaterThan(0);
    expect(screen.getAllByText('group-2').length).toBeGreaterThan(0);
    expect(screen.getAllByText('group-3').length).toBeGreaterThan(0);
  });
});
