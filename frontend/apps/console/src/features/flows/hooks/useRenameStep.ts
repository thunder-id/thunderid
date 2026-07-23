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

import {useReactFlow, type Edge, type Node} from '@xyflow/react';
import {useCallback} from 'react';
import useInteractionState from './useInteractionState';
import type {Resource} from '../models/resources';

/**
 * Node ids live in the flow definition and its references — keep them to the
 * charset the seeded flows use.
 */
const NODE_ID_PATTERN = /^[a-zA-Z0-9_-]+$/;

/**
 * Return type of {@link useRenameStep}.
 */
export interface UseRenameStep {
  /**
   * Whether `candidate` is an acceptable new id for the node currently identified
   * by `currentId` (well-formed and not taken by another node).
   */
  isValidStepId: (candidate: string, currentId: string) => boolean;
  /**
   * Renames the node and rewires everything that references the old id. Returns
   * whether the rename was applied (false when the candidate is invalid).
   */
  renameStep: (currentId: string, nextId: string) => boolean;
}

/**
 * Shared step-rename behavior used by every surface that can edit a step id (the
 * node header title and the properties panel Id field): validates the candidate,
 * rewires edge endpoints and id-prefixed source handles (`{id}_NEXT`,
 * `{id}_INCOMPLETE`), renames the node, and keeps the interaction state (selected
 * resource and step id) pointing at the renamed node so both surfaces stay in sync.
 *
 * @returns The rename handlers.
 */
const useRenameStep = (): UseRenameStep => {
  const {getNodes, setNodes, setEdges} = useReactFlow();
  const {lastInteractedResource, lastInteractedStepId, setLastInteractedResource, setLastInteractedStepId} =
    useInteractionState();

  const isValidStepId = useCallback(
    (candidate: string, currentId: string): boolean =>
      NODE_ID_PATTERN.test(candidate) &&
      !getNodes().some((node: Node) => node.id === candidate && node.id !== currentId),
    [getNodes],
  );

  const renameStep = useCallback(
    (currentId: string, nextId: string): boolean => {
      const next = nextId.trim();
      if (next === currentId) {
        return true;
      }
      if (next === '' || !isValidStepId(next, currentId)) {
        return false;
      }

      setEdges((edges: Edge[]) =>
        edges.map((edge: Edge) => ({
          ...edge,
          source: edge.source === currentId ? next : edge.source,
          target: edge.target === currentId ? next : edge.target,
          sourceHandle: edge.sourceHandle?.startsWith(`${currentId}_`)
            ? edge.sourceHandle.replace(`${currentId}_`, `${next}_`)
            : edge.sourceHandle,
        })),
      );
      setNodes((nodes: Node[]) => nodes.map((node: Node) => (node.id === currentId ? {...node, id: next} : node)));

      // Keep the properties panel (and anything else reading the interaction
      // state) pointed at the renamed node without toggling the panel open.
      if (lastInteractedStepId === currentId) {
        setLastInteractedStepId(next);
      }
      if ((lastInteractedResource as Resource | undefined)?.id === currentId) {
        setLastInteractedResource({...(lastInteractedResource as Resource), id: next}, false);
      }

      return true;
    },
    [
      isValidStepId,
      setEdges,
      setNodes,
      lastInteractedResource,
      lastInteractedStepId,
      setLastInteractedResource,
      setLastInteractedStepId,
    ],
  );

  return {isValidStepId, renameStep};
};

export default useRenameStep;
