// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Session} from '../../models/session';
import SessionsPanel from '../SessionsPanel';

// Resolves inline defaults with interpolation, like the real i18n.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    i18n: {language: 'en-US'},
    t: (key: string, arg?: unknown, values?: Record<string, string>): string => {
      let text = key;
      let vars = values;
      if (typeof arg === 'string') text = arg;
      else if (arg && typeof arg === 'object') {
        const obj = arg as Record<string, string>;
        if (obj['defaultValue']) text = obj['defaultValue'];
        vars = obj;
      }
      return Object.entries(vars ?? {}).reduce((acc, [k, v]) => acc.replace(`{{${k}}}`, v), text);
    },
  }),
}));

const {getSessionsMock, revokeMutate, revokeAllMutate} = vi.hoisted(() => ({
  getSessionsMock: vi.fn(),
  revokeMutate: vi.fn(),
  revokeAllMutate: vi.fn(),
}));
vi.mock('../../api/useGetSessions', () => ({default: getSessionsMock}));
vi.mock('../../api/useRevokeSession', () => ({
  default: () => ({mutate: revokeMutate, reset: vi.fn(), isPending: false, error: null}),
}));
vi.mock('../../api/useRevokeUserSessions', () => ({
  default: () => ({mutate: revokeAllMutate, reset: vi.fn(), isPending: false, error: null}),
}));

const session: Session = {
  id: 'sess-1',
  userId: 'user-1',
  userName: 'Alice',
  userAgent:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
  creationIp: '198.51.100.7',
  lastActiveIp: '203.0.113.24',
  authenticatedAt: '2026-09-30T08:00:00Z',
  createdAt: '2026-09-30T08:00:00Z',
  lastActiveAt: '2026-09-30T08:20:00Z',
  idleExpiresAt: '2026-09-30T08:50:00Z',
  absoluteExpiresAt: '2026-09-30T16:00:00Z',
  participants: [
    {appId: 'app-1', appName: 'Console', firstJoinedAt: '', lastActiveAt: ''},
    {appId: 'app-2', firstJoinedAt: '', lastActiveAt: ''},
  ],
};

function withSessions(sessions: Session[]): void {
  getSessionsMock.mockReturnValue({
    data: {totalResults: sessions.length, startIndex: 1, count: sessions.length, sessions},
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  });
}

describe('SessionsPanel', () => {
  beforeEach(() => {
    getSessionsMock.mockReset();
    revokeMutate.mockReset();
    revokeAllMutate.mockReset();
  });

  it('lists a user’s sessions by device', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(getSessionsMock).toHaveBeenCalledWith({userId: 'user-1'}, 10, 0);
    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByText(/^Last active /)).toBeInTheDocument();
    expect(screen.queryByText(/Alice/)).not.toBeInTheDocument();
  });

  it('names each session’s user on an application panel, with no revoke all', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{appId: 'app-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByText(/^Alice, last active /)).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Alice'})).toHaveAttribute('href', '/users/user-1');
    expect(screen.queryByRole('button', {name: 'Revoke All'})).not.toBeInTheDocument();
  });

  it('shows an empty state', () => {
    withSessions([]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByText('No active sessions')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Revoke All'})).not.toBeInTheDocument();
  });

  it('shows a load failure in place of the list', () => {
    getSessionsMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom'),
      refetch: vi.fn(),
    });

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByText('Failed to load sessions')).toBeInTheDocument();
  });

  it('revokes one session after confirming', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: 'Revoke'}));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Revoke'}));

    expect(revokeMutate).toHaveBeenCalledWith('sess-1', expect.any(Object));
  });

  it('revokes all of a user’s sessions after confirming', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: 'Revoke All'}));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Revoke'}));

    expect(revokeAllMutate).toHaveBeenCalledWith('user-1', expect.any(Object));
  });

  it('expands a session to its details', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('macOS')).toBeInTheDocument();
    expect(screen.getByText('Chrome')).toBeInTheDocument();
    expect(screen.getByText('203.0.113.24')).toBeInTheDocument();
    expect(screen.getByText('198.51.100.7')).toBeInTheDocument();
    expect(screen.getByText('Console, app-2')).toBeInTheDocument();
    expect(screen.queryByText('sess-1')).not.toBeInTheDocument();
    expect(screen.queryByText(session.userAgent!)).not.toBeInTheDocument();
  });

  it('names a mobile SDK session by its device alone', () => {
    withSessions([{...session, userAgent: 'ThunderID-Android/1.4.0 (Android 11; Redmi Note 8 Pro)'}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    expect(screen.queryByText(/ on Redmi Note 8 Pro/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: /^Redmi Note 8 Pro/}));

    expect(screen.getByText('Device')).toBeInTheDocument();
    expect(screen.getAllByText('Redmi Note 8 Pro')).toHaveLength(2);
    expect(screen.queryByText('Browser')).not.toBeInTheDocument();
    expect(screen.getByText('Android')).toBeInTheDocument();
  });

  it('leaves out the sign-in IP and credentials time when they repeat the others', () => {
    withSessions([{...session, creationIp: session.lastActiveIp, authenticatedAt: session.createdAt}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.queryByText('Signed in from IP')).not.toBeInTheDocument();
    expect(screen.queryByText('Last entered credentials')).not.toBeInTheDocument();
  });

  it('pages through more sessions than fit on a page', () => {
    getSessionsMock.mockReturnValue({
      data: {totalResults: 25, startIndex: 1, count: 1, sessions: [session]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /next page/i}));

    expect(getSessionsMock).toHaveBeenLastCalledWith({userId: 'user-1'}, 10, 10);
  });

  it('steps back a page when the one shown has no sessions left', () => {
    getSessionsMock.mockImplementation((_filter: unknown, _limit: number, offset: number) => ({
      data:
        offset === 0
          ? {totalResults: 25, startIndex: 1, count: 1, sessions: [session]}
          : {totalResults: 10, startIndex: 11, count: 0, sessions: []},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }));

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /next page/i}));

    expect(getSessionsMock).toHaveBeenLastCalledWith({userId: 'user-1'}, 10, 0);
    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toBeInTheDocument();
  });
});
