// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, waitFor, renderWithProviders, renderHook, within} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
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
vi.mock('@thunderid/utils', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/utils')>();
  return {
    ...actual,
    generateRandomHumanReadableIdentifiers: () => ['Suggested Name One', 'Suggested Name Two', 'Suggested Name Three'],
  };
});

describe('CreateOrganizationUnitPage', () => {
  let t: (key: string, options?: Record<string, unknown>) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
    mockMutate.mockReset();
    mockLocationState = null;
  });

  it('should render page title and heading', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText(t('organizationUnits:create.title'))).toBeInTheDocument();
    expect(screen.getByText(t('organizationUnits:create.heading'))).toBeInTheDocument();
  });

  it('should render name input field', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByLabelText(/Name/i)).toBeInTheDocument();
  });

  it('should render handle input field', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByLabelText(/Handle/i)).toBeInTheDocument();
  });

  it('should render a name suggestion', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByRole('button', {name: 'Suggested Name One'})).toBeInTheDocument();
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

    const createButton = screen.getByText(t('common:actions.create'));
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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });
  });

  it('should accept a single character name and its generated handle', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'W'}});

    expect(screen.getByLabelText(/Handle/i)).toHaveValue('w');

    await waitFor(() => {
      expect(screen.getByText(t('common:actions.create', {defaultValue: 'Create'}))).not.toBeDisabled();
    });
  });

  it('should reject a handle longer than the maximum length', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    fireEvent.change(screen.getByLabelText(/Name/i), {target: {value: 'Workforce'}});
    fireEvent.change(screen.getByLabelText(/Handle/i), {target: {value: 'a'.repeat(101)}});

    await waitFor(() => {
      expect(
        screen.getByText(
          t('organizationUnits:edit.general.handle.validations.maxLength', {
            max: 100,
            defaultValue: 'Handle cannot exceed 100 characters',
          }),
        ),
      ).toBeInTheDocument();
    });

    expect(screen.getByText(t('common:actions.create', {defaultValue: 'Create'}))).toBeDisabled();
  });

  it('should reject a name longer than the maximum length', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'a'.repeat(101)}});

    await waitFor(() => {
      expect(
        screen.getByText(
          t('organizationUnits:edit.general.name.validations.maxLength', {
            max: 100,
            defaultValue: 'Name cannot exceed 100 characters',
          }),
        ),
      ).toBeInTheDocument();
    });

    expect(screen.getByText(t('common:actions.create', {defaultValue: 'Create'}))).toBeDisabled();
  });

  it('should call mutate on form submit', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
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
    const closeButton = screen.getByRole('button', {name: 'Close'});
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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to create organization unit. Please try again.')).toBeInTheDocument();
    });
    expect(screen.queryByText('Network error')).not.toBeInTheDocument();
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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to create organization unit. Please try again.')).toBeInTheDocument();
    });

    // Close the alert
    const alertCloseButton = within(screen.getByRole('alert')).getByRole('button', {name: /close/i});
    fireEvent.click(alertCloseButton);

    await waitFor(() => {
      expect(screen.queryByText('Failed to create organization unit. Please try again.')).not.toBeInTheDocument();
    });
  });

  it('should clear the create error when a field changes', async () => {
    mockMutate.mockImplementation((_data, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('Network error'));
    });

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    await waitFor(() => {
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    fireEvent.click(screen.getByText(t('common:actions.create')));

    await waitFor(() => {
      expect(screen.getByText('Failed to create organization unit. Please try again.')).toBeInTheDocument();
    });

    fireEvent.change(nameInput, {target: {value: 'Test Organization Updated'}});

    expect(screen.queryByText('Failed to create organization unit. Please try again.')).not.toBeInTheDocument();
  });

  it('should show "Root Organization Unit" in parent field when no parent is provided', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText(t('organizationUnits:edit.general.ou.noParent.label'))).toBeInTheDocument();
  });

  it('should set parent to null when no parent is in navigation state', async () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Test Organization'}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
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

    expect(screen.getByText('Engineering (engineering)')).toBeInTheDocument();
  });

  it('should display parent name without handle when handle is not provided', () => {
    mockLocationState = {parentId: 'ou-1', parentName: 'Engineering'};

    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText('Engineering')).toBeInTheDocument();
  });

  it('should submit with parent ID from navigation state', async () => {
    mockLocationState = {parentId: 'ou-1', parentName: 'Engineering', parentHandle: 'engineering'};

    renderWithProviders(<CreateOrganizationUnitPage />);

    const nameInput = screen.getByLabelText(/Name/i);
    fireEvent.change(nameInput, {target: {value: 'Child Organization'}});

    await waitFor(() => {
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
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

    fireEvent.click(screen.getByRole('button', {name: 'Suggested Name One'}));

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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  it('should handle close navigation error gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    renderWithProviders(<CreateOrganizationUnitPage />);

    const closeButton = screen.getByRole('button', {name: 'Close'});
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
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
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

    fireEvent.change(nameInput, {target: {value: '  Test Organization  '}});
    fireEvent.change(handleInput, {target: {value: '  test-org  '}});

    // Wait for form validation to complete
    await waitFor(() => {
      const createButton = screen.getByText(t('common:actions.create'));
      expect(createButton).not.toBeDisabled();
    });

    const createButton = screen.getByText(t('common:actions.create'));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Organization',
          handle: 'test-org',
        }),
        expect.any(Object),
      );
    });
  });

  it('should render progress bar', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render the suggestion prefix label', () => {
    renderWithProviders(<CreateOrganizationUnitPage />);

    expect(screen.getByText('Need inspiration? How about')).toBeInTheDocument();
  });
});
