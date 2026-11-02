// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {QueryKey} from '@tanstack/react-query';
import {act, fireEvent, screen, waitFor, within} from '@testing-library/react';
import {render} from '@thunderid/test-utils';
import type {AccessStatus} from '@thunderid/types';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import AccountStatusAction from '../AccountStatusAction';
import {LifecycleActionIncompleteError, type LifecycleCategory, type LifecycleOperation} from '../lifecycleAction';

const state = vi.hoisted(() => ({
  available: {} as Partial<Record<string, string>>,
  availabilityError: null as Error | null,
  isPending: false,
  listQueryKeys: [] as unknown[],
  mismatches: [] as string[],
  // When set, the real useLifecycleAction runs against the mocked http client.
  realMutation: false,
}));

const {mockHttpRequest, mockMutate, mockReset, mockShowToast} = vi.hoisted(() => ({
  mockHttpRequest: vi.fn(),
  mockMutate: vi.fn(),
  mockReset: vi.fn(),
  mockShowToast: vi.fn(),
}));

vi.mock('../useAvailableLifecycleActions', () => ({
  default: () => ({
    available: state.available,
    error: state.availabilityError,
    isError: state.availabilityError !== null,
    isPending: state.isPending,
  }),
}));

vi.mock('../useLifecycleAction', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../useLifecycleAction')>();
  return {
    default: (listQueryKey: QueryKey, onSuccess?: () => void) => {
      state.listQueryKeys.push(listQueryKey);
      return state.realMutation
        ? actual.default(listQueryKey, onSuccess)
        : {error: null, isError: false, isPending: false, mutate: mockMutate, reset: mockReset};
    },
  };
});

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  useToast: () => ({showToast: mockShowToast}),
}));

// The package has no i18next instance, so `t` reads the built en-US catalog and records any
// fallback that differs from it.
vi.mock('react-i18next', async () => {
  const {default: enUS} = await import('@thunderid/i18n/locales/en-US');
  const catalog = enUS as unknown as Record<string, Record<string, string>>;
  const t = (
    key: string,
    fallbackOrOptions?: string | Record<string, unknown>,
    values?: Record<string, unknown>,
  ): string => {
    const split = key.indexOf(':');
    const [ns, name] = split > 0 ? [key.slice(0, split), key.slice(split + 1)] : ['common', key];
    const options = typeof fallbackOrOptions === 'object' ? fallbackOrOptions : values;
    const fallback =
      typeof fallbackOrOptions === 'string' ? fallbackOrOptions : (options?.['defaultValue'] as string | undefined);
    const entry = catalog[ns]?.[name];
    if (typeof fallbackOrOptions === 'string' && entry !== undefined && entry !== fallbackOrOptions) {
      state.mismatches.push(key);
    }
    return (entry ?? fallback ?? key).replace(/{{(\w+)}}/g, (match, variable: string) => {
      const value = options?.[variable];
      return typeof value === 'string' ? value : match;
    });
  };
  return {useTranslation: () => ({t})};
});

const ALL_FLOWS: Partial<Record<LifecycleOperation, string>> = {
  suspend: 'flow-suspend',
  unlock: 'flow-unlock',
  unsuspend: 'flow-unsuspend',
};

const COMPLETE = {data: {flowStatus: 'COMPLETE'}};
const REFUSED = {data: {error: {code: 'FET-1112'}, flowStatus: 'ERROR'}};
const LIST_KEY = ['users'];
const FET_1112 =
  'The account operation could not be completed. Check that the account still exists and that the operation applies to its current state.';

function withFlows(available: Partial<Record<LifecycleOperation, string>>, isPending = false): void {
  state.available = available;
  state.isPending = isPending;
}

function renderAction(status?: AccessStatus, category: LifecycleCategory = 'user', onChanged?: () => void) {
  return render(
    <AccountStatusAction
      category={category}
      subject="user-1"
      status={status === undefined ? undefined : {value: status}}
      listQueryKey={LIST_KEY}
      onChanged={onChanged}
    />,
  );
}

/**
 * Opens the Actions menu and returns the operations it offers, as label and hint text.
 */
