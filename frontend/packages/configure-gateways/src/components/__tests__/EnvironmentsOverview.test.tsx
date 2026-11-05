// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import EnvironmentsOverview from '../EnvironmentsOverview';

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
  };
  http = fakeHttp(routes);
});

describe('EnvironmentsOverview', () => {
  it('shows the environments in pipeline order, named, each with its admin portal in a new tab', async () => {
    render(<EnvironmentsOverview onOpenEnvironment={navigate} />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    const prodCard = screen.getByTestId('deploy-stage-gw-prod');
    expect(within(devCard).getByText('Development')).toBeInTheDocument();
    expect(within(prodCard).getByText('Production')).toBeInTheDocument();
    expect(devCard.compareDocumentPosition(prodCard) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    const portal = within(prodCard).getByTestId('stage-admin-console');
    expect(portal).toHaveAttribute('href', 'https://admin.prod.test');
    expect(portal).toHaveAttribute('target', '_blank');
    expect(portal).toHaveAttribute('rel', 'noopener noreferrer');

    await userEvent.click(within(prodCard).getByTestId('stage-open'));
    expect(navigate).toHaveBeenCalledWith('gw-prod');
  });

  it('offers auto-sync on the first environment only, and turns it on', async () => {
    render(<EnvironmentsOverview />);

    const devCard = await screen.findByTestId('deploy-stage-gw-dev');
    expect(within(screen.getByTestId('deploy-stage-gw-prod')).queryByTestId('auto-apply')).not.toBeInTheDocument();
    expect(within(devCard).getByText('Auto-sync configuration to Development')).toBeInTheDocument();
    const toggle = within(devCard).getByRole('checkbox', {name: 'Auto-sync configuration to Development'});
    await waitFor(() => expect(toggle).toBeEnabled());
    expect(toggle).not.toBeChecked();

    await userEvent.click(toggle);

    await waitFor(() =>
      expect(http.calls).toContainEqual(
        expect.objectContaining({method: 'PUT', url: `${SERVER}/server-config/gateway`, data: {autoApply: true}}),
      ),
    );
  });

  it('keeps auto-sync as the deployment configuration sets it when it is set there', async () => {
    routes['GET /server-config/gateway'] = {readOnly: {autoApply: true}, writable: null, merged: {autoApply: true}};

    render(<EnvironmentsOverview />);

    const toggle = await screen.findByRole('checkbox', {name: 'Auto-sync configuration to Development'});
    await waitFor(() => expect(toggle).toBeChecked());
    expect(toggle).toBeDisabled();
  });

  it('promotes the version the environment before runs, to that environment only', async () => {
    render(<EnvironmentsOverview />);

    const promote = within(await screen.findByTestId('environment-promote-gw-prod')).getByRole('button', {
      name: 'Promote',
    });
    await waitFor(() => expect(promote).toBeEnabled());
    await userEvent.click(promote);

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/v2/)).toBeInTheDocument();
    expect(within(dialog).queryByRole('option', {name: 'dev-gateway'})).not.toBeInTheDocument();
  });

  it('draws the path: a rocket into the first environment and a paper plane into each later one', async () => {
    render(<EnvironmentsOverview />);

    const into = await screen.findByTestId('environment-connector-gw-dev');
    expect(within(into).getByTestId('path-rocket')).toBeInTheDocument();
    const promote = await screen.findByTestId('environment-promote-gw-prod');
    expect(within(promote).getByTestId('path-plane')).toBeInTheDocument();
  });

  it('has nothing to promote while the environments run the same version', async () => {
    routes['GET /gateways/gw-prod/applied-version'] = {gatewayId: 'gw-prod', appliedVersion: v2};

    render(<EnvironmentsOverview />);

    const promote = within(await screen.findByTestId('environment-promote-gw-prod')).getByRole('button', {
      name: 'Promote',
    });
    await waitFor(() => expect(screen.getByTestId('deploy-stage-gw-prod')).toBeInTheDocument());
    expect(promote).toBeDisabled();
  });

  it('describes one environment as the place configuration deploys to', async () => {
    routes['GET /gateways/pipeline'] = {
      stages: [{name: 'dev-gateway', adminConsoleUrl: 'https://dev.test/console', gateway: dev}],
    };

    render(<EnvironmentsOverview />);

    expect(await screen.findByText(/Your configuration deploys to dev-gateway/)).toBeInTheDocument();
    expect(screen.queryByText('Promote')).not.toBeInTheDocument();
  });

  it('says so when there is no environment, or the environments cannot be read', async () => {
    routes['GET /gateways/pipeline'] = {stages: []};
    const {unmount} = render(<EnvironmentsOverview />);
    expect(await screen.findByText('Register a gateway to have an environment to deploy to.')).toBeInTheDocument();
    unmount();

    routes['GET /gateways/pipeline'] = new Error('down');
    render(<EnvironmentsOverview />);
    expect(await screen.findByText('The environments could not be read.')).toBeInTheDocument();
  });
});
