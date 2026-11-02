// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDragOperation, type UseDroppableInput} from '@dnd-kit/react';
import {useSortable} from '@dnd-kit/react/sortable';
import {Box} from '@wso2/oxygen-ui';
import {memo, useMemo, type ReactElement} from 'react';

/**
 * Props interface of {@link DropZone}
 */
export interface DropZoneProps {
  id: string;
  index: number;
  position: 'start' | 'end';
  accept?: UseDroppableInput['accept'];
  droppableData?: Record<string, unknown>;
}

/**
 * Sortable drop zone rendered at the edges of a droppable container.
 * Used at the top (position="start") to allow dropping before the first element
 * and at the bottom (position="end") to allow dropping after the last element.
 */
function DropZone({id, index, position, accept = undefined, droppableData = undefined}: DropZoneProps): ReactElement {
  const {ref, sortable, isDropTarget} = useSortable({
    id: `${id}-${position}`,
    index,
    accept,
    data: {...droppableData, [`is${position === 'start' ? 'Start' : 'End'}Zone`]: true, isReordering: true},
  });

  const {source} = useDragOperation();
  const showIndicator = useMemo(
    () => Boolean(source && isDropTarget && sortable.accepts(source)),
    [source, isDropTarget, sortable],
  );

  const indicatorEdge = position === 'start' ? 'bottom' : 'top';

  return (
    <Box
      ref={ref}
      sx={{
        minHeight: position === 'start' ? '20px' : '40px',
        width: '100%',
        flexShrink: 0,
        position: 'relative',
        backgroundColor: showIndicator ? 'rgba(var(--oxygen-palette-success-mainChannel) / 0.1)' : 'transparent',
        transition: 'background-color 0.2s ease',
        ...(showIndicator && {
          [`&::${position === 'start' ? 'after' : 'before'}`]: {
            content: '""',
            position: 'absolute',
            left: 0,
            right: 0,
            [indicatorEdge]: 0,
            height: '3px',
            backgroundColor: 'primary.main',
            borderRadius: '2px',
            zIndex: 100,
            pointerEvents: 'none',
          },
        }),
      }}
    />
  );
}

export default memo(DropZone);
