// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {focusManager} from '@tanstack/react-query';
import {act, render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import DeployPage from '../DeployPage';

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
const navigate = vi.fn();
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {...actual, useNavigate: () => navigate};
});

const versions = [
  {
    version: '2f2d727aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    createdAt: '2026-10-12T13:52:47Z',
    current: true,
    number: 6,
    note: 'Captured automatically after a change',
    createdBy: {id: 'u-1', name: 'alice'},
  },
  {
    version: 'aade3b4bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    createdAt: '2026-10-11T02:45:55Z',
    number: 5,
    createdBy: {id: 'u-me'},
  },
];
const gateways = [
  {id: 'gw-dev', name: 'Dev', baseUrl: 'https://localhost:8090', isDefault: true},
  {id: 'gw-prod', name: 'Prod', baseUrl: 'https://localhost:8097'},
];
const page = (kind: string, values: unknown[]) => ({totalResults: values.length, [kind]: values});

let routes: Record<string, unknown>;

beforeEach(() => {
  navigate.mockReset();
  routes = {
    'GET /gateways': gateways,
    'GET /configuration-versions': versions,
    'GET /gateways/gw-dev/applied-version': {
      gatewayId: 'gw-dev',
      appliedVersion: versions[0].version,
      previousVersion: versions[1].version,
      appliedAt: '2026-10-12T13:55:00Z',
      appliedBy: {id: 'u-2', name: 'bob'},
    },
    'GET /gateways/gw-prod/applied-version': {
      gatewayId: 'gw-prod',
      appliedVersion: versions[1].version,
      appliedAt: '2026-10-11T02:50:46Z',
    },
    [`GET /configuration-versions/${versions[0].version}`]: {
      ...versions[0],
      resources: [
        'resource_type: application\nid: a-1',
        'resource_type: application\nid: a-2',
        'resource_type: flow\nid: f-1',
      ].join('\n---\n'),
    },
    [`GET /configuration-versions/${versions[1].version}`]: {
      ...versions[1],
      resources: 'resource_type: organization_unit\nid: ou-1',
    },
    'GET /gateways/gw-dev/variables': page('variables', [
      {name: 'A', value: '1'},
      {name: 'B', value: '2'},
    ]),
    'GET /gateways/gw-dev/secrets': page('secrets', [{name: 'S', exists: true}]),
    'GET /gateways/gw-prod/variables': page('variables', []),
    'GET /gateways/gw-prod/secrets': page('secrets', []),
  };
  http = fakeHttp(routes);
});

