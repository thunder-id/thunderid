// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen, fireEvent} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TranslationsList from '@/components/TranslationsList';

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

const mockMutate = vi.fn();
vi.mock('@thunderid/i18n', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useGetLanguages: vi.fn().mockReturnValue({
      data: {languages: ['fr-FR', 'de-DE']},
      isLoading: false,
    }),
    useDeleteTranslations: () => ({mutate: mockMutate, isPending: false}),
    getDisplayNameForCode: (code: string) => `Language(${code})`,
    toFlagEmoji: (code: string) => `Flag(${code})`,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

// Provide lightweight MUI mocks for DataGrid + ListingTable
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
      CellIcon: ({primary = null, secondary = null}: {primary?: React.ReactNode; secondary?: React.ReactNode}) => (
        <span>
          {primary}
          {secondary}
        </span>
      ),
    },
  };
});

// Mock TranslationDeleteDialog to control it directly
const mockDeleteDialog = vi.fn();
vi.mock('@/components/TranslationDeleteDialog', () => ({
  default: (props: {open: boolean; language: string | null; onClose: () => void}) => {
    mockDeleteDialog(props);
    return props.open ? (
      <div data-testid="delete-dialog">
        <span data-testid="delete-language">{props.language}</span>
        <button type="button" onClick={props.onClose}>
          close-dialog
        </button>
      </div>
    ) : null;
  },
}));

describe('TranslationsList', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders rows for each language', () => {
    render(<TranslationsList />);

    expect(screen.getByTestId('row-fr-FR')).toBeInTheDocument();
    expect(screen.getByTestId('row-de-DE')).toBeInTheDocument();
  });

  it('navigates to the edit page when a row is clicked', () => {
    render(<TranslationsList />);

    fireEvent.click(screen.getByTestId('row-fr-FR'));

    expect(mockNavigate).toHaveBeenCalledWith('/translations/fr-FR');
  });

  it('navigates to the edit page when the edit button is clicked', async () => {
    const user = userEvent.setup();
    render(<TranslationsList />);

    const editButtons = screen.getAllByRole('button', {name: t('common:actions.edit')});
    await user.click(editButtons[0]);

    expect(mockNavigate).toHaveBeenCalledWith(expect.stringMatching(/\/translations\//));
  });

  it('opens the delete dialog when the delete button is clicked', async () => {
    const user = userEvent.setup();
    render(<TranslationsList />);

    const deleteButtons = screen.getAllByRole('button', {name: t('common:actions.delete')});
    await user.click(deleteButtons[0]);

    expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    expect(screen.getByTestId('delete-language')).toHaveTextContent('fr-FR');
  });

  it('closes the delete dialog and clears the language when dialog onClose is called', async () => {
    const user = userEvent.setup();
    render(<TranslationsList />);

    // Open dialog
    const deleteButtons = screen.getAllByRole('button', {name: t('common:actions.delete')});
    await user.click(deleteButtons[0]);

    expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();

    // Close dialog
    await user.click(screen.getByText('close-dialog'));

    expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
  });
});
