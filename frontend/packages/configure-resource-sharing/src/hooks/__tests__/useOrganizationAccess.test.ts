// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook, act} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {SharingPolicy} from '../../models/sharing-policy';
import useOrganizationAccess from '../useOrganizationAccess';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
    useToast: () => ({showToast: vi.fn()}),
  };
});

const collection = 'https://localhost:8090/applications/app-1/sharing-policies';

function serve(policies: SharingPolicy[]): void {
  mockHttpRequest.mockImplementation(({method}: {method: string}) =>
    Promise.resolve(
      method === 'GET'
        ? {data: {totalResults: policies.length, startIndex: 1, count: policies.length, policies}}
        : {data: {}},
    ),
  );
}

const owned: SharingPolicy = {
  id: 'p1',
  stage: 'share',
  version: 2,
  childUnits: {mode: 'all', excludedOuIds: ['trials']},
};
const reshare: SharingPolicy = {
  id: 'p2',
  stage: 'reshare',
  initiatingOuId: 'retail',
  version: 1,
  childUnits: {mode: 'selected', units: [{ouId: 'retail-eu', reach: 'unit'}]},
};

describe('useOrganizationAccess', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('reads the owner-issued policy and ignores reshares', async () => {
    serve([reshare, owned]);

    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.savedPolicy?.id).toBe('p1');
    expect(result.current.draft.childUnitsAccess).toBe('all');
    expect(result.current.draft.childUnitExclusions).toEqual(['trials']);
    expect(result.current.hasChanges).toBe(false);
  });

  it('does not load the policies on reset while disabled', () => {
    serve([owned]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1', enabled: false}));

    act(() => {
      result.current.reset();
    });

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('carries its wording namespace to the screen', () => {
    serve([owned]);
    const {result} = renderHook(() =>
      useOrganizationAccess({resourcePath: 'applications/app-1', wordingNamespace: 'applications'}),
    );

    expect(result.current.wordingNamespace).toBe('applications');
  });

  it('marks a declared policy', async () => {
    serve([{...owned, isReadOnly: true}]);

    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));

    await waitFor(() => expect(result.current.isDeclared).toBe(true));
  });

  it('saves an exclusion change as an in-place edit', async () => {
    serve([owned]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => {
      result.current.updateDraft((draft) => ({...draft, childUnitExclusions: []}));
    });
    expect(result.current.hasChanges).toBe(true);

    await act(async () => {
      await result.current.save();
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${collection}/p1`,
        method: 'PUT',
        data: {childUnits: {mode: 'all'}, version: 2},
      }),
    );
    await waitFor(() => expect(result.current.hasChanges).toBe(false));
  });

  it('saves against the version the edit started from, even after a reload', async () => {
    serve([owned]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => {
      result.current.updateDraft((draft) => ({...draft, childUnitExclusions: []}));
    });
    // Someone else saves meanwhile, and a reload (window focus, a failed save) brings version 3.
    serve([{...owned, version: 3}]);
    act(() => {
      result.current.reload();
    });
    await waitFor(() => expect(result.current.savedPolicy?.version).toBe(3));

    await act(async () => {
      await result.current.save();
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: `${collection}/p1`, method: 'PUT', data: {childUnits: {mode: 'all'}, version: 2}}),
    );
  });

  it('treats an edit that lands back on the saved access as no change', async () => {
    serve([owned]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => {
      result.current.updateDraft((draft) => ({...draft, childUnitExclusions: ['trials']}));
    });

    expect(result.current.hasChanges).toBe(false);
  });

  it('applies a previewed switch and discards it on reset', async () => {
    serve([owned]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const preview = result.current.previewSwitch('childUnitsAccess', 'none');
    expect(preview.cleared).toEqual([{kind: 'childUnitExclusions', ouIds: ['trials']}]);

    act(() => result.current.applySwitch(preview));
    expect(result.current.draft.childUnitsAccess).toBe('none');
    expect(result.current.hasChanges).toBe(true);

    act(() => result.current.reset());
    expect(result.current.draft.childUnitsAccess).toBe('all');
    expect(result.current.hasChanges).toBe(false);
  });

  it('keeps the draft and exposes the error when saving fails', async () => {
    serve([]);
    const {result} = renderHook(() => useOrganizationAccess({resourcePath: 'applications/app-1'}));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.applySwitch(result.current.previewSwitch('scope', 'chosen')));
    act(() => result.current.applySwitch(result.current.previewSwitch('childUnitsAccess', 'all')));
    mockHttpRequest.mockImplementation(({method}: {method: string}) =>
      method === 'POST'
        ? Promise.reject(new Error('refused'))
        : Promise.resolve({data: {totalResults: 0, startIndex: 1, count: 0, policies: []}}),
    );

    await act(async () => {
      await expect(result.current.save()).rejects.toThrow('refused');
    });

    await waitFor(() => expect(result.current.saveError?.message).toBe('refused'));
    expect(result.current.draft.childUnitsAccess).toBe('all');
    expect(result.current.hasChanges).toBe(true);
  });
});
