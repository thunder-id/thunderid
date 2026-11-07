// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import GatewayPage from '../GatewayPage';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {
    ...actual,
    useThunderID: () => ({http: {request: (call: Call) => http.request(call)}, user: {sub: 'u-me', username: 'me'}}),
  };
});
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});
let gatewayId = 'gw-dev';
const navigate = vi.fn();
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {...actual, useNavigate: () => navigate, useParams: () => ({gatewayId})};
});

const v2 = '2f2d727aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const v1 = 'aade3b4bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const versions = [
  {version: v2, createdAt: '2026-10-12T13:52:47Z', number: 2},
  {version: v1, createdAt: '2026-10-11T02:45:55Z', number: 1},
];
const dev = {id: 'gw-dev', name: 'dev-gateway', baseUrl: 'https://dev.test', isDefault: true};
const prod = {id: 'gw-prod', name: 'prod-gateway', baseUrl: 'https://prod.test'};
const page = (kind: string, values: unknown[]) => ({totalResults: values.length, [kind]: values});

let routes: Record<string, unknown>;

beforeEach(() => {
  gatewayId = 'gw-dev';
  routes = {
    'GET /gateways/pipeline': {
      stages: [
        {name: 'Development', adminConsoleUrl: 'https://dev.test/console', gateway: dev},
        {name: 'Production', adminConsoleUrl: 'https://prod.test/console', gateway: prod},
      ],
    },
    'GET /configuration-versions': versions,
    [`GET /configuration-versions/${v1}`]: {
      ...versions[1],
      references: [{name: 'APP_SECRET', kind: 'secret'}],
    },
    [`GET /configuration-versions/${v2}`]: {...versions[0], references: []},
    'GET /gateways/gw-dev/applied-version': {
      gatewayId: 'gw-dev',
      appliedVersion: v1,
      previousVersion: v1,
      appliedAt: '2026-10-12T14:00:00Z',
    },
    'GET /gateways/gw-prod/applied-version': {gatewayId: 'gw-prod', appliedVersion: v1},
    'GET /gateways/gw-dev/applied-configuration': {
      gatewayId: 'gw-dev',
      resources: [{resourceType: 'application', id: 'a-1', resource: {id: 'a-1', name: 'Orders'}}],
    },
    'GET /gateways/gw-prod/applied-configuration': {gatewayId: 'gw-prod', resources: []},
    'GET /gateways/gw-dev/master': {
      gatewayId: 'gw-dev',
      number: 2,
      summary: {deployed: 1, failed: 1, removed: 0},
      entries: [
        {key: 'application/a-1', resourceType: 'application', name: 'Orders', sourceVersion: v1, state: 'deployed'},
        {
          key: 'flow/f-1',
          resourceType: 'flow',
          name: 'Sign in',
          sourceVersion: v1,
          state: 'failed',
          failure: {message: 'Missing executor'},
        },
      ],
    },
    'GET /gateways/gw-dev/master/history': [
      {
        gatewayId: 'gw-dev',
        number: 2,
        master: 'm2',
        createdAt: '2026-10-12T14:00:00Z',
        summary: {deployed: 1, failed: 1, removed: 0},
      },
    ],
    'GET /gateways/gw-prod/master': {
      gatewayId: 'gw-prod',
      number: 0,
      summary: {deployed: 0, failed: 0, removed: 0},
      entries: [],
    },
    'GET /gateways/gw-prod/master/history': [],
    'GET /gateways/gw-dev/variables': page('variables', [{name: 'APP_ID', value: 'orders'}]),
    'GET /gateways/gw-dev/secrets': page('secrets', []),
    'GET /gateways/gw-prod/variables': page('variables', []),
    'GET /gateways/gw-prod/secrets': page('secrets', []),
    'GET /server-config/gateway': {readOnly: null, writable: null, merged: {autoApply: false}},
    'GET /users': {
      totalResults: 13,
      users: [
        {id: 'u-1', display: 'Jerad', attributes: {email: 'jerad@example.com'}},
        {id: 'u-2', display: 'Ama Kofi', attributes: {email: 'ama@example.com'}},
        {id: 'u-3', display: 'Sam Rivera', attributes: {email: 'sam@example.com'}},
      ],
    },
    // What the latest version would change on Development, resource by resource.
    'POST /gateways/gw-dev/apply': {
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        toVersion: v2,
        changes: [
          {key: 'application/a-1', resourceType: 'application', id: 'a-1', name: 'Orders', change: 'updated'},
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign in', change: 'updated', failed: true},
          {key: 'application/a-2', resourceType: 'application', id: 'a-2', name: 'Billing', change: 'added'},
        ],
      },
    },
    'POST /gateways/gw-prod/apply': {gatewayId: 'gw-prod', dryRun: true, diff: {toVersion: v1, changes: []}},
  };
  http = fakeHttp(routes);
});

