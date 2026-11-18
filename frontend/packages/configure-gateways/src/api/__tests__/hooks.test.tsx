// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {act} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import type {PipelineStage} from '../../models/pipeline';
import useAdministrators from '../useAdministrators';
import {useApplyCheck, useApplyVersion, usePreview, useRevertPreview} from '../useGatewayVersions';
import {useMaster, useMasterHistory, useRefused, useResourceHistory} from '../useMaster';
import {useAutoApply, usePipeline, useProjects, useSetAutoApply, useSetProjectAutoSync} from '../usePipeline';
import {useProjectStatuses} from '../useProjectStatus';

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

const master = {
  gatewayId: 'gw-dev',
  number: 2,
  summary: {deployed: 1, failed: 1, removed: 0},
  entries: [
    {key: 'application/a1', resourceType: 'application', id: 'a1', state: 'deployed'},
    {key: 'flow/f1', resourceType: 'flow', id: 'f1', state: 'failed', failure: {message: 'Missing executor'}},
  ],
};

beforeEach(() => {
  scope.projectId = undefined;
  http = fakeHttp({
    'GET /gateways/gw-dev/master': master,
    'GET /gateways/gw-dev/master/history': [{...master, entries: undefined}],
    'GET /gateways/gw-dev/master/resources/flow/f1': {gatewayId: 'gw-dev', key: 'flow/f1', history: []},
    'GET /gateways/pipeline': {stages: [{name: 'Development', gateway: {id: 'gw-dev', name: 'dev'}}]},
    'GET /projects': {projects: [{id: 'p-shop', displayName: 'Shop', description: 'dropped'}]},
    'GET /server-config/gateway': {readOnly: {autoApply: true}, writable: null, merged: {autoApply: true}},
    'PUT /server-config/gateway': {},
    'PUT /projects/p-shop/auto-sync': {},
    'GET /users': {totalResults: 13, users: [{id: 'u-1', display: 'Jerad'}]},
  });
});

describe('master hooks', () => {
  it("reads a target's master, its history and one resource's history, and lists what it refused", async () => {
    const {result} = renderHook(() => ({
      master: useMaster('gw-dev'),
      history: useMasterHistory('gw-dev'),
      resource: useResourceHistory('gw-dev', 'flow/f1'),
      refused: useRefused('gw-dev'),
    }));

    await waitFor(() => expect(result.current.resource.isSuccess).toBe(true));
    await waitFor(() => expect(result.current.history.isSuccess).toBe(true));
    expect(result.current.master.data?.number).toBe(2);
    expect(result.current.refused.map((entry) => entry.key)).toEqual(['flow/f1']);
  });

  it('asks nothing without a target or a resource', () => {
    renderHook(() => ({master: useMasterHistory('', true), resource: useResourceHistory('gw-dev', null)}));

    expect(http.calls).toHaveLength(0);
  });
});

describe('pipeline hooks', () => {
  it('reads the pipeline, the projects and whether auto-sync is on and locked', async () => {
    const {result} = renderHook(() => ({pipeline: usePipeline(), projects: useProjects(), autoApply: useAutoApply()}));

    await waitFor(() => expect(result.current.autoApply.isSuccess).toBe(true));
    await waitFor(() => expect(result.current.projects.isSuccess).toBe(true));
    expect(result.current.projects.data).toEqual([{id: 'p-shop', displayName: 'Shop'}]);
    expect(result.current.autoApply.data).toEqual({enabled: true, locked: true});
    await waitFor(() => expect(result.current.pipeline.data?.stages).toHaveLength(1));
  });

  it("writes the organization's auto-sync over what is writable, and a project's own choice alone", async () => {
    const {result} = renderHook(() => ({organization: useSetAutoApply(), project: useSetProjectAutoSync()}));

    await act(() => result.current.organization.mutateAsync(false));
    await act(() => result.current.project.mutateAsync({project: 'p-shop', enabled: null}));

    expect(http.calls).toContainEqual(
      expect.objectContaining({method: 'PUT', url: `${SERVER}/server-config/gateway`, data: {autoApply: false}}),
    );
    expect(http.calls).toContainEqual(
      expect.objectContaining({method: 'PUT', url: `${SERVER}/projects/p-shop/auto-sync`, data: {autoSync: null}}),
    );
  });
});

