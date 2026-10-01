// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import * as Icons from '@wso2/oxygen-ui-icons-react';
import {type ReactElement, type ComponentType} from 'react';
import type {Element as FlowElement} from '../../../../models/elements';

/**
 * Icon element type with properties at top level.
 */
export type IconElement = FlowElement & {
  name?: string;
  size?: number;
  color?: string;
};

/**
 * Props interface of {@link IconAdapter}
 */
export interface IconAdapterPropsInterface {
  /**
   * The icon element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for rendering icons from @wso2/oxygen-ui-icons-react (Lucide).
 * The icon is selected by name and rendered at the configured size and color.
 *
 * @param props - Props injected to the component.
 * @returns The IconAdapter component.
 */
function IconAdapter({resource}: IconAdapterPropsInterface): ReactElement {
  const iconElement = resource as IconElement;
  const name = iconElement?.name ?? 'User';
  const size = iconElement?.size ?? 24;
  const color = iconElement?.color ?? 'currentColor';

  const IconComponent = (name in Icons ? Icons[name as keyof typeof Icons] : undefined) as
    | ComponentType<{size?: number; color?: string}>
    | undefined;

  if (!IconComponent) {
    return (
      <Box
        display="inline-flex"
        alignItems="center"
        justifyContent="center"
        sx={{
          border: '1px dashed rgba(0, 0, 0, 0.2)',
          borderRadius: 1,
          color: 'text.secondary',
          fontSize: 10,
          height: size,
          padding: '2px 4px',
          width: size,
        }}
      >
        ?
      </Box>
    );
  }

  return <IconComponent size={size} color={color} />;
}

export default IconAdapter;
