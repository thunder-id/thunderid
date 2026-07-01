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

import type {EmbeddedFlowComponent} from '@thunderid/react';
import {FlowNodeType} from '../models/flows';
import type {FlowDefinitionResponse} from '../models/responses';

/**
 * Extracts the UI components of a flow's first sign-in screen.
 *
 * Walks the flow graph from the START node, following `onSuccess` through
 * non-prompt nodes (e.g. TASK_EXECUTION), until it reaches the first PROMPT node
 * that has renderable components. The returned components are in the same shape
 * consumed by the embedded flow renderer, so they can be passed directly to
 * {@link GatePreview}'s `mock` prop.
 *
 * @param flow - The full flow definition, or undefined while loading
 * @returns The first PROMPT screen's components, or null if the flow has no
 *   renderable prompt screen
 *
 * @public
 */
export default function getFlowEntryComponents(
  flow: FlowDefinitionResponse | undefined,
): EmbeddedFlowComponent[] | null {
  if (!flow?.nodes?.length) {
    return null;
  }

  const start = flow.nodes.find((node) => node.type === FlowNodeType.START);
  const visited = new Set<string>();
  let currentId: string | undefined = start?.onSuccess;

  while (currentId && !visited.has(currentId)) {
    visited.add(currentId);
    const node = flow.nodes.find((candidate) => candidate.id === currentId);
    if (!node) {
      break;
    }
    if (node.type === FlowNodeType.PROMPT && node.meta?.components?.length) {
      return node.meta.components as EmbeddedFlowComponent[];
    }
    currentId = node.onSuccess;
  }

  const anyPrompt = flow.nodes.find((node) => node.type === FlowNodeType.PROMPT && node.meta?.components?.length);

  return (anyPrompt?.meta?.components as EmbeddedFlowComponent[]) ?? null;
}
