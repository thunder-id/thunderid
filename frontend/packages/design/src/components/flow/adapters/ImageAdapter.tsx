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

import {resolveLogoUri, type ResolvedLogo} from '@thunderid/react';
import {cn} from '@thunderid/utils';
import {Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import type {FlowComponent} from '../../../models/flow';

interface ImageAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
  maxWidth?: number | string;
  maxHeight?: number | string;
}

const DEFAULT_EMOJI_CONTAINER_HEIGHT = '4em';

export default function ImageAdapter({
  component,
  resolve,
  maxWidth = '100%',
  maxHeight = '100%',
}: ImageAdapterProps): JSX.Element | null {
  const resolvedSrc = resolve(component.src ?? '') ?? component.src ?? '';
  const resolvedAlt = resolve(component.alt ?? '') ?? component.alt ?? '';

  if (!resolvedSrc) return null;

  const resolvedIcon: ResolvedLogo = resolveLogoUri(resolvedSrc, resolvedAlt);

  if (resolvedIcon.kind === 'emoji') {
    const cssWidth = component.width ? `${component.width}px` : '100%';
    const cssHeight = component.height ? `${component.height}px` : 'auto';

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
      <span
        id={component.id}
        className={[cn('Flow--image'), component.classes].filter(Boolean).join(' ')}
        style={{
          containerType: 'size',
          display: 'inline-grid',
          height: containerHeight,
          placeItems: 'center',
          width: cssWidth,
        }}
      >
        <span aria-label={resolvedAlt} role="img" style={{fontSize: '100cqmin', lineHeight: 1}}>
          {resolvedIcon.glyph}
        </span>
      </span>
    );
  }

  return (
    <Box
      component="img"
      id={component.id}
      className={[cn('Flow--image'), component.classes].filter(Boolean).join(' ')}
      src={resolvedIcon.imgSrc}
      alt={resolvedAlt}
      sx={{
        width: component.width ? `${component.width}px` : 'auto',
        height: component.height ? `${component.height}px` : 'auto',
        maxWidth,
        maxHeight,
        objectFit: 'contain',
      }}
    />
  );
}
