// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseQueryResult} from '@tanstack/react-query';
import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useGetThemes, type ThemeListResponse} from '@thunderid/design';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AppearanceSection from '../AppearanceSection';
import type {Application} from '@thunderid/configure-applications';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@thunderid/design', () => ({
  useGetThemes: vi.fn(),
  useGetLayouts: vi.fn(() => ({data: undefined, isLoading: false})),
}));

describe('AppearanceSection', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    themeId: 'theme-1',
  } as Application;

  const mockThemes = [
    {id: 'theme-1', displayName: 'Default Theme'},
    {id: 'theme-2', displayName: 'Dark Theme'},
    {id: 'theme-3', displayName: 'Light Theme'},
  ];

  const mockOnFieldChange = vi.fn();

  beforeEach(() => {
    mockOnFieldChange.mockClear();
    vi.mocked(useGetThemes).mockReturnValue({
      data: {themes: mockThemes},
      isLoading: false,
    } as UseQueryResult<ThemeListResponse>);
  });

  describe('Rendering', () => {
    it('should render the appearance section', () => {
      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.sections.appearance')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.appearance.description')).toBeInTheDocument();
    });

    it('should render theme autocomplete field', () => {
      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.labels.theme')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('applications:edit.customization.theme.placeholder')).toBeInTheDocument();
    });

    it('should display helper text', () => {
      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.theme.hint')).toBeInTheDocument();
    });
  });

  describe('Loading State', () => {
    it('should show loading indicator when themes are loading', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: undefined,
        isLoading: true,
      } as UseQueryResult<ThemeListResponse>);

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should not show loading indicator when themes are loaded', () => {
      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Theme Selection', () => {
    it('should display current theme from application', () => {
      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(input).toHaveValue('Default Theme');
    });

    it('should prioritize editedApp themeId over application', () => {
      const editedApp = {
        themeId: 'theme-2',
      };

      render(
        <AppearanceSection application={mockApplication} editedApp={editedApp} onFieldChange={mockOnFieldChange} />,
      );

      const input = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(input).toHaveValue('Dark Theme');
    });

    it('should show all available themes in dropdown', async () => {
      const user = userEvent.setup();

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const autocomplete = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      expect(within(listbox).getByText('Default Theme')).toBeInTheDocument();
      expect(within(listbox).getByText('Dark Theme')).toBeInTheDocument();
      expect(within(listbox).getByText('Light Theme')).toBeInTheDocument();
    });

    it('should call onFieldChange when theme is changed', async () => {
      const user = userEvent.setup();

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const autocomplete = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const darkThemeOption = within(listbox).getByText('Dark Theme');
      await user.click(darkThemeOption);

      expect(mockOnFieldChange).toHaveBeenCalledWith('themeId', 'theme-2');
    });

    it('should handle clearing theme selection', async () => {
      const user = userEvent.setup();

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const autocomplete = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      const clearButton = autocomplete.parentElement?.querySelector('[aria-label="Clear"]');

      expect(clearButton).not.toBeNull();
      await user.click(clearButton!);
      expect(mockOnFieldChange).toHaveBeenCalledWith('themeId', '');
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing themeId in application', () => {
      const appWithoutTheme: Partial<Application> = {...mockApplication};
      delete appWithoutTheme.themeId;

      render(
        <AppearanceSection
          application={appWithoutTheme as Application}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const input = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(input).toHaveValue('');
    });

    it('should handle empty themes list', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: []},
        isLoading: false,
      } as unknown as UseQueryResult<ThemeListResponse>);

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByPlaceholderText('applications:edit.customization.theme.placeholder')).toBeInTheDocument();
    });

    it('should handle undefined themes data', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: undefined,
        isLoading: false,
      } as UseQueryResult<ThemeListResponse>);

      render(<AppearanceSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByPlaceholderText('applications:edit.customization.theme.placeholder')).toBeInTheDocument();
    });

    it('should handle themeId not found in themes list', () => {
      const appWithInvalidTheme = {...mockApplication, themeId: 'non-existent-id'};

      render(<AppearanceSection application={appWithInvalidTheme} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(input).toHaveValue('');
    });
  });
});
