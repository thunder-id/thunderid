// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor, renderWithProviders, renderHook} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
import type {OrganizationUnitListResponse} from '../../models/responses';
import OrganizationUnitsListPage from '../OrganizationUnitsListPage';

// Mock navigate
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock logger — stable reference to avoid useCallback churn
const stableLogger = {error: vi.fn(), info: vi.fn(), debug: vi.fn()};
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => stableLogger,
}));

// Mock the API hook
const mockOUData: OrganizationUnitListResponse = {
  totalResults: 2,
  startIndex: 1,
  count: 2,
  organizationUnits: [
    {id: 'ou-1', handle: 'root', name: 'Root Organization', description: 'Root OU', parent: null},
    {id: 'ou-2', handle: 'child', name: 'Child Organization', description: null, parent: 'ou-1'},
  ],
};

vi.mock('@/api/useGetOrganizationUnits', () => ({
  default: () => ({
    data: mockOUData,
    isLoading: false,
    error: null,
  }),
}));

// Mock delete hook
vi.mock('@/api/useDeleteOrganizationUnit', () => ({
  default: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

// Mock ThunderID — stable reference to avoid useCallback churn when tree view renders
const stableHttp = {request: vi.fn()};
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useThunderID: () => ({http: stableHttp}),
  };
});

// Mock useOrganizationUnit hook with React state for reactivity
vi.mock('@/contexts/useOrganizationUnit', async () => {
  const {useState, useCallback} = await import('react');
  type OrganizationUnitTreeItem = import('../../models/organization-unit-tree').OrganizationUnitTreeItem;
  function useOrganizationUnit() {
    const [treeItems, setTreeItems] = useState<OrganizationUnitTreeItem[]>([]);
    const [expandedItems, setExpandedItems] = useState<string[]>([]);
    const [loadedItems, setLoadedItems] = useState<Set<string>>(new Set());
    const resetTreeState = useCallback(() => {
      setTreeItems([]);
      setLoadedItems(new Set());
    }, []);
    return {treeItems, setTreeItems, expandedItems, setExpandedItems, loadedItems, setLoadedItems, resetTreeState};
  }
  return {default: useOrganizationUnit};
});

// Mock config (for tree view)
const stableConfig = {
  getServerUrl: () => 'http://localhost:8080',
  getDocumentationLink: () => undefined,
  config: {brand: {product_name: 'ThunderID'}},
};
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => stableConfig,
  };
});

describe('OrganizationUnitsListPage', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
  });

  it('should render page title', () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    expect(screen.getByText(t('organizationUnits:listing.title'))).toBeInTheDocument();
  });

  it('should set the document title', async () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    await waitFor(() => expect(document.title).toBe('Organization units (OU) | ThunderID'));
  });

  it('should render page subtitle', () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    expect(screen.getByText(t('organizationUnits:listing.subtitle'))).toBeInTheDocument();
  });

  it('should render tree view with organization units', async () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
      expect(screen.getByText('Child Organization')).toBeInTheDocument();
    });
  });

  it('should render add root organization unit button in tree view', async () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    await waitFor(() => {
      expect(screen.getByText('Root Organization')).toBeInTheDocument();
    });

    expect(screen.getByText(t('organizationUnits:listing.addRootOrganizationUnit'))).toBeInTheDocument();
  });
});
