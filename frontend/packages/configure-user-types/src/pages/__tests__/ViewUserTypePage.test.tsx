/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return, @typescript-eslint/no-explicit-any */
import {render, screen, waitFor, within, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ApiUserType, ApiError, LibraryAttribute} from '../../types/user-types';
import ViewUserTypePage from '../ViewUserTypePage';

// Mock the static attribute library (powers the attribute library panel on the Schema tab).
// The library attributes have no displayName, so the left-panel button label and
// the property row header both show the attribute name. Names are distinct and not
// substrings of each other.
const {mockAttributes} = vi.hoisted(() => ({
  mockAttributes: [
    {
      name: 'phone',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'department',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'level',
      displayName: '',
      type: 'number',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
  ] as LibraryAttribute[],
}));

vi.mock('../../constants/attributes', () => ({default: mockAttributes}));

vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    CopyableId: vi.fn(({value}: {value: string}) => <span data-testid="copyable-id">{value}</span>),
  };
});

const mockNavigate = vi.fn();
const mockRefetch = vi.fn();
const mockUpdateMutateAsync = vi.fn();
const mockResetUpdateError = vi.fn();
const mockShowToast = vi.fn();

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({id: 'schema-123'}),
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

// Mock hooks
const mockUseGetUserType = vi.fn<(id?: string) => any>();
const mockUseUpdateUserType = vi.fn<() => any>();
const mockUseDeleteUserType = vi.fn<() => any>();

vi.mock('../../api/useGetUserType', () => ({
  default: (id?: string) => mockUseGetUserType(id),
}));

vi.mock('../../api/useUpdateUserType', () => ({
  default: () => mockUseUpdateUserType(),
}));

vi.mock('../../api/useDeleteUserType', () => ({
  default: () => mockUseDeleteUserType(),
}));

// Mock OrganizationUnitTreePicker
vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: ({value, onChange}: {value: string; onChange: (id: string) => void}) => (
    <div data-testid="ou-tree-picker">
      <span data-testid="ou-value">{value || ''}</span>
      <button type="button" data-testid="select-ou-root" onClick={() => onChange('root-ou')}>
        Root Organization
      </button>
      <button type="button" data-testid="select-ou-child" onClick={() => onChange('child-ou')}>
        Child Organization
      </button>
    </div>
  ),
}));

// Mock shared-contexts (useToast)
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useToast: () => ({showToast: mockShowToast}),
  };
});

/**
 * Helper to navigate to the Schema tab.
 */
const goToSchemaTab = async (user: ReturnType<typeof userEvent.setup>) => {
  const schemaTab = screen.getByRole('tab', {name: /schema/i});
  await user.click(schemaTab);
};

/**
 * Helper to expand a property row by clicking its accessible header button.
 */
const expandProperty = async (user: ReturnType<typeof userEvent.setup>, name: string) => {
  await user.click(screen.getByRole('button', {name}));
};

/**
 * Helper to add a basic attribute to the schema by clicking its button in the
 * left-hand attribute library panel.
 */
const addAttribute = async (user: ReturnType<typeof userEvent.setup>, id: string) => {
  const panel = within(screen.getByRole('region', {name: /available properties/i}));
  await user.click(panel.getByRole('button', {name: new RegExp(`^${id}$`, 'i')}));
  await waitFor(() => {
    expect(panel.queryByRole('button', {name: new RegExp(`^${id}$`, 'i')})).not.toBeInTheDocument();
  });
};

