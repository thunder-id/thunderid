// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen, fireEvent} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import NotificationTemplatesList from '@/components/NotificationTemplatesList';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

const {mockUseGetTemplates} = vi.hoisted(() => ({mockUseGetTemplates: vi.fn()}));
vi.mock('@/api/useGetNotificationTemplates', () => ({
  default: mockUseGetTemplates,
}));

// Provide lightweight MUI mocks for the ListingTable + DataGrid, mirroring the sibling list tests.
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');
  return {
    ...actual,
    ListingTable: {
      Provider: ({children, loading, variant}: {children: React.ReactNode; loading: boolean; variant: string}) => (
        <div data-testid="data-grid" data-loading={String(loading)} data-variant={variant}>
          {children}
        </div>
      ),
      Container: ({children, disablePaper = undefined}: {children: React.ReactNode; disablePaper?: boolean}) => (
        <div data-testid="listing-container" data-disable-paper={disablePaper}>
          {children}
        </div>
      ),
      DataGrid: ({
        rows,
        columns,
        onRowClick = undefined,
      }: {
        rows: {id: string; displayName: string}[];
        columns: {renderCell?: (params: {row: {id: string; displayName: string}}) => React.ReactNode}[];
        onRowClick?: (params: {row: {id: string; displayName: string}}) => void;
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
              {columns.map((col, i) => (
                // eslint-disable-next-line react/no-array-index-key
                <span key={i}>{col.renderCell?.({row})}</span>
              ))}
            </div>
          ))}
        </>
      ),
      RowActions: ({children}: {children: React.ReactNode}) => children,
      CellIcon: ({
        icon = null,
        primary = null,
        secondary = null,
      }: {
        icon?: React.ReactNode;
        primary?: React.ReactNode;
        secondary?: React.ReactNode;
      }) => (
        <span>
          {icon}
          <span>{primary}</span>
          {secondary && <span>{secondary}</span>}
        </span>
      ),
      EmptyState: ({title = null, description = null}: {title?: React.ReactNode; description?: React.ReactNode}) => (
        <div>
          <div>{title}</div>
          <div>{description}</div>
        </div>
      ),
    },
  };
});

// Mock the delete dialog so its open state and selected template can be asserted directly.
vi.mock('@/components/NotificationTemplateDeleteDialog', () => ({
  default: (props: {open: boolean; templateId: string | null; templateName?: string; onClose: () => void}) => {
    if (!props.open) return null;
    return (
      <div data-testid="delete-dialog">
        <span data-testid="delete-template-id">{props.templateId}</span>
        <span data-testid="delete-template-name">{props.templateName}</span>
        <button type="button" onClick={props.onClose}>
          close-dialog
        </button>
      </div>
    );
  },
}));

const templates = [
  {id: 'tmpl-1', handle: 'otp-verification', displayName: 'OTP Verification', description: 'Sends a one-time code'},
  {id: 'tmpl-2', handle: 'password-reset', displayName: 'Password Reset'},
];

describe('NotificationTemplatesList', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetTemplates.mockReturnValue({
      data: {templates, totalResults: templates.length},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('renders a row for each template', () => {
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByTestId('row-tmpl-1')).toBeInTheDocument();
    expect(screen.getByTestId('row-tmpl-2')).toBeInTheDocument();
    expect(screen.getByText('OTP Verification')).toBeInTheDocument();
    expect(screen.getByText('Password Reset')).toBeInTheDocument();
  });

  it('renders the listing as a papered data grid', () => {
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByTestId('data-grid')).toHaveAttribute('data-variant', 'data-grid');
    expect(screen.getByTestId('listing-container')).not.toHaveAttribute('data-disable-paper');
  });

  it('shows the description as the second line only when the template has one', () => {
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByTestId('row-tmpl-1')).toHaveTextContent('Sends a one-time code');
    expect(screen.getByTestId('row-tmpl-2')).toHaveTextContent(/^Password Reset$/);
  });

  it('shows the mail icon for email templates and the message icon for SMS templates', () => {
    const {unmount} = render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByTestId('row-tmpl-1').querySelector('.lucide-mail')).toBeInTheDocument();
    expect(screen.getByTestId('row-tmpl-1').querySelector('.lucide-message-square')).not.toBeInTheDocument();
    unmount();

    render(<NotificationTemplatesList channel="sms" />);

    expect(screen.getByTestId('row-tmpl-1').querySelector('.lucide-message-square')).toBeInTheDocument();
    expect(screen.getByTestId('row-tmpl-1').querySelector('.lucide-mail')).not.toBeInTheDocument();
  });

  it('navigates to the template editor when a row is clicked', () => {
    render(<NotificationTemplatesList channel="email" />);

    fireEvent.click(screen.getByTestId('row-tmpl-1'));

    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email/tmpl-1');
  });

  it('navigates to the template editor when the edit button is clicked', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplatesList channel="email" />);

    const editButtons = screen.getAllByRole('button', {name: t('common:actions.edit')});
    await user.click(editButtons[0]);

    expect(mockNavigate).toHaveBeenCalledWith('/notification-templates/email/tmpl-1');
  });

  it('opens the delete dialog with the selected template when the delete button is clicked', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplatesList channel="email" />);

    const deleteButtons = screen.getAllByRole('button', {name: t('common:actions.delete')});
    await user.click(deleteButtons[0]);

    expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    expect(screen.getByTestId('delete-template-id')).toHaveTextContent('tmpl-1');
    expect(screen.getByTestId('delete-template-name')).toHaveTextContent('OTP Verification');
  });

  it('closes the delete dialog when its onClose is called', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplatesList channel="email" />);

    await user.click(screen.getAllByRole('button', {name: t('common:actions.delete')})[0]);
    expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();

    await user.click(screen.getByText('close-dialog'));

    expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
  });

  it('renders the error notice when the templates query fails', () => {
    mockUseGetTemplates.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom'),
      refetch: vi.fn(),
    });
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByText(t('notificationTemplates:listing.error'))).toBeInTheDocument();
  });
});
