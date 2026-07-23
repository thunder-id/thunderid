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
import {MarkerType} from '@xyflow/react';
import cloneDeep from 'lodash-es/cloneDeep';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import type {Step, StepData} from '@/features/flows/models/steps';
import {ExecutionTypes, StaticStepTypes, StepTypes} from '@/features/flows/models/steps';

/**
 * Pure graph transformations for the "Enable SSO" toggle.
 *
 * The transformations operate on React Flow nodes/edges (edges are the source
 * of truth for connections, mirroring reactFlowTransformer) and must produce
 * graphs that satisfy the backend pairing contract by construction:
 * - `SSOCheckExecutor.properties.checkpointRef` references a `SessionExecutor` node id.
 * - Every `SessionExecutor` node is referenced by at least one SSO check.
 * - A TASK_EXECUTION node's failure branch may only target a PROMPT (View) node.
 */

export interface SsoState {
  enabled: boolean;
  ssoCheckIds: string[];
}

export type JoinResolution =
  | {status: 'ok'; joinNodeId: string}
  | {status: 'ambiguous'; candidateJoinNodeIds: string[]; candidateEdgeIds: string[]}
  | {status: 'no-assert'}
  | {status: 'entry-not-prompt'}
  | {status: 'no-entry'};

export interface EnableSsoResult {
  nodes: Node[];
  edges: Edge[];
  newSsoCheckId: string | null;
  newSessionId: string | null;
}

export interface DisableSsoResult {
  nodes: Node[];
  edges: Edge[];
  removedCount: number;
}

const SSO_CHECK_ID_PREFIX = 'sso_check';
const SESSION_ID_PREFIX = 'session';

function getExecutorName(node: Node): string | undefined {
  return (node.data as StepData | undefined)?.action?.executor?.name;
}

export function isSsoCheckNode(node: Node): boolean {
  return node.type === StepTypes.Execution && getExecutorName(node) === ExecutionTypes.SSOCheck;
}

export function isSessionNode(node: Node): boolean {
  return node.type === StepTypes.Execution && getExecutorName(node) === ExecutionTypes.Session;
}

/**
 * Derives the toggle state from the graph: SSO is enabled iff the flow
 * contains at least one SSO check node. Template-created SSO flows therefore
 * read as enabled without any stored flag or migration.
 */
export function deriveSsoState(nodes: Node[]): SsoState {
  const ssoCheckIds = nodes.filter(isSsoCheckNode).map((node) => node.id);
  return {enabled: ssoCheckIds.length > 0, ssoCheckIds};
}

function findStartEdge(nodes: Node[], edges: Edge[]): Edge | undefined {
  const startNode = nodes.find((node) => node.type === StaticStepTypes.Start);
  if (!startNode) {
    return undefined;
  }
  return edges.find((edge) => edge.source === startNode.id);
}

/**
 * Resolves the join node for a given auth-assert node: the assert itself, or
 * the AuthorizationExecutor immediately before it when that is its sole feeder.
 */
function resolveJoinForAssert(assertNode: Node, nodes: Node[], edges: Edge[]): string {
  const incoming = edges.filter((edge) => edge.target === assertNode.id);
  if (incoming.length === 1) {
    const feeder = nodes.find((node) => node.id === incoming[0].source);
    if (feeder && getExecutorName(feeder) === ExecutionTypes.Authorization) {
      return feeder.id;
    }
  }
  return assertNode.id;
}

/**
 * Finds where the session checkpoint should join the flow. The join point is
 * the authentication-complete boundary: the node feeding AuthAssertExecutor
 * (or the AuthorizationExecutor immediately before it). When the heuristic
 * cannot decide, it reports the ambiguity instead of guessing.
 */
