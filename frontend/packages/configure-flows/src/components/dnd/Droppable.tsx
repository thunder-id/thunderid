// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {pointerIntersection} from '@dnd-kit/collision';
import {useDroppable, useDragOperation, type UseDroppableInput} from '@dnd-kit/react';
import {Box, type BoxProps} from '@wso2/oxygen-ui';
import {memo, type PropsWithChildren, type ReactElement, useMemo, Children} from 'react';
import DroppablePresentation from './DroppablePresentation';
import DropZone from './DropZone';

// Module-scoped cache for sticky "target inside" state per droppable.
// When the dnd-kit target becomes null (pointer over non-sortable elements),
// we return the last known value so the drop highlight doesn't flicker.
const droppableInsideState = new Map<string, boolean>();

/**
 * Props interface of {@link Droppable}
 */
export type DroppableProps = UseDroppableInput<Record<string, unknown>> &
  BoxProps & {
    /** When true, hides the top and bottom drop zone sortables. */
    hideDropZones?: boolean;
  };

/**
 * Droppable component.
 * PERFORMANCE FIX: Uses memoized presentation pattern from dnd-kit issue #389
 * The useDroppable hook causes re-renders during drag operations, but by memoizing
 * the children separately, those re-renders become cheap (only the wrapper re-renders).
 *
 * @param props - Props injected to the component.
 * @returns Droppable component.
 */
function Droppable({
  id,
  children = null,
  sx = {},
  className,
  collisionDetector = pointerIntersection,
  data,
  accept,
  hideDropZones = false,
  ...rest
}: PropsWithChildren<DroppableProps>): ReactElement {
  const {ref, droppable, isDropTarget} = useDroppable<Record<string, unknown>>({
    accept,
    collisionDetector,
    data,
    id,
    ...rest,
  });

  const {source, target} = useDragOperation();
  const count = useMemo(() => Children.count(children), [children]);

  const canAcceptDrop = useMemo(() => {
    if (!source) return true;
    return droppable.accepts(source);
  }, [source, droppable]);

  // Track whether the target is inside this droppable. When `target` moves to
  // an ancestor (e.g. pointer over a non-sortable child like an input field),
  // keep the previous value so the highlight doesn't flicker off — but only
  // while this droppable is still considered a drop target by dnd-kit.
  const isTargetInside = useMemo(() => {
    const key = String(id);

    if (!source) {
      droppableInsideState.delete(key);
      return false;
    }
    if (!target || !droppable.element) {
      return droppableInsideState.get(key) ?? false;
    }

    const targetEl = (target as {element?: Element}).element;
    if (!targetEl) {
      return droppableInsideState.get(key) ?? false;
    }

    const inside = targetEl === droppable.element || droppable.element.contains(targetEl);

    if (!inside && targetEl.contains(droppable.element)) {
      // Target moved to an ancestor (e.g. canvas). Only keep cached state if
      // this droppable is still detected as a target — otherwise the pointer
      // has genuinely left this droppable's bounds (e.g. moved to parent padding).
      if (!isDropTarget) {
        droppableInsideState.set(key, false);
        return false;
      }
      return droppableInsideState.get(key) ?? false;
    }

    droppableInsideState.set(key, inside);
    return inside;
  }, [id, source, target, droppable.element, isDropTarget]);

  const isReordering = (source?.data as {isReordering?: boolean} | undefined)?.isReordering === true;

  const showDropHighlight = useMemo(
    () => Boolean(source && !isReordering && (isDropTarget || isTargetInside)),
    [source, isReordering, isDropTarget, isTargetInside],
  );

  const dropStyles = useMemo(() => {
    if (!showDropHighlight) return {};

    if (canAcceptDrop) {
      return {
        backgroundColor: 'rgba(var(--oxygen-palette-success-mainChannel) / 0.1)',
        border: '2px dashed',
        borderColor: 'success.light',
      };
    }

    return {
      backgroundColor: 'rgba(var(--mui-palette-error-mainChannel) / 0.1)',
      border: '2px dashed',
      borderColor: 'error.light',
    };
  }, [showDropHighlight, canAcceptDrop]);

  return (
    <Box
      ref={ref as BoxProps['ref']}
      className={className}
      data-droppable
      sx={{
        display: 'inline-flex',
        flexDirection: 'column',
        height: '100%',
        width: '100%',
        borderRadius: 'calc(2 * var(--oxygen-shape-borderRadius, 4px))',
        ...dropStyles,
        // When a nested droppable is also active, suppress the parent's highlight
        // to avoid visual doubling. Use transparent border to prevent layout shift.
        '&[data-drop-active]:has([data-drop-active])': {
          backgroundColor: 'transparent',
          borderColor: 'transparent',
        },
      }}
      {...(showDropHighlight ? {'data-drop-active': ''} : {})}
    >
      <DroppablePresentation sx={sx}>{children}</DroppablePresentation>
      {!hideDropZones && <DropZone id={id} index={count} position="end" accept={accept} droppableData={data} />}
    </Box>
  );
}

export default memo(Droppable);
