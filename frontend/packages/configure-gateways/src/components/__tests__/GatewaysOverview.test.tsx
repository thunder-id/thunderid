// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import useResourceDeploy from '../../hooks/useResourceDeploy';
import GatewaysOverview from '../GatewaysOverview';

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
const v1 = 'aade3b4bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const versions = [
  {version: v2, createdAt: '2026-10-12T13:52:47Z', number: 2},
  {version: v1, createdAt: '2026-10-11T02:45:55Z', number: 1},
];
const dev = {id: 'gw-dev', name: 'dev-gateway', baseUrl: 'https://dev.test', isDefault: true};
const prod = {id: 'gw-prod', name: 'prod-gateway', baseUrl: 'https://prod.test'};

let routes: Record<string, unknown>;

beforeEach(() => {
  navigate.mockReset();
  routes = {
    'GET /gateways/pipeline': {
      stages: [
        {name: 'Development', adminConsoleUrl: 'https://dev.test/console', gateway: dev},
        {name: 'Production', adminConsoleUrl: 'https://admin.prod.test', gateway: prod},
      ],
    },
    'GET /configuration-versions': versions,
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev', appliedVersion: v2, previousVersion: v1},
    'GET /gateways/gw-prod/applied-version': {gatewayId: 'gw-prod', appliedVersion: v1},
    'GET /gateways/gw-dev/master': {entries: []},
    'GET /gateways/gw-prod/master': {entries: []},
    'GET /server-config/gateway': {readOnly: null, writable: null, merged: {autoApply: false}},
    'PUT /server-config/gateway': {},
    'GET /gateways/gw-dev/project-status': {
      project: {versions: [{hash: v2, number: 2, resources: 2}], pending: 0, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    },
    'GET /gateways/gw-prod/project-status': {
      project: {versions: [{hash: v1, number: 1, resources: 2}], pending: 1, required: 1},
      shared: {pending: 1, byType: [{resourceType: 'translation', count: 1}]},
      needed: {differing: 0},
    },
  };
  http = fakeHttp(routes);
});

describe('GatewaysOverview', () => {
  it('shows the gateways in pipeline order, named', async () => {
    render(<GatewaysOverview onOpenGateway={navigate} />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const prodCard = screen.getByTestId('deploy-stage-gw-prod');
    expect(within(devCard).getByText('Development')).toBeInTheDocument();
    expect(within(prodCard).getByText('Production')).toBeInTheDocument();
    expect(devCard.compareDocumentPosition(prodCard) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    expect(within(prodCard).queryByText('Open the admin portal')).not.toBeInTheDocument();

    await userEvent.click(within(prodCard).getByTestId('stage-open'));
    expect(navigate).toHaveBeenCalledWith('gw-prod');
  });

  it('opens a gateway on its own page', async () => {
    render(<GatewaysOverview />);

    await userEvent.click(within(await screen.findByTestId('deploy-stage-gw-dev')).getByTestId('stage-open'));

    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-dev');
  });

  it('opens a gateway within the project it is shown for', async () => {
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    await userEvent.click(within(await screen.findByTestId('deploy-stage-gw-dev')).getByTestId('stage-open'));

    expect(navigate).toHaveBeenCalledWith('/projects/p-shop/deploy/gateways/gw-dev');
  });

  it("explores a gateway on the project's own gateway page", async () => {
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await userEvent.click(within(prodCard).getByRole('button', {name: 'Explore Production'}));
    expect(navigate).toHaveBeenCalledWith('/projects/p-shop/deploy/gateways/gw-prod/explore');
  });

  it('explores a gateway on its own pages outside a project', async () => {
    render(<GatewaysOverview />);

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await userEvent.click(within(prodCard).getByRole('button', {name: 'Explore Production'}));
    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-prod/explore');
  });

  it('shows a project that runs in one gateway as a pipeline of one, beside the versions to deploy to it', async () => {
    routes['GET /gateways/pipeline'] = {stages: [{name: 'Development', gateway: dev}]};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const strip = await screen.findByTestId('gateways-strip');
    expect(within(strip).getByTestId('deploy-stage-gw-dev')).toBeInTheDocument();
    expect(within(strip).queryByRole('button', {name: /Promote/})).not.toBeInTheDocument();
    expect(
      screen.getByText('This project runs in one gateway. Your configuration deploys straight to it.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Project versions')).toBeInTheDocument();
    expect(screen.getByTestId('gateway-connector-gw-dev')).toBeInTheDocument();
  });

  it("names the versions as the project's on a project home", async () => {
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    expect(await screen.findByText('Project versions')).toBeInTheDocument();
    expect(
      screen.getByText('Every save creates a version. Deploys always enter the pipeline at Development.'),
    ).toBeInTheDocument();
  });

  it('opens a gateway within the project it is shown for', async () => {
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    await userEvent.click(within(await screen.findByTestId('deploy-stage-gw-dev')).getByTestId('stage-open'));

    expect(navigate).toHaveBeenCalledWith('/projects/p-shop/deploy/gateways/gw-dev');
  });

  it("explores a gateway on the project's own gateway page", async () => {
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await userEvent.click(within(prodCard).getByRole('button', {name: 'Explore Production'}));
    expect(navigate).toHaveBeenCalledWith('/projects/p-shop/deploy/gateways/gw-prod/explore');
  });

  it('explores a gateway on its own pages outside a project', async () => {
    render(<GatewaysOverview />);

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await userEvent.click(within(prodCard).getByRole('button', {name: 'Explore Production'}));
    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-prod/explore');
  });

  it("marks a version with the gateways the project's own resources there came from", async () => {
    // The gateway records v2 on Production, but the project's own resources there came from v1.
    routes['GET /gateways/gw-prod/applied-version'] = {gatewayId: 'gw-prod', appliedVersion: v2};
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const older = await screen.findByTestId(`pipeline-version-${v1}`);
    await waitFor(() => expect(within(older).getByText('Production')).toBeInTheDocument());
    expect(within(screen.getByTestId(`pipeline-version-${v2}`)).queryByText('Production')).not.toBeInTheDocument();
    expect(within(screen.getByTestId('deploy-stage-gw-prod')).getByTestId('card-version')).toHaveTextContent('v1');
  });

  it('says the gateway was rolled back when the project was last rolled back there', async () => {
    routes['GET /gateways/gw-prod/project-status'] = {
      project: {
        versions: [{hash: v1, number: 1, resources: 2}],
        pending: 0,
        required: 0,
        deployedAt: new Date(Date.now() - 3600_000).toISOString(),
        lastAction: 'rolledBack',
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await waitFor(() =>
      expect(within(prodCard).getByTestId('card-deployed')).toHaveTextContent(/^Rolled back 1 hour ago/),
    );
  });

  it("names, in the drawer's steps, the version the project's own resources run on each gateway", async () => {
    // Development records v2, but the project's own resources there came from v1.
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v1, number: 1, resources: 1}], pending: 1, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const latest = await screen.findByTestId(`pipeline-version-${v2}`);
    await userEvent.click(within(latest).getByRole('button', {name: 'Deploy'}));
    const steps = within(await screen.findByTestId('deploy-apply-panel')).getByTestId('pipeline-steps');
    expect(within(steps).getByText('Development').closest('li')).toHaveTextContent('Runs v1');
  });

  it("keeps the latest version's Deploy the main one when it brings none of the project's changes", async () => {
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev', appliedVersion: v1};
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v1, number: 1, resources: 1}], pending: 0, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const latest = await screen.findByTestId(`pipeline-version-${v2}`);
    await waitFor(() =>
      expect(within(latest).getByRole('button', {name: 'Deploy'})).toHaveClass('MuiButton-contained'),
    );
  });

  it("keeps a card's rollback and promote on one row", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v2, number: 2, resources: 1}], pending: 2, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      rollback: {available: true, toVersion: {hash: v1, number: 1}},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const promote = await within(devCard).findByTestId('card-promote');
    const rollback = within(devCard).getByTestId('card-revert');
    await waitFor(() => expect(rollback).toHaveTextContent('Roll back to v1'));
    // Their middles line up: neither wrapped onto a row of its own.
    const middle = (element: HTMLElement): number => {
      const box = element.getBoundingClientRect();
      return box.top + box.height / 2;
    };
    expect(Math.abs(middle(rollback) - middle(promote))).toBeLessThanOrEqual(4);
  });

  it("counts only the project's own refused resources, and says refused shared ones apart", async () => {
    // The gateway refused three resources; one is another project's.
    routes['GET /gateways/gw-dev/master'] = {
      entries: [
        {key: 'application/shop', resourceType: 'application', state: 'failed'},
        {key: 'translation/en', resourceType: 'translation', state: 'failed'},
        {key: 'application/bill', resourceType: 'application', state: 'failed'},
      ],
    };
    routes['POST /gateways/gw-dev/apply'] = {
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        toVersion: v2,
        changes: [
          {
            key: 'application/shop',
            resourceType: 'application',
            name: 'shop',
            change: 'updated',
            failed: true,
            section: 'project',
          },
          {
            key: 'translation/en',
            resourceType: 'translation',
            name: 'en',
            change: 'updated',
            failed: true,
            section: 'shared',
          },
          {
            key: 'application/bill',
            resourceType: 'application',
            name: 'bill',
            change: 'updated',
            failed: true,
            section: 'other',
          },
        ],
      },
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(await within(devCard).findByTestId('stage-refused')).toHaveTextContent('1 refused, not running');
    expect(within(devCard).getByTestId('card-shared-refused')).toHaveTextContent('1 shared resource refused');
  });

  it('counts every refused resource of the gateway outside a project', async () => {
    routes['GET /gateways/gw-dev/master'] = {
      entries: [
        {key: 'application/shop', resourceType: 'application', state: 'failed'},
        {key: 'application/bill', resourceType: 'application', state: 'failed'},
      ],
    };

    render(<GatewaysOverview />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(await within(devCard).findByTestId('stage-refused')).toHaveTextContent('2 refused, not running');
    expect(within(devCard).queryByTestId('card-shared-refused')).not.toBeInTheDocument();
  });

  it("says when a project's auto-sync failed, and retries it as a deploy", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [], pending: 1, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      autoSync: {enabled: true, inherited: false, lastRun: {at: '2026-11-06T10:00:00Z', error: 'flow login refused'}},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(await within(devCard).findByTestId('card-auto-sync-failed')).toHaveTextContent(
      'Auto-sync failed: flow login refused',
    );
    await userEvent.click(within(devCard).getByTestId('card-auto-sync-retry'));

    expect(await screen.findByTestId('deploy-apply-panel')).toBeInTheDocument();
  });

  it('promotes the version the gateway before runs, to that gateway only', async () => {
    render(<GatewaysOverview />);

    const promote = await within(await screen.findByTestId('deploy-stage-gw-dev')).findByTestId('card-promote');
    await userEvent.click(promote);

    const dialog = await screen.findByTestId('deploy-apply-panel');
    // A later gateway is promoted to, from the one before it.
    expect(within(dialog).getByText('Promote to Production')).toBeInTheDocument();
    expect(within(dialog).getByText(/Carries v2 from/)).toBeInTheDocument();
    const steps = within(dialog).getByTestId('pipeline-steps');
    expect(within(steps).getByText('This deploy').closest('li')).toHaveTextContent('Production');
    // Auto-sync belongs to the first gateway, not to a promotion.
    expect(within(dialog).queryByTestId('auto-apply')).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('option', {name: 'dev-gateway'})).not.toBeInTheDocument();
  });

  it('promotes a project with its own changes and what they lack, the shared ones the target lacks, and what is ticked', async () => {
    routes['POST /gateways/gw-prod/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {
            gatewayId: 'gw-prod',
            dryRun: true,
            diff: {
              toVersion: v2,
              summary: {added: 3, updated: 2, deleted: 0, unchanged: 0},
              changes: [
                {
                  key: 'application/shop',
                  resourceType: 'application',
                  id: 'shop',
                  change: 'updated',
                  section: 'project',
                },
                {
                  key: 'flow/login',
                  resourceType: 'flow',
                  id: 'login',
                  change: 'added',
                  section: 'needed',
                  required: true,
                },
                {key: 'flow/common', resourceType: 'flow', id: 'common', change: 'updated', section: 'needed'},
                {
                  key: 'translation/en',
                  resourceType: 'translation',
                  id: 'en',
                  name: 'en',
                  change: 'added',
                  section: 'shared',
                },
                {
                  key: 'theme/teal',
                  resourceType: 'theme',
                  id: 'teal',
                  name: 'Teal',
                  change: 'updated',
                  section: 'shared',
                },
                {key: 'application/bill', resourceType: 'application', id: 'bill', change: 'added', section: 'other'},
              ],
            },
          }
        : {gatewayId: 'gw-prod', dryRun: false, recorded: true, diff: {changes: []}};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const promote = await within(await screen.findByTestId('deploy-stage-gw-dev')).findByTestId('card-promote');
    await userEvent.click(promote);
    const panel = await screen.findByTestId('deploy-apply-panel');
    // A shared resource the target lacks goes by default; one it holds is replaced only when ticked.
    const en = await within(panel).findByRole('checkbox', {name: 'Write en'});
    expect(en).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: 'Write Teal'})).not.toBeChecked();
    expect(within(panel).queryByTestId('deploy-shared-left-out')).not.toBeInTheDocument();
    expect(within(panel).queryByTestId('deploy-include-needed')).not.toBeInTheDocument();

    // Leaving it out is warned about, and the promote says it goes anyway.
    await userEvent.click(en);
    const warning = within(panel).getByTestId('deploy-shared-left-out');
    expect(warning).toHaveTextContent("Some shared resources won't be on Production");
    expect(warning).toHaveTextContent(
      "1 shared resource this gateway doesn't have yet is left out: en. Sign-in pages, branding or text in Production may not work as expected until it is deployed.",
    );
    expect(within(panel).getByRole('button', {name: /Promote anyway/})).toBeInTheDocument();

    // Bringing every shared update ticks the ones the target holds too.
    await userEvent.click(
      within(panel).getByTestId('deploy-changes-bring-all-shared').querySelector('input') as HTMLElement,
    );
    expect(within(panel).getByRole('checkbox', {name: 'Write en'})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: 'Write Teal'})).toBeChecked();
    expect(within(panel).queryByTestId('deploy-shared-left-out')).not.toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('checkbox', {name: 'Write Teal'}));
    await userEvent.click(within(panel).getByRole('button', {name: /Promote 3 changes to Production/}));

    await waitFor(() =>
      expect(http.calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          url: `${SERVER}/gateways/gw-prod/apply`,
          data: {
            version: v2,
            selection: ['application/shop', 'translation/en', 'flow/login'],
            withDependencies: false,
            project: 'p-shop',
          },
        }),
      ),
    );
  });

  it('deploys as the whole organization does when the check places no change in a section', async () => {
    routes['POST /gateways/gw-prod/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {
            gatewayId: 'gw-prod',
            dryRun: true,
            diff: {
              toVersion: v2,
              summary: {added: 1, updated: 0, deleted: 0, unchanged: 0},
              changes: [{key: 'application/shop', resourceType: 'application', id: 'shop', change: 'added'}],
            },
          }
        : {gatewayId: 'gw-prod', dryRun: false, recorded: true, diff: {changes: []}};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const promote = await within(await screen.findByTestId('deploy-stage-gw-dev')).findByTestId('card-promote');
    await userEvent.click(promote);
    const panel = await screen.findByTestId('deploy-apply-panel');
    await userEvent.click(await within(panel).findByRole('button', {name: /Promote 1 change to Production/}));

    await waitFor(() =>
      expect(http.calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          url: `${SERVER}/gateways/gw-prod/apply`,
          data: {version: v2, selection: ['application/shop']},
        }),
      ),
    );
  });

  it('promotes one resource on its own, from what the gateway before runs, to the gateway named', async () => {
    routes['POST /gateways/gw-prod/apply'] = (call: Call) =>
      (call.data as {dryRun?: boolean}).dryRun
        ? {
            gatewayId: 'gw-prod',
            dryRun: true,
            diff: {
              toVersion: v2,
              changes: [
                {
                  key: 'application/shop',
                  resourceType: 'application',
                  id: 'shop',
                  name: 'Shop',
                  change: 'updated',
                  section: 'project',
                },
                {
                  key: 'application/cart',
                  resourceType: 'application',
                  id: 'cart',
                  name: 'Cart',
                  change: 'updated',
                  section: 'project',
                },
              ],
            },
          }
        : {gatewayId: 'gw-prod', dryRun: false, recorded: true, diff: {changes: []}};
    function OpenOne() {
      const resource = useResourceDeploy();
      return (
        <>
          <button type="button" onClick={() => resource.open('application', 'shop', 'Production')}>
            promote shop
          </button>
          {resource.panels}
        </>
      );
    }

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <OpenOne />
      </ProjectScopeProvider>,
    );

    await waitFor(async () => {
      await userEvent.click(screen.getByRole('button', {name: 'promote shop'}));
      expect(screen.getByTestId('deploy-apply-panel')).toBeInTheDocument();
    });
    const panel = screen.getByTestId('deploy-apply-panel');
    expect(within(panel).getByText('Promote to Production')).toBeInTheDocument();
    expect(await within(panel).findByRole('checkbox', {name: 'Write Shop'})).toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: 'Write Cart'})).not.toBeChecked();
    expect(within(panel).getByRole('checkbox', {name: 'Write Cart'})).toBeEnabled();
  });

  it('draws the path: a deploy into the first gateway and a promote into each later one', async () => {
    render(<GatewaysOverview />);

    const into = await screen.findByTestId('gateway-connector-gw-dev');
    expect(within(into).getByText('Deploy')).toBeInTheDocument();
    const promote = await screen.findByTestId('gateway-promote-gw-prod');
    expect(within(promote).getByText('Promote')).toBeInTheDocument();
    expect(within(promote).queryByRole('button')).not.toBeInTheDocument();
  });

  it('has nothing to promote while the gateways run the same version', async () => {
    routes['GET /gateways/gw-prod/applied-version'] = {gatewayId: 'gw-prod', appliedVersion: v2};

    render(<GatewaysOverview />);

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await waitFor(() => expect(within(prodCard).getByTestId('card-standing')).toHaveTextContent('Matches Development'));
    expect(within(screen.getByTestId('deploy-stage-gw-dev')).queryByTestId('card-promote')).not.toBeInTheDocument();
  });

  it('shows one gateway on its own, with nothing to promote or scroll', async () => {
    routes['GET /gateways/pipeline'] = {
      stages: [{name: 'dev-gateway', adminConsoleUrl: 'https://dev.test/console', gateway: dev}],
    };

    render(<GatewaysOverview />);

    expect(await screen.findByText(/then deploy them to your gateway/)).toBeInTheDocument();
    expect(await screen.findByTestId('deploy-stage-gw-dev')).toBeInTheDocument();
    expect(screen.queryByText('Promote')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Show more gateways'})).not.toBeInTheDocument();
    expect(screen.getByText('1 gateway')).toBeInTheDocument();
  });

  it('marks the versions the gateways run, and offers each to deploy', async () => {
    render(<GatewaysOverview />);

    const latest = await screen.findByTestId(`pipeline-version-${v2}`);
    await waitFor(() => expect(within(latest).getByText('Development')).toBeInTheDocument());
    expect(within(latest).getByRole('button', {name: 'Deploy'})).toHaveClass('MuiButton-contained');
    expect(within(screen.getByTestId(`pipeline-version-${v1}`)).getByText('Production')).toBeInTheDocument();
    expect(within(screen.getByTestId('deploy-stage-gw-dev')).getByTestId('card-standing')).toHaveTextContent(
      'In sync with configuration',
    );
    expect(within(screen.getByTestId('deploy-stage-gw-prod')).getByTestId('card-standing')).toHaveTextContent(
      '1 version behind Development',
    );
    expect(within(screen.getByTestId('deploy-stage-gw-dev')).getByTestId('card-revert')).toHaveTextContent(
      'Roll back to v1',
    );
    expect(within(screen.getByTestId('deploy-stage-gw-prod')).getByTestId('card-revert')).toBeDisabled();
  });

  it('says how many changes the first gateway is behind, and offers the latest to deploy', async () => {
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev', appliedVersion: v1};
    routes['POST /gateways/gw-dev/apply'] = {
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {
        toVersion: v2,
        summary: {added: 1, updated: 1, deleted: 0, unchanged: 3},
        changes: [
          {key: 'application/a', resourceType: 'application', id: 'a', change: 'added'},
          {key: 'flow/f', resourceType: 'flow', id: 'f', change: 'updated'},
          {key: 'flow/g', resourceType: 'flow', id: 'g', change: 'unchanged'},
        ],
      },
    };

    render(<GatewaysOverview />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    await waitFor(() => expect(within(devCard).getByTestId('card-standing')).toHaveTextContent('2 changes behind'));
    expect(
      within(screen.getByTestId(`pipeline-version-${v2}`)).getByRole('button', {name: 'Deploy'}),
    ).toBeInTheDocument();
    expect(within(screen.getByTestId('deploy-stage-gw-prod')).getByTestId('card-standing')).toHaveTextContent(
      'Matches Development',
    );
  });

  it('says how a project stands from its own resources, asking each gateway against the one before', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v2, number: 2, resources: 1}], pending: 1, required: 1},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      deployedAt: '2026-10-12T13:52:47Z',
      deployedBy: {id: 'u-me'},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview eyebrow="Shop" />
      </ProjectScopeProvider>,
    );

    expect(await screen.findByText('SHOP')).toBeInTheDocument();
    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    await waitFor(() => expect(within(devCard).getByTestId('card-standing')).toHaveTextContent('2 changes behind'));
    expect(within(devCard).getByTestId('card-version')).toHaveTextContent('v2');
    expect(
      within(screen.getByTestId(`pipeline-version-${v2}`)).getByText(/Latest · .* · 2 changes/),
    ).toBeInTheDocument();
    const prodCard = screen.getByTestId('deploy-stage-gw-prod');
    await waitFor(() =>
      expect(within(prodCard).getByTestId('card-standing')).toHaveTextContent('2 behind Development'),
    );
    expect(http.calls).toContainEqual(
      expect.objectContaining({method: 'GET', url: `${SERVER}/gateways/gw-dev/project-status?project=p-shop`}),
    );
    expect(http.calls).toContainEqual(
      expect.objectContaining({
        method: 'GET',
        url: `${SERVER}/gateways/gw-prod/project-status?project=p-shop&from=gw-dev`,
      }),
    );
  });

  it('tells shared updates apart from the project, and names a project that runs on more than one version', async () => {
    routes['GET /gateways/gw-prod/project-status'] = {
      project: {
        versions: [
          {hash: v2, number: 2, resources: 2},
          {hash: v1, number: 1, resources: 1},
        ],
        pending: 0,
        required: 0,
      },
      shared: {pending: 2, byType: [{resourceType: 'translation', count: 2}]},
      needed: {differing: 1},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await waitFor(() => expect(within(prodCard).getByTestId('card-standing')).toHaveTextContent('Matches Development'));
    expect(within(prodCard).getByTestId('card-version')).toHaveTextContent('v2');
    expect(within(prodCard).getByTestId('card-partly')).toHaveTextContent('Partly on v1');
    expect(within(prodCard).getByTestId('card-shared')).toHaveTextContent('2 shared updates available');
    expect(within(prodCard).getByTestId('card-needed-differing')).toHaveTextContent(
      'Uses 1 needed resource in another form than Development',
    );
    // Shared updates alone are worth a promotion, though the project is up to date.
    expect(await within(screen.getByTestId('deploy-stage-gw-dev')).findByTestId('card-promote')).toBeInTheDocument();
  });

  it('is in sync for a project with nothing of its own waiting, and offers no deploy', async () => {
    routes['GET /gateways/gw-dev/applied-version'] = {gatewayId: 'gw-dev', appliedVersion: v1};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    await waitFor(() =>
      expect(within(devCard).getByTestId('card-standing')).toHaveTextContent('In sync with configuration'),
    );
    expect(within(devCard).queryByTestId('card-deploy')).not.toBeInTheDocument();
  });

  it('says when a project has nothing on a gateway yet', async () => {
    routes['GET /gateways/gw-prod/project-status'] = {
      project: {versions: [], pending: 1, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    await waitFor(() =>
      expect(within(prodCard).getByTestId('card-deployed')).toHaveTextContent('No project resources yet'),
    );
    expect(within(prodCard).queryByTestId('card-version')).not.toBeInTheDocument();
  });

  it("says inline when a project's status cannot be read, never the whole gateway, and reads it again", async () => {
    routes['GET /gateways/gw-prod/project-status'] = Object.assign(new Error('boom'), {response: {status: 500}});

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const prodCard = await screen.findByTestId('deploy-stage-gw-prod');
    expect(await within(prodCard).findByTestId('card-status-failed')).toHaveTextContent(
      "This project's status here could not be read.",
    );
    expect(within(prodCard).queryByTestId('card-standing')).not.toBeInTheDocument();
    expect(within(prodCard).queryByTestId('card-version')).not.toBeInTheDocument();

    routes['GET /gateways/gw-prod/project-status'] = {
      project: {versions: [{hash: v1, number: 1, resources: 2}], pending: 0, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };
    await userEvent.click(within(prodCard).getByTestId('card-status-retry'));
    await waitFor(() => expect(within(prodCard).getByTestId('card-standing')).toHaveTextContent('Matches Development'));
  });

  it('offers no more actions menu on a card, and shows the gateway name as a heading', async () => {
    render(<GatewaysOverview />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(within(devCard).getByRole('heading', {level: 3, name: 'Development'})).toBeInTheDocument();
    expect(within(devCard).queryByTestId('card-more')).not.toBeInTheDocument();
    expect(within(devCard).queryByTestId('card-configure')).not.toBeInTheDocument();
  });

  it('opens every version in a dialog', async () => {
    routes['GET /configuration-versions'] = Array.from({length: 7}, (_, index) => ({
      version: `${index}`.padStart(64, 'c'),
      number: 7 - index,
      createdAt: '2026-10-12T13:52:47Z',
    }));

    render(<GatewaysOverview />);

    await userEvent.click(await screen.findByRole('button', {name: 'View all versions'}));
    const dialog = await screen.findByTestId('pipeline-all-versions');
    expect(within(dialog).getByText('v1')).toBeInTheDocument();
  });

  it('names production as where promotions end only when the last gateway is production', async () => {
    render(<GatewaysOverview />);

    expect(
      await screen.findByText(/then promote them through each gateway to production, just like a CI\/CD pipeline/),
    ).toBeInTheDocument();
  });

  it("rolls back only the project's own resources", async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {versions: [{hash: v2, number: 2, resources: 1}], pending: 0, required: 0},
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
      rollback: {available: true, toVersion: {hash: v1, number: 1}},
    };
    routes['POST /gateways/gw-dev/revert'] = {gatewayId: 'gw-dev', dryRun: true, diff: {changes: []}};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const revert = await within(devCard).findByRole('button', {name: 'Roll back to v1'});
    await userEvent.click(revert);
    const panel = await screen.findByTestId('deploy-revert-panel');
    expect(within(panel).getByTestId('deploy-revert-what')).toHaveTextContent(
      "Only this project's applications and agents on dev-gateway go back one step.",
    );
    await waitFor(() =>
      expect(http.calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          url: `${SERVER}/gateways/gw-dev/revert`,
          data: {dryRun: true, project: 'p-shop'},
        }),
      ),
    );
  });

  it('undoes the last deploy when the rollback stays on the version the project runs, saying how much it changes', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        rollback: {available: true, toVersion: {hash: v2, number: 2}, changes: 3, sameVersion: true},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };
    routes['POST /gateways/gw-dev/revert'] = {gatewayId: 'gw-dev', dryRun: true, diff: {changes: []}};

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const undo = await within(devCard).findByRole('button', {name: 'Undo last deploy'});
    expect(within(devCard).queryByText('Roll back to v2')).not.toBeInTheDocument();
    await userEvent.hover(undo);
    expect(
      await screen.findByText(
        "Puts this project's resources back as they were before the last deploy of v2 (3 changes).",
      ),
    ).toBeInTheDocument();
    await userEvent.click(undo);
    const panel = await screen.findByTestId('deploy-revert-panel');
    expect(within(panel).getByText('Undo the last deploy to Development')).toBeInTheDocument();
  });

  it('undoes the last deploy when the rollback names the version the project runs, without the flag', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        rollback: {available: true, toVersion: {hash: v2, number: 2}},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(await within(devCard).findByRole('button', {name: 'Undo last deploy'})).toBeInTheDocument();
  });

  it('rolls back to an older version by name, with how much it changes in the hint', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        rollback: {available: true, toVersion: {hash: v1, number: 1}, changes: 1, sameVersion: false},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const revert = await within(devCard).findByRole('button', {name: 'Roll back to v1'});
    await userEvent.hover(revert);
    expect(
      await screen.findByText(/back one step\. Shared resources and other projects stay as they are\. \(1 change\)$/),
    ).toBeInTheDocument();
  });

  it('offers no rollback when the status says nothing would change', async () => {
    routes['GET /gateways/gw-dev/project-status'] = {
      project: {
        versions: [{hash: v2, number: 2, resources: 1}],
        pending: 0,
        required: 0,
        rollback: {available: false, toVersion: {hash: v2, number: 2}, changes: 0, sameVersion: true},
      },
      shared: {pending: 0, byType: []},
      needed: {differing: 0},
    };

    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <GatewaysOverview />
      </ProjectScopeProvider>,
    );

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    await waitFor(() => expect(within(devCard).getByTestId('card-revert')).toBeDisabled());
    expect(within(devCard).queryByText('Undo last deploy')).not.toBeInTheDocument();
  });

  it('says so when there is no gateway, or the gateways cannot be read', async () => {
    routes['GET /gateways/pipeline'] = {stages: []};
    const {unmount} = render(<GatewaysOverview />);
    expect(await screen.findByText('Register a gateway to have a gateway to deploy to.')).toBeInTheDocument();
    unmount();

    routes['GET /gateways/pipeline'] = new Error('down');
    render(<GatewaysOverview />);
    expect(await screen.findByText('The gateways could not be read.')).toBeInTheDocument();
  });
});
