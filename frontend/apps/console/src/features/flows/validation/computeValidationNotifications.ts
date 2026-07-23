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
import type {TFunction} from 'i18next';
import get from 'lodash-es/get';
import {createElement, type ReactElement} from 'react';
import {Trans} from 'react-i18next';
import type {GraphValidationRule, RequiredFieldRule, ValidationRuleDefinition} from './validation-rules';
import ValidationConstants from '../constants/ValidationConstants';
import type {Element} from '../models/elements';
import Notification, {NotificationType} from '../models/notification';
import type {Resource} from '../models/resources';
import type {StepData} from '../models/steps';

/**
 * Unresolved placeholders that should be treated as missing values.
 */
const UNRESOLVED_PLACEHOLDERS: ReadonlySet<string> = new Set(['{{IDP_NAME}}', '{{IDP_ID}}', '{{SENDER_ID}}']);

/**
 * Returns true if the value is a non-empty, resolved (non-placeholder) value.
 */
function isResolvedValue(value: unknown): boolean {
  if (!value) {
    return false;
  }

  if (typeof value === 'string' && UNRESOLVED_PLACEHOLDERS.has(value)) {
    return false;
  }

  return true;
}

/**
 * Checks whether a resource has a non-empty, resolved value for a given field path.
 *
 * Check order:
 * 1. resource.config[name]
 * 2. resource[name]
 * 3. Nested path via lodash `get()` (for dotted field names like 'data.properties.idpId').
 */
function hasFieldValue(resource: Resource, fieldName: string): boolean {
  if (isResolvedValue(resource?.config?.[fieldName as keyof typeof resource.config])) {
    return true;
  }

  if (isResolvedValue(resource?.[fieldName as keyof Resource])) {
    return true;
  }

  if (fieldName.includes('.') && isResolvedValue(get(resource, fieldName, null))) {
    return true;
  }

  return false;
}

/**
 * Apply matching validation rules to a single resource.
 */
function applyRules(
  resource: Resource,
  rules: ValidationRuleDefinition[],
  t: TFunction,
  notifications: Map<string, Notification>,
): void {
  for (const rule of rules) {
    if (!rule.match(resource)) {
      continue;
    }

    // Structural / custom validators
    if (rule.customValidator) {
      const notification = rule.customValidator(resource);

      if (notification) {
        notifications.set(notification.getId(), notification);
      }

      continue;
    }

    // Required-field validation
    const missingFields: RequiredFieldRule[] = rule.fields.filter((field) => !hasFieldValue(resource, field.name));

    if (missingFields.length > 0) {
      const errorId = `${resource.id}_${ValidationConstants.REQUIRED_FIELD_ERROR_CODE}`;
      const message: ReactElement = createElement(Trans, {
        i18nKey: rule.generalMessageKey,
        values: {id: resource.id},
        components: {code: createElement('code')},
      });
      const notification = new Notification(errorId, message, NotificationType.ERROR);

      notification.addResource(resource);

      for (const field of missingFields) {
        notification.addResourceFieldNotification(`${resource.id}_${field.name}`, t(field.errorMessageKey));
      }

      notifications.set(errorId, notification);
    }
  }
}

/**
 * Recursively walk an element tree and apply validation rules.
 */
function walkElements(
  elements: Element[],
  rules: ValidationRuleDefinition[],
  t: TFunction,
  notifications: Map<string, Notification>,
): void {
  for (const element of elements) {
    applyRules(element as unknown as Resource, rules, t, notifications);

    if (element.components) {
      walkElements(element.components, rules, t, notifications);
    }
  }
}

/**
 * Pure function that computes all validation notifications from the current
 * set of React Flow nodes and a static rule registry.
 *
 * This replaces the former `useRequiredFields` hook and all per-component
 * validation `useEffect` calls. Notifications are derived data — they are
 * recomputed synchronously on every render where node data changes, with
 * no effect delay.
 *
 * @param nodes - All React Flow nodes in the current flow.
 * @param rules - The validation rule registry.
 * @param t     - i18next translate function.
 * @param graphRules - Cross-node rules applied against the whole node set.
 *                     Empty by default; the host registers the rules that
 *                     apply to the current flow type.
 * @returns A map of notification ID → Notification.
 */
export function computeValidationNotifications(
  nodes: Node[],
  rules: ValidationRuleDefinition[],
  t: TFunction,
  graphRules: GraphValidationRule[] = [],
): Map<string, Notification> {
  const notifications = new Map<string, Notification>();

  for (const node of nodes) {
    const stepData = node.data as StepData | undefined;

    // Apply execution-level rules against the node itself (it's a Step resource).
    applyRules(node as unknown as Resource, rules, t, notifications);

    // Walk the element tree within each step.
    if (stepData?.components) {
      walkElements(stepData.components, rules, t, notifications);
    }
  }

  for (const graphRule of graphRules) {
    for (const notification of graphRule(nodes)) {
      notifications.set(notification.getId(), notification);
    }
  }

  return notifications;
}
