// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditTokenSettings from '../EditTokenSettings';
import type {Application} from '@thunderid/configure-applications';

// Stable mock references, created via vi.hoisted so the hoisted vi.mock factories can use them.
// New identities on every render would re-fire the schema-fetch effect in a loop.
const {mockHttp, mockGetServerUrl, mockLogger} = vi.hoisted(() => ({
  mockHttp: {request: vi.fn().mockResolvedValue({data: {id: 'schema-1', name: 'default', schema: {}}})},
  mockGetServerUrl: vi.fn().mockReturnValue('https://api.example.com'),
  mockLogger: {error: vi.fn(), info: vi.fn(), debug: vi.fn()},
}));

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({data: {types: [{id: 'schema-1', name: 'default'}]}, isLoading: false}),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: mockHttp}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: mockGetServerUrl})};
});

vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();
  return {...actual, useLogger: () => mockLogger};
});

const nativeApplication = (assertion?: Application['assertion']): Application =>
  ({id: 'app-1', name: 'App Native App', allowedUserTypes: ['default'], assertion}) as Application;

/**
 * The validity effect runs again whenever one of its dependencies changes identity, which happens in
 * the console under StrictMode's double invocation of effects. By then the first-render guard is
 * already spent, so the effect must decide for itself whether anything actually changed. Re-rendering
 * with a fresh onFieldChange reproduces that second run deterministically.
 *
 * TokenValidationSection is left real so its react-hook-form Controllers mount and report values.
 */
describe('EditTokenSettings assertion change tracking', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not report an assertion change when the effect re-runs and the validity is unchanged', async () => {
    const application = nativeApplication({validityPeriod: 7200, userAttributes: []});
    const {rerender} = render(<EditTokenSettings application={application} onFieldChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('7200')).toBeInTheDocument();
    });

    const onFieldChange = vi.fn();
    rerender(<EditTokenSettings application={application} onFieldChange={onFieldChange} />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('7200')).toBeInTheDocument();
    });
    expect(onFieldChange).not.toHaveBeenCalled();
  });

  it('does not report an assertion change when no validity period is stored', async () => {
    const application = nativeApplication(undefined);
    const {rerender} = render(<EditTokenSettings application={application} onFieldChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('3600')).toBeInTheDocument();
    });

    const onFieldChange = vi.fn();
    rerender(<EditTokenSettings application={application} onFieldChange={onFieldChange} />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('3600')).toBeInTheDocument();
    });
    expect(onFieldChange).not.toHaveBeenCalled();
  });
});
