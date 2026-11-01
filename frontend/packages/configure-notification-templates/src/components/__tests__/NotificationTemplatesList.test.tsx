// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen, fireEvent, within} from '@thunderid/test-utils';
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

const rowOf = (name: string): HTMLElement => screen.getByText(name).closest<HTMLElement>('tr')!;

const names = (): string[] =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');

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

    expect(rowOf('OTP Verification')).toBeInTheDocument();
    expect(rowOf('Password Reset')).toBeInTheDocument();
  });

  it('renders the listing as a card table', () => {
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getAllByRole('row')).toHaveLength(3);
  });

  it('marks the table busy while the templates load', () => {
    mockUseGetTemplates.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('shows the empty row when the channel has no templates', () => {
    mockUseGetTemplates.mockReturnValue({
      data: {templates: [], totalResults: 0},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<NotificationTemplatesList channel="email" />);

    expect(screen.getByText(t('common:dataTable.noRowsLabel'))).toBeInTheDocument();
  });

  it('sorts the templates by name', async () => {
    const user = userEvent.setup();
    render(<NotificationTemplatesList channel="email" />);

    await user.click(screen.getByRole('button', {name: t('notificationTemplates:listing.columns.name')}));
    expect(names()).toEqual(['OTP VerificationSends a one-time code', 'Password Reset']);

    await user.click(screen.getByRole('button', {name: t('notificationTemplates:listing.columns.name')}));
    expect(names()).toEqual(['Password Reset', 'OTP VerificationSends a one-time code']);
  });

  it('requests the first page of ten templates, then the page the user moves to', async () => {
    const user = userEvent.setup();
    mockUseGetTemplates.mockReturnValue({
      data: {templates, totalResults: 30},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<NotificationTemplatesList channel="email" />);

    expect(mockUseGetTemplates).toHaveBeenLastCalledWith('email', {limit: 10, offset: 0});

    await user.click(screen.getByRole('button', {name: t('common:dataTable.paginationNextPage')}));
    expect(mockUseGetTemplates).toHaveBeenLastCalledWith('email', {limit: 10, offset: 10});

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: '25'}));
    expect(mockUseGetTemplates).toHaveBeenLastCalledWith('email', {limit: 25, offset: 0});
  });

  it('shows the description as the second line only when the template has one', () => {
    render(<NotificationTemplatesList channel="email" />);

    expect(names()).toEqual(['OTP VerificationSends a one-time code', 'Password Reset']);
  });

  it('shows the mail icon for email templates and the message icon for SMS templates', () => {
    const {unmount} = render(<NotificationTemplatesList channel="email" />);

    expect(rowOf('OTP Verification').querySelector('.lucide-mail')).toBeInTheDocument();
    expect(rowOf('OTP Verification').querySelector('.lucide-message-square')).not.toBeInTheDocument();
    unmount();

    render(<NotificationTemplatesList channel="sms" />);

    expect(rowOf('OTP Verification').querySelector('.lucide-message-square')).toBeInTheDocument();
    expect(rowOf('OTP Verification').querySelector('.lucide-mail')).not.toBeInTheDocument();
  });

  it('navigates to the template editor when a row is clicked', () => {
    render(<NotificationTemplatesList channel="email" />);

    fireEvent.click(rowOf('OTP Verification'));

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
