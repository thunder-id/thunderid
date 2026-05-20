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
import {FC, HTMLAttributes, ReactNode} from 'react';
import useStyles from './Divider.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import Typography from '../Typography/Typography';

export type DividerOrientation = 'horizontal' | 'vertical';
export type DividerVariant = 'solid' | 'dashed' | 'dotted';

export interface DividerProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Text to display in the center of the divider
   */
  children?: ReactNode;

  /**
   * Custom color for the divider
   */
  color?: string;

  /**
   * The orientation of the divider
   */
  orientation?: DividerOrientation;

  /**
   * The variant style of the divider
   */
  variant?: DividerVariant;
}

/**
 * Divider component for separating content sections.
 *
 * @example
 * ```tsx
 * // Basic horizontal divider
 * <Divider />
 *
 * // Divider with text
 * <Divider>OR</Divider>
 *
 * // Vertical divider
 * <Divider orientation="vertical" />
 *
 * // Custom styled divider
 * <Divider variant="dashed" color="#ccc">Continue with</Divider>
 * ```
 */
const Divider: FC<DividerProps> = ({
  orientation = 'horizontal',
  variant = 'solid',
  children,
  color,
  className,
  style,
  ...rest
}: DividerProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, orientation, variant, color, !!children);

  if (orientation === 'vertical') {
    return (
      <div
        className={cx(
          withVendorCSSClassPrefix(bem('divider')),
          withVendorCSSClassPrefix(bem('divider', 'vertical')),
          styles['divider'],
          styles['vertical'],
          className,
        )}
        style={style}
        role="separator"
        aria-orientation="vertical"
        {...rest}
      />
    );
  }

  if (children) {
    return (
      <div
        className={cx(
          withVendorCSSClassPrefix(bem('divider')),
          withVendorCSSClassPrefix(bem('divider', 'horizontal')),
          withVendorCSSClassPrefix(bem('divider', 'with-text')),
          styles['divider'],
          styles['horizontal'],
          className,
        )}
        style={style}
        role="separator"
        aria-orientation="horizontal"
        {...rest}
      >
        <div className={cx(withVendorCSSClassPrefix(bem('divider', 'line')), styles['line'])} />
        <Typography
          variant="body2"
          color="textSecondary"
          className={cx(withVendorCSSClassPrefix(bem('divider', 'text')), styles['text'])}
          inline
        >
          {children}
        </Typography>
        <div className={cx(withVendorCSSClassPrefix(bem('divider', 'line')), styles['line'])} />
      </div>
    );
  }

  return (
    <div
      className={cx(
        withVendorCSSClassPrefix(bem('divider')),
        withVendorCSSClassPrefix(bem('divider', 'horizontal')),
        styles['divider'],
        styles['horizontal'],
        className,
      )}
      style={style}
      role="separator"
      aria-orientation="horizontal"
      {...rest}
    />
  );
};

export default Divider;