export function findJoinCandidates(nodes: Node[], edges: Edge[]): JoinResolution {
  const startEdge = findStartEdge(nodes, edges);
  if (!startEdge) {
    return {status: 'no-entry'};
  }

  const entryNode = nodes.find((node) => node.id === startEdge.target);
  if (entryNode?.type !== StepTypes.View) {
    // The SSO check's failure branch must target a PROMPT node (backend rule),
    // so the flow has to start with a view step.
    return {status: 'entry-not-prompt'};
  }

  const assertNodes = nodes.filter((node) => getExecutorName(node) === ExecutionTypes.AuthAssert);
  if (assertNodes.length === 0) {
    return {status: 'no-assert'};
  }

  const joinNodeIds = [...new Set(assertNodes.map((assertNode) => resolveJoinForAssert(assertNode, nodes, edges)))];
  if (joinNodeIds.length === 1) {
    return {status: 'ok', joinNodeId: joinNodeIds[0]};
  }

  const joinIdSet = new Set(joinNodeIds);
  const candidateEdgeIds = edges.filter((edge) => joinIdSet.has(edge.target)).map((edge) => edge.id);
  return {status: 'ambiguous', candidateJoinNodeIds: joinNodeIds, candidateEdgeIds};
}

function generatePairSuffix(existingIds: Set<string>): string {
  let suffix = Math.random().toString(36).substring(2, 6);
  while (
    suffix.length < 4 ||
    existingIds.has(`${SSO_CHECK_ID_PREFIX}_${suffix}`) ||
    existingIds.has(`${SESSION_ID_PREFIX}_${suffix}`)
  ) {
    suffix = Math.random().toString(36).substring(2, 6);
  }
  return suffix;
}

