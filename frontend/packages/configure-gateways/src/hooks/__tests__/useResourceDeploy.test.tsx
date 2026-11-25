// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import useResourceDeploy from '../useResourceDeploy';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

const versions = [
  {version: 'hash-3', number: 3, createdAt: '2026-11-03T10:00:00Z'},
  {version: 'hash-2', number: 2, createdAt: '2026-11-02T10:00:00Z'},
];
const dev = {id: 'gw-dev', name: 'dev', baseUrl: 'https://dev.test', isDefault: true};
const prod = {id: 'gw-prod', name: 'prod', baseUrl: 'https://prod.test'};

/** The applies asked for, by gateway, with what each asked for. */
let applies: {url: string; data: unknown}[] = [];

function DeployOne({target = undefined}: {target?: string}) {
  const deploy = useResourceDeploy();
  return (
    <>
      <button type="button" onClick={() => deploy.open('application', 'app-1', target)}>
        bring
      </button>
      {deploy.panels}
    </>
  );
}

beforeEach(() => {
  applies = [];
  const dryRun = (call: Call) => {
    applies.push({url: call.url, data: call.data});
    return {
      dryRun: true,
      diff: {
        changes: [
          {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
        ],
      },
    };
  };
  http = fakeHttp({
    'GET /gateways/pipeline': {
      stages: [
        {name: 'Development', adminConsoleUrl: 'https://dev.test/console', gateway: dev},
        {name: 'Production', adminConsoleUrl: 'https://prod.test/console', gateway: prod},
      ],
    },
    'GET /configuration-versions': versions,
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev', appliedVersion: 'hash-3'},
    'GET /gateways/gw-prod/applied-version': {gatewayId: 'gw-prod', appliedVersion: 'hash-2'},
    'GET /server-config/gateway': {merged: {autoApply: false}, writable: {}},
    'POST /gateways/gw-dev/apply': dryRun,
    'POST /gateways/gw-prod/apply': dryRun,
  });
});

describe('useResourceDeploy', () => {
  it('deploys one resource of the latest version to the first gateway, with the path shown', async () => {
    render(<DeployOne />);

    await userEvent.click(screen.getByRole('button', {name: 'bring'}));

    const steps = await screen.findByTestId('pipeline-steps');
    const [first, second] = within(steps).getAllByRole('listitem');
    expect(first).toHaveAttribute('data-current', 'true');
    expect(first).toHaveTextContent('Development');
    expect(second).toHaveTextContent('promote once Development is verified');
    expect(await screen.findByRole('checkbox', {name: /Orders/})).toBeChecked();
    expect(applies[0].url).toContain('/gateways/gw-dev/apply');
    expect(applies[0].data).toMatchObject({version: 'hash-3', dryRun: true});
  });

  it('promotes one resource to a later gateway from the version the gateway before it runs', async () => {
    render(<DeployOne target="Production" />);

    await userEvent.click(screen.getByRole('button', {name: 'bring'}));

    const steps = await screen.findByTestId('pipeline-steps');
    expect(within(steps).getAllByRole('listitem')[1]).toHaveAttribute('data-current', 'true');
    expect(await screen.findByText(/from Development to Production/)).toBeInTheDocument();
    expect(applies[0].url).toContain('/gateways/gw-prod/apply');
    expect(applies[0].data).toMatchObject({version: 'hash-3'});
  });
});
