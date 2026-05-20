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

import {cx} from '@emotion/css';
import {withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {FC, CSSProperties} from 'react';
import useStyles from './Spinner.styles';
import useTheme from '../../../contexts/Theme/useTheme';

export type SpinnerSize = 'small' | 'medium' | 'large';

export interface SpinnerProps {
  /**
   * Additional CSS class names
   */
  className?: string;
  /**
   * Custom color for the spinner
   */
  color?: string;
  /**
   * Size of the spinner
   */
  size?: SpinnerSize;
  /**
   * Custom styles
   */
  style?: CSSProperties;
}

/**
 * Spinner component for loading states
 *
 * @example
 * ```tsx
 * // Basic spinner
 * <Spinner />
 *
 * // Large spinner with custom color
 * <Spinner size="large" color="#3b82f6" />
 *
 * // Small spinner
 * <Spinner size="small" />
 * ```
 */
const Spinner: FC<SpinnerProps> = ({size = 'medium', color, className, style}: SpinnerProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, size, color);

  const spinnerClassName: string = cx(
    withVendorCSSClassPrefix(bem('spinner')),
    styles['spinner'],
    size === 'small' && styles['spinnerSmall'],
    size === 'medium' && styles['spinnerMedium'],
    size === 'large' && styles['spinnerLarge'],
    className,
  );

  return <span className={spinnerClassName} style={style} role="status" aria-label="Loading" />;
};

export default Spinner;
