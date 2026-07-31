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

import userEvent from '@testing-library/user-event';
import type {ResourcePermissions} from '@thunderid/configure-resource-servers';
import {render, screen} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {UpdateRoleRequest} from '../../models/requests';
import type {Role} from '../../models/role';
import RoleEditPage from '../RoleEditPage';

// Mock dependencies
vi.mock('../../api/useGetRole');
vi.mock('../../api/useUpdateRole');

vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>();
  return {
    ...actual,
    useIsMutating: vi.fn().mockReturnValue(0),
  };
});

vi.mock('../../components/RoleDeleteDialog', () => ({
  default: ({open}: {open: boolean}) => (open ? <div data-testid="delete-dialog">Delete Dialog</div> : null),
}));

vi.mock('../../components/edit-role/general-settings/EditGeneralSettings', () => ({
  default: ({onDeleteClick}: {onDeleteClick: () => void}) => (
    <div data-testid="edit-general-settings">
      <button type="button" onClick={onDeleteClick}>
        Delete
      </button>
    </div>
  ),
}));

vi.mock('../../components/edit-role/assignments-settings/EditAssignmentsSettings', () => ({
  default: () => <div data-testid="edit-assignments-settings">Assignments Settings</div>,
}));

vi.mock('../../components/edit-role/permissions-settings/EditPermissionsSettings', () => ({
  default: ({
    permissions,
    onPermissionsChange,
  }: {
    permissions: ResourcePermissions[];
    onPermissionsChange: (p: ResourcePermissions[]) => void;
    isReadOnly?: boolean;
  }) => (
    <div data-testid="permissions-settings">
      <span data-testid="permissions-selected">{JSON.stringify(permissions)}</span>
      <button
        type="button"
        data-testid="permissions-change"
        onClick={() => onPermissionsChange([{resourceServerId: 'rs-1', permissions: ['a', 'b']}])}
      >
        Change
      </button>
      <button
        type="button"
        data-testid="permissions-restore"
        onClick={() => onPermissionsChange([{resourceServerId: 'rs-1', permissions: ['a']}])}
      >
        Restore
      </button>
    </div>
  ),
}));

vi.mock('@thunderid/components', () => ({
  CopyableId: vi.fn(() => null),
  PageLoadingAnimation: vi.fn(() => <div data-testid="page-loading-animation" />),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useParams: vi.fn(),
    useNavigate: vi.fn(),
    Link: ({children, to}: {children: React.ReactNode; to: string}) => <a href={to}>{children}</a>,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useToast: () => ({showToast: vi.fn()}),
  };
});

const {default: useGetRole} = await import('../../api/useGetRole');
const {default: useUpdateRole} = await import('../../api/useUpdateRole');
const {useParams, useNavigate} = await import('react-router');
const {useIsMutating} = await import('@tanstack/react-query');

