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

import type {Edge, Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import {deriveSsoState, disableSso, enableSso, findJoinCandidates} from '../ssoGraphTransforms';
import type {Step} from '@/features/flows/models/steps';
import {ExecutionTypes, StaticStepTypes, StepTypes} from '@/features/flows/models/steps';

const ssoCheckResource = {
  category: 'EXECUTOR',
  data: {
    action: {
      executor: {name: ExecutionTypes.SSOCheck},
      onFailure: '',
      onSuccess: '',
      type: 'EXECUTOR',
    },
  },
  display: {header: 'SSO Check Executor', label: 'Check SSO Session'},
  resourceType: 'STEP',
  type: StepTypes.Execution,
} as unknown as Step;

const sessionResource = {
  category: 'EXECUTOR',
  data: {
    action: {
      executor: {name: ExecutionTypes.Session},
      onSuccess: '',
      type: 'EXECUTOR',
    },
  },
  display: {header: 'Session Executor', label: 'Save / Load Session'},
  resourceType: 'STEP',
  type: StepTypes.Execution,
} as unknown as Step;

const ssoResources = {session: sessionResource, ssoCheck: ssoCheckResource};

function makeStart(id = 'start'): Node {
  return {data: {displayOnly: true}, id, position: {x: 0, y: 400}, type: StaticStepTypes.Start};
}

function makeView(id: string, x = 300): Node {
  return {data: {components: []}, id, position: {x, y: 400}, type: StepTypes.View};
}

function makeExecution(id: string, executorName: string, x = 600): Node {
  return {
    data: {action: {executor: {name: executorName}, type: 'EXECUTOR'}},
    id,
    position: {x, y: 400},
    type: StepTypes.Execution,
  };
}

function makeEnd(id = 'end'): Node {
  return {data: {}, id, position: {x: 2000, y: 400}, type: StepTypes.End};
}

function successEdge(source: string, target: string, sourceHandle?: string): Edge {
  return {
    id: `${source}-to-${target}`,
    source,
    sourceHandle: sourceHandle ?? `${source}_NEXT`,
    target,
    type: 'default',
  };
}

function failureEdge(source: string, target: string): Edge {
  return {id: `${source}-failure-to-${target}`, source, sourceHandle: 'failure', target, type: 'default'};
}

/**
 * BASIC-template shaped canvas:
 * start -> prompt -> credentials_auth -> authorization -> assert -> end
 */
function basicFlow(): {nodes: Node[]; edges: Edge[]} {
  return {
    edges: [
      successEdge('start', 'prompt_credentials', 'start_NEXT'),
      successEdge('prompt_credentials', 'credentials_auth', 'submit_button_NEXT'),
      successEdge('credentials_auth', 'authorization_check'),
      successEdge('authorization_check', 'auth_assert'),
      successEdge('auth_assert', 'end'),
    ],
    nodes: [
      makeStart(),
      makeView('prompt_credentials'),
      makeExecution('credentials_auth', 'CredentialsAuthExecutor', 700),
      makeExecution('authorization_check', ExecutionTypes.Authorization, 1400),
      makeExecution('auth_assert', ExecutionTypes.AuthAssert, 1700),
      makeEnd(),
    ],
  };
}

/**
 * BASIC_SSO-template shaped canvas (what a template-created flow loads as).
 */
function basicSsoFlow(): {nodes: Node[]; edges: Edge[]} {
  const ssoCheck = makeExecution('sso_check', ExecutionTypes.SSOCheck, 200);
  ssoCheck.data = {...ssoCheck.data, properties: {checkpointRef: 'session'}};
  return {
    edges: [
      successEdge('start', 'sso_check', 'start_NEXT'),
      successEdge('sso_check', 'session'),
      failureEdge('sso_check', 'prompt_credentials'),
      successEdge('prompt_credentials', 'credentials_auth', 'submit_button_NEXT'),
      successEdge('credentials_auth', 'session'),
      successEdge('session', 'authorization_check'),
      successEdge('authorization_check', 'auth_assert'),
      successEdge('auth_assert', 'end'),
    ],
    nodes: [
      makeStart(),
      ssoCheck,
      makeView('prompt_credentials'),
      makeExecution('credentials_auth', 'CredentialsAuthExecutor', 700),
      makeExecution('session', ExecutionTypes.Session, 1100),
      makeExecution('authorization_check', ExecutionTypes.Authorization, 1400),
      makeExecution('auth_assert', ExecutionTypes.AuthAssert, 1700),
      makeEnd(),
    ],
  };
}

function connectionTriples(edges: Edge[]): string[] {
  return edges.map((edge) => `${edge.source}|${edge.sourceHandle ?? ''}|${edge.target}`).sort();
}

describe('deriveSsoState', () => {
  it('reports disabled for a flow without SSO check nodes', () => {
    const {nodes} = basicFlow();
    expect(deriveSsoState(nodes)).toEqual({enabled: false, ssoCheckIds: []});
  });

  it('reports enabled for a template-created SSO flow', () => {
    const {nodes} = basicSsoFlow();
    expect(deriveSsoState(nodes)).toEqual({enabled: true, ssoCheckIds: ['sso_check']});
  });
});

describe('findJoinCandidates', () => {
  it('hops back from the assert to the authorization node feeding it', () => {
    const {nodes, edges} = basicFlow();
    expect(findJoinCandidates(nodes, edges)).toEqual({joinNodeId: 'authorization_check', status: 'ok'});
  });

  it('uses the assert node itself when no authorization executor feeds it', () => {
    const {nodes, edges} = basicFlow();
    const withoutAuthorization = nodes.filter((node) => node.id !== 'authorization_check');
    const rewired = edges
      .filter((edge) => edge.source !== 'authorization_check' && edge.target !== 'authorization_check')
      .concat(successEdge('credentials_auth', 'auth_assert'));
    expect(findJoinCandidates(withoutAuthorization, rewired)).toEqual({joinNodeId: 'auth_assert', status: 'ok'});
  });

  it('returns no-assert when the flow has no auth assert executor', () => {
    const {nodes, edges} = basicFlow();
    expect(
      findJoinCandidates(
        nodes.filter((node) => node.id !== 'auth_assert'),
        edges,
      ),
    ).toEqual({
      status: 'no-assert',
    });
  });

  it('returns no-entry when start has no outgoing edge', () => {
    const {nodes, edges} = basicFlow();
    expect(
      findJoinCandidates(
        nodes,
        edges.filter((edge) => edge.source !== 'start'),
      ),
    ).toEqual({status: 'no-entry'});
  });

  it('returns entry-not-prompt when start leads to an execution node', () => {
    const {nodes, edges} = basicFlow();
    const rewired = edges.map((edge) => (edge.source === 'start' ? {...edge, target: 'credentials_auth'} : edge));
    expect(findJoinCandidates(nodes, rewired)).toEqual({status: 'entry-not-prompt'});
  });

  it('returns ambiguous with candidate edges when multiple asserts exist', () => {
    const {nodes, edges} = basicFlow();
    const secondAssert = makeExecution('auth_assert_2', ExecutionTypes.AuthAssert, 1700);
    const withSecond = [...nodes, secondAssert];
    const withEdges = [...edges, successEdge('prompt_credentials', 'auth_assert_2', 'other_button_NEXT')];

    const resolution = findJoinCandidates(withSecond, withEdges);
    expect(resolution).toEqual({
      candidateEdgeIds: ['credentials_auth-to-authorization_check', 'prompt_credentials-to-auth_assert_2'],
      candidateJoinNodeIds: ['authorization_check', 'auth_assert_2'],
      status: 'ambiguous',
    });
  });
});

describe('enableSso', () => {
  it('produces the BASIC_SSO topology from a BASIC flow', () => {
    const {nodes, edges} = basicFlow();
    const result = enableSso(nodes, edges, ssoResources, 'default');

    expect(result.newSsoCheckId).toMatch(/^sso_check_[a-z0-9]{4}$/);
    expect(result.newSessionId).toMatch(/^session_[a-z0-9]{4}$/);
    // The pair shares an id suffix so the two nodes read as related.
    expect(result.newSsoCheckId?.replace('sso_check_', '')).toBe(result.newSessionId?.replace('session_', ''));

    const ssoCheckNode = result.nodes.find((node) => node.id === result.newSsoCheckId);
    const sessionNode = result.nodes.find((node) => node.id === result.newSessionId);
    expect((ssoCheckNode?.data as {properties?: {checkpointRef?: string}}).properties?.checkpointRef).toBe(
      result.newSessionId,
    );
    expect((sessionNode?.data as {properties?: unknown}).properties).toBeUndefined();

    expect(connectionTriples(result.edges)).toEqual(
      connectionTriples([
        successEdge('start', result.newSsoCheckId!, 'start_NEXT'),
        successEdge(result.newSsoCheckId!, result.newSessionId!),
        failureEdge(result.newSsoCheckId!, 'prompt_credentials'),
        successEdge('prompt_credentials', 'credentials_auth', 'submit_button_NEXT'),
        successEdge('credentials_auth', result.newSessionId!),
        successEdge(result.newSessionId!, 'authorization_check'),
        successEdge('authorization_check', 'auth_assert'),
        successEdge('auth_assert', 'end'),
      ]),
    );

    // No dangling endpoints.
    const nodeIds = new Set(result.nodes.map((node) => node.id));
    result.edges.forEach((edge) => {
      expect(nodeIds.has(edge.source)).toBe(true);
      expect(nodeIds.has(edge.target)).toBe(true);
    });
  });

  it('preserves default executor properties when setting checkpointRef', () => {
    const {nodes, edges} = basicFlow();
    const ssoCheckWithDefaults = {
      ...ssoCheckResource,
      data: {...(ssoCheckResource as unknown as {data: object}).data, properties: {someDefault: 'value'}},
    } as unknown as Step;

    const result = enableSso(nodes, edges, {session: sessionResource, ssoCheck: ssoCheckWithDefaults}, 'default');

    const ssoCheckNode = result.nodes.find((node) => node.id === result.newSsoCheckId);
    expect((ssoCheckNode?.data as {properties?: Record<string, unknown>}).properties).toEqual({
      checkpointRef: result.newSessionId,
      someDefault: 'value',
    });
  });

  it('is a no-op when SSO is already enabled', () => {
    const {nodes, edges} = basicSsoFlow();
    const result = enableSso(nodes, edges, ssoResources, 'default');
    expect(result.nodes).toBe(nodes);
    expect(result.edges).toBe(edges);
    expect(result.newSsoCheckId).toBeNull();
  });

  it('honors an explicit join node from placement mode', () => {
    const {nodes, edges} = basicFlow();
    const result = enableSso(nodes, edges, ssoResources, 'default', 'auth_assert');

    const triples = connectionTriples(result.edges);
    expect(triples).toContain(`${result.newSessionId}|${result.newSessionId}_NEXT|auth_assert`);
    expect(triples).toContain(`authorization_check|authorization_check_NEXT|${result.newSessionId}`);
  });

  it('retargets multiple fresh-path feeders of the join to the session node', () => {
    const {nodes, edges} = basicFlow();
    const otpAuth = makeExecution('otp_auth', 'OTPExecutor', 900);
    const withOtp = [...nodes, otpAuth];
    const withOtpEdges = [...edges, successEdge('otp_auth', 'authorization_check')];

    const result = enableSso(withOtp, withOtpEdges, ssoResources, 'default');
    const triples = connectionTriples(result.edges);
    expect(triples).toContain(`credentials_auth|credentials_auth_NEXT|${result.newSessionId}`);
    expect(triples).toContain(`otp_auth|otp_auth_NEXT|${result.newSessionId}`);
  });
});

describe('disableSso', () => {
  it('removes the pair and splices the flow back to the BASIC topology', () => {
    const {nodes, edges} = basicSsoFlow();
    const result = disableSso(nodes, edges);

    expect(result.removedCount).toBe(1);
    expect(result.nodes.map((node) => node.id).sort()).toEqual(
      ['auth_assert', 'authorization_check', 'credentials_auth', 'end', 'prompt_credentials', 'start'].sort(),
    );
    expect(connectionTriples(result.edges)).toEqual(connectionTriples(basicFlow().edges));
  });

  it('round-trips: enable then disable restores the original connections', () => {
    const {nodes, edges} = basicFlow();
    const enabled = enableSso(nodes, edges, ssoResources, 'default');
    const restored = disableSso(enabled.nodes, enabled.edges);

    expect(connectionTriples(restored.edges)).toEqual(connectionTriples(edges));
    expect(restored.nodes.map((node) => node.id).sort()).toEqual(nodes.map((node) => node.id).sort());
  });

  it('removes an orphaned session node left from a hand-deleted SSO check', () => {
    const {nodes, edges} = basicSsoFlow();
    const withoutCheck = nodes.filter((node) => node.id !== 'sso_check');
    const withoutCheckEdges = edges.filter((edge) => edge.source !== 'sso_check' && edge.target !== 'sso_check');

    const result = disableSso(withoutCheck, withoutCheckEdges);
    expect(result.removedCount).toBe(0);
    expect(result.nodes.some((node) => node.id === 'session')).toBe(false);
    expect(connectionTriples(result.edges)).toContain('credentials_auth|credentials_auth_NEXT|authorization_check');
  });

  it('is a no-op on a flow without SSO wiring', () => {
    const {nodes, edges} = basicFlow();
    const result = disableSso(nodes, edges);
    expect(result.removedCount).toBe(0);
    expect(result.nodes).toBe(nodes);
    expect(result.edges).toBe(edges);
  });
});
