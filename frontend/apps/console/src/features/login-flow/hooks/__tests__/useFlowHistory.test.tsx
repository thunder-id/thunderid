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

import {act, renderHook} from '@testing-library/react';
import type {Edge, Node} from '@xyflow/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useFlowHistory from '../useFlowHistory';

const DEBOUNCE = 400;

function node(id: string, x = 0, y = 0): Node {
  return {data: {}, id, position: {x, y}, type: 'VIEW'};
}

function edge(id: string, source: string, target: string): Edge {
  return {id, source, target};
}

/**
 * Drives the hook like the state owner does: an external nodes/edges store the
 * test mutates, re-rendering the hook with the new arrays (mirroring
 * useNodesState/useEdgesState in LoginFlowBuilder).
 */
function setupHistory(initialNodes: Node[], initialEdges: Edge[]) {
  let currentNodes = initialNodes;
  let currentEdges = initialEdges;

  const setNodes = vi.fn((update: Node[] | ((prev: Node[]) => Node[])) => {
    currentNodes = typeof update === 'function' ? (update as (p: Node[]) => Node[])(currentNodes) : update;
  });
  const setEdges = vi.fn((update: Edge[] | ((prev: Edge[]) => Edge[])) => {
    currentEdges = typeof update === 'function' ? (update as (p: Edge[]) => Edge[])(currentEdges) : update;
  });

  const view = renderHook(
    ({nodes, edges}: {nodes: Node[]; edges: Edge[]}) =>
      useFlowHistory({edges, maxHistoryItems: 20, nodes, setEdges, setNodes}),
    {initialProps: {edges: initialEdges, nodes: initialNodes}},
  );

  const settle = (nodes: Node[], edges: Edge[]): void => {
    currentNodes = nodes;
    currentEdges = edges;
    view.rerender({edges, nodes});
    act(() => {
      vi.advanceTimersByTime(DEBOUNCE);
    });
  };

  const flushApply = (): void => {
    // apply() releases its guard in a requestAnimationFrame; then re-render with
    // the restored arrays so the observer sees the settled (guarded-past) state.
    act(() => {
      vi.advanceTimersByTime(16);
    });
    view.rerender({edges: currentEdges, nodes: currentNodes});
    act(() => {
      vi.advanceTimersByTime(DEBOUNCE);
    });
  };

  return {flushApply, getEdges: () => currentEdges, getNodes: () => currentNodes, settle, view};
}

