// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Runs an administration flow in place of a native management call, so the change revokes the tokens
 * it invalidates before it is applied.
 *
 * This generalizes the pattern the application deletion and client secret regeneration paths
 * established. Those act on one application and take one input; the authorization changes act on
 * roles, groups and the resource catalog and take between one and three, so the inputs are supplied
 * by the caller rather than built here.
 */

/**
 * Minimal shape of the HTTP client this module needs, so it can be exercised without a React tree.
 */
export interface HttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

/**
 * One entry of the `flow` server-config section, naming the administration flow for an action.
 *
 * There is no mode switch. An action runs through its flow when `defaultHandle` names an
 * administration flow that exists, and through the native endpoint otherwise.
 */
export interface AdministrationFlowConfig {
  defaultHandle?: string;
  expirySeconds?: number;
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
 * The `POST /flow/execute` response, narrowed to what the administration paths inspect.
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
 * Terminal status of a completed flow execution.
 */
export const FlowStatus = {
  COMPLETE: 'COMPLETE',
  ERROR: 'ERROR',
  INCOMPLETE: 'INCOMPLETE',
} as const;

/**
 * Flow type of every administration flow.
 */
export const ADMINISTRATION_FLOW_TYPE = 'ADMINISTRATION';

/**
 * Page size used when walking the flow listing to resolve a handle. Matches the server's maximum page
 * size, so the common case of a handful of administration flows resolves in one request. A larger
 * value is refused rather than clamped.
 */
export const FLOW_PAGE_SIZE = 100;

/**
 * Names of the `flow` server-config entries that drive an authorization change.
 *
 * Each corresponds to one shipped administration flow. Leaving a handle unset is the documented
 * opt-out: that change then runs on its native endpoint and revokes nothing.
 */
export const AdministrationFlowConfigKey = {
  APPLICATION_DELETION: 'applicationDeletionFlow',
  GROUP_DELETION: 'groupDeletionFlow',
  SECRET_REGENERATION: 'secretRegenerationFlow',
  GROUP_MEMBERSHIP_REMOVAL: 'groupMembershipRemovalFlow',
  ROLE_ASSIGNMENT_REMOVAL: 'roleAssignmentRemovalFlow',
  ROLE_DELETION: 'roleDeletionFlow',
  ROLE_PERMISSION_REMOVAL: 'rolePermissionRemovalFlow',
  SCOPE_DELETION: 'scopeDeletionFlow',
  USER_DELETION: 'userDeletionFlow',
} as const;

export type AdministrationFlowConfigKeyValue =
  (typeof AdministrationFlowConfigKey)[keyof typeof AdministrationFlowConfigKey];

/**
 * Identifiers of the inputs the administration executors declare. They deliberately avoid the names
 * the execution request already carries at its top level, so a value cannot be supplied in the wrong
 * place, where the failure is a flow that pauses asking for input.
 */
export const AdministrationFlowInput = {
  ACTION: 'targetActionId',
  APPLICATION: 'targetApplicationId',
  ASSIGNEE: 'targetAssigneeId',
  GROUP: 'targetGroupId',
  MEMBER: 'targetMemberId',
  PERMISSIONS: 'targetPermissions',
  RESOURCE: 'targetResourceId',
  RESOURCE_SERVER: 'targetResourceServerId',
  ROLE: 'targetRoleId',
  SUBJECT: 'subject',
} as const;

/**
 * Keys of the values an administration flow returns on its response. A regenerated client secret is
 * readable only here: it is hashed on write and no read path returns it afterwards.
 */
export const AdministrationFlowData = {
  CLIENT_SECRET: 'clientSecret',
} as const;

/**
 * An error carrying the code and params of a flow step that refused, so a feature's error mapper can
 * resolve it to the localized message for that code.
 *
 * A refusal arrives with HTTP 200 and a failed step rather than as a transport error, so there is no
 * `response.data.code` for the shared mapper to read. Attaching the envelope here is what lets a
 * refusal such as "the role is defined in declarative configuration" reach the user instead of a
 * generic failure.
 */
export class FlowExecutionFailure extends Error {
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
 * Reports whether an error is the server refusing the caller, rather than the request failing.
 */
function isPermissionDenied(error: unknown): boolean {
  const status = error as {status?: number; response?: {status?: number}} | undefined;

  return status?.status === 403 || status?.response?.status === 403;
}

/**
 * Reads the handle of the administration flow configured for an action, or an empty string when none
 * is configured.
 *
 * A genuine read failure propagates. Treating it as "no flow configured" would silently perform the
 * action through the native endpoint, which revokes nothing, and report it as a success.
 *
 * A permission denial is the one exception. Neither this endpoint nor the flow listing is mapped in
 * the server's API permission table, so both require the root `system` permission, while an operator
 * scoped to `system:group` can still perform the underlying change. Failing hard would take group
 * administration away from that operator entirely; falling back leaves them exactly where they were
 * before these flows were wired, which revokes nothing but breaks nothing either. The narrower fix is
 * to make these two reads available to any administrator, at which point this branch is dead.
 */
export async function resolveAdministrationFlowHandle(
  http: HttpLike,
  serverUrl: string,
  configKey: AdministrationFlowConfigKeyValue,
): Promise<string> {
  let response: {data?: unknown};

  try {
    response = await http.request({
      url: `${serverUrl}/server-config/flow`,
      method: 'GET',
    });
  } catch (error) {
    if (isPermissionDenied(error)) {
      return '';
    }
    throw error;
  }
  // The endpoint returns the declarative, writable and merged layers. Only the merged layer is the
  // effective configuration, so reading the envelope directly would always miss the value.
  const layers = (response?.data ?? {}) as ServerConfigLayers<Record<string, AdministrationFlowConfig>>;

  return layers.merged?.[configKey]?.defaultHandle ?? '';
}

/**
 * Finds the identifier of the administration flow carrying the given handle, or null when no such
 * flow exists.
 *
 * `/flow/execute` addresses a flow by id while the server config names it by handle, so the handle is
 * looked up against the administration flows rather than assumed.
 *
 * The listing is paginated and ordered newest first, which puts the bootstrap flows last, so a single
 * page would stop finding them once a deployment accumulates enough administration flows. Pages are
 * therefore walked until the handle is found or the listing is exhausted.
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
 * Runs an administration flow to completion in a single call and returns the values it produced.
 *
 * Every input the flow requires is supplied up front, so it should reach a terminal state without
 * pausing. An INCOMPLETE result therefore means the configured flow asks for something this caller
 * cannot provide, and is surfaced as an error rather than reported as a success that did nothing.
 */
export async function executeAdministrationFlow(
  http: HttpLike,
  serverUrl: string,
  flowId: string,
  inputs: Record<string, string>,
  action: string,
): Promise<Record<string, string>> {
  const response = await http.request({
    url: `${serverUrl}/flow/execute`,
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    data: {flowId, inputs},
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
 * A handle that is set but resolves to no flow is the opposite case: the deployment asked for
 * flow-based revocation and the flow it names is missing, so nothing here can carry that intent out.
 * Deleting a flow neither clears the configuration nor checks who references it, so a handle goes
 * stale silently. Falling back would perform the change with no revocation at all and report it as a
 * success, which is the one outcome the configuration was set to prevent.
 */
export async function runAdministrationFlow(
  http: HttpLike,
  serverUrl: string,
  configKey: AdministrationFlowConfigKeyValue,
  inputs: Record<string, string>,
  action: string,
): Promise<Record<string, string> | null> {
  const handle = await resolveAdministrationFlowHandle(http, serverUrl, configKey);

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

  return executeAdministrationFlow(http, serverUrl, flowId, inputs, action);
}
