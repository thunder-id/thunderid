// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {findAdministrationFlowId, type HttpLike, type ServerConfigLayers} from '@thunderid/utils';

export type {HttpLike};

interface AgentOnboardingFlowConfig {
  defaultHandle?: string;
}

interface FlowSectionConfig {
  agentOnboardingFlow?: AgentOnboardingFlowConfig;
}

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
