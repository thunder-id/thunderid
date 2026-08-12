// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import GroupConstraints from '../../constants/group-constraints';
import GroupEditPage from '../GroupEditPage';

vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    CopyableId: vi.fn(({value}: {value: string}) => (
      <span
        data-testid="copyable-id"
        role="button"
        tabIndex={0}
        onClick={() => void navigator.clipboard.writeText(value)}
        onKeyDown={(e: {key: string}) => {
          if (e.key === 'Enter' || e.key === ' ') {
            void navigator.clipboard.writeText(value);
          }
        }}
      >
        {value}
      </span>
    )),
  };
});

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({groupId: 'g1'}),
    Link: ({to, children = undefined, ...props}: {to: string; children?: ReactNode; [key: string]: unknown}) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e: {preventDefault: () => void}) => {
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
});

const mockUseGetGroup = vi.fn();
vi.mock('../../api/useGetGroup', () => ({
  default: (...args: unknown[]): unknown => mockUseGetGroup(...args),
}));

const mockMutateAsync = vi.fn();
const mockUpdateGroupReset = vi.fn();
let mockIsPending = false;
let mockUpdateError: Error | null = null;
vi.mock('../../api/useUpdateGroup', () => ({
  default: () => ({
    mutateAsync: mockMutateAsync,
    reset: mockUpdateGroupReset,
    get isPending() {
      return mockIsPending;
    },
    get error() {
      return mockUpdateError;
    },
  }),
}));

vi.mock('../../components/GroupDeleteDialog', () => ({
  default: ({
    open,
    onClose,
    onSuccess,
  }: {
    open: boolean;
    groupId: string | null;
    onClose: () => void;
    onSuccess?: () => void;
  }) =>
    open ? (
      <div data-testid="delete-dialog">
        <button type="button" data-testid="close-delete-dialog" onClick={onClose}>
          Close
        </button>
        <button
          type="button"
          data-testid="delete-success"
          onClick={() => {
            onClose();
            onSuccess?.();
          }}
        >
          Confirm Delete
        </button>
      </div>
    ) : null,
}));

vi.mock('../../components/edit-group/general-settings/EditGeneralSettings', () => ({
  default: ({group}: {group: {id: string; name: string}}) => (
    <div data-testid="general-settings">
      <span>{group.name}</span>
    </div>
  ),
}));

vi.mock('../../components/edit-group/members-settings/EditMembersSettings', () => ({
  default: ({group}: {group: {id: string; name: string}}) => (
    <div data-testid="members-settings">
      <span>Members of {group.name}</span>
    </div>
  ),
}));

vi.mock('../../components/edit-group/advanced-settings/EditAdvancedSettings', () => ({
  default: ({onDeleteClick}: {onDeleteClick: () => void}) => (
    <div data-testid="advanced-settings">
      <button type="button" data-testid="delete-click" onClick={onDeleteClick}>
        Delete
      </button>
    </div>
  ),
}));

const mockGroup = {
  id: 'g1',
  name: 'Test Group',
  description: 'A test group',
  ouId: 'ou-1',
  members: [],
};

