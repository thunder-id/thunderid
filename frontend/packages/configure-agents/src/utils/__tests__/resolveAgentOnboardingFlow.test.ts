// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import resolveAgentOnboardingFlow, {
  AgentOnboardingFlowProblem,
  findAdministrationFlowId,
  type HttpLike,
} from '../resolveAgentOnboardingFlow';

const SERVER_URL = 'https://localhost:8090';

const httpReturning = (
  handler: (url: string) => {data?: unknown} | Promise<{data?: unknown}>,
): HttpLike & {calls: string[]} => {
  const calls: string[] = [];

  return {
    calls,
    request: async (config: unknown) => {
      const {url} = config as {url: string};
      calls.push(url);
      return handler(url);
    },
  };
};

const configLayers = (defaultHandle?: string): {data: unknown} => ({
  data: {
    readOnly: {agentOnboardingFlow: {defaultHandle: 'ignored-declarative'}},
    merged: defaultHandle === undefined ? {} : {agentOnboardingFlow: {defaultHandle}},
  },
});

describe('resolveAgentOnboardingFlow', () => {
  it('should resolve the flow id for a configured handle that exists', async () => {
    const http = httpReturning((url) =>
      url.includes('/server-config/flow')
        ? configLayers('default-agent-onboarding-flow')
        : {
            data: {
              flows: [
                {id: 'other-id', handle: 'something-else', flowType: 'ADMINISTRATION'},
                {id: 'flow-id-1', handle: 'default-agent-onboarding-flow', flowType: 'ADMINISTRATION'},
              ],
            },
          },
    );

    const result = await resolveAgentOnboardingFlow(http, SERVER_URL);

    expect(result.flowId).toBe('flow-id-1');
    expect(result.problem).toBeUndefined();
  });

  // The two problems need different remedies, so they must stay distinguishable rather than
  // collapsing into one "onboarding unavailable" state.
  it('should report NOT_CONFIGURED when no handle is set, without listing flows', async () => {
    const http = httpReturning(() => configLayers(undefined));

    const result = await resolveAgentOnboardingFlow(http, SERVER_URL);

    expect(result.problem).toBe(AgentOnboardingFlowProblem.NotConfigured);
    expect(result.flowId).toBeUndefined();
    expect(http.calls.some((url) => url.includes('/flows?'))).toBe(false);
  });

  it('should report FLOW_MISSING with the handle when the configured flow does not exist', async () => {
    const http = httpReturning((url) =>
      url.includes('/server-config/flow') ? configLayers('missing-handle') : {data: {flows: []}},
    );

    const result = await resolveAgentOnboardingFlow(http, SERVER_URL);

    expect(result.problem).toBe(AgentOnboardingFlowProblem.FlowMissing);
    expect(result.handle).toBe('missing-handle');
  });

  // Only the merged layer is what the server applies; reading the declarative layer would report a
  // handle an operator has already overridden.
  it('should read the handle from the merged layer only', async () => {
    const http = httpReturning((url) =>
      url.includes('/server-config/flow')
        ? {data: {readOnly: {agentOnboardingFlow: {defaultHandle: 'declarative-only'}}, merged: {}}}
        : {data: {flows: []}},
    );

    const result = await resolveAgentOnboardingFlow(http, SERVER_URL);

    expect(result.problem).toBe(AgentOnboardingFlowProblem.NotConfigured);
  });

  // A server-config read failure is not a misconfiguration. Swallowing it would send an
  // administrator off to fix a setting that is already correct.
  it('should propagate a server-config read failure rather than reporting it as unconfigured', async () => {
    const http: HttpLike = {request: vi.fn().mockRejectedValue(new Error('boom'))};

    await expect(resolveAgentOnboardingFlow(http, SERVER_URL)).rejects.toThrow('boom');
  });
});

describe('findAdministrationFlowId', () => {
  // The listing is newest first, which puts the bootstrap flow last. A single page would stop
  // finding it once a deployment accumulates enough administration flows.
  it('should walk past a full page to find a handle on a later page', async () => {
    const firstPage = Array.from({length: 100}, (_, i) => ({
      id: `id-${i}`,
      handle: `handle-${i}`,
      flowType: 'ADMINISTRATION',
    }));
    const http = httpReturning((url) =>
      url.includes('offset=0')
        ? {data: {flows: firstPage}}
        : {data: {flows: [{id: 'wanted', handle: 'target', flowType: 'ADMINISTRATION'}]}},
    );

    await expect(findAdministrationFlowId(http, SERVER_URL, 'target')).resolves.toBe('wanted');
    expect(http.calls).toHaveLength(2);
  });

  it('should stop on a short page rather than paging forever', async () => {
    const http = httpReturning(() => ({data: {flows: [{id: 'a', handle: 'x', flowType: 'ADMINISTRATION'}]}}));

    await expect(findAdministrationFlowId(http, SERVER_URL, 'not-there')).resolves.toBeNull();
    expect(http.calls).toHaveLength(1);
  });

  // A handle can be reused across flow types, so matching on handle alone could run a flow the
  // engine would reject.
  it('should ignore a matching handle that is not an administration flow', async () => {
    const http = httpReturning(() => ({
      data: {flows: [{id: 'reg-id', handle: 'target', flowType: 'REGISTRATION'}]},
    }));

    await expect(findAdministrationFlowId(http, SERVER_URL, 'target')).resolves.toBeNull();
  });
});
