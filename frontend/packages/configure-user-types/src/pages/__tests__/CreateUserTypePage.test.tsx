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

import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type useCreateUserTypeHook from '../../api/useCreateUserType';
import UserTypeCreateProvider from '../../contexts/UserTypeCreate/UserTypeCreateProvider';
import type {LibraryAttribute} from '../../types/user-types';
import CreateUserTypePage from '../CreateUserTypePage';

const mockNavigate = vi.fn();
const mockMutateAsync = vi.fn();

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

// Mock useCreateUserType hook
const mockUseCreateUserType = vi.fn<() => ReturnType<typeof useCreateUserTypeHook>>();

vi.mock('../../api/useCreateUserType', () => ({
  default: () => mockUseCreateUserType(),
}));

// Mock the static attribute library (powers the attribute library panel).
// The library attributes have no displayName, so the left-panel button label and
// the property row header both show the attribute name, and the serialized schema
// omits displayName. Names are chosen so none is a substring of another.
const {mockAttributes} = vi.hoisted(() => ({
  mockAttributes: [
    {
      name: 'email',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'username',
      displayName: '',
      type: 'string',
      required: true,
      unique: true,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'age',
      displayName: '',
      type: 'number',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'code',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'password',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: true,
      enum: [],
      regex: '',
    },
    {
      name: 'employeeId',
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

// Mock useHasMultipleOUs (used by ConfigureGeneral to decide whether to show the OU picker)
vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ({
    hasMultipleOUs: true,
    isLoading: false,
    ouList: [
      {id: 'root-ou', name: 'Root Organization', handle: 'root', description: null, parent: null},
      {id: 'child-ou', name: 'Child Organization', handle: 'child', description: null, parent: 'root-ou'},
    ],
  }),
  OrganizationUnitTreePicker: ({value, onChange}: {value: string; onChange: (id: string) => void}) => (
    <div data-testid="ou-tree-picker">
      <span data-testid="ou-value">{value}</span>
      <button type="button" data-testid="select-ou" onClick={() => onChange('ou-123')}>
        Select OU
      </button>
    </div>
  ),
}));

vi.mock('@thunderid/utils', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/utils')>();
  return {
    ...actual,
    generateRandomHumanReadableIdentifiers: () => ['Alpha Users', 'Beta Users', 'Gamma Users'],
  };
});

/**
 * Helper to render the wizard page wrapped in provider.
 */
function renderPage() {
  return render(
    <UserTypeCreateProvider>
      <CreateUserTypePage />
    </UserTypeCreateProvider>,
  );
}

/**
 * Helper to navigate from step 1 (Name) to step 2 (General) by typing a name and clicking Continue.
 */
async function goToGeneralStep(user: ReturnType<typeof userEvent.setup>, name = 'Employee') {
  await user.type(screen.getByLabelText(/User Type Name/i), name);
  await user.click(screen.getByRole('button', {name: /Continue/i}));
  await waitFor(() => {
    expect(screen.getByTestId('configure-general')).toBeInTheDocument();
  });
}

/**
 * Helper to navigate from step 1 to step 3 (Properties) via step 2.
 * The first OU is auto-selected by ConfigureGeneral.
 */
async function goToPropertiesStep(user: ReturnType<typeof userEvent.setup>, name = 'Employee') {
  await goToGeneralStep(user, name);
  // OU is auto-selected from useGetOrganizationUnits data
  await waitFor(() => {
    const continueButton = screen.getByRole('button', {name: /Continue/i});
    expect(continueButton).not.toBeDisabled();
  });
  await user.click(screen.getByRole('button', {name: /Continue/i}));
  await waitFor(() => {
    expect(screen.getByTestId('configure-properties')).toBeInTheDocument();
  });
}

/**
 * Helper to add a basic attribute to the schema by clicking its button in the
 * left-hand attribute library panel. The button's accessible name is the
 * attribute display name, which falls back to the id for our library.
 */
async function addAttribute(user: ReturnType<typeof userEvent.setup>, id: string) {
  const panel = within(screen.getByRole('region', {name: /available properties/i}));
  await user.click(panel.getByRole('button', {name: new RegExp(`^${id}$`, 'i')}));
  // The added attribute is removed from the library panel.
  await waitFor(() => {
    expect(panel.queryByRole('button', {name: new RegExp(`^${id}$`, 'i')})).not.toBeInTheDocument();
  });
}

/** The property-name inputs (one per expanded property card). */
const getPropertyNameInputs = () => screen.queryAllByPlaceholderText(/e\.g\., email, age, address/i);

describe('CreateUserTypePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCreateUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: null,
      isError: false,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useCreateUserTypeHook>);
  });

  // ============================================================================
  // Step 1: Name
  // ============================================================================

  it('renders the wizard with Name step initially', () => {
    renderPage();

    expect(screen.getByTestId('configure-name')).toBeInTheDocument();
    expect(screen.getByText("Let's name your user type")).toBeInTheDocument();
    expect(screen.getByLabelText(/User Type Name/i)).toBeInTheDocument();
  });

  it('closes wizard when X button is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    // The X (close) button navigates back to /user-types
    const closeButtons = screen.getAllByRole('button');
    // The X close button is the first IconButton in the header
    const closeButton = closeButtons[0];
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types');
    });
  });

  it('allows user to enter user type name', async () => {
    const user = userEvent.setup();
    renderPage();

    const nameInput = screen.getByLabelText(/User Type Name/i);
    await user.type(nameInput, 'Employee');

    expect(nameInput).toHaveValue('Employee');
  });

  it('disables Continue button when name is empty', () => {
    renderPage();

    const continueButton = screen.getByRole('button', {name: /Continue/i});
    expect(continueButton).toBeDisabled();
  });

  it('enables Continue button when name is entered', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(screen.getByLabelText(/User Type Name/i), 'Employee');

    const continueButton = screen.getByRole('button', {name: /Continue/i});
    expect(continueButton).not.toBeDisabled();
  });

  it('navigates to General step when Continue is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    expect(screen.getByTestId('configure-general')).toBeInTheDocument();
  });

  // ============================================================================
  // Step 2: General
  // ============================================================================

  it('shows the organization unit tree picker on General step', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    expect(screen.getByTestId('ou-tree-picker')).toBeInTheDocument();
  });

  it('auto-selects the first organization unit', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    // First OU should be auto-selected from useGetOrganizationUnits data
    await waitFor(() => {
      expect(screen.getByTestId('ou-value')).toHaveTextContent('root-ou');
    });
  });

  it('allows selecting a different organization unit via tree picker', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    await user.click(screen.getByTestId('select-ou'));

    expect(screen.getByTestId('ou-value')).toHaveTextContent('ou-123');
  });

  it('enables Continue on General step when OU is auto-selected', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    await waitFor(() => {
      const continueButton = screen.getByRole('button', {name: /Continue/i});
      expect(continueButton).not.toBeDisabled();
    });
  });

  it('navigates back to Name step when Back is clicked on General step', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    await user.click(screen.getByRole('button', {name: /Back/i}));

    await waitFor(() => {
      expect(screen.getByTestId('configure-name')).toBeInTheDocument();
    });
  });

  // ============================================================================
  // Step 3: Properties
  // ============================================================================

  it('navigates to Properties step', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    expect(screen.getByTestId('configure-properties')).toBeInTheDocument();
  });

  it('starts the Properties step with no property cards', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    // No properties until an attribute is added via the library panel.
    expect(getPropertyNameInputs()).toHaveLength(0);
    expect(screen.getByRole('button', {name: /^email$/i})).toBeInTheDocument();
  });

  it('adds a property by clicking an attribute in the library panel', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    const nameInputs = getPropertyNameInputs();
    expect(nameInputs).toHaveLength(1);
    expect(nameInputs[0]).toHaveValue('email');
    expect(nameInputs[0]).not.toBeDisabled();
  });

  it('removes a property when the delete icon is clicked', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');
    await addAttribute(user, 'age');

    const removeButtons = screen.getAllByRole('button', {name: /remove property/i});
    expect(removeButtons).toHaveLength(2);

    await user.click(removeButtons[removeButtons.length - 1]);

    await waitFor(() => {
      expect(screen.getAllByRole('button', {name: /remove property/i})).toHaveLength(1);
    });
  });

  it('removes an added attribute from the library panel', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    const panel = within(screen.getByRole('region', {name: /available properties/i}));
    expect(panel.getByRole('button', {name: /^email$/i})).toBeInTheDocument();

    await addAttribute(user, 'email');

    await waitFor(() => {
      expect(panel.queryByRole('button', {name: /^email$/i})).not.toBeInTheDocument();
    });
    // Other attributes remain available.
    expect(panel.getByRole('button', {name: /^age$/i})).toBeInTheDocument();
  });

  it('allows editing the name and type of a library attribute', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    await waitFor(() => {
      expect(getPropertyNameInputs()).toHaveLength(1);
    });
    const [nameInput] = getPropertyNameInputs();
    expect(nameInput).toHaveValue('email');
    // The name is editable even for a library-seeded attribute.
    expect(nameInput).not.toBeDisabled();

    // The property type select is editable too.
    const lockedSelect = screen.getAllByRole('combobox').find((c) => c.getAttribute('aria-disabled') === 'true');
    expect(lockedSelect).toBeUndefined();
  });

  it('renders unique and credential checkboxes as editable', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    await screen.findByLabelText(/required/i);
    expect(screen.getByLabelText(/unique/i)).not.toBeDisabled();
    expect(screen.getByLabelText(/credential/i)).not.toBeDisabled();
    expect(screen.getByLabelText(/required/i)).not.toBeDisabled();
  });

  it('seeds default flags from the picked attribute definition', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    // username is defined as required + unique in the library.
    await addAttribute(user, 'username');

    await screen.findByLabelText(/required/i);
    expect(screen.getByLabelText(/required/i)).toBeChecked();
    expect(screen.getByLabelText(/unique/i)).toBeChecked();
  });

  it('allows toggling the required checkbox', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    const requiredCheckbox = await screen.findByLabelText(/required/i);
    expect(requiredCheckbox).not.toBeChecked();

    await user.click(requiredCheckbox);
    expect(requiredCheckbox).toBeChecked();

    await user.click(requiredCheckbox);
    expect(requiredCheckbox).not.toBeChecked();
  });

  it('allows editing the display name', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    const displayNameInput = await screen.findByPlaceholderText(/e.g., First Name/i);
    await user.type(displayNameInput, 'Email Address');

    expect(displayNameInput).toHaveValue('Email Address');
  });

  it('allows adding a regex pattern for a string property', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    const regexInput = await screen.findByPlaceholderText('e.g., ^[a-zA-Z0-9]+$');
    await user.click(regexInput);
    await user.paste('^[a-z]+$');

    expect(regexInput).toHaveValue('^[a-z]+$');
  });

  it('allows selecting a display attribute', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');
    await addAttribute(user, 'code');

    // Added rows auto-expand, so each contributes its own Type combobox; the
    // display attribute select is rendered last, in the footer.
    const comboboxes = screen.getAllByRole('combobox');
    const displayAttributeSelect = comboboxes[comboboxes.length - 1];
    await user.click(displayAttributeSelect);

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByRole('option', {name: 'code'}));

    await waitFor(() => {
      expect(displayAttributeSelect).toHaveTextContent('code');
    });
  });

  it('navigates back to General step when Back is clicked on Properties step', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    await user.click(screen.getByRole('button', {name: /Back/i}));

    await waitFor(() => {
      expect(screen.getByTestId('configure-general')).toBeInTheDocument();
    });
  });

  // ============================================================================
  // Full wizard flow: submission
  // ============================================================================

  it('successfully creates user type with valid data', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);

    renderPage();

    // Step 1: Name
    await user.type(screen.getByLabelText(/User Type Name/i), 'Employee');
    await user.click(screen.getByRole('button', {name: /Continue/i}));

    // Step 2: General (OU auto-selected as root-ou)
    await waitFor(() => {
      expect(screen.getByTestId('configure-general')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Continue/i})).not.toBeDisabled();
    });
    await user.click(screen.getByRole('button', {name: /Continue/i}));

    // Step 3: Properties
    await waitFor(() => {
      expect(screen.getByTestId('configure-properties')).toBeInTheDocument();
    });
    await addAttribute(user, 'email');

    const requiredCheckbox = await screen.findByLabelText(/required/i);
    await user.click(requiredCheckbox);

    // Submit
    const submitButton = screen.getByRole('button', {name: /Create User Type/i});
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        name: 'Employee',
        ouId: 'root-ou',
        schema: {
          email: {
            type: 'string',
            required: true,
          },
        },
        systemAttributes: {display: 'email'},
      });
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types');
    });
  });

  it('submits organization unit and registration flag when provided', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);

    renderPage();

    // Step 1: Name
    await user.type(screen.getByLabelText(/User Type Name/i), 'Employee');
    await user.click(screen.getByRole('button', {name: /Continue/i}));

    // Step 2: General - pick a different OU via tree picker and enable self-registration
    await waitFor(() => {
      expect(screen.getByTestId('configure-general')).toBeInTheDocument();
    });
    await user.click(screen.getByTestId('select-ou'));
    await user.click(screen.getByLabelText(/Allow Self Registration/i));
    await user.click(screen.getByRole('button', {name: /Continue/i}));

    // Step 3: Properties
    await waitFor(() => {
      expect(screen.getByTestId('configure-properties')).toBeInTheDocument();
    });
    await addAttribute(user, 'email');

    await user.click(screen.getByRole('button', {name: /Create User Type/i}));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        name: 'Employee',
        ouId: 'ou-123',
        allowSelfRegistration: true,
        schema: {
          email: {
            type: 'string',
            required: false,
          },
        },
        systemAttributes: {display: 'email'},
      });
    });
  });

  it('disables submit when no attributes have been added', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToPropertiesStep(user);

    // The Create User Type button should be disabled when no attribute has been added.
    const submitButton = screen.getByRole('button', {name: /Create User Type/i});
    expect(submitButton).toBeDisabled();

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it('displays error from API', () => {
    const error = new Error('Failed to create user type');

    mockUseCreateUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error,
      isError: true,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useCreateUserTypeHook>);

    renderPage();

    expect(screen.getByText('Failed to create user type')).toBeInTheDocument();
  });

  it('shows loading state during submission on last step', async () => {
    const user = userEvent.setup();
    renderPage();

    // Navigate to properties step with loading=false (default from beforeEach)
    await goToPropertiesStep(user);

    // Add an attribute so the step is "ready"
    await addAttribute(user, 'email');
    await screen.findByLabelText(/required/i);

    // Now switch to loading state
    mockUseCreateUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: true,
      error: null,
      isError: false,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useCreateUserTypeHook>);

    // Trigger a re-render by toggling a checkbox (the hook return value will update)
    await user.click(screen.getByLabelText(/required/i));

    await waitFor(() => {
      expect(screen.getByText('Saving...')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', {name: /Saving/i})).toBeDisabled();
  });

  it('creates schema with string property containing regex pattern', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);

    renderPage();

    await goToPropertiesStep(user, 'RegexTest');

    await addAttribute(user, 'code');

    // Add regex pattern
    const regexInput = await screen.findByPlaceholderText('e.g., ^[a-zA-Z0-9]+$');
    await user.click(regexInput);
    await user.paste('^[A-Z]{3}$');

    // Submit
    const submitButton = screen.getByRole('button', {name: /Create User Type/i});
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        name: 'RegexTest',
        ouId: 'root-ou',
        schema: {
          code: {
            type: 'string',
            required: false,
            regex: '^[A-Z]{3}$',
          },
        },
        systemAttributes: {display: 'code'},
      });
    });
  });

  it('creates schema with a number property seeded from the attribute', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);

    renderPage();

    await goToPropertiesStep(user, 'NumberTest');

    await addAttribute(user, 'employeeId');

    // employeeId is a number type from the library; the type select reflects it.
    await waitFor(() => {
      const typeSelect = screen.getAllByRole('combobox').find((c) => c.textContent?.includes('Number'));
      expect(typeSelect).toBeDefined();
    });

    // Submit
    const submitButton = screen.getByRole('button', {name: /Create User Type/i});
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        name: 'NumberTest',
        ouId: 'root-ou',
        schema: {
          employeeId: {
            type: 'number',
            required: false,
          },
        },
        systemAttributes: {display: 'employeeId'},
      });
    });
  });

  it('handles create error gracefully', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockRejectedValue(new Error('Create failed'));

    renderPage();

    await goToPropertiesStep(user);

    await addAttribute(user, 'email');

    const submitButton = screen.getByRole('button', {name: /Create User Type/i});
    await user.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalled();
      expect(mockNavigate).not.toHaveBeenCalledWith('/user-types');
    });
  });

  // ============================================================================
  // Breadcrumb navigation
  // ============================================================================

  it('shows breadcrumbs reflecting current step', async () => {
    const user = userEvent.setup();
    renderPage();

    // Step 1: Only the name step breadcrumb
    const breadcrumb1 = screen.getByRole('navigation', {name: /breadcrumb/i});
    expect(within(breadcrumb1).getByText('Create a User Type')).toBeInTheDocument();

    await goToGeneralStep(user);

    // Step 2: Name step > "General" breadcrumbs
    const breadcrumb2 = screen.getByRole('navigation', {name: /breadcrumb/i});
    expect(within(breadcrumb2).getByText('Create a User Type')).toBeInTheDocument();
    expect(within(breadcrumb2).getByText('General')).toBeInTheDocument();
  });

  it('allows navigating back via breadcrumb click', async () => {
    const user = userEvent.setup();
    renderPage();

    await goToGeneralStep(user);

    // Click on name step breadcrumb to go back
    const breadcrumb = screen.getByRole('navigation', {name: /breadcrumb/i});
    await user.click(within(breadcrumb).getByText('Create a User Type'));

    await waitFor(() => {
      expect(screen.getByTestId('configure-name')).toBeInTheDocument();
    });
  });

  // ============================================================================
  // Progress bar
  // ============================================================================

  it('shows progress bar', () => {
    renderPage();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });
});