function createSuccessEdge(sourceId: string, targetId: string, edgeStyle: string): Edge {
  return {
    animated: false,
    id: `${sourceId}-to-${targetId}`,
    markerEnd: {type: MarkerType.Arrow},
    source: sourceId,
    sourceHandle: `${sourceId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
    target: targetId,
    type: edgeStyle,
  };
}

function createFailureEdge(sourceId: string, targetId: string, edgeStyle: string): Edge {
  return {
    animated: false,
    id: `${sourceId}-failure-to-${targetId}`,
    markerEnd: {type: MarkerType.Arrow},
    source: sourceId,
    sourceHandle: 'failure',
    target: targetId,
    type: edgeStyle,
  };
}

function buildExecutorNode(resource: Step, id: string, position: {x: number; y: number}): Node {
  const cloned = cloneDeep(resource) as unknown as Node & {display?: unknown};
  return {
    ...cloned,
    // The display metadata is mirrored into data so the Execution node component
    // can render the label, icon and outcome tooltips (same as resolveStepMetadata).
    data: {components: [], ...(cloned.data ?? {}), ...(cloned.display ? {display: cloned.display} : {})},
    deletable: true,
    id,
    position,
  };
}

/**
 * Inserts an SSO check + session checkpoint pair into the flow.
 *
 * The SSO check is spliced in right after Start (success skips to the session
 * checkpoint, failure falls back to the original entry prompt) and the session
 * node is spliced in front of the join node so every fresh-path edge that
 * entered the join now saves the session first.
 *
 * @param joinNodeId - Explicit join override from placement mode; when omitted
 *   the heuristic must resolve to a single join node.
 */
export function enableSso(
  nodes: Node[],
  edges: Edge[],
  ssoResources: {ssoCheck: Step; session: Step},
  edgeStyle: string,
  joinNodeId?: string,
): EnableSsoResult {
  if (deriveSsoState(nodes).enabled) {
    return {edges, newSsoCheckId: null, newSessionId: null, nodes};
  }

  const startEdge = findStartEdge(nodes, edges);
  if (!startEdge) {
    return {edges, newSsoCheckId: null, newSessionId: null, nodes};
  }
  const entryNodeId = startEdge.target;

  let resolvedJoinId = joinNodeId;
  if (!resolvedJoinId) {
    const resolution = findJoinCandidates(nodes, edges);
    if (resolution.status !== 'ok') {
      return {edges, newSsoCheckId: null, newSessionId: null, nodes};
    }
    resolvedJoinId = resolution.joinNodeId;
  }

  const startNode = nodes.find((node) => node.type === StaticStepTypes.Start);
  const joinNode = nodes.find((node) => node.id === resolvedJoinId);
  if (!startNode || !joinNode) {
    return {edges, newSsoCheckId: null, newSessionId: null, nodes};
  }

  const suffix = generatePairSuffix(new Set(nodes.map((node) => node.id)));
  const ssoCheckId = `${SSO_CHECK_ID_PREFIX}_${suffix}`;
  const sessionId = `${SESSION_ID_PREFIX}_${suffix}`;

  const ssoCheckNode = buildExecutorNode(ssoResources.ssoCheck, ssoCheckId, {
    x: startNode.position.x + 200,
    y: startNode.position.y - 50,
  });
  // Merge over any default properties the executor resource declares.
  const ssoCheckProperties = (ssoCheckNode.data as StepData | undefined)?.properties ?? {};
  ssoCheckNode.data = {...ssoCheckNode.data, properties: {...ssoCheckProperties, checkpointRef: sessionId}};

  // SessionExecutor supports no properties on the backend, so none are set.
  const sessionNode = buildExecutorNode(ssoResources.session, sessionId, {
    x: joinNode.position.x - 300,
    y: joinNode.position.y,
  });

  const rewiredEdges = edges.map((edge) => {
    if (edge.id === startEdge.id) {
      // Start now enters the SSO check instead of the original entry step.
      return {...edge, target: ssoCheckId, targetHandle: undefined};
    }
    if (edge.target === resolvedJoinId) {
      // Fresh-path feeders of the join now save the session first.
      return {...edge, target: sessionId, targetHandle: undefined};
    }
    return edge;
  });

  return {
    edges: [
      ...rewiredEdges,
      createSuccessEdge(ssoCheckId, sessionId, edgeStyle),
      createFailureEdge(ssoCheckId, entryNodeId, edgeStyle),
      createSuccessEdge(sessionId, resolvedJoinId, edgeStyle),
    ],
    newSsoCheckId: ssoCheckId,
    newSessionId: sessionId,
    nodes: [...nodes, ssoCheckNode, sessionNode],
  };
}

/**
 * Removes every SSO check and session checkpoint from the flow, splicing the
 * surrounding edges back together: whatever entered an SSO check re-enters its
 * fresh-authentication (failure) target, and whatever entered a session node
 * re-enters the session's success target. Orphaned halves of a pair are
 * removed too, since any leftover SSO wiring is invalid on the backend.
 */
export function disableSso(nodes: Node[], edges: Edge[]): DisableSsoResult {
  const ssoCheckNodes = nodes.filter(isSsoCheckNode);
  const sessionNodes = nodes.filter(isSessionNode);
  const removedIds = new Set([...ssoCheckNodes, ...sessionNodes].map((node) => node.id));

  if (removedIds.size === 0) {
    return {edges, nodes, removedCount: 0};
  }

  // Where traffic entering a removed node should be redirected.
  const redirects = new Map<string, string | undefined>();
  ssoCheckNodes.forEach((node) => {
    const failureEdge = edges.find((edge) => edge.source === node.id && edge.sourceHandle === 'failure');
    const successEdge = edges.find((edge) => edge.source === node.id && edge.sourceHandle !== 'failure');
    redirects.set(node.id, failureEdge?.target ?? successEdge?.target);
  });
  sessionNodes.forEach((node) => {
    const successEdge = edges.find((edge) => edge.source === node.id && edge.sourceHandle !== 'failure');
    redirects.set(node.id, successEdge?.target);
  });

  const resolveTarget = (target: string): string | undefined => {
    let resolved: string | undefined = target;
    let hops = 0;
    while (resolved && removedIds.has(resolved) && hops < removedIds.size + 1) {
      resolved = redirects.get(resolved);
      hops += 1;
    }
    return resolved && !removedIds.has(resolved) ? resolved : undefined;
  };

  const splicedEdges: Edge[] = [];
  const seenConnections = new Set<string>();
  edges.forEach((edge) => {
    if (removedIds.has(edge.source)) {
      return;
    }
    const resolvedTarget = resolveTarget(edge.target);
    if (!resolvedTarget) {
      return;
    }
    const connectionKey = `${edge.source}|${edge.sourceHandle ?? ''}|${resolvedTarget}`;
    if (seenConnections.has(connectionKey)) {
      return;
    }
    seenConnections.add(connectionKey);
    splicedEdges.push(
      edge.target === resolvedTarget ? edge : {...edge, target: resolvedTarget, targetHandle: undefined},
    );
  });

  return {
    edges: splicedEdges,
    nodes: nodes.filter((node) => !removedIds.has(node.id)),
    removedCount: ssoCheckNodes.length,
  };
}
