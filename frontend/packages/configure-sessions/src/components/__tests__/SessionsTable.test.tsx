/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Session, SessionListResponse} from '../../models/sessions';
import SessionsTable from '../SessionsTable';

interface UseGetSessionsReturn {
  data: SessionListResponse | undefined;
  isLoading: boolean;
  isError: boolean;
}

const mockUseGetSessions = vi.fn<() => UseGetSessionsReturn>();

vi.mock('@/api/useGetSessions', () => ({
  default: (...args: unknown[]) => mockUseGetSessions(...args),
}));

function formatExpected(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

describe('SessionsTable', () => {
  // sessionOne carries server-resolved names; sessionTwo omits them so the id-fallback path is covered.
  const sessionOne: Session = {
    id: 'session-1',
    userId: 'user-1',
    userName: 'Alice Doe',
    loginFlowId: 'flow-1',
    authenticatedAt: '2026-01-01T10:00:00Z',
    createdAt: '2026-01-01T10:00:00Z',
    lastActiveAt: '2026-01-02T12:30:00Z',
    idleExpiresAt: '2026-01-10T00:00:00Z',
    participants: [
      {appId: 'app-1', appName: 'My App', firstJoinedAt: '2026-01-01T10:00:00Z', lastActiveAt: '2026-01-02T12:30:00Z'},
    ],
  };

  const sessionTwo: Session = {
    id: 'session-2',
    userId: 'user-2',
    loginFlowId: 'flow-2',
    authenticatedAt: '2026-02-01T08:00:00Z',
    createdAt: '2026-02-01T08:00:00Z',
    lastActiveAt: '2026-02-02T09:15:00Z',
    participants: [{appId: 'app-2', firstJoinedAt: '2026-02-01T08:00:00Z', lastActiveAt: '2026-02-02T09:15:00Z'}],
  };

  const sessionsResponse: SessionListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    sessions: [sessionOne, sessionTwo],
    links: [],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a progressbar while loading', () => {
    mockUseGetSessions.mockReturnValue({data: undefined, isLoading: true, isError: false});

    render(<SessionsTable filter={{userId: 'user-1'}} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders an error state when the request fails', () => {
    mockUseGetSessions.mockReturnValue({data: undefined, isLoading: false, isError: true});

    render(<SessionsTable filter={{userId: 'user-1'}} />);

    expect(screen.getByText(/Unable to load sessions/i)).toBeInTheDocument();
  });

  it('renders session rows with locale-formatted dates and app-name chips from the response', async () => {
    mockUseGetSessions.mockReturnValue({data: sessionsResponse, isLoading: false, isError: false});

    render(<SessionsTable filter={{userId: 'user-1'}} />);

    await waitFor(() => {
      expect(screen.getAllByText(formatExpected(sessionOne.authenticatedAt)).length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText(formatExpected(sessionOne.lastActiveAt)).length).toBeGreaterThan(0);
    expect(screen.getAllByText(formatExpected(sessionTwo.authenticatedAt)).length).toBeGreaterThan(0);

    // App name from the response's participant.appName.
    expect(screen.getByText('My App')).toBeInTheDocument();
    // Falls back to the raw appId when the participant has no resolved name.
    expect(screen.getByText('app-2')).toBeInTheDocument();
  });

  it('shows "Never" for sessions without an idle or absolute expiry', async () => {
    mockUseGetSessions.mockReturnValue({data: sessionsResponse, isLoading: false, isError: false});

    render(<SessionsTable filter={{userId: 'user-1'}} />);

    await waitFor(() => {
      expect(screen.getByText('Never')).toBeInTheDocument();
    });
  });

  it('renders the Expires column from the earliest deadline when both are set', async () => {
    const sessionWithBothDeadlines: Session = {
      id: 'session-3',
      userId: 'user-3',
      loginFlowId: 'flow-3',
      authenticatedAt: '2026-03-01T10:00:00Z',
      createdAt: '2026-03-01T10:00:00Z',
      lastActiveAt: '2026-03-02T09:15:00Z',
      idleExpiresAt: '2026-03-05T00:00:00Z',
      absoluteExpiresAt: '2026-03-20T00:00:00Z',
      participants: [{appId: 'app-3', firstJoinedAt: '2026-03-01T10:00:00Z', lastActiveAt: '2026-03-02T09:15:00Z'}],
    };

    mockUseGetSessions.mockReturnValue({
      data: {totalResults: 1, startIndex: 1, count: 1, sessions: [sessionWithBothDeadlines], links: []},
      isLoading: false,
      isError: false,
    });

    render(<SessionsTable filter={{userId: 'user-3'}} />);

    await waitFor(() => {
      expect(screen.getByText(formatExpected(sessionWithBothDeadlines.idleExpiresAt!))).toBeInTheDocument();
    });
    expect(screen.queryByText(formatExpected(sessionWithBothDeadlines.absoluteExpiresAt!))).not.toBeInTheDocument();
  });

  it('does not show the user column by default', async () => {
    mockUseGetSessions.mockReturnValue({data: sessionsResponse, isLoading: false, isError: false});

    render(<SessionsTable filter={{appId: 'app-1'}} />);

    await waitFor(() => {
      expect(screen.getByText('My App')).toBeInTheDocument();
    });
    expect(screen.queryByText('Alice Doe')).not.toBeInTheDocument();
    expect(screen.queryByText('user-1')).not.toBeInTheDocument();
  });

  it('shows the user column with names from the response when showUser is set, falling back to the raw id', async () => {
    mockUseGetSessions.mockReturnValue({data: sessionsResponse, isLoading: false, isError: false});

    render(<SessionsTable filter={{appId: 'app-1'}} showUser />);

    // Resolved display name from the response (sessionOne.userName).
    await waitFor(() => {
      expect(screen.getByText('Alice Doe')).toBeInTheDocument();
    });
    // Falls back to the raw user id when the session has no resolved name (sessionTwo).
    expect(screen.getByText('user-2')).toBeInTheDocument();
    // The resolved user's raw id is not shown.
    expect(screen.queryByText('user-1')).not.toBeInTheDocument();
  });

  it('hides the participants column when hideParticipants is set', async () => {
    mockUseGetSessions.mockReturnValue({data: sessionsResponse, isLoading: false, isError: false});

    render(<SessionsTable filter={{appId: 'app-1'}} hideParticipants />);

    await waitFor(() => {
      expect(screen.getByText(formatExpected(sessionOne.authenticatedAt))).toBeInTheDocument();
    });
    expect(screen.queryByText('My App')).not.toBeInTheDocument();
    expect(screen.queryByText('app-2')).not.toBeInTheDocument();
  });

  it('shows the empty overlay when there are no sessions', async () => {
    mockUseGetSessions.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, sessions: [], links: []},
      isLoading: false,
      isError: false,
    });

    render(<SessionsTable filter={{userId: 'user-1'}} />);

    await waitFor(() => {
      expect(screen.getByText('No rows')).toBeInTheDocument();
    });
  });
});
