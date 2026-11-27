// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Session} from '../../models/session';
import SessionsPanel from '../SessionsPanel';

const {getSessionsMock} = vi.hoisted(() => ({
  getSessionsMock: vi.fn(),
}));
vi.mock('../../api/useGetSessions', () => ({default: getSessionsMock}));

const session: Session = {
  id: 'sess-1',
  userId: 'user-1',
  userDisplayName: 'Alice',
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
  });

  it('lists a user’s sessions by device', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(getSessionsMock).toHaveBeenCalledWith({userId: 'user-1'}, 10, 0);
    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByText(/^Last active /)).toBeInTheDocument();
    expect(screen.queryByText(/Alice/)).not.toBeInTheDocument();
  });

  it('names each session’s user on an application panel', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{appId: 'app-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByText(/^Alice, last active /)).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Alice'})).toHaveAttribute('href', '/users/user-1');
  });

  it('shows an empty state', () => {
    withSessions([]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByText('No active sessions')).toBeInTheDocument();
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

  it('says the admin may not view the sessions when listing is refused, with no retry', () => {
    getSessionsMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: Object.assign(new Error('forbidden'), {response: {status: 403}}),
      refetch: vi.fn(),
    });

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByText("You don't have permission to view these sessions.")).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Refresh'})).not.toBeInTheDocument();
  });

  it('expands a session to its details', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('macOS')).toBeInTheDocument();
    expect(screen.getByText('Chrome')).toBeInTheDocument();
    expect(screen.getByText('203.0.113.24')).toBeInTheDocument();
    expect(screen.getByText('Signed in from 198.51.100.7')).toBeInTheDocument();
    expect(screen.getAllByRole('img', {name: 'Console'}).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('img', {name: 'app-2'}).length).toBeGreaterThan(0);
    expect(screen.queryByText('sess-1')).not.toBeInTheDocument();
    expect(screen.queryByText(session.userAgent!)).not.toBeInTheDocument();
  });

  it('shows when the session was last active, signed in and ends, without counting down', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    const format = (value: string): string =>
      new Date(value).toLocaleString('en-US', {dateStyle: 'medium', timeStyle: 'short'});
    expect(screen.getByText('Session ends')).toBeInTheDocument();
    expect(screen.getByText(format(session.absoluteExpiresAt!))).toBeInTheDocument();
    expect(screen.getByText(format(session.lastActiveAt))).toBeInTheDocument();
    expect(screen.getByText(format(session.createdAt))).toBeInTheDocument();
    expect(screen.queryByText(format(session.idleExpiresAt!))).not.toBeInTheDocument();
    expect(screen.queryByText(/^(\d+[dhm] ?)+ ago$|^in \d|^Ended$/)).not.toBeInTheDocument();
  });

  it('shows a time in UTC on hover', async () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));
    fireEvent.mouseOver(
      screen.getByText(
        new Date(session.absoluteExpiresAt!).toLocaleString('en-US', {dateStyle: 'medium', timeStyle: 'short'}),
      ),
    );

    expect(await screen.findByText('Sep 30, 2026, 4:00:00 PM UTC')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: /^Chrome on macOS\s*Last active /})).toBeInTheDocument();
  });

  it('shows the operating system and browser together under one label', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByText('OS and browser')).toBeInTheDocument();
    expect(screen.queryByText('Operating system')).not.toBeInTheDocument();
    expect(screen.queryByText('Browser')).not.toBeInTheDocument();
  });

  it('names a mobile SDK session by its device alone', () => {
    withSessions([{...session, userAgent: 'ThunderID-Android/1.4.0 (Android 11; Redmi Note 8 Pro)'}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    expect(screen.queryByText(/ on Redmi Note 8 Pro/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: /^Redmi Note 8 Pro/}));

    expect(screen.getByText('Device')).toBeInTheDocument();
    expect(screen.getAllByText('Redmi Note 8 Pro')).toHaveLength(2);
    expect(screen.queryByText('OS and browser')).not.toBeInTheDocument();
    expect(screen.getByText('Android')).toBeInTheDocument();
  });

  it('leaves out the sign-in IP and credentials time when they repeat the others', () => {
    withSessions([{...session, creationIp: session.lastActiveIp, authenticatedAt: session.createdAt}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.queryByText(/^Signed in from /)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Last entered credentials /)).not.toBeInTheDocument();
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

  it('waits for a page’s own response before stepping back', () => {
    getSessionsMock.mockImplementation((_filter: unknown, _limit: number, offset: number) => ({
      data:
        offset === 0
          ? {totalResults: 25, startIndex: 1, count: 1, sessions: [session]}
          : {totalResults: 25, startIndex: 11, count: 0, sessions: []},
      isLoading: false,
      isPlaceholderData: offset !== 0,
      error: null,
      refetch: vi.fn(),
    }));

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /next page/i}));

    expect(getSessionsMock).toHaveBeenLastCalledWith({userId: 'user-1'}, 10, 10);
  });

  it('changes how many sessions a page shows, from the first page', () => {
    getSessionsMock.mockReturnValue({
      data: {totalResults: 25, startIndex: 1, count: 1, sessions: [session]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    expect(screen.getByText('Rows per page:')).toBeInTheDocument();
    expect(screen.getByText('1–10 of 25')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: '25'}));

    expect(getSessionsMock).toHaveBeenLastCalledWith({userId: 'user-1'}, 25, 0);
  });

  it('shows a spinner while the sessions load', () => {
    getSessionsMock.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByRole('progressbar', {name: 'Loading sessions'})).toBeInTheDocument();
  });

  it('names a session with no recognized device after its application', () => {
    withSessions([{...session, userAgent: 'ThunderID-iOS/1.4.0 (iOS 15.8)'}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);

    expect(screen.getByRole('button', {name: /^Console on iOS/})).toBeInTheDocument();
  });

  it('says what was not recorded about a session', () => {
    withSessions([
      {...session, userAgent: undefined, lastActiveIp: undefined, creationIp: undefined, participants: []},
    ]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /^Unknown device/}));

    expect(screen.getAllByText('Not recorded')).toHaveLength(2);
    expect(screen.getByText('None')).toBeInTheDocument();
  });

  it('names a user by their id when their name is unknown', () => {
    withSessions([{...session, userDisplayName: undefined}]);

    render(<SessionsPanel filter={{appId: 'app-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByText(/^user-1, last active /)).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'user-1'})).toHaveAttribute('href', '/users/user-1');
  });

  it('shows when the user last entered credentials, if after signing in', () => {
    withSessions([{...session, authenticatedAt: '2026-09-30T08:10:00Z'}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    const time = new Date('2026-09-30T08:10:00Z').toLocaleString('en-US', {dateStyle: 'medium', timeStyle: 'short'});
    expect(screen.getByText(`Last entered credentials ${time}`)).toBeInTheDocument();
  });

  it('says when a session has no recorded end', () => {
    withSessions([{...session, absoluteExpiresAt: undefined}]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByText('Not recorded')).toBeInTheDocument();
  });

  it('retries a failed load', () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    getSessionsMock.mockReturnValue({data: undefined, isLoading: false, error: new Error('boom'), refetch});

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /retry|refresh/i}));

    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('collapses an open session when it is clicked again', () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));

    expect(screen.getByRole('button', {name: /Chrome on macOS/})).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows a time in UTC when it takes keyboard focus', async () => {
    withSessions([session]);

    render(<SessionsPanel filter={{userId: 'user-1'}} />);
    fireEvent.click(screen.getByRole('button', {name: /Chrome on macOS/}));
    const ends = screen
      .getByText(
        new Date(session.absoluteExpiresAt!).toLocaleString('en-US', {dateStyle: 'medium', timeStyle: 'short'}),
      )
      .closest('time')!;
    ends.focus();

    expect(ends).toHaveAttribute('dateTime', session.absoluteExpiresAt);
    expect(await screen.findByText('Sep 30, 2026, 4:00:00 PM UTC')).toBeInTheDocument();
    expect(ends).toHaveAccessibleDescription('Sep 30, 2026, 4:00:00 PM UTC');
  });
});
