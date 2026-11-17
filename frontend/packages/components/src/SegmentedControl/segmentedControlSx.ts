// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A segmented control's look: a bordered, rounded track holding the options, the chosen one filled in.
 * Used for every choice a guide offers, so they read as one family.
 *
 * @public
 */
const SEGMENTED_CONTROL_SX = {
  alignSelf: 'flex-start',
  flexWrap: 'wrap',
  p: 0.5,
  gap: 0.5,
  border: 1,
  borderColor: 'divider',
  // The theme's corner radius, as the cards around it use.
  borderRadius: 1,
  '& .MuiToggleButtonGroup-grouped, & .MuiToggleButtonGroup-grouped:not(:first-of-type)': {
    gap: 1,
    m: 0,
    px: 2,
    py: 0.75,
    border: 0,
    borderRadius: 1,
    textTransform: 'none',
    color: 'text.secondary',
  },
  '& .MuiToggleButtonGroup-grouped.Mui-selected, & .MuiToggleButtonGroup-grouped.Mui-selected:hover': {
    bgcolor: 'action.selected',
    color: 'text.primary',
  },
} as const;

export default SEGMENTED_CONTROL_SX;
