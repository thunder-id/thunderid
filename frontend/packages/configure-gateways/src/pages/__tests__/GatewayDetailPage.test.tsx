// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiError, createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewayDetailPage from '../GatewayDetailPage';

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
let mockGatewayId = 'gw-1';
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
  useParams: () => ({gatewayId: mockGatewayId}),
}));

const gateway = {
  id: 'gw-1',
  name: 'production',
  baseUrl: 'https://prod.example.com',
  caCertificate: 'PEM',
  isDefault: true,
};

const routes = {
  'GET /gateways/gw-1': gateway,
};

beforeEach(() => {
  mockGatewayId = 'gw-1';
  mockHttpRequest.mockReset();
  mockNavigate.mockReset();
  mockNavigate.mockResolvedValue(undefined);
});

describe('GatewayDetailPage', () => {
  it('shows the gateway and its settings, and nothing about what it runs', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewayDetailPage />);

    expect(await screen.findByRole('heading', {name: /production/})).toHaveTextContent('Default');
    expect(screen.getByLabelText(/Base URL/)).toHaveValue('https://prod.example.com');
    expect(screen.getByLabelText(/CA certificate/)).toHaveValue('PEM');
    expect(screen.queryByText('You have unsaved changes.')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByText('Variables')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Revert/})).not.toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('removes the gateway once confirmed and goes back to the list', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({...routes, 'DELETE /gateways/gw-1': null}));
    renderWithProviders(<GatewayDetailPage />);

    await user.click(await screen.findByRole('button', {name: 'Remove gateway'}));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', {name: 'Remove'}));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/gateways'));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({method: 'DELETE'}));
  });

  it('saves only the fields that changed', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({...routes, 'PUT /gateways/gw-1': {...gateway, baseUrl: 'https://new.example.com'}}),
    );
    renderWithProviders(<GatewayDetailPage />);

    const baseUrl = await screen.findByLabelText(/Base URL/);
    await user.clear(baseUrl);
    await user.type(baseUrl, 'https://new.example.com');

    expect(screen.getByText('You have unsaved changes.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({method: 'PUT', data: {baseUrl: 'https://new.example.com'}}),
      ),
    );
    await waitFor(() => expect(screen.queryByText('You have unsaved changes.')).not.toBeInTheDocument());
  });

  it('drops unsaved edits when the page moves to another gateway', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        ...routes,
        'GET /gateways/gw-2': {id: 'gw-2', name: 'staging', baseUrl: 'https://staging.example.com'},
      }),
    );
    const {rerender} = renderWithProviders(<GatewayDetailPage />);

    const baseUrl = await screen.findByLabelText(/Base URL/);
    await user.clear(baseUrl);
    await user.type(baseUrl, 'https://edited.example.com');
    expect(screen.getByText('You have unsaved changes.')).toBeInTheDocument();

    mockGatewayId = 'gw-2';
    rerender(<GatewayDetailPage />);

    expect(await screen.findByRole('heading', {name: 'staging'})).toBeInTheDocument();
    expect(screen.getByLabelText(/Base URL/)).toHaveValue('https://staging.example.com');
    expect(screen.queryByText('You have unsaved changes.')).not.toBeInTheDocument();
  });

  it('will not save an empty name, and resets the form', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewayDetailPage />);

    const name = await screen.findByLabelText(/Name/);
    await user.clear(name);

    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Reset'}));
    expect(name).toHaveValue('production');
  });

  it('shows why a gateway declared in a file cannot be changed', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation((call: HttpCall) =>
      call.method === 'PUT' ? Promise.reject(apiError(400, 'GTW-1012')) : createHttpRouter(routes)(call),
    );
    renderWithProviders(<GatewayDetailPage />);

    await user.type(await screen.findByLabelText(/Name/), '-eu');
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(await screen.findByText(/declared in a file, so it cannot be changed or removed here/)).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), 'x');
    expect(screen.queryByText(/declared in a file/)).not.toBeInTheDocument();
  });

  it('opens the key rotation', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter(routes));
    renderWithProviders(<GatewayDetailPage />);

    await user.click(await screen.findByRole('button', {name: 'Rotate key'}));

    expect(await screen.findByRole('dialog')).toHaveTextContent('Replace the key presented to production');
  });

  it('shows a failed read with a way back', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockRejectedValue(apiError(404, 'GTW-1002'));
    renderWithProviders(<GatewayDetailPage />);

    expect(await screen.findByText('Failed to load the gateway')).toBeInTheDocument();
    expect(screen.getByText('The gateway was not found. It may have been removed.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Back to gateways'}));
    expect(mockNavigate).toHaveBeenCalledWith('/gateways');
  });
});
