// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import {StepTypes} from '../models/steps';

/**
 * Node type of the synthetic display node that represents a collapsed run of
 * consecutive executors in compact (non-verbose) mode.
 */
export const EXECUTION_STACK_NODE_TYPE = 'EXECUTION_STACK';

const EXECUTION_STACK_ID_PREFIX = 'execution-stack_';

/**
 * The head (first member) node id a stack id was derived from.
 */
export function getExecutionStackHeadId(stackId: string): string {
  return stackId.startsWith(EXECUTION_STACK_ID_PREFIX) ? stackId.slice(EXECUTION_STACK_ID_PREFIX.length) : stackId;
}

/**
 * Horizontal offset of each background "deck" layer peeking out behind the
 * stack chip.
 */
export const EXECUTION_STACK_LAYER_OFFSET = 4;

/**
 * How many background deck layers a stack renders at most.
 */
export const EXECUTION_STACK_MAX_LAYERS = 2;

/**
 * A member of a collapsed executor stack: the original node id plus its data,
 * enough to render the member chip and open its properties panel.
 */
export interface ExecutionStackMember {
  id: string;
  data: Node['data'];
}

/**
 * Data payload of an EXECUTION_STACK display node.
 */
export interface ExecutionStackData extends Record<string, unknown> {
  memberIds: string[];
  members: ExecutionStackMember[];
}

/**
 * Result of {@link collapseExecutorChains}: the display graph plus a lookup
 * from each synthetic stack id to its member node ids (used to translate
 * canvas changes on a stack back onto the underlying state nodes).
 */
export interface CollapsedExecutorGraph {
  edges: Edge[];
  nodes: Node[];
  stackMembersById: Map<string, string[]>;
}

/**
 * Rendered width of a stack chip: one chip showing the first executor's icon
 * plus the background deck layers peeking out behind it (one per additional
 * member, capped). Kept next to the transform so the layout engine and the
 * component agree.
 */
export function getExecutionStackWidth(memberCount: number): number {
  const layers = Math.min(Math.max(memberCount - 1, 0), EXECUTION_STACK_MAX_LAYERS);
  return VisualFlowConstants.FLOW_BUILDER_COMPACT_EXECUTION_NODE_SIZE + layers * EXECUTION_STACK_LAYER_OFFSET;
}

const isSuccessHandle = (sourceHandle: string | null | undefined): boolean =>
  sourceHandle !== 'failure' &&
  !sourceHandle?.endsWith(VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX) &&
  !sourceHandle?.endsWith(VisualFlowConstants.FLOW_BUILDER_PREVIOUS_HANDLE_SUFFIX);

/**
 * Collapses maximal runs of two or more consecutive executor nodes into a
 * single EXECUTION_STACK display node.
 *
 * A run member must be an EXECUTION node whose entire outgoing connectivity is
 * one success edge (no failure/incomplete edges anywhere), and every member
 * after the first must have exactly one incoming edge (the chain link), so
 * collapsing the run hides no branch anchors. Edges into the first member are
 * retargeted to the stack; the last member's outgoing success edge is
 * re-sourced from the stack. Interior chain edges are dropped.
 *
 * This is a pure display transform: the underlying graph state is never
 * mutated, and edge ids are preserved so decorations keyed by edge id (SSO
 * placement, simulation paths) keep working on the rewired edges.
 *
 * Chains whose stack id is in `expandedStackIds` are left uncollapsed, so the
 * user can open a group into its individual chips without leaving compact
 * mode.
 */
