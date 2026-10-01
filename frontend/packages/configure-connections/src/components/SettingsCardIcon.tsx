// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import {type JSX, type ReactNode} from 'react';

interface SettingsCardIconProps {
  children: ReactNode;
}

export default function SettingsCardIcon({children}: SettingsCardIconProps): JSX.Element {
  return (
    <Box
      sx={{
        width: 30,
        height: 30,
        borderRadius: 1.5,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'action.hover',
        color: 'primary.main',
      }}
    >
      {children}
    </Box>
  );
}
