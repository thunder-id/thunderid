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

import {screen, fireEvent, waitFor, renderWithProviders} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationUnit} from '../../models/organization-unit';
import OrganizationUnitEditPage from '../OrganizationUnitEditPage';

// Mock navigate, useParams, and useLocation
const mockNavigate = vi.fn();
const mockUseLocation = vi.fn<() => {state: unknown; pathname: string; search: string; hash: string; key: string}>();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({id: 'ou-123'}),
    useLocation: () => mockUseLocation(),
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
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

// Mock get OU hook
const mockRefetch = vi.fn();
const mockUseGetOrganizationUnit = vi.fn();
vi.mock('@/api/useGetOrganizationUnit', () => ({
  default: () =>
    mockUseGetOrganizationUnit() as {
      data: OrganizationUnit | undefined;
      isLoading: boolean;
      error: Error | null;
      refetch: () => void;
    },
}));

// Mock update hook
const mockMutateAsync = vi.fn();
vi.mock('@/api/useUpdateOrganizationUnit', () => ({
  default: () => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  }),
}));

// Mock delete hook
const mockDeleteMutate = vi.fn();
vi.mock('@/api/useDeleteOrganizationUnit', () => ({
  default: () => ({
    mutate: mockDeleteMutate,
    isPending: false,
  }),
}));

// Mock child hooks
vi.mock('@/api/useGetChildOrganizationUnits', () => ({
  default: () => ({
    data: {organizationUnits: [], totalResults: 0, startIndex: 1, count: 0},
    isLoading: false,
  }),
}));

vi.mock('@/api/useGetOrganizationUnitUsers', () => ({
  default: () => ({
    data: {users: [], totalResults: 0, startIndex: 1, count: 0},
    isLoading: false,
  }),
}));

vi.mock('@/api/useGetOrganizationUnitGroups', () => ({
  default: () => ({
    data: {groups: [], totalResults: 0, startIndex: 1, count: 0},
    isLoading: false,
  }),
}));

// Mock useOrganizationUnit hook
vi.mock('@/contexts/useOrganizationUnit', () => ({
  default: () => ({
    resetTreeState: vi.fn(),
  }),
}));

// Mock useDataGridLocaleText
vi.mock('@thunderid/hooks', async (importOriginal) => {
  const actual = await importOriginal();
  return {...(actual as object), useDataGridLocaleText: () => ({})};
});

