// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import DeployHeaderButton from '../DeployHeaderButton';

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

const v2 = '2f2d727aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const versions = [{version: v2, createdAt: '2026-10-12T13:52:47Z', number: 2}];
const dev = {id: 'gw-dev', name: 'dev-gateway', baseUrl: 'https://dev.test', isDefault: true};
const prod = {id: 'gw-prod', name: 'prod-gateway', baseUrl: 'https://prod.test'};
const change = (key: string, kind: string, extra: object = {}) => ({
  key,
  resourceType: 'application',
  change: kind,
  ...extra,
});

let routes: Record<string, unknown>;

beforeEach(() => {
  navigate.mockReset();
  routes = {
    'GET /gateways/pipeline': {
      stages: [
        {name: 'Development', adminConsoleUrl: 'https://dev.test/console', gateway: dev},
        {name: 'Production', adminConsoleUrl: 'https://prod.test/console', gateway: prod},
      ],
    },
    'GET /configuration-versions': versions,
    [`GET /configuration-versions/${v2}`]: {...versions[0], references: []},
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev'},
    'GET /gateways/gw-prod/applied-version': {gatewayId: 'gw-prod'},
    'GET /gateways/gw-dev/master': {
      gatewayId: 'gw-dev',
      number: 0,
      summary: {deployed: 0, failed: 0, removed: 0},
      entries: [],
    },
    'GET /gateways/gw-dev/variables': {totalResults: 0, variables: []},
    'GET /gateways/gw-dev/secrets': {totalResults: 0, secrets: []},
    'GET /server-config/gateway': {readOnly: null, writable: null, merged: {autoApply: false}},
    'POST /gateways/gw-dev/apply': {
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        summary: {added: 2, updated: 0, deleted: 0, unchanged: 1},
        changes: [
          change('application/a-1', 'added', {name: 'Orders'}),
          change('application/a-2', 'added', {name: 'Billing'}),
          change('application/a-3', 'added', {name: 'Old', heldBack: true}),
          change('application/a-4', 'unchanged'),
        ],
      },
    },
  };
  http = fakeHttp(routes);
});

describe('DeployHeaderButton', () => {
  it('counts the changes the first environment lacks, less what was left out on purpose', async () => {
    render(<DeployHeaderButton />);

    await waitFor(() => expect(screen.getByTestId('header-deploy')).toHaveTextContent('Deploy 2 changes'));
  });

  it('opens the deploy to the first environment in a drawer, with the path and auto-sync', async () => {
    render(<DeployHeaderButton />);

    await waitFor(() => expect(screen.getByTestId('header-deploy')).toHaveTextContent('Deploy 2 changes'));
    await userEvent.click(screen.getByTestId('header-deploy'));

    const drawer = await screen.findByTestId('deploy-apply-panel');
    const steps = within(drawer).getByTestId('pipeline-steps');
    expect(within(steps).getByText('This deploy').closest('li')).toHaveAttribute('data-current', 'true');
    expect(within(steps).getByText('Production')).toBeInTheDocument();
    expect(within(drawer).getByText('Auto-sync configuration to Development')).toBeInTheDocument();
    expect(await within(drawer).findByRole('button', {name: /Deploy 2 changes to dev-gateway/})).toBeInTheDocument();
  });

  it('goes to the Deploy page while there is no environment', async () => {
    routes['GET /gateways/pipeline'] = {stages: []};
    render(<DeployHeaderButton />);

    await waitFor(() => expect(http.calls.some((call: Call) => call.url.endsWith('/gateways/pipeline'))).toBe(true));
    await userEvent.click(screen.getByTestId('header-deploy'));

    expect(navigate).toHaveBeenCalledWith('/deploy');
  });
});
