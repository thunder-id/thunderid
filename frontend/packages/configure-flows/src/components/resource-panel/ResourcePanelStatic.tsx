// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type HTMLAttributes, type ReactElement} from 'react';
import ResourcePanelItem, {type ResourcePanelItemProps} from './ResourcePanelItem';

/**
 * Props interface of {@link ResourcePanelStatic}
 */
export type ResourcePanelStaticPropsInterface = ResourcePanelItemProps &
  Omit<HTMLAttributes<HTMLDivElement>, 'resource'>;

/**
 * Static item for the resource panel.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePanelStatic component.
 */
function ResourcePanelStatic({
  id,
  resource,
  type = 'static',
  disabled = false,
  ...rest
}: ResourcePanelStaticPropsInterface): ReactElement {
  return <ResourcePanelItem id={id} resource={resource} type={type} disabled={disabled} {...rest} />;
}

export default ResourcePanelStatic;
