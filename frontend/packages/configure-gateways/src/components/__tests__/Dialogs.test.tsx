// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiError, createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewayDeleteDialog from '../GatewayDeleteDialog';
import RegisterGatewayDialog from '../RegisterGatewayDialog';
import RotateKeyDialog from '../RotateKeyDialog';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => SERVER_URL})};
});

const gateway = {id: 'gw-1', name: 'production', baseUrl: 'https://dp.example.com'};

beforeEach(() => {
  mockHttpRequest.mockReset();
});

describe('RegisterGatewayDialog', () => {
  it('needs a name and a base URL before it registers', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={vi.fn()} />);

    const submit = screen.getByRole('button', {name: 'Register'});
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText(/Name/), 'production');
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    expect(submit).toBeEnabled();
  });

  it('registers, shows the key once with a warning, then hands over the gateway', async () => {
    const user = userEvent.setup();
    const onRegistered = vi.fn();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways': {...gateway, key: 'generated-key'}}));
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={onRegistered} />);

    await user.type(screen.getByLabelText(/Name/), ' production ');
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    await user.type(screen.getByLabelText(/CA certificate/), 'PEM');
    await user.click(screen.getByRole('button', {name: 'Register'}));

    expect(await screen.findByText('generated-key')).toBeInTheDocument();
    expect(screen.getByText(/This key will not be shown again/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Copy key'})).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'POST',
        data: {name: 'production', baseUrl: 'https://dp.example.com', caCertificate: 'PEM'},
      }),
    );

    await user.click(screen.getByRole('button', {name: 'I have saved the key'}));
    expect(onRegistered).toHaveBeenCalledWith(expect.objectContaining({id: 'gw-1', key: 'generated-key'}));
  });

  it('keeps the key on screen through Escape or a click outside', async () => {
    const user = userEvent.setup();
    const onRegistered = vi.fn();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways': {...gateway, key: 'generated-key'}}));
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={onRegistered} />);

    await user.type(screen.getByLabelText(/Name/), 'production');
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    await user.click(screen.getByRole('button', {name: 'Register'}));
    await screen.findByText('generated-key');

    screen.getByRole('button', {name: 'I have saved the key'}).focus();
    await user.keyboard('{Escape}');
    const backdrop = document.querySelector('.MuiBackdrop-root');
    expect(backdrop).not.toBeNull();
    await user.click(backdrop!);
    expect(screen.getByText('generated-key')).toBeInTheDocument();
    expect(onRegistered).not.toHaveBeenCalled();
  });

  it('sends a supplied key', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways': {...gateway, key: 'mine'}}));
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={vi.fn()} />);

    await user.type(screen.getByLabelText(/Name/), 'production');
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    await user.type(screen.getByLabelText(/Key/), 'mine');
    await user.click(screen.getByRole('button', {name: 'Register'}));

    await waitFor(() =>
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({data: {name: 'production', baseUrl: 'https://dp.example.com', key: 'mine'}}),
      ),
    );
  });

  it('registers the gateway as the default when asked', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways': {...gateway, key: 'k', isDefault: true}}));
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={vi.fn()} />);

    await user.type(screen.getByLabelText(/Name/), 'production');
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    await user.click(screen.getByRole('checkbox', {name: /Make this the default gateway/}));
    await user.click(screen.getByRole('button', {name: 'Register'}));

    await waitFor(() =>
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({data: {name: 'production', baseUrl: 'https://dp.example.com', isDefault: true}}),
      ),
    );
  });

  it('shows the catalog message for a refused registration and clears it on edit', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockRejectedValue(apiError(409, 'GTW-1004'));
    renderWithProviders(<RegisterGatewayDialog open onClose={vi.fn()} onRegistered={vi.fn()} />);

    await user.type(screen.getByLabelText(/Name/), 'production');
    await user.type(screen.getByLabelText(/Base URL/), 'https://dp.example.com');
    await user.click(screen.getByRole('button', {name: 'Register'}));

    expect(await screen.findByText('Another gateway is already registered under that name.')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), '-2');
    expect(screen.queryByText('Another gateway is already registered under that name.')).not.toBeInTheDocument();
  });

  it('closes without registering on cancel', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<RegisterGatewayDialog open onClose={onClose} onRegistered={vi.fn()} />);

    await user.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(onClose).toHaveBeenCalled();
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});

describe('GatewayDeleteDialog', () => {
  it('removes the gateway on confirm', async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    mockHttpRequest.mockImplementation(createHttpRouter({'DELETE /gateways/gw-1': null}));
    renderWithProviders(<GatewayDeleteDialog open gateway={gateway} onClose={vi.fn()} onSuccess={onSuccess} />);

    expect(screen.getByText('production')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Remove'}));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
  });

  it('explains why a gateway declared in a file cannot be removed', async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    mockHttpRequest.mockRejectedValue(apiError(400, 'GTW-1012'));
    renderWithProviders(<GatewayDeleteDialog open gateway={gateway} onClose={vi.fn()} onSuccess={onSuccess} />);

    await user.click(screen.getByRole('button', {name: 'Remove'}));

    expect(await screen.findByText(/declared in a file, so it cannot be changed or removed here/)).toBeInTheDocument();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('does nothing without a gateway, and closes on cancel', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<GatewayDeleteDialog open gateway={null} onClose={onClose} onSuccess={vi.fn()} />);

    await user.click(screen.getByRole('button', {name: 'Remove'}));
    expect(mockHttpRequest).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(onClose).toHaveBeenCalled();
  });
});

describe('RotateKeyDialog', () => {
  it('rotates to a generated key and shows it to copy', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockHttpRequest.mockImplementation(createHttpRouter({'PUT /gateways/gw-1': gateway}));
    renderWithProviders(<RotateKeyDialog open gatewayId="gw-1" gatewayName="production" onClose={onClose} />);

    const submit = screen.getByRole('button', {name: 'Rotate key'});
    expect(submit).toBeDisabled();

    await user.click(screen.getByRole('button', {name: 'Generate'}));
    expect(screen.getByText('Key to give the gateway')).toBeInTheDocument();
    expect(screen.getByText(/will not be shown again/)).toBeInTheDocument();

    await user.click(submit);

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    const call = mockHttpRequest.mock.calls[0][0];
    expect(call.method).toBe('PUT');
    expect((call.data as {key: string}).key).toMatch(/^[A-Za-z0-9_-]{43}$/);
  });

  it('shows the catalog message when the rotation is refused', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockRejectedValue(apiError(400, 'GTW-1012'));
    renderWithProviders(<RotateKeyDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    await user.type(screen.getByLabelText(/New key/), 'typed-key');
    await user.click(screen.getByRole('button', {name: 'Rotate key'}));

    expect(await screen.findByText(/declared in a file/)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/New key/), '2');
    expect(screen.queryByText(/declared in a file/)).not.toBeInTheDocument();
  });
});
