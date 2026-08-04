// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup, fireEvent} from '@testing-library/react';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {describe, it, expect, afterEach, vi} from 'vitest';
import renderWithProviders from '../../../../test/renderWithProviders';
import BlockAdapter from '../BlockAdapter';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const noop = () => undefined;

const RECOVERY_LABEL =
  '<p data-component-ref="recovery-link"><a href="#" data-action-ref="action_forgot_password">Forgot password?</a></p>';

/** Resolves the recovery meta switch; every other template passes through. */
const resolveWithRecovery = (enabled: boolean) => (template: string | undefined) =>
  template?.includes('isRecoveryFlowEnabled') ? String(enabled) : template;

const recoveryRichText = {
  id: 'rich_text_forgot_password',
  type: 'RICH_TEXT',
  label: RECOVERY_LABEL,
  action: {ref: 'action_forgot_password'},
};

const usernameInput = {id: 'input_001', type: 'TEXT_INPUT', identifier: 'username', label: 'Username'};
const passwordInput = {id: 'input_002', type: 'PASSWORD_INPUT', identifier: 'password', label: 'Password'};
const submitAction = {id: 'action_001', type: 'ACTION', eventType: 'SUBMIT', variant: 'PRIMARY', label: 'Sign in'};
const socialTrigger = {id: 'action_google', type: 'ACTION', eventType: 'TRIGGER', label: 'Continue with Google'};

const BLOCK_VALUES = {username: 'alice', password: 's3cret'};

const renderBlock = (
  component: unknown,
  {
    onSubmit = noop,
    recoveryEnabled = true,
    values = BLOCK_VALUES,
  }: {onSubmit?: (...args: unknown[]) => void; recoveryEnabled?: boolean; values?: Record<string, string>} = {},
) =>
  renderWithProviders(
    <BlockAdapter
      component={component as EmbeddedFlowComponent}
      index={0}
      values={values}
      isLoading={false}
      resolve={resolveWithRecovery(recoveryEnabled)}
      onInputChange={noop}
      onSubmit={onSubmit}
    />,
  );

describe('BlockAdapter: RICH_TEXT inside a form block', () => {
  const formBlock = {
    id: 'block_001',
    type: 'BLOCK',
    components: [usernameInput, passwordInput, recoveryRichText, submitAction],
  };

  it('dispatches the wired action when the nested recovery link is clicked', () => {
    const onSubmit = vi.fn();
    renderBlock(formBlock, {onSubmit});

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
  });

  it('carries the block values into the dispatched action', () => {
    const onSubmit = vi.fn();
    renderBlock(formBlock, {onSubmit, values: {username: 'bob'}});

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit.mock.calls[0][1]).toEqual({username: 'bob'});
  });

  it('does not render the recovery link when the recovery flow is disabled', () => {
    renderBlock(formBlock, {recoveryEnabled: false});

    expect(screen.queryByRole('link', {name: 'Forgot password?'})).toBeNull();
    expect(screen.getByRole('button', {name: 'Sign in'})).toBeTruthy();
  });

  it('still dispatches the primary submit action', () => {
    const onSubmit = vi.fn();
    renderBlock(formBlock, {onSubmit});

    fireEvent.click(screen.getByRole('button', {name: 'Sign in'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_001'}), BLOCK_VALUES);
  });

  it('renders a rich text without a wired action as plain content', () => {
    const onSubmit = vi.fn();
    renderBlock({
      id: 'block_plain',
      type: 'BLOCK',
      components: [{id: 'rich_plain', type: 'RICH_TEXT', label: '<p>Sign in to continue</p>'}, submitAction],
    });

    expect(screen.getByText('Sign in to continue')).toBeTruthy();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('dispatches a recovery link nested inside a stack', () => {
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_stacked',
        type: 'BLOCK',
        components: [usernameInput, {id: 'stack_links', type: 'STACK', components: [recoveryRichText]}, submitAction],
      },
      {onSubmit},
    );

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
  });
});

describe('BlockAdapter: RICH_TEXT inside a trigger-only block', () => {
  const triggerBlock = {
    id: 'block_social',
    type: 'BLOCK',
    components: [socialTrigger, recoveryRichText],
  };

  it('renders and dispatches the recovery link', () => {
    const onSubmit = vi.fn();
    renderBlock(triggerBlock, {onSubmit});

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
  });

  it('does not render the recovery link when the recovery flow is disabled', () => {
    renderBlock(triggerBlock, {recoveryEnabled: false});

    expect(screen.queryByRole('link', {name: 'Forgot password?'})).toBeNull();
    expect(screen.getByRole('button', {name: 'Continue with Google'})).toBeTruthy();
  });

  it('dispatches a recovery link nested inside a stack', () => {
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_social_stacked',
        type: 'BLOCK',
        components: [socialTrigger, {id: 'stack_links', type: 'STACK', components: [recoveryRichText]}],
      },
      {onSubmit},
    );

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
  });
});
