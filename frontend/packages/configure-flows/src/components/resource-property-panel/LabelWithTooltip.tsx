// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Tooltip} from '@wso2/oxygen-ui';
import {InfoIcon} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement} from 'react';

/**
 * Props interface of {@link LabelWithTooltip}
 */
export interface LabelWithTooltipProps {
  /**
   * The label text.
   */
  label: string;
  /**
   * The explanation shown when the label or its info icon is hovered.
   */
  tooltip: string;
}

/**
 * A label followed by an info icon that explains the field in a tooltip.
 *
 * @param props - Props injected to the component.
 * @returns The LabelWithTooltip component.
 */
function LabelWithTooltip({label, tooltip}: LabelWithTooltipProps): ReactElement {
  return (
    <Tooltip title={tooltip} placement="top">
      <Stack component="span" direction="row" alignItems="center" spacing={0.5} sx={{width: 'fit-content'}}>
        <span>{label}</span>
        <InfoIcon size={14} />
      </Stack>
    </Tooltip>
  );
}

export default LabelWithTooltip;
