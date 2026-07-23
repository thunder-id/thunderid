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
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {StaticStepTypes} from '../models/steps';
import getSimulationOptions, {type SimulationOption, type SimulationOptionKinds} from '../utils/getSimulationOptions';

function areOptionsEqual(a: SimulationOption[], b: SimulationOption[]): boolean {
  return (
    a.length === b.length &&
    a.every((option: SimulationOption, index: number) => {
      const other = b[index];
      return (
        option.edgeId === other.edgeId &&
        option.targetNodeId === other.targetNodeId &&
        option.kind === other.kind &&
        option.actionLabel === other.actionLabel &&
        option.sourceComponentId === other.sourceComponentId
      );
    })
  );
}

/**
 * An edge traversed during the simulation, together with the kind of transition
 * it represented (used for color coding).
 */
export interface TraversedEdge {
  edgeId: string;
  kind: SimulationOptionKinds;
}

/**
 * State and actions for simulating a flow by walking its graph step by step.
 */
export interface FlowSimulation {
  /**
   * Whether a simulation is in progress.
   */
  isSimulating: boolean;
  /**
   * Ids of the nodes visited so far, in order.
   */
  pathNodeIds: string[];
  /**
   * Edges traversed so far, in order, with their transition kinds.
   */
  pathEdges: TraversedEdge[];
  /**
   * Id of the current node, or null when not simulating.
   */
  currentNodeId: string | null;
  /**
   * Outgoing transitions available from the current node.
   */
  options: SimulationOption[];
  /**
   * Transition currently hovered in the panel, previewed on the canvas.
   */
  previewedOption: SimulationOption | null;
  /**
   * Starts (or restarts) the simulation from the start node.
   */
  start: () => void;
  /**
   * Starts (or restarts) the simulation focused directly at the given node,
   * e.g. to preview a single screen. No-op when the node does not exist.
   */
  startAt: (nodeId: string) => void;
  /**
   * Follows a transition to its target node.
   */
  choose: (option: SimulationOption) => void;
  /**
   * Steps back to the previously visited node. No-op at the start node.
   */
  back: () => void;
  /**
   * Previews a transition's edge on the canvas (null clears the preview). While
   * the camera follows the flow, previewing also brings the transition's target
   * into view alongside the current step.
   */
  preview: (option: SimulationOption | null) => void;
  /**
   * Whether the canvas camera follows the current step. When false the viewport
   * stays put, giving a static view of the flow.
   */
  followCamera: boolean;
  /**
   * Toggles whether the canvas camera follows the current step.
   */
  toggleFollowCamera: () => void;
  /**
   * Exits the simulation.
   */
  stop: () => void;
}

const FOCUS_OPTIONS = {duration: 500, maxZoom: 1.2, padding: 0.3};

// Grace period before the camera zooms back after a hover preview ends, so
// sweeping across the options list reads as one motion instead of the camera
// bouncing between every row.
const PREVIEW_RESTORE_DELAY_MS = 200;

const NO_OPTIONS: SimulationOption[] = [];

/**
 * Hook that drives the flow simulation mode: tracks the walked path, exposes the
 * available transitions of the current node, and keeps the viewport focused on the
 * current step.
 *
 * @param nodes - Nodes on the canvas.
 * @param edges - Edges on the canvas.
 * @returns The simulation state and actions.
 */
