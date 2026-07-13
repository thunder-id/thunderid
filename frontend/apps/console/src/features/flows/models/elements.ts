/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
  Consent: 'CONSENT',
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
