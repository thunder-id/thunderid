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
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import autoWireWidget, {type AutoWireMeta} from '../autoWireWidget';
import generateUnconnectedEdges from '@/features/login-flow/utils/edgeUtils';

const NEXT = VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX;
const EDGE_STYLE = 'base-edge';

const onSuccessOf = (nodes: Node[], id: string): string | undefined =>
  (nodes.find((n: Node) => n.id === id) as {data?: {action?: {onSuccess?: string}}} | undefined)?.data?.action
    ?.onSuccess;

const executorNode = (id: string, name: string, onSuccess = ''): Node =>
  ({
    id,
    type: 'TASK_EXECUTION',
    position: {x: 0, y: 0},
    data: {action: {executor: {name}, onSuccess}},
  }) as unknown as Node;

const viewNode = (id: string): Node =>
  ({id, type: 'VIEW', position: {x: 0, y: 0}, data: {components: []}}) as unknown as Node;

const endNode = (id: string): Node => ({id, type: 'END', position: {x: 0, y: 0}, data: {}}) as unknown as Node;

const edge = (source: string, target: string, sourceHandle?: string): Edge => ({
  id: `${source}->${target}`,
  source,
  target,
  sourceHandle: sourceHandle ?? `${source}${NEXT}`,
  type: EDGE_STYLE,
});

const has = (edges: Edge[], source: string, target: string): boolean =>
  edges.some((e: Edge) => e.source === source && e.target === target);

