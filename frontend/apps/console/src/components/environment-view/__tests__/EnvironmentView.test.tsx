// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import {renderWithProviders, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {IconButton, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import EnvironmentView from '../EnvironmentView';

interface Call {
  url: string;
  method: string;
  data?: unknown;
}

const SERVER = 'https://localhost:8090';
const request = vi.fn<(call: Call) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  const React = await import('react');
  // The console's client, as the pages read through it; the view puts its own answers in front of it.
  // The request is reached when it is made, as the stand-in is not yet made when this module is.
  const ThunderIDContext = React.createContext<unknown>({http: {request: (call: Call) => request(call)}});
  return {...actual, ThunderIDContext, useThunderID: () => React.useContext(ThunderIDContext)};
});

const select = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER}),
    useEnvironment: () => ({environments: [], select}),
  };
});

let path = '/home';
const navigate = vi.fn();
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {
    ...actual,
    useNavigate: () => navigate,
    useLocation: () => ({...actual.useLocation(), pathname: path}),
  };
});

const environment = {id: 'gw-2', name: 'Prod', baseUrl: 'https://prod.example.com'};

const configuration = {
  gatewayId: 'gw-2',
  version: 'abcdef0123456789',
  appliedAt: '2026-10-09T10:00:00Z',
  resources: [
    {
      resourceType: 'application',
      id: 'app-1',
      resource: {
        id: 'app-1',
        name: 'Orders',
        description: 'Order service',
        ouId: 'ou-1',
        isRegistrationFlowEnabled: false,
        url: '',
        customNote: 'Kept for audits',
        inboundAuthConfig: [
          {
            type: 'oauth2',
            config: {
              clientId: 'var:APP_ORDERS_CLIENT_ID',
              clientSecret: 'sec:APP_ORDERS_CLIENT_SECRET',
              redirectUris: ['var:APP_ORDERS_REDIRECT_URIS'],
              grantTypes: ['authorization_code', 'refresh_token'],
            },
          },
        ],
      },
    },
    {resourceType: 'organization_unit', id: 'ou-1', resource: {id: 'ou-1', name: 'Sales', handle: 'sales'}},
    {resourceType: 'application', id: 'app-2', resource: {id: 'app-2', name: 'Billing'}},
    {resourceType: 'connection', id: 'c-1', resource: {id: 'c-1', name: 'Google', type: 'google'}},
    {
      resourceType: 'server_config',
      id: 'cors',
      resource: {
        readOnly: {},
        writable: {allowedOrigins: ['https://app.test']},
        merged: {allowedOrigins: ['https://app.test']},
      },
    },
    {
      resourceType: 'group',
      id: 'g-1',
      resource: {id: 'g-1', name: 'Admins'},
      parts: {members: {totalResults: 1, members: [{id: 'u-1', type: 'user'}]}},
    },
    {
      resourceType: 'user_type',
      id: 'ut-1',
      resource: {
        id: 'ut-1',
        handle: 'person',
        displayName: 'Person',
        schema: {
          email: {type: 'string', required: true, unique: true, displayName: 'Email'},
          address: {type: 'object', properties: {city: {type: 'string', enum: ['Colombo', 'Kandy']}}},
        },
      },
    },
    {
      resourceType: 'role',
      id: 'r-1',
      resource: {id: 'r-1', name: 'Admin', permissions: [{resourceServerId: 'rs-1', permissions: ['read', 'write']}]},
    },
    {
      resourceType: 'theme',
      id: 't-1',
      resource: {id: 't-1', displayName: 'Dark', theme: {shape: {borderRadius: 8}, defaultColorScheme: 'dark'}},
    },
    {
      resourceType: 'layout',
      id: 'l-1',
      resource: {id: 'l-1', displayName: 'Centered', layout: {screens: {login: {gap: 2}}}},
    },
    {
      resourceType: 'translation',
      id: 'fr',
      resource: {language: 'fr', translations: {signin: {title: 'Connexion'}, auth: {hello: 'Bonjour'}}},
    },
    {
      resourceType: 'flow',
      id: 'f-1',
      resource: {
        id: 'f-1',
        name: 'Sign in',
        flowType: 'AUTHENTICATION',
        nodes: [
          {id: 'start', type: 'START'},
          {id: 'prompt', type: 'PROMPT'},
        ],
      },
    },
  ],
  skipped: [{resourceType: 'mcp', id: 'm-1', code: 'GTW-1021', reason: 'Cannot be shown'}],
};

const page = (kind: string, values: Record<string, unknown>[]) => ({
  totalResults: values.length,
  [kind]: values,
});

let routes: Record<string, unknown>;

