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
import {FC, JSX, useMemo} from 'react';
import useStyles from './Avatar.styles';
import useTheme from '../../../contexts/Theme/useTheme';

export interface AvatarProps {
  /**
   * Alternative text for the avatar image
   */
  alt?: string;
  /**
   * Background generation strategy
   * - 'random': Generate background color based on ASCII values of the name
   * - 'none': Use default theme background
   * - string: Use custom background color
   * @default 'random'
   */
  background?: 'random' | 'none' | string;
  /**
   * Optional className for the avatar
   */
  className?: string;
  /**
   * The URL of the avatar image
   */
  imageUrl?: string;
  /**
   * Loading state of the avatar
   */
  isLoading?: boolean;
  /**
   * The name to use for generating initials when no image is provided
   */
  name?: string;
  /**
   * The size of the avatar in pixels
   */
  size?: number;
  /**
   * The variant of the avatar shape
   * @default 'circular'
   */
  variant?: 'circular' | 'square';
}

export const Avatar: FC<AvatarProps> = ({
  alt = 'User avatar',
  background = 'random',
  className = '',
  imageUrl,
  name,
  size = 64,
  variant = 'circular',
  isLoading = false,
}: AvatarProps): JSX.Element => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();

  const generateBackgroundColor = (inputString: string): string => {
    const hash: number = inputString.split('').reduce((acc: number, char: string) => {
      const charCode: number = char.charCodeAt(0);

      return ((acc << 5) - acc + charCode) & 0xffffffff;
    }, 0);

    const seed: number = Math.abs(hash);

    const generateColor = (offset: number): string => {
      const hue1: number = (seed + offset) % 360;
      const hue2: number = (hue1 + 60 + (seed % 120)) % 360;

      const saturation: number = 70 + (seed % 20);
      const lightness1: number = 55 + (seed % 15);
      const lightness2: number = 60 + ((seed + offset) % 15);

      return `hsl(${hue1}, ${saturation}%, ${lightness1}%), hsl(${hue2}, ${saturation}%, ${lightness2}%)`;
    };

    const angle: number = 45 + (seed % 91);

    const colors: string = generateColor(seed);
    return `linear-gradient(${angle}deg, ${colors})`;
  };

  const backgroundColor: string | undefined = useMemo(() => {
    if (!name || imageUrl) {
      return undefined;
    }

    if (background === 'random') {
      return generateBackgroundColor(name);
    }

    if (background === 'none') {
      return undefined;
    }

    return background;
  }, [background, name, imageUrl]);

  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme, size, variant, backgroundColor);

  // Determine if we're in the default state (no image, no name, not loading)
  const isDefaultState: boolean = !imageUrl && !name && !isLoading;

  const getInitials = (fullName: string): string =>
    fullName
      .split(' ')
      .map((part: string) => part[0])
      .slice(0, 2)
      .join('')
      .toUpperCase();

  const renderContent = (): JSX.Element | string => {
    if (imageUrl) {
      return (
        <img
          src={imageUrl}
          alt={alt}
          className={cx(withVendorCSSClassPrefix(bem('avatar', 'image')), styles['image'])}
        />
      );
    }

    if (name) {
      return getInitials(name);
    }

    if (isLoading) {
      return <div className={cx(withVendorCSSClassPrefix(bem('avatar', 'skeleton')), styles['skeleton'])} />;
    }

    // Default user icon
    return (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 640 640"
        className={cx(withVendorCSSClassPrefix(bem('avatar', 'icon')), styles['icon'])}
      >
        <path d="M240 192C240 147.8 275.8 112 320 112C364.2 112 400 147.8 400 192C400 236.2 364.2 272 320 272C275.8 272 240 236.2 240 192zM448 192C448 121.3 390.7 64 320 64C249.3 64 192 121.3 192 192C192 262.7 249.3 320 320 320C390.7 320 448 262.7 448 192zM144 544C144 473.3 201.3 416 272 416L368 416C438.7 416 496 473.3 496 544L496 552C496 565.3 506.7 576 520 576C533.3 576 544 565.3 544 552L544 544C544 446.8 465.2 368 368 368L272 368C174.8 368 96 446.8 96 544L96 552C96 565.3 106.7 576 120 576C133.3 576 144 565.3 144 552L144 544z" />
      </svg>
    );
  };

  return (
    <div
      className={cx(
        withVendorCSSClassPrefix(bem('avatar')),
        styles['avatar'],
        styles['variant'],
        withVendorCSSClassPrefix(bem('avatar', null, variant)),
        isDefaultState && withVendorCSSClassPrefix(bem('avatar', 'default')),
        className,
      )}
    >
      {renderContent()}
    </div>
  );
};

export default Avatar;