// Mock EmojiPicker
vi.mock('@thunderid/components', async () => {
  const React = await import('react');
  return {
    EmojiPicker: vi.fn(() => null),
    ResourceLogoDialog: vi.fn(
      ({open, onClose, onSelect}: {open: boolean; onClose: () => void; onSelect: (value: string) => void}) => (
        <div data-testid="resource-logo-dialog" style={{display: open ? 'block' : 'none'}}>
          <button type="button" onClick={() => onSelect('emoji:🚀')}>
            Select Icon
          </button>
          <button type="button" onClick={onClose}>
            Close
          </button>
        </div>
      ),
    ),
    UnsavedChangesBar: vi.fn(
      ({
        message,
        resetLabel,
        saveLabel,
        savingLabel,
        isSaving,
        onReset,
        onSave,
      }: {
        message: string;
        resetLabel: string;
        saveLabel: string;
        savingLabel: string;
        isSaving: boolean;
        onReset: () => void;
        onSave: () => void;
      }) => (
        <div data-testid="unsaved-changes-bar">
          <span>{message}</span>
          <button type="button" onClick={onReset}>
            {resetLabel}
          </button>
          <button type="button" onClick={onSave} disabled={isSaving}>
            {isSaving ? savingLabel : saveLabel}
          </button>
        </div>
      ),
    ),
    ResourceAvatar: vi.fn(function MockResourceAvatar({
      value,
      onSelect,
      editAriaLabel,
    }: {
      value?: string;
      onSelect?: (v: string) => void;
      editAriaLabel?: string;
    }) {
      const [open, setOpen] = React.useState(false);
      const [imgError, setImgError] = React.useState(false);
      const isUrl = typeof value === 'string' && (value.startsWith('http://') || value.startsWith('https://'));
      const displayValue =
        typeof value === 'string' && value.startsWith('emoji:') ? value.slice('emoji:'.length) : value;
      return (
        <>
          {isUrl && (
            <button type="button" onClick={() => setOpen(true)}>
              <img src={value} alt="logo" onError={() => setImgError(true)} style={imgError ? {display: 'none'} : {}} />
            </button>
          )}
          {!isUrl && displayValue && <span>{displayValue}</span>}
          {editAriaLabel && onSelect && (
            <button type="button" aria-label={editAriaLabel} onClick={() => setOpen(true)} />
          )}
          <div data-testid="emoji-picker" style={{display: open ? 'block' : 'none'}}>
            <button
              type="button"
              onClick={() => {
                onSelect?.('emoji:🚀');
                setOpen(false);
              }}
            >
              Select Icon
            </button>
            <button type="button" onClick={() => setOpen(false)}>
              Close
            </button>
          </div>
        </>
      );
    }),
    CopyableId: vi.fn(() => null),
    SettingsCard: vi.fn(({children}: {children: ReactNode}) => <div>{children}</div>),
  };
});

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'organizationUnits:edit.page.back': 'Back',
        'organizationUnits:edit.page.backToOU': 'Back to Parent OU',
        'organizationUnits:edit.page.error': 'Failed to load organization unit',
        'organizationUnits:edit.page.notFound': 'Organization unit not found',
        'organizationUnits:edit.page.tabs.general': 'General',
        'organizationUnits:edit.page.tabs.childOUs': 'Child OUs',
        'organizationUnits:edit.page.tabs.users': 'Users',
        'organizationUnits:edit.page.tabs.groups': 'Groups',
        'organizationUnits:edit.page.tabs.customization': 'Customization',
        'organizationUnits:edit.customization.labels.theme': 'Theme',
        'organizationUnits:edit.actions.unsavedChanges.label': 'You have unsaved changes',
        'organizationUnits:edit.actions.reset.label': 'Reset',
        'organizationUnits:edit.actions.save.label': 'Save',
        'organizationUnits:edit.actions.saving.label': 'Saving...',
        'organizationUnits:edit.page.description.placeholder': 'Add a description',
        'organizationUnits:edit.page.description.empty': 'No description',
        'organizationUnits:edit.general.handle.label': 'Handle',
        'organizationUnits:edit.general.ou.id.label': 'Organization Unit ID',
        'organizationUnits:edit.users.sections.manage.listing.columns.id': 'User ID',
        'organizationUnits:edit.users.sections.manage.listing.columns.type': 'User Type',
        'organizationUnits:view.groups.title': 'Groups',
        'organizationUnits:view.groups.subtitle': 'Groups in this OU',
        'organizationUnits:edit.users.sections.manage.listing.columns.name': 'Name',
        'organizationUnits:edit.groups.sections.manage.listing.columns.id': 'ID',
        'organizationUnits:edit.general.dangerZone.delete.button.label': 'Delete Organization Unit',
        'organizationUnits:edit.general.dangerZone.delete.title': 'Delete Organization Unit',
        'organizationUnits:edit.general.dangerZone.delete.message':
          'Are you sure you want to delete this organization unit? This action cannot be undone.',
        'organizationUnits:delete.dialog.title': 'Delete Organization Unit',
        'organizationUnits:delete.dialog.message':
          'Are you sure you want to delete this organization unit? This action cannot be undone.',
        'organizationUnits:delete.dialog.disclaimer': 'This action is permanent and cannot be undone.',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
        'common:status.deleting': 'Deleting...',
        'organizationUnits:listing.columns.name': 'Name',
        'organizationUnits:listing.columns.handle': 'Handle',
        'organizationUnits:listing.columns.description': 'Description',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('OrganizationUnitEditPage', () => {
  const mockOrganizationUnit: OrganizationUnit = {
    id: 'ou-123',
    handle: 'test-ou',
    name: 'Test Organization Unit',
    description: 'A test description',
    parent: null,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
    mockMutateAsync.mockReset();
    mockRefetch.mockReset();
    mockDeleteMutate.mockReset();
    mockUseLocation.mockReturnValue({
      state: null,
      pathname: '/organization-units/ou-123',
      search: '',
      hash: '',
      key: 'default',
    });
    mockUseGetOrganizationUnit.mockReturnValue({
      data: mockOrganizationUnit,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
  });

  it('should render organization unit name', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });
  });

  it('should render organization unit description', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('A test description')).toBeInTheDocument();
    });
  });

  it('should render back button', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Back')).toBeInTheDocument();
    });
  });

  it('should navigate back when back button is clicked', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should render all tabs', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('General')).toBeInTheDocument();
      expect(screen.getByText('Child OUs')).toBeInTheDocument();
      expect(screen.getByText('Users')).toBeInTheDocument();
      expect(screen.getByText('Groups')).toBeInTheDocument();
      expect(screen.getByText('Customization')).toBeInTheDocument();
    });
  });

  it('should show loading state', () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should show error state', async () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
  });

  it('should show not found state when OU is undefined', async () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Organization unit not found')).toBeInTheDocument();
    });
  });

  it('should switch tabs when clicked', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('General')).toBeInTheDocument();
    });

    // Click on Customization tab
    fireEvent.click(screen.getByRole('tab', {name: 'Customization'}));

    await waitFor(() => {
      expect(screen.getByText('Theme')).toBeInTheDocument();
    });
  });

  it('should show edit button for name', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // There should be edit buttons
    const editButtons = screen.getAllByRole('button');
    expect(editButtons.length).toBeGreaterThan(0);
  });

  it('should render "No description" when description is null', async () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: {...mockOrganizationUnit, description: null},
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('No description')).toBeInTheDocument();
    });
  });

  it('should show floating save bar when changes are made', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Find and click edit button for name (second button after back)
    const editButtons = screen.getAllByRole('button');
    // The edit button is near the name
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    // Type new name - get by current display value
    const nameInput = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(nameInput, {target: {value: 'Updated Name'}});
    fireEvent.blur(nameInput);

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should reset changes when reset button is clicked', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Make a change to show the floating bar
    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    const nameInput = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(nameInput, {target: {value: 'Updated Name'}});
    fireEvent.blur(nameInput);

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    // Click reset
    fireEvent.click(screen.getByText('Reset'));

    await waitFor(() => {
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  it('should edit description when edit button is clicked', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('A test description')).toBeInTheDocument();
    });

    // Find edit button next to description
    const editButtons = screen.getAllByRole('button');
    const descriptionEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('A test description'),
    );

    expect(descriptionEditButton).toBeDefined();
    fireEvent.click(descriptionEditButton!);

    // Should show a textbox for editing - get by current display value
    await waitFor(() => {
      const textbox = screen.getByDisplayValue('A test description');
      expect(textbox).toBeInTheDocument();
    });

    const textbox = screen.getByDisplayValue('A test description');
    fireEvent.change(textbox, {target: {value: 'Updated description'}});
    fireEvent.blur(textbox);

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should cancel description editing on Escape key', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('A test description')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByRole('button');
    const descriptionEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('A test description'),
    );

    expect(descriptionEditButton).toBeDefined();
    fireEvent.click(descriptionEditButton!);

    await waitFor(() => {
      expect(screen.getByDisplayValue('A test description')).toBeInTheDocument();
    });

    const textbox = screen.getByDisplayValue('A test description');
    fireEvent.keyDown(textbox, {key: 'Escape'});

    await waitFor(() => {
      expect(screen.getByText('A test description')).toBeInTheDocument();
    });
  });

  it('should save description on Ctrl+Enter', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('A test description')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByRole('button');
    const descriptionEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('A test description'),
    );

    expect(descriptionEditButton).toBeDefined();
    fireEvent.click(descriptionEditButton!);

    await waitFor(() => {
      expect(screen.getByDisplayValue('A test description')).toBeInTheDocument();
    });

    const textbox = screen.getByDisplayValue('A test description');
    fireEvent.change(textbox, {target: {value: 'New description'}});
    fireEvent.keyDown(textbox, {key: 'Enter', ctrlKey: true});

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should cancel name editing on Escape key', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Organization Unit')).toBeInTheDocument();
    });

    const textbox = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.keyDown(textbox, {key: 'Escape'});

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });
  });

  it('should save name on Enter key', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Organization Unit')).toBeInTheDocument();
    });

    const textbox = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(textbox, {target: {value: 'Updated Name'}});
    fireEvent.keyDown(textbox, {key: 'Enter'});

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should call save and refetch when save button is clicked', async () => {
    mockMutateAsync.mockResolvedValue({});

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Make a change to show the save bar
    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    const nameInput = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(nameInput, {target: {value: 'Updated Name'}});
    fireEvent.blur(nameInput);

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    // Click save
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'ou-123',
          data: expect.objectContaining({
            name: 'Updated Name',
          }) as unknown,
        }),
      );
    });
  });

  it('should handle save error gracefully', async () => {
    mockMutateAsync.mockRejectedValue(new Error('Save error'));

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Make a change
    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    const nameInput = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(nameInput, {target: {value: 'Updated Name'}});
    fireEvent.blur(nameInput);

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    // Click save - should not throw
    fireEvent.click(screen.getByText('Save'));
  });

  it('should not save empty name on blur', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    const nameInput = screen.getByDisplayValue('Test Organization Unit');
    fireEvent.change(nameInput, {target: {value: ''}});
    fireEvent.blur(nameInput);

    // Should not show unsaved changes for empty name
    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });
  });

  it('should navigate back from error state', async () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });

    // Click back button
    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should navigate back from not found state', async () => {
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Organization unit not found')).toBeInTheDocument();
    });

    // Click back button
    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should switch to child OUs tab', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('tab', {name: 'Child OUs'}));

    await waitFor(() => {
      expect(screen.getByRole('tab', {name: 'Child OUs', selected: true})).toBeInTheDocument();
    });
  });

  it('should switch to users tab', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('tab', {name: 'Users'}));

    await waitFor(() => {
      expect(screen.getByRole('tab', {name: 'Users', selected: true})).toBeInTheDocument();
    });
  });

  it('should switch to groups tab', async () => {
    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('tab', {name: 'Groups'}));

    await waitFor(() => {
      expect(screen.getByRole('tab', {name: 'Groups', selected: true})).toBeInTheDocument();
    });
  });

  it('should navigate back to parent OU when fromOU is provided', async () => {
    mockUseLocation.mockReturnValue({
      state: {
        fromOU: {
          id: 'parent-ou-id',
          name: 'Parent OU',
        },
      },
      pathname: '/organization-units/ou-123',
      search: '',
      hash: '',
      key: 'default',
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Back button should show the parent OU name - find by partial text
    const backButton = screen.getByText('Back to Parent OU');
    fireEvent.click(backButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/parent-ou-id');
    });
  });

  it('should handle back navigation error in error state', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });

    // Click back button - should not throw
    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should handle back navigation error in not found state', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));
    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Organization unit not found')).toBeInTheDocument();
    });

    // Click back button - should not throw
    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should handle back navigation error in main view', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
    });

    // Click back button - should not throw
    fireEvent.click(screen.getByText('Back'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should handle delete success and navigate to list', async () => {
    // Mock delete to trigger onSuccess
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess?: () => void}) => {
      options.onSuccess?.();
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    // Open delete dialog
    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Delete Organization Unit'));

    await waitFor(() => {
      expect(
        screen.getByText('Are you sure you want to delete this organization unit? This action cannot be undone.'),
      ).toBeInTheDocument();
    });

    // Find and click the delete confirm button in dialog
    const deleteButtons = screen.getAllByText('Delete');
    const confirmDeleteButton = deleteButtons.find((btn) => btn.closest('.MuiDialog-root'));
    expect(confirmDeleteButton).toBeDefined();
    fireEvent.click(confirmDeleteButton!);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  it('should handle delete success navigation error gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));
    // Mock delete to trigger onSuccess
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess?: () => void}) => {
      options.onSuccess?.();
    });

    renderWithProviders(<OrganizationUnitEditPage />);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    // Open delete dialog
    fireEvent.click(screen.getByText('Delete Organization Unit'));

    await waitFor(() => {
      expect(
        screen.getByText('Are you sure you want to delete this organization unit? This action cannot be undone.'),
      ).toBeInTheDocument();
    });

    // Find and click the delete confirm button in dialog
    const deleteButtons = screen.getAllByText('Delete');
    const confirmDeleteButton = deleteButtons.find((btn) => btn.closest('.MuiDialog-root'));
    expect(confirmDeleteButton).toBeDefined();
    fireEvent.click(confirmDeleteButton!);

    // Should not throw - error is logged
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units');
    });
  });

  describe('Avatar Image', () => {
    it('should hide avatar image when image fails to load', () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {...mockOrganizationUnit, logoUrl: 'https://example.com/logo.png'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      const avatar = screen.getByRole('img');
      fireEvent.error(avatar);

      expect(avatar).toHaveStyle({display: 'none'});
    });

    it('should open logo modal when avatar is clicked', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {...mockOrganizationUnit, logoUrl: 'https://example.com/logo.png'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      const avatar = screen.getByRole('img');
      fireEvent.click(avatar);

      await waitFor(() => {
        const modal = screen.getByTestId('emoji-picker');
        expect(modal).toHaveStyle({display: 'block'});
      });
    });

    it('should display edited logoUrl in avatar when editedOU has logoUrl', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {...mockOrganizationUnit, logoUrl: 'https://example.com/original.png'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      // Open logo modal and update logo
      const avatar = screen.getByRole('img');
      fireEvent.click(avatar);

      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'block'});
      });

      fireEvent.click(screen.getByText('Select Icon'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });
  });

  describe('Logo Update Modal', () => {
    it('should open logo modal when edit icon button is clicked', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {
          ...mockOrganizationUnit,
          logoUrl: 'emoji:🚀',
        },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      const logoEditButton = await screen.findByLabelText('organizationUnits:edit.page.logoUpdate.label');
      fireEvent.click(logoEditButton);

      await waitFor(() => {
        const modal = screen.getByTestId('emoji-picker');
        expect(modal).toHaveStyle({display: 'block'});
      });
    });

    it('should close logo modal when close button is clicked', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {
          ...mockOrganizationUnit,
          logoUrl: 'emoji:🚀',
        },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      // Open the modal via logo edit icon button
      const logoEditButton = await screen.findByLabelText('organizationUnits:edit.page.logoUpdate.label');
      fireEvent.click(logoEditButton);

      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'block'});
      });

      // Close the modal
      fireEvent.click(screen.getByText('Close'));

      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'none'});
      });
    });

    it('should update logo and close modal when logo is updated', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {
          ...mockOrganizationUnit,
          logoUrl: 'emoji:🚀',
        },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      // Open the modal
      const logoEditButton = await screen.findByLabelText('organizationUnits:edit.page.logoUpdate.label');
      fireEvent.click(logoEditButton);

      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'block'});
      });

      // Click update logo
      fireEvent.click(screen.getByText('Select Icon'));

      // Modal should close
      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'none'});
      });

      // Should show unsaved changes
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  describe('Delete Error and Snackbar', () => {
    it('should show error snackbar when delete fails', async () => {
      // Mock delete to trigger onError
      mockDeleteMutate.mockImplementation((_id: string, options: {onError?: (err: Error) => void}) => {
        options.onError?.(new Error('Delete failed'));
      });

      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
      });

      // Open delete dialog
      fireEvent.click(screen.getByText('Delete Organization Unit'));

      await waitFor(() => {
        expect(
          screen.getByText('Are you sure you want to delete this organization unit? This action cannot be undone.'),
        ).toBeInTheDocument();
      });

      // Find and click the delete confirm button in dialog
      const deleteButtons = screen.getAllByText('Delete');
      const confirmDeleteButton = deleteButtons.find((btn) => btn.closest('.MuiDialog-root'));
      expect(confirmDeleteButton).toBeDefined();
      fireEvent.click(confirmDeleteButton!);

      // Snackbar should appear with error
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });
    });
  });

  describe('Edited OU Fallbacks', () => {
    it('should display edited name when re-editing after a name change', async () => {
      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
      });

      // First edit: change name
      const editButtons = screen.getAllByRole('button');
      const nameEditButton = editButtons.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
      );

      expect(nameEditButton).toBeDefined();
      fireEvent.click(nameEditButton!);
      const nameInput = screen.getByDisplayValue('Test Organization Unit');
      fireEvent.change(nameInput, {target: {value: 'Updated Name'}});
      fireEvent.blur(nameInput);

      await waitFor(() => {
        expect(screen.getByText('Updated Name')).toBeInTheDocument();
      });

      // Second edit: the input should show the edited name
      const editButtons2 = screen.getAllByRole('button');
      const nameEditButton2 = editButtons2.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Updated Name'),
      );

      expect(nameEditButton2).toBeDefined();
      fireEvent.click(nameEditButton2!);
      expect(screen.getByDisplayValue('Updated Name')).toBeInTheDocument();
    }, 15_000);

    it('should display edited description when re-editing after a description change', async () => {
      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('A test description')).toBeInTheDocument();
      });

      // First edit: change description
      const editButtons = screen.getAllByRole('button');
      const descEditButton = editButtons.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('A test description'),
      );

      expect(descEditButton).toBeDefined();
      fireEvent.click(descEditButton!);
      const descInput = screen.getByDisplayValue('A test description');
      fireEvent.change(descInput, {target: {value: 'Updated Description'}});
      fireEvent.blur(descInput);

      await waitFor(() => {
        expect(screen.getByText('Updated Description')).toBeInTheDocument();
      });

      // Second edit: Escape should restore the edited description
      const editButtons2 = screen.getAllByRole('button');
      const descEditButton2 = editButtons2.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Updated Description'),
      );

      expect(descEditButton2).toBeDefined();
      fireEvent.click(descEditButton2!);
      fireEvent.keyDown(screen.getByDisplayValue('Updated Description'), {key: 'Escape'});

      await waitFor(() => {
        expect(screen.getByText('Updated Description')).toBeInTheDocument();
      });
    });

    it('should restore edited name on Escape key when editedOU has name', async () => {
      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
      });

      // First edit: change name
      const editButtons = screen.getAllByRole('button');
      const nameEditButton = editButtons.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
      );

      expect(nameEditButton).toBeDefined();
      fireEvent.click(nameEditButton!);
      const nameInput = screen.getByDisplayValue('Test Organization Unit');
      fireEvent.change(nameInput, {target: {value: 'Edited Name'}});
      fireEvent.blur(nameInput);

      await waitFor(() => {
        expect(screen.getByText('Edited Name')).toBeInTheDocument();
      });

      // Re-edit and press Escape
      const editButtons2 = screen.getAllByRole('button');
      const nameEditButton2 = editButtons2.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Edited Name'),
      );

      expect(nameEditButton2).toBeDefined();
      fireEvent.click(nameEditButton2!);
      const nameInput2 = screen.getByDisplayValue('Edited Name');
      fireEvent.change(nameInput2, {target: {value: 'Something Else'}});
      fireEvent.keyDown(nameInput2, {key: 'Escape'});

      // Should restore to the edited name, not the original
      await waitFor(() => {
        expect(screen.getByText('Edited Name')).toBeInTheDocument();
      });
    });

    it('should save name changes on Enter with trimmed value', async () => {
      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
      });

      const editButtons = screen.getAllByRole('button');
      const nameEditButton = editButtons.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Test Organization Unit'),
      );

      expect(nameEditButton).toBeDefined();
      fireEvent.click(nameEditButton!);
      const nameInput = screen.getByDisplayValue('Test Organization Unit');
      fireEvent.change(nameInput, {target: {value: ''}});
      fireEvent.keyDown(nameInput, {key: 'Enter'});

      // Should not save empty name, should exit editing
      await waitFor(() => {
        expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
      });
    });
  });

  describe('Save with edited fields', () => {
    it('should include description and themeId in save when edited', async () => {
      mockMutateAsync.mockResolvedValue({});

      renderWithProviders(<OrganizationUnitEditPage />);

      await waitFor(() => {
        expect(screen.getByText('Test Organization Unit')).toBeInTheDocument();
      });

      // Make a description change
      const editButtons = screen.getAllByRole('button');
      const descEditButton = editButtons.find(
        (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('A test description'),
      );

      expect(descEditButton).toBeDefined();
      fireEvent.click(descEditButton!);
      const descInput = screen.getByDisplayValue('A test description');
      fireEvent.change(descInput, {target: {value: 'New description'}});
      fireEvent.blur(descInput);

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Click save
      fireEvent.click(screen.getByText('Save'));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            id: 'ou-123',
            data: expect.objectContaining({
              description: 'New description',
            }) as unknown,
          }),
        );
      });
    });
  });
});