function useFlowSimulation(nodes: Node[], edges: Edge[]): FlowSimulation {
  const {fitView} = useReactFlow();
  const [pathNodeIds, setPathNodeIds] = useState<string[]>([]);
  const [pathEdges, setPathEdges] = useState<TraversedEdge[]>([]);
  const [previewedOption, setPreviewedOption] = useState<SimulationOption | null>(null);
  const [isSimulating, setIsSimulating] = useState<boolean>(false);
  const [followCamera, setFollowCamera] = useState<boolean>(true);

  // Read through a ref so the navigation callbacks stay referentially stable.
  const followCameraRef = useRef<boolean>(true);

  const toggleFollowCamera = useCallback((): void => {
    followCameraRef.current = !followCameraRef.current;
    setFollowCamera(followCameraRef.current);
  }, []);

  // Read through a ref so `start` stays referentially stable across node drag
  // ticks — the returned object feeds memoized right-panel subtrees.
  const nodesRef = useRef<Node[]>(nodes);
  useEffect(() => {
    nodesRef.current = nodes;
  }, [nodes]);

  const currentNodeId: string | null = isSimulating ? (pathNodeIds[pathNodeIds.length - 1] ?? null) : null;

  // Read through a ref so `preview` stays referentially stable while still
  // seeing the step that is current at hover time.
  const currentNodeIdRef = useRef<string | null>(currentNodeId);
  useEffect(() => {
    currentNodeIdRef.current = currentNodeId;
  }, [currentNodeId]);

  const restoreTimerRef = useRef<number | null>(null);
  const hasActivePreviewRef = useRef<boolean>(false);

  const cancelPendingRestore = useCallback((): void => {
    if (restoreTimerRef.current !== null) {
      window.clearTimeout(restoreTimerRef.current);
      restoreTimerRef.current = null;
    }
  }, []);

  useEffect(() => cancelPendingRestore, [cancelPendingRestore]);

  const clearPreview = useCallback((): void => {
    cancelPendingRestore();
    hasActivePreviewRef.current = false;
    setPreviewedOption(null);
  }, [cancelPendingRestore]);

  const focusNode = useCallback(
    (nodeId: string): void => {
      cancelPendingRestore();
      if (!followCameraRef.current) {
        return;
      }
      fitView({...FOCUS_OPTIONS, nodes: [{id: nodeId}]}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    },
    [fitView, cancelPendingRestore],
  );

  const start = useCallback((): void => {
    const startNode = nodesRef.current.find((node: Node) => node.type === StaticStepTypes.Start) ?? nodesRef.current[0];

    if (!startNode) {
      return;
    }

    setIsSimulating(true);
    setPathNodeIds([startNode.id]);
    setPathEdges([]);
    clearPreview();
    focusNode(startNode.id);
  }, [focusNode, clearPreview]);

  const startAt = useCallback(
    (nodeId: string): void => {
      if (!nodesRef.current.some((node: Node) => node.id === nodeId)) {
        return;
      }
      setIsSimulating(true);
      setPathNodeIds([nodeId]);
      setPathEdges([]);
      clearPreview();
      focusNode(nodeId);
    },
    [focusNode, clearPreview],
  );

  const choose = useCallback(
    (option: SimulationOption): void => {
      setPathNodeIds((prev: string[]) => [...prev, option.targetNodeId]);
      setPathEdges((prev: TraversedEdge[]) => [...prev, {edgeId: option.edgeId, kind: option.kind}]);
      clearPreview();
      focusNode(option.targetNodeId);
    },
    [focusNode, clearPreview],
  );

  const back = useCallback((): void => {
    if (pathNodeIds.length <= 1) {
      return;
    }
    const nextPath = pathNodeIds.slice(0, -1);
    setPathNodeIds(nextPath);
    setPathEdges((prev: TraversedEdge[]) => prev.slice(0, -1));
    clearPreview();
    focusNode(nextPath[nextPath.length - 1]);
  }, [pathNodeIds, focusNode, clearPreview]);

  const preview = useCallback(
    (option: SimulationOption | null): void => {
      setPreviewedOption(option);
      if (!followCameraRef.current) {
        hasActivePreviewRef.current = Boolean(option);
        return;
      }
      cancelPendingRestore();
      const currentId = currentNodeIdRef.current;
      if (option) {
        hasActivePreviewRef.current = true;
        // Bring the hovered option's target into view together with the current
        // step, so the previewed transition is fully visible.
        const ids =
          currentId && currentId !== option.targetNodeId ? [currentId, option.targetNodeId] : [option.targetNodeId];
        fitView({...FOCUS_OPTIONS, duration: 400, nodes: ids.map((id) => ({id}))}).catch(() => {
          // Ignore fitView errors - focusing is best-effort
        });
      } else if (hasActivePreviewRef.current) {
        hasActivePreviewRef.current = false;
        if (!currentId) {
          return;
        }
        // Hover ended - restore the single-step focus after a short grace
        // period, so moving on to the next option does not bounce the camera.
        restoreTimerRef.current = window.setTimeout(() => {
          restoreTimerRef.current = null;
          // Re-checked at fire time: the user may have switched to the static
          // view while the restore was pending.
          if (!followCameraRef.current) {
            return;
          }
          fitView({...FOCUS_OPTIONS, nodes: [{id: currentId}]}).catch(() => {
            // Ignore fitView errors - focusing is best-effort
          });
        }, PREVIEW_RESTORE_DELAY_MS);
      }
    },
    [fitView, cancelPendingRestore],
  );

  const stop = useCallback((): void => {
    setIsSimulating(false);
    setPathNodeIds([]);
    setPathEdges([]);
    clearPreview();
    if (followCameraRef.current) {
      fitView({duration: 500, padding: 0.2}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    }
  }, [fitView, clearPreview]);

  const computedOptions: SimulationOption[] = useMemo(
    () => (currentNodeId ? getSimulationOptions(currentNodeId, nodes, edges) : NO_OPTIONS),
    [currentNodeId, nodes, edges],
  );

  // Node drags replace the `nodes` array every tick, recomputing an equal option
  // list with fresh identity. Keep the previous array while contents are unchanged
  // so the aggregate simulation object (and the memoized right-panel subtrees it
  // feeds) stays referentially stable across drag ticks.
  const [options, setOptions] = useState<SimulationOption[]>(computedOptions);
  if (options !== computedOptions && !areOptionsEqual(options, computedOptions)) {
    setOptions(computedOptions);
  }

  // The canvas stays editable while simulating — if the current node is deleted,
  // there is nothing left to preview, so exit instead of stranding a dimmed
  // canvas with no panel. Adjusted during render (guarded) rather than in an
  // effect so the dimmed frame never commits.
  if (isSimulating && currentNodeId && !nodes.some((node: Node) => node.id === currentNodeId)) {
    setIsSimulating(false);
    setPathNodeIds([]);
    setPathEdges([]);
    setPreviewedOption(null);
  }

  return useMemo(
    () => ({
      isSimulating,
      pathNodeIds,
      pathEdges,
      currentNodeId,
      options,
      previewedOption,
      followCamera,
      toggleFollowCamera,
      start,
      startAt,
      choose,
      back,
      preview,
      stop,
    }),
    [
      isSimulating,
      pathNodeIds,
      pathEdges,
      currentNodeId,
      options,
      previewedOption,
      followCamera,
      toggleFollowCamera,
      start,
      startAt,
      choose,
      back,
      preview,
      stop,
    ],
  );
}

export default useFlowSimulation;
