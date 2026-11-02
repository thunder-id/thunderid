// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import type {Context} from 'react';
import {createContext} from 'react';

/**
 * Props interface of {@link FlowEventsContext}
 *
 * Provides a typed, React-scoped event bus for cross-component communication
 * within the flow builder, replacing window-level CustomEvent dispatch/listen patterns.
 */
export interface FlowEventsContextProps {
  /**
   * Notify listeners that a flow element (step, template, widget) was added to the canvas.
   * @param type - The type of element added ('step' | 'template' | 'widget').
   */
  notifyElementAdded: (type: string) => void;
  /**
   * Register a handler for element-added events. Returns an unsubscribe function.
   */
  onElementAdded: (handler: (type: string) => void) => () => void;
  /**
   * Trigger auto-layout of the flow canvas.
   */
  triggerAutoLayout: () => void;
  /**
   * Register a handler for auto-layout events. Returns an unsubscribe function.
   */
  onAutoLayout: (handler: () => void) => () => void;
  /**
   * Restore nodes and edges from version history.
   */
  restoreFromHistory: (nodes: Node[], edges: Edge[]) => void;
  /**
   * Register a handler for restore-from-history events. Returns an unsubscribe function.
   */
  onRestoreFromHistory: (handler: (nodes: Node[], edges: Edge[]) => void) => () => void;
}

const FlowEventsContext: Context<FlowEventsContextProps | undefined> = createContext<
  FlowEventsContextProps | undefined
>(undefined);

FlowEventsContext.displayName = 'FlowEventsContext';

export default FlowEventsContext;
