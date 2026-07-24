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

import {fireEvent, renderWithProviders, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ResourceServer} from '../../../models/resource-server';
import ResourceDetailPanel from '../ResourceDetailPanel';
import type {SelectedNode} from '../ResourceTree';

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

const mockUpdateResourceServerMutate = vi.fn();

vi.mock('../../../api/useUpdateResourceServer', () => ({
  default: () => ({mutate: mockUpdateResourceServerMutate, isPending: false}),
}));

vi.mock('../../../api/useUpdateResource', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
}));

vi.mock('../../../api/useUpdateAction', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
}));

const mockResourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Dark Dodos Smash',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
};

const mockMcpResourceServer: ResourceServer = {
  id: 'rs-mcp',
  name: 'My MCP Server',
  identifier: 'https://mcp.example.com',
  ouId: 'ou-1',
  delimiter: ':',
  type: 'MCP',
};

describe('ResourceDetailPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the select-a-node placeholder when selectedNode is null', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={null} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText(/Select a node from the tree to view its details/i)).toBeInTheDocument();
  });

  it('renders the name field pre-filled when a resource node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByDisplayValue('Documents')).toBeInTheDocument();
  });

  it('renders the permission chip when a resource node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('dark-dodos/documents')).toBeInTheDocument();
  });

  it('renders the handle chip when a resource node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('documents')).toBeInTheDocument();
  });

  it('renders the copy button when a resource node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('renders the identifier field when a server node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByDisplayValue('https://api.example.com')).toBeInTheDocument();
  });

  it('includes the resource server ouId when saving a server identifier change', async () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    fireEvent.change(screen.getByDisplayValue('https://api.example.com'), {
      target: {value: 'https://new-api.example.com'},
    });

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Save/i}));

    expect(mockUpdateResourceServerMutate).toHaveBeenCalledWith(
      {
        id: 'rs-1',
        data: {
          name: 'Dark Dodos Smash',
          description: null,
          identifier: 'https://new-api.example.com',
          ouId: 'ou-1',
        },
      },
      expect.any(Object),
    );
  });

  it('does not save when the server identifier is cleared', async () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    fireEvent.change(screen.getByDisplayValue('https://api.example.com'), {
      target: {value: '   '},
    });

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Save/i}));

    expect(mockUpdateResourceServerMutate).not.toHaveBeenCalled();
  });

  it('renders the delimiter chip when a server node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('/')).toBeInTheDocument();
  });

  it('renders the read-only warning alert when a read-only server node is selected', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: readOnlyResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={readOnlyResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText(/This is a system resource server and cannot be modified/i)).toBeInTheDocument();
  });

  it('does not render the read-only warning alert when the server is not read-only', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/This is a system resource server and cannot be modified/i)).not.toBeInTheDocument();
  });

  it('hides the Save/Discard bar when the name is typed away and back to original', async () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    const input = screen.getByDisplayValue('Documents');
    fireEvent.change(input, {target: {value: 'Docs'}});
    await waitFor(() => expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument());

    fireEvent.change(input, {target: {value: 'Documents'}});

    await waitFor(() => expect(screen.queryByRole('button', {name: /Save/i})).not.toBeInTheDocument());
  });

  it('shows the bar again after Discard if the field is re-edited to a new value', async () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    let input = screen.getByDisplayValue('Documents');
    fireEvent.change(input, {target: {value: 'Docs'}});
    await waitFor(() => expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', {name: /Discard/i}));
    await waitFor(() => expect(screen.queryByRole('button', {name: /Save/i})).not.toBeInTheDocument());
    expect(screen.getByDisplayValue('Documents')).toBeInTheDocument();

    input = screen.getByDisplayValue('Documents');
    fireEvent.change(input, {target: {value: 'Renamed Again'}});
    await waitFor(() => expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument());
  });
});