describe('GroupEditPage', () => {
  let mockRefetch: ReturnType<typeof vi.fn>;
  let mockWriteText: ReturnType<typeof vi.fn>;
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    vi.clearAllMocks();
    mockIsPending = false;
    mockUpdateError = null;
    mockNavigate.mockResolvedValue(undefined);
    mockRefetch = vi.fn().mockResolvedValue(undefined);
    mockWriteText = vi.fn().mockResolvedValue(undefined);
    mockUseGetGroup.mockReturnValue({
      data: mockGroup,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText: mockWriteText},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
  });

  it('should render loading state', () => {
    mockUseGetGroup.mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      refetch: vi.fn(),
    });
    renderWithProviders(<GroupEditPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render error state', () => {
    mockUseGetGroup.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Fetch failed'),
      refetch: vi.fn(),
    });
    renderWithProviders(<GroupEditPage />);

    // Resolved through the i18n catalog, not the raw (unlocalized) error message.
    expect(screen.getByText('Failed to load group')).toBeInTheDocument();
    expect(screen.getByText('Back to Groups')).toBeInTheDocument();
  });

  it('should render not found state when no group', () => {
    mockUseGetGroup.mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderWithProviders(<GroupEditPage />);

    expect(screen.getByText('Group not found')).toBeInTheDocument();
  });

  it('should render group name and description', () => {
    renderWithProviders(<GroupEditPage />);

    expect(screen.getAllByText('Test Group').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('A test group')).toBeInTheDocument();
  });

  it('should render back button and navigate on click', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Back to Groups'));

    expect(mockNavigate).toHaveBeenCalledWith('/groups');
  });

  it('should render tabs for general and members', () => {
    renderWithProviders(<GroupEditPage />);

    expect(screen.getByText('General')).toBeInTheDocument();
    expect(screen.getByText('Members')).toBeInTheDocument();
  });

  it('should show general settings by default', () => {
    renderWithProviders(<GroupEditPage />);

    expect(screen.getByTestId('general-settings')).toBeInTheDocument();
  });

  it('should switch to members tab', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Members'));

    expect(screen.getByTestId('members-settings')).toBeInTheDocument();
  });

  it('should open delete dialog from advanced settings', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Advanced'));
    await user.click(screen.getByTestId('delete-click'));

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should close delete dialog', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Advanced'));
    await user.click(screen.getByTestId('delete-click'));
    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByTestId('close-delete-dialog'));
    await waitFor(() => {
      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });
  });

  it('should navigate on successful delete', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Advanced'));
    await user.click(screen.getByTestId('delete-click'));
    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByTestId('delete-success'));
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups');
    });
  });

  it('should not show floating action bar initially', () => {
    renderWithProviders(<GroupEditPage />);

    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  it('should call useGetGroup with the groupId from params', () => {
    renderWithProviders(<GroupEditPage />);

    expect(mockUseGetGroup).toHaveBeenCalledWith('g1');
  });

  it('should navigate back from error state', async () => {
    mockUseGetGroup.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Fetch failed'),
      refetch: vi.fn(),
    });
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Back to Groups'));

    expect(mockNavigate).toHaveBeenCalledWith('/groups');
  });

  it('should show empty description placeholder when no description', () => {
    mockUseGetGroup.mockReturnValue({
      data: {...mockGroup, description: undefined},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderWithProviders(<GroupEditPage />);

    expect(screen.getByText('No description')).toBeInTheDocument();
  });

  it('should enter name editing mode and save on Enter', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    // Find the name heading (h3) and the adjacent edit button
    const nameHeadings = screen.getAllByText('Test Group');
    // The h3 heading is the one rendered by GroupEditPage (not the mock)
    const h3Heading = nameHeadings.find((el) => el.tagName === 'H3');
    expect(h3Heading).toBeTruthy();
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    expect(nameEditBtn).toBeTruthy();
    await user.click(nameEditBtn!);

    // Should show a text field with the current name
    const nameInput = screen.getByDisplayValue('Test Group');
    expect(nameInput).toBeInTheDocument();

    // Clear and type new name
    await user.clear(nameInput);
    await user.type(nameInput, 'Updated Name');
    await user.keyboard('{Enter}');

    // Should show floating action bar
    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should hide the action bar when the name is retyped back to its original value', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const editName = async (to: string): Promise<void> => {
      const h3 = screen.getAllByText(/Test Group|Renamed Group/).find((el) => el.tagName === 'H3');
      await user.click(h3!.parentElement!.querySelector('button')!);
      const input = screen.getByRole('textbox');
      await user.clear(input);
      await user.type(input, to);
      await user.keyboard('{Enter}');
    };

    await editName('Renamed Group');
    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    await editName('Test Group');
    await waitFor(() => {
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  it('should discard a rename that exceeds the maximum length', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const editName = async (to: string): Promise<void> => {
      const h3 = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
      await user.click(h3!.parentElement!.querySelector('button')!);
      const input = screen.getByRole('textbox');
      fireEvent.change(input, {target: {value: to}});
      fireEvent.keyDown(input, {key: 'Enter'});
    };

    await editName('a'.repeat(GroupConstraints.NAME_MAX_LENGTH + 1));
    await waitFor(() => {
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    expect(screen.getAllByText('Test Group').some((el) => el.tagName === 'H3')).toBe(true);
  });

  it('should cancel name editing on Escape', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);

    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'Updated Name');
    await user.keyboard('{Escape}');

    // Should revert to original name and exit editing mode
    expect(screen.queryByDisplayValue('Updated Name')).not.toBeInTheDocument();
    expect(screen.getAllByText('Test Group').length).toBeGreaterThanOrEqual(1);
  });

  it('should save name on blur', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);

    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'Blur Name');
    await user.tab(); // trigger blur

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should enter description editing mode and save on Ctrl+Enter', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const descText = screen.getByText('A test group');
    const descEditBtn = descText.parentElement?.querySelector('button');
    expect(descEditBtn).toBeTruthy();
    await user.click(descEditBtn!);

    const descInput = screen.getByDisplayValue('A test group');
    expect(descInput).toBeInTheDocument();

    await user.clear(descInput);
    await user.type(descInput, 'Updated description');

    fireEvent.keyDown(descInput, {key: 'Enter', ctrlKey: true});

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should cancel description editing on Escape', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const descText = screen.getByText('A test group');
    const descEditBtn = descText.parentElement?.querySelector('button');
    await user.click(descEditBtn!);

    const descInput = screen.getByDisplayValue('A test group');
    await user.clear(descInput);
    await user.type(descInput, 'Some new text');
    await user.keyboard('{Escape}');

    expect(screen.queryByDisplayValue('Some new text')).not.toBeInTheDocument();
  });

  it('should save description on blur', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const descText = screen.getByText('A test group');
    const descEditBtn = descText.parentElement?.querySelector('button');
    await user.click(descEditBtn!);

    const descInput = screen.getByDisplayValue('A test group');
    await user.clear(descInput);
    await user.type(descInput, 'Blurred desc');
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  it('should show empty placeholder after clearing description', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const descText = screen.getByText('A test group');
    const descEditBtn = descText.parentElement?.querySelector('button');
    await user.click(descEditBtn!);

    const descInput = screen.getByDisplayValue('A test group');
    await user.clear(descInput);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText('No description')).toBeInTheDocument();
    });
  });

  it('should save changes when save button is clicked', async () => {
    mockMutateAsync.mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    // Edit the name to trigger hasChanges
    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Save Changes'));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        groupId: 'g1',
        data: {
          name: 'New Name',
          description: 'A test group',
          ouId: 'ou-1',
        },
      });
    });

    expect(mockRefetch).toHaveBeenCalled();
  });

  it('should display the generic error inline in the save bar, not a toast, when save fails', async () => {
    mockUpdateError = new Error('Request failed');
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    // Edit the name to trigger hasChanges
    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByText('Failed to update group. Please try again.')).toBeInTheDocument();
    });
  });

  it('should reset changes when reset button is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    // Edit the name
    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Reset'));

    await waitFor(() => {
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  it('should navigate back from not found state', async () => {
    mockUseGetGroup.mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Back to Groups'));

    expect(mockNavigate).toHaveBeenCalledWith('/groups');
  });

  it('should handle navigate rejection from back button gracefully', async () => {
    mockNavigate.mockRejectedValue(new Error('Nav failed'));
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    await user.click(screen.getByText('Back to Groups'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups');
    });
  });

  it('should clear a stale update error as soon as another field changes', async () => {
    mockUpdateError = new Error('Request failed');
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByText('Failed to update group. Please try again.')).toBeInTheDocument();
    });

    expect(mockUpdateGroupReset).toHaveBeenCalled();
  });

  it('should clear the update error when Reset is clicked', async () => {
    mockUpdateError = new Error('Request failed');
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(screen.getByText('Reset')).toBeInTheDocument();
    });

    mockUpdateGroupReset.mockClear();
    await user.click(screen.getByText('Reset'));

    expect(mockUpdateGroupReset).toHaveBeenCalled();
  });

  it('should show mapped error message when the group name conflicts on save', async () => {
    const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
    error.response = {data: {code: 'GRP-1004'}};
    mockUpdateError = error;
    const user = userEvent.setup();
    renderWithProviders(<GroupEditPage />);

    // Edit the name to trigger hasChanges, so the save bar (and its error) is visible
    const h3Heading = screen.getAllByText('Test Group').find((el) => el.tagName === 'H3');
    const nameEditBtn = h3Heading!.parentElement?.querySelector('button');
    await user.click(nameEditBtn!);
    const nameInput = screen.getByDisplayValue('Test Group');
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(
        screen.getByText('A group with this name already exists in this organization unit. Choose a different name.'),
      ).toBeInTheDocument();
    });
  });
});
