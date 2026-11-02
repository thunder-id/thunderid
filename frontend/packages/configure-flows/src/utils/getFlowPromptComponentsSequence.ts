// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {EmbeddedFlowComponent} from '@thunderid/react';
import {FlowNodeType} from '../models/flows';
import type {FlowNode} from '../models/responses';

/** The subset of a flow shared by both a persisted {@link FlowDefinitionResponse} and a
 *  not-yet-created {@link CreateFlowRequest} — the only part this walker needs. */
interface FlowLike {
  nodes?: FlowNode[];
}

/** Safety cap on how many screens to walk. `visited` alone guards cycles, not a very long
 *  (but acyclic) chain; no realistic hand-authored flow needs more sequential prompt screens
 *  than this to preview. */
const MAX_STEPS = 20;

/**
 * Follows a single outgoing edge from a node: `onSuccess` when set (covers TASK_EXECUTION nodes,
 * and any PROMPT node wired to advance that way), else a display-only PROMPT's `next`, else an
 * interactive PROMPT's first prompt action's `nextNode` (`next` and `prompts` are mutually
 * exclusive). Sufficient for walking the linear/branching shapes this app generates (see
 * `generateFlowGraph`); nodes with multiple genuinely divergent branches (e.g. the MFA channel
 * choice) are walked via their first branch only.
 */
function nextNodeId(node: FlowNode): string | undefined {
  return node.onSuccess ?? node.next ?? node.prompts?.[0]?.action?.nextNode;
}

/**
 * Extracts the UI components of every sequentially-reachable PROMPT screen in a flow, in order:
 * the primary sign-in screen, then whatever screen follows it (an MFA challenge, a "check your
 * email" wait screen, etc.), for as many screens as the flow actually has. Generalizes what used
 * to be two separate single-purpose extractors (first screen only, first-and-second only) into
 * one N-screen walker, so a manually-selected pre-configured flow with an arbitrary number of
 * screens can be paged through in the live preview.
 *
 * Only follows the first branch at a fork (see {@link nextNodeId}) — a flow with a genuine branch
 * point will only preview one path through it.
 *
 * @param flow - The full flow definition (persisted or not-yet-created), or undefined while loading
 * @returns Each reachable PROMPT screen's components, in order, or null if the flow has no
 *   renderable prompt screen at all
 *
 * @public
 */
export default function getFlowPromptComponentsSequence(flow: FlowLike | undefined): EmbeddedFlowComponent[][] | null {
  if (!flow?.nodes?.length) {
    return null;
  }

  const start = flow.nodes.find((node) => node.type === FlowNodeType.START);
  const visited = new Set<string>();
  const screens: EmbeddedFlowComponent[][] = [];
  let currentId: string | undefined = start?.onSuccess;
  let iterations = 0;

  while (currentId && !visited.has(currentId) && iterations < MAX_STEPS) {
    iterations += 1;
    visited.add(currentId);
    const node: FlowNode | undefined = flow.nodes.find((candidate) => candidate.id === currentId);
    if (!node) {
      break;
    }
    if (node.type === FlowNodeType.PROMPT && node.meta?.components?.length) {
      screens.push(node.meta.components as EmbeddedFlowComponent[]);
    }
    currentId = nextNodeId(node);
  }

  if (screens.length > 0) {
    return screens;
  }

  // The START chain never reached a renderable prompt (e.g. a disconnected START) — fall back to
  // the first PROMPT node with components anywhere in the flow, same as the old single-screen
  // extractor did, so a malformed but otherwise valid flow still previews something.
  const anyPrompt = flow.nodes.find((node) => node.type === FlowNodeType.PROMPT && node.meta?.components?.length);
  return anyPrompt ? [anyPrompt.meta?.components as EmbeddedFlowComponent[]] : null;
}
