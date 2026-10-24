// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import GatewayValuesPage from '../GatewayValuesPage';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});
const navigate = vi.fn();
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {...actual, useNavigate: () => navigate, useParams: () => ({gatewayId: 'gw-dev'})};
});

const hash = '2f2d727aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const page = (kind: string, values: unknown[]) => ({totalResults: values.length, [kind]: values});
let routes: Record<string, unknown>;

beforeEach(() => {
  navigate.mockReset();
  routes = {
    'GET /gateways': [{id: 'gw-dev', name: 'Dev', baseUrl: 'https://localhost:8090'}],
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev', appliedVersion: hash},
    [`GET /configuration-versions/${hash}`]: {
      version: hash,
      createdAt: '2026-10-12T13:52:47Z',
      references: [
        {
          name: 'APP_ORDERS_CLIENT_ID',
          kind: 'variable',
          resourceType: 'application',
          resourceName: 'Orders',
          field: 'clientId',
        },
        {
          name: 'APP_ORDERS_REDIRECT_URIS',
          kind: 'variable',
          resourceType: 'application',
          resourceName: 'Orders',
          field: 'redirectUris',
          list: true,
        },
        {
          name: 'APP_ORDERS_CLIENT_SECRET',
          kind: 'secret',
          resourceType: 'application',
          resourceName: 'Orders',
          field: 'clientSecret',
        },
      ],
    },
    'GET /gateways/gw-dev/variables': page('variables', [
      {name: 'APP_ORDERS_CLIENT_ID', value: 'orders'},
      {name: 'LEFTOVER', value: 'x'},
    ]),
    'GET /gateways/gw-dev/secrets': page('secrets', []),
  };
  http = fakeHttp(routes);
});

describe('GatewayValuesPage', () => {
  it('groups what the gateway holds by resource type and resource, the missing values first and open', async () => {
    render(<GatewayValuesPage />);

    expect(await screen.findByText('Configure Dev')).toBeInTheDocument();
    expect(await screen.findByText('The version Dev runs refers to 2 values it does not hold.')).toBeInTheDocument();
    const table = await screen.findByTestId('values-variable');
    const groups = within(table)
      .getAllByTestId(/^values-group-/)
      .map((row) => row.getAttribute('data-testid'));
    expect(groups).toEqual(['values-group-missing', 'values-group-application', 'values-group-other']);
    expect(within(screen.getByTestId('values-group-missing')).getByText('1')).toBeInTheDocument();
    expect(screen.getByText('Applications: Orders')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('value-row-APP_ORDERS_REDIRECT_URIS')).getByText('Redirect URIs'),
    ).toBeInTheDocument();
    expect(within(screen.getByTestId('value-row-APP_ORDERS_REDIRECT_URIS')).getByText('Not set')).toBeInTheDocument();
    expect(screen.queryByTestId('value-row-APP_ORDERS_CLIENT_ID')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Applications'}));
    const held = await screen.findByTestId('value-row-APP_ORDERS_CLIENT_ID');
    expect(within(held).getByText('Client ID')).toBeInTheDocument();
    expect(within(held).getByText('orders')).toBeInTheDocument();
    expect(screen.getByText('Orders')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Not used by the version this gateway runs'}));
    expect(await screen.findByTestId('value-row-LEFTOVER')).toBeInTheDocument();
  });

  it('sets a list one item per line, and a secret without reading it back', async () => {
    routes['PUT /gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS'] = {name: 'APP_ORDERS_REDIRECT_URIS'};
    routes['PUT /gateways/gw-dev/secrets/APP_ORDERS_CLIENT_SECRET'] = {name: 'APP_ORDERS_CLIENT_SECRET', exists: true};
    render(<GatewayValuesPage />);

    await userEvent.click(await screen.findByRole('button', {name: 'Set APP_ORDERS_REDIRECT_URIS'}));
    await userEvent.type(
      screen.getByLabelText('APP_ORDERS_REDIRECT_URIS'),
      'https://a.test/cb{enter}https://a.test/alt',
    );
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await userEvent.click(screen.getByRole('tab', {name: /Secrets/}));
    await userEvent.click(await screen.findByRole('button', {name: 'Set APP_ORDERS_CLIENT_SECRET'}));
    const secret = screen.getByLabelText('APP_ORDERS_CLIENT_SECRET');
    expect(secret).toHaveAttribute('type', 'password');
    await userEvent.type(secret, 's3cret');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(
        http.calls
          .filter((call: Call) => call.method === 'PUT')
          .map((call: Call) => [call.url.replace(SERVER, ''), call.data]),
      ).toEqual([
        [
          '/gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS',
          {value: '["https://a.test/cb","https://a.test/alt"]', description: undefined},
        ],
        ['/gateways/gw-dev/secrets/APP_ORDERS_CLIENT_SECRET', {value: 's3cret', description: undefined}],
      ]),
    );
  });

  it('removes a value and goes back to Deploy', async () => {
    routes['DELETE /gateways/gw-dev/variables/LEFTOVER'] = {};
    render(<GatewayValuesPage />);

    await userEvent.click(await screen.findByRole('button', {name: 'Not used by the version this gateway runs'}));
    await userEvent.click(await screen.findByRole('button', {name: 'Remove LEFTOVER'}));
    await waitFor(() => expect(http.calls.some((call: Call) => call.method === 'DELETE')).toBe(true));
    await userEvent.click(screen.getByText('Deploy'));
    expect(navigate).toHaveBeenCalledWith('/deploy');
  });

  it('says when what the running version refers to could not be read, rather than showing nothing missing', async () => {
    routes[`GET /configuration-versions/${hash}`] = new Error('down');
    render(<GatewayValuesPage />);

    expect(
      await screen.findByText('Which values the version Dev runs refers to could not be read.'),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('values-variable')).not.toBeInTheDocument();
  });

  it('closes an open editor when the tab changes, so a draft never carries over', async () => {
    render(<GatewayValuesPage />);

    await userEvent.click(await screen.findByRole('button', {name: 'Set APP_ORDERS_REDIRECT_URIS'}));
    await userEvent.type(screen.getByLabelText('APP_ORDERS_REDIRECT_URIS'), 'https://a.test/cb');
    await userEvent.click(screen.getByRole('tab', {name: /Secrets/}));
    await userEvent.click(screen.getByRole('tab', {name: /Variables/}));

    expect(await screen.findByRole('button', {name: 'Set APP_ORDERS_REDIRECT_URIS'})).toBeInTheDocument();
    expect(screen.queryByLabelText('APP_ORDERS_REDIRECT_URIS')).not.toBeInTheDocument();
  });
});
