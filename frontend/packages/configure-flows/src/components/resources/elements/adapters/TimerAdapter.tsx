// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Typography, Box} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';
import type {Resource} from '../../../../models/resources';

/**
 * Props interface for TimerAdapter
 */
export interface TimerAdapterPropsInterface {
  resource?: Resource;
}

/**
 * A canvas placeholder for the Timer element. Features a simulated timer value
 * based on the configured text value replacing the `{time}` dynamic placeholder.
 *
 * @param props - Custom props containing the resource.
 * @returns The TimerAdapter placeholder component.
 */
function TimerAdapter({resource = undefined}: TimerAdapterPropsInterface): ReactElement {
  // Extract text from resource label, default to generic string if missing
  const templateText = (resource as {label?: string})?.label ?? 'Time remaining: {time}';

  // Replace backend dynamic variable format `{time}` with a fake canvas placeholder `05:00`
  const displayText = templateText.replace('{time}', '05:00');

  return (
    <Box sx={{width: '100%', py: 1}}>
      <Typography variant="body2" color="textSecondary" sx={{fontFamily: 'monospace'}}>
        {displayText}
      </Typography>
    </Box>
  );
}

export default TimerAdapter;