function openMenu(): string[] {
  fireEvent.click(screen.getByRole('button', {name: 'Actions'}));

  return screen.getAllByRole('menuitem').map((item) => item.textContent ?? '');
}

/**
 * Chooses one operation from the open menu.
 */
function choose(label: string): void {
  const item = screen.getAllByRole('menuitem').find((candidate) => candidate.textContent?.startsWith(label));

  if (!item) {
    throw new Error(`no menu item for ${label}`);
  }
  fireEvent.click(item);
}

function confirm(label: string): void {
  fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: label}));
}

describe('AccountStatusAction', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.realMutation = false;
    state.mismatches = [];
    state.availabilityError = null;
    state.listQueryKeys = [];
    withFlows(ALL_FLOWS);
  });

  afterEach(() => {
    expect(state.mismatches).toEqual([]);
  });

  describe('menu', () => {
    it('offers only Suspend on an active account', () => {
      renderAction('ACTIVE');

      const offered = openMenu();

      expect(offered).toHaveLength(1);
      expect(offered[0]).toMatch(/^Suspend/);
    });

    it('treats an absent status as active', () => {
      renderAction(undefined);

      expect(openMenu().some((label) => label.startsWith('Suspend'))).toBe(true);
    });

    it('offers Unsuspend, and Unsuspend and unlock, on a suspended user', () => {
      renderAction('SUSPENDED');

      const offered = openMenu();

      expect(offered).toHaveLength(2);
      expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
      expect(offered[1]).toMatch(/^Unsuspend and unlock/);
    });

    it('offers only Unsuspend on a suspended agent', () => {
      renderAction('SUSPENDED', 'agent');

      const offered = openMenu();

      expect(offered).toHaveLength(1);
      expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
    });

    it('offers only Unsuspend on a suspended application', () => {
      withFlows({suspend: 'flow-suspend', unsuspend: 'flow-unsuspend'});
      renderAction('SUSPENDED', 'application');

      const offered = openMenu();

      expect(offered).toHaveLength(1);
      expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
    });

    it('says which release leaves a user locked and which signs in straight away', () => {
      renderAction('SUSPENDED');

      const offered = openMenu();

      expect(offered[0]).toContain('remains locked');
      expect(offered[1]).toContain('can sign in right away');
    });

    it('offers Unlock and Suspend on a locked account', () => {
      renderAction('LOCKED');

      expect(openMenu()).toEqual([
        expect.stringMatching(/^Unlock.*Remove all sign-in locks\./),
        expect.stringMatching(/^Suspend.*Prevent sign-in and revoke existing access\./),
      ]);
    });

    it('uses the authentication wording for an agent', () => {
      renderAction('LOCKED', 'agent');

      expect(openMenu()).toEqual([
        expect.stringMatching(/Remove all authentication locks\./),
        expect.stringMatching(/Prevent authentication and revoke existing access\./),
      ]);
    });

    it('renders nothing while the flows are still resolving', () => {
      withFlows({}, true);
      const {container} = renderAction('ACTIVE');

      expect(container).toBeEmptyDOMElement();
    });

    it('renders nothing for a status this build does not know', () => {
      const {container} = renderAction('DEACTIVATED' as AccessStatus);

      expect(container).toBeEmptyDOMElement();
    });

    it('toasts, and renders nothing, when the flows cannot be resolved', () => {
      state.availabilityError = Object.assign(new Error('Forbidden'), {response: {data: {}}});
      withFlows({});
      const {container} = renderAction('ACTIVE');

      expect(container).toBeEmptyDOMElement();
      expect(mockShowToast).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledWith(
        'The account actions could not be loaded. Reload the page to try again.',
        'error',
      );
    });

    it('toasts a failed resolution once, not on every render', () => {
      state.availabilityError = new Error('boom');
      withFlows({});
      const {rerender} = renderAction('ACTIVE');
      rerender(<AccountStatusAction category="user" subject="user-1" listQueryKey={LIST_KEY} />);

      expect(mockShowToast).toHaveBeenCalledTimes(1);
    });

    it('does not toast when the flows resolve', () => {
      renderAction('ACTIVE');

      expect(mockShowToast).not.toHaveBeenCalled();
    });

    it('renders nothing when no flow is available for the operation', () => {
      // There is no native fallback, so an action without a flow cannot run.
      withFlows({unsuspend: 'flow-unsuspend'});
      const {container} = renderAction('ACTIVE');

      expect(container).toBeEmptyDOMElement();
    });
  });

  describe('running an action', () => {
    it('confirms before suspending rather than firing from the menu', () => {
      renderAction('ACTIVE');
      openMenu();
      choose('Suspend');

      expect(mockMutate).not.toHaveBeenCalled();
      expect(screen.getByText('Suspend user')).toBeInTheDocument();
    });

    it('clears the previous action error before opening the next one', () => {
      renderAction('LOCKED');
      openMenu();
      choose('Suspend');

      expect(mockReset).toHaveBeenCalled();
    });

    it('sends no operator note when none was given', () => {
      renderAction('ACTIVE');
      openMenu();
      choose('Suspend');
      confirm('Suspend');

      expect(mockMutate).toHaveBeenCalledWith(
        {action: 'suspend', category: 'user', flowIds: ['flow-suspend'], operatorNote: undefined, subject: 'user-1'},
        expect.anything(),
      );
    });

    it("sends the operator's note, trimmed", () => {
      renderAction('ACTIVE');
      openMenu();
      choose('Suspend');
      fireEvent.change(within(screen.getByRole('dialog')).getByLabelText('Operator note (optional)'), {
        target: {value: '  Credential exposure  '},
      });
      confirm('Suspend');

      expect(mockMutate).toHaveBeenCalledWith(
        {
          action: 'suspend',
          category: 'user',
          flowIds: ['flow-suspend'],
          operatorNote: 'Credential exposure',
          subject: 'user-1',
        },
        expect.anything(),
      );
    });

    it('runs unlock straight from the menu', () => {
      renderAction('LOCKED');
      openMenu();
      choose('Unlock');

      expect(mockMutate).toHaveBeenCalledWith(
        {action: 'unlock', category: 'user', flowIds: ['flow-unlock'], operatorNote: undefined, subject: 'user-1'},
        expect.anything(),
      );
    });

    it('runs unsuspend before unlock for Unsuspend and unlock', () => {
      renderAction('SUSPENDED');
      openMenu();
      choose('Unsuspend and unlock');
      confirm('Unsuspend and unlock');

      expect(mockMutate).toHaveBeenCalledWith(
        {
          action: 'unsuspendAndUnlock',
          category: 'user',
          flowIds: ['flow-unsuspend', 'flow-unlock'],
          operatorNote: undefined,
          subject: 'user-1',
        },
        expect.anything(),
      );
    });

    it("hands the host's list query to the action, so the list shows the new status", () => {
      renderAction('ACTIVE');

      expect(state.listQueryKeys.at(-1)).toBe(LIST_KEY);
    });

    it('clears the action error when the unlock fails after the release, so the closing dialog shows none', () => {
      const onChanged = vi.fn();
      renderAction('SUSPENDED', 'user', onChanged);
      openMenu();
      choose('Unsuspend and unlock');
      confirm('Unsuspend and unlock');
      mockReset.mockClear();

      const [, options] = mockMutate.mock.calls[0] as [unknown, {onError: (error: Error) => void}];
      act(() => {
        options.onError(new LifecycleActionIncompleteError(new Error('unlock failed')));
      });

      expect(mockReset).toHaveBeenCalledTimes(1);
      expect(onChanged).toHaveBeenCalledTimes(1);
    });

    it('keeps the action error for a dialog failure', () => {
      renderAction('ACTIVE');
      openMenu();
      choose('Suspend');
      confirm('Suspend');
      mockReset.mockClear();

      const [, options] = mockMutate.mock.calls[0] as [unknown, {onError: (error: Error) => void}];
      act(() => {
        options.onError(new Error('refused'));
      });

      expect(mockReset).not.toHaveBeenCalled();
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('keeps the dialog content through its exit instead of switching to Suspend', async () => {
      renderAction('SUSPENDED');
      openMenu();
      choose('Unsuspend');
      fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Cancel'}));

      expect(screen.getByText('Unsuspend user')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
    });
  });

  describe('results', () => {
    beforeEach(() => {
      state.realMutation = true;
    });

    it.each<[LifecycleCategory, string]>([
      ['user', 'User suspended'],
      ['agent', 'Agent suspended'],
      ['application', 'Application suspended'],
    ])('closes the dialog and reports the change on success, naming the %s', async (category, message) => {
      mockHttpRequest.mockResolvedValue(COMPLETE);
      const onChanged = vi.fn();
      renderAction('ACTIVE', category, onChanged);
      openMenu();
      choose('Suspend');
      confirm('Suspend');

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
      expect(onChanged).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledWith(message, 'success');
    });

    it.each<[LifecycleCategory, string]>([
      ['user', 'The user operation could not be completed.'],
      ['agent', 'The agent operation could not be completed.'],
      ['application', 'The application operation could not be completed.'],
    ])('names the %s in a failure without a known code', async (category, message) => {
      mockHttpRequest.mockRejectedValue(new Error('network down'));
      renderAction('ACTIVE', category);
      openMenu();
      choose('Suspend');
      confirm('Suspend');

      expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent(message);
    });

    it.each<[LifecycleCategory, string]>([
      ['user', 'The user operation could not be completed.'],
      ['agent', 'The agent operation could not be completed.'],
    ])('toasts an unlock failure without a known code naming the %s', async (category, message) => {
      mockHttpRequest.mockRejectedValue(new Error('network down'));
      renderAction('LOCKED', category);
      openMenu();
      choose('Unlock');

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith(message, 'error');
      });
    });

    it('shows a failure inside the dialog and keeps it open', async () => {
      mockHttpRequest.mockResolvedValue(REFUSED);
      const onChanged = vi.fn();
      renderAction('ACTIVE', 'user', onChanged);
      openMenu();
      choose('Suspend');
      confirm('Suspend');

      const dialog = screen.getByRole('dialog');
      expect(await within(dialog).findByRole('alert')).toHaveTextContent(FET_1112);
      // Still open once any exit fade would have finished.
      await expect(
        waitFor(
          () => {
            expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
          },
          {timeout: 1000},
        ),
      ).rejects.toThrow();
      expect(onChanged).not.toHaveBeenCalled();
      expect(mockShowToast).not.toHaveBeenCalled();
    });

    it('toasts an unlock failure, since unlock has no dialog', async () => {
      mockHttpRequest.mockResolvedValue(REFUSED);
      renderAction('LOCKED');
      openMenu();
      choose('Unlock');

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith(FET_1112, 'error');
      });
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('disables the controls while the action runs', async () => {
      mockHttpRequest.mockReturnValue(new Promise(() => undefined));
      renderAction('ACTIVE');
      openMenu();
      choose('Suspend');
      confirm('Suspend');

      await waitFor(() => {
        expect(within(screen.getByRole('dialog')).getByRole('button', {name: /Suspend/})).toBeDisabled();
      });
      expect(screen.getByTestId('account-status-actions')).toBeDisabled();
      expect(within(screen.getByRole('dialog')).getByRole('progressbar', {name: 'Loading...'})).toBeInTheDocument();
    });

    it('closes the dialog, refetches and reports when the unlock fails after the release', async () => {
      mockHttpRequest.mockResolvedValueOnce(COMPLETE).mockResolvedValueOnce(REFUSED);
      const onChanged = vi.fn();
      renderAction('SUSPENDED', 'user', onChanged);
      openMenu();
      choose('Unsuspend and unlock');
      confirm('Unsuspend and unlock');

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
      expect(onChanged).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledTimes(1);
      expect(mockShowToast).toHaveBeenCalledWith(
        'The suspension was removed, but the unlock failed. Unlock it from the Actions menu.',
        'error',
      );
    });

    it('shows no error in the dialog while it closes after the unlock fails', async () => {
      mockHttpRequest.mockResolvedValueOnce(COMPLETE).mockResolvedValueOnce(REFUSED);
      renderAction('SUSPENDED');
      openMenu();
      choose('Unsuspend and unlock');
      confirm('Unsuspend and unlock');

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalled();
      });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });
});
