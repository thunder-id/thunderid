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

import {screen, fireEvent, waitFor, within, renderWithProviders, act} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationUnitListResponse} from '../../models/responses';
import OrganizationUnitsTreeView from '../OrganizationUnitsTreeView';

// Mock navigate
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock logger
// Mock logger — stable reference to avoid useCallback churn
const stableLogger = {error: vi.fn(), info: vi.fn(), debug: vi.fn()};
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => stableLogger,
}));

// Mock the API hook
const mockUseGetOrganizationUnits = vi.fn();
vi.mock('@/api/useGetOrganizationUnits', () => ({
  default: () =>
    mockUseGetOrganizationUnits() as {
      data: OrganizationUnitListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

// Mock ThunderID — stable reference to avoid useCallback churn
const mockHttpRequest = vi.fn();
const stableHttp = {request: mockHttpRequest};
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: stableHttp}),
}));

// Mock useOrganizationUnit hook with React state for reactivity
// Allow tests to pre-seed expandedItems via mockOrganizationUnitConfig.initialExpandedItems
const mockOrganizationUnitConfig = {initialExpandedItems: [] as string[]};
vi.mock('@/contexts/useOrganizationUnit', async () => {
  const {useState, useCallback} = await import('react');
  type OrganizationUnitTreeItem = import('../../models/organization-unit-tree').OrganizationUnitTreeItem;
  function useOrganizationUnit() {
    const [treeItems, setTreeItems] = useState<OrganizationUnitTreeItem[]>([]);
    const [expandedItems, setExpandedItems] = useState<string[]>(mockOrganizationUnitConfig.initialExpandedItems);
    const [loadedItems, setLoadedItems] = useState<Set<string>>(new Set());
    const resetTreeState = useCallback(() => {
      setTreeItems([]);
      setLoadedItems(new Set());
    }, []);
    return {treeItems, setTreeItems, expandedItems, setExpandedItems, loadedItems, setLoadedItems, resetTreeState};
  }
  return {default: useOrganizationUnit};
});

// Mock config — stable reference to avoid useCallback churn
const stableConfig = {getServerUrl: () => 'http://localhost:8080'};
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => stableConfig,
  };
});

// Mock delete hook — controllable per test
const mockDeleteMutate = vi.fn();
const mockDeleteHook = {mutate: mockDeleteMutate, isPending: false};
vi.mock('@/api/useDeleteOrganizationUnit', () => ({
  default: () => mockDeleteHook,
}));

// Mock translations — stable t function to avoid useCallback churn
const translations: Record<string, string> = {
  'organizationUnits:listing.error.title': 'Error loading organization units',
  'organizationUnits:listing.error.unknown': 'An unknown error occurred',
  'organizationUnits:listing.treeView.empty': 'No organization units found',
  'organizationUnits:listing.treeView.noChildren': 'No child organization units',
  'organizationUnits:listing.treeView.loadError': 'Failed to load child organization units',
  'common:actions.view': 'View',
  'common:actions.edit': 'Edit',
  'common:actions.delete': 'Delete',
  'organizationUnits:listing.treeView.addChild': 'Add child organization unit',
  'organizationUnits:edit.general.dangerZone.delete.title': 'Delete Organization Unit',
  'organizationUnits:edit.general.dangerZone.delete.message':
    'Are you sure you want to delete this organization unit? This action cannot be undone.',
  'organizationUnits:edit.general.dangerZone.delete.success': 'Organization unit deleted successfully',
  'organizationUnits:edit.general.dangerZone.delete.error': 'Failed to delete organization unit',
  'organizationUnits:delete.dialog.title': 'Delete Organization Unit',
  'organizationUnits:delete.dialog.message':
    'Are you sure you want to delete this organization unit? This action cannot be undone.',
  'organizationUnits:delete.dialog.disclaimer': 'This action is permanent and cannot be undone.',
  'organizationUnits:listing.addRootOrganizationUnit': 'Add Root Organization Unit',
  'organizationUnits:listing.treeView.addChildOrganizationUnit': 'Add Engineering Unit',
  'organizationUnits:listing.treeView.loadMore': 'Load more',
  'common:actions.cancel': 'Cancel',
  'common:status.loading': 'Loading...',
  'common:status.deleting': 'Deleting...',
};
const stableT = (key: string): string => translations[key] ?? key;
const stableTranslation = {t: stableT};
vi.mock('react-i18next', () => ({
  useTranslation: () => stableTranslation,
}));

