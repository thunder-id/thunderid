// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import ApplyPanel from '../ApplyPanel';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

const version = {version: 'hash-2', number: 2, createdAt: '2026-11-01T10:00:00Z'};
const gateway = {id: 'gw-dev', name: 'Dev', baseUrl: 'https://dev.test', isDefault: true};

/** A project's check of three changes: Orders, refused before, needs the Login flow Dev lacks. */
function projectCheck(sent: unknown[]): void {
  http = fakeHttp({
    'POST /gateways/gw-dev/apply': (call: Call) => {
      const data = call.data as {dryRun?: boolean};
      if (!data.dryRun) {
        sent.push(call.data);
        return {gatewayId: 'gw-dev', dryRun: false};
      }
      return {
        gatewayId: 'gw-dev',
        dryRun: true,
        diff: {
          toVersion: version.version,
          changes: [
            {
              key: 'application/app-1',
              resourceType: 'application',
              id: 'app-1',
              name: 'Orders',
              change: 'updated',
              section: 'project',
              failed: true,
              needs: ['flow/f-1'],
            },
            {
              key: 'flow/f-1',
              resourceType: 'flow',
              id: 'f-1',
              name: 'Login',
              change: 'added',
              section: 'needed',
              required: true,
            },
            {
              key: 'application/app-2',
              resourceType: 'application',
              id: 'app-2',
              name: 'Billing',
              change: 'updated',
              section: 'project',
            },
          ],
        },
      };
    },
  });
}

describe('ApplyPanel', () => {
  it("retries what a project's gateway refused with what it needs, scoped to the project", async () => {
    const sent: unknown[] = [];
    projectCheck(sent);
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          only={['application/app-1']}
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </ProjectScopeProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByRole('checkbox', {name: /Orders/})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: /Billing/})).not.toBeChecked();
    await userEvent.click(await within(panel).findByRole('button', {name: /^Deploy/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]; project?: string; withDependencies?: boolean};
    expect([...request.selection].sort()).toEqual(['application/app-1', 'flow/f-1']);
    expect(request.project).toBe('p-shop');
    expect(request.withDependencies).toBe(false);
  });

  it("deploys a project's version with the own changes left unticked out, and what only they need", async () => {
    const sent: unknown[] = [];
    projectCheck(sent);
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </ProjectScopeProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    const orders = await within(panel).findByRole('checkbox', {name: /Orders/});
    expect(orders).toBeChecked();
    expect(orders).toBeEnabled();
    expect(within(panel).getByRole('checkbox', {name: /Billing/})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: /Login/})).toBeDisabled();

    await userEvent.click(orders);
    expect(within(panel).getByRole('checkbox', {name: /Login/})).not.toBeChecked();
    await userEvent.click(await within(panel).findByRole('button', {name: /^Deploy/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]; project?: string};
    expect(request.selection).toEqual(['application/app-2']);
    expect(request.project).toBe('p-shop');
  });
});
