// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, HiddenResourcesProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, queryOf, SERVER, storeListing, type Call} from '../../__tests__/fakeHttp';
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
const page = (kind: 'variables' | 'secrets', values: {name: string; value?: string; exists?: boolean}[]) =>
  storeListing(kind, values);
/** The reads of a gateway's variables or secrets, each by its query. */
const reads = (collection: string): Record<string, string>[] =>
  http.calls
    .filter((call: Call) => call.method === 'GET' && call.url.split('?')[0].endsWith(`/gw-dev/${collection}`))
    .map(queryOf);
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
  it('shows the missing values first and open, and lists what the gateway holds a page at a time', async () => {
    render(<GatewayValuesPage />);

    expect(await screen.findByText('Configure Dev')).toBeInTheDocument();
    expect(await screen.findByText('The version Dev runs refers to 2 values it does not hold.')).toBeInTheDocument();
    const missing = await screen.findByTestId('values-variable');
    expect(
      within(missing)
        .getAllByTestId(/^values-group-/)
        .map((row) => row.getAttribute('data-testid')),
    ).toEqual(['values-group-missing']);
    expect(within(screen.getByTestId('values-group-missing')).getByText('1')).toBeInTheDocument();
    expect(screen.getByText('Applications: Orders')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('value-row-APP_ORDERS_REDIRECT_URIS')).getByText('Redirect URIs'),
    ).toBeInTheDocument();
    expect(within(screen.getByTestId('value-row-APP_ORDERS_REDIRECT_URIS')).getByText('Not set')).toBeInTheDocument();

    const list = screen.getByTestId('values-list-variable');
    expect(await within(list).findByText('orders')).toBeInTheDocument();
    expect(within(list).getByText('Client ID')).toBeInTheDocument();
    expect(within(list).getByText('Application: Orders')).toBeInTheDocument();
    expect(within(list).getByText('LEFTOVER')).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'Variables (2)'})).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'Secrets (0)'})).toBeInTheDocument();

    // Only the names the running version refers to are looked up; everything else is read a page at a time.
    expect(reads('variables')).toEqual(
      expect.arrayContaining([
        {limit: '100', offset: '0', names: 'APP_ORDERS_CLIENT_ID,APP_ORDERS_REDIRECT_URIS'},
        {limit: '10', offset: '0'},
      ]),
    );
    expect(reads('variables').every((query) => Number(query.limit) <= 100)).toBe(true);
    expect(reads('secrets')).toContainEqual({limit: '100', offset: '0', names: 'APP_ORDERS_CLIENT_SECRET'});
  });

  it('names the resource a value belongs to from the current configuration when the running version does not refer to it', async () => {
    const adminConsole = 'APPLICATION__019000000000700080000000000000F1__CLIENT_ID';
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev'};
    routes['GET /configuration-versions/current'] = {
      version: 'current',
      createdAt: '2026-10-12T13:52:47Z',
      references: [
        {
          name: adminConsole,
          kind: 'variable',
          resourceType: 'application',
          resourceId: '01900000-0000-7000-8000-0000000000f1',
          resourceName: 'Admin Console',
          field: 'clientId',
        },
      ],
    };
    routes['GET /gateways/gw-dev/variables'] = page('variables', [
      {name: adminConsole, value: 'ADMIN_CONSOLE'},
      {name: 'LEFTOVER', value: 'x'},
    ]);
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    expect(await within(list).findByText('Application: Admin Console')).toBeInTheDocument();
    expect(within(list).getByText('Client ID')).toBeInTheDocument();
    expect(within(list).getByText(adminConsole)).toBeInTheDocument();
    expect(within(list).getByText('Not used by the running version')).toBeInTheDocument();
    expect(within(list).getByText('LEFTOVER')).toBeInTheDocument();
    expect(screen.queryByText(/refers to .* values it does not hold/)).not.toBeInTheDocument();
  });

  it('saves a list value of a row labelled from the current configuration as a JSON list', async () => {
    const redirectUris = 'APPLICATION__019000000000700080000000000000F1__REDIRECT_URIS';
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev'};
    routes['GET /configuration-versions/current'] = {
      version: 'current',
      createdAt: '2026-10-12T13:52:47Z',
      references: [
        {
          name: redirectUris,
          kind: 'variable',
          resourceType: 'application',
          resourceName: 'Admin Console',
          field: 'redirectUris',
          list: true,
        },
      ],
    };
    routes['GET /gateways/gw-dev/variables'] = page('variables', [
      {name: redirectUris, value: '["https://a.test/cb"]'},
    ]);
    routes[`PUT /gateways/gw-dev/variables/${redirectUris}`] = {name: redirectUris};
    render(<GatewayValuesPage />);

    await userEvent.click(await screen.findByRole('button', {name: `Edit ${redirectUris}`}));
    const field = await screen.findByLabelText(redirectUris);
    expect(field).toHaveValue('https://a.test/cb');
    await userEvent.clear(field);
    await userEvent.type(field, 'https://a.test/cb{enter}https://a.test/alt');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(
        http.calls
          .filter((call: Call) => call.method === 'PUT')
          .map((call: Call) => [call.url.replace(SERVER, ''), call.data]),
      ).toEqual([
        [
          `/gateways/gw-dev/variables/${redirectUris}`,
          {value: '["https://a.test/cb","https://a.test/alt"]', description: undefined},
        ],
      ]),
    );
  });

  it('reads the type and field back from a name the server derived when nothing refers to it', async () => {
    const derived = 'APPLICATION__019000000000700080000000000000F1__CLIENT_ID';
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev'};
    routes['GET /gateways/gw-dev/variables'] = page('variables', [
      {name: derived, value: 'ADMIN_CONSOLE'},
      {name: 'LEFTOVER', value: 'x'},
    ]);
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    expect(await within(list).findByText('Application · Client ID')).toBeInTheDocument();
    expect(within(list).getByText(derived)).toBeInTheDocument();
    expect(within(list).getByText('LEFTOVER')).toBeInTheDocument();
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
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

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

    await userEvent.click(await screen.findByRole('button', {name: 'Remove LEFTOVER'}));
    await waitFor(() => expect(http.calls.some((call: Call) => call.method === 'DELETE')).toBe(true));
    // The removal reads the page and the count again, rather than keeping what it showed.
    const pageReads = (): number => reads('variables').filter((query) => query.limit === '10').length;
    await waitFor(() => expect(pageReads()).toBe(2));
    await userEvent.click(screen.getByText('Deploy'));
    expect(navigate).toHaveBeenCalledWith('/deploy');
  });

  it('offers no edit while a removal is pending, so a save cannot be deleted by it', async () => {
    const answered = fakeHttp(routes);
    let finishRemoval: (() => void) | undefined;
    http = {
      calls: answered.calls,
      request: (call: Call) => {
        if (call.method !== 'DELETE') return answered.request(call);
        answered.calls.push(call);
        return new Promise((resolve) => {
          finishRemoval = () => resolve({data: {}});
        });
      },
    };
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    await userEvent.click(await within(list).findByRole('button', {name: 'Remove LEFTOVER'}));
    await waitFor(() => expect(within(list).getByRole('button', {name: 'Edit LEFTOVER'})).toBeDisabled());

    finishRemoval?.();
    await waitFor(() => expect(within(list).getByRole('button', {name: 'Edit LEFTOVER'})).toBeEnabled());
  });

  it('offers no way to set or remove a value to an admin who may only view them', async () => {
    render(
      <CapabilitiesProvider capabilities={{can: (_resource, action = 'view') => action === 'view', isLoading: false}}>
        <GatewayValuesPage />
      </CapabilitiesProvider>,
    );

    expect(await screen.findByText('LEFTOVER')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Remove LEFTOVER'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Set LEFTOVER'})).not.toBeInTheDocument();
  });

  it('says when what the running version refers to could not be read, rather than showing nothing missing', async () => {
    routes[`GET /configuration-versions/${hash}`] = new Error('down');
    render(<GatewayValuesPage />);

    expect(
      await screen.findByText('Which values the version Dev runs refers to could not be read.'),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('values-variable')).not.toBeInTheDocument();
    expect(screen.queryByTestId('values-list-variable')).not.toBeInTheDocument();
  });

  it('sets a missing value in the same dialog as a held one, and keeps no draft once it is cancelled', async () => {
    render(<GatewayValuesPage />);

    const missing = await screen.findByTestId('value-row-APP_ORDERS_REDIRECT_URIS');
    await userEvent.click(within(missing).getByRole('button', {name: 'Set APP_ORDERS_REDIRECT_URIS'}));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Set APP_ORDERS_REDIRECT_URIS')).toBeInTheDocument();
    await userEvent.type(within(dialog).getByLabelText('APP_ORDERS_REDIRECT_URIS'), 'https://a.test/cb');
    await userEvent.click(within(dialog).getByRole('button', {name: 'Cancel'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await userEvent.click(within(missing).getByRole('button', {name: 'Set APP_ORDERS_REDIRECT_URIS'}));
    expect(await screen.findByLabelText('APP_ORDERS_REDIRECT_URIS')).toHaveValue('');
    expect(http.calls.some((call: Call) => call.method === 'PUT')).toBe(false);
  });

  it('lists a held value only a hidden resource uses as no more than that, without its name or value', async () => {
    // A name the server derived, which spells the hidden resource's type and ID.
    const hiddenID = '0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b7';
    const hiddenName = 'APPLICATION__0198F3A17C2E7B109A445D3E8F21C6B7__CLIENT_ID';
    routes[`GET /configuration-versions/${hash}`] = {
      version: hash,
      createdAt: '2026-10-12T13:52:47Z',
      references: [
        {
          name: hiddenName,
          kind: 'variable',
          resourceType: 'application',
          resourceId: hiddenID,
          resourceName: 'Admin Console',
          field: 'clientId',
        },
      ],
    };
    routes['GET /gateways/gw-dev/variables'] = page('variables', [
      {name: hiddenName, value: 'admin-console-client'},
      {name: 'LEFTOVER', value: 'x'},
    ]);
    http = fakeHttp(routes);
    render(
      <HiddenResourcesProvider hidden={{ids: {application: [hiddenID]}}}>
        <GatewayValuesPage />
      </HiddenResourcesProvider>,
    );

    const list = await screen.findByTestId('values-list-variable');
    expect(await within(list).findByText('Used by a resource not shown here')).toBeInTheDocument();
    expect(within(list).queryByText(hiddenName)).not.toBeInTheDocument();
    expect(within(list).queryByText(/Application · Client ID/)).not.toBeInTheDocument();
    expect(within(list).queryByText(/Client ID/)).not.toBeInTheDocument();
    expect(within(list).queryByText('admin-console-client')).not.toBeInTheDocument();
    expect(within(list).queryByText(/Admin Console/)).not.toBeInTheDocument();
    expect(within(list).queryByText(/0198F3A1|0198f3a1/i)).not.toBeInTheDocument();
    expect(within(list).queryByRole('button', {name: new RegExp(hiddenName)})).not.toBeInTheDocument();
    expect(within(list).getByRole('button', {name: 'Edit LEFTOVER'})).toBeInTheDocument();
    expect(within(list).getByText(/1.2 of 2/)).toBeInTheDocument();
  });

  it('leaves out the values only a hidden resource uses, and the hidden resource from a shared value', async () => {
    routes[`GET /configuration-versions/${hash}`] = {
      version: hash,
      createdAt: '2026-10-12T13:52:47Z',
      references: [
        {
          name: 'ADMIN_CLIENT_ID',
          kind: 'variable',
          resourceType: 'application',
          resourceId: 'app-admin',
          resourceName: 'Admin Console',
          field: 'clientId',
        },
        {
          name: 'SHARED_URL',
          kind: 'variable',
          resourceType: 'application',
          resourceId: 'app-admin',
          resourceName: 'Admin Console',
          field: 'redirectUris',
        },
        {
          name: 'SHARED_URL',
          kind: 'variable',
          resourceType: 'application',
          resourceId: 'app-orders',
          resourceName: 'Orders',
          field: 'redirectUris',
        },
      ],
    };
    http = fakeHttp(routes);
    render(
      <HiddenResourcesProvider hidden={{ids: {application: ['app-admin']}}}>
        <GatewayValuesPage />
      </HiddenResourcesProvider>,
    );

    expect(await screen.findByText('SHARED_URL')).toBeInTheDocument();
    expect(screen.getByText(/Orders/)).toBeInTheDocument();
    expect(screen.queryByText('ADMIN_CLIENT_ID')).not.toBeInTheDocument();
    expect(screen.queryByText(/Admin Console/)).not.toBeInTheDocument();
  });

  it('pages through what the gateway holds on the server, asking for each page by limit and offset', async () => {
    routes['GET /gateways/gw-dev/variables'] = page(
      'variables',
      Array.from({length: 30}, (_, index) => ({name: `VALUE_${String(index).padStart(2, '0')}`, value: `v${index}`})),
    );
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    expect(await within(list).findByText('VALUE_00')).toBeInTheDocument();
    expect(within(list).queryByText('VALUE_10')).not.toBeInTheDocument();
    expect(within(list).getByText(/1.10 of 30/)).toBeInTheDocument();

    await userEvent.click(within(list).getByRole('button', {name: /next page/i}));
    expect(await within(list).findByText('VALUE_10')).toBeInTheDocument();
    expect(within(list).queryByText('VALUE_00')).not.toBeInTheDocument();
    expect(reads('variables')).toContainEqual({limit: '10', offset: '10'});
    // No read asks for more than one page, and none walks the whole store.
    expect(
      reads('variables')
        .filter((query) => !query.names)
        .map((query) => query.offset),
    ).not.toContain('20');
  });

  it('searches by the start of a name on the server, from the first page, once typing settles', async () => {
    routes['GET /gateways/gw-dev/variables'] = page('variables', [
      {name: 'APP_ORDERS_CLIENT_ID', value: 'orders'},
      {name: 'LEFTOVER', value: 'x'},
    ]);
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    expect(await within(list).findByText('LEFTOVER')).toBeInTheDocument();
    await userEvent.type(within(list).getByLabelText('Search variables'), 'LEFT');

    await waitFor(() => expect(within(list).queryByText('orders')).not.toBeInTheDocument());
    expect(within(list).getByText('LEFTOVER')).toBeInTheDocument();
    const searches = reads('variables').filter((query) => query.filter);
    // The search waits for typing to settle, so it asks once rather than once a keystroke.
    expect(searches).toEqual([{limit: '10', offset: '0', filter: 'name sw "LEFT"'}]);
  });

  it('does not ask the server for a search no name could match, and says why', async () => {
    render(<GatewayValuesPage />);

    const list = await screen.findByTestId('values-list-variable');
    await within(list).findByText('LEFTOVER');
    await userEvent.type(within(list).getByLabelText('Search variables'), '1-x');

    expect(
      await within(list).findByText(
        'A name starts with a letter or an underscore and holds only letters, digits and underscores.',
      ),
    ).toBeInTheDocument();
    expect(reads('variables').some((query) => query.filter)).toBe(false);
  });
});
