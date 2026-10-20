// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup, fireEvent} from '@testing-library/react';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import type {JSX} from 'react';
import {describe, it, expect, afterEach, vi} from 'vitest';
import type {RenderFlowField} from '../../../../models/flow';
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

/** Same link with no id, so the renderer falls back to the component index for the key. */
const recoveryRichTextWithoutId = {
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
    onValidate = undefined,
    renderField = undefined,
    isSubmitDisabled = undefined,
    additionalData = undefined,
  }: {
    onSubmit?: (...args: unknown[]) => void;
    recoveryEnabled?: boolean;
    values?: Record<string, string>;
    onValidate?: (components: EmbeddedFlowComponent[]) => boolean;
    renderField?: RenderFlowField;
    isSubmitDisabled?: boolean;
    additionalData?: Record<string, unknown>;
  } = {},
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
      onValidate={onValidate}
      renderField={renderField}
      isSubmitDisabled={isSubmitDisabled}
      additionalData={additionalData}
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

  it('dispatches a recovery link that has no id', () => {
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_no_rich_text_id',
        type: 'BLOCK',
        components: [usernameInput, recoveryRichTextWithoutId, submitAction],
      },
      {onSubmit},
    );

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
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

  it('dispatches a recovery link that has no id', () => {
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_social_no_rich_text_id',
        type: 'BLOCK',
        components: [socialTrigger, recoveryRichTextWithoutId],
      },
      {onSubmit},
    );

    fireEvent.click(screen.getByRole('link', {name: 'Forgot password?'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_forgot_password'}), BLOCK_VALUES);
  });

  it('renders a divider next to the recovery link', () => {
    renderBlock({
      id: 'block_social_divider',
      type: 'BLOCK',
      components: [socialTrigger, {id: 'divider_001', type: 'DIVIDER', label: 'or'}, recoveryRichText],
    });

    expect(screen.getByText('or')).toBeTruthy();
    expect(screen.getByRole('link', {name: 'Forgot password?'})).toBeTruthy();
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

describe('BlockAdapter: RESEND inside a form block', () => {
  const otpInput = {
    id: 'input_otp',
    type: 'OTP_INPUT',
    ref: 'otp',
    identifier: 'otp',
    label: 'One-time code',
    required: true,
  };
  const verifyAction = {id: 'action_verify', type: 'ACTION', eventType: 'SUBMIT', variant: 'PRIMARY', label: 'Verify'};
  const resendAction = {id: 'action_resend', type: 'RESEND', eventType: 'SUBMIT', label: 'Resend code'};

  const otpBlock = {
    id: 'block_otp',
    type: 'BLOCK',
    components: [otpInput, verifyAction, resendAction],
  };

  const EMPTY_OTP = {otp: ''};

  it('dispatches the resend action rather than the primary submit action', () => {
    const onSubmit = vi.fn();
    renderBlock(otpBlock, {onSubmit, values: EMPTY_OTP});

    fireEvent.click(screen.getByRole('button', {name: 'Resend code'}));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_resend'}), EMPTY_OTP);
  });

  it('dispatches the resend without running field validation on the empty code', () => {
    const onSubmit = vi.fn();
    const onValidate = vi.fn(() => false);
    renderBlock(otpBlock, {onSubmit, onValidate, values: EMPTY_OTP});

    fireEvent.click(screen.getByRole('button', {name: 'Resend code'}));

    expect(onValidate).not.toHaveBeenCalled();
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_resend'}), EMPTY_OTP);
  });

  it('does not submit the enclosing form when the resend button is clicked', () => {
    const onSubmit = vi.fn();
    renderBlock(otpBlock, {onSubmit, values: EMPTY_OTP});

    expect(screen.getByRole('button', {name: 'Resend code'}).getAttribute('type')).toBe('button');
    expect(screen.getByRole('button', {name: 'Verify'}).getAttribute('type')).toBe('submit');
  });

  it('still gates the primary submit action behind field validation', () => {
    const onSubmit = vi.fn();
    const onValidate = vi.fn(() => false);
    renderBlock(otpBlock, {onSubmit, onValidate, values: EMPTY_OTP});

    fireEvent.click(screen.getByRole('button', {name: 'Verify'}));

    expect(onValidate).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('carries the current block values into the dispatched resend action', () => {
    const onSubmit = vi.fn();
    renderBlock(otpBlock, {onSubmit, values: {otp: '12'}});

    fireEvent.click(screen.getByRole('button', {name: 'Resend code'}));

    expect(onSubmit.mock.calls[0][1]).toEqual({otp: '12'});
  });

  it('dispatches a resend button nested inside a stack', () => {
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_otp_stacked',
        type: 'BLOCK',
        components: [otpInput, verifyAction, {id: 'stack_resend', type: 'STACK', components: [resendAction]}],
      },
      {onSubmit, values: EMPTY_OTP},
    );

    fireEvent.click(screen.getByRole('button', {name: 'Resend code'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_resend'}), EMPTY_OTP);
  });

  it('dispatches a resend button that has no id', () => {
    // No id means the renderer falls back to the component index for the key.
    const onSubmit = vi.fn();
    renderBlock(
      {
        id: 'block_otp_no_resend_id',
        type: 'BLOCK',
        components: [otpInput, verifyAction, {type: 'RESEND', eventType: 'SUBMIT', label: 'Resend code'}],
      },
      {onSubmit, values: EMPTY_OTP},
    );

    fireEvent.click(screen.getByRole('button', {name: 'Resend code'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({type: 'RESEND'}), EMPTY_OTP);
  });

  it('does not render a RESEND whose eventType is not SUBMIT', () => {
    renderBlock(
      {
        id: 'block_otp_trigger_resend',
        type: 'BLOCK',
        components: [otpInput, verifyAction, {...resendAction, eventType: 'TRIGGER'}],
      },
      {values: EMPTY_OTP},
    );

    expect(screen.queryByRole('button', {name: 'Resend code'})).toBeNull();
    expect(screen.getByRole('button', {name: 'Verify'})).toBeTruthy();
  });
});

describe('BlockAdapter: BOOLEAN_INPUT inside a form block', () => {
  const activeCheckbox = {id: 'input_003', type: 'BOOLEAN_INPUT', ref: 'active', label: 'Active'};
  const booleanBlock = {
    id: 'block_002',
    type: 'BLOCK',
    components: [usernameInput, activeCheckbox, submitAction],
  };

  it('renders a boolean attribute as a checkbox rather than a text field', () => {
    renderBlock(booleanBlock, {values: {...BLOCK_VALUES, active: 'false'}});

    const checkbox: HTMLInputElement = screen.getByRole('checkbox', {name: 'Active'});
    expect(checkbox.checked).toBe(false);
  });

  it('reflects a checked boolean value', () => {
    renderBlock(booleanBlock, {values: {...BLOCK_VALUES, active: 'true'}});

    const checkbox: HTMLInputElement = screen.getByRole('checkbox', {name: 'Active'});
    expect(checkbox.checked).toBe(true);
  });
});

/** A field renderer that answers for one component type and leaves every other type to the shared adapters. */
const renderFieldFor =
  (type: string, element: JSX.Element | null): RenderFlowField =>
  (component) =>
    (component as {type: string}).type === type ? element : undefined;

const nameInput = {id: 'input_name', type: 'TEXT_INPUT', ref: 'name', label: 'Name'};

describe('BlockAdapter: renderField extension', () => {
  const pickerInput = {id: 'input_picker', type: 'CUSTOM_PICKER', ref: 'picker', label: 'Picker'};
  const formBlock = {id: 'block_001', type: 'BLOCK', components: [nameInput, pickerInput, submitAction]};

  it('renders what the callback returns for a type the shared adapters do not cover', () => {
    renderBlock(formBlock, {renderField: renderFieldFor('CUSTOM_PICKER', <p>custom picker</p>)});

    expect(screen.getByText('custom picker')).toBeTruthy();
  });

  it('leaves every other field to the shared adapters', () => {
    renderBlock(formBlock, {renderField: renderFieldFor('CUSTOM_PICKER', <p>custom picker</p>)});

    expect(document.querySelector('input[name="name"]')).toBeTruthy();
    expect(screen.getByRole('button', {name: 'Sign in'})).toBeTruthy();
  });

  it('hands the callback the component and the block field props', () => {
    const renderField = vi.fn<RenderFlowField>(() => undefined);
    renderBlock(formBlock, {renderField, values: {picker: 'value-1'}, additionalData: {rootOuId: 'ou-1'}});

    const [, fieldProps] = renderField.mock.calls.find(
      ([component]) => (component as {type: string}).type === 'CUSTOM_PICKER',
    )!;

    expect(fieldProps).toMatchObject({
      component: expect.objectContaining({id: 'input_picker'}),
      values: {picker: 'value-1'},
      isLoading: false,
      additionalData: {rootOuId: 'ou-1'},
    });
    expect(fieldProps.onInputChange).toBeInstanceOf(Function);
  });

  it('takes precedence over the shared adapter for the same type', () => {
    renderBlock(formBlock, {
      renderField: renderFieldFor('TEXT_INPUT', <p>overridden</p>),
    });

    expect(screen.getByText('overridden')).toBeTruthy();
    expect(document.querySelector('input[name="name"]')).toBeNull();
  });

  it('omits a field the callback returns null for, without falling back to the shared adapter', () => {
    renderBlock(formBlock, {renderField: renderFieldFor('TEXT_INPUT', null)});

    expect(document.querySelector('input[name="name"]')).toBeNull();
  });

  it('reaches a field nested in a stack', () => {
    const stackedBlock = {
      id: 'block_002',
      type: 'BLOCK',
      components: [{id: 'stack_1', type: 'STACK', components: [pickerInput]}, submitAction],
    };

    renderBlock(stackedBlock, {renderField: renderFieldFor('CUSTOM_PICKER', <p>custom picker</p>)});

    expect(screen.getByText('custom picker')).toBeTruthy();
  });

  it('renders a field without an id', () => {
    const {id: omittedId, ...pickerWithoutId} = pickerInput;
    renderBlock(
      {id: 'block_003', type: 'BLOCK', components: [pickerWithoutId, submitAction]},
      {
        renderField: renderFieldFor('CUSTOM_PICKER', <p>custom picker</p>),
      },
    );

    expect(omittedId).toBe('input_picker');
    expect(screen.getByText('custom picker')).toBeTruthy();
  });

  it('changes nothing when no callback is given', () => {
    renderBlock(formBlock);

    expect(document.querySelector('input[name="name"]')).toBeTruthy();
    expect(screen.queryByText('custom picker')).toBeNull();
  });
});

describe('BlockAdapter: isSubmitDisabled', () => {
  const formBlock = {id: 'block_001', type: 'BLOCK', components: [nameInput, submitAction]};

  it('disables the submit button', () => {
    renderBlock(formBlock, {isSubmitDisabled: true});

    expect(screen.getByRole('button', {name: 'Sign in'}).hasAttribute('disabled')).toBe(true);
  });

  it('leaves the inputs enabled', () => {
    renderBlock(formBlock, {isSubmitDisabled: true});

    expect(document.querySelector<HTMLInputElement>('input[name="name"]')!.disabled).toBe(false);
  });

  it('does not submit the form when it is submitted anyway', () => {
    const onSubmit = vi.fn();
    renderBlock(formBlock, {isSubmitDisabled: true, onSubmit});

    fireEvent.submit(document.querySelector('form')!);

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('submits the form when it is not set', () => {
    const onSubmit = vi.fn();
    renderBlock(formBlock, {onSubmit});

    fireEvent.submit(document.querySelector('form')!);

    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('keeps the submit button disabled while the form is loading, and enabled otherwise', () => {
    renderBlock(formBlock, {isSubmitDisabled: false});

    expect(screen.getByRole('button', {name: 'Sign in'}).hasAttribute('disabled')).toBe(false);
  });
});
