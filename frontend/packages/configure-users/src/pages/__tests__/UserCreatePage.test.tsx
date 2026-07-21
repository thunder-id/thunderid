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

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserCreateProvider from '../../contexts/UserCreate/UserCreateProvider';
import type {UserTypeListResponse, ApiUserType, SchemaInterface} from '../../models/users';
import UserCreatePage from '../UserCreatePage';

const mockNavigate = vi.fn();
const mockMutateAsync = vi.fn();
const mockReset = vi.fn();

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
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

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    withComponent: vi.fn().mockReturnThis(),
  }),
}));

// Mock hooks
interface UseCreateUserReturn {
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

const mockUseCreateUser = vi.fn<() => UseCreateUserReturn>();
const mockUseGetUserTypes = vi.fn<() => UseGetUserTypesReturn>();
const mockUseGetUserType = vi.fn<() => UseGetUserTypeReturn>();

vi.mock('@/api/useCreateUser', () => ({
  default: () => mockUseCreateUser(),
}));

vi.mock('@/api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
}));

vi.mock('@/api/useGetUserType', () => ({
  default: () => mockUseGetUserType(),
}));

// Mock useGetChildOrganizationUnits — controls whether OU step appears
interface UseGetChildOUsReturn {
  data: {totalResults: number; startIndex: number; count: number; organizationUnits: unknown[]} | undefined;
  isLoading: boolean;
  error: Error | null;
}
const mockUseGetChildOrganizationUnits = vi.fn<() => UseGetChildOUsReturn>();
vi.mock('@thunderid/configure-organization-units', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useGetChildOrganizationUnits: () => mockUseGetChildOrganizationUnits(),
  };
});

// Mock useThunderID
const mockUseThunderID = vi.fn();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useThunderID: () => mockUseThunderID() as {user: {ouId?: string} | null | undefined},
  };
});

// Mock ConfigureOrganizationUnit — mirrors real component's auto-select behavior
vi.mock('@/components/create-user/ConfigureOrganizationUnit', async () => {
  const React = await import('react');

  function MockConfigureOrganizationUnit({
    rootOuId,
    selectedOuId,
    onOuIdChange,
    onReadyChange = undefined,
  }: {
    rootOuId: string;
    selectedOuId: string;
    onOuIdChange: (ouId: string) => void;
    onReadyChange?: (isReady: boolean) => void;
  }): React.JSX.Element {
    // Replicate the real component's useEffect: auto-select rootOuId when empty
    React.useEffect(() => {
      if (!selectedOuId) {
        onOuIdChange(rootOuId);
      }
    }, [selectedOuId, rootOuId, onOuIdChange]);

    // Replicate onReadyChange effect
    React.useEffect(() => {
      onReadyChange?.(selectedOuId.length > 0);
    }, [selectedOuId, onReadyChange]);

    return (
      <div data-testid="configure-organization-unit" data-root-ou-id={rootOuId} data-selected-ou-id={selectedOuId}>
        <button
          type="button"
          data-testid="select-ou"
          onClick={() => {
            onOuIdChange('child-ou-1');
            onReadyChange?.(true);
          }}
        >
          Select OU
        </button>
        <button
          type="button"
          data-testid="select-root-ou"
          onClick={() => {
            onOuIdChange(rootOuId);
            onReadyChange?.(true);
          }}
        >
          Select Root OU
        </button>
      </div>
    );
  }

  return {default: MockConfigureOrganizationUnit};
});

