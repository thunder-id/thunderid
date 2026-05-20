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

import {screen, fireEvent, waitFor, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationUnit} from '../../../../models/organization-unit';
import ManageChildOrganizationUnitSection from '../ManageChildOrganizationUnitSection';

// Mock the useGetChildOrganizationUnits hook
const mockUseGetChildOrganizationUnits = vi.fn();
vi.mock('@/api/useGetChildOrganizationUnits', () => ({
  default: (id: string): unknown => mockUseGetChildOrganizationUnits(id),
}));

// Mock useDataGridLocaleText hook
vi.mock('@thunderid/hooks', async (importOriginal) => {
  const actual = await importOriginal();
  return {...(actual as object), useDataGridLocaleText: () => ({})};
});

// Mock navigate function
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
  }),
}));

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'organizationUnits:edit.childOUs.sections.manage.title': 'Manage Child Organization Units',
        'organizationUnits:edit.childOUs.sections.manage.description': 'View and manage child organization units',
        'organizationUnits:listing.columns.name': 'Name',
        'organizationUnits:listing.columns.handle': 'Handle',
        'organizationUnits:listing.columns.description': 'Description',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('ManageChildOrganizationUnitSection', () => {
  const mockChildOUs: OrganizationUnit[] = [
    {
      id: 'ou-child-1',
      handle: 'frontend',
      name: 'Frontend Team',
      description: 'Frontend development team',
      parent: 'ou-parent',
    },
    {
      id: 'ou-child-2',
      handle: 'backend',
      name: 'Backend Team',
      description: 'Backend development team',
      parent: 'ou-parent',
    },
    {
      id: 'ou-child-3',
      handle: 'devops',
      name: 'DevOps Team',
      description: null,
      parent: 'ou-parent',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
  });

  it('should render the manage child OUs section', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('Manage Child Organization Units')).toBeInTheDocument();
    expect(screen.getByText('View and manage child organization units')).toBeInTheDocument();
  });

  it('should render data grid with child OUs', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.getByText('Frontend Team')).toBeInTheDocument();
    expect(screen.getByText('Backend Team')).toBeInTheDocument();
    expect(screen.getByText('DevOps Team')).toBeInTheDocument();
  });

  it('should render column headers', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText('Handle')).toBeInTheDocument();
    expect(screen.getByText('Description')).toBeInTheDocument();
  });

  it('should show loading state', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: null,
      isLoading: true,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
    // DataGrid shows loading overlay when isLoading is true
  });

  it('should handle empty child OUs list', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: []},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByRole('grid')).toBeInTheDocument();
    // Grid should show "No rows" message
  });

  it('should handle null child OUs data', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should call useGetChildOrganizationUnits with correct ID', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-456" organizationUnitName="Engineering" />,
    );

    expect(mockUseGetChildOrganizationUnits).toHaveBeenCalledWith('ou-456');
  });

  it('should render handles correctly', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('frontend')).toBeInTheDocument();
    expect(screen.getByText('backend')).toBeInTheDocument();
    expect(screen.getByText('devops')).toBeInTheDocument();
  });

  it('should render descriptions correctly', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('Frontend development team')).toBeInTheDocument();
    expect(screen.getByText('Backend development team')).toBeInTheDocument();
  });

  it('should show "-" for null description', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    // The third OU has null description, should show "-"
    const cells = screen.getAllByText('-');
    expect(cells.length).toBeGreaterThan(0);
  });

  it('should navigate to child OU when row is clicked', async () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    // Get the grid cell with the text "Frontend Team"
    const cell = screen.getByText('Frontend Team');
    fireEvent.click(cell);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        '/organization-units/ou-child-1',
        expect.objectContaining({
          state: {
            fromOU: {
              id: 'ou-parent',
              name: 'Engineering',
            },
          },
        }),
      );
    });
  });

  it('should include navigation state when navigating to child OU', async () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Product Team" />,
    );

    const cell = screen.getByText('Backend Team');
    fireEvent.click(cell);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        '/organization-units/ou-child-2',
        expect.objectContaining({
          state: {
            fromOU: {
              id: 'ou-parent',
              name: 'Product Team',
            },
          },
        }),
      );
    });
  });

  it('should handle navigation errors gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {organizationUnits: mockChildOUs},
      isLoading: false,
    });

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="ou-parent" organizationUnitName="Engineering" />,
    );

    const cell = screen.getByText('Frontend Team');
    fireEvent.click(cell);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalled();
    });

    // Should not throw error - error is logged
  });
});
