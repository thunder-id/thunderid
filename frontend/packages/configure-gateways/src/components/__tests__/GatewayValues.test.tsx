// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiError, createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewaySecretsCard from '../GatewaySecretsCard';
import GatewayVariablesCard from '../GatewayVariablesCard';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => SERVER_URL})};
});

const variables = {
  totalResults: 2,
  startIndex: 1,
  count: 2,
  variables: [
    {name: 'API_URL', value: 'https://api.example.com', description: 'Payments API'},
    {name: 'REGION', value: 'eu-west'},
  ],
  links: [],
};

const secrets = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  secrets: [{name: 'CLIENT_SECRET', exists: true, description: 'Payments client'}],
  links: [],
};

beforeEach(() => {
  mockHttpRequest.mockReset();
});

describe('GatewayVariablesCard', () => {
  it('lists the variables the gateway holds, a page at a time', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1/variables': variables}));
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    expect(await screen.findByText('API_URL')).toBeInTheDocument();
    expect(screen.getByText('https://api.example.com')).toBeInTheDocument();
    expect(screen.getByText('Payments API')).toBeInTheDocument();
    expect(screen.getByText('REGION')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/variables?limit=10&offset=0`, method: 'GET'}),
    );
  });

  it('checks a name before adding a variable', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/variables': variables,
        'POST /gateways/gw-1/variables': (call: HttpCall) => call.data,
      }),
    );
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Add variable'}));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Name/), '1BAD');
    await user.type(within(dialog).getByLabelText(/Value/), 'x');

    expect(within(dialog).getByText(/Use only letters, digits and underscores/)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', {name: 'Add'})).toBeDisabled();

    await user.clear(within(dialog).getByLabelText(/Name/));
    await user.type(within(dialog).getByLabelText(/Name/), 'TIMEOUT');
    await user.type(within(dialog).getByLabelText(/Description/), 'Seconds');
    await user.click(within(dialog).getByRole('button', {name: 'Add'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {name: 'TIMEOUT', value: 'x', description: 'Seconds'}}),
    );
  });

  it('shows why the gateway refused a variable, until the form changes', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation((call: HttpCall) =>
      call.method === 'POST'
        ? Promise.reject(apiError(409, 'VAR-1009'))
        : createHttpRouter({'GET /gateways/gw-1/variables': variables})(call),
    );
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Add variable'}));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Name/), 'REGION');
    await user.type(within(dialog).getByLabelText(/Value/), 'us');
    await user.click(within(dialog).getByRole('button', {name: 'Add'}));

    expect(await within(dialog).findByText('The gateway already holds something by that name.')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();

    await user.type(within(dialog).getByLabelText(/Name/), '2');
    expect(within(dialog).queryByText(/already holds something/)).not.toBeInTheDocument();
  });

  it('edits the value and description of a variable', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/variables': variables,
        'PUT /gateways/gw-1/variables/API_URL': (call: HttpCall) => ({name: 'API_URL', ...(call.data as object)}),
      }),
    );
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Edit API_URL'}));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText(/Name/)).toBeDisabled();
    const value = within(dialog).getByLabelText(/Value/);
    await user.clear(value);
    await user.type(value, 'https://new.example.com');
    await user.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'PUT',
        data: {value: 'https://new.example.com', description: 'Payments API'},
      }),
    );
  });

  it('deletes a variable once confirmed', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({'GET /gateways/gw-1/variables': variables, 'DELETE /gateways/gw-1/variables/REGION': null}),
    );
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Delete REGION'}));
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('Delete the variable REGION from this gateway?');
    await user.click(within(dialog).getByRole('button', {name: 'Delete'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/variables/REGION`, method: 'DELETE'}),
    );
  });

  it('steps back a page when a delete empties the last one', async () => {
    const user = userEvent.setup();
    let total = 11;
    const page = (call: HttpCall) => {
      const offset = Number(new URL(call.url).searchParams.get('offset'));
      const names = Array.from({length: total}, (_, i) => `VAR_${String(i).padStart(2, '0')}`).slice(
        offset,
        offset + 10,
      );
      return {
        totalResults: total,
        startIndex: offset + 1,
        count: names.length,
        variables: names.map((name) => ({name, value: 'x'})),
        links: [],
      };
    };
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/variables': page,
        'DELETE /gateways/gw-1/variables/VAR_10': () => {
          total = 10;
          return null;
        },
      }),
    );
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    await screen.findByText('VAR_00');
    await user.click(screen.getByRole('button', {name: /next page/i}));
    await user.click(await screen.findByRole('button', {name: 'Delete VAR_10'}));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', {name: 'Delete'}));

    expect(await screen.findByText('VAR_09')).toBeInTheDocument();
  });

  it('says so when the gateway cannot be reached', async () => {
    mockHttpRequest.mockRejectedValue(apiError(502, 'GTW-5001'));
    renderWithProviders(<GatewayVariablesCard gatewayId="gw-1" />);

    expect(await screen.findByText(/The gateway could not be reached/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add variable'})).toBeDisabled();
  });
});

describe('GatewaySecretsCard', () => {
  it('lists the secrets the gateway holds without their values', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1/secrets': secrets}));
    renderWithProviders(<GatewaySecretsCard gatewayId="gw-1" />);

    expect(await screen.findByText('CLIENT_SECRET')).toBeInTheDocument();
    expect(screen.getByText('Payments client')).toBeInTheDocument();
    expect(screen.getByText('Set')).toBeInTheDocument();
  });

  it('adds a secret through a password field', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/secrets': secrets,
        'POST /gateways/gw-1/secrets': {name: 'SIGNING_KEY', exists: true},
      }),
    );
    renderWithProviders(<GatewaySecretsCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Add secret'}));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Name/), 'SIGNING_KEY');
    const value = within(dialog).getByLabelText(/^Value/);
    expect(value).toHaveAttribute('type', 'password');
    await user.type(value, 'k3y');
    await user.click(within(dialog).getByRole('button', {name: 'Add'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {name: 'SIGNING_KEY', value: 'k3y'}}),
    );
  });

  it('replaces the value of a secret', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/secrets': secrets,
        'PUT /gateways/gw-1/secrets/CLIENT_SECRET': {name: 'CLIENT_SECRET', exists: true},
      }),
    );
    renderWithProviders(<GatewaySecretsCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Replace the value of CLIENT_SECRET'}));
    const dialog = await screen.findByRole('dialog');
    const value = within(dialog).getByLabelText(/New value/);
    expect(value).toHaveValue('');
    expect(value).toHaveAttribute('type', 'password');
    await user.type(value, 'rotated');
    await user.click(within(dialog).getByRole('button', {name: 'Replace'}));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'PUT', data: {value: 'rotated', description: 'Payments client'}}),
    );
  });

  it('deletes a secret once confirmed', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/secrets': secrets,
        'DELETE /gateways/gw-1/secrets/CLIENT_SECRET': null,
      }),
    );
    renderWithProviders(<GatewaySecretsCard gatewayId="gw-1" />);

    await user.click(await screen.findByRole('button', {name: 'Delete CLIENT_SECRET'}));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', {name: 'Delete'}));

    await waitFor(() =>
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/secrets/CLIENT_SECRET`, method: 'DELETE'}),
      ),
    );
  });

  it('says so when the gateway cannot be reached', async () => {
    mockHttpRequest.mockRejectedValue(apiError(502));
    renderWithProviders(<GatewaySecretsCard gatewayId="gw-1" />);

    expect(await screen.findByText(/The gateway could not be reached/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add secret'})).toBeDisabled();
  });
});