// Mock child components with controlled test behavior
vi.mock('@/components/create-user/ConfigureUserType', () => ({
  default: ({
    schemas,
    selectedSchema,
    onSchemaChange,
    onReadyChange,
  }: {
    schemas: SchemaInterface[];
    selectedSchema: SchemaInterface | null;
    onSchemaChange: (schema: SchemaInterface | null) => void;
    onReadyChange?: (isReady: boolean) => void;
  }) => (
    <div data-testid="configure-user-type">
      <span data-testid="selected-schema-name">{selectedSchema?.name ?? 'none'}</span>
      {schemas.map((s) => (
        <button
          key={s.id}
          type="button"
          data-testid={`select-schema-${s.name}`}
          onClick={() => {
            onSchemaChange(s);
            onReadyChange?.(true);
          }}
        >
          {s.name}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('@/components/create-user/ConfigureUserDetails', () => ({
  default: ({
    onFormValuesChange,
    onReadyChange,
  }: {
    onFormValuesChange: (values: Record<string, unknown>) => void;
    onReadyChange?: (isReady: boolean) => void;
  }) => (
    <div data-testid="configure-user-details">
      <button
        type="button"
        data-testid="fill-form"
        onClick={() => {
          onFormValuesChange({username: 'john_doe', age: 30});
          onReadyChange?.(true);
        }}
      >
        Fill Form
      </button>
      <button
        type="button"
        data-testid="fill-form-with-empty-values"
        onClick={() => {
          onFormValuesChange({username: 'john_doe', age: 30, nickname: '', phone: null, address: undefined});
          onReadyChange?.(true);
        }}
      >
        Fill Form With Empty Values
      </button>
      <button
        type="button"
        data-testid="mark-ready"
        onClick={() => {
          onReadyChange?.(true);
        }}
      >
        Mark Ready
      </button>
    </div>
  ),
}));

const mockSchemasData: UserTypeListResponse = {
  totalResults: 2,
  startIndex: 1,
  count: 2,
  types: [
    {id: 'schema1', name: 'Employee', ouId: 'root-ou'},
    {id: 'schema2', name: 'Contractor', ouId: 'child-ou'},
  ],
};

const mockSchemaData: ApiUserType = {
  id: 'schema1',
  name: 'Employee',
  schema: {
    username: {type: 'string', required: true},
    age: {type: 'number', required: false},
  },
};

/**
 * Helper to render the wizard page wrapped in provider.
 */
function renderPage() {
  return render(
    <UserCreateProvider>
      <UserCreatePage />
    </UserCreateProvider>,
  );
}

/**
 * Helper to navigate from step 1 (User Type) to step 2 (User Details)
 * by selecting a schema and clicking Continue.
 */
async function goToDetailsStep(user: ReturnType<typeof userEvent.setup>, schemaName = 'Employee') {
  await user.click(screen.getByTestId(`select-schema-${schemaName}`));
  await user.click(screen.getByRole('button', {name: /continue/i}));
  await waitFor(() => {
    expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
  });
}

describe('UserCreatePage', () => {
  /**
   * Helper to get the Create User action button (not the breadcrumb).
   * Multiple "Create User" elements exist on the page (breadcrumb + button),
   * so we select the last one which is the action button.
   */
  function getCreateUserButton() {
    const createButtons = screen.getAllByRole('button', {name: /create user/i});
    return createButtons[createButtons.length - 1];
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockMutateAsync.mockResolvedValue({
      id: 'user123',
      ouId: 'root-ou',
      type: 'Employee',
      attributes: {},
    });
    mockUseCreateUser.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: null,
      data: undefined,
      isError: false,
      isSuccess: false,
      isIdle: true,
      reset: mockReset,
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
    // Default: no child OUs → OU step is skipped (2-step flow)
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, organizationUnits: []},
      isLoading: false,
      error: null,
    });
    // Default: user object has no ouId
    mockUseThunderID.mockReturnValue({
      user: {ouId: undefined},
    });
  });

  // ============================================================================
  // Step 1: User Type
  // ============================================================================

  it('renders the wizard with User Type step initially', () => {
    renderPage();

    expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
  });

  it('shows progress bar', () => {
    renderPage();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows breadcrumb navigation', () => {
    renderPage();

    expect(screen.getByLabelText('breadcrumb')).toBeInTheDocument();
  });

  it('navigates to add user page when Add User breadcrumb is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    const addUserBreadcrumb = screen.getByRole('button', {name: /add user/i});
    await user.click(addUserBreadcrumb);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/add');
    });
  });

  it('disables Continue button when no schema is selected', () => {
    renderPage();

    const continueButton = screen.getByRole('button', {name: /continue/i});
    expect(continueButton).toBeDisabled();
  });

  it('enables Continue button when a schema is selected', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByTestId('select-schema-Employee'));

    const continueButton = screen.getByRole('button', {name: /continue/i});
    expect(continueButton).not.toBeDisabled();
  });

  it('closes wizard when X button is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    const closeButtons = screen.getAllByRole('button');
    const closeButton = closeButtons[0];
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });
  });

  it('does not close wizard when loading', async () => {
    mockUseCreateUser.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: true,
      error: null,
      data: undefined,
      isError: false,
      isSuccess: false,
      isIdle: false,
      reset: mockReset,
    });

    const user = userEvent.setup();
    renderPage();

    const closeButtons = screen.getAllByRole('button');
    const closeButton = closeButtons[0];
    await user.click(closeButton);

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  // ============================================================================
  // Step 2: User Details
  // ============================================================================

  it('navigates to User Details step when Continue is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToDetailsStep(user);

    expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
  });

  it('shows loading state when schema is loading on step 2', async () => {
    mockUseGetUserType.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });

    const user = userEvent.setup();
    renderPage();

    // Select schema and navigate to step 2 manually (can't use goToDetailsStep
    // because it waits for configure-user-details which won't render during loading)
    await user.click(screen.getByTestId('select-schema-Employee'));
    await user.click(screen.getByRole('button', {name: /continue/i}));

    await waitFor(() => {
      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });
  });

  it('shows Back button on User Details step', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToDetailsStep(user);

    expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
  });

  it('does not show Back button on User Type step', () => {
    renderPage();

    expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();
  });

  it('navigates back to User Type step when Back is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToDetailsStep(user);

    await user.click(screen.getByRole('button', {name: /back/i}));

    await waitFor(() => {
      expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
    });
  });

  it('navigates to a step via breadcrumb keyboard interaction', async () => {
    const user = userEvent.setup();
    renderPage();

    // Go to step 2 so the breadcrumb for step 1 becomes clickable
    await goToDetailsStep(user);

    // The first breadcrumb item (User Type) should be navigable via keyboard
    const breadcrumbStep = screen.getByRole('button', {name: /user type/i});
    breadcrumbStep.focus();
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
    });
  });

  it('preserves selected schema when navigating back and forward', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToDetailsStep(user);

    // Go back
    await user.click(screen.getByRole('button', {name: /back/i}));

    await waitFor(() => {
      expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
    });

    // Schema should still be selected
    expect(screen.getByTestId('selected-schema-name')).toHaveTextContent('Employee');
  });

  // ============================================================================
  // Submission
  // ============================================================================

  it('submits the form with correct data', async () => {
    const user = userEvent.setup();

    renderPage();

    await goToDetailsStep(user);
    await user.click(screen.getByTestId('fill-form'));

    // Wait for step ready state to update before clicking submit
    await waitFor(() => {
      expect(getCreateUserButton()).not.toBeDisabled();
    });

    await user.click(getCreateUserButton());

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        ouId: 'root-ou',
        type: 'Employee',
        attributes: {username: 'john_doe', age: 30},
      });
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });
  });

  it('filters empty attribute values before submission', async () => {
    const user = userEvent.setup();

    renderPage();

    await goToDetailsStep(user);
    // Use the button that emits empty/null/undefined values alongside valid ones
    await user.click(screen.getByTestId('fill-form-with-empty-values'));

    await waitFor(() => {
      expect(getCreateUserButton()).not.toBeDisabled();
    });

    await user.click(getCreateUserButton());

    await waitFor(() => {
      const calledWith = mockMutateAsync.mock.calls[0][0] as {attributes: Record<string, unknown>};
      // Verify empty/null/undefined values were filtered out
      expect(calledWith.attributes).toEqual({username: 'john_doe', age: 30});
      expect(calledWith.attributes).not.toHaveProperty('nickname');
      expect(calledWith.attributes).not.toHaveProperty('phone');
      expect(calledWith.attributes).not.toHaveProperty('address');
    });
  });

  it('shows saving state on submit button during loading', () => {
    mockUseCreateUser.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: true,
      error: null,
      data: undefined,
      isError: false,
      isSuccess: false,
      isIdle: false,
      reset: mockReset,
    });

    renderPage();

    // On step 1 with isPending=true, the continue button should be disabled
    const continueButton = screen.getByRole('button', {name: /continue/i});
    expect(continueButton).toBeDisabled();
  });

  // ============================================================================
  // Error Handling
  // ============================================================================

  it('displays API error from create user', () => {
    mockUseCreateUser.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: new Error('Failed to create user'),
      data: undefined,
      isError: true,
      isSuccess: false,
      isIdle: false,
      reset: mockReset,
    });

    renderPage();

    expect(screen.getByText('Failed to create user')).toBeInTheDocument();
  });

  it('closes snackbar when dismissed', async () => {
    const user = userEvent.setup();

    mockUseGetUserTypes.mockReturnValue({
      data: {
        ...mockSchemasData,
        types: [{id: 'schema1', name: 'Employee', ouId: ''}],
      },
      isLoading: false,
      error: null,
    });

    renderPage();

    await goToDetailsStep(user);
    await user.click(screen.getByTestId('fill-form'));

    await waitFor(() => {
      expect(getCreateUserButton()).not.toBeDisabled();
    });

    // Trigger validation error to open snackbar
    await user.click(getCreateUserButton());

    await waitFor(() => {
      expect(screen.getByText('Organization unit ID is missing for the selected user type.')).toBeInTheDocument();
    });

    // Close the snackbar
    const snackbarCloseButton = screen.getAllByRole('button', {name: /close/i});
    // The snackbar close button is the last close button rendered
    await user.click(snackbarCloseButton[snackbarCloseButton.length - 1]);

    await waitFor(() => {
      expect(screen.queryByText('Organization unit ID is missing for the selected user type.')).not.toBeInTheDocument();
    });
  });

  it('renders null when schema details are not available on step 2', async () => {
    mockUseGetUserType.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    });

    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByTestId('select-schema-Employee'));
    await user.click(screen.getByRole('button', {name: /continue/i}));

    // Should not render the details form or loading
    await waitFor(() => {
      expect(screen.queryByTestId('configure-user-details')).not.toBeInTheDocument();
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });
  });

  it('shows validation error when schema has missing ouId', async () => {
    const user = userEvent.setup();

    mockUseGetUserTypes.mockReturnValue({
      data: {
        ...mockSchemasData,
        types: [{id: 'schema1', name: 'Employee', ouId: ''}],
      },
      isLoading: false,
      error: null,
    });

    renderPage();

    await goToDetailsStep(user);
    await user.click(screen.getByTestId('fill-form'));

    await waitFor(() => {
      expect(getCreateUserButton()).not.toBeDisabled();
    });

    await user.click(getCreateUserButton());

    await waitFor(() => {
      expect(screen.getByText('Organization unit ID is missing for the selected user type.')).toBeInTheDocument();
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it('handles null schemas data gracefully', () => {
    mockUseGetUserTypes.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    });

    renderPage();

    expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
  });

  it('handles create user rejection gracefully', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockRejectedValue(new Error('Network error'));

    renderPage();

    await goToDetailsStep(user);
    await user.click(screen.getByTestId('fill-form'));

    await waitFor(() => {
      expect(getCreateUserButton()).not.toBeDisabled();
    });

    await user.click(getCreateUserButton());

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalled();
    });
  });

  // ============================================================================
  // Organization Unit Step (3-step flow)
  // ============================================================================

  describe('with child OUs (3-step flow)', () => {
    beforeEach(() => {
      // Enable OU step: the selected schema's OU has child OUs
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: {totalResults: 3, startIndex: 1, count: 3, organizationUnits: [{}, {}, {}]},
        isLoading: false,
        error: null,
      });
    });

    it('shows OU step after selecting user type', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });
    });

    it('passes correct rootOuId to ConfigureOrganizationUnit', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        const ouStep = screen.getByTestId('configure-organization-unit');
        expect(ouStep).toHaveAttribute('data-root-ou-id', 'root-ou');
      });
    });

    it('navigates from OU step to User Details step', async () => {
      const user = userEvent.setup();
      renderPage();

      // Step 1: Select user type
      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Select OU
      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('select-ou'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: User Details
      await waitFor(() => {
        expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
      });
    });

    it('navigates back from OU step to User Type step', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', {name: /back/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
      });
    });

    it('navigates back from User Details to OU step', async () => {
      const user = userEvent.setup();
      renderPage();

      // Go to User Details via OU step
      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });

      await user.click(screen.getByTestId('select-ou'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
      });

      // Go back — should return to OU step, not User Type
      await user.click(screen.getByRole('button', {name: /back/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });
    });

    it('submits with the selected child OU', async () => {
      const user = userEvent.setup();
      renderPage();

      // Step 1: Select user type
      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Select a child OU
      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('select-ou'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: Fill details and submit
      await waitFor(() => {
        expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('fill-form'));

      await waitFor(() => {
        expect(getCreateUserButton()).not.toBeDisabled();
      });
      await user.click(getCreateUserButton());

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          ouId: 'child-ou-1',
          type: 'Employee',
          attributes: {username: 'john_doe', age: 30},
        });
      });
    });

    it('submits with root OU when root is selected in OU step', async () => {
      const user = userEvent.setup();
      renderPage();

      // Step 1: Select user type
      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Select the root OU
      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('select-root-ou'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: Fill details and submit
      await waitFor(() => {
        expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('fill-form'));

      await waitFor(() => {
        expect(getCreateUserButton()).not.toBeDisabled();
      });
      await user.click(getCreateUserButton());

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          ouId: 'root-ou',
          type: 'Employee',
          attributes: {username: 'john_doe', age: 30},
        });
      });
    });

    it('resets OU selection when schema changes', async () => {
      const user = userEvent.setup();
      renderPage();

      // Select first schema (Employee, ouId: 'root-ou')
      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });

      // Select a child OU (not the auto-selected root)
      await user.click(screen.getByTestId('select-ou'));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toHaveAttribute('data-selected-ou-id', 'child-ou-1');
      });

      // Go back to user type
      await user.click(screen.getByRole('button', {name: /back/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
      });

      // Select a different schema (Contractor, ouId: 'child-ou')
      await user.click(screen.getByTestId('select-schema-Contractor'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        const ouStep = screen.getByTestId('configure-organization-unit');
        // After schema change, the previous child-ou-1 selection should be gone.
        // The new schema's root OU (child-ou) is auto-selected by the component.
        expect(ouStep).toHaveAttribute('data-root-ou-id', 'child-ou');
        expect(ouStep).toHaveAttribute('data-selected-ou-id', 'child-ou');
      });
    });

    it('auto-selects root OU and enables Continue on OU step', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      });

      // Root OU is auto-selected, so Continue should be enabled
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).not.toBeDisabled();
      });
    });
  });

  // ============================================================================
  // Without child OUs (2-step flow — OU step skipped)
  // ============================================================================

  describe('without child OUs (2-step flow)', () => {
    it('skips OU step and goes directly to User Details', async () => {
      // Default mock already sets totalResults: 0
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-schema-Employee'));
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Should go straight to User Details, not OU step
      await waitFor(() => {
        expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('configure-organization-unit')).not.toBeInTheDocument();
    });

    it('navigates back from User Details directly to User Type', async () => {
      const user = userEvent.setup();
      renderPage();

      await goToDetailsStep(user);

      await user.click(screen.getByRole('button', {name: /back/i}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
      });
    });
  });
});
