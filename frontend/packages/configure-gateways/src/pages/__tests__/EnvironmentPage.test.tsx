// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import EnvironmentPage from '../EnvironmentPage';

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
    [`GET /configuration-versions/${v1}`]: {...versions[1], references: [{name: 'APP_SECRET', kind: 'secret'}]},
    [`GET /configuration-versions/${v2}`]: {...versions[0], references: []},
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev', appliedVersion: v1, previousVersion: v1},
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
  };
  http = fakeHttp(routes);
});

describe('EnvironmentPage', () => {
  it('shows the environment with its admin portal, endpoints, what it holds and its deploys', async () => {
    render(<EnvironmentPage />);

    expect(await screen.findByRole('heading', {name: 'Development'})).toBeInTheDocument();
    const portal = screen.getByTestId('environment-admin-console');
    expect(portal).toHaveAttribute('href', 'https://dev.test/console');
    expect(portal).toHaveAttribute('target', '_blank');
    expect(screen.getByText('https://dev.test/.well-known/openid-configuration')).toBeInTheDocument();

    // What it holds is summed up by kind, with what it refused, and explored on its own view.
    const resources = await screen.findByTestId('environment-resources');
    expect(await within(resources).findByText('Applications')).toBeInTheDocument();
    expect(within(resources).getByText('1 · shared')).toBeInTheDocument();
    expect(within(resources).getByText('Flows')).toBeInTheDocument();
    expect(within(resources).getByText('1 refused')).toBeInTheDocument();
    expect(within(resources).queryByText('Orders')).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId('environment-explore'));
    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-dev/explore');
    expect(await within(screen.getByTestId('environment-history')).findByText('Deploy 2')).toBeInTheDocument();
    // The first environment says whether saved changes reach it on their own.
    expect(screen.getByText('Auto-sync configuration to Development')).toBeInTheDocument();
  });

  it('offers the latest version to the first environment while it runs an older one', async () => {
    render(<EnvironmentPage />);

    const status = await screen.findByTestId('environment-status');
    await waitFor(() => expect(status).toHaveTextContent('v2 is ready for Development, which runs v1.'));
    await userEvent.click(within(status).getByRole('button', {name: 'Review and deploy'}));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
  });

  it('offers a later environment what the one before runs, and nothing once it runs it', async () => {
    gatewayId = 'gw-prod';
    render(<EnvironmentPage />);

    const status = await screen.findByTestId('environment-status');
    await waitFor(() => expect(status).toHaveTextContent('Production runs everything there is for it now.'));
    expect(screen.queryByText('Auto-sync configuration to Production')).not.toBeInTheDocument();
  });

  it("configures the environment's variables and secrets, the unset ones its version needs first", async () => {
    render(<EnvironmentPage />);

    await userEvent.click(await screen.findByRole('tab', {name: /Secrets/}));

    expect(
      await screen.findByText('The version Development runs refers to 1 values it does not hold.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Not set')).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'Variables (1)'})).toBeInTheDocument();
  });

  it('says so when no environment runs on the gateway', async () => {
    gatewayId = 'gw-gone';
    render(<EnvironmentPage />);

    expect(await screen.findByText('No environment runs on this gateway.')).toBeInTheDocument();
  });
});
