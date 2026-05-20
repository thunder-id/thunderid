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

import {screen, waitFor, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
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
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: stableHttp}),
}));

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
const stableConfig = {getServerUrl: () => 'http://localhost:8080'};
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => stableConfig,
  };
});

// Mock translations — stable reference to avoid useCallback churn
const listTranslations: Record<string, string> = {
  'organizationUnits:listing.title': 'Organization Units',
  'organizationUnits:listing.subtitle': 'Manage your organization units',
  'organizationUnits:listing.addRootOrganizationUnit': 'Add Root Organization Unit',
  'organizationUnits:listing.error.title': 'Error loading organization units',
  'organizationUnits:listing.error.unknown': 'An unknown error occurred',
  'organizationUnits:listing.treeView.empty': 'No organization units found',
  'organizationUnits:listing.treeView.noChildren': 'No child organization units',
  'organizationUnits:listing.treeView.loadError': 'Failed to load child organization units',
  'organizationUnits:listing.treeView.addChild': 'Add child organization unit',
  'organizationUnits:listing.treeView.addChildOrganizationUnit': 'Add Child Organization Unit',
  'organizationUnits:delete.dialog.title': 'Delete Organization Unit',
  'organizationUnits:delete.dialog.message':
    'Are you sure you want to delete this organization unit? This action cannot be undone.',
  'organizationUnits:delete.dialog.disclaimer':
    'Warning: All associated data, configurations, and user assignments will be permanently removed.',
  'common:actions.edit': 'Edit',
  'common:actions.delete': 'Delete',
  'common:actions.cancel': 'Cancel',
};
const stableListT = (key: string): string => listTranslations[key] ?? key;
const stableListTranslation = {t: stableListT};
vi.mock('react-i18next', () => ({
  useTranslation: () => stableListTranslation,
}));

describe('OrganizationUnitsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReset();
  });

  it('should render page title', () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    expect(screen.getByText('Organization Units')).toBeInTheDocument();
  });

  it('should render page subtitle', () => {
    renderWithProviders(<OrganizationUnitsListPage />);

    expect(screen.getByText('Manage your organization units')).toBeInTheDocument();
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

    expect(screen.getByText('Add Root Organization Unit')).toBeInTheDocument();
  });
});
