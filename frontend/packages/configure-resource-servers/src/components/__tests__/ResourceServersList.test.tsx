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
import type {ResourceServerListResponse} from '../../models/resource-server';
import ResourceServersList from '../ResourceServersList';

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

vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  };
});

vi.mock('../ResourceServerDeleteDialog', () => ({
  default: () => null,
}));

const mockUseGetResourceServers = vi.fn();

vi.mock('../../api/useGetResourceServers', () => ({
  default: (...args: unknown[]) =>
    mockUseGetResourceServers(...args) as {
      data: ResourceServerListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

const twoRowsResponse: ResourceServerListResponse = {
  totalResults: 2,
  startIndex: 0,
  count: 2,
  resourceServers: [
    {
      id: 'rs-1',
      name: 'Payments API',
      handle: 'payments-api',
      identifier: 'https://api.example.com',
      ouId: 'ou-1',
      delimiter: ':',
      type: 'API',
    },
    {
      id: 'rs-2',
      name: 'System MCP',
      handle: 'system-mcp',
      identifier: null,
      ouId: 'ou-1',
      delimiter: '/',
      type: 'MCP',
      isReadOnly: true,
    },
  ],
};

describe('ResourceServersList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetResourceServers.mockReturnValue({
      data: twoRowsResponse,
      isLoading: false,
      error: null,
    });
  });

  it('renders the type chip label for a normal row', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByText('API')).toBeInTheDocument();
  });

  it('renders the type chip label for a read-only row', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByText('MCP')).toBeInTheDocument();
  });

  it('shows Edit and Delete buttons for a normal row', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
  });

  it('shows a Read Only button for the read-only row', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByRole('button', {name: 'Read Only'})).toBeInTheDocument();
  });

  it('does not show a Delete button for the read-only row', () => {
    renderWithProviders(<ResourceServersList />);

    const deleteButtons = screen.queryAllByRole('button', {name: 'Delete'});
    expect(deleteButtons).toHaveLength(1);
  });
});
