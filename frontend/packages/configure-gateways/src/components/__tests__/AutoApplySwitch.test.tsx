// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import AutoApplySwitch from '../AutoApplySwitch';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});

const showToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER}),
    useToast: () => ({showToast}),
  };
});

beforeEach(() => {
  showToast.mockReset();
});

describe('AutoApplySwitch', () => {
  it("says auto-sync could not be changed without showing the server's own text", async () => {
    http = fakeHttp({
      'GET /server-config/gateway': {merged: {autoApply: false}, writable: {}},
      'PUT /server-config/gateway': Object.assign(new Error('Request failed'), {
        response: {status: 500, data: {message: 'pq: connection refused on 10.0.0.5'}},
      }),
    });
    render(<AutoApplySwitch target="Development" />);

    await userEvent.click(await screen.findByRole('checkbox'));

    await waitFor(() => expect(showToast).toHaveBeenCalledWith('Auto-sync could not be changed.', 'error'));
    expect(showToast).not.toHaveBeenCalledWith(expect.stringContaining('connection refused'), 'error');
  });

  it("names the hint about shared resources and reaches it from the keyboard in a project's switch", async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/project-status': {
        project: {versions: [], pending: 0, required: 0},
        shared: {pending: 0, byType: []},
        needed: {differing: 0},
        autoSync: {enabled: false, inherited: true},
      },
    });
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <AutoApplySwitch target="Development" targetId="gw-dev" />
      </ProjectScopeProvider>,
    );

    const info = await screen.findByRole('img', {name: 'About shared resources'});
    expect(info).toHaveAttribute('tabindex', '0');
    info.focus();
    expect(info).toHaveFocus();
  });

  it("sets a project's own auto-sync, and has it follow the organization's default again", async () => {
    const sent: unknown[] = [];
    http = fakeHttp({
      'GET /gateways/gw-dev/project-status': {
        project: {versions: [], pending: 0, required: 0},
        shared: {pending: 0, byType: []},
        needed: {differing: 0},
        autoSync: {enabled: true, inherited: false},
      },
      'PUT /projects/p-shop/auto-sync': (call: Call) => {
        sent.push(call.data);
        return {};
      },
    });
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <AutoApplySwitch target="Development" targetId="gw-dev" />
      </ProjectScopeProvider>,
    );

    const toggle = await screen.findByRole('checkbox', {name: 'Auto-sync from configuration'});
    await waitFor(() => expect(toggle).toBeChecked());
    await userEvent.click(toggle);
    await waitFor(() => expect(sent).toHaveLength(1));
    await userEvent.click(await screen.findByTestId('auto-apply-use-default'));
    await waitFor(() => expect(sent).toHaveLength(2));

    expect(sent[0]).toEqual({autoSync: false});
    expect(sent[1]).toEqual({autoSync: null});
  });

  it("says a project's auto-sync is set in the deployment configuration when a change is refused for it", async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/project-status': {
        project: {versions: [], pending: 0, required: 0},
        shared: {pending: 0, byType: []},
        needed: {differing: 0},
        autoSync: {enabled: false, inherited: true},
      },
      'PUT /projects/p-shop/auto-sync': Object.assign(new Error('Request failed'), {
        response: {status: 400, data: {code: 'SCF-1003'}},
      }),
    });
    render(
      <ProjectScopeProvider project={{id: 'p-shop'}}>
        <AutoApplySwitch target="Development" targetId="gw-dev" />
      </ProjectScopeProvider>,
    );

    const toggle = await screen.findByRole('checkbox', {name: 'Auto-sync from configuration'});
    await waitFor(() => expect(toggle).toBeEnabled());
    await userEvent.click(toggle);

    await waitFor(() => expect(toggle).toBeDisabled());
    expect(toggle).not.toBeChecked();
    expect(showToast).not.toHaveBeenCalled();
  });
});
