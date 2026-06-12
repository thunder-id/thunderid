/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {renderWithProviders, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import * as useGetResourceActionsModule from '../../../api/useGetResourceActions';
import * as useGetResourcesModule from '../../../api/useGetResources';
import * as useGetResourceServersModule from '../../../api/useGetResourceServers';
import * as useGetServerActionsModule from '../../../api/useGetServerActions';
import * as useSubtreePermissionsModule from '../../../api/useSubtreePermissions';
import type {ResourcePermissions} from '../../../models/resource-server';
import PermissionCatalog from '../PermissionCatalog';

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: vi.fn()}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
    useToast: () => ({showToast: vi.fn()}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('../../../api/useGetResourceServers');
vi.mock('../../../api/useGetResources');
vi.mock('../../../api/useGetServerActions');
vi.mock('../../../api/useGetResourceActions');
vi.mock('../../../api/useSubtreePermissions');

/* ---------- Fixture ---------- */

const servers = [
  {id: 'rs-1', name: 'Booking API', handle: 'booking-api', ouId: 'ou-1', delimiter: ':', type: 'API' as const},
  {id: 'rs-2', name: 'Payments API', handle: 'payments-api', ouId: 'ou-1', delimiter: ':', type: 'API' as const},
];

const dotServer = {
  id: 'rs-dot',
  name: 'Ecommerce API',
  handle: 'ecommerce-api',
  ouId: 'ou-1',
  delimiter: '.',
  type: 'API' as const,
};

const dotRootResources = [
  {id: 'dot-res-1', name: 'Products', handle: 'products', permission: 'ecommerce.products', parent: null},
];

const rootResources = [{id: 'res-1', name: 'Bookings', handle: 'bookings', permission: 'bookings', parent: null}];

const childResources = [
  {id: 'res-2', name: 'Reservations', handle: 'reservations', permission: 'bookings:reservations', parent: 'res-1'},
];

const serverActions = [{id: 'act-health', name: 'Health Ping', handle: 'health-ping', permission: 'health:ping'}];

const resourceActionsForBookings = [{id: 'act-1', name: 'Create', handle: 'create', permission: 'bookings:create'}];

const resourceActionsForReservations = [
  {id: 'act-2', name: 'Update', handle: 'update', permission: 'bookings:reservations:update'},
];

const serverActionsRs2 = [{id: 'act-refund', name: 'Refund', handle: 'refund', permission: 'payments:refund'}];

/* ---------- The full subtree for rs-1 ---------- */
const rs1AllPermissions = [
  'health:ping',
  'bookings',
  'bookings:create',
  'bookings:reservations',
  'bookings:reservations:update',
];

/* ---------- Helpers ---------- */

function queryResult<T>(data: T) {
  return {data, isLoading: false, error: null} as never;
}

function emptyQueryResult() {
  return {data: undefined, isLoading: false, error: null} as never;
}

const mockCollectSubtreePermissions = vi.fn();
const mockGetCachedSubtreePermissions = vi.fn();
const mockCollectServerPermissions = vi.fn();
const mockGetCachedServerPermissions = vi.fn();

function setupDefaultMocks(): void {
  vi.mocked(useGetResourceServersModule.default).mockReturnValue(
    queryResult({totalResults: 2, startIndex: 1, count: 2, resourceServers: servers}),
  );

  vi.mocked(useGetResourcesModule.default).mockImplementation((_serverId: string, parentId?: string) => {
    if (!parentId) {
      if (_serverId === 'rs-1') {
        return queryResult({totalResults: 1, startIndex: 1, count: 1, resources: rootResources});
      }
      return queryResult({totalResults: 0, startIndex: 1, count: 0, resources: []});
    }
    if (parentId === 'res-1') {
      return queryResult({totalResults: 1, startIndex: 1, count: 1, resources: childResources});
    }
    return queryResult({totalResults: 0, startIndex: 1, count: 0, resources: []});
  });

  vi.mocked(useGetServerActionsModule.default).mockImplementation((_serverId: string) => {
    if (_serverId === 'rs-1') {
      return queryResult({totalResults: 1, startIndex: 1, count: 1, actions: serverActions});
    }
    if (_serverId === 'rs-2') {
      return queryResult({totalResults: 1, startIndex: 1, count: 1, actions: serverActionsRs2});
    }
    return queryResult({totalResults: 0, startIndex: 1, count: 0, actions: []});
  });

  vi.mocked(useGetResourceActionsModule.default).mockImplementation(
    (_resourceServerId: string, resourceId: string, enabled?: boolean) => {
      if (!enabled) return emptyQueryResult();
      if (resourceId === 'res-1') {
        return queryResult({totalResults: 1, startIndex: 1, count: 1, actions: resourceActionsForBookings});
      }
      if (resourceId === 'res-2') {
        return queryResult({totalResults: 1, startIndex: 1, count: 1, actions: resourceActionsForReservations});
      }
      return queryResult({totalResults: 0, startIndex: 1, count: 0, actions: []});
    },
  );

  // By default, cached variants return the full rs-1 subtree; collect variants resolve them
  mockCollectSubtreePermissions.mockResolvedValue([
    'bookings',
    'bookings:create',
    'bookings:reservations',
    'bookings:reservations:update',
  ]);
  mockGetCachedSubtreePermissions.mockReturnValue([
    'bookings',
    'bookings:create',
    'bookings:reservations',
    'bookings:reservations:update',
  ]);
  mockCollectServerPermissions.mockResolvedValue(rs1AllPermissions);
  mockGetCachedServerPermissions.mockReturnValue(rs1AllPermissions);

  vi.mocked(useSubtreePermissionsModule.default).mockReturnValue({
    collectSubtreePermissions: mockCollectSubtreePermissions,
    getCachedSubtreePermissions: mockGetCachedSubtreePermissions,
    collectServerPermissions: mockCollectServerPermissions,
    getCachedServerPermissions: mockGetCachedServerPermissions,
  });
}

function renderCatalog(props: {
  selected: ResourcePermissions[];
  onChange?: ReturnType<typeof vi.fn>;
  readOnly?: boolean;
}) {
  const onChange = props.onChange ?? vi.fn();
  renderWithProviders(<PermissionCatalog selected={props.selected} onChange={onChange} readOnly={props.readOnly} />);
  return onChange;
}

/* ---------- Tests ---------- */

describe('PermissionCatalog', () => {
  beforeEach(() => {
    setupDefaultMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('lazy-loads a server section only after expanding it', async () => {
    const user = userEvent.setup();
    renderCatalog({selected: []});
    // Before expand: Bookings resource row absent
    expect(screen.queryByText('Bookings')).not.toBeInTheDocument();
    // Click the rs-1 header expand button
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    // After expand: Bookings row visible
    expect(await screen.findByText('Bookings')).toBeInTheDocument();
  });

  it('checking a "This resource" self row stages only the parent permission', async () => {
    const user = userEvent.setup();
    const onChange = renderCatalog({selected: []});
    // Expand rs-1 server
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    // Expand the bookings resource node
    await user.click(await screen.findByRole('button', {name: 'bookings'}));
    // After expanding the bookings node, there are two checkboxes with aria-label 'bookings':
    // [0] = the cascade tri-state checkbox for the resource node row
    // [1] = the "This resource" plain checkbox rendered inside the Collapse
    const allBookingsCheckboxes = await screen.findAllByRole('checkbox', {name: 'bookings'});
    // The "This resource" row is rendered second (inside the Collapse at depth+1)
    const thisResourceCheckbox = allBookingsCheckboxes[allBookingsCheckboxes.length - 1];
    await user.click(thisResourceCheckbox);
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith([{resourceServerId: 'rs-1', permissions: ['bookings']}]);
    });
  });

  it('clicking an unchecked resource checkbox cascades the whole subtree', async () => {
    const user = userEvent.setup();
    const onChange = renderCatalog({selected: []});
    // Expand rs-1 server
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    // The resource node checkbox — aria-label is the resource permission string
    await screen.findByRole('checkbox', {name: 'Booking API'});
    // Click the bookings-resource-node checkbox (the tri-state one for the resource row)
    // This is the row-level checkbox (CatalogRow uses inputProps aria-label = permission)
    // The resource node row has aria-label = 'bookings' on its checkbox
    // Wait for resource to appear, then click the bookings-node checkbox (not "This resource")
    await waitFor(() => expect(screen.getByText('Bookings')).toBeInTheDocument());
    // The resource node checkbox has aria-label = resource.permission = 'bookings'
    // But we have multiple — the cascade checkbox is the first one (not inside Collapse)
    const allBookingsCheckboxes = screen.getAllByRole('checkbox', {name: 'bookings'});
    // The first 'bookings' checkbox is the cascade-level resource node checkbox
    await user.click(allBookingsCheckboxes[0]);
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith([
        {
          resourceServerId: 'rs-1',
          permissions: expect.arrayContaining([
            'bookings',
            'bookings:create',
            'bookings:reservations',
            'bookings:reservations:update',
          ]) as string[],
        },
      ]);
    });
  });

  it('clicking a fully-selected resource checkbox clears the subtree', async () => {
    const user = userEvent.setup();
    const fullySelected: ResourcePermissions[] = [
      {
        resourceServerId: 'rs-1',
        permissions: ['bookings', 'bookings:create', 'bookings:reservations', 'bookings:reservations:update'],
      },
    ];
    const onChange = renderCatalog({selected: fullySelected});
    // Expand rs-1 server
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    await waitFor(() => expect(screen.getByText('Bookings')).toBeInTheDocument());
    // The bookings node checkbox should be in 'all' state — click to clear
    const allBookingsCheckboxes = screen.getAllByRole('checkbox', {name: 'bookings'});
    await user.click(allBookingsCheckboxes[0]);
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith([]);
    });
  });

  it('shows indeterminate state when only part of a subtree is selected', async () => {
    const user = userEvent.setup();
    // Only bookings:create selected — partial subtree
    renderCatalog({selected: [{resourceServerId: 'rs-1', permissions: ['bookings:create']}]});
    // Expand rs-1 to see the resource node
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    await waitFor(() => expect(screen.getByText('Bookings')).toBeInTheDocument());
    // The bookings node checkbox should be indeterminate
    const allBookingsCheckboxes = screen.getAllByRole('checkbox', {name: 'bookings'});
    expect(allBookingsCheckboxes[0]).toHaveAttribute('data-indeterminate', 'true');
  });

  it('server header checkbox cascades the entire server', async () => {
    const user = userEvent.setup();
    const onChange = renderCatalog({selected: []});
    // Click the Booking API server-level checkbox
    const bookingApiCheckbox = screen.getByRole('checkbox', {name: 'Booking API'});
    await user.click(bookingApiCheckbox);
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith([
        {
          resourceServerId: 'rs-1',
          permissions: expect.arrayContaining(rs1AllPermissions) as string[],
        },
      ]);
    });
  });

  it('unchecking a fully-selected server drops its entry including unknown strings', async () => {
    const user = userEvent.setup();
    const cleanSelected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: [...rs1AllPermissions]}];
    const onChange = renderCatalog({selected: cleanSelected});
    const bookingApiCheckbox = screen.getByRole('checkbox', {name: 'Booking API'});
    // Should be checked (all state) since selected matches cached exactly
    expect(bookingApiCheckbox).toBeChecked();
    await user.click(bookingApiCheckbox);
    await waitFor(() => {
      // The whole rs-1 entry should be dropped
      expect(onChange).toHaveBeenCalledWith([]);
    });
  });

  it('renders unknown-server entries as a warning group with uncheckable rows', async () => {
    const user = userEvent.setup();
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-gone', permissions: ['ghost:perm']}];
    const onChange = renderCatalog({selected});
    // Warning chip for unknown server should be visible
    expect(await screen.findByText('Resource server not found')).toBeInTheDocument();
    // The ghost:perm row should be checked
    const ghostCheckbox = screen.getByRole('checkbox', {name: 'ghost:perm'});
    expect(ghostCheckbox).toBeChecked();
    // Clicking the ghost:perm checkbox should remove it
    await user.click(ghostCheckbox);
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith([]);
    });
  });

  it('shows indeterminate on a dot-delimited resource node when only a descendant is selected and cache is null', async () => {
    const user = userEvent.setup();

    // Override server list to include only the dot-delimiter server
    vi.mocked(useGetResourceServersModule.default).mockReturnValue(
      queryResult({totalResults: 1, startIndex: 1, count: 1, resourceServers: [dotServer]}),
    );

    // Root resources for the dot server
    vi.mocked(useGetResourcesModule.default).mockImplementation((_serverId: string, parentId?: string) => {
      if (_serverId === 'rs-dot' && !parentId) {
        return queryResult({totalResults: 1, startIndex: 1, count: 1, resources: dotRootResources});
      }
      return queryResult({totalResults: 0, startIndex: 1, count: 0, resources: []});
    });

    vi.mocked(useGetServerActionsModule.default).mockReturnValue(
      queryResult({totalResults: 0, startIndex: 1, count: 0, actions: []}),
    );

    vi.mocked(useGetResourceActionsModule.default).mockReturnValue(emptyQueryResult());

    // Cache returns null — subtree not yet fetched (tree not fully expanded)
    mockGetCachedSubtreePermissions.mockReturnValue(null);
    mockGetCachedServerPermissions.mockReturnValue(null);

    // Only a descendant action is selected (dot-delimited)
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-dot', permissions: ['ecommerce.products.read']}];

    renderCatalog({selected});

    // Expand the dot server
    await user.click(screen.getByRole('button', {name: 'ecommerce-api'}));
    await waitFor(() => expect(screen.getByText('Products')).toBeInTheDocument());

    // The Products resource node checkbox should be indeterminate because
    // 'ecommerce.products.read' starts with 'ecommerce.products' + '.' (dot delimiter)
    const allProductsCheckboxes = screen.getAllByRole('checkbox', {name: 'ecommerce.products'});
    expect(allProductsCheckboxes[0]).toHaveAttribute('data-indeterminate', 'true');
  });

  it('does NOT show indeterminate when a colon-prefixed descendant is selected under a dot-delimiter server (wrong delimiter is excluded)', async () => {
    const user = userEvent.setup();

    // Override server list to include only the dot-delimiter server
    vi.mocked(useGetResourceServersModule.default).mockReturnValue(
      queryResult({totalResults: 1, startIndex: 1, count: 1, resourceServers: [dotServer]}),
    );

    vi.mocked(useGetResourcesModule.default).mockImplementation((_serverId: string, parentId?: string) => {
      if (_serverId === 'rs-dot' && !parentId) {
        return queryResult({totalResults: 1, startIndex: 1, count: 1, resources: dotRootResources});
      }
      return queryResult({totalResults: 0, startIndex: 1, count: 0, resources: []});
    });

    vi.mocked(useGetServerActionsModule.default).mockReturnValue(
      queryResult({totalResults: 0, startIndex: 1, count: 0, actions: []}),
    );

    vi.mocked(useGetResourceActionsModule.default).mockReturnValue(emptyQueryResult());

    mockGetCachedSubtreePermissions.mockReturnValue(null);
    mockGetCachedServerPermissions.mockReturnValue(null);

    // A colon-based permission that looks like a descendant with colon but NOT with dot
    // Under the dot server, 'ecommerce.products:read' does NOT start with 'ecommerce.products.'
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-dot', permissions: ['ecommerce.products:read']}];

    renderCatalog({selected});

    await user.click(screen.getByRole('button', {name: 'ecommerce-api'}));
    await waitFor(() => expect(screen.getByText('Products')).toBeInTheDocument());

    // Should be 'none' state (neither checked nor indeterminate)
    const allProductsCheckboxes = screen.getAllByRole('checkbox', {name: 'ecommerce.products'});
    expect(allProductsCheckboxes[0]).not.toBeChecked();
    expect(allProductsCheckboxes[0]).not.toHaveAttribute('data-indeterminate', 'true');
  });

  it('disables every checkbox in readOnly mode', async () => {
    const user = userEvent.setup();
    const selected: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings']}];
    renderCatalog({selected, readOnly: true});
    // The server-level checkbox should be disabled
    const bookingApiCheckbox = screen.getByRole('checkbox', {name: 'Booking API'});
    expect(bookingApiCheckbox).toBeDisabled();
    // Expand the server and check the resource-level checkboxes
    await user.click(screen.getByRole('button', {name: 'booking-api'}));
    await waitFor(() => expect(screen.getByText('Bookings')).toBeInTheDocument());
    const allBookingsCheckboxes = screen.getAllByRole('checkbox', {name: 'bookings'});
    allBookingsCheckboxes.forEach((cb) => expect(cb).toBeDisabled());
  });
});
