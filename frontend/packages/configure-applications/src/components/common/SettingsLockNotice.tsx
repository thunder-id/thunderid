// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';

interface SettingsLockNoticeProps {
  /** Whether the wrapped settings are active. When false, they are shown but frozen. */
  isUnlocked: boolean;
  /** Explains why the settings are frozen and how to unlock them. */
  message: ReactNode;
  children: ReactNode;
}

/**
 * Wraps settings with an info banner and a frozen (dimmed, non-interactive) look when locked,
 * keeping the values legible. Callers must still disable the inputs themselves (e.g. by forcing
 * `isReadOnly`) when `isUnlocked` is false.
 */
export default function SettingsLockNotice({isUnlocked, message, children}: SettingsLockNoticeProps): ReactNode {
  if (isUnlocked) {
    return children;
  }

  return (
    <Box>
      <Alert severity="info" icon={<Lock size={20} />} sx={{mb: 3}}>
        {message}
      </Alert>
      <Box sx={{opacity: 0.6, pointerEvents: 'none', cursor: 'not-allowed'}}>{children}</Box>
    </Box>
  );
}
