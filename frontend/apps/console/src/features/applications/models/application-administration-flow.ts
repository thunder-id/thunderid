// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * One entry of the `flow` server-config section, naming the administration flow for an action.
 *
 * There is no mode switch. An action runs through its flow when `defaultHandle` names an
 * administration flow that exists, and through the native application endpoint otherwise, mirroring
 * how user deletion falls back to `DELETE /users/{id}`.
 */
export interface AdministrationFlowConfig {
  defaultHandle?: string;
  expirySeconds?: number;
}

/**
 * The subset of the `flow` server-config section the application administration paths read.
 */
export interface FlowSectionConfig {
  applicationDeletionFlow?: AdministrationFlowConfig;
  clientSecretRegenerationFlow?: AdministrationFlowConfig;
}

/**
 * Names of the `flow` server-config entries an application action can be driven by.
 */
export type ApplicationFlowConfigKey = keyof FlowSectionConfig;

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
 * The `POST /flow/execute` response, narrowed to what the application paths inspect.
 *
 * `data.additionalData` carries values a flow produces. The regeneration flow returns the new client
 * secret there, which is the only moment it is readable.
 */
export interface FlowExecutionResponse {
  flowStatus?: string;
  executionId?: string;
  error?: FlowExecutionError;
  data?: {
    additionalData?: Record<string, string>;
  };
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
 * Flow type of the administration flows that can carry out an application action.
 */
export const ADMINISTRATION_FLOW_TYPE = 'ADMINISTRATION';

/**
 * Identifier of the input the application administration flows expect, matching the executor's
 * declared input. It is not `applicationId`, which the execution request already uses at its top
 * level for the application initiating the flow.
 */
export const APPLICATION_TARGET_INPUT = 'targetApplicationId';

/**
 * Key the regeneration flow returns the new client secret under.
 */
export const CLIENT_SECRET_DATA_KEY = 'clientSecret';

/**
 * Page size used when walking the flow listing to resolve a handle. Matches the server's maximum
 * page size, so the common case of a handful of administration flows resolves in one request.
 */
export const FLOW_PAGE_SIZE = 100;
