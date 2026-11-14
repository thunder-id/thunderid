// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, HiddenResourcesProvider, ProjectScopeProvider} from '@thunderid/contexts';
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

function replacementCheck(sent: unknown[]): void {
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
              key: 'resource_server/rs-new',
              resourceType: 'resource_server',
              id: 'rs-new',
              name: 'Orders API',
              change: 'added',
              section: 'shared',
              failed: true,
              replaces: 'resource_server/rs-old',
            },
            {
              key: 'resource_server/rs-old',
              resourceType: 'resource_server',
              id: 'rs-old',
              name: 'Orders API',
              change: 'deleted',
              section: 'shared',
            },
            {
              key: 'resource_server/rs-gone',
              resourceType: 'resource_server',
              id: 'rs-gone',
              name: 'Retired API',
              change: 'deleted',
              section: 'shared',
            },
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
    await waitFor(() => expect(within(panel).getByRole('button', {name: /^Deploy/})).toBeEnabled());
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

  it('leaves a hidden resource out of the changes it shows, but still deploys it', async () => {
    const sent: unknown[] = [];
    projectCheck(sent);
    render(
      <HiddenResourcesProvider hidden={{ids: {application: ['app-2']}}}>
        <ProjectScopeProvider project={{id: 'p-shop'}}>
          <ApplyPanel
            version={version}
            label="v2"
            gateways={[gateway]}
            gatewayId="gw-dev"
            onClose={vi.fn()}
            onDeployed={vi.fn()}
          />
        </ProjectScopeProvider>
      </HiddenResourcesProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByRole('checkbox', {name: /Orders/})).toBeChecked();
    expect(within(panel).queryByRole('checkbox', {name: /Billing/})).not.toBeInTheDocument();
    expect(within(panel).queryByText(/Billing/)).not.toBeInTheDocument();
    await userEvent.click(await within(panel).findByRole('button', {name: /^Deploy/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]};
    expect([...request.selection].sort()).toEqual(['application/app-1', 'application/app-2', 'flow/f-1']);
  });

  it('offers a plain deploy in the drawer when only hidden changes are ticked', async () => {
    const sent: unknown[] = [];
    projectCheck(sent);
    render(
      <HiddenResourcesProvider hidden={{ids: {application: ['app-2']}}}>
        <ProjectScopeProvider project={{id: 'p-shop'}}>
          <ApplyPanel
            version={version}
            label="v2"
            gateways={[gateway]}
            gatewayId="gw-dev"
            variant="drawer"
            targetName="Dev"
            onClose={vi.fn()}
            onDeployed={vi.fn()}
          />
        </ProjectScopeProvider>
      </HiddenResourcesProvider>,
    );

    const orders = await screen.findByRole('checkbox', {name: /Orders/});
    expect(screen.getByRole('button', {name: 'Deploy 2 changes to Dev'})).toBeInTheDocument();
    await userEvent.click(orders);
    await userEvent.click(screen.getByRole('button', {name: 'Deploy'}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]};
    expect(request.selection).toEqual(['application/app-2']);
  });

  it('says there is nothing to deploy in the drawer when every change is unticked', async () => {
    projectCheck([]);
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          variant="drawer"
          targetName="Dev"
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </ProjectScopeProvider>,
    );

    await userEvent.click(await screen.findByRole('checkbox', {name: /Orders/}));
    await userEvent.click(screen.getByRole('checkbox', {name: /Billing/}));

    expect(screen.getByRole('button', {name: 'Nothing to deploy'})).toBeInTheDocument();
  });

  it.each([
    ['a deploy', undefined],
    ['a retry of the refused addition', ['resource_server/rs-new']],
  ])('sends, in %s, the removal of the resource an addition replaces, locked to it', async (_, only) => {
    const sent: unknown[] = [];
    replacementCheck(sent);
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <ApplyPanel
          version={version}
          label="v2"
          gateways={[gateway]}
          gatewayId="gw-dev"
          only={only}
          onClose={vi.fn()}
          onDeployed={vi.fn()}
        />
      </ProjectScopeProvider>,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    const rows = await within(panel).findAllByRole('checkbox', {name: /Orders API/});
    expect(rows).toHaveLength(2);
    rows.forEach((row: HTMLElement) => expect(row).toBeChecked());
    expect(rows.filter((row: HTMLElement) => (row as HTMLInputElement).disabled)).toHaveLength(1);
    expect(within(panel).getByRole('checkbox', {name: /Retired API/})).not.toBeChecked();
    await userEvent.click(await within(panel).findByRole('button', {name: /^Deploy/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0] as {selection: string[]};
    expect([...request.selection].sort()).toEqual(['resource_server/rs-new', 'resource_server/rs-old']);
  });
});
