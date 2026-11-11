// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, type SxProps, type Theme} from '@wso2/oxygen-ui';
import type {JSX, KeyboardEvent, PropsWithChildren} from 'react';

export interface ReadOnlyFieldsetProps extends PropsWithChildren {
  /** Whether the section is locked. */
  readOnly: boolean;
}

/**
 * The native `disabled` covers inputs and buttons. MUI Select triggers and chip delete icons are not form
 * controls, so pointer events are cut on them.
 */
const LOCKED_SX: SxProps<Theme> = {
  border: 0,
  m: 0,
  p: 0,
  minWidth: 0,
  '&:disabled .MuiSelect-select, &:disabled .MuiChip-deleteIcon': {pointerEvents: 'none'},
};

/**
 * Keeps the keyboard from opening a Select or deleting a chip inside a disabled `fieldset`, which the
 * native `disabled` does not reach. Tab still moves focus, so the values stay readable.
 */
function blockLockedControlKeys(event: KeyboardEvent<HTMLFieldSetElement>): void {
  if (
    event.key !== 'Tab' &&
    event.currentTarget.disabled &&
    event.target instanceof Element &&
    event.target.closest('.MuiSelect-select, .MuiChip-root')
  ) {
    event.preventDefault();
    event.stopPropagation();
  }
}

/**
 * Locks every control in a section for an admin who may only view it.
 */
export default function ReadOnlyFieldset({readOnly, children}: ReadOnlyFieldsetProps): JSX.Element {
  return (
    <Box component="fieldset" disabled={readOnly} sx={LOCKED_SX} onKeyDownCapture={blockLockedControlKeys}>
      {children}
    </Box>
  );
}
