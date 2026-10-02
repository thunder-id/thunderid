// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiError, createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import type {GatewayDiff} from '../../models/gateway';
import ApplyVersionDialog from '../ApplyVersionDialog';
import DeployedConfigurationCard from '../DeployedConfigurationCard';
import RevertDialog from '../RevertDialog';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => SERVER_URL})};
});

const mockNavigate = vi.fn();
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

const versions = [{version: 2, note: 'Payments'}, {version: 1}];

const diffTo = (toVersion: number): GatewayDiff => ({
  fromVersion: 1,
  toVersion,
  summary: {added: 1, updated: 0, deleted: 1, unchanged: 2},
  changes: [
    {resourceType: 'application', id: 'app-1', name: 'Payments', change: 'added'},
    {resourceType: 'flow', id: 'flow-9', name: 'Old flow', change: 'deleted'},
  ],
});

const applied = (dryRun: boolean) => ({
  gatewayId: 'gw-1',
  dryRun,
  recorded: !dryRun,
  import: {summary: {totalDocuments: 3, imported: 3, failed: 0}, results: []},
});

/**
 * Answers a dry run with what the gateway lacks until the matching create lands, the way the
 * gateway's store would.
 */
function missingUntilSet(base: ReturnType<typeof applied>, names: {variables?: string[]; secrets?: string[]}) {
  const held = new Set<string>();
  return {
    respond: (call: HttpCall) => {
      const dryRun = (call.data as {dryRun: boolean}).dryRun;
      const variables = (names.variables ?? []).filter((name: string) => !held.has(`variable:${name}`));
      const secrets = (names.secrets ?? []).filter((name: string) => !held.has(`secret:${name}`));
      const missing = variables.length + secrets.length > 0 ? {variables, secrets} : undefined;
      return {...base, dryRun, recorded: !dryRun, ...(missing ? {missing} : {})};
    },
    setVariable: (call: HttpCall) => {
      held.add(`variable:${(call.data as {name: string}).name}`);
      return call.data;
    },
    setSecret: (call: HttpCall) => {
      held.add(`secret:${(call.data as {name: string}).name}`);
      return {name: (call.data as {name: string}).name, exists: true};
    },
  };
}

beforeEach(() => {
  mockHttpRequest.mockReset();
  mockNavigate.mockReset();
  mockNavigate.mockResolvedValue(undefined);
});

