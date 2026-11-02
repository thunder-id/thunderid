// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import * as thunderIdReactModule from '@thunderid/react';
import {renderWithProviders, screen, fireEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {DefaultResourceServerConfigResponse, ResourceServerListResponse} from '../../models/resource-server';
import ResourceServersList from '../ResourceServersList';

vi.mock('@thunderid/react', {spy: true});

// eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- vi.mock({spy:true}) type inference doesn't resolve for this package's conditional exports
vi.mocked(thunderIdReactModule.useThunderID).mockImplementation(() => ({http: {request: vi.fn()}}) as never);

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

vi.mock('../SetDefaultResourceServerDialog', () => ({
  default: ({open, resourceServer}: {open: boolean; resourceServer: {name: string} | null}) =>
    open ? <div data-testid="set-default-dialog">{resourceServer?.name}</div> : null,
}));

const mockUseGetResourceServers = vi.fn();
const mockRefetch = vi.fn();

vi.mock('../../api/useGetResourceServers', () => ({
  default: (...args: unknown[]) =>
    mockUseGetResourceServers(...args) as {
      data: ResourceServerListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
      refetch: () => void;
    },
}));

const mockUseGetDefaultResourceServer = vi.fn();

vi.mock('../../api/useGetDefaultResourceServer', () => ({
  default: () => mockUseGetDefaultResourceServer() as {data: DefaultResourceServerConfigResponse | undefined},
}));

const twoRowsResponse: ResourceServerListResponse = {
  totalResults: 2,
  startIndex: 0,
  count: 2,
  resourceServers: [
    {
      id: 'rs-1',
      name: 'Payments API',
      identifier: 'https://api.example.com',
      ouId: 'ou-1',
      delimiter: ':',
      type: 'API',
    },
    {
      id: 'rs-2',
      name: 'System MCP',
      identifier: 'https://mcp.example.com',
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
      refetch: mockRefetch,
    });
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {resourceServerId: 'rs-1'}},
    });
  });

  it('renders the grid inside a paper container as a plain data grid', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByRole('grid').closest('.MuiPaper-root')).not.toBeNull();
    // The card variant sizes rows to their content, while the plain grid keeps the fixed two-line height.
    expect(screen.getAllByRole('row')[1]).toHaveStyle({minHeight: '64px', maxHeight: '64px'});
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

    expect(screen.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
  });

  it('does not show a Delete button for the read-only row', () => {
    renderWithProviders(<ResourceServersList />);

    const deleteButtons = screen.queryAllByRole('button', {name: 'Delete'});
    expect(deleteButtons).toHaveLength(1);
  });

  it('marks only the read-only row as a system resource server', () => {
    renderWithProviders(<ResourceServersList />);

    const systemRow = screen.getByText('System MCP').closest<HTMLElement>('[role="row"]')!;
    const normalRow = screen.getByText('Payments API').closest<HTMLElement>('[role="row"]')!;
    expect(within(systemRow).getByText('System resource server')).toBeInTheDocument();
    expect(within(normalRow).queryByText('System resource server')).not.toBeInTheDocument();
  });

  it('shows a Default badge on the current default row', () => {
    renderWithProviders(<ResourceServersList />);

    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('does not show a Default badge when no default is set', () => {
    mockUseGetDefaultResourceServer.mockReturnValue({data: {readOnly: {}, writable: {}, merged: {}}});

    renderWithProviders(<ResourceServersList />);

    expect(screen.queryByText('Default')).not.toBeInTheDocument();
  });

  it('renders the resolved catalog message in the error state, never the raw server text, when the fetch fails', () => {
    mockUseGetResourceServers.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('raw backend list failure detail'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServersList />);

    expect(screen.getByText('Failed to load resource servers')).toBeInTheDocument();
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.queryByText('raw backend list failure detail')).not.toBeInTheDocument();
  });

  it('retries the fetch when Refresh is clicked in the error state', () => {
    mockUseGetResourceServers.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServersList />);

    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    expect(mockRefetch).toHaveBeenCalled();
  });
});