describe('useProjectStatuses', () => {
  const stages: PipelineStage[] = [
    {name: 'Development', gateway: {id: 'gw-dev', name: 'dev', baseUrl: ''}},
    {name: 'Production', gateway: {id: 'gw-prod', name: 'prod', baseUrl: ''}},
  ];

  it('reads each gateway against the one before it, says which failed, and reads one again', async () => {
    scope.projectId = 'p-shop';
    let fail = true;
    http = fakeHttp({
      'GET /gateways/gw-dev/project-status': {
        project: {versions: [], pending: 0, required: 0},
        shared: {pending: 0, byType: []},
        needed: {differing: 0},
      },
      'GET /gateways/gw-prod/project-status': () => {
        if (fail) throw new Error('down');
        return {
          project: {versions: [], pending: 1, required: 0},
          shared: {pending: 0, byType: []},
          needed: {differing: 0},
        };
      },
    });
    const {result} = renderHook(() => useProjectStatuses(stages));

    await waitFor(() => expect(result.current.failed['gw-prod']).toBe(true));
    expect(result.current.byGateway['gw-dev']?.project.pending).toBe(0);
    expect(http.calls.map((call) => call.url)).toContain(
      `${SERVER}/gateways/gw-prod/project-status?project=p-shop&from=gw-dev`,
    );

    fail = false;
    act(() => result.current.retry('gw-prod'));
    await waitFor(() => expect(result.current.byGateway['gw-prod']?.project.pending).toBe(1));
    expect(result.current.failed['gw-prod']).toBe(false);
  });
});

describe('useAdministrators', () => {
  it('reads the first administrators and how many there are', async () => {
    const {result} = renderHook(() => useAdministrators(3));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.totalResults).toBe(13);
    expect(http.calls[0].url).toBe(`${SERVER}/users?limit=3&offset=0&include=display`);
  });
});

describe('preview hooks', () => {
  const diff = {toVersion: 'v1', changes: [], missing: {variables: ['HOST']}, removals: {count: 0, threshold: 5}};

  it('reads a preview as a dry run answers, and has the gateway check a deploy only when asked', async () => {
    scope.projectId = 'p-shop';
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': diff,
      'POST /gateways/gw-dev/apply': {gatewayId: 'gw-dev', dryRun: true, diff: {toVersion: 'v1', changes: []}},
    });
    const {result} = renderHook(() => ({
      preview: usePreview('gw-dev', 'v1'),
      revert: useRevertPreview('gw-dev'),
      check: useApplyCheck(),
    }));

    await waitFor(() => expect(result.current.preview.data).toBeDefined());
    await waitFor(() => expect(result.current.revert.data).toBeDefined());
    expect(result.current.preview.data).toEqual({
      gatewayId: 'gw-dev',
      dryRun: true,
      diff: {toVersion: 'v1', changes: []},
      missing: {variables: ['HOST']},
      removals: {count: 0, threshold: 5},
    });
    expect(http.calls.map((call: Call) => `${call.method} ${call.url.replace(SERVER, '')}`)).toEqual([
      'GET /gateways/gw-dev/diff?version=v1&project=p-shop&preview=true',
      'GET /gateways/gw-dev/diff?project=p-shop&revert=true',
    ]);

    await act(async () => {
      await result.current.check.mutateAsync({
        gatewayId: 'gw-dev',
        data: {version: 'v1', selection: ['application/a'], project: 'p-shop'},
      });
    });
    expect(http.calls.filter((call: Call) => call.method === 'POST').map((call: Call) => call.data)).toEqual([
      {version: 'v1', selection: ['application/a'], project: 'p-shop', dryRun: true},
    ]);
  });

  it('reads again only the previews and statuses of the gateway a deploy wrote', async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': diff,
      'GET /gateways/gw-prod/diff': diff,
      'POST /gateways/gw-dev/apply': {gatewayId: 'gw-dev', dryRun: false, diff: {toVersion: 'v1', changes: []}},
    });
    const {result} = renderHook(() => ({
      dev: usePreview('gw-dev', 'v1'),
      prod: usePreview('gw-prod', 'v1'),
      apply: useApplyVersion(),
    }));
    await waitFor(() => expect(result.current.dev.data && result.current.prod.data).toBeTruthy());
    const reads = (gateway: string): number =>
      http.calls.filter((call: Call) => call.url.includes(`/gateways/${gateway}/diff`)).length;

    await act(async () => {
      await result.current.apply.mutateAsync({gatewayId: 'gw-dev', data: {version: 'v1'}});
    });

    await waitFor(() => expect(reads('gw-dev')).toBe(2));
    expect(reads('gw-prod')).toBe(1);
  });

  it('replaces a first read still running when a deploy writes its gateway', async () => {
    let reads = 0;
    let answerFirst: (value: unknown) => void = () => undefined;
    http = fakeHttp({
      'GET /gateways/gw-dev/diff': () => {
        reads += 1;
        if (reads === 1) return new Promise((resolve) => (answerFirst = resolve));
        return {toVersion: 'v1', changes: [], plan: 'after'};
      },
      'POST /gateways/gw-dev/apply': {gatewayId: 'gw-dev', dryRun: false, diff: {toVersion: 'v1', changes: []}},
    });
    const {result} = renderHook(() => ({preview: usePreview('gw-dev', 'v1'), apply: useApplyVersion()}));
    await waitFor(() => expect(reads).toBe(1));

    await act(async () => {
      await result.current.apply.mutateAsync({gatewayId: 'gw-dev', data: {version: 'v1'}});
    });
    answerFirst({toVersion: 'v1', changes: [], plan: 'before'});

    await waitFor(() => expect(result.current.preview.data?.diff.plan).toBe('after'));
    expect(reads).toBe(2);
  });
});
