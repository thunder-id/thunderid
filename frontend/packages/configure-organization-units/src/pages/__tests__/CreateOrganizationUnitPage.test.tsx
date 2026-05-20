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

import {screen, fireEvent, waitFor, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CreateOrganizationUnitPage from '../CreateOrganizationUnitPage';

// Mock navigate and location
const mockNavigate = vi.fn();
let mockLocationState: Record<string, unknown> | null = null;
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({
      pathname: '/organization-units/create',
      search: '',
      hash: '',
      state: mockLocationState,
      key: 'default',
    }),
  };
});

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

// Mock create hook
const mockMutate = vi.fn();
vi.mock('@/api/useCreateOrganizationUnit', () => ({
  default: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}));

// Mock useOrganizationUnit hook
vi.mock('@/contexts/useOrganizationUnit', () => ({
  default: () => ({
    resetTreeState: vi.fn(),
  }),
}));

// Mock name suggestions utility
vi.mock('@thunderid/utils', () => ({
  generateRandomHumanReadableIdentifiers: () => ['Suggested Name One', 'Suggested Name Two', 'Suggested Name Three'],
}));

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'organizationUnits:create.title': 'Create Organization Unit',
        'organizationUnits:create.heading': 'Create a new organization unit',
        'organizationUnits:create.suggestions.label': 'Try these suggestions:',
        'organizationUnits:create.error': 'Failed to create organization unit',
        'organizationUnits:edit.general.name.label': 'Name',
        'organizationUnits:edit.general.name.placeholder': 'Enter organization unit name',
        'organizationUnits:edit.general.handle.label': 'Handle',
        'organizationUnits:edit.general.handle.placeholder': 'Enter handle',
        'organizationUnits:edit.general.handle.hint': 'A unique identifier for this organization unit',
        'organizationUnits:edit.general.description.label': 'Description',
        'organizationUnits:edit.general.description.placeholder': 'Enter description',
        'organizationUnits:edit.general.parent.label': 'Parent Organization Unit',
        'organizationUnits:edit.general.parent.hint': 'The parent organization unit for this new unit',
        'organizationUnits:edit.general.ou.noParent.label': 'Root Organization Unit',
        'common:actions.create': 'Create',
        'common:status.saving': 'Creating...',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('CreateOrganizationUnitPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
    mockMutate.mockReset();
    mockLocationState = null;
  });

  it('should render page title and heading', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText('Create Organization Unit')).toBeInTheDocument();
    expect(screen.getByText('Create a new organization unit')).toBeInTheDocument();
  });

  it('should render name input field', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByLabelText(/Name/i)).toBeInTheDocument();
  });

  it('should render handle input field', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByLabelText(/Handle/i)).toBeInTheDocument();
  });

  it('should render description input field', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByLabelText(/Description/i)).toBeInTheDocument();
  });

  it('should render name suggestions', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByRole('button', {name: 'Suggested Name One'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Suggested Name Two'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Suggested Name Three'})).toBeInTheDocument();
  });

  it('should auto-generate handle from name', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    const handleInput = screen.getByLabelText(/Handle/i);
    expect(handleInput).toHaveValue('test-organization');
  });

  it('should fill name when suggestion is clicked', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    fireEvent.click(screen.getByRole('button', {name: 'Suggested Name One'}));

    const nameInput = screen.getByLabelText(/Name/i);
    expect(nameInput).toHaveValue('Suggested Name One');
  });

  it('should auto-generate handle when suggestion is clicked', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    fireEvent.click(screen.getByRole('button', {name: 'Suggested Name One'}));

    const handleInput = screen.getByLabelText(/Handle/i);
    expect(handleInput).toHaveValue('suggested-name-one');
  });

  it('should not auto-generate handle after manual edit', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const handleInput = screen.getByLabelText(/Handle/i);
    fireEvent.change(handleInput, {target: {value: 'my-custom-handle'}});

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    expect(handleInput).toHaveValue('my-custom-handle');
  });

  it('should disable create button when form is invalid', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const createButton = screen.getByText('Create');
    expect(createButton).toBeDisabled();
  });

  it('should enable create button when form is valid', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    const handleInput = screen.getByLabelText(/Handle/i);

    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});
    fireEvent.change(handleInput, {target: {value: 'test-org'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });
  });

  it('should call mutate on form submit', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Organization',
          handle: 'test-organization',
        }),
        expect.any(Object),
      );
    });
  });

  it('should navigate back when close button is clicked', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    // Find the close button (X icon button)
    const closeButton = screen.getByRole('button', {name: ''});
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should navigate on successful creation', async () => {
    mockMutate.mockImplementation((_data, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should display error on creation failure', async () => {
    mockMutate.mockImplementation((_data, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('Network error'));
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
  });

  it('should close error alert when close button is clicked', async () => {
    mockMutate.mockImplementation((_data, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('Network error'));
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });

    // Close the alert
    const alertCloseButton = screen.getByRole('button', {name: /close/i});
    fireEvent.click(alertCloseButton);

    await waitFor(() => {
      expect(screen.queryByText('Network error')).not.toBeInTheDocument();
    });
  });

  it('should include description in request when provided', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    const descriptionInput = screen.getByLabelText(/Description/i);

    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});
    fireEvent.change(descriptionInput, {target: {value: 'A test description'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          description: 'A test description',
        }),
        expect.any(Object),
      );
    });
  });

  it('should set description to null when empty', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          description: null,
        }),
        expect.any(Object),
      );
    });
  });

  it('should show "Root Organization Unit" in parent field when no parent is provided', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const parentInput = screen.getByLabelText(/Parent Organization Unit/i);
    expect(parentInput).toHaveValue('Root Organization Unit');
    expect(parentInput).toHaveAttribute('readOnly');
  });

  it('should set parent to null when no parent is in navigation state', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          parent: null,
        }),
        expect.any(Object),
      );
    });
  });

  it('should display parent name and handle when navigated with parent state', () => {
    mockLocationState = {parentId: 'ou-1', parentName: 'Engineering', parentHandle: 'engineering'};

    renderWithProviders(<CreateOrganizationUnitPage />);

    const parentInput = screen.getByLabelText(/Parent Organization Unit/i);
    expect(parentInput).toHaveValue('Engineering (engineering)');
    expect(parentInput).toHaveAttribute('readOnly');
  });

  it('should display parent name without handle when handle is not provided', () => {
    mockLocationState = {parentId: 'ou-1', parentName: 'Engineering'};

    renderWithProviders(<CreateOrganizationUnitPage />);

    const parentInput = screen.getByLabelText(/Parent Organization Unit/i);
    expect(parentInput).toHaveValue('Engineering');
  });

  it('should submit with parent ID from navigation state', async () => {
    mockLocationState = {parentId: 'ou-1', parentName: 'Engineering', parentHandle: 'engineering'};

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Child Organization'}});

    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          parent: 'ou-1',
        }),
        expect.any(Object),
      );
    });
  });

  it('should keep handle unchanged after manual edit when suggestion is clicked', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const handleInput = screen.getByLabelText(/Handle/i);
    fireEvent.change(handleInput, {target: {value: 'my-custom-handle'}});

    fireEvent.click(screen.getByRole('button', {name: 'Suggested Name Two'}));

    // Handle should not change after suggestion click since it was manually edited
    expect(handleInput).toHaveValue('my-custom-handle');
  });

  it('should handle error without message', async () => {
    mockMutate.mockImplementation((_data, options: {onError: (err: unknown) => void}) => {
      options.onError({});
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  it('should handle close navigation error gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    renderWithProviders(<CreateOrganizationUnitPage />);

    const closeButton = screen.getByRole('button', {name: ''});
    fireEvent.click(closeButton);

    // Should not throw - error is logged
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should handle success navigation error gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));
    mockMutate.mockImplementation((_data, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    // Should not throw - error is logged
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should trim whitespace from inputs on submit', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    const handleInput = screen.getByLabelText(/Handle/i);
    const descriptionInput = screen.getByLabelText(/Description/i);

    fireEvent.change(nameInput, {target: {value: '  Test Organization  '}});
    fireEvent.change(handleInput, {target: {value: '  test-org  '}});
    fireEvent.change(descriptionInput, {target: {value: '  A description  '}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText('Create');
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText('Create');
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Organization',
          handle: 'test-org',
          description: 'A description',
        }),
        expect.any(Object),
      );
    });
  });

  it('should render progress bar', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render suggestions label', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText('Try these suggestions:')).toBeInTheDocument();
  });
});
