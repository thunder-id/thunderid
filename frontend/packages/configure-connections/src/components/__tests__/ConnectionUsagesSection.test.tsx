// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ConnectionUsagesResponse} from '../../models/connection';
import ConnectionUsagesSection from '../ConnectionUsagesSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: unknown): string =>
      typeof fallback === 'string'
        ? fallback
        : ((fallback as {defaultValue?: string} | undefined)?.defaultValue ?? key),
  }),
}));

interface UsagesState {
  data: ConnectionUsagesResponse | undefined;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
}

const {getUsagesMock} = vi.hoisted(() => ({
  getUsagesMock: vi.fn<(type: string, id: string) => UsagesState>(),
}));
vi.mock('../../api/useGetConnectionUsages', () => ({
  default: (type: string, id: string) => getUsagesMock(type, id),
}));

const state = (overrides: Partial<UsagesState>): UsagesState => ({
  data: undefined,
  isLoading: false,
  error: null,
  refetch: vi.fn(),
  ...overrides,
});

describe('ConnectionUsagesSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('requests the usages of the given connection', () => {
    getUsagesMock.mockReturnValue(state({data: {totalResults: 0, count: 0, summary: {}, usages: []}}));
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    expect(getUsagesMock).toHaveBeenCalledWith('http', 'http1');
  });

  it('lists each referencing flow by name', () => {
    getUsagesMock.mockReturnValue(
      state({
        data: {
          totalResults: 1,
          count: 1,
          summary: {flow: 1},
          usages: [{resourceType: 'flow', id: 'f1', displayName: 'Default sign-in', behaviorOnDelete: 'restrict'}],
        },
      }),
    );
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    const list = screen.getByTestId('connection-usages-list');
    expect(list).toHaveTextContent('Default sign-in');
    expect(list).toHaveTextContent('Flow');
  });

  it('labels a usage of another resource type generically', () => {
    getUsagesMock.mockReturnValue(
      state({
        data: {
          totalResults: 1,
          count: 1,
          summary: {application: 1},
          usages: [{resourceType: 'application', id: 'a1', displayName: 'Shop', behaviorOnDelete: 'restrict'}],
        },
      }),
    );
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    const list = screen.getByTestId('connection-usages-list');
    expect(list).toHaveTextContent('Shop');
    expect(list).toHaveTextContent('Resource');
    expect(list).not.toHaveTextContent('Flow');
  });

  it('describes the section without naming flows', () => {
    getUsagesMock.mockReturnValue(state({data: {totalResults: 0, count: 0, summary: {}, usages: []}}));
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    expect(
      screen.getByText('Resources that use this connection. A connection cannot be deleted while a resource uses it.'),
    ).toBeInTheDocument();
  });

  it('says no resources use the connection when there are none', () => {
    getUsagesMock.mockReturnValue(state({data: {totalResults: 0, count: 0, summary: {}, usages: []}}));
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    expect(screen.getByText('No resources use this connection.')).toBeInTheDocument();
  });

  it('does not claim the connection is unused when its usages are unknown', () => {
    getUsagesMock.mockReturnValue(state({data: {totalResults: null, count: 0, summary: null, usages: []}}));
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    expect(screen.getByText('The resources that use this connection could not be determined.')).toBeInTheDocument();
    expect(screen.queryByText('No resources use this connection.')).not.toBeInTheDocument();
  });

  it('shows an inline error when the usages cannot be loaded', () => {
    getUsagesMock.mockReturnValue(state({error: new Error('boom')}));
    render(<ConnectionUsagesSection connectionType="http" connectionId="http1" />);

    expect(screen.getByText('Failed to load the resources that use this connection.')).toBeInTheDocument();
    expect(screen.queryByText('No resources use this connection.')).not.toBeInTheDocument();
  });
});