describe('ResourceDetailPanel (MCP non-server node)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const toolNode: SelectedNode = {
    type: 'server-action',
    id: 'tool-1',
    data: {id: 'tool-1', name: 'Search Files', handle: 'search-files', permission: 'my-mcp:search-files', kind: 'tool'},
  };

  const namespaceNode: SelectedNode = {
    type: 'resource',
    id: 'ns-1',
    data: {id: 'ns-1', name: 'Booking', handle: 'booking', permission: 'my-mcp:booking'},
  };

  it('renders the node name as an h5 heading', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByRole('heading', {level: 5, name: 'Search Files'})).toBeInTheDocument();
  });

  it('renders the kind label with an icon as a subtitle, not a chip', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const label = screen.getByText('Tool');
    expect(label).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Tool'})).not.toBeInTheDocument();
    expect(label.parentElement?.querySelector('svg')).toBeInTheDocument();
  });

  it('renders the Namespace kind label with an icon for a namespace node', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={namespaceNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const label = screen.getByText('Namespace');
    expect(label).toBeInTheDocument();
    expect(label.parentElement?.querySelector('svg')).toBeInTheDocument();
  });

  it('does not render a breadcrumb', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/›/)).not.toBeInTheDocument();
  });

  it('renders the Handle field as read-only with the node handle', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const handleInput = screen.getByDisplayValue('search-files');
    expect(handleInput).toHaveAttribute('readonly');
  });

  it('does not render a copy button for the Handle field', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const handleInput = screen.getByDisplayValue('search-files');
    const handleFormControl = handleInput.closest('.MuiFormControl-root');
    expect(handleFormControl?.querySelector('button')).not.toBeInTheDocument();
  });

  it('renders the Delimiter field as read-only with the resource server delimiter', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const delimiterInput = screen.getByDisplayValue(':');
    expect(delimiterInput).toHaveAttribute('readonly');
  });

  it('renders Name, Handle, Delimiter and Description fields in that order', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    const labels = screen.getAllByText(/^(Name|Handle \(immutable\)|Delimiter \(immutable\)|Description)$/);
    expect(labels.map((el) => el.textContent)).toEqual([
      'Name',
      'Handle (immutable)',
      'Delimiter (immutable)',
      'Description',
    ]);
  });

  it('renders a copy button for the Permission field', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByRole('button', {name: 'Copy'})).toBeInTheDocument();
  });

  it('does not render an Advanced toggle', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/Advanced/i)).not.toBeInTheDocument();
  });

  it('renders the Permission field as a read-only text field with the derived permission', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('Permission')).toBeInTheDocument();
    const permissionInput = screen.getByDisplayValue('my-mcp:search-files');
    expect(permissionInput).toHaveAttribute('readonly');
  });

  it('renders the permission help text as helper text below the field', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText(/Built from the resource path and the name/i)).toBeInTheDocument();
  });

  it('renders kind-aware helper text for Name, Handle, Delimiter and Description for a tool node', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={toolNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('A human-readable name for this tool.')).toBeInTheDocument();
    expect(
      screen.getByText('Stable identifier for this tool, used to build the permission scope.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Separates segments in the permission scope. Defined by the resource server.'),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Describe what this tool is for.')).toBeInTheDocument();
  });

  it('renders kind-aware helper text for Name and Description for a namespace node', () => {
    renderWithProviders(
      <ResourceDetailPanel selectedNode={namespaceNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.getByText('A human-readable name for this namespace.')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Describe what this namespace is for.')).toBeInTheDocument();
  });
});

describe('ResourceDetailPanel (non-MCP nodes do not show MCP hints)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not render MCP-specific helper text for a generic resource node', () => {
    const selectedNode: SelectedNode = {
      type: 'resource',
      id: 'r-1',
      data: {id: 'r-1', name: 'Documents', handle: 'documents', permission: 'dark-dodos/documents'},
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/A human-readable name for this/i)).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/Describe what this/i)).not.toBeInTheDocument();
  });

  it('does not render MCP-specific helper text for a server node', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-1',
      data: mockResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/A human-readable name for this/i)).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/Describe what this/i)).not.toBeInTheDocument();
  });

  it('does not render MCP-specific helper text for an MCP server node', () => {
    const selectedNode: SelectedNode = {
      type: 'server',
      id: 'rs-mcp',
      data: mockMcpResourceServer,
    };

    renderWithProviders(
      <ResourceDetailPanel selectedNode={selectedNode} resourceServer={mockMcpResourceServer} onRefresh={vi.fn()} />,
    );

    expect(screen.queryByText(/A human-readable name for this/i)).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/Describe what this/i)).not.toBeInTheDocument();
  });
});
