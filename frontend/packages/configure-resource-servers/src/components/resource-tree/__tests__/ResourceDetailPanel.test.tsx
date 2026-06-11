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

import {renderWithProviders, screen} from '@thunderid/test-utils';
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

vi.mock('../../../api/useUpdateResourceServer', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
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
  handle: 'dark-dodos',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
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
});
