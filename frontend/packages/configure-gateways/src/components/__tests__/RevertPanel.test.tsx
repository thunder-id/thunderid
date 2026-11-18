// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import {PREVIEW_STALE_TIME} from '../../api/useGatewayVersions';
import RevertPanel from '../RevertPanel';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

const gateway = {id: 'gw-dev', name: 'Dev', baseUrl: 'https://dev.test', isDefault: true};

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RevertPanel', () => {
  it('waits for a preview read again when that read fails, though the earlier one is still shown', async () => {
    let previews = 0;
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': () => {
        previews += 1;
        return previews === 1
          ? {
              toVersion: 'hash-1',
              changes: [
                {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
              ],
              plan: 'plan-1',
            }
          : Object.assign(new Error('down'), {response: {status: 503}});
      },
    });
    const panel = (key: string) => (
      <RevertPanel key={key} gateway={gateway} previousLabel="v1" onClose={vi.fn()} onReverted={vi.fn()} />
    );
    const {rerender} = render(panel('first'));

    const dialog = await screen.findByTestId('deploy-revert-panel');
    await waitFor(() => expect(within(dialog).getByRole('button', {name: 'Roll back'})).toBeEnabled());

    // The dialog is opened again once its preview is stale, and reading it again fails.
    const now = Date.now();
    vi.spyOn(Date, 'now').mockReturnValue(now + PREVIEW_STALE_TIME + 1);
    rerender(panel('second'));

    const reopened = await screen.findByTestId('deploy-revert-panel');
    expect(await within(reopened).findByRole('button', {name: 'Refresh'})).toBeInTheDocument();
    expect(previews).toBe(2);
    expect(within(reopened).getByRole('button', {name: 'Roll back'})).toBeDisabled();
    expect(within(reopened).getByRole('button', {name: 'Check with gateway'})).toBeDisabled();
    // The failed read is not shown as a refused rollback.
    expect(within(reopened).queryByTestId('revert-failed')).not.toBeInTheDocument();
  });

  it('reads the preview again for a rollback refused for its plan, without calling that a failure', async () => {
    const changes = [
      {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
    ];
    let previews = 0;
    const sent: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': () => {
        previews += 1;
        // The read after the refusal fails once; the retry answers.
        if (previews === 2) return Object.assign(new Error('down'), {response: {status: 503}});
        return {toVersion: 'hash-1', changes, plan: `plan-${previews}`};
      },
      'POST /gateways/gw-dev/revert': (call: Call) => {
        sent.push(call.data);
        return sent.length === 1
          ? Object.assign(new Error('changed'), {response: {status: 409, data: {code: 'GTW-1019'}}})
          : {gatewayId: 'gw-dev', dryRun: false};
      },
    });
    const onReverted = vi.fn();
    render(<RevertPanel gateway={gateway} previousLabel="v1" onClose={vi.fn()} onReverted={onReverted} />);

    const dialog = await screen.findByTestId('deploy-revert-panel');
    await waitFor(() => expect(within(dialog).getByRole('button', {name: 'Roll back'})).toBeEnabled());
    await userEvent.click(within(dialog).getByRole('button', {name: 'Roll back'}));

    expect(await within(dialog).findByTestId('revert-preview-changed')).toBeInTheDocument();
    await userEvent.click(await within(dialog).findByRole('button', {name: 'Refresh'}));
    await waitFor(() => expect(within(dialog).getByRole('button', {name: 'Roll back'})).toBeEnabled());
    expect(within(dialog).queryByRole('button', {name: 'Refresh'})).not.toBeInTheDocument();
    expect(within(dialog).queryByTestId('revert-failed')).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole('button', {name: 'Roll back'}));

    await waitFor(() => expect(onReverted).toHaveBeenCalled());
    expect(sent.map((data) => (data as {plan?: string}).plan)).toEqual(['plan-1', 'plan-3']);
  });

  it('checks the rollback the preview shows, bound to its plan, and reads the preview again when it moved on', async () => {
    const changes = [
      {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'updated'},
    ];
    let previews = 0;
    const checks: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': () => {
        previews += 1;
        return {toVersion: 'hash-1', changes, plan: `plan-${previews}`};
      },
      'POST /gateways/gw-dev/revert': (call: Call) => {
        checks.push(call.data);
        return checks.length === 1
          ? {gatewayId: 'gw-dev', dryRun: true, diff: {toVersion: 'hash-1', changes}, import: {results: []}}
          : Object.assign(new Error('changed'), {response: {status: 409, data: {code: 'GTW-1019'}}});
      },
    });
    render(<RevertPanel gateway={gateway} previousLabel="v1" onClose={vi.fn()} onReverted={vi.fn()} />);

    const dialog = await screen.findByTestId('deploy-revert-panel');
    const check = await within(dialog).findByRole('button', {name: 'Check with gateway'});
    await waitFor(() => expect(check).toBeEnabled());
    await userEvent.click(check);
    expect(await within(dialog).findByTestId('revert-gateway-checked')).toBeInTheDocument();

    await userEvent.click(within(dialog).getByRole('button', {name: 'Check with gateway'}));

    await waitFor(() => expect(previews).toBe(2));
    expect(within(dialog).queryByTestId('revert-gateway-checked')).not.toBeInTheDocument();
    expect(within(dialog).queryByTestId('revert-gateway-check-failed')).not.toBeInTheDocument();
    expect(await within(dialog).findByTestId('revert-preview-changed')).toBeInTheDocument();
    expect(checks).toEqual([
      expect.objectContaining({dryRun: true, plan: 'plan-1'}),
      expect.objectContaining({dryRun: true, plan: 'plan-1'}),
    ]);
  });

  it('names a removal still needed and keeps the rollback waiting while the preview has one', async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': {
        toVersion: 'hash-1',
        changes: [
          {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Login', change: 'deleted'},
          {key: 'application/app-1', resourceType: 'application', id: 'app-1', name: 'Orders', change: 'unchanged'},
        ],
        removalsInUse: [{key: 'flow/f-1', neededBy: 'application/app-1'}],
      },
    });
    render(<RevertPanel gateway={gateway} previousLabel="v1" onClose={vi.fn()} onReverted={vi.fn()} />);

    const dialog = await screen.findByTestId('deploy-revert-panel');

    expect(await within(dialog).findByTestId('revert-removals-in-use')).toHaveTextContent(
      'Dev keeps a resource that still needs what rolling back removes: Login, which Orders needs.',
    );
    expect(within(dialog).getByRole('button', {name: 'Roll back'})).toBeDisabled();
  });
});