describe('DeployPage', () => {
  it('shows the versions with where each runs, and every gateway with what it runs', async () => {
    render(<DeployPage />);

    expect((await screen.findAllByText('Deploy')).length).toBeGreaterThan(0);
    const list = await screen.findByTestId('deploy-versions');
    const newest = within(list).getByTestId(`deploy-version-${versions[0].version}`);
    // The newest version is set apart and open: its name, where it is deployed, when it was captured
    // and for whom, and its Deploy.
    expect(newest).toHaveAttribute('data-latest', 'true');
    expect(within(newest).getByText('v6')).toBeInTheDocument();
    expect(within(newest).getByTestId('timestamp')).toBeInTheDocument();
    expect(within(newest).queryByText('Captured automatically after a change')).not.toBeInTheDocument();
    expect(await within(newest).findByTestId('version-deployed-to')).toHaveTextContent('Dev');
    // Each gateway is tagged in a colour of its own.
    expect(within(newest).getByTestId('version-deployed-to').dataset['color']).not.toBe(
      within(within(list).getByTestId(`deploy-version-${versions[1].version}`)).getByTestId('version-deployed-to')
        .dataset['color'],
    );
    expect(within(newest).getByTestId('actor-name')).toHaveTextContent('alice');
    expect(within(newest).getByRole('button', {name: 'Deploy'})).toBeInTheDocument();
    // The newest version is marked latest, whether or not it is kept up to date in place.
    expect(within(newest).getByText('Latest')).toBeInTheDocument();
    expect(within(newest).queryByText('Current')).not.toBeInTheDocument();
    // One version is open at a time: the newest to begin with, and opening another closes it.
    expect(newest).toHaveAttribute('data-open', 'true');
    const older = within(list).getByTestId(`deploy-version-${versions[1].version}`);
    expect(older).not.toHaveAttribute('data-open');
    expect(within(older).getByText('v5')).toBeInTheDocument();
    expect(within(older).getByTestId('version-deployed-to')).toHaveTextContent('Prod');
    // A closed version has no Deploy, and names who it was captured for by an avatar alone.
    expect(within(older).queryByRole('button', {name: 'Deploy'})).not.toBeInTheDocument();
    expect(within(older).queryByTestId('actor-name')).not.toBeInTheDocument();
    expect(within(older).getByLabelText('me')).toBeInTheDocument();
    await userEvent.click(within(older).getByRole('button', {name: 'Details of v5'}));
    expect(older).toHaveAttribute('data-open', 'true');
    // Recorded with no name, the signed-in user is named by the name they signed in with.
    expect(within(older).getByTestId('actor-name')).toHaveTextContent('me');
    expect(within(older).getByRole('button', {name: 'Deploy'})).toBeInTheDocument();
    const closed = within(list).getByTestId(`deploy-version-${versions[0].version}`);
    expect(closed).not.toHaveAttribute('data-open');
    expect(within(closed).queryByRole('button', {name: 'Deploy'})).not.toBeInTheDocument();
    expect(within(list).getByText('Latest')).toBeInTheDocument();
    // A click anywhere on a row opens it, not only on its chevron, and a click on it again closes it.
    await userEvent.click(within(closed).getByText('v6'));
    expect(closed).toHaveAttribute('data-open', 'true');
    expect(older).not.toHaveAttribute('data-open');
    await userEvent.click(within(closed).getByText('v6'));
    expect(closed).not.toHaveAttribute('data-open');

    const dev = screen.getByTestId('deploy-stage-gw-dev');
    expect(within(dev).getByText('https://localhost:8090')).toBeInTheDocument();
    expect(within(dev).getByRole('button', {name: 'Copy the URL'})).toBeInTheDocument();
    expect(within(dev).getByText('v6')).toBeInTheDocument();
    // When the version was captured and when Dev came to run it are told apart.
    expect(within(dev).getAllByTestId('timestamp')).toHaveLength(2);
    expect(within(within(dev).getByTestId('stage-deployed')).getByText('Deployed')).toBeInTheDocument();
    expect(within(within(dev).getByTestId('stage-deployed')).getByTestId('timestamp')).toBeInTheDocument();
    expect(within(within(dev).getByTestId('stage-deployed')).getByTestId('actor-name')).toHaveTextContent('bob');
    expect(within(dev).queryByText(/variables/)).not.toBeInTheDocument();
    const prod = screen.getByTestId('deploy-stage-gw-prod');
    expect(within(prod).getByText('v5')).toBeInTheDocument();
    expect(within(prod).queryByText('Behind latest')).not.toBeInTheDocument();
    expect(within(prod).getByRole('button', {name: 'Rollback'})).toBeDisabled();
  });

  it('lists the newest versions and every deployed one, and folds the rest away', async () => {
    const many = Array.from({length: 9}, (_, i) => ({
      version: `${String(9 - i).repeat(7)}${'c'.repeat(57)}`,
      createdAt: `2026-10-0${9 - i}T10:00:00Z`,
      number: 9 - i,
    }));
    routes['GET /configuration-versions'] = many;
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev', appliedVersion: many[0].version};
    routes['GET /gateways/gw-prod/applied-version'] = {gatewayId: 'gw-prod', appliedVersion: many[8].version};
    http = fakeHttp(routes);
    render(<DeployPage />);

    const list = await screen.findByTestId('deploy-versions');
    await waitFor(() => expect(within(list).getByTestId(`deploy-version-${many[8].version}`)).toBeInTheDocument());
    // The five newest, and v1, which Prod runs.
    expect(within(list).queryByTestId(`deploy-version-${many[5].version}`)).not.toBeInTheDocument();
    await userEvent.click(within(list).getByRole('button', {name: 'Show 3 older'}));
    expect(within(list).getByTestId(`deploy-version-${many[5].version}`)).toBeInTheDocument();
    await userEvent.click(within(list).getByRole('button', {name: 'Show fewer'}));
    expect(within(list).queryByTestId(`deploy-version-${many[5].version}`)).not.toBeInTheDocument();
  });

  it('opens a gateway"s explorer from its card', async () => {
    render(<DeployPage />);
    await userEvent.click(await screen.findByRole('button', {name: 'Explore Prod'}));

    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-prod/explore');
    await userEvent.click(within(screen.getByTestId('deploy-stage-gw-prod')).getByRole('button', {name: 'Configure'}));
    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-prod/values');
  });

  it('deploys a version to a gateway after showing what it writes, leaving out an unticked resource', async () => {
    routes['POST /gateways/gw-prod/apply'] = (call: Call) => {
      const data = call.data as {dryRun?: boolean; selection?: string[]};
      if (data.dryRun) {
        return {
          gatewayId: 'gw-prod',
          dryRun: true,
          diff: {
            toVersion: versions[0].version,
            summary: {added: 1, updated: 1, deleted: 0, unchanged: 3},
            changes: [
              {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'added'},
              {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign in', change: 'updated'},
              {key: 'role/r-1', resourceType: 'role', id: 'r-1', name: 'Admin', change: 'unchanged'},
            ],
          },
          missing: {
            variables: ['APP_ORDERS_CLIENT_ID'],
            references: [
              {
                name: 'APP_ORDERS_CLIENT_ID',
                kind: 'variable',
                resourceType: 'application',
                resourceId: 'app-1',
                resourceName: 'Orders',
                field: 'clientId',
              },
            ],
          },
        };
      }
      return {
        gatewayId: 'gw-prod',
        dryRun: false,
        diff: {toVersion: versions[0].version, summary: {added: 1, updated: 0, deleted: 0, unchanged: 4}, changes: []},
        import: {summary: {failed: 0}},
      };
    };
    routes['PUT /gateways/gw-prod/variables/APP_ORDERS_CLIENT_ID'] = {name: 'APP_ORDERS_CLIENT_ID', value: 'orders'};
    render(<DeployPage />);

    const newest = await screen.findByTestId(`deploy-version-${versions[0].version}`);
    await userEvent.click(within(newest).getByRole('button', {name: 'Deploy'}));
    // Deploy is not a click on the row, so the version stays open.
    expect(newest).toHaveAttribute('data-open', 'true');
    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(within(panel).getByText('Deploy v6')).toBeInTheDocument();
    // The default gateway is chosen first; Prod is picked instead.
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));

    expect(await within(panel).findByText('2 resources to write')).toBeInTheDocument();
    expect(within(panel).getByText('1 resources are already as the version has them.')).toBeInTheDocument();
    // What the gateway lacks comes before what the version would write.
    expect(
      within(panel)
        .getByLabelText(/APP_ORDERS_CLIENT_ID/)
        .compareDocumentPosition(within(panel).getByText('2 resources to write')) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    const submit = within(panel).getByRole('button', {name: 'Deploy'});
    expect(submit).toBeDisabled();
    const missing = within(panel).getByTestId('missing-values');
    expect(within(missing).getByText('Values this gateway needs')).toBeInTheDocument();
    expect(within(missing).getByText('0 of 1 set')).toBeInTheDocument();
    expect(within(missing).getByText('Needed')).toBeInTheDocument();
    await userEvent.type(within(panel).getByLabelText(/APP_ORDERS_CLIENT_ID/), 'orders');
    expect(within(missing).getByText('All values set')).toBeInTheDocument();
    expect(within(missing).getByText('1 of 1 set')).toBeInTheDocument();
    expect(within(missing).queryByText('Needed')).not.toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Sign in/}));
    expect(submit).toBeEnabled();
    await userEvent.click(submit);

    await waitFor(() => expect(screen.queryByTestId('deploy-apply-panel')).not.toBeInTheDocument());
    const writes = http.calls.filter(
      (call: Call) => call.method !== 'GET' && !(call.data as {dryRun?: boolean})?.dryRun,
    );
    expect(writes.map((call: Call) => [call.method, call.url.replace(SERVER, ''), call.data])).toEqual([
      ['PUT', '/gateways/gw-prod/variables/APP_ORDERS_CLIENT_ID', {value: 'orders', description: undefined}],
      ['POST', '/gateways/gw-prod/apply', {version: versions[0].version, selection: ['application/app-1']}],
    ]);
  });

  it('offers a redeploy only when a value changed since the gateway was deployed, and deploys its version again', async () => {
    const unchanged = {
      toVersion: versions[0].version,
      summary: {added: 0, updated: 0, deleted: 0, unchanged: 3},
      changes: [{key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign in', change: 'unchanged'}],
    };
    routes['POST /gateways/gw-dev/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {gatewayId: 'gw-dev', dryRun: true, diff: unchanged, changed: {secrets: ['APP_SECRET']}}
        : {gatewayId: 'gw-dev', dryRun: false, diff: unchanged, recorded: true};
    routes['POST /gateways/gw-prod/apply'] = () => ({gatewayId: 'gw-prod', dryRun: true, diff: unchanged});
    render(<DeployPage />);

    // Prod's values are as they were deployed, so it has nothing to redeploy.
    const prod = await screen.findByTestId('deploy-stage-gw-prod');
    const prodRedeploy = await within(prod).findByTestId('stage-redeploy');
    expect(prodRedeploy).toBeDisabled();
    await userEvent.hover(prodRedeploy.parentElement!);
    expect(
      await screen.findByText('Nothing to redeploy: no value has changed since Prod was deployed.'),
    ).toBeInTheDocument();

    // A secret was set on Dev since it was deployed: its version is owed again though it did not change.
    const dev = screen.getByTestId('deploy-stage-gw-dev');
    expect(await within(dev).findByTestId('stage-changed')).toHaveTextContent(/1 values? changed since deploy/);
    const redeploy = within(dev).getByTestId('stage-redeploy');
    expect(redeploy).toBeEnabled();
    await userEvent.click(redeploy);

    const panel = await screen.findByTestId('deploy-apply-panel');
    const submit = within(panel).getByRole('button', {name: 'Deploy'});
    await waitFor(() => expect(submit).toBeEnabled());
    await userEvent.click(submit);

    await waitFor(() => expect(screen.queryByTestId('deploy-apply-panel')).not.toBeInTheDocument());
    const writes = http.calls.filter(
      (call: Call) => call.method !== 'GET' && !(call.data as {dryRun?: boolean})?.dryRun,
    );
    expect(writes.map((call: Call) => [call.method, call.url.replace(SERVER, ''), call.data])).toEqual([
      ['POST', '/gateways/gw-dev/apply', {version: versions[0].version, selection: []}],
    ]);
  });

  it('keeps what is typed and unticked when the window regains focus', async () => {
    routes['POST /gateways/gw-dev/apply'] = () => ({
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        toVersion: versions[0].version,
        summary: {added: 1, updated: 1, deleted: 0, unchanged: 0},
        changes: [
          {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'added'},
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign in', change: 'updated'},
        ],
      },
      missing: {variables: ['APP_ORDERS_CLIENT_ID']},
    });
    http = fakeHttp(routes);
    render(<DeployPage />);

    await userEvent.click(
      within(await screen.findByTestId(`deploy-version-${versions[0].version}`)).getByRole('button', {name: 'Deploy'}),
    );
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.type(await within(panel).findByLabelText(/APP_ORDERS_CLIENT_ID/), 'orders');
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Sign in/}));
    const checks = (): number => http.calls.filter((call: Call) => call.url.endsWith('/gw-dev/apply')).length;
    const before = checks();

    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });

    expect(within(panel).getByLabelText(/APP_ORDERS_CLIENT_ID/)).toHaveValue('orders');
    expect(within(panel).getByRole('checkbox', {name: /Sign in/})).not.toBeChecked();
    expect(checks()).toBe(before);
  });

  it('exits to the home page the host routes', async () => {
    render(<DeployPage />);

    await userEvent.click(await screen.findByRole('button', {name: 'Exit'}));

    expect(navigate).toHaveBeenCalledWith('/home');
  });

  it('shows what a gateway holds back, unticked, and deploys exactly what is ticked', async () => {
    const heldBack = {
      key: 'application/app-2',
      resourceType: 'application',
      id: 'app-2',
      name: 'Billing',
      change: 'updated',
      excluded: true,
    };
    routes['POST /gateways/gw-dev/apply'] = (call: Call) => ({
      gatewayId: 'gw-dev',
      dryRun: Boolean((call.data as {dryRun?: boolean}).dryRun),
      diff: {
        toVersion: versions[0].version,
        summary: {added: 0, updated: 2, deleted: 0, unchanged: 0},
        changes: [
          {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
          heldBack,
        ],
      },
    });
    http = fakeHttp(routes);
    render(<DeployPage />);

    // Dev's card says what it holds back of the version it runs.
    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    expect(await within(dev).findByTestId('stage-held-back')).toHaveTextContent('1 held back, not running');

    await userEvent.click(
      within(await screen.findByTestId(`deploy-version-${versions[0].version}`)).getByRole('button', {name: 'Deploy'}),
    );
    const panel = await screen.findByTestId('deploy-apply-panel');
    const billing = await within(panel).findByRole('checkbox', {name: /Billing/});
    // What Dev holds back starts unticked and says so; ticking it deploys it again.
    expect(billing).not.toBeChecked();
    expect(
      within(within(panel).getByTestId('change-application/app-2')).getByTestId('change-held-back'),
    ).toHaveTextContent('Held back on Dev');
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    await waitFor(() => expect(screen.queryByTestId('deploy-apply-panel')).not.toBeInTheDocument());
    const write = http.calls.find(
      (call: Call) =>
        call.method === 'POST' && call.url.endsWith('/gw-dev/apply') && !(call.data as {dryRun?: boolean}).dryRun,
    );
    // The selection is always sent, so the gateway does exactly what was ticked.
    expect(write?.data).toEqual({version: versions[0].version, selection: ['application/app-1']});
  });

  it('rolls a gateway back after showing what that changes', async () => {
    routes['POST /gateways/gw-dev/revert'] = (call: Call) => ({
      gatewayId: 'gw-dev',
      dryRun: Boolean((call.data as {dryRun?: boolean}).dryRun),
      diff: {
        toVersion: versions[1].version,
        summary: {added: 0, updated: 1, deleted: 0, unchanged: 0},
        changes: [{key: 'flow/f-1', resourceType: 'flow', name: 'Sign in', change: 'updated'}],
      },
    });
    routes['POST /gateways/gw-dev/apply'] = () => ({
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {toVersion: versions[0].version, summary: {added: 0, updated: 0, deleted: 0, unchanged: 0}, changes: []},
    });
    const checks = (): number => http.calls.filter((call: Call) => call.url.endsWith('/gw-dev/apply')).length;
    render(<DeployPage />);

    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    await waitFor(() => expect(checks()).toBeGreaterThan(0));
    const before = checks();
    await userEvent.click(within(dev).getByRole('button', {name: 'Rollback'}));
    const panel = await screen.findByTestId('deploy-revert-panel');
    expect(within(panel).getByText('Roll back Dev to v5')).toBeInTheDocument();
    // A rollback goes back one step only, and says so.
    expect(within(panel).getByTestId('deploy-revert-what')).toHaveTextContent('goes back one step, to v5');
    expect(
      await within(panel).findByText('Rolling back writes 1 resources as the earlier version has them.'),
    ).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', {name: 'Roll back'}));

    await waitFor(() => expect(screen.queryByTestId('deploy-revert-panel')).not.toBeInTheDocument());
    expect(http.calls.filter((call: Call) => call.url.endsWith('/revert')).map((call: Call) => call.data)).toEqual([
      {dryRun: true},
      {dryRun: false},
    ]);
    // What the gateway holds back is read again, since the rollback changed what it runs.
    await waitFor(() => expect(checks()).toBeGreaterThan(before));
  });

  it('says how many resources a rollback had refused', async () => {
    routes['POST /gateways/gw-dev/revert'] = (call: Call) => ({
      gatewayId: 'gw-dev',
      dryRun: Boolean((call.data as {dryRun?: boolean}).dryRun),
      diff: {
        toVersion: versions[1].version,
        summary: {added: 0, updated: 1, deleted: 0, unchanged: 0},
        changes: [{key: 'flow/f-1', resourceType: 'flow', name: 'Sign in', change: 'updated'}],
      },
      import: {summary: {failed: 2}},
    });
    render(<DeployPage />);

    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    await userEvent.click(within(dev).getByRole('button', {name: 'Rollback'}));
    const panel = await screen.findByTestId('deploy-revert-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Roll back'}));

    expect(
      await screen.findByText('Dev runs its earlier version again; 2 resources were refused.'),
    ).toBeInTheDocument();
  });

  it('says when there is no gateway to deploy to', async () => {
    routes['GET /gateways'] = [];
    render(<DeployPage />);

    expect(await screen.findByText('Register a gateway to deploy a version to it.')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Deploy'})).not.toBeInTheDocument();
  });

  it('says when the versions could not be read, rather than that none were captured', async () => {
    routes['GET /configuration-versions'] = new Error('down');
    render(<DeployPage />);

    expect(await screen.findByText('The configuration versions could not be read.')).toBeInTheDocument();
  });
});

