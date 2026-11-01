// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {useGetLanguages} from '@thunderid/i18n';
import {render, renderHook, screen, fireEvent} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TranslationsListPage from '@/pages/TranslationsListPage';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/i18n', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useGetLanguages: vi.fn(),
    useDeleteTranslations: vi.fn().mockReturnValue({mutate: vi.fn(), isPending: false}),
    getDisplayNameForCode: (code: string) => `Language(${code})`,
    toFlagEmoji: (code: string) => `Flag(${code})`,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

const mockUseGetLanguages = vi.mocked(useGetLanguages);

const rowOf = (code: string): HTMLElement => screen.getByText(`Language(${code})`).closest<HTMLElement>('tr')!;

describe('TranslationsListPage', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetLanguages.mockReturnValue({
      data: {languages: ['fr-FR', 'de-DE']},
      isLoading: false,
    } as ReturnType<typeof useGetLanguages>);
  });

  describe('Rendering', () => {
    it('renders the page title', () => {
      render(<TranslationsListPage />);

      expect(screen.getByText(t('translations:page.title'))).toBeInTheDocument();
    });

    it('renders the page subtitle', () => {
      render(<TranslationsListPage />);

      expect(screen.getByText(t('translations:page.subtitle'))).toBeInTheDocument();
    });

    it('renders the Add Language button', () => {
      render(<TranslationsListPage />);

      expect(screen.getByRole('button', {name: t('translations:listing.addLanguage')})).toBeInTheDocument();
    });

    it('renders the languages table', () => {
      render(<TranslationsListPage />);

      expect(screen.getByRole('table')).toBeInTheDocument();
    });

    it('renders a row for each language', () => {
      render(<TranslationsListPage />);

      expect(rowOf('fr-FR')).toBeInTheDocument();
      expect(rowOf('de-DE')).toBeInTheDocument();
    });

    it('does not mark the table busy once data has loaded', () => {
      render(<TranslationsListPage />);

      expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'false');
    });

    it('marks the table busy while data is loading', () => {
      mockUseGetLanguages.mockReturnValue({
        data: undefined,
        isLoading: true,
      } as ReturnType<typeof useGetLanguages>);

      render(<TranslationsListPage />);

      expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
    });

    it('renders the empty row when there are no languages', () => {
      mockUseGetLanguages.mockReturnValue({
        data: {languages: []},
        isLoading: false,
      } as unknown as ReturnType<typeof useGetLanguages>);

      render(<TranslationsListPage />);

      expect(screen.queryByText(/Language\(/)).not.toBeInTheDocument();
      expect(screen.getByText(t('common:dataTable.noRowsLabel'))).toBeInTheDocument();
    });
  });

  describe('Navigation', () => {
    it('navigates to /translations/create when Add Language is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsListPage />);

      await user.click(screen.getByRole('button', {name: t('translations:listing.addLanguage')}));

      expect(mockNavigate).toHaveBeenCalledWith('/translations/create');
    });

    it('navigates to the language edit page when a row is clicked', () => {
      render(<TranslationsListPage />);

      fireEvent.click(rowOf('fr-FR'));

      expect(mockNavigate).toHaveBeenCalledWith('/translations/fr-FR');
    });
  });

  describe('Actions menu', () => {
    it('opens the actions menu when the menu button for a row is clicked', () => {
      render(<TranslationsListPage />);

      const editButtons = screen.getAllByRole('button', {name: t('common:actions.edit')});
      expect(editButtons.length).toBeGreaterThan(0);
    });

    it('navigates to the language edit page when Edit is clicked in the actions menu', async () => {
      const user = userEvent.setup();
      render(<TranslationsListPage />);

      const editButtons = screen.getAllByRole('button', {name: t('common:actions.edit')});
      await user.click(editButtons[0]);

      expect(mockNavigate).toHaveBeenCalledWith(expect.stringMatching(/\/translations\//));
    });
  });
});