describe('ApplyVersionDialog', () => {
  it('shows the diff of the latest version and applies it', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': applied(false),
      }),
    );
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    expect(await screen.findByText('Changes from version 1 to version 2')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toHaveTextContent('Latest (version 2)');
    expect(screen.getByText('1 added')).toBeInTheDocument();
    expect(screen.getByText('Old flow')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByRole('button', {name: 'Apply'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Apply'}));

    expect(await screen.findByText('The gateway now holds this configuration.')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {version: '2', dryRun: false}}),
    );
  });

  it('compares the version picked, and runs a dry run before applying for real', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'GET /gateways/gw-1/diff?version=1': {...diffTo(1), fromVersion: undefined},
        'POST /gateways/gw-1/apply': (call: HttpCall) => applied((call.data as {dryRun: boolean}).dryRun),
      }),
    );
    const onClose = vi.fn();
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={onClose} />);

    await screen.findByText('Changes from version 1 to version 2');
    await user.click(screen.getByRole('combobox'));
    await user.click(within(screen.getByRole('listbox')).getByText('Version 1'));
    expect(await screen.findByText('Changes to apply version 1')).toBeInTheDocument();

    await user.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(screen.getByRole('button', {name: 'Run dry run'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Run dry run'}));

    expect(await screen.findByText(/Dry run finished. Nothing was changed/)).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {version: '1', dryRun: true}}));

    await user.click(screen.getByRole('button', {name: 'Apply now'}));
    expect(await screen.findByText('The gateway now holds this configuration.')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {version: '1', dryRun: false}}));

    await user.click(screen.getByRole('button', {name: 'Done'}));
    expect(onClose).toHaveBeenCalled();
  });

  it('applies the version that was reviewed, even after another capture moves latest', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': (call: HttpCall) => applied((call.data as {dryRun: boolean}).dryRun),
      }),
    );
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    await screen.findByText('Changes from version 1 to version 2');
    await user.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(screen.getByRole('button', {name: 'Run dry run'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Run dry run'}));
    await screen.findByText(/Dry run finished. Nothing was changed/);

    await user.click(screen.getByRole('button', {name: 'Apply now'}));
    expect(await screen.findByText('The gateway now holds this configuration.')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {version: '2', dryRun: true}}));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {version: '2', dryRun: false}}));
    expect(mockHttpRequest).not.toHaveBeenCalledWith(
      expect.objectContaining({data: {version: 'latest', dryRun: false}}),
    );
  });

  it('says so when the gateway cannot be reached', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation((call: HttpCall) => {
      if (call.method === 'POST') return Promise.reject(apiError(502));
      return createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
      })(call);
    });
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    await screen.findByText('Changes from version 1 to version 2');
    // The check cannot reach the gateway either, which does not hold the apply back.
    expect(await screen.findByText(/Could not check whether the gateway holds the values/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Apply'}));

    expect(await screen.findByText(/The gateway could not be reached/)).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
  });

  it('asks for a capture when there is nothing to apply', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /configuration-versions': []}));
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    expect(await screen.findByText(/No configuration has been captured yet/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Apply'})).toBeDisabled();
  });

  it('cannot be reviewed when the diff fails to load', async () => {
    mockHttpRequest.mockImplementation((call: HttpCall) => {
      if (call.url.includes('/diff')) return Promise.reject(apiError(500));
      return createHttpRouter({'GET /configuration-versions': versions})(call);
    });
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    expect(await screen.findByText(/this apply cannot be reviewed/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Apply'})).toBeDisabled();
  });

  it('checks for missing values on open, lets them be set in place, and enables the apply once none are missing', async () => {
    const user = userEvent.setup();
    const gateway = missingUntilSet(applied(false), {variables: ['API_URL'], secrets: ['CLIENT_SECRET']});
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': gateway.respond,
        'POST /gateways/gw-1/variables': gateway.setVariable,
        'POST /gateways/gw-1/secrets': gateway.setSecret,
      }),
    );
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    expect(await screen.findByText('1 secret is not configured')).toBeInTheDocument();
    expect(screen.getByText('1 variable has no value')).toBeInTheDocument();
    expect(screen.getByText('CLIENT_SECRET')).toBeInTheDocument();
    expect(screen.getByText('API_URL')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {version: 'latest', dryRun: true}}),
    );
    expect(screen.getByRole('button', {name: 'Apply'})).toBeDisabled();

    await user.click(screen.getByRole('button', {name: 'Set values'}));
    const secretInput = screen.getByLabelText('CLIENT_SECRET');
    expect(secretInput).toHaveAttribute('type', 'password');
    await user.type(secretInput, 's3cret');
    await user.type(screen.getByLabelText('API_URL'), 'https://api.example.com');
    await user.click(screen.getByRole('button', {name: 'Save values'}));

    await waitFor(() => expect(screen.queryByText('1 secret is not configured')).not.toBeInTheDocument());
    expect(screen.queryByText('1 variable has no value')).not.toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${SERVER_URL}/gateways/gw-1/secrets`,
        method: 'POST',
        data: {name: 'CLIENT_SECRET', value: 's3cret'},
      }),
    );
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${SERVER_URL}/gateways/gw-1/variables`,
        method: 'POST',
        data: {name: 'API_URL', value: 'https://api.example.com'},
      }),
    );
    await waitFor(() => expect(screen.getByRole('button', {name: 'Apply'})).toBeEnabled());
  });

  it('keeps what was typed when a value cannot be set', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation((call: HttpCall) => {
      if (call.url.endsWith('/variables')) return Promise.reject(apiError(409, 'VAR-1009'));
      return createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': {...applied(true), missing: {variables: ['API_URL']}},
      })(call);
    });
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    await user.click(await screen.findByRole('button', {name: 'Set values'}));
    await user.type(screen.getByLabelText('API_URL'), 'https://api.example.com');
    await user.click(screen.getByRole('button', {name: 'Save values'}));

    expect(await screen.findByText('The gateway already holds something by that name.')).toBeInTheDocument();
    expect(screen.getByLabelText('API_URL')).toHaveValue('https://api.example.com');
    expect(screen.getByRole('button', {name: 'Apply'})).toBeDisabled();
  });

  it('leads to the secrets and the variables of the gateway', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': {...applied(true), missing: {variables: ['API_URL'], secrets: ['CLIENT_SECRET']}},
      }),
    );
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={onClose} />);

    await user.click(await screen.findByRole('button', {name: 'Manage secrets'}));
    expect(mockNavigate).toHaveBeenCalledWith('/gateways/gw-1?tab=secrets');
    expect(onClose).toHaveBeenCalled();

    await user.click(screen.getByRole('button', {name: 'Manage variables'}));
    expect(mockNavigate).toHaveBeenCalledWith('/gateways/gw-1?tab=variables');
  });

  it('checks again when the apply is refused for missing values', async () => {
    const user = userEvent.setup();
    let dryRuns = 0;
    mockHttpRequest.mockImplementation((call: HttpCall) => {
      if (call.method === 'POST' && !(call.data as {dryRun: boolean}).dryRun) {
        return Promise.reject(apiError(409, 'GTW-1017'));
      }
      return createHttpRouter({
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
        'POST /gateways/gw-1/apply': () => {
          dryRuns += 1;
          return dryRuns === 1 ? applied(true) : {...applied(true), missing: {secrets: ['CLIENT_SECRET']}};
        },
      })(call);
    });
    renderWithProviders(<ApplyVersionDialog open gatewayId="gw-1" gatewayName="production" onClose={vi.fn()} />);

    await waitFor(() => expect(screen.getByRole('button', {name: 'Apply'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Apply'}));

    expect(await screen.findByText(/missing variables or secrets the version needs/)).toBeInTheDocument();
    expect(await screen.findByText('1 secret is not configured')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
  });
});

describe('RevertDialog', () => {
  it('reverts and shows the result', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways/gw-1/revert': applied(false)}));
    renderWithProviders(
      <RevertDialog open gatewayId="gw-1" gatewayName="production" previousVersion={1} onClose={onClose} />,
    );

    expect(screen.getByText('Revert to version 1')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', {name: 'Revert'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Revert'}));

    expect(await screen.findByText('The gateway now holds this configuration.')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {dryRun: false}}));

    await user.click(screen.getByRole('button', {name: 'Done'}));
    expect(onClose).toHaveBeenCalled();
  });

  it('can revert as a dry run', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways/gw-1/revert': applied(true)}));
    renderWithProviders(
      <RevertDialog open gatewayId="gw-1" gatewayName="production" previousVersion={1} onClose={vi.fn()} />,
    );

    await user.click(screen.getByRole('checkbox'));
    await waitFor(() => expect(screen.getByRole('button', {name: 'Revert'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Revert'}));

    expect(await screen.findByText(/Dry run finished/)).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {dryRun: true}}));
  });

  it('explains when there is nothing to revert to', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockHttpRequest.mockRejectedValue(apiError(400, 'GTW-1016'));
    renderWithProviders(
      <RevertDialog open gatewayId="gw-1" gatewayName="production" previousVersion={1} onClose={onClose} />,
    );

    await waitFor(() => expect(screen.getByRole('button', {name: 'Revert'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Revert'}));

    expect(await screen.findByText('This gateway holds no earlier version to revert to.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    expect(onClose).toHaveBeenCalled();
  });
});

describe('RevertDialog missing values', () => {
  it('holds the revert until the gateway has what that version needs', async () => {
    const user = userEvent.setup();
    const gateway = missingUntilSet(applied(false), {secrets: ['CLIENT_SECRET']});
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'POST /gateways/gw-1/revert': gateway.respond,
        'POST /gateways/gw-1/secrets': gateway.setSecret,
      }),
    );
    renderWithProviders(
      <RevertDialog open gatewayId="gw-1" gatewayName="production" previousVersion={1} onClose={vi.fn()} />,
    );

    expect(await screen.findByText('1 secret is not configured')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {dryRun: true}}));
    expect(screen.getByRole('button', {name: 'Revert'})).toBeDisabled();

    await user.click(screen.getByRole('button', {name: 'Set values'}));
    await user.type(screen.getByLabelText('CLIENT_SECRET'), 's3cret');
    await user.click(screen.getByRole('button', {name: 'Save values'}));

    await waitFor(() => expect(screen.queryByText('1 secret is not configured')).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole('button', {name: 'Revert'})).toBeEnabled());
  });
});

