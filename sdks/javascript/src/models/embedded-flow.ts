/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

/**
 * Flow type values used when initiating embedded flows.
 */
export enum EmbeddedFlowType {
  Authentication = 'AUTHENTICATION',
  Recovery = 'RECOVERY',
  Registration = 'REGISTRATION',
  UserOnboarding = 'USER_ONBOARDING',
}

/**
 * Response type values returned by the flow API.
 */
export enum EmbeddedFlowResponseType {
  Redirection = 'REDIRECTION',
  View = 'VIEW',
}

/**
 * Internationalized message structure returned by the backend.
 *
 * The `defaultValue` field carries the untranslated fallback text.
 */
export interface I18nMessage {
  defaultValue?: string;
  key: string;
}

/**
 * Structured error returned in a flow response when flowStatus is ERROR.
 */
export interface FlowExecutionError {
  code: string;
  description: I18nMessage;
  message: I18nMessage;
}

/**
 * Component types supported by the ThunderID embedded flow API.
 *
 * These types define the different UI components that can be rendered
 * as part of the embedded authentication flows. Each type corresponds
 * to a specific UI element with its own behavior and properties.
 *
 * @example
 * ```typescript
 * // Check component type to render appropriate UI
 * if (component.type === EmbeddedFlowComponentType.TextInput) {
 *   // Render text input field
 * } else if (component.type === EmbeddedFlowComponentType.Action) {
 *   // Render button/action
 * }
 * ```
 *
 * @experimental This API may change in future versions
 */
export enum EmbeddedFlowComponentType {
  /** Interactive action component (buttons, links) for user interactions */
  Action = 'ACTION',

  /** Container block component that groups other components */
  Block = 'BLOCK',

  /** Consent component for displaying consent purposes and attributes */
  Consent = 'CONSENT',

  /** Copyable text display component that shows text with a copy-to-clipboard action */
  CopyableText = 'COPYABLE_TEXT',

  /** Divider component for visual separation of content */
  Divider = 'DIVIDER',

  /** Email input field with validation for email addresses. */
  EmailInput = 'EMAIL_INPUT',

  /** Icon display component for rendering named vector icons */
  Icon = 'ICON',

  /** Image display component for logos and illustrations */
  Image = 'IMAGE',

  /** One-time password input field for multi-factor authentication */
  OtpInput = 'OTP_INPUT',

  /** Organization unit tree picker for selecting an OU */
  OuSelect = 'OU_SELECT',

  /** Password input field with masking for sensitive data */
  PasswordInput = 'PASSWORD_INPUT',

  /** Phone number input field with country code support */
  PhoneInput = 'PHONE_INPUT',

  /** Rich text display component that renders formatted HTML content */
  RichText = 'RICH_TEXT',

  /** Select/dropdown input component for single choice selection */
  Select = 'SELECT',

  /** Stack layout component for arranging children in a row or column */
  Stack = 'STACK',

  /** Text display component for labels, headings, and messages */
  Text = 'TEXT',

  /** Standard text input field for user data entry */
  TextInput = 'TEXT_INPUT',

  /** Timer component for displaying a countdown */
  Timer = 'TIMER',

  /** QR code display component for wallet-based flows (e.g. OpenID4VP) */
  QrCode = 'QR_CODE',
}

/**
 * Action variant types for buttons and interactive elements.
 *
 * @experimental This API may change in future versions
 */
export enum EmbeddedFlowActionVariant {
  /** Danger action button for destructive operations */
  Danger = 'DANGER',

  /** Info action button for informational purposes */
  Info = 'INFO',

  /** Link-styled action button */
  Link = 'LINK',

  /** Outlined action button for secondary emphasis */
  Outlined = 'OUTLINED',

  /** Primary action button with highest visual emphasis */
  Primary = 'PRIMARY',

  /** Secondary action button with moderate visual emphasis */
  Secondary = 'SECONDARY',

  /** Success action button for positive confirmations */
  Success = 'SUCCESS',

  /** Tertiary action button with minimal visual emphasis */
  Tertiary = 'TERTIARY',

  /** Warning action button for cautionary actions */
  Warning = 'WARNING',
}

/**
 * Text variant types for typography components.
 *
 * @experimental This API may change in future versions
 */
export enum EmbeddedFlowTextVariant {
  /** Primary body text for main content */
  Body1 = 'BODY_1',

