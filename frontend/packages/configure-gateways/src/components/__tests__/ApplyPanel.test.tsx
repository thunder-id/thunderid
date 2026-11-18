// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, HiddenResourcesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import {fakeHttp, isDryRun, isPreview, SERVER, type Call} from '../../__tests__/fakeHttp';
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
  it('previews the deploy without the gateway checking it, and has the gateway check it once when asked', async () => {
    const orders = {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders'};
    http = fakeHttp({
      // The preview names the value Dev lacks; only the gateway's own check says what it would refuse.
      'GET /gateways/gw-dev/diff': {
        toVersion: version.version,
        changes: [{...orders, change: 'added'}],
        missing: {
          variables: ['APP_ORDERS_CLIENT_ID'],
          references: [
            {name: 'APP_ORDERS_CLIENT_ID', kind: 'variable', resourceType: 'application', resourceId: 'app-1'},
          ],
        },
        removals: {count: 0, threshold: 5, needsConfirmation: false},
      },
      'POST /gateways/gw-dev/apply': {
        gatewayId: 'gw-dev',
        dryRun: true,
        diff: {toVersion: version.version, changes: [{...orders, change: 'added'}]},
        import: {
          results: [
            {resourceType: 'application', resourceId: 'app-1', status: 'failed', code: 'APP-1022', message: 'In use'},
          ],
        },
      },
    });
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        variant="drawer"
        targetName="Dev"
        onClose={vi.fn()}
        onDeployed={vi.fn()}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByLabelText(/APP_ORDERS_CLIENT_ID/)).toBeInTheDocument();
    expect(http.calls.filter(isPreview).map((call: Call) => call.url)).toEqual([
      `${SERVER}/gateways/gw-dev/diff?version=hash-2&preview=true`,
    ]);
    expect(http.calls.filter(isDryRun)).toEqual([]);
    expect(within(panel).queryByTestId('refused-fixes')).not.toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('button', {name: 'Check with gateway'}));

    expect(await within(panel).findByTestId('refused-fix-predicted')).toHaveTextContent('Would be refused');
    expect(within(panel).getByTestId('deploy-gateway-checked')).toHaveTextContent('Dev would refuse 1 change.');
    expect(http.calls.filter(isDryRun).map((call: Call) => call.data)).toEqual([
      {version: 'hash-2', selection: ['application/app-1'], dryRun: true},
    ]);
  });

  it('drops the gateway check when the preview is read again, and waits for it after a refused plan', async () => {
    const changes = [
      {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'deleted'},
      {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing', change: 'deleted'},
      // A translation has no id: the gateway names it by its language.
      {key: 'translation/fr', resourceType: 'translation', name: 'fr', change: 'updated'},
    ];
    const removals = {count: 2, threshold: 1, needsConfirmation: true};
    let previews = 0;
    let release: () => void = () => undefined;
    const sent: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': () => {
        previews += 1;
        if (previews === 1) return {toVersion: version.version, changes, removals, plan: 'plan-1'};
        // The preview read after the refusal answers only once released.
        return new Promise((resolve) => {
          release = () => resolve({toVersion: version.version, changes, removals, plan: 'plan-2'});
        });
      },
      'POST /gateways/gw-dev/apply': (call: Call) => {
        const data = call.data as {dryRun?: boolean};
        if (data.dryRun) {
          return {
            gatewayId: 'gw-dev',
            dryRun: true,
            diff: {toVersion: version.version, changes},
            import: {results: [{resourceType: 'translation', resourceName: 'fr', status: 'failed', message: 'Bad'}]},
          };
        }
        sent.push(call.data);
        return sent.length === 1
          ? Object.assign(new Error('changed'), {response: {status: 409, data: {code: 'GTW-1019'}}})
          : {gatewayId: 'gw-dev', dryRun: false};
      },
    });
    const onDeployed = vi.fn();
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        variant="drawer"
        targetName="Dev"
        onClose={vi.fn()}
        onDeployed={onDeployed}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Check with gateway'}));
    // The refusal of the translation is matched to its change by its name.
    expect(await within(panel).findByTestId('refused-fix-translation/fr')).toBeInTheDocument();
    expect(within(panel).getByTestId('deploy-gateway-checked')).toHaveTextContent('Dev would refuse 1 change.');

    const submit = within(panel).getByRole('button', {name: 'Deploy and remove 2'});
    await userEvent.click(submit);
    // The plan was refused: nothing is sent again, and the gateway's check is not offered, until the
    // preview is read again.
    await waitFor(() => expect(previews).toBe(2));
    expect(within(panel).getByRole('button', {name: 'Deploy and remove 2'})).toBeDisabled();
    expect(within(panel).getByRole('button', {name: 'Check with gateway'})).toBeDisabled();
    release();
    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy and remove 2'})).toBeEnabled());
    // The check was made on the earlier preview, so it no longer holds.
    expect(within(panel).queryByTestId('deploy-gateway-checked')).not.toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy and remove 2'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(sent.map((data) => (data as {plan?: string}).plan)).toEqual(['plan-1', 'plan-2']);
  });

  it('confirms a deploy whose removals, dropped policies included, pass the threshold, bound to the plan', async () => {
    const sent: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {
        toVersion: version.version,
        changes: [
          {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'deleted'},
          {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing', change: 'deleted'},
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Login', change: 'updated'},
        ],
        // The flow sent again drops a sharing policy, which counts as a removal too.
        droppedPolicies: [{resourceType: 'flow', resourceId: 'f-1'}],
        removals: {count: 3, threshold: 2, needsConfirmation: true},
        plan: 'plan-1',
      },
      'POST /gateways/gw-dev/apply': (call: Call) => {
        sent.push(call.data);
        return {gatewayId: 'gw-dev', dryRun: false};
      },
    });
    const onDeployed = vi.fn();
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={onDeployed}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByTestId('deploy-confirm-removals')).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy and remove 3'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(sent).toEqual([
      expect.objectContaining({
        selection: ['application/app-1', 'application/app-2', 'flow/f-1'],
        confirmRemovals: true,
        plan: 'plan-1',
      }),
    ]);
  });

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

