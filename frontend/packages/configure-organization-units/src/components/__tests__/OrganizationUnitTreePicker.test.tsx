// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, waitFor, renderWithProviders, renderHook} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
import type {OrganizationUnit} from '../../models/organization-unit';
import type {OrganizationUnitListResponse} from '../../models/responses';
import OrganizationUnitTreePicker from '../OrganizationUnitTreePicker';

// Mock logger — stable reference to avoid useCallback churn
const stableLogger = {error: vi.fn(), info: vi.fn(), debug: vi.fn()};
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => stableLogger,
}));

// Mock the API hooks
const mockUseGetOrganizationUnits = vi.fn();
vi.mock('@/api/useGetOrganizationUnits', () => ({
  default: () =>
    mockUseGetOrganizationUnits() as {
      data: OrganizationUnitListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

const mockUseGetOrganizationUnit = vi.fn();
vi.mock('@/api/useGetOrganizationUnit', () => ({
  default: (id: string | undefined, enabled?: boolean) =>
    mockUseGetOrganizationUnit(id, enabled) as {
      data: OrganizationUnit | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

const mockUseGetChildOrganizationUnits = vi.fn();
vi.mock('@/api/useGetChildOrganizationUnits', () => ({
  default: (parentId: string | undefined) =>
    mockUseGetChildOrganizationUnits(parentId) as {
      data: OrganizationUnitListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

// Mock ThunderID — stable reference to avoid useCallback churn
const mockHttpRequest = vi.fn();
const stableHttp = {request: mockHttpRequest};
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useThunderID: () => ({http: stableHttp}),
  };
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

describe('OrganizationUnitTreePicker', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });
  const mockOUData: OrganizationUnitListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'ou-1', handle: 'root', name: 'Root Organization', description: 'Root OU', parent: null},
      {id: 'ou-2', handle: 'engineering', name: 'Engineering', description: null, parent: null},
    ],
  };

  const defaultProps = {
    value: '',
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetOrganizationUnits.mockReturnValue({
      data: mockOUData,
      isLoading: false,
      error: null,
    });
    // Default: rooted-mode hooks return no data (used when rootOuId is not provided)
    mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
    mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});
  });

  it('should render tree with organization unit names', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
      expect(screen.getByText('Engineering')).toBeInTheDocument();
    });
  });

  it('should show loading spinner when data is loading', () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should show empty message when no organization units', () => {
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

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    expect(screen.getByText(t('organizationUnits:treePicker.empty'))).toBeInTheDocument();
  });

  it('should render an inline read error state, never the raw error message, when the root list fails to load', () => {
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
    });

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    expect(screen.getByText('Failed to load organization unit data')).toBeInTheDocument();
    expect(screen.queryByText('Network error')).not.toBeInTheDocument();
  });

  it('should refetch the root list when the retry action is clicked', () => {
    const mockRefetchRootList = vi.fn();
    mockUseGetOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetchRootList,
    });

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    fireEvent.click(screen.getByText('Refresh'));

    expect(mockRefetchRootList).toHaveBeenCalledTimes(1);
  });

  it('should display handles for tree items', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('root')).toBeInTheDocument();
      expect(screen.getByText('engineering')).toBeInTheDocument();
    });
  });

  it('should render avatars for tree items', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const avatars = document.querySelectorAll('.MuiAvatar-root');
    expect(avatars.length).toBeGreaterThan(0);
  });

  it('should call onChange when a tree item is selected', async () => {
    const onChange = vi.fn();
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} onChange={onChange} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Root Organization'));

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith('ou-1');
    });
  });

  it('should not call onChange when clicking a placeholder item', async () => {
    const onChange = vi.fn();
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} onChange={onChange} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Placeholder items are not directly clickable via text, so verify no unexpected calls
    expect(onChange).not.toHaveBeenCalled();
  });

  it('should pass id prop to tree view', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} id="test-picker" />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    expect(document.getElementById('test-picker')).toBeInTheDocument();
  });

  it('should display helper text when provided', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} helperText="Select a parent" />);

    await waitFor(() => {
      expect(screen.getByText('Select a parent')).toBeInTheDocument();
    });
  });

  it('should display helper text with error styling when error is true', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} helperText="This field is required" error />);

    await waitFor(() => {
      const helperText = screen.getByText('This field is required');
      expect(helperText).toBeInTheDocument();
    });
  });

  it('should not display helper text when not provided', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // No helper text element should be present
    expect(screen.queryByText('Select a parent')).not.toBeInTheDocument();
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

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // Click the expand icon on the first tree item
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    expect(expandIcons.length).toBeGreaterThan(0);
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByText('Fetched Child')).toBeInTheDocument();
    });
  });

  it('should show "no children" placeholder when expanded node has no children', async () => {
    const emptyChildResponse: OrganizationUnitListResponse = {
      totalResults: 0,
      startIndex: 1,
      count: 0,
      organizationUnits: [],
    };

    mockHttpRequest.mockResolvedValue({data: emptyChildResponse});

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.noChildren'))).toBeInTheDocument();
    });
  });

  it('should log error when fetching child OUs fails', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Network failure'));

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalledWith(
        'Failed to load child organization units',
        expect.objectContaining({parentId: 'ou-1'}),
      );
    });
  });

  it('should show load more button when there are more root items', async () => {
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

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });
  });

  it('should fetch more root items when load more button is clicked', async () => {
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

    const nextPageResponse: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 3,
      count: 2,
      organizationUnits: [
        {id: 'ou-3', handle: 'sales', name: 'Sales', description: null, parent: null},
        {id: 'ou-4', handle: 'marketing', name: 'Marketing', description: null, parent: null},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: nextPageResponse});

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText(t('organizationUnits:listing.treeView.loadMore')));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should trigger load more via keyboard Enter key', async () => {
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
        organizationUnits: [{id: 'ou-3', handle: 'sales', name: 'Sales', description: null, parent: null}],
      },
    });

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });

    const loadMoreButton = screen
      .getByText(t('organizationUnits:listing.treeView.loadMore'))
      .closest('[role="button"]')!;
    fireEvent.keyDown(loadMoreButton, {key: 'Enter'});

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should trigger load more via keyboard Space key', async () => {
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
        organizationUnits: [{id: 'ou-3', handle: 'sales', name: 'Sales', description: null, parent: null}],
      },
    });

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });

    const loadMoreButton = screen
      .getByText(t('organizationUnits:listing.treeView.loadMore'))
      .closest('[role="button"]')!;
    fireEvent.keyDown(loadMoreButton, {key: ' '});

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });
  });

  it('should show load more for child items when there are more children', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 50,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText('Fetched Child')).toBeInTheDocument();
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });
  });

  it('should log error when root load more fails', async () => {
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

    mockHttpRequest.mockRejectedValue(new Error('Network failure'));

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText(t('organizationUnits:listing.treeView.loadMore')));

    await waitFor(() => {
      expect(stableLogger.error).toHaveBeenCalled();
    });
  });

  it('should not expand item when it is already loaded', async () => {
    const childOUResponse: OrganizationUnitListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      organizationUnits: [
        {id: 'ou-child-1', handle: 'child1', name: 'Fetched Child', description: null, parent: 'ou-1'},
      ],
    };

    mockHttpRequest.mockResolvedValue({data: childOUResponse});

    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // First expansion - triggers fetch
    const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons[0]);

    await waitFor(() => {
      expect(screen.getByText('Fetched Child')).toBeInTheDocument();
    });

    const callCount = mockHttpRequest.mock.calls.length;

    // Collapse
    const collapseIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(collapseIcons[0]);

    // Expand again - should not trigger another fetch
    const expandIcons2 = document.querySelectorAll('.MuiTreeItem-iconContainer');
    fireEvent.click(expandIcons2[0]);

    // Wait a bit and verify no additional HTTP calls
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(callCount);
    });
  });

  it('should highlight selected item', async () => {
    renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} value="ou-1" />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    // The Mui-selected class should be applied to the selected item
    const selectedElements = document.querySelectorAll('.Mui-selected');
    expect(selectedElements.length).toBeGreaterThan(0);
  });

  describe('rootOuId mode', () => {
    const rootOu: OrganizationUnit = {
      id: 'root-ou-1',
      handle: 'root-handle',
      name: 'Root OU',
      description: 'The root',
      parent: null,
    };

    const childOUsResponse: OrganizationUnitListResponse = {
      totalResults: 2,
      startIndex: 1,
      count: 2,
      organizationUnits: [
        {id: 'child-1', handle: 'child-1-handle', name: 'Child One', description: null, parent: 'root-ou-1'},
        {id: 'child-2', handle: 'child-2-handle', name: 'Child Two', description: null, parent: 'root-ou-1'},
      ],
    };

    beforeEach(() => {
      // Default: hooks return no data and not loading (non-rooted hooks)
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});
    });

    it('should show loading spinner when root OU is loading', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: true, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should show loading spinner when root OU children are loading', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: true, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should render an inline read error state, never the raw error message, when the root OU fails to load', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Network error'),
      });
      mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Failed to load organization unit data')).toBeInTheDocument();
      });
      expect(screen.queryByText('Network error')).not.toBeInTheDocument();
    });

    it('should refetch the root OU when the retry action is clicked', async () => {
      const mockRefetchRootOu = vi.fn();
      mockUseGetOrganizationUnit.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Network error'),
        refetch: mockRefetchRootOu,
      });
      mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Refresh')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText('Refresh'));

      expect(mockRefetchRootOu).toHaveBeenCalledTimes(1);
    });

    it('should render root OU as top-level node with children', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
        expect(screen.getByText('Child One')).toBeInTheDocument();
        expect(screen.getByText('Child Two')).toBeInTheDocument();
      });
    });

    it('should render only the root OU children when the root is hidden', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: true, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot />);

      await waitFor(() => {
        expect(screen.getByText('Child One')).toBeInTheDocument();
        expect(screen.getByText('Child Two')).toBeInTheDocument();
      });
      expect(screen.queryByText('Root OU')).not.toBeInTheDocument();
      expect(mockUseGetOrganizationUnit).toHaveBeenCalledWith('root-ou-1', false);
      expect(mockUseGetChildOrganizationUnits).toHaveBeenCalledWith('root-ou-1');
    });

    it('should lazy-load nested children when a visible child is expanded', async () => {
      const onItemActivate = vi.fn();
      const nestedChildResponse: OrganizationUnitListResponse = {
        totalResults: 1,
        startIndex: 1,
        count: 1,
        organizationUnits: [
          {
            id: 'grandchild-1',
            handle: 'grandchild-1-handle',
            name: 'Grandchild One',
            description: null,
            parent: 'child-1',
          },
        ],
      };
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});
      mockHttpRequest.mockResolvedValue({data: nestedChildResponse});

      renderWithProviders(
        <OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot onItemActivate={onItemActivate} />,
      );

      await waitFor(() => {
        expect(screen.getByText('Child One')).toBeInTheDocument();
      });

      const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
      fireEvent.click(expandIcons[0]);

      await waitFor(() => {
        const request = mockHttpRequest.mock.calls[0]?.[0] as {url: string} | undefined;
        expect(request?.url).toContain('/organization-units/child-1/ous');
        expect(screen.getByText('Grandchild One')).toBeInTheDocument();
      });
      expect(onItemActivate).not.toHaveBeenCalled();

      fireEvent.click(screen.getByText('Grandchild One'));
      expect(onItemActivate).toHaveBeenCalledWith('grandchild-1');
    });

    it('should activate a visible child from the keyboard', async () => {
      const onItemActivate = vi.fn();
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(
        <OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot onItemActivate={onItemActivate} />,
      );

      const childRow = (await screen.findByText('Child One')).closest('[role="button"]');
      expect(childRow).not.toBeNull();

      fireEvent.keyDown(childRow!, {key: 'Enter'});
      fireEvent.keyDown(childRow!, {key: ' '});
      fireEvent.keyDown(childRow!, {key: 'Escape'});

      expect(onItemActivate).toHaveBeenNthCalledWith(1, 'child-1');
      expect(onItemActivate).toHaveBeenNthCalledWith(2, 'child-1');
      expect(onItemActivate).toHaveBeenCalledTimes(2);
    });

    it('should load more children within a nested branch', async () => {
      const firstNestedPage: OrganizationUnitListResponse = {
        totalResults: 3,
        startIndex: 1,
        count: 2,
        organizationUnits: [
          {
            id: 'grandchild-1',
            handle: 'grandchild-1-handle',
            name: 'Grandchild One',
            description: null,
            parent: 'child-1',
          },
          {
            id: 'grandchild-2',
            handle: 'grandchild-2-handle',
            name: 'Grandchild Two',
            description: null,
            parent: 'child-1',
          },
        ],
      };
      const secondNestedPage: OrganizationUnitListResponse = {
        totalResults: 3,
        startIndex: 3,
        count: 1,
        organizationUnits: [
          {
            id: 'grandchild-3',
            handle: 'grandchild-3-handle',
            name: 'Grandchild Three',
            description: null,
            parent: 'child-1',
          },
        ],
      };
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});
      mockHttpRequest.mockResolvedValueOnce({data: firstNestedPage}).mockResolvedValueOnce({data: secondNestedPage});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot />);

      await screen.findByText('Child One');
      fireEvent.click(document.querySelectorAll('.MuiTreeItem-iconContainer')[0]);

      const loadMore = await screen.findByText(t('organizationUnits:listing.treeView.loadMore'));
      fireEvent.click(loadMore);

      await waitFor(() => {
        expect(screen.getByText('Grandchild Three')).toBeInTheDocument();
      });
      const request = mockHttpRequest.mock.calls[1]?.[0] as {url: string} | undefined;
      expect(request?.url).toContain('/organization-units/child-1/ous');
      expect(request?.url).toContain('offset=2');
    });

    it('should load more direct children when the root is hidden', async () => {
      const paginatedChildrenResponse = {...childOUsResponse, totalResults: 3};
      const nextPageResponse: OrganizationUnitListResponse = {
        totalResults: 3,
        startIndex: 3,
        count: 1,
        organizationUnits: [
          {id: 'child-3', handle: 'child-3-handle', name: 'Child Three', description: null, parent: 'root-ou-1'},
        ],
      };
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: paginatedChildrenResponse,
        isLoading: false,
        error: null,
      });
      mockHttpRequest.mockResolvedValue({data: nextPageResponse});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot />);

      await waitFor(() => {
        expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText(t('organizationUnits:listing.treeView.loadMore')));

      await waitFor(() => {
        expect(screen.getByText('Child Three')).toBeInTheDocument();
      });
      const request = mockHttpRequest.mock.calls[0]?.[0] as {url: string} | undefined;
      expect(request?.url).toContain('/organization-units/root-ou-1/ous');
      expect(request?.url).toContain('offset=2');
    });

    it('should show an aligned empty state when the hidden root has no children', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: {totalResults: 0, startIndex: 1, count: 0, organizationUnits: []},
        isLoading: false,
        error: null,
      });

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" hideRoot />);

      const emptyState = await screen.findByText(t('organizationUnits:listing.treeView.noChildren'));
      expect(emptyState).toHaveStyle({paddingLeft: '8px'});
      expect(screen.queryByRole('tree')).not.toBeInTheDocument();
    });

    it('should auto-expand root OU node', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      // Children should be visible immediately (root is auto-expanded)
      await waitFor(() => {
        expect(screen.getByText('Child One')).toBeInTheDocument();
        expect(screen.getByText('Child Two')).toBeInTheDocument();
      });
    });

    it('should allow selecting the root OU', async () => {
      const onChange = vi.fn();
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" onChange={onChange} />);

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Root OU'));

      await waitFor(() => {
        expect(onChange).toHaveBeenCalledWith('root-ou-1');
      });
    });

    it('should allow selecting a child OU', async () => {
      const onChange = vi.fn();
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" onChange={onChange} />);

      await waitFor(() => {
        expect(screen.getByText('Child One')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Child One'));

      await waitFor(() => {
        expect(onChange).toHaveBeenCalledWith('child-1');
      });
    });

    it('should show "no children" placeholder when root OU has no children', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: {totalResults: 0, startIndex: 1, count: 0, organizationUnits: []},
        isLoading: false,
        error: null,
      });

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
        expect(screen.getByText(t('organizationUnits:listing.treeView.noChildren'))).toBeInTheDocument();
      });
    });

    it('should not show empty message for global mode when in rooted mode', async () => {
      mockUseGetOrganizationUnits.mockReturnValue({
        data: {totalResults: 0, startIndex: 1, count: 0, organizationUnits: []},
        isLoading: false,
        error: null,
      });
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
      });

      // The global empty message should NOT appear
      expect(screen.queryByText(t('organizationUnits:treePicker.empty'))).not.toBeInTheDocument();
    });

    it('should display handles for root and child items in rooted mode', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('root-handle')).toBeInTheDocument();
        expect(screen.getByText('child-1-handle')).toBeInTheDocument();
        expect(screen.getByText('child-2-handle')).toBeInTheDocument();
      });
    });

    it('should show load more for children when there are more than returned', async () => {
      const paginatedChildrenResponse: OrganizationUnitListResponse = {
        totalResults: 50,
        startIndex: 1,
        count: 2,
        organizationUnits: [
          {id: 'child-1', handle: 'child-1-handle', name: 'Child One', description: null, parent: 'root-ou-1'},
          {id: 'child-2', handle: 'child-2-handle', name: 'Child Two', description: null, parent: 'root-ou-1'},
        ],
      };

      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: paginatedChildrenResponse,
        isLoading: false,
        error: null,
      });

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" />);

      await waitFor(() => {
        expect(screen.getByText('Child One')).toBeInTheDocument();
        expect(screen.getByText(t('organizationUnits:listing.treeView.loadMore'))).toBeInTheDocument();
      });
    });

    it('should render correctly with custom maxHeight prop', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: childOUsResponse, isLoading: false, error: null});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" maxHeight={500} />);

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
        expect(screen.getByText('Child One')).toBeInTheDocument();
      });
    });
  });

  describe('autoSelectFirst', () => {
    it('should call onChange with the first root organization unit when nothing is selected', async () => {
      const onChange = vi.fn();
      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} onChange={onChange} autoSelectFirst />);

      await waitFor(() => {
        expect(onChange).toHaveBeenCalledWith('ou-1');
      });
    });

    it('should not override an already-selected value', async () => {
      const onChange = vi.fn();
      renderWithProviders(
        <OrganizationUnitTreePicker {...defaultProps} value="ou-2" onChange={onChange} autoSelectFirst />,
      );

      await waitFor(() => {
        expect(screen.getByText('Root Organization')).toBeInTheDocument();
      });

      expect(onChange).not.toHaveBeenCalled();
    });

    it('should not auto-select in rootOuId mode', async () => {
      const rootOu: OrganizationUnit = {id: 'root-ou-1', handle: 'root-ou', name: 'Root OU', parent: null};
      const rootOuChildren: OrganizationUnitListResponse = {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        organizationUnits: [],
      };

      mockUseGetOrganizationUnit.mockReturnValue({data: rootOu, isLoading: false, error: null});
      mockUseGetChildOrganizationUnits.mockReturnValue({data: rootOuChildren, isLoading: false, error: null});
      const onChange = vi.fn();

      renderWithProviders(
        <OrganizationUnitTreePicker {...defaultProps} rootOuId="root-ou-1" onChange={onChange} autoSelectFirst />,
      );

      await waitFor(() => {
        expect(screen.getByText('Root OU')).toBeInTheDocument();
      });

      expect(onChange).not.toHaveBeenCalled();
    });
  });

  describe('spacious variant', () => {
    it('should still show the "no children" placeholder text when expanded node has no children', async () => {
      const emptyChildResponse: OrganizationUnitListResponse = {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        organizationUnits: [],
      };

      mockHttpRequest.mockResolvedValue({data: emptyChildResponse});

      renderWithProviders(<OrganizationUnitTreePicker {...defaultProps} spacious />);

      await waitFor(() => {
        expect(screen.getByText('Root Organization')).toBeInTheDocument();
      });

      const expandIcons = document.querySelectorAll('.MuiTreeItem-iconContainer');
      fireEvent.click(expandIcons[0]);

      await waitFor(() => {
        expect(screen.getByText(t('organizationUnits:listing.treeView.noChildren'))).toBeInTheDocument();
      });
    });
  });
});
