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
import {FC} from 'react';
import useStyles from './Logo.styles';
import useTheme from '../../../contexts/Theme/useTheme';

export type LogoSize = 'small' | 'medium' | 'large';

/**
 * Props for the Logo component.
 */
export interface LogoProps {
  /**
   * Custom alt text for the logo.
   */
  alt?: string;
  /**
   * Custom CSS class name for the logo.
   */
  className?: string;
  /**
   * Size of the logo.
   */
  size?: LogoSize;
  /**
   * Custom logo URL to override theme logo.
   */
  src?: string;
  /**
   * Custom title for the logo.
   */
  title?: string;
}

/**
 * Logo component that displays the brand logo from theme or custom source.
 *
 * @param props - The props for the Logo component.
 * @returns The rendered Logo component.
 */
const Logo: FC<LogoProps> = ({className, src, alt, title, size = 'medium'}: LogoProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, size);

  const logoConfig: Record<string, string> | undefined = theme.images?.logo as Record<string, string> | undefined;

  const logoSrc: string | undefined = src || logoConfig?.['url'];

  const logoAlt: string = alt || logoConfig?.['alt'] || 'Logo';

  const logoTitle: string | undefined = title || logoConfig?.['title'];

  if (!logoSrc) {
    return null;
  }

  return (
    <img
      src={logoSrc}
      alt={logoAlt}
      title={logoTitle}
      className={cx(
        withVendorCSSClassPrefix(bem('logo')),
        withVendorCSSClassPrefix(bem('logo', size)),
        styles['logo'],
        styles['size'],
        className,
      )}
    />
  );
};

export default Logo;
