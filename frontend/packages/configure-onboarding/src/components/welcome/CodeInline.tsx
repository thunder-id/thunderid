// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

// TODO: Move this to oxygen-ui and use.
export default function CodeInline({children = null}: {children?: ReactNode}): JSX.Element {
  return (
    <Box
      component="code"
      sx={{
        fontFamily: 'monospace',
        fontSize: '0.85em',
        color: 'primary.main',
        bgcolor: 'action.selected',
        borderRadius: 0.5,
        px: 0.5,
      }}
    >
      {children}
    </Box>
  );
}
