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

import type {TypographyProps} from '@wso2/oxygen-ui';
import {TypographyVariants} from '../models/elements';

/**
 * Maps flow typography variant names to Material UI typography variant names.
 */
export const VARIANT_TO_MUI_MAP: Record<string, TypographyProps['variant']> = {
  [TypographyVariants.H1]: 'h1',
  [TypographyVariants.H2]: 'h2',
  [TypographyVariants.H3]: 'h3',
  [TypographyVariants.H4]: 'h4',
  [TypographyVariants.H5]: 'h5',
  [TypographyVariants.H6]: 'h6',
  [TypographyVariants.Body1]: 'body1',
  [TypographyVariants.Body2]: 'body2',
};