describe('GatewayPage', () => {
  it('shows what the gateway runs, resource by resource, against the configuration', async () => {
    render(<GatewayPage />);

    expect(await screen.findByRole('heading', {name: 'Development gateway'})).toBeInTheDocument();
    expect(screen.getByTestId('gateway-tag')).toHaveTextContent('DEVELOPMENT · v1');
    const endpoints = screen.getByTestId('gateway-endpoints');
    expect(within(endpoints).getByText('Issuer')).toBeInTheDocument();
    expect(within(endpoints).getByText('https://dev.test/.well-known/openid-configuration')).toBeInTheDocument();
    expect(screen.queryByText('Open the admin portal')).not.toBeInTheDocument();

    expect(await screen.findByText('Compared with configuration v2')).toBeInTheDocument();
    const orders = await screen.findByTestId('gateway-resource-application/a-1');
    expect(orders).toHaveTextContent('Orders');
    expect(orders).toHaveTextContent('v1');
    expect(orders).toHaveTextContent('Newer version not deployed');
    expect(within(orders).getByRole('link', {name: 'Edit configuration'})).toHaveAttribute('href', '/applications/a-1');
    const flow = await screen.findByTestId('gateway-resource-flow/f-1');
    expect(flow).toHaveTextContent('Refused');
    expect(flow).toHaveTextContent('Missing executor');
    const billing = screen.getByTestId('gateway-resource-application/a-2');
    expect(billing).toHaveTextContent('Not running');
    expect(billing).toHaveTextContent('Not deployed yet');
    await userEvent.click(within(billing).getByRole('button', {name: 'Deploy'}));
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
  });

  it("opens the gateway's admin portal, and lists the administrators who sign in to it", async () => {
    render(<GatewayPage />);

    const portal = await screen.findByTestId('gateway-admin-portal');
    expect(within(portal).getByRole('heading', {name: 'DEVELOPMENT admin portal'})).toBeInTheDocument();
    expect(portal).toHaveTextContent('dev.test');
    expect(portal).toHaveTextContent(
      'Manage the resources that live in this gateway, such as its users, groups, agents and roles.',
    );
    for (const kind of ['Users', 'Groups', 'Agents', 'Roles'])
      expect(within(portal).getByText(kind)).toBeInTheDocument();
    const open = within(portal).getByRole('link', {name: 'Open admin portal (opens in a new tab)'});
    expect(open).toHaveAttribute('href', 'https://dev.test/console');
    expect(open).toHaveAttribute('target', '_blank');
    expect(open).toHaveAttribute('rel', 'noopener noreferrer');

    const administrators = screen.getByTestId('gateway-administrators');
    await waitFor(() => expect(administrators).toHaveTextContent('Administrators 13'));
    expect(administrators).toHaveTextContent('They can sign in to the DEVELOPMENT admin portal');
    expect(within(administrators).getByRole('link', {name: 'View all'})).toHaveAttribute('href', '/users');
    const jerad = within(administrators).getByTestId('gateway-administrator-u-1');
    expect(jerad).toHaveTextContent('Jerad');
    expect(jerad).toHaveTextContent('jerad@example.com');
    expect(jerad).toHaveTextContent('Admin');
    expect(within(jerad).getByRole('link', {name: 'View details for Jerad'})).toHaveAttribute('href', '/users/u-1');
    expect(within(administrators).getAllByRole('listitem')).toHaveLength(3);
    expect(http.calls).toContainEqual(
      expect.objectContaining({method: 'GET', url: `${SERVER}/users?limit=3&offset=0&include=display`}),
    );
  });

  it('leaves the administration out of the other sections', async () => {
    render(<GatewayPage section="endpoints" />);

    expect(await screen.findByTestId('gateway-every-endpoint')).toBeInTheDocument();
    expect(screen.queryByTestId('gateway-administration')).not.toBeInTheDocument();
  });

  it('says how the gateway stands, and offers the deploy that brings it up to date', async () => {
    render(<GatewayPage />);

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() =>
      expect(status).toHaveTextContent("You're viewing the DEVELOPMENT gateway. It runs v1, deployed"),
    );
    await waitFor(() => expect(status).toHaveTextContent('Configuration has 3 changes not deployed here.'));
    expect(within(status).queryByRole('switch')).not.toBeInTheDocument();
    await userEvent.click(within(status).getByRole('button', {name: 'Review and deploy'}));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
  });

  it('has nothing to promote to a later gateway that runs what the one before runs', async () => {
    gatewayId = 'gw-prod';
    render(<GatewayPage />);

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() => expect(status).toHaveTextContent('Everything in the configuration is deployed here.'));
    expect(within(status).queryByRole('button', {name: 'Promote'})).not.toBeInTheDocument();
  });

  it('lists the recent deployments, with a rollback on the latest', async () => {
    render(<GatewayPage />);

    const recent = await screen.findByTestId('gateway-recent');
    expect(await within(recent).findByText('Deploy 2')).toBeInTheDocument();
    expect(within(recent).getByRole('button', {name: 'Roll back to v1'})).toBeInTheDocument();
  });

  it('lists every deployment on the history page, each opened to what it wrote', async () => {
    render(<GatewayPage section="history" />);

    expect(await screen.findByRole('heading', {name: 'Deployment history'})).toBeInTheDocument();
    const history = await screen.findByTestId('gateway-full-history');
    await userEvent.click(await within(history).findByText('Deploy 2'));
    expect(await within(history).findByText('1 deployed, 1 refused, 0 removed')).toBeInTheDocument();
  });

  it('lists every endpoint on the endpoints page', async () => {
    render(<GatewayPage section="endpoints" />);

    const endpoints = await screen.findByTestId('gateway-every-endpoint');
    expect(within(endpoints).getByText('https://dev.test/oauth2/token')).toBeInTheDocument();
  });

  it("sets the gateway's values resource by resource, the others on request", async () => {
    routes[`GET /configuration-versions/${v1}`] = {
      ...versions[1],
      references: [{name: 'APP_SECRET', kind: 'secret', resourceType: 'application', resourceId: 'a-1'}],
    };
    render(<GatewayPage section="values" />);

    expect(await screen.findByRole('heading', {name: 'Variables & secrets'})).toBeInTheDocument();
    expect(await screen.findByText('Not set')).toBeInTheDocument();
    expect(screen.queryByTestId('gateway-other-variable')).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId('gateway-show-all-values'));
    expect(await screen.findByTestId('gateway-other-variable')).toBeInTheDocument();
  });

  it("speaks of the project's own resources and values within a project", async () => {
    routes['POST /gateways/gw-dev/apply'] = {
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        toVersion: v2,
        changes: [
          {
            key: 'application/a-1',
            resourceType: 'application',
            id: 'a-1',
            name: 'Orders',
            change: 'updated',
            section: 'project',
          },
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign in', change: 'unchanged', section: 'needed'},
          {
            key: 'application/b-1',
            resourceType: 'application',
            id: 'b-1',
            name: 'Bills',
            change: 'added',
            section: 'other',
          },
        ],
      },
    };
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v1, number: 1, resources: 1}], pending: 1, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      autoSync: {enabled: false, inherited: true},
      deployedAt: '2026-10-12T14:00:00Z',
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() => expect(status).toHaveTextContent('Configuration has 1 change not deployed here.'));
    expect(await screen.findByTestId('gateway-resource-application/a-1')).toBeInTheDocument();
    // What the project needs is listed apart, edited in the organization's settings, and another
    // project's resources not at all. In sync, it shows once every resource is.
    await userEvent.click(screen.getByTestId('gateway-filter-all'));
    await userEvent.click(within(screen.getByTestId('gateway-group-shared')).getByText('Shared'));
    await userEvent.click(within(screen.getByTestId('gateway-group-flows')).getByText('Flows · 1'));
    const flow = screen.getByTestId('gateway-resource-flow/f-1');
    expect(flow).toHaveTextContent('In sync');
    expect(within(flow).getByRole('link', {name: 'Edit in organization settings'})).toHaveAttribute(
      'href',
      '/flows/f-1',
    );
    expect(screen.queryByTestId('gateway-resource-application/b-1')).not.toBeInTheDocument();
  });

  it("reads when and by whom, and the rollback, from inside the project's status as the control plane answers", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        deployedAt: new Date(Date.now() - 2 * 3600_000).toISOString(),
        deployedBy: {id: 'u-me'},
        rollback: {available: true, toVersion: {hash: v1, number: 1}},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() => expect(status).toHaveTextContent('It runs v2, deployed 2 hours ago.'));
    expect(await screen.findByTestId('gateway-rollback')).toBeEnabled();
  });

  it("offers to undo the project's last deploy when the rollback stays on the version it runs", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        rollback: {available: true, toVersion: {hash: v2, number: 2}, changes: 2, sameVersion: true},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };
    routes['POST /gateways/gw-dev/revert'] = {gatewayId: 'gw-dev', dryRun: true, diff: {changes: []}};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const undo = await screen.findByTestId('gateway-rollback');
    expect(undo).toHaveTextContent('Undo last deploy');
    await userEvent.click(undo);
    expect(await screen.findByText('Undo the last deploy to Development')).toBeInTheDocument();
  });

  it("brings one of the project's resources on its own, with what it needs locked and the others tickable", async () => {
    routes['POST /gateways/gw-dev/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {
            gatewayId: 'gw-dev',
            dryRun: true,
            diff: {
              toVersion: v2,
              changes: [
                {
                  key: 'application/a-1',
                  resourceType: 'application',
                  id: 'a-1',
                  name: 'Orders',
                  change: 'updated',
                  section: 'project',
                  needs: ['flow/f-1'],
                },
                {
                  key: 'application/a-2',
                  resourceType: 'application',
                  id: 'a-2',
                  name: 'Billing',
                  change: 'added',
                  section: 'project',
                },
                {
                  key: 'flow/f-1',
                  resourceType: 'flow',
                  id: 'f-1',
                  name: 'Sign in',
                  change: 'added',
                  section: 'needed',
                  required: true,
                },
                {
                  key: 'flow/f-2',
                  resourceType: 'flow',
                  id: 'f-2',
                  name: 'Recovery',
                  change: 'added',
                  section: 'needed',
                  required: true,
                },
              ],
            },
          }
        : {gatewayId: 'gw-dev', dryRun: false, recorded: true, diff: {changes: []}};
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v1, number: 1, resources: 1}], pending: 2, required: 2},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const orders = await screen.findByTestId('gateway-resource-application/a-1');
    await userEvent.click(within(orders).getByTestId('gateway-resource-deploy'));
    const panel = await screen.findByTestId('deploy-apply-panel');
    const own = await within(panel).findByRole('checkbox', {name: 'Write Orders'});
    expect(own).toBeChecked();
    expect(own).toBeEnabled();
    const other = within(panel).getByRole('checkbox', {name: 'Write Billing'});
    expect(other).not.toBeChecked();
    expect(other).toBeEnabled();
    // What Orders needs goes with it, locked; what only Billing would need does not.
    expect(within(panel).getByRole('checkbox', {name: 'Write Sign in'})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: 'Write Sign in'})).toBeDisabled();
    expect(within(panel).getByRole('checkbox', {name: 'Write Recovery'})).not.toBeChecked();

    await userEvent.click(within(panel).getByRole('button', {name: /Deploy 2 changes to Development/}));
    await waitFor(() =>
      expect(http.calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          url: `${SERVER}/gateways/gw-dev/apply`,
          data: {version: v2, selection: ['application/a-1', 'flow/f-1'], withDependencies: false, project: 'p-shop'},
        }),
      ),
    );
  });

  it("names the versions a project's own resources came from, without a time when none is recorded", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [
          {hash: v2, number: 2, resources: 2},
          {hash: v1, number: 1, resources: 1},
        ],
        pending: 0,
        required: 0,
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() =>
      expect(status).toHaveTextContent("You're viewing the DEVELOPMENT gateway. It runs v2, partly v1. "),
    );
  });

  it('says auto-sync keeps the first gateway on the latest configuration', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v2, number: 2, resources: 1}], pending: 0, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      autoSync: {enabled: true, inherited: false},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewayPage />
      </ProjectScopeProvider>,
    );

    const status = await screen.findByTestId('gateway-status');
    await waitFor(() =>
      expect(status).toHaveTextContent('Auto-sync is on: it runs v2, the latest saved configuration.'),
    );
  });

  it('says so when no gateway runs on the gateway', async () => {
    gatewayId = 'gw-none';
    render(<GatewayPage />);

    expect(await screen.findByText('No gateway runs on this gateway.')).toBeInTheDocument();
  });
});