/** A promotion's check: Orders, which uses a value Prod lacks, and Billing, which the source holds back. */
function promotionCheck(sent: unknown[], refusal?: Error): void {
  http = fakeHttp({
    'PUT /gateways/gw-dev/variables/ORDERS_ID': {name: 'ORDERS_ID'},
    'POST /gateways/gw-dev/apply': (call: Call) => {
      const data = call.data as {dryRun?: boolean};
      if (!data.dryRun) {
        sent.push(call.data);
        return refusal ?? {gatewayId: 'gw-dev', dryRun: false};
      }
      return {
        gatewayId: 'gw-dev',
        dryRun: true,
        diff: {
          toVersion: version.version,
          changes: [
            {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'added'},
            {
              key: 'application/app-2',
              resourceType: 'application',
              id: 'app-2',
              name: 'Billing',
              change: 'added',
              sourceExcluded: true,
            },
          ],
        },
        missing: {
          variables: ['ORDERS_ID'],
          secrets: ['BILLING_SECRET'],
          references: [
            {name: 'ORDERS_ID', kind: 'variable', resourceType: 'application', resourceId: 'app-1'},
            {name: 'BILLING_SECRET', kind: 'secret', resourceType: 'application', resourceId: 'app-2'},
          ],
        },
      };
    },
  });
}

function renderPromotion(): void {
  render(
    <ApplyPanel
      version={version}
      label="v2"
      gateways={[gateway]}
      gatewayId="gw-dev"
      variant="drawer"
      targetName="Prod"
      sourceName="Stage"
      onClose={vi.fn()}
      onDeployed={vi.fn()}
    />,
  );
}

describe('ApplyPanel promoting what the source holds back', () => {
  it('neither sends a change the source holds back nor asks for the values only it uses', async () => {
    const sent: unknown[] = [];
    promotionCheck(sent);
    renderPromotion();

    const orders = await screen.findByLabelText(/ORDERS_ID/);
    expect(screen.queryByLabelText(/BILLING_SECRET/)).not.toBeInTheDocument();
    await userEvent.type(orders, 'orders-prod');
    await userEvent.click(await screen.findByRole('button', {name: /^Promote/}));

    await waitFor(() => expect(sent).toHaveLength(1));
    expect((sent[0] as {selection: string[]}).selection).toEqual(['application/app-1']);
  });
});

