// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type UseDraggableInput, useDraggable} from '@dnd-kit/react';
import {Box} from '@wso2/oxygen-ui';
import type {BoxProps} from '@wso2/oxygen-ui';
import {type PropsWithChildren, type ReactElement} from 'react';

/**
 * Props interface of {@link Draggable}
 */
export interface DraggableProps extends UseDraggableInput {
  /**
   * Set of types that the draggable can be dropped into.
   */
  accept: string[];
}

/**
 * Draggable component.
 *
 * @param props - Props injected to the component.
 * @returns Draggable component.
 */
function Draggable({id, children = null, ...rest}: PropsWithChildren<DraggableProps>): ReactElement {
  const {ref} = useDraggable({
    id,
    ...rest,
  });

  return (
    <Box ref={ref as BoxProps['ref']} sx={{height: '100%', width: '100%'}}>
      {children}
    </Box>
  );
}

export default Draggable;
