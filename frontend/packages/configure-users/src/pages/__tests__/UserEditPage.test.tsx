/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen, waitFor, within, userEvent} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ApiUserType, UserTypeListResponse} from '../../models/users';
import UserEditPage from '../UserEditPage';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    CopyableId: vi.fn(() => null),
  };
});

const mockNavigate = vi.fn();
const mockUpdateMutateAsync = vi.fn();
const mockDeleteMutate = vi.fn();
const mockResetDeleteError = vi.fn();

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: mockLoggerError,
    debug: vi.fn(),
  }),
}));

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({userId: 'user123'}),
    Link: ({to, children = undefined, ...props}: {to: string; children?: ReactNode; [key: string]: unknown}) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e) => {
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
});

// Mock hooks - TanStack Query interfaces
interface UseGetUserReturn {
  data: User | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: ReturnType<typeof vi.fn>;
}

interface UseGetUserTypesReturn {
  data: UserTypeListResponse | undefined;
  isLoading: boolean;
  error: Error | null;
}

interface UseGetUserTypeReturn {
  data: ApiUserType | undefined;
  isLoading: boolean;
  error: Error | null;
}

interface UseUpdateUserReturn {
  mutate: ReturnType<typeof vi.fn>;
  mutateAsync: ReturnType<typeof vi.fn>;
  isPending: boolean;
  error: Error | null;
  data: unknown;
  isError: boolean;
  isSuccess: boolean;
  isIdle: boolean;
  reset: () => void;
}

interface UseDeleteUserReturn {
  mutate: ReturnType<typeof vi.fn>;
  mutateAsync: ReturnType<typeof vi.fn>;
  isPending: boolean;
  error: Error | null;
  data: unknown;
  isError: boolean;
  isSuccess: boolean;
  isIdle: boolean;
  reset: () => void;
}

const mockRefetch = vi.fn();
const mockUseGetUser = vi.fn<() => UseGetUserReturn>();
const mockUseGetUserTypes = vi.fn<() => UseGetUserTypesReturn>();
const mockUseGetUserType = vi.fn<() => UseGetUserTypeReturn>();
const mockUseUpdateUser = vi.fn<() => UseUpdateUserReturn>();
const mockUseDeleteUser = vi.fn<() => UseDeleteUserReturn>();

vi.mock('@/api/useGetUser', () => ({
  default: () => mockUseGetUser(),
}));

vi.mock('@/api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
}));

vi.mock('@/api/useGetUserType', () => ({
  default: () => mockUseGetUserType(),
}));

vi.mock('@/api/useUpdateUser', () => ({
  default: () => mockUseUpdateUser(),
}));

vi.mock('@/api/useDeleteUser', () => ({
  default: () => mockUseDeleteUser(),
}));

// Mock the heavy child components — focus on page-level wiring. Their own deep behavior is
// covered by AttributesSummarySection.test.tsx and EditUserAttributes.test.tsx.
vi.mock('@/components/edit-user/QuickCopySection', () => ({
  default: () => <div data-testid="quick-copy" />,
}));

vi.mock('@/components/edit-user/AttributesSummarySection', () => ({
  default: () => <div data-testid="attributes-summary" />,
}));

vi.mock('@/components/edit-user/EditUserAttributes', () => ({
  default: ({onFieldChange}: {onFieldChange: (field: string, value: unknown) => void}) => (
    <div data-testid="edit-user-attributes">
      <button type="button" onClick={() => onFieldChange('attributes', {department: 'sales'})}>
        Edit an attribute
      </button>
    </div>
  ),
}));

vi.mock('@/components/edit-user/CredentialsTabPanel', () => ({
  default: ({userId, credentialFields}: {userId: string; credentialFields: {fieldName: string}[]}) => (
    <div data-testid="credentials-tab-panel" data-user-id={userId}>
      {credentialFields.map((field) => field.fieldName).join(',')}
    </div>
  ),
}));

