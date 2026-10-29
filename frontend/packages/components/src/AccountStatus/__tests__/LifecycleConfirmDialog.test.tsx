// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {LifecycleAction, LifecycleCategory} from '../lifecycleAction';
import LifecycleConfirmDialog from '../LifecycleConfirmDialog';

const state = vi.hoisted(() => ({mismatches: [] as string[]}));

// The package has no i18next instance, so `t` reads the built en-US catalog and records any
// fallback that differs from it.
vi.mock('react-i18next', async () => {
  const {default: enUS} = await import('@thunderid/i18n/locales/en-US');
  const common = (enUS as unknown as {common: Record<string, string>}).common;
  const t = (key: string, fallback: string): string => {
    const entry = common[key.replace(/^common:/, '')];
    if (entry !== undefined && entry !== fallback) {
      state.mismatches.push(key);
    }
    return entry ?? fallback;
  };
  return {useTranslation: () => ({t})};
});

function renderDialog(
  operation: LifecycleAction,
  category: LifecycleCategory = 'user',
  props: Partial<{submitting: boolean; error: string; onConfirm: () => void; onCancel: () => void}> = {},
) {
  const onConfirm = props.onConfirm ?? vi.fn();
  const onCancel = props.onCancel ?? vi.fn();
  const view = render(
    <LifecycleConfirmDialog
      open
      operation={operation}
      category={category}
      submitting={props.submitting ?? false}
      error={props.error}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />,
  );
  return {...view, dialog: screen.getByRole('dialog'), onCancel, onConfirm};
}

describe('LifecycleConfirmDialog', () => {
  beforeEach(() => {
    state.mismatches = [];
  });

  afterEach(() => {
    expect(state.mismatches).toEqual([]);
  });

  it.each<[LifecycleCategory, string, RegExp]>([
    ['user', 'Suspend user', /no longer be able to sign in/],
    ['agent', 'Suspend agent', /still update the agent details and credentials/],
    ['application', 'Suspend application', /application will no longer be able to authenticate/],
  ])('names the category when suspending a %s', (category, title, body) => {
    const {dialog} = renderDialog('suspend', category);

    expect(within(dialog).getByRole('heading', {name: title})).toBeInTheDocument();
    expect(within(dialog).getByText(body)).toBeInTheDocument();
    expect(dialog.textContent ?? '').not.toMatch(/lock/i);
  });

  it.each<[LifecycleCategory, RegExp, RegExp]>([
    ['user', /verify the user's contact information/, /will remain locked after unsuspension/],
    ['agent', /rotate any potentially compromised agent secrets/, /unless the agent is also locked/],
    ['application', /rotate any potentially compromised application secrets/, /restore the application's access/],
  ])('gives the guidance before the outcome when unsuspending a %s', (category, guidance, body) => {
    const {dialog} = renderDialog('unsuspend', category);

    expect(within(dialog).getByText(guidance)).toBeInTheDocument();
    expect(within(dialog).getByText(body)).toBeInTheDocument();
  });

  it('says Unsuspend and unlock lets the user sign in again', () => {
    const {dialog} = renderDialog('unsuspendAndUnlock');

    expect(within(dialog).getByRole('heading', {name: 'Unsuspend and unlock user'})).toBeInTheDocument();
    expect(within(dialog).getByText(/allowing the user to sign in again/)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', {name: 'Unsuspend and unlock'})).toBeInTheDocument();
  });

  it('collects an operator note only when suspending a user or an agent', () => {
    expect(within(renderDialog('suspend', 'agent').dialog).getByLabelText('Operator note (optional)')).toBeVisible();
  });

  it.each<[LifecycleAction, LifecycleCategory]>([
    ['suspend', 'application'],
    ['unsuspend', 'user'],
  ])('collects no operator note for %s on a %s', (operation, category) => {
    const {dialog} = renderDialog(operation, category);

    expect(within(dialog).queryByLabelText('Operator note (optional)')).not.toBeInTheDocument();
  });

  it('confirms with the trimmed note, or none when the field is blank', () => {
    const {dialog, onConfirm} = renderDialog('suspend');
    const field = within(dialog).getByLabelText('Operator note (optional)');

    fireEvent.change(field, {target: {value: '   '}});
    fireEvent.click(within(dialog).getByRole('button', {name: 'Suspend'}));
    fireEvent.change(field, {target: {value: ' Leaked key '}});
    fireEvent.click(within(dialog).getByRole('button', {name: 'Suspend'}));

    expect(onConfirm).toHaveBeenNthCalledWith(1, undefined);
    expect(onConfirm).toHaveBeenNthCalledWith(2, 'Leaked key');
  });

  it('caps the note at the server limit', () => {
    const {dialog} = renderDialog('suspend');

    expect(within(dialog).getByLabelText('Operator note (optional)')).toHaveAttribute('maxlength', '100');
  });

  it('shows the error inside the dialog', () => {
    const {dialog} = renderDialog('suspend', 'user', {error: 'It failed.'});

    expect(within(dialog).getByRole('alert')).toHaveTextContent('It failed.');
  });

  it('disables both buttons and labels the spinner while submitting', () => {
    const {dialog} = renderDialog('suspend', 'user', {submitting: true});

    expect(within(dialog).getByRole('button', {name: 'Cancel'})).toBeDisabled();
    expect(within(dialog).getByRole('button', {name: /Suspend/})).toBeDisabled();
    expect(within(dialog).getByRole('progressbar', {name: 'Loading...'})).toBeInTheDocument();
  });

  it('clears the note once the dialog has closed', async () => {
    const {dialog, rerender} = renderDialog('suspend');
    fireEvent.change(within(dialog).getByLabelText('Operator note (optional)'), {target: {value: 'stale'}});

    const element = (open: boolean) => (
      <LifecycleConfirmDialog
        open={open}
        operation="suspend"
        category="user"
        submitting={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />
    );
    rerender(element(false));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    rerender(element(true));

    expect(within(screen.getByRole('dialog')).getByLabelText('Operator note (optional)')).toHaveValue('');
  });
});