describe('ViewUserTypePage', () => {
  const mockUserType: ApiUserType = {
    id: 'schema-123',
    name: 'Employee Schema',
    ouId: 'root-ou',
    allowSelfRegistration: false,
    schema: {
      email: {
        type: 'string',
        required: true,
        unique: true,
      },
      age: {
        type: 'number',
        required: false,
      },
      isActive: {
        type: 'boolean',
        required: true,
      },
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUserType.mockReturnValue({
      data: mockUserType,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
    mockUseUpdateUserType.mockReturnValue({
      mutateAsync: mockUpdateMutateAsync,
      error: null,
      reset: mockResetUpdateError,
      isPending: false,
    });
    mockUseDeleteUserType.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      error: null,
      reset: vi.fn(),
      mutate: vi.fn(),
    });
  });

  describe('Loading and Error States', () => {
    it('displays loading state', () => {
      mockUseGetUserType.mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('displays error state with error message', () => {
      const error: ApiError = {
        code: 'LOAD_ERROR',
        message: 'Failed to load user type',
        description: 'Network error',
      };

      mockUseGetUserType.mockReturnValue({
        data: null,
        isLoading: false,
        error,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      expect(screen.getByText('Failed to load user type')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /back to user types/i})).toBeInTheDocument();
    });

    it('displays warning when user type not found', () => {
      mockUseGetUserType.mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      expect(screen.getByText('User type not found')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /back to user types/i})).toBeInTheDocument();
    });

    it('navigates back from error state', async () => {
      const user = userEvent.setup();
      mockUseGetUserType.mockReturnValue({
        data: null,
        isLoading: false,
        error: {code: 'ERROR', message: 'Error', description: ''},
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      const backButton = screen.getByRole('button', {name: /back to user types/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/user-types');
      });
    });
  });

  describe('Header and Navigation', () => {
    it('renders user type name and ID', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByText('Employee Schema')).toBeInTheDocument();
      expect(screen.getByDisplayValue('schema-123')).toBeInTheDocument();
    });

    it('displays General and Schema tabs', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByRole('tab', {name: /general/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /schema/i})).toBeInTheDocument();
    });

    it('navigates back via back button link', () => {
      render(<ViewUserTypePage />);

      const backLink = screen.getByRole('link', {name: /back to user types/i});
      expect(backLink).toHaveAttribute('href', '/user-types');
    });

    it('allows inline editing of user type name', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      // Click the edit name button
      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);

      // Should show a text field with the current name
      const nameInput = screen.getByRole('textbox', {name: /user type name/i});
      expect(nameInput).toHaveValue('Employee Schema');

      // Edit the name
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Schema{Enter}');

      // Name should be updated
      await waitFor(() => {
        expect(screen.getByText('Updated Schema')).toBeInTheDocument();
      });
    });

    it('hides the unsaved-changes bar when the name is retyped back to its original value', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await user.click(screen.getByRole('button', {name: /edit user type name/i}));
      let nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'Renamed Schema{Enter}');

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', {name: /edit user type name/i}));
      nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'Employee Schema{Enter}');

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });
  });

  describe('General Tab', () => {
    it('displays organization unit tree picker', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByTestId('ou-tree-picker')).toBeInTheDocument();
      expect(screen.getByTestId('ou-value')).toHaveTextContent('root-ou');
    });

    it('allows selecting a different organization unit', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await user.click(screen.getByTestId('select-ou-child'));

      await waitFor(() => {
        expect(screen.getByTestId('ou-value')).toHaveTextContent('child-ou');
      });
    });

    it('displays self registration toggle', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByText('Self Registration')).toBeInTheDocument();
      // SettingsCard renders a Switch component with role="switch"
      const toggle = screen.getByRole('switch');
      expect(toggle).not.toBeChecked();
    });

    it('allows toggling self registration', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      const toggle = screen.getByRole('switch');
      await user.click(toggle);

      await waitFor(() => {
        expect(toggle).toBeChecked();
      });
    });

    it('displays display attribute section', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByText('Display Attribute')).toBeInTheDocument();
    });

    it('displays danger zone with delete button', () => {
      render(<ViewUserTypePage />);

      expect(screen.getByText('Danger Zone')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /^delete$/i})).toBeInTheDocument();
    });

    it('displays organization unit tree picker with empty value', () => {
      const userTypeWithEmptyOu: ApiUserType = {
        ...mockUserType,
        ouId: '',
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEmptyOu,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      const ouValue = screen.getByTestId('ou-value');
      expect(ouValue).toHaveTextContent('');
    });

    it('displays organization unit id in tree picker when unit is not found in lookup', () => {
      const userTypeWithUnknownOu: ApiUserType = {
        ...mockUserType,
        ouId: 'unknown-ou-id',
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithUnknownOu,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      const ouValue = screen.getByTestId('ou-value');
      expect(ouValue).toHaveTextContent('unknown-ou-id');
    });
  });

  describe('Schema Tab', () => {
    it('displays existing properties as collapsed rows on Schema tab', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);

      // Each loaded property renders a row header showing its name.
      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.getByText('age')).toBeInTheDocument();
      expect(screen.getByText('isActive')).toBeInTheDocument();

      // Bodies are collapsed, so the name inputs are not mounted yet.
      expect(screen.queryByDisplayValue('email')).not.toBeInTheDocument();
    });

    it('allows editing the property name when a row is expanded', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'email');

      const nameInput = await screen.findByDisplayValue('email');
      expect(nameInput).not.toBeDisabled();
    });

    it('allows editing the property type when a row is expanded', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'email');

      const typeSelect = await screen.findByRole('combobox');
      expect(typeSelect).not.toHaveAttribute('aria-disabled', 'true');
    });

    it('renders editable unique and credential checkboxes for an existing property', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'email');

      await screen.findByLabelText(/required/i);
      expect(screen.getByLabelText(/unique/i)).not.toBeDisabled();
      expect(screen.getByLabelText(/credential/i)).not.toBeDisabled();
    });

    it('reflects the loaded unique flag in the unique checkbox', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      // email is loaded as unique: true.
      await expandProperty(user, 'email');

      await waitFor(() => {
        expect(screen.getByLabelText(/unique/i)).toBeChecked();
      });
    });

    it('allows toggling the required checkbox', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'email');

      const requiredCheckbox = await screen.findByLabelText(/required/i);
      expect(requiredCheckbox).toBeChecked();

      await user.click(requiredCheckbox);

      await waitFor(() => {
        expect(requiredCheckbox).not.toBeChecked();
      });
    });

    it('allows adding enum values to an enum-typed property', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'status');

      const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
      await user.type(enumInput, 'INACTIVE');

      const addButton = screen.getByRole('button', {name: /^add$/i});
      await user.click(addButton);

      await waitFor(() => {
        expect(screen.getByText('INACTIVE')).toBeInTheDocument();
      });
    });

    it('allows adding enum value with Enter key', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'status');

      const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
      await user.type(enumInput, 'PENDING{Enter}');

      await waitFor(() => {
        expect(screen.getByText('PENDING')).toBeInTheDocument();
      });
    });

    it('allows removing enum values', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE', 'INACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'status');

      const activeChip = (await screen.findByText('ACTIVE')).closest('.MuiChip-root');
      const deleteButton = within(activeChip as HTMLElement).getByTestId('CancelIcon');

      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.queryByText('ACTIVE')).not.toBeInTheDocument();
        expect(screen.getByText('INACTIVE')).toBeInTheDocument();
      });
    });

    it('does not add empty enum value', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'status');

      await screen.findByPlaceholderText(/add value and press enter/i);
      const chipsBefore = document.querySelectorAll('.MuiChip-root').length;

      const addButton = screen.getByRole('button', {name: /^add$/i});
      await user.click(addButton);

      // No new chip should have been added since the input was empty.
      expect(document.querySelectorAll('.MuiChip-root').length).toBe(chipsBefore);
    });

    it('allows editing regex pattern', async () => {
      const user = userEvent.setup();
      const userTypeWithString: ApiUserType = {
        ...mockUserType,
        schema: {
          username: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithString,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'username');

      const regexInput = await screen.findByPlaceholderText(/e.g., \^/i);
      await user.click(regexInput);
      await user.paste('^[a-z]+$');

      expect(regexInput).toHaveValue('^[a-z]+$');
    });

    it('adds a new property via the attribute library panel', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);

      await addAttribute(user, 'phone');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'phone'})).toBeInTheDocument();
      });
    });

    it('removes a property when delete button is clicked', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await goToSchemaTab(user);

      const removeButtonsBefore = screen.getAllByRole('button', {name: /remove property/i});
      const countBefore = removeButtonsBefore.length;

      await user.click(removeButtonsBefore[0]);

      await waitFor(() => {
        expect(screen.getAllByRole('button', {name: /remove property/i})).toHaveLength(countBefore - 1);
      });
    });
  });

  describe('Delete Functionality', () => {
    it('opens delete confirmation dialog from danger zone', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      const deleteButton = screen.getByRole('button', {name: /^delete$/i});
      await user.click(deleteButton);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        expect(dialog).toBeInTheDocument();
        expect(screen.getByText(/are you sure you want to delete this user type/i)).toBeInTheDocument();
      });
    });

    it('closes delete dialog when cancel is clicked', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await user.click(screen.getByRole('button', {name: /^delete$/i}));

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
      });

      const cancelButton = within(screen.getByRole('dialog')).getByRole('button', {name: /cancel/i});
      await user.click(cancelButton);

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
    });
  });

  describe('Unsaved Changes Bar', () => {
    it('shows unsaved changes bar when OU is changed', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      await user.click(screen.getByTestId('select-ou-child'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('shows unsaved changes bar when name is edited', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);

      const nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'New Name{Enter}');

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('resets changes when reset button is clicked', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      // Make a change
      await user.click(screen.getByTestId('select-ou-child'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Click reset
      const resetButton = screen.getByRole('button', {name: /reset/i});
      await user.click(resetButton);

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });

      // OU should be back to original
      expect(screen.getByTestId('ou-value')).toHaveTextContent('root-ou');
    });
  });

  describe('Save Functionality', () => {
    it('saves changes via unsaved changes bar', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Change the OU
      await user.click(screen.getByTestId('select-ou-child'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Click save
      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            name: 'Employee Schema',
            ouId: 'child-ou',
            allowSelfRegistration: false,
            schema: expect.any(Object) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves name changes', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Edit the name inline
      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);

      const nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Schema{Enter}');

      // Save
      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            name: 'Updated Schema',
          }),
        });
      });
    });

    it('saves schema changes from Schema tab', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockResolvedValue(undefined);

      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE', 'INACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      // status loads as an enum-typed property; expand it to edit the enum values.
      await expandProperty(user, 'status');

      const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
      await user.type(enumInput, 'PENDING');

      const addButton = screen.getByRole('button', {name: /^add$/i});
      await user.click(addButton);

      await waitFor(() => {
        expect(screen.getByText('PENDING')).toBeInTheDocument();
      });

      // Save via the unsaved changes bar
      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              status: expect.objectContaining({
                type: 'string',
                enum: ['ACTIVE', 'INACTIVE', 'PENDING'],
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves schema with array type properties', async () => {
      const user = userEvent.setup();
      const userTypeWithArray: ApiUserType = {
        ...mockUserType,
        schema: {
          tags: {
            type: 'array',
            required: false,
            items: {type: 'string'},
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithArray,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make any change to trigger save bar (change OU)
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              tags: expect.objectContaining({
                type: 'array',
                items: {type: 'string'},
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves schema with object type properties', async () => {
      const user = userEvent.setup();
      const userTypeWithObject: ApiUserType = {
        ...mockUserType,
        schema: {
          address: {
            type: 'object',
            required: false,
            properties: {},
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithObject,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make any change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              address: expect.objectContaining({
                type: 'object',
                properties: {},
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('preserves unique flag on boolean type during round-trip', async () => {
      const user = userEvent.setup();
      const userTypeWithUniqueBoolean: ApiUserType = {
        ...mockUserType,
        schema: {
          isVerified: {
            type: 'boolean',
            required: false,
            unique: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithUniqueBoolean,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              isVerified: expect.objectContaining({
                type: 'boolean',
                unique: true,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves schema with unique flag for number type', async () => {
      const user = userEvent.setup();
      const userTypeWithUniqueNumber: ApiUserType = {
        ...mockUserType,
        schema: {
          employeeId: {
            type: 'number',
            required: true,
            unique: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithUniqueNumber,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              employeeId: expect.objectContaining({
                type: 'number',
                unique: true,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves schema with regex pattern for string type', async () => {
      const user = userEvent.setup();
      const userTypeWithRegex: ApiUserType = {
        ...mockUserType,
        schema: {
          username: {
            type: 'string',
            required: true,
            regex: '^[a-zA-Z]+$',
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithRegex,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              username: expect.objectContaining({
                type: 'string',
                regex: '^[a-zA-Z]+$',
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('saves schema with enum values for string type', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE', 'INACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              status: expect.objectContaining({
                type: 'string',
                enum: ['ACTIVE', 'INACTIVE'],
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });

    it('handles save error and shows toast', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockRejectedValue(new Error('Save failed'));

      render(<ViewUserTypePage />);

      // Make a change
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith('Save failed', 'error');
      });
    });

    it('shows validation error when saving with empty organization unit', async () => {
      const user = userEvent.setup();
      const userTypeWithEmptyOu: ApiUserType = {
        ...mockUserType,
        ouId: '',
        schema: {
          email: {
            type: 'string',
            required: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEmptyOu,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      // Edit name to trigger save bar (since OU is empty, no change to OU)
      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);
      const nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'New Name{Enter}');

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith('Please provide an organization unit ID', 'error');
      });

      expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
    });

    it('displays saving state', async () => {
      const user = userEvent.setup();

      mockUseUpdateUserType.mockReturnValue({
        mutateAsync: mockUpdateMutateAsync,
        error: null,
        reset: mockResetUpdateError,
        isPending: true,
      });

      render(<ViewUserTypePage />);

      // Make a change
      await user.click(screen.getByTestId('select-ou-child'));

      await waitFor(() => {
        expect(screen.getByText('Saving...')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: /saving.../i})).toBeDisabled();
      });
    });
  });

  describe('Credential Support', () => {
    it('reflects a loaded credential property as a checked credential checkbox', async () => {
      const user = userEvent.setup();
      const userTypeWithCredential: ApiUserType = {
        ...mockUserType,
        schema: {
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithCredential,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'password');

      const credentialCheckbox = await screen.findByLabelText(/credential/i);
      expect(credentialCheckbox).toBeChecked();
    });

    it('shows the credential hint for a loaded credential property', async () => {
      const user = userEvent.setup();
      const userTypeWithCredential: ApiUserType = {
        ...mockUserType,
        schema: {
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithCredential,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      await expandProperty(user, 'password');

      await waitFor(() => {
        expect(screen.getByText(/this field will be treated as a secret/i)).toBeInTheDocument();
      });
    });

    it('saves schema preserving the loaded credential flag', async () => {
      const user = userEvent.setup();

      const userTypeWithCredential: ApiUserType = {
        ...mockUserType,
        schema: {
          password: {
            type: 'string',
            required: true,
            credential: true,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithCredential,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              password: expect.objectContaining({
                credential: true,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });
  });

  describe('Schema Property Handling with Enum Type', () => {
    it('saves schema with enum type converted to string', async () => {
      const user = userEvent.setup();
      const userTypeWithEnum: ApiUserType = {
        ...mockUserType,
        schema: {
          status: {
            type: 'string',
            required: true,
            enum: ['ACTIVE', 'INACTIVE'],
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithEnum,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      await goToSchemaTab(user);
      // status loads as an enum-typed property.
      await expandProperty(user, 'status');

      // Add a new enum value
      const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
      await user.type(enumInput, 'PENDING');

      const addButton = screen.getByRole('button', {name: /^add$/i});
      await user.click(addButton);

      await waitFor(() => {
        expect(screen.getByText('PENDING')).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            schema: expect.objectContaining({
              status: expect.objectContaining({
                type: 'string',
                enum: ['ACTIVE', 'INACTIVE', 'PENDING'],
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }),
        });
      });
    });
  });

  describe('Display Attribute Eligibility', () => {
    it('clears display attribute when the selected property is removed', async () => {
      const user = userEvent.setup();
      const userTypeWithDisplay: ApiUserType = {
        ...mockUserType,
        systemAttributes: {display: 'email'},
        schema: {
          email: {
            type: 'string',
            required: true,
          },
          age: {
            type: 'number',
            required: false,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithDisplay,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Go to schema tab and remove the email property (the display attribute).
      await goToSchemaTab(user);

      const emailHeader = screen.getByRole('button', {name: 'email'});
      const removeButton = within(emailHeader).getByRole('button', {name: /remove property/i});
      await user.click(removeButton);

      // Save should not include the display attribute since it became ineligible.
      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.not.objectContaining({
            systemAttributes: expect.anything(),
          }),
        });
      });
    });

    it('preserves display attribute when selected property remains eligible', async () => {
      const user = userEvent.setup();
      const userTypeWithDisplay: ApiUserType = {
        ...mockUserType,
        systemAttributes: {display: 'email'},
        schema: {
          email: {
            type: 'string',
            required: true,
          },
          age: {
            type: 'number',
            required: false,
          },
        },
      };

      mockUseGetUserType.mockReturnValue({
        data: userTypeWithDisplay,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      mockUpdateMutateAsync.mockResolvedValue(undefined);

      render(<ViewUserTypePage />);

      // Make a change to trigger save bar
      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
          userTypeId: 'schema-123',
          data: expect.objectContaining({
            systemAttributes: {display: 'email'},
          }),
        });
      });
    });
  });

  describe('Save Error Handling', () => {
    it('handles non-Error save rejection with fallback message', async () => {
      const user = userEvent.setup();
      mockUpdateMutateAsync.mockRejectedValue('string error');

      render(<ViewUserTypePage />);

      await user.click(screen.getByTestId('select-ou-child'));

      const saveButton = screen.getByRole('button', {name: /^save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining('Failed to save user type'), 'error');
      });
    });
  });

  describe('Inline Name Editing Edge Cases', () => {
    it('does not save name when escape is pressed', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);

      const nameInput = screen.getByRole('textbox', {name: /user type name/i});
      await user.clear(nameInput);
      await user.type(nameInput, 'Temp Name{Escape}');

      // Should revert to original name
      await waitFor(() => {
        expect(screen.getByText('Employee Schema')).toBeInTheDocument();
      });

      // No unsaved changes bar should appear
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('does not save name when blurred with same value', async () => {
      const user = userEvent.setup();
      render(<ViewUserTypePage />);

      const editNameButton = screen.getByRole('button', {name: /edit user type name/i});
      await user.click(editNameButton);

      // Verify input is shown, then blur without changing the name
      expect(screen.getByRole('textbox', {name: /user type name/i})).toBeInTheDocument();
      await user.tab();

      // No unsaved changes bar should appear
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  describe('Navigation Error Handling', () => {
    it('handles navigation error from user type not found state', async () => {
      const user = userEvent.setup();
      mockNavigate.mockRejectedValue(new Error('Navigation failed'));

      mockUseGetUserType.mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      render(<ViewUserTypePage />);

      const backButton = screen.getByRole('button', {name: /back to user types/i});
      await user.click(backButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/user-types');
      });
    });
  });
});
