// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, within, waitFor} from '@testing-library/react';
import type {AccessStatus, LifecycleCategory, LifecycleOperation} from '@thunderid/types';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AccountStatusAction from '../AccountStatusAction';
import {LifecycleActionIncompleteError} from '../lifecycleAction';

const {mockUseLifecycleActions, mockMutate, mockReset, mockShowToast} = vi.hoisted(() => ({
  mockMutate: vi.fn(),
  mockReset: vi.fn(),
  mockShowToast: vi.fn(),
  mockUseLifecycleActions: vi.fn(),
}));

vi.mock('../useLifecycleActions', () => ({default: mockUseLifecycleActions}));

vi.mock('../useLifecycleAction', () => ({
  default: () => ({
    error: null,
    isError: false,
    isPending: false,
    mutate: mockMutate,
    reset: mockReset,
  }),
}));

vi.mock('@thunderid/contexts', () => ({
  useToast: () => ({showToast: mockShowToast}),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const ALL_FLOWS: Partial<Record<LifecycleOperation, string>> = {
  suspend: 'flow-suspend',
  unlock: 'flow-unlock',
  unsuspend: 'flow-unsuspend',
};

function withFlows(available: Partial<Record<LifecycleOperation, string>>, isPending = false): void {
  mockUseLifecycleActions.mockReturnValue({available, isPending});
}

function renderAction(status?: AccessStatus, category: LifecycleCategory = 'user', onChanged?: () => void): void {
  render(
    <AccountStatusAction
      category={category}
      subject="user-1"
      status={status === undefined ? undefined : {value: status}}
      onChanged={onChanged}
    />,
  );
}

/**
 * Opens the Actions menu and returns the operations it offers, by label.
 *
 * The control is one menu rather than a row of buttons, so every assertion about what is on offer
 * goes through opening it. A test that queried for a bare button would pass against a header that
 * rendered nothing at all.
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

describe('AccountStatusAction', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    withFlows(ALL_FLOWS);
  });

  it('offers only Suspend on an active account', () => {
    // Unlock is withheld here on purpose: an active account can hold a stale failure count, but an
    // action whose effect is invisible is the worse of the two confusions. Unsuspend is withheld
    // because there is nothing to release.
    renderAction('ACTIVE');

    const offered = openMenu();

    expect(offered).toHaveLength(1);
    expect(offered[0]).toMatch(/^Suspend/);
  });

  it('treats an absent status as active', () => {
    // The server omits the field for the ordinary case rather than sending ACTIVE.
    renderAction(undefined);

    expect(openMenu().some((label) => label.startsWith('Suspend'))).toBe(true);
  });

  it('offers Unsuspend, and Unsuspend and unlock, on a suspended account', () => {
    // A bare Unlock is withheld: beside Unsuspend it invites the reading that unlocking releases the
    // suspension. The combined action says what it does.
    renderAction('SUSPENDED');

    const offered = openMenu();

    expect(offered).toHaveLength(2);
    expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
    expect(offered[1]).toMatch(/^Unsuspend and unlock/);
  });

  it('offers only Unsuspend on a suspended agent, which lands on ACTIVE', () => {
    renderAction('SUSPENDED', 'agent');

    const offered = openMenu();

    expect(offered).toHaveLength(1);
    expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
  });

  it('offers only Unsuspend on a suspended application, which has no unlock flow', () => {
    withFlows({suspend: 'flow-suspend', unsuspend: 'flow-unsuspend'});
    renderAction('SUSPENDED', 'application');

    const offered = openMenu();

    expect(offered).toHaveLength(1);
    expect(offered[0]).toMatch(/^Unsuspend(?! and unlock)/);
  });

  it('tells a user operator which release lands on a lock and which signs in straight away', () => {
    renderAction('SUSPENDED');

    const offered = openMenu();

    expect(offered[0]).toContain('remains locked');
    expect(offered[1]).toContain('can sign in right away');
  });

  it('offers Unlock and Suspend on a locked account', () => {
    renderAction('LOCKED');

    const offered = openMenu();

    expect(offered.some((label) => label.startsWith('Unlock'))).toBe(true);
    expect(offered.some((label) => label.startsWith('Suspend'))).toBe(true);
  });

  it('says what each operation does, not only its name', () => {
    // Unsuspend and Unlock both read as "release", and the menu is where the choice is made. Saying
    // so at the point of choosing is cheaper than saying it in a dialog reached only after choosing.
    renderAction('SUSPENDED');

    expect(openMenu().some((label) => label.includes('Remove the suspension'))).toBe(true);
  });

  it('does not offer the remediation window as a reason to suspend', () => {
    // A suspended record stays editable, which is the point of the state and is said in the
    // confirmation. It is not what the operator is deciding as they read the menu, and a line about
    // editing under a destructive action reads as a reassurance rather than a consequence.
    renderAction('ACTIVE');

    const offered = openMenu();

    expect(offered.some((label) => label.includes('editable'))).toBe(false);
    expect(offered.some((label) => label.includes('Prevent sign-in and revoke existing access'))).toBe(true);
  });

  it('never offers an operator lock', () => {
    // Locking is automatic and only ever caused by failed authentication. An operator holding an
    // account uses Suspend.
    renderAction('ACTIVE');

    expect(openMenu().some((label) => label === 'Lock')).toBe(false);
  });

  it('renders nothing while the flows are still resolving', () => {
    // A header is not a place to explain an absence, and a control that appears a moment after the
    // page does is worse than one that arrives with it.
    withFlows({}, true);
    const {container} = render(<AccountStatusAction category="user" subject="user-1" status={{value: 'ACTIVE'}} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing for a status this build does not know', () => {
    // No action table for it, and offering the active account's Suspend would act on a state
    // nobody here can describe.
    const {container} = render(
      <AccountStatusAction category="user" subject="user-1" status={{value: 'DEACTIVATED' as never}} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when the deployment configures no flow for the operation', () => {
    // Unlike deletion there is no REST fallback, so an unresolved flow means the action genuinely
    // cannot run and the console withholds it rather than offering a control that throws.
    withFlows({unsuspend: 'flow-unsuspend'});
    const {container} = render(<AccountStatusAction category="user" subject="user-1" status={{value: 'ACTIVE'}} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('confirms before suspending rather than firing from the menu', () => {
    renderAction('ACTIVE');
    openMenu();
    choose('Suspend');

    expect(mockMutate).not.toHaveBeenCalled();
    expect(screen.getByText('Suspend this account?')).toBeInTheDocument();
  });

  it('clears the previous action error before opening the next one', () => {
    renderAction('LOCKED');
    openMenu();
    choose('Suspend');

    expect(mockReset).toHaveBeenCalled();
  });

  it('runs the suspend flow once confirmed, with no operator note when none was given', () => {
    // The operator note is optional at every layer, so an untouched field must send nothing rather than an
    // empty string the server would store.
    renderAction('ACTIVE');
    openMenu();
    choose('Suspend');
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Suspend'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {action: 'suspend', flowIds: ['flow-suspend'], operatorNote: undefined, subject: 'user-1'},
      expect.anything(),
    );
  });

  it("carries the operator's note through to the flow when one is given", () => {
    renderAction('ACTIVE');
    openMenu();
    choose('Suspend');

    const dialog = screen.getByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Operator note (optional)'), {
      target: {value: '  Credential exposure  '},
    });
    fireEvent.click(within(dialog).getByRole('button', {name: 'Suspend'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {action: 'suspend', flowIds: ['flow-suspend'], operatorNote: 'Credential exposure', subject: 'user-1'},
      expect.anything(),
    );
  });

  it('runs unlock straight from the menu', () => {
    // Remedial and reversible by simply failing again, and it is what the operator came to do. A
    // dialog in front of the recovery path is friction on the wrong side.
    renderAction('LOCKED');
    openMenu();
    choose('Unlock');

    expect(mockMutate).toHaveBeenCalledWith(
      {action: 'unlock', flowIds: ['flow-unlock'], operatorNote: undefined, subject: 'user-1'},
      expect.anything(),
    );
  });

  it('confirms before unsuspending, and says what the release lands on for a user', () => {
    // The operation whose name most misleads. For a user it releases the hold and leaves the
    // account unable to sign in, so the copy has to say what still has to happen.
    renderAction('SUSPENDED');
    openMenu();
    choose('Unsuspend');

    expect(mockMutate).not.toHaveBeenCalled();
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Unsuspend this account?')).toBeInTheDocument();
    expect(within(dialog).getByText(/remains locked/)).toBeInTheDocument();
  });

  it('confirms Unsuspend and unlock, then runs unsuspend before unlock', () => {
    // Unsuspend writes the post-suspension lock; unlock run first would leave it in place.
    renderAction('SUSPENDED');
    openMenu();
    choose('Unsuspend and unlock');

    expect(mockMutate).not.toHaveBeenCalled();
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Unsuspend and unlock this account?')).toBeInTheDocument();
    expect(within(dialog).getByText(/can sign in right away/)).toBeInTheDocument();
    expect(within(dialog).queryByText(/replaced its credentials/)).not.toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', {name: 'Unsuspend and unlock'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        action: 'unsuspendAndUnlock',
        flowIds: ['flow-unsuspend', 'flow-unlock'],
        operatorNote: undefined,
        subject: 'user-1',
      },
      expect.anything(),
    );
  });

  it('refetches and reports when the unlock fails after the release', () => {
    // The record already changed, so the dialog closes on the refetched status rather than holding
    // an error over an action that can no longer be repeated.
    const onChanged = vi.fn();
    mockMutate.mockImplementationOnce((_variables, options: {onError: (error: Error) => void}) => {
      options.onError(new LifecycleActionIncompleteError(new Error('unlock failed')));
    });
    renderAction('SUSPENDED', 'user', onChanged);
    openMenu();
    choose('Unsuspend and unlock');
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Unsuspend and unlock'}));

    expect(onChanged).toHaveBeenCalled();
    expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining('the unlock failed'), 'error');
  });

  it('tells an agent operator that unsuspend returns it to service', () => {
    // An agent lands on ACTIVE: it has no recovery flow, so a held landing would have no clearer
    // that is not a second operator click.
    renderAction('SUSPENDED', 'agent');
    openMenu();
    choose('Unsuspend');

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Unsuspend this agent?')).toBeInTheDocument();
    expect(within(dialog).queryByText(/remains locked/)).not.toBeInTheDocument();
  });

  it('never calls a suspension a lock', () => {
    // A suspension is not a lock, and borrowing the word would suggest it lifts on its own.
    renderAction('ACTIVE');
    openMenu();
    choose('Suspend');

    expect(screen.getByText('Suspend this account?')).toBeInTheDocument();
    expect(screen.getByRole('dialog').textContent ?? '').not.toMatch(/lock/i);
  });

  it('offers no operator note field on unsuspend', () => {
    // Only the operation that places the hold records why. A field on the release would collect text
    // nothing stores.
    renderAction('SUSPENDED');
    openMenu();
    choose('Unsuspend');

    expect(within(screen.getByRole('dialog')).queryByLabelText('Operator note (optional)')).not.toBeInTheDocument();
  });

  it('names the category in the confirmation', () => {
    renderAction('ACTIVE', 'application');
    openMenu();
    choose('Suspend');

    expect(screen.getByText('Suspend this application?')).toBeInTheDocument();
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).queryByLabelText('Operator note (optional)')).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', {name: 'Suspend'}));
    expect(mockMutate).toHaveBeenCalledWith(
      {action: 'suspend', flowIds: ['flow-suspend'], operatorNote: undefined, subject: 'user-1'},
      expect.any(Object),
    );
  });

  it('clears the operator note once the dialog has closed', async () => {
    renderAction('ACTIVE');
    openMenu();
    choose('Suspend');
    fireEvent.change(within(screen.getByRole('dialog')).getByLabelText('Operator note (optional)'), {
      target: {value: 'stale note'},
    });
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Cancel'}));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    openMenu();
    choose('Suspend');

    expect(within(screen.getByRole('dialog')).getByLabelText('Operator note (optional)')).toHaveValue('');
  });

  it('keeps the dialog content through its exit instead of switching to Suspend', async () => {
    renderAction('SUSPENDED');
    openMenu();
    choose('Unsuspend');
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Cancel'}));

    expect(screen.getByText('Unsuspend this account?')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });
});