  /** Secondary body text for supplementary content */
  Body2 = 'BODY_2',

  /** Text styled for button labels */
  ButtonText = 'BUTTON_TEXT',

  /** Small caption text for annotations and descriptions */
  Caption = 'CAPTION',

  /** Largest heading level for main titles */
  Heading1 = 'HEADING_1',

  /** Second level heading for major sections */
  Heading2 = 'HEADING_2',

  /** Third level heading for subsections */
  Heading3 = 'HEADING_3',

  /** Fourth level heading for minor sections */
  Heading4 = 'HEADING_4',

  /** Fifth level heading for detailed sections */
  Heading5 = 'HEADING_5',

  /** Smallest heading level for fine-grained sections */
  Heading6 = 'HEADING_6',

  /** Overline text for labels and categories */
  Overline = 'OVERLINE',

  /** Primary subtitle text with larger emphasis */
  Subtitle1 = 'SUBTITLE_1',

  /** Secondary subtitle text with moderate emphasis */
  Subtitle2 = 'SUBTITLE_2',
}

/**
 * Event types for action components.
 *
 * @experimental This API may change in future versions
 */
export enum EmbeddedFlowEventType {
  /** Navigate back to the previous step */
  Back = 'BACK',

  /** Cancel the current operation */
  Cancel = 'CANCEL',

  /** Navigate to a different flow step or page */
  Navigate = 'NAVIGATE',

  /** Reset form fields to initial state */
  Reset = 'RESET',

  /** Submit form data to the server */
  Submit = 'SUBMIT',

  /** Trigger an action or event */
  Trigger = 'TRIGGER',
}

/**
 * Enhanced component interface for embedded flow components.
 *
 * @experimental This interface may change in future versions
 */
export interface EmbeddedFlowComponent {
  align?: string;
  alt?: string;
  color?: string;
  components?: EmbeddedFlowComponent[];
  config?: Record<string, unknown>;
  direction?: string;
  endIcon?: string;
  eventType?: EmbeddedFlowEventType | string;
  gap?: number;
  height?: string | number;
  id: string;
  items?: string | number;
  justify?: string;
  label?: string;
  name?: string;
  options?: (string | {label: string; value: string})[];
  placeholder?: string;
  ref?: string;
  required?: boolean;
  size?: number;
  source?: string;
  src?: string;
  startIcon?: string;
  type: EmbeddedFlowComponentType | string;
  variant?: EmbeddedFlowActionVariant | EmbeddedFlowTextVariant | string;
  width?: string | number;
}

/**
 * Response data structure for embedded flow API.
 *
 * @experimental This structure may change in future versions
 */
export interface EmbeddedFlowResponseData {
  actions?: {
    eventType?: string;
    nextNode?: string;
    ref: string;
  }[];
  additionalData?: Record<string, any>;
  inputs?: {
    identifier: string;
    ref: string;
    required: boolean;
    type: string;
  }[];
  meta?: {
    components: EmbeddedFlowComponent[];
  };
  redirectURL?: string;
}

export type ConsentPurposeType = 'attributes' | 'permissions';

export interface ConsentAttributeElement {
  approved: boolean;
  name: string;
}

export interface PromptElement {
  name: string;
  parent?: string;
}

export interface ConsentPurposeDecision {
  approved: boolean;
  elements: ConsentAttributeElement[];
  purposeName: string;
}

export interface ConsentDecisions {
  purposes: ConsentPurposeDecision[];
}

export interface ConsentPurposeData {
  description?: string;
  essential: PromptElement[];
  optional: PromptElement[];
  purposeId: string;
  purposeName?: string;
  type?: ConsentPurposeType;
}

export interface ConsentPromptData {
  purposes: ConsentPurposeData[];
}

/**
 * Request configuration for executing embedded flow operations.
 *
 * @template T - Type of the payload data being sent with the request
 */
export interface EmbeddedFlowExecuteRequestConfig<T = any> extends Partial<Request> {
  /**
   * Authentication ID used for OAuth2 flow completion.
   */
  authId?: string;

  /**
   * Base URL for the API endpoint.
   */
  baseUrl?: string;

  /**
   * Payload data to be sent with the request.
   */
  payload?: T;

  /**
   * Full URL for the API endpoint. Overrides baseUrl when provided.
   */
  url?: string;
}
