// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Divider, Paper, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode, Ref} from 'react';

export interface ComposerPanelProps {
  title: ReactNode;
  /** A short line under the title. */
  subtitle?: ReactNode;
  /** Controls at the end of the header, such as a mode switch or a close button. */
  actions?: ReactNode;
  /** Leaves out the body padding, for content that sets its own, such as collapsible sections. */
  flush?: boolean;
  children: ReactNode;
  'aria-label'?: string;
  /** Takes the heading, which can then be focused, as when the panel is opened from the card. */
  headingRef?: Ref<HTMLHeadingElement>;
}

/** One panel of the composer: a header with its title and controls, a divider, and the panel's content. */
export default function ComposerPanel({
  title,
  subtitle = undefined,
  actions = undefined,
  flush = false,
  children,
  'aria-label': ariaLabel = undefined,
  headingRef = undefined,
}: ComposerPanelProps): JSX.Element {
  return (
    <Paper
      component="section"
      variant="outlined"
      aria-label={ariaLabel}
      sx={{borderRadius: 2, minWidth: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden'}}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2,
          px: 3,
          py: 2,
          minHeight: 64,
        }}
      >
        <Box sx={{minWidth: 0}}>
          <Typography
            ref={headingRef}
            tabIndex={headingRef ? -1 : undefined}
            variant="subtitle1"
            component="h3"
            sx={{fontWeight: 600, outline: 'none'}}
            noWrap
          >
            {title}
          </Typography>
          {subtitle && (
            <Typography variant="caption" color="text.secondary" component="div" noWrap>
              {subtitle}
            </Typography>
          )}
        </Box>
        {actions && <Box sx={{display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0}}>{actions}</Box>}
      </Box>
      <Divider />
      <Box sx={flush ? undefined : {p: 3}}>{children}</Box>
    </Paper>
  );
}
