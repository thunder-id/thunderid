// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {RoleListResponse} from '../../models/role';
import RolesList from '../RolesList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

// Mock the dependencies
vi.mock('../../api/useGetRoles');
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});
vi.mock('@thunderid/hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/hooks')>();
  return {
    ...actual,
    useDataGridLocaleText: vi.fn(),
  };
});

// Mock RoleDeleteDialog
vi.mock('../RoleDeleteDialog', () => ({
  default: ({open, onClose}: {open: boolean; onClose: () => void}) =>
    open ? (
      <div role="dialog" data-testid="delete-dialog">
        <button type="button" onClick={onClose}>
          Cancel
        </button>
        <button type="button">Delete</button>
      </div>
    ) : null,
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

const {default: useGetRoles} = await import('../../api/useGetRoles');
const {useNavigate} = await import('react-router');
const {useDataGridLocaleText} = await import('@thunderid/hooks');

const rowOf = (name: string): HTMLElement => screen.getByText(name).closest<HTMLElement>('tr')!;

const bodyCells = (column: number): string[] =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[column].textContent ?? '');

describe('RolesList', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockRolesData: RoleListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    roles: [
      {
        id: 'role-1',
        name: 'Admin Role',
        description: 'Administrator role',
        ouId: 'ou-1',
      },
      {
        id: 'role-2',
        name: 'Viewer Role',
        description: undefined,
        ouId: 'ou-2',
      },
    ],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockLoggerError.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useDataGridLocaleText).mockReturnValue({});
  });

  const renderComponent = () => render(<RolesList />);

  it('should mark the table busy while fetching', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('should render error state', () => {
    const error = new Error('Network error');
    vi.mocked(useGetRoles).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByText('Failed to load roles')).toBeInTheDocument();
    // Resolved through the i18n catalog, not the raw (unlocalized) error message.
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('should render roles list successfully', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByText('Admin Role')).toBeInTheDocument();
    expect(screen.getByText('Viewer Role')).toBeInTheDocument();
    expect(screen.getByText('Administrator role')).toBeInTheDocument();
  });

  it('should render the description as a secondary line only when present', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(bodyCells(0)).toEqual(['Admin RoleAdministrator role', 'Viewer Role']);
  });

  it('should not render a secondary line for an empty description', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: {...mockRolesData, roles: [{id: 'role-3', name: 'Auditor Role', description: '', ouId: 'ou-1'}]},
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(bodyCells(0)).toEqual(['Auditor Role']);
  });

  it('should render the roles in a card table', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getAllByRole('row')).toHaveLength(3);
  });

  it('should render the organization unit, falling back to the ID and then a dash', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: {
        ...mockRolesData,
        roles: [
          {id: 'role-1', name: 'Admin Role', ouId: 'ou-1', ouHandle: 'engineering'},
          {id: 'role-2', name: 'Viewer Role', ouId: 'ou-2'},
          {id: 'role-3', name: 'Auditor Role'},
        ],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(bodyCells(1)).toEqual(['engineering', 'ou-2', '-']);
  });

  it('should sort the roles by name and by organization unit', async () => {
    const user = userEvent.setup();
    vi.mocked(useGetRoles).mockReturnValue({
      data: {
        ...mockRolesData,
        roles: [
          {id: 'role-1', name: 'Beta', ouId: 'ou-1', ouHandle: 'sales'},
          {id: 'role-2', name: 'Alpha', ouId: 'ou-2', ouHandle: 'support'},
          {id: 'role-3', name: 'Gamma', ouId: 'ou-3', ouHandle: 'engineering'},
        ],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetRoles>);

    renderComponent();

    await user.click(screen.getByRole('button', {name: 'Name'}));
    expect(bodyCells(0)).toEqual(['Alpha', 'Beta', 'Gamma']);

    await user.click(screen.getByRole('button', {name: 'Organization unit'}));
    expect(bodyCells(1)).toEqual(['engineering', 'sales', 'support']);
  });

  it('should request the first page of ten roles, then the page the user moves to', async () => {
    const user = userEvent.setup();
    vi.mocked(useGetRoles).mockReturnValue({
      data: {...mockRolesData, totalResults: 30},
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(useGetRoles).toHaveBeenLastCalledWith({limit: 10, offset: 0});

    await user.click(screen.getByRole('button', {name: 'Go to next page'}));
    expect(useGetRoles).toHaveBeenLastCalledWith({limit: 10, offset: 10});

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: '25'}));
    expect(useGetRoles).toHaveBeenLastCalledWith({limit: 25, offset: 0});
  });

  it('should label the row action buttons', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: {
        ...mockRolesData,
        roles: [...mockRolesData.roles, {id: 'role-3', name: 'System Role', ouId: 'ou-1', isReadOnly: true}],
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(2);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
    expect(screen.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
  });

  it('should open delete dialog when clicking Delete action', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should close delete dialog when cancelled', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', {name: /cancel/i});
    await user.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });
  });

  it('should navigate when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    await user.click(rowOf('Admin Role'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-1');
    });
  });

  it('should navigate when clicking Edit action button', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-1');
    });
  });

  it('should render no rows when roles list is empty', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 0,
        count: 0,
        roles: [],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('No rows')).toBeInTheDocument();
  });

  it('should navigate to correct role when clicking different row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    await user.click(rowOf('Viewer Role'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-2');
    });
  });

  it('should handle navigation error gracefully', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    await user.click(rowOf('Admin Role'));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith('Failed to navigate to role', {
        error: navigationError,
        roleId: 'role-1',
      });
    });

    expect(screen.getByRole('table')).toBeInTheDocument();
  });
});
