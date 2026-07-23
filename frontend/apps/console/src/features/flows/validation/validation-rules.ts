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

import type {Node} from '@xyflow/react';
import {createElement} from 'react';
import {Trans} from 'react-i18next';
import {ActionEventTypes, BlockTypes, ElementCategories, ElementTypes} from '../models/elements';
import type {Element as FlowElement} from '../models/elements';
import Notification, {NotificationType} from '../models/notification';
import type {Resource} from '../models/resources';
import {ExecutionTypes, StepTypes} from '../models/steps';
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
  // SMS executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.SMSExecutor,
    fields: [{name: 'data.properties.senderId', errorMessageKey: 'flows:core.validation.fields.input.senderId'}],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },
  // OTP executor
  {
    match: (r) => (r as {data?: StepData}).data?.action?.executor?.name === ExecutionTypes.OTPExecutor,
    fields: [],
    generalMessageKey: 'flows:core.validation.fields.executor.general',
  },

  // ---------------------------------------------------------------------------
  // Node Validation Rules
  // ---------------------------------------------------------------------------

  // CALL: referenced flow must be set
  {
    match: (r) => (r as {type?: string}).type === StepTypes.Call,
    fields: [{name: 'data.flow.ref', errorMessageKey: 'flows:core.validation.fields.call.flowRef'}],
    generalMessageKey: 'flows:core.validation.fields.call.general',
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

// ---------------------------------------------------------------------------
// Graph Validation Rules (cross-node)
// ---------------------------------------------------------------------------

/**
 * A validation rule that inspects the whole node set instead of a single
 * resource, for constraints that span multiple nodes.
 */
export type GraphValidationRule = (nodes: Node[]) => Notification[];

function getNodeExecutorName(node: Node): string | undefined {
  return (node.data as StepData | undefined)?.action?.executor?.name;
}

function createGraphErrorNotification(errorId: string, i18nKey: string, node: Node): Notification {
  const message = createElement(Trans, {
    i18nKey,
    values: {id: node.id},
    components: {code: createElement('code')},
  });
  const notification = new Notification(errorId, message, NotificationType.ERROR);

  notification.addResource(node as unknown as Resource);

  return notification;
}

/**
 * Mirrors the backend SSO pairing contract: every SSO check must reference an
 * existing session node via `checkpointRef`, and every session node must be
 * referenced by at least one SSO check. Surfaces hand-deleting half of a pair
 * immediately instead of at save time.
 */
export const ssoPairingRule: GraphValidationRule = (nodes: Node[]): Notification[] => {
  const notifications: Notification[] = [];

  const ssoCheckNodes = nodes.filter((node) => getNodeExecutorName(node) === ExecutionTypes.SSOCheck);
  const sessionNodeIds = new Set(
    nodes.filter((node) => getNodeExecutorName(node) === ExecutionTypes.Session).map((node) => node.id),
  );

  const referencedSessionIds = new Set<string>();

  for (const node of ssoCheckNodes) {
    const checkpointRef = (node.data as StepData | undefined)?.properties?.checkpointRef;

    if (typeof checkpointRef !== 'string' || checkpointRef === '') {
      notifications.push(
        createGraphErrorNotification(
          `${node.id}_SSO_MISSING_CHECKPOINT_REF`,
          'flows:core.validation.sso.missingCheckpointRef',
          node,
        ),
      );
      continue;
    }

    if (!sessionNodeIds.has(checkpointRef)) {
      notifications.push(
        createGraphErrorNotification(
          `${node.id}_SSO_INVALID_CHECKPOINT_REF`,
          'flows:core.validation.sso.invalidCheckpointRef',
          node,
        ),
      );
      continue;
    }

    referencedSessionIds.add(checkpointRef);
  }

  for (const node of nodes) {
    if (getNodeExecutorName(node) === ExecutionTypes.Session && !referencedSessionIds.has(node.id)) {
      notifications.push(
        createGraphErrorNotification(`${node.id}_SSO_ORPHAN_SESSION`, 'flows:core.validation.sso.orphanSession', node),
      );
    }
  }

  return notifications;
};

export const GRAPH_VALIDATION_RULES: GraphValidationRule[] = [ssoPairingRule];
