/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {createElement} from 'react';
import {Trans} from 'react-i18next';
import {ActionEventTypes, BlockTypes, ElementCategories, ElementTypes} from '../models/elements';
import type {Element as FlowElement} from '../models/elements';
import Notification, {NotificationType} from '../models/notification';
import type {Resource} from '../models/resources';
import {ExecutionTypes} from '../models/steps';
import type {StepData} from '../models/steps';

/**
 * A single field that must have a non-empty value on a resource.
 */
export interface RequiredFieldRule {
  /** Field path — top-level key, config key, or lodash-style nested path (e.g. 'data.properties.idpId'). */
  name: string;
  /** i18n key for the per-field error message. */
  errorMessageKey: string;
}

/**
 * Declarative validation rule definition.
 */
export interface ValidationRuleDefinition {
  /** Determines whether this rule applies to a given resource. */
  match: (resource: Resource) => boolean;
  /** Required fields to validate when the rule matches. Empty for structural-only rules. */
  fields: RequiredFieldRule[];
  /** i18n key for the general error message. */
  generalMessageKey: string;
  /**
   * Optional structural validator for checks beyond field presence (e.g. "form has inputs but no submit button").
   * Return a Notification if invalid, or null if valid.
   */
  customValidator?: (resource: Resource) => Notification | null;
}

// ---------------------------------------------------------------------------
// Element Validation Rules
// ---------------------------------------------------------------------------

const DEFAULT_INPUT_TYPES: readonly string[] = [
  ElementTypes.TextInput,
  ElementTypes.PasswordInput,
  ElementTypes.EmailInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
];

export const VALIDATION_RULES: ValidationRuleDefinition[] = [
  // Default inputs (text, password, email, number, date)
  {
    match: (r) => DEFAULT_INPUT_TYPES.includes(r.type),
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.input.label'},
      {name: 'ref', errorMessageKey: 'flows:core.validation.fields.input.ref'},
    ],
    generalMessageKey: 'flows:core.validation.fields.input.general',
  },
  // Select
  {
    match: (r) => r.type === ElementTypes.Select,
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.input.label'},
      {name: 'ref', errorMessageKey: 'flows:core.validation.fields.input.ref'},
    ],
    generalMessageKey: 'flows:core.validation.fields.input.general',
  },
  // Checkbox
  {
    match: (r) => r.type === ElementTypes.Checkbox,
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.checkbox.label'},
      {name: 'ref', errorMessageKey: 'flows:core.validation.fields.checkbox.ref'},
    ],
    generalMessageKey: 'flows:core.validation.fields.checkbox.general',
  },
  // Phone number input
  {
    match: (r) => r.type === ElementTypes.PhoneInput,
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.phoneNumberInput.label'},
      {name: 'ref', errorMessageKey: 'flows:core.validation.fields.phoneNumberInput.ref'},
    ],
    generalMessageKey: 'flows:core.validation.fields.phoneNumberInput.general',
  },
  // OTP input
  {
    match: (r) => r.type === ElementTypes.OtpInput,
    fields: [{name: 'label', errorMessageKey: 'flows:core.validation.fields.otpInput.label'}],
    generalMessageKey: 'flows:core.validation.fields.otpInput.general',
  },
  // Button (Action element)
  {
    match: (r) => r.type === ElementTypes.Action,
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.button.label'},
      {name: 'variant', errorMessageKey: 'flows:core.validation.fields.button.variant'},
    ],
    generalMessageKey: 'flows:core.validation.fields.button.general',
  },
  // Resend button
  {
    match: (r) => r.type === ElementTypes.Resend,
    fields: [{name: 'label', errorMessageKey: 'flows:core.validation.fields.button.label'}],
    generalMessageKey: 'flows:core.validation.fields.button.general',
  },
  // Typography (Text element)
  {
    match: (r) => r.type === ElementTypes.Text,
    fields: [
      {name: 'label', errorMessageKey: 'flows:core.validation.fields.typography.label'},
      {name: 'variant', errorMessageKey: 'flows:core.validation.fields.typography.variant'},
    ],
    generalMessageKey: 'flows:core.validation.fields.typography.general',
  },
  // Rich text
  {
    match: (r) => r.type === ElementTypes.RichText,
    fields: [{name: 'label', errorMessageKey: 'flows:core.validation.fields.richText.label'}],
    generalMessageKey: 'flows:core.validation.fields.richText.general',
  },
  // Divider
  {
    match: (r) => r.type === ElementTypes.Divider,
    fields: [{name: 'variant', errorMessageKey: 'flows:core.validation.fields.divider.variant'}],
    generalMessageKey: 'flows:core.validation.fields.divider.general',
  },
  // Image
  {
    match: (r) => r.type === ElementTypes.Image,
    fields: [{name: 'src', errorMessageKey: 'flows:core.validation.fields.image.src'}],
    generalMessageKey: 'flows:core.validation.fields.image.general',
  },

  // ---------------------------------------------------------------------------
  // Execution Validation Rules
  // ---------------------------------------------------------------------------

  // Google federation executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.GoogleFederation,
    fields: [{name: 'data.properties.idpId', errorMessageKey: 'flows:core.validation.fields.input.idpId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // GitHub federation executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.GithubFederation,
    fields: [{name: 'data.properties.idpId', errorMessageKey: 'flows:core.validation.fields.input.idpId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // Generic OAuth executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.OAuthExecutor,
    fields: [{name: 'data.properties.idpId', errorMessageKey: 'flows:core.validation.fields.input.idpId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // Generic OIDC executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.OIDCAuthExecutor,
    fields: [{name: 'data.properties.idpId', errorMessageKey: 'flows:core.validation.fields.input.idpId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // SMS OTP executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.SMSOTPAuth,
    fields: [{name: 'data.properties.senderId', errorMessageKey: 'flows:core.validation.fields.input.senderId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // SMS executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.SMSExecutor,
    fields: [{name: 'data.properties.senderId', errorMessageKey: 'flows:core.validation.fields.input.senderId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },

  // ---------------------------------------------------------------------------
  // Structural Validation Rules
  // ---------------------------------------------------------------------------

  // Form: must have a submit button when it contains input fields
  {
    match: (r) => r.type === BlockTypes.Form && r.category === ElementCategories.Block,
    fields: [],
    generalMessageKey: '',
    customValidator: (resource: Resource): Notification | null => {
      const formElement = resource as FlowElement;

      const hasInputFields = formElement.components?.some(
        (element: FlowElement) =>
          element.category === ElementCategories.Field || element.type === ElementTypes.DynamicInputPlaceholder,
      );

      const hasSubmitButton = formElement.components?.some(
        (element: FlowElement) =>
          element.type === ElementTypes.Action &&
          (element as FlowElement & {eventType?: string}).eventType === ActionEventTypes.Submit,
      );

      if (hasInputFields && !hasSubmitButton) {
        const errorId = `${resource.id}_FORM_NO_SUBMIT_BUTTON`;
        const message = createElement(Trans, {
          i18nKey: 'flows:core.validation.fields.form.noSubmitButton',
          values: {id: resource.id},
          components: {code: createElement('code')},
        });
        const notification = new Notification(errorId, message, NotificationType.ERROR);

        notification.addResource(resource);

        return notification;
      }

      return null;
    },
  },
];
