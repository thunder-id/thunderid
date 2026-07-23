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
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import {StepTypes} from '@/features/flows/models/steps';

/**
 * Which exit of a widget node carries the flow onward to the downstream node.
 */
export type AutoWireHandle = 'success' | 'failure' | 'incomplete';

/**
 * A reference to one of a widget's own steps by its `__generationMeta__.replacers` key
 * (pre-substitution). Resolved to a concrete node id at drop time.
 */
export interface AutoWireStepRef {
  stepRef: string;
}

/**
 * A candidate node to attach the widget cluster against. Matched by executor name or node type,
 * never by id (ids differ across templates, e.g. "END" vs "end").
 */
export interface AutoWireAnchorCandidate {
  executorName?: string;
  nodeType?: string;
}

/**
 * Declares how a dropped widget cluster attaches to the existing flow graph.
 */
export interface AutoWireMeta {
  /** Internal node that receives the incoming edge from the upstream node. */
  entry?: AutoWireStepRef;
  /** Internal node + exit handle that connects onward to the downstream node. */
  exit?: AutoWireStepRef & {handle: AutoWireHandle};
  /**
   * Ordered upstream candidates. The first one present in the flow has its success edge
   * redirected into the widget entry. If none match, the entry is left unconnected for the
   * user to wire manually.
   */
  spliceAfter?: AutoWireAnchorCandidate[];
  /**
   * Ordered downstream candidates. The widget exit is connected to the first one present in
   * the flow. If none match, the exit is left unconnected for the user to wire manually.
   */
  spliceBefore?: AutoWireAnchorCandidate[];
  /** Dedup rules: reuse an existing equivalent node instead of the widget's bundled copy. */
  reuse?: {
    stepRef: string;
    matchBy: 'executorName';
    match: string;
  }[];
}

interface ActionCarrier {
  data?: {
    action?: {
      executor?: {name?: string};
      onSuccess?: string;
    };
  };
}

const executorName = (node: Node): string | undefined => (node as ActionCarrier).data?.action?.executor?.name;

const matchesCandidate = (node: Node, candidate: AutoWireAnchorCandidate): boolean => {
  if (candidate.executorName) {
    return executorName(node) === candidate.executorName;
  }
  if (candidate.nodeType) {
    return node.type === candidate.nodeType;
  }

  return false;
};

const handleId = (nodeId: string, handle: AutoWireHandle): string => {
  switch (handle) {
    case 'failure':
      return 'failure';
    case 'incomplete':
      return `${nodeId}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`;
    case 'success':
    default:
      return `${nodeId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`;
  }
};

const makeEdge = (id: string, source: string, sourceHandle: string, target: string, edgeStyle: string): Edge => ({
  animated: false,
  id,
  markerEnd: {type: MarkerType.Arrow},
  source,
  sourceHandle,
  target,
  type: edgeStyle,
});

/**
 * Splices a freshly dropped widget cluster into the existing flow graph by adding and removing
 * edges according to the widget's declarative `autoWire` metadata. When no metadata is present,
 * or no anchor/reusable node can be found, the graph is returned unchanged (append-only fallback,
 * identical to the pre-existing behaviour).
 *
 * @param preExistingNodes - The nodes present before this widget was dropped.
 * @param resultNodes - The full node set after the widget's steps were appended and id-resolved.
 * @param resultEdges - The edges after the widget's internal edges were generated.
 * @param autoWire - The widget's auto-wiring metadata, if any.
 * @param resolvedIds - Map of replacer key to concrete id, from `updateTemplatePlaceholderReferences`.
 * @param edgeStyle - The edge style to apply to generated edges.
 * @returns The spliced node and edge sets.
 */
