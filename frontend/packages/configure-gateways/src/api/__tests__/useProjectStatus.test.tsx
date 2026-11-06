// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import useProjectStatus, {useProjectChanges} from '../useProjectStatus';

let http = fakeHttp({});
const scope = vi.hoisted(() => ({projectId: undefined as string | undefined}));
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: {request: (call: Call) => http.request(call)}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER}),
    useProjectScope: () => ({projectId: scope.projectId}),
  };
});

const status = {
  project: {versions: [], pending: 0, required: 0},
  shared: {pending: 0, byType: []},
  needed: {differing: 0},
};

describe('useProjectStatus', () => {
  beforeEach(() => {
    scope.projectId = 'p-shop';
    http = fakeHttp({'GET /gateways/gw-prod/project-status': status});
  });

  it("reads the selected project's status on a target against the one before it", async () => {
    const {result} = renderHook(() => useProjectStatus('gw-prod', 'gw-dev'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(http.calls[0].url).toBe(`${SERVER}/gateways/gw-prod/project-status?project=p-shop&from=gw-dev`);
  });

  it('asks nothing outside a project', () => {
    scope.projectId = undefined;
    renderHook(() => useProjectStatus('gw-prod'));

    expect(http.calls).toHaveLength(0);
  });
});

describe('useProjectChanges', () => {
  it("counts the project's own changes the first environment does not run, by resource type", async () => {
    scope.projectId = 'p-shop';
    http = fakeHttp({
      'GET /gateways/pipeline': {stages: [{name: 'Development', gateway: {id: 'gw-dev', name: 'dev'}}]},
      'GET /configuration-versions': [{version: 'v2hash', number: 2}],
      'POST /gateways/gw-dev/apply': {
        gatewayId: 'gw-dev',
        dryRun: true,
        diff: {
          changes: [
            {key: 'application/a', resourceType: 'application', change: 'added', section: 'project'},
            {key: 'application/b', resourceType: 'application', change: 'updated', section: 'project'},
            {key: 'agent/c', resourceType: 'agent', change: 'unchanged', section: 'project'},
            {key: 'application/d', resourceType: 'application', change: 'added', section: 'other'},
            {key: 'flow/e', resourceType: 'flow', change: 'added', section: 'needed', required: true},
          ],
        },
      },
    });

    const {result} = renderHook(() => useProjectChanges());

    await waitFor(() => expect(result.current).toEqual({application: 2}));
  });
});
