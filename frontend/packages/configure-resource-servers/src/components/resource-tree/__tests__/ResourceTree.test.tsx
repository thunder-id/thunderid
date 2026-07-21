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
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API' as const,
};

const mockMcpResourceServer = {
  id: 'rs-mcp',
  name: 'My MCP Server',
  identifier: 'https://mcp.example.com',
  ouId: 'ou-1',
  delimiter: ':',
  type: 'MCP' as const,
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
  ResourceNode: ({node, kindFilter = undefined}: {node: {name: string}; kindFilter?: string}) => (
    <div data-testid="resource-node" data-kind-filter={kindFilter ?? 'all'}>
      {node.name}
    </div>
  ),
  ActionNode: ({action}: {action: {name: string; kind?: string}}) => (
    <div data-testid="action-node" data-kind={action.kind ?? 'action'}>
      {action.name}
    </div>
  ),
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

const withMcpTools: ActionListResponse = {
  totalResults: 1,
  startIndex: 0,
  count: 1,
  actions: [
    {id: 'tool-1', name: 'Search Files', handle: 'search-files', permission: 'my-mcp:search-files', kind: 'tool'},
  ],
};

const withMcpResources: ActionListResponse = {
  totalResults: 1,
  startIndex: 0,
  count: 1,
  actions: [
    {id: 'res-1', name: 'File Contents', handle: 'file-contents', permission: 'my-mcp:file-contents', kind: 'resource'},
  ],
};

const withMcpMixed: ActionListResponse = {
  totalResults: 2,
  startIndex: 0,
  count: 2,
  actions: [
    {id: 'tool-1', name: 'Search Files', handle: 'search-files', permission: 'my-mcp:search-files', kind: 'tool'},
    {id: 'res-1', name: 'File Contents', handle: 'file-contents', permission: 'my-mcp:file-contents', kind: 'resource'},
  ],
};

describe('ResourceTree (API type — generic tree)', () => {
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

  it('does not render the Capabilities panel header for an API-type server', () => {
    renderWithProviders(<ResourceTree resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText('Capabilities')).not.toBeInTheDocument();
  });
});

describe('ResourceTree (MCP type — Capabilities panel)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetResources.mockReturnValue({data: emptyResources, isLoading: false});
    mockUseGetServerActions.mockReturnValue({data: emptyActions, isLoading: false});
  });

  it('renders the Capabilities panel header', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByText('Capabilities')).toBeInTheDocument();
  });

  it('does not render separate TOOLS or RESOURCES subsection headers', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    // The chip labels in the radiogroup use "Tools" / "Resources" text, but there must be
    // no additional subsection header elements (i.e. no second occurrence as a section title).
    // We verify there is no heading-level or caption element acting as a section divider.
    // The only "Tools" / "Resources" text present should be inside the radiogroup chips.
    const radioGroup = screen.getByRole('radiogroup');
    expect(radioGroup).toBeInTheDocument();
    // Both tools and resources render as action-nodes in a single list (no section grouping).
    expect(screen.getByText('Search Files')).toBeInTheDocument();
    expect(screen.getByText('File Contents')).toBeInTheDocument();
  });

  it('does not render a separate Groups section header', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText('Groups')).not.toBeInTheDocument();
  });

  it('shows the loading spinner while data is loading', () => {
    mockUseGetResources.mockReturnValue({data: undefined, isLoading: true});
    mockUseGetServerActions.mockReturnValue({data: undefined, isLoading: true});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the MCP empty message and action buttons when all sections are empty', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByText(/No capabilities have been added to this MCP server yet/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add tool permission'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add resource permission'})).toBeInTheDocument();
  });

  it('does not render the filter chips toolbar when there are no capabilities', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /All/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /Tools/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /Resources/i})).not.toBeInTheDocument();
  });

  it('does not render the detail panel when all sections are empty', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByTestId('resource-detail-panel')).not.toBeInTheDocument();
  });

  it('renders the detail panel once capabilities exist', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByTestId('resource-detail-panel')).toBeInTheDocument();
  });

  it('does not show the old empty hint text (not subsection messages) when everything is empty', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(
      screen.queryByText('No capabilities yet. Use + to add a tool permission or resource permission.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('No tools yet.')).not.toBeInTheDocument();
    expect(screen.queryByText('No resources yet.')).not.toBeInTheDocument();
  });

  it('opens the add dialog in mcp-server-tool mode from the empty-state Add tool permission button', async () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Add tool permission'}));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('mcp-server-tool');
    });
  });

  it('opens the add dialog in mcp-server-resource mode from the empty-state Add resource permission button', async () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Add resource permission'}));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('mcp-server-resource');
    });
  });

  it('renders tools and resources intermixed in a single list (no section grouping)', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    const actionNodes = screen.getAllByTestId('action-node');
    expect(actionNodes.length).toBe(2);
    // Both kinds present in same flat list
    expect(actionNodes.some((n) => n.getAttribute('data-kind') === 'tool')).toBe(true);
    expect(actionNodes.some((n) => n.getAttribute('data-kind') === 'resource')).toBe(true);
  });

  it('renders tools in the unified tree when kind=tool actions exist', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpTools, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getAllByTestId('action-node').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Search Files')).toBeInTheDocument();
  });

  it('renders kind=resource actions in the unified tree', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpResources, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getAllByTestId('action-node').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('File Contents')).toBeInTheDocument();
  });

  it('renders the filter chip group for MCP', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('radiogroup', {name: /Filter capabilities/i})).toBeInTheDocument();
  });

  it('renders All, Tools, Resources filter chips', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    const radioGroup = screen.getByRole('radiogroup');
    expect(radioGroup).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /All/i})).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /Tools/i})).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /Resources/i})).toBeInTheDocument();
  });

  it('hides resource ActionNodes when Tools filter chip is selected', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('radio', {name: /Tools/i}));

    await waitFor(() => {
      expect(screen.getByText('Search Files')).toBeInTheDocument();
      expect(screen.queryByText('File Contents')).not.toBeInTheDocument();
    });
  });

  it('hides tool ActionNodes when Resources filter chip is selected', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('radio', {name: /Resources/i}));

    await waitFor(() => {
      expect(screen.getByText('File Contents')).toBeInTheDocument();
      expect(screen.queryByText('Search Files')).not.toBeInTheDocument();
    });
  });

  it('shows All filter as active restores both tools and resources', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('radio', {name: /Tools/i}));

    await waitFor(() => {
      expect(screen.queryByText('File Contents')).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('radio', {name: /All/i}));

    await waitFor(() => {
      expect(screen.getByText('Search Files')).toBeInTheDocument();
      expect(screen.getByText('File Contents')).toBeInTheDocument();
    });
  });

  it('opens the add dialog in mcp-server-tool mode from the header + menu', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Add'}));

    await waitFor(() => {
      expect(screen.getByRole('menuitem', {name: 'Add tool permission'})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('menuitem', {name: 'Add tool permission'}));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('mcp-server-tool');
    });
  });

  it('opens the add dialog in mcp-server-resource mode from the header + menu', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Add'}));

    await waitFor(() => {
      expect(screen.getByRole('menuitem', {name: 'Add resource permission'})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('menuitem', {name: 'Add resource permission'}));

    await waitFor(() => {
      expect(screen.getByRole('dialog', {name: 'add-node-dialog'})).toBeInTheDocument();
      expect(screen.getByTestId('dialog-mode').textContent).toBe('mcp-server-resource');
    });
  });

  it('does not render an Add namespace menu item for an MCP server', async () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Add'}));

    await waitFor(() => {
      expect(screen.getByRole('menuitem', {name: 'Add tool permission'})).toBeInTheDocument();
    });

    expect(screen.queryByText('Add namespace')).not.toBeInTheDocument();
  });

  it('does not render the generic Resource Hierarchy header for an MCP server', () => {
    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.queryByText('Resource Hierarchy')).not.toBeInTheDocument();
  });

  it('shows the All filter chip as selected by default (aria-checked=true)', () => {
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('radio', {name: /All/i})).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByRole('radio', {name: /Tools/i})).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByRole('radio', {name: /Resources/i})).toHaveAttribute('aria-checked', 'false');
  });

  it('shows filter chip labels without counts', () => {
    mockUseGetResources.mockReturnValue({data: emptyResources, isLoading: false});
    mockUseGetServerActions.mockReturnValue({data: withMcpMixed, isLoading: false});

    renderWithProviders(<ResourceTree resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByRole('radio', {name: /^All$/})).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /^Tools$/})).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /^Resources$/})).toBeInTheDocument();
  });
});
