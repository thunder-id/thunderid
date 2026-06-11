/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {EmbeddedFlowExecuteRequestConfig as EmbeddedFlowExecuteRequestConfigV1} from '../embedded-flow';

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
 * This interface provides better support for modern form handling and user experience.
 * It includes properties for labels, placeholders, and required field validation
 * that are directly provided by the API response.
 *
 * @example
 * ```typescript
 * const component: EmbeddedFlowComponent = {
 *   id: 'username_field',
 *   type: EmbeddedFlowComponentType.TextInput,
 *   label: 'Username',
 *   placeholder: 'Enter your username',
 *   required: true,
 *   variant: 'TEXT',
 *   eventType: 'SUBMIT',
 *   components: []
 * };
 * ```
 *
 * @experimental This interface may change in future versions
 */
export interface EmbeddedFlowComponent {
  /**
   * Alignment of children along the cross axis (for Stack components).
   */
  align?: string;

  /**
   * Alternative text for Image components.
   */
  alt?: string;

  /**
   * Icon color, CSS color value (for Icon components).
   */
  color?: string;

  /**
   * Nested child components for container components like Block and Stack.
   */
  components?: EmbeddedFlowComponent[];

  /**
   * Layout direction for Stack components ('row' | 'column').
   */
  direction?: string;

  /**
   * Icon to render at the end of an Action button (URL string).
   */
  endIcon?: string;

  /**
   * Event type for action components that defines the interaction behavior.
   * Only relevant for Action components.
   */
  eventType?: EmbeddedFlowEventType | string;

  /**
   * Gap between children in Stack components (number, maps to spacing units).
   */
  gap?: number;

  /**
   * Height of the component (for Image components, can be string with units or number for pixels).
   * The value depends on the component type (e.g., for Image components).
   */
  height?: string | number;

  /**
   * Unique identifier for the component
   */
  id: string;

  /**
   * Number of items across the main axis (for Stack grid-like layouts).
   */
  items?: string | number;

  /**
   * Justification of children along the main axis (for Stack components).
   */
  justify?: string;

  /**
   * Display label for the component (e.g., field label, button text).
   * Supports internationalization and may contain template strings.
   */
  label?: string;

  /**
   * Icon name for Icon components (e.g., lucide-react icon names like 'ArrowLeftRight').
   */
  name?: string;

  /**
   * Options for SELECT components.
   * Each option can be a string value or an object with value and label.
   */
  options?: (string | {label: string; value: string})[];

  /**
   * Placeholder text for input components.
   * Provides helpful hints to users about expected input format.
   */
  placeholder?: string;

  /**
   * Reference identifier for the component (e.g., field name, action ref)
   */
  ref?: string;

  /**
   * Indicates whether this component represents a required field.
   * Used for form validation and UI indicators.
   */
  required?: boolean;

  /**
   * Icon size in pixels (for Icon components).
   */
  size?: number;

  /**
   * Data source key for dynamic components (e.g., COPYABLE_TEXT).
   * References a key in additionalData whose value is resolved at render time.
   */
  source?: string;

  /**
   * Image source URL (for Image components).
   */
  src?: string;

  /**
   * Icon to render at the start of an Action button (URL string).
   */
  startIcon?: string;

  /**
   * Component type that determines rendering behavior
   */
  type: EmbeddedFlowComponentType | string;

  /**
   * Component variant that affects visual styling and behavior.
   * The value depends on the component type (e.g., button variants, text variants).
   */
  variant?: EmbeddedFlowActionVariant | EmbeddedFlowTextVariant | string;

  /**
   * Width of the component (for Image components, can be string with units or number for pixels).
   * The value depends on the component type (e.g., for Image components).
   */
  width?: string | number;
}

/**
 * Response data structure for embedded flow API.
 *
 * This interface defines the structure of data returned by the API,
 * which includes both legacy input/action arrays for backward compatibility
 * and the new meta.components structure for modern component-driven UIs.
 *
 * The key improvement is the meta.components field, which provides
 * a rich component tree with proper labels, placeholders, and hierarchy
 * that can be directly rendered without additional transformation.
 *
 * @example
 * ```typescript
 * const response: EmbeddedFlowResponseData = {
 *   // Legacy format (for backward compatibility)
 *   inputs: [
 *     { ref: 'input_001', identifier: 'username', type: 'TEXT_INPUT', required: true }
 *   ],
 *   actions: [
 *     { ref: 'action_001', nextNode: 'basic_auth', eventType: 'SUBMIT' }
 *   ],
 *   // Modern format (recommended)
 *   meta: {
 *     components: [
 *       {
 *         id: 'text_001',
 *         type: 'TEXT',
 *         label: '{{ t(signin:heading.label) }}',
 *         variant: 'HEADING_1'
 *       },
 *       {
 *         id: 'block_001',
 *         type: 'BLOCK',
 *         components: [
 *           {
 *             id: 'input_001',
 *             type: 'TEXT_INPUT',
 *             label: '{{ t(signin:fields.username.label) }}',
 *             placeholder: '{{ t(signin:fields.username.placeholder) }}',
 *             required: true
 *           },
 *           {
 *             id: 'action_001',
 *             type: 'ACTION',
 *             label: '{{ t(signin:buttons.submit.label) }}',
 *             variant: 'PRIMARY',
 *             eventType: 'ACTIVATE'
 *           }
 *         ]
 *       }
 *     ]
 *   }
 * };
 * ```
 *
 * @experimental This structure may change in future versions
 */
export interface EmbeddedFlowResponseData {
  /**
   * Legacy action definitions for backward compatibility.
   * @deprecated Use meta.components for new implementations
   */
  actions?: {
    /** Event type for the action (SUBMIT, ACTIVATE, etc.) */
    eventType?: string;
    /** Next flow node to navigate to (optional) */
    nextNode?: string;
    /** Reference identifier for the action */
    ref: string;
  }[];

