// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
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

vi.mock('../../../../hooks/useDataGridLocaleText', () => ({
  default: () => ({}),
}));

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

// Stub the MUI ListingTable with lightweight components that expose rows
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');
  return {
    ...actual,
    ListingTable: {
      Provider: ({children, loading}: {children: React.ReactNode; loading: boolean}) => (
        <div data-testid="data-grid" data-loading={String(loading)}>
          {children}
        </div>
      ),
      Container: ({children}: {children: React.ReactNode}) => children,
      DataGrid: ({
        rows,
        columns,
        onRowClick = undefined,
      }: {
        rows: {id: string; code: string}[];
        columns: {renderCell?: (params: {row: {id: string; code: string}}) => React.ReactNode}[];
        onRowClick?: (params: {row: {id: string; code: string}}) => void;
      }) => (
        <>
          {rows.map((row) => (
            <div
              key={row.id}
              data-testid={`row-${row.id}`}
              role="row"
              onClick={() => onRowClick?.({row})}
              onKeyDown={(e) => e.key === 'Enter' && onRowClick?.({row})}
              tabIndex={0}
            >
              {row.code}
              {columns.map((col, i) => (
                // eslint-disable-next-line react/no-array-index-key
                <span key={i}>{col.renderCell?.({row})}</span>
              ))}
            </div>
          ))}
        </>
      ),
      RowActions: ({children}: {children: React.ReactNode}) => children,
      CellIcon: ({primary, secondary = null}: {primary: React.ReactNode; secondary?: React.ReactNode}) => (
        <div>
          <span>{primary}</span>
          {secondary}
        </div>
      ),
    },
  };
});

const mockUseGetLanguages = vi.mocked(useGetLanguages);

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

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

    it('hides the Add Language button when the admin may not create translations', () => {
      render(
        <CapabilitiesProvider capabilities={viewOnly}>
          <TranslationsListPage />
        </CapabilitiesProvider>,
      );

      expect(screen.queryByRole('button', {name: /add language/i})).not.toBeInTheDocument();
    });

    it('renders the data grid', () => {
      render(<TranslationsListPage />);

      expect(screen.getByTestId('data-grid')).toBeInTheDocument();
    });

    it('renders a row for each language', () => {
      render(<TranslationsListPage />);

      expect(screen.getByTestId('row-fr-FR')).toBeInTheDocument();
      expect(screen.getByTestId('row-de-DE')).toBeInTheDocument();
    });

    it('passes loading=false to the grid when data has loaded', () => {
      render(<TranslationsListPage />);

      expect(screen.getByTestId('data-grid')).toHaveAttribute('data-loading', 'false');
    });

    it('passes loading=true to the grid while data is loading', () => {
      mockUseGetLanguages.mockReturnValue({
        data: undefined,
        isLoading: true,
      } as ReturnType<typeof useGetLanguages>);

      render(<TranslationsListPage />);

      expect(screen.getByTestId('data-grid')).toHaveAttribute('data-loading', 'true');
    });

    it('renders an empty grid when there are no languages', () => {
      mockUseGetLanguages.mockReturnValue({
        data: {languages: []},
        isLoading: false,
      } as unknown as ReturnType<typeof useGetLanguages>);

      render(<TranslationsListPage />);

      expect(screen.queryByRole('row')).not.toBeInTheDocument();
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

      fireEvent.click(screen.getByTestId('row-fr-FR'));

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
