// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Checkbox, FormControlLabel, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';

/**
 * Props interface of {@link CheckboxWithHint}
 */
export interface CheckboxWithHintProps {
  /**
   * Whether the checkbox is checked.
   */
  checked: boolean;
  /**
   * The checkbox label.
   */
  label: string;
  /**
   * Explanatory caption shown underneath the label, aligned with it.
   */
  hint?: string;
  /**
   * Change handler receiving the new checked state.
   */
  onChange: (checked: boolean) => void;
}

/**
 * A checkbox row for executor property panels, following the application edit
 * page's toggle pattern: a compact label with the explanation as a small
 * caption underneath, indented to align with the label text.
 *
 * @param props - Props injected to the component.
 * @returns The CheckboxWithHint component.
 */
function CheckboxWithHint({checked, label, hint = undefined, onChange}: CheckboxWithHintProps): ReactElement {
  return (
    <Box>
      <FormControlLabel
        control={<Checkbox checked={checked} onChange={(e) => onChange(e.target.checked)} size="small" />}
        label={<Typography variant="subtitle2">{label}</Typography>}
        sx={{mr: 0}}
      />
      {hint && (
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '38px'}}>
          {hint}
        </Typography>
      )}
    </Box>
  );
}

export default CheckboxWithHint;
