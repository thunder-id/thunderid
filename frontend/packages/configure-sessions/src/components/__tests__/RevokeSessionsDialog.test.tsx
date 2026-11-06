// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import RevokeSessionsDialog from '../RevokeSessionsDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, arg?: unknown): string => {
      if (typeof arg === 'string') return arg;
      return (arg as {defaultValue?: string} | undefined)?.defaultValue ?? key;
    },
  }),
}));

const {revoke, revokeAll} = vi.hoisted(() => ({
  revoke: {mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null as Error | null},
  revokeAll: {mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null as Error | null},
}));
vi.mock('../../api/useRevokeSession', () => ({default: () => revoke}));
vi.mock('../../api/useRevokeUserSessions', () => ({default: () => revokeAll}));

describe('RevokeSessionsDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    revoke.error = null;
    revoke.isPending = false;
  });

  it('closes only after the revocation succeeds', () => {
    const onClose = vi.fn();
    revoke.mutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => options.onSuccess());

    render(<RevokeSessionsDialog target={{sessionId: 'sess-1'}} onClose={onClose} />);
    expect(screen.getByText('Revoke session')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Revoke'}));

    expect(revoke.mutate).toHaveBeenCalledWith('sess-1', expect.any(Object));
    expect(onClose).toHaveBeenCalled();
  });

  it('keeps a failure in the dialog', () => {
    revoke.error = new Error('boom');

    render(<RevokeSessionsDialog target={{sessionId: 'sess-1'}} onClose={vi.fn()} />);

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to revoke the session. Please try again.');
  });

  it('cancels and clears a previous failure', () => {
    const onClose = vi.fn();

    render(<RevokeSessionsDialog target={{userId: 'user-1'}} onClose={onClose} />);
    expect(screen.getByText('Revoke all sessions')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(revoke.reset).toHaveBeenCalled();
    expect(revokeAll.reset).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
    expect(revokeAll.mutate).not.toHaveBeenCalled();
  });

  it('can’t be closed while revoking', () => {
    const onClose = vi.fn();
    revoke.isPending = true;

    render(<RevokeSessionsDialog target={{sessionId: 'sess-1'}} onClose={onClose} />);
    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole('button', {name: 'Revoking...'})).toBeDisabled();
  });
});
