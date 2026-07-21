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

import {screen, fireEvent, waitFor, renderWithProviders, renderHook} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
import type {OrganizationUnit} from '../../../../models/organization-unit';
import AppearanceSection from '../AppearanceSection';

// Mock useGetThemes hook
const mockUseGetThemes = vi.fn();
vi.mock('@thunderid/design', () => ({
  useGetThemes: (): unknown => mockUseGetThemes(),
}));

describe('AppearanceSection', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });
  const mockOrganizationUnit: OrganizationUnit = {
    id: 'ou-123',
    handle: 'engineering',
    name: 'Engineering',
    description: 'Engineering department',
    parent: null,
    themeId: 'default-theme',
  };

  const mockThemes = [
    {id: 'default-theme', displayName: 'Default Theme'},
    {id: 'dark-theme', displayName: 'Dark Theme'},
    {id: 'light-theme', displayName: 'Light Theme'},
  ];

  const mockOnFieldChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the appearance section', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByText(t('organizationUnits:edit.customization.sections.appearance'))).toBeInTheDocument();
    expect(
      screen.getByText(t('organizationUnits:edit.customization.sections.appearance.description')),
    ).toBeInTheDocument();
  });

  it('should render theme label', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByText(t('organizationUnits:edit.customization.labels.theme'))).toBeInTheDocument();
  });

  it('should show loading spinner when themes are loading', () => {
    mockUseGetThemes.mockReturnValue({
      data: null,
      isLoading: true,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render autocomplete with theme options', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByPlaceholderText(t('organizationUnits:edit.customization.theme.placeholder'));
    expect(autocomplete).toBeInTheDocument();
  });

  it('should display current theme from organizationUnit', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByDisplayValue('Default Theme')).toBeInTheDocument();
  });

  it('should display edited theme when available', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    const editedOU: Partial<OrganizationUnit> = {
      themeId: 'dark-theme',
    };

    renderWithProviders(
      <AppearanceSection
        organizationUnit={mockOrganizationUnit}
        editedOU={editedOU}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByDisplayValue('Dark Theme')).toBeInTheDocument();
  });

  it('should call onFieldChange when theme is selected', async () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByRole('combobox');
    fireEvent.mouseDown(autocomplete); // MUI Autocomplete usually responds to mouseDown to open

    await waitFor(() => {
      expect(screen.getByText('Light Theme')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Light Theme'));

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenCalledWith('themeId', 'light-theme');
    });
  });

  it('should call onFieldChange with empty string when theme is cleared', async () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByPlaceholderText(t('organizationUnits:edit.customization.theme.placeholder'));
    const clearButton = autocomplete.parentElement?.querySelector('[title="Clear"]');

    expect(clearButton).toBeTruthy();
    fireEvent.click(clearButton!);

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenCalledWith('themeId', '');
    });
  });

  it('should handle empty themes list', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: []},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByPlaceholderText(t('organizationUnits:edit.customization.theme.placeholder'));
    expect(autocomplete).toBeInTheDocument();
  });

  it('should handle null themes data', () => {
    mockUseGetThemes.mockReturnValue({
      data: null,
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByPlaceholderText(t('organizationUnits:edit.customization.theme.placeholder'));
    expect(autocomplete).toBeInTheDocument();
  });

  it('should render helper text', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByText(t('organizationUnits:edit.customization.theme.hint'))).toBeInTheDocument();
  });

  it('should handle getOptionLabel with string values', async () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    });

    renderWithProviders(
      <AppearanceSection organizationUnit={mockOrganizationUnit} editedOU={{}} onFieldChange={mockOnFieldChange} />,
    );

    const autocomplete = screen.getByRole('combobox');
    expect(autocomplete).toBeInTheDocument();
    // Verify the component handles both string and object option types
    fireEvent.mouseDown(autocomplete);

    await waitFor(() => {
      expect(screen.getByText('Default Theme')).toBeInTheDocument();
      expect(screen.getByText('Dark Theme')).toBeInTheDocument();
      expect(screen.getByText('Light Theme')).toBeInTheDocument();
    });
  });

  it('should handle when theme cannot be found in empty options list', () => {
    mockUseGetThemes.mockReturnValue({
      data: {themes: []},
      isLoading: false,
    });

    const editedOU: Partial<OrganizationUnit> = {
      themeId: 'non-existent-theme',
    };

    renderWithProviders(
      <AppearanceSection
        organizationUnit={mockOrganizationUnit}
        editedOU={editedOU}
        onFieldChange={mockOnFieldChange}
      />,
    );

    const autocomplete = screen.getByPlaceholderText(t('organizationUnits:edit.customization.theme.placeholder'));
    expect(autocomplete).toBeInTheDocument();
  });
});
