// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CollisionPriority} from '@dnd-kit/abstract';
import {RestrictToVerticalAxis} from '@dnd-kit/abstract/modifiers';
import {pointerIntersection} from '@dnd-kit/collision';
import {useDragDropManager, useDragOperation} from '@dnd-kit/react';
import {type UseSortableInput, useSortable} from '@dnd-kit/react/sortable';
import {Box, type CSSProperties} from '@wso2/oxygen-ui';
import {
  memo,
  type PropsWithChildren,
  type ReactElement,
  type ReactNode,
  type RefObject,
  useMemo,
  useSyncExternalStore,
} from 'react';

interface DragOperationState {
  isDragging: boolean;
  sourceIndex: number | undefined;
  isReordering: boolean;
}

let globalDragOperationState: DragOperationState = {
  isDragging: false,
  sourceIndex: undefined,
  isReordering: false,
};
const dragOperationListeners = new Set<() => void>();

function setGlobalDragOperationState(state: DragOperationState): void {
  // Only notify if state actually changed
  if (
    globalDragOperationState.isDragging !== state.isDragging ||
    globalDragOperationState.sourceIndex !== state.sourceIndex ||
    globalDragOperationState.isReordering !== state.isReordering
  ) {
    globalDragOperationState = state;
    dragOperationListeners.forEach((listener) => listener());
  }
}

function subscribeToDragOperation(callback: () => void): () => void {
  dragOperationListeners.add(callback);
  return () => dragOperationListeners.delete(callback);
}

function getDragOperationState(): DragOperationState {
  return globalDragOperationState;
}

/**
 * Hook to subscribe to global drag operation state with minimal re-renders.
 */
function useGlobalDragOperationState(): DragOperationState {
  return useSyncExternalStore(subscribeToDragOperation, getDragOperationState, getDragOperationState);
}

// Use WeakMap keyed by manager instance to track setup
// This ensures listeners are added only once per manager, and handles HMR correctly
const sortableSetupManagersMap = new WeakMap<object, boolean>();

/**
 * Hook to set up drag operation monitoring once per manager instance.
 */
function useDragOperationMonitorSetup(): void {
  const manager = useDragDropManager();

  // Skip if no manager or already set up for this manager instance
  if (!manager || sortableSetupManagersMap.has(manager)) return;

  // Mark this manager as set up
  sortableSetupManagersMap.set(manager, true);

  manager.monitor.addEventListener('dragstart', (event) => {
    const {source} = event.operation;
    const sourceIndex = (source as {index?: number} | undefined)?.index;
    const isReordering = (source?.data as {isReordering?: boolean} | undefined)?.isReordering === true;

    setGlobalDragOperationState({
      isDragging: true,
      sourceIndex,
      isReordering,
    });
  });

  manager.monitor.addEventListener('dragend', () => {
    setGlobalDragOperationState({
      isDragging: false,
      sourceIndex: undefined,
      isReordering: false,
    });
  });
}

/**
 * Props interface of {@link Sortable}
 */
export interface SortableProps extends UseSortableInput {
  /**
   * Handle reference.
   */
  handleRef?: RefObject<HTMLElement | null>;
}

/**
 * Props interface for SortablePresentation
 */
interface SortablePresentationProps {
  children: ReactNode;
  elementStyle: CSSProperties;
}

/**
 * Memoized presentation component for Sortable content.
 * PERFORMANCE FIX: Based on dnd-kit issue #389 - separate presentation from hook
 * This prevents children from re-rendering when useSortable causes parent re-renders.
 * @see https://github.com/clauderic/dnd-kit/issues/389
 *
 * @param props - Props injected to the component.
 * @returns SortablePresentation component.
 */
function SortablePresentation({children, elementStyle}: SortablePresentationProps): ReactElement {
  return <Box sx={{height: '100%', width: '100%', ...elementStyle}}>{children}</Box>;
}

const MemoizedSortablePresentation = memo(SortablePresentation);

/**
 * Sortable component.
 * PERFORMANCE FIX: Uses memoized presentation pattern from dnd-kit issue #389
 * The useSortable hook causes re-renders during drag operations, but by memoizing
 * the children separately, those re-renders become cheap (only the wrapper re-renders).
 *
 * @param props - Props injected to the component.
 * @returns Sortable component.
 */
function Sortable({
  id,
  index,
  children = null,
  handleRef = undefined,
  collisionDetector,
  ...rest
}: PropsWithChildren<SortableProps>) {
  const {ref, sortable, isDragging, isDropTarget} = useSortable({
    collisionDetector: collisionDetector ?? pointerIntersection,
    collisionPriority: CollisionPriority.High,
    handle: handleRef,
    id,
    index,
    modifiers: [RestrictToVerticalAxis],
    ...rest,
  });

  const {source} = useDragOperation();

  useDragOperationMonitorSetup();

  // This only re-renders when drag state actually changes, not on every mouse move
  const {isDragging: isDragActive, isReordering: isReorderingOperation} = useGlobalDragOperationState();

  // Check if this sortable can accept the current draggable
  const canAcceptDrop = useMemo(() => {
    if (!source) {
      return true;
    }
    return sortable.accepts(source);
  }, [source, sortable]);

  // Only show the drop indicator when a NEW item is being dragged in from
  // the resource panel — not during reordering. Reordering relies on
  // dnd-kit's built-in visual feedback (opacity change, snap-to-position).
  const showDropIndicator = useMemo(
    () => isDragActive && isDropTarget && !isDragging && canAcceptDrop && !isReorderingOperation,
    [isDragActive, isDropTarget, isDragging, isReorderingOperation, canAcceptDrop],
  );

  const elementStyle: CSSProperties = useMemo(
    () => ({
      opacity: isDragging ? 0.4 : 1,
      transform: isDragging ? 'scale(1.01)' : 'none',
      // Disable transitions for ALL sortables while a drag is active so
      // sibling elements snap into place instantly instead of lagging behind.
      transition: isDragActive ? 'none' : 'opacity 0.2s ease, transform 0.2s ease',
    }),
    [isDragging, isDragActive],
  );

  const dropIndicatorStyles = useMemo(
    () => ({
      position: 'relative' as const,
      paddingTop: '4px',
      paddingBottom: '4px',
      ...(showDropIndicator && {
        '&::before': {
          content: '""',
          position: 'absolute' as const,
          left: 0,
          right: 0,
          top: 0,
          height: '2px',
          backgroundColor: 'primary.main',
          borderRadius: '1px',
          zIndex: 100,
          pointerEvents: 'none' as const,
        },
      }),
    }),
    [showDropIndicator],
  );

  return (
    <Box ref={ref} sx={dropIndicatorStyles}>
      <MemoizedSortablePresentation elementStyle={elementStyle}>{children}</MemoizedSortablePresentation>
    </Box>
  );
}

export default memo(Sortable);
