// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

export interface SelectionCardProps {
  /** Shown at the end, such as a button to choose another. */
  action?: ReactNode;
  caption: string;
  /** Whether the choice comes from elsewhere and is not made here. */
  linked?: boolean;
  thumbnail: ReactNode;
  title: string;
}

/**
 * A theme or layout in use, as a thumbnail with its name.
 */
export default function SelectionCard({
  action = undefined,
  caption,
  linked = false,
  thumbnail,
  title,
}: SelectionCardProps): JSX.Element {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1.5}
      sx={{
        p: 1,
        border: 1,
        borderStyle: linked ? 'dashed' : 'solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        bgcolor: linked ? 'action.hover' : 'background.paper',
      }}
    >
      <Box
        sx={{
          width: 72,
          flexShrink: 0,
          aspectRatio: '4 / 3',
          borderRadius: 1,
          overflow: 'hidden',
          pointerEvents: 'none',
        }}
      >
        {thumbnail}
      </Box>
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="body2" fontWeight={500} noWrap>
          {title}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {caption}
        </Typography>
      </Box>
      {action}
    </Stack>
  );
}
