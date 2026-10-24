// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
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

    expect(screen.getByRole('heading', {name: 'Parent organization unit'})).toBeInTheDocument();
    expect(screen.getByText('The parent organization unit in the hierarchy.')).toBeInTheDocument();
    expect(screen.getAllByText('Parent organization unit')).toHaveLength(1);
  });

  it('should identify a root organization unit when no parent exists', () => {
    const rootOU: OrganizationUnit = {
      ...mockOrganizationUnit,
      parent: null,
    };

    mockUseGetOrganizationUnit.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(<ParentSettingsSection organizationUnit={rootOU} />);

    expect(screen.getByText('This is a root organization unit.')).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
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
