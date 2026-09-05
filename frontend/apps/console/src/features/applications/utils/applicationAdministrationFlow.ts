// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  ADMINISTRATION_FLOW_TYPE,
  APPLICATION_TARGET_INPUT,
  CLIENT_SECRET_DATA_KEY,
  FLOW_PAGE_SIZE,
  FlowStatus,
  type ApplicationFlowConfigKey,
  type FlowExecutionError,
  type FlowExecutionResponse,
  type FlowListResponse,
  type FlowSectionConfig,
  type ServerConfigLayers,
} from '../models/application-administration-flow';

/**
 * Minimal shape of the HTTP client this module needs, so it can be exercised without a React tree.
 */
export interface HttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

/**
 * Reads the handle of the administration flow configured for an application action, or an empty
 * string when none is configured.
 *
 * Read failures propagate. Treating them as "no flow configured" would silently perform the action
 * through the native endpoint, which revokes nothing, and report it as a success.
 */
export async function resolveApplicationFlowHandle(
  http: HttpLike,
  serverUrl: string,
  configKey: ApplicationFlowConfigKey,
): Promise<string> {
  const response = await http.request({
    url: `${serverUrl}/server-config/flow`,
    method: 'GET',
  });
  // The endpoint returns the declarative, writable and merged layers. Only the merged layer is the
  // effective configuration, so reading the envelope directly would always miss the value.
  const layers = (response?.data ?? {}) as ServerConfigLayers<FlowSectionConfig>;

  return layers.merged?.[configKey]?.defaultHandle ?? '';
}

/**
 * Finds the identifier of the administration flow carrying the given handle, or null when no such
 * flow exists.
 *
 * `/flow/execute` addresses a flow by id while the server config names it by handle, so the handle
 * is looked up against the administration flows rather than assumed.
 *
 * The listing is paginated and ordered newest first, which puts the bootstrap flows last, so a
 * single page would stop finding them once a deployment accumulates enough administration flows.
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
 * An error carrying the code and params of a flow step that refused, so the feature's error mapper can
 * resolve it to the localized message for that code.
 *
 * A refusal arrives with HTTP 200 and a failed step rather than as a transport error, so there is no
 * `response.data.code` for the shared mapper to read. Attaching the envelope here is what lets a
 * refusal such as "the application has no client secret" reach the user instead of a generic failure.
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

/**
 * Runs an application administration flow to completion in a single call and returns the values it
 * produced.
 *
 * Every input the flow requires is supplied up front, so it should reach a terminal state without
 * pausing. An INCOMPLETE result therefore means the configured flow asks for something this caller
 * cannot provide, and is surfaced as an error rather than reported as a success that did nothing.
 */
export async function executeApplicationFlow(
  http: HttpLike,
  serverUrl: string,
  flowId: string,
  applicationId: string,
  action: string,
): Promise<Record<string, string>> {
  const response = await http.request({
    url: `${serverUrl}/flow/execute`,
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    data: {
      flowId,
      inputs: {[APPLICATION_TARGET_INPUT]: applicationId},
    },
  });
  const result = (response?.data ?? {}) as FlowExecutionResponse;

  if (result.flowStatus === FlowStatus.COMPLETE) {
    return result.data?.additionalData ?? {};
  }

  if (result.flowStatus === FlowStatus.INCOMPLETE) {
    throw new Error(`The ${action} flow requires additional input and could not be completed`);
  }

  throw new FlowExecutionFailure(result.error ?? {}, `The ${action} flow did not complete`);
}

/**
 * Resolves the flow configured for an action and runs it, or returns null when no flow is configured
 * so the caller can fall back to the native endpoint.
 *
 * Only an unset handle yields null. That is the documented opt-out: the deployment has asked for the
 * native behaviour, which revokes nothing by design.
 *
 * A handle that is set but resolves to no flow is the opposite case — the deployment asked for
 * flow-based revocation and the flow it names is missing, so nothing here can carry that intent out.
 * Deleting a flow neither clears the configuration nor checks who references it, so a handle goes
 * stale silently. Falling back would perform the delete or the rotation with no revocation at all and
 * report it as a success, which is the one outcome the configuration was set to prevent.
 */
async function runConfiguredFlow(
  http: HttpLike,
  serverUrl: string,
  configKey: ApplicationFlowConfigKey,
  applicationId: string,
  action: string,
): Promise<Record<string, string> | null> {
  const handle = await resolveApplicationFlowHandle(http, serverUrl, configKey);

  if (!handle) {
    return null;
  }
  const flowId = await findAdministrationFlowId(http, serverUrl, handle);

  if (!flowId) {
    throw new Error(
      `The configured ${action} flow "${handle}" no longer exists, so the action was not performed. ` +
        'Restore the flow or clear the handle from the server configuration.',
    );
  }

  return executeApplicationFlow(http, serverUrl, flowId, applicationId, action);
}

/**
 * Deletes an application through the native endpoint.
 */
export async function deleteApplicationNatively(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/applications/${applicationId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}

/**
 * Deletes an application through the configured deletion flow, falling back to the native endpoint.
 *
 * The presence of flow configuration is the switch, so a deployment carrying none keeps deleting
 * applications. Going through the flow is what revokes the application's tokens and detaches its
 * sessions before the record is removed; the native endpoint does neither. A configured flow that
 * cannot be resolved refuses rather than falling back, since that is a broken opt-in, not an opt-out.
 */
export async function deleteApplicationViaFlow(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<void> {
  const result = await runConfiguredFlow(
    http,
    serverUrl,
    'applicationDeletionFlow',
    applicationId,
    'application deletion',
  );

  if (result === null) {
    await deleteApplicationNatively(http, serverUrl, applicationId);
  }
}

/**
 * Regenerates an application's client secret through the configured regeneration flow, returning the
 * new secret, or null when no flow is configured.
 *
 * The secret is generated by the server rather than the browser, and the flow's response is the only
 * moment it is readable: no read path returns it afterwards.
 */
export async function regenerateClientSecretViaFlow(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<string | null> {
  const result = await runConfiguredFlow(
    http,
    serverUrl,
    'clientSecretRegenerationFlow',
    applicationId,
    'client secret regeneration',
  );

  if (result === null) {
    return null;
  }

  const secret = result[CLIENT_SECRET_DATA_KEY];

  if (!secret) {
    throw new Error('The client secret regeneration flow completed without returning a secret');
  }

  return secret;
}
