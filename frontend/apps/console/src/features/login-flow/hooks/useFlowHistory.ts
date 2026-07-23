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
import cloneDeep from 'lodash-es/cloneDeep';
import type {Dispatch, SetStateAction} from 'react';
import {useCallback, useEffect, useRef, useState} from 'react';
import type {StepData} from '@/features/flows/models/steps';

/**
 * A point-in-time copy of the canvas graph.
 */
interface Snapshot {
  nodes: Node[];
  edges: Edge[];
}

export interface UseFlowHistoryProps {
  /** Current canvas nodes (the full, unfiltered set). */
  nodes: Node[];
  /** Current canvas edges (the full, unfiltered set). */
  edges: Edge[];
  /** Setter for canvas nodes. */
  setNodes: Dispatch<SetStateAction<Node[]>>;
  /** Setter for canvas edges. */
  setEdges: Dispatch<SetStateAction<Edge[]>>;
  /** Maximum number of undo steps to retain. */
  maxHistoryItems: number;
  /** Debounce window (ms) for coalescing rapid edits into one history entry. */
  debounceMs?: number;
}

export interface UseFlowHistoryReturn {
  undo: () => void;
  redo: () => void;
  canUndo: boolean;
  canRedo: boolean;
  /**
   * Signature of the last settled (committed) graph state. Updates after edits
   * settle, after undo/redo, and after {@link UseFlowHistoryReturn.resetHistory}
   * — cheap to compare for dirty tracking without re-serializing per render.
   */
  settledSignature: string | null;
  /**
   * Discard all history and rebase the baseline on the current graph.
   * Returns the signature of that baseline.
   */
  resetHistory: () => string;
}

/**
 * Builds a structural signature of the graph. Captures everything an edit can
 * change (ids, types, executors, positions, edge endpoints, node properties and
 * components) so equal graphs share a signature and drag ticks that settle to
 * the same layout do not record spurious history.
 */
export function computeGraphSignature(nodes: Node[], edges: Edge[]): string {
  let signature = '';
  for (const node of nodes) {
    const data = node.data as StepData | undefined;
    signature +=
      `${node.id}|${node.type ?? ''}|${Math.round(node.position?.x ?? 0)},${Math.round(node.position?.y ?? 0)}|` +
      `${data?.action?.executor?.name ?? ''}|${JSON.stringify(data?.properties ?? null)}|` +
      `${JSON.stringify(data?.components ?? null)};`;
  }
  signature += '::';
  for (const edge of edges) {
    signature += `${edge.id}|${edge.source}|${edge.sourceHandle ?? ''}|${edge.target}|${edge.targetHandle ?? ''};`;
  }
  return signature;
}

/**
 * Snapshot-based undo/redo for the flow builder canvas.
 *
 * Because graph mutations are scattered across many hooks and go through both
 * `setNodes`/`setEdges` and React Flow's internal store (which syncs back into
 * the controlled arrays), history is observed at the state owner rather than at
 * each call site: a debounced effect commits the previous graph to the undo
 * stack whenever the graph settles into a new structural state.
 *
 * The stacks live in refs and are mutated synchronously from event handlers and
 * the debounce timer, so rapid undo/redo (held shortcut, double clicks) never
 * works off stale state; `canUndo`/`canRedo` are mirrored into state for the
 * UI. Undo/redo first flush any pending (not yet debounced) edit so they always
 * operate on exactly what the user sees.
 */
