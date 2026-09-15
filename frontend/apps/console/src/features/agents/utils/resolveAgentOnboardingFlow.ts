// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Minimal shape of the HTTP client this module needs, so it can be used outside a React tree. */
export interface HttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

interface AgentOnboardingFlowConfig {
  defaultHandle?: string;
}

interface FlowSectionConfig {
  agentOnboardingFlow?: AgentOnboardingFlowConfig;
}

interface ServerConfigLayers<T> {
  readOnly?: T;
  writable?: T;
  merged?: T;
}

interface BasicFlowSummary {
  id: string;
  handle: string;
  flowType: string;
}

interface FlowListResponse {
  flows?: BasicFlowSummary[];
}

const ADMINISTRATION_FLOW_TYPE = 'ADMINISTRATION';

/**
 * Page size used when walking the flow listing to resolve a handle. Matches the server's maximum
 * page size, so the common case of a handful of administration flows resolves in one request.
 */
const FLOW_PAGE_SIZE = 100;

/** Why the onboarding flow could not be resolved. Each case needs a different remedy. */
export const AgentOnboardingFlowProblem = {
  /** No handle is configured, so the deployment has not chosen an onboarding flow. */
  NotConfigured: 'NOT_CONFIGURED',
  /** A handle is configured but no administration flow carries it. */
  FlowMissing: 'FLOW_MISSING',
} as const;

export type AgentOnboardingFlowProblem = (typeof AgentOnboardingFlowProblem)[keyof typeof AgentOnboardingFlowProblem];

export type AgentOnboardingFlowResolution =
  | {flowId: string; handle: string; problem?: never}
  | {flowId?: never; handle: string; problem: AgentOnboardingFlowProblem};

/**
 * Reads the handle of the configured agent onboarding flow, or an empty string when none is set.
 *
 * Read failures propagate rather than being reported as "not configured", so a transient failure
 * is not mistaken for a misconfiguration.
 */
export async function resolveAgentOnboardingFlowHandle(http: HttpLike, serverUrl: string): Promise<string> {
  const response = await http.request({
    url: `${serverUrl}/server-config/flow`,
    method: 'GET',
  });
  // The endpoint returns the declarative, writable and merged layers, and only the merged layer
  // is the effective configuration.
  const layers = (response?.data ?? {}) as ServerConfigLayers<FlowSectionConfig>;

  return layers.merged?.agentOnboardingFlow?.defaultHandle ?? '';
}

/**
 * Finds the identifier of the administration flow carrying the given handle, or null when no such
 * flow exists. `/flow/execute` addresses a flow by id while the server config names it by handle.
 *
 * The listing is paginated and ordered newest first, which puts a bootstrap flow last, so pages
 * are walked until the handle is found or the listing is exhausted.
 *
 * `deleteUserViaFlow.ts` in `@thunderid/configure-users` walks the listing the same way; a fix to
 * the paging here belongs there too.
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
 * Resolves the agent onboarding flow to run, reporting which of the two remediable problems applies
 * when it cannot. There is no non-flow path to fall back to, the flow's provisioning node being
 * what creates the agent, so the caller reports the problem rather than working around it.
 */
export default async function resolveAgentOnboardingFlow(
  http: HttpLike,
  serverUrl: string,
): Promise<AgentOnboardingFlowResolution> {
  const handle = await resolveAgentOnboardingFlowHandle(http, serverUrl);

  if (!handle) {
    return {handle: '', problem: AgentOnboardingFlowProblem.NotConfigured};
  }

  const flowId = await findAdministrationFlowId(http, serverUrl, handle);

  if (!flowId) {
    return {handle, problem: AgentOnboardingFlowProblem.FlowMissing};
  }

  return {flowId, handle};
}