  /**
   * Additional data dictionary for dynamic flow response properties.
   * Can be used to pass custom data like passkey challenges, server alerts, etc.
   */
  additionalData?: Record<string, any>;

  /**
   * Legacy input definitions for backward compatibility.
   * @deprecated Use meta.components for new implementations
   */
  inputs?: {
    /** Field identifier used in form submission */
    identifier: string;
    /** Reference identifier for the input */
    ref: string;
    /** Whether this input is required for form submission */
    required: boolean;
    /** Input type (TEXT_INPUT, PASSWORD_INPUT, etc.) */
    type: string;
  }[];

  /**
   * Modern component-driven metadata structure.
   * This contains the complete UI component tree with proper
   * hierarchy, labels, and configuration that can be directly rendered.
   *
   * **This is the primary data source for implementations.**
   * The legacy inputs/actions arrays are maintained only for backward compatibility.
   */
  meta?: {
    /** Array of components that define the complete UI structure */
    components: EmbeddedFlowComponent[];
  };

  /**
   * Optional redirect URL for flow completion or external authentication.
   */
  redirectURL?: string;
}

/**
 * Discriminator identifying the kind of consent a purpose represents. The same
 * `ConsentPurposeData` envelope is used for both attribute and permission consent;
 * the populated fields differ based on this discriminator.
 *
 * @experimental This type may change in future versions
 */
export type ConsentPurposeType = 'attributes' | 'permissions';

/**
 * Individual consent attribute/element decision.
 *
 * @experimental This interface may change in future versions
 */
export interface ConsentAttributeElement {
  /** Whether the user approved collection of this attribute */
  approved: boolean;
  /** The name of the attribute being consented */
  name: string;
}

/**
 * A single element presented for consent within a consent purpose. For attribute purposes
 * the element is an attribute name. For permission purposes the element is a permission
 * string and `parent` may carry rollup linkage supplied by the server: when set, the UI
 * may render this permission as a child of `parent` and offer a single rollup toggle.
 *
 * @experimental This interface may change in future versions
 */
export interface PromptElement {
  /** Canonical element name (attribute name or permission string) */
  name: string;
  /**
   * Canonical name of the rollup parent, permission-purpose only. Undefined for attribute
   * elements and for top-level permissions.
   */
  parent?: string;
}

/**
 * Consent decision for a single purpose.
 *
 * @experimental This interface may change in future versions
 */
export interface ConsentPurposeDecision {
  /** Whether the user approved this purpose */
  approved: boolean;
  /** Per-attribute decisions for this purpose */
  elements: ConsentAttributeElement[];
  /** The name of the consent purpose */
  purposeName: string;
}

/**
 * Full consent decisions structure sent to the backend when user submits the consent form.
 *
 * @experimental This interface may change in future versions
 */
export interface ConsentDecisions {
  /** Array of per-purpose decisions */
  purposes: ConsentPurposeDecision[];
}

/**
 * Single consent purpose data returned by the backend in additionalData.consent_prompt.
 * The same envelope carries both attribute and permission purposes, distinguished by `type`.
 *
 * @experimental This interface may change in future versions
 */
export interface ConsentPurposeData {
  /** Optional human-readable description of the purpose */
  description?: string;
  /**
   * Elements that are mandatory and cannot be declined. Used by attribute purposes;
   * permission purposes today have no essential elements.
   */
  essential: PromptElement[];
  /**
   * Elements the user can opt in or out of. For attribute purposes these are optional
   * attribute names. For permission purposes these are permission elements (which may
   * carry rollup parent linkage).
   */
  optional: PromptElement[];
  /** Unique identifier for the purpose */
  purposeId: string;
  /** Human-readable purpose name */
  purposeName?: string;
  /**
   * Discriminator selecting between attribute and permission consent semantics.
   */
  type?: ConsentPurposeType;
}

/**
 * Consent prompt data structure stored in additionalData.consent_prompt.
 *
 * @experimental This interface may change in future versions
 */
export interface ConsentPromptData {
  /** Array of consent purposes requiring user review */
  purposes: ConsentPurposeData[];
}

/**
 * Extended request configuration for ThunderID V2 embedded flow operations.
 *
 * This interface extends the base request configuration with V2-specific
 * properties required for the enhanced embedded flow API. The authId parameter
 * is particularly important for the V2 OAuth2 flow completion process.
 *
 * @template T The type of the payload data being sent with the request
 *
 * @example
 * ```typescript
 * const config: EmbeddedFlowExecuteRequestConfigV2 = {
 *   baseUrl: 'https://localhost:8090',
 *   payload: {
 *     flowType: 'AUTHENTICATION',
 *     inputs: { username: 'user@example.com' }
 *   },
 *   authId: 'auth_12345', // V2-specific for OAuth completion
 *   headers: {
 *     'Authorization': 'Bearer token'
 *   }
 * };
 * ```
 *
 * @experimental This configuration is part of the new ThunderID V2 platform
 */
export interface EmbeddedFlowExecuteRequestConfig<T = any> extends EmbeddedFlowExecuteRequestConfigV1<T> {
  /**
   * Authentication ID used for OAuth2 flow completion in V2 API.
   *
   * When the embedded flow completes successfully and returns an assertion,
   * this authId is used to complete the OAuth2 authorization flow by calling
   * the `/oauth2/auth/callback` endpoint. This enables seamless transition from
   * embedded flow to traditional OAuth2 flow completion.
   *
   * @example "auth_abc123def456"
   */
  authId?: string;
}