const autoWireWidget = (
  preExistingNodes: Node[],
  resultNodes: Node[],
  resultEdges: Edge[],
  autoWire: AutoWireMeta | undefined,
  resolvedIds: Map<string, string>,
  edgeStyle: string,
): {nodes: Node[]; edges: Edge[]} => {
  if (!autoWire) {
    return {nodes: resultNodes, edges: resultEdges};
  }

  const preIds = new Set<string>(preExistingNodes.map((node: Node) => node.id));
  const clusterIds = new Set<string>(resultNodes.filter((node: Node) => !preIds.has(node.id)).map((n) => n.id));

  const resolve = (ref: string): string => resolvedIds.get(ref) ?? ref;

  let edges: Edge[] = [...resultEdges];
  const nodesToDrop = new Set<string>();

  // Reconcile node `action.onSuccess` with the rewired edges once all passes are done. `remapTarget`
  // repoints any reference to a dropped node; `nodeOnSuccess` sets a specific node's success target.
  // Leaving a stale onSuccess would make a later generateUnconnectedEdges pass re-create the old
  // edge (forking the flow) or leave a dangling reference to a removed node.
  const remapTarget = new Map<string, string>();
  const nodeOnSuccess = new Map<string, string>();

  // Pass A: reuse / dedup. Drop the widget's bundled copy of a node the flow already has and
  // redirect edges that pointed at the bundled copy onto the pre-existing one.
  (autoWire.reuse ?? []).forEach((rule) => {
    const dupId: string = resolve(rule.stepRef);
    const dupNode: Node | undefined = resultNodes.find((node: Node) => node.id === dupId);
    const existing: Node | undefined = preExistingNodes.find((node: Node) => executorName(node) === rule.match);

    if (dupNode && existing) {
      edges = edges
        .filter((edge: Edge) => edge.source !== dupId)
        .map((edge: Edge) => (edge.target === dupId ? {...edge, id: `${edge.id}__reuse`, target: existing.id} : edge));
      nodesToDrop.add(dupId);
      remapTarget.set(dupId, existing.id);
    }
  });

  // Pass B: two-sided splice. Connect the widget entry from an upstream node and its exit to a
  // downstream node. Each side is optional: when no candidate matches, that edge is deliberately
  // left unconnected for the user to wire wherever they want.
  if (autoWire.entry && autoWire.exit) {
    const entryId: string = resolve(autoWire.entry.stepRef);
    const exitId: string = resolve(autoWire.exit.stepRef);

    const findPresent = (candidates: AutoWireAnchorCandidate[] | undefined): Node | undefined => {
      for (const candidate of candidates ?? []) {
        const match: Node | undefined = resultNodes.find(
          (node: Node) => !clusterIds.has(node.id) && !nodesToDrop.has(node.id) && matchesCandidate(node, candidate),
        );

        if (match) {
          return match;
        }
      }

      return undefined;
    };

    // Downstream: connect the widget exit to the first present downstream candidate.
    const downstream: Node | undefined = findPresent(autoWire.spliceBefore);

    if (downstream) {
      edges.push(
        makeEdge(
          `${exitId}->${downstream.id}__autowire`,
          exitId,
          handleId(exitId, autoWire.exit.handle),
          downstream.id,
          edgeStyle,
        ),
      );

      if (autoWire.exit.handle === 'success') {
        nodeOnSuccess.set(exitId, downstream.id);
      }
    }

    // Upstream: redirect the first present upstream candidate's success edge into the widget entry
    // (inserting the widget right after it). If it has no success edge yet, add one.
    const upstream: Node | undefined = findPresent(autoWire.spliceAfter);

    if (upstream) {
      const successHandle: string = handleId(upstream.id, 'success');
      const outgoing: Edge[] = edges.filter(
        (edge: Edge) => edge.source === upstream.id && edge.sourceHandle === successHandle,
      );

      if (outgoing.length > 0) {
        edges = edges.map((edge: Edge) =>
          edge.source === upstream.id && edge.sourceHandle === successHandle
            ? {...edge, id: `${upstream.id}->${entryId}__autowire`, target: entryId}
            : edge,
        );
      } else {
        edges.push(makeEdge(`${upstream.id}->${entryId}__autowire`, upstream.id, successHandle, entryId, edgeStyle));
      }

      nodeOnSuccess.set(upstream.id, entryId);
    }
  }

  // Pass C: resolve a cluster executor's literal "END" onSuccess to the END node found by type
  // (create-flow templates use the id "end", so a literal match in edge generation misses it).
  const endNode: Node | undefined = resultNodes.find((node: Node) => node.type === StepTypes.End);

  if (endNode) {
    resultNodes.forEach((node: Node) => {
      if (!clusterIds.has(node.id) || nodesToDrop.has(node.id)) {
        return;
      }

      if ((node as ActionCarrier).data?.action?.onSuccess === 'END') {
        const source: string = handleId(node.id, 'success');
        const alreadyWired: boolean = edges.some(
          (edge: Edge) => edge.source === node.id && edge.sourceHandle === source,
        );

        if (!alreadyWired) {
          edges.push(makeEdge(`${node.id}->${endNode.id}__end`, node.id, source, endNode.id, edgeStyle));
        }

        nodeOnSuccess.set(node.id, endNode.id);
      }
    });
  }

  const finalNodes: Node[] = resultNodes
    .filter((node: Node) => !nodesToDrop.has(node.id))
    .map((node: Node) => {
      const action = (node as ActionCarrier).data?.action;

      if (!action) {
        return node;
      }

      const current: string | undefined = action.onSuccess;
      let next: string | undefined;

      if (nodeOnSuccess.has(node.id)) {
        next = nodeOnSuccess.get(node.id);
      } else if (current !== undefined && remapTarget.has(current)) {
        next = remapTarget.get(current);
      }

      if (next === undefined || next === current) {
        return node;
      }

      return {...node, data: {...node.data, action: {...action, onSuccess: next}}};
    });
  const finalIds = new Set<string>(finalNodes.map((node: Node) => node.id));
  const seen = new Set<string>();

  const finalEdges: Edge[] = edges
    .filter((edge: Edge) => finalIds.has(edge.source) && finalIds.has(edge.target))
    .filter((edge: Edge) => {
      const key = `${edge.source}|${edge.sourceHandle ?? ''}|${edge.target}`;

      if (seen.has(key)) {
        return false;
      }
      seen.add(key);

      return true;
    });

  return {nodes: finalNodes, edges: finalEdges};
};

export default autoWireWidget;
