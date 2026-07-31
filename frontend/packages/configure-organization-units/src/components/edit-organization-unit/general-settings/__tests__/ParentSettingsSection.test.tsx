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

import {screen, renderWithProviders, renderHook} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
import type {OrganizationUnit} from '../../../../models/organization-unit';
import ParentSettingsSection from '../ParentSettingsSection';

// Mock the useGetOrganizationUnit hook
const mockUseGetOrganizationUnit = vi.fn();
vi.mock('@/api/useGetOrganizationUnit', () => ({
  default: (id?: string, enabled?: boolean): unknown => mockUseGetOrganizationUnit(id, enabled),
}));

// Mock navigate function
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('ParentSettingsSection', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });
  const mockOrganizationUnit: OrganizationUnit = {
    id: 'ou-child-123',
    handle: 'engineering-frontend',
    name: 'Frontend Engineering',
    description: 'Frontend team',
    parent: 'ou-parent-123',
  };

  const mockParentOU: OrganizationUnit = {
    id: 'ou-parent-123',
    handle: 'engineering',
    name: 'Engineering',
    description: 'Engineering department',
    parent: null,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the parent settings section', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockParentOU,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    expect(
      screen.getByRole('heading', {name: t('organizationUnits:edit.general.sections.parentOUSettings.title')}),
    ).toBeInTheDocument();
    expect(
      screen.getByText(t('organizationUnits:edit.general.sections.parentOUSettings.description')),
    ).toBeInTheDocument();
  });

  it('should show "Root Organization Unit" when no parent exists', () => {
    const rootOU: OrganizationUnit = {
      ...mockOrganizationUnit,
      parent: null,
    };

    mockUseGetOrganizationUnit.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={rootOU} />);

    const input = screen.getByDisplayValue(t('organizationUnits:edit.general.ou.noParent.label'));
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute('readonly');
  });

  it('should show loading spinner while fetching parent', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: null,
      isLoading: true,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render parent name as link when parent is loaded', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockParentOU,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    const link = screen.getByText('Engineering');
    expect(link).toBeInTheDocument();
    expect(link.tagName).toBe('A');
    expect(link).toHaveAttribute('href', '/organization-units/ou-parent-123');
  });

  it('should render parent ID alongside parent name', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockParentOU,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    expect(screen.getByText('Engineering')).toBeInTheDocument();
    expect(screen.getByText('(ou-parent-123)')).toBeInTheDocument();
  });

  it('should include navigation state in parent link', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockParentOU,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    const link = screen.getByText('Engineering');
    const stateAttr = link.getAttribute('data-state') ?? '{}';
    const state: unknown = JSON.parse(stateAttr);
    expect(state).toEqual({
      fromOU: {
        id: 'ou-child-123',
        name: 'Frontend Engineering',
      },
    });
  });

  it('should show raw parent ID when parent cannot be loaded', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    const input = screen.getByDisplayValue('ou-parent-123');
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute('readonly');
  });

  it('should not fetch parent when parent is null', () => {
    const rootOU: OrganizationUnit = {
      ...mockOrganizationUnit,
      parent: null,
    };

    renderWithProviders(<ParentSettingsSection organizationUnit={rootOU} />);

    expect(mockUseGetOrganizationUnit).toHaveBeenCalledWith(undefined, false);
  });

  it('should fetch parent when parent ID exists', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockParentOU,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={mockOrganizationUnit} />);

    expect(mockUseGetOrganizationUnit).toHaveBeenCalledWith('ou-parent-123', true);
  });
});
