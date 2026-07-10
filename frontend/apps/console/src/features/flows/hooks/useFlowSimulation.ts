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
   * Follows a transition to its target node.
   */
  choose: (option: SimulationOption) => void;
  /**
   * Steps back to the previously visited node. No-op at the start node.
   */
  back: () => void;
  /**
   * Previews a transition's edge on the canvas (null clears the preview).
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

  const focusNode = useCallback(
    (nodeId: string): void => {
      if (!followCameraRef.current) {
        return;
      }
      fitView({...FOCUS_OPTIONS, nodes: [{id: nodeId}]}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    },
    [fitView],
  );

  const start = useCallback((): void => {
    const startNode = nodesRef.current.find((node: Node) => node.type === StaticStepTypes.Start) ?? nodesRef.current[0];

    if (!startNode) {
      return;
    }

    setIsSimulating(true);
    setPathNodeIds([startNode.id]);
    setPathEdges([]);
    setPreviewedOption(null);
    focusNode(startNode.id);
  }, [focusNode]);

  const choose = useCallback(
    (option: SimulationOption): void => {
      setPathNodeIds((prev: string[]) => [...prev, option.targetNodeId]);
      setPathEdges((prev: TraversedEdge[]) => [...prev, {edgeId: option.edgeId, kind: option.kind}]);
      setPreviewedOption(null);
      focusNode(option.targetNodeId);
    },
    [focusNode],
  );

  const back = useCallback((): void => {
    if (pathNodeIds.length <= 1) {
      return;
    }
    const nextPath = pathNodeIds.slice(0, -1);
    setPathNodeIds(nextPath);
    setPathEdges((prev: TraversedEdge[]) => prev.slice(0, -1));
    setPreviewedOption(null);
    focusNode(nextPath[nextPath.length - 1]);
  }, [pathNodeIds, focusNode]);

  const preview = useCallback((option: SimulationOption | null): void => {
    setPreviewedOption(option);
  }, []);

  const stop = useCallback((): void => {
    setIsSimulating(false);
    setPathNodeIds([]);
    setPathEdges([]);
    setPreviewedOption(null);
    if (followCameraRef.current) {
      fitView({duration: 500, padding: 0.2}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    }
  }, [fitView]);

  const options: SimulationOption[] = useMemo(
    () => (currentNodeId ? getSimulationOptions(currentNodeId, nodes, edges) : NO_OPTIONS),
    [currentNodeId, nodes, edges],
  );

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
      choose,
      back,
      preview,
      stop,
    ],
  );
}

export default useFlowSimulation;