describe('UserEditPage', () => {
  const mockUserData: User = {
    id: 'user123',
    ouId: 'test-ou',
    type: 'Employee',
    attributes: {
      username: 'john_doe',
      email: 'john@example.com',
      age: 30,
      active: true,
    },
  };

  const mockSchemasData: UserTypeListResponse = {
    totalResults: 1,
    startIndex: 1,
    count: 1,
    types: [{id: 'employee', name: 'Employee', ouId: 'test-ou'}],
  };

  const mockSchemaData: ApiUserType = {
    id: 'employee',
    name: 'Employee',
    schema: {
      username: {type: 'string', required: true},
      email: {type: 'string', required: true},
    },
  };

  const defaultUpdateReturn: UseUpdateUserReturn = {
    mutate: vi.fn(),
    mutateAsync: mockUpdateMutateAsync,
    isPending: false,
    error: null,
    data: undefined,
    isError: false,
    isSuccess: false,
    isIdle: true,
    reset: vi.fn(),
  };

  const defaultDeleteReturn: UseDeleteUserReturn = {
    mutate: mockDeleteMutate,
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
    data: undefined,
    isError: false,
    isSuccess: false,
    isIdle: true,
    reset: mockResetDeleteError,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUpdateMutateAsync.mockResolvedValue(mockUserData);
    mockRefetch.mockResolvedValue({});
    mockDeleteMutate.mockImplementation((_userId: string, options?: {onSuccess?: () => void}) => {
      options?.onSuccess?.();
    });
    mockUseGetUser.mockReturnValue({
      data: mockUserData,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
    mockUseGetUserTypes.mockReturnValue({
      data: mockSchemasData,
      isLoading: false,
      error: null,
    });
    mockUseGetUserType.mockReturnValue({
      data: mockSchemaData,
      isLoading: false,
      error: null,
    });
    mockUseUpdateUser.mockReturnValue({...defaultUpdateReturn});
    mockUseDeleteUser.mockReturnValue({...defaultDeleteReturn});
  });

  describe('Loading and Error States', () => {
    it('displays loading spinner when user data is loading', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: mockRefetch,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('displays loading spinner when schema is loading', () => {
      mockUseGetUserType.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('displays error alert when user fails to load', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('User not found'),
        refetch: mockRefetch,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('alert')).toHaveTextContent('User not found');
      expect(screen.getByRole('button', {name: /back to users/i})).toBeInTheDocument();
    });

    it('handles navigation error when clicking back button in error state', async () => {
      const user = userEvent.setup();
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('User not found'),
        refetch: mockRefetch,
      });

      mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

      render(<UserEditPage />);

      const backButton = screen.getByRole('button', {name: /back to users/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users');
      });

      consoleSpy.mockRestore();
    });

    it('displays error alert when schema fails to load', () => {
      mockUseGetUserType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Schema not found'),
      });

      render(<UserEditPage />);

      expect(screen.getByRole('alert')).toHaveTextContent('Schema not found');
    });

    it('displays generic error message when error message is empty', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error(''),
        refetch: mockRefetch,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('alert')).toHaveTextContent('');
    });

    it('displays warning when user is null but no error', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('alert')).toHaveTextContent('User not found');
    });

    it('handles navigation error when clicking back button in user not found state', async () => {
      const user = userEvent.setup();
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

      render(<UserEditPage />);

      const backButton = screen.getByRole('button', {name: /back to users/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users');
      });

      consoleSpy.mockRestore();
    });

    it('displays fallback error message when error messages are undefined', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error(),
        refetch: mockRefetch,
      });
      mockUseGetUserType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  describe('View Mode', () => {
    it('renders user profile page with header', () => {
      render(<UserEditPage />);

      const headings = screen.getAllByRole('heading', {name: 'user123'});
      expect(headings.length).toBeGreaterThanOrEqual(1);
    });

    it('displays basic user information', () => {
      render(<UserEditPage />);

      expect(screen.getAllByText('user123').length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('Employee')).toBeInTheDocument();
    });

    it('renders the General and Attributes tabs, and the Delete button', () => {
      render(<UserEditPage />);

      expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['General', 'Attributes']);
      expect(screen.getByRole('button', {name: /^delete$/i})).toBeInTheDocument();
    });

    it('shows the read-only attributes summary and QuickCopy on the General tab', () => {
      render(<UserEditPage />);

      expect(screen.getByTestId('quick-copy')).toBeInTheDocument();
      expect(screen.getByTestId('attributes-summary')).toBeInTheDocument();
      expect(screen.queryByTestId('edit-user-attributes')).not.toBeInTheDocument();
    });

    it('navigates back when Back button is clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      const backButton = screen.getByRole('button', {name: /back to users/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users');
      });
    });

    it('handles navigation error when back button is clicked', async () => {
      const user = userEvent.setup();
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
      mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

      render(<UserEditPage />);

      const backButton = screen.getByRole('button', {name: /back to users/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users');
      });

      consoleSpy.mockRestore();
    });
  });

  describe('Attributes tab', () => {
    it('switches to the Attributes tab content when clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));

      expect(screen.getByTestId('edit-user-attributes')).toBeInTheDocument();
    });

    it('surfaces the page-level unsaved-changes bar when an attribute is edited', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Reset'})).toBeInTheDocument();
    });

    it('does not submit if user ouId is missing', async () => {
      const user = userEvent.setup();
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, ouId: undefined as unknown as string},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
      mockUseGetUserTypes.mockReturnValue({
        data: {...mockSchemasData, types: [{...mockSchemasData.types[0], ouId: ''}]},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
      });
    });

    it('does not submit if user type is missing', async () => {
      const user = userEvent.setup();
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, type: undefined as unknown as string},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
      });
    });

    it('successfully updates the user with the staged attribute edit', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userId: 'user123',
          data: {
            ouId: 'test-ou',
            type: 'Employee',
            attributes: {department: 'sales'},
          },
        });
      });
    });

    it('uses schema organization unit when updating user', async () => {
      const user = userEvent.setup();
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, ouId: 'stale-ou'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
      mockUseGetUserTypes.mockReturnValue({
        data: {...mockSchemasData, types: [{...mockSchemasData.types[0], ouId: 'schema-ou'}]},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalled();
        const callArgs = mockUpdateMutateAsync.mock.calls[0][0] as {data: {ouId: string}};
        expect(callArgs.data.ouId).toBe('schema-ou');
      });
    });

    it('falls back to user organization unit when schema does not provide one', async () => {
      const user = userEvent.setup();
      mockUseGetUserTypes.mockReturnValue({
        data: {...mockSchemasData, types: [{...mockSchemasData.types[0], ouId: ''}]},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalled();
        const callArgs = mockUpdateMutateAsync.mock.calls[0][0] as {data: {ouId: string}};
        expect(callArgs.data.ouId).toBe('test-ou');
      });
    });

    it('hides the unsaved-changes bar after a successful save', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('clears the staged edit and hides the bar when Reset is clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Reset'}));

      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
    });

    it('disables Save and Reset while saving', async () => {
      const user = userEvent.setup();
      const neverResolvingUpdate = vi.fn().mockImplementation(() => new Promise(() => null));
      mockUseUpdateUser.mockReturnValue({
        ...defaultUpdateReturn,
        mutateAsync: neverResolvingUpdate,
        isPending: true,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(screen.getByRole('button', {name: 'Saving…'})).toBeDisabled();
    });

    it('logs an error when the update fails', async () => {
      const user = userEvent.setup();
      const error = new Error('Update failed');
      const failingUpdateMutateAsync = vi.fn().mockRejectedValue(error);
      mockUseUpdateUser.mockReturnValue({
        ...defaultUpdateReturn,
        mutateAsync: failingUpdateMutateAsync,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockLoggerError).toHaveBeenCalledWith('Failed to update user', {error});
      });
    });
  });

  describe('Credentials tab', () => {
    const schemaWithCredentials: ApiUserType = {
      id: 'employee',
      name: 'Employee',
      schema: {
        username: {type: 'string', required: true},
        password: {type: 'string', required: true, credential: true},
        pin: {type: 'string', credential: true},
      },
    };

    it('does not render a Credentials tab when the schema has no credential fields', () => {
      render(<UserEditPage />);

      expect(screen.queryByRole('tab', {name: 'Credentials'})).not.toBeInTheDocument();
    });

    it('renders a Credentials tab when the schema has credential fields', () => {
      mockUseGetUserType.mockReturnValue({
        data: schemaWithCredentials,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
        'General',
        'Attributes',
        'Credentials',
      ]);
    });

    it('does not render a Credentials tab for a read-only user, even with credential fields', () => {
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, isReadOnly: true},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
      mockUseGetUserType.mockReturnValue({
        data: schemaWithCredentials,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.queryByRole('tab', {name: 'Credentials'})).not.toBeInTheDocument();
    });

    it('passes the userId and derived credential fields to CredentialsTabPanel when selected', async () => {
      const user = userEvent.setup();
      mockUseGetUserType.mockReturnValue({
        data: schemaWithCredentials,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Credentials'}));

      const panel = screen.getByTestId('credentials-tab-panel');
      expect(panel).toHaveAttribute('data-user-id', 'user123');
      expect(panel).toHaveTextContent('password,pin');
    });
  });

  describe('Delete Functionality', () => {
    it('opens delete confirmation dialog when Delete button is clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      const deleteButton = screen.getByRole('button', {name: /^delete$/i});
      await user.click(deleteButton);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        expect(dialog).toBeInTheDocument();
        expect(within(dialog).getByText('Delete User')).toBeInTheDocument();
        expect(within(dialog).getByText(/Are you sure you want to delete this user/i)).toBeInTheDocument();
      });
    });

    it('calls mutateAsync with correct userId when delete is confirmed', async () => {
      const user = userEvent.setup();

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const confirmButton = within(dialog).getByRole('button', {name: /^delete$/i});
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockDeleteMutate).toHaveBeenCalledWith('user123', expect.any(Object));
      });
    });

    it('closes delete dialog when Cancel is clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const cancelButton = within(dialog).getByRole('button', {name: /cancel/i});
      await user.click(cancelButton);

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
    });

    it('successfully deletes user and navigates to users list', async () => {
      const user = userEvent.setup();

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const confirmButton = within(dialog).getByRole('button', {name: /^delete$/i});
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockDeleteMutate).toHaveBeenCalledWith('user123', expect.any(Object));
        expect(mockNavigate).toHaveBeenCalledWith('/users');
      });
    });

    it('displays delete error in dialog', async () => {
      const user = userEvent.setup();
      const deleteError = new Error('Failed to delete user');
      mockDeleteMutate.mockImplementation((_userId: string, options?: {onError?: (err: Error) => void}) => {
        options?.onError?.(deleteError);
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const confirmButton = within(dialog).getByRole('button', {name: /^delete$/i});
      await user.click(confirmButton);

      await waitFor(() => {
        expect(within(dialog).getByText('Failed to delete user')).toBeInTheDocument();
      });
    });

    it('disables buttons during deletion', async () => {
      const user = userEvent.setup();
      mockUseDeleteUser.mockReturnValue({
        ...defaultDeleteReturn,
        isPending: true,
        isIdle: false,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      expect(within(dialog).getByRole('button', {name: /deleting.../i})).toBeDisabled();
      expect(within(dialog).getByRole('button', {name: /cancel/i})).toBeDisabled();
    });

    it('shows error message when delete fails', async () => {
      const user = userEvent.setup();
      const error = new Error('Delete failed');

      mockDeleteMutate.mockImplementation((_userId: string, options?: {onError?: (err: Error) => void}) => {
        options?.onError?.(error);
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const confirmButton = within(dialog).getByRole('button', {name: /^delete$/i});
      await user.click(confirmButton);

      await waitFor(() => {
        expect(within(dialog).getByText('Delete failed')).toBeInTheDocument();
      });
    });

    it('keeps dialog open after delete error so user can retry', async () => {
      const user = userEvent.setup();
      mockDeleteMutate.mockImplementation((_userId: string, options?: {onError?: (err: Error) => void}) => {
        options?.onError?.(new Error('Delete failed'));
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      const dialog = screen.getByRole('dialog');
      const confirmButton = within(dialog).getByRole('button', {name: /^delete$/i});
      await user.click(confirmButton);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(within(dialog).getByText('Delete failed')).toBeInTheDocument();
      });
    });
  });
});
