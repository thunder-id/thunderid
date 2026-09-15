// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The `userDeletionFlow` entry of the `flow` server-config section.
 *
 * There is no mode switch. Deletion runs through the flow when `defaultHandle` names an
 * administration flow that exists, and through `DELETE /users/{id}` otherwise, mirroring how user
 * onboarding falls back to manual creation when its flow is unavailable.
 */
export interface UserDeletionFlowConfig {
  defaultHandle?: string;
  expirySeconds?: number;
}

/**
 * The subset of the `flow` server-config section this package reads.
 */
export interface FlowSectionConfig {
  userDeletionFlow?: UserDeletionFlowConfig;
}

/**
 * `GET /server-config/{name}` returns the declarative and writable layers alongside the effective
 * value. Only the merged layer describes what the server actually applies.
 */
export interface ServerConfigLayers<T> {
  readOnly?: T;
  writable?: T;
  merged?: T;
}

/**
 * One entry of `GET /flows`, narrowed to the fields needed to resolve a handle to an id.
 */
export interface BasicFlowSummary {
  id: string;
  handle: string;
  flowType: string;
}

/**
 * The `GET /flows` response, narrowed.
 */
export interface FlowListResponse {
  flows?: BasicFlowSummary[];
}

/**
 * One localizable message of a flow error, as the engine serializes it.
 */
export interface FlowI18nMessage {
  key?: string;
  defaultValue?: string;
  params?: Record<string, string>;
}

/**
 * The error a failed step carries. A node that refuses reports its own executor error here, so this
 * is where a refusal's code and reason come from.
 */
export interface FlowExecutionError {
  code?: string;
  message?: FlowI18nMessage;
  description?: FlowI18nMessage;
}

/**
 * The `POST /flow/execute` response, narrowed to what the deletion path inspects.
 */
export interface FlowExecutionResponse {
  flowStatus?: string;
  executionId?: string;
  error?: FlowExecutionError;
}

/**
 * Terminal status of a completed flow execution.
 */
export const FlowStatus = {
  COMPLETE: 'COMPLETE',
  ERROR: 'ERROR',
  INCOMPLETE: 'INCOMPLETE',
} as const;

/**
 * Flow type of the administration flows that can carry out a deletion.
 */
export const ADMINISTRATION_FLOW_TYPE = 'ADMINISTRATION';

/**
 * Identifier of the input the deletion flow expects, matching the executor's declared input.
 */
export const DELETION_SUBJECT_INPUT = 'subject';

/**
 * Page size used when walking the flow listing to resolve a handle. Matches the server's maximum
 * page size, so the common case of a handful of administration flows resolves in one request.
 */
export const FLOW_PAGE_SIZE = 100;
