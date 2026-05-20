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

import {extractEmojiFromUri, isEmojiUri} from '@thunderid/browser';
import {CSSProperties, FC, SyntheticEvent} from 'react';
import useTheme from '../../contexts/Theme/useTheme';
import {AdapterProps} from '../../models/adapters';

const DEFAULT_EMOJI_CONTAINER_HEIGHT = '4em';

/**
 * Image component for sign-up forms.
 */
const ImageComponent: FC<AdapterProps> = ({component}: AdapterProps) => {
  const {theme} = useTheme();
  const config: Record<string, unknown> = component.config || {};
  const src: string = (config['src'] as string) || '';
  const alt: string = (config['alt'] as string) || (config['label'] as string) || 'Image';
  const width: string = (config['width'] as string) || '100%';
  const height: string = (config['height'] as string) || 'auto';
  const variant: string = component.variant?.toLowerCase() || 'image_block';

  const imageStyle: CSSProperties = {
    borderRadius: theme.vars.borderRadius.small,
    display: 'block',
    margin: variant === 'image_block' ? '1rem auto' : '0',
  };

  if (!src) {
    return null;
  }

  if (isEmojiUri(src)) {
    // Bare numbers (e.g. "48") are valid for <img> width/height attributes but
    // are unit-less and ignored as CSS properties — normalize them to px.
    const toCSSLength = (value: string): string => (/^\d+(\.\d+)?$/.test(value) ? `${value}px` : value);
    const cssWidth: string = toCSSLength(width);
    const cssHeight: string = toCSSLength(height);

    // container-type: size needs a concrete block dimension — percentage and
    // 'auto' values both collapse to 0 when the parent has no defined height.
    // Priority: explicit height → explicit width (square) → fallback constant.
    const isConcrete = (v: string): boolean => v !== 'auto' && !v.endsWith('%');
    let containerHeight: string;
    if (isConcrete(cssHeight)) {
      containerHeight = cssHeight;
    } else if (isConcrete(cssWidth)) {
      containerHeight = cssWidth;
    } else {
      containerHeight = DEFAULT_EMOJI_CONTAINER_HEIGHT;
    }

    return (
      <div key={component.id} style={{textAlign: 'center'}}>
        {/*
         * container-type: size lets the inner span use cqmin (= min(cqw, cqh))
         * so the emoji font-size tracks the rendered container dimensions
         * rather than the parent's font-size.
         */}
        <span
          style={{
            ...imageStyle,
            containerType: 'size',
            display: 'inline-grid',
            height: containerHeight,
            placeItems: 'center',
            width: cssWidth,
          }}
        >
          <span aria-label={alt} role="img" style={{fontSize: '100cqmin', lineHeight: 1}}>
            {extractEmojiFromUri(src)}
          </span>
        </span>
      </div>
    );
  }

  return (
    <div key={component.id} style={{textAlign: 'center'}}>
      <img
        src={src}
        alt={alt}
        height={height}
        width={width}
        style={imageStyle}
        onError={(e: SyntheticEvent<HTMLImageElement>): void => {
          // Hide broken images
          e.currentTarget.style.display = 'none';
        }}
      />
    </div>
  );
};

export default ImageComponent;
