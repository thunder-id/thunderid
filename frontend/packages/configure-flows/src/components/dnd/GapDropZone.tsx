// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDroppable, useDragOperation, type UseDroppableInput} from '@dnd-kit/react';
import {Box} from '@wso2/oxygen-ui';
import {memo, useMemo, type ReactElement} from 'react';

/**
 * Props interface of {@link GapDropZone}
 */
export interface GapDropZoneProps {
  id: string;
  accept?: UseDroppableInput['accept'];
  data?: Record<string, unknown>;
}

/**
 * A thin drop zone placed between sortable elements to provide
 * reliable drop targets in gaps where sortable collision detection
 * may not reach (e.g. inside React Flow transformed nodes).
 */
function GapDropZone({id, accept = undefined, data = undefined}: GapDropZoneProps): ReactElement {
  const {ref, droppable, isDropTarget} = useDroppable({
    id,
    accept,
    data,
  });

  const {source} = useDragOperation();

  const canAccept = useMemo(() => {
    if (!source) return false;
    return droppable.accepts(source);
  }, [source, droppable]);

  const showIndicator = Boolean(source && isDropTarget && canAccept);

  return (
    <Box
      ref={ref}
      sx={{
        minHeight: showIndicator ? '12px' : '8px',
        width: '100%',
        position: 'relative',
        transition: 'min-height 0.15s ease',
        ...(showIndicator && {
          '&::before': {
            content: '""',
            position: 'absolute',
            left: 0,
            right: 0,
            top: '50%',
            transform: 'translateY(-50%)',
            height: '2px',
            backgroundColor: 'primary.main',
            borderRadius: '1px',
            zIndex: 100,
            pointerEvents: 'none',
          },
        }),
      }}
    />
  );
}

export default memo(GapDropZone);
