// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
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
function projectCheck(sent: unknown[], extra: unknown[] = []): void {
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
            ...extra,
          ],
        },
      };
    },
  });
}

describe('ApplyPanel', () => {
  it("says why the missing values are locked to an admin who may not set the gateway's values", async () => {
    http = fakeHttp({
      'POST /gateways/gw-dev/apply': () => ({
        gatewayId: 'gw-dev',
        dryRun: true,
        diff: {
          toVersion: version.version,
          changes: [
            {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'added'},
          ],
        },
        missing: {variables: ['APP_ORDERS_CLIENT_ID']},
      }),
    });
    render(
      <CapabilitiesProvider
        capabilities={{
          can: (resource) => resource !== 'gatewayvariable' && resource !== 'gatewaysecret',
          isLoading: false,
        }}
      >
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </CapabilitiesProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByLabelText(/APP_ORDERS_CLIENT_ID/)).toBeDisabled();
    expect(
      within(panel).getByText(/Your account may not set this gateway's variables and secrets/),
    ).toBeInTheDocument();
  });

  it("doesn't say the deploy waits when only a refused resource's values are locked", async () => {
    projectCheck([]);
    render(
      <CapabilitiesProvider
        capabilities={{
          can: (resource) => resource !== 'gatewayvariable' && resource !== 'gatewaysecret',
          isLoading: false,
        }}
      >
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          only={['application/app-1']}
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </CapabilitiesProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByRole('button', {name: /^Deploy/})).toBeEnabled();
    expect(within(panel).queryByText(/this deploy waits for an admin/)).not.toBeInTheDocument();
  });

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

  it('ticks the allowed origins and a translation a project relies on, but not one held back or other shared updates', async () => {
    const sent: unknown[] = [];
    projectCheck(sent, [
      {
        key: 'server_config/cors',
        resourceType: 'server_config',
        name: 'cors',
        change: 'updated',
        section: 'shared',
        carried: true,
      },
      {
        key: 'translation/de-DE',
        resourceType: 'translation',
        name: 'de-DE',
        change: 'updated',
        section: 'shared',
        carried: true,
      },
      {key: 'theme/t-1', resourceType: 'theme', id: 't-1', name: 'Brand', change: 'updated', section: 'shared'},
      {
        key: 'translation/fr-FR',
        resourceType: 'translation',
        name: 'fr-FR',
        change: 'added',
        section: 'shared',
        carried: true,
        heldBack: true,
      },
    ]);
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
    const cors = await within(panel).findByRole('checkbox', {name: /cors/});
    expect(cors).toBeChecked();
    expect(cors).toBeEnabled();
    expect(within(panel).getByRole('checkbox', {name: /de-DE/})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: /Brand/})).not.toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: /fr-FR/})).not.toBeChecked();

    await userEvent.click(within(panel).getByRole('checkbox', {name: /de-DE/}));
    await userEvent.click(await within(panel).findByRole('button', {name: /^Deploy/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]};
    expect(request.selection).toContain('server_config/cors');
    expect(request.selection).not.toContain('translation/de-DE');
    expect(request.selection).not.toContain('theme/t-1');
    expect(request.selection).not.toContain('translation/fr-FR');
  });
});
