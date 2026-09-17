// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX, ReactNode} from 'react';
import {useInk} from './theme';

/**
 * One numbered step: the circle, the connector down to the next step, and a
 * content column.
 *
 * Every step on an entry's page and in its guides draws through this, so the
 * two are the same treatment rather than two that happen to look alike.
 */
export default function StepShell({
  index,
  total,
  title,
  accent,
  children = undefined,
}: {
  index: number;
  total: number;
  title: ReactNode;
  accent: string;
  children?: ReactNode;
}): JSX.Element {
  const ink = useInk();
  const isLast = index === total - 1;

  return (
    <Box sx={{display: 'flex', gap: 2.25, pb: isLast ? 0 : 3.5}}>
      <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0}}>
        <Box
          component="span"
          sx={{
            width: 26,
            height: 26,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderRadius: '50%',
            border: '1px solid',
            borderColor: `color-mix(in srgb, ${accent} 40%, transparent)`,
            bgcolor: `color-mix(in srgb, ${accent} 10%, transparent)`,
            color: accent,
            fontFamily: 'monospace',
            fontSize: '11.5px',
          }}
        >
          {index + 1}
        </Box>
        {!isLast && (
          <Box
            sx={{
              flex: 1,
              width: '1px',
              minHeight: 20,
              mt: 1,
              background: `linear-gradient(180deg, color-mix(in srgb, ${accent} 30%, transparent), ${ink(0.04, 0.04)})`,
            }}
          />
        )}
      </Box>

      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography
          component="h3"
          sx={{fontSize: '15px', fontWeight: 600, letterSpacing: '-0.01em', color: 'text.primary', mt: '2px', mb: 0.75}}
        >
          {title}
        </Typography>
        {children}
      </Box>
    </Box>
  );
}
