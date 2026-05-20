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

import {EmbeddedFlowTextVariant} from '@thunderid/react';
import type {TypographyVariant} from '@wso2/oxygen-ui';

/**
 * Maps EmbeddedFlowTextVariant enum values to corresponding MUI Typography variants
 * for consistent text styling across embedded flow components.
 *
 * @param variant - The EmbeddedFlowTextVariant to map
 * @returns The corresponding MUI TypographyVariant
 *
 * @example
 * ```tsx
 * import {mapEmbeddedFlowTextVariant} from '@thunderid/design';
 *
 * const variant = mapEmbeddedFlowTextVariant(EmbeddedFlowTextVariant.Heading1);
 * // Returns 'h2'
 *
 * <Typography variant={variant}>
 *   My Heading
 * </Typography>
 * ```
 */
export function mapEmbeddedFlowTextVariant(variant: EmbeddedFlowTextVariant | string | undefined): TypographyVariant {
  switch (variant) {
    case EmbeddedFlowTextVariant.Heading1:
      return 'h1';
    case EmbeddedFlowTextVariant.Heading2:
      return 'h2';
    case EmbeddedFlowTextVariant.Heading3:
      return 'h3';
    case EmbeddedFlowTextVariant.Heading4:
      return 'h4';
    case EmbeddedFlowTextVariant.Heading5:
      return 'h5';
    case EmbeddedFlowTextVariant.Heading6:
      return 'h6';
    case EmbeddedFlowTextVariant.Subtitle1:
      return 'subtitle1';
    case EmbeddedFlowTextVariant.Subtitle2:
      return 'subtitle2';
    case EmbeddedFlowTextVariant.Body1:
      return 'body1';
    case EmbeddedFlowTextVariant.Body2:
      return 'body2';
    case EmbeddedFlowTextVariant.Caption:
      return 'caption';
    case EmbeddedFlowTextVariant.Overline:
      return 'overline';
    default:
      // Default fallback for unknown or undefined variants
      return 'body1';
  }
}

export default mapEmbeddedFlowTextVariant;
