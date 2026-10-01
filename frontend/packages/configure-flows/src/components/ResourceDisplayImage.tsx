// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Avatar, Box, useColorScheme} from '@wso2/oxygen-ui';
import * as Icons from '@wso2/oxygen-ui-icons-react';
import type {ComponentType, ReactElement} from 'react';
import resolveStaticResourcePath from '../utils/resolveStaticResourcePath';

const isImagePath = (value: string): boolean => value.includes('/') || value.includes(':');

/**
 * Props interface of {@link ResourceDisplayImage}
 */
export interface ResourceDisplayImageProps {
  /**
   * The resource `display.image` value: either a static asset path or an icon name
   * from `@wso2/oxygen-ui-icons-react`.
   */
  image?: string;
  /**
   * Accessible label for the image (usually the resource `display.label`).
   */
  label?: string;
  /**
   * Rendered size in pixels.
   */
  size?: number;
  /**
   * Skip the dark-mode inversion filter, for full-color brand logos.
   */
  preserveColor?: boolean;
}

/**
 * Renders a flow resource's `display.image` the same way everywhere (resource panel
 * items and canvas nodes): static asset paths become an Avatar with dark-mode
 * inversion, anything else is resolved as an icon name from the icon library.
 *
 * @param props - Props injected to the component.
 * @returns The ResourceDisplayImage component, or null when the image cannot be resolved.
 */
function ResourceDisplayImage({
  image = undefined,
  label = undefined,
  size = 20,
  preserveColor = false,
}: ResourceDisplayImageProps): ReactElement | null {
  const {mode, systemMode} = useColorScheme();

  // Determine the effective mode - if mode is 'system', use systemMode
  const effectiveMode = mode === 'system' ? systemMode : mode;

  if (!image) {
    return null;
  }

  if (isImagePath(image)) {
    return (
      <Avatar
        src={resolveStaticResourcePath(image)}
        alt={label}
        variant="square"
        sx={{
          height: size,
          width: size,
          backgroundColor: 'transparent !important',
          color: 'text.primary',
          '& .MuiAvatar-img': {
            filter: effectiveMode === 'dark' && !preserveColor ? 'brightness(0.9) invert(1)' : 'none',
          },
        }}
      />
    );
  }

  const IconComponent = Icons[image as keyof typeof Icons] as ComponentType<{
    size?: number;
    color?: string;
  }>;

  return IconComponent ? (
    <Box display="inline-flex" alignItems="center" justifyContent="center" sx={{color: 'text.primary', flexShrink: 0}}>
      <IconComponent size={size} />
    </Box>
  ) : null;
}

export default ResourceDisplayImage;
