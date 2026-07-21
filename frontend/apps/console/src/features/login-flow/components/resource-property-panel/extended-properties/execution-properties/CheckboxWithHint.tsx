/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
