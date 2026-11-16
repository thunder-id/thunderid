// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

/**
 * One step of a {@link GuideStepList}.
 *
 * @public
 */
export interface GuideStepItem {
  /** A key unique among the steps */
  key: string;
  /** The step's heading */
  title: ReactNode;
  /** What the step says to do */
  content: ReactNode;
}

/**
 * Numbered steps, all open, each a heading followed by what to do.
 *
 * @public
 */
export default function GuideStepList({steps}: {steps: GuideStepItem[]}): JSX.Element {
  return (
    <Stack component="ol" spacing={3.5} sx={{listStyle: 'none', m: 0, p: 0}}>
      {steps.map((step, index) => (
        <Box
          key={step.key}
          component="li"
          sx={{display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr)', columnGap: 2, alignItems: 'start'}}
        >
          <Box
            aria-hidden
            sx={{
              width: 24,
              height: 24,
              borderRadius: '50%',
              bgcolor: 'action.selected',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              typography: 'caption',
            }}
          >
            {index + 1}
          </Box>
          <Stack spacing={1.5} sx={{minWidth: 0}}>
            <Typography variant="subtitle1" component="h3">
              {step.title}
            </Typography>
            {step.content}
          </Stack>
        </Box>
      ))}
    </Stack>
  );
}
