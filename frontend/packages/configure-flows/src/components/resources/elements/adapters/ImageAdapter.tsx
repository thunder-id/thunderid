// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isI18nTemplatePattern, isMetaTemplatePattern} from '@thunderid/utils';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ImageIcon} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type CSSProperties, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element as FlowElement} from '../../../../models/elements';

/**
 * Image element type with properties at top level.
 */
export type ImageElement = FlowElement & {
  src?: string;
  alt?: string;
  width?: string;
  height?: string;
  styles?: CSSProperties;
};

/**
 * Props interface of {@link ImageAdapter}
 */
export interface ImageAdapterPropsInterface {
  /**
   * The image element properties.
   */
  resource: FlowElement;
}

/**
 * Adapter for displaying images.
 *
 * @param props - Props injected to the component.
 * @returns The ImageAdapter component.
 */
function ImageAdapter({resource}: ImageAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();
  const imageElement = resource as ImageElement;
  const [hasError, setHasError] = useState(false);

  // Check if src is empty or has errored
  const src = imageElement?.src?.trim() ?? '';
  const isDynamicSrc = isMetaTemplatePattern(src) || isI18nTemplatePattern(src);
  const shouldShowPlaceholder = !src || hasError;

  const handleImageError = useCallback(() => {
    setHasError(true);
  }, []);

  // Placeholder UI
  const placeholderUI = (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      sx={{
        width: '100%',
        minHeight: 120,
        backgroundColor: 'rgba(0, 0, 0, 0.04)',
        borderRadius: 1,
        border: '1px dashed rgba(0, 0, 0, 0.2)',
      }}
    >
      <ImageIcon size={32} />
      <Typography variant="body2" color="textSecondary" sx={{mt: 1}}>
        {t('flows:core.placeholders.image')}
      </Typography>
    </Box>
  );

  if (isDynamicSrc) {
    return (
      <Box
        display="flex"
        flexDirection="column"
        alignItems="center"
        justifyContent="center"
        sx={{
          width: '100%',
          minHeight: 120,
          backgroundColor: 'background.default',
          borderRadius: 1,
          border: '1px dashed',
          borderColor: 'palette.divider',
        }}
      >
        <ImageIcon size={32} />
        <Typography
          variant="caption"
          color="textSecondary"
          sx={{mt: 1, fontStyle: 'italic', textAlign: 'center', px: 1}}
        >
          {t('flows:core.placeholders.image.dynamicSrc')}
        </Typography>
      </Box>
    );
  }

  if (shouldShowPlaceholder) {
    return placeholderUI;
  }

  return (
    <Box display="flex" alignItems="center" justifyContent="center">
      <img
        src={src}
        alt={imageElement?.alt}
        width={imageElement?.width ?? '100%'}
        height={imageElement?.height}
        style={imageElement?.styles}
        onError={handleImageError}
      />
    </Box>
  );
}

export default ImageAdapter;