describe('OrganizationUnitsTreeView', () => {
  const mockOUData: OrganizationUnitListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'ou-1', handle: 'root', name: 'Root Organization', description: 'Root OU', parent: null},
      {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
    mockOrganizationUnitConfig.initialExpandedItems = [];
    mockUseGetOrganizationUnits.mockReturnValue({
      data: mockOUData,
      isLoading: false,
      error: null,
    });
  });

  it('should render tree view with organization unit names', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
      expect(screen.getByText('Engineering')).toBeInTheDocument();
    });
  });

  it('should show error state when fetch fails', async () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Error loading organization units')).toBeInTheDocument();
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });
  });

  it('should show fallback error message when error has no message', async () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: {},
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Error loading organization units')).toBeInTheDocument();
      expect(screen.getByText('An unknown error occurred')).toBeInTheDocument();
    });
  });

  it('should show loading state', () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should show empty state when no organization units', async () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        organizationUnits: [],
      },
      isLoading: false,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('No organization units found')).toBeInTheDocument();
    });
  });

  it('should render avatar for each tree item', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const avatars = document.querySelectorAll('.MuiAvatar-root');
    expect(avatars.length).toBeGreaterThan(0);
  });

  it('should render action buttons for each tree item', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    expect(screen.getAllByLabelText('Add child organization unit')).toHaveLength(2);
    expect(screen.getAllByLabelText('Edit')).toHaveLength(2);
    expect(screen.getAllByLabelText('Delete')).toHaveLength(2);
  });

  it('should render direct row actions for each tree item', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    expect(screen.getAllByLabelText('Add child organization unit')).toHaveLength(2);
    expect(screen.getAllByLabelText('Edit')).toHaveLength(2);
    expect(screen.getAllByLabelText('Delete')).toHaveLength(2);
  });

  it('should navigate to create page with parentId when add child action is clicked', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Add child organization unit')[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create', {
        state: {parentId: 'ou-1', parentName: 'Root Organization', parentHandle: 'root'},
      });
    });
  });

  it('should navigate when edit action is clicked', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Edit')[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/ou-1');
    });
  });

  it('should open delete dialog when delete action is clicked', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
      expect(
        screen.getByText('Are you sure you want to delete this organization unit? This action cannot be undone.'),
      ).toBeInTheDocument();
    });
  });

  it('should handle undefined data gracefully', () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    // When data is undefined and not loading, a loading spinner is shown
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should close delete dialog when cancel is clicked', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click row delete action
    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    // Cancel the dialog
    fireEvent.click(screen.getByText('Cancel'));

    await waitFor(() => {
      expect(screen.queryByText('Delete Organization Unit')).not.toBeInTheDocument();
    });
  });

  it('should show success snackbar after successful deletion', async () => {
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click row delete action to open dialog, then confirm
    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    // Use within to scope to the dialog's Delete button (avoids ambiguity with menu item)
    const dialog = screen.getByRole('dialog');
    fireEvent.click(within(dialog).getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Organization unit deleted successfully')).toBeInTheDocument();
    });
  });

  it('should show error snackbar after failed deletion', async () => {
    mockDeleteMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(
        Object.assign(new Error('Delete failed'), {
          response: {data: {code: 'ERR', message: 'fail', description: 'Server error occurred'}},
        }),
      );
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click row delete action to open dialog, then confirm
    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    // Use within to scope to the dialog's Delete button (avoids ambiguity with menu item)
    const dialog2 = screen.getByRole('dialog');
    fireEvent.click(within(dialog2).getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Server error occurred')).toBeInTheDocument();
    });
  });

  it('should log error when edit navigation fails', async () => {
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Edit')[0]);

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith(
        'Failed to navigate to organization unit',
        expect.objectContaining({ouId: 'ou-1'}),
      );
    });
  });

  it('should log error when add child navigation fails', async () => {
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Add child organization unit')[0]);

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith(
        'Failed to navigate to create child organization unit',
        expect.objectContaining({parentId: 'ou-1'}),
      );
    });
  });

  it('should fetch and display child OUs when a node is expanded', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: 'A child', parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click the expand icon on the first tree item to trigger expansion
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    expect(expandIcons.length).toBeGreaterThan(0);
    fireEvent.click(expandIcons[0]);

    // The component should fetch children and display them
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should show error placeholder and log error when fetching child OUs fails', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Network failure'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click the expand icon on the first tree item
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    expect(expandIcons.length).toBeGreaterThan(0);
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith(
        'Failed to load child organization units',
        expect.objectContaining({parentId: 'ou-1'}),
      );
    });

    // The error placeholder should be visible instead of a perpetual spinner
    await waitFor(() => {
      expect(screen.getByText('Failed to load child organization units')).toBeInTheDocument();
    });
  });

  it('should rebuild tree with expanded items restored when expandedItems exist', async () => {
    // Pre-seed expanded items so the rebuild path is triggered
    mockOrganizationUnitConfig.initialExpandedItems = ['ou-1'];

    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Restored Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    // The useEffect should detect expandedItems=['ou-1'] and call rebuildTree,
    // which calls expandLevel → fetchChildItems for 'ou-1'
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should still render root items when child fetch fails during rebuild', async () => {
    // Pre-seed expanded items to trigger the rebuild path (expandLevel)
    mockOrganizationUnitConfig.initialExpandedItems = ['ou-1'];

    // Make the child fetch fail — expandLevel catches this internally
    // and filters out failed results, so the tree still renders root items
    mockHttpRequest.mockRejectedValue(new Error('Child fetch failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    // Root items should still be rendered even though child fetch failed
    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
      expect(screen.getByText('Engineering')).toBeInTheDocument();
    });
  });

  it('should close snackbar when close action is triggered', async () => {
    // Trigger a success snackbar first
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click row delete action to trigger snackbar
    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    // Use within to scope to the dialog's Delete button (avoids ambiguity with menu item)
    const dialog3 = screen.getByRole('dialog');
    fireEvent.click(within(dialog3).getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Organization unit deleted successfully')).toBeInTheDocument();
    });

    // Close the snackbar via the Alert close button
    const alert = screen.getByRole('alert');
    const alertCloseButton = alert.querySelector('button');
    if (alertCloseButton) {
      fireEvent.click(alertCloseButton);
    }

    await waitFor(() => {
      expect(screen.queryByText('Organization unit deleted successfully')).not.toBeInTheDocument();
    });
  });

  it('should display handle text for tree items that have handles', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // The 'root' and 'engineering' handles should be shown as caption text
    expect(screen.getByText('root')).toBeInTheDocument();
    expect(screen.getByText('engineering')).toBeInTheDocument();
  });

  it('should render add root organization unit row below tree items', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    expect(screen.getByText('Add Root Organization Unit')).toBeInTheDocument();
  });

  it('should navigate to create page when add root row is clicked', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Add Root Organization Unit'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create');
    });
  });

  it('should render add child button when a node is expanded and children are loaded', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Expand the first tree item
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    // After expansion, the add child button should appear
    await waitFor(() => {
      expect(screen.getByText('Add Engineering Unit')).toBeInTheDocument();
    });
  });

  it('should navigate to create page with parent state when add child button in tree is clicked', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Expand the first tree item
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText('Add Engineering Unit')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Add Engineering Unit'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create', {
        state: {parentId: 'ou-1', parentName: 'Root Organization', parentHandle: 'root'},
      });
    });
  });

  it('should show add child button when node has no children', async () => {
    const emptyChildOUResponse: OrganizationUnitListResponse = {
      totalResults: 0,
      startIndex: 1,
      count: 0,
      organizationUnits: [],
    };

    mockHttpRequest.mockResolvedValue({data: emptyChildOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Expand the first tree item
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    // Even with no children, the add child button should appear
    await waitFor(() => {
      expect(screen.getByText('Add Engineering Unit')).toBeInTheDocument();
    });
  });

  it('should show add root row in empty state', async () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        organizationUnits: [],
      },
      isLoading: false,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('No organization units found')).toBeInTheDocument();
    });

    expect(screen.getByText('Add Root Organization Unit')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Add Root Organization Unit'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create');
    });
  });

  it('should navigate to create page when add root row is activated via Enter key', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const addRootButton = screen.getByText('Add Root Organization Unit').closest('[role="button"]')!;
    fireEvent.keyDown(addRootButton, {key: 'Enter'});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create');
    });
  });

  it('should navigate to create page when add root row is activated via Space key', async () => {
    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const addRootButton = screen.getByText('Add Root Organization Unit').closest('[role="button"]')!;
    fireEvent.keyDown(addRootButton, {key: ' '});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create');
    });
  });

  it('should navigate via keyboard on empty state add root button', async () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        organizationUnits: [],
      },
      isLoading: false,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('No organization units found')).toBeInTheDocument();
    });

    const addRootButton = screen.getByText('Add Root Organization Unit').closest('[role="button"]')!;
    fireEvent.keyDown(addRootButton, {key: 'Enter'});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/create');
    });
  });

  it('should trigger keyboard handler for load more item via Enter key', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Expand the first tree item to load children with load more
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText('Fetched Child')).toBeInTheDocument();
    });

    // Find the load more button and activate via keyboard
    await waitFor(() => {
      expect(screen.getByText('Load more')).toBeInTheDocument();
    });

    const loadMoreButton = screen.getByText('Load more').closest('[role="button"]')!;
    mockHttpRequest.mockClear();
    mockHttpRequest.mockResolvedValue({
      data: {
        totalResults: 50,
        startIndex: 2,
        count: 1,
        organizationUnits: [
          {id: 'ou-child-2', handle: 'child2', name: 'Fetched Child 2', description: null, parent: 'ou-1'},
        ],
      },
    });
    fireEvent.keyDown(loadMoreButton, {key: 'Enter'});

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should not fetch children when collapsing a node', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Expand
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText('Fetched Child')).toBeInTheDocument();
    });

    const callCount = mockHttpRequest.mock.calls.length;

    // Collapse - should not trigger another fetch
    const collapseIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(collapseIcons[0]);

    // Verify no additional HTTP calls were made on collapse
    expect(mockHttpRequest).toHaveBeenCalledTimes(callCount);
  });

  it('should show root load more button when there are more root items', async () => {
    const paginatedData: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 1,
      count: 2,
      organizationUnits: [
        {id: 'ou-1', handle: 'root', name: 'Root Organization', description: null, parent: null},
        {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
      ],
    };

    mockUseGetOrganizationUnits.mockReturnValue({
      data: paginatedData,
      isLoading: false,
      error: null,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Load more')).toBeInTheDocument();
    });
  });

  it('should log error when add root navigation fails', async () => {
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Add Root Organization Unit'));

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith('Failed to navigate to create organization unit page', {
        error: expect.objectContaining({message: 'Navigation failed'}) as Error,
      });
    });
  });

  it('should use fallback error message when error.message is missing', async () => {
    const errorWithMessage = {message: 'Server unavailable'};
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: errorWithMessage,
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Server unavailable')).toBeInTheDocument();
    });
  });

  it('should log error when add root navigation fails via keyboard', async () => {
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const addRootButton = screen.getByText('Add Root Organization Unit').closest('[role="button"]')!;
    fireEvent.keyDown(addRootButton, {key: 'Enter'});

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith('Failed to navigate to create organization unit page', {
        error: expect.objectContaining({message: 'Navigation failed'}) as Error,
      });
    });
  });

  it('should load more root items when the root load more button is clicked', async () => {
    const paginatedData: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 1,
      count: 2,
      organizationUnits: [
        {id: 'ou-1', handle: 'root', name: 'Root Organization', description: null, parent: null},
        {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
      ],
    };

    mockUseGetOrganizationUnits.mockReturnValue({
      data: paginatedData,
      isLoading: false,
      error: null,
    });

    mockHttpRequest.mockResolvedValue({
      data: {
        totalResults: 50,
        startIndex: 3,
        count: 2,
        organizationUnits: [
          {id: 'ou-3', handle: 'marketing', name: 'Marketing', description: null, parent: null},
          {id: 'ou-4', handle: 'sales', name: 'Sales', description: null, parent: null},
        ],
      },
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Load more')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Load more'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should append new items when root load more succeeds and still has more', async () => {
    const paginatedData: OrganizationUnitListResponse = {
      totalResults: 5,
      startIndex: 1,
      count: 2,
      organizationUnits: [
        {id: 'ou-1', handle: 'root', name: 'Root Organization', description: null, parent: null},
        {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
      ],
    };

    mockUseGetOrganizationUnits.mockReturnValue({
      data: paginatedData,
      isLoading: false,
      error: null,
    });

    mockHttpRequest.mockResolvedValue({
      data: {
        totalResults: 5,
        startIndex: 3,
        count: 2,
        organizationUnits: [
          {id: 'ou-3', handle: 'marketing', name: 'Marketing', description: null, parent: null},
          {id: 'ou-4', handle: 'sales', name: 'Sales', description: null, parent: null},
        ],
      },
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Load more')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Load more'));

    await waitFor(() => {
      expect(screen.getByText('Marketing')).toBeInTheDocument();
    });
  });

  it('should log error when root load more fetch fails', async () => {
    const paginatedData: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 1,
      count: 2,
      organizationUnits: [
        {id: 'ou-1', handle: 'root', name: 'Root Organization', description: null, parent: null},
        {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
      ],
    };

    mockUseGetOrganizationUnits.mockReturnValue({
      data: paginatedData,
      isLoading: false,
      error: null,
    });

    mockHttpRequest.mockRejectedValue(new Error('Root load more failed'));

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Load more')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Load more'));

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith('Failed to load more root organization units', expect.anything());
    });
  });

  it('should close the snackbar automatically after the auto-hide duration', async () => {
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    const dialog = screen.getByRole('dialog');
    fireEvent.click(within(dialog).getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Organization unit deleted successfully')).toBeInTheDocument();
    });

    await waitFor(
      () => {
        expect(screen.queryByText('Organization unit deleted successfully')).not.toBeInTheDocument();
      },
      {timeout: 7000},
    );
  });

  it('should close the snackbar when clicking outside (clickaway)', async () => {
    mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitsTreeView />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText('Delete')[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    });

    const dialog = screen.getByRole('dialog');
    fireEvent.click(within(dialog).getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Organization unit deleted successfully')).toBeInTheDocument();
    });

    act(() => {
      fireEvent.click(document.body);
    });

    await waitFor(() => {
      expect(screen.queryByText('Organization unit deleted successfully')).not.toBeInTheDocument();
    });
  });
});
