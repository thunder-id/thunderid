// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  ADMINISTRATION_FLOW_TYPE,
  DELETION_SUBJECT_INPUT,
  FLOW_PAGE_SIZE,
  FlowStatus,
  type FlowExecutionError,
  type FlowExecutionResponse,
  type FlowListResponse,
  type FlowSectionConfig,
  type ServerConfigLayers,
} from '../models/user-deletion';

/**
 * Minimal shape of the HTTP client this module needs, so it can be exercised without a React tree.
 */
export interface HttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

/**
 * Reads the handle of the configured user deletion flow, or an empty string when none is configured.
 *
 * Read failures propagate. Treating them as "no flow configured" would silently perform a deletion
 * that skips grant revocation and session termination, and report it as a success.
 */
export async function resolveDeletionFlowHandle(http: HttpLike, serverUrl: string): Promise<string> {
  const response = await http.request({
    url: `${serverUrl}/server-config/flow`,
    method: 'GET',
  });
  // The endpoint returns the declarative, writable and merged layers. Only the merged layer is the
  // effective configuration, so reading the envelope directly would always miss the value.
  const layers = (response?.data ?? {}) as ServerConfigLayers<FlowSectionConfig>;

  return layers.merged?.userDeletionFlow?.defaultHandle ?? '';
}

/**
 * Finds the identifier of the administration flow carrying the given handle, or null when no such
 * flow exists.
 *
 * `/flow/execute` addresses a flow by id while the server config names it by handle, so the handle
 * is looked up against the administration flows rather than assumed.
 *
 * The listing is paginated and ordered newest first, which puts the bootstrap deletion flow last, so
 * a single page would stop finding it once a deployment accumulates enough administration flows.
 * Pages are therefore walked until the handle is found or the listing is exhausted.
 */
export async function findAdministrationFlowId(
  http: HttpLike,
  serverUrl: string,
  handle: string,
): Promise<string | null> {
  for (let offset = 0; ; offset += FLOW_PAGE_SIZE) {
    const response = await http.request({
      url: `${serverUrl}/flows?flowType=${ADMINISTRATION_FLOW_TYPE}&limit=${FLOW_PAGE_SIZE}&offset=${offset}`,
      method: 'GET',
    });
    const flows = ((response?.data ?? {}) as FlowListResponse).flows ?? [];
    const match = flows.find((flow) => flow.handle === handle && flow.flowType === ADMINISTRATION_FLOW_TYPE);

    if (match) {
      return match.id;
    }
    // A short page is the last page, so the handle is genuinely absent rather than further along.
    if (flows.length < FLOW_PAGE_SIZE) {
      return null;
    }
  }
}

/**
 * Runs the deletion flow to completion in a single call.
 *
 * Every input the flow requires is supplied up front, so it should reach a terminal state without
 * pausing. An INCOMPLETE result therefore means the configured flow asks for something this caller
 * cannot provide, and is surfaced as an error rather than silently leaving the user undeleted.
 */
/**
 * An error carrying the code and params of a flow step that refused, so getUserErrorMessage can
 * resolve it to the localized message for that code.
 *
 * A refusal arrives with HTTP 200 and a failed step rather than as a transport error, so there is no
 * `response.data.code` for the shared mapper to read. Attaching the envelope here is what lets a
 * refusal reach the user instead of a generic failure.
 */
class FlowExecutionFailure extends Error {
  readonly code?: string;

  readonly error: FlowExecutionError;

  constructor(flowError: FlowExecutionError, fallbackMessage: string) {
    super(flowError.message?.defaultValue ?? flowError.description?.defaultValue ?? fallbackMessage);
    this.name = 'FlowExecutionFailure';
    this.code = flowError.code;
    this.error = flowError;
  }
}

export async function executeDeletionFlow(
  http: HttpLike,
  serverUrl: string,
  flowId: string,
  userId: string,
): Promise<void> {
  const response = await http.request({
    url: `${serverUrl}/flow/execute`,
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    data: {
      flowId,
      inputs: {[DELETION_SUBJECT_INPUT]: userId},
    },
  });
  const result = (response?.data ?? {}) as FlowExecutionResponse;

  if (result.flowStatus === FlowStatus.COMPLETE) {
    return;
  }

  if (result.flowStatus === FlowStatus.INCOMPLETE) {
    throw new Error('The user deletion flow requires additional input and could not be completed');
  }

  throw new FlowExecutionFailure(result.error ?? {}, 'The user deletion flow did not complete');
}

/**
 * Deletes a user through the native endpoint.
 */
export async function deleteUserNatively(http: HttpLike, serverUrl: string, userId: string): Promise<void> {
  await http.request({
    url: `${serverUrl}/users/${userId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}

/**
 * Deletes a user through the configured deletion flow, falling back to the native endpoint.
 *
 * The presence of a usable flow is the switch: deletion runs through the flow when one is configured
 * and exists, and through the endpoint otherwise. This mirrors user onboarding, which falls back to
 * manual creation when its flow is unavailable, and means a deployment carrying no flow configuration
 * keeps deleting users.
 *
 * Only a missing flow triggers the fallback. Failures from the flow itself propagate, so a flow that
 * exists but refuses or errors is never quietly downgraded to a deletion that skips revocation.
 */
export default async function deleteUser(http: HttpLike, serverUrl: string, userId: string): Promise<void> {
  const handle = await resolveDeletionFlowHandle(http, serverUrl);

  if (handle) {
    const flowId = await findAdministrationFlowId(http, serverUrl, handle);

    if (flowId) {
      await executeDeletionFlow(http, serverUrl, flowId, userId);
      return;
    }
  }

  await deleteUserNatively(http, serverUrl, userId);
}