describe('ResourceServersList make default action', () => {
  const eligibleRow = {
    id: 'rs-eligible',
    name: 'Billing API',
    identifier: 'https://billing.example.com',
    ouId: 'ou-1',
    delimiter: ':',
    type: 'API' as const,
  };

  const listWith = (...resourceServers: ResourceServerListResponse['resourceServers']): ResourceServerListResponse => ({
    totalResults: resourceServers.length,
    startIndex: 0,
    count: resourceServers.length,
    resourceServers,
  });

  const renderWithDefault = (
    rows: ResourceServerListResponse,
    defaultConfig: DefaultResourceServerConfigResponse | undefined,
    defaultQueryState: {isLoading?: boolean; error?: Error | null} = {},
  ): void => {
    mockUseGetResourceServers.mockReturnValue({data: rows, isLoading: false, error: null, refetch: mockRefetch});
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: defaultConfig,
      isLoading: defaultQueryState.isLoading ?? false,
      error: defaultQueryState.error ?? null,
    });
    renderWithProviders(<ResourceServersList />);
  };

  const noDefault: DefaultResourceServerConfigResponse = {readOnly: {}, writable: {}, merged: {}};
  const moreButton = () => screen.queryByRole('button', {name: /more actions/i});

  const openMakeDefaultItem = (): HTMLElement => {
    fireEvent.click(moreButton()!);
    return screen.getByRole('menuitem', {name: /make default resource server/i});
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the actions menu available on every editable row', () => {
    renderWithDefault(listWith(eligibleRow), noDefault);

    expect(moreButton()).toBeInTheDocument();
  });

  it('enables the action for a resource server that could become the default', () => {
    renderWithDefault(listWith(eligibleRow), noDefault);

    expect(openMakeDefaultItem()).not.toHaveAttribute('aria-disabled', 'true');
  });

  it('disables the action for the resource server that is already the default', () => {
    renderWithDefault(listWith(eligibleRow), {
      readOnly: {},
      writable: {resourceServerId: eligibleRow.id},
      merged: {resourceServerId: eligibleRow.id},
    });

    const item = openMakeDefaultItem();

    expect(item).toHaveAttribute('aria-disabled', 'true');
    expect(item).toHaveTextContent(/already the default/i);
  });

  it('disables the action for an MCP server', () => {
    renderWithDefault(listWith({...eligibleRow, type: 'MCP'}), noDefault);

    const item = openMakeDefaultItem();

    expect(item).toHaveAttribute('aria-disabled', 'true');
    expect(item).toHaveTextContent(/only api and custom/i);
  });

  it('disables the action when a declarative default has locked it', () => {
    renderWithDefault(listWith(eligibleRow), {
      readOnly: {resourceServerId: 'rs-locked'},
      writable: {},
      merged: {resourceServerId: 'rs-locked'},
    });

    const item = openMakeDefaultItem();

    expect(item).toHaveAttribute('aria-disabled', 'true');
    expect(item).toHaveTextContent(/deployment configuration/i);
  });

  it('does not offer any actions for a read-only resource server', () => {
    renderWithDefault(listWith({...eligibleRow, isReadOnly: true}), noDefault);

    expect(moreButton()).toBeNull();
  });

  it('opens the set default dialog for the chosen resource server', () => {
    renderWithDefault(listWith(eligibleRow), noDefault);

    fireEvent.click(openMakeDefaultItem());

    expect(screen.getByTestId('set-default-dialog')).toHaveTextContent('Billing API');
  });

  it('does not open the set default dialog when the action is disabled', () => {
    renderWithDefault(listWith(eligibleRow), {
      readOnly: {resourceServerId: 'rs-locked'},
      writable: {},
      merged: {resourceServerId: 'rs-locked'},
    });

    fireEvent.click(openMakeDefaultItem());

    expect(screen.queryByTestId('set-default-dialog')).toBeNull();
  });

  it('disables the action while the default configuration is still loading', () => {
    renderWithDefault(listWith(eligibleRow), undefined, {isLoading: true});

    const item = openMakeDefaultItem();

    expect(item).toHaveAttribute('aria-disabled', 'true');
    expect(item).toHaveTextContent(/checking the current default/i);
  });

  it('disables the action when the default configuration failed to load', () => {
    renderWithDefault(listWith(eligibleRow), undefined, {error: new Error('boom')});

    expect(openMakeDefaultItem()).toHaveAttribute('aria-disabled', 'true');
  });
});
