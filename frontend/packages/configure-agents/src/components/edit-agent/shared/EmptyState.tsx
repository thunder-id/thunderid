// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

interface EmptyStateProps {
  icon: ReactNode;
  message: string;
}

/**
 * Centered icon + message used for empty lists across the agent edit page (groups, roles,
 * attributes), instead of a plain left-aligned line of text.
 */
export default function EmptyState({icon, message}: EmptyStateProps): JSX.Element {
  return (
    <Stack alignItems="center" spacing={1} sx={{py: 3}}>
      <Box sx={{color: 'text.disabled', display: 'inline-flex'}}>{icon}</Box>
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Stack>
  );
}
