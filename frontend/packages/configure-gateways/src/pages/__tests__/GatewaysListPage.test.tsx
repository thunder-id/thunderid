// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiError, createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewaysListPage from '../GatewaysListPage';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => SERVER_URL})};
});

const mockNavigate = vi.fn();
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

const gateways = [
  {
    id: 'gw-1',
    name: 'production',
    baseUrl: 'https://prod.example.com',
    isDefault: true,
    createdAt: '2026-01-01T00:00:00Z',
  },
  {id: 'gw-2', name: 'staging', baseUrl: 'https://staging.example.com'},
];

const routes = {
  'GET /gateways': gateways,
};

beforeEach(() => {
  mockHttpRequest.mockReset();
  mockNavigate.mockReset();
  mockNavigate.mockResolvedValue(undefined);
});

describe('GatewaysListPage', () => {
  it('lists the gateways, marking the default, and nothing about what they run', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewaysListPage />);

    expect(screen.getByRole('heading', {name: 'Gateway Management'})).toBeInTheDocument();
    expect(await screen.findByText('production')).toBeInTheDocument();
    expect(screen.getByText('https://staging.example.com')).toBeInTheDocument();
    expect(screen.getAllByText('Default')).toHaveLength(1);
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByText('Applied version')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Capture current configuration'})).not.toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('opens a gateway from its row', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewaysListPage />);

    await user.click(await screen.findByText('production'));
    expect(mockNavigate).toHaveBeenCalledWith('/gateways/gw-1');

    await user.click(screen.getAllByRole('button', {name: 'Edit'})[1]);
    expect(mockNavigate).toHaveBeenCalledWith('/gateways/gw-2');
  });

  it('confirms before removing a gateway', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({...routes, 'DELETE /gateways/gw-1': null}));
    renderWithProviders(<GatewaysListPage />);

    await screen.findByText('production');
    await user.click(screen.getAllByRole('button', {name: 'Delete'})[0]);

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('production')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', {name: 'Remove'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({method: 'DELETE'}));
  });

  it('registers a gateway and goes to it once the key is saved', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        ...routes,
        'POST /gateways': {id: 'gw-9', name: 'edge', baseUrl: 'https://edge.example.com', key: 'k'},
      }),
    );
    renderWithProviders(<GatewaysListPage />);

    await user.click(screen.getByRole('button', {name: 'Register gateway'}));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Name/), 'edge');
    await user.type(within(dialog).getByLabelText(/Base URL/), 'https://edge.example.com');
    await user.click(within(dialog).getByRole('button', {name: 'Register'}));
    await user.click(await within(dialog).findByRole('button', {name: 'I have saved the key'}));

    expect(mockNavigate).toHaveBeenCalledWith('/gateways/gw-9');
  });

  it('shows a failed listing in place of the table', async () => {
    mockHttpRequest.mockRejectedValue(apiError(500));
    renderWithProviders(<GatewaysListPage />);

    expect(await screen.findByText('Failed to load gateways')).toBeInTheDocument();
  });
});