describe('DeployedConfigurationCard', () => {
  it('shows the applied and previous versions and offers a revert', async () => {
    const user = userEvent.setup();
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'GET /gateways/gw-1/applied-version': {gatewayId: 'gw-1', appliedVersion: 2, previousVersion: 1},
        'GET /configuration-versions': versions,
        'GET /gateways/gw-1/diff?version=latest': diffTo(2),
      }),
    );
    renderWithProviders(<DeployedConfigurationCard gatewayId="gw-1" gatewayName="production" />);

    expect(await screen.findByText('Version 2')).toBeInTheDocument();
    expect(screen.getByText('Version 1')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Revert to version 1'}));
    expect(await screen.findByRole('dialog')).toHaveTextContent('Return production to version 1');
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', {name: 'Apply a version'}));
    expect(await screen.findByText('Apply a configuration version')).toBeInTheDocument();
  });

  it('shows a gateway nothing was applied to, with no revert', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1/applied-version': {gatewayId: 'gw-1'}}));
    renderWithProviders(<DeployedConfigurationCard gatewayId="gw-1" gatewayName="production" />);

    expect(await screen.findByText('Not applied')).toBeInTheDocument();
    expect(screen.getByText('None')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Revert to version/})).not.toBeInTheDocument();
  });

  it('shows a read failure in place', async () => {
    mockHttpRequest.mockRejectedValue(apiError(500));
    renderWithProviders(<DeployedConfigurationCard gatewayId="gw-1" gatewayName="production" />);

    expect(await screen.findByText('Failed to load the deployed configuration')).toBeInTheDocument();
  });
});