describe('ApplyPanel explaining a refusal', () => {
  it.each([
    [
      "a known refusal's details, naming each resource as the changes do",
      {
        code: 'GTW-1032',
        description: {
          key: 'error.gatewayservice.dependency_not_deployed.description',
          defaultValue: 'raw server words',
          params: {missing: 'role/r-1 needs application/app-1'},
        },
      },
      'Some selected resources need ones the gateway does not hold: role/r-1 needs Orders application. Tick what ' +
        'they need as well, or deploy with what these need.',
    ],
    [
      'a known refusal without details as the catalog has it',
      {code: 'GTW-1029'},
      'Another deploy to this gateway is running. Try again once it finishes.',
    ],
    [
      'the generic line for a refusal the catalog does not know, never the server text',
      {code: 'GTW-9998', description: {key: 'unknown', defaultValue: 'Person cannot be promoted.', params: {}}},
      'The version could not be deployed.',
    ],
    [
      'the generic line for a plain error a service in between answered with',
      {error: 'only what an environment runs can be promoted'},
      'The version could not be deployed.',
    ],
  ])('shows %s', async (_, data, shown) => {
    promotionCheck([], Object.assign(new Error('Request failed'), {response: {status: 409, data}}));
    renderPromotion();

    await userEvent.type(await screen.findByLabelText(/ORDERS_ID/), 'orders-prod');
    await userEvent.click(await screen.findByRole('button', {name: /^Promote/}));

    expect(await screen.findByText(shown)).toBeInTheDocument();
    expect(
      screen.queryByText(/raw server words|Person cannot be promoted|only what an environment runs/),
    ).not.toBeInTheDocument();
  });
});