beforeEach(() => {
  path = '/home';
  select.mockReset();
  navigate.mockReset();
  routes = {
    'GET /gateways/gw-2/applied-configuration': configuration,
    'GET /gateways/gw-2/variables': page('variables', [
      {name: 'APP_ORDERS_REDIRECT_URIS', value: '["https://prod.example.com/cb","https://prod.example.com/alt"]'},
    ]),
    'GET /gateways/gw-2/secrets': page('secrets', [{name: 'APP_ORDERS_CLIENT_SECRET', exists: true}]),
    'GET /connections/meta': 'from the server',
  };
  request.mockReset();
  request.mockImplementation((call: Call) => {
    const [route] = call.url.replace(SERVER, '').split('?');
    const answer = routes[`${call.method} ${route}`];
    if (answer === undefined) return Promise.reject(new Error(`Unexpected ${call.method} ${route}`));
    return Promise.resolve({data: answer});
  });
});

/** Stands for a page of the configuration: it reads through the console's client and offers to change. */
function ConfigurationPage(): JSX.Element {
  const {http} = useThunderID();
  const {data} = useQuery({
    queryKey: ['applications'],
    queryFn: async () => {
      const applications = (await http.request({url: `${SERVER}/applications?limit=30&offset=0`, method: 'GET'})) as {
        data: {applications: {name: string}[]};
      };
      const meta = (await http.request({url: `${SERVER}/connections/meta`, method: 'GET'})) as {data: string};
      return {names: applications.data.applications.map((app) => app.name).join(', '), meta: meta.data};
    },
  });
  return (
    <div>
      <span>configuration page</span>
      <PageTitle>
        <PageTitle.Header>Applications</PageTitle.Header>
        <PageTitle.Actions>
          <button type="button" data-testid="create">
            Create
          </button>
        </PageTitle.Actions>
      </PageTitle>
      <span data-testid="served">{data?.names}</span>
      <span data-testid="server-answer">{data?.meta}</span>
      <IconButton color="error" data-testid="delete" />
      <IconButton data-testid="view" />
    </div>
  );
}

const show = (at: string) => {
  path = at;
  return renderWithProviders(
    <EnvironmentView environment={environment}>
      <ConfigurationPage />
    </EnvironmentView>,
  );
};

