/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {css} from '@emotion/css';
import {Theme} from '@thunderid/browser';
import {useMemo} from 'react';

export type FormControlHelperTextAlign = 'left' | 'center';

/**
 * Creates styles for the FormControl component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param helperTextAlign - The alignment for helper text
 * @param helperTextMarginLeft - Custom margin left for helper text
 * @param hasError - Whether the form control has an error
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  helperTextAlign: FormControlHelperTextAlign,
  helperTextMarginLeft?: string,
  hasError?: boolean,
): Record<string, string> =>
  useMemo(() => {
    const formControl: string = css`
      text-align: start;
      font-family: ${theme.vars.typography.fontFamily};
    `;

    const helperText: string = css`
      margin-top: calc(${theme.vars.spacing.unit} / 2);
      text-align: ${helperTextAlign === 'left' ? 'start' : helperTextAlign};
      ${helperTextMarginLeft && `margin-inline-start: ${helperTextMarginLeft};`}
    `;

    const helperTextError: string = css`
      color: ${theme.vars.colors.error.main};
    `;

    return {
      formControl,
      helperText,
      helperTextError,
    };
  }, [theme, colorScheme, helperTextAlign, helperTextMarginLeft, hasError]);

export default useStyles;
