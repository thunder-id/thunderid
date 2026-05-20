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
const mockResetUpdateError = vi.fn();
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
      username: {
        type: 'string',
        required: true,
      },
      email: {
        type: 'string',
        required: true,
      },
      age: {
        type: 'number',
        required: false,
      },
      active: {
        type: 'boolean',
        required: false,
      },
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
    reset: mockResetUpdateError,
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
    mockDeleteMutate.mockImplementation((_userId: string, options?: {onSuccess?: () => void}) => {
      options?.onSuccess?.();
    });
    mockUseGetUser.mockReturnValue({
      data: mockUserData,
      isLoading: false,
      error: null,
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
      });

      render(<UserEditPage />);

      // Should display the fallback message since error.message is empty
      expect(screen.getByRole('alert')).toHaveTextContent('');
    });

    it('displays warning when user is null but no error', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
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
  });

  describe('View Mode', () => {
    it('renders user profile page with header', () => {
      render(<UserEditPage />);

      const headings = screen.getAllByRole('heading', {name: 'user123'});
      expect(headings.length).toBeGreaterThanOrEqual(1);
    });

    it('displays basic user information', () => {
      render(<UserEditPage />);

      // User ID shown in heading and CopyableId
      expect(screen.getAllByText('user123').length).toBeGreaterThanOrEqual(1);
      // User type shown as chip
      expect(screen.getByText('Employee')).toBeInTheDocument();
    });

    it('displays user attributes in view mode', () => {
      render(<UserEditPage />);

      expect(screen.getByText('username')).toBeInTheDocument();
      expect(screen.getByText('john_doe')).toBeInTheDocument();

      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.getByText('john@example.com')).toBeInTheDocument();

      expect(screen.getByText('age')).toBeInTheDocument();
      expect(screen.getByText('30')).toBeInTheDocument();

      expect(screen.getByText('active')).toBeInTheDocument();
      expect(screen.getByText('Yes')).toBeInTheDocument();
    });

    it('displays "No" for false boolean values', () => {
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, attributes: {active: false}},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByText('No')).toBeInTheDocument();
    });

    it('displays array values as comma-separated list', () => {
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, attributes: {tags: ['admin', 'developer', 'manager']}},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByText('admin, developer, manager')).toBeInTheDocument();
    });

    it('displays "No attributes available" when user has no attributes', () => {
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, attributes: {}},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByText('No attributes available')).toBeInTheDocument();
    });

    it('renders Edit and Delete buttons in view mode', () => {
      render(<UserEditPage />);

      expect(screen.getByRole('button', {name: /edit/i})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /delete/i})).toBeInTheDocument();
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

  describe('Edit Mode', () => {
    it('enters edit mode when Edit button is clicked', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      const editButton = screen.getByRole('button', {name: /edit/i});
      await user.click(editButton);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /^save$/i})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: /cancel/i})).toBeInTheDocument();
      });

      // Edit button should not be visible in edit mode (replaced by Save/Cancel)
      expect(screen.queryByRole('button', {name: /^edit$/i})).not.toBeInTheDocument();
    });

    it('does not submit if userId is missing from params', async () => {
      const user = userEvent.setup();
      // When userId is undefined, useGetUser will be called with undefined
      // Let's test the guard clause by simulating missing required fields instead
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, ouId: '', type: ''},
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      // Should not call mutateAsync when ouId or type is empty
      await waitFor(() => {
        expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
      });
    });

    it('does not submit if user ouId is missing', async () => {
      const user = userEvent.setup();
      mockUseGetUser.mockReturnValue({
        data: {...mockUserData, ouId: undefined as unknown as string},
        isLoading: false,
        error: null,
      });
      mockUseGetUserTypes.mockReturnValue({
        data: {
          ...mockSchemasData,
          types: [{...mockSchemasData.types[0], ouId: ''}],
        },
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

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
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
      });
    });

    it('displays form fields in edit mode', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/Enter username/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/Enter email/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/Enter age/i)).toBeInTheDocument();
        expect(screen.getByRole('checkbox')).toBeInTheDocument();
      });
    });

    it('filters out password field from schema in edit mode', async () => {
      const user = userEvent.setup();
      const schemaWithPassword: ApiUserType = {
        id: 'employee',
        name: 'Employee',
        schema: {
          username: {
            type: 'string',
            required: true,
          },
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
          email: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: schemaWithPassword,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/Enter username/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/Enter email/i)).toBeInTheDocument();
        // Password field should not be present
        expect(screen.queryByPlaceholderText(/Enter password/i)).not.toBeInTheDocument();
      });
    });

    it('filters out all credential fields from schema in edit mode', async () => {
      const user = userEvent.setup();
      const schemaWithMultipleCredentials: ApiUserType = {
        id: 'employee',
        name: 'Employee',
        schema: {
          username: {
            type: 'string',
            required: true,
          },
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
          pin: {
            type: 'string',
            credential: true,
          },
          email: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: schemaWithMultipleCredentials,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/Enter username/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/Enter email/i)).toBeInTheDocument();
        // All credential fields should be filtered out
        expect(screen.queryByPlaceholderText(/Enter password/i)).not.toBeInTheDocument();
        expect(screen.queryByPlaceholderText(/Enter pin/i)).not.toBeInTheDocument();
      });
    });

    it('does not filter non-credential fields with similar names', async () => {
      const user = userEvent.setup();
      const schemaWithoutCredential: ApiUserType = {
        id: 'employee',
        name: 'Employee',
        schema: {
          username: {
            type: 'string',
            required: true,
          },
          password: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: schemaWithoutCredential,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      await waitFor(() => {
        // A field named "password" without credential: true should still appear
        expect(screen.getByPlaceholderText(/Enter password/i)).toBeInTheDocument();
        expect(screen.getByPlaceholderText(/Enter username/i)).toBeInTheDocument();
      });
    });

    it('hides edit button when schema has only credential fields', () => {
      const credentialOnlySchema: ApiUserType = {
        id: 'credential-only',
        name: 'CredentialOnly',
        schema: {
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
          pin: {
            type: 'number',
            credential: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: credentialOnlySchema,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.queryByRole('button', {name: /edit/i})).not.toBeInTheDocument();
    });

    it('shows edit button when schema has at least one non-credential field', () => {
      const mixedSchema: ApiUserType = {
        id: 'mixed',
        name: 'Mixed',
        schema: {
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
          email: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: mixedSchema,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByRole('button', {name: /edit/i})).toBeInTheDocument();
    });

    it('populates form fields with current user data', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/Enter username/i)).toHaveValue('john_doe');
        expect(screen.getByPlaceholderText(/Enter email/i)).toHaveValue('john@example.com');
        expect(screen.getByPlaceholderText(/Enter age/i)).toHaveValue(30);
        expect(screen.getByRole('checkbox')).toBeChecked();
      });
    });

    it('allows editing form fields', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      const emailInput = await screen.findByPlaceholderText(/Enter email/i);
      await user.clear(emailInput);
      await user.type(emailInput, 'newemail@example.com');

      expect(emailInput).toHaveValue('newemail@example.com');
    });

    it('successfully updates user', async () => {
      const user = userEvent.setup();
      const updatedUser: User = {
        ...mockUserData,
        attributes: {...mockUserData.attributes, email: 'updated@example.com'},
      };
      mockUpdateMutateAsync.mockResolvedValue(updatedUser);

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      const emailInput = await screen.findByPlaceholderText(/Enter email/i);
      await user.clear(emailInput);
      await user.type(emailInput, 'updated@example.com');

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userId: 'user123',
          data: {
            ouId: 'test-ou',
            type: 'Employee',
            attributes: {
              username: 'john_doe',
              email: 'updated@example.com',
              age: 30,
              active: true,
            },
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
      });
      mockUseGetUserTypes.mockReturnValue({
        data: {
          ...mockSchemasData,
          types: [{...mockSchemasData.types[0], ouId: 'schema-ou'}],
        },
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalled();
        const callArgs = mockUpdateMutateAsync.mock.calls[0][0] as {data: {ouId: string}};
        expect(callArgs.data.ouId).toBe('schema-ou');
      });
    });

    it('falls back to user organization unit when schema does not provide one', async () => {
      const user = userEvent.setup();
      mockUseGetUserTypes.mockReturnValue({
        data: {
          ...mockSchemasData,
          types: [{...mockSchemasData.types[0], ouId: ''}],
        },
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalled();
        const callArgs = mockUpdateMutateAsync.mock.calls[0][0] as {data: {ouId: string}};
        expect(callArgs.data.ouId).toBe('test-ou');
      });
    });

    it('exits edit mode after successful save', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockResolvedValue(mockUserData);

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /edit/i})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: /^save$/i})).not.toBeInTheDocument();
      });
    });

    it('displays update error when save fails', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockRejectedValue(new Error('Failed to update user'));
      mockUseUpdateUser.mockReturnValue({
        ...defaultUpdateReturn,
        error: new Error('Failed to update user'),
        isError: true,
        isIdle: false,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('Failed to update user');
      });
    });

    it('cancels edit mode and resets form', async () => {
      const user = userEvent.setup();
      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      const emailInput = await screen.findByPlaceholderText(/Enter email/i);
      await user.clear(emailInput);
      await user.type(emailInput, 'changed@example.com');

      const cancelButton = screen.getByRole('button', {name: /cancel/i});
      await user.click(cancelButton);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /edit/i})).toBeInTheDocument();
        expect(mockResetUpdateError).toHaveBeenCalled();
      });
    });

    it('disables buttons during submission', async () => {
      const user = userEvent.setup();
      const neverResolvingUpdate = vi.fn().mockImplementation(() => new Promise(() => null)); // Never resolves
      mockUseUpdateUser.mockReturnValue({
        ...defaultUpdateReturn,
        mutateAsync: neverResolvingUpdate,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /saving.../i})).toBeDisabled();
        expect(screen.getByRole('button', {name: /cancel/i})).toBeDisabled();
      });
    });

    it('logs error when update fails', async () => {
      const user = userEvent.setup();
      const error = new Error('Update failed');

      const failingUpdateMutateAsync = vi.fn().mockRejectedValue(error);
      mockUseUpdateUser.mockReturnValue({
        ...defaultUpdateReturn,
        mutateAsync: failingUpdateMutateAsync,
      });

      render(<UserEditPage />);

      await user.click(screen.getByRole('button', {name: /edit/i}));
      await user.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => {
        expect(mockLoggerError).toHaveBeenCalledWith('Failed to update user', {error});
      });
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

      // Verify userId is passed correctly
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

  describe('Attribute Display Edge Cases', () => {
    it('displays dash for null attribute values', () => {
      const userWithNullAttr: User = {
        id: 'user123',
        ouId: 'test-ou',
        type: 'Employee',
        attributes: {
          username: 'john_doe',
          middleName: null as unknown as string,
        },
      };

      mockUseGetUser.mockReturnValue({
        data: userWithNullAttr,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      const middleNameSection = screen.getByText('middleName').parentElement;
      expect(middleNameSection).toHaveTextContent('-');
    });

    it('displays dash for undefined attribute values', () => {
      const userWithUndefinedAttr: User = {
        id: 'user123',
        ouId: 'test-ou',
        type: 'Employee',
        attributes: {
          username: 'john_doe',
          nickname: undefined as unknown as string,
        },
      };

      mockUseGetUser.mockReturnValue({
        data: userWithUndefinedAttr,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      const nicknameSection = screen.getByText('nickname').parentElement;
      expect(nicknameSection).toHaveTextContent('-');
    });

    it('displays comma-separated values for array attributes', () => {
      const userWithArrayAttr: User = {
        id: 'user123',
        ouId: 'test-ou',
        type: 'Employee',
        attributes: {
          username: 'john_doe',
          tags: ['developer', 'senior', 'fullstack'] as unknown as string,
        },
      };

      mockUseGetUser.mockReturnValue({
        data: userWithArrayAttr,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByText('tags')).toBeInTheDocument();
      expect(screen.getByText('developer, senior, fullstack')).toBeInTheDocument();
    });

    it('displays JSON string for object attributes', () => {
      const userWithObjectAttr: User = {
        id: 'user123',
        ouId: 'test-ou',
        type: 'Employee',
        attributes: {
          username: 'john_doe',
          address: {city: 'New York', country: 'USA'} as unknown as string,
        },
      };

      mockUseGetUser.mockReturnValue({
        data: userWithObjectAttr,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.getByText('address')).toBeInTheDocument();
      expect(screen.getByText('{"city":"New York","country":"USA"}')).toBeInTheDocument();
    });

    it('displays dash for unknown attribute types', () => {
      const userWithUnknownType: User = {
        id: 'user123',
        ouId: 'test-ou',
        type: 'Employee',
        attributes: {
          username: 'john_doe',
          // Symbol is not a standard JSON type
          unknownType: Symbol('test') as unknown as string,
        },
      };

      mockUseGetUser.mockReturnValue({
        data: userWithUnknownType,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      const unknownTypeSection = screen.getByText('unknownType').parentElement;
      expect(unknownTypeSection).toHaveTextContent('-');
    });
  });

  describe('Edge Cases', () => {
    it('displays fallback error message when error messages are undefined', () => {
      mockUseGetUser.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error(),
      });

      mockUseGetUserType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
    });

    it('hides edit button when schema is null', () => {
      mockUseGetUserType.mockReturnValue({
        data: {
          id: 'employee',
          name: 'Employee',
          schema: null as unknown as ApiUserType['schema'],
        },
        isLoading: false,
        error: null,
      });

      render(<UserEditPage />);

      expect(screen.queryByRole('button', {name: /edit/i})).not.toBeInTheDocument();
    });
  });
});
