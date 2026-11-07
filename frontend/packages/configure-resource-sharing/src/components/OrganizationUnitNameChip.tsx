// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

export interface OrganizationUnitNameChipProps {
  /**
   * The organization unit's name. `<Trans>` passes the interpolated name in as children.
   */
  children?: ReactNode;
}

/**
 * An organization unit's name set inline in a sentence, so it stands apart from the text around it.
 */
export default function OrganizationUnitNameChip({children = undefined}: OrganizationUnitNameChipProps): JSX.Element {
  return (
    <Chip
      component="span"
      size="small"
      variant="filled"
      label={children}
      sx={{verticalAlign: 'middle', maxWidth: '100%'}}
    />
  );
}