export default function collapseExecutorChains(
  nodes: Node[],
  edges: Edge[],
  expandedStackIds?: ReadonlySet<string>,
): CollapsedExecutorGraph {
  const outgoingByNode = new Map<string, Edge[]>();
  const incomingByNode = new Map<string, Edge[]>();
  for (const edge of edges) {
    outgoingByNode.set(edge.source, [...(outgoingByNode.get(edge.source) ?? []), edge]);
    incomingByNode.set(edge.target, [...(incomingByNode.get(edge.target) ?? []), edge]);
  }

  const nodeById = new Map(nodes.map((node) => [node.id, node]));

  // A node qualifies for stacking when it is an executor whose only outgoing
  // edge is a success edge: no failure/incomplete branches leave it.
  const isStackable = (node: Node | undefined): node is Node => {
    if (node?.type !== StepTypes.Execution) {
      return false;
    }
    const outgoing = outgoingByNode.get(node.id) ?? [];
    return outgoing.length === 1 && isSuccessHandle(outgoing[0].sourceHandle);
  };

  // A chain link A -> B holds when both ends are stackable executors and B's
  // only incoming edge is that link.
  const linksTo = (from: Node): Node | null => {
    const [edge] = outgoingByNode.get(from.id) ?? [];
    const target = nodeById.get(edge?.target ?? '');
    if (!isStackable(target) || (incomingByNode.get(target.id) ?? []).length !== 1) {
      return null;
    }
    return target;
  };

  // Heads are stackable nodes that are not themselves the tail of a link.
  const linkedTargets = new Set<string>();
  for (const node of nodes) {
    if (isStackable(node)) {
      const next = linksTo(node);
      if (next) {
        linkedTargets.add(next.id);
      }
    }
  }

  const stackByHeadId = new Map<string, Node[]>();
  for (const node of nodes) {
    if (!isStackable(node) || linkedTargets.has(node.id)) {
      continue;
    }
    const chain: Node[] = [node];
    let current = node;
    for (let next = linksTo(current); next; next = linksTo(current)) {
      chain.push(next);
      current = next;
    }
    if (chain.length >= 2 && !expandedStackIds?.has(`${EXECUTION_STACK_ID_PREFIX}${node.id}`)) {
      stackByHeadId.set(node.id, chain);
    }
  }

  if (stackByHeadId.size === 0) {
    return {edges, nodes, stackMembersById: new Map()};
  }

  const memberToStackId = new Map<string, string>();
  const tailIds = new Set<string>();
  const interiorEdgeIds = new Set<string>();
  const stackMembersById = new Map<string, string[]>();

  for (const [headId, chain] of stackByHeadId) {
    const stackId = `${EXECUTION_STACK_ID_PREFIX}${headId}`;
    stackMembersById.set(
      stackId,
      chain.map((member) => member.id),
    );
    chain.forEach((member, index) => {
      memberToStackId.set(member.id, stackId);
      if (index < chain.length - 1) {
        const [linkEdge] = outgoingByNode.get(member.id) ?? [];
        if (linkEdge) {
          interiorEdgeIds.add(linkEdge.id);
        }
      } else {
        tailIds.add(member.id);
      }
    });
  }

  const displayNodes: Node[] = [];
  for (const node of nodes) {
    const stackId = memberToStackId.get(node.id);
    if (!stackId) {
      displayNodes.push(node);
      continue;
    }
    const chain = stackByHeadId.get(node.id);
    if (!chain) {
      // Non-head member: dropped, represented by the head's stack node.
      continue;
    }
    displayNodes.push({
      // A stack id exists only on the canvas, so connecting or deleting one
      // would write an endpoint the real graph has no node for. Both are
      // disabled; the run has to be expanded before its members can be
      // rewired or removed. Handles still render, so existing edges anchor.
      connectable: false,
      data: {
        memberIds: chain.map((member) => member.id),
        members: chain.map((member) => ({data: member.data, id: member.id})),
      } satisfies ExecutionStackData,
      deletable: false,
      id: stackId,
      position: node.position,
      selected: chain.every((member) => member.selected === true),
      type: EXECUTION_STACK_NODE_TYPE,
    });
  }

  const displayEdges: Edge[] = [];
  for (const edge of edges) {
    if (interiorEdgeIds.has(edge.id)) {
      continue;
    }
    let displayEdge = edge;
    const targetStackId = memberToStackId.get(edge.target);
    if (targetStackId) {
      displayEdge = {...displayEdge, target: targetStackId, targetHandle: null};
    }
    const sourceStackId = memberToStackId.get(edge.source);
    if (sourceStackId && tailIds.has(edge.source)) {
      displayEdge = {
        ...displayEdge,
        source: sourceStackId,
        sourceHandle: `${sourceStackId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
      };
    }
    displayEdges.push(displayEdge);
  }

  return {edges: displayEdges, nodes: displayNodes, stackMembersById};
}
