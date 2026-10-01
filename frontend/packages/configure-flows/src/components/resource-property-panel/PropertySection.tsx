// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactElement, ReactNode} from 'react';

/**
 * Props interface of {@link PropertySection}
 */
export interface PropertySectionPropsInterface {
  /**
   * Section heading.
   */
  title: string;
  /**
   * Fields belonging to the section.
   */
  children: ReactNode;
}

/**
 * Groups related properties under a quiet heading so identity, content, layout and
 * validation settings are scannable instead of being one flat list of fields.
 *
 * @param props - Props injected to the component.
 * @returns The PropertySection component.
 */
function PropertySection({title, children}: PropertySectionPropsInterface): ReactElement {
  return (
    <Box component="section" sx={{'& + &': {mt: 1}}}>
      <Typography
        variant="overline"
        color="text.secondary"
        sx={{display: 'block', letterSpacing: '0.08em', lineHeight: 1.6, mb: 0.5}}
      >
        {title}
      </Typography>
      <Stack gap={2}>{children}</Stack>
    </Box>
  );
}

export default PropertySection;
