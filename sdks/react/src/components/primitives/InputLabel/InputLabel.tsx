/**
 * Copyright (c) 2024, WSO2 LLC. (https://www.wso2.com).
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

import {cx} from '@emotion/css';
import {bem, withVendorCSSClassPrefix} from '@thunderid/browser';
import {CSSProperties, FC, LabelHTMLAttributes, ReactNode} from 'react';
import useStyles from './InputLabel.styles';
import useTheme from '../../../contexts/Theme/useTheme';

export type InputLabelVariant = 'block' | 'inline';

export interface InputLabelProps extends Omit<LabelHTMLAttributes<HTMLLabelElement>, 'style'> {
  /**
   * Label text or content
   */
  children: ReactNode;
  /**
   * Additional CSS class names
   */
  className?: string;
  /**
   * Whether there's an error state
   */
  error?: boolean;
  /**
   * Custom margin bottom (useful for different form layouts)
   */
  marginBottom?: string;
  /**
   * Whether the field is required
   */
  required?: boolean;
  /**
   * Custom style overrides
   */
  style?: CSSProperties;
  /**
   * Display type for label positioning
   */
  variant?: InputLabelVariant;
}

const InputLabel: FC<InputLabelProps> = ({
  children,
  required = false,
  error = false,
  variant = 'block',
  marginBottom,
  className,
  style = {},
  ...rest
}: InputLabelProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, variant, error, marginBottom);

  return (
    <label
      className={cx(
        withVendorCSSClassPrefix(bem('input-label')),
        withVendorCSSClassPrefix(bem('input-label', variant)),
        styles['label'],
        variant === 'block' ? styles['block'] : styles['inline'],
        {
          [withVendorCSSClassPrefix(bem('input-label', 'error'))]: error,
          [styles['error']]: error,
        },
        className,
      )}
      style={style}
      {...rest}
    >
      {children}
      {required && (
        <span className={cx(withVendorCSSClassPrefix(bem('input-label', 'required')), styles['requiredIndicator'])}>
          {' *'}
        </span>
      )}
    </label>
  );
};

export default InputLabel;