describe('ApplyPanel after a refused write', () => {
  it('holds back only the preview whose plan was refused, not another gateway it switches to', async () => {
    const prod = {id: 'gw-prod', name: 'Prod', baseUrl: 'https://prod.test', isDefault: false};
    const changes = [
      {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
    ];
    let devPreviews = 0;
    http = fakeHttp({
      'GET /gateways/gw-prod/diff': {toVersion: version.version, changes, plan: 'prod-plan'},
      'GET /gateways/gw-dev/diff': () => {
        devPreviews += 1;
        // Dev's preview read again after the refusal does not answer, so Dev keeps waiting.
        return devPreviews === 1
          ? {toVersion: version.version, changes, plan: 'dev-plan'}
          : new Promise(() => undefined);
      },
      'POST /gateways/gw-dev/apply': () =>
        Object.assign(new Error('changed'), {response: {status: 409, data: {code: 'GTW-1019'}}}),
    });
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway, prod]}
        gatewayId="gw-prod"
        onClose={vi.fn()}
        onDeployed={vi.fn()}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());
    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Dev'}));
    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    await waitFor(() => expect(devPreviews).toBe(2));
    expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeDisabled();

    await userEvent.click(within(panel).getByRole('combobox', {name: 'Gateway'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Prod'}));

    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());
    expect(within(panel).getByRole('button', {name: 'Check with gateway'})).toBeEnabled();
  });

  it('writes a correction again on a retry after its write failed, though the check that asked for it is gone', async () => {
    const orders = {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders'};
    const writes: unknown[] = [];
    const sent: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {toVersion: version.version, changes: [{...orders, change: 'updated'}]},
      [`GET /configuration-versions/${version.version}`]: {
        ...version,
        references: [{name: 'ORDERS_URL', kind: 'variable', resourceType: 'application', resourceId: 'app-1'}],
      },
      'GET /gateways/gw-dev/variables': {totalResults: 1, variables: [{name: 'ORDERS_URL', value: 'old'}]},
      'GET /gateways/gw-dev/secrets': {totalResults: 0, secrets: []},
      'PUT /gateways/gw-dev/variables/ORDERS_URL': (call: Call) => {
        writes.push(call.data);
        return writes.length === 1
          ? Object.assign(new Error('down'), {response: {status: 503}})
          : {name: 'ORDERS_URL', value: 'new'};
      },
      'POST /gateways/gw-dev/apply': (call: Call) => {
        if ((call.data as {dryRun?: boolean}).dryRun) {
          return {
            gatewayId: 'gw-dev',
            dryRun: true,
            diff: {toVersion: version.version, changes: [{...orders, change: 'updated'}]},
            import: {
              results: [
                {resourceType: 'application', resourceId: 'app-1', status: 'failed', message: 'ORDERS_URL is invalid'},
              ],
            },
          };
        }
        sent.push(call.data);
        return {gatewayId: 'gw-dev', dryRun: false};
      },
    });
    const onDeployed = vi.fn();
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={onDeployed}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Check with gateway'}));
    const input = await within(panel).findByLabelText('ORDERS_URL');
    await waitFor(() => expect(input).toHaveValue('old'));
    await userEvent.clear(input);
    await userEvent.type(input, 'new');
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    await waitFor(() => expect(writes).toHaveLength(1));
    expect(sent).toEqual([]);
    // The write cleared the gateway's check, so the correction is no longer shown, but is still kept.
    await waitFor(() => expect(within(panel).queryByTestId('refused-fixes')).not.toBeInTheDocument());

    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(writes).toEqual([expect.objectContaining({value: 'new'}), expect.objectContaining({value: 'new'})]);
    expect(sent).toHaveLength(1);
  });

  it('points out a ticked removal the preview found still needed, and leaves the deploy to the control plane', async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {
        toVersion: version.version,
        changes: [
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Login', change: 'deleted'},
          {
            key: 'application/app-1',
            resourceType: 'application',
            id: 'app-1',
            name: 'Orders',
            change: 'updated',
            heldBack: true,
          },
        ],
        removalsInUse: [{key: 'flow/f-1', neededBy: 'application/app-1'}],
      },
    });
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={vi.fn()}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByTestId('deploy-removals-in-use')).toHaveTextContent(
      'In the preview, Dev keeps a resource that still needs what this deploy removes: Login flow, which Orders application needs.',
    );
    // The preview's finding is not a check of what is ticked: the deploy is not held up by it.
    expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled();
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Orders/}));
    expect(within(panel).getByTestId('deploy-removals-in-use')).toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('checkbox', {name: /Login/}));

    expect(within(panel).queryByTestId('deploy-removals-in-use')).not.toBeInTheDocument();
  });

  it("shows the gateway check's failure only for the deploy it checked", async () => {
    const changes = [
      {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
      {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing', change: 'updated'},
    ];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {toVersion: version.version, changes},
      'POST /gateways/gw-dev/apply': () => Object.assign(new Error('down'), {response: {status: 503}}),
    });
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={vi.fn()}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Check with gateway'}));
    expect(await within(panel).findByTestId('deploy-gateway-check-failed')).toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('checkbox', {name: /Billing/}));
    expect(within(panel).queryByTestId('deploy-gateway-check-failed')).not.toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Billing/}));
    expect(within(panel).getByTestId('deploy-gateway-check-failed')).toBeInTheDocument();
  });

  it('ties a value a translation uses to that language, not to another translation', async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {
        toVersion: version.version,
        changes: [
          // Translations have no id: each is named by its language, and so are the references in it.
          {key: 'translation/fr-FR', resourceType: 'translation', name: 'fr-FR', change: 'updated'},
          {key: 'translation/de-DE', resourceType: 'translation', name: 'de-DE', change: 'updated'},
        ],
        missing: {
          variables: ['FR_GREETING'],
          references: [{name: 'FR_GREETING', kind: 'variable', resourceType: 'translation', resourceName: 'fr-FR'}],
        },
      },
    });
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={vi.fn()}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    expect(await within(panel).findByLabelText(/FR_GREETING/)).toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('checkbox', {name: /fr-FR/}));

    expect(within(panel).queryByLabelText(/FR_GREETING/)).not.toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('checkbox', {name: /de-DE/}));
    await userEvent.click(within(panel).getByRole('checkbox', {name: /fr-FR/}));
    expect(within(panel).getByLabelText(/FR_GREETING/)).toBeInTheDocument();
  });

  it('retries a correction two refused resources share while either of them is still ticked', async () => {
    const orders = {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders'};
    const billing = {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing'};
    const {writes, sent, onDeployed} = correctionRoutes([orders, billing], ['app-1', 'app-2'], 0);
    renderCorrections(onDeployed);

    const panel = await failFirstCorrection();
    // Billing is no longer sent, but Orders, which uses the same value, still is.
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Billing/}));
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(writes).toEqual([expect.objectContaining({value: 'new'}), expect.objectContaining({value: 'new'})]);
    expect(sent).toEqual([expect.objectContaining({selection: ['application/app-1']})]);
  });

  it('keeps a correction while what uses it is unticked, and writes it once that is ticked again', async () => {
    const orders = {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders'};
    const catalog = {key: 'application/app-3', resourceType: 'application', id: 'app-3', name: 'Catalog'};
    const {writes, sent, onDeployed} = correctionRoutes([orders, catalog], ['app-1'], 1);
    renderCorrections(onDeployed);

    const panel = await failFirstCorrection();
    // A deploy of Catalog alone leaves the value unwritten, and fails.
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Orders/}));
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    await waitFor(() => expect(sent).toHaveLength(1));
    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());
    expect(writes).toHaveLength(1);

    await userEvent.click(within(panel).getByRole('checkbox', {name: /Orders/}));
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(writes).toEqual([expect.objectContaining({value: 'new'}), expect.objectContaining({value: 'new'})]);
    expect(sent).toEqual([
      expect.objectContaining({selection: ['application/app-3']}),
      expect.objectContaining({selection: ['application/app-1', 'application/app-3']}),
    ]);
  });

  it('retries a correction only while the deploy still sends what uses it, on the gateway it started for', async () => {
    const orders = {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders'};
    const billing = {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing'};
    const writes: Call[] = [];
    const sent: unknown[] = [];
    let release: () => void = () => undefined;
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {
        toVersion: version.version,
        changes: [
          {...orders, change: 'updated'},
          {...billing, change: 'updated'},
        ],
      },
      [`GET /configuration-versions/${version.version}`]: {
        ...version,
        references: [{name: 'ORDERS_URL', kind: 'variable', resourceType: 'application', resourceId: 'app-1'}],
      },
      'GET /gateways/gw-dev/variables': {totalResults: 1, variables: [{name: 'ORDERS_URL', value: 'old'}]},
      'GET /gateways/gw-dev/secrets': {totalResults: 0, secrets: []},
      'PUT /gateways/gw-dev/variables/ORDERS_URL': (call: Call) => {
        writes.push(call);
        // The first write fails only once the test lets it, so the dialog is seen while it runs.
        return new Promise((_resolve, reject) => {
          release = () => reject(Object.assign(new Error('down'), {response: {status: 503}}));
        });
      },
      'POST /gateways/gw-dev/apply': (call: Call) => {
        if ((call.data as {dryRun?: boolean}).dryRun) {
          return {
            gatewayId: 'gw-dev',
            dryRun: true,
            diff: {toVersion: version.version, changes: [{...orders, change: 'updated'}]},
            import: {
              results: [
                {resourceType: 'application', resourceId: 'app-1', status: 'failed', message: 'ORDERS_URL is invalid'},
              ],
            },
          };
        }
        sent.push(call.data);
        return {gatewayId: 'gw-dev', dryRun: false};
      },
    });
    const onDeployed = vi.fn();
    render(
      <ApplyPanel
        version={version}
        label="v2"
        gateways={[gateway, {id: 'gw-prod', name: 'Prod', baseUrl: 'https://prod.test', isDefault: false}]}
        gatewayId="gw-dev"
        onClose={vi.fn()}
        onDeployed={onDeployed}
      />,
    );

    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: 'Check with gateway'}));
    const input = await within(panel).findByLabelText('ORDERS_URL');
    await waitFor(() => expect(input).toHaveValue('old'));
    await userEvent.clear(input);
    await userEvent.type(input, 'new');
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
    await waitFor(() => expect(writes).toHaveLength(1));
    // While the deploy writes, the gateway it writes to cannot be changed.
    expect(within(panel).getByRole('combobox', {name: 'Gateway'})).toHaveAttribute('aria-disabled', 'true');
    release();
    await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());

    // Orders, which uses the correction, is no longer sent, so the retry leaves the value as it is.
    await userEvent.click(within(panel).getByRole('checkbox', {name: /Orders/}));
    await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));

    await waitFor(() => expect(onDeployed).toHaveBeenCalled());
    expect(writes).toHaveLength(1);
    expect(sent).toEqual([expect.objectContaining({selection: ['application/app-2']})]);
  });
});

