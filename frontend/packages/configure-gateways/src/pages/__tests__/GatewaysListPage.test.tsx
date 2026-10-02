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
  {id: 'gw-1', name: 'production', baseUrl: 'https://prod.example.com', createdAt: '2026-01-01T00:00:00Z'},
  {id: 'gw-2', name: 'staging', baseUrl: 'https://staging.example.com'},
];

const routes = {
  'GET /gateways': gateways,
  'GET /gateways/gw-1/applied-version': {gatewayId: 'gw-1', appliedVersion: 3},
  'GET /gateways/gw-2/applied-version': {gatewayId: 'gw-2'},
  'GET /configuration-versions': [{version: 3, note: 'Payments', createdAt: '2026-01-02T00:00:00Z'}, {version: 2}],
};

beforeEach(() => {
  mockHttpRequest.mockReset();
  mockNavigate.mockReset();
  mockNavigate.mockResolvedValue(undefined);
});

describe('GatewaysListPage', () => {
  it('lists gateways with the version each holds', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewaysListPage />);

    expect(screen.getByRole('heading', {name: 'Gateway Management'})).toBeInTheDocument();
    expect(await screen.findByText('production')).toBeInTheDocument();
    expect(screen.getByText('https://staging.example.com')).toBeInTheDocument();
    expect(await screen.findByText('Version 3')).toBeInTheDocument();
    expect(await screen.findByText('Not applied')).toBeInTheDocument();
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

  it('lists captured versions and captures a new one', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({...routes, 'POST /configuration-versions': {version: 4}}));
    renderWithProviders(<GatewaysListPage />);

    await user.click(screen.getByRole('tab', {name: 'Configuration versions'}));

    expect(await screen.findByText('Payments')).toBeInTheDocument();
    expect(screen.getByText('Version 2')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Register gateway'})).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Capture current configuration'}));
    await user.click(await screen.findByRole('button', {name: 'Capture'}));

    await waitFor(() =>
      expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({method: 'POST', data: {}})),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('tab', {name: 'Gateways'}));
    expect(await screen.findByText('production')).toBeInTheDocument();
  });

  it('shows a failed versions listing in place', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation((call: HttpCall) =>
      call.url.endsWith('/configuration-versions') ? Promise.reject(apiError(500)) : createHttpRouter(routes)(call),
    );
    renderWithProviders(<GatewaysListPage />);

    await user.click(screen.getByRole('tab', {name: 'Configuration versions'}));

    expect(await screen.findByText('Failed to load configuration versions')).toBeInTheDocument();
  });
});
