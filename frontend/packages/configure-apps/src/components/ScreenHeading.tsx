// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

/**
 * The heading of a screen's settings in the builder's right panel: its icon, name and what it covers.
 */
export default function ScreenHeading({
  description,
  icon,
  title,
}: {
  description: string;
  icon: ReactNode;
  title: string;
}): JSX.Element {
  return (
    <Stack spacing={0.5}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{color: 'text.secondary'}}>
        {icon}
        <Typography variant="subtitle2" fontWeight={600} color="text.primary">
          {title}
        </Typography>
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {description}
      </Typography>
    </Stack>
  );
}
