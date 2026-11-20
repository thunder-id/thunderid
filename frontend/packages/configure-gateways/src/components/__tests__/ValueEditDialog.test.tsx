// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import type {ValueKind, ValueReference} from '../../models/deploy';
import ValueEditDialog from '../ValueEditDialog';
import type {ValueRow} from '../ValuesTable';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

const reference = (name: string, kind: ValueKind, list = false): ValueReference => ({
  name,
  kind,
  resourceType: 'application',
  resourceName: 'Orders',
  field: list ? 'redirectUris' : 'clientId',
  ...(list ? {list: true} : {}),
});

/** The PUTs sent, each by the path it was sent to and what it carried. */
const puts = (): [string, unknown][] =>
  http.calls
    .filter((call: Call) => call.method === 'PUT')
    .map((call: Call) => [call.url.replace(SERVER, ''), call.data]);

let onClose = vi.fn<() => void>();

const open = (kind: ValueKind, row: ValueRow): void => {
  render(<ValueEditDialog gatewayId="gw-dev" kind={kind} row={row} onClose={onClose} />);
};

beforeEach(() => {
  onClose = vi.fn<() => void>();
  http = fakeHttp({
    'PUT /gateways/gw-dev/variables/CLIENT_ID': {name: 'CLIENT_ID', value: 'new'},
    'PUT /gateways/gw-dev/variables/REDIRECT_URIS': {name: 'REDIRECT_URIS'},
    'PUT /gateways/gw-dev/secrets/CLIENT_SECRET': {name: 'CLIENT_SECRET', exists: true},
  });
});

describe('ValueEditDialog', () => {
  it('sets a value the gateway does not hold, and closes once it is saved', async () => {
    open('variable', {name: 'CLIENT_ID', references: [reference('CLIENT_ID', 'variable')], current: []});

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Set CLIENT_ID')).toBeInTheDocument();
    const field = within(dialog).getByLabelText('CLIENT_ID');
    expect(field).toHaveValue('');
    await userEvent.type(field, 'orders');
    await userEvent.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(puts()).toEqual([['/gateways/gw-dev/variables/CLIENT_ID', {value: 'orders', description: undefined}]]);
  });

  it('edits a variable from what the gateway holds, keeping its description, and saves it unchanged as it was', async () => {
    const stored = '["alpha","beta"]';
    open('variable', {
      name: 'CLIENT_ID',
      stored: {name: 'CLIENT_ID', value: stored, description: 'the client'},
      references: [],
      current: [],
    });

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Edit CLIENT_ID')).toBeInTheDocument();
    // Not a list, so a value that only looks like one is edited as it is stored.
    expect(within(dialog).getByLabelText('CLIENT_ID')).toHaveValue(stored);
    await userEvent.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(puts()).toEqual([['/gateways/gw-dev/variables/CLIENT_ID', {value: stored, description: 'the client'}]]);
  });

  it('shows a list one item per line and saves it as a JSON list', async () => {
    open('variable', {
      name: 'REDIRECT_URIS',
      stored: {name: 'REDIRECT_URIS', value: '["https://a.test/cb"]'},
      references: [reference('REDIRECT_URIS', 'variable', true)],
      current: [],
    });

    const field = await screen.findByLabelText('REDIRECT_URIS');
    expect(field).toHaveValue('https://a.test/cb');
    expect(screen.getByText('One per line.')).toBeInTheDocument();
    await userEvent.type(field, '{enter}https://a.test/alt');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(puts()).toEqual([
        [
          '/gateways/gw-dev/variables/REDIRECT_URIS',
          {value: '["https://a.test/cb","https://a.test/alt"]', description: undefined},
        ],
      ]),
    );
  });

  it('replaces a secret without reading it back, and needs a value to do so', async () => {
    open('secret', {
      name: 'CLIENT_SECRET',
      stored: {name: 'CLIENT_SECRET', exists: true},
      references: [reference('CLIENT_SECRET', 'secret')],
      current: [],
    });

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Replace CLIENT_SECRET')).toBeInTheDocument();
    const field = within(dialog).getByLabelText('CLIENT_SECRET');
    expect(field).toHaveAttribute('type', 'password');
    expect(field).toHaveValue('');
    expect(within(dialog).getByRole('button', {name: 'Save'})).toBeDisabled();
    await userEvent.type(field, 's3cret');
    await userEvent.click(within(dialog).getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(puts()).toEqual([['/gateways/gw-dev/secrets/CLIENT_SECRET', {value: 's3cret', description: undefined}]]);
  });

  it('says why a save failed, from the error catalog, and stays open', async () => {
    http = fakeHttp({
      'PUT /gateways/gw-dev/variables/CLIENT_ID': Object.assign(new Error('Request failed'), {
        response: {status: 400, data: {code: 'NOT-A-KNOWN-CODE'}},
      }),
    });
    open('variable', {name: 'CLIENT_ID', references: [], current: []});

    await userEvent.type(await screen.findByLabelText('CLIENT_ID'), 'x');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    expect(await screen.findByText('The value could not be saved.')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('cannot be edited, cancelled or saved again while a save is pending', async () => {
    let answer: (value: unknown) => void = () => undefined;
    const calls: Call[] = [];
    http = {
      calls,
      request: vi.fn((call: Call) => {
        calls.push(call);
        return new Promise((resolve) => {
          answer = resolve;
        });
      }),
    };
    open('variable', {name: 'CLIENT_ID', references: [], current: []});

    const field = await screen.findByLabelText('CLIENT_ID');
    await userEvent.type(field, 'x');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() => expect(field).toBeDisabled());
    expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    await userEvent.keyboard('{Escape}');
    expect(onClose).not.toHaveBeenCalled();

    answer({data: {name: 'CLIENT_ID', value: 'x'}});
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it('closes on cancel without saving', async () => {
    open('variable', {name: 'CLIENT_ID', references: [], current: []});

    await userEvent.type(await screen.findByLabelText('CLIENT_ID'), 'x');
    await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(puts()).toEqual([]);
  });
});
