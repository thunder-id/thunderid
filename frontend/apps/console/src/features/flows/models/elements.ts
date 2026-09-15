// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Base} from './base';

/**
 * Interface for a component.
 */
export interface Element<T = unknown> extends Base<T> {
  /**
   * Nested set of elements.
   * @remarks Some elements like `Form` can have nested elements.
   */
  components?: Element[];
  action?: {
    onSuccess?: string;
    [key: string]: unknown;
  };
  /**
   * Semantic prompt action this element raises (see {@link PromptActionTypes}).
   * Lives on the element rather than the edge so the choice survives redrawing
   * the connection, and round-trips for free through `meta.components`.
   */
  actionType?: string;
  /**
   * Space-separated list of CSS class names to apply to the rendered element.
   */
  classes?: string;
}

export const ElementCategories = {
  Action: 'ACTION',
  Block: 'BLOCK',
  Display: 'DISPLAY',
  Field: 'FIELD',
  Miscellaneous: 'MISCELLANEOUS',
} as const;

export const ElementTypes = {
  TextInput: 'TEXT_INPUT',
  PasswordInput: 'PASSWORD_INPUT',
  EmailInput: 'EMAIL_INPUT',
  PhoneInput: 'PHONE_INPUT',
  NumberInput: 'NUMBER_INPUT',
  DateInput: 'DATE_INPUT',
  OtpInput: 'OTP_INPUT',
  Checkbox: 'CHECKBOX',
  Dropdown: 'DROPDOWN',
  Select: 'SELECT',
  Action: 'ACTION',
  Captcha: 'CAPTCHA',
  Divider: 'DIVIDER',
  Icon: 'ICON',
  Image: 'IMAGE',
  RichText: 'RICH_TEXT',
  Stack: 'STACK',
  Text: 'TEXT',
  DynamicInputPlaceholder: 'DYNAMIC_INPUT_PLACEHOLDER',
  Resend: 'RESEND',
  Timer: 'TIMER',
  QrCode: 'QR_CODE',
  Consent: 'CONSENT',
  ConsentInput: 'CONSENT_INPUT',
  Custom: 'CUSTOM',
} as const;

export const BlockTypes = {
  Form: 'BLOCK',
} as const;

export const InputVariants = {
  Text: 'TEXT',
  Password: 'PASSWORD',
  Email: 'EMAIL',
  Telephone: 'TELEPHONE',
  Number: 'NUMBER',
  Checkbox: 'CHECKBOX',
  OTP: 'OTP',
} as const;

export const ButtonVariants = {
  Primary: 'PRIMARY',
  Secondary: 'SECONDARY',
  Outlined: 'OUTLINED',
  Text: 'TEXT',
} as const;

export const ButtonTypes = {
  Submit: 'submit',
  Button: 'button',
} as const;

export const TypographyVariants = {
  H1: 'HEADING_1',
  H2: 'HEADING_2',
  H3: 'HEADING_3',
  H4: 'HEADING_4',
  H5: 'HEADING_5',
  H6: 'HEADING_6',
  Body1: 'BODY_1',
  Body2: 'BODY_2',
} as const;

export const TypographyColors = {
  Error: 'ERROR',
  Warning: 'WARNING',
  Success: 'SUCCESS',
  Info: 'INFO',
  Primary: 'PRIMARY',
  Secondary: 'SECONDARY',
} as const;

export const DividerVariants = {
  Horizontal: 'HORIZONTAL',
  Vertical: 'VERTICAL',
} as const;

/**
 * Event types for ACTION components.
 * Defines the interaction semantics for buttons and actions.
 */
export const ActionEventTypes = {
  Trigger: 'TRIGGER',
  Submit: 'SUBMIT',
  Navigate: 'NAVIGATE',
  Cancel: 'CANCEL',
  Reset: 'RESET',
  Back: 'BACK',
} as const;

export type ActionEventTypes = (typeof ActionEventTypes)[keyof typeof ActionEventTypes];

/**
 * Semantic type of the prompt action a button raises, serialized as
 * `prompts[].action.type` and forwarded by the prompt node to the next
 * executor as edge metadata. Distinct from {@link ActionEventTypes}, which
 * describes how the button behaves in the rendered form, and from
 * `action.type` on the element, which carries canvas navigation semantics.
 *
 * The values are deliberately not tied to a single use case: `Confirm` is read
 * by the session sign-out executor today, but any executor that routes to a
 * confirmation prompt can consume it.
 */
export const PromptActionTypes = {
  Confirm: 'CONFIRM',
} as const;

export type PromptActionTypes = (typeof PromptActionTypes)[keyof typeof PromptActionTypes];