describe('EnvironmentView', () => {
  it('shows a resource in the tabs its configuration page has, with the values the environment fills', async () => {
    show('/applications/app-1');

    expect(await screen.findByRole('heading', {name: 'Orders', level: 3})).toBeInTheDocument();
    expect(screen.getByText('Order service')).toBeInTheDocument();
    expect((await screen.findAllByRole('tab')).map((tab) => tab.textContent)).toEqual([
      'Overview',
      'Credentials',
      'Flows',
      'Advanced',
      'Other',
    ]);
    const overview = screen.getByRole('tabpanel', {name: 'Overview'});
    expect(within(overview).getByText('Application details')).toBeInTheDocument();
    expect(within(overview).getByText('Application ID')).toBeInTheDocument();
    expect(within(overview).getByRole('link', {name: 'Sales'})).toHaveAttribute('href', '/organization-units/ou-1');
    expect(within(overview).getByText('APP_ORDERS_CLIENT_ID')).toBeInTheDocument();
    expect(within(overview).getByText('Not set')).toBeInTheDocument();
    // Identifiers can be copied, as on the configuration's page; nothing else offers to.
    expect(within(overview).getByRole('button', {name: 'Copy Application ID'})).toBeInTheDocument();
    // The environment's own endpoints, which a developer copies from here.
    expect(within(overview).getByText('Useful Endpoints')).toBeInTheDocument();
    expect(within(overview).getByText('https://prod.example.com/oauth2/token')).toBeInTheDocument();
    expect(within(overview).getByRole('button', {name: 'Copy Token'})).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('tab', {name: 'Credentials'}));
    const credentials = screen.getByRole('tabpanel', {name: 'Credentials'});
    expect(within(credentials).getByText('Set, not shown')).toBeInTheDocument();
    expect(within(credentials).getByText('APP_ORDERS_CLIENT_SECRET')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('tab', {name: 'Flows'}));
    expect(within(screen.getByRole('tabpanel', {name: 'Flows'})).getByText('Sign-up enabled')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('tab', {name: 'Advanced'}));
    const advanced = screen.getByRole('tabpanel', {name: 'Advanced'});
    expect(within(advanced).getByText('OAuth 2 Configuration')).toBeInTheDocument();
    expect(within(advanced).getByText('Authorized redirect URIs')).toBeInTheDocument();
    expect(await within(advanced).findByText('https://prod.example.com/cb')).toBeInTheDocument();
    expect(within(advanced).getByText('https://prod.example.com/alt')).toBeInTheDocument();
    // Short plain values read as tags, as the configuration's page shows grant types.
    expect(within(advanced).getByText('authorization_code')).toHaveClass('MuiChip-label');
    expect(within(advanced).queryByRole('button', {name: /^Copy /})).not.toBeInTheDocument();

    // A field no tab of the configuration's page names is shown all the same.
    await userEvent.click(screen.getByRole('tab', {name: 'Other'}));
    expect(within(screen.getByRole('tabpanel', {name: 'Other'})).getByText('Kept for audits')).toBeInTheDocument();
  });

  it('shows a theme in the sections its builder has', async () => {
    show('/design/t-1');

    expect((await screen.findAllByRole('tab')).map((tab) => tab.textContent)).toEqual(['Shape', 'Settings']);
    expect(await screen.findByRole('tabpanel', {name: 'Shape'})).toHaveTextContent('8');
    await userEvent.click(screen.getByRole('tab', {name: 'Settings'}));
    expect(screen.getByText('Default Color Scheme')).toBeInTheDocument();
  });

  it('shows a layout in the sections its builder has', async () => {
    show('/design/l-1');
    expect(await screen.findByRole('tabpanel', {name: 'Screens'})).toHaveTextContent('Login');
  });

  it('shows a schema as the schema editor lays it out', async () => {
    show('/user-types/ut-1');

    await userEvent.click(await screen.findByRole('tab', {name: 'Schema'}));
    const table = within(screen.getByRole('tabpanel', {name: 'Schema'}));
    expect(table.getByRole('columnheader', {name: 'Property'})).toBeInTheDocument();
    expect(table.getByText('Email')).toBeInTheDocument();
    expect(table.getByText('Required')).toBeInTheDocument();
    expect(table.getByText('Unique')).toBeInTheDocument();
    expect(table.getByText('city')).toBeInTheDocument();
    expect(table.getByText('Kandy')).toBeInTheDocument();
  });

  it("shows a role's permissions with the permissions as tags", async () => {
    show('/roles/r-1');

    await userEvent.click(await screen.findByRole('tab', {name: 'Permissions'}));
    const table = within(screen.getByRole('tabpanel', {name: 'Permissions'}));
    expect(table.getByText('Resource server ID')).toBeInTheDocument();
    expect(table.getByText('rs-1')).toBeInTheDocument();
    expect(table.getByText('write')).toHaveClass('MuiChip-label');
  });

  it('shows a tab to each namespace of a translation', async () => {
    show('/translations/fr');

    expect((await screen.findAllByRole('tab')).map((tab) => tab.textContent)).toEqual(['auth', 'signin']);
    expect(await screen.findByRole('tabpanel', {name: 'auth'})).toHaveTextContent('Bonjour');
  });

  it('shows a resource whose page has no tabs by its fields', async () => {
    show('/flows/f-1');

    expect((await screen.findAllByRole('tab')).map((tab) => tab.textContent)).toEqual(['General', 'Nodes']);
    expect(await screen.findByRole('tabpanel', {name: 'General'})).toHaveTextContent('AUTHENTICATION');
    await userEvent.click(screen.getByRole('tab', {name: 'Nodes'}));
    expect(screen.getByRole('tabpanel', {name: 'Nodes'})).toHaveTextContent('PROMPT');
  });

  it('shows the parts a resource carries', async () => {
    show('/groups/g-1');

    await userEvent.click(await screen.findByRole('tab', {name: 'Members'}));
    const members = screen.getByRole('tabpanel', {name: 'Members'});
    expect(within(members).getByText('u-1')).toBeInTheDocument();
  });

  it('sets a value the environment fills a field with', async () => {
    routes['PUT /gateways/gw-2/variables/APP_ORDERS_CLIENT_ID'] = {name: 'APP_ORDERS_CLIENT_ID', value: 'orders'};
    show('/applications/app-1');
    await userEvent.click(await screen.findByRole('tab', {name: 'Credentials'}));

    await userEvent.click(await screen.findByRole('button', {name: 'Set variable APP_ORDERS_CLIENT_ID'}));
    const dialog = await screen.findByRole('dialog');
    await userEvent.type(within(dialog).getByLabelText('Value'), 'orders');
    await userEvent.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(request).toHaveBeenCalledWith(
        expect.objectContaining({
          method: 'PUT',
          url: `${SERVER}/gateways/gw-2/variables/APP_ORDERS_CLIENT_ID`,
          data: {value: 'orders', description: undefined},
        }),
      ),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('sets a list value one item per line', async () => {
    routes['PUT /gateways/gw-2/variables/APP_ORDERS_REDIRECT_URIS'] = {};
    show('/applications/app-1');
    await userEvent.click(await screen.findByRole('tab', {name: 'Advanced'}));

    await userEvent.click(await screen.findByRole('button', {name: 'Set variable APP_ORDERS_REDIRECT_URIS'}));
    const field = within(await screen.findByRole('dialog')).getByLabelText('Value');
    expect(field).toHaveValue('https://prod.example.com/cb\nhttps://prod.example.com/alt');
    await userEvent.clear(field);
    await userEvent.click(field);
    await userEvent.paste('https://a.test\n\nhttps://b.test');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(request).toHaveBeenCalledWith(
        expect.objectContaining({method: 'PUT', data: {value: '["https://a.test","https://b.test"]'}}),
      ),
    );
  });

  it('sets a secret without reading it back', async () => {
    show('/applications/app-1');
    await userEvent.click(await screen.findByRole('tab', {name: 'Credentials'}));

    await userEvent.click(await screen.findByRole('button', {name: 'Set secret APP_ORDERS_CLIENT_SECRET'}));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('Value')).toHaveValue('');
    expect(within(dialog).getByRole('button', {name: 'Save'})).toBeDisabled();
    await userEvent.click(within(dialog).getByRole('button', {name: 'Cancel'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('says when a value cannot be set', async () => {
    show('/applications/app-1');
    await userEvent.click(await screen.findByRole('tab', {name: 'Credentials'}));

    await userEvent.click(await screen.findByRole('button', {name: 'Set variable APP_ORDERS_CLIENT_ID'}));
    await userEvent.type(screen.getByLabelText('Value'), 'orders');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    expect(await screen.findByText('The value could not be set. Please try again.')).toBeInTheDocument();
  });

  it('shows a resource with one section without tabs', async () => {
    show('/organization-units/ou-1');

    expect(await screen.findByRole('tabpanel', {name: 'General'})).toHaveTextContent('sales');
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
  });

  it('says when the environment does not run the resource asked for', async () => {
    show('/applications/app-9');
    expect(await screen.findByText('Prod does not run this resource.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Back to Applications'})).toHaveAttribute('href', '/applications');
  });

  it('says when nothing has been deployed to the environment', async () => {
    routes['GET /gateways/gw-2/applied-configuration'] = {gatewayId: 'gw-2', resources: []};
    show('/applications');
    expect(await screen.findByText('Nothing has been deployed to Prod yet.')).toBeInTheDocument();
  });

  it('says when what the environment runs cannot be read', async () => {
    delete routes['GET /gateways/gw-2/applied-configuration'];
    show('/applications');
    expect(await screen.findByText('What Prod runs could not be read.')).toBeInTheDocument();
  });

  it('shows the home page and the lists as the configuration has them, served from the environment', async () => {
    show('/applications');

    expect(await screen.findByText('configuration page')).toBeInTheDocument();
    expect(await screen.findByText('Orders, Billing')).toBeInTheDocument();
    expect(screen.getByTestId('server-answer')).toHaveTextContent('from the server');
    // The list was answered here; only what the environment cannot answer went to the server.
    expect(request).not.toHaveBeenCalledWith(
      expect.objectContaining({url: expect.stringContaining('/applications') as string}),
    );
    // What the configuration's page offers to change it is not shown here.
    expect(screen.getByTestId('create')).not.toBeVisible();
    expect(screen.getByTestId('delete')).not.toBeVisible();
    expect(screen.getByTestId('view')).toBeVisible();
  });

  it('leads back to the configuration from what belongs to it alone, and from making things', async () => {
    show('/import-export');
    await userEvent.click(screen.getByRole('button', {name: 'Show the configuration'}));
    expect(select).toHaveBeenCalledWith(undefined);

    show('/applications/types');
    expect(screen.getAllByText('This belongs to the configuration, not to an environment.')).not.toHaveLength(0);
  });

  it('opens a connection by its type and id, and a theme below its kind', async () => {
    show('/connections/google/c-1');
    expect(await screen.findByRole('heading', {name: 'Google', level: 3})).toBeInTheDocument();

    show('/design/themes/t-1');
    expect(await screen.findByRole('heading', {name: 'Dark', level: 3})).toBeInTheDocument();
  });

  it('shows the settings the environment runs', async () => {
    show('/settings');
    expect(await screen.findByRole('tabpanel', {name: 'Settings'})).toHaveTextContent('https://app.test');
  });

  it('shows a section that is not configuration as it always is', async () => {
    show('/gateways');
    expect(screen.getByText('configuration page')).toBeInTheDocument();
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith(expect.objectContaining({url: `${SERVER}/applications?limit=30&offset=0`})),
    );
    expect(screen.getByTestId('create')).toBeVisible();
  });
});