describe('RoleEditPage', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockRole: Role = {
    id: 'role-1',
    name: 'Admin Role',
    description: 'Administrator role',
    ouId: 'ou-1',
    permissions: [{resourceServerId: 'rs-1', permissions: ['a']}],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();

    vi.mocked(useParams).mockReturnValue({roleId: 'role-1'});
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useIsMutating).mockReturnValue(0);

    vi.mocked(useGetRole).mockReturnValue({
      data: mockRole,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useGetRole>);

    vi.mocked(useUpdateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: true,
      isPaused: false,
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading State', () => {
    it('should show CircularProgress while loading', () => {
      vi.mocked(useGetRole).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: vi.fn(),
      } as unknown as ReturnType<typeof useGetRole>);

      render(<RoleEditPage />);

      expect(screen.getByTestId('page-loading-animation')).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('should show error alert when fetch fails', () => {
      vi.mocked(useGetRole).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Failed to load'),
        refetch: vi.fn(),
      } as unknown as ReturnType<typeof useGetRole>);

      render(<RoleEditPage />);

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  describe('Rendering (with role data)', () => {
    it('should display role name in header', () => {
      render(<RoleEditPage />);

      expect(screen.getByText('Admin Role')).toBeInTheDocument();
    });

    it('should render role description', () => {
      render(<RoleEditPage />);

      expect(screen.getByText('Administrator role')).toBeInTheDocument();
    });

    it('should render three tabs', () => {
      render(<RoleEditPage />);

      const tabs = screen.getAllByRole('tab');
      expect(tabs).toHaveLength(3);
    });

    it('should show General tab panel by default', () => {
      render(<RoleEditPage />);

      expect(screen.getByTestId('edit-general-settings')).toBeInTheDocument();
    });
  });

  describe('Tab Navigation', () => {
    it('shows the permissions tab between General and Assignments', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      const tabs = await screen.findAllByRole('tab');
      expect(tabs.map((tab) => tab.textContent)).toEqual([
        'edit.page.tabs.general',
        'edit.page.tabs.permissions',
        'edit.page.tabs.assignments',
      ]);

      await user.click(tabs[1]);
      expect(screen.getByTestId('permissions-settings')).toBeInTheDocument();
    });

    it('should switch to Assignments tab panel when Assignments tab clicked', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      const tabs = screen.getAllByRole('tab');
      await user.click(tabs[2]);

      expect(screen.getByTestId('edit-assignments-settings')).toBeInTheDocument();
    });
  });

  describe('Delete Flow', () => {
    it('should open delete dialog when delete is triggered', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  describe('Save Bar concurrency gate', () => {
    it('should disable Save button while a role mutation is in flight even when updateRole.isPending is false', async () => {
      const user = userEvent.setup();
      vi.mocked(useIsMutating).mockReturnValue(1);

      render(<RoleEditPage />);

      const nameEditButton = screen.getByRole('button', {name: /edit.page.editName/i});
      await user.click(nameEditButton);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'New Name');
      await user.tab();

      const saveButton = screen.getByRole('button', {name: /edit.page.save/i});
      expect(saveButton).toBeDisabled();
    });
  });

  describe('General field revert', () => {
    it('hides the save bar when the name is retyped back to its original value', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      await user.click(screen.getByRole('button', {name: /edit.page.editName/i}));
      let nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Renamed Role');
      await user.tab();

      expect(screen.getByRole('button', {name: /edit.page.save/i})).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: /edit.page.editName/i}));
      nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Admin Role');
      await user.tab();

      expect(screen.queryByRole('button', {name: /edit.page.save/i})).not.toBeInTheDocument();
    });
  });

  describe('Permissions staged save', () => {
    it('shows the save bar when permissions change and saves them in one PUT', async () => {
      const user = userEvent.setup();
      const mockMutateAsync = vi.fn().mockResolvedValue({});
      const mockRefetch = vi.fn().mockResolvedValue({});
      vi.mocked(useGetRole).mockReturnValue({
        data: mockRole,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      } as unknown as ReturnType<typeof useGetRole>);
      vi.mocked(useUpdateRole).mockReturnValue({
        mutate: vi.fn(),
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        isSuccess: false,
        error: null,
        data: undefined,
        reset: vi.fn(),
        context: undefined,
        failureCount: 0,
        failureReason: null,
        isIdle: true,
        isPaused: false,
        status: 'idle',
        submittedAt: 0,
        variables: undefined,
      });

      render(<RoleEditPage />);

      // Switch to Permissions tab
      const tabs = screen.getAllByRole('tab');
      await user.click(tabs[1]);

      // Fire the catalog's onChange via the stub
      await user.click(screen.getByTestId('permissions-change'));

      // Save bar should be visible
      const saveButton = screen.getByRole('button', {name: /edit.page.save/i});
      expect(saveButton).toBeInTheDocument();

      // Click Save
      await user.click(saveButton);

      // Expect mutateAsync called with General fields + changed permissions
      expect(mockMutateAsync).toHaveBeenCalledWith({
        roleId: 'role-1',
        data: expect.objectContaining({
          name: 'Admin Role',
          ouId: 'ou-1',
          permissions: [{resourceServerId: 'rs-1', permissions: ['a', 'b']}],
        }) as UpdateRoleRequest,
      });
    });

    it('clears staged permissions when edits return to the server state', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      // Switch to Permissions tab
      const tabs = screen.getAllByRole('tab');
      await user.click(tabs[1]);

      // Change permissions → save bar appears
      await user.click(screen.getByTestId('permissions-change'));
      expect(screen.getByRole('button', {name: /edit.page.save/i})).toBeInTheDocument();

      // Restore to original (order-insensitively equal) → save bar hides
      await user.click(screen.getByTestId('permissions-restore'));
      expect(screen.queryByRole('button', {name: /edit.page.save/i})).not.toBeInTheDocument();
    });

    it('reset discards staged permission changes', async () => {
      const user = userEvent.setup();
      render(<RoleEditPage />);

      // Switch to Permissions tab
      const tabs = screen.getAllByRole('tab');
      await user.click(tabs[1]);

      // Change permissions
      await user.click(screen.getByTestId('permissions-change'));
      expect(screen.getByTestId('permissions-selected')).toHaveTextContent(
        JSON.stringify([{resourceServerId: 'rs-1', permissions: ['a', 'b']}]),
      );

      // Click Reset
      const resetButton = screen.getByRole('button', {name: /edit.page.reset/i});
      await user.click(resetButton);

      // Catalog selected should return to role.permissions
      expect(screen.getByTestId('permissions-selected')).toHaveTextContent(JSON.stringify(mockRole.permissions));
    });
  });
});