const refusal = (code: string, status = 409): Error =>
  Object.assign(new Error(code), {response: {status, data: {code}}});

describe('DeployPage: apply safety', () => {
  const dryRunWithRemovals = {
    gatewayId: 'gw-prod',
    dryRun: true,
    diff: {
      toVersion: versions[0].version,
      summary: {added: 0, updated: 0, deleted: 2, unchanged: 1},
      changes: [
        {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'deleted'},
        {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing', change: 'deleted'},
        {key: 'role/r-1', resourceType: 'role', id: 'r-1', name: 'Admin', change: 'unchanged'},
      ],
    },
    changed: {
      secrets: ['APP_ORDERS_CLIENT_SECRET'],
      references: [
        {
          name: 'APP_ORDERS_CLIENT_SECRET',
          kind: 'secret',
          resourceType: 'application',
          resourceId: 'app-1',
          resourceName: 'Orders',
          field: 'clientSecret',
        },
      ],
    },
  };

  it('points out the values set since the last deploy, and asks before a deploy that removes many', async () => {
    let applies = 0;
    routes['POST /gateways/gw-prod/apply'] = (call: Call) => {
      const data = call.data as {dryRun?: boolean; confirmRemovals?: boolean};
      if (data.dryRun) return dryRunWithRemovals;
      applies += 1;
      if (!data.confirmRemovals) return refusal('GTW-1030');
      return {gatewayId: 'gw-prod', dryRun: false, diff: {...dryRunWithRemovals.diff}, import: {summary: {failed: 0}}};
    };
    render(<DeployPage />);

    const newest = await screen.findByTestId(`deploy-version-${versions[0].version}`);
    await userEvent.click(within(newest).getByRole('button', {name: 'Deploy'}));
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));
    const changed = await within(panel).findByTestId('changed-values');
    expect(within(changed).getByText('Values set since the last deploy')).toBeInTheDocument();
    expect(within(changed).getByText('APP_ORDERS_CLIENT_SECRET')).toBeInTheDocument();
    expect(within(changed).getByText('used by Orders')).toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    const warning = await within(panel).findByTestId('deploy-confirm-removals');
    expect(warning).toHaveTextContent('Deploying removes 2 resources from Prod.');
    expect(screen.getByTestId('deploy-apply-panel')).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy and remove 2'}));

    await waitFor(() => expect(screen.queryByTestId('deploy-apply-panel')).not.toBeInTheDocument());
    expect(applies).toBe(2);
    const sent = http.calls
      .filter((call: Call) => call.url.endsWith('/gw-prod/apply') && !(call.data as {dryRun?: boolean}).dryRun)
      .map((call: Call) => (call.data as {confirmRemovals?: boolean}).confirmRemovals);
    expect(sent).toEqual([undefined, true]);
  });

  it('keeps a confirmation across a refocus, and drops it when what is ticked changes', async () => {
    routes['POST /gateways/gw-prod/apply'] = (call: Call) => {
      const data = call.data as {dryRun?: boolean; confirmRemovals?: boolean};
      if (data.dryRun) return dryRunWithRemovals;
      if (!data.confirmRemovals) return refusal('GTW-1030');
      return {gatewayId: 'gw-prod', dryRun: false, diff: {...dryRunWithRemovals.diff}, import: {summary: {failed: 0}}};
    };
    render(<DeployPage />);

    const newest = await screen.findByTestId(`deploy-version-${versions[0].version}`);
    await userEvent.click(within(newest).getByRole('button', {name: 'Deploy'}));
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));
    await userEvent.click(await within(panel).findByRole('button', {name: 'Deploy'}));
    expect(await within(panel).findByTestId('deploy-confirm-removals')).toBeInTheDocument();

    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });
    expect(within(panel).getByTestId('deploy-confirm-removals')).toBeInTheDocument();
    expect(within(panel).getByRole('button', {name: 'Deploy and remove 2'})).toBeInTheDocument();

    // Keeping Orders changes what would be removed, so the warning no longer covers it.
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Orders/}));
    expect(within(panel).queryByTestId('deploy-confirm-removals')).not.toBeInTheDocument();
    expect(within(panel).getByRole('checkbox', {name: /Orders/})).not.toBeChecked();
    expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled();
  });

  it('asks before a rollback that removes many, and sends the confirmation', async () => {
    routes['POST /gateways/gw-dev/revert'] = (call: Call) => {
      const data = call.data as {dryRun?: boolean; confirmRemovals?: boolean};
      if (data.dryRun) {
        // A deletion the gateway holds back is not sent, so it is not counted as a removal.
        const heldBack = {
          key: 'role/r-2',
          resourceType: 'role',
          id: 'r-2',
          name: 'Ops',
          change: 'deleted',
          excluded: true,
        };
        const diff = {...dryRunWithRemovals.diff, changes: [...dryRunWithRemovals.diff.changes, heldBack]};
        return {...dryRunWithRemovals, gatewayId: 'gw-dev', diff, changed: undefined};
      }
      if (!data.confirmRemovals) return refusal('GTW-1030');
      return {gatewayId: 'gw-dev', dryRun: false, diff: {...dryRunWithRemovals.diff}};
    };
    render(<DeployPage />);

    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    await userEvent.click(within(dev).getByRole('button', {name: 'Rollback'}));
    const panel = await screen.findByTestId('deploy-revert-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Roll back'}));
    expect(await within(panel).findByTestId('deploy-confirm-removals')).toHaveTextContent(
      'Rolling back removes 2 resources from Dev.',
    );
    await userEvent.click(within(panel).getByRole('button', {name: 'Roll back and remove 2'}));

    await waitFor(() => expect(screen.queryByTestId('deploy-revert-panel')).not.toBeInTheDocument());
    expect(http.calls.filter((call: Call) => call.url.endsWith('/revert')).map((call: Call) => call.data)).toEqual([
      {dryRun: true},
      {dryRun: false},
      {dryRun: false, confirmRemovals: true},
    ]);
  });

  it('says another deploy is running when the gateway is busy, and why any other refusal happened', async () => {
    let answer: Error = refusal('GTW-1029');
    routes['POST /gateways/gw-prod/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun ? {...dryRunWithRemovals, changed: undefined} : answer;
    render(<DeployPage />);

    const newest = await screen.findByTestId(`deploy-version-${versions[0].version}`);
    await userEvent.click(within(newest).getByRole('button', {name: 'Deploy'}));
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));
    await userEvent.click(await within(panel).findByRole('button', {name: 'Deploy'}));
    expect(
      await within(panel).findByText('Another deploy to this gateway is running. Try again once it finishes.'),
    ).toBeInTheDocument();

    answer = refusal('GTW-9999', 500);
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    expect(await within(panel).findByText('The version could not be deployed.')).toBeInTheDocument();
    expect(within(panel).queryByText('GTW-9999')).not.toBeInTheDocument();

    answer = refusal('GTW-9999', 403);
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    expect(await within(panel).findByText('Your account may not deploy to this gateway.')).toBeInTheDocument();
  });

  it('says another deploy is running when a rollback finds the gateway busy', async () => {
    routes['POST /gateways/gw-dev/revert'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {...dryRunWithRemovals, gatewayId: 'gw-dev', changed: undefined}
        : refusal('GTW-1029');
    render(<DeployPage />);

    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    await userEvent.click(within(dev).getByRole('button', {name: 'Rollback'}));
    const panel = await screen.findByTestId('deploy-revert-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Roll back'}));
    expect(
      await within(panel).findByText('Another deploy to this gateway is running. Try again once it finishes.'),
    ).toBeInTheDocument();
  });

  it('names the values set since the last deploy before a rollback, as before a deploy', async () => {
    routes['POST /gateways/gw-dev/revert'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {...dryRunWithRemovals, gatewayId: 'gw-dev'}
        : {gatewayId: 'gw-dev', dryRun: false, diff: {...dryRunWithRemovals.diff}};
    render(<DeployPage />);

    const dev = await screen.findByTestId('deploy-stage-gw-dev');
    await userEvent.click(within(dev).getByRole('button', {name: 'Rollback'}));
    const panel = await screen.findByTestId('deploy-revert-panel');
    const changed = await within(panel).findByTestId('changed-values');
    expect(within(changed).getByText('APP_ORDERS_CLIENT_SECRET')).toBeInTheDocument();
  });

  it('says why the deploy check failed and checks again on request', async () => {
    let failing = true;
    routes['POST /gateways/gw-prod/apply'] = () =>
      failing ? refusal('GTW-1029') : {...dryRunWithRemovals, changed: undefined};
    render(<DeployPage />);

    const newest = await screen.findByTestId(`deploy-version-${versions[0].version}`);
    await userEvent.click(within(newest).getByRole('button', {name: 'Deploy'}));
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));
    expect(
      await within(panel).findByText('Another deploy to this gateway is running. Try again once it finishes.'),
    ).toBeInTheDocument();

    failing = false;
    await userEvent.click(within(panel).getByRole('button', {name: 'Refresh'}));
    expect(await within(panel).findByRole('checkbox', {name: /Orders/})).toBeInTheDocument();
    expect(
      within(panel).queryByText('Another deploy to this gateway is running. Try again once it finishes.'),
    ).not.toBeInTheDocument();
  });
});