describe('useFlowHistory', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal(
      'requestAnimationFrame',
      (cb: FrameRequestCallback) => setTimeout(() => cb(0), 0) as unknown as number,
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('starts with empty undo/redo stacks', () => {
    const {view} = setupHistory([node('a')], []);
    expect(view.result.current.canUndo).toBe(false);
    expect(view.result.current.canRedo).toBe(false);
  });

  it('does not record the initial load as a history entry', () => {
    const {view, settle} = setupHistory([node('a')], []);
    // First settle establishes the baseline only.
    settle([node('a')], []);
    expect(view.result.current.canUndo).toBe(false);
  });

  it('records a committed edit and can undo it', () => {
    const {view, settle} = setupHistory([node('a')], []);
    settle([node('a')], []); // baseline
    settle([node('a'), node('b')], []); // add node b

    expect(view.result.current.canUndo).toBe(true);
    expect(view.result.current.canRedo).toBe(false);
  });

  it('undo restores the previous graph and enables redo', () => {
    const {view, settle, getNodes} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);

    act(() => {
      view.result.current.undo();
    });

    expect(getNodes().map((n) => n.id)).toEqual(['a']);
    expect(view.result.current.canRedo).toBe(true);
  });

  it('redo re-applies an undone edit', () => {
    const {view, settle, getNodes, flushApply} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);

    act(() => {
      view.result.current.undo();
    });
    flushApply();

    act(() => {
      view.result.current.redo();
    });
    expect(getNodes().map((n) => n.id)).toEqual(['a', 'b']);
  });

  it('clears the redo stack when a new edit is made after an undo', () => {
    const {view, settle, flushApply} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);

    act(() => {
      view.result.current.undo();
    });
    flushApply();
    expect(view.result.current.canRedo).toBe(true);

    settle([node('a'), node('c')], []); // divergent new edit
    expect(view.result.current.canRedo).toBe(false);
  });

  it('coalesces rapid changes within the debounce window into one entry', () => {
    const {view, settle} = setupHistory([node('a')], []);
    settle([node('a')], []); // baseline

    // Three rapid re-renders inside one debounce window -> single commit.
    view.rerender({edges: [], nodes: [node('a', 1, 0)]});
    view.rerender({edges: [], nodes: [node('a', 2, 0)]});
    view.rerender({edges: [], nodes: [node('a', 3, 0)]});
    act(() => {
      vi.advanceTimersByTime(DEBOUNCE);
    });

    expect(view.result.current.canUndo).toBe(true);
    act(() => {
      view.result.current.undo();
    });
    // Only one undo returns to baseline.
    expect(view.result.current.canUndo).toBe(false);
  });

  it('caps the undo stack at maxHistoryItems', () => {
    let current: Node[] = [node('n0')];
    const setNodes = vi.fn((update: Node[] | ((prev: Node[]) => Node[])) => {
      current = typeof update === 'function' ? (update as (p: Node[]) => Node[])(current) : update;
    });
    const setEdges = vi.fn();
    const view = renderHook(
      ({nodes}: {nodes: Node[]}) => useFlowHistory({edges: [], maxHistoryItems: 3, nodes, setEdges, setNodes}),
      {initialProps: {nodes: [node('n0')]}},
    );

    const commit = (nodes: Node[]): void => {
      current = nodes;
      view.rerender({nodes});
      act(() => {
        vi.advanceTimersByTime(DEBOUNCE);
      });
    };

    commit([node('n0')]); // baseline
    for (let i = 1; i <= 6; i += 1) {
      commit([node(`n${i}`)]);
    }

    // At most 3 undos are retained.
    let undoCount = 0;
    while (view.result.current.canUndo && undoCount < 10) {
      act(() => {
        view.result.current.undo();
      });
      act(() => {
        vi.advanceTimersByTime(16);
      });
      // Reflect the restored graph back into the hook, as the state owner does.
      view.rerender({nodes: current});
      undoCount += 1;
    }
    expect(undoCount).toBe(3);
  });

  it('resetHistory clears both stacks', () => {
    const {view, settle} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);
    expect(view.result.current.canUndo).toBe(true);

    act(() => {
      view.result.current.resetHistory();
    });
    expect(view.result.current.canUndo).toBe(false);
    expect(view.result.current.canRedo).toBe(false);
  });

  it('records edge changes', () => {
    const {view, settle} = setupHistory([node('a'), node('b')], []);
    settle([node('a'), node('b')], []);
    settle([node('a'), node('b')], [edge('e1', 'a', 'b')]);
    expect(view.result.current.canUndo).toBe(true);
  });

  it('undo within the debounce window reverts exactly the in-flight edit', () => {
    const {view, settle, getNodes} = setupHistory([node('a')], []);
    settle([node('a')], []); // baseline

    // Edit that has NOT yet passed the debounce window.
    view.rerender({edges: [], nodes: [node('a'), node('b')]});

    act(() => {
      view.result.current.undo();
    });

    // The pending edit is flushed into history first, so undo restores the
    // graph the user was looking at just before the edit — not two steps back.
    expect(getNodes().map((n) => n.id)).toEqual(['a']);
    expect(view.result.current.canRedo).toBe(true);
  });

  it('a pending edit invalidates the redo branch', () => {
    const {view, settle, flushApply} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);

    act(() => {
      view.result.current.undo();
    });
    flushApply();
    expect(view.result.current.canRedo).toBe(true);

    // New divergent edit, still inside the debounce window.
    view.rerender({edges: [], nodes: [node('a'), node('c')]});

    act(() => {
      view.result.current.redo();
    });

    // The flush committed the divergent edit and cleared the redo stack; the
    // redo itself became a no-op instead of applying a stale future state.
    expect(view.result.current.canRedo).toBe(false);
  });

  it('ignores a second undo while the first restore is still applying', () => {
    const {view, settle, getNodes} = setupHistory([node('a')], []);
    settle([node('a')], []);
    settle([node('a'), node('b')], []);
    settle([node('a'), node('b'), node('c')], []);

    act(() => {
      view.result.current.undo();
      // Second call lands before the restore has been observed (rAF pending).
      view.result.current.undo();
    });

    // Only one step back, not two.
    expect(getNodes().map((n) => n.id)).toEqual(['a', 'b']);
  });

  it('exposes the settled signature and returns it from resetHistory', () => {
    const {view, settle} = setupHistory([node('a')], []);
    expect(view.result.current.settledSignature).toBeNull();

    settle([node('a')], []); // baseline
    const baselineSignature = view.result.current.settledSignature;
    expect(baselineSignature).not.toBeNull();

    settle([node('a'), node('b')], []);
    expect(view.result.current.settledSignature).not.toBe(baselineSignature);

    let returned = '';
    act(() => {
      returned = view.result.current.resetHistory();
    });
    expect(returned).toBe(view.result.current.settledSignature);
  });
});
