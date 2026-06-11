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

import {renderWithProviders, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ResourceListResponse, ActionListResponse} from '../../../models/resource-server';
import ResourceTree from '../ResourceTree';

const mockResourceServer = {
  id: 'rs-1',
  name: 'Dark Dodos Smash',
  handle: 'dark-dodos',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API' as const,
};

const mockUseGetResources = vi.fn();
const mockUseGetServerActions = vi.fn();

vi.mock('../../../api/useGetResources', () => ({
  default: () =>
    mockUseGetResources() as {
      data: ResourceListResponse | undefined;
      isLoading: boolean;
    },
}));

vi.mock('../../../api/useGetServerActions', () => ({
  default: () =>
    mockUseGetServerActions() as {
      data: ActionListResponse | undefined;
      isLoading: boolean;
    },
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: vi.fn()}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('../AddNodeDialog', () => ({
  default: ({open, mode, onClose}: {open: boolean; mode: string; onClose: () => void}) =>
    open ? (
      <div role="dialog" aria-label="add-node-dialog">
        <span data-testid="dialog-mode">{mode}</span>
        <button onClick={onClose}>Close</button>
      </div>
    ) : null,
}));

vi.mock('../ResourceDetailPanel', () => ({
  default: () => <div data-testid="resource-detail-panel" />,
}));

vi.mock('../ResourceTreeNode', () => ({
  ResourceNode: ({node}: {node: {name: string}}) => <div data-testid="resource-node">{node.name}</div>,
  ActionNode: ({action}: {action: {name: string}}) => <div data-testid="action-node">{action.name}</div>,
}));

const emptyResources: ResourceListResponse = {
  totalResults: 0,
  startIndex: 0,
  count: 0,
  resources: [],
};

const emptyActions: ActionListResponse = {
  totalResults: 0,
  startIndex: 0,
  count: 0,
  actions: [],
};

const withResources: ResourceListResponse = {
  totalResults: 1,
  startIndex: 0,
  count: 1,
  resources: [{id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'}],
};

const withActions: ActionListResponse = {
  totalResults: 1,
  startIndex: 0,
  count: 1,
  actions: [{id: 'a-1', name: 'Read All', handle: 'read-all', permission: 'dark-dodos:read-all'}],
};

describe('ResourceTree', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetResources.mockReturnValue({data: emptyResources, isLoading: false});
    mockUseGetServerActions.mockReturnValue({data: emptyActions, isLoading: false});
  });

  it('does not render the resource server name as a root tree node', () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText('Dark Dodos Smash')).not.toBeInTheDocument();
  });

  it('does not render the server handle chip as a root tree node', () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText('dark-dodos')).not.toBeInTheDocument();
  });

  it('shows the loading spinner while data is loading', () => {
    mockUseGetResources.mockReturnValue({data: undefined, isLoading: true});
    mockUseGetServerActions.mockReturnValue({data: undefined, isLoading: true});

    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the empty hint text when there are no resources and no actions', () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByText(/No resources yet/i)).toBeInTheDocument();
  });

  it('does not show the empty hint when there are resources', () => {
    mockUseGetResources.mockReturnValue({data: withResources, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText(/No resources yet/i)).not.toBeInTheDocument();
  });

  it('does not show the empty hint when there are server actions', () => {
    mockUseGetServerActions.mockReturnValue({data: withActions, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText(/No resources yet/i)).not.toBeInTheDocument();
  });

  it('renders a single Add icon button in the toolbar', () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('button', {name: /Add/i})).toBeInTheDocument();
  });

  it('opens the add menu when the toolbar Add button is clicked', async () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: /Add/i}));

    await waitFor(() => {
      expect(screen.getByText('Add resource')).toBeInTheDocument();
      expect(screen.getByText('Add action')).toBeInTheDocument();
    });
  });

  it('opens the add dialog in resource mode when "Add resource" menu item is clicked', async () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: /Add/i}));

    await waitFor(() => {
      expect(screen.getByText('Add resource')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Add resource'));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('resource');
    });
  });

  it('opens the add dialog in server-action mode when "Add action" menu item is clicked', async () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: /Add/i}));

    await waitFor(() => {
      expect(screen.getByText('Add action')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Add action'));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('server-action');
    });
  });

  it('renders resource nodes from the API data', () => {
    mockUseGetResources.mockReturnValue({data: withResources, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByTestId('resource-node')).toBeInTheDocument();
    expect(screen.getByText('Documents')).toBeInTheDocument();
  });

  it('renders action nodes from the API data', () => {
    mockUseGetServerActions.mockReturnValue({data: withActions, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByTestId('action-node')).toBeInTheDocument();
    expect(screen.getByText('Read All')).toBeInTheDocument();
  });
});