describe('autoWireWidget', () => {
  it('returns inputs unchanged when no autoWire metadata is present', () => {
    const nodes: Node[] = [executorNode('cred', 'CredentialsAuthExecutor'), endNode('end')];
    const edges: Edge[] = [edge('cred', 'end')];

    const result = autoWireWidget(nodes, nodes, edges, undefined, new Map(), EDGE_STYLE);

    expect(result.nodes).toBe(nodes);
    expect(result.edges).toBe(edges);
  });

  describe('two-sided splice (consent / provisioning shape)', () => {
    const consentAutoWire: AutoWireMeta = {
      entry: {stepRef: 'CONSENT_EXECUTOR_STEP_ID'},
      exit: {stepRef: 'CONSENT_EXECUTOR_STEP_ID', handle: 'success'},
      spliceAfter: [{executorName: 'AuthorizationExecutor'}],
      spliceBefore: [{executorName: 'AuthAssertExecutor'}],
    };
    const resolved = new Map([['CONSENT_EXECUTOR_STEP_ID', 'consent_check_1']]);

    it('inserts the widget between the authorization executor and the auth assert generator', () => {
      const preExisting: Node[] = [
        executorNode('authz', 'AuthorizationExecutor', 'auth_assert'),
        executorNode('auth_assert', 'AuthAssertExecutor'),
        endNode('end'),
      ];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor'), viewNode('consent_view_1')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [edge('authz', 'auth_assert'), edge('auth_assert', 'end')];

      const {nodes, edges} = autoWireWidget(
        preExisting,
        resultNodes,
        resultEdges,
        consentAutoWire,
        resolved,
        EDGE_STYLE,
      );

      // The authorization -> auth assert edge is redirected through the widget.
      expect(has(edges, 'authz', 'auth_assert')).toBe(false);
      expect(has(edges, 'authz', 'consent_check_1')).toBe(true);
      expect(
        edges.some(
          (e: Edge) =>
            e.source === 'consent_check_1' && e.sourceHandle === `consent_check_1${NEXT}` && e.target === 'auth_assert',
        ),
      ).toBe(true);
      expect(has(edges, 'auth_assert', 'end')).toBe(true);

      // Node action.onSuccess is reconciled with the rewired edges, so it no longer points at the
      // bypassed target.
      expect(onSuccessOf(nodes, 'authz')).toBe('consent_check_1');
      expect(onSuccessOf(nodes, 'consent_check_1')).toBe('auth_assert');
    });

    it('leaves no stale action.onSuccess, so a later edge-regeneration pass adds no fork edge', () => {
      const preExisting: Node[] = [
        executorNode('authz', 'AuthorizationExecutor', 'auth_assert'),
        executorNode('auth_assert', 'AuthAssertExecutor'),
        endNode('end'),
      ];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor'), viewNode('consent_view_1')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [edge('authz', 'auth_assert'), edge('auth_assert', 'end')];

      const {nodes, edges} = autoWireWidget(
        preExisting,
        resultNodes,
        resultEdges,
        consentAutoWire,
        resolved,
        EDGE_STYLE,
      );

      // Re-running edge generation (as the next widget drop would) finds nothing to repair.
      expect(generateUnconnectedEdges(edges, nodes, EDGE_STYLE)).toHaveLength(0);
    });

    it('leaves the entry unconnected when there is no authorization executor upstream', () => {
      const preExisting: Node[] = [
        executorNode('cred', 'CredentialsAuthExecutor'),
        executorNode('auth_assert', 'AuthAssertExecutor'),
        endNode('end'),
      ];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [edge('cred', 'auth_assert'), edge('auth_assert', 'end')];

      const {edges} = autoWireWidget(preExisting, resultNodes, resultEdges, consentAutoWire, resolved, EDGE_STYLE);

      // Exit wires to the auth assert generator; entry is left dangling for the user.
      expect(has(edges, 'consent_check_1', 'auth_assert')).toBe(true);
      expect(edges.some((e: Edge) => e.target === 'consent_check_1')).toBe(false);
      // The pre-existing incoming edge of the anchor is not stolen.
      expect(has(edges, 'cred', 'auth_assert')).toBe(true);
    });

    it('leaves the exit unconnected when there is no auth assert generator downstream', () => {
      const preExisting: Node[] = [executorNode('authz', 'AuthorizationExecutor'), endNode('end')];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [edge('authz', 'end')];

      const {edges} = autoWireWidget(preExisting, resultNodes, resultEdges, consentAutoWire, resolved, EDGE_STYLE);

      // Entry wires from authorization; exit is left dangling for the user.
      expect(has(edges, 'authz', 'end')).toBe(false);
      expect(has(edges, 'authz', 'consent_check_1')).toBe(true);
      expect(edges.some((e: Edge) => e.source === 'consent_check_1')).toBe(false);
    });

    it('adds an edge from the upstream node when it has no outgoing success edge yet', () => {
      const preExisting: Node[] = [executorNode('authz', 'AuthorizationExecutor')];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [];

      const {edges} = autoWireWidget(preExisting, resultNodes, resultEdges, consentAutoWire, resolved, EDGE_STYLE);

      expect(
        edges.some(
          (e: Edge) => e.source === 'authz' && e.sourceHandle === `authz${NEXT}` && e.target === 'consent_check_1',
        ),
      ).toBe(true);
    });

    it('leaves the graph unchanged when neither anchor is present (blank canvas)', () => {
      const preExisting: Node[] = [viewNode('v')];
      const cluster: Node[] = [executorNode('consent_check_1', 'ConsentExecutor')];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [];

      const {nodes, edges} = autoWireWidget(
        preExisting,
        resultNodes,
        resultEdges,
        consentAutoWire,
        resolved,
        EDGE_STYLE,
      );

      expect(nodes).toHaveLength(2);
      expect(edges).toHaveLength(0);
    });
  });

  describe('reuse / dedup (federation shape)', () => {
    const federationAutoWire: AutoWireMeta = {
      reuse: [{stepRef: 'AUTH_ASSERT_EXECUTOR_ID', matchBy: 'executorName', match: 'AuthAssertExecutor'}],
    };
    const resolved = new Map([['AUTH_ASSERT_EXECUTOR_ID', 'bundled_auth']]);

    it('drops the bundled AuthAssert and redirects the executor edge onto the existing one', () => {
      const preExisting: Node[] = [viewNode('v'), executorNode('auth_assert', 'AuthAssertExecutor'), endNode('end')];
      const cluster: Node[] = [
        executorNode('google', 'GoogleOIDCAuthExecutor', 'bundled_auth'),
        executorNode('bundled_auth', 'AuthAssertExecutor', 'END'),
      ];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      const resultEdges: Edge[] = [edge('google', 'bundled_auth'), edge('bundled_auth', 'end')];

      const {nodes, edges} = autoWireWidget(
        preExisting,
        resultNodes,
        resultEdges,
        federationAutoWire,
        resolved,
        EDGE_STYLE,
      );

      expect(nodes.some((n: Node) => n.id === 'bundled_auth')).toBe(false);
      expect(has(edges, 'google', 'auth_assert')).toBe(true);
      expect(edges.some((e: Edge) => e.source === 'bundled_auth' || e.target === 'bundled_auth')).toBe(false);
      // The executor's action.onSuccess is repointed off the dropped bundled node onto the reused one.
      expect(onSuccessOf(nodes, 'google')).toBe('auth_assert');
    });

    it('keeps the bundled AuthAssert on a blank canvas and wires it to the END node by type', () => {
      const preExisting: Node[] = [viewNode('v'), endNode('end')];
      const cluster: Node[] = [
        executorNode('google', 'GoogleOIDCAuthExecutor'),
        executorNode('bundled_auth', 'AuthAssertExecutor', 'END'),
      ];
      const resultNodes: Node[] = [...preExisting, ...cluster];
      // generateUnconnectedEdges misses bundled_auth -> end because the END id is "end", not "END".
      const resultEdges: Edge[] = [edge('google', 'bundled_auth')];

      const {nodes, edges} = autoWireWidget(
        preExisting,
        resultNodes,
        resultEdges,
        federationAutoWire,
        resolved,
        EDGE_STYLE,
      );

      expect(nodes.some((n: Node) => n.id === 'bundled_auth')).toBe(true);
      expect(has(edges, 'bundled_auth', 'end')).toBe(true);
    });
  });
});
