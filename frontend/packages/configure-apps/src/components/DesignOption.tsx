// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

export interface DesignOptionProps {
  label: string;
  onClick: () => void;
  selected: boolean;
  thumbnail: ReactNode;
}

/**
 * A theme or layout to choose, as a thumbnail with its name.
 */
export default function DesignOption({label, onClick, selected, thumbnail}: DesignOptionProps): JSX.Element {
  return (
    <ButtonBase
      onClick={onClick}
      aria-pressed={selected}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'stretch',
        borderRadius: 1.5,
        border: 2,
        borderColor: selected ? 'primary.main' : 'divider',
        overflow: 'hidden',
        textAlign: 'start',
        bgcolor: 'background.paper',
        transition: 'border-color 0.15s',
        '&:hover': {borderColor: selected ? 'primary.main' : 'text.disabled'},
      }}
    >
      <Box sx={{aspectRatio: '4 / 3', overflow: 'hidden', pointerEvents: 'none'}}>{thumbnail}</Box>
      <Typography variant="caption" fontWeight={500} noWrap sx={{px: 1, py: 0.75}}>
        {label}
      </Typography>
    </ButtonBase>
  );
}
