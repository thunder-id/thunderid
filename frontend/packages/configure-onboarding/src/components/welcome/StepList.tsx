// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

export interface StepListProps {
  steps: ReactNode[];
  startFrom?: number;
}

export default function StepList({steps, startFrom = 1}: StepListProps): JSX.Element {
  return (
    <Stack spacing={1} component="ol" sx={{pl: 0, m: 0, listStyle: 'none'}}>
      {steps.map((step, i) => (
        <Stack
          // eslint-disable-next-line react/no-array-index-key
          key={i}
          component="li"
          direction="row"
          spacing={1.5}
          alignItems="flex-start"
        >
          <Box
            sx={{
              width: 20,
              height: 20,
              borderRadius: '50%',
              bgcolor: 'action.selected',
              color: 'text.secondary',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '0.7rem',
              fontWeight: 700,
              flexShrink: 0,
              mt: 0.15,
            }}
          >
            {startFrom + i}
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{flex: 1}}>
            {step}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}
