// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SxProps, Theme} from '@wso2/oxygen-ui';

/**
 * Shared style for the persistent dashed "add" buttons rendered inside views
 * and form blocks on the flow builder canvas.
 */
const dashedAddButtonSx: SxProps<Theme> = {
  py: 1,
  borderRadius: 1.5,
  border: '1.5px dashed',
  borderColor: 'divider',
  color: 'primary.main',
  fontWeight: 500,
  textTransform: 'none',
  backgroundColor: 'action.hover',
  transition: 'all 0.18s ease',
  '&:hover': {
    border: '1.5px dashed',
    borderColor: 'primary.main',
    color: 'primary.main',
    backgroundColor: 'primary.50',
  },
};

export default dashedAddButtonSx;
