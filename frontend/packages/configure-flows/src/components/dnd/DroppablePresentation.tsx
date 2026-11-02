// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, type BoxProps} from '@wso2/oxygen-ui';
import {memo, type ReactElement, type ReactNode} from 'react';

/**
 * Props interface for DroppablePresentation
 */
export interface DroppablePresentationProps {
  children: ReactNode;
  className?: string;
  sx?: BoxProps['sx'];
}

/**
 * Memoized presentation component for Droppable content.
 * PERFORMANCE FIX: Based on dnd-kit issue #389 - separate presentation from hook
 * This prevents children from re-rendering when useDroppable causes parent re-renders.
 * @see https://github.com/clauderic/dnd-kit/issues/389
 *
 * @param props - Props injected to the component.
 * @returns DroppablePresentation component.
 */
function DroppablePresentation({children, className = undefined, sx = {}}: DroppablePresentationProps): ReactElement {
  return (
    <Box
      className={className}
      sx={{
        display: 'inline-flex',
        flexDirection: 'column',
        height: '100%',
        width: '100%',
        ...sx,
      }}
    >
      {children}
    </Box>
  );
}

export default memo(DroppablePresentation);
