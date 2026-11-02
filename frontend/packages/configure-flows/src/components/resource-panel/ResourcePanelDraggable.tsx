// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type HTMLAttributes, type ReactElement} from 'react';
import ResourcePanelItem, {type ResourcePanelItemProps} from './ResourcePanelItem';
import Draggable from '../dnd/Draggable';

/**
 * Props interface of {@link ResourcePanelDraggable}
 */
export type ResourcePanelDraggablePropsInterface = ResourcePanelItemProps &
  Omit<HTMLAttributes<HTMLDivElement>, 'resource'>;

/**
 * Draggable item for the resource panel.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePanelDraggable component.
 */
function ResourcePanelDraggable({
  id,
  resource,
  onAdd,
  type = 'draggable',
  disabled = false,
  ...rest
}: ResourcePanelDraggablePropsInterface): ReactElement {
  return (
    <Draggable
      id={id!}
      data={{dragged: resource}}
      type={resource.type}
      accept={[resource.type]}
      disabled={disabled}
      {...rest}
    >
      <ResourcePanelItem resource={resource} type={type} onAdd={onAdd} disabled={disabled} />
    </Draggable>
  );
}

export default ResourcePanelDraggable;
