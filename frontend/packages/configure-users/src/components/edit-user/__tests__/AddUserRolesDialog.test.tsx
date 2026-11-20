// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {UserRole} from '../../../models/users';
import AddUserRolesDialog from '../AddUserRolesDialog';

const editor: UserRole = {id: 'role-editor', name: 'Editor', description: 'Can edit', ouId: 'ou-1'};
const viewer: UserRole = {id: 'role-viewer', name: 'Viewer', ouId: 'ou-1'};

describe('AddUserRolesDialog', () => {
  it('lists the roles, with a dash for one without a description', async () => {
    render(<AddUserRolesDialog open roles={[editor, viewer]} onClose={vi.fn()} onAdd={vi.fn()} />);

    const dialog = await screen.findByRole('dialog', {name: 'Add roles'});
    expect(within(dialog).getByText('Can edit')).toBeInTheDocument();
    expect(within(dialog).getByText('-')).toBeInTheDocument();
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', {name: 'Add'})).toBeDisabled();
  });

  it('adds the picked roles', async () => {
    const onAdd = vi.fn();
    const user = userEvent.setup();
    render(<AddUserRolesDialog open roles={[editor, viewer]} onClose={vi.fn()} onAdd={onAdd} />);

    const row = (await screen.findByText('Viewer')).closest('[role="row"]')!;
    await user.click(within(row as HTMLElement).getByRole('checkbox'));
    await user.click(screen.getByRole('button', {name: 'Add (1)'}));

    expect(onAdd).toHaveBeenCalledWith(['role-viewer']);
  });

  it('closes on cancel', async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<AddUserRolesDialog open roles={[editor]} onClose={onClose} onAdd={vi.fn()} />);

    await user.click(await screen.findByRole('button', {name: 'Cancel'}));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('shows why the last add failed and locks the actions while submitting', async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(
      <AddUserRolesDialog
        open
        roles={[editor]}
        onClose={onClose}
        onAdd={vi.fn()}
        error="Could not assign the roles Editor. Try again."
        isSubmitting
      />,
    );

    const dialog = await screen.findByRole('dialog', {name: 'Add roles'});
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Could not assign the roles Editor. Try again.');
    expect(within(dialog).getByRole('button', {name: 'Cancel'})).toBeDisabled();

    // Escape does not close the dialog while the roles are being assigned.
    await user.keyboard('{Escape}');

    expect(onClose).not.toHaveBeenCalled();
  });
});