const useFlowHistory = ({
  nodes,
  edges,
  setNodes,
  setEdges,
  maxHistoryItems,
  debounceMs = 400,
}: UseFlowHistoryProps): UseFlowHistoryReturn => {
  const pastRef = useRef<Snapshot[]>([]);
  const futureRef = useRef<Snapshot[]>([]);

  const [availability, setAvailability] = useState<{canUndo: boolean; canRedo: boolean}>({
    canRedo: false,
    canUndo: false,
  });
  const [settledSignature, setSettledSignature] = useState<string | null>(null);

  // The last graph state recorded as a committed baseline.
  const committedRef = useRef<Snapshot>({edges, nodes});
  const committedSignatureRef = useRef<string | null>(null);

  // Guards the observer so an undo/redo (which calls setNodes/setEdges) does not
  // record its own restore as a new edit, and undo/redo don't re-enter while a
  // restore is still being observed.
  const isApplyingRef = useRef<boolean>(false);
  // The first observed state (initial mount / flow load) becomes the baseline.
  const isInitializedRef = useRef<boolean>(false);

  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const syncAvailability = useCallback(() => {
    setAvailability((previous) => {
      const next = {canRedo: futureRef.current.length > 0, canUndo: pastRef.current.length > 0};
      return previous.canRedo === next.canRedo && previous.canUndo === next.canUndo ? previous : next;
    });
  }, []);

  /**
   * Commits the given graph as the new baseline, pushing the previous baseline
   * to the undo stack. Returns whether anything was committed.
   */
  const commitIfChanged = useCallback(
    (nextNodes: Node[], nextEdges: Edge[]): boolean => {
      const nextSignature = computeGraphSignature(nextNodes, nextEdges);

      if (!isInitializedRef.current) {
        // First settle after load — establish the baseline, record nothing.
        isInitializedRef.current = true;
        committedRef.current = {edges: cloneDeep(nextEdges), nodes: cloneDeep(nextNodes)};
        committedSignatureRef.current = nextSignature;
        setSettledSignature(nextSignature);
        return false;
      }

      if (nextSignature === committedSignatureRef.current) {
        return false;
      }

      pastRef.current = [...pastRef.current, committedRef.current].slice(-maxHistoryItems);
      futureRef.current = [];
      committedRef.current = {edges: cloneDeep(nextEdges), nodes: cloneDeep(nextNodes)};
      committedSignatureRef.current = nextSignature;
      setSettledSignature(nextSignature);
      syncAvailability();
      return true;
    },
    [maxHistoryItems, syncAvailability],
  );

  /**
   * Cancels the debounce and commits any in-flight edit immediately, so
   * undo/redo act on the graph the user currently sees.
   */
  const flushPending = useCallback(
    (currentNodes: Node[], currentEdges: Edge[]): boolean => {
      if (debounceTimerRef.current !== null) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
      return commitIfChanged(currentNodes, currentEdges);
    },
    [commitIfChanged],
  );

  // Observe graph changes and, once settled, commit the prior state to history.
  useEffect(() => {
    if (isApplyingRef.current) {
      return undefined;
    }

    if (debounceTimerRef.current !== null) {
      clearTimeout(debounceTimerRef.current);
    }

    debounceTimerRef.current = setTimeout(() => {
      debounceTimerRef.current = null;
      commitIfChanged(nodes, edges);
    }, debounceMs);

    return () => {
      if (debounceTimerRef.current !== null) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, [nodes, edges, debounceMs, commitIfChanged]);

  const apply = useCallback(
    (snapshot: Snapshot) => {
      isApplyingRef.current = true;
      committedRef.current = snapshot;
      const signature = computeGraphSignature(snapshot.nodes, snapshot.edges);
      committedSignatureRef.current = signature;
      setSettledSignature(signature);
      setNodes(cloneDeep(snapshot.nodes));
      setEdges(cloneDeep(snapshot.edges));
      // Release the guard after the resulting state change has been observed.
      requestAnimationFrame(() => {
        isApplyingRef.current = false;
      });
    },
    [setNodes, setEdges],
  );

  const undo = useCallback(() => {
    if (isApplyingRef.current) {
      return;
    }
    // An edit still inside the debounce window becomes the top history entry,
    // so this undo reverts exactly what is on screen.
    flushPending(nodes, edges);
    if (pastRef.current.length === 0) {
      return;
    }
    const previous = pastRef.current[pastRef.current.length - 1];
    pastRef.current = pastRef.current.slice(0, -1);
    futureRef.current = [committedRef.current, ...futureRef.current];
    apply(previous);
    syncAvailability();
  }, [nodes, edges, flushPending, apply, syncAvailability]);

  const redo = useCallback(() => {
    if (isApplyingRef.current) {
      return;
    }
    // A pending edit invalidates the redo branch (same as a committed edit).
    if (flushPending(nodes, edges)) {
      return;
    }
    if (futureRef.current.length === 0) {
      return;
    }
    const [next, ...rest] = futureRef.current;
    futureRef.current = rest;
    pastRef.current = [...pastRef.current, committedRef.current];
    apply(next);
    syncAvailability();
  }, [nodes, edges, flushPending, apply, syncAvailability]);

  const resetHistory = useCallback((): string => {
    if (debounceTimerRef.current !== null) {
      clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }
    pastRef.current = [];
    futureRef.current = [];
    const signature = computeGraphSignature(nodes, edges);
    committedRef.current = {edges: cloneDeep(edges), nodes: cloneDeep(nodes)};
    committedSignatureRef.current = signature;
    isInitializedRef.current = true;
    setSettledSignature(signature);
    syncAvailability();
    return signature;
  }, [nodes, edges, syncAvailability]);

  return {
    canRedo: availability.canRedo,
    canUndo: availability.canUndo,
    redo,
    resetHistory,
    settledSignature,
    undo,
  };
};

export default useFlowHistory;
