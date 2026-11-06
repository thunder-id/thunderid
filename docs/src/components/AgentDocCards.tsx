// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import React from 'react';

export function AgentDocCards({children}: {children: React.ReactNode}): React.ReactElement {
  return (
    <Box
      component="ul"
      role="list"
      sx={{
        display: 'grid',
        gap: '1rem',
        gridTemplateColumns: {xs: 'minmax(0, 1fr)', sm: 'repeat(2, minmax(0, 1fr))'},
        listStyle: 'none',
        margin: '1.5rem 0',
        padding: 0,
      }}
    >
      {children}
    </Box>
  );
}

export function AgentDocCard({
  title,
  icon,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
}): React.ReactElement {
  return (
    <Box
      component="li"
      sx={{
        background: 'var(--ifm-background-surface-color)',
        border: '1px solid var(--ifm-color-emphasis-300)',
        borderRadius: '12px',
        margin: 0,
        minWidth: 0,
        overflowWrap: 'anywhere',
        padding: '1rem 1.25rem',
      }}
    >
      <Box component="p" sx={{alignItems: 'center', display: 'flex', gap: '0.65rem', marginBottom: '0.65rem'}}>
        <Box
          component="span"
          aria-hidden="true"
          sx={{color: 'var(--ifm-color-primary)', display: 'inline-flex', flexShrink: 0}}
        >
          {icon}
        </Box>
        <strong>{title}</strong>
      </Box>
      <Box sx={{'& > :last-child': {marginBottom: 0}}}>{children}</Box>
    </Box>
  );
}
