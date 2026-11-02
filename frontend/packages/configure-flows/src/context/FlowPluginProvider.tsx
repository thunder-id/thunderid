// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type PropsWithChildren, type ReactElement, useCallback, useMemo, useRef} from 'react';
import FlowPluginContext, {
  type EdgeDeleteHandler,
  type ElementFilterHandler,
  type NodeDeleteHandler,
  type NodeElementDeleteHandler,
  type PropertyChangeHandler,
  type PropertyPanelOpenHandler,
  type TemplateLoadHandler,
} from './FlowPluginContext';

/**
 * Creates a ref-based plugin channel with subscribe and emit functions.
 * Follows the same pattern as FlowEventsProvider — handlers are stored in a
 * ref-backed Set so registration never causes re-renders.
 */
function usePluginChannel<T extends (...args: never[]) => boolean>() {
  const handlers = useRef(new Set<T>());

  const subscribe = useCallback((handler: T): (() => void) => {
    handlers.current.add(handler);
    return () => {
      handlers.current.delete(handler);
    };
  }, []);

  const emit = useCallback((...args: Parameters<T>): boolean => {
    if (handlers.current.size === 0) return true;
    return [...handlers.current].every((handler) => handler(...args));
  }, []);

  return {subscribe, emit};
}

/**
 * Provider for the flow plugin context.
 * Uses ref-based handler sets so callbacks are stable and never cause re-renders.
 *
 * Unlike FlowEventsProvider (fire-and-forget), plugin handlers return boolean values
 * that are aggregated with `.every()` to support interception semantics.
 */
function FlowPluginProvider({children}: PropsWithChildren): ReactElement {
  const propertyChange = usePluginChannel<PropertyChangeHandler>();
  const propertyPanelOpen = usePluginChannel<PropertyPanelOpenHandler>();
  const elementFilter = usePluginChannel<ElementFilterHandler>();
  const edgeDelete = usePluginChannel<EdgeDeleteHandler>();
  const nodeDelete = usePluginChannel<NodeDeleteHandler>();
  const nodeElementDelete = usePluginChannel<NodeElementDeleteHandler>();
  const templateLoad = usePluginChannel<TemplateLoadHandler>();

  const value = useMemo(
    () => ({
      onPropertyChange: propertyChange.subscribe,
      emitPropertyChange: propertyChange.emit,
      onPropertyPanelOpen: propertyPanelOpen.subscribe,
      emitPropertyPanelOpen: propertyPanelOpen.emit,
      onElementFilter: elementFilter.subscribe,
      emitElementFilter: elementFilter.emit,
      onEdgeDelete: edgeDelete.subscribe,
      emitEdgeDelete: edgeDelete.emit,
      onNodeDelete: nodeDelete.subscribe,
      emitNodeDelete: nodeDelete.emit,
      onNodeElementDelete: nodeElementDelete.subscribe,
      emitNodeElementDelete: nodeElementDelete.emit,
      onTemplateLoad: templateLoad.subscribe,
      emitTemplateLoad: templateLoad.emit,
    }),
    [
      propertyChange.subscribe,
      propertyChange.emit,
      propertyPanelOpen.subscribe,
      propertyPanelOpen.emit,
      elementFilter.subscribe,
      elementFilter.emit,
      edgeDelete.subscribe,
      edgeDelete.emit,
      nodeDelete.subscribe,
      nodeDelete.emit,
      nodeElementDelete.subscribe,
      nodeElementDelete.emit,
      templateLoad.subscribe,
      templateLoad.emit,
    ],
  );

  return <FlowPluginContext.Provider value={value}>{children}</FlowPluginContext.Provider>;
}

export default FlowPluginProvider;