interface Named {
  key: string;
  resourceType: string;
  id: string;
  name: string;
}

/**
 * Routes a deploy of the given changes in which the gateway's check refuses those with the ids given for
 * SHOP_URL, which they all use. The first write of the value fails, and so do as many deploys as given.
 */
function correctionRoutes(
  changes: Named[],
  using: string[],
  failedDeploys: number,
): {writes: unknown[]; sent: unknown[]; onDeployed: ReturnType<typeof vi.fn>} {
  const writes: unknown[] = [];
  const sent: unknown[] = [];
  const updated = changes.map((change: Named) => ({...change, change: 'updated'}));
  http = fakeHttp({
    'GET /gateways/gw-dev/diff': {toVersion: version.version, changes: updated},
    [`GET /configuration-versions/${version.version}`]: {
      ...version,
      references: using.map((id: string) => ({
        name: 'SHOP_URL',
        kind: 'variable',
        resourceType: 'application',
        resourceId: id,
      })),
    },
    'GET /gateways/gw-dev/variables': {totalResults: 1, variables: [{name: 'SHOP_URL', value: 'old'}]},
    'GET /gateways/gw-dev/secrets': {totalResults: 0, secrets: []},
    'PUT /gateways/gw-dev/variables/SHOP_URL': (call: Call) => {
      writes.push(call.data);
      return writes.length === 1
        ? Object.assign(new Error('down'), {response: {status: 503}})
        : {name: 'SHOP_URL', value: 'new'};
    },
    'POST /gateways/gw-dev/apply': (call: Call) => {
      if ((call.data as {dryRun?: boolean}).dryRun) {
        return {
          gatewayId: 'gw-dev',
          dryRun: true,
          diff: {toVersion: version.version, changes: updated},
          import: {
            results: using.map((id: string) => ({
              resourceType: 'application',
              resourceId: id,
              status: 'failed',
              message: 'SHOP_URL is invalid',
            })),
          },
        };
      }
      sent.push(call.data);
      return sent.length <= failedDeploys
        ? Object.assign(new Error('down'), {response: {status: 503}})
        : {gatewayId: 'gw-dev', dryRun: false};
    },
  });
  return {writes, sent, onDeployed: vi.fn()};
}

function renderCorrections(onDeployed: ReturnType<typeof vi.fn>): void {
  render(
    <ApplyPanel
      version={version}
      label="v2"
      gateways={[gateway]}
      gatewayId="gw-dev"
      onClose={vi.fn()}
      onDeployed={onDeployed}
    />,
  );
}

/** Has the gateway check the deploy, corrects SHOP_URL, and deploys, which fails at writing the value. */
async function failFirstCorrection(): Promise<HTMLElement> {
  const panel = await screen.findByTestId('deploy-apply-panel');
  await userEvent.click(await within(panel).findByRole('button', {name: 'Check with gateway'}));
  const input = (await within(panel).findAllByLabelText('SHOP_URL'))[0];
  await waitFor(() => expect(input).toHaveValue('old'));
  await userEvent.clear(input);
  await userEvent.type(input, 'new');
  await userEvent.click(within(panel).getByRole('button', {name: 'Deploy'}));
  await waitFor(() => expect(within(panel).getByRole('button', {name: 'Deploy'})).toBeEnabled());
  await waitFor(() => expect(within(panel).queryByTestId('refused-fixes')).not.toBeInTheDocument());
  return panel;
}
